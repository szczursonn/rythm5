package logfile_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/szczursonn/rythm5/internal/logging/logfile"
)

// ⚠️⚠️⚠️ AI-generated ⚠️⚠️⚠️

func TestNewBufferedLogFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "test.log")
	w, err := logfile.NewBufferedLogFile(path, 4096, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("unexpected Close error: %v", err)
	}
}

func TestNewBufferedLogFile_InvalidPath(t *testing.T) {
	t.Parallel()

	// Directory does not exist — open should fail.
	path := filepath.Join(t.TempDir(), "no", "such", "dir", "test.log")
	w, err := logfile.NewBufferedLogFile(path, 4096, 100*time.Millisecond)
	if err == nil {
		w.Close()
		t.Fatal("expected error for invalid path, got nil")
	}
}

func TestWriteAndClose(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "test.log")
	w, err := logfile.NewBufferedLogFile(path, 4096, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := []byte("hello world\n")
	n, err := w.Write(msg)
	if err != nil {
		t.Fatalf("unexpected Write error: %v", err)
	}
	if n != len(msg) {
		t.Fatalf("expected %d bytes written, got %d", len(msg), n)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("unexpected Close error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("unexpected ReadFile error: %v", err)
	}
	if string(got) != string(msg) {
		t.Fatalf("file content mismatch: got %q, want %q", got, msg)
	}
}

func TestWriteFlushes(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "test.log")
	const flushInterval = 50 * time.Millisecond

	w, err := logfile.NewBufferedLogFile(path, 4096, flushInterval)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer w.Close()

	msg := []byte("periodic flush check\n")
	if _, err := w.Write(msg); err != nil {
		t.Fatalf("unexpected Write error: %v", err)
	}

	// Wait for at least one flush tick.
	time.Sleep(flushInterval * 3)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("unexpected ReadFile error: %v", err)
	}
	if string(got) != string(msg) {
		t.Fatalf("expected data to be flushed, got %q", got)
	}
}

func TestCloseFlushesRemaining(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "test.log")
	// Large buffer + long interval so data stays buffered until Close.
	w, err := logfile.NewBufferedLogFile(path, 1<<20, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := []byte("buffered until close\n")
	if _, err := w.Write(msg); err != nil {
		t.Fatalf("unexpected Write error: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("unexpected Close error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("unexpected ReadFile error: %v", err)
	}
	if string(got) != string(msg) {
		t.Fatalf("file content mismatch: got %q, want %q", got, msg)
	}
}
