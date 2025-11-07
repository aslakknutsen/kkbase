package gateway

import (
	"encoding/json"
	"fmt"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kagenti/kkbase/pkg/models"
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

// GatewayClassToGraphNode converts a Gateway API GatewayClass to a graph node
func GatewayClassToGraphNode(gatewayClass *gatewayv1.GatewayClass) *models.GraphNode {
	properties := map[string]interface{}{
		"name":            gatewayClass.Name,
		"controller_name": string(gatewayClass.Spec.ControllerName),
	}

	if gatewayClass.Spec.Description != nil {
		properties["description"] = *gatewayClass.Spec.Description
	}

	// Add status conditions
	if len(gatewayClass.Status.Conditions) > 0 {
		for _, condition := range gatewayClass.Status.Conditions {
			if condition.Type == string(gatewayv1.GatewayClassConditionStatusAccepted) {
				properties["accepted"] = string(condition.Status)
				if condition.Message != "" {
					properties["status_message"] = condition.Message
				}
				break
			}
		}
	}

	// Add labels
	if len(gatewayClass.Labels) > 0 {
		properties["labels"] = serializeMap(gatewayClass.Labels)
	}

	return models.NewGraphNode(models.NodeType(NodeTypeGatewayClass), models.GetNodeID(NodeTypeGatewayClass, "", gatewayClass.Name), properties)
}

// GatewayToGraphNode converts a Gateway API Gateway to a graph node
func GatewayToGraphNode(gateway *gatewayv1.Gateway) *models.GraphNode {
	properties := map[string]interface{}{
		"name":               gateway.Name,
		"namespace":          gateway.Namespace,
		"gateway_class_name": string(gateway.Spec.GatewayClassName),
	}

	// Add listeners information
	if len(gateway.Spec.Listeners) > 0 {
		listeners := make([]map[string]interface{}, len(gateway.Spec.Listeners))
		for i, listener := range gateway.Spec.Listeners {
			listenerInfo := map[string]interface{}{
				"name":     string(listener.Name),
				"port":     int(listener.Port),
				"protocol": string(listener.Protocol),
			}
			if listener.Hostname != nil {
				listenerInfo["hostname"] = string(*listener.Hostname)
			}
			if listener.TLS != nil {
				listenerInfo["tls_mode"] = string(*listener.TLS.Mode)
				if len(listener.TLS.CertificateRefs) > 0 {
					certRefs := make([]string, len(listener.TLS.CertificateRefs))
					for j, certRef := range listener.TLS.CertificateRefs {
						namespace := gateway.Namespace
						if certRef.Namespace != nil {
							namespace = string(*certRef.Namespace)
						}
						certRefs[j] = fmt.Sprintf("%s/%s", namespace, string(certRef.Name))
					}
					listenerInfo["certificate_refs"] = certRefs
				}
			}
			listeners[i] = listenerInfo
		}
		if b, err := json.Marshal(listeners); err == nil {
			properties["listeners"] = string(b)
		}
	}

	// Add addresses
	if len(gateway.Status.Addresses) > 0 {
		addresses := make([]string, len(gateway.Status.Addresses))
		for i, addr := range gateway.Status.Addresses {
			addresses[i] = addr.Value
		}
		properties["addresses"] = addresses
	}

	// Add status conditions
	if len(gateway.Status.Conditions) > 0 {
		for _, condition := range gateway.Status.Conditions {
			if condition.Type == string(gatewayv1.GatewayConditionAccepted) {
				properties["accepted"] = string(condition.Status)
			} else if condition.Type == string(gatewayv1.GatewayConditionProgrammed) {
				properties["programmed"] = string(condition.Status)
			}
		}
	}

	// Add labels
	if len(gateway.Labels) > 0 {
		properties["labels"] = serializeMap(gateway.Labels)
	}

	return models.NewGraphNode(models.NodeType(NodeTypeGateway), models.GetNodeID(NodeTypeGateway, gateway.Namespace, gateway.Name), properties)
}

// HTTPRouteToGraphNode converts a Gateway API HTTPRoute to a graph node
func HTTPRouteToGraphNode(httpRoute *gatewayv1.HTTPRoute) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      httpRoute.Name,
		"namespace": httpRoute.Namespace,
	}

	// Add hostnames
	if len(httpRoute.Spec.Hostnames) > 0 {
		hostnames := make([]string, len(httpRoute.Spec.Hostnames))
		for i, hostname := range httpRoute.Spec.Hostnames {
			hostnames[i] = string(hostname)
		}
		properties["hostnames"] = hostnames
	}

	// Add parent refs (gateways this route attaches to)
	if len(httpRoute.Spec.ParentRefs) > 0 {
		parentRefs := make([]map[string]interface{}, len(httpRoute.Spec.ParentRefs))
		for i, parentRef := range httpRoute.Spec.ParentRefs {
			ref := map[string]interface{}{
				"name": string(parentRef.Name),
			}
			if parentRef.Namespace != nil {
				ref["namespace"] = string(*parentRef.Namespace)
			} else {
				ref["namespace"] = httpRoute.Namespace
			}
			if parentRef.SectionName != nil {
				ref["section_name"] = string(*parentRef.SectionName)
			}
			parentRefs[i] = ref
		}
		if b, err := json.Marshal(parentRefs); err == nil {
			properties["parent_refs"] = string(b)
		}
	}

	// Add rules summary
	if len(httpRoute.Spec.Rules) > 0 {
		properties["rule_count"] = len(httpRoute.Spec.Rules)

		// Serialize rules
		rules := make([]map[string]interface{}, len(httpRoute.Spec.Rules))
		for i, rule := range httpRoute.Spec.Rules {
			ruleInfo := map[string]interface{}{}

			// Add matches
			if len(rule.Matches) > 0 {
				matches := make([]map[string]interface{}, len(rule.Matches))
				for j, match := range rule.Matches {
					matchInfo := map[string]interface{}{}
					if match.Path != nil {
						matchInfo["path_type"] = string(*match.Path.Type)
						if match.Path.Value != nil {
							matchInfo["path_value"] = *match.Path.Value
						}
					}
					matches[j] = matchInfo
				}
				ruleInfo["matches"] = matches
			}

			// Add backend refs
			if len(rule.BackendRefs) > 0 {
				backends := make([]string, len(rule.BackendRefs))
				for j, backend := range rule.BackendRefs {
					namespace := httpRoute.Namespace
					if backend.Namespace != nil {
						namespace = string(*backend.Namespace)
					}
					backends[j] = fmt.Sprintf("%s/%s", namespace, string(backend.Name))
				}
				ruleInfo["backends"] = backends
			}

			rules[i] = ruleInfo
		}
		if b, err := json.Marshal(rules); err == nil {
			properties["rules"] = string(b)
		}
	}

	// Add status
	if len(httpRoute.Status.Parents) > 0 {
		for _, parent := range httpRoute.Status.Parents {
			for _, condition := range parent.Conditions {
				if condition.Type == string(gatewayv1.RouteConditionAccepted) {
					properties["accepted"] = string(condition.Status)
					break
				}
			}
		}
	}

	// Add labels
	if len(httpRoute.Labels) > 0 {
		properties["labels"] = serializeMap(httpRoute.Labels)
	}

	return models.NewGraphNode(models.NodeType(NodeTypeHTTPRoute), models.GetNodeID(NodeTypeHTTPRoute, httpRoute.Namespace, httpRoute.Name), properties)
}

