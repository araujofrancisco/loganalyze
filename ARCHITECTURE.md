# Architecture

## Overview

Log Analyzer processes log files through a streaming pipeline. Both CLI and server modes share the same `internal/` engine — no code duplication.

```
CLI mode:   Reader → Parser → Filter → Analyzer → Renderer
Server mode: Browser → HTTP API → Server Handlers → Engine (same internal/)
```

## Project Structure

```
loganalyzer/
├── main.go                   # Entry point, calls cmd.Execute()
├── go.mod / go.sum
├── Dockerfile                # Multi-stage (golang:1.22 → alpine:3.20)
├── docker-compose.yml        # One-command deployment (port 8081:8080)
├── AGENTS.md                 # Agent context for AI coding sessions
├── README.md                 # User-facing documentation
├── ARCHITECTURE.md           # This file
├── SPEC.md                   # Technical specification
├── cmd/                      # CLI subcommands (cobra)
│   ├── root.go               # Root command + persistent flags + os.Exit(1)
│   ├── flags.go              # buildFilterConfig(), getAIConfig()
│   ├── pipeline.go           # startPipeline() — reads, parses, filters
│   ├── scan.go               # Full analysis report
│   ├── errors.go             # Error lines (forces LevelError)
│   ├── top.go                # Top N error patterns (forces LevelError)
│   ├── grep.go               # Regex search (pattern = last positional arg)
│   └── serve.go              # HTTP server with web UI
├── internal/
│   ├── model/
│   │   └── event.go          # Event, Group, Report, Level types
│   ├── reader/
│   │   └── reader.go         # File/stdin/glob reading with binary detection
│   ├── parser/
│   │   ├── parser.go         # Level, timestamp, message extraction
│   │   └── patterns.go       # Timestamp regex patterns
│   ├── normalizer/
│   │   └── normalizer.go     # Signature normalization (UUID→<uuid>, etc.)
│   ├── filter/
│   │   └── filter.go         # Level/regex/time filtering
│   ├── analyzer/
│   │   └── analyzer.go       # Streaming counts + top-K min-heap grouping
│   ├── renderer/
│   │   ├── console.go        # ANSI-colored terminal output
│   │   ├── json.go           # JSON / NDJSON export
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
│       ├── embed.go          # go:embed static directive
│       └── static/
│           ├── index.html    # SPA shell
│           ├── app.js        # Vanilla ES module (~1078 lines)
│           └── style.css     # CSS custom properties (~1209 lines)
└── testdata/
    └── samples/              # errors.log, syslog.log, apache.log
```

## Data Model

### `internal/model/event.go`

**Level** is `int8` with non-iota values. Higher numeric value = higher severity:

| Constant | Value | Aliases |
|---|---|---|
| `LevelDebug` | -1 | DEBUG, TRACE |
| `LevelInfo` | 0 | INFO |
| `LevelWarn` | 1 | WARN, WARNING |
| `LevelError` | 2 | ERROR |
| `LevelFatal` | 3 | FATAL, CRITICAL, PANIC |

- `Level` implements `json.Marshaler`/`json.Unmarshaler` — serializes as uppercase strings (`"ERROR"`, `"INFO"`)
- `ParseLevel(s string) (Level, bool)` — case-insensitive lookup
- `DetectLevel(raw string) (Level, bool)` — searches raw string with word boundaries

**Event** — a single parsed log line:

```go
type Event struct {
    Timestamp time.Time `json:"timestamp"`
    Level     Level     `json:"level"`
    Source    string    `json:"source"`   // filename or "stdin"
    Message   string    `json:"message"`  // extracted message content
    Raw       string    `json:"raw"`      // original line
    LineNum   int       `json:"line"`     // 1-based (JSON key "line", not "line_num")
}
```

**Group** — a normalized error group:

```go
type Group struct {
    Signature     string    `json:"signature"`       // normalized message
    SampleMessage string    `json:"sample"`          // first raw message (not normalized)
    Count         int       `json:"count"`
    FirstSeen     time.Time `json:"first_seen"`
    LastSeen      time.Time `json:"last_seen"`
    Index         int       `json:"-"`               // heap position (unexported from JSON)
}
```

