#!/usr/bin/env bash
# probe-first-verdict.sh — timed first-value probe (age-wedge-all-in-dyr0.5).
#
# Proves the README quickstart golden path end to end in a CLEAN temp git repo:
#   1. ao quick-start          (scaffold + ledger path + reviewer readiness)
#   2. a small change, committed
#   3. ao verify my-first-change
# and asserts a verdict landed in the temp repo's provenance ledger
# (docs/provenance/ledger.jsonl) within the wall-clock budget (default 300s).
#
# TWO MODES:
#   default (mock)  A MOCK reviewer named `codex` is placed FIRST on PATH. CI
#                   runners hold no codex/agy subscription auth, so a live probe
#                   there would always fail; this mode proves the PATH MECHANICS
#                   and the TIMING FLOOR only, and says so in its output. It is
#                   NOT the honest first-verdict number.
#   --live          Uses the REAL reviewer CLI on PATH (requires codex). This is
#                   the honest 5-minute number — run it on the operator's box on
#                   release cadence and record it next to the receipts.
#
# The probe also greps README.md for the exact golden-path commands it executes
# (`ao quick-start`, `ao verify my-first-change`), so a README edit cannot
# silently break the documented path.
#
# Usage:
#   scripts/probe-first-verdict.sh [--live] [--budget SECONDS]
# Env:
#   PROBE_AO_BIN   path to the ao binary to probe (default: cli/bin/ao, then a
#                  fresh `go build`, then PATH `ao`). The build is excluded from
#                  the timed window (the user path starts at quick-start).
# Exit codes:
#   0 pass · 1 assertion failed · 2 usage / precondition
#
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

MODE="mock"
BUDGET=300
CHANGE_ID="my-first-change"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --live) MODE="live"; shift ;;
    --budget)
      shift
      [[ $# -gt 0 ]] || { echo "--budget requires a value" >&2; exit 2; }
      BUDGET="$1"; shift ;;
    --budget=*) BUDGET="${1#--budget=}"; shift ;;
    -h|--help) sed -n '2,36p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "probe-first-verdict: unknown arg: $1" >&2; exit 2 ;;
  esac
done

log()  { printf '[probe %s] %s\n' "$(date -u +%H:%M:%S)" "$*" >&2; }
fail() { log "FAIL: $*"; echo "PROBE FAIL (mode=$MODE): $*"; exit 1; }
precondition() { log "PRECONDITION: $*"; echo "PROBE PRECONDITION (mode=$MODE): $*"; exit 2; }

require_cmd git
require_cmd jq "brew install jq / apt-get install jq"

# --- README drift gate: the documented golden path must be the one we run ----
grep -qF 'ao quick-start' "$REPO_ROOT/README.md" \
  || fail "README drift: 'ao quick-start' is no longer in README.md — the documented quickstart diverged from the probed path"
grep -qF "ao verify $CHANGE_ID" "$REPO_ROOT/README.md" \
  || fail "README drift: 'ao verify $CHANGE_ID' is no longer in README.md — the documented quickstart diverged from the probed path"
log "README drift gate: OK (README carries the exact probed commands)"

with_tmpdir work first-verdict-probe
# with_tmpdir assigns via printf -v, which shellcheck cannot see (SC2154).
work="${work:?with_tmpdir did not assign work}"

