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
	"fmt"
	"time"
)

// EndeavourExport is the complete export format for an endeavour and all its contents.
type EndeavourExport struct {
	Version     int    `json:"version"`
	ExportedAt  string `json:"exported_at"`
	EndeavourID string `json:"endeavour_id"`

	Endeavour    *Endeavour           `json:"endeavour"`
	Tasks        []Task               `json:"tasks"`
	Demands      []Demand             `json:"demands"`
	Artifacts    []Artifact           `json:"artifacts"`
	Rituals      []Ritual             `json:"rituals"`
	RitualRuns   []RitualRun          `json:"ritual_runs"`
	DoDPolicies  []backupDoDPolicy    `json:"dod_policies"`
	Endorsements []backupEndorsement  `json:"endorsements"`
	Comments     []Comment            `json:"comments"`
	Approvals    []Approval           `json:"approvals"`
	Relations    []EntityRelation     `json:"relations"`
	Messages     []Message            `json:"messages"`
	Deliveries   []MessageDelivery    `json:"deliveries"`
}

// OrgExport is the complete export format for an organization and all its contents.
type OrgExport struct {
	Version        int    `json:"version"`
	ExportedAt     string `json:"exported_at"`
	OrganizationID string `json:"organization_id"`

	Organization *Organization      `json:"organization"`
	Members      []backupOrgMember  `json:"members"`
	Endeavours   []EndeavourExport  `json:"endeavours"`
	Relations    []EntityRelation   `json:"relations"`
}

// backupOrgMember holds a resource and its role within the organization.
type backupOrgMember struct {
	Resource *Resource `json:"resource"`
	Role     string    `json:"role"`
}