// GRPCRouteToGraphNode converts a Gateway API GRPCRoute to a graph node
func GRPCRouteToGraphNode(grpcRoute *gatewayv1.GRPCRoute) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      grpcRoute.Name,
		"namespace": grpcRoute.Namespace,
	}

	// Add hostnames
	if len(grpcRoute.Spec.Hostnames) > 0 {
		hostnames := make([]string, len(grpcRoute.Spec.Hostnames))
		for i, hostname := range grpcRoute.Spec.Hostnames {
			hostnames[i] = string(hostname)
		}
		properties["hostnames"] = hostnames
	}

	// Add parent refs (gateways this route attaches to)
	if len(grpcRoute.Spec.ParentRefs) > 0 {
		parentRefs := make([]map[string]interface{}, len(grpcRoute.Spec.ParentRefs))
		for i, parentRef := range grpcRoute.Spec.ParentRefs {
			ref := map[string]interface{}{
				"name": string(parentRef.Name),
			}
			if parentRef.Namespace != nil {
				ref["namespace"] = string(*parentRef.Namespace)
			} else {
				ref["namespace"] = grpcRoute.Namespace
			}
			if parentRef.SectionName != nil {
				ref["section_name"] = string(*parentRef.SectionName)
			}
			parentRefs[i] = ref
		}
		if b, err := json.Marshal(parentRefs); err == nil {
			properties["parent_refs"] = string(b)
		}
	}

	// Add rules summary
	if len(grpcRoute.Spec.Rules) > 0 {
		properties["rule_count"] = len(grpcRoute.Spec.Rules)

		// Serialize rules
		rules := make([]map[string]interface{}, len(grpcRoute.Spec.Rules))
		for i, rule := range grpcRoute.Spec.Rules {
			ruleInfo := map[string]interface{}{}

			// Add matches
			if len(rule.Matches) > 0 {
				matches := make([]map[string]interface{}, len(rule.Matches))
				for j, match := range rule.Matches {
					matchInfo := map[string]interface{}{}
					if match.Method != nil {
						if match.Method.Service != nil {
							matchInfo["service"] = *match.Method.Service
						}
						if match.Method.Method != nil {
							matchInfo["method"] = *match.Method.Method
						}
					}
					matches[j] = matchInfo
				}
				ruleInfo["matches"] = matches
			}

			// Add backend refs
			if len(rule.BackendRefs) > 0 {
				backends := make([]string, len(rule.BackendRefs))
				for j, backend := range rule.BackendRefs {
					namespace := grpcRoute.Namespace
					if backend.Namespace != nil {
						namespace = string(*backend.Namespace)
					}
					backends[j] = fmt.Sprintf("%s/%s", namespace, string(backend.Name))
				}
				ruleInfo["backends"] = backends
			}

			rules[i] = ruleInfo
		}
		if b, err := json.Marshal(rules); err == nil {
			properties["rules"] = string(b)
		}
	}

	// Add status
	if len(grpcRoute.Status.Parents) > 0 {
		for _, parent := range grpcRoute.Status.Parents {
			for _, condition := range parent.Conditions {
				if condition.Type == string(gatewayv1.RouteConditionAccepted) {
					properties["accepted"] = string(condition.Status)
					break
				}
			}
		}
	}

	// Add labels
	if len(grpcRoute.Labels) > 0 {
		properties["labels"] = serializeMap(grpcRoute.Labels)
	}

	return models.NewGraphNode(models.NodeType(NodeTypeGRPCRoute), models.GetNodeID(NodeTypeGRPCRoute, grpcRoute.Namespace, grpcRoute.Name), properties)
}

