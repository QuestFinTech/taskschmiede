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

// Person represents a person profile linked to a user account.
type Person struct {
	ID                  string
	UserID              string
	FirstName           string
	MiddleNames         string
	LastName            string
	Phone               string
	Country             string
	Language            string
	AccountType         string // "private" or "business"
	CompanyName         string
	CompanyRegistration string
	Metadata            map[string]interface{}
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// UpdatePersonFields holds the fields to update on a person.
// Only non-nil pointer fields are applied. Metadata is applied when non-nil.
type UpdatePersonFields struct {
	FirstName           *string
	MiddleNames         *string
	LastName            *string
	Phone               *string
	Country             *string
	Language            *string
	AccountType         *string
	CompanyName         *string
	CompanyRegistration *string
	Metadata            map[string]interface{}
}

// ErrPersonNotFound is returned when a person record cannot be found.
var ErrPersonNotFound = errors.New("person not found")

// FullName returns the person's full name as "FirstName [MiddleNames] LastName".
func (p *Person) FullName() string {
	if p.MiddleNames != "" {
		return p.FirstName + " " + p.MiddleNames + " " + p.LastName
	}
	return p.FirstName + " " + p.LastName
}

// CreatePerson inserts a new person profile linked to a user.
func (db *DB) CreatePerson(userID, firstName, lastName, accountType, companyName, country string) (*Person, error) {
	if accountType == "" {
		accountType = "private"
	}

	id := generateID("per")
	now := UTCNow()
	nowStr := now.Format(time.RFC3339)

	var companyNameVal, countryVal *string
	if companyName != "" {
		companyNameVal = &companyName
	}
	if country != "" {
		countryVal = &country
	}

	_, err := db.Exec(
		`INSERT INTO person (id, user_id, first_name, last_name, account_type, company_name, country, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, '{}', ?, ?)`,
		id, userID, firstName, lastName, accountType, companyNameVal, countryVal, nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert person: %w", err)
	}

	return &Person{
		ID:          id,
		UserID:      userID,
		FirstName:   firstName,
		LastName:    lastName,
		AccountType: accountType,
		CompanyName: companyName,
		Country:     country,
		Metadata:    map[string]interface{}{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// GetPerson retrieves a person by ID.
func (db *DB) GetPerson(id string) (*Person, error) {
	row := db.QueryRow(
		`SELECT id, user_id, first_name, middle_names, last_name, phone, country, language, account_type, company_name, company_registration, metadata, created_at, updated_at
		 FROM person WHERE id = ?`,
		id,
	)
	p, err := scanPersonRow(row)
	if err == sql.ErrNoRows {
		return nil, ErrPersonNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get person: %w", err)
	}
	return p, nil
}

// GetPersonByUserID retrieves a person by their linked user ID.
func (db *DB) GetPersonByUserID(userID string) (*Person, error) {
	row := db.QueryRow(
		`SELECT id, user_id, first_name, middle_names, last_name, phone, country, language, account_type, company_name, company_registration, metadata, created_at, updated_at
		 FROM person WHERE user_id = ?`,
		userID,
	)
	p, err := scanPersonRow(row)
	if err == sql.ErrNoRows {
		return nil, ErrPersonNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get person by user_id: %w", err)
	}
	return p, nil
}

// UpdatePerson applies partial updates to a person.
func (db *DB) UpdatePerson(id string, fields UpdatePersonFields) error {
	var setClauses []string
	var params []interface{}

	if fields.FirstName != nil {
		setClauses = append(setClauses, "first_name = ?")
		params = append(params, *fields.FirstName)
	}
	if fields.MiddleNames != nil {
		setClauses = append(setClauses, "middle_names = ?")
		params = append(params, *fields.MiddleNames)
	}
	if fields.LastName != nil {
		setClauses = append(setClauses, "last_name = ?")
		params = append(params, *fields.LastName)
	}
	if fields.Phone != nil {
		setClauses = append(setClauses, "phone = ?")
		params = append(params, *fields.Phone)
	}
	if fields.Country != nil {
		setClauses = append(setClauses, "country = ?")
		params = append(params, *fields.Country)
	}
	if fields.Language != nil {
		setClauses = append(setClauses, "language = ?")
		params = append(params, *fields.Language)
	}
	if fields.AccountType != nil {
		setClauses = append(setClauses, "account_type = ?")
		params = append(params, *fields.AccountType)
	}
	if fields.CompanyName != nil {
		setClauses = append(setClauses, "company_name = ?")
		params = append(params, *fields.CompanyName)
	}
	if fields.CompanyRegistration != nil {
		setClauses = append(setClauses, "company_registration = ?")
		params = append(params, *fields.CompanyRegistration)
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

	setClauses = append(setClauses, "updated_at = ?")
	params = append(params, UTCNow().Format(time.RFC3339))

	query := fmt.Sprintf("UPDATE person SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	params = append(params, id)

	result, err := db.Exec(query, params...)
	if err != nil {
		return fmt.Errorf("update person: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrPersonNotFound
	}

	return nil
}

// DeletePerson physically deletes a person record. This is intended for
// GDPR erasure requests where soft-delete is insufficient.
func (db *DB) DeletePerson(id string) error {
	result, err := db.Exec(`DELETE FROM person WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete person: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrPersonNotFound
	}

	return nil
}

// scanPersonRow scans a single person row from a *sql.Row.
func scanPersonRow(row *sql.Row) (*Person, error) {
	var p Person
	var middleNames, phone, country, language, companyName, companyRegistration sql.NullString
	var metadataJSON string
	var createdAt, updatedAt string

	err := row.Scan(
		&p.ID, &p.UserID, &p.FirstName, &middleNames, &p.LastName,
		&phone, &country, &language, &p.AccountType,
		&companyName, &companyRegistration, &metadataJSON,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if middleNames.Valid {
		p.MiddleNames = middleNames.String
	}
	if phone.Valid {
		p.Phone = phone.String
	}
	if country.Valid {
		p.Country = country.String
	}
	if language.Valid {
		p.Language = language.String
	}
	if companyName.Valid {
		p.CompanyName = companyName.String
	}
	if companyRegistration.Valid {
		p.CompanyRegistration = companyRegistration.String
	}

	_ = json.Unmarshal([]byte(metadataJSON), &p.Metadata)
	p.CreatedAt = ParseDBTime(createdAt)
	p.UpdatedAt = ParseDBTime(updatedAt)

	return &p, nil
}

