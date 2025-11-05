# Agent System Deployment Guide

This guide covers deploying the Gemini-powered AI Agent system for autonomous Kubernetes incident investigation.

## Overview

The agent system consists of three components running in a single integrated pod:

1. **Watcher** - Monitors Kubernetes resources and events
2. **MCP Server** - Provides tools and data access via Model Context Protocol
3. **Agent** - Uses Gemini LLM to autonomously investigate incidents

## Prerequisites

1. Kubernetes cluster with access
2. Neo4j database deployed
3. Google Gemini API key ([Get one here](https://aistudio.google.com/app/apikey))
4. Docker registry access (for custom builds)

## Quick Start

### 1. Configure Secrets

Update `deploy/secret.yaml` with your credentials:

```yaml
stringData:
  NEO4J_PASSWORD: "your-neo4j-password"
  LLM_API_KEY: "your-gemini-api-key"
```

### 2. Configure Agent Behavior (Optional)

Edit `deploy/configmap.yaml` to customize:

```yaml
# Agent settings
AGENT_ENABLED: "true"
AGENT_WORKERS: "1"          # Number of concurrent investigations

# LLM settings
LLM_MODEL: "gemini-2.0-flash-exp"
LLM_TEMPERATURE: "0.2"      # Lower = more deterministic
LLM_MAX_TOKENS: "2048"

# Event filtering
EVENT_FILTER_ALLOWLIST: ""  # Empty = process all
EVENT_FILTER_DENYLIST: ""   # Comma-separated reasons to ignore
```

### 3. Deploy

```bash
# Deploy integrated pod (watcher + mcp-server + agent)
make deploy-integrated

# Or manually:
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/secret.yaml
kubectl apply -f deploy/deployment-integrated.yaml
kubectl apply -f deploy/service-integrated.yaml
```

### 4. Verify

```bash
# Check pod status
kubectl get pods -l app=kkbase-integrated

# Check agent logs
kubectl logs -f deployment/kkbase-integrated -c agent

# Check all containers
kubectl logs -f deployment/kkbase-integrated --all-containers
```

## Event Sources

The agent receives events from three sources:

### 1. Kubernetes Events

Automatically monitored from Neo4j graph. The agent polls for new events every 10 seconds.

### 2. Prometheus Alertmanager

Configure Alertmanager to send webhooks:

```yaml
# alertmanager.yml
receivers:
  - name: 'kkbase-agent'
    webhook_configs:
      - url: 'http://kkbase-integrated:8082/webhook/alertmanager'
        send_resolved: false
```

### 3. Custom Webhooks

Send custom events via HTTP POST:

```bash
curl -X POST http://kkbase-integrated:8082/webhook/custom \
  -H "Content-Type: application/json" \
  -d '{
    "type": "custom",
    "severity": "critical",
    "resource": {
      "type": "Pod",
      "namespace": "production",
      "name": "my-app-12345"
    },
    "reason": "CustomAlert",
    "message": "Application health check failing"
  }'
```

## Event Filtering

Configure which events the agent processes:

### By Severity (Default)

Only `warning` and `critical` events are processed by default.

### By Allowlist

Process only specific event reasons:

```yaml
EVENT_FILTER_ALLOWLIST: "OOMKilled,CrashLoopBackOff,Failed,ImagePullBackOff"
```

### By Denylist

Process all except specific reasons:

```yaml
EVENT_FILTER_DENYLIST: "Pulling,Pulled,Created,Started"
```

## Monitoring

### Health Checks

- **Liveness**: `http://localhost:8082/healthz`
- **Readiness**: `http://localhost:8082/ready`

### Metrics

Watch agent activity:

```bash
# View real-time logs
kubectl logs -f deployment/kkbase-integrated -c agent

# View investigation sessions in UI
kubectl port-forward svc/kkbase-integrated 8081:8081
# Open http://localhost:8081
```

## Investigation Results

Results are stored in Neo4j and visible via:

1. **MCP Server UI** - Real-time dashboard at `http://kkbase-integrated:8081`
2. **Neo4j** - Query `AgentSession` nodes with recommendations
3. **Logs** - Structured JSON logs with investigation details

### Query Sessions

```cypher
// List recent agent sessions
MATCH (s:AgentSession)
WHERE s.status = 'completed'
RETURN s.id, s.initial_symptom, s.summary, s.created_at
ORDER BY s.created_at DESC
LIMIT 10

// Get recommendations for a session
MATCH (s:AgentSession {id: 'session-id'})-[:HAS_RECOMMENDATION]->(r:Recommendation)
RETURN r.action, r.description, r.risk_level, r.auto_approved
```

## Troubleshooting

### Agent Not Starting

Check the LLM_API_KEY is set:

```bash
kubectl get secret kkbase-watcher-secret -o jsonpath='{.data.LLM_API_KEY}' | base64 -d
```

### No Events Being Processed

1. Check event filtering configuration
2. Verify events exist in Neo4j:
   ```cypher
   MATCH (e:Event)
   WHERE e.last_timestamp > datetime() - duration('PT10M')
   RETURN count(e)
   ```
3. Check agent logs for filter messages

### Gemini API Errors

Common issues:

- **401 Unauthorized**: Invalid API key
- **429 Rate Limited**: Too many requests (reduce workers or add delays)
- **503 Service Unavailable**: Gemini service issue (retry with backoff)

### High Resource Usage

Reduce concurrent investigations:

```yaml
AGENT_WORKERS: "1"  # Default, reduce if needed
```

## Development

### Build Custom Image

```bash
# Build locally
make build-agent
make docker-build-agent-fast

# Build in CI/CD
make docker-build-agent

# Push to registry
make docker-push-agent
```

### Local Testing

Run agent locally against deployed MCP server:

```bash
# Port forward MCP server
kubectl port-forward svc/kkbase-integrated 8081:8081

# Set environment variables
export NEO4J_URI="bolt://localhost:7687"
export NEO4J_USERNAME="neo4j"
export NEO4J_PASSWORD="your-password"
export NEO4J_DATABASE="neo4j"
export AGENT_MCP_SERVER_URL="http://localhost:8081/mcp"
export LLM_API_KEY="your-gemini-api-key"
export AGENT_ENABLED="true"
export AGENT_WORKERS="1"

# Run agent
./agent
```

## Best Practices

1. **Start with 1 worker** - Monitor resource usage before scaling
2. **Use specific event filters** - Reduce noise and costs
3. **Monitor API usage** - Gemini has rate limits and costs
4. **Review recommendations** - Don't auto-apply without review
5. **Set resource limits** - Prevent runaway LLM costs
6. **Enable logging** - Critical for debugging investigations

## Cost Considerations

Gemini API costs scale with:
- Number of events processed
- Tokens per investigation
- Number of function calls per event

To minimize costs:
- Use strict event filtering
- Set appropriate LLM_MAX_TOKENS
- Use a single worker initially
- Monitor token usage in Gemini console

## Next Steps

- [View Agent Architecture](../development/architecture.md)
- [Configure Event Sources](./investigation-tools.md)
- [Review MCP Tools](../reference/agent-mcp-tools.md)
- [Understand Investigation Flow](../reference/agent-execution-flow.md)

