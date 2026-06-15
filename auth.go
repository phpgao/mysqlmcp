package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/auth"
)

// ResolveToken resolves the server token from command-line flag > env var > config.
func ResolveToken(flagToken, configToken string) (string, error) {
	// 1. Command-line flag (highest priority)
	if flagToken != "" {
		return flagToken, nil
	}

	// 2. Environment variable
	if env := os.Getenv("MYSQLMCP_TOKEN"); env != "" {
		return env, nil
	}

	// 3. Config file
	if configToken != "" {
		return configToken, nil
	}

	return "", fmt.Errorf("no token configured: set -token flag, MYSQLMCP_TOKEN env, or server.token in config.yaml")
}

// NewAuthMiddleware returns an HTTP middleware that validates Bearer tokens.
func NewAuthMiddleware(token string) func(http.Handler) http.Handler {
	verifier := func(ctx context.Context, tok string, req *http.Request) (*auth.TokenInfo, error) {
		if tok != token {
			return nil, fmt.Errorf("invalid token")
		}
		return &auth.TokenInfo{}, nil
	}
	return auth.RequireBearerToken(verifier, nil)
}
