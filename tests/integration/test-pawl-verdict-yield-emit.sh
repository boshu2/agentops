#!/usr/bin/env bash
# test-pawl-verdict-yield-emit.sh — age-uxva acceptance.
#
# Proves the pawl-verdict.sh `write` chokepoint emits a yield-ledger gate-verdict
# for every PANEL verdict (so catches are logged even with no merge attempt):
#   1. a REFUTED cross-family verdict → one gate-verdict event (REFUTED, cross_family=true)
#   2. emission is idempotent on re-run (no duplicate)
#   3. a CONFIRMED verdict also emits
#   4. family normalization: codex+gpt (same canonical family) → cross_family=false
#   5. the canonical checkout is not dirtied (ledger lives under gitignored .agents/)
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PAWL_VERDICT="$ROOT/scripts/pawl-verdict.sh"
# Prefer a freshly-built ao; fall back to PATH.
AO_BUILT="$ROOT/cli/bin/ao"

fails=0
check() { if eval "$2"; then echo "PASS: $1"; else echo "FAIL: $1"; fails=$((fails + 1)); fi; }

command -v jq >/dev/null 2>&1 || { echo "SKIP: jq not available"; exit 0; }
if [[ ! -x "$AO_BUILT" ]] && ! command -v ao >/dev/null 2>&1; then
  echo "SKIP: no ao binary (build cli/bin/ao first)"; exit 0
fi

TMP="$(mktemp -d)"
cleanup() { chmod -R u+w "$TMP" 2>/dev/null || true; /bin/rm -rf "$TMP" 2>/dev/null || true; }
trap cleanup EXIT

# Put the built ao on PATH (the script calls `ao` by name).
BIN="$TMP/bin"; mkdir -p "$BIN"
if [[ -x "$AO_BUILT" ]]; then ln -sf "$AO_BUILT" "$BIN/ao"; fi
export PATH="$BIN:$PATH"

# sandbox git repo
REPO="$TMP/repo"
mkdir -p "$REPO"; cd "$REPO" || exit 1
git init -q -b main; git config user.email t@t; git config user.name t
mkdir -p .agents/yield .agents/pawl-verdicts docs/provenance scripts
# Copy the script under test INTO the sandbox so its REPO_ROOT (SCRIPT_DIR/..)
# resolves to the sandbox, not the real repo (pawl-verdict.sh is root-relative
# to its own location, not cwd).
cp "$PAWL_VERDICT" scripts/pawl-verdict.sh; chmod +x scripts/pawl-verdict.sh
PAWL_VERDICT="$REPO/scripts/pawl-verdict.sh"
echo seed > seed.txt; git add -A; git commit -qm seed --no-verify
HEAD_SHA="$(git rev-parse HEAD)"
LEDGER="$REPO/.agents/yield/yield-ledger.jsonl"

# helper: count gate-verdict events for a bead+disposition
gv_count() { jq -c --arg b "$1" --arg d "$2" 'select(.event=="gate-verdict" and .bead_id==$b and .body.disposition==$d)' "$LEDGER" 2>/dev/null | grep -c . ; }

# 1) REFUTED cross-family verdict (claude + gpt), no merge
echo ev1 > "$TMP/ev1"; echo ev2 > "$TMP/ev2"
bash "$PAWL_VERDICT" write age-test-r 0 \
  --disposition REFUTED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "claude:REFUTED:ctx-claude:$TMP/ev1" \
  --refuter "gpt:REFUTED:ctx-gpt:$TMP/ev2" \
  --domain concurrency --reason "missed a fail-open" >/dev/null 2>&1 || true

check "REFUTED verdict emitted exactly one gate-verdict event" \
  "[ \"\$(gv_count age-test-r REFUTED)\" = '1' ]"
check "the event carries cross_family=true (2 distinct families)" \
  "jq -e 'select(.bead_id==\"age-test-r\") | .body.cross_family==true' '$LEDGER' >/dev/null"
check "refuter_families normalized to [claude,gpt]" \
  "[ \"\$(jq -r 'select(.bead_id==\"age-test-r\") | .body.refuter_families | sort | join(\",\")' '$LEDGER')\" = 'claude,gpt' ]"
check "what-was-missed reason recorded" \
  "jq -e 'select(.bead_id==\"age-test-r\") | .body.reason==\"missed a fail-open\"' '$LEDGER' >/dev/null"

# 2) idempotency: identical re-run adds NO duplicate
bash "$PAWL_VERDICT" write age-test-r 0 \
  --disposition REFUTED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "claude:REFUTED:ctx-claude:$TMP/ev1" \
  --refuter "gpt:REFUTED:ctx-gpt:$TMP/ev2" \
  --domain concurrency --reason "missed a fail-open" >/dev/null 2>&1 || true
check "re-run emits NO duplicate (still exactly one)" \
  "[ \"\$(gv_count age-test-r REFUTED)\" = '1' ]"

# 3) CONFIRMED verdict also emits
bash "$PAWL_VERDICT" write age-test-c 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev1" >/dev/null 2>&1 || true
check "CONFIRMED verdict emitted a gate-verdict event" \
  "[ \"\$(gv_count age-test-c CONFIRMED)\" = '1' ]"

