# Log Analyzer

A CLI tool and web application that reads log files and answers: *what's failing, when, how often, and where?*

- **Streaming-first:** processes files line-by-line — handles GB-sized logs in low memory
- **Zero config:** detects format, level, and timestamps heuristically — no schema required
- **Smart grouping:** normalizes UUIDs, IPs, numbers, paths to collapse near-identical errors
- **CLI + Web:** same engine powers both terminal and browser UI
- **Docker:** single-command deployment with embedded web UI (~7.6 MB image)

---

## Quick Start

### Docker (recommended)

```bash
docker compose up --build
# Open http://localhost:8081
```

### From source

```bash
go build -o loganalyze ./main.go
./loganalyze scan testdata/samples/errors.log
```

---

## Usage — CLI

### Commands

| Command | Example | Description |
|---|---|---|
| `scan` | `loganalyze scan app.log` | Full report: totals, level breakdown, top errors |
| `errors` | `loganalyze errors app.log --since 1h` | Error-level lines with context |
| `top` | `loganalyze top app.log --limit 20` | Top N recurring error patterns (supports `--ai-endpoint`) |
| `grep` | `loganalyze grep app.log "timeout\|panic"` | Regex search with highlighting |
| `serve` | `loganalyze serve --addr :8080` | HTTP server with web UI (supports `--ai-endpoint`) |

### Global flags

| Flag | Default | Description |
|---|---|---|
| `--since` | — | Relative time filter (`1h`, `30m`, `24h`) |
| `--until` | — | Absolute end time (RFC 3339) |
| `--level` | `""` | Minimum level: debug, info, warn, error, fatal |
| `--json` | false | JSON output (scan → single object; errors/grep → NDJSON) |
| `--csv` | false | CSV output |
| `--no-color` | false | Disable ANSI colors |
| `--limit` | 10 | Max results (top errors, grep matches) |
| `--regex` | `""` | Regex pattern filter (applies to scan/errors/top) |
| `--ai-endpoint` | `""` | OpenAI-compatible API endpoint for AI summary (also: `LOGANALYZE_AI_ENDPOINT`) |
| `--ai-model` | `gpt-4o-mini` | AI model name (also: `LOGANALYZE_AI_MODEL`) |

### Examples

```bash
# Full report
loganalyze scan app.log

# Error lines from the last hour
loganalyze errors app.log --since 1h

# Top 10 error patterns
loganalyze top app.log --limit 10

# Regex search
loganalyze grep app.log "timeout|panic|refused"

# Pipe from stdin
cat app.log | loganalyze scan

# Export as JSON
loganalyze scan app.log --json > report.json

# Combined filters
loganalyze grep app.log "5[0-9][0-9]" --level warn --since 2h
```

### Example output

```
File: app.log
Total lines: 15,342
Time range:  2026-07-08 08:01:12 — 2026-07-08 17:45:33 (9h44m)

Level     Count      %
──────────────────────────────
ERROR       342    2.2%
WARN        891    5.8%
INFO      14,109   91.9%

Top errors:
 1. database connection timeout after <n>s     47x  [10:12 - 14:33]
 2. request failed: POST /api/users <n>        23x  [11:04 - 13:12]
```

### Exit codes

| Code | Meaning |
|---|---|---|
| 0 | Success |
| 1 | General error (bad flags, read failure) |
| 130 | Interrupted by SIGINT/SIGTERM |

---

## Usage — Web UI

```bash
docker compose up
# → http://localhost:8081
```

### Pages

| Route | Content |
|---|---|
| `/` | **Dashboard** — stat cards, searchable session table |
| `/upload` | **Upload & Analyze** — drag-and-drop file upload, command/flag options |
| `/session/:id` | **Session detail** — four tabs: Overview, Events, AI Insights, Raw |

### Overview tab

- Stat cards: total lines, errors, warnings, info
- SVG bar chart: visual level breakdown with percentage labels
- Error groups: collapsible accordion with count, time range, and sample message

### AI Insights tab

- AI-powered analysis of error patterns using an OpenAI-compatible API
- Streaming markdown output with real-time rendering
- Requires `--ai-endpoint` / `LOGANALYZE_AI_ENDPOINT` to configure
- Cached on session after first generation

### Events tab

- Filterable, paginated event list (100 per page)
- Level dropdown filter, text search within results
- Expandable rows showing the raw log line
- Page navigation with page number buttons

### Raw tab

- Line-numbered view of the original uploaded file
- Useful for understanding context around matched events

### Keyboard shortcuts

| Shortcut | Action |
|---|---|
| `⌘1` / `Ctrl+1` | Dashboard |
| `⌘U` / `Ctrl+U` | Upload page |

### Theme

Toggle between dark and light themes using the button in the sidebar footer. The preference is persisted in localStorage and respects `prefers-color-scheme` on first visit.

---

## Filtering

Filters combine with AND logic:

