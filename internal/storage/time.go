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


package storage

import "time"

// SQLite datetime format produced by datetime('now').
const sqliteDatetimeFormat = "2006-01-02 15:04:05"

// UTCNow returns the current time in UTC.
//
// All timestamps in Taskschmiede use UTC internally. Do not use
// time.Now() directly -- use this function or time.Now().UTC().
// The lint-utc Makefile target enforces this policy.
func UTCNow() time.Time {
	return time.Now().UTC()
}

// ParseDBTime parses a timestamp string from the database.
// Handles both RFC3339 (from Go-formatted inserts) and SQLite's
// datetime format (from DEFAULT expressions and datetime() calls).
// Returns zero time if the string cannot be parsed.
func ParseDBTime(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	t, _ := time.Parse(sqliteDatetimeFormat, s)
	return t
}
