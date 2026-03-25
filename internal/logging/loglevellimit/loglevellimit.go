package loglevellimit

import (
	"context"
	"log/slog"
)

type levelLimitHandler struct {
	handler slog.Handler
	level   slog.Level
}

func NewLevelLimitHandler(handler slog.Handler, level slog.Level) slog.Handler {
	return &levelLimitHandler{
		handler: handler,
		level:   level,
	}
}

func (llh *levelLimitHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if level < llh.level {
		return false
	}

	return llh.handler.Enabled(ctx, level)
}

func (llh *levelLimitHandler) Handle(ctx context.Context, record slog.Record) error {
	return llh.handler.Handle(ctx, record)
}

func (llh *levelLimitHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelLimitHandler{
		handler: llh.handler.WithAttrs(attrs),
		level:   llh.level,
	}
}

func (llh *levelLimitHandler) WithGroup(name string) slog.Handler {
	return &levelLimitHandler{
		handler: llh.handler.WithGroup(name),
		level:   llh.level,
	}
}
