# Health Endpoint

## Overview

The MCP server exposes a `/health` endpoint for monitoring and Kubernetes health checks (liveness and readiness probes).

## Endpoint Details

**URL**: `GET /health`  
**Port**: Same as MCP server (default: 8080)  
**Content-Type**: `application/json`

## Response Format

### Success Response

**Status Code**: `200 OK`

```json
{
  "status": "healthy",
  "service": "kkbase-mcp",
  "version": "1.0.0"
}
```

## Usage

### Manual Check

```bash
# Local
curl http://localhost:8080/health

# In Kubernetes cluster
curl http://kkbase-integrated:8080/health
```

### Kubernetes Health Checks

The endpoint is used for both liveness and readiness probes in Kubernetes deployments.

#### Liveness Probe

Checks if the container is alive and should be restarted if failing:

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3
```

#### Readiness Probe

Checks if the container is ready to accept traffic:

```yaml
readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3
```

## Implementation

The health endpoint is a simple HTTP handler that always returns success if the server is running:

```go
mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)

    response := map[string]interface{}{
        "status":  "healthy",
        "service": "kkbase-mcp",
        "version": "1.0.0",
    }

    json.NewEncoder(w).Encode(response)
})
```

## Health Check Behavior

### What it Checks

Currently, the health endpoint performs a **basic liveness check**:
- ✅ HTTP server is running
- ✅ HTTP handler is responsive
- ✅ JSON encoding works

### What it Does NOT Check

The endpoint does **not** currently validate:
- ❌ Neo4j connectivity
- ❌ Prometheus availability
- ❌ MCP server internal state
- ❌ Frontend embedding status

This is intentional - the health endpoint is designed for Kubernetes liveness probes, which should only check if the process needs restarting, not if dependencies are available.

## Future Enhancements

### Deep Health Check (Optional)

For production monitoring, you may want to add a separate `/health/deep` endpoint that checks dependencies:

```go
mux.HandleFunc("/health/deep", func(w http.ResponseWriter, r *http.Request) {
    health := map[string]interface{}{
        "status": "healthy",
        "checks": map[string]interface{}{
            "neo4j": checkNeo4j(),
            "prometheus": checkPrometheus(),
        },
    }
    
    // Return 503 if any check fails
    if !allHealthy(health["checks"]) {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
    
    json.NewEncoder(w).Encode(health)
})
```

### Metrics Endpoint

Consider adding a Prometheus metrics endpoint:

```go
mux.Handle("/metrics", promhttp.Handler())
```

## Troubleshooting

### Health Check Fails

If the health endpoint returns an error or times out:

1. **Check if server is running**:
   ```bash
   kubectl get pods -n kkbase
   # Should show READY 2/2
   ```

2. **Check container logs**:
   ```bash
   kubectl logs -n kkbase -l app=kkbase-integrated -c mcp-server
   ```

3. **Port forward and test locally**:
   ```bash
   kubectl port-forward -n kkbase svc/kkbase-integrated 8080:8080
   curl http://localhost:8080/health
   ```

4. **Common issues**:
   - Server failed to start (check logs for startup errors)
   - Port conflict or misconfiguration
   - Network policy blocking traffic

### Liveness Probe Restarting Container

If the liveness probe keeps restarting the container:

1. **Increase `initialDelaySeconds`** - Server may need more time to start:
   ```yaml
   initialDelaySeconds: 30  # Give more time
   ```

2. **Increase `failureThreshold`** - Allow more failed checks before restart:
   ```yaml
   failureThreshold: 5  # Allow 5 failures instead of 3
   ```

3. **Check timeout** - Ensure health endpoint responds quickly:
   ```yaml
   timeoutSeconds: 5  # Enough time for response
   ```

### Readiness Probe Not Ready

If the readiness probe keeps the pod in NotReady state:

1. **Check health endpoint manually**:
   ```bash
   kubectl exec -n kkbase deployment/kkbase-integrated -c mcp-server -- \
     curl -s http://localhost:8080/health
   ```

2. **Review readiness probe config** - May be too aggressive:
   ```yaml
   readinessProbe:
     initialDelaySeconds: 5
     periodSeconds: 10  # Check less frequently
     failureThreshold: 3
   ```

## Monitoring

### Prometheus Metrics (Future)

Once metrics are added, you can monitor health check failures:

```promql
# Rate of health check failures
rate(http_requests_total{path="/health",status=~"5.."}[5m])

# Health check response time
histogram_quantile(0.99, http_request_duration_seconds{path="/health"})
```

### Grafana Dashboard

Create alerts for:
- Health endpoint returning errors
- Health endpoint response time > 1s
- Health check probe failures causing restarts

## Testing

### Unit Test

```go
func TestHealthEndpoint(t *testing.T) {
    req := httptest.NewRequest("GET", "/health", nil)
    w := httptest.NewRecorder()
    
    handler.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    
    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)
    
    assert.Equal(t, "healthy", response["status"])
    assert.Equal(t, "kkbase-mcp", response["service"])
}
```

### Integration Test

```bash
#!/bin/bash
# test-health-endpoint.sh

# Start server
./mcp-server &
SERVER_PID=$!

# Wait for startup
sleep 2

# Test health endpoint
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health)

if [ "$RESPONSE" -eq 200 ]; then
    echo "✅ Health endpoint working"
    exit 0
else
    echo "❌ Health endpoint failed: HTTP $RESPONSE"
    exit 1
fi

# Cleanup
kill $SERVER_PID
```

## Related Endpoints

- `/` - Dashboard (Frontend)
- `/mcp` - MCP SSE endpoint
- `/assets/` - Frontend assets
- `/health` - Health check (current)

## API Contract

The health endpoint follows a simple contract:

- **Method**: `GET` only (returns 405 for other methods)
- **Status**: Always 200 if server is running
- **Format**: Always JSON
- **Fields**: 
  - `status`: Always "healthy"
  - `service`: Always "kkbase-mcp"
  - `version`: Current version string

This contract ensures Kubernetes health probes work reliably.

## Summary

✅ **Endpoint**: `GET /health`  
✅ **Status**: 200 OK if server running  
✅ **Format**: JSON with status, service, version  
✅ **Used by**: Kubernetes liveness/readiness probes  
✅ **Simple**: No dependency checks (by design)  
✅ **Fast**: Always responds in < 10ms  

The health endpoint provides a reliable way for Kubernetes to monitor the MCP server's liveness and readiness state.

