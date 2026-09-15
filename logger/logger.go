package logger

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

type Fields map[string]any

func New(service string, level slog.Leveler, getTraceIDFn ...func(context.Context) string) *slog.Logger {
	var handler slog.Handler

	handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	if len(getTraceIDFn) > 0 && getTraceIDFn[0] != nil {
		handler = NewTraceHandler(handler, getTraceIDFn[0])
	}

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

func Fatal(message string, fields Fields) {
	Error(message, fields)
	panic(message)
}

func Warn(message string, fields Fields) {
	Log(context.Background(), slog.LevelWarn, message, fields)
}

func LogHTTPFailure(
	message string,
	provider string,
	event string,
	reqData Fields,
	resp *http.Response,
	respBody []byte,
	err error,
) {
	fields := Fields{
		"event":    event,
		"provider": provider,
		"request":  reqData,
	}

	if resp != nil {
		fields["status_code"] = resp.StatusCode

		if len(respBody) > 0 {
			fields["response_body"] = Truncate(string(respBody), 1500)
		}
	}

	if err != nil {
		fields["error"] = err.Error()
	}

	Error(message, fields)
}

func Truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}

	return s[:max] + "...(truncated)"
}
