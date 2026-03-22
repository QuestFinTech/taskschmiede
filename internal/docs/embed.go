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

package docs

import (
	"embed"
	"io/fs"
	"path/filepath"
	"strings"
)

// contentFS holds embedded documentation files (guides and workflows).
//
//go:embed content
var contentFS embed.FS

// ContentDoc represents an embedded documentation file (guide or workflow).
// These are loaded from Markdown files in the content/ directory at compile time.
type ContentDoc struct {
	// Name is derived from the filename (e.g., "onboard-agent" from "onboard-agent.md").
	Name string

	// Type is derived from the parent directory ("guide" or "workflow").
	Type string

	// Title from the YAML frontmatter.
	Title string

	// Summary from the YAML frontmatter "description" field.
	Summary string

	// Body is the Markdown content with frontmatter stripped.
	Body string
}

// LoadEmbeddedDocs reads all Markdown files from the embedded content filesystem
// and returns them as ContentDoc entries. Files are expected to have YAML frontmatter
// with "title" and "description" fields. The doc type is derived from the directory
// name (guides/ -> "guide", workflows/ -> "workflow").
func LoadEmbeddedDocs() []*ContentDoc {
	var docs []*ContentDoc

	dirMap := map[string]string{
		"content/guides":    "guide",
		"content/workflows": "workflow",
	}

	for dir, docType := range dirMap {
		entries, err := fs.ReadDir(contentFS, dir)
		if err != nil {
			// Directory may not exist (empty content during bare go build)
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			// Skip index files
			if entry.Name() == "_index.md" {
				continue
			}

			data, err := contentFS.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}

			name := strings.TrimSuffix(entry.Name(), ".md")
			title, summary, body := parseFrontmatter(string(data))

			docs = append(docs, &ContentDoc{
				Name:    name,
				Type:    docType,
				Title:   title,
				Summary: summary,
				Body:    body,
			})
		}
	}

	return docs
}

// parseFrontmatter extracts title and description from YAML frontmatter
// and returns the body with frontmatter stripped. This is a lightweight parser
// that avoids pulling in a full YAML dependency for simple key: value extraction.
func parseFrontmatter(content string) (title, description, body string) {
	if !strings.HasPrefix(content, "---\n") {
		return "", "", content
	}

	// Find closing ---
	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return "", "", content
	}

	frontmatter := content[4 : 4+end]
	body = strings.TrimLeft(content[4+end+4:], "\n")

	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if key, val, ok := parseFMLine(line, "title"); ok {
			title = val
			_ = key
		}
		if key, val, ok := parseFMLine(line, "description"); ok {
			description = val
			_ = key
		}
	}

	return title, description, body
}

// parseFMLine extracts a value for a given key from a YAML frontmatter line.
// Returns the key, unquoted value, and whether the key matched.
func parseFMLine(line, key string) (string, string, bool) {
	prefix := key + ":"
	if !strings.HasPrefix(line, prefix) {
		return "", "", false
	}
	val := strings.TrimSpace(line[len(prefix):])
	// Remove surrounding quotes
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		val = val[1 : len(val)-1]
	}
	return key, val, true
}
