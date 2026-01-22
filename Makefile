.PHONY: help build run fmt deps

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
	mkdir ~/.terminal-gameplay | true
	mv ${BINARY_NAME} ~/.terminal-gameplay/${BINARY_NAME}

run: ## Run the application without building binary
	@echo "🚀 Running in dev mode..."
	go run main.go

fmt: ## Format Go code
	@echo "✨ Formatting code..."
	gofmt -s -w $(GO_FILES)
	go fmt $(GO_PACKAGES)

lint: fmt vet ## Run linters (fmt + vet)
	@echo "✅ Linting complete"

