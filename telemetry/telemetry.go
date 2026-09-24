package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type Config struct {
	ServiceName  string
	CollectorURL string // URL of the OpenTelemetry collector to send traces to
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Configure the OpenTelemetry SDK with a gRPC exporter to send traces to the specified collector URL.
// Traces are sent in batches with a timeout of 5 seconds before sending each batch or reach limit of 512 spans.
func InitTelemetry(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(cfg.CollectorURL),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String(cfg.ServiceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(5*time.Second)),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// NewTelemetryMiddleware returns a new http.Handler to intercept incoming HTTP
// request context and link observability between different applications.
func NewTelemetryMiddleware(next http.Handler) http.Handler {
	return otelhttp.NewHandler(next, "http-request")
}

// NewClient return an HTTP Client configured to handle requests injecting the traceparent on it through otel transport.
func NewClient() *http.Client {
	return &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
}

// WrapClient receives an existing HTTP client and wraps it with OpenTelemetry instrumentation.
// This allows the client to automatically propagate trace context and collect telemetry data for outgoing requests.
func WrapClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}

	baseTransport := client.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}

	client.Transport = otelhttp.NewTransport(baseTransport)

	return client
}

// GetTraceID returns the trace ID from the context. That way we can correlate logs and traces.
// If no trace ID is found, it return an empty string.
func GetTraceID(ctx context.Context) string {
	span := oteltrace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}
