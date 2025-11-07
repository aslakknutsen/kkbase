# Quick Start: Full Stack Setup

## Get KKBase Running in 5 Minutes

This guide gets you from zero to a running KKBase system with agent investigation dashboard.

## Prerequisites

- Go 1.21+
- Node.js 18+
- Docker (for Neo4j)
- Kubernetes cluster access (for watcher)

## Step 1: Start Neo4j

```bash
# Start Neo4j with APOC plugin
docker run -d \
  --name neo4j-kkbase \
  -p 7474:7474 \
  -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/password123 \
  -e NEO4J_PLUGINS='["apoc"]' \
  neo4j:5.15
```

**Wait 30 seconds** for Neo4j to start, then verify:

```bash
# Should show Neo4j browser login
curl http://localhost:7474/
```

## Step 2: Clone and Build

```bash
# Clone repository
git clone <repository-url>
cd kkbase

# Build everything (backend + frontend)
make build-mcp-server

# Verify binary
ls -lh mcp-server
# Should show ~17 MB binary
```

## Step 3: Configure Environment

```bash
# Create environment file
cat > .env <<EOF
# Neo4j Connection
NEO4J_URI=bolt://localhost:7687
NEO4J_USERNAME=neo4j
NEO4J_PASSWORD=password123
NEO4J_DATABASE=neo4j

# MCP Server
MCP_PORT=8080

# Prometheus (optional)
PROMETHEUS_URL=http://prometheus.example.com:9090

# Logging
LOG_LEVEL=info
EOF
```

## Step 4: Start MCP Server

```bash
# Load environment and run
source .env
./mcp-server

# Should see:
# INFO  Connected to Neo4j successfully
# INFO  Metrics integration enabled (if Prometheus URL set)
# INFO  Agent session manager initialized
# INFO  Embedded frontend enabled
# INFO  MCP server listening
#       dashboard=http://localhost:8080/
#       mcp_endpoint=http://localhost:8080/mcp
```

## Step 5: Open Dashboard

**Open browser:** http://localhost:8080/

You should see:
- **Left sidebar**: "Active Investigations" (empty)
- **Main area**: "No Active Investigations" message

✅ Dashboard is ready!

## Step 6: Start Watcher (Optional)

The watcher populates Neo4j with Kubernetes cluster state.

```bash
# In a new terminal
cd kkbase

# Build watcher
make build-watcher

# Run with kubeconfig
export KUBECONFIG=~/.kube/config
source .env
./watcher

# Should see:
# INFO  Starting kkbase watcher
# INFO  Connected to Neo4j successfully
# INFO  Starting Kubernetes watchers...
# INFO  Watching Pods, Services, Deployments, etc.
```

After a few seconds, verify cluster data:

```bash
# Check Neo4j has data
curl -u neo4j:password123 \
  -H "Content-Type: application/json" \
  -d '{"statements":[{"statement":"MATCH (n) RETURN count(n) as count"}]}' \
  http://localhost:7474/db/neo4j/tx/commit

# Should show nodes created
```

## Step 7: Configure Cursor MCP

