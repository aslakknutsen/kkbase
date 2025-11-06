package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/kagenti/kkbase/pkg/agent"
	"github.com/kagenti/kkbase/pkg/agent/mcp"
	"github.com/kagenti/kkbase/pkg/agent/sources"
	"github.com/kagenti/kkbase/pkg/config"
	"github.com/kagenti/kkbase/pkg/llm"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

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
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Skip if agent is disabled
	if !cfg.AgentEnabled {
		fmt.Println("Agent is disabled, exiting")
		return nil
	}

	// Initialize logger
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

	// Create Kubernetes client
	logger.Info("initializing Kubernetes client")
	clientset, k8sConfig, err := createKubernetesClient(cfg.KubeConfigPath)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create MCP client
	logger.Info("connecting to MCP server", zap.String("url", cfg.AgentMCPServerURL))
	mcpClient, err := mcp.NewClient(cfg.AgentMCPServerURL, logger)
	if err != nil {
		return fmt.Errorf("failed to create MCP client: %w", err)
	}
	defer mcpClient.Close()

	// Create LLM client
	logger.Info("initializing LLM client",
		zap.String("provider", cfg.LLMProvider),
		zap.String("model", cfg.LLMModel))
	geminiClient, err := llm.NewGeminiClientFromConfig(
		llm.Config{
			Provider:       cfg.LLMProvider,
			APIKey:         cfg.LLMAPIKey,
			Model:          cfg.LLMModel,
			Temperature:    cfg.LLMTemperature,
			MaxTokens:      cfg.LLMMaxTokens,
			MaxIterations:  cfg.LLMMaxIterations,
			RateLimitRPS:   cfg.LLMRateLimitRPS,
			RateLimitBurst: cfg.LLMRateLimitBurst,
		},
		mcpClient,
		logger,
	)
	if err != nil {
		return fmt.Errorf("failed to create LLM client: %w", err)
	}
	defer geminiClient.Close()

	// Create agents (one per worker)
	// Agents now use MCP Server for all database operations
	logger.Info("creating agent workers", zap.Int("count", cfg.AgentWorkers))
	agents := make([]*agent.Agent, cfg.AgentWorkers)
	for i := 0; i < cfg.AgentWorkers; i++ {
		agents[i] = agent.NewAgent(
			fmt.Sprintf("agent-%d", i),
			geminiClient,
			mcpClient,
			logger,
		)
	}

	// Create event router
	logger.Info("initializing event router",
		zap.Strings("allowlist", cfg.EventFilterAllowlist),
		zap.Strings("denylist", cfg.EventFilterDenylist))
	router := agent.NewEventRouter(cfg, logger)

	// Create worker pool
	logger.Info("initializing worker pool", zap.Int("workers", cfg.AgentWorkers))
	workerPool := agent.NewWorkerPool(cfg.AgentWorkers, agents, logger)

	// Create unified HTTP server with mux
	logger.Info("initializing unified webhook server", zap.Int("port", cfg.AgentPort))
	mux := http.NewServeMux()

	// Register health endpoints
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		// Agent is ready if it can reach the MCP server
		// MCP server handles Neo4j connectivity
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	// Create event sources (conditionally based on config)
	logger.Info("initializing event sources")
	eventSources := []sources.EventSource{}

	if cfg.K8sEventsEnabled {
		logger.Info("enabling K8s events source")
		eventSources = append(eventSources, sources.NewK8sEventsSource(
			clientset, k8sConfig, cfg.Namespace, cfg.ResyncPeriod, logger))
	}

	if cfg.AlertmanagerWebhookEnabled {
		logger.Info("enabling Alertmanager webhook source")
		eventSources = append(eventSources, sources.NewAlertmanagerWebhook(mux, logger))
	}

	if cfg.CustomWebhookEnabled {
		logger.Info("enabling custom webhook source")
		eventSources = append(eventSources, sources.NewCustomWebhook(mux, logger))
	}

	if len(eventSources) == 0 {
		logger.Warn("no event sources enabled - agent will not receive any events")
	}

	// Start unified HTTP server
	unifiedServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.AgentPort),
		Handler: mux,
	}
	go func() {
		logger.Info("unified server listening", zap.Int("port", cfg.AgentPort))
		if err := unifiedServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("unified server error", zap.Error(err))
		}
	}()
	defer unifiedServer.Shutdown(context.Background())

	// Create context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker pool
	if err := workerPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	// Start event sources (K8sEventsSource starts its informer here)
	for _, source := range eventSources {
		if err := source.Start(ctx); err != nil {
			logger.Error("failed to start event source",
				zap.String("source", source.Name()),
				zap.Error(err))
			return fmt.Errorf("failed to start event source %s: %w", source.Name(), err)
		}
		logger.Info("event source started", zap.String("source", source.Name()))
	}

	// Start event processing loops (one per source)
	for _, source := range eventSources {
		go processEventSource(ctx, source, router, workerPool, logger)
	}

	logger.Info("agent service started successfully",
		zap.Int("port", cfg.AgentPort),
		zap.Int("workers", cfg.AgentWorkers))

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down gracefully...")

	// Stop event sources
	for _, source := range eventSources {
		if err := source.Stop(); err != nil {
			logger.Warn("error stopping event source",
				zap.String("source", source.Name()),
				zap.Error(err))
		}
	}

	// Stop worker pool
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

// createKubernetesClient creates a Kubernetes clientset and returns the config
func createKubernetesClient(kubeconfigPath string) (*kubernetes.Clientset, *rest.Config, error) {
	var config *rest.Config
	var err error

	if kubeconfigPath != "" {
		// Use kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
		}
	} else {
		// Use in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get in-cluster config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return clientset, config, nil
}
