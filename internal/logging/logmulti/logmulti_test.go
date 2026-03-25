package logmulti_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/szczursonn/rythm5/internal/logging/logmulti"
)

// ⚠️⚠️⚠️ AI-generated ⚠️⚠️⚠️

// mockHandler is a minimal slog.Handler for testing.
type mockHandler struct {
	enabled    bool
	handleErr  error
	records    []slog.Record
	attrs      []slog.Attr
	groupNames []string
}

func (h *mockHandler) Enabled(_ context.Context, _ slog.Level) bool { return h.enabled }

func (h *mockHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return h.handleErr
}

func (h *mockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &mockHandler{
		enabled:    h.enabled,
		handleErr:  h.handleErr,
		attrs:      append(h.attrs, attrs...),
		groupNames: h.groupNames,
	}
}

func (h *mockHandler) WithGroup(name string) slog.Handler {
	return &mockHandler{
		enabled:    h.enabled,
		handleErr:  h.handleErr,
		attrs:      h.attrs,
		groupNames: append(h.groupNames, name),
	}
}

func TestNew_SingleHandler(t *testing.T) {
	t.Parallel()

	h := &mockHandler{enabled: true}
	got := logmulti.NewMultiHandler(h)
	if got != h {
		t.Fatal("expected single handler to be returned directly, got a wrapper")
	}
}

func TestNew_MultipleHandlers(t *testing.T) {
	t.Parallel()

	h1 := &mockHandler{enabled: true}
	h2 := &mockHandler{enabled: true}
	got := logmulti.NewMultiHandler(h1, h2)

	if got == h1 || got == h2 {
		t.Fatal("expected a new multiHandler wrapper, got one of the inputs")
	}
}

func TestEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		handlers []slog.Handler
		want     bool
	}{
		{
			name:     "all_enabled",
			handlers: []slog.Handler{&mockHandler{enabled: true}, &mockHandler{enabled: true}},
			want:     true,
		},
		{
			name:     "one_enabled",
			handlers: []slog.Handler{&mockHandler{enabled: false}, &mockHandler{enabled: true}},
			want:     true,
		},
		{
			name:     "none_enabled",
			handlers: []slog.Handler{&mockHandler{enabled: false}, &mockHandler{enabled: false}},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := logmulti.NewMultiHandler(tt.handlers...)
			if got := h.Enabled(context.Background(), slog.LevelInfo); got != tt.want {
				t.Fatalf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandle(t *testing.T) {
	t.Parallel()

	enabled := &mockHandler{enabled: true}
	disabled := &mockHandler{enabled: false}
	h := logmulti.NewMultiHandler(enabled, disabled)

	rec := slog.NewRecord(time.Time{}, slog.LevelInfo, "test message", 0)

	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(enabled.records) != 1 {
		t.Fatalf("expected enabled handler to receive 1 record, got %d", len(enabled.records))
	}
	if len(disabled.records) != 0 {
		t.Fatalf("expected disabled handler to receive 0 records, got %d", len(disabled.records))
	}
}

func TestHandle_ErrorAggregation(t *testing.T) {
	t.Parallel()

	errA := errors.New("handler A failed")
	errB := errors.New("handler B failed")

	tests := []struct {
		name     string
		handlers []slog.Handler
		wantErrs []error
	}{
		{
			name: "one_error",
			handlers: []slog.Handler{
				&mockHandler{enabled: true, handleErr: errA},
				&mockHandler{enabled: true},
			},
			wantErrs: []error{errA},
		},
		{
			name: "two_errors",
			handlers: []slog.Handler{
				&mockHandler{enabled: true, handleErr: errA},
				&mockHandler{enabled: true, handleErr: errB},
			},
			wantErrs: []error{errA, errB},
		},
		{
			name: "no_errors",
			handlers: []slog.Handler{
				&mockHandler{enabled: true},
				&mockHandler{enabled: true},
			},
			wantErrs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := logmulti.NewMultiHandler(tt.handlers...)
			err := h.Handle(context.Background(), slog.Record{})

			if tt.wantErrs == nil {
				if err != nil {
					t.Fatalf("expected nil error, got: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}
			for _, want := range tt.wantErrs {
				if !errors.Is(err, want) {
					t.Errorf("expected error to contain %v", want)
				}
			}
		})
	}
}

func TestWithAttrs(t *testing.T) {
	t.Parallel()

	h1 := &mockHandler{enabled: true}
	h2 := &mockHandler{enabled: true}
	multi := logmulti.NewMultiHandler(h1, h2)

	attrs := []slog.Attr{slog.String("key", "value")}
	derived := multi.WithAttrs(attrs)

	// The derived handler should be usable (satisfies slog.Handler).
	if derived == nil {
		t.Fatal("expected non-nil handler from WithAttrs")
	}

	// It should be a different handler than the original.
	if derived == multi {
		t.Fatal("expected WithAttrs to return a new handler")
	}

	// Verify attrs propagated by calling Handle on the derived handler.
	if err := derived.Handle(context.Background(), slog.Record{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithGroup(t *testing.T) {
	t.Parallel()

	h1 := &mockHandler{enabled: true}
	h2 := &mockHandler{enabled: true}
	multi := logmulti.NewMultiHandler(h1, h2)

	derived := multi.WithGroup("grp")

	if derived == nil {
		t.Fatal("expected non-nil handler from WithGroup")
	}
	if derived == multi {
		t.Fatal("expected WithGroup to return a new handler")
	}

	if err := derived.Handle(context.Background(), slog.Record{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
