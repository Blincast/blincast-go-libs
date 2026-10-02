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

const tracerName = "github.com/Blincast/blincast-go-libs/telemetry"

type Config struct {
	ServiceName  string
	CollectorURL string // URL of the OpenTelemetry collector to send traces to
}

// InitTelemetry configures the OpenTelemetry SDK with a gRPC exporter to send traces to the
// specified collector URL. Traces are batched and sent every 5 seconds, or as soon as a batch
// reaches 512 spans, whichever comes first.
func InitTelemetry(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(cfg.CollectorURL),
	)
	if err != nil {
		return noop, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String(cfg.ServiceName)),
	)
	if err != nil {
		return exporter.Shutdown, fmt.Errorf("failed to create resource: %w", err)
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

// NewTelemetryMiddleware wraps next with OpenTelemetry HTTP instrumentation, so incoming
// requests are traced.
func NewTelemetryMiddleware(next http.Handler) http.Handler {
	return otelhttp.NewHandler(next, "http-request")
}

// NewClient returns an *http.Client whose transport injects the traceparent header into
// outgoing requests via OTel.
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

// GetTraceFields returns trace_id and span_id from the active span in ctx, suitable for
// logger.InitializeWithTraces's getTraceFieldsFn parameter so log lines carry both. Returns nil
// when there's no valid span in ctx.
func GetTraceFields(ctx context.Context) map[string]string {
	sc := oteltrace.SpanFromContext(ctx).SpanContext()
	if !sc.IsValid() {
		return nil
	}
	return map[string]string{
		"trace_id": sc.TraceID().String(),
		"span_id":  sc.SpanID().String(),
	}
}

// StartSpan starts a span named name as a child of the span in ctx, or as a new trace if ctx has
// none, and returns a ctx carrying it. Use it for work no middleware wraps (e.g. a polling run)
func StartSpan(ctx context.Context, name string) (context.Context, oteltrace.Span) {
	return otel.Tracer(tracerName).Start(ctx, name)
}
