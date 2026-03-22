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
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/QuestFinTech/taskschmiede/internal/service"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// msgTestEnv extends testEnv with messaging support and a second user.
type msgTestEnv struct {
	*testEnv
	msgDB *storage.MessageDB

	// Second user (recipient for tests)
	recipientToken      string
	recipientUserID     string
	recipientResourceID string
}

func newMsgTestEnv(t *testing.T) *msgTestEnv {
	t.Helper()

	base := newTestEnv(t)

	// Open in-memory MessageDB.
	mdb, err := storage.OpenMessageDB(":memory:")
	if err != nil {
		t.Fatalf("open message db: %v", err)
	}
	t.Cleanup(func() { _ = mdb.Close() })

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	msgSvc := service.NewMessageService(mdb, base.db, logger)

	// Recreate API with messaging enabled.
	base.api = New(&Config{
		DB:          base.db,
		Logger:      logger,
		AuthService: base.authSvc,
		MsgService:  msgSvc,
	})
	base.handler = base.api.Handler()

	// Create a second user and resource.
	const recipientResID = "res_test_recipient"
	_, err = base.db.Exec(
		`INSERT INTO resource (id, type, name, capacity_model, metadata, status)
		 VALUES (?, 'agent', 'Test Recipient', 'always_on', '{}', 'active')`,
		recipientResID,
	)
	if err != nil {
		t.Fatalf("create recipient resource: %v", err)
	}

	const recipientUserID = "usr_test_recipient"
	_, err = base.db.Exec(
		`INSERT INTO user (id, name, email, resource_id, password_hash, tier, user_type, metadata, status)
		 VALUES (?, 'Test Recipient', 'recipient@test.local', ?, ?, 1, 'agent', '{}', 'active')`,
		recipientUserID, recipientResID, mustHashPassword(t, "TestPass123!"),
	)
	if err != nil {
		t.Fatalf("create recipient user: %v", err)
	}

	token, _, err := base.authSvc.CreateToken(context.Background(), recipientUserID, "recipient-token", nil)
	if err != nil {
		t.Fatalf("create recipient token: %v", err)
	}

	return &msgTestEnv{
		testEnv:             base,
		msgDB:               mdb,
		recipientToken:      token,
		recipientUserID:     recipientUserID,
		recipientResourceID: recipientResID,
	}
}

