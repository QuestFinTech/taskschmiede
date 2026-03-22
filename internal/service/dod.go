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


package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// Valid DoD policy origins.
var validDodOrigins = map[string]bool{
	"template": true,
	"custom":   true,
	"derived":  true,
}

// Valid DoD strictness values.
var validDodStrictness = map[string]bool{
	"all":  true,
	"n_of": true,
}

// DodService handles Definition of Done business logic.
type DodService struct {
	db     *storage.DB
	logger *slog.Logger
}

// NewDodService creates a new DodService.
func NewDodService(db *storage.DB, logger *slog.Logger) *DodService {
	return &DodService{db: db, logger: logger}
}

// Create creates a new DoD policy.
func (s *DodService) Create(ctx context.Context, name, description, origin, createdBy string, conditions []storage.DodCondition, strictness string, quorum int, metadata map[string]interface{}) (*storage.DodPolicy, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(conditions) == 0 {
		return nil, fmt.Errorf("at least one condition is required")
	}

	if origin == "" {
		origin = "custom"
	}
	if !validDodOrigins[origin] {
		return nil, fmt.Errorf("invalid origin: %s (must be template, custom, or derived)", origin)
	}

	if strictness == "" {
		strictness = "all"
	}
	if !validDodStrictness[strictness] {
		return nil, fmt.Errorf("invalid strictness: %s (must be all or n_of)", strictness)
	}
	if strictness == "n_of" && quorum <= 0 {
		return nil, fmt.Errorf("quorum is required when strictness is n_of")
	}

	policy, err := s.db.CreateDodPolicy(name, description, origin, createdBy, conditions, strictness, quorum, "task", 1, "", metadata)
	if err != nil {
		return nil, fmt.Errorf("create dod policy: %w", err)
	}

	s.logger.Info("DoD policy created", "id", policy.ID, "name", name, "origin", origin)
	return policy, nil
}

// Get retrieves a DoD policy by ID.
func (s *DodService) Get(ctx context.Context, id string) (*storage.DodPolicy, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	return s.db.GetDodPolicy(id)
}

// List queries DoD policies with filters.
func (s *DodService) List(ctx context.Context, opts storage.ListDodPoliciesOpts) ([]*storage.DodPolicy, int, error) {
	return s.db.ListDodPolicies(opts)
}

// Update applies partial updates to a DoD policy.
// Rejects updates to template policies.
func (s *DodService) Update(ctx context.Context, id string, fields storage.UpdateDodPolicyFields) ([]string, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	policy, err := s.db.GetDodPolicy(id)
	if err != nil {
		return nil, err
	}

	if policy.Origin == "template" {
		return nil, fmt.Errorf("cannot update template policies; fork to create a derived version")
	}

	if fields.Status != nil {
		switch *fields.Status {
		case "active", "archived":
			// valid
		default:
			return nil, fmt.Errorf("invalid status: %s (must be active or archived)", *fields.Status)
		}
	}

	updatedFields, err := s.db.UpdateDodPolicy(id, fields)
	if err != nil {
		return nil, err
	}

	s.logger.Info("DoD policy updated", "id", id, "fields", updatedFields)
	return updatedFields, nil
}

