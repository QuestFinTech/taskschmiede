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


// Package i18n provides internationalization support for Taskschmiede's
// web UIs. It loads JSON locale files embedded at compile time and exposes
// a template function for string lookup with fallback to English.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

// localeFS holds embedded JSON locale files compiled into the binary.
//
//go:embed locales/*.json
var localeFS embed.FS

// Language holds display metadata for a loaded locale.
type Language struct {
	Code       string // "de"
	Name       string // "German"
	NativeName string // "Deutsch"
	Dir        string // "ltr" or "rtl"
}

// Bundle holds all loaded translations keyed by language code.
type Bundle struct {
	langs    map[string]map[string]string // lang -> key -> value
	meta     map[string]Language          // lang -> metadata
	fallback string                       // default language code
}

// New loads all locale files from the embedded filesystem.
// It expects locales/*.json files and a _meta.json with language metadata.
func New() (*Bundle, error) {
	b := &Bundle{
		langs:    make(map[string]map[string]string),
		meta:     make(map[string]Language),
		fallback: "en",
	}

	// Load metadata
	metaData, err := localeFS.ReadFile("locales/_meta.json")
	if err != nil {
		return nil, fmt.Errorf("i18n: failed to read _meta.json: %w", err)
	}

	var rawMeta map[string]struct {
		Name       string `json:"name"`
		NativeName string `json:"native_name"`
		Dir        string `json:"dir"`
	}
	if err := json.Unmarshal(metaData, &rawMeta); err != nil {
		return nil, fmt.Errorf("i18n: failed to parse _meta.json: %w", err)
	}

	for code, m := range rawMeta {
		dir := m.Dir
		if dir == "" {
			dir = "ltr"
		}
		b.meta[code] = Language{
			Code:       code,
			Name:       m.Name,
			NativeName: m.NativeName,
			Dir:        dir,
		}
	}

	// Load locale files
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return nil, fmt.Errorf("i18n: failed to read locales directory: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") || name == "_meta.json" {
			continue
		}
		code := strings.TrimSuffix(name, ".json")

		data, err := localeFS.ReadFile("locales/" + name)
		if err != nil {
			return nil, fmt.Errorf("i18n: failed to read %s: %w", name, err)
		}

		var translations map[string]string
		if err := json.Unmarshal(data, &translations); err != nil {
			return nil, fmt.Errorf("i18n: failed to parse %s: %w", name, err)
		}

		b.langs[code] = translations
		slog.Info("i18n: loaded locale", "lang", code, "keys", len(translations))
	}

	// Verify fallback language exists
	if _, ok := b.langs[b.fallback]; !ok {
		return nil, fmt.Errorf("i18n: fallback language %q not found", b.fallback)
	}

	// Log missing key warnings
	b.logMissingKeys()

	return b, nil
}

// T returns the translated string for the given language and key.
// Falls back to English if the key is missing in the requested language.
// Falls back to the raw key if also missing in English.
// Extra args are passed to fmt.Sprintf for parameterized strings.
func (b *Bundle) T(lang, key string, args ...interface{}) string {
	// Try requested language
	if translations, ok := b.langs[lang]; ok {
		if val, ok := translations[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(val, args...)
			}
			return val
		}
	}

	// Fallback to English
	if lang != b.fallback {
		if translations, ok := b.langs[b.fallback]; ok {
			if val, ok := translations[key]; ok {
				if len(args) > 0 {
					return fmt.Sprintf(val, args...)
				}
				return val
			}
		}
	}

	// Return the raw key (makes missing translations visible)
	return key
}

// Tp returns a plural-aware translated string. It selects between
// "key.one" and "key.other" based on the count value, then formats
// the chosen string with the count. For languages not loaded or keys
// not found, it falls back to the standard T behavior.
//
// Usage in templates: {{tp .Lang "messages.unread" 5}}
// Requires keys: "messages.unread.one" = "You have %d unread message"
//                "messages.unread.other" = "You have %d unread messages"
func (b *Bundle) Tp(lang, key string, count int) string {
	suffix := ".other"
	if count == 1 {
		suffix = ".one"
	}
	return b.T(lang, key+suffix, count)
}

// HasLanguage returns true if the given language code is loaded.
func (b *Bundle) HasLanguage(code string) bool {
	_, ok := b.langs[code]
	return ok
}

