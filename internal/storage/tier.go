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
	"fmt"
	"strconv"
	"sync"
	"time"
)

// TierDefinition holds the limits for a subscription tier.
type TierDefinition struct {
	ID                   int       `json:"id"`
	Name                 string    `json:"name"`
	MaxUsers             int       `json:"max_users"`
	MaxOrgs              int       `json:"max_orgs"`
	MaxAgentsPerOrg      int       `json:"max_agents_per_org"`
	MaxEndeavoursPerOrg  int       `json:"max_endeavours_per_org"`
	MaxActiveEndeavours  int       `json:"max_active_endeavours"`
	MaxTeamsPerOrg       int       `json:"max_teams_per_org"`
	MaxCreationsPerHour  int       `json:"max_creations_per_hour"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// tierCache caches tier definitions in memory. Tiers change rarely.
var tierCache sync.Map

// GetTierDefinition returns the tier definition for the given tier ID.
// Results are cached in memory and invalidated on update.
func (db *DB) GetTierDefinition(tierID int) (*TierDefinition, error) {
	if cached, ok := tierCache.Load(tierID); ok {
		return cached.(*TierDefinition), nil
	}

	td := &TierDefinition{}
	var createdAt, updatedAt string
	err := db.QueryRow(
		`SELECT id, name, max_users, max_orgs, max_agents_per_org, max_endeavours_per_org,
		        max_active_endeavours, max_teams_per_org, max_creations_per_hour,
		        created_at, updated_at
		 FROM tier_definition WHERE id = ?`, tierID,
	).Scan(
		&td.ID, &td.Name, &td.MaxUsers, &td.MaxOrgs, &td.MaxAgentsPerOrg,
		&td.MaxEndeavoursPerOrg, &td.MaxActiveEndeavours,
		&td.MaxTeamsPerOrg, &td.MaxCreationsPerHour,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("tier %d not found: %w", tierID, err)
	}
	td.CreatedAt = ParseDBTime(createdAt)
	td.UpdatedAt = ParseDBTime(updatedAt)

	tierCache.Store(tierID, td)
	return td, nil
}

// ListTierDefinitions returns all tier definitions.
func (db *DB) ListTierDefinitions() ([]*TierDefinition, error) {
	rows, err := db.Query(
		`SELECT id, name, max_users, max_orgs, max_agents_per_org, max_endeavours_per_org,
		        max_active_endeavours, max_teams_per_org, max_creations_per_hour,
		        created_at, updated_at
		 FROM tier_definition ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tiers []*TierDefinition
	for rows.Next() {
		td := &TierDefinition{}
		var createdAt, updatedAt string
		if err := rows.Scan(
			&td.ID, &td.Name, &td.MaxUsers, &td.MaxOrgs, &td.MaxAgentsPerOrg,
			&td.MaxEndeavoursPerOrg, &td.MaxActiveEndeavours,
			&td.MaxTeamsPerOrg, &td.MaxCreationsPerHour,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		td.CreatedAt = ParseDBTime(createdAt)
		td.UpdatedAt = ParseDBTime(updatedAt)
		tiers = append(tiers, td)
	}
	return tiers, rows.Err()
}

// UpdateTierDefinition updates specific fields of a tier definition.
// Invalidates the cache for the updated tier.
func (db *DB) UpdateTierDefinition(tierID int, fields map[string]interface{}) error {
	allowed := map[string]bool{
		"name":                    true,
		"max_users":               true,
		"max_orgs":                true,
		"max_agents_per_org":      true,
		"max_endeavours_per_org":  true,
		"max_active_endeavours":   true,
		"max_teams_per_org":       true,
		"max_creations_per_hour":  true,
	}

	setClauses := ""
	args := make([]interface{}, 0, len(fields)+2)
	for key, val := range fields {
		if !allowed[key] {
			continue
		}
		if setClauses != "" {
			setClauses += ", "
		}
		setClauses += key + " = ?"
		args = append(args, val)
	}
	if setClauses == "" {
		return nil
	}

	setClauses += ", updated_at = ?"
	args = append(args, UTCNow().Format(time.RFC3339))
	args = append(args, tierID)

	_, err := db.Exec(
		fmt.Sprintf("UPDATE tier_definition SET %s WHERE id = ?", setClauses),
		args...,
	)
	if err != nil {
		return err
	}

	// Invalidate cache.
	tierCache.Delete(tierID)
	return nil
}

// InvalidateTierCache clears the entire tier definition cache.
func InvalidateTierCache() {
	tierCache.Range(func(key, _ interface{}) bool {
		tierCache.Delete(key)
		return true
	})
}

// TierUsageSummary holds per-tier entity counts for the admin dashboard.
type TierUsageSummary struct {
	TierID     int    `json:"tier_id"`
	TierName   string `json:"tier_name"`
	Users      int    `json:"users"`
	Orgs       int    `json:"orgs"`
	Endeavours int    `json:"endeavours"`
	Teams      int    `json:"teams"`
	Agents     int    `json:"agents"`
}

// GetTierUsageSummary returns entity counts grouped by tier for admin display.
// Counts: users per tier, orgs owned by those users, endeavours/agents/teams in those orgs.
func (db *DB) GetTierUsageSummary() ([]*TierUsageSummary, error) {
	tiers, err := db.ListTierDefinitions()
	if err != nil {
		return nil, err
	}

	// Build result map keyed by tier ID.
	byTier := make(map[int]*TierUsageSummary, len(tiers))
	result := make([]*TierUsageSummary, len(tiers))
	for i, td := range tiers {
		s := &TierUsageSummary{TierID: td.ID, TierName: td.Name}
		byTier[td.ID] = s
		result[i] = s
	}

	// Users per tier (exclude master admin -- system account, not a tier user).
	rows, err := db.Query(
		`SELECT tier, COUNT(*) FROM user
		 WHERE status NOT IN ('archived') AND is_admin = 0
		 GROUP BY tier`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var tier, count int
		if err := rows.Scan(&tier, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if s, ok := byTier[tier]; ok {
			s.Users = count
		}
	}
	_ = rows.Close()

	// Orgs per tier (org owner = user whose resource is owner-member of org).
	rows, err = db.Query(
		`SELECT u.tier, COUNT(DISTINCT er.source_entity_id) FROM entity_relation er
		 JOIN user u ON u.resource_id = er.target_entity_id
		 JOIN organization o ON o.id = er.source_entity_id
		 WHERE er.relationship_type = 'has_member'
		   AND er.source_entity_type = 'organization'
		   AND er.target_entity_type = 'resource'
		   AND json_extract(er.metadata, '$.role') = 'owner'
		   AND o.status != 'archived'
		 GROUP BY u.tier`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var tier, count int
		if err := rows.Scan(&tier, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if s, ok := byTier[tier]; ok {
			s.Orgs = count
		}
	}
	_ = rows.Close()

	// Endeavours per tier (org -> participates_in -> endeavour, grouped by org owner tier).
	rows, err = db.Query(
		`SELECT u.tier, COUNT(DISTINCT edv_rel.target_entity_id) FROM entity_relation edv_rel
		 JOIN endeavour e ON e.id = edv_rel.target_entity_id AND e.status NOT IN ('archived', 'completed')
		 JOIN entity_relation owner_rel
		   ON owner_rel.source_entity_type = 'organization'
		   AND owner_rel.source_entity_id = edv_rel.source_entity_id
		   AND owner_rel.relationship_type = 'has_member'
		   AND owner_rel.target_entity_type = 'resource'
		   AND json_extract(owner_rel.metadata, '$.role') = 'owner'
		 JOIN user u ON u.resource_id = owner_rel.target_entity_id
		 WHERE edv_rel.relationship_type = 'participates_in'
		   AND edv_rel.source_entity_type = 'organization'
		   AND edv_rel.target_entity_type = 'endeavour'
		 GROUP BY u.tier`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var tier, count int
		if err := rows.Scan(&tier, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if s, ok := byTier[tier]; ok {
			s.Endeavours = count
		}
	}
	_ = rows.Close()

	// Agents per tier (org -> has_member -> resource -> user with user_type='agent').
	rows, err = db.Query(
		`SELECT owner_u.tier, COUNT(DISTINCT agent_u.id) FROM entity_relation agent_rel
		 JOIN user agent_u ON agent_u.resource_id = agent_rel.target_entity_id AND agent_u.user_type = 'agent'
		 JOIN entity_relation owner_rel
		   ON owner_rel.source_entity_type = 'organization'
		   AND owner_rel.source_entity_id = agent_rel.source_entity_id
		   AND owner_rel.relationship_type = 'has_member'
		   AND owner_rel.target_entity_type = 'resource'
		   AND json_extract(owner_rel.metadata, '$.role') = 'owner'
		 JOIN user owner_u ON owner_u.resource_id = owner_rel.target_entity_id
		 WHERE agent_rel.relationship_type = 'has_member'
		   AND agent_rel.source_entity_type = 'organization'
		   AND agent_rel.target_entity_type = 'resource'
		 GROUP BY owner_u.tier`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var tier, count int
		if err := rows.Scan(&tier, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if s, ok := byTier[tier]; ok {
			s.Agents = count
		}
	}
	_ = rows.Close()

	// Teams per tier (org -> has_member -> resource with type='team').
	rows, err = db.Query(
		`SELECT owner_u.tier, COUNT(DISTINCT team_rel.target_entity_id) FROM entity_relation team_rel
		 JOIN resource r ON r.id = team_rel.target_entity_id AND r.type = 'team' AND r.status = 'active'
		 JOIN entity_relation owner_rel
		   ON owner_rel.source_entity_type = 'organization'
		   AND owner_rel.source_entity_id = team_rel.source_entity_id
		   AND owner_rel.relationship_type = 'has_member'
		   AND owner_rel.target_entity_type = 'resource'
		   AND json_extract(owner_rel.metadata, '$.role') = 'owner'
		 JOIN user owner_u ON owner_u.resource_id = owner_rel.target_entity_id
		 WHERE team_rel.relationship_type = 'has_member'
		   AND team_rel.source_entity_type = 'organization'
		   AND team_rel.target_entity_type = 'resource'
		 GROUP BY owner_u.tier`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var tier, count int
		if err := rows.Scan(&tier, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if s, ok := byTier[tier]; ok {
			s.Teams = count
		}
	}
	_ = rows.Close()

	return result, nil
}

// TierSeedDef holds a tier definition for first-run seeding from config.
type TierSeedDef struct {
	ID                  int
	Name                string
	MaxUsers            int
	MaxOrgs             int
	MaxAgentsPerOrg     int
	MaxEndeavoursPerOrg int
	MaxActiveEndeavours int
	MaxTeamsPerOrg      int
	MaxCreationsPerHour int
}

// SeedTierDefinitions inserts tier definitions when the table is empty.
// Called once at startup with config-driven definitions. No-op if tiers
// already exist (subsequent runs use the DB values).
func (db *DB) SeedTierDefinitions(defs []TierSeedDef) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM tier_definition").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil // already seeded
	}

	now := UTCNow().Format("2006-01-02T15:04:05Z")
	for _, d := range defs {
		if _, err := db.Exec(
			`INSERT INTO tier_definition (id, name, max_users, max_orgs, max_agents_per_org, max_endeavours_per_org, max_active_endeavours, max_teams_per_org, max_creations_per_hour, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			d.ID, d.Name, d.MaxUsers, d.MaxOrgs, d.MaxAgentsPerOrg, d.MaxEndeavoursPerOrg,
			d.MaxActiveEndeavours, d.MaxTeamsPerOrg, d.MaxCreationsPerHour, now, now,
		); err != nil {
			return fmt.Errorf("seed tier %s: %w", d.Name, err)
		}
	}
	return nil
}

// DefaultTierID returns the configured default tier ID from the policy table.
// Falls back to 1 if not set.
func (db *DB) DefaultTierID() int {
	v, err := db.GetPolicy("tiers.default_tier")
	if err != nil || v == "" {
		return 1
	}
	id, err := strconv.Atoi(v)
	if err != nil || id < 1 {
		return 1
	}
	return id
}

// TierName returns the human-readable name for a tier ID.
// Reads from the tier cache / database. Falls back to "tier-N" if not found.
func (db *DB) TierName(tierID int) string {
	td, err := db.GetTierDefinition(tierID)
	if err != nil {
		return fmt.Sprintf("tier-%d", tierID)
	}
	return td.Name
}

// LimitExceeded checks if the current count has reached or exceeded the limit.
// Returns false (not exceeded) for unlimited limits (-1).
func LimitExceeded(current, limit int) bool {
	if limit < 0 {
		return false // unlimited
	}
	return current >= limit
}
