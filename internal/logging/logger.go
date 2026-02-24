// Package logging provides structured logging helpers for gh-cost-center.
//
// It wraps the standard library's log/slog with a console handler (stderr)
// and an optional rotating file handler.  SIGPIPE is handled gracefully so
// piped output (e.g. `gh cost-center list-users | head`) does not produce
// noisy error messages.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

// Options controls the behaviour of the logger returned by New.
type Options struct {
	// Level is the minimum log level. Defaults to INFO.
	Level slog.Level
	// FilePath is the optional path for a log file.  When set, a second
	// handler writes DEBUG-level logs to this file.  The parent directory
	// is created automatically.
	FilePath string
}

// New creates a new slog.Logger with a console handler (stderr) and, if
// Options.FilePath is set, an additional file handler.  It also installs a
// SIGPIPE handler to exit cleanly when the output pipe is closed.
func New(opts Options) (*slog.Logger, error) {
	installSIGPIPEHandler()

	// Console handler (stderr) at the configured level.
	consoleHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: opts.Level,
	})

	if opts.FilePath == "" {
		return slog.New(consoleHandler), nil
	}

	// Ensure the log directory exists.
	dir := filepath.Dir(opts.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return slog.New(consoleHandler), nil // fall back to console-only
	}

	f, err := os.OpenFile(opts.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return slog.New(consoleHandler), nil // fall back to console-only
	}

	// File handler always logs at DEBUG for full diagnostic traces.
	fileHandler := slog.NewTextHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	return slog.New(newMultiHandler(consoleHandler, fileHandler)), nil
}

// ParseLevel converts a human-readable level string (e.g. "DEBUG", "info",
// "WARNING") into a slog.Level.
func ParseLevel(s string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO", "":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// installSIGPIPEHandler exits cleanly when the output pipe is closed (e.g.
// `gh cost-center list-users | head`).
func installSIGPIPEHandler() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGPIPE)
	go func() {
		<-ch
		os.Exit(0)
	}()
}

// multiHandler fans out log records to multiple slog.Handler implementations.
type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(handlers ...slog.Handler) *multiHandler {
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) Enabled(_ context.Context, level slog.Level) bool {
	for _, hh := range h.handlers {
		if hh.Enabled(context.Background(), level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, hh := range h.handlers {
		if hh.Enabled(ctx, r.Level) {
			_ = hh.Handle(ctx, r)
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, hh := range h.handlers {
		handlers[i] = hh.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, hh := range h.handlers {
		handlers[i] = hh.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

// Ensure multiHandler satisfies the slog.Handler interface at compile time.
var _ slog.Handler = (*multiHandler)(nil)

// Discard is a convenience writer that discards all output (used in tests).
var Discard io.Writer = io.Discard
