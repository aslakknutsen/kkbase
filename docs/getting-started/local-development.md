# Local Development Guide

Set up a local development environment for contributing to kkbase or developing custom features.

## What You'll Get

- Local Neo4j in Docker
- Hot-reload backend development
- Hot-reload frontend development (Vite)
- Quick iteration cycle

## Prerequisites

- Go 1.21+
- Node.js 18+
- Docker
- kubectl (for watcher)
- Access to a Kubernetes cluster

## Quick Setup (5 minutes)

### Step 1: Clone Repository

```bash
git clone https://github.com/aslakknutsen/kkbase.git
cd kkbase
```

### Step 2: Start Neo4j

```bash
docker run -d \
  --name neo4j-kkbase \
  -p 7474:7474 \
  -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/password123 \
  -e NEO4J_PLUGINS='["apoc"]' \
  neo4j:5.15
```

Wait 30 seconds for Neo4j to start, then verify:

```bash
curl http://localhost:7474/
# Should return Neo4j browser HTML
```

### Step 3: Create Environment File

```bash
cat > .env <<EOF
# Neo4j Connection
NEO4J_URI=bolt://localhost:7687
NEO4J_USERNAME=neo4j
NEO4J_PASSWORD=password123
NEO4J_DATABASE=neo4j

# Watcher Configuration
KUBECONFIG=$HOME/.kube/config
NAMESPACE=""
RESYNC_PERIOD=30s
LOG_LEVEL=debug

# MCP Server
MCP_PORT=8080

# Prometheus (optional)
PROMETHEUS_URL=http://localhost:9090

# Logging
LOG_LEVEL=debug
EOF
```

### Step 4: Build Everything

```bash
# Build all binaries
make all

# Or build individually:
# make build-watcher
# make build-mcp-server
# make build-agent
```

## Development Workflows

### Workflow 1: Backend Development (MCP Server)

Terminal 1 - Run MCP server with hot reload:

```bash
# Load environment
source .env

# Run with air (hot reload)
air -c .air-mcp-server.toml

# Or without hot reload:
go run ./cmd/mcp-server
```

Terminal 2 - Run frontend with Vite dev server:

```bash
cd frontend
npm install
npm run dev
```

Access:
- **Dashboard**: http://localhost:5173/ (Vite dev server with hot module reload)
- **MCP Endpoint**: http://localhost:8080/mcp (proxied through Vite)

Changes to:
- Backend Go files → Auto-rebuild and restart
- Frontend files → Instant hot module reload

### Workflow 2: Watcher Development

Terminal 1 - Run watcher:

```bash
source .env
go run ./cmd/watcher
```

Terminal 2 - Watch logs:

```bash
# See what's being synced
kubectl get pods --watch
```

Terminal 3 - Query Neo4j:

```bash
# Check if resources are synced
docker exec neo4j-kkbase cypher-shell -u neo4j -p password123 \
  "MATCH (n) RETURN labels(n)[0] as type, count(n) as count"
```

Changes to watcher code:
1. Stop watcher (Ctrl+C)
2. Restart: `go run ./cmd/watcher`

### Workflow 3: Agent Development

Terminal 1 - Run MCP server:

```bash
source .env
go run ./cmd/mcp-server
```

Terminal 2 - Run agent:

```bash
source .env
go run ./cmd/agent
```

Test agent with webhook:

```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "alert": "Service down: orders-api",
    "severity": "critical"
  }'
```

### Workflow 4: Full Stack Development

Use Makefile targets for common tasks:

```bash
# Run watcher
make run-watcher

# Run MCP server
make run-mcp-server

# Run agent
make run-agent

# Run tests
make test

# Run linter
make lint

# Build Docker images (fast mode)
make docker-build-fast
```

## Make Targets Reference

```bash
# Build
make build-watcher          # Build watcher binary
make build-mcp-server       # Build MCP server binary
make build-agent            # Build agent binary
make all                    # Build all binaries

# Run
make run-watcher           # Run watcher locally
make run-mcp-server        # Run MCP server locally
make run-agent             # Run agent locally

# Test
make test                  # Run all tests
make test-watcher          # Run watcher tests
make test-mcp              # Run MCP server tests
make test-integration      # Run integration tests

# Docker
make docker-build-fast     # Build Docker images (fast mode)
make docker-build          # Build Docker images (full mode)

# Clean
make clean                 # Remove build artifacts
```

## Testing

### Unit Tests

```bash
# Run all tests
go test ./...

# Run specific package
go test ./pkg/mcp/...

# Run with coverage
go test -cover ./...

# Run with verbose output
go test -v ./pkg/mcp/...
```

### Integration Tests

```bash
# Requires Neo4j and Kubernetes cluster
make test-integration
```

### Frontend Tests

```bash
cd frontend
npm test
```

## Debugging

### Debug MCP Server with Delve

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Run with debugger
dlv debug ./cmd/mcp-server
```

In delve:
```
(dlv) break main.main
(dlv) continue
(dlv) step
```

### Debug with VS Code

Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug MCP Server",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/mcp-server",
      "env": {
        "NEO4J_URI": "bolt://localhost:7687",
        "NEO4J_USERNAME": "neo4j",
        "NEO4J_PASSWORD": "password123"
      }
    },
    {
      "name": "Debug Watcher",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/watcher",
      "env": {
        "NEO4J_URI": "bolt://localhost:7687",
        "NEO4J_USERNAME": "neo4j",
        "NEO4J_PASSWORD": "password123",
        "KUBECONFIG": "${env:HOME}/.kube/config"
      }
    }
  ]
}
```

