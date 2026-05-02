#!/usr/bin/env bash
# Test for hooks/eval-verdict-compiler.sh.
# Wave-1: 5 critical assertions (shellcheck, improved-mutates-up, legacy-compat,
# dry-run-readonly, corpus-active-precondition).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HOOK="$REPO_ROOT/hooks/eval-verdict-compiler.sh"
FIX_DIR="$REPO_ROOT/tests/fixtures/eval-verdicts"

PASS=0; FAIL=0
pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

# Test 1: shellcheck
if shellcheck --severity=error "$HOOK" 2>&1; then
    pass "shellcheck clean on $HOOK"
else
    fail "shellcheck flagged errors on $HOOK"
fi

make_temp_root() { local tmp; tmp="$(mktemp -d)"; mkdir -p "$tmp/runs"; printf '%s\n' "$tmp"; }

# Test 2: improved verdict mutates utility upward (EMA)
test_improved() {
    local tmp learning manifest_dir
    tmp="$(make_temp_root)"
    learning="$tmp/learning.md"
    manifest_dir="$tmp/runs/run-improved"
    mkdir -p "$manifest_dir"
    cp "$FIX_DIR/learning-existing.md" "$learning"
    jq --arg path "$learning" '.verdict.applicable_artifacts = [$path]' \
        "$FIX_DIR/manifest-improved.json" > "$manifest_dir/manifest.json"
    AGENTOPS_EVALS_ROOT="$tmp" AGENTOPS_VERDICT_QUIET=1 \
        bash "$HOOK" --manifest "$manifest_dir/manifest.json" >/dev/null 2>&1 || {
            fail "improved-mutates-upward: hook exited non-zero"; return; }
    local got reward
    got="$(awk '/^utility:/ {gsub(/^utility:[[:space:]]*|[[:space:]]+$/,""); print; exit}' "$learning")"
    reward="$(awk '/^reward_count:/ {gsub(/^reward_count:[[:space:]]*|[[:space:]]+$/,""); print; exit}' "$learning")"
    [ "$got" = "0.605000" ] && pass "improved-mutates-upward: utility 0.5 -> $got" \
        || fail "improved-mutates-upward: expected 0.605000, got $got"
    [ "$reward" = "1" ] && pass "improved-mutates-upward: reward_count 0 -> $reward" \
        || fail "improved-mutates-upward: expected 1, got $reward"
}
test_improved

# Test 3: legacy string-form verdict (pre-mortem C1)
test_legacy() {
    local tmp learning manifest_dir
    tmp="$(make_temp_root)"
    learning="$tmp/learning.md"
    manifest_dir="$tmp/runs/run-legacy"
    mkdir -p "$manifest_dir"
    cp "$FIX_DIR/learning-existing.md" "$learning"
    cp "$FIX_DIR/manifest-legacy-string-verdict.json" "$manifest_dir/manifest.json"
    if AGENTOPS_EVALS_ROOT="$tmp" AGENTOPS_VERDICT_QUIET=1 \
        bash "$HOOK" --manifest "$manifest_dir/manifest.json" >/dev/null 2>&1; then
        pass "legacy-string-verdict: hook accepts string-form (exit 0)"
    else
        fail "legacy-string-verdict: hook errored on string-form"
    fi
}
test_legacy

# Test 4: --dry-run is read-only
test_dry_run() {
    local tmp learning manifest_dir before after
    tmp="$(make_temp_root)"
    learning="$tmp/learning.md"
    manifest_dir="$tmp/runs/run-dry"
    mkdir -p "$manifest_dir"
    cp "$FIX_DIR/learning-existing.md" "$learning"
    jq --arg path "$learning" '.verdict.applicable_artifacts = [$path]' \
        "$FIX_DIR/manifest-improved.json" > "$manifest_dir/manifest.json"
    before="$(shasum "$learning" | awk '{print $1}')"
    AGENTOPS_EVALS_ROOT="$tmp" AGENTOPS_VERDICT_QUIET=1 \
        bash "$HOOK" --dry-run --manifest "$manifest_dir/manifest.json" >/dev/null 2>&1 || {
            fail "dry-run-readonly: hook exited non-zero"; return; }
    after="$(shasum "$learning" | awk '{print $1}')"
    [ "$before" = "$after" ] && pass "dry-run-readonly: learning bit-identical pre/post" \
        || fail "dry-run-readonly: learning mutated under --dry-run"
    [ ! -f "$tmp/processed.jsonl" ] && pass "dry-run-readonly: watermark not written" \
        || fail "dry-run-readonly: watermark appeared under --dry-run"
}
test_dry_run

# Test 5: corpus-active precondition
test_corpus_active() {
    local tmp; tmp="$(make_temp_root)"
    if AGENTOPS_EVALS_ROOT="$tmp" AGENTOPS_VERDICT_QUIET=1 \
        bash "$HOOK" >/dev/null 2>&1; then
        pass "corpus-active-precondition: silent exit 0 with no manifests"
    else
        fail "corpus-active-precondition: hook errored with no manifests"
    fi
}
test_corpus_active

echo
echo "Total: PASS=$PASS FAIL=$FAIL"
[ "$FAIL" -eq 0 ]
