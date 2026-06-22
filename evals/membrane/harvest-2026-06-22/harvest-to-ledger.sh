#!/usr/bin/env bash
# harvest-to-ledger.sh <scorecard.json> <ledger-root> <run-id>
#
# Turn a scripts/eval-membrane.sh scorecard into a REAL escape series in an
# ISOLATED yield ledger, so E5 (the SPC governor, tz2s.7.1 — "gated on real
# escape data existing") has fuel WITHOUT polluting the production
# .agents/yield/yield-ledger.jsonl.
#
# An escape in the yield ledger = a CONFIRMED gate-verdict that a later,
# higher-attempt REFUTED overturns (see `ao membrane derive-checks`). The weak
# Qwen-32B producer's self-declared "done" is the wrong CONFIRMED@1; the catch is
# the overturning REFUTED. Two real escape classes come out of the harvest:
#   - caught  (oracle FAIL + membrane REFUTE): producer CONFIRMED@1 -> membrane
#             REFUTED@2. The membrane working — a producer false-done it caught.
#   - escaped (oracle FAIL + membrane ACK):    producer CONFIRMED@1 -> membrane
#             ACK = CONFIRMED@2 -> oracle REFUTED@3. The membrane MISS — the
#             highest-value fuel (what E5 must learn to harden against).
# false_refute / correct_ack are not escapes and are skipped.
#
# Isolation: ao yield emit resolves .agents/yield/ by walking up from CWD, so we
# git-init a self-contained <ledger-root> and emit from there.
set -uo pipefail

SCORECARD="${1:?Usage: harvest-to-ledger.sh <scorecard.json> <ledger-root> <run-id>}"
LEDGER_ROOT="${2:?ledger-root}"
RUN_ID="${3:?run-id}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
AO="${AGENTOPS_AO_BIN:-/tmp/ao-pawl}"
[ -x "$AO" ] || { ( cd "$REPO_ROOT/cli" && go build -o /tmp/ao-pawl ./cmd/ao ) && AO=/tmp/ao-pawl; }
[ -f "$SCORECARD" ] || { echo "no scorecard: $SCORECARD" >&2; exit 1; }

mkdir -p "$LEDGER_ROOT"
# Isolation guard: `cd` MUST be on its own `|| exit` line. The footgun form
# `cd X && [ -d .git ] || { git init …; }` runs the git-init fallback in the
# CALLER'S cwd when `cd` fails (A&&B||C: failed cd → C runs here) — a repo-mutation
# data-loss bug. Guarding cd first makes a failed cd abort before any git init.
( cd "$LEDGER_ROOT" || exit 1
  [ -d .git ] || { git init -q . && git config user.email h@h && git config user.name h && git commit -qm base --allow-empty; } ) || { echo "harvest-to-ledger: cannot init ledger root $LEDGER_ROOT" >&2; exit 1; }

# Deterministic 7-hex synthetic sha from a label (no Date/rand; stable for resume).
sha7() { printf '%s' "$1" | shasum | cut -c1-7; }

