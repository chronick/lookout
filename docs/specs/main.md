# lookout-go — Spec

OTEL trace collector for AI workflows. Go implementation.

## Overview

lookout-go accepts OpenTelemetry traces via standard OTLP (gRPC + HTTP), stores
them in SQLite, and serves an analytics API. It understands AI workflow semantics
— chat completions, tool calls, agent sessions — and auto-enriches spans with
cost, throughput, and anomaly data.

Designed for the berth agentic coding stack. Works with OpenClaw, Claude Code,
bosun agent containers, and any OTEL-instrumented service.

## Goals

- **Standard ingest**: OTLP gRPC (:4317) and HTTP/protobuf (:4318), no custom SDKs
- **AI-native**: first-class support for GenAI semantic conventions + agent workflows
- **Simple ops**: single binary, SQLite storage, no external dependencies at runtime
- **Fast reads**: in-memory ring buffer for live dashboard, SQLite for historical queries
- **Actionable**: cost tracking, throughput metrics, anomaly detection out of the box

## Non-Goals

- Full APM/distributed tracing platform (use Jaeger/Tempo for that)
- Metrics or logs collection (traces only)
- Multi-tenant / multi-user access control
- Horizontal scaling (single-node by design)

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                      lookout-go                          │
│                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │ OTLP gRPC   │  │ OTLP HTTP   │  │  Analytics API  │  │
│  │ :4317       │  │ :4318       │  │  :4320          │  │
│  └──────┬──────┘  └──────┬──────┘  └────────┬────────┘  │
│         │                │                   │           │
│         └────────┬───────┘                   │           │
│                  ▼                           │           │
│         ┌────────────────┐                   │           │
│         │   Enrichment   │                   │           │
│         │ cost, tok/s,   │                   │           │
│         │ anomaly flags  │                   │           │
│         └───────┬────────┘                   │           │
│                 ▼                            │           │
│  ┌──────────────────────────────┐            │           │
│  │           Store              │◄───────────┘           │
│  │  ┌──────────┐ ┌───────────┐ │                         │
│  │  │   Ring   │ │  SQLite   │ │──► Broadcast (WS)       │
│  │  │  Buffer  │ │   WAL     │ │                         │
│  │  └──────────┘ └───────────┘ │                         │
│  └──────────────────────────────┘                        │
│                                                          │
│  ┌──────────────┐  ┌──────────────┐                      │
│  │  TUI (dash)  │  │ Push Metrics │                      │
│  │  bubbletea   │  │  HTTP POST   │                      │
│  └──────────────┘  └──────────────┘                      │
└──────────────────────────────────────────────────────────┘
```

## Data Model

### SpanRecord (flattened for storage)

| Field | Type | Source |
|-------|------|--------|
| trace_id | string | OTLP |
| span_id | string | OTLP |
| parent_span_id | string | OTLP |
| name | string | OTLP |
| kind | int | OTLP (0=unspecified, 1=internal, 2=server, 3=client, 4=producer, 5=consumer) |
| start_time_unix_nano | uint64 | OTLP |
| end_time_unix_nano | uint64 | OTLP |
| attributes_json | string | OTLP attributes serialized |
| resource_json | string | OTLP resource attributes serialized |
| status_code | int | OTLP (0=unset, 1=ok, 2=error) |
| status_message | string | OTLP |
| ai_system | string? | Extracted from `gen_ai.system` |
| ai_model | string? | Extracted from `gen_ai.request.model` or `gen_ai.response.model` |
| ai_input_tokens | int64? | Extracted from `gen_ai.usage.input_tokens` |
| ai_output_tokens | int64? | Extracted from `gen_ai.usage.output_tokens` |
| agent_name | string? | Extracted from `agent.name` |
| agent_task_id | string? | Extracted from `agent.task_id` |
| agent_session_id | string? | Extracted from `agent.session_id` |
| inserted_at | datetime | Server-side timestamp |

### AI Semantic Conventions

Follows [OpenTelemetry GenAI semconv](https://opentelemetry.io/docs/specs/semconv/gen-ai/) plus agent extensions:

| Span Name | Attributes | Example |
|-----------|-----------|---------|
| `gen_ai.chat_completion` | `gen_ai.system`, `gen_ai.request.model`, `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens` | Claude API call |
| `gen_ai.tool_call` | `gen_ai.tool.name`, `gen_ai.tool.call_id` | Bash, Read, Edit |
| `agent.session` | `agent.name`, `agent.session_id`, `agent.task_id`, `agent.repo` | bosun lifecycle |
| `agent.step` | `agent.step.type` (claim, work, pr, reset) | Single work unit |
| `agent.sandbox` | `container.id`, `container.image` | Spawned via skiff |

## Enrichment

Applied automatically on ingest:

### Cost Calculation

Lookup model in pricing table, compute: `(input_tokens * input_price + output_tokens * output_price) / 1M`

| Model | Input $/1M | Output $/1M |
|-------|-----------|------------|
| claude-opus-4 | 15.00 | 75.00 |
| claude-sonnet-4 | 3.00 | 15.00 |
| claude-haiku-4-5 | 0.80 | 4.00 |
| gpt-4o | 2.50 | 10.00 |
| gpt-4.1 | 2.00 | 8.00 |
| o3 | 10.00 | 40.00 |

### Token Throughput

`output_tokens / duration_seconds` — computed per span.

### Anomaly Detection

| Condition | Flag |
|-----------|------|
| `status_code == 2` | `error: {message}` |
| duration > 10 min | `long_duration: {N}s` |
| output_tokens > 100k | `high_output_tokens: {N}` |
| AI span with 0 output tokens (>1s) | `zero_output_tokens` |
| AI span with <5 tok/s (>5s) | `low_throughput: {N} tok/s` |

## Storage

### Ring Buffer

- Fixed capacity (default 10,000 spans)
- `sync.RWMutex` protected
- Write overwrites oldest on full
- Read returns most recent N in reverse chronological order
- Used by TUI dashboard and `/v1/recent` for zero-query-latency reads

### SQLite

- WAL journal mode, busy timeout 5s
- Single `spans` table with indexes on: trace_id, start_time, ai_model, agent_name, agent_session_id, status_code
- Pure Go via `modernc.org/sqlite` (no CGO required)
- Configurable retention (default 7 days, hourly cleanup)

## Analytics API

All endpoints on `:4320`.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/traces` | List spans (filter: `?agent=`, `?model=`, `?limit=`) |
| GET | `/v1/traces/{trace_id}` | All spans for a trace |
| GET | `/v1/recent` | Recent spans from ring buffer (`?limit=`) |
| GET | `/v1/stats` | Aggregate stats (total spans, traces, tokens, AI spans, sessions) |
| GET | `/v1/stats/by-model` | Stats grouped by model |
| GET | `/v1/stats/by-agent` | Stats grouped by agent name |
| GET | `/v1/stats/cost` | Cost report (`?bucket=hour|day`, `?since=24h`, `?group_by=model|agent`) |
| GET | `/v1/sessions` | Agent sessions with aggregated stats (`?limit=`) |
| GET | `/v1/anomalies` | Anomalous spans (`?limit=`) |
| WS | `/v1/live` | WebSocket stream of incoming spans (enriched) |
| GET | `/health` | Health check → `ok` |

