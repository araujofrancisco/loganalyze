# Log Analyzer — Technical Specification

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Data Model](#2-data-model)
3. [Core Functions](#3-core-functions)
4. [Streaming Pipeline](#4-streaming-pipeline)
5. [Normalization Engine](#5-normalization-engine)
6. [Analyzer Implementation](#6-analyzer-implementation)
7. [AI Summarizer](#7-ai-summarizer)
8. [Server API](#8-server-api)
9. [Web UI Architecture](#9-web-ui-architecture)
10. [Docker Deployment](#10-docker-deployment)
11. [Testing Strategy](#11-testing-strategy)

---

## 1. Architecture Overview

```
CLI mode:   Reader → Parser → Filter → Analyzer → Renderer
Server mode: Browser → HTTP API → Server Handlers → Engine (same internal/)
```

Both modes share the same streaming pipeline from `internal/` — no code duplication.

---

## 2. Data Model

### Location: `internal/model/event.go`

**Level** is `int8` with non-iota values. Higher numeric value = higher severity:

```go
const (
    LevelDebug Level = -1  // lowest severity
    LevelInfo  Level = 0
    LevelWarn  Level = 1
    LevelError Level = 2
    LevelFatal Level = 3   // highest severity
)
```

**Level ordering:** filters check `evt.Level < cfg.MinLevel` — lower numeric value = less severe. Since `LevelDebug` is -1 and `LevelFatal` is 3, the comparison `evt.Level < cfg.MinLevel` correctly keeps events at or above the minimum level.

**Aliases:** CRITICAL and PANIC map to Fatal; WARNING maps to Warn; TRACE maps to Debug.

**JSON serialization:** Level implements `json.Marshaler` and `json.Unmarshaler` to serialize as uppercase strings (`"ERROR"`, `"INFO"`). On deserialization, unrecognized levels default to `LevelInfo`.

### Key structs

```go
type Event struct {
    Timestamp time.Time `json:"timestamp"`
    Level     Level     `json:"level"`
    Source    string    `json:"source"`   // filename or "stdin"
    Message   string    `json:"message"`  // extracted message content
    Raw       string    `json:"raw"`      // original line
    LineNum   int       `json:"line"`     // 1-based (JSON key "line")
}

type Group struct {
    Signature     string    `json:"signature"`       // normalized message
    SampleMessage string    `json:"sample"`          // first raw message (not normalized)
    Count         int       `json:"count"`
    FirstSeen     time.Time `json:"first_seen"`
    LastSeen      time.Time `json:"last_seen"`
    Index         int       `json:"-"`               // heap position (internal)
}

type Report struct {
    Source     string         `json:"source"`
    TotalLines int            `json:"total_lines"`
    Levels     map[Level]int  `json:"-"`              // internal map
    LevelsStr  map[string]int `json:"levels"`         // serialized as strings
    TopErrors  []Group        `json:"top_errors,omitempty"`
    FirstLine  time.Time      `json:"first_line"`
    LastLine   time.Time      `json:"last_line"`
}
```

`Report` has a custom `MarshalJSON` that populates `LevelsStr` from `Levels` before serializing. `UnmarshalJSON` reverses the conversion. This allows the internal map to use `Level` keys while the JSON uses strings.

---

## 3. Core Functions

Every package defines a single exported function (not an interface), except `summarizer` which uses an interface:

| Package | Signatures | Purpose |
|---|---|---|
| `reader.ReadLines` | `(paths []string, stdin bool) → chan Line` | Read line-by-line with line numbers |
| `parser.ParseLine` | `(raw string, lineNum int, source string) → model.Event` | Extract level, timestamp, message |
| `normalizer.Normalize` | `(string) → string` | Replace IDs/paths with tokens |
| `filter.Matches` | `(evt model.Event, cfg Config) → bool` | Level/time/regex filter (value semantics) |
| `analyzer.Analyze` | `(events <-chan model.Event, limit int) → Report` | Streaming analysis (value semantics) |
| `summarizer.Summarizer` | Interface with `Summarize` and `SummarizeStream` | AI-powered error analysis |
| `renderer.Print*` | `(model.Report or <-chan model.Event, io.Writer)` | Console/JSON/CSV output |

---

## 4. Streaming Pipeline

### Input discovery — `reader.ReadLines`

```go
func ReadLines(paths []string, stdin bool) chan Line
```

- If `stdin` is true or `paths` is empty, reads from `os.Stdin`
- Otherwise, expands glob patterns via `filepath.Glob`; if no matches, uses pattern as literal path
- Silently skips missing files and binary files
- Binary detection: reads first 8192 bytes, checks for null byte (`\x00`)
- Scanner buffer: 1 MB initial, 1 MB max line length

### Parser — `parser.ParseLine`

```go
func ParseLine(raw string, lineNum int, source string) model.Event
```

- Returns `model.Event` (value type, not pointer)
- Extracts timestamp using ordered regex patterns (ISO 8601 → date+space → syslog → Apache CLF → epoch)
- Extracts level using ordered keyword match with word boundaries
- Extracts message: removes timestamp prefix, strips level keyword, cleans leading `:-[]()` characters
- Falls back to entire raw line if no message can be extracted
- Syslog dates use current year; adjusts back 1 year if > 24h in future

### Filter — `filter.Matches`

```go
func Matches(evt model.Event, cfg Config) bool
```

```go
type Config struct {
    Regex    *regexp.Regexp
    MinLevel model.Level
    Since    time.Time
    Until    time.Time
}
```

- Level: `cfg.MinLevel > LevelDebug && evt.Level < cfg.MinLevel` (skips debug when any filter is set)
- Regex: matched against `evt.Raw` (full line), not `evt.Message`
- Time: zero-value timestamps pass through
- All conditions must pass (AND logic)
- Value semantics for both `Event` and `Config`

### Analyzer — `analyzer.Analyze`

```go
func Analyze(events <-chan model.Event, limit int) Report
```

- Counts all events by level
- Tracks time range (min/max timestamp across all events)
- For error-level events (`evt.Level >= LevelError`): normalizes message and tracks top-K groups
- Group lookup is O(1) via `map[string]*Group`
- Uses a min-heap for top-K tracking (bounded by `limit`; if `limit == 0`, unbounded)
- Final groups sorted descending by count

---

## 5. Normalization Engine

### Location: `internal/normalizer/normalizer.go`

**Purpose:** Convert error lines to a canonical form so near-identical errors collapse into one group.

### Replacement order and patterns

| Step | Pattern | Replacement | Example |
|---|---|---|---|
| 1 | UUID (8-4-4-4-12 hex) | `<uuid>` | `a1b2c3d4-...` → `<uuid>` |
| 2 | Request ID (`req_`, `request_`, `trace_`, `span_` + 8+ alphanum) | `<req>` | `req_abc123` → `<req>` |
| 3 | IPv6 (full or `::1`) | `<ip>` | `2001:db8::1` → `<ip>` |
| 4 | IPv4 (dotted decimal) | `<ip>` | `10.0.0.5` → `<ip>` |
| 5 | Hex (`0x` prefix) | `<hex>` | `0xabcd1234` → `<hex>` |
| 6 | File path (`/.../...`) | `<path>` | `/var/log/app.log` → `<path>` |
| 7 | Hash (40+ hex chars) | `<hash>` | `da39a3ee5e6b4b0d...` → `<hash>` |
| 8 | Standalone number | `<n>` | `5432`, `342` → `<n>` |

### Design decisions

- **Order matters:** UUID before hex/numbers prevents partial matches. File paths before numbers prevents path components (like line numbers) from being consumed separately.
- **Only error-level:** normalization is gated on `LevelError` and above. Non-error events are never normalized.
- **Boundary rules:** Most patterns use `\b` word boundaries. The file path pattern uses its own delimiter-based matching (`/`-starting paths).

---

## 6. Analyzer Implementation

### Location: `internal/analyzer/analyzer.go`

### Event counting

```go
counts := make(map[model.Level]int)
for evt := range events {
    counts[evt.Level]++
}
```

### Top-K error grouping

Uses a **min-heap** (`container/heap`) bounded to `limit` with O(1) map lookup:

```go
groups := make(map[string]*Group)
gh := &groupHeap{}

for evt := range events {
    r.TotalLines++
    r.Levels[evt.Level]++

    if evt.Level < model.LevelError {
        continue // only group errors
    }

    sig := normalizer.Normalize(evt.Message)
    if g, ok := groups[sig]; ok {
        g.Count++
        if evt.Timestamp.After(g.LastSeen) {
            g.LastSeen = evt.Timestamp
        }
        heap.Fix(gh, g.Index)
    } else if gh.Len() < limit || limit == 0 {
        heap.Push(gh, g)
        groups[sig] = g
    } else if g.Count > (*gh)[0].Count {
        // evict min, add new
    }
}
```

### Group heap

`groupHeap` implements `container/heap.Interface`. Min-heap ordered by Count ascending. `Push` sets `g.Index = n` and `Pop` sets `g.Index = -1` for map management.

### Design decisions

- **Map-based lookup (O(1)):** Tracks groups in `map[string]*Group` for immediate access, not O(n) linear scan.
- **Min-heap, not max-heap:** When full, the smallest-count group sits at position 0 and is evicted first.
- **heap.Fix for updates:** When an existing group's count is incremented, `heap.Fix` re-heapifies in O(log n).
- **No deduplication during counting:** level counts include all events regardless of grouping.
- **Events are not stored in Report:** The `Report` struct has no `Events` field. Events are returned separately by server handlers for `errors`/`grep` commands.

---

## 7. AI Summarizer

### Location: `internal/summarizer/`

**Purpose:** Generate an AI-powered natural-language analysis of error patterns. Two implementations: noop (default) and LLM (OpenAI-compatible HTTP).

### Interface

```go
type Summarizer interface {
    Summarize(ctx context.Context, req SummaryRequest) (*Summary, error)
    SummarizeStream(ctx context.Context, req SummaryRequest) (<-chan string, error)
}
```

### `SummaryRequest`

| Field | Type | Description |
|---|---|---|
| `Source` | string | Filename or "stdin" |
| `TotalLines` | int | Lines processed |
| `Levels` | map[string]int | Level distribution (ERROR, WARN, etc.) |
| `TimeRange` | string | Human-readable time range |
| `TopErrors` | []ErrorGroupSummary | Up to `--limit` normalized error groups |

### LLM implementation (`llm.go`)

- Constructs a prompt with level distribution and top error groups (not raw lines)
- POSTs to `{endpoint}/chat/completions` (OpenAI-compatible API)
- System prompt: "You are a log analysis assistant. Be concise and direct."
- Supports both sync and SSE streaming via `stream: true`
- API key sent as `Authorization: Bearer` header from `LOGANALYZE_AI_KEY` env var
- Temperature: 0.3, MaxTokens: 1000
- Sync timeout: 60s (http.Client) + context-based timeout
- Stream timeout: 120s (context-based)
- Defaults to noop when no endpoint is configured

### Integration

- **CLI:** `scan` and `top` commands call `Summarize` after analysis if `--ai-endpoint` is set
- **Server:** `GET /api/insights/{id}` (sync, 60s timeout) and `GET /api/insights/{id}/stream` (SSE, 120s timeout); results cached on session
- **Web UI:** AI Insights tab streams markdown content via `EventSource` and renders it client-side
- Background summary: server also generates summary automatically after analysis completes via `generateSummary` goroutine

### Security

- API key read from `LOGANALYZE_AI_KEY` env var only (not from flags)
- Shared `NewSummaryRequestFromReport` helper avoids duplication between CLI and server

---

## 8. Server API

### Location: `internal/server/handlers.go`, `internal/server/server.go`

### Session management — `internal/session/session.go`

- In-memory `map[string]*Session` protected by `sync.RWMutex`
- Session ID: 16 random bytes → 32-char hex string (not UUID)
- Cleanup every 10 minutes via background goroutine with `time.Ticker`
- TTL per session: 1 hour since **creation** (not last access)
- Status values: `"uploaded"` → `"running"` → `"complete"` or `"error"`

### Router

Uses Go 1.22 `http.ServeMux` with method-based routing. The server applies a middleware chain: panic recovery → request ID injection → request logging → rate limiting. All API errors return structured JSON (`{"error":"message"}`).

### Graceful shutdown

The server implements graceful shutdown via `signal.NotifyContext` for `SIGINT`/`SIGTERM`. On signal, active connections are drained with a 30-second timeout via `http.Server.Shutdown`. Background goroutines receive context cancellation.

### Rate limiting

A sliding-window rate limiter per client IP (default 10 requests/minute) applies to all endpoints. When exceeded, the server returns `429 Too Many Requests` with a `Retry-After` header. The rate limit can be configured via the `WithRateLimit` option.

### Endpoints

#### `POST /api/upload`

- Accepts multipart form with field `file`
- Max file size: 100 MB via `http.MaxBytesReader`
- Copies file to `{dataDir}/{id}.log` (random 16-byte hex filename)
- Returns JSON: `{"session_id": "..."}` (201 Created)

#### `POST /api/analyze/{id}`

- Body: `{"command":"scan","level":"error","regex":"","limit":10,"since":"","until":""}`
- Validates session exists, sets status to `"running"`
- Launches analysis in a background goroutine
- Returns 202 with `{"status":"running"}`
- `command` determines analysis mode: `scan`/`top` (grouped via analyzer), `errors`/`grep` (collect all events)
- `errors` and `top` force `MinLevel` to `LevelError` regardless of request

#### `GET /api/results/{id}`

- Returns JSON with `status`, `command`, and if complete: `report`, `events`, `summary`
- Returns 200 even for incomplete — caller should check `status` field
- Returns 404 if session not found

#### `GET /api/results/{id}/events`

- Query params: `offset` (default 0), `limit` (default 100, max 1000)
- Returns paginated events with total count
- Response shape: `{"events":[...],"total":342,"offset":0,"limit":100}`
- Returns 200 with empty array if no events (rather than 400)

#### `GET /api/status/{id}`

- Server-Sent Events stream: `event: progress\ndata: {"status":"running","progress":"..."}\n\n`
- Events: `progress` (during), `complete` (on finish), `error` (on failure)
- Polls every 500ms while streaming

#### `GET /api/sessions`

- Lists all active sessions with id, file_name, status, created_at
- Response: `{"sessions":[...]}`

#### `DELETE /api/sessions/{id}`

- Deletes session and uploaded file from disk
- Returns 200 with `{"deleted": true}`
- Idempotent

#### `GET /api/uploaded/{id}`

- Returns the raw uploaded file as `text/plain; charset=utf-8`
- Returns 404 if session not found

#### `GET /health`

- Returns `{"status":"ok"}` (200)

#### `GET /api/insights/{id}`

- Returns `{"summary":"...", "model":"...", "cached":true|false}`
- Returns cached result if already generated
- Returns 501 if summarizer not configured
- Returns 409 if analysis is not yet complete

#### `GET /api/insights/{id}/stream`

- Server-Sent Events stream of markdown content
- Sends tokens as `data: {"type":"text","content":"..."}\n\n`
- Sends `event: complete` with `{"type":"done"}` when finished
- Sends `event: error` with `{"type":"error","content":"..."}` on failure
- Caches the complete response on session after streaming finishes
- Returns cached text immediately if already generated

### Background analysis

```go
func (s *Server) runAnalysis(ses *session.Session) {
    // reads file, parses lines, filters, dispatches to scan/top or errors/grep
    if s.summarizer != nil && ses.Report != nil {
        go s.generateSummary(ses)
    }
}
```

- File existence checked before processing
- Progress reported every 1000 parsed lines
- AI summary generated in background after analysis completes (if configured)

---

## 9. Web UI Architecture

### Location: `internal/web/static/`

### Stack

- **HTML:** Single-page application shell with inline theme detection
- **CSS:** Design system with custom properties, no preprocessor (~1209 lines)
- **JS:** Vanilla ES module with zero dependencies (~1078 lines)

### CSS Design System

- ~40+ custom properties for colors, spacing, typography
- Dark theme (default) and light theme via `[data-theme]` attribute on `<html>`
- Layout: CSS Grid with fixed sidebar (260px) and scrollable main area
- Responsive: collapses sidebar to top nav at < 768px
- Respects `prefers-reduced-motion`

### JS Architecture

- Module pattern via IIFE returning public API / module-level functions
- Routing: hash-based (`location.pathname`)
- State: module-level variables (not `window.__state`)
- Event bus: custom events on `document` for cross-component communication

#### Pages

| Route | Handler | Description |
|---|---|---|
| `#/` | `renderDashboard()` | Stats cards, session table, delete |
| `#/upload` | `renderUpload()` | Drag-and-drop, command config, progress SSE |
| `#/session/:id` | `renderSession(id)` | 4-tab detail view |

#### Session detail tabs

1. **Overview** — stat cards, SVG bar chart (level breakdown with percentage labels), collapsible error group accordion
2. **Events** — level dropdown filter, text search, paginated table (100/page), expandable raw line rows
3. **AI Insights** — streaming markdown via `EventSource(/api/insights/{id}/stream)` with markdown rendering (headings, lists, bold, italic, code, horizontal rules)
4. **Raw** — line-numbered original file via `GET /api/uploaded/{id}`

### Theme

- Detected on first visit via `prefers-color-scheme`
- Toggled via sidebar button, persisted to `localStorage` key `theme`
- If no stored preference, follows system preference

### Keyboard shortcuts

| Shortcut | Action |
|---|---|
| `⌘1` / `Ctrl+1` | Navigate to dashboard |
| `⌘U` / `Ctrl+U` | Navigate to upload |

### Toast notification system

- Queue-based: `showToast(message, type)` creates auto-dismissed `<div class="toast">`
- Types: `info`, `error`, `success`
- Each toast auto-dismisses after 4 seconds

---

## 10. Docker Deployment

### `Dockerfile`

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/loganalyze ./main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/loganalyze /usr/local/bin/loganalyze
EXPOSE 8080
ENTRYPOINT ["loganalyze"]
CMD ["serve", "--addr", ":8080", "--data", "/data"]
```

Key decisions:
- Multi-stage to minimize image size (~7.6 MB)
- `CGO_ENABLED=0` for static binary
- `-ldflags="-s -w"` strips debug symbols
- Alpine runtime with `ca-certificates` and `tzdata` for HTTPS and timezone support

### `docker-compose.yml`

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

- External port 8081 mapped to internal 8080
- Named volume for uploaded file persistence
- UTC timezone for consistent timestamp handling

---

## 11. Testing Strategy

### Unit tests

Each `internal/` package has a `*_test.go` file:

| Package | What's tested |
|---|---|
| `model` | Level parsing, JSON roundtrip, time comparison |
| `reader` | Line reading, binary detection, stdin handling |
| `parser` | Each format/timestamp/level pattern, edge cases |
| `normalizer` | Each replacement rule, ordering, boundary cases |
| `filter` | Level, regex, time-range filtering |
| `analyzer` | Counting, grouping, min-heap behavior, limit |
| `renderer` | Console/JSON/CSV format output |
| `session` | CRUD, TTL expiry, concurrent access |
| `server` | HTTP handler integration |
| `summarizer` | Prompt building, LLM sync/stream, HTTP error handling, auth, timeout, noop fallback |

### Test data

`testdata/samples/` contains three sample log files:

| File | Format | Lines | Description |
|---|---|---|---|
| `errors.log` | Mixed timestamps | 15 | Error lines with UUIDs, IPs, paths for normalization testing |
| `syslog.log` | BSD syslog | 7 | Kernel messages with various levels |
| `apache.log` | Apache CLF | 10 | Web access logs with mixed status codes |

### Running tests

```bash
go test ./...
```

### Test conventions

- All test helpers use `t.Helper()`
- Table-driven tests where appropriate
- No external test dependencies (no test frameworks, no fixtures server)
- Sample files are committed alongside source code
- Golden files are used for renderer output comparison

---

## Appendix A: Glossary

| Term | Definition |
|---|---|
| **CLI** | Command-line interface (terminal application) |
| **Event** | A single parsed log line with timestamp, level, message |
| **Group** | A collection of error events with the same normalized signature |
| **Message** | The extracted textual content of a log line (excluding timestamp and level) |
| **Normalizer** | Component that replaces variable data (IDs, IPs, numbers) with tokens |
| **Report** | The complete analysis output (counts, groups, time range) |
| **Session** | Server-side state for an uploaded file + its analysis results |
| **Signature** | A normalized message string used for grouping identical errors |
| **SSE** | Server-Sent Events — unidirectional event stream for progress updates |
| **SPA** | Single-Page Application — client-side routing and rendering |

## Appendix B: Edge Cases

| Scenario | Behavior |
|---|---|
| Empty file | Returns report with zero counts, no errors |
| Binary file | Silently skipped (no output), no error |
| No timestamps found | All events get zero-value time.Time (0001-01-01) |
| No level keyword | Defaults to INFO |
| Duplicate uploads | Each upload creates a new session with a new hex ID |
| Concurrent analysis | Each session runs analysis in its own goroutine |
| Session expiry | Cleanup runs every 10 minutes, removes sessions older than 1h since creation |
| Server restart | All sessions are lost (in-memory store, no persistence) |
| Large files (>100MB) | Rejected at upload with 413 Payload Too Large |
| No events for command | scan/top return empty events array (server returns 200 with empty array) |
| Missing file in DELETE | No error — idempotent |
| Partial analysis on shutdown | Results are discarded; client gets 404 on next fetch |
| No signal handling | SIGINT/SIGTERM terminates immediately (no graceful shutdown) |

## Appendix C: HTTP Status Codes

| Code | Usage |
|---|---|
| 200 | Successful GET/POST response with body |
| 201 | Upload successful (session created) |
| 202 | Analysis started (async) |
| 400 | Bad request (invalid command, missing file, missing field) |
| 404 | Session not found, file not found |
| 409 | Analysis not complete yet |
| 413 | Upload too large (>100 MB) |
| 429 | Rate limit exceeded |
| 500 | Internal server error |
| 501 | AI summarizer not configured |
