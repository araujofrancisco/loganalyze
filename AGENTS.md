# Log Analyzer (loganalyze)

## Stack
- Language: Go 1.22+
- CLI framework: `github.com/spf13/cobra`
- Color: `github.com/fatih/color`
- Test: `testing` stdlib
- Web UI: embedded vanilla HTML+JS via `//go:embed`
- Docker: multi-stage build, alpine runtime

## Conventions
- All business logic in `internal/` — never in `cmd/`
- `cmd/` files only wire up flags and call internal functions
- Streaming-first: process line-by-by, never load full file
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

## Testing
- Unit tests in each internal package
- Integration tests with testdata/samples/*.log
- Run: `go test ./...`
- Test logs live in testdata/samples/ — add new ones alongside new parsers

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
    Level     Level (Debug/Info/Warn/Error/Fatal)
    Source    string  // filename or "stdin"
    Message   string  // extracted message text
    Raw       string  // original line
    LineNum   int
  }

## Normalization rules
  Replace with <placeholder> tokens:
  - UUIDs, IPv4/IPv6, numbers, hex, file paths, hashes, request IDs
  Used to group near-identical error lines

## API (server mode)
  POST  /api/upload          upload log file → session_id
  POST  /api/analyze/{id}    run analysis with options
  GET   /api/results/{id}    get report (JSON)
  GET   /api/status/{id}     SSE progress stream
  GET   /api/sessions        list active sessions
  DELETE /api/sessions/{id}  delete session
  GET   /health              health check

## Building
  go build -o loganalyze ./main.go
  docker build -t loganalyze .
  docker compose up
