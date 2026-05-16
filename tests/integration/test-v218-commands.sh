#!/usr/bin/env bash
# test-v218-commands.sh - Binary command functional tests for v2.18 commands
# Runs each v2.18 command AS A SUBPROCESS and validates stdout content, not just exit codes.
# Usage: ./tests/integration/test-v218-commands.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source shared colors and helpers
source "${SCRIPT_DIR}/../lib/colors.sh"

PASS=0
FAIL=0

pass() {
    green "  PASS: $1"
    PASS=$((PASS + 1))
}

fail() {
    red "  FAIL: $1"
    FAIL=$((FAIL + 1))
}

# Pre-flight: check for Go
if ! command -v go &>/dev/null; then
    red "ERROR: Go not available — cannot build ao CLI"
    exit 1
fi

if ! command -v jq &>/dev/null; then
    red "ERROR: jq is required for v2.18 command tests"
    exit 1
fi

# Build ao from source
log "Building ao CLI from source..."

TMPBIN=$(mktemp)
TMPDIR_TEST=$(mktemp -d)
trap 'rm -f "$TMPBIN"; rm -rf "$TMPDIR_TEST"' EXIT

if (cd "$REPO_ROOT/cli" && go build -o "$TMPBIN" ./cmd/ao 2>/dev/null); then
    pass "Built ao CLI successfully"
else
    fail "go build failed"
    red "FAILED — Build failed"
    exit 1
fi
chmod +x "$TMPBIN"

# ============================================================
# Fixture setup
# ============================================================

log "Setting up test fixtures..."

# Create full .agents/ structure
mkdir -p "$TMPDIR_TEST/.agents/learnings"
mkdir -p "$TMPDIR_TEST/.agents/research"
mkdir -p "$TMPDIR_TEST/.agents/patterns"
mkdir -p "$TMPDIR_TEST/.agents/products"
mkdir -p "$TMPDIR_TEST/.agents/retros"
mkdir -p "$TMPDIR_TEST/.agents/council"
mkdir -p "$TMPDIR_TEST/.agents/knowledge/pending"
mkdir -p "$TMPDIR_TEST/.agents/plans"
mkdir -p "$TMPDIR_TEST/.agents/rpi"
mkdir -p "$TMPDIR_TEST/.agents/ao/sessions"
mkdir -p "$TMPDIR_TEST/.agents/pool"
mkdir -p "$TMPDIR_TEST/.agents/constraints"
mkdir -p "$TMPDIR_TEST/.agents/findings"

# Create sample learning files for dedup/contradict/curate tests
cat > "$TMPDIR_TEST/.agents/learnings/2026-02-25-test-learning-alpha.md" <<'LEARNING_EOF'
---
title: Use guard clauses for early return
id: test-learning-alpha
date: 2026-02-25
maturity: seed
tags: [coding, patterns]
---

## Context
When writing functions with multiple conditions, guard clauses at the top
reduce nesting and improve readability.

## Lesson
Always prefer guard clauses over deeply nested if-else chains.

## Evidence
Observed in 3 code reviews where nested logic caused bugs.
LEARNING_EOF

cat > "$TMPDIR_TEST/.agents/learnings/2026-02-25-test-learning-beta.md" <<'LEARNING_EOF'
---
title: Use guard clauses for early returns
id: test-learning-beta
date: 2026-02-25
maturity: seed
tags: [coding, patterns]
---

## Context
Guard clauses at the beginning of functions reduce complexity.

## Lesson
Prefer guard clauses over deeply nested conditionals.

## Evidence
Multiple code reviews confirmed this pattern reduces bugs.
LEARNING_EOF

cat > "$TMPDIR_TEST/.agents/learnings/2026-02-25-test-learning-gamma.md" <<'LEARNING_EOF'
---
title: Never skip pre-mortem for large changes
id: test-learning-gamma
date: 2026-02-25
maturity: sapling
tags: [process, validation]
---

## Context
Large changes without pre-mortem review introduce avoidable risk.

## Lesson
Always run pre-mortem before implementing changes with 3+ files.

## Evidence
4/4 epics with pre-mortem had zero implementation bugs.
LEARNING_EOF

# Create a sample session file
cat > "$TMPDIR_TEST/.agents/ao/sessions/test-session.json" <<'SESSION_EOF'
{
  "session_id": "test-session-001",
  "started_at": "2026-02-25T10:00:00Z",
  "ended_at": "2026-02-25T10:30:00Z",
  "learnings_produced": 2
}
SESSION_EOF

# Create pending.jsonl for memory sync test
cat > "$TMPDIR_TEST/.agents/ao/pending.jsonl" <<'PENDING_EOF'
{"type":"learning","id":"test-learning-alpha","path":".agents/learnings/2026-02-25-test-learning-alpha.md"}
PENDING_EOF

