package logging

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"debug", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"info", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"error", slog.LevelError},
		{"  info  ", slog.LevelInfo},
		{"UNKNOWN", slog.LevelInfo},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := ParseLevel(tt.input)
			if got != tt.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNew_ConsoleOnly(t *testing.T) {
	t.Parallel()
	logger, err := New(Options{Level: slog.LevelWarn})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if logger == nil {
		t.Fatal("New() returned nil logger")
	}
	if logger.Handler().Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected INFO to be disabled at WARN level")
	}
	if !logger.Handler().Enabled(context.Background(), slog.LevelWarn) {
		t.Error("expected WARN to be enabled at WARN level")
	}
}

func TestNew_WithFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sub", "test.log")
	logger, err := New(Options{Level: slog.LevelInfo, FilePath: logPath})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	logger.Info("hello info", "key", "val")
	logger.Debug("hello debug", "key", "val")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "hello info") {
		t.Errorf("log file missing INFO message; got:\n%s", content)
	}
	if !strings.Contains(content, "hello debug") {
		t.Errorf("log file missing DEBUG message; got:\n%s", content)
	}
}

func TestNew_BadFilePath_FallsBack(t *testing.T) {
	t.Parallel()
	logger, err := New(Options{Level: slog.LevelInfo, FilePath: "/dev/null/\x00/impossible.log"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if logger == nil {
		t.Fatal("expected fallback logger, got nil")
	}
}

func TestMultiHandler_Enabled(t *testing.T) {
	t.Parallel()
	warnH := slog.NewTextHandler(Discard, &slog.HandlerOptions{Level: slog.LevelWarn})
	debugH := slog.NewTextHandler(Discard, &slog.HandlerOptions{Level: slog.LevelDebug})
	mh := newMultiHandler(warnH, debugH)
	if !mh.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("multiHandler should enable DEBUG when one handler accepts it")
	}
	if !mh.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("multiHandler should enable WARN")
	}
}

func TestMultiHandler_Handle_FanOut(t *testing.T) {
	t.Parallel()
	var buf1, buf2 bytes.Buffer
	h1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelDebug})
	h2 := slog.NewTextHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelDebug})
	mh := newMultiHandler(h1, h2)
	logger := slog.New(mh)
	logger.Info("test msg", "k", "v")
	if !strings.Contains(buf1.String(), "test msg") {
		t.Errorf("handler 1 missing message; got: %s", buf1.String())
	}
	if !strings.Contains(buf2.String(), "test msg") {
		t.Errorf("handler 2 missing message; got: %s", buf2.String())
	}
}

func TestMultiHandler_Handle_LevelFiltering(t *testing.T) {
	t.Parallel()
	var warnBuf, debugBuf bytes.Buffer
	warnH := slog.NewTextHandler(&warnBuf, &slog.HandlerOptions{Level: slog.LevelWarn})
	debugH := slog.NewTextHandler(&debugBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	mh := newMultiHandler(warnH, debugH)
	logger := slog.New(mh)
	logger.Info("info msg")
	if strings.Contains(warnBuf.String(), "info msg") {
		t.Error("WARN handler should not receive INFO messages")
	}
	if !strings.Contains(debugBuf.String(), "info msg") {
		t.Error("DEBUG handler should receive INFO messages")
	}
}

func TestMultiHandler_WithAttrs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	mh := newMultiHandler(h)
	mhWithAttrs := mh.WithAttrs([]slog.Attr{slog.String("component", "test")})
	logger := slog.New(mhWithAttrs)
	logger.Info("with attrs")
	if !strings.Contains(buf.String(), "component=test") {
		t.Errorf("expected attrs in output; got: %s", buf.String())
	}
}

func TestMultiHandler_WithGroup(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	mh := newMultiHandler(h)
	mhGrouped := mh.WithGroup("mygroup")
	logger := slog.New(mhGrouped)
	logger.Info("grouped msg", "field", "value")
	if !strings.Contains(buf.String(), "mygroup.field=value") {
		t.Errorf("expected grouped key in output; got: %s", buf.String())
	}
}

func TestDiscard(t *testing.T) {
	t.Parallel()
	n, err := Discard.Write([]byte("should be discarded"))
	if err != nil {
		t.Errorf("Discard.Write error: %v", err)
	}
	if n != 19 {
		t.Errorf("Discard.Write returned %d, want 19", n)
	}
}
