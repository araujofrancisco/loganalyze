# Log Analyzer — Specification

## Overview

A CLI tool that reads one or more log files and answers: *what's failing, when, how often, and where?*

Binary name: `loganalyze`

Normalizer is **not** called per-line in the pipeline. It is invoked only inside the Analyzer during the grouping step, operating on the already-parsed `Message` field.

---

## CLI Interface

### Commands

#### `loganalyze scan [files...]`

Full analysis report of all log files.

```
loganalyze scan app.log
loganalyze scan /var/log/*.log --level error --json
```

Output: summary table with total lines, level breakdown, top recurring errors, time range, and first/last occurrence.

#### `loganalyze errors [files...]`

Filter and display only error-level lines with context.

```
loganalyze errors app.log --since 1h
loganalyze errors syslog --level fatal --no-color
```

Output: one error line per row with level badge, timestamp, source, and message.

#### `loganalyze top [files...]`

Show top N most frequent error patterns (normalized and grouped).

```
loganalyze top app.log --limit 20
loganalyze top *.log --json
```

Output: ranked list of signatures with count and time range.

#### `loganalyze grep [files...] <pattern>`

Regex search with match highlighting.

```
loganalyze grep app.log "timeout|refused|panic"
loganalyze grep /var/log/*.log "5[0-9][0-9]" --no-color
```

Output: matching lines with level badge and matched term highlighted.

---

### Global Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--since` | duration | - | Relative time filter (e.g. `1h`, `30m`, `24h`) |
| `--until` | timestamp | - | Absolute end time (RFC 3339) |
| `--level` | string | `""` | Minimum level: debug, info, warn, error, fatal |
| `--json` | bool | false | Output as JSON |
| `--csv` | bool | false | Output as CSV |
| `--no-color` | bool | false | Disable ANSI color output |
| `--limit` | int | 10 | Max results (top errors, grep matches) |

`--since` and `--until` are relative to the current time when `--since` is a duration, or absolute when a timestamp.

---

### Exit Codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | General error (bad flags, read failure) |
| 2 | Partial failure (≥1 file succeeded, ≥1 failed) |
| 130 | Interrupted by SIGINT/SIGTERM |

---

### Input

#### Sources (in priority order)

1. **File paths** — one or more paths, expanded via shell glob
2. **Standard input** — when no files given or `-` is passed
3. **Glob patterns** — paths containing `*`, `?`, `[...]` are expanded

#### Binary detection

Files with a null byte in the first 8192 bytes are skipped with a warning.

#### Encoding

Assumes UTF-8. Non-UTF-8 bytes are preserved in raw output but may be replaced with `�` in structured fields.

#### Future phase: Gzip support

Files ending in `.gz` will be transparently decompressed using `compress/gzip`. Not in MVP.

---

## Data Model

### `Event`

```go
type Event struct {
    Timestamp time.Time   // extracted or zero-value
    Level     Level
    Source    string      // filename or "stdin"
    Message   string      // extracted message
    Raw       string      // original line
    LineNum   int         // 1-based
}
```

### `Level`

```go
type Level int8

const (
    LevelDebug Level = -1
    LevelInfo  Level = 0
    LevelWarn  Level = 1
    LevelError Level = 2
    LevelFatal Level = 3
)
```

Level detection: case-insensitive word-boundary match. First recognized keyword on the line wins, searched in order: Fatal > Error > Warn > Info > Debug.

### Level aliases

| Canonical | Aliases |
|---|---|
| LevelFatal | FATAL, CRITICAL, PANIC |
| LevelError | ERROR |
| LevelWarn | WARN, WARNING |
| LevelInfo | INFO |
| LevelDebug | DEBUG, TRACE |

### `Group`

```go
type Group struct {
    Signature     string    // normalized message
    SampleMessage string    // first raw message seen
    Count         int
    FirstSeen     time.Time
    LastSeen      time.Time
}
```

---

## Parsing

### Timestamp detection (in priority order)

