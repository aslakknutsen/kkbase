package observability

import (
	"context"
	"fmt"

	"github.com/kagenti/kkbase/pkg/graph"
	"go.uber.org/zap"
)

// ServiceInfo represents a discovered service
type ServiceInfo struct {
	Name      string
	Namespace string
}

// DiscoverMonitoredServices queries the graph for all Service nodes to monitor
func DiscoverMonitoredServices(ctx context.Context, graphStore graph.GraphStore, logger *zap.Logger) ([]ServiceInfo, error) {
	// Query Neo4j for all Service nodes
	query := `
		MATCH (s:Service)
		RETURN s.name as name, s.namespace as namespace
	`

	result, err := graphStore.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query services: %w", err)
	}

	services := make([]ServiceInfo, 0, len(result))
	for _, record := range result {
		name, nameOk := record["name"].(string)
		namespace, nsOk := record["namespace"].(string)

		if nameOk && nsOk && name != "" && namespace != "" {
			services = append(services, ServiceInfo{
				Name:      name,
				Namespace: namespace,
			})
		}
	}

	logger.Debug("discovered services for monitoring",
		zap.Int("count", len(services)))

	return services, nil
}

// ExtractServiceNames extracts just the service names from ServiceInfo list
func ExtractServiceNames(services []ServiceInfo) []string {
	names := make([]string, 0, len(services))
	for _, svc := range services {
		names = append(names, svc.Name)
	}
	return names
}
