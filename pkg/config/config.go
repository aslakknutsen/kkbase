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

	// Agent configuration
	AgentEnabled      bool
	AgentPort         int
	AgentWorkers      int
	AgentMCPServerURL string

	// LLM configuration
	LLMProvider    string
	LLMAPIKey      string
	LLMModel       string
	LLMTemperature float32
	LLMMaxTokens   int

	// Event filtering
	EventFilterAllowlist []string
	EventFilterDenylist  []string

	// Event source configuration
	K8sEventsEnabled           bool
	AlertmanagerWebhookEnabled bool
	CustomWebhookEnabled       bool
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

		// Agent configuration
		AgentEnabled:      getBoolEnv("AGENT_ENABLED", true),
		AgentPort:         getIntEnv("AGENT_PORT", 8082),
		AgentWorkers:      getIntEnv("AGENT_WORKERS", 1),
		AgentMCPServerURL: getEnv("AGENT_MCP_SERVER_URL", "http://localhost:8081/mcp"),

		// LLM configuration
		LLMProvider:    getEnv("LLM_PROVIDER", "gemini"),
		LLMAPIKey:      getEnv("LLM_API_KEY", ""),
		LLMModel:       getEnv("LLM_MODEL", "gemini-2.0-flash-exp"),
		LLMTemperature: getFloat32Env("LLM_TEMPERATURE", 0.2),
		LLMMaxTokens:   getIntEnv("LLM_MAX_TOKENS", 2048),

		// Event filtering
		EventFilterAllowlist: getStringSliceEnv("EVENT_FILTER_ALLOWLIST", []string{}),
		EventFilterDenylist:  getStringSliceEnv("EVENT_FILTER_DENYLIST", []string{}),

		// Event source configuration (all enabled by default)
		K8sEventsEnabled:           getBoolEnv("K8S_EVENTS_ENABLED", false),
		AlertmanagerWebhookEnabled: getBoolEnv("ALERTMANAGER_WEBHOOK_ENABLED", true),
		CustomWebhookEnabled:       getBoolEnv("CUSTOM_WEBHOOK_ENABLED", true),
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

// getFloat32Env gets a float32 environment variable with a default value
func getFloat32Env(key string, defaultValue float32) float32 {
	if value := os.Getenv(key); value != "" {
		floatValue, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return defaultValue
		}
		return float32(floatValue)
	}
	return defaultValue
}

// getStringSliceEnv gets a comma-separated string environment variable as a slice
func getStringSliceEnv(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Split by comma and trim spaces
		parts := []string{}
		for _, part := range splitAndTrim(value, ",") {
			if part != "" {
				parts = append(parts, part)
			}
		}
		if len(parts) > 0 {
			return parts
		}
	}
	return defaultValue
}

// splitAndTrim splits a string by separator and trims spaces from each part
func splitAndTrim(s, sep string) []string {
	parts := []string{}
	for _, part := range splitString(s, sep) {
		trimmed := trimSpace(part)
		parts = append(parts, trimmed)
	}
	return parts
}

// splitString splits a string by separator
func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

// trimSpace removes leading and trailing whitespace
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
