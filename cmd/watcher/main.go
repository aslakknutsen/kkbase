package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kagenti/kkbase/pkg/config"
	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/graph/neo4j"
	"github.com/kagenti/kkbase/pkg/mcp"
	"github.com/kagenti/kkbase/pkg/observability"
	"github.com/kagenti/kkbase/pkg/observability/jaeger"
	"github.com/kagenti/kkbase/pkg/observability/prometheus"
	"github.com/kagenti/kkbase/pkg/watchers"
	"github.com/kagenti/kkbase/pkg/watchers/handlers/core"
	"github.com/kagenti/kkbase/pkg/watchers/handlers/extensions/gateway"
	"github.com/kagenti/kkbase/pkg/watchers/handlers/extensions/istio"
	"go.uber.org/zap"
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
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger
	logger, err := initLogger(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logger.Sync()

	logger.Info("starting kubernetes knowledge graph watcher",
		zap.String("namespace", cfg.Namespace),
		zap.Duration("resync_period", cfg.ResyncPeriod),
	)

	// Create Kubernetes clientset and config
	clientset, k8sConfig, err := createKubernetesClient(cfg.KubeConfigPath)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

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

	// Start MCP server if enabled
	var mcpHTTPServer *http.Server
	if cfg.MCPEnabled {
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

		// Create MCP server with optional metrics processor
		var mcpServer *mcp.Server
		if metricsProcessor != nil {
			mcpServer, err = mcp.NewServer(graphStore, logger, mcp.WithMetricsProcessor(metricsProcessor))
		} else {
			mcpServer, err = mcp.NewServer(graphStore, logger)
		}
		if err != nil {
			return fmt.Errorf("failed to create MCP server: %w", err)
		}
		defer mcpServer.Close()

		mcpHandler := mcp.CreateHTTPHandler(mcpServer.GetMCPServer(), logger)
		mcpHTTPServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.MCPPort),
			Handler:      mcpHandler,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		go func() {
			logger.Info("starting MCP server",
				zap.Int("port", cfg.MCPPort),
				zap.String("endpoint", fmt.Sprintf("http://localhost:%d/mcp", cfg.MCPPort)))

			if err := mcpHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("MCP server failed", zap.Error(err))
			}
		}()

		logger.Info("MCP server enabled and started",
			zap.Int("port", cfg.MCPPort))
	}

	// Initialize observability registry
	obsRegistry := observability.NewRegistry()
	defer obsRegistry.Close()

	// Initialize trace provider if enabled
	if cfg.EnableTraces && cfg.JaegerQueryURL != "" {
		jaegerProvider := jaeger.NewProvider(cfg.JaegerQueryURL, logger)
		obsRegistry.RegisterTracesProvider(jaegerProvider)

		traceProcessor := observability.NewTraceProcessor(graphStore, logger, cfg.JaegerSpanRetention)

		// Start trace streaming in background
		go func() {
			if err := startTraceStreaming(ctx, jaegerProvider, traceProcessor, graphStore, cfg, logger); err != nil {
				logger.Error("trace streaming failed", zap.Error(err))
			}
		}()

		logger.Info("started Jaeger trace polling",
			zap.String("url", cfg.JaegerQueryURL),
			zap.Duration("interval", cfg.JaegerPollInterval),
			zap.Duration("retention", cfg.JaegerSpanRetention))
	}

	// TODO: Register other observability providers when implemented
	// if cfg.EnableMetrics {
	//     obsRegistry.RegisterMetricsProvider(metricsProvider)
	// }

	// Create watcher manager with dynamic client
	watcherManager, err := watchers.NewManager(watchers.Config{
		Clientset:    clientset,
		GraphStore:   graphStore,
		Logger:       logger,
		ResyncPeriod: cfg.ResyncPeriod,
		Namespace:    cfg.Namespace,
	}, k8sConfig)
	if err != nil {
		return fmt.Errorf("failed to create watcher manager: %w", err)
	}

	// Get shared factory and clients
	factory := watcherManager.GetFactory()
	dynamicClient := watcherManager.GetDynamicClient()

	// Register core handlers (always available - no CRD check needed)
	core.RegisterCoreHandlers(watcherManager, clientset, factory, graphStore, logger)

	// Register Gateway API handlers (dynamic registration via CRD watcher)
	gateway.RegisterGatewayAPIHandlers(watcherManager, dynamicClient, factory, graphStore, logger)

	// Register Istio handlers (dynamic registration via CRD watcher)
	istio.RegisterIstioHandlers(watcherManager, dynamicClient, factory, graphStore, logger)

	logger.Info("handler registration complete")

	// Start health check server
	healthServer := startHealthServer(logger, graphStore)
	defer healthServer.Shutdown(context.Background())

	// Start watchers in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := watcherManager.Start(ctx); err != nil {
			errChan <- fmt.Errorf("watcher manager failed: %w", err)
		}
	}()

	// Start placeholder monitoring
	go monitorPlaceholders(ctx, graphStore, logger)

	logger.Info("watcher started successfully")

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		logger.Info("received shutdown signal")
		cancel()
	case err := <-errChan:
		logger.Error("watcher error", zap.Error(err))
		cancel()
		return err
	}

	logger.Info("shutting down gracefully")

	// Shutdown MCP server if it's running
	if mcpHTTPServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		logger.Info("shutting down MCP server")
		if err := mcpHTTPServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("error shutting down MCP server", zap.Error(err))
		} else {
			logger.Info("MCP server shut down successfully")
		}
	}

	return nil
}

