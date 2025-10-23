.PHONY: build run test clean docker-build docker-push deploy undeploy fmt vet

# Variables
BINARY_NAME=watcher
DOCKER_IMAGE=quay.io/aslakknutsen/kkbase-watcher
DOCKER_TAG=latest

# Build the application
build:
	go build -o $(BINARY_NAME) ./cmd/watcher

# Run the application locally
run:
	go run ./cmd/watcher

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	go clean

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Push Docker image
docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

# Deploy to Kubernetes
deploy:
	kubectl apply -f deploy/rbac.yaml
	kubectl apply -f deploy/configmap.yaml
	kubectl apply -f deploy/secret.yaml
	kubectl apply -f deploy/deployment.yaml

# Remove from Kubernetes
undeploy:
	kubectl delete -f deploy/deployment.yaml --ignore-not-found
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
	@echo "  build        - Build the binary"
	@echo "  run          - Run locally"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-push  - Push Docker image"
	@echo "  deploy       - Deploy to Kubernetes"
	@echo "  undeploy     - Remove from Kubernetes"
	@echo "  fmt          - Format code"
	@echo "  vet          - Run go vet"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  logs         - Show application logs"

