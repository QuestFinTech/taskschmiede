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
	"net/http"
	"testing"
)

func TestCommentCRUD(t *testing.T) {
	env := newTestEnv(t)

	// Create a task to comment on
	taskID := env.createTask("Commentable Task", "")

	// Create comment
	rec := env.doRequest("POST", "/api/v1/comments", map[string]interface{}{
		"entity_type": "task",
		"entity_id":   taskID,
		"content":     "This is a test comment",
		"metadata":    map[string]interface{}{"source": "test"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	cmtID := data["id"].(string)
	if data["entity_type"] != "task" {
		t.Errorf("entity_type = %v, want task", data["entity_type"])
	}
	if data["entity_id"] != taskID {
		t.Errorf("entity_id = %v, want %v", data["entity_id"], taskID)
	}
	if data["content"] != "This is a test comment" {
		t.Errorf("content = %v, want This is a test comment", data["content"])
	}
	if data["author_id"] != env.adminResourceID {
		t.Errorf("author_id = %v, want %v", data["author_id"], env.adminResourceID)
	}

	// Get comment
	rec = env.doRequest("GET", "/api/v1/comments/"+cmtID, nil)
	data = env.parseData(rec)
	if data["id"] != cmtID {
		t.Errorf("get: id = %v, want %v", data["id"], cmtID)
	}
	if data["author_name"] != "Test Admin" {
		t.Errorf("get: author_name = %v, want Test Admin", data["author_name"])
	}
	// Should have empty replies array
	replies, ok := data["replies"].([]interface{})
	if !ok {
		t.Error("get: replies should be an array")
	}
	if len(replies) != 0 {
		t.Errorf("get: replies = %d, want 0", len(replies))
	}

	// Create reply
	rec = env.doRequest("POST", "/api/v1/comments", map[string]interface{}{
		"entity_type": "task",
		"entity_id":   taskID,
		"content":     "This is a reply",
		"reply_to_id": cmtID,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("reply: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	replyData := env.parseData(rec)
	replyID := replyData["id"].(string)
	if replyData["reply_to_id"] != cmtID {
		t.Errorf("reply: reply_to_id = %v, want %v", replyData["reply_to_id"], cmtID)
	}

	// Get parent should include reply
	rec = env.doRequest("GET", "/api/v1/comments/"+cmtID, nil)
	data = env.parseData(rec)
	replies = data["replies"].([]interface{})
	if len(replies) != 1 {
		t.Fatalf("get parent: replies = %d, want 1", len(replies))
	}
	replyMap := replies[0].(map[string]interface{})
	if replyMap["id"] != replyID {
		t.Errorf("reply id = %v, want %v", replyMap["id"], replyID)
	}

	// List comments for entity
	rec = env.doRequest("GET", "/api/v1/comments?entity_type=task&entity_id="+taskID, nil)
	items, meta := env.parseList(rec)
	if int(meta["total"].(float64)) != 2 {
		t.Errorf("list: total = %v, want 2", meta["total"])
	}
	if len(items) != 2 {
		t.Errorf("list: items = %d, want 2", len(items))
	}

	// Update comment
	rec = env.doRequest("PATCH", "/api/v1/comments/"+cmtID, map[string]interface{}{
		"content": "Updated comment content",
	})
	data = env.parseData(rec)
	if data["content"] != "Updated comment content" {
		t.Errorf("update: content = %v, want Updated comment content", data["content"])
	}
	if data["edited_at"] == nil {
		t.Error("update: edited_at should be set after edit")
	}

	// Delete comment (soft-delete)
	rec = env.doRequest("DELETE", "/api/v1/comments/"+cmtID, nil)
	data = env.parseData(rec)
	if data["deleted"] != true {
		t.Errorf("delete: deleted = %v, want true", data["deleted"])
	}

	// Get deleted comment should show [deleted]
	rec = env.doRequest("GET", "/api/v1/comments/"+cmtID, nil)
	data = env.parseData(rec)
	if data["content"] != "[deleted]" {
		t.Errorf("get deleted: content = %v, want [deleted]", data["content"])
	}
	if data["deleted_at"] == nil {
		t.Error("get deleted: deleted_at should be set")
	}

	// Get not found
	rec = env.doRequest("GET", "/api/v1/comments/cmt_nonexistent", nil)
	env.parseError(rec, http.StatusNotFound)

	// Create comment with missing entity_type
	rec = env.doRequest("POST", "/api/v1/comments", map[string]interface{}{
		"entity_id": taskID,
		"content":   "missing type",
	})
	env.parseError(rec, http.StatusBadRequest)
}
