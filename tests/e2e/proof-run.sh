#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
FIXTURE_DIR="$REPO_ROOT/tests/fixtures/flywheel-proof"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/flywheel-proof-XXXXXX")"
BUILD_DIR="$WORK_DIR/bin"
HOME_DIR="$WORK_DIR/home"
REPO_DIR="$WORK_DIR/repo"
AO_BIN="$BUILD_DIR/ao"
LOG_FILE="$WORK_DIR/proof-run.log"
PASS_COUNT=0
LOOKUP_QUERY="task-scoped lookup queries"

cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

log() {
  printf '[proof-run] %s\n' "$*" | tee -a "$LOG_FILE"
}

pass() {
  PASS_COUNT=$((PASS_COUNT + 1))
  log "PASS: $*"
}

fail() {
  log "FAIL: $*"
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
require_cmd go
require_cmd jq

mkdir -p "$BUILD_DIR" "$HOME_DIR" "$REPO_DIR"
export HOME="$HOME_DIR"
export PATH="$BUILD_DIR:$PATH"

log "Building local ao binary"
(
  cd "$REPO_ROOT/cli"
  go build -o "$AO_BIN" ./cmd/ao
) >/dev/null
pass "built local ao binary"

log "Creating isolated proof repo"
(
  cd "$REPO_DIR"
  git init -q
  git config user.email "proof-run@agentops.test"
  git config user.name "Proof Run"
  printf '# Flywheel Proof Repo\n' > README.md
  git add README.md
  git commit -q -m "init"
)
pass "initialized isolated repo"

TRANSCRIPT="$REPO_DIR/seed-session.jsonl"
cp "$FIXTURE_DIR/seed-session.jsonl" "$TRANSCRIPT"
pass "copied raw transcript fixture"

log "Phase 1: forge transcript into pending learnings"
run_ao forge transcript "$TRANSCRIPT" --quiet >/dev/null
PENDING_DIR="$REPO_DIR/.agents/knowledge/pending"
PENDING_COUNT="$(count_files "$PENDING_DIR" '*.md')"
if [[ "$PENDING_COUNT" -lt 1 ]]; then
  fail "expected pending learnings after forge, found $PENDING_COUNT"
fi
pass "forge produced $PENDING_COUNT pending learning(s)"

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
ARTIFACT_JSON="$(printf '%s' "$ARTIFACT_PATH" | jq -R '.')"

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

log "FLYWHEEL PROOF: PASS ($PASS_COUNT checks)"
