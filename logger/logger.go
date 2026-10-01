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

func newHandler(level slog.Leveler) slog.Handler {
	return slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
}

func setupSlog(handler slog.Handler, service string) *slog.Logger {
	return slog.New(handler).With(
		slog.String("service", service),
	)
}

// Initialize builds a JSON logger for service, filtered at level, and installs it as slog's
// default via slog.SetDefault.
func Initialize(service string, level slog.Leveler) {
	logHandler := newHandler(level)

	slog.SetDefault(setupSlog(logHandler, service))
}

// InitializeWithTraces is Initialize, but every log line is enriched with trace_id/span_id from
// the active span via getTraceFieldsFn (e.g. telemetry.GetTraceFields).
func InitializeWithTraces(service string, level slog.Leveler, getTraceFieldsFn func(context.Context) map[string]string) {
	logHandler := newHandler(level)
	traceHandler := NewTraceHandler(logHandler, getTraceFieldsFn)

	slog.SetDefault(setupSlog(traceHandler, service))
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

// InfoContext logs at Info level using ctx, so a trace/span active on ctx (see NewWithTraces's
// getTraceFieldsFn) is attached to the log line. Prefer this over Info wherever a request-scoped
// ctx is available.
func InfoContext(ctx context.Context, message string, fields Fields) {
	Log(ctx, slog.LevelInfo, message, fields)
}

// WarnContext is Warn's context-aware counterpart. See InfoContext.
func WarnContext(ctx context.Context, message string, fields Fields) {
	Log(ctx, slog.LevelWarn, message, fields)
}

// ErrorContext is Error's context-aware counterpart. See InfoContext.
func ErrorContext(ctx context.Context, message string, fields Fields) {
	Log(ctx, slog.LevelError, message, fields)
}

// FatalContext is Fatal's context-aware counterpart. See InfoContext.
func FatalContext(ctx context.Context, message string, fields Fields) {
	ErrorContext(ctx, message, fields)
	panic(message)
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
