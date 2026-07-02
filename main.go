package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to YAML config file")
	transport := flag.String("transport", "stdio", "transport mode: stdio or http")
	port := flag.Int("port", 0, "HTTP port (overrides config)")
	flagToken := flag.String("token", "", "HTTP Bearer token (overrides config and env)")
	flag.Parse()

	// Load config
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("mysqlmcp: %v", err)
	}

	// Init connection manager
	cm := NewConnectionManager()
	if err := cm.InitFromConfig(cfg); err != nil {
		log.Fatalf("mysqlmcp: %v", err)
	}

	// Create server
	server := NewServer(cm)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal handling
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		log.Println("mysqlmcp: shutting down...")
		cancel()
	}()

	switch *transport {
	case "stdio":
		// Start config watcher before entering stdio loop
		go watchConfig(*configPath, cm)

		log.Printf("mysqlmcp %s starting on stdio...", ServerVersion)
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
			log.Fatalf("mysqlmcp: server error: %v", err)
		}

	case "http":
		token, err := ResolveToken(*flagToken, cfg.Server.Token)
		if err != nil {
			log.Fatalf("mysqlmcp: %v", err)
		}

		listenPort := cfg.Server.Port
		if *port > 0 {
			listenPort = *port
		}
		if listenPort == 0 {
			listenPort = 8000
		}

		addr := fmt.Sprintf(":%d", listenPort)
		handler := mcp.NewStreamableHTTPHandler(
			func(r *http.Request) *mcp.Server { return server },
			nil,
		)

		authMw := NewAuthMiddleware(token)
		http.Handle("/mcp", authMw(handler))

		// Start config watcher
		go watchConfig(*configPath, cm)

		log.Printf("mysqlmcp %s HTTP server starting on %s/mcp", ServerVersion, addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("mysqlmcp: HTTP server error: %v", err)
		}

	default:
		log.Fatalf("mysqlmcp: unknown transport %q (use stdio or http)", *transport)
	}

	// Clean shutdown: close all connections
	cm.Close()
}
