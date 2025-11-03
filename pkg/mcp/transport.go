package mcp

import (
	"encoding/json"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// CreateHTTPHandler creates an HTTP handler for the MCP server using SSE transport (supports notifications)
func CreateHTTPHandler(mcpServer *mcp.Server, logger *zap.Logger) http.Handler {
	mux := http.NewServeMux()

	// Create the SSE handler for MCP (bidirectional with notifications)
	sseHandler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			logger.Info("SSE session created", zap.String("remote_addr", r.RemoteAddr))
			return mcpServer
		},
		&mcp.StreamableHTTPOptions{},
	)

	// Main MCP endpoint - handles both SSE connections and message posts
	mux.Handle("/mcp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("received MCP request",
			zap.String("method", r.Method),
			zap.String("remote_addr", r.RemoteAddr))

		sseHandler.ServeHTTP(w, r)
	}))

	// Also handle /mcp/ path for session messages
	mux.Handle("/mcp/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("received MCP session message",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr))

		sseHandler.ServeHTTP(w, r)
	}))

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := map[string]interface{}{
			"status":  "healthy",
			"service": "kkbase-mcp",
			"version": "1.0.0",
		}

		json.NewEncoder(w).Encode(response)
	})

	logger.Info("HTTP handlers configured",
		zap.Strings("endpoints", []string{"/mcp", "/health"}))

	return mux
}