// NewVersion creates a new version of an existing policy.
// The old policy is archived, governed_by relations are repointed, and endorsements are superseded.
func (s *DodService) NewVersion(ctx context.Context, id, name, description string, conditions []storage.DodCondition, strictness string, quorum int, metadata map[string]interface{}, createdBy string) (*storage.DodPolicy, error) {
	old, err := s.db.GetDodPolicy(id)
	if err != nil {
		return nil, err
	}

	if old.Status == "archived" {
		return nil, fmt.Errorf("cannot create new version of archived policy")
	}
	if old.Origin == "template" {
		return nil, fmt.Errorf("cannot version template policies; fork to create a derived version")
	}

	// Defaults from old version
	if name == "" {
		name = old.Name
	}
	if description == "" {
		description = old.Description
	}
	if len(conditions) == 0 {
		conditions = old.Conditions
	}
	if strictness == "" {
		strictness = old.Strictness
	}
	if quorum == 0 {
		quorum = old.Quorum
	}

	origin := old.Origin
	if origin == "template" {
		origin = "derived"
	}

	// Create new version
	newPolicy, err := s.db.CreateDodPolicy(name, description, origin, createdBy, conditions, strictness, quorum, old.Scope, old.Version+1, old.ID, metadata)
	if err != nil {
		return nil, fmt.Errorf("create new policy version: %w", err)
	}

	// Archive old policy
	archived := "archived"
	_, _ = s.db.UpdateDodPolicy(id, storage.UpdateDodPolicyFields{Status: &archived})

	// Repoint governed_by relations from old to new
	rels, _, _ := s.db.ListRelations(storage.ListRelationsOpts{
		TargetEntityType: storage.EntityDodPolicy,
		TargetEntityID:   old.ID,
		RelationshipType: storage.RelGovernedBy,
	})
	for _, rel := range rels {
		_ = s.db.SetRelation(storage.RelGovernedBy, rel.SourceEntityType, rel.SourceEntityID, storage.EntityDodPolicy, newPolicy.ID, createdBy)
	}

	// Supersede endorsements for old version
	superseded, _ := s.db.SupersedeDodEndorsements(old.ID, old.Version)

	s.logger.Info("DoD policy new version",
		"old_id", old.ID, "new_id", newPolicy.ID,
		"version", newPolicy.Version,
		"endorsements_superseded", superseded)

	return newPolicy, nil
}

// Lineage returns the full predecessor chain for a policy.
func (s *DodService) Lineage(ctx context.Context, id string) ([]*storage.DodPolicy, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	return s.db.GetDodPolicyLineage(id)
}

// Assign assigns a DoD policy to an endeavour via the governed_by FRM relation.
func (s *DodService) Assign(ctx context.Context, endeavourID, policyID, assignedBy string) error {
	if endeavourID == "" {
		return fmt.Errorf("endeavour_id is required")
	}
	if policyID == "" {
		return fmt.Errorf("policy_id is required")
	}

	// Verify both exist
	if _, err := s.db.GetEndeavour(endeavourID); err != nil {
		return storage.ErrEndeavourNotFound
	}
	if _, err := s.db.GetDodPolicy(policyID); err != nil {
		return err
	}

	return s.db.SetRelation(storage.RelGovernedBy, storage.EntityEndeavour, endeavourID, storage.EntityDodPolicy, policyID, assignedBy)
}

// Unassign removes the DoD policy from an endeavour.
func (s *DodService) Unassign(ctx context.Context, endeavourID, removedBy string) error {
	if endeavourID == "" {
		return fmt.Errorf("endeavour_id is required")
	}

	return s.db.SetRelation(storage.RelGovernedBy, storage.EntityEndeavour, endeavourID, storage.EntityDodPolicy, "", removedBy)
}

// Endorse records a resource's endorsement of the current DoD policy for an endeavour.
func (s *DodService) Endorse(ctx context.Context, resourceID, endeavourID string) (*storage.DodEndorsement, error) {
	if resourceID == "" {
		return nil, fmt.Errorf("resource_id is required")
	}
	if endeavourID == "" {
		return nil, fmt.Errorf("endeavour_id is required")
	}

	// Look up the current policy for this endeavour
	policy, err := s.GetEndeavourPolicy(ctx, endeavourID)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, fmt.Errorf("no DoD policy assigned to endeavour %s", endeavourID)
	}

	endorsement, err := s.db.CreateDodEndorsement(policy.ID, policy.Version, resourceID, endeavourID)
	if err != nil {
		return nil, fmt.Errorf("create endorsement: %w", err)
	}

	s.logger.Info("DoD policy endorsed",
		"endorsement_id", endorsement.ID,
		"policy_id", policy.ID,
		"policy_version", policy.Version,
		"resource_id", resourceID,
		"endeavour_id", endeavourID)

	return endorsement, nil
}

