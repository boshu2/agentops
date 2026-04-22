#!/usr/bin/env bash
# test-finding-prevention-ratchet.sh - End-to-end regression for the finding prevention ratchet
# Covers registry intake -> promoted artifact -> compiled outputs -> task-validation enforcement -> citation feedback

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HOOKS_DIR="$REPO_ROOT/hooks"

PASS=0
FAIL=0
SKIP=0

pass() {
    echo "PASS: $1"
    PASS=$((PASS + 1))
}

fail() {
    echo "FAIL: $1"
    FAIL=$((FAIL + 1))
}

skip() {
    echo "SKIP: $1"
    SKIP=$((SKIP + 1))
}

if ! command -v git >/dev/null 2>&1; then
    echo "ERROR: git is required"
    exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "ERROR: jq is required"
    exit 1
fi

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

BIN_DIR="$TMPDIR/bin"
mkdir -p "$BIN_DIR"
AO_BIN=""

if [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
    AO_BIN="$REPO_ROOT/cli/bin/ao"
    pass "using prebuilt ao binary for prevention-ratchet test"
elif command -v go >/dev/null 2>&1; then
    BUILD_AO_BIN="$BIN_DIR/ao"
    BUILD_LOG="$TMPDIR/ao-build.log"
    if (cd "$REPO_ROOT/cli" && go build -o "$BUILD_AO_BIN" ./cmd/ao > /dev/null 2>"$BUILD_LOG"); then
        AO_BIN="$BUILD_AO_BIN"
        pass "built ao binary for prevention-ratchet test"
    else
        BUILD_REASON="$(head -1 "$BUILD_LOG" 2>/dev/null || true)"
        skip "unable to build repo ao binary for prevention-ratchet test${BUILD_REASON:+ ($BUILD_REASON)}"
        echo
        echo "Results: $PASS passed, $FAIL failed, $SKIP skipped"
        exit 0
    fi
else
    skip "go toolchain unavailable and cli/bin/ao is not built"
    echo
    echo "Results: $PASS passed, $FAIL failed, $SKIP skipped"
    exit 0
fi

WORK_REPO="$TMPDIR/repo"
mkdir -p "$WORK_REPO/.agents/findings" "$WORK_REPO/.agents/ao" "$WORK_REPO/docs"
git -C "$WORK_REPO" init -q >/dev/null 2>&1
git -C "$WORK_REPO" config user.email "test@example.com"
git -C "$WORK_REPO" config user.name "Test User"

cat > "$WORK_REPO/.agents/findings/registry.jsonl" <<'EOF'
{"id":"f-end-to-end","version":1,"tier":"local","source":{"repo":"agentops/crew/nami","session":"2026-03-09","file":".agents/council/finding.md","skill":"post-mortem"},"date":"2026-03-09","severity":"significant","category":"validation-gap","pattern":"Compiled findings should become preventive artifacts before the next task repeats the same miss.","detection_question":"Did the next cycle load or compile the finding before implementation?","checklist_item":"Promote the finding into artifacts the next planning and validation step can consume.","applicable_languages":["markdown","shell"],"applicable_when":["plan-shape","validation-gap"],"status":"active","superseded_by":null,"dedup_key":"validation-gap|compiled-findings-prevent-repeat-miss|plan-shape","hit_count":0,"last_cited":null,"ttl_days":30,"confidence":"high"}
EOF

if (cd "$WORK_REPO" && bash "$HOOKS_DIR/finding-compiler.sh" >/dev/null 2>&1); then
    pass "finding-compiler ingests registry rows"
else
    fail "finding-compiler ingests registry rows"
fi

for path in \
    "$WORK_REPO/.agents/findings/f-end-to-end.md" \
    "$WORK_REPO/.agents/planning-rules/f-end-to-end.md" \
    "$WORK_REPO/.agents/pre-mortem-checks/f-end-to-end.md"; do
    if [ -f "$path" ]; then
        pass "compiled artifact exists: ${path#"$WORK_REPO"/}"
    else
        fail "compiled artifact exists: ${path#"$WORK_REPO"/}"
    fi
done

# Registry intake currently promotes advisory artifacts by default. Upgrade the promoted
# artifact to an active mechanical finding so the same test covers the runtime enforcement path.
cat > "$WORK_REPO/.agents/findings/f-end-to-end.md" <<'EOF'
---
id: f-end-to-end
type: finding
title: Require SAFE_MARKER before task completion
summary: Require SAFE_MARKER before task completion.
pattern: Require SAFE_MARKER before task completion.
detection_question: Does the changed docs file include SAFE_MARKER?
checklist_item: Ensure SAFE_MARKER is present before task completion.
severity: significant
detectability: mechanical
status: active
compiler_targets: [plan, pre-mortem, constraint]
scope_tags: [validation-gap, task-validation]
applicable_when: [validation-gap, task-validation]
applicable_languages: [markdown, shell]
hit_count: 0
last_cited:
compiler:
  applies_to:
    scope: files
    issue_types: [feature]
    path_globs: [docs/*.md]
    languages: [markdown]
  detector:
    kind: content_pattern
    mode: must_contain
    pattern: SAFE_MARKER
    message: SAFE_MARKER required
---

# Require SAFE_MARKER before task completion

Mechanical promotion of the registry finding for end-to-end prevention-ratchet coverage.
EOF

if (
    cd "$WORK_REPO" && \
    bash "$HOOKS_DIR/finding-compiler.sh" "$WORK_REPO/.agents/findings/f-end-to-end.md" >/dev/null 2>&1
); then
    pass "finding-compiler recompiles promoted mechanical artifact"
else
    fail "finding-compiler recompiles promoted mechanical artifact"
fi

if jq -e '.constraints[] | select(.id == "f-end-to-end" and .status == "active" and .detector.kind == "content_pattern")' \
    "$WORK_REPO/.agents/constraints/index.json" >/dev/null 2>&1; then
    pass "constraint index contains active compiled finding"
else
    fail "constraint index contains active compiled finding"
fi

printf 'hello\n' > "$WORK_REPO/docs/guide.md"
EC=0
OUTPUT=$(
    cd "$WORK_REPO" && \
    printf '%s' '{"metadata":{"issue_type":"feature","files":["docs/guide.md"]}}' | \
        bash "$HOOKS_DIR/task-validation-gate.sh" 2>&1
) || EC=$?

if [ "$EC" -eq 2 ] && echo "$OUTPUT" | grep -q 'SAFE_MARKER required'; then
    pass "task-validation enforces compiled constraint"
else
    fail "task-validation enforces compiled constraint"
fi

cat > "$WORK_REPO/.agents/ao/citations.jsonl" <<'EOF'
{"artifact_path":".agents/findings/f-end-to-end.md","session_id":"session-e2e","cited_at":"2026-03-10T01:00:00Z","citation_type":"applied","feedback_given":false}
EOF

cat > "$WORK_REPO/.agents/ao/last-session-outcome.json" <<'EOF'
{"outcome":"success"}
EOF

if (
    cd "$WORK_REPO" && \
    PATH="$BIN_DIR:/usr/bin:/bin:$PATH" \
    "$AO_BIN" flywheel close-loop --quiet >/dev/null 2>&1
); then
    pass "flywheel close-loop applies citation feedback to finding artifacts"
else
    fail "flywheel close-loop applies citation feedback to finding artifacts"
fi

if grep -q 'hit_count: 1' "$WORK_REPO/.agents/findings/f-end-to-end.md" && \
    grep -q 'last_cited: 2026-03-10T01:00:00Z' "$WORK_REPO/.agents/findings/f-end-to-end.md"; then
    pass "finding artifact lifecycle fields update after citation feedback"
else
    fail "finding artifact lifecycle fields update after citation feedback"
fi

if jq -e '.feedback_given == true' "$WORK_REPO/.agents/ao/citations.jsonl" >/dev/null 2>&1; then
    pass "citations are marked feedback_given after close-loop"
else
    fail "citations are marked feedback_given after close-loop"
fi

echo
echo "Results: $PASS passed, $FAIL failed, $SKIP skipped"
[ "$FAIL" -eq 0 ]
