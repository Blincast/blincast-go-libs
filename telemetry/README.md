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

Until a tagged version exists, pin to a commit with a pseudo-version instead:

```bash
go get github.com/Blincast/blincast-go-libs/telemetry@<commit-sha>
```

## Usage

This part is the same no matter which router the service uses — call it once at startup:

```go
import "github.com/Blincast/blincast-go-libs/telemetry"

const serviceName = "my-app"

shutdown, err := telemetry.InitTelemetry(ctx, telemetry.Config{
    ServiceName:  serviceName,
    CollectorURL: os.Getenv("OTEL_COLLECTOR_URL"), // e.g. http://localhost:4317
})
defer shutdown(ctx)

go telemetry.NewMetricServer().ListenAndServe() // must run in a goroutine: ListenAndServe blocks, and main() still has to start the actual service

logger.Configure(serviceName, level, telemetry.GetTraceID) // correlate logger/logger.go output with traces

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

 It exists so a service that doesn't use Gin isn't forced to add it as a dependency just to get tracing/metrics. Don't register both the Gin and `net/http` middlewares on the same service; pick whichever one matches the router actually in use.

## API

- `Config{ServiceName, CollectorURL}` / `InitTelemetry(ctx, cfg) (shutdown func(context.Context) error, err error)` — sets up the `TracerProvider` with an OTLP/gRPC exporter (insecure, always-sample, 5s batch timeout), sets the global propagator (`TraceContext` + `Baggage`). Returns a shutdown func to defer.
- `GetTraceID(ctx) string` — reads the trace ID off the current span; empty string if none. Used to correlate logs (feed it to `logger.Configure`) and to tag metric exemplars.
- `NewClient()` / `WrapClient(client *http.Client) *http.Client` — an `http.Client` (new, or an existing one wrapped) whose `Transport` injects the `traceparent` header on outgoing requests.
- `NewTelemetryMiddleware(next http.Handler) http.Handler` — plain `net/http` tracing middleware (`otelhttp`), for non-Gin services.
- `NewMetricServer() *http.Server` — Prometheus `/metrics` endpoint on `:2112` (Go + process collectors registered by default). `ListenAndServe()` on it blocks like any `http.Server`, so it must be started in its own goroutine (`go telemetry.NewMetricServer().ListenAndServe()`) — otherwise it stalls `main()` before the rest of the service ever starts.
- `MetricsMiddleware(next http.Handler)` / `GinMetricsMiddleware()` — records `http_requests_total{method,path,status}` and `http_request_duration_seconds{method,path}` (with trace-ID exemplars when available) for `net/http` and Gin respectively.

## Steps

1. Add `OTEL_COLLECTOR_URL` to the service's env (`.env.example`, deployment config). Mostly it will be the default 'alloy:2112' connecting through docker network.
2. In `main.go`, build a `context.Background()` up front and call `telemetry.InitTelemetry` + `defer shutdown(ctx)`.
3. Start `telemetry.NewMetricServer()` **in a goroutine**: `go telemetry.NewMetricServer().ListenAndServe()`. Its `ListenAndServe()` blocks for as long as the server runs, same as any `http.Server` — calling it directly (without `go`) on the main goroutine would freeze `main()` right there and the rest of the service (the Gin router, etc.) would never start.
4. Call `logger.Configure(serviceName, level, telemetry.GetTraceID)` so log lines carry the active trace ID.
5. Register the tracing and metrics middlewares on the Gin router: `otelgin.Middleware(serviceName)` and `telemetry.GinMetricsMiddleware()`.
   > `otelgin.Middleware` is imported from `go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin` — a **third-party OpenTelemetry package, not part of `blincast-go-libs`**. The consuming service needs it in its own `go.mod` (`go get go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin`); this repo only provides `telemetry.GinMetricsMiddleware()`.
6. In every outbound API client (PMS, DASPI, etc.), wrap the `*http.Client` with `telemetry.WrapClient` before using it.

## Releasing a new version

No `telemetry/vX.Y.Z` tag exists yet. First release, from `main` after merging the feature branch:

```bash
git checkout main && git pull
git tag telemetry/v0.1.0
git push origin telemetry/v0.1.0
```

Optional: `gh release create telemetry/v0.1.0 --title "telemetry/v0.1.0" --notes "..."` for visibility.

Consumers then swap their pseudo-version for the tag:

```bash
go get github.com/Blincast/blincast-go-libs/telemetry@v0.1.0
```
