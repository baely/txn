// Package logger provides a standardized logging approach for the TXN project
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Logger levels
const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// New creates a new structured logger with the given options
func New(opts ...Option) *slog.Logger {
	config := defaultConfig()
	for _, opt := range opts {
		opt(config)
	}

	handler := slog.NewJSONHandler(config.output, &slog.HandlerOptions{
		Level: config.level,
	})

	return slog.New(handler)
}

// Config holds the logger configuration
type config struct {
	level  slog.Level
	output io.Writer
}

func defaultConfig() *config {
	return &config{
		level:  LevelInfo,
		output: os.Stdout,
	}
}

// Option configures the logger
type Option func(*config)

// WithLevel sets the minimum log level
func WithLevel(level slog.Level) Option {
	return func(c *config) {
		c.level = level
	}
}

// WithOutput sets the output writer
func WithOutput(w io.Writer) Option {
	return func(c *config) {
		c.output = w
	}
}

// WithContext returns a logger with values from the context
func WithContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	// In a real application, you might extract trace ID, user ID, etc. from context
	return logger
}