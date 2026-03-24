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
	"strings"
	"sync"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// validateTaskCreate validates fields for task creation.
func validateTaskCreate(title, description, endeavourID, demandID, assigneeID string, metadata map[string]interface{}) *APIError {
	if err := security.ValidateTitle(title); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateDescription(description); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateID(endeavourID, "endeavour_id"); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateID(demandID, "demand_id"); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateID(assigneeID, "assignee_id"); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateMetadata(metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateTaskUpdate validates fields for task update.
func validateTaskUpdate(id string, f storage.UpdateTaskFields) *APIError {
	if err := security.ValidateID(id, "id"); err != nil {
		return validationErr(err)
	}
	if f.Title != nil {
		if err := security.ValidateTitle(*f.Title); err != nil {
			return validationErr(err)
		}
	}
	if f.Description != nil {
		if err := security.ValidateDescription(*f.Description); err != nil {
			return validationErr(err)
		}
	}
	if f.EndeavourID != nil && *f.EndeavourID != "" {
		if err := security.ValidateID(*f.EndeavourID, "endeavour_id"); err != nil {
			return validationErr(err)
		}
	}
	if f.DemandID != nil && *f.DemandID != "" {
		if err := security.ValidateID(*f.DemandID, "demand_id"); err != nil {
			return validationErr(err)
		}
	}
	if f.AssigneeID != nil && *f.AssigneeID != "" {
		if err := security.ValidateID(*f.AssigneeID, "assignee_id"); err != nil {
			return validationErr(err)
		}
	}
	if f.CanceledReason != nil {
		if err := security.ValidateStringField(*f.CanceledReason, "canceled_reason", security.MaxDescriptionLen); err != nil {
			return validationErr(err)
		}
	}
	if err := security.ValidateMetadata(f.Metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateDemandCreate validates fields for demand creation.
func validateDemandCreate(dtype, title, description, priority, endeavourID string, metadata map[string]interface{}) *APIError {
	if err := security.ValidateName(dtype); err != nil {
		err.Field = "type"
		return validationErr(err)
	}
	if err := security.ValidateTitle(title); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateDescription(description); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateStringField(priority, "priority", security.MaxNameLen); err != nil {
		return validationErr(err)
	}
	if endeavourID == "" {
		return errInvalidInput("endeavour_id is required")
	}
	if err := security.ValidateID(endeavourID, "endeavour_id"); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateMetadata(metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateDemandUpdate validates fields for demand update.
func validateDemandUpdate(id string, f storage.UpdateDemandFields) *APIError {
	if err := security.ValidateID(id, "id"); err != nil {
		return validationErr(err)
	}
	if f.Title != nil {
		if err := security.ValidateTitle(*f.Title); err != nil {
			return validationErr(err)
		}
	}
	if f.Description != nil {
		if err := security.ValidateDescription(*f.Description); err != nil {
			return validationErr(err)
		}
	}
	if f.Type != nil {
		if err := security.ValidateStringField(*f.Type, "type", security.MaxNameLen); err != nil {
			return validationErr(err)
		}
	}
	if f.Priority != nil {
		if err := security.ValidateStringField(*f.Priority, "priority", security.MaxNameLen); err != nil {
			return validationErr(err)
		}
	}
	if f.EndeavourID != nil && *f.EndeavourID != "" {
		if err := security.ValidateID(*f.EndeavourID, "endeavour_id"); err != nil {
			return validationErr(err)
		}
	}
	if f.CanceledReason != nil {
		if err := security.ValidateStringField(*f.CanceledReason, "canceled_reason", security.MaxDescriptionLen); err != nil {
			return validationErr(err)
		}
	}
	if err := security.ValidateMetadata(f.Metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateEndeavourCreate validates fields for endeavour creation.
func validateEndeavourCreate(name, description string, goals []storage.Goal, metadata map[string]interface{}) *APIError {
	if err := security.ValidateName(name); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateDescription(description); err != nil {
		return validationErr(err)
	}
	if apiErr := validateGoals(goals); apiErr != nil {
		return apiErr
	}
	if err := security.ValidateMetadata(metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateEndeavourUpdate validates fields for endeavour update.
func validateEndeavourUpdate(id string, f storage.UpdateEndeavourFields) *APIError {
	if err := security.ValidateID(id, "id"); err != nil {
		return validationErr(err)
	}
	if f.Name != nil {
		if err := security.ValidateName(*f.Name); err != nil {
			return validationErr(err)
		}
	}
	if f.Description != nil {
		if err := security.ValidateDescription(*f.Description); err != nil {
			return validationErr(err)
		}
	}
	if f.Goals != nil {
		if apiErr := validateGoals(f.Goals); apiErr != nil {
			return apiErr
		}
	}
	if err := security.ValidateMetadata(f.Metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateGoals validates structured goals by extracting titles and running
// tag validation on them.
func validateGoals(goals []storage.Goal) *APIError {
	titles := make([]string, len(goals))
	for i, g := range goals {
		titles[i] = g.Title
	}
	if err := security.ValidateTags(titles); err != nil {
		err.Field = "goals"
		return validationErr(err)
	}
	// Validate goal status values
	validStatuses := map[string]bool{"": true, "open": true, "achieved": true, "abandoned": true}
	for _, g := range goals {
		if !validStatuses[g.Status] {
			return errInvalidInput("Invalid goal status: " + g.Status + ". Must be open, achieved, or abandoned.")
		}
	}
	return nil
}

// validateOrganizationCreate validates fields for organization creation.
func validateOrganizationCreate(name, description string, metadata map[string]interface{}) *APIError {
	if err := security.ValidateName(name); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateDescription(description); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateMetadata(metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateOrganizationUpdate validates fields for organization update.
func validateOrganizationUpdate(id string, f storage.UpdateOrganizationFields) *APIError {
	if err := security.ValidateID(id, "id"); err != nil {
		return validationErr(err)
	}
	if f.Name != nil {
		if err := security.ValidateName(*f.Name); err != nil {
			return validationErr(err)
		}
	}
	if f.Description != nil {
		if err := security.ValidateDescription(*f.Description); err != nil {
			return validationErr(err)
		}
	}
	if err := security.ValidateMetadata(f.Metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateCommentCreate validates fields for comment creation.
func validateCommentCreate(entityType, entityID, content, replyToID string, metadata map[string]interface{}) *APIError {
	if err := security.ValidateStringField(entityType, "entity_type", security.MaxNameLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateID(entityID, "entity_id"); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateStringField(content, "content", security.MaxContentLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateID(replyToID, "reply_to_id"); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateMetadata(metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateCommentUpdate validates fields for comment update.
func validateCommentUpdate(id string, content *string, metadata map[string]interface{}) *APIError {
	if err := security.ValidateID(id, "id"); err != nil {
		return validationErr(err)
	}
	if content != nil {
		if err := security.ValidateStringField(*content, "content", security.MaxContentLen); err != nil {
			return validationErr(err)
		}
	}
	if err := security.ValidateMetadata(metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateMessageSend validates fields for message sending.
func validateMessageSend(subject, content, intent, entityType, entityID, scopeType, scopeID, replyToID string, recipientIDs []string, metadata map[string]interface{}) *APIError {
	if err := security.ValidateStringField(subject, "subject", security.MaxTitleLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateStringField(content, "content", security.MaxContentLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateStringField(intent, "intent", security.MaxNameLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateStringField(entityType, "entity_type", security.MaxNameLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateID(entityID, "entity_id"); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateStringField(scopeType, "scope_type", security.MaxNameLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateID(scopeID, "scope_id"); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateID(replyToID, "reply_to_id"); err != nil {
		return validationErr(err)
	}
	for _, rid := range recipientIDs {
		if err := security.ValidateID(rid, "recipient_ids"); err != nil {
			return validationErr(err)
		}
	}
	if err := security.ValidateMetadata(metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateArtifactCreate validates fields for artifact creation.
func validateArtifactCreate(kind, title, rawURL, summary string, tags []string, endeavourID, taskID string, metadata map[string]interface{}) *APIError {
	if err := security.ValidateStringField(kind, "kind", security.MaxNameLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateTitle(title); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateURL(rawURL); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateDescription(summary); err != nil {
		err.Field = "summary"
		return validationErr(err)
	}
	if err := security.ValidateTags(tags); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateID(endeavourID, "endeavour_id"); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateID(taskID, "task_id"); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateMetadata(metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateArtifactUpdate validates fields for artifact update.
func validateArtifactUpdate(id string, f storage.UpdateArtifactFields) *APIError {
	if err := security.ValidateID(id, "id"); err != nil {
		return validationErr(err)
	}
	if f.Title != nil {
		if err := security.ValidateTitle(*f.Title); err != nil {
			return validationErr(err)
		}
	}
	if f.URL != nil {
		if err := security.ValidateURL(*f.URL); err != nil {
			return validationErr(err)
		}
	}
	if f.Summary != nil {
		if err := security.ValidateStringField(*f.Summary, "summary", security.MaxDescriptionLen); err != nil {
			return validationErr(err)
		}
	}
	if f.Tags != nil {
		if err := security.ValidateTags(*f.Tags); err != nil {
			return validationErr(err)
		}
	}
	if f.EndeavourID != nil && *f.EndeavourID != "" {
		if err := security.ValidateID(*f.EndeavourID, "endeavour_id"); err != nil {
			return validationErr(err)
		}
	}
	if f.TaskID != nil && *f.TaskID != "" {
		if err := security.ValidateID(*f.TaskID, "task_id"); err != nil {
			return validationErr(err)
		}
	}
	if err := security.ValidateMetadata(f.Metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateRitualCreate validates fields for ritual creation.
func validateRitualCreate(name, prompt, description, endeavourID string, metadata map[string]interface{}) *APIError {
	if err := security.ValidateName(name); err != nil {
		return validationErr(err)
	}
	if err := security.ValidatePrompt(prompt); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateDescription(description); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateID(endeavourID, "endeavour_id"); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateMetadata(metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateRitualUpdate validates fields for ritual update.
func validateRitualUpdate(id string, f storage.UpdateRitualFields) *APIError {
	if err := security.ValidateID(id, "id"); err != nil {
		return validationErr(err)
	}
	if f.Name != nil {
		if err := security.ValidateName(*f.Name); err != nil {
			return validationErr(err)
		}
	}
	if f.Description != nil {
		if err := security.ValidateDescription(*f.Description); err != nil {
			return validationErr(err)
		}
	}
	if f.EndeavourID != nil && *f.EndeavourID != "" {
		if err := security.ValidateID(*f.EndeavourID, "endeavour_id"); err != nil {
			return validationErr(err)
		}
	}
	if err := security.ValidateMetadata(f.Metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateResourceCreate validates fields for resource creation.
func validateResourceCreate(rtype, name, capacityModel string, skills []string, metadata map[string]interface{}) *APIError {
	if err := security.ValidateStringField(rtype, "type", security.MaxNameLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateName(name); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateStringField(capacityModel, "capacity_model", security.MaxNameLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateTags(skills); err != nil {
		err.Field = "skills"
		return validationErr(err)
	}
	if err := security.ValidateMetadata(metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateResourceUpdate validates fields for resource update.
func validateResourceUpdate(id string, f storage.UpdateResourceFields) *APIError {
	if err := security.ValidateID(id, "id"); err != nil {
		return validationErr(err)
	}
	if f.Name != nil {
		if err := security.ValidateName(*f.Name); err != nil {
			return validationErr(err)
		}
	}
	if f.Skills != nil {
		if err := security.ValidateTags(f.Skills); err != nil {
			err.Field = "skills"
			return validationErr(err)
		}
	}
	if err := security.ValidateMetadata(f.Metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateRelationCreate validates fields for relation creation.
func validateRelationCreate(relType, srcType, srcID, tgtType, tgtID string, metadata map[string]interface{}) *APIError {
	if err := security.ValidateStringField(relType, "relationship_type", security.MaxNameLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateStringField(srcType, "source_entity_type", security.MaxNameLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateID(srcID, "source_entity_id"); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateStringField(tgtType, "target_entity_type", security.MaxNameLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateID(tgtID, "target_entity_id"); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateMetadata(metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateApprovalCreate validates fields for approval creation.
func validateApprovalCreate(entityType, entityID, verdict, role, comment string, metadata map[string]interface{}) *APIError {
	if err := security.ValidateStringField(entityType, "entity_type", security.MaxNameLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateID(entityID, "entity_id"); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateStringField(verdict, "verdict", security.MaxNameLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateStringField(role, "role", security.MaxNameLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateStringField(comment, "comment", security.MaxContentLen); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateMetadata(metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateDodCreate validates fields for DoD policy creation.
func validateDodCreate(name, description string, metadata map[string]interface{}) *APIError {
	if err := security.ValidateName(name); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateDescription(description); err != nil {
		return validationErr(err)
	}
	if err := security.ValidateMetadata(metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateDodUpdate validates fields for DoD policy update.
func validateDodUpdate(id string, f storage.UpdateDodPolicyFields) *APIError {
	if err := security.ValidateID(id, "id"); err != nil {
		return validationErr(err)
	}
	if f.Name != nil {
		if err := security.ValidateName(*f.Name); err != nil {
			return validationErr(err)
		}
	}
	if f.Description != nil {
		if err := security.ValidateDescription(*f.Description); err != nil {
			return validationErr(err)
		}
	}
	if err := security.ValidateMetadata(f.Metadata); err != nil {
		return validationErr(err)
	}
	return nil
}

// validateEntityID validates a single ID parameter.
func validateEntityID(id, fieldName string) *APIError {
	if err := security.ValidateID(id, fieldName); err != nil {
		return validationErr(err)
	}
	return nil
}

// validationErr converts a security.ValidationError to an APIError.
func validationErr(err *security.ValidationError) *APIError {
	return errInvalidInput(err.Error())
}

// scoreAndAnnotate runs heuristic injection scoring on the given text fields
// and merges the result into the metadata map. If the score is 0, metadata
// is returned unchanged. This is advisory only -- writes are never blocked.
func scoreAndAnnotate(metadata map[string]interface{}, fields ...string) map[string]interface{} {
	var buf strings.Builder
	for _, f := range fields {
		if f != "" {
			if buf.Len() > 0 {
				buf.WriteByte('\n')
			}
			buf.WriteString(f)
		}
	}
	if buf.Len() == 0 {
		return metadata
	}

	hs := security.ScoreContent(buf.String())
	if hs.Score == 0 {
		return metadata
	}

	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["harm_score"] = hs.Score
	// Convert []string to []interface{} for JSON serialization consistency.
	signals := make([]interface{}, len(hs.Signals))
	for i, s := range hs.Signals {
		signals[i] = s
	}
	metadata["harm_signals"] = signals

	// Queue for LLM content scoring if above threshold (WS-4.5).
	if hs.Score >= contentGuardScoreThreshold {
		metadata["harm_score_llm_status"] = "pending"
	}

	return metadata
}

// contentGuardScoreThreshold is the minimum heuristic harm_score that triggers
// LLM content scoring. Set via SetContentGuardThreshold at startup.
var contentGuardScoreThreshold = 20

// SetContentGuardThreshold configures the minimum heuristic score that triggers
// LLM content review. Called at startup from the content-guard config.
// Use 0 to send all entities to the LLM regardless of heuristic score.
func SetContentGuardThreshold(threshold int) {
	if threshold >= 0 {
		contentGuardScoreThreshold = threshold
	}
}

// derefStr safely dereferences a string pointer, returning "" for nil.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ---------------------------------------------------------------------------
// Org Alert Terms scoring
// ---------------------------------------------------------------------------

// orgTermCacheEntry holds cached org alert terms with TTL.
type orgTermCacheEntry struct {
	terms   []*storage.OrgAlertTerm
	fetched time.Time
}

var (
	orgTermCache   = map[string]*orgTermCacheEntry{}
	orgTermCacheMu sync.RWMutex
	orgTermCacheTTL = 5 * time.Minute
)

// applyOrgAlertTerms checks entity text fields against per-org alert terms
// and additively increases the harm_score. Org terms are strictly additive.
func (a *API) applyOrgAlertTerms(metadata map[string]interface{}, endeavourID string, fields ...string) map[string]interface{} {
	if endeavourID == "" {
		return metadata
	}

	// Resolve endeavour -> org via relation.
	orgID := a.db.GetRelationSourceID(
		storage.RelParticipatesIn,
		storage.EntityOrganization,
		storage.EntityEndeavour,
		endeavourID,
	)
	if orgID == "" {
		return metadata
	}

	// Load terms with cache.
	terms := a.loadOrgTermsCached(orgID)
	if len(terms) == 0 {
		return metadata
	}

	// Concatenate fields for matching.
	var buf strings.Builder
	for _, f := range fields {
		if f != "" {
			if buf.Len() > 0 {
				buf.WriteByte(' ')
			}
			buf.WriteString(f)
		}
	}
	text := strings.ToLower(buf.String())
	if text == "" {
		return metadata
	}

	// Case-insensitive plain-text matching.
	var termScore int
	for _, t := range terms {
		if strings.Contains(text, strings.ToLower(t.Term)) {
			termScore += t.Weight
		}
	}
	if termScore == 0 {
		return metadata
	}

	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	// Add to existing harm_score (capped at 100).
	existing := 0
	if v, ok := metadata["harm_score"]; ok {
		switch n := v.(type) {
		case int:
			existing = n
		case float64:
			existing = int(n)
		}
	}
	newScore := existing + termScore
	if newScore > 100 {
		newScore = 100
	}
	metadata["harm_score"] = newScore

	// Add "org_alert_terms" to signals.
	var signals []interface{}
	if v, ok := metadata["harm_signals"]; ok {
		if arr, ok := v.([]interface{}); ok {
			signals = arr
		}
	}
	hasOrgSignal := false
	for _, s := range signals {
		if s == "org_alert_terms" {
			hasOrgSignal = true
			break
		}
	}
	if !hasOrgSignal {
		signals = append(signals, "org_alert_terms")
	}
	metadata["harm_signals"] = signals

	// Queue for LLM if crossed threshold.
	if newScore >= contentGuardScoreThreshold {
		if _, ok := metadata["harm_score_llm_status"]; !ok {
			metadata["harm_score_llm_status"] = "pending"
		}
	}

	return metadata
}

// loadOrgTermsCached returns org alert terms with in-memory caching.
func (a *API) loadOrgTermsCached(orgID string) []*storage.OrgAlertTerm {
	orgTermCacheMu.RLock()
	entry, ok := orgTermCache[orgID]
	orgTermCacheMu.RUnlock()

	if ok && storage.UTCNow().Sub(entry.fetched) < orgTermCacheTTL {
		return entry.terms
	}

	terms, err := a.db.ListOrgAlertTerms(orgID)
	if err != nil {
		return nil
	}

	orgTermCacheMu.Lock()
	orgTermCache[orgID] = &orgTermCacheEntry{
		terms:   terms,
		fetched: storage.UTCNow(),
	}
	orgTermCacheMu.Unlock()

	return terms
}
