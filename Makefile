SHELL := /usr/bin/env bash

.PHONY: local-ci local-ci-fast ci build build-flywheel verify-buildtags test docs-check regen-all regen-check clean help

# Default: run the release-grade local CI gate.
# Note: scripts/ci-local-release.sh already includes build + test + release-binary validation.
.DEFAULT_GOAL := local-ci

local-ci: ## Run full local CI release gate (includes build and release binary validation)
	./scripts/ci-local-release.sh

local-ci-fast: ## Run local CI without e2e install test, using quick security mode
	./scripts/ci-local-release.sh --skip-e2e-install --security-mode quick

ci: local-ci ## Alias for local-ci

build: ## Build ao CLI binary (default = spine; archived satellites omitted)
	$(MAKE) -C cli build

build-flywheel: ## Build ao with the ADR-0012 archived satellites restored (flywheel + legacy)
	$(MAKE) -C cli build-flywheel

verify-buildtags: ## Verify the ADR-0012 build-tag mechanism (default omits; flywheel/legacy restore)
	$(MAKE) -C cli verify-buildtags

test: ## Run CLI tests
	$(MAKE) -C cli test

docs-check: ## Run docs and hook safety drift checks
	./scripts/generate-cli-reference.sh --check
	./scripts/check-doc-hooks-drift.sh
	./tests/docs/validate-doc-release.sh

regen-all: ## Regenerate every derived artifact after adding a skill/command (one-command finalizer)
	./scripts/regen-all.sh

regen-check: ## Run the derived-artifact drift/gate sweep (no writes; pre-push gate)
	./scripts/regen-all.sh --check

clean: ## Clean CLI build artifacts
	$(MAKE) -C cli clean

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"; printf "Targets:\n"} /^[a-zA-Z0-9_.-]+:.*##/ { printf "  %-14s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
