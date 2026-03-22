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
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// HarmScore represents the result of heuristic injection analysis.
type HarmScore struct {
	Score   int      `json:"harm_score"`   // 0-100
	Signals []string `json:"harm_signals"` // which pattern categories matched
}

// injectionPattern defines a single detection pattern.
type injectionPattern struct {
	Name     string
	Category string
	Re       *regexp.Regexp
	Weight   int
}

// BuiltinPattern exposes a builtin pattern's metadata for admin display.
type BuiltinPattern struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Pattern  string `json:"pattern"`
	Weight   int    `json:"weight"`
}

// CustomPattern defines an admin-added scoring pattern.
type CustomPattern struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Pattern  string `json:"pattern"`
	Weight   int    `json:"weight"`
}

// PatternOverrides holds system-level overrides for the scoring engine.
type PatternOverrides struct {
	Disabled        map[string]bool `json:"disabled,omitempty"`
	WeightOverrides map[string]int  `json:"weight_overrides,omitempty"`
	Added           []CustomPattern `json:"added,omitempty"`
	compiled        []injectionPattern
}

var (
	globalOverrides   *PatternOverrides
	globalOverridesMu sync.RWMutex
)

// SetPatternOverrides compiles custom patterns and stores the overrides.
// Safe for concurrent use.
func SetPatternOverrides(o *PatternOverrides) {
	if o == nil {
		globalOverridesMu.Lock()
		globalOverrides = nil
		globalOverridesMu.Unlock()
		return
	}

	// Pre-compile custom patterns.
	compiled := make([]injectionPattern, 0, len(o.Added))
	for _, cp := range o.Added {
		re, err := regexp.Compile("(?i)" + cp.Pattern)
		if err != nil {
			continue // skip invalid patterns (validated at save time)
		}
		compiled = append(compiled, injectionPattern{
			Name:     cp.Name,
			Category: cp.Category,
			Re:       re,
			Weight:   cp.Weight,
		})
	}
	o.compiled = compiled

	globalOverridesMu.Lock()
	globalOverrides = o
	globalOverridesMu.Unlock()
}

// ListBuiltinPatterns returns metadata for all hardcoded scoring patterns.
func ListBuiltinPatterns() []BuiltinPattern {
	result := make([]BuiltinPattern, len(builtinRaw))
	for i, r := range builtinRaw {
		result[i] = BuiltinPattern(r)
	}
	return result
}

// ValidateCustomPattern checks that a custom pattern is valid.
func ValidateCustomPattern(p CustomPattern) error {
	if p.Name == "" {
		return fmt.Errorf("pattern name is required")
	}
	if len(p.Name) > 100 {
		return fmt.Errorf("pattern name too long (max 100)")
	}
	if p.Category == "" {
		return fmt.Errorf("pattern category is required")
	}
	if p.Pattern == "" {
		return fmt.Errorf("pattern regex is required")
	}
	if _, err := regexp.Compile("(?i)" + p.Pattern); err != nil {
		return fmt.Errorf("invalid regex: %w", err)
	}
	if p.Weight < 1 || p.Weight > 25 {
		return fmt.Errorf("weight must be between 1 and 25")
	}
	return nil
}

// Pattern categories used by the injection scoring engine.
const (
	catDirectOverride = "direct_override"
	catRolePlay       = "role_play"
	catSystemExtract  = "system_extract"
	catEncoding       = "encoding_trick"
	catSocialEng      = "social_engineering"
	catExfiltration   = "exfiltration"
	catDelimiter      = "delimiter_injection"
)

