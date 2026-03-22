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


package security

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Field length limits enforced during input validation.
const (
	MaxTitleLen       = 500
	MaxNameLen        = 500
	MaxDescriptionLen = 50_000
	MaxContentLen     = 50_000  // comment/message content (Markdown)
	MaxPromptLen      = 100_000
	MaxMetadataBytes  = 100_000 // 100 KB
	MaxSearchLen      = 500
	MaxIDLen          = 128
	MaxEmailLen       = 254 // RFC 5321
	MaxURLLen         = 2048
	MaxPasswordLen    = 128
	MaxTagLen         = 100
	MaxTagsCount      = 50
)

// ValidationError represents a validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// emailRegex validates email format with TLD requirement.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// idRegex validates entity ID format (prefix_suffix where suffix is alphanumeric/underscore).
// Production IDs use prefix_hex (e.g., tsk_fa57dac4368f0951e46a77f2).
var idRegex = regexp.MustCompile(`^[a-z]{2,10}_[a-z0-9_]{2,64}$`)

// ValidateTitle checks a title field.
func ValidateTitle(value string) *ValidationError {
	return ValidateStringField(value, "title", MaxTitleLen)
}

// ValidateName checks a name field.
func ValidateName(value string) *ValidationError {
	return ValidateStringField(value, "name", MaxNameLen)
}

// ValidateDescription checks a description field.
func ValidateDescription(value string) *ValidationError {
	if value == "" {
		return nil
	}
	if utf8.RuneCountInString(value) > MaxDescriptionLen {
		return &ValidationError{Field: "description", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxDescriptionLen)}
	}
	if containsNullByte(value) {
		return &ValidationError{Field: "description", Message: "contains invalid characters"}
	}
	return nil
}

// ValidatePrompt checks a ritual prompt field.
func ValidatePrompt(value string) *ValidationError {
	if value == "" {
		return nil
	}
	if utf8.RuneCountInString(value) > MaxPromptLen {
		return &ValidationError{Field: "prompt", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxPromptLen)}
	}
	if containsNullByte(value) {
		return &ValidationError{Field: "prompt", Message: "contains invalid characters"}
	}
	return nil
}

// ValidateMetadata checks metadata JSON size.
func ValidateMetadata(m map[string]interface{}) *ValidationError {
	if m == nil {
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return &ValidationError{Field: "metadata", Message: "invalid JSON structure"}
	}
	if len(b) > MaxMetadataBytes {
		return &ValidationError{Field: "metadata", Message: fmt.Sprintf("exceeds maximum size of %d bytes", MaxMetadataBytes)}
	}
	return nil
}

// ValidateEmail checks email format and length.
func ValidateEmail(email string) *ValidationError {
	if email == "" {
		return nil
	}
	if len(email) > MaxEmailLen {
		return &ValidationError{Field: "email", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxEmailLen)}
	}
	if !emailRegex.MatchString(email) {
		return &ValidationError{Field: "email", Message: "invalid email format"}
	}
	return nil
}

// ValidateURL checks URL format and length.
func ValidateURL(rawURL string) *ValidationError {
	if rawURL == "" {
		return nil
	}
	if len(rawURL) > MaxURLLen {
		return &ValidationError{Field: "url", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxURLLen)}
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return &ValidationError{Field: "url", Message: "invalid URL format"}
	}
	return nil
}

// ValidateID checks an entity ID format.
func ValidateID(id, fieldName string) *ValidationError {
	if id == "" {
		return nil
	}
	if len(id) > MaxIDLen {
		return &ValidationError{Field: fieldName, Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxIDLen)}
	}
	if !idRegex.MatchString(id) {
		return &ValidationError{Field: fieldName, Message: "invalid ID format"}
	}
	return nil
}

// ValidateSearch checks a search query string.
func ValidateSearch(value string) *ValidationError {
	if value == "" {
		return nil
	}
	if utf8.RuneCountInString(value) > MaxSearchLen {
		return &ValidationError{Field: "search", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxSearchLen)}
	}
	return nil
}

