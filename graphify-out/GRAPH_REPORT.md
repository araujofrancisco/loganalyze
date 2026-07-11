# Graph Report - loganalyzer  (2026-07-11)

## Corpus Check
- 53 files · ~26,792 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 557 nodes · 1005 edges · 40 communities (37 shown, 3 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 154 edges (avg confidence: 0.79)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `7997b603`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Event
- app.js
- Architecture
- Log Analyzer
- llm_test.go
- ResponseWriter
- TailFile
- chain
- ParseLine
- newRateLimiter
- session_test.go
- Endpoints
- Session
- Analyze
- middleware.go
- Matches
- Normalize
- Fold
- 10. Web UI Architecture
- Log Analyzer — Technical Specification
- 8. AI Summarizer
- 4. Streaming Pipeline
- 7. Analyzer Implementation
- root.go
- 12. Testing Strategy
- dependencies
- opencode.json
- 5. Stack Trace Folding
- 6. Normalization Engine
- graphify.js
- 11. Docker Deployment
- 2. Data Model
- github.com/araujofrancisco/loganalyze

## God Nodes (most connected - your core abstractions)
1. `Session` - 21 edges
2. `Event` - 20 edges
3. `ParseLine()` - 20 edges
4. `$()` - 19 edges
5. `Log Analyzer — Technical Specification` - 17 edges
6. `Log Analyzer` - 16 edges
7. `Analyze()` - 14 edges
8. `Report` - 14 edges
9. `Server` - 14 edges
10. `chain()` - 13 edges

## Surprising Connections (you probably didn't know these)
- `startPipeline()` --calls--> `Fold()`  [INFERRED]
  cmd/pipeline.go → internal/fold/fold.go
- `startPipeline()` --calls--> `ParseLine()`  [INFERRED]
  cmd/pipeline.go → internal/parser/parser.go
- `startPipeline()` --calls--> `ReadLines()`  [INFERRED]
  cmd/pipeline.go → internal/reader/reader.go
- `startTailPipeline()` --calls--> `Matches()`  [INFERRED]
  cmd/watch.go → internal/filter/filter.go
- `startTailPipeline()` --calls--> `Fold()`  [INFERRED]
  cmd/watch.go → internal/fold/fold.go

## Import Cycles
- None detected.

## Communities (40 total, 3 thin omitted)

### Community 0 - "Event"
Cohesion: 0.06
Nodes (42): buildFilterConfig(), Config, Config, Context, Duration, printPeriodicReport(), runLiveWatch(), runPeriodicWatch() (+34 more)

### Community 1 - "app.js"
Cohesion: 0.12
Nodes (45): buildErrorGroupsHTML(), buildEventRowHTML(), buildReportHTML(), bus, debounce(), ensureToastContainer(), escapeHtml(), _eventsState (+37 more)

### Community 2 - "Architecture"
Cohesion: 0.05
Nodes (39): AI summarizer (`internal/summarizer/`), Architecture, Build, Commands, Conventions, graphify, Key gotchas, Log Analyzer (loganalyze) (+31 more)

### Community 3 - "Log Analyzer"
Cohesion: 0.05
Nodes (38): Adding a new log format, Adding a new output format, AI Insights tab, Analyze request body, API, CLI mode, Commands, Data Pipeline (+30 more)

### Community 4 - "llm_test.go"
Cohesion: 0.10
Nodes (28): Client, buildPrompt(), Config, Context, NewLLM(), T, TestBuildPrompt(), TestLLMStream() (+20 more)

### Community 5 - "ResponseWriter"
Cohesion: 0.21
Nodes (12): buildFilterConfig(), Config, Request, ResponseWriter, Server, isGzipBytes(), readLastEvents(), sanitizeFilename() (+4 more)

### Community 6 - "TailFile"
Cohesion: 0.15
Nodes (22): File, isBinary(), isGzip(), readFile(), ReadLines(), readReader(), T, TestReadLinesGzip() (+14 more)

### Community 7 - "chain"
Cohesion: 0.21
Nodes (20): chain(), Server, New(), T, itoa(), TestAnalyzeLimitClamping(), TestJSONError(), TestJSONErrorWithSpecialChars() (+12 more)

### Community 8 - "ParseLine"
Cohesion: 0.19
Nodes (21): extractLevel(), extractTimestamp(), Time, isAlpha(), padDay(), padMonth(), ParseLine(), containsTimestamp() (+13 more)

### Community 9 - "newRateLimiter"
Cohesion: 0.19
Nodes (16): clientIP(), Duration, Request, Time, newRateLimiter(), rateLimitMiddleware(), T, TestRateLimiterAllow() (+8 more)

### Community 10 - "session_test.go"
Cohesion: 0.23
Nodes (19): NewStore(), T, itoa(), TestCleanup(), TestCleanupSkipsRecent(), TestConcurrentAccess(), TestCreateAndGet(), TestDelete() (+11 more)

### Community 11 - "Endpoints"
Cohesion: 0.10
Nodes (20): 9. Server API, Background analysis, `DELETE /api/sessions/{id}`, Endpoints, `GET /api/insights/{id}`, `GET /api/insights/{id}/stream`, `GET /api/results/{id}`, `GET /api/results/{id}/events` (+12 more)

### Community 12 - "Session"
Cohesion: 0.16
Nodes (7): generateID(), Duration, Time, RWMutex, AnalyzeConfig, Session, Store

### Community 13 - "Analyze"
Cohesion: 0.23
Nodes (9): groupHeap, Analyze(), T, TestAnalyzeCounts(), TestAnalyzeEmpty(), TestAnalyzeFirstLastTime(), TestAnalyzeGrouping(), TestAnalyzeIgnoresNonErrors() (+1 more)

### Community 14 - "middleware.go"
Cohesion: 0.22
Nodes (12): Handler, loggingMiddleware(), recoveryMiddleware(), requestIDMiddleware(), T, TestChain(), TestLoggingMiddleware(), TestRecoveryMiddleware() (+4 more)

### Community 15 - "Matches"
Cohesion: 0.22
Nodes (12): Config, startPipeline(), Config, Time, Matches(), T, TestFilterAllConditions(), TestFilterLevel() (+4 more)

### Community 16 - "Normalize"
Cohesion: 0.34
Nodes (12): Normalize(), T, TestNormalizeAlreadyNormalized(), TestNormalizeHash(), TestNormalizeHex(), TestNormalizeIPv4(), TestNormalizeMultiplePatterns(), TestNormalizeNoChange() (+4 more)

### Community 17 - "Fold"
Cohesion: 0.38
Nodes (9): Fold(), isContinuation(), T, TestFoldEmptyChannel(), TestFoldMaxLines(), TestFoldMergesContinuationLines(), TestFoldNotStart(), TestFoldPreservesLineNumSource() (+1 more)

### Community 18 - "10. Web UI Architecture"
Cohesion: 0.20
Nodes (10): 10. Web UI Architecture, CSS Design System, JS Architecture, Keyboard shortcuts, Location: `internal/web/static/`, Pages, Session detail tabs, Stack (+2 more)

### Community 19 - "Log Analyzer — Technical Specification"
Cohesion: 0.25
Nodes (7): 1. Architecture Overview, 3. Core Functions, Appendix A: Glossary, Appendix B: Edge Cases, Appendix C: HTTP Status Codes, Log Analyzer — Technical Specification, Table of Contents

### Community 20 - "8. AI Summarizer"
Cohesion: 0.29
Nodes (7): 8. AI Summarizer, Integration, Interface, LLM implementation (`llm.go`), Location: `internal/summarizer/`, Security, `SummaryRequest`

### Community 21 - "4. Streaming Pipeline"
Cohesion: 0.33
Nodes (6): 4. Streaming Pipeline, Analyzer — `analyzer.Analyze`, File tailing — `reader.TailFile`, Filter — `filter.Matches`, Input discovery — `reader.ReadLines`, Parser — `parser.ParseLine`

### Community 22 - "7. Analyzer Implementation"
Cohesion: 0.33
Nodes (6): 7. Analyzer Implementation, Design decisions, Event counting, Group heap, Location: `internal/analyzer/analyzer.go`, Top-K error grouping

### Community 24 - "12. Testing Strategy"
Cohesion: 0.40
Nodes (5): 12. Testing Strategy, Running tests, Test conventions, Test data, Unit tests

### Community 25 - "dependencies"
Cohesion: 0.50
Nodes (3): @opencode-ai/plugin, dependencies, @opencode-ai/plugin

### Community 26 - "opencode.json"
Cohesion: 0.50
Nodes (3): plugin, $schema, .opencode/plugins/graphify.js

### Community 27 - "5. Stack Trace Folding"
Cohesion: 0.50
Nodes (4): 5. Stack Trace Folding, Design, Integration, Location: `internal/fold/fold.go`

### Community 28 - "6. Normalization Engine"
Cohesion: 0.50
Nodes (4): 6. Normalization Engine, Design decisions, Location: `internal/normalizer/normalizer.go`, Replacement order and patterns

### Community 30 - "11. Docker Deployment"
Cohesion: 0.67
Nodes (3): 11. Docker Deployment, `docker-compose.yml`, `Dockerfile`

### Community 31 - "2. Data Model"
Cohesion: 0.67
Nodes (3): 2. Data Model, Key structs, Location: `internal/model/event.go`

## Knowledge Gaps
- **136 isolated node(s):** `$schema`, `.opencode/plugins/graphify.js`, `@opencode-ai/plugin`, `github.com/araujofrancisco/loganalyze`, `jsonEvent` (+131 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **3 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Session` connect `Session` to `Event`, `llm_test.go`, `ResponseWriter`, `chain`?**
  _High betweenness centrality (0.126) - this node is a cross-community bridge._
- **Why does `Event` connect `Event` to `ResponseWriter`, `ParseLine`, `Session`, `Analyze`, `Matches`?**
  _High betweenness centrality (0.079) - this node is a cross-community bridge._
- **Why does `Analyze()` connect `Analyze` to `Event`, `Normalize`, `ResponseWriter`?**
  _High betweenness centrality (0.059) - this node is a cross-community bridge._
- **Are the 16 inferred relationships involving `ParseLine()` (e.g. with `startPipeline()` and `startTailPipeline()`) actually correct?**
  _`ParseLine()` has 16 INFERRED edges - model-reasoned connections that need verification._
- **What connects `$schema`, `.opencode/plugins/graphify.js`, `@opencode-ai/plugin` to the rest of the system?**
  _137 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Event` be split into smaller, more focused modules?**
  _Cohesion score 0.06390977443609022 - nodes in this community are weakly interconnected._
- **Should `app.js` be split into smaller, more focused modules?**
  _Cohesion score 0.12488436632747456 - nodes in this community are weakly interconnected._