// builtinRaw holds the raw pattern definitions for admin display via ListBuiltinPatterns.
// injectionPatterns is the compiled version used at runtime.
//
// Weights are tuned so that:
//   - A single low-confidence match scores 5-10 (noise floor)
//   - Multiple matches from the same category stack to ~30-40
//   - Cross-category matches push toward 50-80
//   - A deliberate injection payload typically scores 60+
var builtinRaw = []rawPattern{
	// --- Direct instruction override ---
	{"ignore_previous", catDirectOverride, `ignore\s+(all\s+)?(previous|prior|above|earlier)\s+(instructions?|rules?|prompts?|guidelines?)`, 20},
	{"disregard_prior", catDirectOverride, `disregard\s+(all\s+)?(previous|prior|above|earlier)\s+(instructions?|rules?|prompts?|context)`, 20},
	{"forget_instructions", catDirectOverride, `forget\s+(all\s+)?(your\s+)?(previous|prior|instructions?|rules?|guidelines?)`, 18},
	{"override_instructions", catDirectOverride, `override\s+(all\s+)?(your\s+)?(previous|prior|instructions?|rules?|guidelines?)`, 18},
	{"new_instructions", catDirectOverride, `(new|updated|revised|actual)\s+(system\s+)?(instructions?|prompt|rules?|guidelines?)(\s+are)?:`, 15},
	{"do_not_follow", catDirectOverride, `do\s+not\s+follow\s+(your\s+)?(previous|prior|original)\s+(instructions?|rules?)`, 20},
	{"stop_being", catDirectOverride, `stop\s+being\s+(a|an|the)\s+\w+\s+(assistant|agent|ai|bot)`, 15},
	{"from_now_on", catDirectOverride, `from\s+now\s+on\s+(you\s+)?(will|must|should|are)`, 10},
	{"important_override", catDirectOverride, `\[?\s*(important|critical|urgent)\s*[\]:]?\s*(new\s+)?(instructions?|rules?|override)`, 15},

	// --- Role-play exploits ---
	{"you_are_now", catRolePlay, `you\s+are\s+now\s+(a|an|the|my)\s+`, 12},
	{"act_as", catRolePlay, `act\s+(as|like)\s+(a|an|the|my)\s+`, 10},
	{"pretend_to_be", catRolePlay, `pretend\s+(to\s+be|you\s+are)\s+`, 12},
	{"developer_mode", catRolePlay, `(enter|enable|activate|switch\s+to)\s+(developer|admin|debug|god|sudo|root)\s+mode`, 20},
	{"jailbreak", catRolePlay, `(jailbreak|jail\s+break|dan\s+mode|do\s+anything\s+now)`, 25},
	{"roleplay_persona", catRolePlay, `(imagine|suppose|assume)\s+you\s+are\s+(a|an|the|not)\s+`, 8},
	{"bypass_restrictions", catRolePlay, `(bypass|circumvent|evade|disable|remove)\s+(your\s+)?(restrictions?|limitations?|filters?|safeguards?|guardrails?)`, 20},

	// --- System prompt extraction ---
	{"repeat_instructions", catSystemExtract, `(repeat|show|display|print|output|reveal|tell\s+me)\s+(me\s+)?(your\s+)?(system\s+)?(instructions?|prompt|rules?|guidelines?)`, 15},
	{"what_are_rules", catSystemExtract, `what\s+(are|were)\s+your\s+(system\s+)?(instructions?|rules?|guidelines?|prompt)`, 12},
	{"beginning_of_conversation", catSystemExtract, `(beginning|start)\s+of\s+(the\s+)?(conversation|prompt|context|session)`, 8},
	{"system_prompt_ref", catSystemExtract, `(system\s+prompt|system\s+message|initial\s+prompt|initial\s+instructions?)`, 8},
	{"hidden_prompt", catSystemExtract, `(hidden|secret|internal)\s+(prompt|instructions?|rules?|context)`, 10},

	// --- Encoding tricks ---
	{"base64_block", catEncoding, `[a-zA-Z0-9+/]{40,}={0,2}`, 5},
	{"hex_encoded", catEncoding, `(\\x[0-9a-f]{2}){8,}`, 8},
	{"unicode_escape", catEncoding, `(\\u[0-9a-f]{4}){4,}`, 8},
	{"rot13_reference", catEncoding, `\b(rot13|rot-13|caesar\s+cipher)\b`, 10},
	{"encoded_instruction", catEncoding, `(decode|decrypt|deobfuscate|base64)\s+(the\s+)?(following|this|above|below)`, 12},

	// --- Social engineering ---
	{"urgent_comply", catSocialEng, `(urgent|emergency|critical)[:\s]+(you\s+)?(must|need\s+to|have\s+to)\s+(comply|obey|follow|execute)`, 15},
	{"developers_said", catSocialEng, `(the\s+)?(developers?|creators?|admins?|operators?|owners?)\s+(said|told|instructed|want|asked)\s+(you\s+to|that)`, 12},
	{"authorized_by", catSocialEng, `(authorized|approved|sanctioned|permitted)\s+by\s+(the\s+)?(admin|developer|creator|owner|system)`, 10},
	{"trust_me", catSocialEng, `(trust\s+me|i\s+am\s+(the|a|an)\s+(admin|developer|owner|root|superuser))`, 12},
	{"security_test", catSocialEng, `this\s+is\s+(a|an)\s+(security|penetration|pen)\s+(test|audit|exercise)`, 8},
	{"must_comply", catSocialEng, `you\s+(must|have\s+to|need\s+to|are\s+required\s+to)\s+(comply|obey|follow|execute|do\s+as)`, 10},

	// --- Data exfiltration ---
	{"send_to_url", catExfiltration, `(send|post|forward|transmit|upload|exfiltrate)\s+.{0,50}?\s+to\s+(https?://|a\s+url|a\s+server|a\s+webhook)`, 18},
	{"curl_fetch", catExfiltration, `\b(curl|wget|fetch|http\.get|http\.post)\s+(https?://|['\"])`, 12},
	{"webhook_ref", catExfiltration, `\bwebhook[s]?\s*(url|endpoint|address)?\s*[:=]?\s*https?://`, 15},
	{"eval_exec", catExfiltration, `\b(eval|exec|execute|run|system)\s*\(\s*['"]`, 15},

	// --- Delimiter injection ---
	{"xml_tag_inject", catDelimiter, `<\s*/?\s*(system|instructions?|prompt|rules?|assistant|user|human|tool_result)\s*>`, 15},
	{"markdown_delimiter", catDelimiter, `---\s*(system|instructions?|prompt|new\s+context)\s*---`, 12},
	{"triple_backtick_inject", catDelimiter, "```\\s*(system|instructions?|prompt|override)", 12},
	{"bracket_inject", catDelimiter, `\[\[?\s*(system|instructions?|prompt|override|INST)\s*\]?\]`, 12},
	{"end_of_prompt", catDelimiter, `(end\s+of\s+(system\s+)?(prompt|instructions?|context)|<\|end\|>|<\|im_end\|>)`, 15},
}

