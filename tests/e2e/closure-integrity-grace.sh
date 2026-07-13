#!/usr/bin/env bash
# Regression test: close-before-commit grace window + br (beads_rust) semantics.
# Verifies that a bead closed BEFORE its qualifying commit still passes when the
# commit lands within the 24h grace window, and that the audit reads children
# from `br show <epic> --json` .dependents (bd/Dolt retired). Also covers the
# single-epic-closure path: a closed epic with no children but commit-backed
# closure PASSES, an invalid no-child epic FAILS as collection_failed.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
AUDIT_SCRIPT="$REPO_ROOT/skills/postmortem/scripts/closure-integrity-audit.sh"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/closure-grace-XXXXXX")"
PASS=0
FAIL=0

cleanup() { rm -rf "$WORK_DIR"; }
trap cleanup EXIT

pass() { PASS=$((PASS + 1)); echo "PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "FAIL: $1"; }

# Setup: isolated git repo with a br mock
BR_DIR="$WORK_DIR/br-data"
BIN_DIR="$WORK_DIR/bin"
REPO_DIR="$WORK_DIR/repo"
mkdir -p "$BR_DIR" "$BIN_DIR" "$REPO_DIR"

# Mock br: serves `show <id> --json` (one-element array) and `show <id>` (human)
# from fixture files. The audit derives children from the epic's --json
# .dependents, so the epic fixture must carry a dependents array.
cat > "$BIN_DIR/br" <<'MOCK'
#!/usr/bin/env bash
case "$1" in
  show)
    id="$2"
    if [[ "${3:-}" == "--json" ]]; then
      cat "$BR_DIR/show-${id}.json" 2>/dev/null || echo '[]'
    else
      cat "$BR_DIR/show-${id}.txt" 2>/dev/null || echo "NOT FOUND"
    fi
    ;;
esac
MOCK
chmod +x "$BIN_DIR/br"
sed -i.bak "s|\$BR_DIR|$BR_DIR|g" "$BIN_DIR/br" 2>/dev/null || \
  sed -i '' "s|\$BR_DIR|$BR_DIR|g" "$BIN_DIR/br"

export PATH="$BIN_DIR:$PATH"
export BR_DIR

# Helper: write the epic fixture so its parent-child dependents are the given ids.
write_epic_with_children() {
  local epic="$1"; shift
  local deps=""
  local sep=""
  local id
  for id in "$@"; do
    deps+="${sep}{\"id\":\"${id}\",\"dependency_type\":\"parent-child\"}"
    sep=","
  done
  cat > "$BR_DIR/show-${epic}.json" <<JSON
[{"id":"${epic}","issue_type":"epic","status":"closed","dependents":[${deps}]}]
JSON
}

# Initialize isolated repo
(
  cd "$REPO_DIR"
  git init -q
  git config user.email "test@test.com"
  git config user.name "Test"
  echo "init" > README.md
  git add README.md
  git commit -q -m "init"
)

# Create a file that the issue scopes to
SCOPED_FILE="cli/cmd/ao/fix.go"
mkdir -p "$REPO_DIR/cli/cmd/ao"

# Scenario: bead closed at T, qualifying commit lands at T+2h (within grace)
CLOSE_TIME="2026-03-20T10:00:00+00:00"
COMMIT_TIME="2026-03-20T12:00:00+00:00"

write_epic_with_children test-epic test-epic.1

cat > "$BR_DIR/show-test-epic.1.json" <<JSON
[{
  "id": "test-epic.1",
  "status": "closed",
  "created_at": "2026-03-19T10:00:00+00:00",
  "closed_at": "$CLOSE_TIME",
  "description": "Fix the handler logic.\n\nFiles:\n- \`$SCOPED_FILE\`"
}]
JSON

# Create qualifying commit AFTER close time
(
  cd "$REPO_DIR"
  echo "package ao" > "$SCOPED_FILE"
  git add "$SCOPED_FILE"
  GIT_AUTHOR_DATE="$COMMIT_TIME" GIT_COMMITTER_DATE="$COMMIT_TIME" \
    git commit -q -m "fix: handler logic"
)

# Test 1: Without grace, this would fail (commit is after closed_at)
# With grace, it should pass
result="$(cd "$REPO_DIR" && bash "$AUDIT_SCRIPT" --scope commit test-epic 2>&1)"
verdict="$(echo "$result" | jq -r '.children[0].status')"
detail="$(echo "$result" | jq -r '.children[0].detail')"

if [[ "$verdict" == "pass" ]] && [[ "$detail" == *"grace window"* ]]; then
  pass "close-before-commit detected via grace window"
