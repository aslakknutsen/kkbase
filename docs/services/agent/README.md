# Agent Service

The Agent service provides autonomous diagnostic capabilities, responding to alerts and incidents automatically to investigate cluster issues.

## What It Does

The agent:
- **Responds** to webhooks from monitoring systems (Prometheus, PagerDuty, etc.)
- **Pulls** issues from external systems (ticket systems, incident managers)
- **Investigates** using MCP tools to query the knowledge graph
- **Generates** findings and recommendations automatically
- **Reports** results to incident channels (Slack, tickets, etc.)
- **Learns** from past investigations to improve diagnostics

## When to Use

Deploy the agent when you want:

- **Autonomous troubleshooting** - Automatic response to alerts
- **24/7 on-call coverage** - Agent handles initial investigation
- **Incident acceleration** - Faster time to diagnosis
- **Knowledge capture** - Record investigation patterns
- **Alert enrichment** - Add context to notifications

## Architecture

```
┌────────────────────────────────────────────┐
│      External Systems                       │
│  ┌─────────────┐  ┌──────────────────┐    │
│  │ Prometheus  │  │   PagerDuty      │    │
│  │   Alerts    │  │   Incidents      │    │
│  └──────┬──────┘  └────────┬─────────┘    │
└─────────┼──────────────────┼───────────────┘
          │                  │
          │ Webhooks         │ API Polling
          ↓                  ↓
┌─────────────────────────────────────────┐
│        Agent Service                    │
│                                         │
│  ┌───────────────────────────────────┐ │
│  │  Webhook Receiver                 │ │
│  │  - /webhook endpoint              │ │
│  │  - Signature validation           │ │
│  │  - Alert parsing                  │ │
│  └───────────────────────────────────┘ │
│                                         │
│  ┌───────────────────────────────────┐ │
│  │  Issue Pullers (optional)         │ │
│  │  - PagerDuty puller               │ │
│  │  - Jira puller                    │ │
│  │  - Custom pullers                 │ │
│  └───────────────────────────────────┘ │
│                                         │
│  ┌───────────────────────────────────┐ │
│  │  Investigation Engine             │ │
│  │  - LLM integration (GPT, Claude)  │ │
│  │  - MCP tool execution             │ │
│  │  - Finding synthesis              │ │
│  │  - Recommendation generation      │ │
│  └───────────────────────────────────┘ │
│                                         │
│  ┌───────────────────────────────────┐ │
│  │  Reporters                        │ │
│  │  - Slack notifications            │ │
│  │  - Ticket updates                 │ │
│  │  - Email summaries                │ │
│  └───────────────────────────────────┘ │
└──────────┬──────────────────────────────┘
           │
           │ MCP Client
           ↓
┌──────────────────────┐
│    MCP Server        │
│                      │
│ Uses tools:          │
│ - start_agent_session│
│ - query_with_session │
│ - update_hypothesis  │
│ - record_finding     │
│ - record_recommend...|
│ - complete_agent_... │
└──────────────────────┘
```

## Key Features

### Webhook Integration

Receive and process alerts from monitoring systems:

**Supported Sources**:
- Prometheus Alertmanager
- PagerDuty webhooks
- Grafana alerts
- Custom webhook providers

**Features**:
- Signature validation for security
- Alert deduplication
- Priority-based queuing
- Concurrent investigation handling

### Issue Pulling

Actively pull incidents from external systems:

**Supported Sources**:
- PagerDuty API
- Jira API
- ServiceNow
- Custom integrations

**Features**:
- Configurable polling intervals
- Status filtering (new, acknowledged)
- Automatic sync
- State tracking

### Investigation Engine

LLM-powered autonomous diagnostics:

**Capabilities**:
- Natural language understanding of alerts
- Strategic query planning
- Multi-hop reasoning
- Context-aware investigation
- Hypothesis evolution
- Finding synthesis

**LLM Integrations**:
- OpenAI GPT-4/GPT-3.5
- Anthropic Claude
- Google Gemini
- Custom LLM endpoints

### Reporting

Communicate findings to stakeholders:

**Channels**:
- Slack messages with formatted findings
- Ticket updates (Jira, PagerDuty)
- Email summaries
- Webhook callbacks

**Content**:
- Investigation summary
- Root cause analysis
- Blast zone visualization
- Actionable recommendations
- Timeline of events

## Deployment Modes

### Mode 1: Webhook-Triggered (Recommended)

Agent responds to incoming webhooks:

```
Alert → Webhook → Agent → Investigate → Report
```

**When to use**: Production environments with existing monitoring

### Mode 2: Polling-Based

Agent polls external systems for new incidents:

```
Agent → Poll PagerDuty → New Incident? → Investigate → Report
```

**When to use**: Integration with ticket systems, scheduled diagnostics

### Mode 3: Hybrid

Combination of webhooks and polling:

```
Alert → Webhook → Agent ←→ Poll Tickets
                    ↓
                Investigate → Report
```

**When to use**: Complex incident management workflows

## Configuration

Key configuration options:

| Variable | Purpose | Default |
|----------|---------|---------|
| `WEBHOOK_PORT` | Webhook receiver port | `9090` |
| `WEBHOOK_SECRET` | Signature validation secret | - |
| `MCP_SERVER_URL` | MCP server endpoint | `http://kkbase-mcp-server:8080/mcp` |
| `LLM_PROVIDER` | LLM service (openai, anthropic, gemini) | `openai` |
| `LLM_API_KEY` | LLM API key | - |
| `LLM_MODEL` | Model name | `gpt-4` |

See [Configuration Guide](configuration.md) for all options.

## Quick Deploy

```bash
# Create configuration
kubectl apply -f agent-config.yaml

# Create secrets
kubectl create secret generic kkbase-agent-secret \
  --from-literal=LLM_API_KEY=your-api-key \
  --from-literal=WEBHOOK_SECRET=your-webhook-secret

# Deploy agent
kubectl apply -f agent-deployment.yaml

# Expose webhook endpoint
kubectl apply -f agent-service.yaml
```

See [Deployment Guide](deployment.md) for detailed instructions.

## Webhook Setup

### Prometheus Alertmanager

Configure Alertmanager to send webhooks:

```yaml
receivers:
- name: kkbase-agent
  webhook_configs:
  - url: http://kkbase-agent:9090/webhook
    send_resolved: true
    http_config:
      bearer_token: your-webhook-secret
```

### PagerDuty

Add webhook extension:

1. Go to PagerDuty → Extensions
2. Add Generic V3 Webhook
3. URL: `https://your-cluster/kkbase-agent/webhook`
4. Custom Header: `X-Webhook-Secret: your-secret`

### Grafana

Configure alert notification channel:

```
Type: webhook
URL: http://kkbase-agent:9090/webhook
HTTP Method: POST
Headers: X-Webhook-Secret: your-secret
```

See [Integration Guide](integration.md) for complete setup.

## Investigation Flow

### Automatic Investigation Process

1. **Alert Received** - Webhook or poll detects incident
2. **Session Started** - Agent calls `start_agent_session`
3. **Context Gathering** - Queries graph for related resources
4. **Hypothesis Formation** - Agent forms initial theory
5. **Investigation** - Executes queries, spawns metrics investigations
6. **Finding Synthesis** - Combines automatic and analyzed findings
7. **Recommendations** - Generates actionable next steps
8. **Session Completion** - Finalizes investigation
9. **Reporting** - Sends findings to configured channels

### Example Investigation

```
Alert: "High error rate in orders-api"
  ↓
Agent receives webhook
  ↓
start_agent_session(symptom="High error rate in orders-api")
  ↓
query_with_session("Find orders-api service and pods")
  → Discovers 3 pods, 2 are CrashLoopBackOff
  ↓
update_hypothesis("Pods crashing, likely OOM or config issue")
  ↓
query_with_session("Check events for pods")
  → Finds OOMKilled events
  ↓
spawn_investigation("Pull memory metrics for affected pods")
  → Confirms memory spike before crashes
  ↓
update_hypothesis("Memory leak in recent deployment")
  ↓
query_with_session("Find recent deployments")
  → Deployment v2.3.5 deployed 15 minutes before issue
  ↓
record_recommendation(
  type="root_cause_fix",
  title="Rollback to v2.3.4",
  action_items=[...]
)
  ↓
complete_agent_session(summary="Memory leak in v2.3.5...")
  ↓
Report to Slack with findings and recommendations
```