var injectionPatterns = compilePatterns(builtinRaw)

// rawPattern is used during initialization before compilation.
type rawPattern struct {
	Name     string
	Category string
	Pattern  string
	Weight   int
}

// compilePatterns compiles raw pattern definitions into injectionPatterns.
func compilePatterns(raw []rawPattern) []injectionPattern {
	patterns := make([]injectionPattern, 0, len(raw))
	for _, r := range raw {
		re, err := regexp.Compile("(?i)" + r.Pattern)
		if err != nil {
			// Programming error: panic at init time so tests catch it.
			panic("invalid injection pattern " + r.Name + ": " + err.Error())
		}
		patterns = append(patterns, injectionPattern{
			Name:     r.Name,
			Category: r.Category,
			Re:       re,
			Weight:   r.Weight,
		})
	}
	return patterns
}

// ScoreContent runs heuristic injection detection on the given text.
// Returns a HarmScore with a score (0-100) and the list of matched signal categories.
// A score of 0 means no injection signals detected. The score is advisory and
// must never be used to block writes.
func ScoreContent(text string) HarmScore {
	if text == "" {
		return HarmScore{}
	}

	normalized := normalizeForScoring(text)
	if len(normalized) == 0 {
		return HarmScore{}
	}

	globalOverridesMu.RLock()
	overrides := globalOverrides
	globalOverridesMu.RUnlock()

	var score int
	seen := make(map[string]bool)

	// Run builtin patterns with override checks.
	for i := range injectionPatterns {
		p := &injectionPatterns[i]

		if overrides != nil {
			if overrides.Disabled[p.Name] {
				continue
			}
		}

		if p.Re.MatchString(normalized) {
			w := p.Weight
			if overrides != nil {
				if ow, ok := overrides.WeightOverrides[p.Name]; ok {
					w = ow
				}
			}
			score += w
			if !seen[p.Category] {
				seen[p.Category] = true
			}
		}
	}

	// Run compiled custom patterns from overrides.
	if overrides != nil {
		for i := range overrides.compiled {
			cp := &overrides.compiled[i]
			if cp.Re.MatchString(normalized) {
				score += cp.Weight
				if !seen[cp.Category] {
					seen[cp.Category] = true
				}
			}
		}
	}

	if score > 100 {
		score = 100
	}

	signals := make([]string, 0, len(seen))
	for cat := range seen {
		signals = append(signals, cat)
	}

	return HarmScore{
		Score:   score,
		Signals: signals,
	}
}