else
  fail "close-before-commit should pass via grace window (got status=$verdict detail=$detail)"
fi

# Test 2: Commit way outside grace window (T+48h) should fail
(
  cd "$REPO_DIR"
  git reset --hard HEAD~1 -q
  mkdir -p "$(dirname "$SCOPED_FILE")"
  LATE_TIME="2026-03-22T10:00:00+00:00"
  echo "package ao" > "$SCOPED_FILE"
  git add "$SCOPED_FILE"
  GIT_AUTHOR_DATE="$LATE_TIME" GIT_COMMITTER_DATE="$LATE_TIME" \
    git commit -q -m "fix: late handler logic"
)

result="$(cd "$REPO_DIR" && bash "$AUDIT_SCRIPT" --scope commit test-epic 2>&1)"
verdict="$(echo "$result" | jq -r '.children[0].status')"
ftype="$(echo "$result" | jq -r '.failures[0].failure_type')"

if [[ "$verdict" == "fail" ]] && [[ "$ftype" == "timing_miss" ]]; then
  pass "commit outside grace window correctly classified as timing_miss"
else
  fail "commit outside grace should be timing_miss (got status=$verdict failure_type=$ftype)"
fi

# Test 3: Issue with no scoped files should be parser_miss
cat > "$BR_DIR/show-test-epic.1.json" <<JSON
[{
  "id": "test-epic.1",
  "status": "closed",
  "created_at": "2026-03-19T10:00:00+00:00",
  "closed_at": "$CLOSE_TIME",
  "description": "Fix the handler logic without specifying files."
}]
JSON

result="$(cd "$REPO_DIR" && bash "$AUDIT_SCRIPT" --scope commit test-epic 2>&1)"
ftype="$(echo "$result" | jq -r '.failures[0].failure_type')"

if [[ "$ftype" == "parser_miss" ]]; then
  pass "missing scoped files correctly classified as parser_miss"
else
  fail "missing scoped files should be parser_miss (got $ftype)"
fi

# Test 4: Bead with no scoped files AND no evidence-only packet should FAIL
write_epic_with_children test-epic test-epic.2

cat > "$BR_DIR/show-test-epic.2.json" <<JSON
[{
  "id": "test-epic.2",
  "status": "closed",
  "created_at": "2026-03-19T10:00:00+00:00",
  "closed_at": "$CLOSE_TIME",
  "description": "Refactored internal logic with no specific files mentioned."
}]
JSON

# Ensure no evidence-only packet exists
rm -rf "$REPO_DIR/.agents/releases/evidence-only-closures" "$REPO_DIR/.agents/council/evidence-only-closures"

result="$(cd "$REPO_DIR" && bash "$AUDIT_SCRIPT" --scope auto test-epic 2>&1)"
verdict="$(echo "$result" | jq -r '.children[0].status')"
ftype="$(echo "$result" | jq -r '.failures[0].failure_type')"

if [[ "$verdict" == "fail" ]] && [[ "$ftype" == "parser_miss" ]]; then
  pass "no scoped files + no evidence-only packet correctly fails as parser_miss"
else
  fail "no scoped files + no evidence-only packet should be parser_miss (got status=$verdict failure_type=$ftype)"
fi

# Test 5: Bead with evidence-only packet but invalid schema should fall through
# to parser_miss (packet_is_valid rejects it).
write_epic_with_children test-epic test-epic.3

cat > "$BR_DIR/show-test-epic.3.json" <<JSON
[{
  "id": "test-epic.3",
  "status": "closed",
  "created_at": "2026-03-19T10:00:00+00:00",
  "closed_at": "$CLOSE_TIME",
  "description": "Policy-only closure with no code delta."
}]
JSON

# Create an invalid evidence-only packet (missing required fields)
mkdir -p "$REPO_DIR/.agents/releases/evidence-only-closures"
cat > "$REPO_DIR/.agents/releases/evidence-only-closures/test-epic.3.json" <<JSON
{
  "target_id": "test-epic.3",
  "evidence_mode": "invalid_mode",
  "evidence": {"artifacts": []}
}
JSON

result="$(cd "$REPO_DIR" && bash "$AUDIT_SCRIPT" --scope auto test-epic 2>&1)"
verdict="$(echo "$result" | jq -r '.children[0].status')"
ftype="$(echo "$result" | jq -r '.failures[0].failure_type')"

if [[ "$verdict" == "fail" ]] && [[ "$ftype" == "parser_miss" ]]; then
  pass "invalid evidence-only packet correctly falls through to parser_miss"
