.DEFAULT_GOAL := help

GO ?= go
GOFLAGS ?=
BIN_DIR ?= bin
FUZZTIME ?= 10s

.PHONY: audit build check clean demo-safety-case fmt fuzz help test test-race vet

audit: ## Run the extended release-readiness verification matrix.
	FUZZTIME=$(FUZZTIME) bash scripts/verify-release.sh

help: ## Show available targets.
	@awk 'BEGIN {FS = ":.*## "; printf "InferLab development targets:\n\n"} /^[a-zA-Z_-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the InferLab CLI.
	$(GO) build $(GOFLAGS) -trimpath -o $(BIN_DIR)/inferlab ./cmd/inferlab

demo-safety-case: build ## Reproduce signed BLOCK and INCONCLUSIVE public fixtures.
	bash scripts/demo-safety-case.sh

fmt: ## Format Go source files.
	$(GO) fmt ./...

fuzz: ## Run short untrusted-input and privacy fuzz campaigns.
	$(GO) test -run '^$$' -fuzz '^FuzzDecoderNeverPanics$$' -fuzztime=$(FUZZTIME) -parallel=2 ./pkg/trace
	$(GO) test -run '^$$' -fuzz '^FuzzProtectorDeterministic$$' -fuzztime=$(FUZZTIME) -parallel=2 ./pkg/trace
	$(GO) test -run '^$$' -fuzz '^FuzzDecoderNeverPanics$$' -fuzztime=$(FUZZTIME) -parallel=2 ./pkg/change
	$(GO) test -run '^$$' -fuzz '^FuzzDecoderNeverPanics$$' -fuzztime=$(FUZZTIME) -parallel=2 ./pkg/evidence
	$(GO) test -run '^$$' -fuzz '^FuzzRuntimeDecoderNeverPanics$$' -fuzztime=$(FUZZTIME) -parallel=2 ./pkg/evidence
	$(GO) test -run '^$$' -fuzz '^FuzzDecoderNeverPanics$$' -fuzztime=$(FUZZTIME) -parallel=2 ./pkg/adapter
	$(GO) test -run '^$$' -fuzz '^FuzzDecoderNeverPanics$$' -fuzztime=$(FUZZTIME) -parallel=2 ./pkg/gate
	$(GO) test -run '^$$' -fuzz '^FuzzDecoderNeverPanics$$' -fuzztime=$(FUZZTIME) -parallel=2 ./pkg/safetycase

vet: ## Run Go static analysis.
	$(GO) vet ./...

test: ## Run the unit test suite.
	$(GO) test ./...

test-race: ## Run tests with the race detector.
	$(GO) test -race ./...

check: fmt vet test test-race ## Run the local merge gate.

clean: ## Remove local build and coverage output.
	$(GO) clean
	rm -rf $(BIN_DIR) coverage dist .gocache
