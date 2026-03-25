package logmulti

import (
	"context"
	"errors"
	"log/slog"
)

type multiHandler struct {
	handlers []slog.Handler
}

func NewMultiHandler(handlers ...slog.Handler) slog.Handler {
	if len(handlers) == 1 {
		return handlers[0]
	}

	return &multiHandler{handlers: handlers}
}

func (mh *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range mh.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (mh *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error

	for _, handler := range mh.handlers {
		if handler.Enabled(ctx, r.Level) {
			if err := handler.Handle(ctx, r); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

func (mh *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, 0, len(mh.handlers))
	for _, handler := range mh.handlers {
		newHandlers = append(newHandlers, handler.WithAttrs(attrs))
	}
	return &multiHandler{handlers: newHandlers}
}

func (mh *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, 0, len(mh.handlers))
	for _, handler := range mh.handlers {
		newHandlers = append(newHandlers, handler.WithGroup(name))
	}

	return &multiHandler{handlers: newHandlers}
}