**Report** — complete analysis output:

```go
type Report struct {
    Source     string         `json:"source"`                // filename or "stdin"
    TotalLines int            `json:"total_lines"`
    Levels     map[Level]int  `json:"-"`                     // internal map
    LevelsStr  map[string]int `json:"levels"`                // serialized as strings
    TopErrors  []Group        `json:"top_errors,omitempty"`
    FirstLine  time.Time      `json:"first_line"`
    LastLine   time.Time      `json:"last_line"`
}
```

- `Levels` (keyed by `Level`) is internal; `LevelsStr` (keyed by string) is what gets serialized
- Custom `MarshalJSON`/`UnmarshalJSON` converts between the two

## Streaming Pipeline

Every component is channel-based and processes one line at a time. No full file is ever loaded into memory.

### 1. Reader — `reader.ReadLines(paths []string, stdin bool) chan Line`

- Glob expansion via `filepath.Glob`; if no matches, uses pattern as literal path
- Binary detection: reads first 8192 bytes, checks for null byte (`\x00`) — silently skips
- Scanner buffer: 1 MB initial, 1 MB max line length
- Returns lines on a channel; closing the channel signals EOF

### 2. Parser — `parser.ParseLine(raw string, lineNum int, source string) model.Event`

- Returns `model.Event` (value type, not pointer)
- Timestamp detection (ordered by priority):
  1. ISO 8601 / RFC 3339 (`2026-07-08T10:12:33Z`)
  2. Date + space (`2026-07-08 10:12:33`)
  3. Syslog BSD (`Jul  8 10:12:33`)
  4. Apache CLF (`08/Jul/2026:10:12:33 +0000`)
  5. Unix epoch seconds (`1720421553`)
- Syslog dates use current year; adjusts back 1 year if > 24h in future
- Level detection: ordered keyword match with word boundaries
- Message extraction: removes timestamp prefix, strips level keyword, cleans leading `:-[]()` characters

### 3. Filter — `filter.Matches(evt model.Event, cfg Config) bool`

```go
type Config struct {
    Regex    *regexp.Regexp
    MinLevel model.Level
    Since    time.Time
    Until    time.Time
}
```

- Level: `cfg.MinLevel > LevelDebug && evt.Level < cfg.MinLevel`
- Regex: matched against `evt.Raw` (full line), not `evt.Message`
- Time: zero-value timestamps pass through
- All conditions must pass (AND logic)
- Value semantics for both `Event` and `Config`

### 4. Analyzer — `analyzer.Analyze(events <-chan model.Event, limit int) Report`

- Counts all events by level
- Tracks time range (min/max timestamp across all events)
- For error-level events (`evt.Level >= LevelError`): normalizes message via `normalizer.Normalize`, tracks top-K groups
- Group lookup is O(1) via `map[string]*Group`
- Top-K tracking uses a **min-heap** (`container/heap`) bounded by `limit`:
  - When heap is full and a new group has a higher count than the minimum, the min is evicted
  - Uses `heap.Fix` for existing group count updates (re-heapifies after increment)
- Final groups sorted descending by count
- Events at `LevelInfo` and below are counted but never grouped

### 5. Renderer — three formats

| Package | Function | Output |
|---|---|---|
| `renderer/console.go` | `PrintReport`, `PrintTop`, `PrintErrors`, `PrintGrep`, `PrintAISummary` | ANSI-colored terminal |
| `renderer/json.go` | `PrintReportJSON`, `PrintEventsJSON`, `PrintEventJSON` | JSON / NDJSON |
| `renderer/csv.go` | `PrintScanCSV`, `PrintEventsCSV` | CSV |

- `PrintReportJSON` → single JSON object (used by `scan`, `top`)
- `PrintEventsJSON` → NDJSON, one object per line (used by `errors`, `grep`)
- CLI AI summary output strips markdown formatting for terminal display

## Normalization Engine

**Location:** `internal/normalizer/normalizer.go`

Applied in order to the `Message` field of error-level events during Analyzer grouping. Only error-level events are normalized (gated on `LevelError`).