func TestMessageSendDirect(t *testing.T) {
	env := newMsgTestEnv(t)

	// Admin sends a message to recipient.
	rec := env.doRequest("POST", "/api/v1/messages", map[string]interface{}{
		"recipient_ids": []string{env.recipientResourceID},
		"content":       "Hello from admin",
		"subject":       "Greetings",
		"intent":        "info",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("send: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	msgID := data["id"].(string)
	if data["sender_id"] != env.adminResourceID {
		t.Errorf("sender_id = %v, want %v", data["sender_id"], env.adminResourceID)
	}
	if data["content"] != "Hello from admin" {
		t.Errorf("content = %v, want Hello from admin", data["content"])
	}
	if data["subject"] != "Greetings" {
		t.Errorf("subject = %v, want Greetings", data["subject"])
	}
	if data["intent"] != "info" {
		t.Errorf("intent = %v, want info", data["intent"])
	}
	if data["sender_name"] != "Test Admin" {
		t.Errorf("sender_name = %v, want Test Admin", data["sender_name"])
	}

	// Recipient checks inbox -> should see the message.
	rec = env.doRequestAs(env.recipientToken, "GET", "/api/v1/messages", nil)
	items, meta := env.parseList(rec)
	if int(meta["total"].(float64)) != 1 {
		t.Fatalf("inbox: total = %v, want 1", meta["total"])
	}
	if len(items) != 1 {
		t.Fatalf("inbox: items = %d, want 1", len(items))
	}
	item := items[0].(map[string]interface{})
	if item["id"] != msgID {
		t.Errorf("inbox item id = %v, want %v", item["id"], msgID)
	}
	if item["sender_name"] != "Test Admin" {
		t.Errorf("inbox sender_name = %v, want Test Admin", item["sender_name"])
	}

	// Admin's inbox should be empty (sender excluded from delivery).
	rec = env.doRequest("GET", "/api/v1/messages", nil)
	_, meta = env.parseList(rec)
	if int(meta["total"].(float64)) != 0 {
		t.Errorf("admin inbox: total = %v, want 0", meta["total"])
	}
}

func TestMessageSendGroup(t *testing.T) {
	env := newMsgTestEnv(t)

	// Create a third resource + user for group messaging.
	const thirdResID = "res_test_third"
	_, err := env.db.Exec(
		`INSERT INTO resource (id, type, name, capacity_model, metadata, status)
		 VALUES (?, 'human', 'Third User', 'hours_per_week', '{}', 'active')`,
		thirdResID,
	)
	if err != nil {
		t.Fatalf("create third resource: %v", err)
	}
	const thirdUserID = "usr_test_third"
	_, err = env.db.Exec(
		`INSERT INTO user (id, name, email, resource_id, password_hash, tier, user_type, metadata, status)
		 VALUES (?, 'Third User', 'third@test.local', ?, ?, 1, 'human', '{}', 'active')`,
		thirdUserID, thirdResID, mustHashPassword(t, "TestPass123!"),
	)
	if err != nil {
		t.Fatalf("create third user: %v", err)
	}
	thirdToken, _, err := env.authSvc.CreateToken(context.Background(), thirdUserID, "third-token", nil)
	if err != nil {
		t.Fatalf("create third token: %v", err)
	}

	// Create endeavour and add members.
	edvID := env.createEndeavour("Messaging Team")

	// Add recipient and third resource as members.
	env.doRequest("POST", "/api/v1/relations", map[string]interface{}{
		"relationship_type":  "member_of",
		"source_entity_type": "resource",
		"source_entity_id":   env.recipientResourceID,
		"target_entity_type": "endeavour",
		"target_entity_id":   edvID,
	})
	env.doRequest("POST", "/api/v1/relations", map[string]interface{}{
		"relationship_type":  "member_of",
		"source_entity_type": "resource",
		"source_entity_id":   thirdResID,
		"target_entity_type": "endeavour",
		"target_entity_id":   edvID,
	})
	// Also add admin so they're a member (but will be excluded as sender).
	env.doRequest("POST", "/api/v1/relations", map[string]interface{}{
		"relationship_type":  "member_of",
		"source_entity_type": "resource",
		"source_entity_id":   env.adminResourceID,
		"target_entity_type": "endeavour",
		"target_entity_id":   edvID,
	})

	// Admin sends to the endeavour scope.
	rec := env.doRequest("POST", "/api/v1/messages", map[string]interface{}{
		"scope_type": "endeavour",
		"scope_id":   edvID,
		"content":    "Team announcement",
		"intent":     "info",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("send group: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	if data["scope_type"] != "endeavour" {
		t.Errorf("scope_type = %v, want endeavour", data["scope_type"])
	}
	if data["scope_id"] != edvID {
		t.Errorf("scope_id = %v, want %v", data["scope_id"], edvID)
	}

	// Recipient's inbox should have the message.
	rec = env.doRequestAs(env.recipientToken, "GET", "/api/v1/messages", nil)
	_, meta := env.parseList(rec)
	if int(meta["total"].(float64)) != 1 {
		t.Errorf("recipient inbox: total = %v, want 1", meta["total"])
	}

	// Third user's inbox should also have the message.
	rec = env.doRequestAs(thirdToken, "GET", "/api/v1/messages", nil)
	_, meta = env.parseList(rec)
	if int(meta["total"].(float64)) != 1 {
		t.Errorf("third inbox: total = %v, want 1", meta["total"])
	}

	// Admin inbox should be empty (sender excluded).
	rec = env.doRequest("GET", "/api/v1/messages", nil)
	_, meta = env.parseList(rec)
	if int(meta["total"].(float64)) != 0 {
		t.Errorf("admin inbox: total = %v, want 0", meta["total"])
	}
}

func TestMessageInbox(t *testing.T) {
	env := newMsgTestEnv(t)

	// Send 3 messages from admin to recipient.
	for i := 0; i < 3; i++ {
		rec := env.doRequest("POST", "/api/v1/messages", map[string]interface{}{
			"recipient_ids": []string{env.recipientResourceID},
			"content":       "Message number",
			"intent":        "info",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("send %d: expected 201, got %d: %s", i, rec.Code, rec.Body.String())
		}
	}

	// Recipient inbox: all 3.
	rec := env.doRequestAs(env.recipientToken, "GET", "/api/v1/messages", nil)
	_, meta := env.parseList(rec)
	if int(meta["total"].(float64)) != 3 {
		t.Errorf("inbox all: total = %v, want 3", meta["total"])
	}

	// Pagination: limit=2, offset=0.
	rec = env.doRequestAs(env.recipientToken, "GET", "/api/v1/messages?limit=2&offset=0", nil)
	items, meta := env.parseList(rec)
	if int(meta["total"].(float64)) != 3 {
		t.Errorf("inbox paginated: total = %v, want 3 (total count stays)", meta["total"])
	}
	if len(items) != 2 {
		t.Errorf("inbox paginated: items = %d, want 2", len(items))
	}

	// Pagination: limit=2, offset=2.
	rec = env.doRequestAs(env.recipientToken, "GET", "/api/v1/messages?limit=2&offset=2", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("inbox page 2: items = %d, want 1", len(items))
	}

	// Unread filter: all should be unread (status=pending).
	rec = env.doRequestAs(env.recipientToken, "GET", "/api/v1/messages?unread=true", nil)
	_, meta = env.parseList(rec)
	if int(meta["total"].(float64)) != 3 {
		t.Errorf("inbox unread: total = %v, want 3", meta["total"])
	}

	// Read one message to change its status.
	rec = env.doRequestAs(env.recipientToken, "GET", "/api/v1/messages?limit=1", nil)
	items, _ = env.parseList(rec)
	firstItem := items[0].(map[string]interface{})
	firstID := firstItem["id"].(string)

	rec = env.doRequestAs(env.recipientToken, "PATCH", "/api/v1/messages/"+firstID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("read: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Now unread filter should return 2.
	rec = env.doRequestAs(env.recipientToken, "GET", "/api/v1/messages?unread=true", nil)
	_, meta = env.parseList(rec)
	if int(meta["total"].(float64)) != 2 {
		t.Errorf("inbox after read: total = %v, want 2", meta["total"])
	}

	// Status filter: read.
	rec = env.doRequestAs(env.recipientToken, "GET", "/api/v1/messages?status=read", nil)
	_, meta = env.parseList(rec)
	if int(meta["total"].(float64)) != 1 {
		t.Errorf("inbox status=read: total = %v, want 1", meta["total"])
	}
}

func TestMessageRead(t *testing.T) {
	env := newMsgTestEnv(t)

	// Admin sends a message.
	rec := env.doRequest("POST", "/api/v1/messages", map[string]interface{}{
		"recipient_ids": []string{env.recipientResourceID},
		"content":       "Please read this",
		"intent":        "action",
	})
	data := env.parseData(rec)
	msgID := data["id"].(string)

	// Recipient GETs message -> auto-marks as read.
	rec = env.doRequestAs(env.recipientToken, "GET", "/api/v1/messages/"+msgID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	data = env.parseData(rec)
	if data["content"] != "Please read this" {
		t.Errorf("content = %v, want 'Please read this'", data["content"])
	}

	delivery := data["delivery"].(map[string]interface{})
	if delivery["status"] != "read" {
		t.Errorf("delivery status = %v, want read", delivery["status"])
	}
	if delivery["read_at"] == nil {
		t.Error("read_at should be set")
	}
	if delivery["recipient_id"] != env.recipientResourceID {
		t.Errorf("delivery recipient_id = %v, want %v", delivery["recipient_id"], env.recipientResourceID)
	}

	// PATCH same message -> still read (idempotent).
	rec = env.doRequestAs(env.recipientToken, "PATCH", "/api/v1/messages/"+msgID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	data = env.parseData(rec)
	delivery = data["delivery"].(map[string]interface{})
	if delivery["status"] != "read" {
		t.Errorf("delivery status after patch = %v, want read", delivery["status"])
	}
}

func TestMessageReply(t *testing.T) {
	env := newMsgTestEnv(t)

	// Admin sends a message to recipient.
	rec := env.doRequest("POST", "/api/v1/messages", map[string]interface{}{
		"recipient_ids": []string{env.recipientResourceID},
		"content":       "Can you help?",
		"intent":        "question",
	})
	data := env.parseData(rec)
	origID := data["id"].(string)

	// Recipient replies.
	rec = env.doRequestAs(env.recipientToken, "POST", "/api/v1/messages/"+origID+"/reply", map[string]interface{}{
		"content": "Sure, happy to help!",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("reply: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data = env.parseData(rec)
	replyID := data["id"].(string)
	if data["reply_to_id"] != origID {
		t.Errorf("reply_to_id = %v, want %v", data["reply_to_id"], origID)
	}
	if data["sender_id"] != env.recipientResourceID {
		t.Errorf("reply sender_id = %v, want %v", data["sender_id"], env.recipientResourceID)
	}
	if data["content"] != "Sure, happy to help!" {
		t.Errorf("reply content = %v, want 'Sure, happy to help!'", data["content"])
	}
	if data["sender_name"] != "Test Recipient" {
		t.Errorf("reply sender_name = %v, want Test Recipient", data["sender_name"])
	}

	// Admin should see the reply in their inbox.
	rec = env.doRequest("GET", "/api/v1/messages", nil)
	items, meta := env.parseList(rec)
	if int(meta["total"].(float64)) != 1 {
		t.Errorf("admin inbox after reply: total = %v, want 1", meta["total"])
	}
	if len(items) > 0 {
		inboxItem := items[0].(map[string]interface{})
		if inboxItem["id"] != replyID {
			t.Errorf("admin inbox item id = %v, want %v", inboxItem["id"], replyID)
		}
	}
}

func TestMessageThread(t *testing.T) {
	env := newMsgTestEnv(t)

	// Message 1: admin -> recipient.
	rec := env.doRequest("POST", "/api/v1/messages", map[string]interface{}{
		"recipient_ids": []string{env.recipientResourceID},
		"content":       "Thread message 1",
		"subject":       "Thread Test",
	})
	data := env.parseData(rec)
	msg1ID := data["id"].(string)

	// Message 2: recipient replies.
	rec = env.doRequestAs(env.recipientToken, "POST", "/api/v1/messages/"+msg1ID+"/reply", map[string]interface{}{
		"content": "Thread message 2",
	})
	data = env.parseData(rec)
	msg2ID := data["id"].(string)

	// Message 3: admin replies to the reply.
	rec = env.doRequest("POST", "/api/v1/messages/"+msg2ID+"/reply", map[string]interface{}{
		"content": "Thread message 3",
	})
	data = env.parseData(rec)
	msg3ID := data["id"].(string)

	// Get thread from any message in the chain.
	rec = env.doRequest("GET", "/api/v1/messages/"+msg2ID+"/thread", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("thread: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	data = env.parseData(rec)
	total := int(data["total"].(float64))
	if total != 3 {
		t.Fatalf("thread total = %d, want 3", total)
	}

	messages := data["messages"].([]interface{})
	if len(messages) != 3 {
		t.Fatalf("thread messages = %d, want 3", len(messages))
	}

	// Verify chronological order.
	m1 := messages[0].(map[string]interface{})
	m2 := messages[1].(map[string]interface{})
	m3 := messages[2].(map[string]interface{})
	if m1["id"] != msg1ID {
		t.Errorf("thread[0] = %v, want %v", m1["id"], msg1ID)
	}
	if m2["id"] != msg2ID {
		t.Errorf("thread[1] = %v, want %v", m2["id"], msg2ID)
	}
	if m3["id"] != msg3ID {
		t.Errorf("thread[2] = %v, want %v", m3["id"], msg3ID)
	}

	// Verify sender names are resolved.
	if m1["sender_name"] != "Test Admin" {
		t.Errorf("thread[0] sender_name = %v, want Test Admin", m1["sender_name"])
	}
	if m2["sender_name"] != "Test Recipient" {
		t.Errorf("thread[1] sender_name = %v, want Test Recipient", m2["sender_name"])
	}
}

func TestMessageIntent(t *testing.T) {
	env := newMsgTestEnv(t)

	intents := []string{"info", "question", "action", "alert"}
	for _, intent := range intents {
		rec := env.doRequest("POST", "/api/v1/messages", map[string]interface{}{
			"recipient_ids": []string{env.recipientResourceID},
			"content":       "Intent test: " + intent,
			"intent":        intent,
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("send intent=%s: expected 201, got %d: %s", intent, rec.Code, rec.Body.String())
		}
		data := env.parseData(rec)
		if data["intent"] != intent {
			t.Errorf("intent = %v, want %v", data["intent"], intent)
		}
	}

	// Verify intent filter works on inbox.
	rec := env.doRequestAs(env.recipientToken, "GET", "/api/v1/messages?intent=alert", nil)
	_, meta := env.parseList(rec)
	if int(meta["total"].(float64)) != 1 {
		t.Errorf("inbox intent=alert: total = %v, want 1", meta["total"])
	}
}

func TestMessageEntityContext(t *testing.T) {
	env := newMsgTestEnv(t)

	// Create a task for context.
	taskID := env.createTask("Context Task", "")

	rec := env.doRequest("POST", "/api/v1/messages", map[string]interface{}{
		"recipient_ids": []string{env.recipientResourceID},
		"content":       "Regarding this task",
		"entity_type":   "task",
		"entity_id":     taskID,
		"metadata":      map[string]interface{}{"priority": "high"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("send with context: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	if data["entity_type"] != "task" {
		t.Errorf("entity_type = %v, want task", data["entity_type"])
	}
	if data["entity_id"] != taskID {
		t.Errorf("entity_id = %v, want %v", data["entity_id"], taskID)
	}
	meta := data["metadata"].(map[string]interface{})
	if meta["priority"] != "high" {
		t.Errorf("metadata.priority = %v, want high", meta["priority"])
	}

	// Filter inbox by entity.
	rec = env.doRequestAs(env.recipientToken, "GET", "/api/v1/messages?entity_type=task&entity_id="+taskID, nil)
	_, listMeta := env.parseList(rec)
	if int(listMeta["total"].(float64)) != 1 {
		t.Errorf("inbox entity filter: total = %v, want 1", listMeta["total"])
	}
}

func TestMessageValidation(t *testing.T) {
	env := newMsgTestEnv(t)

	// Empty content.
	rec := env.doRequest("POST", "/api/v1/messages", map[string]interface{}{
		"recipient_ids": []string{env.recipientResourceID},
	})
	env.parseError(rec, http.StatusBadRequest)

	// Invalid intent.
	rec = env.doRequest("POST", "/api/v1/messages", map[string]interface{}{
		"recipient_ids": []string{env.recipientResourceID},
		"content":       "Test",
		"intent":        "invalid_intent",
	})
	env.parseError(rec, http.StatusBadRequest)

	// No recipients.
	rec = env.doRequest("POST", "/api/v1/messages", map[string]interface{}{
		"content": "Test",
	})
	env.parseError(rec, http.StatusBadRequest)

	// Invalid scope_type.
	rec = env.doRequest("POST", "/api/v1/messages", map[string]interface{}{
		"content":    "Test",
		"scope_type": "invalid",
		"scope_id":   "some_id",
	})
	env.parseError(rec, http.StatusBadRequest)

	// scope_type without scope_id.
	rec = env.doRequest("POST", "/api/v1/messages", map[string]interface{}{
		"content":    "Test",
		"scope_type": "endeavour",
	})
	env.parseError(rec, http.StatusBadRequest)

	// Non-existent message for reply.
	rec = env.doRequest("POST", "/api/v1/messages/msg_nonexistent/reply", map[string]interface{}{
		"content": "Reply to nothing",
	})
	env.parseError(rec, http.StatusNotFound)

	// Non-existent message for read.
	rec = env.doRequestAs(env.recipientToken, "GET", "/api/v1/messages/msg_nonexistent", nil)
	env.parseError(rec, http.StatusNotFound)
}
