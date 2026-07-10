# Log Analyzer (loganalyze)

## Stack
- Language: Go 1.22+
- CLI framework: `github.com/spf13/cobra`
- Color: `github.com/fatih/color`
- Test: `testing` stdlib
- Web UI: embedded vanilla HTML+JS via `//go:embed`
- Docker: multi-stage build (golang:1.22 → alpine:3.20), ~7.6MB image

## Conventions
- All business logic in `internal/` — never in `cmd/`
- `cmd/` files only wire up flags and call internal functions
- Streaming-first: process line-by-line, never load full file
- Errors returned, not logged; main.go is the only place that calls os.Exit
- ANSI color codes via `fatih/color`, not raw escape sequences
- Format detection is best-effort heuristic, no schema required
- All diagnostics go to stderr; only output data goes to stdout
- Signals (SIGINT/SIGTERM) produce partial output gracefully (exit 130)
- Server reuses the same internal/ engine as the CLI — no duplication

## Architecture (data flow)
CLI mode:
  Reader → Parser → Filter → Normalizer → Analyzer → Renderer
    │         │        │         │           │          │
  file/     level/   regex/    signature   counts/    console/
  stdin     ts      level     generation  grouping   json/csv

Server mode (serve command):
  Browser → HTTP API → Server handlers → Engine (same internal/)
    │           │            │
  Upload     Sessions     Run analysis in
  file       CRUD         background goroutine

  Note: Normalizer is only called during the Analyzer grouping step, not per-line.

## Commands
- `loganalyze scan <files...>` — full analysis report
- `loganalyze errors <files...>` — error lines with context
- `loganalyze top <files...>` — top N recurring error patterns
- `loganalyze grep <files...> <pattern>` — regex line search
- `loganalyze serve [--addr :8080] [--data /data]` — HTTP server with web UI
- Global flags: --since, --until, --level, --json, --csv, --no-color, --limit

## Data Model (internal/model/event.go)
  Event {
    Timestamp time.Time
    Level     Level (Debug/Info/Warn/Error/Fatal) — serializes as JSON string via MarshalJSON/UnmarshalJSON
    Source    string  // filename or "stdin"
    Message   string  // extracted message text
    Raw       string  // original line
    LineNum   int
  }

## Normalization rules
  Replace with <placeholder> tokens (in order):
  - UUIDs, request IDs, IPv6, IPv4, hex, file paths, hashes (40+ hex chars), standalone numbers
  Used to group near-identical error lines

## API (server mode)
  POST  /api/upload              upload log file → session_id
  POST  /api/analyze/{id}        run analysis with options
  GET   /api/results/{id}        get report (JSON)
  GET   /api/results/{id}/events paginated events (?offset=0&limit=100)
  GET   /api/status/{id}         SSE progress stream
  GET   /api/sessions            list active sessions
  DELETE /api/sessions/{id}      delete session
  GET   /api/uploaded/{id}       download original uploaded file
  GET   /health                  health check

## Web UI (internal/web/static/)
  - SPA with 3 pages: Dashboard (#/), Upload (#/upload), Session (#/session/:id)
  - Session detail: 3 tabs (Overview with SVG level chart, paginated Events, Raw file view)
  - CSS design system with dark/light themes persisted to localStorage
  - Keyboard shortcuts: ⌘1/Ctrl+1 → dashboard, ⌘U/Ctrl+U → upload
  - Toast notifications, drag-and-drop upload, inline event filters

## Testing
- Unit tests in each internal package
- Integration tests with testdata/samples/*.log
- Run: `go test ./...`
- Test logs live in testdata/samples/ — add new ones alongside new parsers

## Building
  go build -o loganalyze ./main.go
  docker build -t loganalyze .
  docker compose up
