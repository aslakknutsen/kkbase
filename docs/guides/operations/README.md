# Operations Guides

Day-to-day operations, troubleshooting, and scaling for kkbase.

## What's in This Section?

| Guide | Purpose |
|-------|---------|
| [Monitoring](monitoring.md) | Health checks, logs, metrics |
| [Troubleshooting](troubleshooting.md) | Common issues and solutions |
| [Scaling](scaling.md) | Horizontal and vertical scaling |

## Quick Links

### Health Checks

```bash
# Watcher
kubectl get pods -l app=kkbase-watcher

# MCP Server
curl http://localhost:8080/health

# Agent
curl http://localhost:9090/health

# Neo4j
kubectl exec neo4j-0 -- cypher-shell -u neo4j -p changeme "RETURN 1"
```

### View Logs

```bash
# Watcher
kubectl logs -f deployment/kkbase-watcher

# MCP Server
kubectl logs -f deployment/kkbase-mcp-server

# Agent
kubectl logs -f deployment/kkbase-agent

# Neo4j
kubectl logs neo4j-0
```

### Common Operations

```bash
# Restart watcher
kubectl rollout restart deployment/kkbase-watcher

# Scale MCP server
kubectl scale deployment/kkbase-mcp-server --replicas=3

# Update configuration
kubectl edit configmap kkbase-config

# Check resource usage
kubectl top pods
```

## See Also

- [Service Documentation](../../services/)
- [Deployment Guides](../../getting-started/)