| Step | Pattern | Replaces | Example |
|---|---|---|---|
| 1 | UUID (8-4-4-4-12 hex) | `<uuid>` | `a1b2c3d4-...` → `<uuid>` |
| 2 | Request IDs (`req_`, `request_`, `trace_`, `span_` + 8+ alphanum) | `<req>` | `req_abc123` → `<req>` |
| 3 | IPv6 (full or `::1`) | `<ip>` | `2001:db8::1` → `<ip>` |
| 4 | IPv4 (dotted decimal) | `<ip>` | `10.0.0.5` → `<ip>` |
| 5 | Hex (`0x` prefix) | `<hex>` | `0xabcd1234` → `<hex>` |
| 6 | File paths (`/.../...`) | `<path>` | `/var/log/app.log` → `<path>` |
| 7 | Hashes (40+ hex chars) | `<hash>` | `da39a3ee5e6b...` → `<hash>` |
| 8 | Standalone numbers | `<n>` | `5432` → `<n>` |

**Design decisions:**
- Order matters: UUID before hex/numbers prevents partial matches
- File paths before numbers prevents path components from being consumed separately
- Most patterns use `\b` word boundaries; file path pattern uses its own delimiter-based matching

## AI Summarizer

**Location:** `internal/summarizer/`

**Interface:**

```go
type Summarizer interface {
    Summarize(ctx context.Context, req SummaryRequest) (*Summary, error)
    SummarizeStream(ctx context.Context, req SummaryRequest) (<-chan string, error)
}
```

**Two implementations:**

| Implementation | When used | Behavior |
|---|---|---|
| `noop` (default) | No `--ai-endpoint` configured | Returns static "not configured" message; zero weight |
| `llm` | `--ai-endpoint` or `LOGANALYZE_AI_ENDPOINT` set | POSTs to OpenAI-compatible `/chat/completions` |

**LLM implementation details:**
- API key from `LOGANALYZE_AI_KEY` env var (not from flags)
- Prompt built from normalized error groups + level distribution (not raw lines)
- System prompt: "You are a log analysis assistant. Be concise and direct."
- Temperature: 0.3, MaxTokens: 1000
- Sync timeout: 60s, Stream timeout: 120s
- Auto-appends `/chat/completions` to endpoint URL
- SSE streaming: reads `data: ` lines, stops on `[DONE]`

**Integration:**
- CLI: `scan` and `top` call `Summarize` after analysis if endpoint is set
- Server: `GET /api/insights/{id}` (sync) and `GET /api/insights/{id}/stream` (SSE)
- Results cached on session after first generation

## Server Architecture

**Location:** `internal/server/`

### Router

Uses Go 1.22 `http.ServeMux` with method-based routing:

```go
mux.HandleFunc("POST /api/upload", ...)
mux.HandleFunc("POST /api/analyze/{id}", ...)
mux.HandleFunc("GET /api/results/{id}", ...)
mux.HandleFunc("GET /api/results/{id}/events", ...)
mux.HandleFunc("GET /api/status/{id}", ...)
mux.HandleFunc("GET /api/sessions", ...)
mux.HandleFunc("DELETE /api/sessions/{id}", ...)
mux.HandleFunc("GET /api/uploaded/{id}", ...)
mux.HandleFunc("GET /api/insights/{id}", ...)
mux.HandleFunc("GET /api/insights/{id}/stream", ...)
mux.HandleFunc("GET /health", ...)
```

Static files served at `/static/` via embedded FS; root `/` serves `index.html`.

### Middleware stack

The server applies a middleware chain to all routes:
- **Panic recovery**: catches panics per-request, returns `500` with JSON error, logs stack trace
- **Request ID**: injects `X-Request-ID` header (16 hex chars) if not provided by client
- **Logging**: logs `remote method path status duration request_id` for every request
- **Rate limiting**: sliding window per-client-IP, default 10 requests/min, returns `429 Too Many Requests` with `Retry-After` header

### Signal handling

The server implements **graceful shutdown** via `signal.NotifyContext` for `SIGINT`/`SIGTERM`. On signal:
1. A 30-second shutdown timeout context is created
2. `http.Server.Shutdown()` is called, which drains active connections
3. Background goroutines (session cleanup, AI summary generation) receive context cancellation

