# Troubleshooting Guide

Common issues and solutions for kkbase services.

## Watcher Issues

### Watcher Won't Start

**Symptom**: Pod CrashLoopBackOff

**Check logs**:
```bash
kubectl logs deployment/kkbase-watcher | tail -50
```

**Common Causes**:
1. **Neo4j not accessible**
   - Verify: `kubectl get pods -l app=neo4j`
   - Test: `kubectl exec deployment/kkbase-watcher -- nc -zv neo4j 7687`

2. **Wrong password**
   - Check secret: `kubectl get secret kkbase-secret -o yaml`
   - Update: `kubectl create secret generic kkbase-secret --from-literal=NEO4J_PASSWORD=correctpass --dry-run=client -o yaml | kubectl apply -f -`

3. **RBAC permissions missing**
   - Check: `kubectl get serviceaccount kkbase-watcher`
   - Apply: `kubectl apply -f rbac.yaml`

### Resources Not Syncing

**Symptom**: Graph is empty or missing resources

**Check**:
```bash
# Verify watcher is running
kubectl get pods -l app=kkbase-watcher

# Check logs for sync activity
kubectl logs deployment/kkbase-watcher | grep "synced"

# Check Neo4j
kubectl exec neo4j-0 -- cypher-shell -u neo4j -p changeme \
  "MATCH (n) RETURN labels(n)[0] as type, count(*) as count"
```

**Solutions**:
1. Check handler is enabled in ConfigMap
2. Verify RBAC for resource type
3. Restart watcher: `kubectl rollout restart deployment/kkbase-watcher`

## MCP Server Issues

### MCP Server Won't Start

**Check logs**:
```bash
kubectl logs deployment/kkbase-mcp-server
```

**Common Causes**:
1. **Neo4j connection failed**
   - Same as watcher troubleshooting above

2. **Port conflict** (integrated mode)
   - Change MCP_PORT to 8081
   - Or use standalone mode

### Dashboard Not Loading

**Check**:
```bash
kubectl logs deployment/kkbase-mcp-server | grep frontend
```

**Solution**: Ensure image has embedded frontend (`make build-mcp-server`)

### AI Tool Connection Failed

**Test MCP endpoint**:
```bash
kubectl port-forward svc/kkbase-mcp-server 8080:8080

curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

**Solutions**:
1. Verify port forward is active
2. Check MCP configuration in Cursor/Claude
3. Restart IDE

## Agent Issues

### Webhooks Not Received

**Test webhook**:
```bash
kubectl port-forward svc/kkbase-agent 9090:9090

curl -X POST http://localhost:9090/webhook \
  -H "X-Webhook-Secret: your-secret" \
  -H "Content-Type: application/json" \
  -d '{"test":"alert"}'
```

**Check logs**:
```bash
kubectl logs deployment/kkbase-agent | grep webhook
```

**Solutions**:
1. Verify service is exposed
2. Check webhook secret matches
3. Verify ingress/networking

### LLM API Failures

**Check logs**:
```bash
kubectl logs deployment/kkbase-agent | grep -i "llm\|api"
```

**Common Causes**:
1. Invalid API key
2. Rate limiting
3. Network issues

**Solutions**:
1. Verify API key: `kubectl get secret kkbase-agent-secret -o jsonpath='{.data.LLM_API_KEY}' | base64 -d`
2. Check quota/billing
3. Test connectivity

## Neo4j Issues

### Neo4j Pod Not Ready

**Check**:
```bash
kubectl describe pod neo4j-0
kubectl logs neo4j-0
```

**Common Causes**:
1. Insufficient resources
2. Storage issues
3. Wrong password

### Neo4j Performance

**Check connections**:
```cypher
CALL dbms.listConnections()
```

**Check query performance**:
```cypher
CALL dbms.listQueries()
```

**Solutions**:
1. Increase resources
2. Add indexes
3. Optimize queries

## Performance Issues

### High Memory Usage

**Check**:
```bash
kubectl top pods
```

**Solutions**:
1. Increase limits in deployment
2. Check for memory leaks in logs
3. Restart pod

### Slow Query Performance

**Profile query**:
```cypher
PROFILE MATCH (n:Pod) RETURN n LIMIT 100
```

**Solutions**:
1. Add indexes
2. Use LIMIT
3. Filter early

## See Also

- [Monitoring](monitoring.md)
- [Scaling](scaling.md)
- Service-specific troubleshooting:
  - [Watcher](../../services/watcher/deployment.md#troubleshooting)
  - [MCP Server](../../services/mcp-server/deployment.md#troubleshooting)
  - [Agent](../../services/agent/deployment.md)

