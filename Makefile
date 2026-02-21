SHELL := /usr/bin/env bash

.PHONY: local-ci local-ci-fast ci build test docs-check clean help

# Default: run the release-grade local CI gate.
# Note: scripts/ci-local-release.sh already includes build + test + release-binary validation.
.DEFAULT_GOAL := local-ci

local-ci: ## Run full local CI release gate (includes build and release binary validation)
	./scripts/ci-local-release.sh

local-ci-fast: ## Run local CI without e2e install test, using quick security mode
	./scripts/ci-local-release.sh --skip-e2e-install --security-mode quick

ci: local-ci ## Alias for local-ci

build: ## Build ao CLI binary
	$(MAKE) -C cli build

test: ## Run CLI tests
	$(MAKE) -C cli test

docs-check: ## Run docs and hook safety drift checks
	./scripts/generate-cli-reference.sh --check
	./scripts/validate-hook-preflight.sh
	./tests/docs/validate-doc-release.sh

clean: ## Clean CLI build artifacts
	$(MAKE) -C cli clean

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"; printf "Targets:\n"} /^[a-zA-Z0-9_.-]+:.*##/ { printf "  %-14s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
