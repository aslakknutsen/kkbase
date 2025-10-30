# MCP Server Deployment Options

The kkbase MCP server can be deployed in two ways:

1. **Standalone Mode** - Separate binary that only runs the MCP server
2. **Integrated Mode** - Combined with the watcher in a single binary

## Option 1: Standalone MCP Server (Recommended for Production)

Deploy the MCP server as a separate service from the watcher. This provides better isolation and allows independent scaling.

### Build and Run

```bash
# Build the standalone MCP server
make build-mcp-server

# Configure connection to Neo4j
export NEO4J_URI="bolt://localhost:7687"
export NEO4J_USERNAME="neo4j"
export NEO4J_PASSWORD="your-password"
export NEO4J_DATABASE="neo4j"
export MCP_PORT="8080"
export LOG_LEVEL="info"

# Run the MCP server
./mcp-server
```

### Kubernetes Deployment

Create a deployment for the standalone MCP server:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kkbase-mcp-server
  namespace: kkbase
spec:
  replicas: 2
  selector:
    matchLabels:
      app: kkbase-mcp-server
  template:
    metadata:
      labels:
        app: kkbase-mcp-server
    spec:
      containers:
      - name: mcp-server
        image: your-registry/kkbase-mcp-server:latest
        ports:
        - containerPort: 8080
          name: mcp
        env:
        - name: NEO4J_URI
          value: "bolt://neo4j:7687"
        - name: NEO4J_USERNAME
          value: "neo4j"
        - name: NEO4J_PASSWORD
          valueFrom:
            secretKeyRef:
              name: neo4j-auth
              key: password
        - name: NEO4J_DATABASE
          value: "neo4j"
        - name: MCP_PORT
          value: "8080"
        - name: LOG_LEVEL
          value: "info"
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: kkbase-mcp-server
  namespace: kkbase
spec:
  selector:
    app: kkbase-mcp-server
  ports:
  - port: 8080
    targetPort: 8080
    name: mcp
  type: ClusterIP
```

### Advantages of Standalone Mode

- **Isolation** - MCP server failures don't affect watcher operation
- **Independent Scaling** - Scale MCP server based on AI agent traffic
- **Security** - Can apply different security policies (RBAC, network policies)
- **Resource Management** - Separate resource limits and monitoring
- **Updates** - Deploy updates to MCP server without restarting watcher

## Option 2: Integrated Mode (Both in One Binary)

Run both the watcher and MCP server in a single process. Useful for development, small deployments, or when you want to minimize operational complexity.

### Build and Run

```bash
# Build the watcher (includes MCP support)
make build-watcher

# Configure both services
export NEO4J_URI="bolt://localhost:7687"
export NEO4J_USERNAME="neo4j"
export NEO4J_PASSWORD="your-password"
export NEO4J_DATABASE="neo4j"
export MCP_ENABLED="true"         # Enable MCP server
export MCP_PORT="8081"             # Use different port than health check (8080)
export LOG_LEVEL="info"

# Run the combined binary
./watcher
```

### Kubernetes Deployment

Modify the existing watcher deployment to enable MCP:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kkbase-watcher
  namespace: kkbase
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kkbase-watcher
  template:
    metadata:
      labels:
        app: kkbase-watcher
    spec:
      serviceAccountName: kkbase-watcher
      containers:
      - name: watcher
        image: your-registry/kkbase-watcher:latest
        ports:
        - containerPort: 8080
          name: health
        - containerPort: 8081
          name: mcp
        env:
        - name: NEO4J_URI
          value: "bolt://neo4j:7687"
        - name: NEO4J_USERNAME
          value: "neo4j"
        - name: NEO4J_PASSWORD
          valueFrom:
            secretKeyRef:
              name: neo4j-auth
              key: password
        - name: NEO4J_DATABASE
          value: "neo4j"
        - name: MCP_ENABLED
          value: "true"
        - name: MCP_PORT
          value: "8081"
        - name: LOG_LEVEL
          value: "info"
        - name: NAMESPACE
          value: ""  # Watch all namespaces
        resources:
          requests:
            memory: "256Mi"
            cpu: "200m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: kkbase-watcher
  namespace: kkbase
spec:
  selector:
    app: kkbase-watcher
  ports:
  - port: 8080
    targetPort: 8080
    name: health
  - port: 8081
    targetPort: 8081
    name: mcp
  type: ClusterIP
```

