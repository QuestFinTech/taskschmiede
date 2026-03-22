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
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// MessageDB wraps a separate SQLite database for messages.
// Messages live in their own DB to keep the main DB lean and allow
// independent archival and retention management.
type MessageDB struct {
	db *sql.DB
}

// Message represents a push message sent between resources.
type Message struct {
	ID         string
	SenderID   string
	SenderName string // denormalized from resource table (resolved at query time via main DB)
	Subject    string
	Content    string
	Intent     string // info, question, action, alert
	ReplyToID  string
	EntityType string // optional context link
	EntityID   string // optional context link
	ScopeType  string // NULL (direct), endeavour, organization
	ScopeID    string
	Metadata   map[string]interface{}
	CreatedAt  time.Time
}

// MessageDelivery tracks per-recipient delivery status.
type MessageDelivery struct {
	ID          string
	MessageID   string
	RecipientID string
	Channel     string // internal, email
	Status      string // pending, copied, delivered, read, failed
	CopyEmail   string // email address for external copy (set at send time)
	DeliveredAt *time.Time
	ReadAt      *time.Time
	CreatedAt   time.Time
}

// InboxItem joins message + delivery for the inbox view.
type InboxItem struct {
	Message
	DeliveryID string
	Channel    string
	Status     string
	ReadAt     *time.Time
}

// ListInboxOpts holds filters for the inbox query.
type ListInboxOpts struct {
	Status     string // pending, delivered, read, or empty for all
	Intent     string
	EntityType string
	EntityID   string
	Unread     bool // shorthand for status != "read"
	Limit      int
	Offset     int
}

// DeliveryTarget describes a single recipient and their delivery channel.
type DeliveryTarget struct {
	RecipientID string
	Channel     string
}

// PendingDelivery is returned by ListPendingEmailDeliveries for outbound processing.
type PendingDelivery struct {
	DeliveryID  string
	MessageID   string
	RecipientID string
	SenderID    string
	Subject     string
	Content     string
	Intent      string
	EntityType  string
	EntityID    string
	Metadata    map[string]interface{}
	CreatedAt   time.Time
}

// Message and delivery error sentinels.
var (
	// ErrMessageNotFound is returned when a message cannot be found by its ID.
	ErrMessageNotFound = errors.New("message not found")
	// ErrDeliveryNotFound is returned when a delivery record cannot be found.
	ErrDeliveryNotFound = errors.New("delivery not found")
)

// OpenMessageDB opens or creates the message SQLite database.
func OpenMessageDB(path string) (*MessageDB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open message database: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping message database: %w", err)
	}

	mdb := &MessageDB{db: db}
	if err := mdb.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate message database: %w", err)
	}

	return mdb, nil
}

// Close closes the message database connection.
func (mdb *MessageDB) Close() error {
	return mdb.db.Close()
}

// BackupTo creates a consistent backup of the message database using VACUUM INTO.
// The destination path must not already exist. Returns the backup file size in bytes.
func (mdb *MessageDB) BackupTo(destPath string) (int64, error) {
	_, err := mdb.db.Exec(`VACUUM INTO ?`, destPath)
	if err != nil {
		return 0, fmt.Errorf("vacuum into %s: %w", destPath, err)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return 0, fmt.Errorf("stat backup %s: %w", destPath, err)
	}

	return info.Size(), nil
}