# --- Resolve the ao binary under probe (outside the timed window) -----------
AO="${PROBE_AO_BIN:-}"
if [[ -z "$AO" ]]; then
  if [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
    AO="$REPO_ROOT/cli/bin/ao"
  elif command -v go >/dev/null 2>&1; then
    log "building ao from $REPO_ROOT/cli (build time is excluded from the timed window)"
    ( cd "$REPO_ROOT/cli" && go build -o "$work/ao" ./cmd/ao ) \
      || precondition "go build ./cmd/ao failed"
    AO="$work/ao"
  elif command -v ao >/dev/null 2>&1; then
    AO="$(command -v ao)"
  fi
fi
[[ -n "$AO" && -x "$AO" ]] || precondition "no ao binary (set PROBE_AO_BIN, build cli/bin/ao, or install ao)"
log "ao under probe: $AO ($("$AO" --version 2>/dev/null | head -1 || echo version-unknown))"

# --- Reviewer: mock (default) or live ---------------------------------------
if [[ "$MODE" == "mock" ]]; then
  mkdir -p "$work/mockbin"
  cat > "$work/mockbin/codex" <<'MOCK'
#!/bin/sh
# Mock reviewer for probe-first-verdict.sh (mock mode): answers --version
# instantly and CONFIRMs any review packet. Proves PATH mechanics only.
if [ "${1:-}" = "--version" ]; then echo "mock-codex 0.0.1"; exit 0; fi
cat >/dev/null
echo "codex"
echo "Mock review: packet received and read; no blocking defect found."
echo "tokens used: 42"
echo "VERDICT: CONFIRMED"
MOCK
  chmod +x "$work/mockbin/codex"
  PATH="$work/mockbin:$PATH"
  export PATH
  log "mock reviewer 'codex' installed FIRST on PATH ($work/mockbin) — mock mode measures mechanics + timing floor only"
else
  command -v codex >/dev/null 2>&1 \
    || precondition "--live requires a real codex CLI on PATH (npm install -g @openai/codex && codex login)"
  log "live mode: real reviewer $(command -v codex)"
fi

# --- Clean temp git repo (a repo that has never seen AgentOps) ---------------
repo="$work/repo"
mkdir -p "$repo"
git -C "$repo" init -q
git -C "$repo" config user.email "probe@example.invalid"
git -C "$repo" config user.name "first-verdict-probe"
git -C "$repo" config commit.gpgsign false
printf '# probe target\n' > "$repo/README.md"
git -C "$repo" add README.md
git -C "$repo" commit -qm "initial"
log "temp repo ready at $repo"

# --- Timed window: the user's path, quick-start → verdict --------------------
start="$(date +%s)"

log "STEP 1/3: ao quick-start --no-beads"
qs_log="$work/quick-start.log"
if ! ( cd "$repo" && "$AO" quick-start --no-beads </dev/null ) >"$qs_log" 2>&1; then
  sed 's/^/[quick-start] /' "$qs_log" >&2
  fail "ao quick-start exited non-zero"
fi
grep -qF "ao verify $CHANGE_ID" "$qs_log" \
  || { sed 's/^/[quick-start] /' "$qs_log" >&2; fail "quick-start did not print the golden-path command 'ao verify $CHANGE_ID'"; }
for tombstone in 'ao factory' 'ao orchestrate' 'ao codex' '/rpi' '/validation'; do
  if grep -qF "$tombstone" "$qs_log"; then
    sed 's/^/[quick-start] /' "$qs_log" >&2
    fail "quick-start emitted removed route '$tombstone'"
  fi
done
last_ao=$(grep -o 'ao [[:alnum:]][[:alnum:]_-]*[^[:cntrl:]]*' "$qs_log" | tail -1)
[[ "$last_ao" == "ao verify $CHANGE_ID" ]] \
  || fail "quick-start terminal ao command is '$last_ao', want 'ao verify $CHANGE_ID'"
log "quick-start OK (printed the exact next command)"

log "STEP 2/3: small change, committed"
printf 'probe change at %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$repo/README.md"
git -C "$repo" add README.md
git -C "$repo" commit -qm "probe: small change"

log "STEP 3/3: ao verify $CHANGE_ID (mode=$MODE)"
v_log="$work/verify.log"
v_rc=0
( cd "$repo" && "$AO" verify "$CHANGE_ID" ) >"$v_log" 2>&1 || v_rc=$?
end="$(date +%s)"
elapsed=$(( end - start ))
sed 's/^/[verify] /' "$v_log" >&2
[[ "$v_rc" -eq 0 ]] || fail "ao verify exited $v_rc (expected 0 = CONFIRMED + verdict written)"

# --- Assertions ---------------------------------------------------------------
verdict_json="$repo/.agents/pawl-verdicts/$CHANGE_ID.json"
[[ -s "$verdict_json" ]] || fail "no verdict file at $verdict_json"
disposition="$(jq -r '.disposition // empty' "$verdict_json" 2>/dev/null || true)"
[[ "$disposition" == "CONFIRMED" ]] || fail "verdict disposition is '$disposition', want CONFIRMED ($verdict_json)"
log "verdict file OK: $verdict_json (disposition=CONFIRMED)"

ledger="$repo/docs/provenance/ledger.jsonl"
[[ -s "$ledger" ]] || fail "no ledger at $ledger — the verdict edge was not recorded in the user's repo"
grep -qF "$CHANGE_ID" "$ledger" \
  || fail "ledger at $ledger has no line for $CHANGE_ID"
log "ledger line: $(grep -F "$CHANGE_ID" "$ledger" | tail -1)"

[[ "$elapsed" -lt "$BUDGET" ]] \
  || fail "wall clock ${elapsed}s exceeded the ${BUDGET}s budget"

# --- Verdict ------------------------------------------------------------------
echo "PROBE PASS (mode=$MODE): install-to-first-verdict path landed a verdict in the temp repo's ledger in ${elapsed}s (budget ${BUDGET}s)"
if [[ "$MODE" == "mock" ]]; then
  echo "NOTE: mock mode measured the PATH MECHANICS and TIMING FLOOR with a MOCK reviewer on PATH."
  echo "      This is NOT the honest first-verdict number. Run 'scripts/probe-first-verdict.sh --live'"
  echo "      on a box with a real codex CLI for the honest 5-minute measurement."
fi
exit 0
