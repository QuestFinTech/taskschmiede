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


// Package docs provides self-documenting tool definitions and documentation generation.
package docs

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ToolDoc contains comprehensive documentation for an MCP tool.
type ToolDoc struct {
	// Name is the tool identifier (e.g., "ts.usr.create").
	Name string `json:"name" yaml:"name"`

	// Category groups tools (e.g., "auth", "user", "org", "task").
	Category string `json:"category" yaml:"category"`

	// Summary is a one-line description for listings.
	Summary string `json:"summary" yaml:"summary"`

	// Description is a detailed explanation of what the tool does.
	Description string `json:"description" yaml:"description"`

	// Parameters defines the input schema with documentation.
	Parameters []ParamDoc `json:"parameters" yaml:"parameters"`

	// RequiredParams lists required parameter names.
	RequiredParams []string `json:"required_params" yaml:"required_params"`

	// Returns describes the response structure.
	Returns ReturnDoc `json:"returns" yaml:"returns"`

	// Errors lists possible error codes and their meanings.
	Errors []ErrorDoc `json:"errors" yaml:"errors"`

	// Examples shows usage examples.
	Examples []ExampleDoc `json:"examples" yaml:"examples"`

	// RequiresAuth indicates whether the tool requires authentication.
	RequiresAuth bool `json:"requires_auth" yaml:"requires_auth"`

	// RelatedTools lists related tool names.
	RelatedTools []string `json:"related_tools" yaml:"related_tools"`

	// Since indicates the version when this tool was added.
	Since string `json:"since" yaml:"since"`

	// Deprecated indicates the tool is deprecated and should not be used.
	Deprecated bool `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`

	// Visibility controls whether the tool appears in public docs ("public" or "internal").
	Visibility string `json:"visibility,omitempty" yaml:"visibility,omitempty"`
}

// ParamDoc documents a single parameter.
type ParamDoc struct {
	Name        string      `json:"name" yaml:"name"`
	Type        string      `json:"type" yaml:"type"`
	Description string      `json:"description" yaml:"description"`
	Required    bool        `json:"required" yaml:"required"`
	Default     interface{} `json:"default,omitempty" yaml:"default,omitempty"`
	Example     interface{} `json:"example,omitempty" yaml:"example,omitempty"`
}

// ReturnDoc documents the return value.
type ReturnDoc struct {
	Description string                 `json:"description" yaml:"description"`
	Schema      map[string]interface{} `json:"schema" yaml:"schema"`
	Example     map[string]interface{} `json:"example,omitempty" yaml:"example,omitempty"`
}

// ErrorDoc documents a possible error.
type ErrorDoc struct {
	Code        string `json:"code" yaml:"code"`
	Description string `json:"description" yaml:"description"`
	Example     string `json:"example,omitempty" yaml:"example,omitempty"`
}

// ExampleDoc shows a usage example.
type ExampleDoc struct {
	Title       string                 `json:"title" yaml:"title"`
	Description string                 `json:"description" yaml:"description"`
	Input       map[string]interface{} `json:"input" yaml:"input"`
	Output      map[string]interface{} `json:"output" yaml:"output"`
}

// Registry holds all tool and content documentation.
type Registry struct {
	tools       map[string]*ToolDoc
	contentDocs map[string]*ContentDoc
	version     string
	baseURL     string
}

// NewRegistry creates a new documentation registry.
func NewRegistry(version, baseURL string) *Registry {
	return &Registry{
		tools:       make(map[string]*ToolDoc),
		contentDocs: make(map[string]*ContentDoc),
		version:     version,
		baseURL:     baseURL,
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool *ToolDoc) {
	r.tools[tool.Name] = tool
}

// Get returns a tool by name.
func (r *Registry) Get(name string) *ToolDoc {
	return r.tools[name]
}

// RegisterContentDoc adds an embedded content document to the registry.
func (r *Registry) RegisterContentDoc(doc *ContentDoc) {
	r.contentDocs[doc.Name] = doc
}

// GetContentDoc returns a content document by name.
func (r *Registry) GetContentDoc(name string) *ContentDoc {
	return r.contentDocs[name]
}

// AllContentDocs returns all content documents sorted by name.
func (r *Registry) AllContentDocs() []*ContentDoc {
	docs := make([]*ContentDoc, 0, len(r.contentDocs))
	for _, d := range r.contentDocs {
		docs = append(docs, d)
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Name < docs[j].Name
	})
	return docs
}

