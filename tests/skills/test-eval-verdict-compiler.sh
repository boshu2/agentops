#!/usr/bin/env bash
# Test for hooks/eval-verdict-compiler.sh.
# Wave-1: 5 critical assertions (shellcheck, improved-mutates-up, legacy-compat,
# dry-run-readonly, corpus-active-precondition).
# Wave-1.5: 3 more assertions (regressed-mutates-down, breach-queues-retire,
# idempotency-via-watermark) — covers the int_ge / threshold-breach branch
# that gets the bash-native rewrite from wave-1-residual|awk-portability-int-ge.

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

# Test 6: regressed verdict mutates utility downward + increments harmful_count
test_regressed() {
    local tmp learning manifest_dir
    tmp="$(make_temp_root)"
    learning="$tmp/learning.md"
    manifest_dir="$tmp/runs/run-regressed"
    mkdir -p "$manifest_dir"
    cp "$FIX_DIR/learning-existing.md" "$learning"
    jq --arg path "$learning" '.verdict.applicable_artifacts = [$path]' \
        "$FIX_DIR/manifest-regressed.json" > "$manifest_dir/manifest.json"
    AGENTOPS_EVALS_ROOT="$tmp" AGENTOPS_VERDICT_QUIET=1 \
        bash "$HOOK" --manifest "$manifest_dir/manifest.json" >/dev/null 2>&1 || {
            fail "regressed-mutates-downward: hook exited non-zero"; return; }
    local got harmful
    got="$(awk '/^utility:/ {gsub(/^utility:[[:space:]]*|[[:space:]]+$/,""); print; exit}' "$learning")"
    harmful="$(awk '/^harmful_count:/ {gsub(/^harmful_count:[[:space:]]*|[[:space:]]+$/,""); print; exit}' "$learning")"
    # EMA(0.7, 0.3) on (0.5, 0.1) = 0.7*0.5 + 0.3*0.1 = 0.380000
    [ "$got" = "0.380000" ] && pass "regressed-mutates-downward: utility 0.5 -> $got" \
        || fail "regressed-mutates-downward: expected 0.380000, got $got"
    [ "$harmful" = "1" ] && pass "regressed-mutates-downward: harmful_count 0 -> $harmful" \
        || fail "regressed-mutates-downward: expected 1, got $harmful"
}
test_regressed

# Test 7: harmful>=3 + utility<0.3 queues a verdict-driven retire candidate.
# This is the branch the int_ge bash-native rewrite (wave-1-residual|
# awk-portability-int-ge) protects from BSD-awk syntax noise.
test_breach_queues_retire() {
    local tmp learning manifest_dir
    tmp="$(make_temp_root)"
    learning="$tmp/learning.md"
    manifest_dir="$tmp/runs/run-breach"
    mkdir -p "$manifest_dir"
    cp "$FIX_DIR/learning-near-breach.md" "$learning"
    jq --arg path "$learning" '.verdict.applicable_artifacts = [$path]' \
        "$FIX_DIR/manifest-regressed-breach.json" > "$manifest_dir/manifest.json"
    # Run from inside $tmp so ROOT (git rev-parse fallback) lands in $tmp,
    # and the queued retire entry writes to $tmp/.agents/rpi/next-work.jsonl
    # rather than the actual repo's queue.
    (cd "$tmp" && AGENTOPS_EVALS_ROOT="$tmp" AGENTOPS_VERDICT_QUIET=1 \
        bash "$HOOK" --manifest "$manifest_dir/manifest.json" >/dev/null 2>&1) || {
            fail "breach-queues-retire: hook exited non-zero"; return; }
    local queue="$tmp/.agents/rpi/next-work.jsonl"
    if [ -s "$queue" ] && \
        jq -e '.source == "eval-verdict-compiler" and (.description | contains("verdict-driven retire candidate"))' \
            "$queue" >/dev/null 2>&1; then
        pass "breach-queues-retire: next-work entry written with verdict-driven retire"
    else
        fail "breach-queues-retire: expected verdict-driven retire entry in $queue"
    fi
}
test_breach_queues_retire

# Test 8: re-running the compiler on a watermarked manifest is a no-op
# (idempotency via processed.jsonl + find -newer gate).
test_idempotent() {
    local tmp learning manifest_dir
    tmp="$(make_temp_root)"
    learning="$tmp/learning.md"
    manifest_dir="$tmp/runs/run-idempotent"
    mkdir -p "$manifest_dir"
    cp "$FIX_DIR/learning-existing.md" "$learning"
    jq --arg path "$learning" '.verdict.applicable_artifacts = [$path]' \
        "$FIX_DIR/manifest-improved.json" > "$manifest_dir/manifest.json"
    # First invocation: drives the watermark and mutates utility 0.5 -> 0.605.
    AGENTOPS_EVALS_ROOT="$tmp" AGENTOPS_VERDICT_QUIET=1 \
        bash "$HOOK" >/dev/null 2>&1 || {
            fail "idempotency-via-watermark: first run exited non-zero"; return; }
    local mid
    mid="$(awk '/^utility:/ {gsub(/^utility:[[:space:]]*|[[:space:]]+$/,""); print; exit}' "$learning")"
    [ "$mid" = "0.605000" ] || { fail "idempotency-via-watermark: setup expected 0.605000 after first run, got $mid"; return; }
    # Second invocation without --manifest: corpus-active precondition checks
    # find -newer "$WATERMARK"; manifest is older, so the loop must skip.
    local before after
    before="$(shasum "$learning" | awk '{print $1}')"
    AGENTOPS_EVALS_ROOT="$tmp" AGENTOPS_VERDICT_QUIET=1 \
        bash "$HOOK" >/dev/null 2>&1 || {
            fail "idempotency-via-watermark: second run exited non-zero"; return; }
    after="$(shasum "$learning" | awk '{print $1}')"
    [ "$before" = "$after" ] && pass "idempotency-via-watermark: learning bit-identical across re-runs" \
        || fail "idempotency-via-watermark: learning re-mutated under second run (utility now $(awk '/^utility:/ {gsub(/^utility:[[:space:]]*|[[:space:]]+$/,""); print; exit}' "$learning"))"
}
test_idempotent

echo
echo "Total: PASS=$PASS FAIL=$FAIL"
[ "$FAIL" -eq 0 ]
