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

package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/email"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// handleGetMyPerson handles GET /api/v1/auth/person.
func (a *API) handleGetMyPerson(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	person, err := a.db.GetPersonByUserID(authUser.UserID)
	if err != nil {
		if err == storage.ErrPersonNotFound {
			writeError(w, http.StatusNotFound, "not_found", "No person record found")
			return
		}
		a.logger.Error("Failed to get person", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get person")
		return
	}

	result := map[string]interface{}{
		"id":                   person.ID,
		"first_name":           person.FirstName,
		"middle_names":         person.MiddleNames,
		"last_name":            person.LastName,
		"phone":                person.Phone,
		"country":              person.Country,
		"language":             person.Language,
		"account_type":         person.AccountType,
		"company_name":         person.CompanyName,
		"company_registration": person.CompanyRegistration,
		"created_at":           person.CreatedAt,
		"updated_at":           person.UpdatedAt,
	}

	// Include primary address if linked.
	addresses, _ := a.db.ListAddressesByEntity(storage.EntityPerson, person.ID)
	if len(addresses) > 0 {
		addr := addresses[0]
		result["address"] = map[string]interface{}{
			"id":          addr.ID,
			"label":       addr.Label,
			"street":      addr.Street,
			"street2":     addr.Street2,
			"city":        addr.City,
			"state":       addr.State,
			"postal_code": addr.PostalCode,
			"country":     addr.Country,
		}
	}

	writeData(w, http.StatusOK, result)
}

// handleUpdateMyPerson handles PATCH /api/v1/auth/person.
func (a *API) handleUpdateMyPerson(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var body struct {
		FirstName           *string `json:"first_name"`
		MiddleNames         *string `json:"middle_names"`
		LastName            *string `json:"last_name"`
		Phone               *string `json:"phone"`
		Country             *string `json:"country"`
		AccountType         *string `json:"account_type"`
		CompanyName         *string `json:"company_name"`
		CompanyRegistration *string `json:"company_registration"`
		// Address fields (optional, creates or updates the person's primary address).
		Street     *string `json:"street"`
		Street2    *string `json:"street2"`
		City       *string `json:"city"`
		State      *string `json:"state"`
		PostalCode *string `json:"postal_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	// Validate account_type if provided.
	if body.AccountType != nil {
		at := *body.AccountType
		if at != "private" && at != "business" {
			writeError(w, http.StatusBadRequest, "invalid_input", "account_type must be 'private' or 'business'")
			return
		}
	}

	person, err := a.db.GetPersonByUserID(authUser.UserID)
	if err == storage.ErrPersonNotFound {
		// Auto-create Person from the provided fields.
		firstName := ""
		lastName := ""
		accountType := "private"
		if body.FirstName != nil {
			firstName = *body.FirstName
		}
		if body.LastName != nil {
			lastName = *body.LastName
		}
		if body.AccountType != nil {
			accountType = *body.AccountType
		}
		person, err = a.db.CreatePerson(authUser.UserID, firstName, lastName, accountType, "", "")
		if err != nil {
			a.logger.Error("Failed to create person", "user_id", authUser.UserID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create person")
			return
		}
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get person")
		return
	}

	fields := storage.UpdatePersonFields{
		FirstName:           body.FirstName,
		MiddleNames:         body.MiddleNames,
		LastName:            body.LastName,
		Phone:               body.Phone,
		Country:             body.Country,
		AccountType:         body.AccountType,
		CompanyName:         body.CompanyName,
		CompanyRegistration: body.CompanyRegistration,
	}

	if err := a.db.UpdatePerson(person.ID, fields); err != nil {
		a.logger.Error("Failed to update person", "person_id", person.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update person")
		return
	}

	// Handle address create-or-update if any address field is provided.
	hasAddr := body.Street != nil || body.Street2 != nil || body.City != nil || body.State != nil || body.PostalCode != nil
	if hasAddr {
		// Determine country for address: use person's country.
		addrCountry := ""
		if body.Country != nil {
			addrCountry = *body.Country
		} else {
			addrCountry = person.Country
		}

		str := func(p *string) string {
			if p != nil {
				return *p
			}
			return ""
		}

		addresses, _ := a.db.ListAddressesByEntity(storage.EntityPerson, person.ID)
		if len(addresses) > 0 {
			// Update existing address.
			addrFields := storage.UpdateAddressFields{
				Street:     body.Street,
				Street2:    body.Street2,
				City:       body.City,
				State:      body.State,
				PostalCode: body.PostalCode,
				Country:    &addrCountry,
			}
			if err := a.db.UpdateAddress(addresses[0].ID, addrFields); err != nil {
				a.logger.Error("Failed to update address", "address_id", addresses[0].ID, "error", err)
			}
		} else {
			// Create new address and link to person.
			addr, err := a.db.CreateAddress(addrCountry, "Personal", str(body.Street), str(body.Street2), str(body.City), str(body.State), str(body.PostalCode))
			if err != nil {
				a.logger.Error("Failed to create address", "person_id", person.ID, "error", err)
			} else {
				_, err = a.db.CreateRelation(storage.RelHasAddress, storage.EntityPerson, person.ID, storage.EntityAddress, addr.ID, nil, authUser.UserID)
				if err != nil {
					a.logger.Error("Failed to link address to person", "person_id", person.ID, "error", err)
				}
			}
		}
	}

	if a.auditSvc != nil {
		a.auditSvc.Log(&security.AuditEntry{
			Action:    "person_updated",
			ActorID:   authUser.UserID,
			ActorType: "user",
			Resource:  "person:" + person.ID,
			IP:        security.ExtractIP(r),
			Source:    auditSource(r),
		})
	}

	writeData(w, http.StatusOK, map[string]interface{}{"status": "updated"})
}

// handleRequestEmailChange handles POST /api/v1/auth/email/change.
// Creates a pending email change and sends a verification code to the new address.
func (a *API) handleRequestEmailChange(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var body struct {
		NewEmail string `json:"new_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	body.NewEmail = strings.TrimSpace(body.NewEmail)
	if body.NewEmail == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "New email is required")
		return
	}

	// Get current user to check if same email.
	user, err := a.db.GetUser(authUser.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get user")
		return
	}
	if user.Email == body.NewEmail {
		writeError(w, http.StatusBadRequest, "invalid_input", "New email is the same as current")
		return
	}

	pending, err := a.db.CreatePendingEmailChange(authUser.UserID, body.NewEmail, 15*time.Minute)
	if err != nil {
		if strings.Contains(err.Error(), "already in use") {
			writeError(w, http.StatusConflict, "email_taken", "This email address is already in use")
			return
		}
		a.logger.Error("Failed to create pending email change", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to initiate email change")
		return
	}

	// Send verification email to the NEW address with a profile-specific link.
	emailSent := a.sendEmailChangeEmail(body.NewEmail, user.Name, pending.VerificationCode)

	if a.auditSvc != nil {
		a.auditSvc.Log(&security.AuditEntry{
			Action:    "email_change_requested",
			ActorID:   authUser.UserID,
			ActorType: "user",
			Resource:  "user:" + authUser.UserID,
			IP:        security.ExtractIP(r),
			Source:    auditSource(r),
		})
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"status":     "verification_sent",
		"new_email":  body.NewEmail,
		"email_sent": emailSent,
	})
}