// normalizeForScoring prepares text for pattern matching:
//   - Collapse multiple whitespace to single space
//   - Strip control characters (keep spaces and newlines as spaces)
//   - Trim leading/trailing whitespace
//
// Note: patterns use (?i) flag for case-insensitive matching,
// so we don't lowercase here.
func normalizeForScoring(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return strings.TrimSpace(b.String())
}

// ---------------------------------------------------------------------------
// Encoded content pre-processing for LLM scoring
// ---------------------------------------------------------------------------

// base64Re matches base64-encoded blocks (40+ chars, optional padding).
// Anchored to word boundaries to avoid matching normal text.
var base64Re = regexp.MustCompile(`[A-Za-z0-9+/]{40,}={0,2}`)

// hexRe matches hex-encoded sequences (\xNN repeated 8+ times).
var hexRe = regexp.MustCompile(`(?i)(\\x[0-9a-f]{2}){8,}`)

// DecodedBlock represents a single decoded encoded section found in the text.
type DecodedBlock struct {
	Encoding string // "base64" or "hex"
	Encoded  string // the original encoded string
	Decoded  string // the decoded plaintext
}

// DecodeEncodedContent scans text for base64 and hex-encoded blocks, attempts
// to decode them, and returns any successfully decoded blocks. Only returns
// blocks that decode to valid UTF-8 text (filters out binary data).
func DecodeEncodedContent(text string) []DecodedBlock {
	var blocks []DecodedBlock

	// Base64 blocks.
	for _, match := range base64Re.FindAllString(text, 10) {
		decoded, err := base64.StdEncoding.DecodeString(match)
		if err != nil {
			// Try with padding.
			padded := match
			if m := len(padded) % 4; m != 0 {
				padded += strings.Repeat("=", 4-m)
			}
			decoded, err = base64.StdEncoding.DecodeString(padded)
			if err != nil {
				continue
			}
		}
		if len(decoded) > 0 && utf8.Valid(decoded) && isPrintableText(decoded) {
			blocks = append(blocks, DecodedBlock{
				Encoding: "base64",
				Encoded:  match,
				Decoded:  string(decoded),
			})
		}
	}

	// Hex-encoded sequences (\x41\x42...).
	for _, match := range hexRe.FindAllString(text, 10) {
		hexStr := strings.ReplaceAll(match, "\\x", "")
		hexStr = strings.ReplaceAll(hexStr, "\\X", "")
		decoded, err := hex.DecodeString(hexStr)
		if err != nil {
			continue
		}
		if len(decoded) > 0 && utf8.Valid(decoded) && isPrintableText(decoded) {
			blocks = append(blocks, DecodedBlock{
				Encoding: "hex",
				Encoded:  match,
				Decoded:  string(decoded),
			})
		}
	}

	return blocks
}

// PrepareTextForLLM takes entity text, detects encoded content, decodes it,
// and appends the decoded blocks so the LLM can classify the actual content.
// Returns the original text unmodified if no encoded content is found.
func PrepareTextForLLM(text string) string {
	blocks := DecodeEncodedContent(text)
	if len(blocks) == 0 {
		return text
	}

	var sb strings.Builder
	sb.WriteString(text)
	sb.WriteString("\n\n--- Decoded content (pre-processed by Layer 1) ---")
	for _, b := range blocks {
		fmt.Fprintf(&sb, "\n[%s] %s", b.Encoding, b.Decoded)
	}
	return sb.String()
}

// isPrintableText checks that decoded bytes are mostly printable text,
// not binary data. Returns true if at least 80% of runes are printable.
func isPrintableText(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	printable := 0
	total := 0
	for _, r := range string(data) {
		total++
		if unicode.IsPrint(r) || unicode.IsSpace(r) {
			printable++
		}
	}
	if total == 0 {
		return false
	}
	return float64(printable)/float64(total) >= 0.8
}
