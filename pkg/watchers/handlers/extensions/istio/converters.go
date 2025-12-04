package istio

import (
	"encoding/json"

	istiov1 "istio.io/client-go/pkg/apis/networking/v1"
	istiosecurityv1 "istio.io/client-go/pkg/apis/security/v1"

	"github.com/aslakknutsen/kkbase/pkg/models"
	"github.com/aslakknutsen/kkbase/pkg/watchers/handlers/common"
)

// serializeMap converts a map to JSON string for Neo4j storage
func serializeMap(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

// IstioGatewayToGraphNode converts an Istio Gateway to a graph node
func IstioGatewayToGraphNode(gateway *istiov1.Gateway) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      gateway.Name,
		"namespace": gateway.Namespace,
		"uid":       string(gateway.UID),
		"created":   gateway.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(gateway.Labels) > 0 {
		properties["labels"] = serializeMap(gateway.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(gateway.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	// Add selector
	if gateway.Spec.Selector != nil && len(gateway.Spec.Selector) > 0 {
		properties["selector"] = serializeMap(gateway.Spec.Selector)
	}

	// Add servers (serialized JSON)
	if len(gateway.Spec.Servers) > 0 {
		serversJSON, err := json.Marshal(gateway.Spec.Servers)
		if err == nil {
			properties["servers"] = string(serversJSON)
		}
	}

	return models.NewGraphNode(models.NodeType(NodeTypeIstioGateway), models.GetNodeID(NodeTypeIstioGateway, gateway.Namespace, gateway.Name), properties)
}

// VirtualServiceToGraphNode converts an Istio VirtualService to a graph node
func VirtualServiceToGraphNode(vs *istiov1.VirtualService) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      vs.Name,
		"namespace": vs.Namespace,
		"uid":       string(vs.UID),
		"created":   vs.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(vs.Labels) > 0 {
		properties["labels"] = serializeMap(vs.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(vs.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	// Add hosts
	if len(vs.Spec.Hosts) > 0 {
		hostsJSON, err := json.Marshal(vs.Spec.Hosts)
		if err == nil {
			properties["hosts"] = string(hostsJSON)
		}
	}

	// Add gateways
	if len(vs.Spec.Gateways) > 0 {
		gatewaysJSON, err := json.Marshal(vs.Spec.Gateways)
		if err == nil {
			properties["gateways"] = string(gatewaysJSON)
		}
	}

	// Add HTTP routes (serialized JSON)
	if len(vs.Spec.Http) > 0 {
		httpRoutesJSON, err := json.Marshal(vs.Spec.Http)
		if err == nil {
			properties["http_routes"] = string(httpRoutesJSON)
		}
	}

	// Add TCP routes (serialized JSON)
	if len(vs.Spec.Tcp) > 0 {
		tcpRoutesJSON, err := json.Marshal(vs.Spec.Tcp)
		if err == nil {
			properties["tcp_routes"] = string(tcpRoutesJSON)
		}
	}

	// Add TLS routes (serialized JSON)
	if len(vs.Spec.Tls) > 0 {
		tlsRoutesJSON, err := json.Marshal(vs.Spec.Tls)
		if err == nil {
			properties["tls_routes"] = string(tlsRoutesJSON)
		}
	}

	return models.NewGraphNode(models.NodeType(NodeTypeVirtualService), models.GetNodeID(NodeTypeVirtualService, vs.Namespace, vs.Name), properties)
}

// DestinationRuleToGraphNode converts an Istio DestinationRule to a graph node
func DestinationRuleToGraphNode(dr *istiov1.DestinationRule) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      dr.Name,
		"namespace": dr.Namespace,
		"uid":       string(dr.UID),
		"created":   dr.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(dr.Labels) > 0 {
		properties["labels"] = serializeMap(dr.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(dr.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	// Add host
	if dr.Spec.Host != "" {
		properties["host"] = dr.Spec.Host
	}

	// Add subsets (serialized JSON)
	if len(dr.Spec.Subsets) > 0 {
		subsetsJSON, err := json.Marshal(dr.Spec.Subsets)
		if err == nil {
			properties["subsets"] = string(subsetsJSON)
		}
	}

	// Add traffic policy (serialized JSON)
	if dr.Spec.TrafficPolicy != nil {
		trafficPolicyJSON, err := json.Marshal(dr.Spec.TrafficPolicy)
		if err == nil {
			properties["traffic_policy"] = string(trafficPolicyJSON)
		}
	}

	return models.NewGraphNode(models.NodeType(NodeTypeDestinationRule), models.GetNodeID(NodeTypeDestinationRule, dr.Namespace, dr.Name), properties)
}

// ServiceEntryToGraphNode converts an Istio ServiceEntry to a graph node
func ServiceEntryToGraphNode(se *istiov1.ServiceEntry) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      se.Name,
		"namespace": se.Namespace,
		"uid":       string(se.UID),
		"created":   se.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(se.Labels) > 0 {
		properties["labels"] = serializeMap(se.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(se.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	// Add hosts
	if len(se.Spec.Hosts) > 0 {
		hostsJSON, err := json.Marshal(se.Spec.Hosts)
		if err == nil {
			properties["hosts"] = string(hostsJSON)
		}
	}

	// Add location
	if se.Spec.Location.String() != "" {
		properties["location"] = se.Spec.Location.String()
	}

	// Add resolution
	if se.Spec.Resolution.String() != "" {
		properties["resolution"] = se.Spec.Resolution.String()
	}

	// Add endpoints (serialized JSON)
	if len(se.Spec.Endpoints) > 0 {
		endpointsJSON, err := json.Marshal(se.Spec.Endpoints)
		if err == nil {
			properties["endpoints"] = string(endpointsJSON)
		}
	}

	return models.NewGraphNode(models.NodeType(NodeTypeServiceEntry), models.GetNodeID(NodeTypeServiceEntry, se.Namespace, se.Name), properties)
}

// SidecarToGraphNode converts an Istio Sidecar to a graph node
func SidecarToGraphNode(sidecar *istiov1.Sidecar) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      sidecar.Name,
		"namespace": sidecar.Namespace,
		"uid":       string(sidecar.UID),
		"created":   sidecar.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(sidecar.Labels) > 0 {
		properties["labels"] = serializeMap(sidecar.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(sidecar.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	// Add workload selector
	if sidecar.Spec.WorkloadSelector != nil && len(sidecar.Spec.WorkloadSelector.Labels) > 0 {
		workloadSelectorJSON, err := json.Marshal(sidecar.Spec.WorkloadSelector)
		if err == nil {
			properties["workload_selector"] = string(workloadSelectorJSON)
		}
	}

	// Add egress hosts
	if len(sidecar.Spec.Egress) > 0 {
		var egressHosts []string
		for _, egress := range sidecar.Spec.Egress {
			egressHosts = append(egressHosts, egress.Hosts...)
		}
		if len(egressHosts) > 0 {
			egressHostsJSON, err := json.Marshal(egressHosts)
			if err == nil {
				properties["egress_hosts"] = string(egressHostsJSON)
			}
		}
	}

	// Add ingress listeners (serialized JSON)
	if len(sidecar.Spec.Ingress) > 0 {
		ingressJSON, err := json.Marshal(sidecar.Spec.Ingress)
		if err == nil {
			properties["ingress_listeners"] = string(ingressJSON)
		}
	}

	// Add egress listeners (serialized JSON)
	if len(sidecar.Spec.Egress) > 0 {
		egressJSON, err := json.Marshal(sidecar.Spec.Egress)
		if err == nil {
			properties["egress_listeners"] = string(egressJSON)
		}
	}

	return models.NewGraphNode(models.NodeType(NodeTypeSidecar), models.GetNodeID(NodeTypeSidecar, sidecar.Namespace, sidecar.Name), properties)
}

// AuthorizationPolicyToGraphNode converts an Istio AuthorizationPolicy to a graph node
func AuthorizationPolicyToGraphNode(policy *istiosecurityv1.AuthorizationPolicy) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      policy.Name,
		"namespace": policy.Namespace,
		"uid":       string(policy.UID),
		"created":   policy.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(policy.Labels) > 0 {
		properties["labels"] = serializeMap(policy.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(policy.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	// Add selector
	if policy.Spec.Selector != nil && len(policy.Spec.Selector.MatchLabels) > 0 {
		selectorJSON, err := json.Marshal(policy.Spec.Selector)
		if err == nil {
			properties["selector"] = string(selectorJSON)
		}
	}

	// Add action
	if policy.Spec.Action.String() != "" {
		properties["action"] = policy.Spec.Action.String()
	}

	// Add rules (serialized JSON)
	if len(policy.Spec.Rules) > 0 {
		rulesJSON, err := json.Marshal(policy.Spec.Rules)
		if err == nil {
			properties["rules"] = string(rulesJSON)
		}
	}

	return models.NewGraphNode(models.NodeType(NodeTypeAuthorizationPolicy), models.GetNodeID(NodeTypeAuthorizationPolicy, policy.Namespace, policy.Name), properties)
}

// PeerAuthenticationToGraphNode converts an Istio PeerAuthentication to a graph node
func PeerAuthenticationToGraphNode(pa *istiosecurityv1.PeerAuthentication) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      pa.Name,
		"namespace": pa.Namespace,
		"uid":       string(pa.UID),
		"created":   pa.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(pa.Labels) > 0 {
		properties["labels"] = serializeMap(pa.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(pa.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	// Add selector
	if pa.Spec.Selector != nil && len(pa.Spec.Selector.MatchLabels) > 0 {
		selectorJSON, err := json.Marshal(pa.Spec.Selector)
		if err == nil {
			properties["selector"] = string(selectorJSON)
		}
	}

	// Add mtls mode
	if pa.Spec.Mtls != nil && pa.Spec.Mtls.Mode.String() != "" {
		properties["mtls_mode"] = pa.Spec.Mtls.Mode.String()
	}

	// Add port level mtls (serialized JSON)
	if pa.Spec.PortLevelMtls != nil && len(pa.Spec.PortLevelMtls) > 0 {
		portLevelMtlsJSON, err := json.Marshal(pa.Spec.PortLevelMtls)
		if err == nil {
			properties["port_level_mtls"] = string(portLevelMtlsJSON)
		}
	}

	return models.NewGraphNode(models.NodeType(NodeTypePeerAuthentication), models.GetNodeID(NodeTypePeerAuthentication, pa.Namespace, pa.Name), properties)
}

// RequestAuthenticationToGraphNode converts an Istio RequestAuthentication to a graph node
func RequestAuthenticationToGraphNode(ra *istiosecurityv1.RequestAuthentication) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      ra.Name,
		"namespace": ra.Namespace,
		"uid":       string(ra.UID),
		"created":   ra.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(ra.Labels) > 0 {
		properties["labels"] = serializeMap(ra.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(ra.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	// Add selector
	if ra.Spec.Selector != nil && len(ra.Spec.Selector.MatchLabels) > 0 {
		selectorJSON, err := json.Marshal(ra.Spec.Selector)
		if err == nil {
			properties["selector"] = string(selectorJSON)
		}
	}

	// Add JWT rules (serialized JSON)
	if len(ra.Spec.JwtRules) > 0 {
		jwtRulesJSON, err := json.Marshal(ra.Spec.JwtRules)
		if err == nil {
			properties["jwt_rules"] = string(jwtRulesJSON)
		}
	}

	return models.NewGraphNode(models.NodeType(NodeTypeRequestAuthentication), models.GetNodeID(NodeTypeRequestAuthentication, ra.Namespace, ra.Name), properties)
}