// initLogger initializes the zap logger
func initLogger(level string) (*zap.Logger, error) {
	var zapLevel zap.AtomicLevel
	switch level {
	case "debug":
		zapLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapLevel = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapLevel = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	config := zap.NewProductionConfig()
	config.Level = zapLevel
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

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

// startHealthServer starts an HTTP server for health checks
func startHealthServer(logger *zap.Logger, graphStore graph.GraphStore) *http.Server {
	mux := http.NewServeMux()

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

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		logger.Info("starting health check server", zap.String("addr", ":8080"))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server error", zap.Error(err))
		}
	}()

	return server
}

// startTraceStreaming starts the trace streaming process
func startTraceStreaming(ctx context.Context, provider observability.TracesProvider,
	processor *observability.TraceProcessor, graphStore graph.GraphStore,
	cfg *config.Config, logger *zap.Logger) error {

	// Discover services every 5 minutes
	discoveryTicker := time.NewTicker(5 * time.Minute)
	defer discoveryTicker.Stop()

	// Initial service discovery
	services, err := observability.DiscoverMonitoredServices(ctx, graphStore, logger)
	if err != nil {
		logger.Error("failed to discover services", zap.Error(err))
		services = []observability.ServiceInfo{}
	}
	serviceNames := observability.ExtractServiceNames(services)

	logger.Info("starting trace stream",
		zap.Int("service_count", len(serviceNames)),
		zap.Strings("services", serviceNames))

	// Start streaming traces
	traceStream, err := provider.StreamTraces(ctx, serviceNames, cfg.JaegerPollInterval)
	if err != nil {
		return fmt.Errorf("failed to start trace stream: %w", err)
	}

	for {
		select {
		case trace, ok := <-traceStream:
			if !ok {
				logger.Info("trace stream closed")
				return nil
			}

			// Process the trace
			if err := processor.ProcessTrace(ctx, trace); err != nil {
				logger.Warn("failed to process trace",
					zap.String("trace_id", trace.TraceID),
					zap.Error(err))
			} else {
				logger.Debug("processed trace",
					zap.String("trace_id", trace.TraceID),
					zap.Int("span_count", trace.SpanCount),
					zap.Bool("has_errors", trace.HasErrors))
			}

		case <-discoveryTicker.C:
			// Re-discover services periodically
			newServices, err := observability.DiscoverMonitoredServices(ctx, graphStore, logger)
			if err != nil {
				logger.Error("failed to rediscover services", zap.Error(err))
				continue
			}

			newServiceNames := observability.ExtractServiceNames(newServices)
			if len(newServiceNames) != len(serviceNames) {
				logger.Info("service list updated",
					zap.Int("previous_count", len(serviceNames)),
					zap.Int("new_count", len(newServiceNames)))
				serviceNames = newServiceNames
			}

		case <-ctx.Done():
			logger.Info("stopping trace stream")
			return nil
		}
	}
}

// monitorPlaceholders monitors and cleans up placeholder nodes
func monitorPlaceholders(ctx context.Context, graphStore graph.GraphStore, logger *zap.Logger) {
	ticker := time.NewTicker(20 * time.Minute)
	defer ticker.Stop()

	nodeTypes := []string{
		"Service", "Pod", "ConfigMap", "Secret", "Node",
		"PersistentVolume", "PersistentVolumeClaim", "Namespace",
		"Deployment", "ReplicaSet", "StatefulSet", "DaemonSet",
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping placeholder monitoring")
			return
		case <-ticker.C:
			// Check for placeholder nodes
			for _, nodeType := range nodeTypes {
				placeholders, err := graphStore.GetPlaceholderNodes(ctx, nodeType)
				if err != nil {
					logger.Warn("failed to get placeholder nodes",
						zap.String("type", nodeType),
						zap.Error(err))
					continue
				}

				if len(placeholders) > 0 {
					logger.Info("placeholder nodes detected",
						zap.String("type", nodeType),
						zap.Int("count", len(placeholders)))
				}
			}

			// Cleanup orphaned placeholders older than 1 hour
			if err := graphStore.CleanupOrphanedPlaceholders(ctx, 1*time.Hour); err != nil {
				logger.Warn("failed to cleanup orphaned placeholders", zap.Error(err))
			}
		}
	}
}
