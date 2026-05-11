SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

WORKBENCH_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
TASKS_DIR := $(WORKBENCH_DIR)tasks

GO_TASKS := go-01 go-02 go-03 go-04 go-05
PY_TASKS := py-01 py-02 py-03 py-04
OPS_TASKS := ops-01 ops-02 ops-03
ALL_TASKS := $(GO_TASKS) $(PY_TASKS) $(OPS_TASKS)

.PHONY: verify verify-golden verify-broken clean help head-to-head

help:
	@echo "Eval workbench targets:"
	@echo "  verify        — run all golden-state verifications (components + tasks)"
	@echo "  verify-golden — apply golden source to each task and confirm score passes"
	@echo "  verify-broken — run setup for each task and confirm score fails"
	@echo "  clean         — remove temp dirs"

verify: verify-components verify-golden
	@echo ""
	@echo "=== All verifications passed ==="

verify-components:
	@echo "=== Verifying Go CLI golden state ==="
	cd $(WORKBENCH_DIR)go-cli && go build ./... && go vet ./... && go test ./...
	@echo ""
	@echo "=== Verifying Python API golden state ==="
	cd $(WORKBENCH_DIR)python-api && source .venv/bin/activate 2>/dev/null && python -m pytest tests/ -q
	@echo ""
	@echo "=== Verifying DevOps golden state ==="
	cd $(WORKBENCH_DIR)devops && bash tests/test-deploy.sh && bash tests/test-healthcheck.sh

verify-golden:
	@echo ""
	@echo "=== Verifying task scoring against golden source ==="
	@pass=0; fail=0; \
	for task in $(ALL_TASKS); do \
		tmpdir=$$(mktemp -d); \
		if bash $(TASKS_DIR)/$$task/setup.sh "$$tmpdir" >/dev/null 2>&1; then \
			if echo "$$task" | grep -q "^go-"; then \
				cp -r $(WORKBENCH_DIR)go-cli/* "$$tmpdir/"; \
			elif echo "$$task" | grep -q "^py-"; then \
				rsync -a --exclude=.venv $(WORKBENCH_DIR)python-api/ "$$tmpdir/" 2>/dev/null; \
			elif echo "$$task" | grep -q "^ops-"; then \
				cp -r $(WORKBENCH_DIR)devops/* "$$tmpdir/"; \
			fi; \
			result=$$(bash $(TASKS_DIR)/$$task/score.sh "$$tmpdir" 2>/dev/null | tail -1); \
			is_pass=$$(echo "$$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('pass','false'))" 2>/dev/null || echo "false"); \
			if [ "$$is_pass" = "True" ] || [ "$$is_pass" = "true" ]; then \
				echo "  PASS (golden): $$task — $$result"; \
				pass=$$((pass + 1)); \
			else \
				echo "  FAIL (golden): $$task — $$result"; \
				fail=$$((fail + 1)); \
			fi; \
		else \
			echo "  FAIL (setup): $$task"; \
			fail=$$((fail + 1)); \
		fi; \
		rm -rf "$$tmpdir"; \
	done; \
	echo ""; \
	echo "Golden verify: $$pass passed, $$fail failed out of $(words $(ALL_TASKS))"; \
	[ $$fail -eq 0 ]

verify-broken:
	@echo "=== Verifying task scoring detects broken state ==="
	@pass=0; fail=0; \
	for task in $(ALL_TASKS); do \
		tmpdir=$$(mktemp -d); \
		if bash $(TASKS_DIR)/$$task/setup.sh "$$tmpdir" >/dev/null 2>&1; then \
			result=$$(bash $(TASKS_DIR)/$$task/score.sh "$$tmpdir" 2>/dev/null | tail -1); \
			is_pass=$$(echo "$$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('pass','false'))" 2>/dev/null || echo "false"); \
			if [ "$$is_pass" = "False" ] || [ "$$is_pass" = "false" ]; then \
				echo "  PASS (broken detected): $$task — $$result"; \
				pass=$$((pass + 1)); \
			else \
				echo "  FAIL (broken not detected): $$task — $$result"; \
				fail=$$((fail + 1)); \
			fi; \
		else \
			echo "  FAIL (setup): $$task"; \
			fail=$$((fail + 1)); \
		fi; \
		rm -rf "$$tmpdir"; \
	done; \
	echo ""; \
	echo "Broken verify: $$pass detected, $$fail missed out of $(words $(ALL_TASKS))"; \
	[ $$fail -eq 0 ]

head-to-head:
	@# D10: regenerate scorecard-latest.json and compare to baseline. The
	@# heavy lifting lives in scripts/check-eval-workbench.sh — invoke it
	@# rather than duplicating the delta logic here.
	@bash $(WORKBENCH_DIR)../../scripts/check-eval-workbench.sh

clean:
	rm -rf /tmp/wb-* $(WORKBENCH_DIR)scorecard-latest.json
