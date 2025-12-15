# Makefile for External DNS Traffic Manager Webhook Provider

# Variables
BINARY_NAME=webhook
DOCKER_IMAGE=traffic-manager-webhook
DOCKER_TAG=latest
DOCKER_REGISTRY=your-registry.azurecr.io

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build directory
BUILD_DIR=bin

.PHONY: all build clean test run docker-build docker-push deploy help

all: test build

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="-w -s" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/webhook

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out

## run: Run locally
run:
	@echo "Running locally..."
	$(GOCMD) run ./cmd/webhook/main.go

## tidy: Tidy go modules
tidy:
	@echo "Tidying go modules..."
	$(GOMOD) tidy

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## lint: Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	golangci-lint run

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

## docker-push: Push Docker image to registry
docker-push: docker-build
	@echo "Pushing Docker image to registry..."
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)

## deploy: Deploy to Kubernetes
deploy:
	@echo "Deploying to Kubernetes..."
	kubectl apply -f deploy/kubernetes/rbac.yaml
	kubectl apply -f deploy/kubernetes/config.yaml
	kubectl apply -f deploy/kubernetes/deployment.yaml

## deploy-example: Deploy example service
deploy-example:
	@echo "Deploying example service..."
	kubectl apply -f deploy/examples/service-example.yaml

## logs: Show logs from webhook container
logs:
	kubectl logs -n external-dns -l app=external-dns-traffic-manager -c traffic-manager-webhook -f

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
