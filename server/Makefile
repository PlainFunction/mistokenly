# Makefile for generating gRPC code and building services

# Variables
PROTO_DIR = proto
PB_DIR = internal/common/grpc/pb

# Default target
.PHONY: all
all: clean generate build

# Generate gRPC code from proto files using buf
.PHONY: generate
generate:
	@echo "Generating gRPC code with buf..."
	@buf generate
	@echo "gRPC code generated successfully!"

# Clean generated files
.PHONY: clean
clean:
	@echo "Cleaning generated files..."
	@find $(PROTO_DIR) -name "*.pb.go" -type f -delete 2>/dev/null || true
	@powershell -Command "Get-ChildItem -Path $(PROTO_DIR) -Recurse -Filter '*.pb.go' | Remove-Item -Force" 2>/dev/null || true
	@echo "Clean complete!"

# Build all services
.PHONY: build
build:
	@echo "Building API service..."
	@go build -o bin/api ./cmd/api
	@echo "Building Audit service..."
	@go build -o bin/audit ./cmd/audit
	@echo "Building Persistence service..."
	@go build -o bin/persistence ./cmd/persistence
	@echo "Build complete!"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test ./...
	@echo "Tests complete!"

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	@go mod tidy
	@echo "Dependencies installed!"

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Code formatted!"

# Run linter
.PHONY: lint
lint:
	@echo "Running linter..."
	@golangci-lint run
	@echo "Linting complete!"

# Install protoc plugins
.PHONY: install-protoc-plugins
install-protoc-plugins:
	@echo "Installing protoc plugins..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Protoc plugins installed!"

# Docker build
.PHONY: docker-build
docker-build:
	@echo "Building Docker images..."
	@docker build -f build/api.Dockerfile -t pii-cache/api:latest .
	@docker build -f build/audit.Dockerfile -t pii-cache/audit:latest .
	@docker build -f build/persistence.Dockerfile -t pii-cache/persistence:latest .
	@echo "Docker images built!"