// TCPRouteToGraphNode converts a Gateway API TCPRoute to a graph node
func TCPRouteToGraphNode(tcpRoute *gatewayv1alpha2.TCPRoute) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      tcpRoute.Name,
		"namespace": tcpRoute.Namespace,
	}

	// Add parent refs
	if len(tcpRoute.Spec.ParentRefs) > 0 {
		parentRefs := make([]map[string]interface{}, len(tcpRoute.Spec.ParentRefs))
		for i, parentRef := range tcpRoute.Spec.ParentRefs {
			ref := map[string]interface{}{
				"name": string(parentRef.Name),
			}
			if parentRef.Namespace != nil {
				ref["namespace"] = string(*parentRef.Namespace)
			} else {
				ref["namespace"] = tcpRoute.Namespace
			}
			parentRefs[i] = ref
		}
		if b, err := json.Marshal(parentRefs); err == nil {
			properties["parent_refs"] = string(b)
		}
	}

	// Add rules summary
	if len(tcpRoute.Spec.Rules) > 0 {
		properties["rule_count"] = len(tcpRoute.Spec.Rules)
	}

	// Add labels
	if len(tcpRoute.Labels) > 0 {
		properties["labels"] = serializeMap(tcpRoute.Labels)
	}

	return models.NewGraphNode(models.NodeType(NodeTypeTCPRoute), models.GetNodeID(NodeTypeTCPRoute, tcpRoute.Namespace, tcpRoute.Name), properties)
}

