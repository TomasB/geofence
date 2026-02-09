.PHONY: help build test clean run docker-build docker-run docker-down fmt vet proto proto-tools deps-download deps-tidy test-unit test-integration coverage all

# Variables
BINARY_NAME := geofence
GO := go
DOCKER_IMAGE := geofence:latest

GOBIN ?= $(shell $(GO) env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell $(GO) env GOPATH)/bin
endif

PROTOC_GEN_GO := $(GOBIN)/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(GOBIN)/protoc-gen-go-grpc
PROTOC_GEN_GO_VERSION ?= v1.36.10
PROTOC_GEN_GO_GRPC_VERSION ?= v1.4.0

help: ## Display this help menu
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the geofence binary
	CGO_ENABLED=0 GOOS=linux $(GO) build -o bin/$(BINARY_NAME) ./cmd/geofence

run: ## Run the service locally (requires MMDB_PATH set)
	MMDB_PATH=./testdata/GeoLite2-Country-Test.mmdb $(GO) run ./cmd/geofence/main.go

test: ## Run all tests with verbose output
	$(GO) test -v -cover ./...

test-unit: ## Run unit tests only (excluding integration)
	$(GO) test -v -cover -short ./...

test-integration: ## Run integration tests only
	$(GO) test -v -run Integration ./...

coverage: ## Generate test coverage report
	$(GO) test -v -cover -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

fmt: ## Format Go code with gofmt
	$(GO) fmt ./...

vet: ## Run go vet for code quality checks
	$(GO) vet ./...

clean: ## Remove built artifacts
	rm -f bin/$(BINARY_NAME)
	rm -f coverage.out coverage.html

docker-build: build ## Build Docker image (requires Docker)
	@command -v docker >/dev/null 2>&1 || { echo "error: docker not found. Install from https://www.docker.com/products/docker-desktop"; exit 1; }
	docker compose build

docker-run: ## Run service via docker-compose (requires Docker)
	@command -v docker >/dev/null 2>&1 || { echo "error: docker not found. Install from https://www.docker.com/products/docker-desktop"; exit 1; }
	docker compose up -d

docker-down: ## Stop docker-compose services (requires Docker)
	@command -v docker >/dev/null 2>&1 || { echo "error: docker not found. Install from https://www.docker.com/products/docker-desktop"; exit 1; }
	docker compose down

proto: proto-tools ## Generate Go code from .proto files
	PATH="$(GOBIN):$(PATH)" protoc --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. ./pkg/geofence/v1/geofence.proto

proto-tools: ## Install protoc Go plugins if missing
	@command -v protoc >/dev/null 2>&1 || { echo "error: protoc not found. Install from https://grpc.io/docs/protoc-installation/"; exit 1; }
	@test -x "$(PROTOC_GEN_GO)" || GOBIN=$(GOBIN) $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	@test -x "$(PROTOC_GEN_GO_GRPC)" || GOBIN=$(GOBIN) $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

deps-download: ## Download Go module dependencies
	$(GO) mod download

deps-tidy: ## Tidy Go module dependencies
	$(GO) mod tidy

all: fmt vet test build ## Run fmt, vet, test, and build
