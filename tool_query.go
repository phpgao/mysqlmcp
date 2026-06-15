package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type QueryInput struct {
	InstanceID string `json:"instance_id" jsonschema:"required, the instance ID to query"`
	SQL        string `json:"sql" jsonschema:"required, the SQL statement to execute (SELECT requires LIMIT)"`
}

type QueryOutput struct {
	Columns     []string         `json:"columns"`
	Rows        []map[string]any `json:"rows"`
	RowCount    int              `json:"row_count"`
	Truncated   bool             `json:"truncated,omitempty"`
	Affected    int64            `json:"affected_rows,omitempty"`
	InstanceID  string           `json:"instance_id"`
	Environment string           `json:"environment"`
}

func toolQuery() *mcp.Tool {
	return &mcp.Tool{
		Name:        "query",
		Description: "Execute a SQL statement on a MySQL instance. SELECT requires LIMIT. Maximum row count and timeout per instance config. Write operations rejected on read-only instances.",
	}
}

func handleQuery(ctx context.Context, req *mcp.CallToolRequest, input QueryInput) (
	*mcp.CallToolResult,
	*QueryOutput,
	error,
) {
	db, err := GetDB(input.InstanceID)
	if err != nil {
		return nil, nil, err
	}

	inst, err := GetInstance(input.InstanceID)
	if err != nil {
		return nil, nil, err
	}

	// 4-layer SQL analysis
	result, err := Analyze(input.SQL, inst.MaxRows, inst.ReadOnly)
	if err != nil {
		return nil, nil, fmt.Errorf("query rejected: %w", err)
	}

	// Create timeout context
	timeout := time.Duration(inst.TimeoutSeconds) * time.Second
	qCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	isSelect := result.StatementType == "SELECT"

	if isSelect {
		return executeSelect(qCtx, db, input, inst)
	}
	return executeWrite(qCtx, db, input, inst, result.StatementType)
}

func executeSelect(ctx context.Context, db *sql.DB, input QueryInput, inst InstanceConfig) (
	*mcp.CallToolResult,
	*QueryOutput,
	error,
) {
	rows, err := db.QueryContext(ctx, input.SQL)
	if err != nil {
		return nil, nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read columns: %w", err)
	}

	var rowData []map[string]any
	count := 0
	truncated := false

	for rows.Next() {
		if count >= inst.MaxRows {
			truncated = true
			break
		}

		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]any)
		for i, col := range columns {
			row[col] = formatValue(values[i])
		}
		rowData = append(rowData, row)
		count++
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("row iteration error: %w", err)
	}

	output := &QueryOutput{
		Columns:     columns,
		Rows:        rowData,
		RowCount:    count,
		Truncated:   truncated,
		InstanceID:  input.InstanceID,
		Environment: inst.Environment,
	}

	msg := fmt.Sprintf("Query returned %d rows", count)
	if truncated {
		msg += fmt.Sprintf(" (truncated, max %d rows)", inst.MaxRows)
	}
	msg += fmt.Sprintf("\nInstance: %s (%s)", input.InstanceID, inst.Environment)
	msg += fmt.Sprintf("\nColumns: %s", strings.Join(columns, ", "))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}, output, nil
}

func executeWrite(ctx context.Context, db *sql.DB, input QueryInput, inst InstanceConfig, stmtType string) (
	*mcp.CallToolResult,
	*QueryOutput,
	error,
) {
	res, err := db.ExecContext(ctx, input.SQL)
	if err != nil {
		return nil, nil, fmt.Errorf("exec failed: %w", err)
	}

	affected, _ := res.RowsAffected()

	output := &QueryOutput{
		Affected:    affected,
		InstanceID:  input.InstanceID,
		Environment: inst.Environment,
	}

	msg := fmt.Sprintf("%s executed successfully", stmtType)
	if affected > 0 {
		msg += fmt.Sprintf(", %d rows affected", affected)
	}
	msg += fmt.Sprintf("\nInstance: %s (%s)", input.InstanceID, inst.Environment)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}, output, nil
}

func formatValue(v any) any {
	switch val := v.(type) {
	case []byte:
		return string(val)
	case time.Time:
		return val.Format(time.RFC3339)
	default:
		return v
	}
}
