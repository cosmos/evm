package server

import (
	"context"
	"log/slog"

	evm "github.com/ethereum/go-ethereum/log"

	sdk "cosmossdk.io/log"
)

// slogAdapter bridges Geth's slog logs to the existing Cosmos SDK logger.
type slogAdapter struct {
	logger sdk.Logger
	level  slog.Level
}

// SetEVMLogger sets the default evm logger and the default slog logger
func SetEVMLogger(logger *slog.Logger, level slog.Level) {
	// slog defaults
	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(level)

	// evm logger defaults
	evmLogger := evm.NewLogger(slog.Default().Handler())
	evm.SetDefault(evmLogger)
}

// NewSlogFromCosmosLogger wraps cosmos sdk logger to slog
func NewSlogFromCosmosLogger(logger sdk.Logger, levelStr string) (*slog.Logger, slog.Level) {
	a := &slogAdapter{
		logger: logger,
		level:  slog.LevelInfo,
	}

	// try to parse or leave as info
	_ = a.level.UnmarshalText([]byte(levelStr))

	return slog.New(a), a.level
}

// Handle processes slog records and forwards them to your Cosmos SDK logger.
func (a *slogAdapter) Handle(_ context.Context, r slog.Record) error {
	attrs := []any{}
	r.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr.Key, attr.Value.Any())
		return true
	})

	// Map slog levels to Cosmos SDK logger
	switch r.Level {
	case slog.LevelDebug:
		a.logger.Debug(r.Message, attrs...)
	case slog.LevelInfo:
		a.logger.Info(r.Message, attrs...)
	case slog.LevelWarn:
		a.logger.Warn(r.Message, attrs...)
	case slog.LevelError:
		a.logger.Error(r.Message, attrs...)
	default:
		a.logger.Info(r.Message, attrs...)
	}

	return nil
}

// Enabled determines if the handler should log a given level.
func (a *slogAdapter) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl >= a.level
}

// WithAttrs allows adding additional attributes.
func (a *slogAdapter) WithAttrs(attrs []slog.Attr) slog.Handler {
	flatten := make([]any, 0, len(attrs)*2)
	for _, attr := range attrs {
		flatten = append(flatten, attr.Key, attr.Value.Any())
	}

	return &slogAdapter{
		logger: a.logger.With(flatten...),
		level:  a.level,
	}
}

// WithGroup is required to implement slog.Handler
func (a *slogAdapter) WithGroup(group string) slog.Handler {
	return &slogAdapter{
		logger: a.logger.With("group", group),
		level:  a.level,
	}
}