// UDPRouteToGraphNode converts a Gateway API UDPRoute to a graph node
func UDPRouteToGraphNode(udpRoute *gatewayv1alpha2.UDPRoute) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      udpRoute.Name,
		"namespace": udpRoute.Namespace,
	}

	// Add parent refs
	if len(udpRoute.Spec.ParentRefs) > 0 {
		parentRefs := make([]map[string]interface{}, len(udpRoute.Spec.ParentRefs))
		for i, parentRef := range udpRoute.Spec.ParentRefs {
			ref := map[string]interface{}{
				"name": string(parentRef.Name),
			}
			if parentRef.Namespace != nil {
				ref["namespace"] = string(*parentRef.Namespace)
			} else {
				ref["namespace"] = udpRoute.Namespace
			}
			parentRefs[i] = ref
		}
		if b, err := json.Marshal(parentRefs); err == nil {
			properties["parent_refs"] = string(b)
		}
	}

	// Add rules summary
	if len(udpRoute.Spec.Rules) > 0 {
		properties["rule_count"] = len(udpRoute.Spec.Rules)
	}

	// Add labels
	if len(udpRoute.Labels) > 0 {
		properties["labels"] = serializeMap(udpRoute.Labels)
	}

	return models.NewGraphNode(models.NodeType(NodeTypeUDPRoute), models.GetNodeID(NodeTypeUDPRoute, udpRoute.Namespace, udpRoute.Name), properties)
}

// TLSRouteToGraphNode converts a Gateway API TLSRoute to a graph node
func TLSRouteToGraphNode(tlsRoute *gatewayv1alpha2.TLSRoute) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      tlsRoute.Name,
		"namespace": tlsRoute.Namespace,
	}

	// Add hostnames
	if len(tlsRoute.Spec.Hostnames) > 0 {
		hostnames := make([]string, len(tlsRoute.Spec.Hostnames))
		for i, hostname := range tlsRoute.Spec.Hostnames {
			hostnames[i] = string(hostname)
		}
		properties["hostnames"] = hostnames
	}

	// Add parent refs
	if len(tlsRoute.Spec.ParentRefs) > 0 {
		parentRefs := make([]map[string]interface{}, len(tlsRoute.Spec.ParentRefs))
		for i, parentRef := range tlsRoute.Spec.ParentRefs {
			ref := map[string]interface{}{
				"name": string(parentRef.Name),
			}
			if parentRef.Namespace != nil {
				ref["namespace"] = string(*parentRef.Namespace)
			} else {
				ref["namespace"] = tlsRoute.Namespace
			}
			parentRefs[i] = ref
		}
		if b, err := json.Marshal(parentRefs); err == nil {
			properties["parent_refs"] = string(b)
		}
	}

	// Add rules summary
	if len(tlsRoute.Spec.Rules) > 0 {
		properties["rule_count"] = len(tlsRoute.Spec.Rules)
	}

	// Add labels
	if len(tlsRoute.Labels) > 0 {
		properties["labels"] = serializeMap(tlsRoute.Labels)
	}

	return models.NewGraphNode(models.NodeType(NodeTypeTLSRoute), models.GetNodeID(NodeTypeTLSRoute, tlsRoute.Namespace, tlsRoute.Name), properties)
}

