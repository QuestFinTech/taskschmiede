// Package logging provides shared structured logging for Taskschmiede binaries.
//
// Each binary calls SetupLogging with a LogConfig and module name to create
// a *slog.Logger that writes lines in the format:
//
//	2006-01-02 15:04:05 LEVEL [module] message key=value ...
package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// LogConfig holds logging settings shared across all Taskschmiede binaries.
type LogConfig struct {
	File  string `yaml:"file"`
	Level string `yaml:"level"`
}

// handler is a custom slog handler with a fixed-width level and module tag.
type handler struct {
	level  slog.Level
	writer *os.File
	module string
}

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *handler) Handle(_ context.Context, r slog.Record) error {
	timestamp := r.Time.Format("2006-01-02 15:04:05")
	level := strings.ToUpper(r.Level.String())

	var attrs strings.Builder
	r.Attrs(func(a slog.Attr) bool {
		if attrs.Len() > 0 {
			attrs.WriteString(" ")
		}
		attrs.WriteString(a.Key)
		attrs.WriteString("=")
		fmt.Fprintf(&attrs, "%v", a.Value.Any())
		return true
	})

	if attrs.Len() > 0 {
		_, _ = fmt.Fprintf(h.writer, "%s %-5s [%s] %s %s\n", timestamp, level, h.module, r.Message, attrs.String())
	} else {
		_, _ = fmt.Fprintf(h.writer, "%s %-5s [%s] %s\n", timestamp, level, h.module, r.Message)
	}
	return nil
}

func (h *handler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{level: h.level, writer: h.writer, module: name}
}

// SetupLogging creates a *slog.Logger that writes to the file (or stdout)
// described by cfg, tagged with the given module name.
//
// When a log file is opened, the caller receives it as the second return value
// and is responsible for closing it. A nil *os.File means stdout is used.
//
// The returned logger is also installed as slog.Default.
func SetupLogging(cfg LogConfig, module string) (*slog.Logger, *os.File, error) {
	var level slog.Level
	switch strings.ToUpper(cfg.Level) {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "WARN", "WARNING":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var writer *os.File
	var err error

	if cfg.File != "" && cfg.File != "-" {
		logDir := filepath.Dir(cfg.File)
		if err := os.MkdirAll(logDir, 0750); err != nil {
			return nil, nil, fmt.Errorf("create log directory: %w", err)
		}
		writer, err = os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file: %w", err)
		}
	} else {
		writer = os.Stdout
	}

	h := &handler{
		level:  level,
		writer: writer,
		module: module,
	}
	logger := slog.New(h)
	slog.SetDefault(logger)

	if writer != os.Stdout {
		return logger, writer, nil
	}
	return logger, nil, nil
}