emitted_escapes=0; caught=0; missed=0; skipped=0; failed=0
# Stream per_task rows as TSV. IFS=$'\t' is WHITESPACE, so `read` collapses
# consecutive tabs and strips empties — a row with an empty `why` (the degraded
# tasks) would shift later fields left and corrupt `degraded`. So `why` (the only
# possibly-empty field) is LAST, and every guard field precedes it.
# Order: task \t oracle_pass \t verdict \t degraded \t why
while IFS=$'\t' read -r task oracle verdict degraded why; do
  [ -n "$task" ] || continue
  if [ "$degraded" = "true" ]; then skipped=$((skipped+1)); continue; fi
  # Only an ADJUDICATED verdict can be an escape. DRY/empty = the membrane never
  # reviewed (degraded producer) — not a membrane miss; skip fail-closed.
  if [ "$verdict" != "ACK" ] && [ "$verdict" != "REFUTE" ]; then skipped=$((skipped+1)); continue; fi
  # Only false-dones (oracle FAIL) are escapes.
  if [ "$oracle" = "true" ]; then skipped=$((skipped+1)); continue; fi

  bead="age-harvest-$task"
  s1="$(sha7 "$task-producer")"; s2="$(sha7 "$task-membrane")"; s3="$(sha7 "$task-oracle")"
  domain="membrane-eval"
  # ge returns the real `ao yield emit` exit code (the >/dev/null only hides
  # output, not the status). Callers MUST check it — a failed emit is a failed
  # escape record, never a silent success.
  ge() { "$AO" yield emit gate-verdict --bead "$bead" --run "$RUN_ID" --json "$1" >/dev/null 2>&1; }
  whyjson="$(printf '%s' "$why" | python3 -c 'import sys,json;print(json.dumps(sys.stdin.read()))')"

  # Every emit in the chain is `|| exit 1`-guarded inside the subshell, so ANY
  # failed `ao yield emit` aborts the chain with a nonzero status the outer `if`
  # catches — fail-closed: a partially-emitted or failed chain is NOT counted.
  if ( set -e
       cd "$LEDGER_ROOT"
       # CONFIRMED@1 — the weak producer's own "done".
       ge "{\"disposition\":\"CONFIRMED\",\"head_sha\":\"$s1\",\"attempt\":1,\"pawl_verdict_ref\":{\"bead_id\":\"$bead\",\"head_sha\":\"$s1\"},\"author_context_id\":\"qwen2.5-coder-32b\",\"author_family\":\"qwen\",\"difficulty\":3}" || exit 1
       if [ "$verdict" = "REFUTE" ]; then
         # caught: membrane overturns the producer's false-done.
         ge "{\"disposition\":\"REFUTED\",\"head_sha\":\"$s2\",\"attempt\":2,\"pawl_verdict_ref\":{\"bead_id\":\"$bead\",\"head_sha\":\"$s2\"},\"author_context_id\":\"codex\",\"author_family\":\"gpt\",\"difficulty\":3,\"domain\":\"$domain\",\"reason\":$whyjson,\"cross_family\":true,\"author_ne_reviewer\":true}" || exit 1
       else
         # escaped (membrane MISS): membrane ACK = wrong CONFIRMED@2, oracle overturns@3.
         ge "{\"disposition\":\"CONFIRMED\",\"head_sha\":\"$s2\",\"attempt\":2,\"pawl_verdict_ref\":{\"bead_id\":\"$bead\",\"head_sha\":\"$s2\"},\"author_context_id\":\"codex\",\"author_family\":\"gpt\",\"difficulty\":3,\"cross_family\":true,\"author_ne_reviewer\":true}" || exit 1
         ge "{\"disposition\":\"REFUTED\",\"head_sha\":\"$s3\",\"attempt\":3,\"pawl_verdict_ref\":{\"bead_id\":\"$bead\",\"head_sha\":\"$s3\"},\"author_context_id\":\"oracle-deterministic\",\"author_family\":\"oracle\",\"difficulty\":3,\"domain\":\"$domain\",\"reason\":\"deterministic oracle (score.sh) FAILs: the cross-family membrane ACKed a real false-done — a genuine membrane miss\",\"detector_kind\":\"semantic\"}" || exit 1
       fi
     ); then
    if [ "$verdict" = "REFUTE" ]; then caught=$((caught+1)); else missed=$((missed+1)); fi
    emitted_escapes=$((emitted_escapes+1))
  else
    echo "harvest-to-ledger: ERROR — ao yield emit failed for $task; chain NOT counted (fail-closed)" >&2
    failed=$((failed+1))
  fi
done < <(python3 -c '
import sys, json
d = json.load(open(sys.argv[1]))
for t in d.get("per_task", []):
    print("\t".join([
        str(t.get("task","")),
        "true" if t.get("oracle_pass") else "false",
        str(t.get("verdict","")),
        "true" if t.get("degraded") else "false",
        str(t.get("why","")).replace("\t"," ").replace("\n"," "),
    ]))
' "$SCORECARD")

echo "harvest-to-ledger: emitted $emitted_escapes escape chain(s) into $LEDGER_ROOT/.agents/yield/ (run=$RUN_ID)"
echo "  caught (membrane working): $caught   |   escaped (membrane MISS — E5's target): $missed   |   skipped (true-done/degraded): $skipped   |   failed: $failed"
# Fail-closed: a nonzero exit if any chain failed to fully emit, so a caller
# (or CI) never reads a partial harvest as a clean one.
[ "$failed" -eq 0 ] || exit 1