// migrate creates the message and message_delivery tables.
func (mdb *MessageDB) migrate() error {
	_, err := mdb.db.Exec(`
		CREATE TABLE IF NOT EXISTS message (
			id          TEXT PRIMARY KEY,
			sender_id   TEXT NOT NULL,
			subject     TEXT NOT NULL DEFAULT '',
			content     TEXT NOT NULL,
			intent      TEXT NOT NULL DEFAULT 'info',
			reply_to_id TEXT,
			entity_type TEXT,
			entity_id   TEXT,
			scope_type  TEXT,
			scope_id    TEXT,
			metadata    TEXT NOT NULL DEFAULT '{}',
			created_at  TEXT NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_message_sender ON message(sender_id);
		CREATE INDEX IF NOT EXISTS idx_message_entity ON message(entity_type, entity_id);
		CREATE INDEX IF NOT EXISTS idx_message_created ON message(created_at);
		CREATE INDEX IF NOT EXISTS idx_message_reply ON message(reply_to_id);

		CREATE TABLE IF NOT EXISTS message_delivery (
			id           TEXT PRIMARY KEY,
			message_id   TEXT NOT NULL REFERENCES message(id),
			recipient_id TEXT NOT NULL,
			channel      TEXT NOT NULL DEFAULT 'internal',
			status       TEXT NOT NULL DEFAULT 'pending',
			delivered_at TEXT,
			read_at      TEXT,
			created_at   TEXT NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_delivery_recipient ON message_delivery(recipient_id, status);
		CREATE INDEX IF NOT EXISTS idx_delivery_message ON message_delivery(message_id);
		CREATE INDEX IF NOT EXISTS idx_delivery_pending_email ON message_delivery(channel, status)
			WHERE channel = 'email' AND status = 'pending';
	`)
	if err != nil {
		return fmt.Errorf("create message tables: %w", err)
	}

	// Add email routing columns for intercom header-based matching.
	// email_message_id: RFC 2822 Message-ID set on outbound emails.
	// ref_code: short hex code for subject-line [TS-xxxx] fallback.
	// copy_email: email address for external copy of internal messages.
	migrations := []string{
		`ALTER TABLE message_delivery ADD COLUMN email_message_id TEXT`,
		`ALTER TABLE message_delivery ADD COLUMN ref_code TEXT`,
		`ALTER TABLE message_delivery ADD COLUMN copy_email TEXT`,
	}
	for _, m := range migrations {
		_, _ = mdb.db.Exec(m) // ignore "duplicate column" errors on re-run
	}

	// Unique partial indexes for lookup (WHERE NOT NULL avoids indexing rows without routing).
	indexes := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_delivery_email_message_id ON message_delivery(email_message_id) WHERE email_message_id IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_delivery_ref_code ON message_delivery(ref_code) WHERE ref_code IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_delivery_pending_copy ON message_delivery(status) WHERE copy_email IS NOT NULL AND status = 'pending'`,
	}
	for _, idx := range indexes {
		if _, err := mdb.db.Exec(idx); err != nil {
			return fmt.Errorf("create email routing index: %w", err)
		}
	}

	return nil
}

// CreateMessage inserts a new message.
func (mdb *MessageDB) CreateMessage(senderID, subject, content, intent, replyToID, entityType, entityID, scopeType, scopeID string, metadata map[string]interface{}) (*Message, error) {
	id := generateID("msg")

	metadataJSON := "{}"
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	now := UTCNow()
	nowStr := now.Format(time.RFC3339)

	var replyVal, entityTypeVal, entityIDVal, scopeTypeVal, scopeIDVal *string
	if replyToID != "" {
		replyVal = &replyToID
	}
	if entityType != "" {
		entityTypeVal = &entityType
	}
	if entityID != "" {
		entityIDVal = &entityID
	}
	if scopeType != "" {
		scopeTypeVal = &scopeType
	}
	if scopeID != "" {
		scopeIDVal = &scopeID
	}

	_, err := mdb.db.Exec(
		`INSERT INTO message (id, sender_id, subject, content, intent, reply_to_id,
		 entity_type, entity_id, scope_type, scope_id, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, senderID, subject, content, intent, replyVal,
		entityTypeVal, entityIDVal, scopeTypeVal, scopeIDVal, metadataJSON, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}

	return &Message{
		ID:         id,
		SenderID:   senderID,
		Subject:    subject,
		Content:    content,
		Intent:     intent,
		ReplyToID:  replyToID,
		EntityType: entityType,
		EntityID:   entityID,
		ScopeType:  scopeType,
		ScopeID:    scopeID,
		Metadata:   metadata,
		CreatedAt:  now,
	}, nil
}

// GetMessage retrieves a message by ID.
func (mdb *MessageDB) GetMessage(id string) (*Message, error) {
	var m Message
	var replyToID, entityType, entityID, scopeType, scopeID sql.NullString
	var metadataJSON, createdAt string

	err := mdb.db.QueryRow(
		`SELECT id, sender_id, subject, content, intent, reply_to_id,
		        entity_type, entity_id, scope_type, scope_id, metadata, created_at
		 FROM message WHERE id = ?`, id,
	).Scan(&m.ID, &m.SenderID, &m.Subject, &m.Content, &m.Intent, &replyToID,
		&entityType, &entityID, &scopeType, &scopeID, &metadataJSON, &createdAt)

	if err == sql.ErrNoRows {
		return nil, ErrMessageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query message: %w", err)
	}

	if replyToID.Valid {
		m.ReplyToID = replyToID.String
	}
	if entityType.Valid {
		m.EntityType = entityType.String
	}
	if entityID.Valid {
		m.EntityID = entityID.String
	}
	if scopeType.Valid {
		m.ScopeType = scopeType.String
	}
	if scopeID.Valid {
		m.ScopeID = scopeID.String
	}
	_ = json.Unmarshal([]byte(metadataJSON), &m.Metadata)
	m.CreatedAt = ParseDBTime(createdAt)

	return &m, nil
}