else
  fail "invalid evidence-only packet should fall through to parser_miss (got status=$verdict failure_type=$ftype)"
fi

# Test 6: Bead with expired grace window should FAIL
write_epic_with_children test-epic test-epic.1

EXPIRED_CLOSE="2026-03-15T10:00:00+00:00"
cat > "$BR_DIR/show-test-epic.1.json" <<JSON
[{
  "id": "test-epic.1",
  "status": "closed",
  "created_at": "2026-03-10T10:00:00+00:00",
  "closed_at": "$EXPIRED_CLOSE",
  "description": "Fix the handler logic.\n\nFiles:\n- \`$SCOPED_FILE\`"
}]
JSON

# Reset repo - commit is at 2026-03-20T12:00:00, close was 2026-03-15 (5 days before commit, well outside 24h grace)
(
  cd "$REPO_DIR"
  # Remove any evidence-only packets
  rm -rf .agents
  git reset --hard HEAD~1 -q 2>/dev/null || true
  mkdir -p "$(dirname "$SCOPED_FILE")"
  LATE_TIME="2026-03-20T12:00:00+00:00"
  echo "package ao" > "$SCOPED_FILE"
  git add "$SCOPED_FILE"
  GIT_AUTHOR_DATE="$LATE_TIME" GIT_COMMITTER_DATE="$LATE_TIME" \
    git commit -q -m "fix: handler logic"
)

result="$(cd "$REPO_DIR" && bash "$AUDIT_SCRIPT" --scope commit test-epic 2>&1)"
verdict="$(echo "$result" | jq -r '.children[0].status')"
ftype="$(echo "$result" | jq -r '.failures[0].failure_type')"

if [[ "$verdict" == "fail" ]] && [[ "$ftype" == "timing_miss" ]]; then
  pass "expired grace window correctly classified as timing_miss"
else
  fail "expired grace window should be timing_miss (got status=$verdict failure_type=$ftype)"
fi

# Test 7: Discovery-phase seed that was never persisted (.agents/brainstorm/,
# .agents/research/, .agents/discovery/) on a CLOSED bead with a substantive
# close_reason should WARN as discovery_miss, NOT hard-fail as timing_miss.
# (close_reason now read from --json, not human output.)
write_epic_with_children test-epic test-epic.7

cat > "$BR_DIR/show-test-epic.7.json" <<JSON
[{
  "id": "test-epic.7",
  "status": "closed",
  "created_at": "2026-04-14T10:00:00+00:00",
  "closed_at": "2026-04-14T20:00:00+00:00",
  "description": "Add opt-in long-haul controller.\n\nSeed: .agents/brainstorm/2026-04-14-long-haul-value.md",
  "close_reason": "Completed: landed the controller plus regression coverage; parent remains open for follow-up."
}]
JSON

(
  cd "$REPO_DIR"
  rm -rf .agents 2>/dev/null || true
  git reset --hard HEAD -q 2>/dev/null || true
)

result="$(cd "$REPO_DIR" && bash "$AUDIT_SCRIPT" --scope auto test-epic 2>&1)"
verdict="$(echo "$result" | jq -r '.children[0].status')"
mode="$(echo "$result" | jq -r '.children[0].evidence_mode')"
detail="$(echo "$result" | jq -r '.children[0].detail')"
failures_len="$(echo "$result" | jq -r '.failures | length')"

if [[ "$verdict" == "warn" ]] && [[ "$mode" == "discovery-seed-missing" ]] \
   && [[ "$detail" == discovery_miss:* ]] && [[ "$failures_len" == "0" ]]; then
  pass "discovery-phase seed miss on CLOSED bead classifies as discovery_miss WARN (not timing_miss FAIL)"
else
  fail "discovery-phase seed miss should warn as discovery_miss (got status=$verdict mode=$mode detail=$detail failures=$failures_len)"
fi

# Test 8: Non-discovery scoped file (cli/foo.go) that doesn't exist in git
# must still hard-fail as timing_miss — the discovery downgrade is NOT a
# generic escape hatch.
write_epic_with_children test-epic test-epic.8

cat > "$BR_DIR/show-test-epic.8.json" <<JSON
[{
  "id": "test-epic.8",
  "status": "closed",
  "created_at": "2026-04-14T10:00:00+00:00",
  "closed_at": "2026-04-14T20:00:00+00:00",
  "description": "Refactor handler.\n\nFiles:\n- \`cli/cmd/ao/nonexistent_handler.go\`",
  "close_reason": "Completed: refactored handler thoroughly across the codebase."
}]
JSON

