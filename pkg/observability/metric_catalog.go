package observability

// MetricDefinition describes a metric and how to query it
type MetricDefinition struct {
	Name              string
	Type              MetricType
	Description       string
	Unit              string
	PromQLTemplate    string   // Template with placeholders like {{.PodName}}, {{.Namespace}}
	ApplicableToTypes []string // ["Pod", "Container", "Node"]
	Category          string   // "resource", "performance", "error"
}

// MetricCatalog provides extensible metric definitions
type MetricCatalog struct {
	metrics map[string]MetricDefinition
}

// NewMetricCatalog creates a catalog with core metrics
func NewMetricCatalog() *MetricCatalog {
	catalog := &MetricCatalog{
		metrics: make(map[string]MetricDefinition),
	}

	// Register core metrics
	catalog.registerCoreMetrics()

	return catalog
}

func (mc *MetricCatalog) registerCoreMetrics() {
	// Memory metrics
	mc.Register(MetricDefinition{
		Name:              "container_memory_usage_bytes",
		Type:              MetricTypeGauge,
		Description:       "Current memory usage in bytes",
		Unit:              "bytes",
		PromQLTemplate:    `container_memory_usage_bytes{pod="{{.PodName}}", namespace="{{.Namespace}}", container="{{.Container}}"}`,
		ApplicableToTypes: []string{"Pod", "Container"},
		Category:          "resource",
	})

	mc.Register(MetricDefinition{
		Name:              "container_memory_working_set_bytes",
		Type:              MetricTypeGauge,
		Description:       "Memory working set (used for OOM decisions)",
		Unit:              "bytes",
		PromQLTemplate:    `container_memory_working_set_bytes{pod="{{.PodName}}", namespace="{{.Namespace}}", container="{{.Container}}"}`,
		ApplicableToTypes: []string{"Pod", "Container"},
		Category:          "resource",
	})

	// CPU metrics
	mc.Register(MetricDefinition{
		Name:              "container_cpu_usage_seconds_total",
		Type:              MetricTypeCounter,
		Description:       "Cumulative CPU usage in seconds",
		Unit:              "seconds",
		PromQLTemplate:    `rate(container_cpu_usage_seconds_total{pod="{{.PodName}}", namespace="{{.Namespace}}", container="{{.Container}}"}[1m])`,
		ApplicableToTypes: []string{"Pod", "Container"},
		Category:          "resource",
	})

	// Network metrics
	mc.Register(MetricDefinition{
		Name:              "container_network_receive_errors_total",
		Type:              MetricTypeCounter,
		Description:       "Network receive errors",
		Unit:              "errors",
		PromQLTemplate:    `rate(container_network_receive_errors_total{pod="{{.PodName}}", namespace="{{.Namespace}}"}[1m])`,
		ApplicableToTypes: []string{"Pod"},
		Category:          "error",
	})

	mc.Register(MetricDefinition{
		Name:              "container_network_transmit_errors_total",
		Type:              MetricTypeCounter,
		Description:       "Network transmit errors",
		Unit:              "errors",
		PromQLTemplate:    `rate(container_network_transmit_errors_total{pod="{{.PodName}}", namespace="{{.Namespace}}"}[1m])`,
		ApplicableToTypes: []string{"Pod"},
		Category:          "error",
	})

	// Application-level metrics (if available)
	mc.Register(MetricDefinition{
		Name:              "http_request_duration_seconds",
		Type:              MetricTypeHistogram,
		Description:       "HTTP request latency",
		Unit:              "seconds",
		PromQLTemplate:    `histogram_quantile(0.95, rate(http_request_duration_seconds_bucket{namespace="{{.Namespace}}", service="{{.Service}}"}[1m]))`,
		ApplicableToTypes: []string{"Service", "Pod"},
		Category:          "performance",
	})

	mc.Register(MetricDefinition{
		Name:              "http_requests_total",
		Type:              MetricTypeCounter,
		Description:       "Total HTTP requests",
		Unit:              "requests",
		PromQLTemplate:    `rate(http_requests_total{namespace="{{.Namespace}}", service="{{.Service}}"}[1m])`,
		ApplicableToTypes: []string{"Service", "Pod"},
		Category:          "performance",
	})

	// Node-level metrics
	mc.Register(MetricDefinition{
		Name:              "node_cpu_seconds_total",
		Type:              MetricTypeCounter,
		Description:       "Node CPU usage",
		Unit:              "seconds",
		PromQLTemplate:    `rate(node_cpu_seconds_total{node="{{.NodeName}}"}[1m])`,
		ApplicableToTypes: []string{"Node"},
		Category:          "resource",
	})

	mc.Register(MetricDefinition{
		Name:              "node_memory_MemAvailable_bytes",
		Type:              MetricTypeGauge,
		Description:       "Available memory on node",
		Unit:              "bytes",
		PromQLTemplate:    `node_memory_MemAvailable_bytes{node="{{.NodeName}}"}`,
		ApplicableToTypes: []string{"Node"},
		Category:          "resource",
	})

	mc.Register(MetricDefinition{
		Name:              "node_load1",
		Type:              MetricTypeGauge,
		Description:       "Node 1-minute load average",
		Unit:              "load",
		PromQLTemplate:    `node_load1{node="{{.NodeName}}"}`,
		ApplicableToTypes: []string{"Node"},
		Category:          "resource",
	})
}

// Register adds a metric definition (allows dynamic extension)
func (mc *MetricCatalog) Register(def MetricDefinition) {
	mc.metrics[def.Name] = def
}

// GetMetricsForResourceType returns relevant metrics for a resource type
func (mc *MetricCatalog) GetMetricsForResourceType(resourceType string) []MetricDefinition {
	var result []MetricDefinition
	for _, def := range mc.metrics {
		for _, applicableType := range def.ApplicableToTypes {
			if applicableType == resourceType {
				result = append(result, def)
				break
			}
		}
	}
	return result
}

// GetMetricsByCategory returns metrics by category
func (mc *MetricCatalog) GetMetricsByCategory(category string) []MetricDefinition {
	var result []MetricDefinition
	for _, def := range mc.metrics {
		if def.Category == category {
			result = append(result, def)
		}
	}
	return result
}

// Get returns a metric definition by name
func (mc *MetricCatalog) Get(name string) (MetricDefinition, bool) {
	def, exists := mc.metrics[name]
	return def, exists
}

// GetAllMetrics returns all registered metrics
func (mc *MetricCatalog) GetAllMetrics() []MetricDefinition {
	result := make([]MetricDefinition, 0, len(mc.metrics))
	for _, def := range mc.metrics {
		result = append(result, def)
	}
	return result
}

// GetMetricNames returns all registered metric names
func (mc *MetricCatalog) GetMetricNames() []string {
	names := make([]string, 0, len(mc.metrics))
	for name := range mc.metrics {
		names = append(names, name)
	}
	return names
}