// ListInbox returns messages delivered to a recipient, joining message + delivery.
func (mdb *MessageDB) ListInbox(recipientID string, opts ListInboxOpts) ([]*InboxItem, int, error) {
	query := `SELECT m.id, m.sender_id, m.subject, m.content, m.intent, m.reply_to_id,
	                 m.entity_type, m.entity_id, m.scope_type, m.scope_id, m.metadata, m.created_at,
	                 d.id, d.channel, d.status, d.read_at
	          FROM message_delivery d
	          JOIN message m ON d.message_id = m.id`
	countQuery := `SELECT COUNT(*) FROM message_delivery d JOIN message m ON d.message_id = m.id`

	conditions := []string{"d.recipient_id = ?"}
	params := []interface{}{recipientID}

	if opts.Status != "" {
		conditions = append(conditions, "d.status = ?")
		params = append(params, opts.Status)
	}
	if opts.Unread {
		conditions = append(conditions, "d.status != 'read'")
	}
	if opts.Intent != "" {
		conditions = append(conditions, "m.intent = ?")
		params = append(params, opts.Intent)
	}
	if opts.EntityType != "" {
		conditions = append(conditions, "m.entity_type = ?")
		params = append(params, opts.EntityType)
	}
	if opts.EntityID != "" {
		conditions = append(conditions, "m.entity_id = ?")
		params = append(params, opts.EntityID)
	}

	where := " WHERE " + strings.Join(conditions, " AND ")
	query += where
	countQuery += where

	var total int
	_ = mdb.db.QueryRow(countQuery, params...).Scan(&total)

	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	query += ` ORDER BY m.created_at DESC LIMIT ? OFFSET ?`
	params = append(params, opts.Limit, opts.Offset)

	rows, err := mdb.db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query inbox: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []*InboxItem
	for rows.Next() {
		var item InboxItem
		var replyToID, entityType, entityID, scopeType, scopeID, readAt sql.NullString
		var metadataJSON, createdAt string

		if err := rows.Scan(&item.ID, &item.SenderID, &item.Subject, &item.Content, &item.Intent, &replyToID,
			&entityType, &entityID, &scopeType, &scopeID, &metadataJSON, &createdAt,
			&item.DeliveryID, &item.Channel, &item.Status, &readAt); err != nil {
			return nil, 0, fmt.Errorf("scan inbox item: %w", err)
		}

		if replyToID.Valid {
			item.ReplyToID = replyToID.String
		}
		if entityType.Valid {
			item.EntityType = entityType.String
		}
		if entityID.Valid {
			item.EntityID = entityID.String
		}
		if scopeType.Valid {
			item.ScopeType = scopeType.String
		}
		if scopeID.Valid {
			item.ScopeID = scopeID.String
		}
		_ = json.Unmarshal([]byte(metadataJSON), &item.Metadata)
		item.CreatedAt = ParseDBTime(createdAt)
		if readAt.Valid {
			t := ParseDBTime(readAt.String)
			item.ReadAt = &t
		}

		items = append(items, &item)
	}

	return items, total, nil
}

