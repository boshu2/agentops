#!/usr/bin/env bats
# Regression: jq | wc -l substitutions under `set -euo pipefail` must not
# abort the whole script when jq exits non-zero on a malformed JSON line.
# A bare `jq 2>/dev/null | wc -l` propagates jq's failure through pipefail,
# and set -e aborts mid-run (partial state). The fix appends `|| echo 0`
# so the count defaults to 0 and the script continues. (ag-1j1)

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    EVOLVE="$REPO_ROOT/scripts/evolve-capture-daily-learning.sh"
    MATURITY="$REPO_ROOT/scripts/bootstrap-maturity.sh"

    TMP_DIR="$(mktemp -d)"
}

teardown() {
    rm -rf "$TMP_DIR"
}

# --- bootstrap-maturity.sh -------------------------------------------------

@test "bootstrap-maturity.sh: malformed JSONL line does not abort the script" {
    LEARN="$TMP_DIR/learnings"
    mkdir -p "$LEARN"

    # First line is valid; second line is not JSON at all → jq exits non-zero.
    cat > "$LEARN/broken.jsonl" <<'EOF'
{"id":"a","claim":"thing1"}
this is not json at all
EOF

    run bash "$MATURITY" "$LEARN"
    # Without the `|| echo 0` guard the `missing=$(jq ... | wc -l ...)`
    # substitution would propagate jq's non-zero exit through pipefail and
    # set -e would abort before the summary lines print.
    [ "$status" -eq 0 ]
    # The final summary is only reached if the script ran to completion.
    [[ "$output" == *"Bootstrap maturity complete:"* ]]
}

# --- evolve-capture-daily-learning.sh --------------------------------------

@test "evolve-capture-daily-learning.sh: malformed cycle-history line does not abort" {
    EVODIR="$TMP_DIR/repo"
    mkdir -p "$EVODIR/.agents/evolve" "$EVODIR/.agents/learnings" "$EVODIR/scripts"
    # Copy the script into a throwaway repo so REPO_ROOT resolves there and
    # the script writes only inside $TMP_DIR.
    cp "$EVOLVE" "$EVODIR/scripts/evolve-capture-daily-learning.sh"

    DATE="2026-05-28"
    # cycle-history.jsonl: one valid object, then a malformed line. The
    # PRODUCTIVE/SCOUTS/IDLE/REGRESSED counts run jq -r over this and pipe to
    # wc -l; jq exits non-zero on the malformed line.
    cat > "$EVODIR/.agents/evolve/cycle-history.jsonl" <<EOF
{"started_at":"${DATE}T01:00:00Z","cycle":1,"result":"improved","title":"good"}
not valid json here
EOF

    run bash "$EVODIR/scripts/evolve-capture-daily-learning.sh" "$DATE"
    # Without the guard, set -e aborts at the first jq|wc line and the
    # consolidated file is never written.
    [ "$status" -eq 0 ]
    [ -f "$EVODIR/.agents/learnings/${DATE}-evolve-loop-learnings.md" ]
}

@test "evolve-capture-daily-learning.sh: counts default to 0 when jq fails on every line" {
    EVODIR="$TMP_DIR/repo2"
    mkdir -p "$EVODIR/.agents/evolve" "$EVODIR/.agents/learnings" "$EVODIR/scripts"
    cp "$EVOLVE" "$EVODIR/scripts/evolve-capture-daily-learning.sh"

    DATE="2026-05-28"
    # Entirely malformed history → jq fails; the guarded substitution must
    # yield 0 for every count, and the script must still complete.
    cat > "$EVODIR/.agents/evolve/cycle-history.jsonl" <<'EOF'
totally not json
also not json
EOF

    run bash "$EVODIR/scripts/evolve-capture-daily-learning.sh" "$DATE"
    [ "$status" -eq 0 ]

    OUT="$EVODIR/.agents/learnings/${DATE}-evolve-loop-learnings.md"
    [ -f "$OUT" ]
    # The Counts table must render 0 for the derived metrics rather than a
    # blank/crashed value.
    run grep -E '^\| Productive \(commit landed\) \| 0 \|' "$OUT"
    [ "$status" -eq 0 ]
}

# Direct test of the guarded jq|wc expression shape used in lines 59-62.
# In normal operation TODAY_CYCLES is pre-sanitized (the line-55 jq filter
# carries `|| true`), so these lines only see well-formed objects. But the
# expression must remain abort-proof under `set -euo pipefail` if malformed
# content ever reaches it (defense-in-depth, matching the already-guarded
# line 58). This pins the guard so a future edit cannot silently drop it.
@test "guarded jq|wc expression: malformed input does not abort under set -euo pipefail" {
    # Fully malformed input: jq emits nothing then exits non-zero. With the
    # guard the substitution completes and the count is 0 (wc -l of empty
    # input is 0; the `|| echo 0` covers jq's non-zero pipefail exit).
    run bash -c '
        set -euo pipefail
        CYCLES="this line is not valid json
neither is this one"
        COUNT=$(printf "%s\n" "$CYCLES" | jq -r "select(.result==\"improved\") | .cycle" 2>/dev/null | wc -l | tr -d " " || echo 0)
        echo "done COUNT=$COUNT"
    '
    [ "$status" -eq 0 ]
    # The guard ran to completion. The count resolves to 0 (either wc's "0"
    # for empty jq output, or the `|| echo 0` fallback).
    [[ "$output" == *"done COUNT="* ]]
    [[ "$output" == *"0"* ]]
}

@test "unguarded jq|wc expression: malformed input aborts under set -euo pipefail (negative control)" {
    # Same expression WITHOUT the `|| echo 0` guard must abort before "done".
    run bash -c '
        set -euo pipefail
        CYCLES="{\"result\":\"improved\",\"cycle\":1}
this line is not valid json"
        COUNT=$(printf "%s\n" "$CYCLES" | jq -r "select(.result==\"improved\") | .cycle" 2>/dev/null | wc -l | tr -d " ")
        echo "done COUNT=$COUNT"
    '
    [ "$status" -ne 0 ]
    [[ "$output" != *"done"* ]]
}