### Session Management

**Location:** `internal/session/session.go`

- In-memory `map[string]*Session` protected by `sync.RWMutex`
- Session ID: 16 random bytes → 32-char hex string
- TTL: 1 hour since **last access** (touched on every `Store.Get()`)
- Cleanup runs every 10 minutes in a background goroutine
- Status values: `"uploaded"` → `"running"` → `"complete"` or `"error"`

### Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/api/upload` | POST | Multipart file upload (max 100 MB), returns `session_id` |
| `/api/analyze/{id}` | POST | Start analysis with command/flags; launches goroutine |
| `/api/results/{id}` | GET | Report JSON + events + summary (if complete) |
| `/api/results/{id}/events` | GET | Paginated events (offset/limit, max 1000) |
| `/api/status/{id}` | GET | SSE progress stream (polls every 500ms) |
| `/api/sessions` | GET | List active sessions (id, file_name, status, created_at) |
| `/api/sessions/{id}` | DELETE | Remove session + uploaded file |
| `/api/uploaded/{id}` | GET | Download original uploaded file |
| `/api/insights/{id}` | GET | AI summary (sync, cached) |
| `/api/insights/{id}/stream` | GET | AI summary (SSE streaming, cached after complete) |
| `/health` | GET | `{"status":"ok"}` |

## Web UI

**Location:** `internal/web/static/`

### Stack
- **HTML:** Single-page application shell with inline theme detection
- **CSS:** Design system with custom properties, no preprocessor (~1209 lines)
- **JS:** Vanilla ES module with zero dependencies (~1078 lines)

### Architecture
- Hash-based routing: `#/` (dashboard), `#/upload`, `#/session/:id`
- Event bus via custom events on `document`
- State tracking via module-level variables (not a framework)

### Pages

| Route | Content |
|---|---|
| `#/` | Dashboard — stat cards, session table |
| `#/upload` | Upload — drag-and-drop, command config, progress SSE |
| `#/session/:id` | Session detail with 4 tabs |

### Session Detail Tabs

| Tab | Content |
|---|---|
| **Overview** | Stat cards, SVG bar chart, error group accordion |
| **Events** | Filterable, paginated event list (100/page) |
| **AI Insights** | Streaming markdown via `EventSource` |
| **Raw** | Line-numbered original file |

### Theme
- Detected via `prefers-color-scheme` on first visit
- Toggled via sidebar button, persisted to `localStorage`
- Dark theme is default; light theme available

## Concurrency Model

| Component | Mechanism |
|---|---|
| Pipeline (Reader → Analyzer) | Go channels (`chan Line`, `chan Event`) |
| Server analysis | Background goroutine per session |
| Session store | `sync.RWMutex` protecting `map[string]*Session` |
| Session cleanup | `time.Ticker` in background goroutine |
| AI summary | Background goroutine (server) or sync (CLI) |

## Docker Deployment

- **Builder:** `golang:1.22`, `CGO_ENABLED=0`, `-ldflags="-s -w"`
- **Runtime:** `alpine:3.20` with `ca-certificates` + `tzdata`
- **Image size:** ~7.6 MB
- **Default command:** `serve --addr :8080 --data /data`
- **Docker Compose mapping:** external `8081` → internal `8080`
- **Volume:** named `logdata` at `/data` for persistence

## Design Decisions

| Decision | Rationale |
|---|---|
| Streaming-first | Handles GB-sized logs in low memory; no full-file loading |
| Value types (not pointers) | Events are small; avoids heap allocation and GC pressure |
| Normalizer only on error-level | Non-error events pass through the pipeline untouched |
| O(1) map for group lookup | More efficient than O(n) linear scan for large limits |
| No external HTTP router | Go 1.22 `http.ServeMux` is sufficient for the API surface |
| No middleware | Keeps server minimal; middleware can be added when needed |
| No signal handling | Server is typically deployed behind a reverse proxy that manages lifecycle |
| Vanilla JS (no framework) | Zero dependencies, small bundle, no build step |
