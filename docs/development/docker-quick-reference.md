# Docker Build Quick Reference

## TL;DR

```bash
# Local development (fast)
make docker-build-fast        # Build both images (~30s)
make docker-build-watcher-fast
make docker-build-mcp-server-fast

# Production/CI (full rebuild)
make docker-build-all         # Build both images (~5min)
make docker-build-watcher
make docker-build-mcp-server

# Push to registry
make docker-push-all
make docker-push-watcher
make docker-push-mcp-server

# Build + push
make docker-release           # Full build + push
make docker-release-fast      # Fast build + push
```

## Makefile Targets Cheat Sheet

### Build Targets

| Target | Description | Time | When to Use |
|--------|-------------|------|-------------|
| `docker-build-all` | Build both images (full) | ~5 min | CI/CD, production |
| `docker-build-watcher` | Build watcher (full) | ~3 min | Production watcher |
| `docker-build-mcp-server` | Build mcp-server (full) | ~5 min | Production mcp-server |
| `docker-build-fast` | Build both (fast) | ~30 sec | Local development |
| `docker-build-watcher-fast` | Build watcher (fast) | ~15 sec | Quick watcher iteration |
| `docker-build-mcp-server-fast` | Build mcp-server (fast) | ~20 sec | Quick mcp-server iteration |

### Push Targets

| Target | Description | When to Use |
|--------|-------------|-------------|
| `docker-push-all` | Push both images | After build-all or build-fast |
| `docker-push-watcher` | Push watcher only | After watcher changes |
| `docker-push-mcp-server` | Push mcp-server only | After mcp-server changes |

### Combined Targets

| Target | Description | When to Use |
|--------|-------------|-------------|
| `docker-release` | Full build + push both | Production releases |
| `docker-release-fast` | Fast build + push both | Quick test deployments |

## Common Workflows

### Workflow 1: Local Development

```bash
# Make code changes
vim pkg/mcp/server.go

# Quick rebuild
make docker-build-mcp-server-fast  # ~20s

# Test locally
docker run -it --rm \
  -e NEO4J_URI=bolt://host.docker.internal:7687 \
  -e NEO4J_USERNAME=neo4j \
  -e NEO4J_PASSWORD=password \
  -p 8080:8080 \
  quay.io/aslakknutsen/kkbase-mcp-server:latest
```

### Workflow 2: Frontend Changes

```bash
# Make frontend changes
vim frontend/src/components/BlastZoneGraph.tsx

# Rebuild (includes npm build)
make docker-build-mcp-server-fast  # ~25s

# Push and deploy
make docker-push-mcp-server
kubectl rollout restart deployment/kkbase-integrated -n kkbase
```

### Workflow 3: Production Release

```bash
# Tag release
git tag v1.2.3
git push origin v1.2.3

# Full build with version tag
DOCKER_TAG=v1.2.3 make docker-release

# Also push latest
DOCKER_TAG=latest make docker-release
```

### Workflow 4: CI/CD Pipeline

```yaml
# Example GitHub Actions / GitLab CI
build:
  script:
    - make docker-build-all
    - make docker-push-all
    - kubectl apply -f deploy/deployment-integrated.yaml
```

## Environment Variables

### Docker Registry

```bash
# Change registry (default: quay.io/aslakknutsen)
export DOCKER_REGISTRY=ghcr.io/myorg
make docker-build-all
```

### Docker Tag

```bash
# Change tag (default: latest)
export DOCKER_TAG=v1.2.3
make docker-build-all

# Or inline
DOCKER_TAG=dev make docker-build-fast
```

### Image Names

```bash
# Override image names
export WATCHER_IMAGE=myregistry/watcher
export MCP_SERVER_IMAGE=myregistry/mcp-server
make docker-build-all
```

## Prerequisites

### For Full Build

- Docker installed
- No other requirements (builds in container)

### For Fast Build

- Docker installed
- **Go 1.21+** installed locally
- **Node.js 18+** for mcp-server (frontend build)
- Run `make build-watcher` or `make build-mcp-server` first

## File Sizes

### Container Images

| Image | Runtime Size | Layers |
|-------|-------------|--------|
| kkbase-watcher | ~90 MB | 5 |
| kkbase-mcp-server | ~92 MB | 5 |

### Build Context

| Build Mode | Context Size | Upload Time |
|------------|--------------|-------------|
| Full | ~50 MB | ~5s |
| Fast (watcher) | ~15 MB | ~1s |
| Fast (mcp-server) | ~17 MB | ~2s |

## Troubleshooting

### Problem: "Binary not found" in fast build

