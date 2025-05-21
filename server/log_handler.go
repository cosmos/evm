package server

import (
	"context"
	"log/slog"

	"cosmossdk.io/log"
)

// SDKSlogHandler is a compat log handler between the eth root logger and the SDK's log types
type SDKSlogHandler struct {
	logger log.Logger
}

// Handle is the main entrypoint for log handling
func (h *SDKSlogHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := []interface{}{}
	r.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr.Key, attr.Value.Any())
		return true
	})

	switch r.Level {
	case slog.LevelDebug:
		h.logger.Debug(r.Message, attrs...)
	case slog.LevelInfo:
		h.logger.Info(r.Message, attrs...)
	case slog.LevelWarn:
		h.logger.Warn(r.Message, attrs...)
	case slog.LevelError:
		h.logger.Error(r.Message, attrs...)
	default:
		h.logger.Info(r.Message, attrs...)
	}

	return nil
}

// Enabled handles varying log levels
func (h *SDKSlogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// WithAttrs implements slog.Handler
func (h *SDKSlogHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

// WithGroup implements slog.Handler
func (h *SDKSlogHandler) WithGroup(_ string) slog.Handler {
	return h
}