// backupEndorsement holds a DoD endorsement for backup.
type backupEndorsement struct {
	ID            string  `json:"id"`
	PolicyID      string  `json:"policy_id"`
	PolicyVersion int     `json:"policy_version"`
	ResourceID    string  `json:"resource_id"`
	EndeavourID   string  `json:"endeavour_id"`
	Status        string  `json:"status"`
	EndorsedAt    string  `json:"endorsed_at"`
	SupersededAt  string  `json:"superseded_at,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

// UserBackup is the complete export format for a user's data.
type UserBackup struct {
	Version   int       `json:"version"`
	ExportedAt string   `json:"exported_at"`
	UserID    string    `json:"user_id"`

	User      *User              `json:"user"`
	Resource  *Resource          `json:"resource,omitempty"`
	Person    *Person            `json:"person,omitempty"`
	Consents  []*Consent         `json:"consents,omitempty"`
	Tokens    []backupToken      `json:"tokens"`

	Organizations []Organization    `json:"organizations"`
	Endeavours    []Endeavour       `json:"endeavours"`
	Tasks         []Task            `json:"tasks"`
	Demands       []Demand          `json:"demands"`
	Artifacts     []Artifact        `json:"artifacts"`
	Rituals       []Ritual          `json:"rituals"`
	RitualRuns    []RitualRun       `json:"ritual_runs"`
	DoDPolicies   []backupDoDPolicy `json:"dod_policies"`
	Comments      []Comment         `json:"comments"`
	Approvals     []Approval        `json:"approvals"`
	Relations     []EntityRelation  `json:"relations"`

	// Messages from the separate MessageDB.
	Messages   []Message          `json:"messages"`
	Deliveries []MessageDelivery  `json:"deliveries"`
}

// backupToken stores token data for backup (hash only, not the raw token).
type backupToken struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	TokenHash string `json:"token_hash"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// backupDoDPolicy holds DoD policy data for backup.
type backupDoDPolicy struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Scope         string                 `json:"scope"`
	Strictness    string                 `json:"strictness"`
	Quorum        int                    `json:"quorum"`
	Conditions    json.RawMessage        `json:"conditions"`
	Origin        string                 `json:"origin"`
	PredecessorID string                 `json:"predecessor_id,omitempty"`
	CreatedBy     string                 `json:"created_by"`
	Status        string                 `json:"status"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
}

// ensureSlices replaces nil slices with empty slices so JSON marshalling
// produces [] instead of null. Consumers should not need to null-check arrays.
func (e *EndeavourExport) ensureSlices() {
	if e.Tasks == nil { e.Tasks = []Task{} }
	if e.Demands == nil { e.Demands = []Demand{} }
	if e.Artifacts == nil { e.Artifacts = []Artifact{} }
	if e.Rituals == nil { e.Rituals = []Ritual{} }
	if e.RitualRuns == nil { e.RitualRuns = []RitualRun{} }
	if e.DoDPolicies == nil { e.DoDPolicies = []backupDoDPolicy{} }
	if e.Endorsements == nil { e.Endorsements = []backupEndorsement{} }
	if e.Comments == nil { e.Comments = []Comment{} }
	if e.Approvals == nil { e.Approvals = []Approval{} }
	if e.Relations == nil { e.Relations = []EntityRelation{} }
	if e.Messages == nil { e.Messages = []Message{} }
	if e.Deliveries == nil { e.Deliveries = []MessageDelivery{} }
}

func (e *OrgExport) ensureSlices() {
	if e.Members == nil { e.Members = []backupOrgMember{} }
	if e.Endeavours == nil { e.Endeavours = []EndeavourExport{} }
	if e.Relations == nil { e.Relations = []EntityRelation{} }
}

func (b *UserBackup) ensureSlices() {
	if b.Tokens == nil { b.Tokens = []backupToken{} }
	if b.Organizations == nil { b.Organizations = []Organization{} }
	if b.Endeavours == nil { b.Endeavours = []Endeavour{} }
	if b.Tasks == nil { b.Tasks = []Task{} }
	if b.Demands == nil { b.Demands = []Demand{} }
	if b.Artifacts == nil { b.Artifacts = []Artifact{} }
	if b.Rituals == nil { b.Rituals = []Ritual{} }
	if b.RitualRuns == nil { b.RitualRuns = []RitualRun{} }
	if b.DoDPolicies == nil { b.DoDPolicies = []backupDoDPolicy{} }
	if b.Comments == nil { b.Comments = []Comment{} }
	if b.Approvals == nil { b.Approvals = []Approval{} }
	if b.Relations == nil { b.Relations = []EntityRelation{} }
	if b.Messages == nil { b.Messages = []Message{} }
	if b.Deliveries == nil { b.Deliveries = []MessageDelivery{} }
}

// ExportUserData creates a complete backup of a user's owned data.
// It collects: user record, resource, tokens, owned organizations,
// owned endeavours, and all entities within those endeavours.
func (db *DB) ExportUserData(userID string) (*UserBackup, error) {
	user, err := db.GetUser(userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	backup := &UserBackup{
		Version:    1,
		ExportedAt: UTCNow().Format(time.RFC3339),
		UserID:     userID,
		User:       user,
	}

	resourceID := ""
	if user.ResourceID != nil && *user.ResourceID != "" {
		resourceID = *user.ResourceID
		res, err := db.GetResource(resourceID)
		if err == nil {
			backup.Resource = res
		}
	}

	// Tokens
	backup.Tokens = db.exportTokens(userID)

	// Find owned organizations (user's resource has role "owner" in has_member).
	var ownedOrgIDs []string
	if resourceID != "" {
		ownedOrgIDs = db.findOwnedOrgs(resourceID)
		for _, orgID := range ownedOrgIDs {
			org, err := db.GetOrganization(orgID)
			if err == nil {
				backup.Organizations = append(backup.Organizations, *org)
			}
		}
	}

	// Find owned endeavours (user has role "owner" in member_of).
	ownedEndeavourIDs := db.findOwnedEndeavours(userID)
	for _, edvID := range ownedEndeavourIDs {
		edv, err := db.GetEndeavour(edvID)
		if err == nil {
			backup.Endeavours = append(backup.Endeavours, *edv)
		}
	}

	// Collect all entity IDs in scope for relation export.
	entityIDs := map[string]bool{userID: true}
	if resourceID != "" {
		entityIDs[resourceID] = true
	}
	for _, orgID := range ownedOrgIDs {
		entityIDs[orgID] = true
	}

	// Export entities within owned endeavours.
	for _, edvID := range ownedEndeavourIDs {
		entityIDs[edvID] = true

		tasks := db.exportEndeavourTasks(edvID)
		backup.Tasks = append(backup.Tasks, tasks...)
		for _, t := range tasks {
			entityIDs[t.ID] = true
		}

		demands := db.exportEndeavourDemands(edvID)
		backup.Demands = append(backup.Demands, demands...)
		for _, d := range demands {
			entityIDs[d.ID] = true
		}

		artifacts := db.exportEndeavourArtifacts(edvID)
		backup.Artifacts = append(backup.Artifacts, artifacts...)
		for _, a := range artifacts {
			entityIDs[a.ID] = true
		}

		rituals := db.exportEndeavourRituals(edvID)
		backup.Rituals = append(backup.Rituals, rituals...)
		for _, r := range rituals {
			entityIDs[r.ID] = true
		}

		runs := db.exportEndeavourRitualRuns(edvID)
		backup.RitualRuns = append(backup.RitualRuns, runs...)

		policies := db.exportEndeavourDoDPolicies(edvID)
		backup.DoDPolicies = append(backup.DoDPolicies, policies...)
		for _, p := range policies {
			entityIDs[p.ID] = true
		}
	}

	// Export comments authored by this user's resource.
	if resourceID != "" {
		backup.Comments = db.exportUserComments(resourceID)
		backup.Approvals = db.exportUserApprovals(resourceID)
	}

	// Export relations where any entity in scope is source or target.
	backup.Relations = db.exportRelationsForEntities(entityIDs)

	// Person record
	person, _ := db.GetPersonByUserID(userID)
	if person != nil {
		backup.Person = person
	}

	// Consent records
	consents, _ := db.ListConsents(userID)
	if len(consents) > 0 {
		backup.Consents = consents
	}

	backup.ensureSlices()
	return backup, nil
}

// ExportUserMessages exports messages from the MessageDB for backup.
// Call this separately since it uses a different database handle.
func (mdb *MessageDB) ExportUserMessages(resourceID string) ([]Message, []MessageDelivery) {
	var messages []Message
	var deliveries []MessageDelivery

	// Messages sent by the user.
	rows, err := mdb.db.Query(
		`SELECT id, sender_id, subject, content, intent,
		        COALESCE(reply_to_id, ''), COALESCE(entity_type, ''), COALESCE(entity_id, ''),
		        COALESCE(scope_type, ''), COALESCE(scope_id, ''),
		        COALESCE(metadata, '{}'), created_at
		 FROM message WHERE sender_id = ?
		 ORDER BY created_at ASC`, resourceID,
	)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var m Message
			var metaStr, createdStr string
			if err := rows.Scan(&m.ID, &m.SenderID, &m.Subject, &m.Content, &m.Intent,
				&m.ReplyToID, &m.EntityType, &m.EntityID, &m.ScopeType, &m.ScopeID,
				&metaStr, &createdStr); err != nil {
				continue
			}
			_ = json.Unmarshal([]byte(metaStr), &m.Metadata)
			m.CreatedAt = ParseDBTime(createdStr)
			messages = append(messages, m)
		}
	}

	// Deliveries where user is recipient.
	dRows, err := mdb.db.Query(
		`SELECT d.id, d.message_id, d.recipient_id, d.channel, d.status,
		        d.delivered_at, d.read_at, d.created_at
		 FROM message_delivery d
		 WHERE d.recipient_id = ?
		 ORDER BY d.created_at ASC`, resourceID,
	)
	if err == nil {
		defer func() { _ = dRows.Close() }()
		for dRows.Next() {
			var d MessageDelivery
			var deliveredAt, readAt, createdAt sql.NullString
			if err := dRows.Scan(&d.ID, &d.MessageID, &d.RecipientID, &d.Channel, &d.Status,
				&deliveredAt, &readAt, &createdAt); err != nil {
				continue
			}
			if deliveredAt.Valid {
				t := ParseDBTime(deliveredAt.String)
				d.DeliveredAt = &t
			}
			if readAt.Valid {
				t := ParseDBTime(readAt.String)
				d.ReadAt = &t
			}
			if createdAt.Valid {
				d.CreatedAt = ParseDBTime(createdAt.String)
			}
			deliveries = append(deliveries, d)
		}
	}

	return messages, deliveries
}

// RestoreUserData imports a user's data from a backup.
// It re-creates the user account and all owned entities.
// Returns the new user ID (same as backup if no conflicts).
func (db *DB) RestoreUserData(backup *UserBackup) error {
	if backup == nil || backup.User == nil {
		return fmt.Errorf("invalid backup: missing user data")
	}

	// Phase 1: Resource
	if backup.Resource != nil {
		db.restoreResource(backup.Resource)
	}

	// Phase 2: User
	if err := db.restoreUser(backup.User); err != nil {
		return fmt.Errorf("restore user: %w", err)
	}

	// Phase 3: Organizations
	for i := range backup.Organizations {
		db.restoreOrganization(&backup.Organizations[i])
	}

	// Phase 4: Endeavours
	for i := range backup.Endeavours {
		db.restoreEndeavour(&backup.Endeavours[i])
	}

	// Phase 5: Demands and Tasks
	for i := range backup.Demands {
		db.restoreDemand(&backup.Demands[i])
	}
	for i := range backup.Tasks {
		db.restoreTask(&backup.Tasks[i])
	}

	// Phase 6: Artifacts, Rituals, DoD Policies
	for i := range backup.Artifacts {
		db.restoreArtifact(&backup.Artifacts[i])
	}
	for i := range backup.Rituals {
		db.restoreRitual(&backup.Rituals[i])
	}
	for i := range backup.DoDPolicies {
		db.restoreDoDPolicy(&backup.DoDPolicies[i])
	}

	// Phase 7: Ritual Runs
	for i := range backup.RitualRuns {
		db.restoreRitualRun(&backup.RitualRuns[i])
	}

	// Phase 8: Relations
	for i := range backup.Relations {
		db.restoreRelation(&backup.Relations[i])
	}

	// Phase 9: Comments and Approvals
	for i := range backup.Comments {
		db.restoreComment(&backup.Comments[i])
	}
	for i := range backup.Approvals {
		db.restoreApproval(&backup.Approvals[i])
	}

	// Phase 10: Tokens
	for i := range backup.Tokens {
		db.restoreToken(&backup.Tokens[i])
	}

	// Reactivate user.
	_, _ = db.Exec(
		`UPDATE user SET status = 'active', updated_at = ? WHERE id = ?`,
		UTCNow().Format(time.RFC3339), backup.UserID,
	)

	return nil
}

// RestoreUserMessages imports messages from a backup into the MessageDB.
func (mdb *MessageDB) RestoreUserMessages(messages []Message, deliveries []MessageDelivery) {
	for _, m := range messages {
		metaBytes, _ := json.Marshal(m.Metadata)
		_, _ = mdb.db.Exec(
			`INSERT OR IGNORE INTO message (id, sender_id, subject, content, intent, reply_to_id, entity_type, entity_id, scope_type, scope_id, metadata, created_at)
			 VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?)`,
			m.ID, m.SenderID, m.Subject, m.Content, m.Intent,
			m.ReplyToID, m.EntityType, m.EntityID, m.ScopeType, m.ScopeID,
			string(metaBytes), m.CreatedAt.Format(time.RFC3339),
		)
	}

	for _, d := range deliveries {
		var deliveredAt, readAt interface{}
		if d.DeliveredAt != nil {
			deliveredAt = d.DeliveredAt.Format(time.RFC3339)
		}
		if d.ReadAt != nil {
			readAt = d.ReadAt.Format(time.RFC3339)
		}
		_, _ = mdb.db.Exec(
			`INSERT OR IGNORE INTO message_delivery (id, message_id, recipient_id, channel, status, delivered_at, read_at, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			d.ID, d.MessageID, d.RecipientID, d.Channel, d.Status,
			deliveredAt, readAt, d.CreatedAt.Format(time.RFC3339),
		)
	}
}