result="$(cd "$REPO_DIR" && bash "$AUDIT_SCRIPT" --scope auto test-epic 2>&1)"
verdict="$(echo "$result" | jq -r '.children[0].status')"
ftype="$(echo "$result" | jq -r '.failures[0].failure_type // "none"')"

if [[ "$verdict" == "fail" ]] && [[ "$ftype" == "timing_miss" ]]; then
  pass "non-discovery scoped file without evidence still hard-fails as timing_miss"
else
  fail "non-discovery miss must remain timing_miss FAIL (got status=$verdict failure_type=$ftype)"
fi

# Test 9: Bead with NO scoped files but a valid evidence-only packet
# (containing both `evidence_mode` and `repo_state`) should PASS via the
# evidence-only-packet short-circuit, NOT trip parser_miss or timing_miss.
write_epic_with_children test-epic test-epic.9

cat > "$BR_DIR/show-test-epic.9.json" <<JSON
[{
  "id": "test-epic.9",
  "status": "closed",
  "created_at": "2026-04-14T10:00:00+00:00",
  "closed_at": "2026-04-14T20:00:00+00:00",
  "description": "Maintenance closure with no code delta. Proven via evidence-only packet.",
  "close_reason": "Completed: maintenance closure backed by evidence-only packet."
}]
JSON

(
  cd "$REPO_DIR"
  rm -rf .agents 2>/dev/null || true
  git reset --hard HEAD -q 2>/dev/null || true
  mkdir -p .agents/releases/evidence-only-closures
  cat > .agents/releases/evidence-only-closures/test-epic.9.json <<'PACKET'
{
  "target_id": "test-epic.9",
  "target_type": "task",
  "producer": "post-mortem",
  "evidence_mode": "commit",
  "validation_commands": ["bash scripts/validate-manifests.sh"],
  "repo_state": {
    "repo_root": ".",
    "git_branch": "main",
    "git_dirty": false,
    "head_sha": "deadbeef",
    "modified_files": [],
    "staged_files": [],
    "unstaged_files": [],
    "untracked_files": []
  },
  "evidence": {
    "summary": "Closed via evidence-only packet for maintenance audit.",
    "artifacts": [".agents/releases/evidence-only-closures/test-epic.9.json"],
    "notes": []
  }
}
PACKET
)

result="$(cd "$REPO_DIR" && bash "$AUDIT_SCRIPT" --scope auto test-epic 2>&1)"
verdict="$(echo "$result" | jq -r '.children[0].status')"
mode="$(echo "$result" | jq -r '.children[0].evidence_mode')"
detail="$(echo "$result" | jq -r '.children[0].detail')"
failures_len="$(echo "$result" | jq -r '.failures | length')"

if [[ "$verdict" == "pass" ]] && [[ "$mode" == "evidence-only-packet" ]] \
   && [[ "$detail" == *"short-circuit"* ]] && [[ "$failures_len" == "0" ]]; then
  pass "evidence-only packet short-circuits classification to PASS"
else
  fail "evidence-only packet should short-circuit to PASS evidence-only-packet (got status=$verdict mode=$mode detail=$detail failures=$failures_len)"
fi

# Test 10: same as Test 9 but using --scope commit, verifying the short-circuit
# fires for the scope-mode classifier path too.
result="$(cd "$REPO_DIR" && bash "$AUDIT_SCRIPT" --scope commit test-epic 2>&1)"
verdict="$(echo "$result" | jq -r '.children[0].status')"
mode="$(echo "$result" | jq -r '.children[0].evidence_mode')"

if [[ "$verdict" == "pass" ]] && [[ "$mode" == "evidence-only-packet" ]]; then
  pass "evidence-only packet short-circuits under --scope commit too"
else
  fail "evidence-only packet should short-circuit under --scope commit (got status=$verdict mode=$mode)"
fi

# Test 11: single-epic closure — a CLOSED epic with NO children whose closure is
# commit-backed (a commit references the epic id) must PASS via the single-epic
# path with closure_mode single-epic, NOT trip collection_failed.
cat > "$BR_DIR/show-solo-epic.json" <<JSON
[{
  "id": "solo-epic",
  "issue_type": "epic",
  "status": "closed",
  "created_at": "2026-05-01T10:00:00+00:00",
  "closed_at": "2026-05-01T20:00:00+00:00",
  "description": "Single-epic closure tracked directly on the epic.",
  "close_reason": "Completed directly on the epic.",
  "dependents": []
}]
JSON

(
  cd "$REPO_DIR"
  echo "package ao" > "cli/cmd/ao/solo.go"
  git add "cli/cmd/ao/solo.go"
  git commit -q -m "feat: land solo-epic work directly"
)