| Pattern | Example | Regex |
|---|---|---|
| ISO 8601 / RFC 3339 | `2026-07-08T10:12:33Z` | `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z\|[+-]\d{2}:?\d{2})?` |
| ISO 8601 date + time | `2026-07-08 10:12:33` | `\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}` |
| Syslog (BSD) | `Jul  8 10:12:33` | `(Jan\|Feb\|...)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}` |
| Apache CLF | `08/Jul/2026:10:12:33 +0000` | `\d{2}/(Jan\|Feb\|...)/\d{4}:\d{2}:\d{2}:\d{2} \+\d{4}` |
| Unix epoch seconds | `1720421553` | `^\d{10}` |

Each pattern is tried in order. First match wins. If no pattern matches, `Timestamp` is zero-value.

#### Syslog year resolution

Syslog lines (`Jul  8 10:12:33`) lack a year. The parser:
1. Uses the current year at runtime
2. If the resulting timestamp is in the future, subtracts 1 year
3. If file modification time is available (not stdin), prefers its year

### Message extraction

1. Strip the detected timestamp prefix (if any) from the line
2. Strip the detected level token from what remains
3. Strip leading/trailing whitespace from what remains
4. If nothing remains, use the whole raw line

**Guarantee:** `Message` NEVER contains the detected timestamp or level token. This is critical for normalizer correctness — without it, timestamp numbers pollute the grouping signature.

---

## Filtering

All active filters AND together:

- **Level filter:** `evt.Level >= cfg.MinLevel`
- **Regex filter:** `cfg.Regex.MatchString(evt.Raw)` (notably on raw line, not parsed message)
- **Time range:** if `evt.Timestamp` is non-zero and since/until are set, check `cfg.Since <= evt.Timestamp <= cfg.Until`

---

## Normalization

Used to group near-identical error lines into signatures.

### Replacement rules (applied in order)

| Pattern | Replace With | Example |
|---|---|---|
| UUID | `<uuid>` | `550e8400-e29b-41d4-a716-446655440000` |
| IPv4 | `<ip>` | `192.168.1.1` |
| IPv6 | `<ip>` | `::1` |
| Numbers (standalone) | `<n>` | `timeout after 30s` → `timeout after <n>s` |
| Hex numbers | `<hex>` | `error 0xdeadbeef` |
| File paths | `<path>` | `/var/log/app.log:123` |
| Hashes (40+ hex chars) | `<hash>` | `commit abc123def456...` |
| Unix timestamps (10-digit) | `<n>` | `logged in at 1720421553` |

Normalization is applied to the **Message** field, not Raw. The Message field MUST have timestamp and level tokens already stripped (see Message extraction guarantee).

### Precondition

Normalization is called ONLY during the Analyzer grouping step, on events at LevelError and above. It is never called per-line in the streaming pipeline.

### Replacement order (specific → general)

1. UUIDs → `<uuid>`
2. Request IDs → `<req>` (patterns: `req-[a-zA-Z0-9]+`, `trace=[a-z0-9]+`)
3. IPv6 → `<ip>`
4. IPv4 → `<ip>`
5. Hex numbers → `<hex>` (0x prefix)
6. File paths → `<path>`
7. Hashes (≥40 hex chars) → `<hash>`
8. Numbers → `<n>` (regex `\b\d+\b` — word-boundary match)

Order is critical: UUIDs are matched before hex/hashes, and IPs before general numbers.

---

## Analysis

### `Report` structure

```go
type Report struct {
    Source     string            // filename or "stdin"
    TotalLines int
    Levels     map[Level]int     // count per level
    TopErrors  []Group           // sorted by count desc, limited
    FirstLine  time.Time
    LastLine   time.Time
    Duration   time.Duration
}
```

### Grouping behavior

- Only events at `LevelError` and above are grouped
- Normalize the Message field to produce a signature
- Events with matching signatures are merged: count++, update LastSeen
- FirstSeen is set on first encounter and never overwritten
- After all events are processed, groups are sorted by Count descending

### Top-K algorithm

Groups are stored in a fixed-capacity min-heap of size `--limit`:
- Seen-before signature → increment count, fix heap position
- New signature, heap not full → push with count=1
- New signature, heap full → if count(1) > min(heap), pop-min and push-new

This is a standard streaming top-K approximation. Practically correct for all realistic log distributions.

### Memory guarantees

- Level counters: fixed small map
- Error groups: bounded by `--limit` (min-heap, see Top-K algorithm above)
- No event list stored — fully streaming
- Exception: `errors`/`grep` output is streaming (NDJSON), never buffered