All responses are JSON. Span responses include an `enrichment` object with `cost_usd`, `tokens_per_sec`, and `anomaly`.

## CLI

```
lookout-go [command] [flags]

Commands:
  serve     Start the collector daemon (default)
  dash      Launch the TUI dashboard
  query     Query spans from the CLI

Global flags:
  --grpc-addr      OTLP gRPC address      (env: LOOKOUT_GRPC_ADDR,   default: 0.0.0.0:4317)
  --http-addr      OTLP HTTP address      (env: LOOKOUT_HTTP_ADDR,   default: 0.0.0.0:4318)
  --api-addr       Analytics API address   (env: LOOKOUT_API_ADDR,    default: 0.0.0.0:4320)
  --db-path        SQLite database path    (env: LOOKOUT_DB_PATH,     default: ~/.lookout/traces.db)
  --ring-size      Ring buffer capacity    (env: LOOKOUT_RING_SIZE,   default: 10000)
  --retention-days Retention period        (env: LOOKOUT_RETENTION_DAYS, default: 7)
  --push-url       Push metrics URL        (env: LOOKOUT_PUSH_URL,    default: "")
  --push-interval  Push interval seconds   (env: LOOKOUT_PUSH_INTERVAL, default: 60)

Query flags:
  --trace-id       Filter by trace ID
  --agent          Filter by agent name
  --model          Filter by model
  --limit          Max results (default: 20)
```

