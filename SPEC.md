# Log Analyzer — Technical Specification

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Data Model](#2-data-model)
3. [Core Interfaces](#3-core-interfaces)
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
┌─────────────────────────────────────────────────────────┐
│                      main.go                            │
│  cobra.Command tree: root, scan, errors, top, grep,    │
│  serve                                                  │
└──────────┬──────────────────────────────────────────────┘
           │
     ┌─────┴───────┬──────────────────────────────┐
     │             │                              │
     ▼             ▼                              ▼
┌──────────┐  ┌──────────┐                 ┌──────────┐
│ CLI mode │  │ Serve    │                 │ Shared        │
│          │  │ mode     │                 │ Flags         │
│ scan,    │  │ HTTP     │                 │ --since       │
│ errors,  │  │ server   │                 │ --until       │
│ top,     │  │ with web │                 │ --level       │
│ grep     │  │ UI       │                 │ --json        │
│          │  │ + AI     │                 │ --ai-endpoint │
└──────────┘  └──────────┘                 │ --csv    │
                                           │ --no-col │
                                           │ --limit  │
                                           └──────────┘
```

Both modes share the same streaming pipeline from `internal/` — no code duplication.

---

## 2. Data Model

### Location: `internal/model/event.go`

```go
const (
    LevelFatal Level = iota  // 0 — highest severity
    LevelError               // 1
    LevelWarn                // 2
    LevelInfo                // 3
    LevelDebug               // 4 — lowest severity
)
```

**Level ordering:** numeric value defines severity (`LevelFatal` = highest). Filters check `evt.Level >= cfg.MinLevel` where the comparison operator means "at least as severe as" (since lower numeric value = higher severity).

**JSON serialization:** Level implements `json.Marshaler` and `json.Unmarshaler` to serialize as uppercase strings (`"ERROR"`, `"INFO"`).

```go
func (l Level) MarshalJSON() ([]byte, error) {
    return json.Marshal(l.String())
}
func (l *Level) UnmarshalJSON(b []byte) error {
    var s string
    if err := json.Unmarshal(b, &s); err != nil {
        return err
    }
    *l = levelFromString(s)
    return nil
}
```

This is required for the Web UI's JSON responses — the frontend expects Level as strings, not integers.

### Key structs

```go
type Event struct {
    Timestamp time.Time `json:"timestamp,omitempty"`
    Level     Level     `json:"level"`
    Source    string    `json:"source"`     // filename or "stdin"
    Message   string    `json:"message"`    // extracted message
    Raw       string    `json:"raw"`        // original line
    LineNum   int       `json:"line_num"`
}

type Group struct {
    Signature string   `json:"signature"`  // normalized message
    Count     int      `json:"count"`
    FirstSeen string   `json:"first_seen"` // ISO 8601
    LastSeen  string   `json:"last_seen"`
    Sample    string   `json:"sample"`     // first raw error line
}

type Report struct {
    Filename    string            `json:"filename"`
    TotalLines  int               `json:"total_lines"`
    LevelCounts map[string]int    `json:"level_counts"`
    Errors      []Group           `json:"errors,omitempty"`
    Events      []Event           `json:"events,omitempty"`
    StartTime   string            `json:"start_time,omitempty"`  // min timestamp
    EndTime     string            `json:"end_time,omitempty"`    // max timestamp
    Duration    string            `json:"duration,omitempty"`    // human duration
	Command     string            `json:"command,omitempty"`     // "scan", "errors", "top", "grep"
}
```

---

## 3. Core Interfaces

Every package defines a single exported function (not an interface):

| Package | Signatures | Purpose |
|---|---|---|
| `reader.ReadLines` | `(io.Reader, int) → chan Line` | Read line-by-line with line numbers |
| `parser.ParseLine` | `(int, string, string) → *Event` | Extract level, timestamp, message |
| `normalizer.Normalize` | `(string) → string` | Replace IDs/paths with tokens |
| `filter.ShouldKeep` | `(*Event, *Config) → bool` | Level/time/regex filter |
| `analyzer.Analyze` | `(chan *Event, *Config) → *Report` | Streaming analysis |
| `summarizer.Summarize` | `(context, SummaryRequest) → *Summary` | AI-powered error analysis |
| `renderer.Render*` | `(*Report, io.Writer)` | Console/JSON/CSV output |

---

## 4. Streaming Pipeline

### Input discovery — `reader.ReadLines`

```go
func ReadLines(r io.Reader, fileThreshold int) chan Line
```

- Determines if source is file vs stdin via size hint / `os.Stdin.Stat()`
- For files: reads line-by-line via `bufio.Scanner`
- For stdin: reads all bytes then splits lines (stdin pipes don't report size)
- Returns lines on a channel; closing the channel signals EOF
- Binary detection: reads first 8 KB, checks for null byte (`\x00`) — if found, logs warning and skips processing but still reports line count

### Parser — `parser.ParseLine`

```go
func ParseLine(lineNum int, line, source string) *Event
```

- Returns `*Event` or `nil` if line cannot be parsed
- Finds first timestamp using ordered regex patterns, parses to UTC
- Finds level keyword using ordered match (FATAL → ERROR → WARN → INFO → DEBUG)
- Extracts message (everything after timestamp; or after level keyword; or entire line)

### Filter — `filter.ShouldKeep`

```go
func ShouldKeep(evt *Event, cfg *Config) bool
```

- Level: `evt.Level >= cfg.MinLevel` (lower number = more severe)
- Regex: `cfg.Regex.MatchString(evt.Raw)`
- Time: start/end time bounds (zero-value = no filter)
- All conditions must pass (AND logic)
- Events without timestamps pass time filters

### Analyzer — `analyzer.Analyze`

```go
func Analyze(events chan *Event, cfg *Config) *Report
```

Counts all events by level; for error-level events, normalizes message and tracks top-K groups via min-heap.

---

## 5. Normalization Engine

### Location: `internal/normalizer/normalizer.go`

**Purpose:** Convert error lines to a canonical form so near-identical errors collapse into one group.

### Replacement order and patterns

| Step | Pattern | Replacement | Example |
|---|---|---|---|
| 1 | UUID | `<uuid>` | `a1b2c3d4-...` → `<uuid>` |
| 2 | Request ID | `<req>` | `req_abc123`, `rid:xyz` |
| 3 | IPv6 | `<ip>` | `2001:db8::1` → `<ip>` |
| 4 | IPv4 | `<ip>` | `10.0.0.5` → `<ip>` |
| 5 | Hex | `<hex>` | `0xabcd1234` → `<hex>` |
| 6 | File path | `<path>` | `/var/log/app.log` → `<path>` |
| 7 | Hash | `<hash>` | `da39a3ee5e6b4b0d...` → `<hash>` |
| 8 | Number | `<n>` | `5432`, `342`, `10.5` → `<n>` |

### Design decisions

- **Order matters:** applying UUID before number prevents UUIDs from being partially matched as hex/numbers. Applying file paths before numbers prevents path components (like line numbers) from being consumed separately.
- **Only error-level:** normalization is gated on `LevelError` and above. Non-error events are never normalized — they pass through to the event channel untouched.
- **Boundary respect:** all patterns use `\b` word boundaries to avoid matching inside words (e.g., `v1` stays as-is, `log4j` stays as-is).

---

## 6. Analyzer Implementation

### Location: `internal/analyzer/analyzer.go`

### Event counting

```go
counts := make(map[string]int)
for evt := range events {
    counts[evt.Level.String()]++
}
```

### Top-K error grouping

Uses a **min-heap** (`container/heap`) bounded to `cfg.Limit`:

```go
h := &GroupHeap{}
for evt := range events {
    if evt.Level >= model.LevelError {  // only group errors
        sig := normalizer.Normalize(evt.Message)
        if existing := h.Find(sig); existing != nil {
            existing.Count++
            existing.LastSeen = evt.Timestamp
        } else {
            heap.Push(h, &model.Group{
                Signature: sig,
                Count:     1,
                FirstSeen: evt.Timestamp,
                LastSeen:  evt.Timestamp,
                Sample:    evt.Raw,
            })
        }
    }
}
// Trim to limit if over
if h.Len() > cfg.Limit {
    for h.Len() > cfg.Limit {
        heap.Pop(h)
    }
}
```

### Find operation

`GroupHeap.Find` is O(n) linear scan with `Limit` items — acceptable for `Limit` ≤ `1000`. Each error event triggers a normalized key + linear scan.

### Design decisions

- **Min-heap, not max-heap:** pops the smallest count when full; the surviving groups have the highest counts. Sorted by Count (ascending) in heap order.
- **No deduplication during counting:** level counts include `FATAL`, `ERROR`, `WARN`, `INFO`, `DEBUG` regardless of grouping.
- **Events go to two places:** events are always added to `Report.Events` (for errors/grep commands); only error-level events also update the heap.

---

## 7. AI Summarizer

### Location: `internal/summarizer/`

**Purpose:** Generate an AI-powered natural-language analysis of error patterns. Two implementations: noop (default, zero weight) and LLM (OpenAI-compatible HTTP).

### Interface

```go
type Summarizer interface {
    Summarize(ctx context.Context, req SummaryRequest) (*Summary, error)
    SummarizeStream(ctx context.Context, req SummaryRequest) (<-chan string, error)
}
```

### Request (`SummaryRequest`)

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
- Supports both sync and SSE streaming via `stream: true`
- API key sent as `Authorization: Bearer` header
- Defaults to noop when no endpoint is configured
- 60s timeout on sync, 120s on stream

### Integration

- **CLI:** `scan` and `top` commands call `Summarize` after analysis if `--ai-endpoint` is set, render via `PrintAISummary` (strips markdown)
- **Server:** `GET /api/insights/{id}` (sync) and `GET /api/insights/{id}/stream` (SSE) endpoints; results cached on session
- **Web UI:** AI Insights tab streams markdown content via `EventSource` and renders it client-side

### Security

- API key read from `LOGANALYZE_AI_KEY` env var only (not from flags)
- Shared `NewSummaryRequestFromReport` helper avoids duplication between CLI and server

---

## 8. Server API

### Location: `internal/server/handlers.go`, `internal/server/server.go`

### Session management — `internal/session/session.go`

- In-memory `sync.Map` keyed by UUID
- Configurable cleanup interval (default 1 minute)
- TTL per session: 1 hour since last access
- Cleanup runs in a background goroutine

### Endpoints

#### `POST /api/upload`

- Accepts multipart form with field `file`
- Max file size: 100 MB (configurable via `http.MaxBytesReader`)
- Copies file to `{dataDir}/{sessionID}.log`
- Returns JSON: `{"session_id": "uuid","filename": "original.log","size": 12345}`

#### `POST /api/analyze/{id}`

- Body: `{"command":"scan","level":"error","regex":"","limit":10,"since":"","until":""}`
- Validates session exists, checks for existing analysis
- Launches analysis in a background goroutine
- Returns 202 with `{"status":"started","session_id":"..."}`
- Stores results in session on completion
- `command` field determines analysis mode: `scan` (no events list), `errors` (error-level events), `top` (grouped errors only), `grep` (all events)

#### `GET /api/results/{id}`

- Returns full `Report` as JSON with all events included
- Includes `command` field in response
- Returns 404 if session not found or analysis not complete
- Returns 409 if session was deleted

#### `GET /api/results/{id}/events`

- Query params: `offset` (default 0), `limit` (default 100, max 1000)
- Returns paginated events with total count, offset, and limit
- Response shape: `{"events":[...],"total":342,"offset":0,"limit":100}`
- Events are sorted by line number
- Returns 404 if session/analysis not found, or 400 if the command type has no events (scan/top return zero events)

#### `GET /api/status/{id}`

- Server-Sent Events stream: `{"phase":"reading|parsing|analyzing|done","progress":50,"total":100}`
- Phase transitions: `reading` → `parsing` → `analyzing` → `done`
- Sent every 500ms or on phase change, whichever is sooner

#### `GET /api/sessions`

- Lists all active sessions with metadata (filename, size, command, status, created_at)
- Response: `{"sessions":[...]}`

#### `DELETE /api/sessions/{id}`

- Deletes session and uploaded file from disk
- Returns 204 No Content
- Idempotent (missing file is not an error)

#### `GET /api/uploaded/{id}`

- Returns the raw uploaded file as `application/octet-stream`
- Useful for viewing the original file content in the Web UI's Raw tab
- Returns 404 if session not found

#### `GET /health`

- Returns `{"status":"ok","timestamp":"..."}`

#### `GET /api/insights/{id}`

- Returns AI-generated summary as JSON: `{"summary":"...", "model":"...", "cached":true|false}`
- Requires `--ai-endpoint` to be configured at server start
- Returns 501 if summarizer not configured
- Returns 409 if analysis is not yet complete
- Results are cached on the session after first generation

#### `GET /api/insights/{id}/stream`

- Server-Sent Events stream of markdown content
- Same preconditions as sync endpoint
- Streams tokens as `data: {"type":"text","content":"..."}\n\n`
- Sends `event: complete` with `{"type":"done"}` when finished
- Sends `event: error` with `{"type":"error","content":"..."}` on failure
- Caches the complete response on session after streaming finishes

### Router

Uses `http.ServeMux` (Go 1.22+ method-based routing):

```go
mux := http.NewServeMux()
mux.HandleFunc("POST /api/upload", ...)
mux.HandleFunc("POST /api/analyze/{id}", ...)
mux.HandleFunc("GET /api/results/{id}", ...)
mux.HandleFunc("GET /api/results/{id}/events", ...)  // Go 1.22+ path prefix match
mux.HandleFunc("GET /api/status/{id}", ...)
mux.HandleFunc("GET /api/sessions", ...)
mux.HandleFunc("DELETE /api/sessions/{id}", ...)
mux.HandleFunc("GET /api/uploaded/{id}", ...)
mux.HandleFunc("GET /api/insights/{id}", ...)
mux.HandleFunc("GET /api/insights/{id}/stream", ...)
mux.HandleFunc("GET /health", ...)
```

Static files are served via `//go:embed` using:

```go
mux.Handle("/", http.FileServer(http.FS(webFS)))
```

Note: `http.StripPrefix` is NOT used — the embedded `static/` directory is structured so the file server serves `index.html` at root and all other assets at their relative paths.

### Middleware

- **CORS:** sets `Access-Control-Allow-Origin: *` and related headers on all API routes
- **Content-Type:** enforces JSON content type for `POST` requests to `/api/analyze`
- **Logging:** logs method, path, status, and duration to stderr
- **Recovery:** catches panics, returns 500, logs stack trace
- **Cleanup:** background goroutine removes expired sessions every 10 minutes

### Signal handling

```go
sig := make(chan os.Signal, 1)
signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
<-sig
```

Graceful shutdown: stop HTTP server, cancel contexts, wait for active analyses to finish (with 30s timeout).

---

## 9. Web UI Architecture

### Location: `internal/web/static/`

### Stack

- **HTML:** Single-page application shell with inline theme detection
- **CSS:** Design system with custom properties, no preprocessor (~450 lines)
- **JS:** Vanilla ES module with zero dependencies (~550 lines)

### CSS Design System

```css
:root {
  /* Light theme (default) */
  --bg-primary: #ffffff;
  --bg-secondary: #f8f9fa;
  --text-primary: #1a1a2e;
  --text-secondary: #6c757d;
  /* ... 40+ custom properties */
}
[data-theme="dark"] {
  --bg-primary: #1a1a2e;
  --bg-secondary: #16213e;
  --text-primary: #e4e4e7;
  --text-secondary: #a1a1aa;
  /* ... same properties, dark values */
}
```

- **Layout:** CSS Grid with fixed sidebar (260px) and scrollable main area
- **Components:** cards, badges (level colors), tables, forms, buttons, progress bars, accordions, toast notifications, tabs
- **Responsive:** collapses sidebar to top nav on screens < 768px
- **Animations:** logo spin on hover, card entrance fade-up, accordion slide, toast slide-in
- **Typography:** system font stack, monospace for logs

### JS Architecture

- **Module pattern:** IIFE returns `{init, state}` public API
- **Routing:** hash-based (`/upload`, `/session/abc123`, `/` for dashboard)
- **Event bus:** custom events on `document` for cross-component communication
- **State:** `window.__state` object with sessions, current analysis, theme

#### Pages

| Route | Handler | Description |
|---|---|---|
| `#/` | `renderDashboard()` | Stats cards, session table, quick actions |
| `#/upload` | `renderUpload()` | Drag-and-drop file area, command config, progress |
| `#/session/:id` | `renderSession(id)` | 4-tab detail view (Overview, Events, AI Insights, Raw) |

#### Dashboard (`renderDashboard`)

- Fetches `GET /api/sessions` on mount
- Stat cards (files analyzed today, total errors found, etc.)
- Session table: filename, command, size, status, created, actions (view, delete)
- Loading state with fallback text, error state with toast notification

#### Upload (`renderUpload`)

- Drag-and-drop file upload with visual feedback (highlight on dragover)
- Command selector (scan, errors, top, grep) with conditional regex input
- Flag toggles (JSON mode, color, level selector)
- Progress bar during analysis (polls `GET /api/status/{id}` SSE)
- On completion, redirects to session detail page

#### Session Detail (`renderSession`)

Three vertical tabs: Overview, Events, Raw

**Overview tab:**
- Stat cards: total lines, errors, warnings, info, time range
- SVG bar chart: level breakdown with percentage labels and color-coded bars
- Error groups: collapsible accordion, each showing count, time range, normalized signature, sample raw line

**Events tab:**
- Dropdown filter for event level
- Text search within displayed results
- Paginated table (100 events per page) with page number navigation
- Expandable rows showing the full raw log line
- Fetches from `GET /api/results/{id}/events?offset=N&limit=M`
- Shows empty state for scan/top commands (no events available)

**Raw tab:**
- Fetches from `GET /api/uploaded/{id}`
- Line-numbered display in monospace font
- Useful for seeing the original file context

### Theme

- Detected on first visit via `prefers-color-scheme`
- Toggled via button click (delegated event on `#theme-btn`)
- Persisted to `localStorage` key `theme`
- If no stored preference, follows system preference
- Updates all CSS custom properties via `[data-theme]` attribute on `<html>`

### Keyboard shortcuts

| Shortcut | Handler |
|---|---|
| `⌘1` / `Ctrl+1` | Navigate to dashboard (`#/`) |
| `⌘U` / `Ctrl+U` | Navigate to upload (`#/upload`) |

### Toast notification system

- Queue-based: `showToast(message, type)` creates a `<div class="toast">` with auto-dismiss
- Types: `info`, `error`, `success`
- Stacked: multiple toasts can be visible (each auto-dismisses after 4s)
- Top-right positioning with slide-in animation

---

## 10. Docker Deployment

### `Dockerfile`

```dockerfile
# Builder
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/loganalyze ./main.go

# Runtime
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
- Exposes port 8080 by default

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

- Named volume for uploaded file persistence
- UTC timezone for consistent timestamp handling
- Auto-restart unless explicitly stopped

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
| `server` | HTTP handler integration (if present) |
| `summarizer` | Prompt building, LLM sync/stream, HTTP error handling, auth, timeout, noop fallback |

### Test data

`testdata/samples/` contains three sample log files:

| File | Format | Lines | Description |
|---|---|---|---|
| `errors.log` | Mixed timestamps | ~50 | Hand-crafted error lines with UUIDs, IPs, paths |
| `syslog.log` | BSD syslog | ~30 | Kernel messages with various levels |
| `apache.log` | Apache CLF | ~20 | Web access logs with mixed status codes |

### Running tests

```bash
go test ./...
```

### Test conventions

- All test helpers use `t.Helper()`
- Table-driven tests where appropriate (`tests []struct{name, input, expected}`)
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
| **Report** | The complete analysis output (counts, groups, events, time range) |
| **Session** | Server-side state for an uploaded file + its analysis results |
| **Signature** | A normalized message string used for grouping identical errors |
| **SSE** | Server-Sent Events — unidirectional event stream for progress updates |
| **SPA** | Single-Page Application — client-side routing and rendering |

## Appendix B: Edge Cases

| Scenario | Behavior |
|---|---|
| Empty file | Returns report with zero counts, no errors |
| Binary file | Logs warning to stderr, skips content, reports total_lines as 0 |
| No timestamps found | All events get zero-value time.Time (0001-01-01) |
| No level keyword | Defaults to INFO |
| Duplicate uploads | Each upload creates a new session with a new UUID |
| Concurrent analysis | Each session runs analysis in its own goroutine |
| Session expiry | Cleanup runs every 10 minutes, removes sessions older than 1h |
| Server restart | All sessions are lost (in-memory store, no persistence) |
| Large files (>100MB) | Rejected at upload with 413 Payload Too Large |
| No events for command | scan/top return empty events array; /events endpoint returns 400 |
| Missing file in DELETE | No error — idempotent (file may already be cleaned up) |
| Partial analysis on shutdown | Results are discarded; client gets 404 on next fetch |

## Appendix C: HTTP Status Codes

| Code | Usage |
|---|---|
| 200 | Successful GET/POST response with body |
| 202 | Analysis started (async) |
| 204 | Successful DELETE (no body) |
| 400 | Bad request (invalid command, missing file, no events for command type) |
| 404 | Session not found, analysis not yet complete |
| 409 | Session was deleted |
| 413 | Upload too large (>100 MB) |
| 500 | Internal server error |
