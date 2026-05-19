#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
FIXTURE_DIR="$REPO_ROOT/tests/fixtures/flywheel-proof"

# Shared e2e harness (skill: testing-real-service-e2e-no-mocks):
#   guards  — refuse-to-run if HOME/repo/ao binary look real
#   logger  — JSON-line sidecar for CI parseability
#   factory — sandbox + repo + ao binary builders
# shellcheck source=../lib/e2e-guards.sh
source "$REPO_ROOT/tests/lib/e2e-guards.sh"
# shellcheck source=../lib/e2e-logger.sh
source "$REPO_ROOT/tests/lib/e2e-logger.sh"
# shellcheck source=../lib/e2e-factory.sh
source "$REPO_ROOT/tests/lib/e2e-factory.sh"

# Note: proof-run.sh never executes ao relative to PWD — every run_ao call
# chdirs into REPO_DIR. So e2e_guard_not_repo_root is intentionally NOT
# called here; the per-invocation REPO/HOME/AO guards below are sufficient.

WORK_DIR="$(e2e_factory_sandbox flywheel-proof)"
BUILD_DIR="$WORK_DIR/bin"
HOME_DIR="$WORK_DIR/home"
REPO_DIR="$WORK_DIR/repo"
AO_BIN="$BUILD_DIR/ao"
LOG_FILE="$WORK_DIR/proof-run.log"
SIDECAR_LOG="$WORK_DIR/proof-run.jsonl"
PASS_COUNT=0
LOOKUP_QUERY="task-scoped lookup queries"