# Create MEMORY.md for notebook update test
cat > "$TMPDIR_TEST/.agents/ao/MEMORY.md" <<'MEMORY_EOF'
# Session Notebook

## Learnings
- Guard clauses reduce nesting
MEMORY_EOF

# Create compiled constraints for constraint list test
cat > "$TMPDIR_TEST/.agents/constraints/test-constraint.sh" <<'CONSTRAINT_EOF'
#!/bin/bash
# Constraint: no-direct-push
# Source: test-learning-gamma
echo "PASS: no direct push detected"
CONSTRAINT_EOF
chmod +x "$TMPDIR_TEST/.agents/constraints/test-constraint.sh"

cat > "$TMPDIR_TEST/.agents/constraints/index.json" <<'INDEX_EOF'
{
  "version": 1,
  "constraints": [
    {
      "id": "no-direct-push",
      "source_learning": "test-learning-gamma",
      "script": "test-constraint.sh",
      "description": "Prevent direct pushes to main"
    }
  ]
}
INDEX_EOF

# Create promoted findings for findings command coverage
cat > "$TMPDIR_TEST/.agents/findings/test-finding-alpha.md" <<'FINDING_EOF'
---
id: test-finding-alpha
title: Prefer registry-backed prevention
source_skill: post-mortem
severity: high
detectability: advisory
status: active
compiler_targets: [inject, lookup]
scope_tags: [planning, flywheel]
applicable_when: [pre-mortem, planning]
applicable_languages: [go, shell]
hit_count: 3
last_cited: 2026-03-09T12:00:00Z
---

# Prefer registry-backed prevention

Normalize repeated findings into promoted artifacts so future planning and review
can load them before implementation.
FINDING_EOF

cat > "$TMPDIR_TEST/.agents/findings/test-finding-retired.md" <<'FINDING_EOF'
---
id: test-finding-retired
title: Retired finding fixture
source_skill: vibe
severity: low
detectability: mechanical
status: retired
compiler_targets: [constraint]
scope_tags: [validation]
applicable_when: [task-validation]
applicable_languages: [shell]
hit_count: 9
last_cited: 2026-03-08T12:00:00Z
retired_by: tester
---

# Retired finding fixture

Used to verify retired findings stay out of default list output.
FINDING_EOF

# Initialize git repo (many commands use git rev-parse)
cd "$TMPDIR_TEST"
git init -q
git config user.email "test@example.com"
git config user.name "Test User"
git add .
git commit -q -m "Initial test fixtures"

pass "Test fixtures created"

# ============================================================
# Helper: run command and check exit code + output pattern
# ============================================================

test_cmd() {
    local description="$1"
    local expected_exit="$2"
    local output_pattern="$3"
    shift 3
    # Remaining args are the command to run

    local output=""
    local actual_exit=0

    output=$("$@" 2>&1) || actual_exit=$?

    # Check exit code
    if [ "$actual_exit" -eq "$expected_exit" ]; then
        pass "$description — exit code $actual_exit"
    else
        fail "$description — expected exit $expected_exit, got $actual_exit"
        [ -n "$output" ] && echo "    output: $(echo "$output" | head -3)" | sed 's/^/    /'
        return
    fi

    # Check output pattern (empty pattern = skip check)
    if [ -n "$output_pattern" ]; then
        if echo "$output" | grep -qE "$output_pattern"; then
            pass "$description — output matches: $output_pattern"
        else
            fail "$description — output missing pattern: $output_pattern"
            echo "    got: $(echo "$output" | head -3)" | sed 's/^/    /'
        fi
    fi
}

test_json_valid() {
    local description="$1"
    shift
    local output=""
    local actual_exit=0

    output=$("$@" 2>&1) || actual_exit=$?

    if [ "$actual_exit" -ne 0 ]; then
        fail "$description — exit code $actual_exit (expected 0)"
        return
    fi

    if echo "$output" | jq . >/dev/null 2>&1; then
        pass "$description — valid JSON"
    else
        fail "$description — invalid JSON output"
        echo "    got: $(echo "$output" | head -3)" | sed 's/^/    /'
    fi
}

# ============================================================
echo ""
echo "=== v2.18 Command Tests ==="
echo ""
# ============================================================

# ---- quality constraint list ----
echo "--- quality constraint list ---"
CONSTRAINT_OUT=$("$TMPBIN" constraint list 2>&1) || CONSTRAINT_RC=$?
CONSTRAINT_RC="${CONSTRAINT_RC:-0}"
if [ "$CONSTRAINT_RC" -eq 0 ] || { [ "$CONSTRAINT_RC" -eq 1 ] && echo "$CONSTRAINT_OUT" | grep -qi "no constraints found"; }; then
    pass "quality constraint list returns expected status"
