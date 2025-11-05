package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kagenti/kkbase/pkg/agent"
	"github.com/kagenti/kkbase/pkg/agent/mcp"
	"github.com/kagenti/kkbase/pkg/agent/sources"
	"github.com/kagenti/kkbase/pkg/config"
	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/graph/neo4j"
	"github.com/kagenti/kkbase/pkg/llm"
	"github.com/kagenti/kkbase/pkg/observability"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Skip if agent is disabled
	if !cfg.AgentEnabled {
		fmt.Println("Agent is disabled, exiting")
		return nil
	}

	// 2. Initialize logger
	logger, err := initLogger(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}
	defer logger.Sync()

	logger.Info("starting kkbase agent",
		zap.String("version", "1.0.0"),
		zap.String("llm_provider", cfg.LLMProvider),
		zap.String("llm_model", cfg.LLMModel),
		zap.Int("workers", cfg.AgentWorkers))

	// Validate LLM API key
	if cfg.LLMAPIKey == "" {
		return fmt.Errorf("LLM_API_KEY is required when agent is enabled")
	}

	// 3. Connect to Neo4j
	logger.Info("connecting to Neo4j", zap.String("uri", cfg.Neo4jURI))
	graphStore, err := neo4j.NewStore(neo4j.Config{
		URI:      cfg.Neo4jURI,
		Username: cfg.Neo4jUsername,
		Password: cfg.Neo4jPassword,
		Database: cfg.Neo4jDatabase,
	}, logger)
	if err != nil {
		return fmt.Errorf("failed to connect to Neo4j: %w", err)
	}
	defer graphStore.Close()

	// 4. Create MCP client
	logger.Info("connecting to MCP server", zap.String("url", cfg.AgentMCPServerURL))
	mcpClient, err := mcp.NewClient(cfg.AgentMCPServerURL, logger)
	if err != nil {
		return fmt.Errorf("failed to create MCP client: %w", err)
	}
	defer mcpClient.Close()

	// 5. Create LLM client
	logger.Info("initializing LLM client",
		zap.String("provider", cfg.LLMProvider),
		zap.String("model", cfg.LLMModel))
	geminiClient, err := llm.NewGeminiClientFromConfig(
		llm.Config{
			Provider:    cfg.LLMProvider,
			APIKey:      cfg.LLMAPIKey,
			Model:       cfg.LLMModel,
			Temperature: cfg.LLMTemperature,
			MaxTokens:   cfg.LLMMaxTokens,
		},
		mcpClient,
		logger,
	)
	if err != nil {
		return fmt.Errorf("failed to create LLM client: %w", err)
	}
	defer geminiClient.Close()

	// 6. Create agent session manager
	logger.Info("initializing agent session manager")
	sessionManager := observability.NewAgentSessionManager(
		graphStore,
		nil, // No metrics processor for now
		cfg,
		logger,
	)

	// 7. Create agents (one per worker)
	logger.Info("creating agent workers", zap.Int("count", cfg.AgentWorkers))
	agents := make([]*agent.Agent, cfg.AgentWorkers)
	for i := 0; i < cfg.AgentWorkers; i++ {
		agents[i] = agent.NewAgent(
			fmt.Sprintf("agent-%d", i),
			geminiClient,
			mcpClient,
			graphStore,
			sessionManager,
			logger,
		)
	}

	// 8. Create event router
	logger.Info("initializing event router",
		zap.Strings("allowlist", cfg.EventFilterAllowlist),
		zap.Strings("denylist", cfg.EventFilterDenylist))
	router := agent.NewEventRouter(cfg, logger)

	// 9. Create worker pool
	logger.Info("initializing worker pool", zap.Int("workers", cfg.AgentWorkers))
	workerPool := agent.NewWorkerPool(cfg.AgentWorkers, agents, logger)

	// 10. Create event sources
	logger.Info("initializing event sources")
	eventSources := []sources.EventSource{
		sources.NewK8sEventsSource(graphStore, logger),
		sources.NewAlertmanagerWebhook(cfg.AgentPort, logger),
		sources.NewCustomWebhook(cfg.AgentPort, logger),
	}

	// 11. Create context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 12. Start worker pool
	if err := workerPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	// 13. Start event sources
	for _, source := range eventSources {
		if err := source.Start(ctx); err != nil {
			logger.Error("failed to start event source",
				zap.String("source", source.Name()),
				zap.Error(err))
			return fmt.Errorf("failed to start event source %s: %w", source.Name(), err)
		}
		logger.Info("event source started", zap.String("source", source.Name()))
	}

	// 14. Start event processing loops (one per source)
	for _, source := range eventSources {
		go processEventSource(ctx, source, router, workerPool, logger)
	}

	// 15. Start health server
	healthServer := startHealthServer(cfg.AgentPort, logger, graphStore)
	defer healthServer.Shutdown(context.Background())

	logger.Info("agent service started successfully",
		zap.Int("port", cfg.AgentPort),
		zap.Int("workers", cfg.AgentWorkers))

	// 16. Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down gracefully...")

	// 17. Stop event sources
	for _, source := range eventSources {
		if err := source.Stop(); err != nil {
			logger.Warn("error stopping event source",
				zap.String("source", source.Name()),
				zap.Error(err))
		}
	}

	// 18. Stop worker pool
	workerPool.Stop()

	logger.Info("shutdown complete")
	return nil
}

// processEventSource processes events from a source
func processEventSource(ctx context.Context, source sources.EventSource, router *agent.EventRouter, pool *agent.WorkerPool, logger *zap.Logger) {
	logger.Info("starting event processor", zap.String("source", source.Name()))

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping event processor", zap.String("source", source.Name()))
			return
		case event, ok := <-source.Events():
			if !ok {
				logger.Info("event channel closed", zap.String("source", source.Name()))
				return
			}

			// Process through router
			accepted, processed, err := router.Process(event)
			if err != nil {
				logger.Error("router error",
					zap.String("source", source.Name()),
					zap.String("event_id", event.ID),
					zap.Error(err))
				continue
			}

			if !accepted {
				// Event was filtered out
				continue
			}

			// Submit to worker pool
			if err := pool.Submit(*processed); err != nil {
				logger.Warn("failed to submit event to worker pool",
					zap.String("event_id", event.ID),
					zap.Error(err))
			}
		}
	}
}

// startHealthServer starts the health check HTTP server
func startHealthServer(port int, logger *zap.Logger, graphStore graph.GraphStore) *http.Server {
	mux := http.NewServeMux()

	// Liveness probe
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Readiness probe - checks Neo4j connectivity
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		// Simple health check by running a query
		_, err := graphStore.Query(ctx, "RETURN 1", nil)
		if err != nil {
			logger.Warn("health check failed", zap.Error(err))
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		logger.Info("health server listening", zap.Int("port", port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server error", zap.Error(err))
		}
	}()

	return server
}

// initLogger initializes the zap logger
func initLogger(level string) (*zap.Logger, error) {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapLevel)
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	return config.Build()
}
