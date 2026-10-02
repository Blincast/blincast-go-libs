# telemetry

`github.com/Blincast/blincast-go-libs/telemetry` — OpenTelemetry traces (OTLP/gRPC exporter) and Prometheus metrics, with Gin and `net/http` integration. Own Go module (own `go.mod`), tagged `telemetry/vX.Y.Z`.

## Installation

Private repo, so consumers need:

```bash
export GOPRIVATE=github.com/Blincast/*
```

and Git configured to authenticate over SSH/HTTPS for `github.com/Blincast/*` (same auth `go mod download` already needs to clone this repo).

```bash
go get github.com/Blincast/blincast-go-libs/telemetry@vX.Y.Z
```

## Usage

This part is the same no matter which router the service uses — call it once at startup:

```go
import (
    "context"
    "log/slog"
    "os"

    "github.com/Blincast/blincast-go-libs/logger"
    "github.com/Blincast/blincast-go-libs/telemetry"
)

const serviceName = "my-app"

ctx := context.Background()

// set the default logger first, so this lib's own logs (e.g. the missing-metrics-port warning)
// carry the service field and trace IDs
slog.SetDefault(logger.NewWithTraces(serviceName, level, telemetry.GetTraceFields))

shutdown, err := telemetry.InitTelemetry(ctx, telemetry.Config{
    ServiceName:  serviceName,
    CollectorURL: os.Getenv("OTEL_COLLECTOR_URL"), // e.g. localhost:4317
})
if err != nil {
    logger.Error("failed to initialize telemetry", logger.Fields{"error": err.Error()})
}
defer shutdown(ctx)

go telemetry.NewMetricServer(os.Getenv("METRICS_PORT")).ListenAndServe() // must run in a goroutine: ListenAndServe blocks, and main() still has to start the actual service

httpClient = telemetry.WrapClient(httpClient) // outgoing clients propagate the trace header
```

Incoming-request tracing and metrics, on the other hand, depend on which router the service uses — pick one:

### With Gin

```go
r := gin.Default()
r.Use(otelgin.Middleware(serviceName))  // third-party (go.opentelemetry.io/contrib/.../otelgin), not part of this repo — incoming trace context
r.Use(telemetry.GinMetricsMiddleware()) // http_requests_total / http_request_duration_seconds
```

### With plain `net/http`

```go
mux := http.NewServeMux()
// ... register handlers on mux ...

var handler http.Handler = mux
handler = telemetry.NewTelemetryMiddleware(handler) // otelhttp — incoming trace context
handler = telemetry.MetricsMiddleware(handler)       // http_requests_total / http_request_duration_seconds

http.ListenAndServe(":8080", handler)
```

This variant is for services whose router isn't Gin. Don't register both the Gin and `net/http` middlewares on the same service; pick whichever one matches the router actually in use.

## API

