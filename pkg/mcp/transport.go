package mcp

import (
	"encoding/json"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// CreateHTTPHandler creates an HTTP handler for the MCP server using streamable HTTP transport
func CreateHTTPHandler(mcpServer *mcp.Server, logger *zap.Logger) http.Handler {
	mux := http.NewServeMux()

	// Create the streamable HTTP handler for MCP
	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			return mcpServer
		},
		&mcp.StreamableHTTPOptions{},
	)

	// Main MCP endpoint
	mux.Handle("/mcp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("received MCP request",
			zap.String("method", r.Method),
			zap.String("remote_addr", r.RemoteAddr))

		mcpHandler.ServeHTTP(w, r)
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
