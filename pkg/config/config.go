package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the application configuration
type Config struct {
	// Neo4j configuration
	Neo4jURI      string
	Neo4jUsername string
	Neo4jPassword string
	Neo4jDatabase string

	// Kubernetes configuration
	KubeConfigPath string
	Namespace      string // Empty string means all namespaces
	ResyncPeriod   time.Duration

	// Logging
	LogLevel string

	// Feature flags
	EnableMetrics bool
	EnableLogs    bool
	EnableTraces  bool

	// Jaeger configuration
	JaegerQueryURL       string
	JaegerPollInterval   time.Duration
	JaegerLookbackWindow time.Duration
	JaegerSpanRetention  time.Duration

	// Prometheus configuration
	PrometheusURL string

	// MCP Server configuration
	MCPPort    int
	MCPEnabled bool

	// Agent session configuration
	CompletedSessionRetentionMinutes int
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		Neo4jURI:      getEnv("NEO4J_URI", "bolt://localhost:7687"),
		Neo4jUsername: getEnv("NEO4J_USERNAME", "neo4j"),
		Neo4jPassword: getEnv("NEO4J_PASSWORD", ""),
		Neo4jDatabase: getEnv("NEO4J_DATABASE", "neo4j"),

		KubeConfigPath: getEnv("KUBECONFIG", ""),
		Namespace:      getEnv("NAMESPACE", ""),
		LogLevel:       getEnv("LOG_LEVEL", "info"),

		EnableMetrics: getBoolEnv("ENABLE_METRICS", false),
		EnableLogs:    getBoolEnv("ENABLE_LOGS", false),
		EnableTraces:  getBoolEnv("ENABLE_TRACES", false),

		JaegerQueryURL: getEnv("JAEGER_QUERY_URL", "http://localhost:16686"),

		PrometheusURL: getEnv("PROMETHEUS_URL", ""),

		MCPPort:    getIntEnv("MCP_PORT", 8080),
		MCPEnabled: getBoolEnv("MCP_ENABLED", false),

		CompletedSessionRetentionMinutes: getIntEnv("COMPLETED_SESSION_RETENTION_MINUTES", 1440),
	}

	// Parse resync period
	resyncPeriodStr := getEnv("RESYNC_PERIOD", "30s")
	resyncPeriod, err := time.ParseDuration(resyncPeriodStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RESYNC_PERIOD: %w", err)
	}
	cfg.ResyncPeriod = resyncPeriod

	// Parse Jaeger poll interval
	pollIntervalStr := getEnv("JAEGER_POLL_INTERVAL", "30s")
	pollInterval, err := time.ParseDuration(pollIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid JAEGER_POLL_INTERVAL: %w", err)
	}
	cfg.JaegerPollInterval = pollInterval

	// Parse Jaeger lookback window
	lookbackStr := getEnv("JAEGER_LOOKBACK_WINDOW", "5m")
	lookback, err := time.ParseDuration(lookbackStr)
	if err != nil {
		return nil, fmt.Errorf("invalid JAEGER_LOOKBACK_WINDOW: %w", err)
	}
	cfg.JaegerLookbackWindow = lookback

	// Parse Jaeger span retention
	retentionStr := getEnv("JAEGER_SPAN_RETENTION", "1h")
	retention, err := time.ParseDuration(retentionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid JAEGER_SPAN_RETENTION: %w", err)
	}
	cfg.JaegerSpanRetention = retention

	// Validate required fields
	if cfg.Neo4jPassword == "" {
		return nil, fmt.Errorf("NEO4J_PASSWORD is required")
	}

	return cfg, nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getBoolEnv gets a boolean environment variable with a default value
func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return defaultValue
		}
		return boolValue
	}
	return defaultValue
}

// getIntEnv gets an integer environment variable with a default value
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return defaultValue
		}
		return intValue
	}
	return defaultValue
}
