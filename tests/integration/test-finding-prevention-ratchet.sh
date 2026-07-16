#!/usr/bin/env bash
# test-finding-prevention-ratchet.sh - End-to-end regression for the finding prevention ratchet
# Covers evidence-backed catches -> distinct-objective recurrence -> advisory producer candidate.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

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

if command -v go >/dev/null 2>&1; then
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
elif [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
    AO_BIN="$REPO_ROOT/cli/bin/ao"
    pass "using prebuilt ao binary because the Go toolchain is unavailable"
else
    skip "go toolchain unavailable and cli/bin/ao is not built"
    echo
    echo "Results: $PASS passed, $FAIL failed, $SKIP skipped"
    exit 0
fi

WORK_REPO="$TMPDIR/repo"
mkdir -p "$WORK_REPO/docs"
git -C "$WORK_REPO" init -q >/dev/null 2>&1
git -C "$WORK_REPO" config user.email "test@example.com"
git -C "$WORK_REPO" config user.name "Test User"

# Learn/bookkeeper recurrence is objective-scoped: four attempts in objective-a
# plus one catch in objective-b are two occurrences, not five. The one-off class
# must remain an observation and create no policy candidate.
for head in \
    aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1 \
    aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa2 \
    aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa3 \
    aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa4; do
    if ! (
        cd "$WORK_REPO" &&
        "$AO_BIN" membrane catch \
            --bead objective-a \
            --domain docs \
            --class stale-surface \
            --reason "A retired surface remained in active documentation." \
            --paths docs/guide.md \
            --head "$head" >/dev/null
    ); then
        fail "records retry observation for objective-a"
    fi
done

if (
    cd "$WORK_REPO" &&
    "$AO_BIN" membrane catch \
        --bead objective-b \
        --domain docs \
        --class stale-surface \
        --reason "A retired surface remained in active documentation." \
        --paths docs/guide.md \
        --head bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb >/dev/null
); then
    pass "records the same finding class in a second objective"
else
    fail "records the same finding class in a second objective"
fi

if (
    cd "$WORK_REPO" &&
    "$AO_BIN" membrane catch \
        --bead objective-c \
        --domain shell \
        --class one-off \
        --reason "A one-off shell typo was caught." \
        --paths scripts/example.sh \
        --head cccccccccccccccccccccccccccccccccccccccc >/dev/null
); then
    pass "records one-off finding without promotion"
else
    fail "records one-off finding without promotion"
fi

DIGEST_JSON="$(cd "$WORK_REPO" && "$AO_BIN" membrane digest --json)"
if printf '%s' "$DIGEST_JSON" | jq -e '
    .producer_candidates as $c
    | ($c | length) == 1
      and $c[0].advisory == true
      and $c[0].recurrence_count == 2
      and ($c[0].evidence | map(.objective_id) == ["objective-a", "objective-b"])
' >/dev/null; then
    pass "distinct objectives create one advisory producer candidate without retry inflation"
else
    fail "distinct objectives create one advisory producer candidate without retry inflation"
fi

echo
echo "Results: $PASS passed, $FAIL failed, $SKIP skipped"
[ "$FAIL" -eq 0 ]
