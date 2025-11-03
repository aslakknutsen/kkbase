.PHONY: build build-watcher build-mcp-server build-frontend all run run-mcp-server test clean clean-frontend \
	docker-build-all docker-build-watcher docker-build-mcp-server \
	docker-build-fast docker-build-watcher-fast docker-build-mcp-server-fast \
	docker-push-all docker-push-watcher docker-push-mcp-server \
	docker-release docker-release-fast \
	deploy deploy-mcp-standalone deploy-integrated deploy-all undeploy fmt vet deps logs help

# Variables
BINARY_NAME=watcher
MCP_BINARY_NAME=mcp-server
DOCKER_REGISTRY=quay.io/aslakknutsen
DOCKER_TAG=latest
FRONTEND_DIR=frontend

# Docker image names
WATCHER_IMAGE=$(DOCKER_REGISTRY)/kkbase-watcher
MCP_SERVER_IMAGE=$(DOCKER_REGISTRY)/kkbase-mcp-server

# Build all binaries (includes frontend)
all: build-watcher build-mcp-server

# Build the watcher application
build: build-watcher

build-watcher:
	go build -o $(BINARY_NAME) ./cmd/watcher

# Build the MCP server (includes frontend)
build-mcp-server: build-frontend
	@echo "Copying frontend build to cmd/mcp-server..."
	rm -rf cmd/mcp-server/frontend
	mkdir -p cmd/mcp-server/frontend
	cp -r $(FRONTEND_DIR)/dist cmd/mcp-server/frontend/
	go build -o $(MCP_BINARY_NAME) ./cmd/mcp-server

# Build the frontend
build-frontend:
	@echo "Building frontend..."
	cd $(FRONTEND_DIR) && npm install && npm run build

# Run the watcher application locally
run:
	go run ./cmd/watcher

# Run the MCP server locally
run-mcp-server:
	go run ./cmd/mcp-server

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean: clean-frontend
	rm -f $(BINARY_NAME) $(MCP_BINARY_NAME)
	rm -rf cmd/mcp-server/frontend
	go clean

# Clean frontend build
clean-frontend:
	rm -rf $(FRONTEND_DIR)/dist
	rm -rf $(FRONTEND_DIR)/node_modules

# Docker: Build all images (full rebuild in container)
docker-build-all: docker-build-watcher docker-build-mcp-server

# Docker: Build watcher image (full rebuild)
docker-build-watcher:
	@echo "Building watcher image (full rebuild)..."
	docker build -f Dockerfile.watcher -t $(WATCHER_IMAGE):$(DOCKER_TAG) .

# Docker: Build MCP server image (full rebuild)
docker-build-mcp-server:
	@echo "Building MCP server image (full rebuild)..."
	docker build -f Dockerfile.mcp-server -t $(MCP_SERVER_IMAGE):$(DOCKER_TAG) .

# Docker: Fast build - uses pre-built binaries (LOCAL DEV ONLY)
docker-build-fast: docker-build-watcher-fast docker-build-mcp-server-fast

# Docker: Fast build watcher (uses pre-built binary)
docker-build-watcher-fast: build-watcher
	@echo "Building watcher image (fast - using pre-built binary)..."
	docker build -f Dockerfile.watcher.fast -t $(WATCHER_IMAGE):$(DOCKER_TAG) .

# Docker: Fast build MCP server (uses pre-built binary)
docker-build-mcp-server-fast: build-mcp-server
	@echo "Building MCP server image (fast - using pre-built binary)..."
	docker build -f Dockerfile.mcp-server.fast -t $(MCP_SERVER_IMAGE):$(DOCKER_TAG) .

# Docker: Push all images
docker-push-all: docker-push-watcher docker-push-mcp-server

# Docker: Push watcher image
docker-push-watcher:
	@echo "Pushing watcher image..."
	docker push $(WATCHER_IMAGE):$(DOCKER_TAG)

# Docker: Push MCP server image
docker-push-mcp-server:
	@echo "Pushing MCP server image..."
	docker push $(MCP_SERVER_IMAGE):$(DOCKER_TAG)

# Docker: Build and push all (full rebuild)
docker-release: docker-build-all docker-push-all

# Docker: Build and push all (fast - for local testing)
docker-release-fast: docker-build-fast docker-push-all

# Deploy to Kubernetes (watcher only)
deploy:
	kubectl apply -f deploy/rbac.yaml
	kubectl apply -f deploy/configmap.yaml
	kubectl apply -f deploy/secret.yaml
	kubectl apply -f deploy/deployment.yaml
	kubectl apply -f deploy/service.yaml

