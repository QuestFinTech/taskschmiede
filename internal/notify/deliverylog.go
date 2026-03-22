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


package notify

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DeliveryLog tracks event receipts and delivery attempts in a local
// SQLite database. This provides an audit trail and retry capability
// without coupling to the main application database.
type DeliveryLog struct {
	db *sql.DB
}

// NewDeliveryLog opens (or creates) the delivery log database at the
// given path and runs schema migrations.
func NewDeliveryLog(path string) (*DeliveryLog, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open delivery log: %w", err)
	}

	if err := migrateDeliveryLog(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate delivery log: %w", err)
	}

	return &DeliveryLog{db: db}, nil
}

// Close closes the underlying database connection.
func (dl *DeliveryLog) Close() error {
	return dl.db.Close()
}

// migrateDeliveryLog creates the events and deliveries tables if they do not exist.
func migrateDeliveryLog(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS events (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			event_type  TEXT    NOT NULL,
			severity    TEXT    NOT NULL,
			summary     TEXT    NOT NULL,
			entity_type TEXT    NOT NULL DEFAULT '',
			entity_id   TEXT    NOT NULL DEFAULT '',
			agent_id    TEXT    NOT NULL DEFAULT '',
			owner_id    TEXT    NOT NULL DEFAULT '',
			received_at TEXT    NOT NULL
		);

		CREATE TABLE IF NOT EXISTS deliveries (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			event_id   INTEGER NOT NULL REFERENCES events(id),
			channel    TEXT    NOT NULL,
			recipient  TEXT    NOT NULL DEFAULT '',
			status     TEXT    NOT NULL DEFAULT 'pending',
			error_msg  TEXT    NOT NULL DEFAULT '',
			attempts   INTEGER NOT NULL DEFAULT 0,
			created_at TEXT    NOT NULL,
			updated_at TEXT    NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_deliveries_event  ON deliveries(event_id);
		CREATE INDEX IF NOT EXISTS idx_deliveries_status ON deliveries(status);
	`
	_, err := db.Exec(schema)
	return err
}

// RecordEvent inserts an incoming event and returns its local ID.
func (dl *DeliveryLog) RecordEvent(event *ServiceEvent) (int64, error) {
	result, err := dl.db.Exec(`
		INSERT INTO events (event_type, severity, summary, entity_type, entity_id, agent_id, owner_id, received_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		event.Type, event.Severity, event.Summary,
		event.EntityType, event.EntityID,
		event.AgentID, event.OwnerID,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("record event: %w", err)
	}
	return result.LastInsertId()
}

// RecordDelivery inserts a delivery attempt for an event.
func (dl *DeliveryLog) RecordDelivery(eventID int64, channel, recipient, status, errMsg string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := dl.db.Exec(`
		INSERT INTO deliveries (event_id, channel, recipient, status, error_msg, attempts, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
		eventID, channel, recipient, status, errMsg, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("record delivery: %w", err)
	}
	return result.LastInsertId()
}

// UpdateDelivery updates the status and error message of a delivery.
func (dl *DeliveryLog) UpdateDelivery(deliveryID int64, status, errMsg string) error {
	_, err := dl.db.Exec(`
		UPDATE deliveries SET status = ?, error_msg = ?, attempts = attempts + 1, updated_at = ?
		WHERE id = ?`,
		status, errMsg, time.Now().UTC().Format(time.RFC3339), deliveryID,
	)
	return err
}

// PendingDeliveries returns deliveries that need retry (status = 'failed', attempts < maxRetries).
func (dl *DeliveryLog) PendingDeliveries(maxRetries int, limit int) ([]PendingDelivery, error) {
	rows, err := dl.db.Query(`
		SELECT d.id, d.event_id, d.channel, d.recipient, d.attempts,
			   e.event_type, e.severity, e.summary, e.entity_type, e.entity_id, e.agent_id, e.owner_id
		FROM deliveries d
		JOIN events e ON e.id = d.event_id
		WHERE d.status = 'failed' AND d.attempts < ?
		ORDER BY d.created_at ASC
		LIMIT ?`, maxRetries, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query pending deliveries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []PendingDelivery
	for rows.Next() {
		var pd PendingDelivery
		if err := rows.Scan(
			&pd.DeliveryID, &pd.EventID, &pd.Channel, &pd.Recipient, &pd.Attempts,
			&pd.EventType, &pd.Severity, &pd.Summary, &pd.EntityType, &pd.EntityID,
			&pd.AgentID, &pd.OwnerID,
		); err != nil {
			return nil, fmt.Errorf("scan pending delivery: %w", err)
		}
		results = append(results, pd)
	}
	return results, rows.Err()
}

// Stats returns summary counts of the delivery log.
func (dl *DeliveryLog) Stats() (DeliveryStats, error) {
	var s DeliveryStats
	err := dl.db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&s.TotalEvents)
	if err != nil {
		return s, err
	}
	_ = dl.db.QueryRow(`SELECT COUNT(*) FROM deliveries WHERE status = 'delivered'`).Scan(&s.Delivered)
	_ = dl.db.QueryRow(`SELECT COUNT(*) FROM deliveries WHERE status = 'failed'`).Scan(&s.Failed)
	_ = dl.db.QueryRow(`SELECT COUNT(*) FROM deliveries WHERE status = 'rate_limited'`).Scan(&s.RateLimited)
	return s, nil
}

// PendingDelivery contains the data needed to retry a failed delivery.
type PendingDelivery struct {
	DeliveryID int64
	EventID    int64
	Channel    string
	Recipient  string
	Attempts   int
	EventType  string
	Severity   string
	Summary    string
	EntityType string
	EntityID   string
	AgentID    string
	OwnerID    string
}

// DeliveryStats holds aggregate delivery log counts.
type DeliveryStats struct {
	TotalEvents int
	Delivered   int
	Failed      int
	RateLimited int
}
