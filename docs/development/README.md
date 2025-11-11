# Development Guide

Technical documentation for kkbase developers.

## What's in This Section?

| Document | Purpose | Audience |
|----------|---------|----------|
| [Deep Dive](deep-dive.md) | Detailed architecture and internals | Contributors |
| [Building](building.md) | Build, test, and development workflow | Contributors |
| [Extending](extending.md) | Add handlers, extend functionality | Developers |

## Quick Start for Developers

### Prerequisites

- Go 1.21+
- Docker
- kubectl
- Neo4j (local or cluster)

### Clone and Build

```bash
git clone https://github.com/aslakknutsen/kkbase.git
cd kkbase

# Build all services
make build

# Or build individually
make build-watcher
make build-mcp-server
make build-agent
```

### Run Locally

```bash
# Set environment variables
export NEO4J_URI=bolt://localhost:7687
export NEO4J_USERNAME=neo4j
export NEO4J_PASSWORD=changeme

# Run watcher
./watcher

# Run MCP server (different terminal)
./mcp-server

# Run agent (different terminal)
./agent
```

### Run Tests

```bash
make test
```

## Project Structure

```
kkbase/
├── cmd/              # Entry points
│   ├── watcher/
│   ├── mcp-server/
│   └── agent/
├── pkg/              # Core packages
│   ├── watchers/     # Kubernetes watchers
│   ├── mcp/          # MCP server implementation
│   ├── agent/        # Agent logic
│   ├── graph/        # Neo4j interface
│   ├── models/       # Shared types
│   └── observability/# Metrics and tracing
├── docs/             # Documentation
├── deploy/           # Kubernetes manifests
└── frontend/         # React dashboard
```

## Key Concepts

### Handler Registry Pattern

Handlers watch specific Kubernetes resource types and convert them to graph nodes/edges.

See [Deep Dive](deep-dive.md#handler-registry-pattern)

### MCP Protocol

Model Context Protocol for AI tool integration.

See [Deep Dive](deep-dive.md#mcp-server-architecture)

### Agent Session Management

Track investigation sessions in Neo4j.

See [Deep Dive](deep-dive.md#agent-session-tracking)

## Common Development Tasks

### Add New Handler

1. Create handler file in `pkg/watchers/handlers/`
2. Implement `Handler` interface
3. Register in `pkg/watchers/handlers/registry.go`
4. Add tests

See [Extending Guide](extending.md#adding-handlers)

### Add MCP Tool

1. Define tool in `pkg/mcp/tools.go`
2. Implement handler
3. Add to tool list
4. Update documentation

See [Extending Guide](extending.md#adding-mcp-tools)

### Modify Graph Schema

1. Update handlers to add nodes/relationships
2. Update schema documentation
3. Add migration if needed

See [Extending Guide](extending.md#modifying-schema)

## Build Modes

### Development Builds

Fast compilation:

```bash
make build-watcher
make build-mcp-server
make build-agent
```

### Docker Builds

```bash
# Standard builds
docker build -f build/Containerfile.watcher -t kkbase-watcher .

# Dev builds (for iteration)
docker build -f build/Containerfile.watcher.dev -t kkbase-watcher .
```

See [Building Guide](building.md#docker-builds)

## Testing

### Unit Tests

```bash
go test ./pkg/...
```

### Integration Tests

```bash
go test -tags=integration ./pkg/observability/
```

### End-to-End Tests

```bash
# Deploy to kind cluster
kind create cluster
kubectl apply -f deploy/
# Run tests
```

See [Building Guide](building.md#testing)

## Contributing

### Code Style

- Follow standard Go conventions
- Use `gofmt`
- Write tests for new features
- Update documentation

### PR Process

1. Fork repository
2. Create feature branch
3. Make changes with tests
4. Update documentation
5. Submit PR

## Resources

### Internal Docs

- [Deep Dive Architecture](deep-dive.md)
- [Building Guide](building.md)
- [Extending Guide](extending.md)

### External Resources

- [Neo4j Go Driver](https://github.com/neo4j/neo4j-go-driver)
- [client-go](https://github.com/kubernetes/client-go)
- [MCP Protocol](https://modelcontextprotocol.io/)

## See Also

- [Getting Started](../getting-started/) - User documentation
- [Service Documentation](../services/) - Service-specific details
- [Reference](../reference/) - Technical reference