cleanup() {
  chmod -R u+w "$WORK_DIR" 2>/dev/null || true
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

log() {
  printf '[proof-run] %s\n' "$*" | tee -a "$LOG_FILE"
}

pass() {
  PASS_COUNT=$((PASS_COUNT + 1))
  log "PASS: $*"
  e2e_log_pass "$*"
}

fail() {
  log "FAIL: $*"
  e2e_log_fail "$*"
  e2e_log_summary
  exit 1
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "missing required command: $1"
  fi
}

assert_file_exists() {
  local label="$1"
  local path="$2"
  if [[ -f "$path" ]]; then
    pass "$label"
    return
  fi
  fail "$label (missing $path)"
}

assert_json_match() {
  local label="$1"
  local file="$2"
  local filter="$3"
  if jq -e "$filter" "$file" >/dev/null 2>&1; then
    pass "$label"
    return
  fi
  log "jq filter failed: $filter"
  sed -n '1,200p' "$file" | tee -a "$LOG_FILE" >/dev/null
  fail "$label"
}

count_files() {
  local dir="$1"
  local pattern="$2"
  if [[ ! -d "$dir" ]]; then
    echo 0
    return
  fi
  find "$dir" -maxdepth 1 -type f -name "$pattern" | wc -l | tr -d ' '
}

run_ao() {
  (
    cd "$REPO_DIR"
    "$AO_BIN" "$@"
  )
}

require_cmd git
require_cmd jq

mkdir -p "$HOME_DIR"
export HOME="$HOME_DIR"
e2e_log_init "flywheel-proof" "$SIDECAR_LOG"
e2e_log_phase setup

# Build (or reuse) the ao binary inside the sandbox. Honors PROOF_AO_BIN and
# PROOF_FORCE_BUILD — see tests/lib/e2e-factory.sh for the resolution rules.
AO_BIN="$(e2e_factory_ao_bin "$BUILD_DIR" "$REPO_ROOT")"
export PATH="$BUILD_DIR:$PATH"
pass "ao binary resolved at $AO_BIN"

# Production safety guards — every assertion must hold before we touch state.
# The skill's "never hit prod from tests" rule, translated to the CLI domain:
# $HOME must point inside the sandbox, the repo must be under a temp prefix,
# and the binary must live in the sandbox build dir (or repo-local cli/bin).
e2e_guard_home "$HOME_DIR"
e2e_guard_ao_bin "$AO_BIN"

log "Creating isolated proof repo"
REPO_DIR="$(e2e_factory_repo "$REPO_DIR")"
e2e_guard_repo "$REPO_DIR"
pass "initialized isolated repo"

TRANSCRIPT="$(e2e_factory_fixture "$FIXTURE_DIR/seed-session.jsonl" "$REPO_DIR")"
pass "copied raw transcript fixture"

e2e_log_phase forge
log "Phase 1: forge transcript into pending learnings"
run_ao forge transcript "$TRANSCRIPT" --quiet >/dev/null
PENDING_DIR="$REPO_DIR/.agents/knowledge/pending"
PENDING_COUNT="$(count_files "$PENDING_DIR" '*.md')"
if [[ "$PENDING_COUNT" -lt 1 ]]; then
  fail "expected pending learnings after forge, found $PENDING_COUNT"
fi
pass "forge produced $PENDING_COUNT pending learning(s)"

e2e_log_phase pool-ingest
log "Phase 2: pool ingest lands pending learnings as pool candidates"
# Use 'ao pool ingest' rather than 'ao flywheel close-loop' here: close-loop now
# auto-promotes candidates in the same call (commit b69a00f4 removed the
# citation-required deadlock), so the candidate would no longer be observable in
# pool/pending. This phase verifies ingest in isolation before Phase 3 exercises
# the cite -> promote path via close-loop.
INGEST_JSON="$WORK_DIR/pool-ingest.json"
run_ao pool ingest --json > "$INGEST_JSON"
assert_json_match "pool ingest added pending learnings" "$INGEST_JSON" '.added >= 1'

POOL_PENDING_DIR="$REPO_DIR/.agents/pool/pending"
CANDIDATE_PATH="$(find "$POOL_PENDING_DIR" -maxdepth 1 -type f -name '*.json' | head -n 1)"
if [[ -z "$CANDIDATE_PATH" ]]; then
  fail "expected a pool candidate after ingest"
fi
assert_file_exists "pool candidate exists after ingest" "$CANDIDATE_PATH"

e2e_log_phase cite-promote
log "Phase 3: cite the pool candidate and promote it into a retrievable artifact"
run_ao metrics cite "$CANDIDATE_PATH" --type reference --session proof-promotion --query "$LOOKUP_QUERY" >/dev/null
CLOSE2_JSON="$WORK_DIR/close-loop-2.json"
run_ao flywheel close-loop --threshold 0h --json > "$CLOSE2_JSON"
assert_json_match "close-loop promoted a cited candidate" "$CLOSE2_JSON" '.auto_promote.promoted >= 1'

ARTIFACT_PATH="$(jq -r '.auto_promote.artifacts[0] // empty' "$CLOSE2_JSON")"
if [[ -z "$ARTIFACT_PATH" ]]; then
  fail "expected promoted artifact path in close-loop output"
fi
assert_file_exists "promoted artifact exists on disk" "$ARTIFACT_PATH"
e2e_log_artifact "$ARTIFACT_PATH" "promoted-artifact"
ARTIFACT_JSON="$(printf '%s' "$ARTIFACT_PATH" | jq -R '.')"

e2e_log_phase lookup
log "Phase 4: lookup retrieves the promoted artifact and records retrieved evidence"
LOOKUP_JSON="$WORK_DIR/lookup.json"
run_ao lookup --query "$LOOKUP_QUERY" --json > "$LOOKUP_JSON"
assert_json_match "lookup surfaces promoted knowledge" "$LOOKUP_JSON" '((.learnings | length) + (.patterns | length)) >= 1'

CITATIONS_PATH="$REPO_DIR/.agents/ao/citations.jsonl"
assert_file_exists "citations log exists" "$CITATIONS_PATH"
assert_json_match \
  "lookup recorded a retrieved citation for the promoted artifact" \
  "$CITATIONS_PATH" \
  "select(.artifact_path == $ARTIFACT_JSON and .citation_type == \"retrieved\")"

e2e_log_phase feedback
log "Phase 5: record applied evidence and close the feedback loop"
run_ao metrics cite "$ARTIFACT_PATH" --type applied --session proof-apply --query "$LOOKUP_QUERY" >/dev/null
mkdir -p "$REPO_DIR/.agents/ao"
cp "$FIXTURE_DIR/last-session-outcome.success.json" "$REPO_DIR/.agents/ao/last-session-outcome.json"
pass "seeded deterministic success outcome"

CLOSE3_JSON="$WORK_DIR/close-loop-3.json"
run_ao flywheel close-loop --threshold 0h --json > "$CLOSE3_JSON"
assert_json_match "close-loop rewarded applied artifact feedback" "$CLOSE3_JSON" '.citation_feedback.rewarded >= 1'

FEEDBACK_PATH="$REPO_DIR/.agents/ao/feedback.jsonl"
assert_file_exists "feedback log exists" "$FEEDBACK_PATH"
assert_json_match \
  "feedback log records rewarded applied evidence" \
  "$FEEDBACK_PATH" \
  "select(.artifact_path == $ARTIFACT_JSON and .decision == \"rewarded\" and .reason == \"artifact-applied\" and .utility_after > .utility_before)"
assert_json_match \
  "applied citation is marked feedback-given" \
  "$CITATIONS_PATH" \
  "select(.artifact_path == $ARTIFACT_JSON and .citation_type == \"applied\" and .feedback_given == true)"

e2e_log_phase nightly
log "Phase 6: run nightly dream cycle proof against the isolated corpus"
NIGHTLY_DIR="$WORK_DIR/nightly"
mkdir -p "$NIGHTLY_DIR"
bash "$REPO_ROOT/scripts/nightly-dream-cycle.sh" \
  --ao "$AO_BIN" \
  --repo-root "$REPO_DIR" \
  --output-dir "$NIGHTLY_DIR" >/dev/null
assert_file_exists "nightly retrieval report exists" "$NIGHTLY_DIR/retrieval-bench.json"
assert_file_exists "nightly summary exists" "$NIGHTLY_DIR/summary.json"
assert_json_match "nightly summary exposes retrieval_live" "$NIGHTLY_DIR/summary.json" '.retrieval_live != null'
assert_json_match "nightly retrieval report has coverage" "$NIGHTLY_DIR/retrieval-bench.json" '.coverage >= 0'

e2e_log_phase "complete"
e2e_log_summary
log "FLYWHEEL PROOF: PASS ($PASS_COUNT checks) — sidecar: $SIDECAR_LOG"
