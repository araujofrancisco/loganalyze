# Log Analyzer (loganalyze)

## Stack
- Go 1.22+, `spf13/cobra`, `fatih/color`, stdlib `testing`
- Web UI: embedded vanilla HTML+JS via `//go:embed`
- Docker: multi-stage (golang:1.22 → alpine:3.20), ~7.6MB image

## Architecture
See [`ARCHITECTURE.md`](./ARCHITECTURE.md) for detailed architecture, data model, pipeline design, and component documentation.

## Conventions
- All business logic in `internal/`; `cmd/` only wires flags and calls internal
- Streaming-first: process line-by-line, never load full file
- Errors returned, not logged (except `server/` handlers which use `log.Printf` for diagnostics)
- `main.go` delegates to `cmd.Execute()`; `os.Exit(1)` called in `cmd/root.go`, `cmd/grep.go`, `cmd/serve.go`
- ANSI colors via `fatih/color`, not raw escape sequences
- Format detection is best-effort heuristic, no schema required
- All diagnostics go to stderr; only output data to stdout
- Server reuses same `internal/` engine as CLI — no duplication

## Commands
- `scan [files...]` — full report (supports multi-file via positional args)
- `errors [files...]` — error lines (auto-forces `--level error`)
- `top [files...]` — top N patterns (auto-forces `--level error`)
- `grep [files...] <pattern>` — regex search (pattern is last positional arg)
- `watch [files...] [--every 30s] [--no-tail]` — tail files with live filtering / periodic summaries
- `serve [--addr :8080] [--data /data] [--rate-limit 60]` — HTTP server with web UI
- Global flags: `--since`, `--until`, `--level`, `--json`, `--csv`, `--no-color`, `--limit`,
  `--regex`, `--ai-endpoint`, `--ai-model`, `--fold`

## Key gotchas
- `--json` differs per command: `scan`/`top` → single JSON object; `errors`/`grep` → NDJSON (one object per line)
- `errors` and `top` override min level to ERROR regardless of `--level` (both CLI and server)
- `grep` takes pattern as last arg; `--regex` can be used as a global filter on `scan`/`errors`/`top`
- Server sessions expire after 1h since **last access** (cleanup every 10min); upload max 100 MB
- Docker Compose maps port **8081:8080** externally (not 8080:8080)
- Server uses Go 1.22 `{id}` path patterns with `r.PathValue("id")`
- Server has middleware: panic recovery, request ID, logging, rate limiting (60/min default, configurable via `--rate-limit`)
- Server implements graceful shutdown on SIGINT/SIGTERM (30s timeout)
- All API errors return structured JSON (`{"error":"message"}`), not plain text
- Server has `/api/watch/{id}` SSE endpoint for live log tailing (no rate-limit, read-only)
- Reader supports glob patterns; binary files (null bytes) are silently skipped; gzip files are transparently decompressed
- `watch` command uses `reader.TailFile` (poll-based, no fsnotify dep); supports `--every` for periodic summaries and `--no-tail` to start from beginning
- `--fold` merges stack trace continuation lines (leading whitespace) into their parent event; available in scan, errors, and watch
- Normalizer is only called during the Analyzer grouping step, not per-line
- `os.Exit` is called from `cmd/` package, not just `main.go`
- Fold operates between Reader and Parser: `Reader → Fold → Parser → Filter → Analyzer`

## Normalization (applied in order)
UUIDs → `<uuid>`, request IDs → `<req>`, IPv6/IPv4 → `<ip>`, hex → `<hex>`,
file paths → `<path>`, hashes (40+ hex) → `<hash>`, standalone numbers → `<n>`

## AI summarizer (`internal/summarizer/`)
- Interface: `Summarizer` with `Summarize` (sync) and `SummarizeStream` (SSE)
- Two impls: `noop` (default, zero weight) and `llm` (OpenAI-compatible HTTP, stdlib only)
- Configured via `--ai-endpoint` / `LOGANALYZE_AI_ENDPOINT` env var and `LOGANALYZE_AI_KEY`
- SSE streaming in web UI via `GET /api/insights/{id}/stream`
- Summary cached on session after first generation
- CLI flags available on `scan`, `top`, and `serve` commands

## Testing
- `go test ./...` — unit tests per internal package
- Test data in `testdata/samples/`; add new samples alongside new parsers
- Run `go vet ./...` before tests

## Build
- `go build -o loganalyze ./main.go`
- `docker build -t loganalyze .` / `docker compose up`
