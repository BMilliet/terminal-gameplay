.PHONY: help build run clean test fmt vet lint deps install dev

# Variables
BINARY_NAME=terminal-gameplay
GO_FILES=$(shell find . -name '*.go' -not -path "./vendor/*")
GO_PACKAGES=$(shell go list ./... | grep -v /vendor/)

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

deps: ## Download and install dependencies
	@echo "📦 Installing dependencies..."
	go mod tidy

build: ## Build the application
	@echo "🏗️  Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) .

run: ## Run the application without building binary
	@echo "🚀 Running in dev mode..."
	go run main.go

test: ## Run tests
	@echo "🧪 Running tests..."
	go test -v -race -coverprofile=coverage.out $(GO_PACKAGES)

test-coverage: test ## Run tests with coverage report
	@echo "📊 Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

fmt: ## Format Go code
	@echo "✨ Formatting code..."
	gofmt -s -w $(GO_FILES)
	go fmt $(GO_PACKAGES)

vet: ## Run go vet
	@echo "🔍 Running go vet..."
	go vet $(GO_PACKAGES)

lint: fmt vet ## Run linters (fmt + vet)
	@echo "✅ Linting complete"

clean: ## Clean build artifacts
	@echo "🧹 Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	go clean

all: clean deps lint build ## Clean, install deps, lint and build

watch: ## Watch for changes and rebuild (requires entr)
	@echo "👀 Watching for changes..."
	@command -v entr >/dev/null 2>&1 || { echo "entr not installed. Install with: brew install entr"; exit 1; }
	find . -name '*.go' | entr -r make run