Add to your Cursor MCP config (`~/.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "kkbase": {
      "command": "node",
      "args": ["-e", "console.log('MCP proxy')"],
      "env": {
        "MCP_SERVER_URL": "http://localhost:8080/mcp"
      }
    }
  }
}
```

Or if using direct connection:

```json
{
  "mcpServers": {
    "kkbase": {
      "url": "http://localhost:8080/mcp",
      "transport": "sse"
    }
  }
}
```

Restart Cursor to load MCP configuration.

## Step 8: Test Agent Investigation

**In Cursor, ask the AI:**

```
Using the kkbase MCP tools, start an investigation for "Test symptom: 
checking if MCP tools work"
```

**The AI should:**
1. Call `start_agent_session`
2. Return a session ID

**In Dashboard:**
- Refresh browser (or wait 5 seconds for auto-poll)
- New session should appear in sidebar!

🎉 **Success!** You have a working KKbase system.

## Verification Checklist

- [ ] Neo4j accessible at http://localhost:7474/
- [ ] MCP server running on port 8080
- [ ] Dashboard loads at http://localhost:8080/
- [ ] Watcher populating Neo4j (if running)
- [ ] Cursor can call MCP tools
- [ ] Test session appears in dashboard

## Common Issues

### Issue: Neo4j Won't Start

```bash
# Check container
docker ps -a | grep neo4j

# Check logs
docker logs neo4j-kkbase

# Restart
docker restart neo4j-kkbase
```

### Issue: MCP Server Can't Connect to Neo4j

```bash
# Test connection manually
docker exec neo4j-kkbase cypher-shell -u neo4j -p password123 "RETURN 1"

# Should return: 1

# Check MCP server logs for error details
```

### Issue: Frontend Won't Load

```bash
# Verify frontend was embedded
cd cmd/mcp-server
ls -la frontend/dist/

# Should show:
# index.html
# assets/

# If missing, rebuild:
make clean
make build-mcp-server
```

### Issue: Dashboard Shows No Sessions

**Agent hasn't created any sessions yet.**

Test manually:

```bash
# Call MCP tool directly
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "start_agent_session",
      "arguments": {
        "symptom": "Manual test session"
      }
    }
  }'

# Should return session_id

# Refresh dashboard - session should appear
```

### Issue: Watcher Not Populating Data

```bash
# Check watcher has cluster access
kubectl get nodes

# Check watcher logs
./watcher 2>&1 | grep -i error

# Verify Neo4j has data
curl -u neo4j:password123 \
  -H "Content-Type: application/json" \
  -d '{"statements":[{"statement":"MATCH (p:Pod) RETURN count(p)"}]}' \
  http://localhost:7474/db/neo4j/tx/commit
```

## Next Steps

### Run a Real Investigation

See [Agent Investigation Workflow](./agent-investigation-workflow.md) for complete examples.

### Explore the Dashboard

- View active sessions in sidebar
- Click a session to see details
- Watch blast zone graph update in real-time
- Review findings and query history
- Check timeline for event flow

### Query Neo4j Directly

```cypher
// See all agent sessions
MATCH (s:AgentSession)
RETURN s.id, s.initial_symptom, s.status, s.created_at
ORDER BY s.created_at DESC

// See findings for a session
MATCH (s:AgentSession {id: 'session-123'})-[:HAS_FINDING]->(f:Finding)
RETURN f.type, f.severity, f.description

// See blast zone for a session
MATCH (s:AgentSession {id: 'session-123'})-[:HAS_FINDING]->(f:Finding)-[:AFFECTS]->(affected)
CALL apoc.path.subgraphAll(affected, {
  relationshipFilter: 'CALLS|MANAGES|SELECTS_PODS',
  maxLevel: 3
}) YIELD nodes, relationships
RETURN nodes, relationships
```

### Production Deployment

See [Deployment Guide](./deployment.md) for:
- Kubernetes deployment manifests
- TLS configuration
- Authentication setup
- Prometheus integration
- High availability configuration

## Development Mode

For active development:

```bash
# Terminal 1: Backend
make run-mcp-server

# Terminal 2: Frontend (with hot reload)
cd frontend
npm run dev

# Open: http://localhost:3000
# Vite proxies /mcp to backend at :8080
```

## Monitoring

### Health Check

```bash
curl http://localhost:8080/health

# Should return: {"status": "healthy"}
```

### MCP Tool List

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list"
  }'

# Should list all available tools including:
# - start_agent_session
# - query_with_session
# - update_hypothesis
# - record_finding
# - spawn_investigation
# - complete_agent_session
# - get_active_sessions
# - get_session_details
# - get_blast_zone
# - get_session_timeline
```

### Neo4j Data

```bash
# Node counts
docker exec neo4j-kkbase cypher-shell -u neo4j -p password123 \
  "MATCH (n) RETURN labels(n)[0] as type, count(n) as count"

# Should show: Pod, Service, Deployment, AgentSession, etc.
```

## Resource Usage

Typical resource consumption:

- **Neo4j**: 512 MB RAM, 1 GB disk
- **MCP Server**: 50 MB RAM, negligible CPU
- **Watcher**: 30 MB RAM, negligible CPU
- **Dashboard**: Client-side only (browser)

Total: ~600 MB RAM for full stack.

## Backup and Restore

### Backup Neo4j Data

```bash
# Stop watcher first
docker exec neo4j-kkbase neo4j-admin database dump neo4j --to-path=/backups

# Copy from container
docker cp neo4j-kkbase:/backups/neo4j.dump ./neo4j-backup.dump
```

### Restore Neo4j Data

```bash
# Stop watcher
docker exec neo4j-kkbase neo4j-admin database load neo4j --from-path=/backups
```

## Cleanup

```bash
# Stop all services
pkill -f mcp-server
pkill -f watcher

# Stop and remove Neo4j
docker stop neo4j-kkbase
docker rm neo4j-kkbase

# Clean build artifacts
make clean
```

## Summary

You now have:
- ✅ Neo4j graph database with APOC
- ✅ MCP server with embedded dashboard
- ✅ Watcher populating cluster state
- ✅ Cursor configured to use MCP tools
- ✅ Web dashboard for observing investigations

**Ready to investigate!** 🚀

