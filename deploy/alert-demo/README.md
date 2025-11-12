# Alert Demo Setup

Demo-optimized Prometheus alerts and AlertManager configuration for triggering kkbase agent investigations.

## Prerequisites

- Kubernetes cluster with Prometheus Operator (kube-prometheus-stack)
- kkbase deployed (integrated mode recommended)
- testservice instances deployed with ServiceMonitors

## Files

- `prometheus-rules.yaml` - Alert rules for detecting HTTP errors, resource spikes, and pod crashes
- `alertmanager-config.yaml` - AlertManager configuration (AlertmanagerConfig CRD) to send alerts to kkbase webhook

## Installation

### 1. Deploy Alert Rules

```bash
kubectl apply -f prometheus-rules.yaml
```

This creates a PrometheusRule in the `monitoring` namespace that Prometheus will automatically pick up.

### 2. Configure AlertManager

Apply the AlertmanagerConfig resource:

```bash
kubectl apply -f alertmanager-config.yaml
```

This creates an `AlertmanagerConfig` CRD that Prometheus Operator will automatically merge into the Alertmanager configuration.

**Note:** Your Alertmanager must be configured to watch `AlertmanagerConfig` resources. If using kube-prometheus-stack, ensure `alertmanagerConfigSelector` is set (it's usually enabled by default).

Check the configuration was picked up:

```bash
kubectl get alertmanagerconfig -n monitoring
```

AlertManager will reload automatically (may take 30-60 seconds).

### 3. Verify Setup

Check that Prometheus loaded the rules:

```bash
kubectl get prometheusrules -n monitoring kkbase-demo-alerts
```

Check AlertManager config:

```bash
kubectl exec -n monitoring alertmanager-kube-prometheus-alertmanager-0 -- amtool config show
```

Check kkbase is receiving webhooks (check logs):

```bash
kubectl logs -n default -l app=kkbase-integrated --tail=50 -f
```

## Alert Types

### HTTP Errors

- **HTTPClientErrors** - 4xx client errors (e.g., 415 from config issues)
- **HTTPServerErrors** - 5xx server errors from the service itself
- **UpstreamServerErrors** - 5xx errors from upstream dependencies

Triggers after 20 seconds of continuous errors.

### Resource Spikes

- **CPUSpike** - CPU usage > 50% for 15 seconds
- **MemoryGrowth** - Memory growing > 5MB/sec for 30 seconds
- **GoroutineSpike** - Goroutine count > 50 or growing rapidly

### Pod Crashes

- **HighRestartRate** - Pod restarting frequently (requires kube-state-metrics)

## Testing Alerts

### Trigger HTTP Client Errors (415)

Inject a behavior that causes the testservice to make requests with wrong content-type:

```bash
# Replace with your service endpoint
SERVICE_URL="http://checkout.sf-shopping.svc.cluster.local:8080"

# Trigger 415 errors by misconfiguring upstream
curl "${SERVICE_URL}/?behavior=checkout:error=415:1.0"
```

Or use a misconfigured upstream in the deployment's UPSTREAMS env var.

### Trigger HTTP Server Errors (500)

```bash
# 100% error rate
curl "${SERVICE_URL}/?behavior=error=500:1.0"

# Or 30% error rate for more realistic demo
curl "${SERVICE_URL}/?behavior=error=500:0.3"
```

### Trigger Upstream Errors

Configure the testservice to call an upstream and inject errors there:

```bash
# Assuming 'payment-api' is an upstream
curl "${SERVICE_URL}/?behavior=payment-api:error=503:1.0"
```

### Trigger CPU Spike

```bash
# 10 second CPU spike at 80% intensity
curl "${SERVICE_URL}/?behavior=cpu=spike:10s:80"

# Default 5s spike
curl "${SERVICE_URL}/?behavior=cpu=spike"
```

### Trigger Memory Growth

```bash
# Fast memory leak
curl "${SERVICE_URL}/?behavior=memory=leak-fast"

# Slow memory leak
curl "${SERVICE_URL}/?behavior=memory=leak-slow:5m"
```

### Trigger Pod Crash

**⚠️ Warning:** This will actually crash the pod!

```bash
# 100% chance to crash (use for testing)
curl "${SERVICE_URL}/?behavior=panic=1.0"

# 10% chance to crash (more realistic)
curl "${SERVICE_URL}/?behavior=panic=0.1"
```

## Checking Alert Status

### View Active Alerts in Prometheus

```bash
kubectl port-forward -n monitoring svc/kube-prometheus-kube-prometheus 9090:9090
```

Open http://localhost:9090/alerts

### View AlertManager Status

```bash
kubectl port-forward -n monitoring svc/kube-prometheus-kube-alertmanager 9093:9093
```

Open http://localhost:9093

### Query for Specific Metrics

```bash
# Check if errors are being recorded
kubectl port-forward -n monitoring svc/kube-prometheus-kube-prometheus 9090:9090

# Then query:
rate(http_client_requests_total{status_code=~"4.."}[1m])
rate(http_server_requests_total{status_code=~"5.."}[1m])
process_cpu_seconds_total
```

## Demo Workflow

1. **Start kkbase** and verify it's running
2. **Deploy testservice** with ServiceMonitor
3. **Apply alert rules** and AlertManager config
4. **Trigger an error** using curl with behavior parameter
5. **Wait 20-30 seconds** for alert to fire
6. **Check kkbase logs** to see webhook received
7. **Watch agent investigation** via MCP tools or logs

## Troubleshooting

**Alerts not firing:**
- Check Prometheus has scraped recent metrics: `up{job=~".*checkout.*"}`
- Verify ServiceMonitor is working: `kubectl get servicemonitor -A`
- Check alert evaluation: Prometheus UI → Alerts tab

**Webhook not received:**
- Check kkbase service exists: `kubectl get svc kkbase-integrated -n default`
- Check kkbase logs for webhook errors
- Verify AlertmanagerConfig was created: `kubectl get alertmanagerconfig -n monitoring kkbase-webhook-config`
- Check AlertManager picked up the config: `kubectl exec -n monitoring alertmanager-kube-prometheus-alertmanager-0 -- amtool config show`

**Alerts firing but no investigation:**
- Check kkbase agent is enabled: `ALERTMANAGER_WEBHOOK_ENABLED=true`
- Check event source logs in kkbase
- Verify webhook payload format matches expected schema

## Customization

### Change Alert Thresholds

Edit `prometheus-rules.yaml` and adjust:
- `for: 20s` - How long condition must be true before firing
- `[2m]` in rate() - Time window for rate calculation (must be 4x scrape interval minimum)
- `> 0` - Threshold value (e.g., change to `> 0.1` for 0.1 errors/sec)

**Note:** Rate windows are set to `[2m]` to work with 30s scrape intervals. If you change your scrape interval, adjust rate windows to be at least 4x that value.

### Change Webhook URL

If kkbase is in a different namespace, edit `alertmanager-config.yaml`:

```yaml
webhookConfigs:
- url: 'http://kkbase-integrated.<namespace>.svc.cluster.local:8082/webhook/alertmanager'
```

### Add More Alert Routes

Add routes in `alertmanager-config.yaml` under `spec.route.routes`:

```yaml
- matchers:
  - name: alertname
    value: CPUSpike
  receiver: 'kkbase-webhook'
  groupWait: 15s
```

