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


package i18n

import (
	"testing"
)

func TestNew(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if !b.HasLanguage("en") {
		t.Fatal("expected English to be loaded")
	}
}

func TestT_ExistingKey(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	got := b.T("en", "common.save")
	if got != "Save" {
		t.Errorf("T(en, common.save) = %q, want %q", got, "Save")
	}
}

func TestT_MissingKeyReturnsRaw(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	got := b.T("en", "nonexistent.key")
	if got != "nonexistent.key" {
		t.Errorf("T(en, nonexistent.key) = %q, want raw key", got)
	}
}

func TestT_FallbackToEnglish(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	// Unknown language should fall back to English
	got := b.T("xx", "common.save")
	if got != "Save" {
		t.Errorf("T(xx, common.save) = %q, want English fallback %q", got, "Save")
	}
}

func TestT_WithArgs(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	got := b.T("en", "common.total", 42)
	if got != "42 total" {
		t.Errorf("T(en, common.total, 42) = %q, want %q", got, "42 total")
	}
}

func TestT_ShowingRange(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	got := b.T("en", "common.showing_range", 1, 10, 50)
	if got != "Showing 1 - 10 of 50" {
		t.Errorf("T(en, common.showing_range, 1,10,50) = %q, want %q", got, "Showing 1 - 10 of 50")
	}
}

func TestHasLanguage(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if !b.HasLanguage("en") {
		t.Error("expected HasLanguage(en) = true")
	}
	if b.HasLanguage("xx") {
		t.Error("expected HasLanguage(xx) = false")
	}
}

func TestLanguages(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	langs := b.Languages()
	if len(langs) < 1 {
		t.Fatal("Languages() returned 0, want at least 1")
	}
	// Languages are sorted by Code; verify English is present
	found := false
	for _, l := range langs {
		if l.Code == "en" && l.NativeName == "English" {
			found = true
		}
	}
	if !found {
		t.Error("Languages() does not contain English")
	}
}

func TestDir(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if got := b.Dir("en"); got != "ltr" {
		t.Errorf("Dir(en) = %q, want %q", got, "ltr")
	}
	// Unknown language defaults to ltr
	if got := b.Dir("xx"); got != "ltr" {
		t.Errorf("Dir(xx) = %q, want %q", got, "ltr")
	}
}

func TestMatchAcceptLanguage(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	tests := []struct {
		header string
		want   string
	}{
		{"", ""},
		{"en", "en"},
		{"en-US", "en"},
		{"en-US,en;q=0.9", "en"},
		{"xx,en;q=0.5", "en"},
		{"xx", ""},
		{"de-DE,de;q=0.9,en;q=0.8", "de"}, // de is loaded, matches first
		{"fr-FR,fr;q=0.9,en;q=0.8", "fr"}, // fr is loaded, matches first
		{"es-MX,es;q=0.9", "es"},           // es is loaded
	}

	for _, tt := range tests {
		got := b.MatchAcceptLanguage(tt.header)
		if got != tt.want {
			t.Errorf("MatchAcceptLanguage(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestMissingKeys(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	// English is the fallback, so MissingKeys should return nil
	if keys := b.MissingKeys("en"); keys != nil {
		t.Errorf("MissingKeys(en) = %v, want nil", keys)
	}
	// Unknown language returns nil (not loaded)
	if keys := b.MissingKeys("xx"); keys != nil {
		t.Errorf("MissingKeys(xx) = %v, want nil", keys)
	}
}

func TestTp_Singular(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	got := b.Tp("en", "portal.dashboard.unread_messages_count", 1)
	if got != "You have 1 unread message." {
		t.Errorf("Tp(en, unread, 1) = %q, want %q", got, "You have 1 unread message.")
	}
}

func TestTp_Plural(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	got := b.Tp("en", "portal.dashboard.unread_messages_count", 5)
	if got != "You have 5 unread messages." {
		t.Errorf("Tp(en, unread, 5) = %q, want %q", got, "You have 5 unread messages.")
	}
}

func TestTp_Zero(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	// Zero uses .other form
	got := b.Tp("en", "portal.dashboard.unread_messages_count", 0)
	if got != "You have 0 unread messages." {
		t.Errorf("Tp(en, unread, 0) = %q, want %q", got, "You have 0 unread messages.")
	}
}

func TestTp_Fallback(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	// Non-existent plural key falls back to raw key
	got := b.Tp("en", "nonexistent.plural", 2)
	if got != "nonexistent.plural.other" {
		t.Errorf("Tp(en, nonexistent, 2) = %q, want %q", got, "nonexistent.plural.other")
	}
}
