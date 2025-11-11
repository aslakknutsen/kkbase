# Docker Build Modes

## Overview

KKBase provides two Docker build modes optimized for different use cases:

1. **Full Build** - Complete rebuild in container (CI/CD, production releases)
2. **Dev Build** - Uses pre-built binaries from local machine (local development)

## Build Modes Comparison

| Feature | Full Build | Dev Build |
|---------|-----------|------------|
| **Containerfiles** | `build/Containerfile.watcher`<br/>`build/Containerfile.mcp-server` | `build/Containerfile.watcher.dev`<br/>`build/Containerfile.mcp-server.dev` |
| **Build Time** | ~3-5 minutes | ~10-20 seconds |
| **Go Compilation** | Inside container | On local machine |
| **Use Case** | CI/CD, production | Local development iteration |
| **Reproducibility** | High (hermetic) | Medium (depends on local Go version) |
| **Makefile Targets** | `make docker-build-all` | `make docker-build-dev` |

## Full Build (Production)

### How It Works

```
┌─────────────────────────────────────────────────┐
│ Stage 1: Builder (UBI9 + Go Toolset 1.21)      │
│                                                 │
│  1. COPY go.mod, go.sum                        │
│  2. go mod download                             │
│  3. COPY all source code                        │
│  4. go build -o binary                          │
└────────────┬────────────────────────────────────┘
             │ Copy binary
             ↓
┌─────────────────────────────────────────────────┐
│ Stage 2: Runtime (UBI9 Minimal)                │
│                                                 │
│  - Install ca-certificates                      │
│  - Create non-root user                         │
│  - Set permissions                              │
│  - ENTRYPOINT [binary]                          │
└─────────────────────────────────────────────────┘
```

### When to Use

✅ **CI/CD pipelines** - Hermetic, reproducible builds  
✅ **Production releases** - Consistent build environment  
✅ **Multi-architecture builds** - BuildKit cross-compilation  
✅ **First-time builds** - No local dependencies needed  

### Commands

```bash
# Build both images (watcher + mcp-server)
make docker-build-all

# Build individual images
make docker-build-watcher
make docker-build-mcp-server

# Build and push to registry
make docker-release
```

### Build Time

**First build** (cold cache):
- Watcher: ~2-3 minutes
- MCP Server: ~3-5 minutes (includes frontend)

**Subsequent builds** (warm cache):
- Watcher: ~30-60 seconds
- MCP Server: ~1-2 minutes

### Advantages

✅ No local Go installation required  
✅ Reproducible across all machines  
✅ Same environment as production  
✅ Docker layer caching optimized  

### Disadvantages

❌ Slower iteration cycle  
❌ Large builder images (~850 MB)  
❌ Network overhead (downloading dependencies)  

## Dev Build (Local Development)

### How It Works

```
Local Machine:
  1. make build-watcher       # Build Go binary locally
  2. make build-mcp-server    # Build binary + frontend locally

Docker Build:
┌─────────────────────────────────────────────────┐
│ Single Stage: Runtime (UBI9 Minimal)           │
│                                                 │
│  1. Install ca-certificates                     │
│  2. COPY pre-built binary from context         │
│  3. Create non-root user                        │
│  4. Set permissions                             │
│  5. ENTRYPOINT [binary]                         │
└─────────────────────────────────────────────────┘
```

### When to Use

✅ **Local development** - Fast iteration cycle  
✅ **Testing changes** - Quick rebuild after code edits  
✅ **Debugging** - Rapid container updates  
✅ **Frontend changes** - Quick mcp-server rebuilds  

### Commands

```bash
# Build both images using pre-built binaries
make docker-build-dev

# Build individual images
make docker-build-watcher-dev
make docker-build-mcp-server-dev

# Build locally, then build Docker images
make all docker-build-dev
```

### Build Time

**Complete workflow**:
- Local Go build: ~10-30 seconds
- Docker build: ~10-20 seconds
- **Total: ~20-50 seconds**

### Advantages

✅ **10x faster** than full build  
✅ Uses local Go toolchain (no download)  
✅ Leverages Go build cache  
✅ Quick iteration for development  

### Disadvantages

❌ Requires local Go 1.21+ installation  
❌ Requires `make build-*` before Docker build  
❌ Less reproducible (depends on local environment)  
❌ **Not suitable for CI/CD or production**  

## Workflow Examples

### Example 1: First Time Setup

```bash
# Use full build (no local dependencies)
make docker-build-all
make docker-push-all

# Deploy to Kubernetes
kubectl apply -f deploy/deployment-integrated.yaml
```

### Example 2: Local Development Iteration

```bash
# Initial build
make all  # Build watcher + mcp-server binaries locally

# Make code changes...
vim pkg/mcp/server.go

# Quick rebuild
make build-mcp-server           # Rebuild binary (~10s)
make docker-build-mcp-server-dev  # Rebuild image (~15s)

# Test locally
docker run -it --rm kkbase-mcp-server:latest

# Push to test cluster (optional)
make docker-push-mcp-server
kubectl rollout restart deployment/kkbase-integrated
```

### Example 3: Frontend Development

```bash
# Rebuild frontend + binary + image
make build-mcp-server           # Includes npm build
make docker-build-mcp-server-dev

# Or in one command
make docker-build-mcp-server-dev  # Automatically runs build-mcp-server
```

