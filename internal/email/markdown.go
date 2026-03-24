// Copyright 2026 Quest Financial Technologies S.a r.l.-S., Luxembourg
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

package email

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	goldhtml "github.com/yuin/goldmark/renderer/html"
)

// mdConverter is a reusable goldmark instance with table support.
var mdConverter = goldmark.New(
	goldmark.WithExtensions(extension.Table, extension.Strikethrough),
	goldmark.WithRendererOptions(goldhtml.WithUnsafe()),
)

// MarkdownToHTML converts Markdown text to an HTML email body wrapped in
// the Taskschmiede email template. Returns the full HTML document suitable
// for the HTMLBody field of OutgoingMessage.
func MarkdownToHTML(subject, markdown string) string {
	var buf bytes.Buffer
	if err := mdConverter.Convert([]byte(markdown), &buf); err != nil {
		return ""
	}
	body := wrapTables(buf.String())
	return renderEmailTemplate(subject, body)
}

// wrapTables wraps each <table> in a scrollable div for email clients.
func wrapTables(html string) string {
	html = strings.ReplaceAll(html, "<table>", `<div class="table-wrap"><table>`)
	html = strings.ReplaceAll(html, "</table>", `</table></div>`)
	return html
}

// renderEmailTemplate wraps an HTML body in the Taskschmiede email template.
func renderEmailTemplate(subject, body string) string {
	// Use string concatenation instead of html/template to avoid escaping
	// the already-rendered HTML body.
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; font-size: 14px; line-height: 1.6; color: #333; margin: 0; padding: 0; background: #f5f5f5; }
  .wrapper { width: 95%; max-width: 1200px; margin: 0 auto; background: #fff; }
  .header { background: #1a1a2e; color: #fff; padding: 16px 24px; }
  .header h1 { margin: 0; font-size: 16px; font-weight: 600; }
  .content { padding: 24px; }
  .content h3 { font-size: 15px; color: #1a1a2e; border-bottom: 1px solid #e0e0e0; padding-bottom: 6px; margin-top: 24px; }
  .table-wrap { overflow-x: auto; margin: 12px 0; -webkit-overflow-scrolling: touch; }
  .content table { border-collapse: collapse; width: 100%; min-width: 500px; font-size: 13px; }
  .content th { background: #f0f0f0; text-align: left; padding: 8px 10px; border: 1px solid #ddd; font-weight: 600; white-space: nowrap; }
  .content td { padding: 8px 10px; border: 1px solid #ddd; vertical-align: top; }
  .content td:first-child { white-space: nowrap; }
  .content tr:nth-child(even) td { background: #fafafa; }
  .content code { font-family: 'SF Mono', Monaco, Consolas, monospace; background: #f0f0f0; padding: 1px 4px; border-radius: 3px; font-size: 12px; word-break: break-all; }
  .content ul, .content ol { padding-left: 20px; }
  .content li { margin-bottom: 6px; }
  .content hr { border: none; border-top: 1px solid #e0e0e0; margin: 20px 0; }
  .content em { color: #666; }
  .content p:first-child { font-size: 15px; font-weight: 600; }
  .footer { background: #f5f5f5; padding: 16px 24px; font-size: 12px; color: #888; border-top: 1px solid #e0e0e0; }
  .footer a { color: #555; }
</style>
</head>
<body>
<div class="wrapper">
  <div class="header">
    <h1>`)
	b.WriteString(subject)
	b.WriteString(`</h1>
  </div>
  <div class="content">
    `)
	b.WriteString(body)
	b.WriteString(`
  </div>
  <div class="footer">
    <p>Sent via <a href="https://taskschmiede.com">Taskschmiede</a> Intercom</p>
  </div>
</div>
</body>
</html>
`)
	return b.String()
}

// ContainsMarkdown returns true if the text appears to contain Markdown
// formatting (headings, tables, bold, lists). Used to decide whether to
// convert content to HTML for email delivery.
func ContainsMarkdown(s string) bool {
	// Check for common Markdown patterns.
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") ||
			strings.HasPrefix(trimmed, "## ") ||
			strings.HasPrefix(trimmed, "### ") ||
			strings.HasPrefix(trimmed, "| ") ||
			strings.HasPrefix(trimmed, "- ") ||
			strings.HasPrefix(trimmed, "1. ") ||
			strings.Contains(trimmed, "**") {
			return true
		}
	}
	return false
}
