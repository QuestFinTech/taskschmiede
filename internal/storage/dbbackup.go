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

import (
	"fmt"
	"os"
)

// BackupTo creates a consistent backup of the database using VACUUM INTO.
// The destination path must not already exist. Returns the backup file size in bytes.
// Safe to call on a live database with WAL mode enabled (requires SQLite 3.27+).
func (db *DB) BackupTo(destPath string) (int64, error) {
	_, err := db.Exec(`VACUUM INTO ?`, destPath)
	if err != nil {
		return 0, fmt.Errorf("vacuum into %s: %w", destPath, err)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return 0, fmt.Errorf("stat backup %s: %w", destPath, err)
	}

	return info.Size(), nil
}