# Deploy standalone MCP server
deploy-mcp-standalone:
	kubectl apply -f deploy/rbac.yaml
	kubectl apply -f deploy/configmap.yaml
	kubectl apply -f deploy/secret.yaml
	kubectl apply -f deploy/mcp-server-deployment.yaml
	kubectl apply -f deploy/mcp-server-service.yaml

# Deploy integrated mode (watcher + MCP in one pod)
deploy-integrated:
	kubectl apply -f deploy/rbac.yaml
	kubectl apply -f deploy/configmap.yaml
	kubectl apply -f deploy/secret.yaml
	kubectl apply -f deploy/deployment-integrated.yaml
	kubectl apply -f deploy/service-integrated.yaml

# Deploy complete stack (watcher + standalone MCP)
deploy-all:
	kubectl apply -f deploy/rbac.yaml
	kubectl apply -f deploy/configmap.yaml
	kubectl apply -f deploy/secret.yaml
	kubectl apply -f deploy/deployment.yaml
	kubectl apply -f deploy/service.yaml
	kubectl apply -f deploy/mcp-server-deployment.yaml
	kubectl apply -f deploy/mcp-server-service.yaml

# Remove from Kubernetes
undeploy:
	kubectl delete -f deploy/deployment.yaml --ignore-not-found
	kubectl delete -f deploy/deployment-integrated.yaml --ignore-not-found
	kubectl delete -f deploy/mcp-server-deployment.yaml --ignore-not-found
	kubectl delete -f deploy/service.yaml --ignore-not-found
	kubectl delete -f deploy/service-integrated.yaml --ignore-not-found
	kubectl delete -f deploy/mcp-server-service.yaml --ignore-not-found
	kubectl delete -f deploy/secret.yaml --ignore-not-found
	kubectl delete -f deploy/configmap.yaml --ignore-not-found
	kubectl delete -f deploy/rbac.yaml --ignore-not-found

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Download dependencies
deps:
	go mod download
	go mod tidy

# Show logs
logs:
	kubectl logs -f deployment/kkbase-watcher

# Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build:"
	@echo "  all                        - Build all binaries (watcher + mcp-server)"
	@echo "  build                      - Build the watcher binary (default)"
	@echo "  build-watcher              - Build the watcher binary"
	@echo "  build-mcp-server           - Build the MCP server binary (includes frontend)"
	@echo "  build-frontend             - Build the React frontend only"
	@echo ""
	@echo "Run Locally:"
	@echo "  run                        - Run watcher locally"
	@echo "  run-mcp-server             - Run MCP server locally"
	@echo ""
	@echo "Development:"
	@echo "  test                       - Run tests"
	@echo "  clean                      - Clean all build artifacts"
	@echo "  clean-frontend             - Clean frontend build artifacts"
	@echo "  fmt                        - Format code"
	@echo "  vet                        - Run go vet"
	@echo "  deps                       - Download and tidy dependencies"
	@echo ""
	@echo "Docker (Full Build - for CI/CD):"
	@echo "  docker-build-all           - Build both images (full rebuild in container)"
	@echo "  docker-build-watcher       - Build watcher image (full rebuild)"
	@echo "  docker-build-mcp-server    - Build MCP server image (full rebuild)"
	@echo ""
	@echo "Docker (Fast Build - for local dev):"
	@echo "  docker-build-fast          - Build both images using pre-built binaries"
	@echo "  docker-build-watcher-fast  - Build watcher image using pre-built binary"
	@echo "  docker-build-mcp-server-fast - Build MCP server using pre-built binary"
	@echo ""
	@echo "Docker (Push):"
	@echo "  docker-push-all            - Push both images to registry"
	@echo "  docker-push-watcher        - Push watcher image"
	@echo "  docker-push-mcp-server     - Push MCP server image"
	@echo ""
	@echo "Docker (Combined):"
	@echo "  docker-release             - Build (full) and push all images"
	@echo "  docker-release-fast        - Build (fast) and push all images"
	@echo ""
	@echo "Kubernetes Deployment:"
	@echo "  deploy                     - Deploy watcher only"
	@echo "  deploy-mcp-standalone      - Deploy standalone MCP server"
	@echo "  deploy-integrated          - Deploy watcher + MCP integrated (RECOMMENDED)"
	@echo "  deploy-all                 - Deploy watcher + standalone MCP"
	@echo "  undeploy                   - Remove all deployments"
	@echo "  logs                       - Show application logs"

