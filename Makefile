.PHONY: build build-watcher build-mcp-server all run run-mcp-server test clean docker-build docker-push deploy deploy-mcp-standalone deploy-integrated deploy-all undeploy fmt vet deps logs help

# Variables
BINARY_NAME=watcher
MCP_BINARY_NAME=mcp-server
DOCKER_IMAGE=quay.io/aslakknutsen/kkbase-watcher
DOCKER_TAG=latest

# Build all binaries
all: build-watcher build-mcp-server

# Build the watcher application
build: build-watcher

build-watcher:
	go build -o $(BINARY_NAME) ./cmd/watcher

# Build the MCP server
build-mcp-server:
	go build -o $(MCP_BINARY_NAME) ./cmd/mcp-server

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
clean:
	rm -f $(BINARY_NAME) $(MCP_BINARY_NAME)
	go clean

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Push Docker image
docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

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
	@echo "  all                   - Build all binaries (watcher + mcp-server)"
	@echo "  build                 - Build the watcher binary (default)"
	@echo "  build-watcher         - Build the watcher binary"
	@echo "  build-mcp-server      - Build the MCP server binary"
	@echo ""
	@echo "Run Locally:"
	@echo "  run                   - Run watcher locally"
	@echo "  run-mcp-server        - Run MCP server locally"
	@echo ""
	@echo "Development:"
	@echo "  test                  - Run tests"
	@echo "  clean                 - Clean build artifacts"
	@echo "  fmt                   - Format code"
	@echo "  vet                   - Run go vet"
	@echo "  deps                  - Download and tidy dependencies"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build          - Build Docker image"
	@echo "  docker-push           - Push Docker image"
	@echo ""
	@echo "Kubernetes Deployment:"
	@echo "  deploy                - Deploy watcher only"
	@echo "  deploy-mcp-standalone - Deploy standalone MCP server"
	@echo "  deploy-integrated     - Deploy watcher + MCP integrated"
	@echo "  deploy-all            - Deploy watcher + standalone MCP"
	@echo "  undeploy              - Remove all deployments"
	@echo "  logs                  - Show application logs"