// GetThread retrieves all messages in a conversation thread.
// It walks up the reply chain to find the root, then returns all messages
// that share the same root, ordered chronologically.
func (mdb *MessageDB) GetThread(messageID string) ([]*Message, error) {
	// Walk up to find root
	rootID := messageID
	for {
		var replyToID sql.NullString
		err := mdb.db.QueryRow(`SELECT reply_to_id FROM message WHERE id = ?`, rootID).Scan(&replyToID)
		if err == sql.ErrNoRows {
			return nil, ErrMessageNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("walk thread: %w", err)
		}
		if !replyToID.Valid || replyToID.String == "" {
			break
		}
		rootID = replyToID.String
	}

	// Collect all messages in thread using recursive CTE
	rows, err := mdb.db.Query(
		`WITH RECURSIVE thread AS (
			SELECT id FROM message WHERE id = ?
			UNION ALL
			SELECT m.id FROM message m JOIN thread t ON m.reply_to_id = t.id
		)
		SELECT m.id, m.sender_id, m.subject, m.content, m.intent, m.reply_to_id,
		       m.entity_type, m.entity_id, m.scope_type, m.scope_id, m.metadata, m.created_at
		FROM message m
		JOIN thread t ON m.id = t.id
		ORDER BY m.created_at ASC`, rootID,
	)
	if err != nil {
		return nil, fmt.Errorf("query thread: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var messages []*Message
	for rows.Next() {
		var m Message
		var replyToID, entityType, entityID, scopeType, scopeID sql.NullString
		var metadataJSON, createdAt string

		if err := rows.Scan(&m.ID, &m.SenderID, &m.Subject, &m.Content, &m.Intent, &replyToID,
			&entityType, &entityID, &scopeType, &scopeID, &metadataJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("scan thread message: %w", err)
		}

		if replyToID.Valid {
			m.ReplyToID = replyToID.String
		}
		if entityType.Valid {
			m.EntityType = entityType.String
		}
		if entityID.Valid {
			m.EntityID = entityID.String
		}
		if scopeType.Valid {
			m.ScopeType = scopeType.String
		}
		if scopeID.Valid {
			m.ScopeID = scopeID.String
		}
		_ = json.Unmarshal([]byte(metadataJSON), &m.Metadata)
		m.CreatedAt = ParseDBTime(createdAt)

		messages = append(messages, &m)
	}

	return messages, nil
}

// CreateDelivery inserts a single delivery record.
func (mdb *MessageDB) CreateDelivery(messageID, recipientID, channel string) (*MessageDelivery, error) {
	id := generateID("mdl")
	now := UTCNow()
	nowStr := now.Format(time.RFC3339)

	_, err := mdb.db.Exec(
		`INSERT INTO message_delivery (id, message_id, recipient_id, channel, status, created_at)
		 VALUES (?, ?, ?, ?, 'pending', ?)`,
		id, messageID, recipientID, channel, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert delivery: %w", err)
	}

	return &MessageDelivery{
		ID:          id,
		MessageID:   messageID,
		RecipientID: recipientID,
		Channel:     channel,
		Status:      "pending",
		CreatedAt:   now,
	}, nil
}

// CreateDeliveryBatch inserts multiple delivery records for a message.
func (mdb *MessageDB) CreateDeliveryBatch(messageID string, targets []DeliveryTarget) error {
	if len(targets) == 0 {
		return nil
	}

	tx, err := mdb.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(
		`INSERT INTO message_delivery (id, message_id, recipient_id, channel, status, created_at)
		 VALUES (?, ?, ?, ?, 'pending', ?)`)
	if err != nil {
		return fmt.Errorf("prepare delivery insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	nowStr := UTCNow().Format(time.RFC3339)
	for _, t := range targets {
		id := generateID("mdl")
		if _, err := stmt.Exec(id, messageID, t.RecipientID, t.Channel, nowStr); err != nil {
			return fmt.Errorf("insert delivery for %s: %w", t.RecipientID, err)
		}
	}

	return tx.Commit()
}

// MarkDelivered sets delivery status to "delivered" with a timestamp.
func (mdb *MessageDB) MarkDelivered(deliveryID string) error {
	now := UTCNow().Format(time.RFC3339)
	result, err := mdb.db.Exec(
		`UPDATE message_delivery SET status = 'delivered', delivered_at = ? WHERE id = ? AND status = 'pending'`,
		now, deliveryID,
	)
	if err != nil {
		return fmt.Errorf("mark delivered: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDeliveryNotFound
	}
	return nil
}

// MarkCopied sets delivery status to "copied" (email copy sent).
func (mdb *MessageDB) MarkCopied(deliveryID string) error {
	now := UTCNow().Format(time.RFC3339)
	result, err := mdb.db.Exec(
		`UPDATE message_delivery SET status = 'copied', delivered_at = ? WHERE id = ? AND status = 'pending'`,
		now, deliveryID,
	)
	if err != nil {
		return fmt.Errorf("mark copied: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDeliveryNotFound
	}
	return nil
}

// MarkRead sets delivery status to "read" with a timestamp.
func (mdb *MessageDB) MarkRead(deliveryID string) error {
	now := UTCNow().Format(time.RFC3339)
	result, err := mdb.db.Exec(
		`UPDATE message_delivery SET status = 'read', read_at = ?
		 WHERE id = ? AND status IN ('pending', 'delivered', 'copied')`,
		now, deliveryID,
	)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDeliveryNotFound
	}
	return nil
}

// MarkFailed sets delivery status to "failed".
func (mdb *MessageDB) MarkFailed(deliveryID string) error {
	result, err := mdb.db.Exec(
		`UPDATE message_delivery SET status = 'failed' WHERE id = ? AND status = 'pending'`,
		deliveryID,
	)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDeliveryNotFound
	}
	return nil
}

// GetDelivery retrieves a delivery record by ID.
func (mdb *MessageDB) GetDelivery(id string) (*MessageDelivery, error) {
	var d MessageDelivery
	var deliveredAt, readAt sql.NullString
	var createdAt string

	err := mdb.db.QueryRow(
		`SELECT id, message_id, recipient_id, channel, status, delivered_at, read_at, created_at
		 FROM message_delivery WHERE id = ?`, id,
	).Scan(&d.ID, &d.MessageID, &d.RecipientID, &d.Channel, &d.Status,
		&deliveredAt, &readAt, &createdAt)

	if err == sql.ErrNoRows {
		return nil, ErrDeliveryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query delivery: %w", err)
	}

	if deliveredAt.Valid {
		t := ParseDBTime(deliveredAt.String)
		d.DeliveredAt = &t
	}
	if readAt.Valid {
		t := ParseDBTime(readAt.String)
		d.ReadAt = &t
	}
	d.CreatedAt = ParseDBTime(createdAt)

	return &d, nil
}

// GetDeliveryByRecipient finds a delivery for a specific message and recipient.
func (mdb *MessageDB) GetDeliveryByRecipient(messageID, recipientID string) (*MessageDelivery, error) {
	var d MessageDelivery
	var deliveredAt, readAt sql.NullString
	var createdAt string

	err := mdb.db.QueryRow(
		`SELECT id, message_id, recipient_id, channel, status, delivered_at, read_at, created_at
		 FROM message_delivery WHERE message_id = ? AND recipient_id = ?`, messageID, recipientID,
	).Scan(&d.ID, &d.MessageID, &d.RecipientID, &d.Channel, &d.Status,
		&deliveredAt, &readAt, &createdAt)

	if err == sql.ErrNoRows {
		return nil, ErrDeliveryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query delivery by recipient: %w", err)
	}

	if deliveredAt.Valid {
		t := ParseDBTime(deliveredAt.String)
		d.DeliveredAt = &t
	}
	if readAt.Valid {
		t := ParseDBTime(readAt.String)
		d.ReadAt = &t
	}
	d.CreatedAt = ParseDBTime(createdAt)

	return &d, nil
}

// ListPendingEmailDeliveries returns pending email deliveries for outbound processing.
func (mdb *MessageDB) ListPendingEmailDeliveries(limit int) ([]*PendingDelivery, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := mdb.db.Query(
		`SELECT d.id, d.message_id, d.recipient_id,
		        m.sender_id, m.subject, m.content, m.intent, m.entity_type, m.entity_id, m.metadata, m.created_at
		 FROM message_delivery d
		 JOIN message m ON d.message_id = m.id
		 WHERE d.channel = 'email' AND d.status = 'pending'
		 ORDER BY d.created_at ASC
		 LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query pending email deliveries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*PendingDelivery
	for rows.Next() {
		var pd PendingDelivery
		var entityType, entityID sql.NullString
		var metadataJSON, createdAt string

		if err := rows.Scan(&pd.DeliveryID, &pd.MessageID, &pd.RecipientID,
			&pd.SenderID, &pd.Subject, &pd.Content, &pd.Intent, &entityType, &entityID,
			&metadataJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("scan pending delivery: %w", err)
		}

		if entityType.Valid {
			pd.EntityType = entityType.String
		}
		if entityID.Valid {
			pd.EntityID = entityID.String
		}
		_ = json.Unmarshal([]byte(metadataJSON), &pd.Metadata)
		pd.CreatedAt = ParseDBTime(createdAt)

		results = append(results, &pd)
	}

	return results, nil
}

// SetCopyEmail sets the copy_email field on a delivery record.
// Called at send time for internal-channel recipients whose user has email_copy enabled.
func (mdb *MessageDB) SetCopyEmail(deliveryID, email string) error {
	_, err := mdb.db.Exec(
		`UPDATE message_delivery SET copy_email = ? WHERE id = ?`,
		email, deliveryID,
	)
	if err != nil {
		return fmt.Errorf("set copy email: %w", err)
	}
	return nil
}

// ListPendingCopyDeliveries returns internal deliveries with copy_email set
// that are still in "pending" status (email copy not yet sent).
func (mdb *MessageDB) ListPendingCopyDeliveries(limit int) ([]*PendingDelivery, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := mdb.db.Query(
		`SELECT d.id, d.message_id, d.recipient_id, d.copy_email,
		        m.sender_id, m.subject, m.content, m.intent, m.entity_type, m.entity_id, m.metadata, m.created_at
		 FROM message_delivery d
		 JOIN message m ON d.message_id = m.id
		 WHERE d.copy_email IS NOT NULL AND d.status = 'pending'
		 ORDER BY d.created_at ASC
		 LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query pending copy deliveries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*PendingDelivery
	for rows.Next() {
		var pd PendingDelivery
		var copyEmail string
		var entityType, entityID sql.NullString
		var metadataJSON, createdAt string

		if err := rows.Scan(&pd.DeliveryID, &pd.MessageID, &pd.RecipientID, &copyEmail,
			&pd.SenderID, &pd.Subject, &pd.Content, &pd.Intent, &entityType, &entityID,
			&metadataJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("scan pending copy delivery: %w", err)
		}

		if entityType.Valid {
			pd.EntityType = entityType.String
		}
		if entityID.Valid {
			pd.EntityID = entityID.String
		}
		_ = json.Unmarshal([]byte(metadataJSON), &pd.Metadata)
		pd.CreatedAt = ParseDBTime(createdAt)
		// Store copy_email in metadata for the intercom to use
		if pd.Metadata == nil {
			pd.Metadata = make(map[string]interface{})
		}
		pd.Metadata["_copy_email"] = copyEmail

		results = append(results, &pd)
	}

	return results, nil
}

// EmailDeliveryLookup holds the result of looking up an email delivery by
// its RFC 2822 Message-ID or subject ref_code.
type EmailDeliveryLookup struct {
	DeliveryID  string
	MessageID   string // internal message ID (msg_xxx)
	RecipientID string // who received the outbound email
	SenderID    string // who sent the original internal message
	DeliveredAt *time.Time
}

// SetEmailRouting stores the email Message-ID and ref_code on a delivery record
// so inbound replies can be matched back to the original message.
func (mdb *MessageDB) SetEmailRouting(deliveryID, emailMessageID, refCode string) error {
	result, err := mdb.db.Exec(
		`UPDATE message_delivery SET email_message_id = ?, ref_code = ? WHERE id = ?`,
		emailMessageID, refCode, deliveryID,
	)
	if err != nil {
		return fmt.Errorf("set email routing: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDeliveryNotFound
	}
	return nil
}

// LookupDeliveryByEmailMessageID finds a delivery by the RFC 2822 Message-ID
// that was set on the outbound email. Joins with message to return sender_id.
func (mdb *MessageDB) LookupDeliveryByEmailMessageID(emailMessageID string) (*EmailDeliveryLookup, error) {
	var l EmailDeliveryLookup
	var deliveredAt sql.NullString

	err := mdb.db.QueryRow(
		`SELECT d.id, d.message_id, d.recipient_id, m.sender_id, d.delivered_at
		 FROM message_delivery d
		 JOIN message m ON d.message_id = m.id
		 WHERE d.email_message_id = ?`, emailMessageID,
	).Scan(&l.DeliveryID, &l.MessageID, &l.RecipientID, &l.SenderID, &deliveredAt)

	if err == sql.ErrNoRows {
		return nil, ErrDeliveryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lookup by email message id: %w", err)
	}

	if deliveredAt.Valid {
		t := ParseDBTime(deliveredAt.String)
		l.DeliveredAt = &t
	}
	return &l, nil
}

// LookupDeliveryByRefCode finds a delivery by the short ref_code used in
// subject-line [TS-xxxx] tags. Joins with message to return sender_id.
func (mdb *MessageDB) LookupDeliveryByRefCode(refCode string) (*EmailDeliveryLookup, error) {
	var l EmailDeliveryLookup
	var deliveredAt sql.NullString

	err := mdb.db.QueryRow(
		`SELECT d.id, d.message_id, d.recipient_id, m.sender_id, d.delivered_at
		 FROM message_delivery d
		 JOIN message m ON d.message_id = m.id
		 WHERE d.ref_code = ?`, refCode,
	).Scan(&l.DeliveryID, &l.MessageID, &l.RecipientID, &l.SenderID, &deliveredAt)

	if err == sql.ErrNoRows {
		return nil, ErrDeliveryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lookup by ref code: %w", err)
	}

	if deliveredAt.Valid {
		t := ParseDBTime(deliveredAt.String)
		l.DeliveredAt = &t
	}
	return &l, nil
}