// ListEndorsements queries endorsements with filters.
func (s *DodService) ListEndorsements(ctx context.Context, opts storage.ListDodEndorsementsOpts) ([]*storage.DodEndorsement, int, error) {
	return s.db.ListDodEndorsements(opts)
}

// GetEndeavourPolicy returns the DoD policy assigned to an endeavour, or nil if none.
func (s *DodService) GetEndeavourPolicy(ctx context.Context, endeavourID string) (*storage.DodPolicy, error) {
	policyID := s.db.GetRelationTargetID(storage.RelGovernedBy, storage.EntityEndeavour, endeavourID, storage.EntityDodPolicy)
	if policyID == "" {
		return nil, nil
	}
	return s.db.GetDodPolicy(policyID)
}

// ConditionResult holds the evaluation result for a single DoD condition.
type ConditionResult struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Label  string `json:"label"`
	Status string `json:"status"` // passed, failed, skipped
	Detail string `json:"detail,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

// CheckResult holds the full DoD evaluation result for a task.
type CheckResult struct {
	PolicyID      string            `json:"policy_id"`
	PolicyName    string            `json:"policy_name"`
	PolicyVersion int               `json:"policy_version"`
	Result        string            `json:"result"` // met, not_met
	Conditions    []ConditionResult `json:"conditions"`
	Hint          string            `json:"hint,omitempty"`
}

// Check evaluates DoD conditions for a task without modifying it.
// Returns nil if no DoD policy applies.
func (s *DodService) Check(ctx context.Context, taskID, resourceID string) (*CheckResult, error) {
	task, err := s.db.GetTask(taskID)
	if err != nil {
		return nil, err
	}

	if task.EndeavourID == "" {
		return nil, nil // no endeavour, no DoD
	}

	policy, err := s.GetEndeavourPolicy(ctx, task.EndeavourID)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, nil // no policy assigned
	}

	result := &CheckResult{
		PolicyID:      policy.ID,
		PolicyName:    policy.Name,
		PolicyVersion: policy.Version,
		Result:        "met",
	}

	// Check endorsement
	if resourceID != "" {
		endorsement, err := s.db.GetActiveEndorsement(resourceID, task.EndeavourID, policy.ID)
		if err != nil {
			return nil, fmt.Errorf("check endorsement: %w", err)
		}
		if endorsement == nil {
			result.Result = "not_met"
			result.Hint = fmt.Sprintf("You have not endorsed the current DoD policy (%s v%d). Use ts.dod.endorse to endorse it.", policy.Name, policy.Version)
			return result, nil
		}
		if endorsement.PolicyVersion != policy.Version {
			result.Result = "not_met"
			result.Hint = fmt.Sprintf("Your endorsement is for v%d but the current policy is v%d. Re-endorse with ts.dod.endorse.", endorsement.PolicyVersion, policy.Version)
			return result, nil
		}
	}

	// Evaluate each condition
	var failCount int
	for _, cond := range policy.Conditions {
		cr := s.evaluateCondition(ctx, cond, task)
		result.Conditions = append(result.Conditions, cr)
		if cr.Status == "failed" && cond.Required {
			failCount++
		}
	}

	// Apply strictness
	switch policy.Strictness {
	case "all":
		if failCount > 0 {
			result.Result = "not_met"
			result.Hint = fmt.Sprintf("Definition of Done not satisfied: %d of %d required conditions failed", failCount, len(policy.Conditions))
		}
	case "n_of":
		passedRequired := 0
		for i, cond := range policy.Conditions {
			if cond.Required && result.Conditions[i].Status == "passed" {
				passedRequired++
			}
		}
		if passedRequired < policy.Quorum {
			result.Result = "not_met"
			result.Hint = fmt.Sprintf("Definition of Done not satisfied: %d of %d required conditions passed (need %d)", passedRequired, len(policy.Conditions), policy.Quorum)
		}
	}

	return result, nil
}

// evaluateCondition dispatches to the appropriate condition evaluator.
func (s *DodService) evaluateCondition(ctx context.Context, cond storage.DodCondition, task *storage.Task) ConditionResult {
	cr := ConditionResult{
		ID:    cond.ID,
		Type:  cond.Type,
		Label: cond.Label,
	}

	switch cond.Type {
	case "peer_review":
		s.evalPeerReview(cond, task, &cr)
	case "comment_required":
		s.evalCommentRequired(cond, task, &cr)
	case "stakeholder_sign_off":
		s.evalStakeholderSignOff(cond, task, &cr)
	case "field_populated":
		s.evalFieldPopulated(cond, task, &cr)
	case "manual_attestation":
		s.evalManualAttestation(cond, task, &cr)
	case "checklist_complete":
		s.evalChecklistComplete(cond, task, &cr)
	case "tests_pass":
		s.evalTestsPass(cond, task, &cr)
	case "time_in_status":
		s.evalTimeInStatus(cond, task, &cr)
	case "custom_check":
		cr.Status = "skipped"
		cr.Detail = "Custom checks require webhook integration (not yet implemented)"
	default:
		cr.Status = "skipped"
		cr.Detail = fmt.Sprintf("Unknown condition type: %s", cond.Type)
	}

	return cr
}

// evalPeerReview checks for approved reviews from non-assignee resources.
func (s *DodService) evalPeerReview(cond storage.DodCondition, task *storage.Task, cr *ConditionResult) {
	minReviewers := getIntParam(cond.Params, "min_reviewers", 1)
	excludeAuthor := getBoolParam(cond.Params, "exclude_author", true)

	approvals, _, _ := s.db.ListApprovals(storage.ListApprovalsOpts{
		EntityType: "task",
		EntityID:   task.ID,
		Verdict:    "approved",
		Limit:      100,
	})

	count := 0
	for _, a := range approvals {
		if excludeAuthor && a.ApproverID == task.AssigneeID {
			continue
		}
		count++
	}

	if count >= minReviewers {
		cr.Status = "passed"
		cr.Detail = fmt.Sprintf("%d of %d required reviews received", count, minReviewers)
	} else {
		cr.Status = "failed"
		cr.Detail = fmt.Sprintf("%d of %d required reviews received", count, minReviewers)
		cr.Hint = "Ask a peer to review and approve this task"
	}
}

// evalCommentRequired checks for minimum comment count.
func (s *DodService) evalCommentRequired(cond storage.DodCondition, task *storage.Task, cr *ConditionResult) {
	minComments := getIntParam(cond.Params, "min_comments", 1)

	count, _ := s.db.CountComments("task", task.ID)

	if count >= minComments {
		cr.Status = "passed"
		cr.Detail = fmt.Sprintf("%d of %d required comments", count, minComments)
	} else {
		cr.Status = "failed"
		cr.Detail = fmt.Sprintf("%d of %d required comments", count, minComments)
		cr.Hint = "Add a comment before closing this task"
	}
}

// evalStakeholderSignOff checks for an approved verdict with a matching role.
func (s *DodService) evalStakeholderSignOff(cond storage.DodCondition, task *storage.Task, cr *ConditionResult) {
	role := getStringParam(cond.Params, "role", "")
	if role == "" {
		cr.Status = "skipped"
		cr.Detail = "No role specified in condition params"
		return
	}

	approvals, _, _ := s.db.ListApprovals(storage.ListApprovalsOpts{
		EntityType: "task",
		EntityID:   task.ID,
		Verdict:    "approved",
		Role:       role,
		Limit:      10,
	})

	if len(approvals) > 0 {
		cr.Status = "passed"
		cr.Detail = fmt.Sprintf("Approved by %s", role)
	} else {
		cr.Status = "failed"
		cr.Detail = fmt.Sprintf("No approval from role: %s", role)
		cr.Hint = fmt.Sprintf("Request sign-off from a %s", role)
	}
}

// evalFieldPopulated checks that a task field or metadata key is non-empty.
func (s *DodService) evalFieldPopulated(cond storage.DodCondition, task *storage.Task, cr *ConditionResult) {
	field := getStringParam(cond.Params, "field", "")
	if field == "" {
		cr.Status = "skipped"
		cr.Detail = "No field specified in condition params"
		return
	}

	var value string
	switch field {
	case "description":
		value = task.Description
	case "title":
		value = task.Title
	case "canceled_reason":
		value = task.CanceledReason
	default:
		// Check metadata
		if task.Metadata != nil {
			if v, ok := task.Metadata[field]; ok {
				value = fmt.Sprintf("%v", v)
			}
		}
	}

	if value != "" {
		cr.Status = "passed"
		cr.Detail = fmt.Sprintf("Field '%s' is populated", field)
	} else {
		cr.Status = "failed"
		cr.Detail = fmt.Sprintf("Field '%s' is empty", field)
		cr.Hint = fmt.Sprintf("Populate the '%s' field before closing", field)
	}
}

// evalManualAttestation checks for explicit attestation in task metadata.
func (s *DodService) evalManualAttestation(cond storage.DodCondition, task *storage.Task, cr *ConditionResult) {
	if task.Metadata != nil {
		if attested, ok := task.Metadata["dod_attested"]; ok {
			if attested == true {
				cr.Status = "passed"
				cr.Detail = "Attestation recorded"
				return
			}
		}
	}

	cr.Status = "failed"
	prompt := getStringParam(cond.Params, "prompt", "Confirm that the work is complete")
	cr.Detail = "No attestation recorded"
	cr.Hint = fmt.Sprintf("Set task metadata dod_attested=true to attest: %s", prompt)
}

// evalChecklistComplete checks that all checklist items in metadata are checked.
func (s *DodService) evalChecklistComplete(cond storage.DodCondition, task *storage.Task, cr *ConditionResult) {
	if task.Metadata == nil {
		cr.Status = "failed"
		cr.Detail = "No checklist found in task metadata"
		cr.Hint = "Add a checklist to task metadata"
		return
	}

	checklistRaw, ok := task.Metadata["checklist"]
	if !ok {
		cr.Status = "failed"
		cr.Detail = "No checklist found in task metadata"
		cr.Hint = "Add a checklist to task metadata"
		return
	}

	checklist, ok := checklistRaw.([]interface{})
	if !ok {
		cr.Status = "failed"
		cr.Detail = "Checklist is not an array"
		return
	}

	if len(checklist) == 0 {
		cr.Status = "passed"
		cr.Detail = "Checklist is empty (vacuously complete)"
		return
	}

	total := len(checklist)
	checked := 0
	for _, item := range checklist {
		if m, ok := item.(map[string]interface{}); ok {
			if c, ok := m["checked"]; ok && c == true {
				checked++
			}
		}
	}

	if checked >= total {
		cr.Status = "passed"
		cr.Detail = fmt.Sprintf("%d of %d checklist items checked", checked, total)
	} else {
		cr.Status = "failed"
		cr.Detail = fmt.Sprintf("%d of %d checklist items checked", checked, total)
		cr.Hint = "Complete remaining checklist items before closing"
	}
}

// evalTestsPass checks for a linked test artifact with passing result.
func (s *DodService) evalTestsPass(cond storage.DodCondition, task *storage.Task, cr *ConditionResult) {
	tag := getStringParam(cond.Params, "artifact_tag", "test-report")

	// Look for artifacts linked to this task via FRM
	rels, _, _ := s.db.ListRelations(storage.ListRelationsOpts{
		SourceEntityType: storage.EntityTask,
		SourceEntityID:   task.ID,
		TargetEntityType: storage.EntityArtifact,
	})

	for _, rel := range rels {
		art, err := s.db.GetArtifact(rel.TargetEntityID)
		if err != nil {
			continue
		}
		// Check if artifact has the required tag
		hasTag := false
		for _, t := range art.Tags {
			if t == tag {
				hasTag = true
				break
			}
		}
		if !hasTag {
			continue
		}
		// Check metadata for result
		if art.Metadata != nil {
			if result, ok := art.Metadata["result"]; ok && fmt.Sprintf("%v", result) == "passed" {
				cr.Status = "passed"
				cr.Detail = fmt.Sprintf("Test artifact %s passed", art.ID)
				return
			}
		}
	}

	cr.Status = "failed"
	cr.Detail = fmt.Sprintf("No passing test artifact with tag '%s' linked to task", tag)
	cr.Hint = "Link a test artifact with tag '" + tag + "' and metadata result='passed'"
}

// evalTimeInStatus checks that the task has been active for a minimum duration.
func (s *DodService) evalTimeInStatus(cond storage.DodCondition, task *storage.Task, cr *ConditionResult) {
	minDurationStr := getStringParam(cond.Params, "min_duration", "1h")
	minDuration, err := time.ParseDuration(minDurationStr)
	if err != nil {
		cr.Status = "skipped"
		cr.Detail = fmt.Sprintf("Invalid min_duration: %s", minDurationStr)
		return
	}

	if task.StartedAt == nil {
		cr.Status = "failed"
		cr.Detail = "Task has not been started"
		cr.Hint = "Task must be active for at least " + minDurationStr
		return
	}

	elapsed := storage.UTCNow().Sub(*task.StartedAt)
	if elapsed >= minDuration {
		cr.Status = "passed"
		cr.Detail = fmt.Sprintf("Task active for %s (minimum: %s)", elapsed.Round(time.Minute), minDuration)
	} else {
		cr.Status = "failed"
		remaining := minDuration - elapsed
		cr.Detail = fmt.Sprintf("Task active for %s (minimum: %s)", elapsed.Round(time.Minute), minDuration)
		cr.Hint = fmt.Sprintf("Wait %s before closing", remaining.Round(time.Minute))
	}
}

// Override marks a task as having a DoD override, bypassing enforcement.
func (s *DodService) Override(ctx context.Context, taskID, resourceID, reason string) error {
	if taskID == "" {
		return fmt.Errorf("task_id is required")
	}
	if reason == "" {
		return fmt.Errorf("reason is required for DoD override")
	}

	task, err := s.db.GetTask(taskID)
	if err != nil {
		return err
	}

	// Set override metadata on the task
	metadata := task.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["dod_override"] = true
	metadata["dod_override_by"] = resourceID
	metadata["dod_override_reason"] = reason
	metadata["dod_override_at"] = storage.UTCNow().Format(time.RFC3339)

	_, err = s.db.UpdateTask(taskID, storage.UpdateTaskFields{Metadata: metadata})
	if err != nil {
		return fmt.Errorf("set override metadata: %w", err)
	}

	s.logger.Info("DoD override applied", "task_id", taskID, "resource_id", resourceID, "reason", reason)
	return nil
}

// HasOverride checks if a task has an active DoD override.
func (s *DodService) HasOverride(task *storage.Task) bool {
	if task.Metadata == nil {
		return false
	}
	override, ok := task.Metadata["dod_override"]
	return ok && override == true
}

// Helper functions for extracting typed params from condition parameters.

func getIntParam(params map[string]interface{}, key string, defaultVal int) int {
	if params == nil {
		return defaultVal
	}
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return defaultVal
	}
}

func getStringParam(params map[string]interface{}, key, defaultVal string) string {
	if params == nil {
		return defaultVal
	}
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	if s, ok := v.(string); ok {
		return s
	}
	return defaultVal
}

func getBoolParam(params map[string]interface{}, key string, defaultVal bool) bool {
	if params == nil {
		return defaultVal
	}
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return defaultVal
}