// ExportEndeavourData creates a complete export of an endeavour and all its contents.
func (db *DB) ExportEndeavourData(endeavourID string) (*EndeavourExport, error) {
	edv, err := db.GetEndeavour(endeavourID)
	if err != nil {
		return nil, fmt.Errorf("get endeavour: %w", err)
	}

	export := &EndeavourExport{
		Version:     1,
		ExportedAt:  UTCNow().Format(time.RFC3339),
		EndeavourID: endeavourID,
		Endeavour:   edv,
	}

	// Collect all entity IDs in scope for relation/comment/approval export.
	entityIDs := map[string]bool{endeavourID: true}

	tasks := db.exportEndeavourTasks(endeavourID)
	export.Tasks = tasks
	for _, t := range tasks {
		entityIDs[t.ID] = true
	}

	demands := db.exportEndeavourDemands(endeavourID)
	export.Demands = demands
	for _, d := range demands {
		entityIDs[d.ID] = true
	}

	artifacts := db.exportEndeavourArtifacts(endeavourID)
	export.Artifacts = artifacts
	for _, a := range artifacts {
		entityIDs[a.ID] = true
	}

	rituals := db.exportEndeavourRituals(endeavourID)
	export.Rituals = rituals
	for _, r := range rituals {
		entityIDs[r.ID] = true
	}

	export.RitualRuns = db.exportEndeavourRitualRuns(endeavourID)

	policies := db.exportEndeavourDoDPolicies(endeavourID)
	export.DoDPolicies = policies
	for _, p := range policies {
		entityIDs[p.ID] = true
	}

	export.Endorsements = db.exportEndeavourEndorsements(endeavourID)
	export.Comments = db.exportScopedComments(entityIDs)
	export.Approvals = db.exportScopedApprovals(entityIDs)
	export.Relations = db.exportRelationsForEntities(entityIDs)

	export.ensureSlices()
	return export, nil
}