```bash
# Solution: Build binary first
make build-watcher
make docker-build-watcher-fast
```

### Problem: "Permission denied" when running

```bash
# Solution: Ensure binary is executable
chmod +x watcher
chmod +x mcp-server
```

### Problem: Images not pushed

```bash
# Solution: Login to registry first
docker login quay.io
make docker-push-all
```

### Problem: Build cache not working

```bash
# Solution: Enable BuildKit
export DOCKER_BUILDKIT=1
make docker-build-all
```

### Problem: Cross-platform build needed

```bash
# Solution: Use docker buildx
docker buildx create --use
docker buildx build --platform linux/amd64,linux/arm64 \
  -f Dockerfile.watcher \
  -t quay.io/aslakknutsen/kkbase-watcher:latest \
  --push .
```

## Docker Commands (Without Makefile)

### Full Build

```bash
# Watcher
docker build -f Dockerfile.watcher \
  -t quay.io/aslakknutsen/kkbase-watcher:latest .

# MCP Server
docker build -f Dockerfile.mcp-server \
  -t quay.io/aslakknutsen/kkbase-mcp-server:latest .
```

### Fast Build

```bash
# Build binaries first
go build -o watcher ./cmd/watcher
go build -o mcp-server ./cmd/mcp-server

# Build images
docker build -f Dockerfile.watcher.fast \
  -t quay.io/aslakknutsen/kkbase-watcher:latest .

docker build -f Dockerfile.mcp-server.fast \
  -t quay.io/aslakknutsen/kkbase-mcp-server:latest .
```

### Push

```bash
docker push quay.io/aslakknutsen/kkbase-watcher:latest
docker push quay.io/aslakknutsen/kkbase-mcp-server:latest
```

## Performance Tips

### 1. Use BuildKit

```bash
export DOCKER_BUILDKIT=1
export BUILDKIT_PROGRESS=plain
```

### 2. Leverage Layer Caching

```bash
# Pull existing image for cache
docker pull quay.io/aslakknutsen/kkbase-watcher:latest

# Build with cache
docker build --cache-from quay.io/aslakknutsen/kkbase-watcher:latest \
  -f Dockerfile.watcher \
  -t quay.io/aslakknutsen/kkbase-watcher:latest .
```

### 3. Parallel Builds

```bash
# Build both images in parallel
make docker-build-watcher & \
make docker-build-mcp-server & \
wait
```

### 4. Use Fast Build for Development

```bash
# 10x faster iteration
make docker-build-fast
```

## Security Scanning

### Trivy

```bash
# Scan for vulnerabilities
trivy image quay.io/aslakknutsen/kkbase-watcher:latest
trivy image quay.io/aslakknutsen/kkbase-mcp-server:latest
```

### Docker Scout

```bash
docker scout cves quay.io/aslakknutsen/kkbase-watcher:latest
```

### Snyk

```bash
snyk container test quay.io/aslakknutsen/kkbase-watcher:latest
```

## Multi-Architecture Builds

### Setup

```bash
docker buildx create --name multiarch --use
docker buildx inspect --bootstrap
```

### Build

```bash
# Build for AMD64 and ARM64
docker buildx build --platform linux/amd64,linux/arm64 \
  -f Dockerfile.watcher \
  -t quay.io/aslakknutsen/kkbase-watcher:latest \
  --push .

docker buildx build --platform linux/amd64,linux/arm64 \
  -f Dockerfile.mcp-server \
  -t quay.io/aslakknutsen/kkbase-mcp-server:latest \
  --push .
```

## Registry Alternatives

### GitHub Container Registry

```bash
export DOCKER_REGISTRY=ghcr.io/kagenti
docker login ghcr.io
make docker-release
```

### Docker Hub

```bash
export DOCKER_REGISTRY=docker.io/myusername
docker login docker.io
make docker-release
```

### Harbor

```bash
export DOCKER_REGISTRY=harbor.example.com/kkbase
docker login harbor.example.com
make docker-release
```

## Summary

**Quick Commands:**

```bash
# Development
make docker-build-fast           # ~30s

# Production
make docker-release              # ~5min

# Push only
make docker-push-all

# Individual images
make docker-build-watcher-fast   # ~15s
make docker-build-mcp-server-fast # ~20s
```

**Default Images:**
- `quay.io/aslakknutsen/kkbase-watcher:latest`
- `quay.io/aslakknutsen/kkbase-mcp-server:latest`

**See Also:**
- `docs/development/docker-build-modes.md` - Detailed comparison
- `make help` - Full Makefile targets list

