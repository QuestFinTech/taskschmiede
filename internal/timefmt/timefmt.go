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


// Package timefmt provides shared timestamp formatting for web templates.
package timefmt

import (
	"fmt"
	"time"
)

// Timestamp layout constants used by the formatting functions.
const (
	dateTimeLayout = "2 Jan 2006, 15:04"
	dateLayout     = "2 Jan 2006"
	timeLayout     = "15:04"
)

// FormatDateTime formats a timestamp as "2 Jan 2006, 15:04" in the given timezone.
// Accepts time.Time or string (RFC3339, ISO 8601, Go time.String()).
// Returns empty string for zero/empty values.
func FormatDateTime(ts interface{}, tz string) string {
	t, ok := parseTS(ts)
	if !ok {
		return ""
	}
	return t.In(loadLocation(tz)).Format(dateTimeLayout)
}

// FormatDate formats a timestamp as "2 Jan 2006" in the given timezone.
func FormatDate(ts interface{}, tz string) string {
	t, ok := parseTS(ts)
	if !ok {
		return ""
	}
	return t.In(loadLocation(tz)).Format(dateLayout)
}

// FormatTime formats a timestamp as "15:04" in the given timezone.
func FormatTime(ts interface{}, tz string) string {
	t, ok := parseTS(ts)
	if !ok {
		return ""
	}
	return t.In(loadLocation(tz)).Format(timeLayout)
}

func loadLocation(tz string) *time.Location {
	if tz == "" || tz == "UTC" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

func parseTS(ts interface{}) (time.Time, bool) {
	switch v := ts.(type) {
	case time.Time:
		if v.IsZero() {
			return v, false
		}
		return v, true
	case *time.Time:
		if v == nil || v.IsZero() {
			return time.Time{}, false
		}
		return *v, true
	case string:
		if v == "" {
			return time.Time{}, false
		}
		return parseString(v)
	case fmt.Stringer:
		s := v.String()
		if s == "" {
			return time.Time{}, false
		}
		return parseString(s)
	default:
		return time.Time{}, false
	}
}

func parseString(s string) (time.Time, bool) {
	// Try common formats in order of likelihood
	for _, layout := range []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05 +0000 UTC",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
