.DEFAULT_GOAL := help

GO ?= go
GOFLAGS ?=
BIN_DIR ?= bin

.PHONY: build check clean fmt help test test-race vet

help: ## Show available targets.
	@awk 'BEGIN {FS = ":.*## "; printf "InferLab development targets:\n\n"} /^[a-zA-Z_-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the InferLab CLI.
	$(GO) build $(GOFLAGS) -trimpath -o $(BIN_DIR)/inferlab ./cmd/inferlab

fmt: ## Format Go source files.
	$(GO) fmt ./...

vet: ## Run Go static analysis.
	$(GO) vet ./...

test: ## Run the unit test suite.
	$(GO) test ./...

test-race: ## Run tests with the race detector.
	$(GO) test -race ./...

check: fmt vet test test-race ## Run the local merge gate.

clean: ## Remove local build and coverage output.
	$(GO) clean
	rm -rf $(BIN_DIR) coverage dist
