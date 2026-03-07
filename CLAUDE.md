# CLAUDE.md

## Project

**lookout-go** -- OTEL trace collector for AI workflows (Go implementation).

Go 1.25+, stdlib + minimal deps. Accepts OpenTelemetry traces via gRPC and HTTP,
stores them in SQLite, serves an analytics API. AI-aware: enriches spans with
cost calculation, token throughput, and anomaly detection.

Alternative to the Rust `lookout` implementation — same functionality, Go ecosystem.

## Commands

```bash
go build -o lookout-go ./cmd/lookout     # build
go test ./...                             # test all
./lookout-go serve                        # start collector
./lookout-go dash                         # TUI dashboard
./lookout-go query --limit 5              # query recent spans
```

## Layout

```
cmd/lookout/main.go              # CLI entry point
internal/
  otlp/
    grpc.go                      # OTLP gRPC receiver (google.golang.org/grpc)
    http.go                      # OTLP HTTP receiver (net/http)
    types.go                     # OTLP trace types (protobuf generated)
  ai/
    semantic.go                  # GenAI semantic convention constants
    enrichment.go                # Cost calc, token throughput, anomaly detection
  store/
    store.go                     # Dual-layer store (ring + SQLite)
    ring.go                      # In-memory ring buffer
    sqlite.go                    # SQLite persistence + queries
  api/
    api.go                       # Analytics HTTP API (net/http)
    routes.go                    # Route handlers
    ws.go                        # WebSocket live stream
  tui/
    tui.go                       # TUI dashboard (bubbletea/lipgloss)
proto/                           # OpenTelemetry proto files
```

## Key Design

- Dual ingest: OTLP gRPC (:4317) + HTTP (:4318), analytics API on :4320
- Dual storage: in-memory ring buffer (hot) + SQLite WAL (persistent)
- AI enrichment: auto-extracts gen_ai.* and agent.* semantic attributes
- Cost calculation: model + tokens -> USD using built-in pricing table
- Zero-config: sensible defaults, all options via CLI flags or env vars
- Retention: automatic cleanup of spans older than N days
- Proto: uses official opentelemetry-proto with protoc-gen-go

## Dependencies

- google.golang.org/grpc — gRPC server
- google.golang.org/protobuf — protobuf runtime
- modernc.org/sqlite — pure-Go SQLite (no CGO)
- nhooyr.io/websocket — WebSocket support
- github.com/charmbracelet/bubbletea — TUI framework
- github.com/charmbracelet/lipgloss — TUI styling

<!-- br-agent-instructions-v1 -->

---

## Beads Workflow Integration

This project uses [beads_rust](https://github.com/Dicklesworthstone/beads_rust) (`br`/`bd`) for issue tracking. Issues are stored in `.beads/` and tracked in git.

### Essential Commands

```bash
# View ready issues (unblocked, not deferred)
br ready              # or: bd ready

# List and search
br list --status=open # All open issues
br show <id>          # Full issue details with dependencies
br search "keyword"   # Full-text search

# Create and update
br create --title="..." --description="..." --type=task --priority=2
br update <id> --status=in_progress
br close <id> --reason="Completed"
br close <id1> <id2>  # Close multiple issues at once

# Sync with git
br sync --flush-only  # Export DB to JSONL
br sync --status      # Check sync status
```

### Workflow Pattern

1. **Start**: Run `br ready` to find actionable work
2. **Claim**: Use `br update <id> --status=in_progress`
3. **Work**: Implement the task
4. **Complete**: Use `br close <id>`
5. **Sync**: Always run `br sync --flush-only` at session end

### Key Concepts

- **Dependencies**: Issues can block other issues. `br ready` shows only unblocked work.
- **Priority**: P0=critical, P1=high, P2=medium, P3=low, P4=backlog (use numbers 0-4, not words)
- **Types**: task, bug, feature, epic, chore, docs, question
- **Blocking**: `br dep add <issue> <depends-on>` to add dependencies

### Session Protocol

**Before ending any session, run this checklist:**

```bash
git status              # Check what changed
git add <files>         # Stage code changes
br sync --flush-only    # Export beads changes to JSONL
git commit -m "..."     # Commit everything
git push                # Push to remote
```

### Best Practices

- Check `br ready` at session start to find available work
- Update status as you work (in_progress → closed)
- Create new issues with `br create` when you discover tasks
- Use descriptive titles and set appropriate priority/type
- Always sync before ending session

<!-- end-br-agent-instructions -->
