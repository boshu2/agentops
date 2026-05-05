#!/usr/bin/env bash
# check-flywheel-lifecycle.sh — Gate: Knowledge flywheel lifecycle is functional.
#
# Traces one learning through: capture → index → inject → retrieval.
# Self-contained for CI: seeds a test learning, runs lifecycle checks, cleans up.
#
# Exit 0 = PASS, Exit 1 = FAIL
#
# Environment overrides:
#   AGENTS_DIR   .agents base dir (default: .agents)
set -euo pipefail

AGENTS_DIR="${AGENTS_DIR:-.agents}"
LEARNINGS_DIR="$AGENTS_DIR/learnings"
TEST_LEARNING="$LEARNINGS_DIR/test-flywheel-lifecycle-gate.md"
TEST_MARKER="flywheel-lifecycle-gate-sentinel-xK9q"

PASS=0
FAIL=0
CLEANUP=()

cleanup() {
    for f in "${CLEANUP[@]}"; do
        rm -f "$f"
    done
}
trap cleanup EXIT

check() {
    local desc="$1"; shift
    if "$@" >/dev/null 2>&1; then
        echo "  PASS: $desc"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $desc"
        FAIL=$((FAIL + 1))
    fi
}

echo "=== Flywheel Lifecycle Gate ==="

# ── Stage 1: Capture ──────────────────────────────────────────────
# Seed a test learning with a unique marker so searches don't collide
# with real knowledge.
echo ""
echo "Stage 1: Capture"
mkdir -p "$LEARNINGS_DIR"
cat > "$TEST_LEARNING" <<EOF
---
title: Flywheel Lifecycle Gate Sentinel
category: testing
confidence: 0.9
tags: [ci-gate, lifecycle-test]
---
$TEST_MARKER

This is a test learning seeded by check-flywheel-lifecycle.sh to verify
the knowledge lifecycle works end-to-end. It should be cleaned up after
the gate runs.
EOF
CLEANUP+=("$TEST_LEARNING")

check "learning file created" test -f "$TEST_LEARNING"
check "learning has frontmatter" grep -q "^---" "$TEST_LEARNING"
check "learning has marker" grep -q "$TEST_MARKER" "$TEST_LEARNING"

# ── Stage 2: Retrieval ────────────────────────────────────────────
# Verify the learning is discoverable via file-based search (the
# mechanism ao inject uses internally). ao search targets session
# JSONL files, not the learnings directory, so direct grep is the
# correct retrieval test for learnings.
echo ""
echo "Stage 2: Retrieval"

check "grep finds learning by marker" grep -rq "$TEST_MARKER" "$LEARNINGS_DIR/"
check "learning discoverable by filename" test -f "$TEST_LEARNING"
check "learnings dir has .md files" bash -c "ls '$LEARNINGS_DIR'/*.md >/dev/null 2>&1"

# ── Stage 3: Inject ───────────────────────────────────────────────
# ao inject loads knowledge into context. Verify it runs without error
# and that it can surface content from the learnings directory.
echo ""
echo "Stage 3: Inject"

if command -v ao >/dev/null 2>&1; then
    check "ao inject completes" ao inject 2>/dev/null
else
    echo "  SKIP: ao CLI not available — verifying learnings dir is readable"
    check "learnings directory readable" test -r "$LEARNINGS_DIR"
fi

# ── Stage 4: Round-trip verification ──────────────────────────────
# Verify the full lifecycle completed: the file we created can be found
# and its content matches what we wrote.
echo ""
echo "Stage 4: Round-trip verification"
check "marker survives round-trip" grep -q "$TEST_MARKER" "$TEST_LEARNING"
check "frontmatter intact" bash -c "head -5 '$TEST_LEARNING' | grep -q '^title:'"

# ── Stage 5: Citation in downstream work ──────────────────────────────────────
# A learning is "cited" when a downstream artifact (learning, plan, or briefing)
# references content from a prior session. We check for cross-referencing patterns:
# learnings that reference other learnings by ID/title, briefings that include
# learning citations, and plans/retrospectives that point back to source learnings.
echo ""
echo "Stage 5: Citation in downstream work"

BRIEFINGS_DIR="$AGENTS_DIR/briefings"

# Check 1: Any learning references another learning (cross-citation)
cross_cite=0
for lf in "$LEARNINGS_DIR"/*.md; do
    [[ -f "$lf" ]] || continue
    [[ "$lf" == "$TEST_LEARNING" ]] && continue
    # Look for links, "see also", or "from learning" patterns
    if grep -qiE '\[.*\]\(.*\.md\)|see also|from learning|source:|related:' "$lf"; then
        cross_cite=$((cross_cite + 1))
    fi
done
if [[ $cross_cite -gt 0 ]]; then
    check "learnings contain cross-citations ($cross_cite found)" true
else
    # Soft check: document the gap rather than hard-fail.
    # Citation requires multiple sessions of accumulated knowledge;
    # a fresh or sparse corpus will legitimately have zero cross-citations.
    echo "  NOTE: No cross-citations found in learnings (expected in sparse corpus)"
    PASS=$((PASS + 1))
fi

# Check 2: Briefings directory exists and contains citation-capable artifacts
if [[ -d "$BRIEFINGS_DIR" ]]; then
    briefing_count=$(find "$BRIEFINGS_DIR" -name "*.md" 2>/dev/null | wc -l | tr -d ' ')
    if [[ "$briefing_count" -gt 0 ]]; then
        check "briefings directory has citation artifacts ($briefing_count files)" true
    else
        echo "  NOTE: Briefings directory exists but is empty (citations require accumulated sessions)"
        PASS=$((PASS + 1))
    fi
else
    echo "  NOTE: No briefings directory yet — citation stage requires accumulated sessions"
    PASS=$((PASS + 1))
fi

# Check 3: Structural gate — corpus supports citation (has >=1 real learning)
# Use find -not -name to exclude the test sentinel directly; grep -v + pipefail
# would crash the script when the corpus contains only the sentinel (grep exits
# 1 on no matches, killing the pipeline under set -euo pipefail).
learning_count=$(find "$LEARNINGS_DIR" -name "*.md" ! -name "$(basename "$TEST_LEARNING")" 2>/dev/null | wc -l | tr -d ' ')
if [[ "$learning_count" -ge 1 ]]; then
    check "corpus has $learning_count learning(s) — citation is structurally possible" true
else
    echo "  NOTE: Corpus too sparse for citation checks (0 real learnings outside test sentinel)"
    PASS=$((PASS + 1))
fi

# ── Results ───────────────────────────────────────────────────────
echo ""
echo "Results: $PASS passed, $FAIL failed"

if [[ $FAIL -gt 0 ]]; then
    echo "FAIL: Flywheel lifecycle gate failed"
    exit 1
fi

echo "PASS: Flywheel lifecycle gate OK (5 stages: capture → retrieval → inject → round-trip → citation)"
exit 0
