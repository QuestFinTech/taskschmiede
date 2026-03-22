package docs

import (
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantTitle   string
		wantDesc    string
		wantBodyLen int // > 0 means check body is non-empty
	}{
		{
			name:        "standard frontmatter",
			input:       "---\ntitle: \"Test Guide\"\ndescription: \"A test guide\"\nweight: 10\ntype: docs\n---\n\n## Content\n\nSome text.",
			wantTitle:   "Test Guide",
			wantDesc:    "A test guide",
			wantBodyLen: 1,
		},
		{
			name:        "no frontmatter",
			input:       "## Just content\n\nNo frontmatter here.",
			wantTitle:   "",
			wantDesc:    "",
			wantBodyLen: 1,
		},
		{
			name:        "unquoted values",
			input:       "---\ntitle: Unquoted Title\ndescription: Unquoted description\n---\n\nBody",
			wantTitle:   "Unquoted Title",
			wantDesc:    "Unquoted description",
			wantBodyLen: 1,
		},
		{
			name:        "empty body",
			input:       "---\ntitle: \"Empty\"\ndescription: \"Nothing\"\n---\n",
			wantTitle:   "Empty",
			wantDesc:    "Nothing",
			wantBodyLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, desc, body := parseFrontmatter(tt.input)
			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			if desc != tt.wantDesc {
				t.Errorf("description = %q, want %q", desc, tt.wantDesc)
			}
			if tt.wantBodyLen > 0 && len(body) == 0 {
				t.Error("body should not be empty")
			}
		})
	}
}

func TestLoadEmbeddedDocs(t *testing.T) {
	docs := LoadEmbeddedDocs()

	// With content synced, we should have guides and workflows
	if len(docs) == 0 {
		t.Skip("no embedded docs found (content not synced)")
	}

	// Check that all docs have required fields
	for _, doc := range docs {
		if doc.Name == "" {
			t.Error("doc has empty name")
		}
		if doc.Type != "guide" && doc.Type != "workflow" {
			t.Errorf("doc %q has unexpected type %q", doc.Name, doc.Type)
		}
		if doc.Title == "" {
			t.Errorf("doc %q has empty title", doc.Name)
		}
		if doc.Body == "" {
			t.Errorf("doc %q has empty body", doc.Name)
		}
	}

	// Check type distribution
	guides := 0
	workflows := 0
	for _, doc := range docs {
		switch doc.Type {
		case "guide":
			guides++
		case "workflow":
			workflows++
		}
	}
	if guides == 0 {
		t.Error("expected at least one guide")
	}
	if workflows == 0 {
		t.Error("expected at least one workflow")
	}
}

func TestRegistryContentDocs(t *testing.T) {
	r := DefaultRegistry("test")

	all := r.AllContentDocs()
	if len(all) == 0 {
		t.Skip("no embedded docs found (content not synced)")
	}

	// Test type filtering
	guides := r.ContentDocsByType("guide")
	workflows := r.ContentDocsByType("workflow")

	if len(guides)+len(workflows) != len(all) {
		t.Errorf("guide count (%d) + workflow count (%d) != total (%d)",
			len(guides), len(workflows), len(all))
	}

	// Test GetContentDoc
	first := all[0]
	got := r.GetContentDoc(first.Name)
	if got == nil {
		t.Errorf("GetContentDoc(%q) returned nil", first.Name)
	}

	// Test non-existent
	got = r.GetContentDoc("does-not-exist")
	if got != nil {
		t.Error("GetContentDoc for non-existent name should return nil")
	}
}
