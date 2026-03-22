// Copyright 2026 Quest Financial Technologies S.a r.l.-S., Luxembourg
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

import "fmt"

// RegistrationIdentity holds the identity fields captured at registration time.
type RegistrationIdentity struct {
	AccountType         string // "private" or "business"
	FirstName           string
	LastName            string
	CompanyName         string // required for business accounts
	CompanyRegistration string // trade register number
	VATNumber           string
	Street              string
	Street2             string
	PostalCode          string
	City                string
	State               string
	Country             string
	AcceptDPA           bool // required for business accounts
}

// RecordRegistrationIdentity creates person, address, and consent records for a newly created user.
// It looks up the current policy versions for terms, privacy, and DPA.
func (db *DB) RecordRegistrationIdentity(userID string, identity *RegistrationIdentity, ipAddress, userAgent string) (*Person, error) {
	if identity == nil {
		return nil, nil
	}

	// Build company registration field: combine registration number and VAT
	companyReg := identity.CompanyRegistration
	if identity.VATNumber != "" {
		if companyReg != "" {
			companyReg += " / VAT: " + identity.VATNumber
		} else {
			companyReg = "VAT: " + identity.VATNumber
		}
	}

	// Create person record
	person, err := db.CreatePerson(
		userID, identity.FirstName, identity.LastName,
		identity.AccountType, identity.CompanyName, identity.Country,
	)
	if err != nil {
		return nil, fmt.Errorf("create person: %w", err)
	}

	// Store company registration if provided
	if companyReg != "" {
		if err := db.UpdatePerson(person.ID, UpdatePersonFields{CompanyRegistration: &companyReg}); err != nil {
			return nil, fmt.Errorf("update company registration: %w", err)
		}
	}

	// Create address if any address fields were provided (business accounts)
	if identity.Street != "" || identity.City != "" || identity.PostalCode != "" {
		addr, err := db.CreateAddress(
			identity.Country, "Legal address",
			identity.Street, identity.Street2,
			identity.City, identity.State, identity.PostalCode,
		)
		if err != nil {
			return nil, fmt.Errorf("create address: %w", err)
		}
		// Link address to person
		if _, err := db.CreateRelation(
			RelHasAddress, EntityPerson, person.ID, EntityAddress, addr.ID, nil, userID,
		); err != nil {
			return nil, fmt.Errorf("link address to person: %w", err)
		}
	}

	// Record consent: terms
	termsVersion, _ := db.GetPolicy("legal.terms_version")
	if termsVersion == "" {
		termsVersion = "1.0.0"
	}
	if _, err := db.CreateConsent(userID, ConsentTerms, termsVersion, "", ipAddress, userAgent); err != nil {
		return nil, fmt.Errorf("record terms consent: %w", err)
	}

	// Record consent: privacy
	privacyVersion, _ := db.GetPolicy("legal.privacy_version")
	if privacyVersion == "" {
		privacyVersion = "1.0.0"
	}
	if _, err := db.CreateConsent(userID, ConsentPrivacy, privacyVersion, "", ipAddress, userAgent); err != nil {
		return nil, fmt.Errorf("record privacy consent: %w", err)
	}

	// Record consent: DPA (business accounts)
	if identity.AcceptDPA {
		dpaVersion, _ := db.GetPolicy("legal.dpa_version")
		if dpaVersion == "" {
			dpaVersion = "1.0.0"
		}
		if _, err := db.CreateConsent(userID, ConsentDPA, dpaVersion, "", ipAddress, userAgent); err != nil {
			return nil, fmt.Errorf("record dpa consent: %w", err)
		}
	}

	// Record age declaration
	if _, err := db.CreateConsent(userID, ConsentAgeDeclaration, "1.0.0", "", ipAddress, userAgent); err != nil {
		return nil, fmt.Errorf("record age consent: %w", err)
	}

	return person, nil
}