// ContentDocsByType returns content documents of a specific type, sorted by name.
func (r *Registry) ContentDocsByType(docType string) []*ContentDoc {
	var docs []*ContentDoc
	for _, d := range r.contentDocs {
		if d.Type == docType {
			docs = append(docs, d)
		}
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Name < docs[j].Name
	})
	return docs
}

// RegisterEmbeddedDocs loads embedded Markdown files and registers them.
func RegisterEmbeddedDocs(r *Registry) {
	for _, doc := range LoadEmbeddedDocs() {
		r.RegisterContentDoc(doc)
	}
}

// All returns all tools sorted by name.
func (r *Registry) All() []*ToolDoc {
	tools := make([]*ToolDoc, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools
}

// ByCategory returns tools grouped by category.
func (r *Registry) ByCategory() map[string][]*ToolDoc {
	result := make(map[string][]*ToolDoc)
	for _, t := range r.tools {
		result[t.Category] = append(result[t.Category], t)
	}
	// Sort each category
	for cat := range result {
		sort.Slice(result[cat], func(i, j int) bool {
			return result[cat][i].Name < result[cat][j].Name
		})
	}
	return result
}

// Categories returns sorted category names.
func (r *Registry) Categories() []string {
	cats := make(map[string]bool)
	for _, t := range r.tools {
		cats[t.Category] = true
	}
	result := make([]string, 0, len(cats))
	for c := range cats {
		result = append(result, c)
	}
	sort.Strings(result)
	return result
}

// Version returns the API version, stripped of any -dirty suffix for public display.
func (r *Registry) Version() string {
	return strings.TrimSuffix(r.version, "-dirty")
}

// BaseURL returns the base URL.
func (r *Registry) BaseURL() string {
	return r.baseURL
}

// ToJSON exports the registry as JSON (full schema with parameters, examples, etc.).
func (r *Registry) ToJSON() ([]byte, error) {
	export := struct {
		Version string     `json:"version"`
		BaseURL string     `json:"base_url"`
		Tools   []*ToolDoc `json:"tools"`
	}{
		Version: r.version,
		BaseURL: r.baseURL,
		Tools:   r.All(),
	}
	return json.MarshalIndent(export, "", "  ")
}

// ParamSummary is a lightweight parameter entry for the tool index.
type ParamSummary struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// ToolSummary is a lightweight tool entry for discovery.
type ToolSummary struct {
	Name         string         `json:"name"`
	Category     string         `json:"category"`
	Summary      string         `json:"summary"`
	RequiresAuth bool           `json:"requires_auth"`
	Since        string         `json:"since"`
	Parameters   []ParamSummary `json:"parameters,omitempty"`
	Deprecated   bool           `json:"deprecated,omitempty"`
	DocURL       string         `json:"doc_url,omitempty"`
	Visibility   string         `json:"visibility,omitempty"`
}

// ToToolsJSON exports a lightweight tool index for discovery.
func (r *Registry) ToToolsJSON() ([]byte, error) {
	tools := r.All()
	summaries := make([]ToolSummary, len(tools))
	for i, t := range tools {
		// Map parameters to lightweight summaries
		var params []ParamSummary
		if len(t.Parameters) > 0 {
			params = make([]ParamSummary, len(t.Parameters))
			for j, p := range t.Parameters {
				params[j] = ParamSummary{
					Name:     p.Name,
					Type:     p.Type,
					Required: p.Required,
				}
			}
		}

		// Default visibility to "public" if not set
		visibility := t.Visibility
		if visibility == "" {
			visibility = "public"
		}

		summaries[i] = ToolSummary{
			Name:         t.Name,
			Category:     t.Category,
			Summary:      t.Summary,
			RequiresAuth: t.RequiresAuth,
			Since:        t.Since,
			Parameters:   params,
			Deprecated:   t.Deprecated,
			DocURL:       r.baseURL + "/tools/" + t.Name + ".html",
			Visibility:   visibility,
		}
	}
	export := struct {
		Version string        `json:"version"`
		BaseURL string        `json:"base_url"`
		Tools   []ToolSummary `json:"tools"`
	}{
		Version: r.version,
		BaseURL: r.baseURL,
		Tools:   summaries,
	}
	return json.MarshalIndent(export, "", "  ")
}

// ToMarkdown generates markdown documentation for a single tool.
func (t *ToolDoc) ToMarkdown() string {
	var sb strings.Builder

	// Header with YAML frontmatter for agents
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "tool: %s\n", t.Name)
	fmt.Fprintf(&sb, "category: %s\n", t.Category)
	fmt.Fprintf(&sb, "requires_auth: %v\n", t.RequiresAuth)
	sb.WriteString("---\n\n")

	// Title
	fmt.Fprintf(&sb, "# %s\n\n", t.Name)

	// Summary
	fmt.Fprintf(&sb, "%s\n\n", t.Summary)

	// Authentication requirement
	if t.RequiresAuth {
		sb.WriteString("**Authentication required**: Yes (Bearer token)\n\n")
	} else {
		sb.WriteString("**Authentication required**: No\n\n")
	}

	// Description
	sb.WriteString("## Description\n\n")
	sb.WriteString(t.Description)
	sb.WriteString("\n\n")

	// Parameters
	sb.WriteString("## Parameters\n\n")
	if len(t.Parameters) == 0 {
		sb.WriteString("No parameters.\n\n")
	} else {
		sb.WriteString("| Name | Type | Required | Default | Description |\n")
		sb.WriteString("|------|------|----------|---------|-------------|\n")
		for _, p := range t.Parameters {
			req := ""
			if p.Required {
				req = "Yes"
			}
			def := ""
			if p.Default != nil {
				def = fmt.Sprintf("%v", p.Default)
			}
			fmt.Fprintf(&sb, "| `%s` | %s | %s | %s | %s |\n",
				p.Name, p.Type, req, def, p.Description)
		}
		sb.WriteString("\n")
	}

	// Returns
	sb.WriteString("## Response\n\n")
	sb.WriteString(t.Returns.Description)
	sb.WriteString("\n\n")
	if t.Returns.Example != nil {
		sb.WriteString("```json\n")
		jsonBytes, _ := json.MarshalIndent(t.Returns.Example, "", "  ")
		sb.WriteString(string(jsonBytes))
		sb.WriteString("\n```\n\n")
	}

	// Errors
	if len(t.Errors) > 0 {
		sb.WriteString("## Errors\n\n")
		sb.WriteString("| Code | Description |\n")
		sb.WriteString("|------|-------------|\n")
		for _, e := range t.Errors {
			fmt.Fprintf(&sb, "| `%s` | %s |\n", e.Code, e.Description)
		}
		sb.WriteString("\n")
	}

	// Examples
	if len(t.Examples) > 0 {
		sb.WriteString("## Examples\n\n")
		for _, ex := range t.Examples {
			fmt.Fprintf(&sb, "### %s\n\n", ex.Title)
			if ex.Description != "" {
				sb.WriteString(ex.Description + "\n\n")
			}
			sb.WriteString("**Request:**\n")
			sb.WriteString("```json\n")
			jsonBytes, _ := json.MarshalIndent(ex.Input, "", "  ")
			sb.WriteString(string(jsonBytes))
			sb.WriteString("\n```\n\n")
			sb.WriteString("**Response:**\n")
			sb.WriteString("```json\n")
			jsonBytes, _ = json.MarshalIndent(ex.Output, "", "  ")
			sb.WriteString(string(jsonBytes))
			sb.WriteString("\n```\n\n")
		}
	}

	// Related tools
	if len(t.RelatedTools) > 0 {
		sb.WriteString("## Related Tools\n\n")
		for _, rt := range t.RelatedTools {
			fmt.Fprintf(&sb, "- [%s](%s.md)\n", rt, rt)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
