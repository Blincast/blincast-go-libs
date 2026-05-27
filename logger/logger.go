package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Fields map[string]any

func New(service string, level slog.Leveler) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(handler).With(
		slog.String("service", service),
	)
}

func Configure(service string, level slog.Leveler) {
	slog.SetDefault(New(service, level))
}

func ParseLevel(value string) (slog.Level, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return slog.LevelInfo, nil
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToUpper(value))); err != nil {
		return slog.LevelInfo, fmt.Errorf("invalid log level %q: %w", value, err)
	}

	return level, nil
}

func attrsFromFields(fields Fields) []slog.Attr {
	if fields == nil {
		return nil
	}

	attrs := make([]slog.Attr, 0, len(fields))
	for key, value := range fields {
		if key == "" {
			continue
		}

		attrs = append(attrs, slog.Any(key, value))
	}

	return attrs
}

func Log(ctx context.Context, level slog.Level, message string, fields Fields) {
	slog.LogAttrs(
		ctx,
		level,
		message,
		attrsFromFields(fields)...,
	)
}

func Info(message string, fields Fields) {
	Log(context.Background(), slog.LevelInfo, message, fields)
}

func Error(message string, fields Fields) {
	Log(context.Background(), slog.LevelError, message, fields)
}
