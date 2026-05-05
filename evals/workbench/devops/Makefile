SHELL := /bin/bash
SCRIPTS_DIR := scripts
TESTS_DIR := tests

.PHONY: test lint clean

test:
	@echo "=== Running deploy tests ==="
	@bash $(TESTS_DIR)/test-deploy.sh
	@echo ""
	@echo "=== Running healthcheck tests ==="
	@bash $(TESTS_DIR)/test-healthcheck.sh
	@echo ""
	@echo "=== All tests passed ==="

lint:
	@if command -v shellcheck &>/dev/null; then \
		echo "=== Running shellcheck ===" ; \
		shellcheck $(SCRIPTS_DIR)/*.sh $(TESTS_DIR)/*.sh ; \
		echo "shellcheck: all files passed" ; \
	else \
		echo "WARNING: shellcheck not installed, skipping lint" ; \
	fi

clean:
	@rm -rf /tmp/wb-deploy-logs
	@echo "Cleaned deploy logs"