```bash
# Level + time
loganalyze errors app.log --level error --since 2h

# Regex + level
loganalyze grep app.log "5[0-9][0-9]" --level warn
```

- **Level:** `evt.Level >= cfg.MinLevel` (searched order: FATAL > ERROR > WARN > INFO > DEBUG)
- **Regex:** matched against the raw line (not the parsed message)
- **Time:** requires a detected timestamp; zero-timestamp events pass through

---

## Error Grouping

Near-identical errors are collapsed using normalized signatures:

| Before | After |
|---|---|
| `timeout connecting to db-01 (10.0.0.5:5432)` | `timeout connecting to <host> (<ip>:<n>)` |
| `timeout connecting to db-02 (10.0.0.6:5432)` | `timeout connecting to <host> (<ip>:<n>)` |

**Replaced tokens:** UUIDs, IPv4/IPv6, numbers, hex, file paths, 40+ char hashes, request IDs.

Groups are tracked by a streaming top-K min-heap (bounded by `--limit`). Only events at `LevelError` and above are grouped.

---

## API

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/upload` | Upload log file (multipart, field `file`, max 100 MB) → `session_id` |
| `POST` | `/api/analyze/{id}` | Run analysis with command and flags |
| `GET` | `/api/results/{id}` | Get report JSON and events |
| `GET` | `/api/results/{id}/events` | Paginated events (`?offset=0&limit=100`) |
| `GET` | `/api/status/{id}` | SSE progress stream |
| `GET` | `/api/sessions` | List active sessions |
| `DELETE` | `/api/sessions/{id}` | Delete session and uploaded file |
| `GET` | `/api/uploaded/{id}` | Download the original uploaded file |
| `GET` | `/api/insights/{id}` | AI summary (sync, cached after first generation) |
| `GET` | `/api/insights/{id}/stream` | AI summary (SSE streaming, cached after streaming) |
| `GET` | `/health` | Health check |

### Upload + analyze via curl

```bash
# Upload
SESSION=$(curl -s -F "file=@app.log" http://localhost:8080/api/upload | \
  python3 -c "import json,sys; print(json.load(sys.stdin)['session_id'])")

# Start analysis
curl -s -X POST http://localhost:8080/api/analyze/$SESSION \
  -H "Content-Type: application/json" \
  -d '{"command":"scan","level":"error","limit":10}'

# Wait for completion, then fetch results
sleep 2
curl -s http://localhost:8080/api/results/$SESSION | python3 -m json.tool

# Paginated events (for errors/grep commands)
curl -s "http://localhost:8080/api/results/$SESSION/events?offset=0&limit=50"

# Download raw file
curl -s http://localhost:8080/api/uploaded/$SESSION
```

### Analyze request body

```json
{
  "command": "scan",
  "level": "error",
  "regex": "",
  "limit": 10,
  "since": "1h",
  "until": ""
}
```

`command` must be one of: `scan`, `errors`, `top`, `grep`.

---

## Deployment

### Docker Compose

```yaml
services:
  loganalyze:
    build: .
    ports:
      - "8081:8080"
    env_file:
      - .env
    environment:
      - TZ=UTC
    volumes:
      - logdata:/data
    restart: unless-stopped
volumes:
  logdata:
```

### Production considerations

- **Reverse proxy:** place behind nginx/Caddy for TLS, rate limiting, and auth
- **Resource limits:** 256 MB RAM is sufficient for most workloads
- **Persistence:** uploaded files live in `/data` volume; sessions expire after 1 hour
- **Health checks:** use `/health` endpoint for container orchestration
- **File size:** uploads are limited to 100 MB per file
- **Port conflicts:** use `--addr :9090` or change `docker-compose.yml` port mapping

---

## Project Structure

```
loganalyzer/
├── main.go                   # CLI entry point, cobra setup
├── go.mod / go.sum
├── Dockerfile                # Multi-stage (golang:1.22 → alpine:3.20)
├── docker-compose.yml        # One-command deployment
├── AGENTS.md                 # Agent context for AI coding sessions
├── README.md
├── SPEC.md                   # Technical specification
├── cmd/                      # CLI subcommands (cobra)
│   ├── root.go               # Root command + persistent flags + os.Exit(1)
│   ├── flags.go              # buildFilterConfig(), getAIConfig()
│   ├── pipeline.go           # startPipeline() — reads, parses, filters
│   ├── scan.go               # Full analysis report
│   ├── errors.go             # Error lines with context (forces LevelError)
│   ├── top.go                # Top N error patterns (forces LevelError)
│   ├── grep.go               # Regex search (pattern = last positional arg)
│   └── serve.go              # HTTP server with web UI
├── internal/
│   ├── model/
│   │   └── event.go          # Event, Group, Report, Level types
│   ├── reader/
│   │   └── reader.go         # File/stdin/glob with binary detection
│   ├── parser/
│   │   ├── parser.go         # Level, timestamp, message extraction
│   │   └── patterns.go       # Timestamp regex patterns
│   ├── normalizer/
│   │   └── normalizer.go     # Signature normalization
│   ├── filter/
│   │   └── filter.go         # Level/regex/time filtering
│   ├── analyzer/
│   │   └── analyzer.go       # Streaming counts, top-K min-heap
│   ├── renderer/
│   │   ├── console.go        # ANSI-colored terminal output
│   │   ├── json.go           # JSON/NDJSON export
│   │   └── csv.go            # CSV export
│   ├── server/
│   │   ├── server.go         # HTTP router, background cleanup goroutine
│   │   └── handlers.go       # Upload, analyze, results, SSE, CRUD
│   ├── session/
│   │   └── session.go        # In-memory session store with RWMutex + TTL
│   ├── summarizer/
│   │   ├── summarizer.go     # Summarizer interface + SummaryRequest
│   │   ├── llm.go            # OpenAI-compatible HTTP implementation
│   │   └── noop.go           # Default fallback (zero weight)
│   └── web/
│       ├── embed.go          # go:embed directive
│       └── static/
│           ├── index.html    # SPA shell
│           ├── app.js        # Vanilla ES module (~1078 lines)
│           └── style.css     # CSS custom properties (~1209 lines)
└── testdata/
    └── samples/              # errors.log, syslog.log, apache.log
