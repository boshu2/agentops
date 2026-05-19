#!/usr/bin/env bash
# tests/e2e/goals-scenarios-link.sh — F1 e2e for epic soc-58nt.
#
# Exercises `ao goals scenarios --create` and `--lint` end-to-end in an
# isolated temp repo: bidirectional directive↔scenario link creation, a clean
# lint pass, a broken-link lint failure, and byte-for-byte preservation of
# non-target GOALS.md content (the Three-Gap section and Gates table).
#
# Fully automated, local-only, CI-runnable. Never touches this repo's GOALS.md
# or .agents/ — all work happens under a mktemp directory.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# Shared e2e harness (skill: testing-real-service-e2e-no-mocks):
# shellcheck source=../lib/e2e-guards.sh
source "$REPO_ROOT/tests/lib/e2e-guards.sh"
# shellcheck source=../lib/e2e-factory.sh
source "$REPO_ROOT/tests/lib/e2e-factory.sh"

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

WORK="$(e2e_factory_sandbox goals-scenarios-link)"
trap 'rm -rf "$WORK"' EXIT
log "temp root: $WORK"
e2e_guard_repo "$WORK"

AO="$(e2e_factory_ao_bin "$WORK/bin" "$REPO_ROOT")"
e2e_guard_ao_bin "$AO"
log "ao binary: $AO"

cd "$WORK"
# This test runs `ao goals scenarios` with PWD-relative GOALS.md, so check
# that the chdir landed us in the sandbox (not the agentops repo root).
e2e_guard_not_repo_root

cat > GOALS.md <<'EOF'
# Goals

e2e fixture for the executable-spec link layer.

## Directives

### 1. First directive

Body text for the first directive.

**Steer:** increase (x)

### 2. Target directive

Body text for the target directive.

**Steer:** decrease (y)

## Three-Gap Contract Proof Surface

This non-target section must survive every patch byte-for-byte.

## Gates

| ID | Check | Weight | Description | Tags |
|----|-------|--------|-------------|------|
| g | `true` | 5 | a gate | warn-only |
EOF
log "fixture GOALS.md written ($(wc -l < GOALS.md) lines)"
NONTARGET_BEFORE="$(sed -n '/## Three-Gap/,$p' GOALS.md)"

# --- step 1: create a scenario and link it to directive 2 ---
log "step 1: argv = ao goals scenarios --create '...' --directive 2 --status active -o json"
if ! CREATE_JSON="$("$AO" goals scenarios --create "target behaviour is observable" \
  --directive 2 --status active -o json 2>create.err)"; then
  cat create.err >&2
  fail "create exited non-zero"
fi
printf '%s\n' "$CREATE_JSON"
SCEN_ID="$(printf '%s' "$CREATE_JSON" | jq -r '.scenario_id')"
SCEN_PATH="$(printf '%s' "$CREATE_JSON" | jq -r '.scenario_path')"
LINKED="$(printf '%s' "$CREATE_JSON" | jq -r '.linked')"
DIRECTIVE_ID="$(printf '%s' "$CREATE_JSON" | jq -r '.directive_id')"
log "created scenario $SCEN_ID at $SCEN_PATH (linked=$LINKED, directive=$DIRECTIVE_ID)"
[[ "$LINKED" == "true" ]] || fail "scenario reported as not linked"

# --- step 2: verify the link is bidirectional ---
[[ -f "$SCEN_PATH" ]] || fail "scenario file missing: $SCEN_PATH"
SCEN_DIRECTIVE="$(jq -r '.directive_id' "$SCEN_PATH")"
[[ "$SCEN_DIRECTIVE" == "$DIRECTIVE_ID" ]] \
  || fail "scenario directive_id ($SCEN_DIRECTIVE) != directive ($DIRECTIVE_ID)"
grep -qF "**Scenarios:** $SCEN_ID" GOALS.md \
  || fail "GOALS.md directive is missing the Scenarios link to $SCEN_ID"
log "step 2: bidirectional link verified (scenario→directive and directive→scenario)"

# --- step 3: lint a complete link graph — must report zero errors ---
log "step 3: argv = ao goals scenarios --lint -o json"
if ! LINT_JSON="$("$AO" goals scenarios --lint -o json 2>lint.err)"; then
  cat lint.err >&2
  fail "lint exited non-zero on a complete link graph"
fi
printf '%s\n' "$LINT_JSON"
LINT_ERRORS="$(printf '%s' "$LINT_JSON" | jq -r '.errors')"
[[ "$LINT_ERRORS" == "0" ]] || fail "lint reported $LINT_ERRORS error(s) on a clean link graph"
log "step 3: lint clean (errors=0)"

# --- step 4: break the link — lint must now fail ---
rm -f "$SCEN_PATH"
log "step 4: removed $SCEN_PATH; argv = ao goals scenarios --lint -o json"
set +e
LINT2_JSON="$("$AO" goals scenarios --lint -o json 2>lint2.err)"
LINT2_EXIT=$?
set -e
printf '%s\n' "$LINT2_JSON"
log "lint exit code: $LINT2_EXIT"
[[ "$LINT2_EXIT" -ne 0 ]] || fail "lint should exit non-zero when a linked scenario is missing"
printf '%s' "$LINT2_JSON" | jq -e '.findings[] | select(.code == "missing-scenario")' >/dev/null \
  || fail "lint did not report the missing-scenario error"
log "step 4: broken link correctly detected as a missing-scenario error"

# --- step 5: non-target GOALS.md content survived every patch ---
NONTARGET_AFTER="$(sed -n '/## Three-Gap/,$p' GOALS.md)"
[[ "$NONTARGET_BEFORE" == "$NONTARGET_AFTER" ]] \
  || fail "non-target GOALS.md content (Three-Gap section / Gates table) changed"
log "step 5: non-target GOALS.md content preserved byte-for-byte"

log "PASS: F1 executable-spec link e2e (create → verify → lint clean → break → lint fails)"
