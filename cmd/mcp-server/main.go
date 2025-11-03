package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kagenti/kkbase/pkg/config"
	"github.com/kagenti/kkbase/pkg/graph/neo4j"
	"github.com/kagenti/kkbase/pkg/mcp"
	"github.com/kagenti/kkbase/pkg/observability"
	"github.com/kagenti/kkbase/pkg/observability/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger
	logger, err := initLogger(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logger.Sync()

	logger.Info("starting kkbase MCP server",
		zap.Int("port", cfg.MCPPort),
		zap.String("neo4j_uri", cfg.Neo4jURI))

	// Initialize Neo4j graph store
	graphStore, err := neo4j.NewStore(neo4j.Config{
		URI:        cfg.Neo4jURI,
		Username:   cfg.Neo4jUsername,
		Password:   cfg.Neo4jPassword,
		Database:   cfg.Neo4jDatabase,
		MaxRetries: 3,
	}, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize graph store: %w", err)
	}
	defer graphStore.Close()

	logger.Info("successfully connected to Neo4j", zap.String("uri", cfg.Neo4jURI))

	// Create context that listens for interrupt signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Initialize metrics processor (optional)
	var metricsProcessor *observability.InvestigationMetricsProcessor
	if cfg.PrometheusURL != "" {
		logger.Info("initializing metrics integration for MCP",
			zap.String("prometheus_url", cfg.PrometheusURL))

		// Create Prometheus provider
		promProvider := prometheus.NewProvider(cfg.PrometheusURL, logger)

		// Create investigation metrics processor
		metricsProcessor = observability.NewInvestigationMetricsProcessor(
			graphStore,
			promProvider,
			logger,
		)

		logger.Info("metrics integration enabled - investigation tools available")
	} else {
		logger.Info("PROMETHEUS_URL not set - metrics investigation tools disabled")
	}

	// Create agent session manager for investigation tracking
	sessionManager := observability.NewAgentSessionManager(graphStore, metricsProcessor, logger)

	// Create notification broadcaster
	broadcaster := mcp.NewNotificationBroadcaster(logger)

	// Create MCP server with optional components
	var mcpServer *mcp.Server
	if metricsProcessor != nil {
		mcpServer, err = mcp.NewServer(
			graphStore,
			logger,
			mcp.WithMetricsProcessor(metricsProcessor),
			mcp.WithAgentSessionManager(sessionManager),
			mcp.WithBroadcaster(broadcaster),
		)
	} else {
		mcpServer, err = mcp.NewServer(
			graphStore,
			logger,
			mcp.WithAgentSessionManager(sessionManager),
			mcp.WithBroadcaster(broadcaster),
		)
	}
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}
	defer mcpServer.Close()

	// Create HTTP handler
	mcpHandler := mcp.CreateHTTPHandler(mcpServer.GetMCPServer(), logger)

	// Create main HTTP mux
	mux := http.NewServeMux()

	// Health check endpoints
	// Liveness probe - just checks if the app is running
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Readiness probe - checks if Neo4j is accessible
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if err := graphStore.HealthCheck(ctx); err != nil {
			logger.Error("readiness check failed", zap.Error(err))
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("not ready"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	// Mount MCP handler at /mcp
	mux.Handle("/mcp", mcpHandler)
	mux.Handle("/mcp/", mcpHandler)

	// Register SSE endpoint for push notifications
	mux.Handle("/events", broadcaster.GetSSEHandler())

	// Serve embedded frontend
	frontendDist, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		return fmt.Errorf("failed to access embedded frontend: %w", err)
	}
	fileServer := http.FileServer(http.FS(frontendDist))
	mux.Handle("/assets/", fileServer)
	mux.Handle("/", fileServer)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.MCPPort),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start HTTP server in background
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("MCP server listening",
			zap.Int("port", cfg.MCPPort),
			zap.String("dashboard", fmt.Sprintf("http://localhost:%d/", cfg.MCPPort)),
			zap.String("mcp_endpoint", fmt.Sprintf("http://localhost:%d/mcp", cfg.MCPPort)),
			zap.String("events_endpoint", fmt.Sprintf("http://localhost:%d/events", cfg.MCPPort)),
			zap.String("health_endpoints", fmt.Sprintf("http://localhost:%d/healthz, http://localhost:%d/ready", cfg.MCPPort, cfg.MCPPort)))

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("MCP server failed: %w", err)
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case <-sigChan:
		logger.Info("received shutdown signal")
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info("context cancelled")
	}

	// Graceful shutdown
	logger.Info("shutting down gracefully")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	logger.Info("shutting down MCP server")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("error shutting down MCP server", zap.Error(err))
	} else {
		logger.Info("MCP server shut down successfully")
	}

	return nil
}

// initLogger initializes a zap logger with the specified log level
func initLogger(level string) (*zap.Logger, error) {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", level, err)
	}

	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapLevel)
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return logger, nil
}
