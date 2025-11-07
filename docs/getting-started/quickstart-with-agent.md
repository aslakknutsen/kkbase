# Quick Start: Full Stack with AI Agent Integration

Deploy the complete kkbase stack with AI agent integration in 20 minutes.

## What You'll Get

- Watcher service syncing cluster to Neo4j
- MCP Server exposing graph to AI agents
- Web dashboard for monitoring investigations
- Integration with Cursor or Claude Desktop

## Prerequisites

- Kubernetes cluster (v1.19+)
- kubectl configured
- Helm 3.x
- Cursor or Claude Desktop (for AI integration)

## Part 1: Deploy Core Stack (10 minutes)

### Step 1: Deploy Neo4j

```bash
helm repo add neo4j https://helm.neo4j.com/neo4j
helm repo update

helm install neo4j neo4j/neo4j \
  --set neo4j.name=neo4j \
  --set neo4j.password=changeme \
  --set neo4j.edition=community \
  --set neo4j.acceptLicenseAgreement=yes \
  --set volumes.data.mode=defaultStorageClass

kubectl wait --for=condition=ready pod -l app=neo4j --timeout=300s
```

### Step 2: Deploy Prometheus (Optional - Enables Metrics Investigation)

Prometheus enables the metrics-based RCA investigation tools:

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false

kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=prometheus \
  -n monitoring \
  --timeout=300s
```

### Step 3: Configure kkbase

Create secret:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: kkbase-secret
  namespace: default
type: Opaque
stringData:
  NEO4J_PASSWORD: "changeme"
EOF
```

Create configuration:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-config
  namespace: default
data:
  # Neo4j Connection
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "neo4j"
  
  # Watcher Configuration
  LOG_LEVEL: "info"
  NAMESPACE: ""
  RESYNC_PERIOD: "30s"
  
  # MCP Server
  MCP_ENABLED: "true"
  MCP_PORT: "8080"
  
  # Prometheus (if deployed)
  PROMETHEUS_URL: "http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090"
EOF
```

### Step 4: Deploy Watcher + MCP Server

Using integrated deployment:

```bash
# Deploy RBAC
kubectl apply -f https://raw.githubusercontent.com/kagenti/kkbase/main/deploy/rbac.yaml

# Deploy integrated watcher+MCP server
kubectl apply -f https://raw.githubusercontent.com/kagenti/kkbase/main/deploy/deployment-integrated.yaml

# Expose MCP server
kubectl apply -f https://raw.githubusercontent.com/kagenti/kkbase/main/deploy/service-integrated.yaml

# Wait for deployment
kubectl wait --for=condition=available deployment/kkbase --timeout=120s
```

**Verify**:
```bash
kubectl logs deployment/kkbase | grep -i "mcp\|watcher"
```

Expected:
```
INFO  successfully connected to Neo4j
INFO  watcher started successfully
INFO  MCP server listening  port=8080
INFO  embedded frontend enabled
```

### Step 5: Access Dashboard

Port forward to MCP server:

```bash
kubectl port-forward svc/kkbase 8080:8080
```

Open browser: http://localhost:8080/

You should see the kkbase dashboard with "No Active Investigations" initially.

## Part 2: Configure AI Tool (5 minutes)

### Option A: Cursor Configuration

Add to your Cursor MCP config (`~/.cursor/mcp.json` or via Cursor settings):

```json
{
  "mcpServers": {
    "kkbase": {
      "url": "http://localhost:8080/mcp",
      "transport": "sse"
    }
  }
}
```

Restart Cursor to load the configuration.

### Option B: Claude Desktop Configuration

Add to Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "kkbase": {
      "url": "http://localhost:8080/mcp",
      "transport": "streamable-http"
    }
  }
}
```

Restart Claude Desktop.

## Part 3: Test AI Investigation (5 minutes)

### In Cursor or Claude

Ask the AI:

```
Using the kkbase MCP tools, start an investigation session for:
"Testing kkbase setup - checking if MCP tools are working"

Then query the graph to show me all pods in the default namespace.
```

**Expected AI Actions**:
1. Calls `start_agent_session` with your symptom
2. Returns a session ID
3. Calls `query_with_session` to list pods
4. Returns pod information

### In Dashboard

Refresh http://localhost:8080/ or wait 5 seconds for auto-refresh.

**You should see**:
- New session in sidebar
- Your symptom description
- Session status: Active
- Query history showing the pod query
- Any auto-detected findings

🎉 **Success!** Your AI agent can now investigate cluster issues.

## Available MCP Tools

The AI agent has access to these tools:

### Core Tools
- `query` - Execute Cypher queries
- `structure` - Get graph schema

### Agent Session Tools
- `start_agent_session` - Begin investigation
- `update_hypothesis` - Record current theory
- `query_with_session` - Query with session tracking
- `record_finding` - Log discovered issues
- `record_recommendation` - Document action items
- `complete_agent_session` - Finish investigation

### Investigation Tools (if Prometheus enabled)
- `spawn_investigation` - Analyze metrics for a resource
- `get_investigation_status` - Check investigation progress