### Example 4: CI/CD Pipeline

```yaml
# .gitlab-ci.yml or .github/workflows/build.yml
build:
  script:
    # Always use full build in CI/CD
    - make docker-build-all
    - make docker-push-all
```

## Choosing the Right Mode

### Use Full Build If:

- Building in CI/CD pipeline
- Creating production release
- Don't have Go installed locally
- Need reproducible builds
- Building on different architecture

### Use Dev Build If:

- Developing locally
- Iterating on code changes
- Testing frontend updates
- Need quick feedback loop
- Have Go 1.21+ installed

## Performance Comparison

### Full Build Timeline

```
[0s - 30s]   Download UBI9 base images
[30s - 60s]  go mod download
[60s - 150s] go build (watcher)
[150s - 300s] go build (mcp-server + frontend)
[300s - 320s] Create runtime image

Total: ~5 minutes
```

### Dev Build Timeline

```
[0s - 10s]   Local go build (watcher)
[10s - 40s]  Local go build + npm build (mcp-server)
[40s - 50s]  Docker build watcher (copy binary)
[50s - 60s]  Docker build mcp-server (copy binary)

Total: ~1 minute
```

**Speed improvement: 5x - 10x faster**

## Containerfile Comparison

### Full Build (build/Containerfile.watcher)

```dockerfile
# Stage 1: Builder
FROM registry.access.redhat.com/ubi9/go-toolset:1.21 AS builder
USER root
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o watcher ./cmd/watcher

# Stage 2: Runtime
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
RUN microdnf install -y ca-certificates && microdnf clean all
WORKDIR /app
COPY --from=builder /build/watcher .  # Copy from builder stage
RUN useradd -u 1000 -r -s /sbin/nologin kkbase
RUN chown -R kkbase:kkbase /app && chmod +x /app/watcher
USER 1000
ENTRYPOINT ["./watcher"]
```

### Dev Build (build/Containerfile.watcher.dev)

```dockerfile
# Single stage: Runtime only
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
RUN microdnf install -y ca-certificates && microdnf clean all
WORKDIR /app
COPY out/watcher .  # Copy pre-built binary from local context
RUN useradd -u 1000 -r -s /sbin/nologin kkbase
RUN chown -R kkbase:kkbase /app && chmod +x /app/watcher
USER 1000
ENTRYPOINT ["./watcher"]
```

**Key difference**: Dev build skips entire Go build stage.

## Best Practices

### For Local Development

1. **Use dev build by default**:
   ```bash
   alias ddev='make docker-build-dev'
   ```

2. **Keep binaries fresh**:
   ```bash
   # Rebuild binaries before Docker build
   make all && make docker-build-dev
   ```

3. **Use BuildKit for caching**:
   ```bash
   export DOCKER_BUILDKIT=1
   ```

### For CI/CD

1. **Always use full build**:
   ```bash
   make docker-build-all
   ```

2. **Tag with commit SHA**:
   ```bash
   DOCKER_TAG=$(git rev-parse --short HEAD) make docker-build-all
   ```

3. **Enable layer caching**:
   ```bash
   docker build --cache-from kkbase-watcher:latest ...
   ```

### For Production Releases

1. **Use full build with explicit tags**:
   ```bash
   DOCKER_TAG=v1.2.3 make docker-build-all
   DOCKER_TAG=v1.2.3 make docker-push-all
   ```

2. **Sign images** (optional):
   ```bash
   cosign sign quay.io/aslakknutsen/kkbase-watcher:v1.2.3
   ```

3. **Scan for vulnerabilities**:
   ```bash
   trivy image kkbase-watcher:v1.2.3
   ```

## Troubleshooting

### Dev Build: Binary not found

```
Error: COPY failed: file not found in build context: out/watcher
```

**Solution**: Build binary first:
```bash
make build-watcher
make docker-build-watcher-dev
```

### Dev Build: Binary incompatible with container

```
Error: exec format error
```

**Cause**: Binary built for wrong architecture (e.g., macOS ARM vs Linux AMD64)

**Solution**: Build for Linux:
```bash
GOOS=linux GOARCH=amd64 go build -o out/watcher ./cmd/watcher
make docker-build-watcher-dev
```

Or use full build (cross-compiles in container):
```bash
make docker-build-watcher
```

### Full Build: Network timeout

```
Error: failed to fetch module ...
```

**Solution**: Increase Docker timeout or use local proxy:
```bash
export DOCKER_BUILD_TIMEOUT=600
```

### Build cache not working

**Solution**: Enable BuildKit:
```bash
export DOCKER_BUILDKIT=1
export BUILDKIT_PROGRESS=plain
```

## Summary

| Scenario | Recommended Mode | Command |
|----------|------------------|---------|
| **Local development** | Dev | `make docker-build-dev` |
| **CI/CD pipeline** | Full | `make docker-build-all` |
| **Production release** | Full | `make docker-release` |
| **First-time build** | Full | `make docker-build-all` |
| **Quick iteration** | Dev | `make docker-build-dev` |
| **Testing changes** | Dev | `make docker-build-dev` |
| **Cross-platform** | Full | `docker buildx build ...` |

**Key Takeaway**: Use **dev build for speed**, **full build for reproducibility**.