result="$(cd "$REPO_DIR" && bash "$AUDIT_SCRIPT" --scope auto solo-epic 2>&1)"
verdict="$(echo "$result" | jq -r '.children[0].status')"
cmode="$(echo "$result" | jq -r '.children[0].closure_mode // "none"')"
cfailed="$(echo "$result" | jq -r '.summary.collection_failed // false')"

if [[ "$verdict" == "pass" ]] && [[ "$cmode" == "single-epic" ]] && [[ "$cfailed" == "false" ]]; then
  pass "commit-backed single-epic closure PASSES via single-epic path"
else
  fail "commit-backed single-epic closure should PASS single-epic (got status=$verdict closure_mode=$cmode collection_failed=$cfailed)"
fi

# Test 12: invalid no-child epic — a CLOSED epic with NO children, NO commit
# reference, and only a generic close reason (no SHA) must FAIL as
# collection_failed (the single-epic path must NOT rubber-stamp it).
cat > "$BR_DIR/show-empty-epic.json" <<JSON
[{
  "id": "empty-epic",
  "issue_type": "epic",
  "status": "closed",
  "created_at": "2026-05-02T10:00:00+00:00",
  "closed_at": "2026-05-02T20:00:00+00:00",
  "description": "Closed with no children and no proof.",
  "close_reason": "done",
  "dependents": []
}]
JSON

result="$(cd "$REPO_DIR" && bash "$AUDIT_SCRIPT" --scope auto empty-epic 2>&1)" || true
cfailed="$(echo "$result" | jq -r '.summary.collection_failed // false')"

if [[ "$cfailed" == "true" ]]; then
  pass "invalid no-child epic correctly fails as collection_failed"
else
  fail "invalid no-child epic should fail as collection_failed (got collection_failed=$cfailed)"
fi

# Test 13: single-epic closure proven via a close_reason that cites a REAL landed
# commit SHA (no commit references the epic id, no children) must PASS via the
# close-reason path with evidence_mode close-reason.
REAL_SHA="$(cd "$REPO_DIR" && git rev-parse HEAD)"
cat > "$BR_DIR/show-sha-epic.json" <<JSON
[{
  "id": "sha-epic",
  "issue_type": "epic",
  "status": "closed",
  "created_at": "2026-05-03T10:00:00+00:00",
  "closed_at": "2026-05-03T20:00:00+00:00",
  "description": "Single-epic closure proven by a landed commit SHA.",
  "close_reason": "Completed: landed in ${REAL_SHA}.",
  "dependents": []
}]
JSON

result="$(cd "$REPO_DIR" && bash "$AUDIT_SCRIPT" --scope auto sha-epic 2>&1)"
verdict="$(echo "$result" | jq -r '.children[0].status')"
mode="$(echo "$result" | jq -r '.children[0].evidence_mode')"
cmode="$(echo "$result" | jq -r '.children[0].closure_mode // "none"')"

if [[ "$verdict" == "pass" ]] && [[ "$mode" == "close-reason" ]] && [[ "$cmode" == "single-epic" ]]; then
  pass "single-epic closure with close_reason citing a REAL commit SHA PASSES"
else
  fail "real-SHA close_reason should PASS close-reason single-epic (got status=$verdict mode=$mode closure_mode=$cmode)"
fi

# Test 14: close_reason citing a hex token that is NOT a real commit must NOT
# false-pass — a bare hex token is not proof. This guards the fail-open a
# cross-family review caught (an incidental hex word would otherwise rubber-stamp
# an unproven epic closure).
cat > "$BR_DIR/show-fakesha-epic.json" <<JSON
[{
  "id": "fakesha-epic",
  "issue_type": "epic",
  "status": "closed",
  "created_at": "2026-05-04T10:00:00+00:00",
  "closed_at": "2026-05-04T20:00:00+00:00",
  "description": "Closed with an incidental hex word but no real commit.",
  "close_reason": "Completed: cleaned up deadbeefdeadbeef cafef00d references.",
  "dependents": []
}]
JSON

result="$(cd "$REPO_DIR" && bash "$AUDIT_SCRIPT" --scope auto fakesha-epic 2>&1)" || true
cfailed="$(echo "$result" | jq -r '.summary.collection_failed // false')"

if [[ "$cfailed" == "true" ]]; then
  pass "close_reason with a non-resolvable hex token does NOT false-pass (collection_failed)"
else
  fail "non-resolvable hex token must NOT pass single-epic (got collection_failed=$cfailed)"
fi

echo ""
echo "Results: $PASS passed, $FAIL failed"
[[ "$FAIL" -eq 0 ]]