---

## Output

### Console (default)

Uses `text/tabwriter` for aligned columns, `fatih/color` for ANSI coloring.

**`scan` output:**
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
 3. memory usage at <n>%, gc triggered         12x  [09:30 - 15:01]
```

**Color rules:**

| Level | Color |
|---|---|
| FATAL | White on red background |
| ERROR | Red bold |
| WARN | Yellow bold |
| INFO | Cyan |
| DEBUG | Dim white |

Grep matches: yellow background underline on matched portion.

### JSON output (`--json`)

```json
{
  "source": "app.log",
  "total_lines": 15342,
  "time_range": {
    "first": "2026-07-08T08:01:12Z",
    "last": "2026-07-08T17:45:33Z",
    "duration_sec": 35061
  },
  "levels": {
    "debug": 0,
    "info": 14109,
    "warn": 891,
    "error": 342,
    "fatal": 0
  },
  "top_errors": [
    {
      "signature": "database connection timeout after <n>s",
      "sample": "database connection timeout after 30s",
      "count": 47,
      "first_seen": "2026-07-08T10:12:33Z",
      "last_seen": "2026-07-08T14:33:12Z"
    }
  ]
}
```

For `errors` and `grep` commands with `--json`: output is **JSON Lines (NDJSON)** — one JSON object per line, LF-terminated. This preserves the streaming guarantee.

```
{"timestamp":"2026-07-08T10:12:33Z","level":"error","source":"app.log","message":"timeout","line":42}
{"timestamp":"2026-07-08T10:12:34Z","level":"error","source":"app.log","message":"refused","line":43}
```

For `scan` with `--json`: output is a single pretty-printed JSON object (the Report). This is safe because the Report is small by construction.

### CSV output (`--csv`)

For `scan`:
```
level,count,pct
ERROR,342,2.23
WARN,891,5.81
INFO,14109,91.96
```

For `errors`/`grep`:
```
timestamp,level,source,message
2026-07-08T10:12:33Z,error,app.log,database connection timeout after 30s
```

---

## Error Handling

- **Bad flags:** print usage to stderr, exit 1
- **File not found:** print warning to stderr, continue with remaining files
- **Permission denied:** print warning to stderr, continue
- **Binary file:** skip with warning, continue
- **Empty file:** treat as valid input with 0 lines
- **No input files and no stdin pipe:** print usage to stderr, exit 1

### Output discipline

- All diagnostic/warning messages → stderr (never stdout)
- Only the requested output format (console/JSON/CSV) → stdout
- This invariant is maintained regardless of partial failures

### Signal handling

On SIGINT/SIGTERM:
- `scan` mode: print partial report collected so far, then exit 130
- `errors`/`grep` mode: flush buffered output, then exit 130
- `top` mode: print partial top-N list, then exit 130

---

## Performance Targets

- **1 GB log file:** process in under 30 seconds on modern hardware
- **Memory:** under 100 MB for any file size (streaming; no `[]Event` accumulation)
- **Startup:** under 100ms to first output line
- **Throughput:** 50,000+ lines/second per core

---

## Project Structure

```
loganalyzer/
├── main.go                 # CLI entry, cobra setup
├── go.mod
├── cmd/
│   ├── root.go             # Root command + persistent flags
│   ├── scan.go             # scan subcommand
│   ├── errors.go           # errors subcommand
│   ├── top.go              # top subcommand
│   └── grep.go             # grep subcommand
├── internal/
│   ├── model/
│   │   └── event.go        # Event struct, Level type
│   ├── reader/
│   │   └── reader.go       # File / stdin / glob iteration
│   ├── parser/
│   │   ├── parser.go       # Level & timestamp detection
│   │   └── patterns.go     # Compiled regexes
│   ├── normalizer/
│   │   └── normalizer.go   # Signature normalization
│   ├── analyzer/
│   │   └── analyzer.go     # Counts, grouping, summary
│   ├── filter/
│   │   └── filter.go       # Regex / level / time filtering
│   └── renderer/
│       ├── console.go      # Colorized terminal output
│       ├── json.go         # JSON export
│       └── csv.go          # CSV export
└── testdata/
    └── samples/            # Sample log files for testing
```