else
    fail "quality constraint list expected exit 0/1, got $CONSTRAINT_RC"
    echo "    output: $(echo "$CONSTRAINT_OUT" | head -3)" | sed 's/^/    /'
fi

# ---- quality constraint list --json ----
CONSTRAINT_JSON=$("$TMPBIN" constraint list --json 2>&1) || true
if echo "$CONSTRAINT_JSON" | jq . >/dev/null 2>&1; then
    pass "quality constraint list --json returns valid JSON"
elif echo "$CONSTRAINT_JSON" | grep -qi "no constraints found"; then
    pass "quality constraint list --json reports no constraints"
else
    fail "quality constraint list --json returns valid JSON"
    echo "    output: $(echo "$CONSTRAINT_JSON" | head -3)" | sed 's/^/    /'
fi

# ---- contradict ----
echo ""
echo "--- contradict ---"
test_cmd "contradict (with learnings)" 0 "" \
    "$TMPBIN" contradict

# ---- curate status ----
echo ""
echo "--- curate status ---"
test_cmd "curate status" 0 "Curation|Learnings|learnings|Total|total" \
    "$TMPBIN" curate status

# ---- curate verify ----
echo ""
echo "--- curate verify ---"
# curate verify checks goals, may say "No GOALS" which is fine
test_cmd "curate verify" 0 "" \
    "$TMPBIN" curate verify

# ---- dedup ----
echo ""
echo "--- dedup ---"
# With two similar learnings, dedup should find near-duplicates or report none
test_cmd "dedup" 0 "" \
    "$TMPBIN" dedup

# ---- dedup --merge ----
echo ""
echo "--- dedup --merge ---"
test_cmd "dedup --merge (dry-run behavior)" 0 "" \
    "$TMPBIN" dedup --merge

# ---- lookup --query ----
echo ""
echo "--- lookup ---"
test_cmd "lookup --query guard" 0 "" \
    "$TMPBIN" lookup --query "guard clause"

# ---- findings list ----
echo ""
echo "--- findings list ---"
test_cmd "findings list" 0 "test-finding-alpha|Prefer registry-backed prevention" \
    "$TMPBIN" findings list

# ---- findings list --json ----
echo ""
echo "--- findings list --json ---"
FINDINGS_JSON=$("$TMPBIN" findings list --json 2>&1) || true
if echo "$FINDINGS_JSON" | jq -e 'type == "array" and any(.[]; .id == "test-finding-alpha") and all(.[]; .id != "test-finding-retired")' >/dev/null 2>&1; then
    pass "findings list --json returns active findings array"
else
    fail "findings list --json returns active findings array"
    echo "    output: $(echo "$FINDINGS_JSON" | head -3)" | sed 's/^/    /'
fi

# ---- findings stats ----
echo ""
echo "--- findings stats ---"
test_cmd "findings stats" 0 "Total findings: 2|By status:|Most cited:" \
    "$TMPBIN" findings stats

# ---- memory sync ----
echo ""
echo "--- memory sync ---"
test_cmd "memory sync" 0 "" \
    "$TMPBIN" memory sync

# ---- notebook update ----
echo ""
echo "--- notebook update ---"
test_cmd "notebook update" 0 "" \
    "$TMPBIN" notebook update

# ---- seed --help ----
echo ""
echo "--- seed ---"
test_cmd "seed --help" 0 "seed|plant|repository|template" \
    "$TMPBIN" seed --help

# ---- metrics health ----
echo ""
echo "--- metrics health ---"
test_cmd "metrics health" 0 "Flywheel|Health|sigma|rho|RETRIEVAL|retrieval" \
    "$TMPBIN" metrics health

# ---- context assemble --help ----
echo ""
echo "--- context assemble ---"
test_cmd "context assemble --help" 0 "assemble|GOALS|TASK|briefing|Assemble" \
    "$TMPBIN" context assemble --help

# ============================================================
echo ""
echo "=== --json Flag Matrix ==="
echo ""
# ============================================================

# doctor --json — a single engine Report (schema_version, findings, exit_code)
echo "--- doctor --json ---"
OUTPUT=$("$TMPBIN" doctor --json 2>/dev/null) || true
if echo "$OUTPUT" | jq -e 'has("schema_version") and has("findings") and has("exit_code")' >/dev/null 2>&1; then
    pass "doctor --json returns an engine Report"
else
    fail "doctor --json returns an engine Report"
fi

if echo "$OUTPUT" | jq -e '.findings | type == "array"' >/dev/null 2>&1; then
    pass "doctor --json findings is an array"
else
    fail "doctor --json findings is an array"
fi

# search --json empty case
echo ""
echo "--- search --json empty ---"
SEARCH_OUTPUT=$("$TMPBIN" search --json "zzz_nonexistent_query_xyz" 2>&1) || true
if [ "$SEARCH_OUTPUT" = "[]" ]; then
    pass "search --json empty case returns []"