### Inspect Neo4j Data

Neo4j Browser:
```bash
# Open in browser
open http://localhost:7474
```

Cypher Shell:
```bash
docker exec -it neo4j-kkbase cypher-shell -u neo4j -p password123
```

Common debugging queries:
```cypher
// See all node types
MATCH (n) RETURN DISTINCT labels(n), count(*)

// See all relationship types  
MATCH ()-[r]->() RETURN DISTINCT type(r), count(*)

// Find placeholder nodes
MATCH (n {placeholder: true}) RETURN n LIMIT 10

// Check agent sessions
MATCH (s:AgentSession) RETURN s ORDER BY s.created_at DESC LIMIT 5
```

## Frontend Development

### Setup

```bash
cd frontend
npm install
```

### Development Server

```bash
npm run dev
# Opens http://localhost:5173
```

Features:
- Hot module reload
- TypeScript type checking
- Vite dev server
- Proxy to backend MCP server

### Build for Production

```bash
npm run build
# Output: frontend/dist/
```

### Preview Production Build

```bash
npm run preview
# Opens http://localhost:4173
```

### Type Checking

```bash
npm run type-check
```

### Linting

```bash
npm run lint
```

## Common Development Tasks

### Adding a New MCP Tool

1. Define types in `pkg/mcp/types.go`
2. Implement handler in `pkg/mcp/tools.go`
3. Register in `pkg/mcp/server.go`
4. Add tests in `pkg/mcp/tools_test.go`
5. Update documentation

See [Extending MCP Server](../development/extending.md)

### Adding a Custom Handler

1. Create handler in `pkg/watchers/handlers/extensions/`
2. Implement interface methods
3. Register in appropriate register.go
4. Add to main.go
5. Add tests

See [Custom Handlers Guide](../services/watcher/custom-handlers.md)

### Adding Frontend Components

1. Create component in `frontend/src/components/`
2. Import in parent component
3. Use TypeScript for type safety
4. Follow existing patterns

## Performance Profiling

### CPU Profiling

```bash
# Run with CPU profiling
go run ./cmd/mcp-server -cpuprofile=cpu.prof

# Analyze profile
go tool pprof cpu.prof
```

### Memory Profiling

```bash
# Run with memory profiling
go run ./cmd/mcp-server -memprofile=mem.prof

# Analyze profile
go tool pprof mem.prof
```

### HTTP Profiling

Add to code:
```go
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

Access profiles:
- http://localhost:6060/debug/pprof/
- http://localhost:6060/debug/pprof/heap
- http://localhost:6060/debug/pprof/goroutine

## Troubleshooting

### Neo4j Connection Failed

```bash
# Check Neo4j is running
docker ps | grep neo4j

# Check logs
docker logs neo4j-kkbase

# Restart
docker restart neo4j-kkbase
```

### Watcher Can't Connect to Cluster

```bash
# Verify kubeconfig
kubectl cluster-info

# Check KUBECONFIG env var
echo $KUBECONFIG

# Test connection
kubectl get nodes
```

### Frontend Won't Start

```bash
# Clear node_modules and reinstall
cd frontend
rm -rf node_modules package-lock.json
npm install

# Check Node version
node --version  # Should be 18+
```

### Port Already in Use

```bash
# Find process using port 8080
lsof -i :8080

# Kill process
kill -9 <PID>

# Or use different port
export MCP_PORT=8081
```

## Best Practices

### Code Style

- Follow Go conventions
- Use `gofmt` for formatting
- Run `golangci-lint` before committing
- Write tests for new features
- Update documentation

### Commit Messages

Follow conventional commits:
```
feat: add new MCP tool for blast zone analysis
fix: correct pod relationship extraction
docs: update installation guide
test: add integration tests for agent sessions
```

### Branch Strategy

- Create feature branches from `main`
- Name branches: `feature/my-feature` or `fix/issue-123`
- Keep commits focused and atomic
- Rebase before merging

### Testing

- Write unit tests for new code
- Add integration tests for complex features
- Test error paths
- Verify against real cluster when possible

## Resources

- [System Architecture](../ARCHITECTURE.md)
- [Development Deep Dive](../development/deep-dive.md)
- [Custom Handlers](../services/watcher/custom-handlers.md)
- [Extending MCP](../development/extending.md)
- [Testing Guide](../development/testing.md)

## Clean Up

```bash
# Stop Neo4j
docker stop neo4j-kkbase
docker rm neo4j-kkbase

# Clean build artifacts
make clean

# Remove binaries
rm -f watcher mcp-server agent
```

## Summary

You now have:
- ✅ Local development environment
- ✅ Hot-reload for fast iteration
- ✅ Debugging tools configured
- ✅ Testing framework ready
- ✅ Ready to contribute

**Start coding!** See [Architecture Deep Dive](../development/deep-dive.md) for implementation details.