### Dashboard Tools
- `get_active_sessions` - List active investigations
- `get_session_details` - Get session state
- `get_blast_zone` - Get affected resources
- `get_session_timeline` - Get event timeline

See [MCP Tools Reference](../services/mcp-server/tools-reference.md) for complete details.

## Real Investigation Example

Try a real investigation in your AI tool:

```
Start an investigation session with symptom:
"Service 'my-service' is returning 503 errors"

1. Query the graph to find the service and its backend pods
2. Check the health status of those pods
3. If pods are unhealthy, check recent events
4. Provide recommendations for fixing the issue
```

Watch the dashboard in real-time as the AI:
- Creates the session
- Executes queries
- Discovers findings
- Updates hypothesis
- Records recommendations

## Production Deployment

For production use, consider:

### 1. TLS/HTTPS

Add Ingress with TLS:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kkbase-ingress
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - kkbase.example.com
    secretName: kkbase-tls
  rules:
  - host: kkbase.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: kkbase
            port:
              number: 8080
```

### 2. Authentication

Add OAuth2 proxy or API key authentication:

```bash
# Install oauth2-proxy
helm install oauth2-proxy oauth2-proxy/oauth2-proxy \
  --set config.clientID=<your-client-id> \
  --set config.clientSecret=<your-client-secret> \
  --set config.cookieSecret=<random-secret>
```

### 3. Resource Limits

Set appropriate limits:

```yaml
resources:
  limits:
    memory: "1Gi"
    cpu: "1000m"
  requests:
    memory: "512Mi"
    cpu: "500m"
```

### 4. High Availability

For HA Neo4j:

```bash
helm install neo4j neo4j/neo4j \
  --set neo4j.name=neo4j \
  --set neo4j.edition=enterprise \
  --set neo4j.cluster.enabled=true \
  --set neo4j.cluster.servers=3
```

## Troubleshooting

### MCP Server Not Accessible

Check service and port-forward:
```bash
kubectl get svc kkbase
kubectl port-forward svc/kkbase 8080:8080
curl http://localhost:8080/health
```

### AI Tool Can't Connect

Verify MCP endpoint:
```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list"
  }'
```

Should return list of available tools.

### Dashboard Shows No Sessions

Sessions only appear when AI creates them. Test manually:

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "start_agent_session",
      "arguments": {
        "symptom": "Manual test session"
      }
    }
  }'
```

Refresh dashboard - session should appear.

### Prometheus Not Found

If you didn't deploy Prometheus, remove from ConfigMap:

```yaml
# Remove this line:
# PROMETHEUS_URL: "..."
```

Investigation tools will be disabled but basic tools still work.

## Configuration Options

### Watch Specific Namespace

```yaml
data:
  NAMESPACE: "production"
```

### Standalone MCP Server

Deploy watcher and MCP server separately:

```bash
# Deploy watcher only
kubectl apply -f https://raw.githubusercontent.com/kagenti/kkbase/main/deploy/deployment.yaml

# Deploy MCP server separately
kubectl apply -f https://raw.githubusercontent.com/kagenti/kkbase/main/deploy/mcp-server-deployment.yaml
```

See [MCP Deployment Options](../services/mcp-server/deployment.md)

## Next Steps

### Learn Investigation Patterns

- [Investigation Workflow](../guides/investigations/workflow.md)
- [Agent Execution Flow](../guides/investigations/agent-sessions.md)
- [Best Practices](../guides/investigations/best-practices.md)

### Master the Tools

- [MCP Tools Reference](../services/mcp-server/tools-reference.md)
- [Query Guide](../guides/querying/)
- [RCA Query Patterns](../guides/querying/rca-patterns.md)

### Deploy Agent Service (Autonomous)

For fully autonomous diagnostics:

- [Agent Service](../services/agent/)
- [Agent Configuration](../services/agent/configuration.md)
- [Webhook Setup](../services/agent/integration.md)

### Operations

- [Monitoring](../guides/operations/monitoring.md)
- [Troubleshooting](../guides/operations/troubleshooting.md)
- [Scaling](../guides/operations/scaling.md)

## Clean Up

```bash
# Remove kkbase
kubectl delete deployment kkbase
kubectl delete service kkbase
kubectl delete configmap kkbase-config
kubectl delete secret kkbase-secret
kubectl delete -f https://raw.githubusercontent.com/kagenti/kkbase/main/deploy/rbac.yaml

# Remove Neo4j
helm uninstall neo4j

# Remove Prometheus (if installed)
helm uninstall prometheus -n monitoring
kubectl delete namespace monitoring
```

## Summary

You now have:
- ✅ Complete kkbase stack deployed
- ✅ AI agent integration (Cursor/Claude)
- ✅ Real-time investigation dashboard
- ✅ Metrics investigation (if Prometheus enabled)
- ✅ Ready for autonomous diagnostics

**Start investigating!** Try the investigation examples or explore the [Investigation Workflow Guide](../guides/investigations/workflow.md).

