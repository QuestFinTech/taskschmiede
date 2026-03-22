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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Address represents a physical address.
type Address struct {
	ID         string
	Label      string
	Street     string
	Street2    string
	City       string
	State      string
	PostalCode string
	Country    string
	Metadata   map[string]interface{}
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UpdateAddressFields holds optional fields for partial address updates.
type UpdateAddressFields struct {
	Label      *string
	Street     *string
	Street2    *string
	City       *string
	State      *string
	PostalCode *string
	Country    *string
	Metadata   map[string]interface{} // nil = no change; replaces existing
}

// ErrAddressNotFound is returned when an address cannot be found by its ID.
var ErrAddressNotFound = errors.New("address not found")

// CreateAddress creates a new address record.
func (db *DB) CreateAddress(country string, label, street, street2, city, state, postalCode string) (*Address, error) {
	id := generateID("adr")

	metadataJSON := "{}"

	var labelVal, streetVal, street2Val, cityVal, stateVal, postalCodeVal interface{}
	if label != "" {
		labelVal = label
	}
	if street != "" {
		streetVal = street
	}
	if street2 != "" {
		street2Val = street2
	}
	if city != "" {
		cityVal = city
	}
	if state != "" {
		stateVal = state
	}
	if postalCode != "" {
		postalCodeVal = postalCode
	}

	now := UTCNow()
	_, err := db.Exec(
		`INSERT INTO address (id, label, street, street2, city, state, postal_code, country, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, labelVal, streetVal, street2Val, cityVal, stateVal, postalCodeVal, country, metadataJSON,
		now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert address: %w", err)
	}

	return &Address{
		ID:         id,
		Label:      label,
		Street:     street,
		Street2:    street2,
		City:       city,
		State:      state,
		PostalCode: postalCode,
		Country:    country,
		Metadata:   map[string]interface{}{},
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// GetAddress retrieves an address by ID.
func (db *DB) GetAddress(id string) (*Address, error) {
	var addr Address
	var label, street, street2, city, state, postalCode sql.NullString
	var metadataJSON string
	var createdAt, updatedAt string

	err := db.QueryRow(
		`SELECT id, label, street, street2, city, state, postal_code, country, metadata, created_at, updated_at
		 FROM address WHERE id = ?`,
		id,
	).Scan(&addr.ID, &label, &street, &street2, &city, &state, &postalCode,
		&addr.Country, &metadataJSON, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrAddressNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query address: %w", err)
	}

	if label.Valid {
		addr.Label = label.String
	}
	if street.Valid {
		addr.Street = street.String
	}
	if street2.Valid {
		addr.Street2 = street2.String
	}
	if city.Valid {
		addr.City = city.String
	}
	if state.Valid {
		addr.State = state.String
	}
	if postalCode.Valid {
		addr.PostalCode = postalCode.String
	}
	_ = json.Unmarshal([]byte(metadataJSON), &addr.Metadata)
	addr.CreatedAt = ParseDBTime(createdAt)
	addr.UpdatedAt = ParseDBTime(updatedAt)

	return &addr, nil
}

// UpdateAddress applies partial updates to an address.
func (db *DB) UpdateAddress(id string, fields UpdateAddressFields) error {
	var setClauses []string
	var params []interface{}

	if fields.Label != nil {
		setClauses = append(setClauses, "label = ?")
		params = append(params, *fields.Label)
	}
	if fields.Street != nil {
		setClauses = append(setClauses, "street = ?")
		params = append(params, *fields.Street)
	}
	if fields.Street2 != nil {
		setClauses = append(setClauses, "street2 = ?")
		params = append(params, *fields.Street2)
	}
	if fields.City != nil {
		setClauses = append(setClauses, "city = ?")
		params = append(params, *fields.City)
	}
	if fields.State != nil {
		setClauses = append(setClauses, "state = ?")
		params = append(params, *fields.State)
	}
	if fields.PostalCode != nil {
		setClauses = append(setClauses, "postal_code = ?")
		params = append(params, *fields.PostalCode)
	}
	if fields.Country != nil {
		setClauses = append(setClauses, "country = ?")
		params = append(params, *fields.Country)
	}
	if fields.Metadata != nil {
		b, err := json.Marshal(fields.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		setClauses = append(setClauses, "metadata = ?")
		params = append(params, string(b))
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no fields to update")
	}

	setClauses = append(setClauses, "updated_at = datetime('now')")
	query := fmt.Sprintf("UPDATE address SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	params = append(params, id)

	result, err := db.Exec(query, params...)
	if err != nil {
		return fmt.Errorf("update address: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrAddressNotFound
	}

	return nil
}

// DeleteAddress hard-deletes an address and all its relations (for GDPR compliance).
func (db *DB) DeleteAddress(id string) error {
	// Delete all relations involving this address.
	_, _ = db.Exec(`DELETE FROM entity_relation
		WHERE (source_entity_type = 'address' AND source_entity_id = ?)
		   OR (target_entity_type = 'address' AND target_entity_id = ?)`, id, id)

	result, err := db.Exec(`DELETE FROM address WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete address: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrAddressNotFound
	}
	return nil
}

// ListAddressesByEntity returns all addresses linked to the given entity via
// has_address relations in the entity_relation table.
func (db *DB) ListAddressesByEntity(entityType, entityID string) ([]*Address, error) {
	rows, err := db.Query(
		`SELECT a.id, a.label, a.street, a.street2, a.city, a.state, a.postal_code, a.country, a.metadata, a.created_at, a.updated_at
		 FROM address a
		 JOIN entity_relation er ON a.id = er.target_entity_id
		   AND er.target_entity_type = 'address'
		   AND er.source_entity_type = ?
		   AND er.source_entity_id = ?
		   AND er.relationship_type = 'has_address'
		 ORDER BY a.created_at DESC`,
		entityType, entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("query addresses by entity: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var addresses []*Address
	for rows.Next() {
		var addr Address
		var label, street, street2, city, state, postalCode sql.NullString
		var metadataJSON string
		var createdAt, updatedAt string

		if err := rows.Scan(&addr.ID, &label, &street, &street2, &city, &state, &postalCode,
			&addr.Country, &metadataJSON, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan address: %w", err)
		}

		if label.Valid {
			addr.Label = label.String
		}
		if street.Valid {
			addr.Street = street.String
		}
		if street2.Valid {
			addr.Street2 = street2.String
		}
		if city.Valid {
			addr.City = city.String
		}
		if state.Valid {
			addr.State = state.String
		}
		if postalCode.Valid {
			addr.PostalCode = postalCode.String
		}
		_ = json.Unmarshal([]byte(metadataJSON), &addr.Metadata)
		addr.CreatedAt = ParseDBTime(createdAt)
		addr.UpdatedAt = ParseDBTime(updatedAt)

		addresses = append(addresses, &addr)
	}

	return addresses, nil
}
