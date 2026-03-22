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


package ticker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// BackupConfig holds configuration for the database backup handler.
type BackupConfig struct {
	DB         *storage.DB
	MsgDB      *storage.MessageDB
	DBPath     string // path to the main database file (for naming)
	MsgDBPath  string // path to the message database file (for naming)
	BackupDir  string // directory to store backups
	MaxBackups int    // number of backups to keep per database (default 7)
}

// NewBackupHandler returns a handler that creates daily database backups
// using VACUUM INTO and rotates old backups.
func NewBackupHandler(logger *slog.Logger, cfg BackupConfig) Handler {
	if cfg.MaxBackups <= 0 {
		cfg.MaxBackups = 7
	}
	return Handler{
		Name:     "db-backup",
		Interval: 24 * time.Hour,
		Fn:       dbBackup(logger, cfg),
	}
}

func dbBackup(logger *slog.Logger, cfg BackupConfig) func(context.Context, time.Time) error {
	return func(_ context.Context, _ time.Time) error {
		if err := os.MkdirAll(cfg.BackupDir, 0700); err != nil {
			return fmt.Errorf("create backup dir: %w", err)
		}

		now := storage.UTCNow()
		stamp := now.Format("20060102-150405")

		// Backup main database.
		mainName := filepath.Base(cfg.DBPath)
		mainDest := filepath.Join(cfg.BackupDir, fmt.Sprintf("%s_%s", stamp, mainName))
		mainSize, err := cfg.DB.BackupTo(mainDest)
		if err != nil {
			return fmt.Errorf("backup main db: %w", err)
		}
		logger.Info("Database backup created",
			"db", mainName,
			"path", mainDest,
			"size_bytes", mainSize,
		)

		// Backup message database (if configured).
		if cfg.MsgDB != nil && cfg.MsgDBPath != "" {
			msgName := filepath.Base(cfg.MsgDBPath)
			msgDest := filepath.Join(cfg.BackupDir, fmt.Sprintf("%s_%s", stamp, msgName))
			msgSize, err := cfg.MsgDB.BackupTo(msgDest)
			if err != nil {
				logger.Warn("Failed to backup message db",
					"db", msgName,
					"error", err,
				)
			} else {
				logger.Info("Database backup created",
					"db", msgName,
					"path", msgDest,
					"size_bytes", msgSize,
				)
			}
		}

		// Rotate old backups.
		rotateBackups(logger, cfg.BackupDir, filepath.Base(cfg.DBPath), cfg.MaxBackups)
		if cfg.MsgDBPath != "" {
			rotateBackups(logger, cfg.BackupDir, filepath.Base(cfg.MsgDBPath), cfg.MaxBackups)
		}

		return nil
	}
}

// rotateBackups keeps only the newest maxKeep backup files matching the given
// database name suffix. Files are sorted lexicographically (timestamp prefix
// ensures chronological order).
func rotateBackups(logger *slog.Logger, dir, dbName string, maxKeep int) {
	suffix := "_" + dbName
	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Warn("Failed to read backup dir for rotation", "dir", dir, "error", err)
		return
	}

	var matching []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), suffix) {
			matching = append(matching, e.Name())
		}
	}

	if len(matching) <= maxKeep {
		return
	}

	sort.Strings(matching)
	toDelete := matching[:len(matching)-maxKeep]

	for _, name := range toDelete {
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil {
			logger.Warn("Failed to remove old backup", "path", path, "error", err)
		} else {
			logger.Info("Old backup removed", "path", path)
		}
	}
}