- `Config{ServiceName, CollectorURL}` / `InitTelemetry(ctx, cfg) (shutdown func(context.Context) error, err error)` — sets up the `TracerProvider` with an OTLP/gRPC exporter (insecure, always-sample, 5s batch timeout), sets the global propagator (`TraceContext` + `Baggage`). Returns a shutdown func to defer.
- `GetTraceFields(ctx) map[string]string` — reads `trace_id` and `span_id` off the current span; `nil` if none. Feed this to `logger.NewWithTraces`'s `getTraceFieldsFn` argument (`slog.SetDefault(logger.NewWithTraces(serviceName, level, telemetry.GetTraceFields))`) to correlate logs with traces — see [`logger/README.md`](../logger/README.md)'s "Pairing with telemetry" section. (There's also an unexported `getTraceID` used internally by `MetricsMiddleware`/`GinMetricsMiddleware` to tag Prometheus exemplars with just the trace ID — not part of the public API.)
- `StartSpan(ctx, name) (context.Context, trace.Span)` — starts a span (child of the one in `ctx`, or a new trace) and returns a `ctx` carrying it. For work no middleware wraps, e.g. a polling run. The caller must `defer span.End()`. Before `InitTelemetry` (or if it failed) it returns a no-op span, never `nil`.
- `NewClient()` / `WrapClient(client *http.Client) *http.Client` — an `http.Client` (new, or an existing one wrapped) whose `Transport` injects the `traceparent` header on outgoing requests.
- `NewTelemetryMiddleware(next http.Handler) http.Handler` — plain `net/http` tracing middleware (`otelhttp`), for non-Gin services.
- `NewMetricServer(port string) *http.Server` — Prometheus `/metrics` endpoint on `:<port>` (Go + process collectors registered by default). `ListenAndServe()` on it blocks like any `http.Server`, so it must be started in its own goroutine (`go telemetry.NewMetricServer(port).ListenAndServe()`) — otherwise it stalls `main()` before the rest of the service ever starts. An empty `port` never stops the app: it logs a `metrics_port_missing` warning via `slog`, and the server ends up on a random port Prometheus can't scrape.
- `MetricsMiddleware(next http.Handler)` / `GinMetricsMiddleware()` — records `http_requests_total{method,path,status}` and `http_request_duration_seconds{method,path}` (with trace-ID exemplars when available) for `net/http` and Gin respectively.

## Steps

1. Add `OTEL_COLLECTOR_URL` and `METRICS_PORT` to the service's env (`.env.example`, deployment config). `OTEL_COLLECTOR_URL` will mostly be the default `alloy:4317` (Alloy's OTLP gRPC receiver) through the docker network; `METRICS_PORT` is usually `2112`, the port Prometheus scrapes.
2. In `main.go`, call `slog.SetDefault(logger.NewWithTraces(serviceName, level, telemetry.GetTraceFields))` before anything from this lib, so its logs carry the service's `service` field (several services share the same server, so logs without it can't be traced back to one). Then build a `context.Background()` and call `telemetry.InitTelemetry` + `defer shutdown(ctx)`.
3. Start `telemetry.NewMetricServer(port)` **in a goroutine**: `go telemetry.NewMetricServer(os.Getenv("METRICS_PORT")).ListenAndServe()`. Its `ListenAndServe()` blocks for as long as the server runs, same as any `http.Server` — calling it directly (without `go`) on the main goroutine would freeze `main()` right there and the rest of the service (the Gin router, etc.) would never start.
4. The `slog.SetDefault` from step 2 is also what lets log lines carry the active trace ID and span ID. This alone isn't enough, though — it only takes effect on logs written via `logger.InfoContext`/`WarnContext`/`ErrorContext`/`FatalContext`, so request-scoped logging needs to use those instead of the plain `Info`/`Warn`/`Error`/`Fatal` — see `logger/README.md`'s "Pairing with telemetry" section.
5. Register the tracing and metrics middlewares on the Gin router: `otelgin.Middleware(serviceName)` and `telemetry.GinMetricsMiddleware()` (for a plain `net/http` router, use `telemetry.NewTelemetryMiddleware` + `telemetry.MetricsMiddleware` instead — see [Usage](#with-plain-nethttp)).
   > `otelgin.Middleware` is imported from `go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin` — a **third-party OpenTelemetry package, not part of `blincast-go-libs`**. The consuming service needs it in its own `go.mod` (`go get go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin`); this repo only provides `telemetry.GinMetricsMiddleware()`.
6. Services without incoming requests (polling workers, cron jobs) have no middleware to start a span, so their logs get no `trace_id` and each outgoing call becomes its own trace. Wrap each unit of work (one span per run, never one for the whole process) instead:
   ```go
   func runSync(ctx context.Context) {
       ctx, span := telemetry.StartSpan(ctx, "sync_rooms")
       defer span.End()

       doSync(ctx) // outgoing calls and *Context logs in here share the span's trace
   }
   ```
7. In every outbound API client (PMS, DASPI, etc.), wrap the `*http.Client` with `telemetry.WrapClient` before using it.

## Releasing a new version

From `main` after merging the feature branch, tag the next version (`X.Y.Z` following semver: patch for fixes and non-breaking additions, minor for new features, major for breaking API changes):

```bash
git checkout main && git pull
git tag telemetry/vX.Y.Z
git push origin telemetry/vX.Y.Z
gh release create telemetry/vX.Y.Z --title "vX.Y.Z - YYYY/MM/DD - telemetry" --notes "..."
```

Consumers then update to the new tag:

```bash
go get github.com/Blincast/blincast-go-libs/telemetry@vX.Y.Z
```
