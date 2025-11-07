# Agent Integration Guide

Integrate the kkbase Agent with external monitoring and incident management systems.

## Prometheus Alertmanager

Configure Alertmanager to send webhooks to the agent.

### Alertmanager Configuration

Edit `alertmanager.yml`:

```yaml
receivers:
- name: kkbase-agent
  webhook_configs:
  - url: http://kkbase-agent:9090/webhook
    send_resolved: true
    http_config:
      bearer_token: your-webhook-secret

route:
  receiver: kkbase-agent
  group_by: ['alertname', 'cluster']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h
```

### Test Webhook

```bash
# Send test alert
curl -X POST http://kkbase-agent:9090/webhook \
  -H "Authorization: Bearer your-webhook-secret" \
  -H "Content-Type: application/json" \
  -d '{
    "alerts": [{
      "labels": {
        "alertname": "HighErrorRate",
        "service": "orders-api",
        "severity": "critical"
      },
      "annotations": {
        "summary": "High error rate detected in orders-api"
      },
      "status": "firing"
    }]
  }'
```

## PagerDuty

Configure PagerDuty to send webhooks for new incidents.

### Setup Steps

1. Go to **PagerDuty** → **Extensions**
2. Click **New Extension**
3. Select **Generic V3 Webhook**
4. **Name**: kkbase Agent
5. **URL**: `https://your-cluster/kkbase-agent/webhook`
6. **Custom Headers**:
   ```
   X-Webhook-Secret: your-webhook-secret
   ```
7. **Events**: Select incident.triggered, incident.acknowledged
8. Save

### Test

Trigger a test incident in PagerDuty and verify the agent receives it in logs:

```bash
kubectl logs -f deployment/kkbase-agent | grep pagerduty
```

## Grafana

Configure Grafana alert notification channel.

### Setup Steps

1. Go to **Grafana** → **Alerting** → **Notification channels**
2. Click **New channel**
3. **Type**: webhook
4. **URL**: `http://kkbase-agent:9090/webhook`
5. **HTTP Method**: POST
6. **Headers**:
   ```
   X-Webhook-Secret: your-webhook-secret
   ```
7. Test and Save

## Custom Webhooks

For custom integrations, send POST requests to `/webhook` endpoint.

### Request Format

```json
{
  "alert": {
    "name": "string",
    "severity": "critical|warning|info",
    "service": "string",
    "description": "string",
    "timestamp": "ISO8601"
  }
}
```

### Required Headers

```
Content-Type: application/json
X-Webhook-Secret: your-webhook-secret
```

### Example

```bash
curl -X POST https://kkbase.example.com/agent/webhook \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: your-secret" \
  -d '{
    "alert": {
      "name": "Database Connection Pool Exhausted",
      "severity": "critical",
      "service": "postgres-primary",
      "description": "Connection pool at 100% capacity",
      "timestamp": "2024-11-07T10:30:00Z"
    }
  }'
```

## Webhook Security

### Signature Validation

The agent validates webhook signatures using HMAC-SHA256:

1. Sender computes: `HMAC-SHA256(payload, secret)`
2. Sender includes in header: `X-Webhook-Signature: sha256=<hash>`
3. Agent verifies signature matches

### IP Allowlisting

Restrict webhook sources via NetworkPolicy:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: kkbase-agent-webhook
spec:
  podSelector:
    matchLabels:
      app: kkbase-agent
  policyTypes:
  - Ingress
  ingress:
  - from:
    - ipBlock:
        cidr: 10.0.0.0/8  # Your monitoring system IPs
    ports:
    - protocol: TCP
      port: 9090
```

## See Also

- [Agent README](README.md)
- [Deployment Guide](deployment.md)
- [Configuration](configuration.md)

