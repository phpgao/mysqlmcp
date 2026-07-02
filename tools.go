package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ServerName    = "mysqlmcp"
	ServerVersion = "0.1.0"
)

func NewServer(cm *ConnectionManager) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    ServerName,
			Version: ServerVersion,
		},
		nil,
	)

	mcp.AddTool(server, toolListInstances(), func(ctx context.Context, req *mcp.CallToolRequest, input ListInstancesInput) (
		*mcp.CallToolResult, *ListInstancesOutput, error,
	) {
		return handleListInstances(ctx, req, input, cm)
	})

	mcp.AddTool(server, toolQuery(), func(ctx context.Context, req *mcp.CallToolRequest, input QueryInput) (
		*mcp.CallToolResult, *QueryOutput, error,
	) {
		return handleQuery(ctx, req, input, cm)
	})

	mcp.AddTool(server, toolDescribeTable(), func(ctx context.Context, req *mcp.CallToolRequest, input DescribeTableInput) (
		*mcp.CallToolResult, *DescribeTableOutput, error,
	) {
		return handleDescribeTable(ctx, req, input, cm)
	})

	mcp.AddTool(server, toolExplainQuery(), func(ctx context.Context, req *mcp.CallToolRequest, input ExplainQueryInput) (
		*mcp.CallToolResult, *ExplainQueryOutput, error,
	) {
		return handleExplainQuery(ctx, req, input, cm)
	})

	return server
}