```

---

## Data Pipeline

### CLI mode

```
Reader → Parser → Filter → Analyzer → Renderer
  │         │        │         │          │
file/     level/   regex/   counts/    console/
stdin     timestamp level   grouping   json/csv
```

### Server mode

```
Browser → HTTP API → Server Handlers → Engine (same internal/)
  │           │             │
Upload     Sessions     Background analysis
  file       CRUD        goroutine
```

Normalizer is only called during the Analyzer grouping step (error-level events), never per-line.

---

## Timestamp Formats

| Format | Example | Priority |
|---|---|---|
| ISO 8601 / RFC 3339 | `2026-07-08T10:12:33Z` | 1 (highest) |
| ISO 8601 date + time | `2026-07-08 10:12:33` | 2 |
| Syslog (BSD) | `Jul  8 10:12:33` | 3 |
| Apache CLF | `08/Jul/2026:10:12:33 +0000` | 4 |
| Unix epoch seconds | `1720421553` | 5 |

All timestamps are normalized to UTC.

---

## Level Detection

Keywords are detected case-insensitively with word-boundary matching:

| Level | Aliases | JSON value |
|---|---|---|
| FATAL | FATAL, CRITICAL, PANIC | `"FATAL"` |
| ERROR | ERROR | `"ERROR"` |
| WARN | WARN, WARNING | `"WARN"` |
| INFO | INFO | `"INFO"` |
| DEBUG | DEBUG, TRACE | `"DEBUG"` |

Lines without a recognized keyword default to INFO.

---

## Normalization Rules

Applied in order to the Message field of error-level events:

1. UUIDs → `<uuid>`
2. Request IDs → `<req>`
3. IPv6 → `<ip>`
4. IPv4 → `<ip>`
5. Hex numbers (0x prefix) → `<hex>`
6. File paths → `<path>`
7. Hashes (40+ hex chars) → `<hash>`
8. Standalone numbers → `<n>`

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `No input files` | No file paths or stdin pipe | Pass paths as args or pipe data |
| `Binary file` warning | File contains null bytes | Skip — not a text log |
| Timestamps look wrong | Non-UTC timezone in log | All timestamps are normalized to UTC |
| Server won't start | Port conflict | `--addr :9090` to use another port |
| Upload fails (413) | File > 100 MB | Split the file or compress it |
| High memory | Too many error groups | Lower `--limit` (default 10) |
| No level detected | Non-standard level keyword | Tool defaults to INFO; use `--level` |
| Events tab shows 0 events | Command is `scan` (no events) | Use `errors` or `grep` command |
| Theme not persisting | localStorage blocked | Check browser privacy settings |

---

## Development

```bash
# Build
go build -o loganalyze ./main.go

# Test
go test ./...

# Vet
go vet ./...

# Run server locally
./loganalyze serve --addr :8080 --data /tmp/logdata
```

### Adding a new log format

1. Add a timestamp regex in `internal/parser/patterns.go`
2. Add the parse branch in `internal/parser/parser.go`
3. Add a test sample to `testdata/samples/`
4. Run `go test ./internal/parser/`

### Adding a new output format

1. Add the format function in `internal/renderer/`
2. Add the format switch branch in the relevant `cmd/*.go` file
3. Add the flag to `cmd/flags.go`

---

## Roadmap

| Phase | Features |
|---|---|
| **3** | `watch` (tail -f), multi-file merged reports, stack trace folding, gzip decompression |
| **4** | JSON log field extraction, spike/anomaly detection, config file |
| **5** | CI/CD (goreleaser), shell completions, Homebrew tap |
| **6** | Interactive TUI (bubbletea) |
