SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

WORKBENCH_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
TASKS_DIR := $(WORKBENCH_DIR)tasks

GO_TASKS := go-01 go-02 go-03 go-04 go-05
PY_TASKS := py-01 py-02 py-03 py-04
OPS_TASKS := ops-01 ops-02 ops-03
ALL_TASKS := $(GO_TASKS) $(PY_TASKS) $(OPS_TASKS)

.PHONY: verify verify-golden verify-broken clean help head-to-head

# --- Environment prerequisites (skip-not-fail) ---------------------------------
# The go/py/ops workbench-agent-v1 tasks were authored for a GNU/Linux CI
# environment: the go/ops setup.sh scripts use GNU-only `sed -i`/`i\`/`c\`
# syntax, and the py tasks score against a pre-built python-api/.venv (CI builds
# it — see .github/workflows/validate.yml; bare checkouts and macOS BSD sed do
# not have these). Rather than emit spurious FAILs on a host that simply lacks
# the toolchain, verify-golden / verify-broken SKIP a task whose prerequisites
# are absent and only FAIL on a task that genuinely ran wrong. This mirrors the
# skip semantics already used by scripts/check-eval-workbench.sh (the live gate)
# for the python venv. (age-mgz)
#
#   HAS_GNU_SED — go/ops setup scripts need GNU sed (BSD sed on macOS errors).
#   HAS_PY_VENV — py tasks score against python-api/.venv.
HAS_GNU_SED := $(shell sed --version >/dev/null 2>&1 && echo 1 || echo 0)
HAS_PY_VENV := $(shell test -d $(WORKBENCH_DIR)python-api/.venv && echo 1 || echo 0)

# task_prereq_ok: emits "1" if this host can run $$task, else "0".
# Consumes the HAS_* env vars expanded above.
define task_prereq_ok
	case "$$task" in \
		go-*|ops-*) [ "$(HAS_GNU_SED)" = "1" ] && echo 1 || echo 0 ;; \
		py-*)       [ "$(HAS_PY_VENV)" = "1" ] && echo 1 || echo 0 ;; \
		*)          echo 1 ;; \
	esac
endef

help:
	@echo "Eval workbench targets:"
	@echo "  verify        — run all golden-state verifications (components + tasks)"
	@echo "  verify-golden — apply golden source to each task and confirm score passes"
	@echo "  verify-broken — run setup for each task and confirm score fails"
	@echo "  clean         — remove temp dirs"
	@echo ""
	@echo "  Prereqs (tasks lacking these SKIP, they do not FAIL):"
	@echo "    GNU sed (go/ops setup): HAS_GNU_SED=$(HAS_GNU_SED)"
	@echo "    python venv (py score): HAS_PY_VENV=$(HAS_PY_VENV)"

verify: verify-components verify-golden
	@echo ""
	@echo "=== All verifications passed ==="

verify-components:
	@echo "=== Verifying Go CLI golden state ==="
	cd $(WORKBENCH_DIR)go-cli && go build ./... && go vet ./... && go test ./...
	@echo ""
	@if [ "$(HAS_PY_VENV)" = "1" ]; then \
		echo "=== Verifying Python API golden state ==="; \
		cd $(WORKBENCH_DIR)python-api && source .venv/bin/activate 2>/dev/null && python -m pytest tests/ -q; \
	else \
		echo "=== SKIP Python API golden state (no python-api/.venv) ==="; \
	fi
	@echo ""
	@echo "=== Verifying DevOps golden state ==="
	cd $(WORKBENCH_DIR)devops && bash tests/test-deploy.sh && bash tests/test-healthcheck.sh

verify-golden:
	@echo ""
	@echo "=== Verifying task scoring against golden source ==="
	@pass=0; fail=0; skip=0; \
	for task in $(ALL_TASKS); do \
		runnable=$$($(task_prereq_ok)); \
		if [ "$$runnable" != "1" ]; then \
			echo "  SKIP (prereq): $$task"; \
			skip=$$((skip + 1)); \
			continue; \
		fi; \
		tmpdir=$$(mktemp -d); \
		if bash $(TASKS_DIR)/$$task/setup.sh "$$tmpdir" >/dev/null 2>&1; then \
			if echo "$$task" | grep -q "^go-"; then \
				cp -r $(WORKBENCH_DIR)go-cli/* "$$tmpdir/"; \
			elif echo "$$task" | grep -q "^py-"; then \
				rsync -a $(WORKBENCH_DIR)python-api/ "$$tmpdir/" 2>/dev/null; \
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
	echo "Golden verify: $$pass passed, $$fail failed, $$skip skipped out of $(words $(ALL_TASKS))"; \
	[ $$fail -eq 0 ]

verify-broken:
	@echo "=== Verifying task scoring detects broken state ==="
	@pass=0; fail=0; skip=0; \
	for task in $(ALL_TASKS); do \
		runnable=$$($(task_prereq_ok)); \
		if [ "$$runnable" != "1" ]; then \
			echo "  SKIP (prereq): $$task"; \
			skip=$$((skip + 1)); \
			continue; \
		fi; \
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
	echo "Broken verify: $$pass detected, $$fail missed, $$skip skipped out of $(words $(ALL_TASKS))"; \
	[ $$fail -eq 0 ]

head-to-head:
	@# D10: regenerate scorecard-latest.json and compare to baseline. The
	@# heavy lifting lives in scripts/check-eval-workbench.sh — invoke it
	@# rather than duplicating the delta logic here.
	@bash $(WORKBENCH_DIR)../../scripts/check-eval-workbench.sh

clean:
	rm -rf /tmp/wb-* $(WORKBENCH_DIR)scorecard-latest.json