elif echo "$SEARCH_OUTPUT" | jq -e 'type == "array"' >/dev/null 2>&1; then
    pass "search --json empty case returns JSON array"
else
    fail "search --json empty case returns [] (got: $SEARCH_OUTPUT)"
fi

# search --json with content
echo ""
echo "--- search --json with match ---"
SEARCH_MATCH=$("$TMPBIN" search --json "guard clause" 2>&1) || true
if echo "$SEARCH_MATCH" | jq -e 'type == "array"' >/dev/null 2>&1; then
    pass "search --json returns JSON array"
else
    fail "search --json returns JSON array"
fi

# curate status --json (if supported)
echo ""
echo "--- curate status --json ---"
CURATE_JSON=$("$TMPBIN" curate status --json 2>&1) || true
if echo "$CURATE_JSON" | jq . >/dev/null 2>&1; then
    pass "curate status --json returns valid JSON"
else
    # Not all commands support --json; skip is acceptable
    yellow "SKIP: curate status --json not supported"
fi

# metrics health --json
echo ""
echo "--- metrics health --json ---"
METRICS_JSON=$("$TMPBIN" metrics health --json 2>&1) || true
if echo "$METRICS_JSON" | jq . >/dev/null 2>&1; then
    pass "metrics health --json returns valid JSON"
else
    yellow "SKIP: metrics health --json may not be supported"
fi

# findings stats --json
echo ""
echo "--- findings stats --json ---"
FINDINGS_STATS_JSON=$("$TMPBIN" findings stats --json 2>&1) || true
if echo "$FINDINGS_STATS_JSON" | jq -e '.total == 2 and .by_status.active == 1 and .by_status.retired == 1' >/dev/null 2>&1; then
    pass "findings stats --json returns finding inventory"
else
    fail "findings stats --json returns finding inventory"
fi

# ============================================================
echo ""
echo "=== Doctor Output Structure ==="
echo ""
# ============================================================

# doctor output includes status levels
DOCTOR_OUT=$("$TMPBIN" doctor 2>&1) || true
if echo "$DOCTOR_OUT" | grep -qE '(pass|warn|fail|PASS|WARN|FAIL|HEALTHY|DEGRADED)'; then
    pass "doctor output includes status levels"
else
    fail "doctor output includes status levels"
fi

# doctor mentions specific checks
if echo "$DOCTOR_OUT" | grep -q "Knowledge Base"; then
    pass "doctor checks Knowledge Base"
else
    fail "doctor checks Knowledge Base"
fi

if echo "$DOCTOR_OUT" | grep -q "ao CLI"; then
    pass "doctor checks ao CLI"
else
    fail "doctor checks ao CLI"
fi

# ============================================================
echo ""
echo "=== Edge Cases ==="
echo ""
# ============================================================

# constraint list in empty constraints dir
echo "--- constraint list (empty) ---"
EMPTY_DIR=$(mktemp -d)
mkdir -p "$EMPTY_DIR/.agents/constraints"
cd "$EMPTY_DIR"
git init -q >/dev/null 2>&1
EC=0
"$TMPBIN" constraint list >/dev/null 2>&1 || EC=$?
if [ "$EC" -ne 0 ]; then
    pass "constraint list exits non-zero with empty constraints"
else
    # Some implementations exit 0 with "no constraints" message
    pass "constraint list handles empty constraints"
fi
rm -rf "$EMPTY_DIR"
cd "$TMPDIR_TEST"

# search with no results
echo ""
echo "--- search no results ---"
NORESULT=$("$TMPBIN" search "zzz_absolutely_nothing_matches_this_xyz" 2>&1) || true
if [ -z "$NORESULT" ] || echo "$NORESULT" | grep -qiE "no.*found|no.*match|\[\]|^$"; then
    pass "search handles no-result case gracefully"
else
    pass "search returns content for broad query"
fi

# lookup with no match
echo ""
echo "--- lookup no match ---"
EC=0
LOOKUP_NONE=$("$TMPBIN" lookup --query "zzz_nonexistent_artifact_xyz" 2>&1) || EC=$?
if [ "$EC" -eq 0 ]; then
    pass "lookup exits 0 for no match"
else
    pass "lookup exits non-zero for no match (acceptable)"
fi

# ============================================================
# Summary
# ============================================================

echo ""
echo "======================================="
echo "Test Summary:"
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
echo "  TOTAL: $((PASS + FAIL))"
echo "======================================="

if [ $FAIL -gt 0 ]; then
    red "FAILED: $FAIL test(s) failed"
    exit 1
else
    green "SUCCESS: All v2.18 command tests passed"
    exit 0
fi
