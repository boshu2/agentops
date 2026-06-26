#!/usr/bin/env bash
# shellcheck disable=SC2016
# test-allowlist-negative.sh — legacy filename for council contract negatives.
#
# Council no longer owns reference allowlists. The in-repo skill is an extracted
# routing stub, and validate-skill.sh now enforces the current repo contract:
# council must point mixed-model panels at /dual-pane-atm and must not resurrect
# obsolete --technique/--profile flag rows.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
VALIDATE="$SCRIPT_DIR/validate-skill.sh"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

PASS=0
FAIL=0

TMPROOT="$(mktemp -d)"
trap 'rm -rf "$TMPROOT"' EXIT

pass() {
    echo -e "  ${GREEN}✓${NC} $1"
    PASS=$((PASS + 1))
}

fail() {
    echo -e "  ${RED}✗${NC} $1"
    shift || true
    if [ "$#" -gt 0 ]; then
        printf '%s\n' "$@"
    fi
    FAIL=$((FAIL + 1))
}

make_council_fixture() {
    local dir="$1"
    local body="$2"

    mkdir -p "$dir"
    cat > "$dir/SKILL.md" <<EOF
---
name: council
description: Test council fixture.
---

# council fixture

$body
EOF
}

assert_contains() {
    local desc="$1"
    local file="$2"
    local needle="$3"

    if grep -Fq -- "$needle" "$file"; then
        pass "$desc"
    else
        fail "$desc" "Expected output to contain: $needle" "Output was:" "$(cat "$file")"
    fi
}

assert_passes() {
    local desc="$1"
    local out="$2"
    shift 2

    if "$@" >"$out" 2>&1; then
        pass "$desc"
    else
        fail "$desc" "Expected success, got failure. Output was:" "$(cat "$out")"
    fi
}

assert_fails() {
    local desc="$1"
    local out="$2"
    shift 2

    if "$@" >"$out" 2>&1; then
        fail "$desc" "Expected failure, got success. Output was:" "$(cat "$out")"
    else
        pass "$desc"
    fi
}

echo "=== Council Contract Negative Fixture Tests ==="

DELEGATION_MSG="Council: delegates the mixed-model duel substrate to /dual-pane-atm"
OBSOLETE_ABSENT_MSG="Council: obsolete --technique/--profile flag rows absent"
OBSOLETE_FAIL_MSG="Council: obsolete --technique/--profile flag rows must not return"

echo ""
echo "Test 1: Repo council validation passes current contract"
repo_out="$TMPROOT/repo-council.out"
assert_passes "repo council validates" "$repo_out" "$VALIDATE" "$REPO_ROOT/skills/council"
assert_contains "repo council reports /dual-pane-atm delegation" "$repo_out" "$DELEGATION_MSG"
assert_contains "repo council reports obsolete rows absent" "$repo_out" "$OBSOLETE_ABSENT_MSG"

echo ""
echo "Test 2: Minimal current-contract fixture passes"
good_dir="$TMPROOT/good/council"
good_out="$TMPROOT/good.out"
make_council_fixture "$good_dir" 'Mixed-model panels delegate to `/dual-pane-atm`.'
assert_passes "minimal council fixture validates" "$good_out" "$VALIDATE" "$good_dir"
assert_contains "minimal fixture reports /dual-pane-atm delegation" "$good_out" "$DELEGATION_MSG"
assert_contains "minimal fixture reports obsolete rows absent" "$good_out" "$OBSOLETE_ABSENT_MSG"

echo ""
echo "Test 3: Missing /dual-pane-atm delegation fails"
missing_delegate_dir="$TMPROOT/missing-delegate/council"
missing_delegate_out="$TMPROOT/missing-delegate.out"
make_council_fixture "$missing_delegate_dir" 'Mixed-model panels are described without the required delegation pointer.'
assert_fails "missing /dual-pane-atm pointer fails" "$missing_delegate_out" "$VALIDATE" "$missing_delegate_dir"
assert_contains "missing pointer failure reports delegation contract" "$missing_delegate_out" "$DELEGATION_MSG"

echo ""
echo "Test 4: Obsolete --technique flag row fails"
technique_dir="$TMPROOT/obsolete-technique/council"
technique_out="$TMPROOT/obsolete-technique.out"
make_council_fixture "$technique_dir" 'Mixed-model panels delegate to `/dual-pane-atm`.

| Flag | Meaning |
|------|---------|
| `--technique=<name>` | obsolete council flag row |'
assert_fails "obsolete --technique row fails" "$technique_out" "$VALIDATE" "$technique_dir"
assert_contains "obsolete --technique failure reports moved flag contract" "$technique_out" "$OBSOLETE_FAIL_MSG"

echo ""
echo "Test 5: Obsolete --profile flag row fails"
profile_dir="$TMPROOT/obsolete-profile/council"
profile_out="$TMPROOT/obsolete-profile.out"
make_council_fixture "$profile_dir" 'Mixed-model panels delegate to `/dual-pane-atm`.

| Flag | Meaning |
|------|---------|
| `--profile=<name>` | obsolete council flag row |'
assert_fails "obsolete --profile row fails" "$profile_out" "$VALIDATE" "$profile_dir"
assert_contains "obsolete --profile failure reports moved flag contract" "$profile_out" "$OBSOLETE_FAIL_MSG"

echo ""
TOTAL=$((PASS + FAIL))
echo "=== Results: $PASS/$TOTAL passed ==="

if [ "$FAIL" -gt 0 ]; then
    echo -e "${RED}$FAIL test(s) failed${NC}"
    exit 1
fi

echo -e "${GREEN}All tests passed${NC}"
