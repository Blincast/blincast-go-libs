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

logger.Configure(serviceName, level, telemetry.GetTraceID) // 3rd arg optional, see "Pairing with telemetry" below

logger.Info("service started", logger.Fields{
    "event":    "service_started",
    "logLevel": logLevel,
})

if err := daspiAPI.UpdateGuestData(room, guestName, "S", hotelID); err != nil {
    logger.Error("failed to update guest data on daspi", logger.Fields{
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

- `New(service, level, getTraceIDFn...) *slog.Logger` — build a logger without installing it as the global default.
- `Configure(service, level, getTraceIDFn...)` — build and install as `slog.Default()`. This is what every real call site uses (`New` is only needed if you want a logger instance without touching the global default).
- `ParseLevel(value string) (slog.Level, error)` — parse a level from config/env; empty string returns `Info` with no error.
- `Fields map[string]any`, `Log(ctx, level, msg, fields)`, `Info`/`Warn`/`Error`(msg, fields)`, `Fatal(msg, fields)` (logs at `Error`, then panics).
- `LogHTTPFailure(message, provider, event, reqData, resp, respBody, err)` — standard shape for logging a failed upstream HTTP call; truncates the response body via `Truncate`.
- `Truncate(s, max)` — truncate a string (e.g. a response body) to `max` chars before logging it.
- `TraceHandler` / `NewTraceHandler(next slog.Handler, fn func(context.Context) string)` — wraps a `slog.Handler` to inject `trace_id` into every record; this is what `Configure`'s `getTraceIDFn` argument wires up internally.

`Warn`, `Fatal`, `Log`, `LogHTTPFailure`, and `Truncate` exist and work, but aren't exercised anywhere in the current reference integration (`erbon` only calls `ParseLevel`, `Configure`, `Info`, and `Error`) — worth knowing if you're looking for a real example of one of them and don't find one yet.

## Pairing with `telemetry` (optional)

`Configure`'s third argument is `getTraceIDFn func(context.Context) string` — pass it `telemetry.GetTraceID` (from the sibling [`telemetry`](../telemetry/README.md) package) and every log line picks up the active `trace_id`:

```go
logger.Configure(serviceName, level, telemetry.GetTraceID)
```

`logger` has no Go-module dependency on `telemetry` (or vice versa) — this is purely a call-site pairing. Omit the third argument entirely and `Configure` still works, just without `trace_id` on log lines.

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