## TUI Dashboard

Three tabs, navigated with Tab/arrow keys:

### Overview
- Left panel: summary stats (spans, traces, AI spans, sessions, tokens)
- Left panel: by-model breakdown table
- Left panel: by-agent breakdown table
- Right panel: recent spans with name, model, duration, cost, anomaly flag

### Spans
- Full span table: span_id, trace_id, name, model, agent, duration, cost, tok/s
- Scrollable with j/k or arrow keys

### Anomalies
- Filtered to anomalous spans only
- Shows span_id, name, model, agent, anomaly reason
- Red-highlighted rows

Refresh: auto-refresh from store every 2 seconds. Quit: `q` or `Esc`.

## Configuration

### Environment / Flags

All CLI flags have corresponding environment variables (see CLI section).

### skiff.yml entry

```yaml
obs.lookout:
  image: ghcr.io/chronick/lookout-go
  ports:
    - "4317:4317"
    - "4318:4318"
    - "4320:4320"
  volumes:
    - "~/.lookout:/data"
  env:
    LOOKOUT_DB_PATH: /data/traces.db
    LOOKOUT_PUSH_URL: http://core.skiff.local:8080/v1/metrics
```

### Client configuration

Any OTEL SDK exporter pointed at lookout works:

```bash
# Environment variables for OTEL SDK
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf

# Or gRPC
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
```

## Dependencies

| Package | Purpose |
|---------|---------|
| google.golang.org/grpc | OTLP gRPC server |
| google.golang.org/protobuf | Protobuf runtime |
| go.opentelemetry.io/proto/otlp | OTLP proto definitions |
| modernc.org/sqlite | Pure-Go SQLite driver |
| nhooyr.io/websocket | WebSocket for /v1/live |
| github.com/charmbracelet/bubbletea | TUI framework |
| github.com/charmbracelet/lipgloss | TUI styling |

## Phases

| Phase | Tasks | Outcome |
|-------|-------|---------|
| **1 — Core** | CLI scaffold, proto codegen, gRPC receiver, HTTP receiver, SQLite store | Accepts and stores OTLP traces |
| **2 — Intelligence** | Ring buffer, AI enrichment, analytics API, WebSocket, sessions, Dockerfile + CI | Full analytics + live streaming |
| **3 — Visibility** | TUI dashboard, push metrics | Operational visibility |

## Comparison: lookout (Rust) vs lookout-go

| Aspect | lookout (Rust) | lookout-go |
|--------|---------------|------------|
| SQLite | rusqlite (C binding) | modernc.org/sqlite (pure Go) |
| gRPC | tonic | google.golang.org/grpc |
| HTTP API | axum | net/http |
| TUI | ratatui + crossterm | bubbletea + lipgloss |
| WebSocket | axum ws | nhooyr.io/websocket |
| Binary size | Smaller | Larger (Go runtime) |
| Build | cargo build | go build (faster, no C compiler) |
| CGO | Required (rusqlite) | Not required |
| Cross-compile | Harder | Trivial (GOOS/GOARCH) |

Both implementations share the same: ports, API endpoints, data model, enrichment logic, semantic conventions, and TUI layout.