### Advantages of Integrated Mode

- **Simplicity** - Single binary to build, deploy, and manage
- **Shared Resources** - Single Neo4j connection pool
- **Lower Overhead** - No duplicate processes or networking between services
- **Development** - Easier to run locally during development
- **Small Deployments** - Ideal for testing, demos, or small clusters

### Important Notes for Integrated Mode

⚠️ **Port Configuration**: The watcher's health server runs on port 8080 by default. When enabling MCP in integrated mode, you should either:
- Use a different port for MCP (e.g., `MCP_PORT=8081`)
- Or, modify the watcher to serve both health checks and MCP on the same port (requires code changes)

⚠️ **Resource Planning**: Both services share the same resource limits. Ensure adequate CPU and memory allocation.

⚠️ **Failure Impact**: If the MCP server encounters issues, it could affect the watcher (and vice versa).

## Configuration Comparison

| Configuration | Standalone Mode | Integrated Mode |
|---------------|-----------------|-----------------|
| `MCP_ENABLED` | Not used | `true` to enable |
| `MCP_PORT` | `8080` (default) | `8081` (recommended) |
| Binary | `./mcp-server` | `./watcher` |
| Build Command | `make build-mcp-server` | `make build-watcher` |
| Process Count | 2 (watcher + mcp) | 1 (combined) |
| Kubernetes Services | 2 separate services | 1 service, 2 ports |

## Choosing the Right Mode

### Use Standalone Mode When:
- Running in production with multiple AI agents
- Need independent scaling of MCP server
- Want to apply different security policies
- Require high availability for MCP server
- Have compliance requirements for isolation

### Use Integrated Mode When:
- Running locally for development
- Operating a small demo or test environment
- Minimizing operational complexity is priority
- Resource constraints require consolidation
- Simple use cases with low AI agent traffic

## Testing Both Modes

### Test Standalone Mode

```bash
# Terminal 1: Start watcher
export NEO4J_PASSWORD="changeme"
./watcher

# Terminal 2: Start MCP server
export NEO4J_PASSWORD="changeme"
./mcp-server

# Terminal 3: Test MCP endpoint
curl http://localhost:8080/health
```

### Test Integrated Mode

```bash
# Terminal 1: Start combined binary
export NEO4J_PASSWORD="changeme"
export MCP_ENABLED="true"
export MCP_PORT="8081"
./watcher

# Terminal 2: Test both endpoints
curl http://localhost:8080/healthz  # Watcher health
curl http://localhost:8081/health   # MCP health
```

## Graceful Shutdown

Both modes implement graceful shutdown:

1. **Signal Handling**: Listens for SIGINT and SIGTERM
2. **Shutdown Timeout**: 30 seconds for in-flight requests
3. **Resource Cleanup**: Properly closes Neo4j connections
4. **Ordered Shutdown**: In integrated mode, MCP server shuts down after watcher

## Monitoring and Logging

Both modes provide structured logging with zap:

```json
{
  "level": "info",
  "ts": 1730278500.123,
  "msg": "MCP server enabled and started",
  "port": 8081
}
```

Set `LOG_LEVEL=debug` for detailed request/response logging.

## Security Considerations

Regardless of deployment mode:

- Always use TLS/HTTPS in production
- Implement authentication for the MCP endpoint
- Use Kubernetes NetworkPolicies to restrict access
- Monitor and audit all MCP queries
- Apply rate limiting to prevent abuse

## Migration Path

### From Integrated to Standalone

1. Deploy standalone MCP server
2. Update AI agents to point to new MCP endpoint
3. Disable MCP in watcher (`MCP_ENABLED=false`)
4. Remove MCP port from watcher service

### From Standalone to Integrated

1. Enable MCP in watcher (`MCP_ENABLED=true`)
2. Configure appropriate port (`MCP_PORT=8081`)
3. Update AI agents to point to watcher MCP endpoint
4. Scale down standalone MCP server deployment
5. Remove standalone MCP service

## Next Steps

- [MCP Server User Guide](mcp-server.md) - Detailed usage and examples
- [Integration with Claude Desktop](mcp-server.md#integration-with-ai-agents)
- [Query Examples](../reference/cypher-queries.md)
- [Graph Schema](../reference/graph-schema.md)

