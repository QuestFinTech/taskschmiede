// Copyright 2026 Quest Financial Technologies S.à r.l.-S., Luxembourg
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// Consent document type constants.
const (
	ConsentTerms          = "terms"
	ConsentPrivacy        = "privacy"
	ConsentDPA            = "dpa"
	ConsentAgeDeclaration = "age_declaration"
)

// consentPolicyKeys maps document types to their corresponding policy table keys.
var consentPolicyKeys = map[string]string{
	ConsentTerms:   "legal.terms_version",
	ConsentPrivacy: "legal.privacy_version",
	ConsentDPA:     "legal.dpa_version",
}

// Consent represents a legal consent record.
type Consent struct {
	ID              string
	UserID          string
	DocumentType    string
	DocumentVersion string
	DocumentURL     string
	AcceptedAt      time.Time
	IPAddress       string
	UserAgent       string
	CreatedAt       time.Time
}

// CreateConsent records that a user has accepted a specific document version.
func (db *DB) CreateConsent(userID, docType, docVersion, docURL, ipAddress, userAgent string) (*Consent, error) {
	id := generateID("con")
	now := UTCNow()

	var docURLPtr *string
	if docURL != "" {
		docURLPtr = &docURL
	}
	var ipPtr *string
	if ipAddress != "" {
		ipPtr = &ipAddress
	}
	var uaPtr *string
	if userAgent != "" {
		uaPtr = &userAgent
	}

	_, err := db.Exec(
		`INSERT INTO consent (id, user_id, document_type, document_version, document_url, accepted_at, ip_address, user_agent, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, docType, docVersion, docURLPtr,
		now.Format(time.RFC3339), ipPtr, uaPtr, now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert consent: %w", err)
	}

	return &Consent{
		ID:              id,
		UserID:          userID,
		DocumentType:    docType,
		DocumentVersion: docVersion,
		DocumentURL:     docURL,
		AcceptedAt:      now,
		IPAddress:       ipAddress,
		UserAgent:       userAgent,
		CreatedAt:       now,
	}, nil
}

// GetLatestConsent returns the most recent consent record for a user and document type.
// Returns nil if no consent exists.
func (db *DB) GetLatestConsent(userID, docType string) (*Consent, error) {
	var c Consent
	var docURL, ipAddress, userAgent sql.NullString
	var acceptedAt, createdAt string

	err := db.QueryRow(
		`SELECT id, user_id, document_type, document_version, document_url, accepted_at, ip_address, user_agent, created_at
		 FROM consent
		 WHERE user_id = ? AND document_type = ?
		 ORDER BY accepted_at DESC LIMIT 1`,
		userID, docType,
	).Scan(&c.ID, &c.UserID, &c.DocumentType, &c.DocumentVersion, &docURL, &acceptedAt, &ipAddress, &userAgent, &createdAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query latest consent: %w", err)
	}

	c.AcceptedAt = ParseDBTime(acceptedAt)
	c.CreatedAt = ParseDBTime(createdAt)
	if docURL.Valid {
		c.DocumentURL = docURL.String
	}
	if ipAddress.Valid {
		c.IPAddress = ipAddress.String
	}
	if userAgent.Valid {
		c.UserAgent = userAgent.String
	}

	return &c, nil
}

// ListConsents returns all consent records for a user, ordered by acceptance time descending.
func (db *DB) ListConsents(userID string) ([]*Consent, error) {
	rows, err := db.Query(
		`SELECT id, user_id, document_type, document_version, document_url, accepted_at, ip_address, user_agent, created_at
		 FROM consent
		 WHERE user_id = ?
		 ORDER BY accepted_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list consents: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var consents []*Consent
	for rows.Next() {
		var c Consent
		var docURL, ipAddress, userAgent sql.NullString
		var acceptedAt, createdAt string

		if err := rows.Scan(&c.ID, &c.UserID, &c.DocumentType, &c.DocumentVersion, &docURL, &acceptedAt, &ipAddress, &userAgent, &createdAt); err != nil {
			continue
		}

		c.AcceptedAt = ParseDBTime(acceptedAt)
		c.CreatedAt = ParseDBTime(createdAt)
		if docURL.Valid {
			c.DocumentURL = docURL.String
		}
		if ipAddress.Valid {
			c.IPAddress = ipAddress.String
		}
		if userAgent.Valid {
			c.UserAgent = userAgent.String
		}

		consents = append(consents, &c)
	}

	return consents, nil
}

// HasAcceptedVersion checks if a user has accepted a specific version of a document.
func (db *DB) HasAcceptedVersion(userID, docType, requiredVersion string) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM consent
		 WHERE user_id = ? AND document_type = ? AND document_version = ?`,
		userID, docType, requiredVersion,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check accepted version: %w", err)
	}

	return count > 0, nil
}

// GetPendingConsents compares a user's latest accepted versions against the policy table
// and returns document types that need re-acceptance. It checks the policy keys
// legal.terms_version and legal.privacy_version.
func (db *DB) GetPendingConsents(userID string) ([]string, error) {
	var pending []string

	for docType, policyKey := range consentPolicyKeys {
		var requiredVersion string
		err := db.QueryRow(
			`SELECT value FROM policy WHERE key = ?`,
			policyKey,
		).Scan(&requiredVersion)
		if err == sql.ErrNoRows {
			// No policy defined for this document type; skip.
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("query policy %s: %w", policyKey, err)
		}

		// DPA only applies to business accounts. Skip if the user has never
		// accepted any DPA version (private accounts never do).
		if docType == ConsentDPA {
			var anyDPA int
			_ = db.QueryRow(
				`SELECT COUNT(*) FROM consent WHERE user_id = ? AND document_type = ?`,
				userID, ConsentDPA,
			).Scan(&anyDPA)
			if anyDPA == 0 {
				continue
			}
		}

		accepted, err := db.HasAcceptedVersion(userID, docType, requiredVersion)
		if err != nil {
			return nil, err
		}
		if !accepted {
			pending = append(pending, docType)
		}
	}

	return pending, nil
}
