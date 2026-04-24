package loglevellimit_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/szczursonn/rythm5/internal/logging/loglevellimit"
)

// ⚠️⚠️⚠️ AI-generated ⚠️⚠️⚠️

type mockHandler struct {
	enabled        bool
	enabledCalls   int
	handleErr      error
	records        []slog.Record
	attrs          []slog.Attr
	groupNames     []string
	withAttrsCalls int
	withGroupCalls int
	lastAttrs      []slog.Attr
	lastGroupName  string
}

func (h *mockHandler) Enabled(_ context.Context, _ slog.Level) bool {
	h.enabledCalls++
	return h.enabled
}

func (h *mockHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return h.handleErr
}

func (h *mockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.withAttrsCalls++
	h.lastAttrs = attrs
	return &mockHandler{
		enabled:    h.enabled,
		handleErr:  h.handleErr,
		attrs:      append(append([]slog.Attr(nil), h.attrs...), attrs...),
		groupNames: h.groupNames,
	}
}

func (h *mockHandler) WithGroup(name string) slog.Handler {
	h.withGroupCalls++
	h.lastGroupName = name
	return &mockHandler{
		enabled:    h.enabled,
		handleErr:  h.handleErr,
		attrs:      h.attrs,
		groupNames: append(append([]string(nil), h.groupNames...), name),
	}
}

func TestEnabled_BelowLimit_ShortCircuits(t *testing.T) {
	t.Parallel()

	inner := &mockHandler{enabled: true}
	h := loglevellimit.NewLevelLimitHandler(inner, slog.LevelWarn)

	for _, level := range []slog.Level{slog.LevelDebug, slog.LevelInfo} {
		if got := h.Enabled(context.Background(), level); got {
			t.Errorf("Enabled(%v) = true, want false", level)
		}
	}
	if inner.enabledCalls != 0 {
		t.Fatalf("inner Enabled should not be called for below-limit levels, got %d calls", inner.enabledCalls)
	}
}

func TestEnabled_AtOrAboveLimit_DelegatesToInner(t *testing.T) {
	t.Parallel()

	t.Run("inner_enabled", func(t *testing.T) {
		t.Parallel()
		inner := &mockHandler{enabled: true}
		h := loglevellimit.NewLevelLimitHandler(inner, slog.LevelWarn)

		for _, level := range []slog.Level{slog.LevelWarn, slog.LevelError} {
			if got := h.Enabled(context.Background(), level); !got {
				t.Errorf("Enabled(%v) = false, want true", level)
			}
		}
		if inner.enabledCalls != 2 {
			t.Fatalf("expected 2 inner Enabled calls, got %d", inner.enabledCalls)
		}
	})

	t.Run("inner_disabled", func(t *testing.T) {
		t.Parallel()
		inner := &mockHandler{enabled: false}
		h := loglevellimit.NewLevelLimitHandler(inner, slog.LevelWarn)

		if got := h.Enabled(context.Background(), slog.LevelWarn); got {
			t.Fatal("Enabled(Warn) = true, want false (inner disabled)")
		}
		if inner.enabledCalls != 1 {
			t.Fatalf("expected 1 inner Enabled call, got %d", inner.enabledCalls)
		}
	})
}

func TestHandle_DelegatesToInner(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		inner := &mockHandler{enabled: true}
		h := loglevellimit.NewLevelLimitHandler(inner, slog.LevelWarn)

		rec := slog.NewRecord(time.Time{}, slog.LevelError, "hello", 0)
		if err := h.Handle(context.Background(), rec); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(inner.records) != 1 {
			t.Fatalf("expected inner to receive 1 record, got %d", len(inner.records))
		}
		if inner.records[0].Message != "hello" {
			t.Errorf("record not forwarded correctly: got message %q", inner.records[0].Message)
		}
	})

	t.Run("propagates_error", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("handle failed")
		inner := &mockHandler{enabled: true, handleErr: wantErr}
		h := loglevellimit.NewLevelLimitHandler(inner, slog.LevelWarn)

		err := h.Handle(context.Background(), slog.Record{})
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected error %v, got %v", wantErr, err)
		}
	})
}

func TestWithAttrs_PreservesLevelAndAppliesAttrs(t *testing.T) {
	t.Parallel()

	inner := &mockHandler{enabled: true}
	h := loglevellimit.NewLevelLimitHandler(inner, slog.LevelWarn)

	attrs := []slog.Attr{slog.String("k", "v")}
	derived := h.WithAttrs(attrs)

	if derived == nil {
		t.Fatal("WithAttrs returned nil")
	}
	if derived == h {
		t.Fatal("WithAttrs should return a new handler instance")
	}
	if inner.withAttrsCalls != 1 {
		t.Fatalf("expected inner WithAttrs to be called once, got %d", inner.withAttrsCalls)
	}
	if len(inner.lastAttrs) != 1 || inner.lastAttrs[0].Key != "k" {
		t.Fatalf("inner WithAttrs received wrong attrs: %+v", inner.lastAttrs)
	}

	// Level limit is preserved in the derived handler.
	if derived.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("derived Enabled(Debug) = true, want false (level limit must persist)")
	}
	if !derived.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("derived Enabled(Warn) = false, want true")
	}
}

func TestWithGroup_PreservesLevelAndAppliesGroup(t *testing.T) {
	t.Parallel()

	inner := &mockHandler{enabled: true}
	h := loglevellimit.NewLevelLimitHandler(inner, slog.LevelWarn)

	derived := h.WithGroup("grp")

	if derived == nil {
		t.Fatal("WithGroup returned nil")
	}
	if derived == h {
		t.Fatal("WithGroup should return a new handler instance")
	}
	if inner.withGroupCalls != 1 {
		t.Fatalf("expected inner WithGroup to be called once, got %d", inner.withGroupCalls)
	}
	if inner.lastGroupName != "grp" {
		t.Fatalf("inner WithGroup received wrong name: %q", inner.lastGroupName)
	}

	if derived.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("derived Enabled(Debug) = true, want false (level limit must persist)")
	}
	if !derived.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("derived Enabled(Warn) = false, want true")
	}
}