## Performance

### Resource Usage

Typical consumption:
```
Memory: 30MB baseline + LLM client (~50MB per investigation)
CPU: <0.1 core average
Concurrency: 10+ simultaneous investigations
Latency: 10-60 seconds per investigation (depends on LLM)
```

### Scalability

- **Horizontal scaling**: Multiple replicas supported
- **Load balancing**: Round-robin webhook distribution
- **Rate limiting**: Configurable per source
- **Queue depth**: Handles bursts up to 100 alerts

## Security

### Webhook Validation

- **Signature verification**: HMAC-SHA256 validation
- **Secret rotation**: Support for key rollover
- **IP allowlisting**: Restrict source IPs
- **TLS**: HTTPS only in production

### LLM API Keys

- **Secret storage**: Kubernetes Secrets
- **Key rotation**: Update without restart
- **Access control**: RBAC for secret access

### Network Security

- **Network policies**: Restrict egress to MCP server and LLM APIs
- **Service mesh**: mTLS for internal communication

## Monitoring

### Health Checks

```bash
# Health endpoint
curl http://localhost:9090/health

# Expected: {"status":"healthy"}
```

### Metrics

Agent exposes Prometheus metrics:

```
# Webhook requests
agent_webhooks_received_total{source="prometheus"}

# Investigation duration
agent_investigation_duration_seconds

# Investigation outcomes
agent_investigation_status_total{status="completed"}

# LLM token usage
agent_llm_tokens_total{provider="openai"}
```

### Logs

```bash
# View agent logs
kubectl logs -f deployment/kkbase-agent

# Filter by severity
kubectl logs deployment/kkbase-agent | grep ERROR

# Monitor investigations
kubectl logs -f deployment/kkbase-agent | grep "investigation"
```

## Troubleshooting

### Webhooks Not Received

```bash
# Check service
kubectl get svc kkbase-agent

# Test webhook endpoint
curl -X POST http://kkbase-agent:9090/webhook \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: your-secret" \
  -d '{"alert":"test"}'

# Check logs
kubectl logs deployment/kkbase-agent | grep webhook
```

### Agent Can't Connect to MCP Server

```bash
# Test MCP server connectivity
kubectl exec deployment/kkbase-agent -- \
  curl -X POST http://kkbase-mcp-server:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Check configuration
kubectl get configmap kkbase-agent-config -o yaml | grep MCP_SERVER_URL
```

### LLM API Failures

```bash
# Check API key
kubectl get secret kkbase-agent-secret -o jsonpath='{.data.LLM_API_KEY}' | base64 -d

# Test LLM connectivity (from agent pod)
kubectl exec deployment/kkbase-agent -- \
  curl -H "Authorization: Bearer $LLM_API_KEY" \
  https://api.openai.com/v1/models

# Check logs for LLM errors
kubectl logs deployment/kkbase-agent | grep -i "llm\|api"
```

See [Troubleshooting Guide](../../guides/operations/troubleshooting.md) for more solutions.

## Best Practices

1. **Start with webhooks** - Easier than polling setup
2. **Use signature validation** - Secure webhook endpoints
3. **Monitor LLM costs** - Track token usage
4. **Review investigations** - Check dashboard for quality
5. **Tune prompts** - Customize for your environment
6. **Set rate limits** - Prevent alert storms
7. **Configure reporting** - Send to right channels

## Documentation

- **[Deployment Guide](deployment.md)** - Step-by-step deployment
- **[Configuration](configuration.md)** - All configuration options
- **[Integration Guide](integration.md)** - Webhook and API setup
- **[Investigation Workflow](../../guides/investigations/workflow.md)** - How agents investigate

## Quick Links

- [Getting Started](../../getting-started/)
- [System Architecture](../../ARCHITECTURE.md)
- [MCP Server](../mcp-server/)
- [Operations Guide](../../guides/operations/)

