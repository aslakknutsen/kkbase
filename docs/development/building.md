# Building kkbase

Build and test kkbase services.

## Prerequisites

- **Go**: 1.21 or later
- **Docker**: For building images
- **Make**: For build automation
- **kubectl**: For deployment testing
- **Neo4j**: Local or remote instance

## Local Development

### Setup

```bash
# Clone repository
git clone https://github.com/aslakknutsen/kkbase.git
cd kkbase

# Install dependencies
go mod download
```

### Build Binaries

```bash
# Build all services
make build

# Build individually
make build-watcher
make build-mcp-server
make build-agent
```

### Run Services

```bash
# Set environment
export NEO4J_URI=bolt://localhost:7687
export NEO4J_USERNAME=neo4j
export NEO4J_PASSWORD=changeme

# Run watcher
./watcher

# Run MCP server (different terminal)
export MCP_PORT=8080
./mcp-server

# Run agent (different terminal)
export WEBHOOK_PORT=9090
export LLM_API_KEY=your-key
./agent
```

## Docker Builds

### Standard Builds

```bash
# Watcher
docker build -f build/Containerfile.watcher -t kkbase-watcher:latest .

# MCP Server
docker build -f build/Containerfile.mcp-server -t kkbase-mcp-server:latest .

# Agent
docker build -f build/Containerfile.agent -t kkbase-agent:latest .
```

### Dev Builds (Development)

For faster iteration:

```bash
docker build -f build/Containerfile.watcher.dev -t kkbase-watcher:dev .
```

See [Docker Build Modes](docker-build-modes.md) for details.

## Frontend Build

```bash
cd frontend

# Install dependencies
npm install

# Development
npm run dev

# Production build
npm run build

# Output: frontend/dist/
# Copy to: cmd/mcp-server/frontend/dist/
```

## Testing

### Unit Tests

```bash
# All packages
go test ./pkg/...

# Specific package
go test ./pkg/mcp/

# With coverage
go test -cover ./pkg/...
```

### Integration Tests

```bash
# Requires Neo4j
go test -tags=integration ./pkg/observability/
```

### MCP Server Tests

```bash
go test ./pkg/mcp/ -v
```

## Code Quality

### Format

```bash
gofmt -w .
```

### Lint

```bash
golangci-lint run
```

### Vet

```bash
go vet ./...
```

## Makefile Targets

```bash
make build          # Build all binaries
make build-watcher  # Build watcher
make build-mcp-server # Build MCP server
make build-agent    # Build agent
make test           # Run tests
make docker         # Build all Docker images
make clean          # Clean build artifacts
```

## See Also

- [Development README](README.md)
- [Deep Dive](deep-dive.md)
- [Extending](extending.md)
- [Docker Build Modes](docker-build-modes.md)