# 4) normalization: codex + gpt are the SAME canonical family → cross_family=false
bash "$PAWL_VERDICT" write age-test-norm 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "codex:CONFIRMED:ctx-codex:$TMP/ev1" \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev2" >/dev/null 2>&1 || true
check "codex+gpt collapse to one family → cross_family=false (not gamed)" \
  "jq -e 'select(.bead_id==\"age-test-norm\") | .body.cross_family==false' '$LEDGER' >/dev/null"

# 4b) JOIN PROOF (cross-family REFUTE was a false alarm): an `accept` whose
#     gate_verdict_ref.head_sha == the REVIEWED head admits against the chokepoint's
#     CONFIRMED gate-verdict at that same head. reconcile-pr's accept uses cur_head
#     (the reviewed head), NOT the merge/rebound head — so gauge A admits it (no loss).
bash "$PAWL_VERDICT" write age-join 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev1" >/dev/null 2>&1 || true
( cd "$REPO" && ao yield emit accept --bead age-join --run age-join \
  --json "{\"merge_sha\":\"deadbeef1234567\",\"merged_by\":\"test\",\"gate_verdict_ref\":{\"bead_id\":\"age-join\",\"head_sha\":\"$HEAD_SHA\"}}" ) >/dev/null 2>&1 || true
check "accept@reviewed-head ADMITS against the chokepoint CONFIRMED (aligned default run: 0 unadmitted, A=1)" \
  "[ \"\$(cd '$REPO' && ao yield gauge --run age-join --json 2>/dev/null | jq -r '.gauges.unadmitted_accepts, .gauges.a_accepted | tostring' | paste -sd, -)\" = '0,1' ]"

# 4c) NEGATIVE (locks the run_id-mismatch REFUTE): an accept emitted under a
#     DIVERGENT run_id (the old reconcile-$bead default) does NOT admit against
#     the chokepoint CONFIRMED (which is under $bead) — same-run join only. This
#     is exactly the break the run_id alignment fix prevents.
bash "$PAWL_VERDICT" write age-divrun 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev1" >/dev/null 2>&1 || true
( cd "$REPO" && ao yield emit accept --bead age-divrun --run "reconcile-age-divrun" \
  --json "{\"merge_sha\":\"deadbeef1234567\",\"merged_by\":\"test\",\"gate_verdict_ref\":{\"bead_id\":\"age-divrun\",\"head_sha\":\"$HEAD_SHA\"}}" ) >/dev/null 2>&1 || true
check "divergent run_id accept is UNADMITTED (proves alignment is load-bearing)" \
  "[ \"\$(cd '$REPO' && ao yield gauge --run reconcile-age-divrun --json 2>/dev/null | jq -r '.gauges.unadmitted_accepts')\" = '1' ]"

# 4d) dedup is RUN-SCOPED: the same bead/head/attempt/disposition/reason in TWO
#     different runs must BOTH emit (gauge joins per-run; a later run re-emitting
#     must not be suppressed by an earlier run's row), while a same-run re-run
#     stays deduped. Drives the run via AGENTOPS_RUN_ID (the symmetric override).
AGENTOPS_RUN_ID=run-r1 bash "$PAWL_VERDICT" write age-dedup 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev1" >/dev/null 2>&1 || true
AGENTOPS_RUN_ID=run-r2 bash "$PAWL_VERDICT" write age-dedup 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev1" >/dev/null 2>&1 || true
AGENTOPS_RUN_ID=run-r2 bash "$PAWL_VERDICT" write age-dedup 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev1" >/dev/null 2>&1 || true
check "cross-run NOT deduped, same-run IS deduped (exactly 2 events: r1 + r2)" \
  "[ \"\$(gv_count age-dedup CONFIRMED)\" = '2' ]"

# 4e) cwd-INDEPENDENCE: invoked from a repo SUBDIR, the emit still lands in the
#     canonical REPO_ROOT ledger (ao yield resolves by cwd, so the emit cd's to
#     REPO_ROOT) — NOT a subdir ledger. Otherwise a subdir-run accept and a
#     root-run gate-verdict split into different ledgers and never join.
mkdir -p "$REPO/sub/deep"
( cd "$REPO/sub/deep" && bash "$PAWL_VERDICT" write age-cwd 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev1" ) >/dev/null 2>&1 || true
check "subdir-invoked emit lands in the REPO_ROOT ledger (cwd-independent)" \
  "[ \"\$(gv_count age-cwd CONFIRMED)\" = '1' ]"
check "subdir-invoked emit did NOT create a stray subdir ledger" \
  "[ ! -e '$REPO/sub/deep/.agents/yield/yield-ledger.jsonl' ]"

# 5) the yield log does NOT pollute the tracked tree — it lives under gitignored
#    .agents/. (The provenance ledger under docs/ is a separate, pre-existing
#    tracked artifact and is not this change's concern.)
printf '.agents/\n' > .gitignore; git add .gitignore; git commit -qm gitignore --no-verify
check "yield ledger is gitignored (the membrane log never dirties the tracked tree)" \
  "git -C '$REPO' check-ignore .agents/yield/yield-ledger.jsonl >/dev/null"
check "no yield artifact appears as a tracked/untracked-non-ignored change" \
  "[ -z \"\$(git -C '$REPO' status --porcelain .agents/)\" ]"

echo
if [[ "$fails" -eq 0 ]]; then echo "ALL PASS (14 checks)"; exit 0; fi
echo "$fails CHECK(S) FAILED"; exit 1
