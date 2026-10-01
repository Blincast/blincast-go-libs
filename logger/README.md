# logger

`github.com/Blincast/blincast-go-libs/logger` — structured logging for Blincast services: a thin wrapper over `log/slog` with JSON output to stdout, so every service emits logs in the same shape instead of each one rolling its own formatting. Own Go module (own `go.mod`), tagged `logger/vX.Y.Z`.

## Installation

Private repo, so consumers need:

```bash
export GOPRIVATE=github.com/Blincast/*
```

and Git configured to authenticate over SSH/HTTPS for `github.com/Blincast/*` (same auth `go mod download` already needs to clone this repo).

```bash
go get github.com/Blincast/blincast-go-libs/logger@vX.Y.Z
```

Until a tagged version exists, pin to a commit with a pseudo-version instead:

```bash
go get github.com/Blincast/blincast-go-libs/logger@<commit-sha>
```

## Usage

Based on the actual usage pattern in [`erbon`](https://github.com/Blincast/erbon) (`cmd/main.go`, `pkg/api/handlers.go`):

```go
import "github.com/Blincast/blincast-go-libs/logger"

logLevel := os.Getenv("logLevel")
level, err := logger.ParseLevel(logLevel)
if err != nil {
    level = slog.LevelInfo // fall back instead of failing startup on a bad env value
}

logger.InitializeWithTraces(serviceName, level, telemetry.GetTraceFields) // see "Pairing with telemetry" below; use Initialize(serviceName, level) instead if you don't need trace correlation

logger.Info("service started", logger.Fields{
    "event":    "service_started",
    "logLevel": logLevel,
})

// Inside a request handler, use the *Context variants with the request's own ctx
// so the log line picks up trace_id/span_id from the active span:
if err := daspiAPI.UpdateGuestData(room, guestName, "S", hotelID); err != nil {
    logger.ErrorContext(c.Request.Context(), "failed to update guest data on daspi", logger.Fields{
        "event":     "daspi_update_guest_data_failed",
        "bookingId": bookingID,
        "room":      room,
        "hotelId":   hotelID,
        "error":     err.Error(),
    })
}
```

### Conventions seen in real usage

- Every call site passes `logger.Fields{"event": "some_snake_case_name", ...}`. `event` is the stable, machine-queryable identifier for that log line — the human-readable message string is free to change without breaking a dashboard or alert built on `event`.
- Errors go in as `"error": err.Error()` (a string), not the raw `error` value.
- Everything else — `bookingId`, `hotelId`, `guestId`, `clientIp`, `room`, etc. — is just whatever fields make sense for that log line. `Fields` is `map[string]any`; the library doesn't enforce a schema, so consistent naming across services is on convention, not tooling.

## API

- `Initialize(service, level)` — build a JSON logger for `service`, filtered at `level`, and install it as `slog.Default()`.
- `InitializeWithTraces(service, level, getTraceFieldsFn)` — same as `Initialize`, but every log line is enriched with whatever `getTraceFieldsFn` returns (e.g. `trace_id`, `span_id`) via a `TraceHandler` in front of the JSON handler.
- `ParseLevel(value string) (slog.Level, error)` — parse a level from config/env; empty string returns `Info` with no error.
- `Fields map[string]any`, `Log(ctx, level, msg, fields)`.
- `Info`/`Warn`/`Error`(msg, fields)`, `Fatal(msg, fields)` (logs at `Error`, then panics) — no `ctx`, always `context.Background()` under the hood, so no trace/span correlation. Use these when there's genuinely no request/operation context available (e.g. at startup, before a request is in flight).
- `InfoContext`/`WarnContext`/`ErrorContext`(ctx, msg, fields)`, `FatalContext(ctx, msg, fields)` — same as above but take `ctx` and thread it to `Log`, so a trace/span active on `ctx` gets attached to the line via `InitializeWithTraces`'s `getTraceFieldsFn`. **Prefer these whenever a request-scoped `ctx` is available** (e.g. `c.Request.Context()` in a Gin handler) — this is what actually makes trace correlation work.
- `LogHTTPFailure(message, provider, event, reqData, resp, respBody, err)` — standard shape for logging a failed upstream HTTP call; truncates the response body via `Truncate`.
- `Truncate(s, max)` — truncate a string (e.g. a response body) to `max` chars before logging it.
- `TraceHandler` / `NewTraceHandler(next slog.Handler, fn func(context.Context) map[string]string)` — wraps a `slog.Handler` to inject whatever fields `fn` returns (e.g. `trace_id`, `span_id`) into every record; this is what `InitializeWithTraces`'s `getTraceFieldsFn` argument wires up internally.

`Warn`, `Fatal`, `Log`, `LogHTTPFailure`, and `Truncate` exist and work, but aren't exercised anywhere in the current reference integration — worth knowing if you're looking for a real example of one of them and don't find one yet.

## Pairing with `telemetry` (optional)

`InitializeWithTraces`'s third argument is `getTraceFieldsFn func(context.Context) map[string]string` — pass it `telemetry.GetTraceFields` (from the sibling [`telemetry`](../telemetry/README.md) package) and every log line written through `Log`/`InfoContext`/`WarnContext`/`ErrorContext`/`FatalContext` picks up the active `trace_id` and `span_id`:

```go
logger.InitializeWithTraces(serviceName, level, telemetry.GetTraceFields)
```

`logger` has no Go-module dependency on `telemetry` (or vice versa) — the callback type is a plain `func(context.Context) map[string]string`, not a named type from either package, so this is purely a call-site pairing, deliberately kept that way so each library can be used (and changed) without the other's dependencies. Use plain `Initialize` instead if you don't want those fields on log lines. Note this pairing only takes effect through the `*Context` functions — the plain `Info`/`Warn`/`Error`/`Fatal` never carry a real `ctx`, so they never pick up `trace_id`/`span_id` even when `InitializeWithTraces` was used.

## Releasing a new version

No `logger/vX.Y.Z` tag exists yet. First release, from `main` after merging the feature branch:

```bash
git checkout main && git pull
git tag logger/v0.1.0
git push origin logger/v0.1.0
```

Optional: `gh release create logger/v0.1.0 --title "logger/v0.1.0" --notes "..."` for visibility.

Consumers then swap their pseudo-version for the tag:

```bash
go get github.com/Blincast/blincast-go-libs/logger@v0.1.0
```