// handleVerifyEmailChange handles POST /api/v1/auth/email/verify.
// Verifies the code and applies the email change.
func (a *API) handleVerifyEmailChange(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	newEmail, err := a.db.VerifyAndApplyEmailChange(authUser.UserID, body.Code)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_code", err.Error())
		return
	}

	if a.auditSvc != nil {
		a.auditSvc.Log(&security.AuditEntry{
			Action:    "email_changed",
			ActorID:   authUser.UserID,
			ActorType: "user",
			Resource:  "user:" + authUser.UserID,
			IP:        security.ExtractIP(r),
			Source:    auditSource(r),
		})
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"status":    "email_updated",
		"new_email": newEmail,
	})
}

// sendEmailChangeEmail sends a verification email for an email change
// with a link pointing to /profile?email_code=CODE instead of /verify.
func (a *API) sendEmailChangeEmail(toEmail, name, code string) bool {
	if a.emailSender == nil {
		return false
	}
	ts, ok := a.emailSender.(TemplatedEmailSender)
	if !ok {
		return false
	}

	t := func(key string, args ...interface{}) string {
		if a.i18n != nil {
			return a.i18n.T("en", key, args...)
		}
		return key
	}

	actionURL := ""
	if a.portalURL != "" {
		actionURL = a.portalURL + "/profile?email_code=" + code
	}

	data := &email.VerificationCodeData{
		Greeting:   t("email.greeting", name),
		Intro:      t("email.verification.intro"),
		Code:       code,
		ExpiryNote: t("email.verification.expiry", "15 minutes"),
		ActionURL:  actionURL,
		ButtonText: t("email.verification.button"),
		ManualNote: t("email.verification.manual_note"),
		Closing:    t("email.closing"),
		TeamName:   t("email.team_name"),
	}
	if err := ts.SendVerificationCode(toEmail, t("email.verification.subject"), data); err != nil {
		a.logger.Error("Failed to send email change verification", "to", toEmail, "error", err)
		return false
	}
	return true
}

// handleGetPendingEmailChange handles GET /api/v1/auth/email/pending.
func (a *API) handleGetPendingEmailChange(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	pending, err := a.db.GetPendingEmailChange(authUser.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check pending email change")
		return
	}
	if pending == nil {
		writeData(w, http.StatusOK, map[string]interface{}{"pending": false})
		return
	}
	writeData(w, http.StatusOK, map[string]interface{}{
		"pending":   true,
		"new_email": pending.NewEmail,
	})
}

// handleCancelEmailChange handles POST /api/v1/auth/email/cancel.
func (a *API) handleCancelEmailChange(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	_, _ = a.db.Exec("DELETE FROM pending_email_change WHERE user_id = ?", authUser.UserID)
	writeData(w, http.StatusOK, map[string]interface{}{"status": "cancelled"})
}

// handleListMyConsents handles GET /api/v1/auth/consents.
func (a *API) handleListMyConsents(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	consents, err := a.db.ListConsents(authUser.UserID)
	if err != nil {
		a.logger.Error("Failed to list consents", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list consents")
		return
	}

	items := make([]map[string]interface{}, 0, len(consents))
	for _, c := range consents {
		items = append(items, map[string]interface{}{
			"id":               c.ID,
			"document_type":    c.DocumentType,
			"document_version": c.DocumentVersion,
			"accepted_at":      c.AcceptedAt,
		})
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"items": items,
		"total": len(items),
	})
}
