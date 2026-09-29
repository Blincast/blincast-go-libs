package logger

import (
	"context"
	"log/slog"
)

type TraceHandler struct {
	slog.Handler
	getTraceFieldsFunc func(context.Context) map[string]string
}

// NewTraceHandler returns a TraceHandler instance that wraps the provided slog.Handler and adds
// whatever context-derived fields fn returns (e.g. trace_id, span_id) to the log records.
func NewTraceHandler(next slog.Handler, fn func(context.Context) map[string]string) *TraceHandler {
	return &TraceHandler{
		Handler:            next,
		getTraceFieldsFunc: fn,
	}
}

// Handle method gets telemetry fields from context and send it to logs.
func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.getTraceFieldsFunc != nil {
		for key, value := range h.getTraceFieldsFunc(ctx) {
			if value != "" {
				r.AddAttrs(slog.String(key, value))
			}
		}
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs and WithGroup re-wrap the underlying handler's result in a new TraceHandler so the
// trace-field injection in Handle survives calls to Logger.With/WithGroup
func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithAttrs(attrs), getTraceFieldsFunc: h.getTraceFieldsFunc}
}

// Rewrap WithGroup method from slogHandler. See WithAttrs for further details.
func (h *TraceHandler) WithGroup(name string) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithGroup(name), getTraceFieldsFunc: h.getTraceFieldsFunc}
}
