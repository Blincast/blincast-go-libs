package logger

import (
	"context"
	"log/slog"
)

type TraceHandler struct {
	slog.Handler
	getTraceIDFunc func(context.Context) string
}

// NewTraceHandler returns a TraceHandler instance that wraps the provided slog.Handler and adds the trace ID to the log records.
func NewTraceHandler(next slog.Handler, fn func(context.Context) string) *TraceHandler {
	return &TraceHandler{
		Handler:        next,
		getTraceIDFunc: fn,
	}
}

// Handle executa a interceptação do log antes dele ser impresso no console
func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.getTraceIDFunc != nil {
		if traceID := h.getTraceIDFunc(ctx); traceID != "" {
			r.AddAttrs(slog.String("trace_id", traceID))
		}
	}
	return h.Handler.Handle(ctx, r)
}
