package main

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ServerName    = "mysqlmcp"
	ServerVersion = "0.1.0"
)

func NewServer() *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    ServerName,
			Version: ServerVersion,
		},
		nil,
	)

	mcp.AddTool(server, toolListInstances(), handleListInstances)
	mcp.AddTool(server, toolQuery(), handleQuery)
	mcp.AddTool(server, toolDescribeTable(), handleDescribeTable)
	mcp.AddTool(server, toolExplainQuery(), handleExplainQuery)

	return server
}