// Languages returns metadata for all loaded languages, sorted by code.
func (b *Bundle) Languages() []Language {
	var langs []Language
	for code := range b.langs {
		if m, ok := b.meta[code]; ok {
			langs = append(langs, m)
		} else {
			langs = append(langs, Language{Code: code, Name: code, NativeName: code, Dir: "ltr"})
		}
	}
	sort.Slice(langs, func(i, j int) bool {
		return langs[i].Code < langs[j].Code
	})
	return langs
}

// Dir returns the text direction for the given language ("ltr" or "rtl").
func (b *Bundle) Dir(lang string) string {
	if m, ok := b.meta[lang]; ok {
		return m.Dir
	}
	return "ltr"
}

// MatchAcceptLanguage parses an Accept-Language header value and returns
// the best matching loaded language code, or "" if none match.
// Supports simple matching (exact and prefix). Does not implement full
// RFC 4647 matching -- sufficient for the target language set.
func (b *Bundle) MatchAcceptLanguage(header string) string {
	if header == "" {
		return ""
	}

	type langQ struct {
		code string
		q    float64
	}

	var prefs []langQ
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		code := part
		q := 1.0

		if idx := strings.Index(part, ";"); idx >= 0 {
			code = strings.TrimSpace(part[:idx])
			qPart := strings.TrimSpace(part[idx+1:])
			if strings.HasPrefix(qPart, "q=") {
				if _, err := fmt.Sscanf(qPart, "q=%f", &q); err != nil {
					q = 0
				}
			}
		}

		prefs = append(prefs, langQ{code: strings.ToLower(code), q: q})
	}

	// Sort by quality descending
	sort.Slice(prefs, func(i, j int) bool {
		return prefs[i].q > prefs[j].q
	})

	// Match against loaded languages
	for _, p := range prefs {
		// Exact match
		if b.HasLanguage(p.code) {
			return p.code
		}
		// Prefix match (e.g., "de-DE" matches "de")
		if idx := strings.Index(p.code, "-"); idx >= 0 {
			prefix := p.code[:idx]
			if b.HasLanguage(prefix) {
				return prefix
			}
		}
	}

	return ""
}

// MissingKeys returns keys present in the fallback language (English)
// but missing in the given language. Returns nil for the fallback language.
func (b *Bundle) MissingKeys(lang string) []string {
	if lang == b.fallback {
		return nil
	}

	target, ok := b.langs[lang]
	if !ok {
		return nil
	}

	fallback := b.langs[b.fallback]
	var missing []string
	for key := range fallback {
		if _, ok := target[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
}

// LangStats holds translation statistics for a single language.
type LangStats struct {
	Language   Language
	KeyCount   int
	Missing    int
	Coverage   float64 // 0.0 to 100.0
	MissingTop []string // first N missing keys (for display)
}

// Stats returns translation statistics for all loaded languages.
func (b *Bundle) Stats() []LangStats {
	baseCount := len(b.langs[b.fallback])
	var stats []LangStats

	for _, lang := range b.Languages() {
		count := len(b.langs[lang.Code])
		missing := b.MissingKeys(lang.Code)
		missingCount := len(missing)
		coverage := 100.0
		if baseCount > 0 && lang.Code != b.fallback {
			translated := 0
			for key := range b.langs[b.fallback] {
				if _, ok := b.langs[lang.Code][key]; ok {
					translated++
				}
			}
			coverage = float64(translated) / float64(baseCount) * 100
		}

		top := missing
		if len(top) > 20 {
			top = top[:20]
		}

		stats = append(stats, LangStats{
			Language:   lang,
			KeyCount:   count,
			Missing:    missingCount,
			Coverage:   coverage,
			MissingTop: top,
		})
	}

	return stats
}

// BaseKeyCount returns the number of keys in the fallback (English) locale.
func (b *Bundle) BaseKeyCount() int {
	return len(b.langs[b.fallback])
}

// logMissingKeys logs warnings for each non-English locale with missing keys.
func (b *Bundle) logMissingKeys() {
	for code := range b.langs {
		if code == b.fallback {
			continue
		}
		missing := b.MissingKeys(code)
		if len(missing) > 0 {
			slog.Warn("i18n: locale has missing keys",
				"lang", code,
				"missing", len(missing),
				"total", len(b.langs[b.fallback]),
			)
		}
	}
}
