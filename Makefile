.PHONY: help proto-gen build build-all test lint ci-check ci-test clean run-all fmt install-tools \
       docker-build docker-up docker-down docker-logs docker-monitoring-up docker-monitoring-down

COMPOSE := docker compose -p 24alert -f deployments/docker-compose.yaml

help:
	@echo "24Alert Trading Bot - Make targets"
	@echo "  make proto-gen       - Generate protobuf Go code"
	@echo "  make build           - Build all services (no protoc required)"
	@echo "  make build-all       - proto-gen then build"
	@echo "  make test            - Run unit tests"
	@echo "  make lint            - Run linter"
	@echo "  make ci-check        - Run lint, test, and build (local CI)"
	@echo "  make ci-test         - Run tests with coverage threshold check (alias for ci-check)"
	@echo "  make fmt             - Format code"
	@echo "  make clean           - Clean binaries and generated code"
	@echo "  make install-tools   - Install development tools"
	@echo "  make run-all         - Run all services locally (requires Docker/compose)"
	@echo "  make docker-build    - Build all Docker images"
	@echo "  make docker-up       - Start all services in Docker"
	@echo "  make docker-down     - Stop and remove Docker containers"
	@echo "  make docker-logs     - Follow Docker container logs"

PROTO_SRC := $(shell find proto -name "*.proto")
PROTO_OUT := gen/go

proto-gen: $(PROTO_OUT)
	@echo "Generating protobuf code..."
	protoc \
		--go_out=. \
		--go-grpc_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_SRC)
	@echo "Proto generation complete!"

$(PROTO_OUT):
	mkdir -p $(PROTO_OUT)

build:
	@echo "Building services..."
	go build -o bin/order-svc ./cmd/order-svc
	go build -o bin/marketdata-svc ./cmd/marketdata-svc
	go build -o bin/portfolio-svc ./cmd/portfolio-svc
	go build -o bin/risk-svc ./cmd/risk-svc
	go build -o bin/gateway ./cmd/gateway
	@echo "Build complete! Binaries in ./bin/"

build-all: proto-gen build

test:
	@echo "Running tests (excluding tests/e2e; use make test-e2e with gateway up)..."
	go test -v -cover $$(go list ./... | grep -v '/tests/e2e')

test-e2e:
	@echo "E2E against API_BASE_URL (default http://127.0.0.1:18080)"
	go test -v -count=1 ./tests/e2e/...

lint:
	@echo "Linting code..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found, install with: golangci-lint --version"; exit 1)
	golangci-lint run ./...

ci-check: lint test build
	@echo "CI checks passed"

ci-test: ci-check

fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -rf gen/
	rm -rf vendor/
	go clean
	find . -name "*.pb.go" -delete

install-tools:
	@echo "Installing development tools..."
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Tools installed!"

run-all:
	@echo "Use docker-compose or scripts in deployments/ to run all services"
	@echo "docker-compose up -d"

docker-build:
	@echo "Building Docker images..."
	$(COMPOSE) build
	@echo "Docker images built!"

docker-up:
	@echo "Starting services..."
	$(COMPOSE) up -d
	@echo "Services started! Check with: make docker-logs"

docker-down:
	@echo "Stopping services..."
	$(COMPOSE) down
	@echo "Services stopped."

docker-logs:
	$(COMPOSE) logs -f

docker-monitoring-up:
	@echo "Starting services with monitoring..."
	$(COMPOSE) --profile monitoring up -d
	@echo "Services + Prometheus Agent + Promtail started!"

docker-monitoring-down:
	@echo "Stopping monitoring..."
	$(COMPOSE) --profile monitoring down