// ExportEndeavourMessages exports messages scoped to an endeavour from the MessageDB.
func (mdb *MessageDB) ExportEndeavourMessages(endeavourID string) ([]Message, []MessageDelivery) {
	var messages []Message
	rows, err := mdb.db.Query(
		`SELECT id, sender_id, subject, content, intent,
		        COALESCE(reply_to_id, ''), COALESCE(entity_type, ''), COALESCE(entity_id, ''),
		        COALESCE(scope_type, ''), COALESCE(scope_id, ''),
		        COALESCE(metadata, '{}'), created_at
		 FROM message WHERE scope_type = 'endeavour' AND scope_id = ?
		 ORDER BY created_at ASC`, endeavourID,
	)
	if err != nil {
		return nil, nil
	}
	defer func() { _ = rows.Close() }()

	var messageIDs []string
	for rows.Next() {
		var m Message
		var metaStr, createdStr string
		if err := rows.Scan(&m.ID, &m.SenderID, &m.Subject, &m.Content, &m.Intent,
			&m.ReplyToID, &m.EntityType, &m.EntityID, &m.ScopeType, &m.ScopeID,
			&metaStr, &createdStr); err != nil {
			continue
		}
		_ = json.Unmarshal([]byte(metaStr), &m.Metadata)
		m.CreatedAt = ParseDBTime(createdStr)
		messages = append(messages, m)
		messageIDs = append(messageIDs, m.ID)
	}

	var deliveries []MessageDelivery
	for _, msgID := range messageIDs {
		dRows, err := mdb.db.Query(
			`SELECT id, message_id, recipient_id, channel, status,
			        delivered_at, read_at, created_at
			 FROM message_delivery WHERE message_id = ?
			 ORDER BY created_at ASC`, msgID,
		)
		if err != nil {
			continue
		}
		for dRows.Next() {
			var d MessageDelivery
			var deliveredAt, readAt, createdAt sql.NullString
			if err := dRows.Scan(&d.ID, &d.MessageID, &d.RecipientID, &d.Channel, &d.Status,
				&deliveredAt, &readAt, &createdAt); err != nil {
				continue
			}
			if deliveredAt.Valid {
				t := ParseDBTime(deliveredAt.String)
				d.DeliveredAt = &t
			}
			if readAt.Valid {
				t := ParseDBTime(readAt.String)
				d.ReadAt = &t
			}
			if createdAt.Valid {
				d.CreatedAt = ParseDBTime(createdAt.String)
			}
			deliveries = append(deliveries, d)
		}
		_ = dRows.Close()
	}

	return messages, deliveries
}

// ExportOrganizationData creates a complete export of an organization,
// its members, and all linked endeavours with their contents.
func (db *DB) ExportOrganizationData(orgID string, mdb *MessageDB) (*OrgExport, error) {
	org, err := db.GetOrganization(orgID)
	if err != nil {
		return nil, fmt.Errorf("get organization: %w", err)
	}

	export := &OrgExport{
		Version:        1,
		ExportedAt:     UTCNow().Format(time.RFC3339),
		OrganizationID: orgID,
		Organization:   org,
	}

	// Members
	export.Members = db.exportOrgMembers(orgID)

	// Linked endeavours -- full export for each
	edvIDs, _ := db.GetOrganizationEndeavourIDs(orgID)
	for _, edvID := range edvIDs {
		edvExport, err := db.ExportEndeavourData(edvID)
		if err != nil {
			continue
		}
		// Attach messages from MessageDB if available.
		if mdb != nil {
			msgs, dels := mdb.ExportEndeavourMessages(edvID)
			if msgs != nil {
				edvExport.Messages = msgs
			}
			if dels != nil {
				edvExport.Deliveries = dels
			}
		}
		export.Endeavours = append(export.Endeavours, *edvExport)
	}

	// Org-level relations (org -> resource, org -> endeavour)
	orgEntityIDs := map[string]bool{orgID: true}
	for _, m := range export.Members {
		orgEntityIDs[m.Resource.ID] = true
	}
	export.Relations = db.exportRelationsForEntities(orgEntityIDs)

	export.ensureSlices()
	return export, nil
}

