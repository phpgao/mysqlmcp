package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ===== list_instances =====

type ListInstancesInput struct{}

type InstanceMeta struct {
	InstanceID     string `json:"instance_id"`
	Environment    string `json:"environment"`
	ReadOnly       bool   `json:"read_only"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	MaxRows        int    `json:"max_rows"`
}

type ListInstancesOutput struct {
	Instances []InstanceMeta `json:"instances"`
}

func toolListInstances() *mcp.Tool {
	return &mcp.Tool{
		Name:        "list_instances",
		Description: "List all configured MySQL instances with their metadata (environment, read_only status, limits).",
	}
}

func handleListInstances(ctx context.Context, req *mcp.CallToolRequest, input ListInstancesInput) (
	*mcp.CallToolResult,
	*ListInstancesOutput,
	error,
) {
	var metas []InstanceMeta
	for _, id := range getInstanceIDs() {
		inst, _ := GetInstance(id)
		metas = append(metas, InstanceMeta{
			InstanceID:     inst.InstanceID,
			Environment:    inst.Environment,
			ReadOnly:       inst.ReadOnly,
			TimeoutSeconds: inst.TimeoutSeconds,
			MaxRows:        inst.MaxRows,
		})
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d instance(s) available:\n", len(metas)))
	for _, m := range metas {
		sb.WriteString(fmt.Sprintf("  - %s (env=%s, read_only=%v, timeout=%ds, max_rows=%d)\n",
			m.InstanceID, m.Environment, m.ReadOnly, m.TimeoutSeconds, m.MaxRows))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: sb.String()},
		},
	}, &ListInstancesOutput{Instances: metas}, nil
}

// ===== describe_table =====

type DescribeTableInput struct {
	InstanceID string `json:"instance_id" jsonschema:"required, the instance ID"`
	Table      string `json:"table" jsonschema:"required, the table name to describe"`
}

type DescribeTableOutput struct {
	Rows []map[string]any `json:"rows"`
}

func toolDescribeTable() *mcp.Tool {
	return &mcp.Tool{
		Name:        "describe_table",
		Description: "Show the schema of a table (DESCRIBE). Safe read-only operation.",
	}
}

func handleDescribeTable(ctx context.Context, req *mcp.CallToolRequest, input DescribeTableInput) (
	*mcp.CallToolResult,
	*DescribeTableOutput,
	error,
) {
	db, err := GetDB(input.InstanceID)
	if err != nil {
		return nil, nil, err
	}

	_, err = GetInstance(input.InstanceID)
	if err != nil {
		return nil, nil, err
	}

	sql := fmt.Sprintf("DESCRIBE `%s`", strings.ReplaceAll(input.Table, "`", "``"))
	rows, err := db.QueryContext(ctx, sql)
	if err != nil {
		return nil, nil, fmt.Errorf("describe failed: %w", err)
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	var rowData []map[string]any

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, fmt.Errorf("scan failed: %w", err)
		}
		row := make(map[string]any)
		for i, col := range columns {
			row[col] = formatValue(values[i])
		}
		rowData = append(rowData, row)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Table `%s` schema (instance: %s):\n", input.Table, input.InstanceID))
	for _, r := range rowData {
		sb.WriteString(fmt.Sprintf("  %v | %v | %v | %v | %v\n",
			r["Field"], r["Type"], r["Null"], r["Key"], r["Default"]))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: sb.String()},
		},
	}, &DescribeTableOutput{Rows: rowData}, nil
}

// ===== explain_query =====

type ExplainQueryInput struct {
	InstanceID string `json:"instance_id" jsonschema:"required, the instance ID"`
	SQL        string `json:"sql" jsonschema:"required, the SQL to explain"`
}

type ExplainQueryOutput struct {
	Rows []map[string]any `json:"rows"`
}

func toolExplainQuery() *mcp.Tool {
	return &mcp.Tool{
		Name:        "explain_query",
		Description: "Show the execution plan for a SQL query (EXPLAIN). The SQL is validated but not executed.",
	}
}

func handleExplainQuery(ctx context.Context, req *mcp.CallToolRequest, input ExplainQueryInput) (
	*mcp.CallToolResult,
	*ExplainQueryOutput,
	error,
) {
	db, err := GetDB(input.InstanceID)
	if err != nil {
		return nil, nil, err
	}

	_, err = GetInstance(input.InstanceID)
	if err != nil {
		return nil, nil, err
	}

	// Validate that the SQL itself parses
	_, err = Analyze(input.SQL, 1000, true) // use lenient limits for EXPLAIN
	if err != nil {
		return nil, nil, fmt.Errorf("SQL validation failed: %w", err)
	}

	sql := fmt.Sprintf("EXPLAIN %s", input.SQL)
	rows, err := db.QueryContext(ctx, sql)
	if err != nil {
		return nil, nil, fmt.Errorf("explain failed: %w", err)
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	var rowData []map[string]any

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, fmt.Errorf("scan failed: %w", err)
		}
		row := make(map[string]any)
		for i, col := range columns {
			row[col] = formatValue(values[i])
		}
		rowData = append(rowData, row)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("EXPLAIN for: %s\n\n", input.SQL))
	sb.WriteString(strings.Join(columns, " | ") + "\n")
	sb.WriteString(strings.Repeat("-", 60) + "\n")
	for _, r := range rowData {
		parts := make([]string, len(columns))
		for i, col := range columns {
			parts[i] = fmt.Sprintf("%v", r[col])
		}
		sb.WriteString(strings.Join(parts, " | ") + "\n")
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: sb.String()},
		},
	}, &ExplainQueryOutput{Rows: rowData}, nil
}
