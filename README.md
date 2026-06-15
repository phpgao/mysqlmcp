# mysqlmcp

MySQL MCP server — multi-instance MySQL access for AI assistants via MCP protocol.

## Features

- **Multi-instance**: Connect to multiple MySQL servers simultaneously
- **Read-only enforcement**: Prevent write operations on production read replicas
- **SQL static analysis**: Parse and validate SQL before execution
- **Security**: LIMIT required on SELECT, single-statement only, timeout control
- **Dual transport**: stdio (local) and HTTP (remote with Bearer token auth)
- **Zero dependencies on model**: Works with any MCP-compatible client

## Quick Start

### Install

```
go install github.com/phpgao/mysqlmcp@latest
```

### Configure

Create `config.yaml`:

```yaml
instances:
  - instance_id: "local"
    dsn: "root:password@tcp(127.0.0.1:3306)/mydb?charset=utf8mb4&parseTime=true"
    environment: "development"
    read_only: false
```

### Run (stdio mode)

Add to your MCP client config (`~/.codebuddy/mcp.json`):

```json
{
  "mcpServers": {
    "mysqlmcp": {
      "command": "mysqlmcp",
      "args": ["-config", "/path/to/config.yaml"]
    }
  }
}
```

### Run (HTTP mode)

```bash
# Generate a secure token
export MYSQLMCP_TOKEN="your-secure-token"

# Start HTTP server
mysqlmcp -transport http -port 8000
```

MCP client config (remote access):

```json
{
  "mcpServers": {
    "mysqlmcp": {
      "url": "http://localhost:8000/mcp",
      "headers": {
        "Authorization": "Bearer your-secure-token"
      }
    }
  }
}
```

### Configuration Reference

```yaml
server:
  token: "your-secret-http-token"  # HTTP mode Bearer token
  port: 8000

defaults:
  timeout_seconds: 30    # Max query timeout
  max_rows: 10000        # Max return rows

instances:
  - instance_id: "prod-readonly"     # Unique ID for quick lookup
    dsn: "user:pass@tcp(host:3306)/db?charset=utf8mb4&parseTime=true"
    environment: "production"
    read_only: true                   # Rejects INSERT/UPDATE/DELETE
    timeout_seconds: 30
    max_rows: 1000

  - instance_id: "staging-rw"
    dsn: "user:pass@tcp(host:3307)/db?charset=utf8mb4&parseTime=true"
    environment: "staging"
    read_only: false
    timeout_seconds: 60
    max_rows: 5000
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `list_instances` | List all configured instances with metadata |
| `query` | Execute SQL (SELECT requires LIMIT, read-only enforced) |
| `describe_table` | Show table schema (DESCRIBE) |
| `explain_query` | Show execution plan (EXPLAIN) |

## SQL Security

Every SQL statement goes through 4-layer validation:

1. **Parse & Inject Detection**: SQL must parse cleanly, dangerous operations blocked (DROP DATABASE, etc.)
2. **Single Statement**: Multiple statements separated by `;` are rejected
3. **LIMIT Check**: SELECT must include a LIMIT clause, cannot exceed `max_rows`
4. **Read-only Check**: Write operations (INSERT/UPDATE/DELETE) rejected on read-only instances

Plus: **Query timeout** enforced per-instance, **row count** truncated at `max_rows`.

## License

MIT