// --- Export helpers ---

func (db *DB) exportTokens(userID string) []backupToken {
	rows, err := db.Query(
		`SELECT id, user_id, token_hash, COALESCE(name, ''), created_at, COALESCE(expires_at, '')
		 FROM token WHERE user_id = ?`, userID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var tokens []backupToken
	for rows.Next() {
		var t backupToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.Name, &t.CreatedAt, &t.ExpiresAt); err != nil {
			continue
		}
		tokens = append(tokens, t)
	}
	return tokens
}

func (db *DB) findOwnedOrgs(resourceID string) []string {
	rows, err := db.Query(
		`SELECT source_entity_id FROM entity_relation
		 WHERE relationship_type = 'has_member'
		   AND source_entity_type = 'organization'
		   AND target_entity_type = 'resource'
		   AND target_entity_id = ?
		   AND json_extract(metadata, '$.role') = 'owner'`,
		resourceID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func (db *DB) findOwnedEndeavours(userID string) []string {
	rows, err := db.Query(
		`SELECT target_entity_id FROM entity_relation
		 WHERE relationship_type = 'member_of'
		   AND source_entity_type = 'user'
		   AND source_entity_id = ?
		   AND target_entity_type = 'endeavour'
		   AND json_extract(metadata, '$.role') = 'owner'`,
		userID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func (db *DB) exportEndeavourTasks(endeavourID string) []Task {
	// Find tasks linked to the endeavour via entity_relation.
	rows, err := db.Query(
		`SELECT source_entity_id FROM entity_relation
		 WHERE relationship_type = 'belongs_to'
		   AND source_entity_type = 'task'
		   AND target_entity_type = 'endeavour'
		   AND target_entity_id = ?`,
		endeavourID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var tasks []Task
	for rows.Next() {
		var taskID string
		if err := rows.Scan(&taskID); err == nil {
			task, err := db.GetTask(taskID)
			if err == nil {
				tasks = append(tasks, *task)
			}
		}
	}
	return tasks
}

func (db *DB) exportEndeavourDemands(endeavourID string) []Demand {
	rows, err := db.Query(
		`SELECT source_entity_id FROM entity_relation
		 WHERE relationship_type = 'belongs_to'
		   AND source_entity_type = 'demand'
		   AND target_entity_type = 'endeavour'
		   AND target_entity_id = ?`,
		endeavourID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var demands []Demand
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			d, err := db.GetDemand(id)
			if err == nil {
				demands = append(demands, *d)
			}
		}
	}
	return demands
}

func (db *DB) exportEndeavourArtifacts(endeavourID string) []Artifact {
	rows, err := db.Query(
		`SELECT source_entity_id FROM entity_relation
		 WHERE relationship_type = 'belongs_to'
		   AND source_entity_type = 'artifact'
		   AND target_entity_type = 'endeavour'
		   AND target_entity_id = ?`,
		endeavourID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var artifacts []Artifact
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			a, err := db.GetArtifact(id)
			if err == nil {
				artifacts = append(artifacts, *a)
			}
		}
	}
	return artifacts
}

func (db *DB) exportEndeavourRituals(endeavourID string) []Ritual {
	rows, err := db.Query(
		`SELECT source_entity_id FROM entity_relation
		 WHERE relationship_type = 'governs'
		   AND source_entity_type = 'ritual'
		   AND target_entity_type = 'endeavour'
		   AND target_entity_id = ?`,
		endeavourID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var rituals []Ritual
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			r, err := db.GetRitual(id)
			if err == nil {
				rituals = append(rituals, *r)
			}
		}
	}
	return rituals
}

func (db *DB) exportEndeavourRitualRuns(endeavourID string) []RitualRun {
	// Get ritual IDs for this endeavour, then their runs.
	rows, err := db.Query(
		`SELECT source_entity_id FROM entity_relation
		 WHERE relationship_type = 'governs'
		   AND source_entity_type = 'ritual'
		   AND target_entity_type = 'endeavour'
		   AND target_entity_id = ?`,
		endeavourID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var ritualIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ritualIDs = append(ritualIDs, id)
		}
	}

	var runs []RitualRun
	for _, ritualID := range ritualIDs {
		ritualRuns, _, _ := db.ListRitualRuns(ListRitualRunsOpts{RitualID: ritualID, Limit: 1000})
		for _, r := range ritualRuns {
			runs = append(runs, *r)
		}
	}
	return runs
}

func (db *DB) exportEndeavourDoDPolicies(endeavourID string) []backupDoDPolicy {
	rows, err := db.Query(
		`SELECT target_entity_id FROM entity_relation
		 WHERE relationship_type = 'governed_by'
		   AND source_entity_type = 'endeavour'
		   AND source_entity_id = ?
		   AND target_entity_type = 'dod_policy'`,
		endeavourID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var policies []backupDoDPolicy
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			p := db.exportDoDPolicy(id)
			if p != nil {
				policies = append(policies, *p)
			}
		}
	}
	return policies
}