// ReferenceGrantToGraphNode converts a Gateway API ReferenceGrant to a graph node
func ReferenceGrantToGraphNode(referenceGrant *gatewayv1beta1.ReferenceGrant) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      referenceGrant.Name,
		"namespace": referenceGrant.Namespace,
	}

	// Add from references (who can reference)
	if len(referenceGrant.Spec.From) > 0 {
		froms := make([]map[string]interface{}, len(referenceGrant.Spec.From))
		for i, from := range referenceGrant.Spec.From {
			fromInfo := map[string]interface{}{
				"group":     string(from.Group),
				"kind":      string(from.Kind),
				"namespace": string(from.Namespace),
			}
			froms[i] = fromInfo
		}
		if b, err := json.Marshal(froms); err == nil {
			properties["from"] = string(b)
		}
	}

	// Add to references (what can be referenced)
	if len(referenceGrant.Spec.To) > 0 {
		tos := make([]map[string]interface{}, len(referenceGrant.Spec.To))
		for i, to := range referenceGrant.Spec.To {
			toInfo := map[string]interface{}{
				"group": string(to.Group),
				"kind":  string(to.Kind),
			}
			if to.Name != nil {
				toInfo["name"] = string(*to.Name)
			}
			tos[i] = toInfo
		}
		if b, err := json.Marshal(tos); err == nil {
			properties["to"] = string(b)
		}
	}

	// Add labels
	if len(referenceGrant.Labels) > 0 {
		properties["labels"] = serializeMap(referenceGrant.Labels)
	}

	return models.NewGraphNode(models.NodeType(NodeTypeReferenceGrant), models.GetNodeID(NodeTypeReferenceGrant, referenceGrant.Namespace, referenceGrant.Name), properties)
}

// BackendTLSPolicyToGraphNode converts a Gateway API BackendTLSPolicy to a graph node
func BackendTLSPolicyToGraphNode(backendTLSPolicy *gatewayv1.BackendTLSPolicy) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      backendTLSPolicy.Name,
		"namespace": backendTLSPolicy.Namespace,
	}

	// Add target refs (what backends this policy applies to)
	if len(backendTLSPolicy.Spec.TargetRefs) > 0 {
		targetRefs := make([]map[string]interface{}, len(backendTLSPolicy.Spec.TargetRefs))
		for i, targetRef := range backendTLSPolicy.Spec.TargetRefs {
			ref := map[string]interface{}{
				"group":     string(targetRef.Group),
				"kind":      string(targetRef.Kind),
				"name":      string(targetRef.Name),
				"namespace": backendTLSPolicy.Namespace, // LocalPolicyTargetReference is always in the same namespace
			}
			if targetRef.SectionName != nil {
				ref["section_name"] = string(*targetRef.SectionName)
			}
			targetRefs[i] = ref
		}
		if b, err := json.Marshal(targetRefs); err == nil {
			properties["target_refs"] = string(b)
		}
	}

	// Add validation info
	if backendTLSPolicy.Spec.Validation.Hostname != "" {
		properties["hostname"] = string(backendTLSPolicy.Spec.Validation.Hostname)
	}

	// Add CA certificate refs
	if len(backendTLSPolicy.Spec.Validation.CACertificateRefs) > 0 {
		caCertRefs := make([]string, len(backendTLSPolicy.Spec.Validation.CACertificateRefs))
		for i, certRef := range backendTLSPolicy.Spec.Validation.CACertificateRefs {
			// LocalObjectReference is always in the same namespace as the policy
			caCertRefs[i] = fmt.Sprintf("%s/%s/%s", certRef.Group, backendTLSPolicy.Namespace, certRef.Name)
		}
		properties["ca_certificate_refs"] = caCertRefs
	}

	// Add well-known CA certificates if present
	if backendTLSPolicy.Spec.Validation.WellKnownCACertificates != nil {
		properties["well_known_ca_certificates"] = string(*backendTLSPolicy.Spec.Validation.WellKnownCACertificates)
	}

	// Add labels
	if len(backendTLSPolicy.Labels) > 0 {
		properties["labels"] = serializeMap(backendTLSPolicy.Labels)
	}

	return models.NewGraphNode(models.NodeType(NodeTypeBackendTLSPolicy), models.GetNodeID(NodeTypeBackendTLSPolicy, backendTLSPolicy.Namespace, backendTLSPolicy.Name), properties)
}