// ValidateTags checks a tags array.
func ValidateTags(tags []string) *ValidationError {
	if tags == nil {
		return nil
	}
	if len(tags) > MaxTagsCount {
		return &ValidationError{Field: "tags", Message: fmt.Sprintf("exceeds maximum of %d tags", MaxTagsCount)}
	}
	for i, tag := range tags {
		if utf8.RuneCountInString(tag) > MaxTagLen {
			return &ValidationError{Field: "tags", Message: fmt.Sprintf("tag at index %d exceeds maximum length of %d characters", i, MaxTagLen)}
		}
		if containsNullByte(tag) {
			return &ValidationError{Field: "tags", Message: fmt.Sprintf("tag at index %d contains invalid characters", i)}
		}
	}
	return nil
}

// ValidateStringField checks an arbitrary string field against a max length.
func ValidateStringField(value, fieldName string, maxLen int) *ValidationError {
	if value == "" {
		return nil
	}
	if utf8.RuneCountInString(value) > maxLen {
		return &ValidationError{Field: fieldName, Message: fmt.Sprintf("exceeds maximum length of %d characters", maxLen)}
	}
	if containsNullByte(value) {
		return &ValidationError{Field: fieldName, Message: "contains invalid characters"}
	}
	return nil
}

// SanitizeString trims whitespace and removes null bytes.
// Deprecated: Use SanitizeInput for full input hygiene.
func SanitizeString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\x00", "")
	return s
}

// SanitizeInput applies write-time input hygiene (WS-4.1):
//  1. NFC unicode normalization (prevents homoglyph/zero-width attacks)
//  2. Strip null bytes
//  3. Strip control characters except \n, \t, \r (preserves formatting)
//  4. Strip zero-width characters (U+200B..U+200F, U+FEFF, U+2060..U+2064)
//  5. Trim leading/trailing whitespace
func SanitizeInput(s string) string {
	if s == "" {
		return s
	}
	// 1. NFC normalize
	s = norm.NFC.String(s)
	// 2-4. Strip unwanted characters in a single pass
	s = strings.Map(func(r rune) rune {
		// Null byte
		if r == 0 {
			return -1
		}
		// Zero-width and invisible formatting characters
		if isZeroWidth(r) {
			return -1
		}
		// Control characters (keep \n, \t, \r)
		if unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r' {
			return -1
		}
		return r
	}, s)
	// 5. Trim whitespace
	s = strings.TrimSpace(s)
	return s
}

// SanitizeMap applies SanitizeInput to all string values in a metadata map.
// Non-string values and nested maps are left unchanged.
func SanitizeMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	for k, v := range m {
		if s, ok := v.(string); ok {
			m[k] = SanitizeInput(s)
		}
	}
	return m
}

// SanitizeTags applies SanitizeInput to each element in a string slice.
func SanitizeTags(tags []string) []string {
	if tags == nil {
		return nil
	}
	for i, t := range tags {
		tags[i] = SanitizeInput(t)
	}
	return tags
}

// isZeroWidth returns true for zero-width and invisible formatting characters.
func isZeroWidth(r rune) bool {
	switch r {
	case '\u200B', // zero-width space
		'\u200C', // zero-width non-joiner
		'\u200D', // zero-width joiner
		'\u200E', // left-to-right mark
		'\u200F', // right-to-left mark
		'\uFEFF', // byte order mark / zero-width no-break space
		'\u2060', // word joiner
		'\u2061', // function application
		'\u2062', // invisible times
		'\u2063', // invisible separator
		'\u2064': // invisible plus
		return true
	}
	return false
}

// EscapeLike escapes SQL LIKE special characters (%, _) in a search string
// using the given escape character. The caller must add ESCAPE clause to the query.
func EscapeLike(s string, escape rune) string {
	esc := string(escape)
	s = strings.ReplaceAll(s, esc, esc+esc)
	s = strings.ReplaceAll(s, "%", esc+"%")
	s = strings.ReplaceAll(s, "_", esc+"_")
	return s
}

// containsNullByte checks for null bytes in a string.
func containsNullByte(s string) bool {
	return strings.ContainsRune(s, '\x00')
}