func (db *DB) exportDoDPolicy(id string) *backupDoDPolicy {
	var p backupDoDPolicy
	var condStr, metaStr string
	err := db.QueryRow(
		`SELECT id, name, COALESCE(description, ''), scope, strictness, COALESCE(quorum, 0),
		        conditions, origin, COALESCE(predecessor_id, ''), COALESCE(created_by, ''),
		        status, COALESCE(metadata, '{}'), created_at, updated_at
		 FROM dod_policy WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Scope, &p.Strictness, &p.Quorum,
		&condStr, &p.Origin, &p.PredecessorID, &p.CreatedBy,
		&p.Status, &metaStr, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil
	}
	p.Conditions = json.RawMessage(condStr)
	_ = json.Unmarshal([]byte(metaStr), &p.Metadata)
	return &p
}

func (db *DB) exportUserComments(resourceID string) []Comment {
	rows, err := db.Query(
		`SELECT id, entity_type, entity_id, author_id, COALESCE(reply_to_id, ''),
		        content, COALESCE(metadata, '{}'), created_at, updated_at
		 FROM comment WHERE author_id = ? AND deleted_at IS NULL
		 ORDER BY created_at ASC`, resourceID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var comments []Comment
	for rows.Next() {
		var c Comment
		var metaStr, createdStr, updatedStr string
		if err := rows.Scan(&c.ID, &c.EntityType, &c.EntityID, &c.AuthorID, &c.ReplyToID,
			&c.Content, &metaStr, &createdStr, &updatedStr); err != nil {
			continue
		}
		_ = json.Unmarshal([]byte(metaStr), &c.Metadata)
		c.CreatedAt = ParseDBTime(createdStr)
		c.UpdatedAt = ParseDBTime(updatedStr)
		comments = append(comments, c)
	}
	return comments
}

func (db *DB) exportUserApprovals(resourceID string) []Approval {
	rows, err := db.Query(
		`SELECT id, entity_type, entity_id, approver_id, COALESCE(role, ''),
		        verdict, COALESCE(comment, ''), COALESCE(metadata, '{}'), created_at
		 FROM approval WHERE approver_id = ?
		 ORDER BY created_at ASC`, resourceID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var approvals []Approval
	for rows.Next() {
		var a Approval
		var metaStr, createdStr string
		if err := rows.Scan(&a.ID, &a.EntityType, &a.EntityID, &a.ApproverID, &a.Role,
			&a.Verdict, &a.Comment, &metaStr, &createdStr); err != nil {
			continue
		}
		_ = json.Unmarshal([]byte(metaStr), &a.Metadata)
		a.CreatedAt = ParseDBTime(createdStr)
		approvals = append(approvals, a)
	}
	return approvals
}

func (db *DB) exportRelationsForEntities(entityIDs map[string]bool) []EntityRelation {
	// Get all relations where either source or target is in our entity set.
	rows, err := db.Query(
		`SELECT id, relationship_type, source_entity_type, source_entity_id,
		        target_entity_type, target_entity_id, COALESCE(metadata, '{}'),
		        COALESCE(created_by, ''), created_at
		 FROM entity_relation
		 ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var relations []EntityRelation
	for rows.Next() {
		var r EntityRelation
		var metaStr, createdStr string
		if err := rows.Scan(&r.ID, &r.RelationshipType, &r.SourceEntityType, &r.SourceEntityID,
			&r.TargetEntityType, &r.TargetEntityID, &metaStr, &r.CreatedBy, &createdStr); err != nil {
			continue
		}
		// Only include relations where at least one side is in scope.
		if !entityIDs[r.SourceEntityID] && !entityIDs[r.TargetEntityID] {
			continue
		}
		_ = json.Unmarshal([]byte(metaStr), &r.Metadata)
		r.CreatedAt = ParseDBTime(createdStr)
		relations = append(relations, r)
	}
	return relations
}

// --- Restore helpers ---

func (db *DB) restoreResource(res *Resource) {
	metaBytes, _ := json.Marshal(res.Metadata)
	skillsBytes, _ := json.Marshal(res.Skills)
	_, _ = db.Exec(
		`INSERT OR IGNORE INTO resource (id, type, name, status, capacity_model, capacity_value, skills, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		res.ID, res.Type, res.Name, res.Status,
		res.CapacityModel, res.CapacityValue,
		string(skillsBytes), string(metaBytes),
		res.CreatedAt.Format(time.RFC3339), res.UpdatedAt.Format(time.RFC3339),
	)
}

func (db *DB) restoreUser(user *User) error {
	metaBytes, _ := json.Marshal(user.Metadata)
	var lastActive interface{}
	if user.LastActiveAt != nil {
		lastActive = user.LastActiveAt.Format(time.RFC3339)
	}
	_, err := db.Exec(
		`INSERT OR REPLACE INTO user (id, name, email, resource_id, status, tier, user_type, last_active_at, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Name, user.Email, user.ResourceID,
		user.Status, user.Tier, user.UserType, lastActive,
		string(metaBytes),
		user.CreatedAt.Format(time.RFC3339), user.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

func (db *DB) restoreOrganization(org *Organization) {
	metaBytes, _ := json.Marshal(org.Metadata)
	_, _ = db.Exec(
		`INSERT OR IGNORE INTO organization (id, name, description, status, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		org.ID, org.Name, org.Description, org.Status,
		string(metaBytes),
		org.CreatedAt.Format(time.RFC3339), org.UpdatedAt.Format(time.RFC3339),
	)
}

func (db *DB) restoreEndeavour(edv *Endeavour) {
	metaBytes, _ := json.Marshal(edv.Metadata)
	goalsBytes, _ := json.Marshal(edv.Goals)
	var startDate, endDate interface{}
	if edv.StartDate != nil {
		startDate = edv.StartDate.Format(time.RFC3339)
	}
	if edv.EndDate != nil {
		endDate = edv.EndDate.Format(time.RFC3339)
	}
	_, _ = db.Exec(
		`INSERT OR IGNORE INTO endeavour (id, name, description, status, start_date, end_date, goals, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		edv.ID, edv.Name, edv.Description, edv.Status,
		startDate, endDate, string(goalsBytes), string(metaBytes),
		edv.CreatedAt.Format(time.RFC3339), edv.UpdatedAt.Format(time.RFC3339),
	)
}

func (db *DB) restoreDemand(d *Demand) {
	metaBytes, _ := json.Marshal(d.Metadata)
	var dueDate interface{}
	if d.DueDate != nil {
		dueDate = d.DueDate.Format(time.RFC3339)
	}
	_, _ = db.Exec(
		`INSERT OR IGNORE INTO demand (id, type, title, description, status, priority, due_date, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Type, d.Title, d.Description, d.Status, d.Priority,
		dueDate, string(metaBytes),
		d.CreatedAt.Format(time.RFC3339), d.UpdatedAt.Format(time.RFC3339),
	)
}

func (db *DB) restoreTask(t *Task) {
	metaBytes, _ := json.Marshal(t.Metadata)
	var dueDate, startedAt, completedAt interface{}
	if t.DueDate != nil {
		dueDate = t.DueDate.Format(time.RFC3339)
	}
	if t.StartedAt != nil {
		startedAt = t.StartedAt.Format(time.RFC3339)
	}
	if t.CompletedAt != nil {
		completedAt = t.CompletedAt.Format(time.RFC3339)
	}
	_, _ = db.Exec(
		`INSERT OR IGNORE INTO task (id, title, description, status, estimate, actual, due_date, started_at, completed_at, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Title, t.Description, t.Status, t.Estimate, t.Actual,
		dueDate, startedAt, completedAt, string(metaBytes),
		t.CreatedAt.Format(time.RFC3339), t.UpdatedAt.Format(time.RFC3339),
	)
}

func (db *DB) restoreArtifact(a *Artifact) {
	metaBytes, _ := json.Marshal(a.Metadata)
	tagsBytes, _ := json.Marshal(a.Tags)
	_, _ = db.Exec(
		`INSERT OR IGNORE INTO artifact (id, kind, title, summary, url, status, tags, metadata, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Kind, a.Title, a.Summary, a.URL, a.Status,
		string(tagsBytes), string(metaBytes), a.CreatedBy,
		a.CreatedAt.Format(time.RFC3339), a.UpdatedAt.Format(time.RFC3339),
	)
}

func (db *DB) restoreRitual(r *Ritual) {
	metaBytes, _ := json.Marshal(r.Metadata)
	schedBytes, _ := json.Marshal(r.Schedule)
	_, _ = db.Exec(
		`INSERT OR IGNORE INTO ritual (id, name, description, prompt, origin, predecessor_id, is_enabled, status, schedule, metadata, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.Description, r.Prompt, r.Origin, r.PredecessorID,
		r.IsEnabled, r.Status, string(schedBytes), string(metaBytes), r.CreatedBy,
		r.CreatedAt.Format(time.RFC3339), r.UpdatedAt.Format(time.RFC3339),
	)
}

func (db *DB) restoreDoDPolicy(p *backupDoDPolicy) {
	metaBytes, _ := json.Marshal(p.Metadata)
	_, _ = db.Exec(
		`INSERT OR IGNORE INTO dod_policy (id, name, description, scope, strictness, quorum, conditions, origin, predecessor_id, created_by, status, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.Scope, p.Strictness, p.Quorum,
		string(p.Conditions), p.Origin, p.PredecessorID, p.CreatedBy,
		p.Status, string(metaBytes), p.CreatedAt, p.UpdatedAt,
	)
}

func (db *DB) restoreRitualRun(r *RitualRun) {
	metaBytes, _ := json.Marshal(r.Metadata)
	effectsBytes, _ := json.Marshal(r.Effects)
	errorBytes, _ := json.Marshal(r.Error)
	var startedAt, finishedAt interface{}
	if r.StartedAt != nil {
		startedAt = r.StartedAt.Format(time.RFC3339)
	}
	if r.FinishedAt != nil {
		finishedAt = r.FinishedAt.Format(time.RFC3339)
	}
	_, _ = db.Exec(
		`INSERT OR IGNORE INTO ritual_run (id, ritual_id, status, trigger, run_by, result_summary, effects, error, metadata, started_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.RitualID, r.Status, r.Trigger, r.RunBy,
		r.ResultSummary, string(effectsBytes), string(errorBytes),
		string(metaBytes), startedAt, finishedAt,
	)
}

func (db *DB) restoreRelation(r *EntityRelation) {
	metaBytes, _ := json.Marshal(r.Metadata)
	_, _ = db.Exec(
		`INSERT OR IGNORE INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id, target_entity_type, target_entity_id, metadata, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.RelationshipType, r.SourceEntityType, r.SourceEntityID,
		r.TargetEntityType, r.TargetEntityID, string(metaBytes), r.CreatedBy,
		r.CreatedAt.Format(time.RFC3339),
	)
}

func (db *DB) restoreComment(c *Comment) {
	metaBytes, _ := json.Marshal(c.Metadata)
	_, _ = db.Exec(
		`INSERT OR IGNORE INTO comment (id, entity_type, entity_id, author_id, content, reply_to_id, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?)`,
		c.ID, c.EntityType, c.EntityID, c.AuthorID, c.Content,
		c.ReplyToID, string(metaBytes),
		c.CreatedAt.Format(time.RFC3339), c.UpdatedAt.Format(time.RFC3339),
	)
}

func (db *DB) restoreApproval(a *Approval) {
	metaBytes, _ := json.Marshal(a.Metadata)
	_, _ = db.Exec(
		`INSERT OR IGNORE INTO approval (id, entity_type, entity_id, approver_id, role, verdict, comment, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.EntityType, a.EntityID, a.ApproverID, a.Role,
		a.Verdict, a.Comment, string(metaBytes),
		a.CreatedAt.Format(time.RFC3339),
	)
}

func (db *DB) restoreToken(t *backupToken) {
	_, _ = db.Exec(
		`INSERT OR IGNORE INTO token (id, user_id, token_hash, name, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, NULLIF(?, ''))`,
		t.ID, t.UserID, t.TokenHash, t.Name, t.CreatedAt, t.ExpiresAt,
	)
}

// --- Entity-scoped export helpers ---

func (db *DB) exportOrgMembers(orgID string) []backupOrgMember {
	rows, err := db.Query(
		`SELECT target_entity_id, COALESCE(json_extract(metadata, '$.role'), 'member')
		 FROM entity_relation
		 WHERE relationship_type = 'has_member'
		   AND source_entity_type = 'organization'
		   AND source_entity_id = ?
		   AND target_entity_type = 'resource'`,
		orgID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var members []backupOrgMember
	for rows.Next() {
		var resID, role string
		if err := rows.Scan(&resID, &role); err != nil {
			continue
		}
		res, err := db.GetResource(resID)
		if err != nil {
			continue
		}
		members = append(members, backupOrgMember{Resource: res, Role: role})
	}
	return members
}

func (db *DB) exportEndeavourEndorsements(endeavourID string) []backupEndorsement {
	rows, err := db.Query(
		`SELECT id, policy_id, policy_version, resource_id, endeavour_id,
		        status, endorsed_at, COALESCE(superseded_at, ''), created_at
		 FROM dod_endorsement WHERE endeavour_id = ?
		 ORDER BY created_at ASC`, endeavourID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var endorsements []backupEndorsement
	for rows.Next() {
		var e backupEndorsement
		if err := rows.Scan(&e.ID, &e.PolicyID, &e.PolicyVersion, &e.ResourceID, &e.EndeavourID,
			&e.Status, &e.EndorsedAt, &e.SupersededAt, &e.CreatedAt); err != nil {
			continue
		}
		endorsements = append(endorsements, e)
	}
	return endorsements
}

func (db *DB) exportScopedComments(entityIDs map[string]bool) []Comment {
	rows, err := db.Query(
		`SELECT id, entity_type, entity_id, author_id, COALESCE(reply_to_id, ''),
		        content, COALESCE(metadata, '{}'), created_at, updated_at
		 FROM comment WHERE deleted_at IS NULL
		 ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var comments []Comment
	for rows.Next() {
		var c Comment
		var metaStr, createdStr, updatedStr string
		if err := rows.Scan(&c.ID, &c.EntityType, &c.EntityID, &c.AuthorID, &c.ReplyToID,
			&c.Content, &metaStr, &createdStr, &updatedStr); err != nil {
			continue
		}
		if !entityIDs[c.EntityID] {
			continue
		}
		_ = json.Unmarshal([]byte(metaStr), &c.Metadata)
		c.CreatedAt = ParseDBTime(createdStr)
		c.UpdatedAt = ParseDBTime(updatedStr)
		comments = append(comments, c)
	}
	return comments
}

func (db *DB) exportScopedApprovals(entityIDs map[string]bool) []Approval {
	rows, err := db.Query(
		`SELECT id, entity_type, entity_id, approver_id, COALESCE(role, ''),
		        verdict, COALESCE(comment, ''), COALESCE(metadata, '{}'), created_at
		 FROM approval
		 ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var approvals []Approval
	for rows.Next() {
		var a Approval
		var metaStr, createdStr string
		if err := rows.Scan(&a.ID, &a.EntityType, &a.EntityID, &a.ApproverID, &a.Role,
			&a.Verdict, &a.Comment, &metaStr, &createdStr); err != nil {
			continue
		}
		if !entityIDs[a.EntityID] {
			continue
		}
		_ = json.Unmarshal([]byte(metaStr), &a.Metadata)
		a.CreatedAt = ParseDBTime(createdStr)
		approvals = append(approvals, a)
	}
	return approvals
}
