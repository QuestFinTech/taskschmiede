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
	"time"
)

// PurgeAuditLog deletes audit_log entries with created_at before the given cutoff.
// Returns the number of rows deleted.
func (db *DB) PurgeAuditLog(before time.Time) (int64, error) {
	result, err := db.Exec(
		`DELETE FROM audit_log WHERE created_at < ?`,
		before.Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("purge audit_log: %w", err)
	}
	return result.RowsAffected()
}

// PurgeEntityChanges deletes entity_change entries with created_at before the given cutoff.
// Returns the number of rows deleted.
func (db *DB) PurgeEntityChanges(before time.Time) (int64, error) {
	result, err := db.Exec(
		`DELETE FROM entity_change WHERE created_at < ?`,
		before.Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("purge entity_change: %w", err)
	}
	return result.RowsAffected()
}
