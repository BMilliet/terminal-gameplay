.PHONY: help deps build run fmt fmt-check vet lint test tests

# Variables
BINARY_NAME=terminal-gameplay
INSTALL_DIR=$(HOME)/.terminal-gameplay
GO_FILES=$(shell find . -type f -name '*.go' -not -path './vendor/*')
GO_PACKAGES=./...

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

deps: ## Download and install dependencies
	@echo "📦 Installing dependencies..."
	go mod tidy

build: test ## Build the application
	@echo "🏗️  Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) .
	mkdir -p $(INSTALL_DIR)
	mv $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)

run: ## Run the application without building binary
	@echo "🚀 Running in dev mode..."
	go run main.go

fmt: ## Format Go code
	@echo "✨ Formatting code..."
	gofmt -s -w $(GO_FILES)

fmt-check: ## Check Go formatting without modifying files
	@echo "🔎 Checking Go formatting..."
	@unformatted="$$(gofmt -s -l $(GO_FILES))"; \
	if [ -n "$$unformatted" ]; then \
		echo "Files need formatting:"; \
		echo "$$unformatted"; \
		echo "Run 'make fmt' to fix them."; \
		exit 1; \
	fi

vet: ## Run go vet
	@echo "🔎 Running go vet..."
	go vet $(GO_PACKAGES)

lint: fmt-check vet ## Run linters without modifying files
	@echo "✅ Linting complete"

test: lint ## Run tests
	go test ./...

tests: test ## Alias for test
