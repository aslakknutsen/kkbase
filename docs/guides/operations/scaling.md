# Scaling kkbase

Scale kkbase services for larger clusters and higher load.

## Horizontal Scaling

### MCP Server

Scale MCP server for more concurrent investigations:

```bash
kubectl scale deployment kkbase-mcp-server --replicas=3
```

Add load balancer:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: kkbase-mcp-server
spec:
  type: LoadBalancer  # or use Ingress
```

### Agent Service

Scale agents for more concurrent webhook processing:

```bash
kubectl scale deployment kkbase-agent --replicas=3
```

### Watcher Service

**Note**: Watcher should run as single replica to avoid duplicate writes to Neo4j.

## Vertical Scaling

### Increase Resources

```yaml
resources:
  limits:
    memory: "2Gi"
    cpu: "2000m"
  requests:
    memory: "1Gi"
    cpu: "1000m"
```

Apply:

```bash
kubectl set resources deployment/kkbase-mcp-server \
  --limits=memory=2Gi,cpu=2000m \
  --requests=memory=1Gi,cpu=1000m
```

## Neo4j Scaling

### Increase Resources

```yaml
resources:
  limits:
    memory: "4Gi"
    cpu: "2000m"
```

### Clustering (Enterprise)

Neo4j Enterprise supports clustering for high availability.

## Large Cluster Optimizations

### For 100+ Nodes

- Increase watcher resync period: `RESYNC_PERIOD=5m`
- Scale MCP server replicas: 3+
- Increase Neo4j resources

### For 1000+ Nodes

- Consider namespace sharding (multiple watchers)
- Increase Neo4j memory: 8Gi+
- Monitor query performance
- Add database indexes

### For 10000+ Nodes

- Implement graph partitioning
- Use Neo4j cluster
- Consider read replicas
- Optimize frequent queries

## Monitoring Scaling

Watch resource usage:

```bash
kubectl top pods
kubectl top nodes
```

Check Neo4j size:

```cypher
MATCH (n) RETURN count(n)
```

## See Also

- [Monitoring](monitoring.md)
- [Troubleshooting](troubleshooting.md)

