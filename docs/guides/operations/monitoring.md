# Monitoring kkbase

Monitor the health and performance of kkbase services.

## Health Checks

### Watcher Service

```bash
# Liveness
kubectl get pods -l app=kkbase-watcher

# Readiness (checks Neo4j connection)
kubectl exec deployment/kkbase-watcher -- curl -f http://localhost:8080/ready
```

### MCP Server

```bash
# Health endpoint
kubectl exec deployment/kkbase-mcp-server -- curl -f http://localhost:8080/health
```

### Agent Service

```bash
# Health endpoint
kubectl exec deployment/kkbase-agent -- curl -f http://localhost:9090/health
```

## Logs

### Watcher Logs

```bash
# View logs
kubectl logs -f deployment/kkbase-watcher

# Check sync status
kubectl logs deployment/kkbase-watcher | grep "synced"

# See errors
kubectl logs deployment/kkbase-watcher | grep ERROR
```

### MCP Server Logs

```bash
# View logs
kubectl logs -f deployment/kkbase-mcp-server

# Monitor sessions
kubectl logs -f deployment/kkbase-mcp-server | grep "agent session"
```

### Agent Logs

```bash
# View logs
kubectl logs -f deployment/kkbase-agent

# Monitor investigations
kubectl logs -f deployment/kkbase-agent | grep investigation
```

## Metrics (Planned)

Future Prometheus metrics for monitoring.

## Neo4j Monitoring

```bash
# Check Neo4j status
kubectl get pods -l app=neo4j

# View Neo4j logs
kubectl logs neo4j-0

# Check database size
kubectl exec neo4j-0 -- cypher-shell -u neo4j -p changeme \
  "MATCH (n) RETURN count(n) as total_nodes"
```

## See Also

- [Troubleshooting](troubleshooting.md)
- [Scaling](scaling.md)

