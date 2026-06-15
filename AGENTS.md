# AGENTS.md — mysqlmcp

AI coding assistant guide for the mysqlmcp project.

> **Strict**: This file overrides defaults when generating code for this repo.

## Project Identity

- **Module**: `github.com/phpgao/mysqlmcp`
- **Language**: Go 1.25+
- **Type**: MCP Server (Stdio + HTTP)
- **Package structure**: Single `package main` — flat files, no sub-packages

## Architecture

```
main.go          Entry: flag parsing → config load → connect init → server dispatch
config.go        YAML config parsing + validation (gopkg.in/yaml.v3)
connect.go       Multi-instance sql.DB pool (map[instanceID]*sql.DB + sync.Once)
parser.go        4-layer SQL static analysis (github.com/xwb1989/sqlparser)
auth.go          Bearer Token resolver + HTTP middleware
tools.go         MCP server creation + tool registration
tool_query.go    query tool: 4-layer check → timeout context → db.QueryContext/db.ExecContext
tool_meta.go     list_instances, describe_table, explain_query tools
go.mod / go.sum  MCP SDK v1.6.1 + mysql driver + sqlparser + yaml.v3
docs/            GitHub Pages site (index.html)
```

### Startup Flow

```
main()
  1. Parse flags: -config, -transport, -port, -token
  2. LoadConfig(path) → validate (instance IDs unique, timeout/max_rows ≤ defaults)
  3. InitConnections(cfg) → sql.Open + Ping for every instance → fail-fast if unreachable
  4. NewServer() → register 4 MCP tools
  5. Dispatch: stdio → server.Run(ctx, &StdioTransport{})
              http  → NewStreamableHTTPHandler + BearerToken middleware + ListenAndServe
```

### Tool Registration Pattern

All tools registered in `tools.go:NewServer()`:

```go
mcp.AddTool(server, toolXxx(), handleXxx)
```

Each tool has 3 parts:
1. `Input`/`Output` structs with `json:` and `jsonschema:` tags
2. `toolXxx()` returning `*mcp.Tool{Name, Description}`
3. `handleXxx(ctx, req, input)` → `(*CallToolResult, *Output, error)`

## Coding Rules

### Language
- All comments and documentation in **English**
- Commit messages in **English**

### Code Style
- `go fmt` before commit
- No `goimports` required (project is simple flat structure)
- `go vet ./...` must pass
- `go build ./...` must succeed

### Commit Rules
- **Single responsibility**: one feature/fix per commit
- Commit message: explain **why**, not just what
- Never `git add -A` or `git add .`
- Only commit files related to the current task
- Pre-commit: `go vet ./... && go build ./...`

### Error Handling
- Every error must be wrapped with context: `fmt.Errorf("failed to X: %w", err)`
- Never silently ignore errors
- Fail fast: config validation returns on first error

### Dependencies
```
github.com/modelcontextprotocol/go-sdk v1.6.1  # MCP protocol
github.com/go-sql-driver/mysql v1.10.0           # MySQL driver
github.com/xwb1989/sqlparser                      # SQL parsing (vitess-based)
gopkg.in/yaml.v3 v3.0.1                           # YAML config
```

## Key Design Decisions

### Single Package
All code in `package main`. The server is small enough that sub-packages would add overhead without benefit. Files are split by concern.

### Startup Connection Init
MySQL connections are established at startup, not lazily. If any instance fails to connect, the server exits immediately. Rationale: fail-fast is better than runtime surprises.

### 4-Layer SQL Security (parser.go)
```
SQL string
  → 1. Parse (vitess) — reject unparseable / injection patterns
  → 2. Single-statement check — reject multi-statement (; separated)
  → 3. LIMIT check — SELECT must have LIMIT ≤ instance.max_rows
  → 4. Read-only check — reject writes on read_only instances
```

All 4 layers execute before any DB call. No partial validation.

### Timeout Control
`context.WithTimeout` is created per query from `instance.timeout_seconds`. Context deadline is always set — the DB driver handles cancellation.

### Google Fonts
**Do not add Google Fonts**. The user prefers system fonts.

## Tool Details

| Tool | Input | SQL Check | Notes |
|------|-------|-----------|-------|
| `list_instances` | none | — | Returns metadata (no DSN secrets) |
| `query` | `instance_id`, `sql` | Full 4-layer | SELECT → rows, Write → affected |
| `describe_table` | `instance_id`, `table` | — | Internal `DESCRIBE` (not user SQL) |
| `explain_query` | `instance_id`, `sql` | Parse validation | Internal `EXPLAIN` (not user SQL) |

## Releasing

```bash
# 1. Build & test
go vet ./... && go build ./...

# 2. Commit & tag
git add <files> && git commit -m "feat: description"
git tag vX.Y.Z && git push origin main && git push origin vX.Y.Z

# 3. Create GitHub Release
gh release create vX.Y.Z --title "vX.Y.Z" --notes "..."
```

## GitHub Pages

Published via `docs/` folder on main branch.
Repo Settings → Pages → Source: `main`, folder: `/docs`.
URL: `https://phpgao.github.io/mysqlmcp/`
