# Agent Deployment Guide

Deployment guide for the kkbase Agent service (autonomous diagnostics).

## Prerequisites

- Kubernetes v1.19+
- MCP Server deployed and accessible
- LLM API key (OpenAI, Anthropic, or Gemini)
- Webhook secret for signature validation

## Quick Deploy

```bash
# Create configuration
kubectl apply -f agent-config.yaml

# Create secrets
kubectl create secret generic kkbase-agent-secret \
  --from-literal=LLM_API_KEY=your-openai-api-key \
  --from-literal=WEBHOOK_SECRET=your-webhook-secret

# Deploy agent
kubectl apply -f agent-deployment.yaml

# Expose webhook endpoint
kubectl apply -f agent-service.yaml
```

## Configuration Files

### agent-config.yaml

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-agent-config
  namespace: default
data:
  # MCP Server
  MCP_SERVER_URL: "http://kkbase-mcp-server:8080/mcp"
  
  # Webhook
  WEBHOOK_PORT: "9090"
  
  # LLM Configuration
  LLM_PROVIDER: "openai"  # openai, anthropic, gemini
  LLM_MODEL: "gpt-4"
  
  # Logging
  LOG_LEVEL: "info"
```

### agent-deployment.yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kkbase-agent
  namespace: default
  labels:
    app: kkbase-agent
spec:
  replicas: 2  # Multiple replicas for HA
  selector:
    matchLabels:
      app: kkbase-agent
  template:
    metadata:
      labels:
        app: kkbase-agent
    spec:
      containers:
      - name: agent
        image: kkbase-agent:latest
        imagePullPolicy: IfNotPresent
        
        envFrom:
        - configMapRef:
            name: kkbase-agent-config
        
        env:
        - name: LLM_API_KEY
          valueFrom:
            secretKeyRef:
              name: kkbase-agent-secret
              key: LLM_API_KEY
        - name: WEBHOOK_SECRET
          valueFrom:
            secretKeyRef:
              name: kkbase-agent-secret
              key: WEBHOOK_SECRET
        
        ports:
        - name: webhook
          containerPort: 9090
          protocol: TCP
        
        resources:
          limits:
            memory: "512Mi"
            cpu: "1000m"
          requests:
            memory: "256Mi"
            cpu: "500m"
        
        livenessProbe:
          httpGet:
            path: /health
            port: 9090
          initialDelaySeconds: 10
          periodSeconds: 30
        
        readinessProbe:
          httpGet:
            path: /health
            port: 9090
          initialDelaySeconds: 5
          periodSeconds: 10
```

### agent-service.yaml

```yaml
apiVersion: v1
kind: Service
metadata:
  name: kkbase-agent
  namespace: default
  labels:
    app: kkbase-agent
spec:
  type: ClusterIP
  ports:
  - port: 9090
    targetPort: 9090
    protocol: TCP
    name: webhook
  selector:
    app: kkbase-agent
```

## Verify Deployment

```bash
# Check deployment
kubectl get deployment kkbase-agent

# Check logs
kubectl logs -f deployment/kkbase-agent

# Test webhook endpoint
kubectl port-forward svc/kkbase-agent 9090:9090
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: your-secret" \
  -d '{"test": "alert"}'
```

## Next Steps

- [Configuration Guide](configuration.md) - Detailed options
- [Integration Guide](integration.md) - Setup webhooks
- [Agent README](README.md) - Service overview

