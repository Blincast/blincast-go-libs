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

const tracerName = "blincast-telemetry-lib"

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

// Middleware to intercept incomming HTTP requests.
// It handle Trace ID (get if exists or create a new one and inject in the context)
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		tracer := otel.GetTracerProvider().Tracer(tracerName)
		ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s", r.Method, r.URL.Path),
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		)
		defer span.End()

		span.SetAttributes(
			semconv.HTTPMethodKey.String(r.Method),
			semconv.HTTPTargetKey.String(r.URL.Path),
			semconv.HTTPUserAgentKey.String(r.UserAgent()),
		)

		r = r.WithContext(ctx)
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		span.SetAttributes(semconv.HTTPStatusCodeKey.Int(rw.statusCode))
	})
}

// NewClient return an HTTP Client configured to handle requests injecting the traceparent on it.
func NewClient() *http.Client {
	return &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
}
