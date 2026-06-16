#!/usr/bin/env bash
# recovery-statemachine.sh — Codex/local-runtime recovery state machine (M2, age-d16-self-hosting-route-nkr.3).
#
# WHAT: when an unattended Codex/local run hits a failure, decide ONE recovery
# branch — fix-forward | re-scope-as-new-acceptance | andon — and update the
# bead accordingly, with CRISP terminal behavior: no spin (bounded retries),
# no silent defer (every path emits a terminal verdict + one bead mutation),
# no mis-close (recovery NEVER closes the bead as done — that is the merge/pawl
# door's authority, scripts/reconcile-pr.sh).
#
# WHY: this PORTS the Claude-only fix-forward policy from .claude/workflows/
# ship-beads.js (polling -> flake-rerun [budget 1] -> fix-forward -> terminal
# merged|blocked|abandoned) into the bo-mac Codex/local runtime (pure shell +
# br, no Claude / no Workflow engine), and ADDS the two missing branches.
# It replaces the `|| echo "timed out or failed"` SILENT DEFER that
# scripts/run-rpi-phases.sh does on failure today.
#
# Non-goals: the parked Linux high-assurance daemon; a Claude executor; new
# gate or ledger schema. Reuse the policy, swap the executor.
#
# Exit codes (a launching loop branches on these):
#   0  recovered | rescoped   — the run may proceed to the next bead
#   2  usage error            — bad/insufficient arguments (loud, not deferred)
#   3  andon                  — pull the cord: HALT the line, a human/next run takes over
set -euo pipefail

# Ported from ship-beads.js FLAKE_RERUN_BUDGET: exactly ONE fix-forward retry so
# a real break can't be masked by unbounded retrying (the "no spin" guard).
FIX_FORWARD_BUDGET=1

usage() {
  cat >&2 <<'EOF'
Usage: recovery-statemachine.sh --bead <id> --failure-kind <drift|flake|rescope|hard|auto> [opts]

  --bead <id>               (required) bead the failed run was working
  --failure-kind <kind>     (required) drift|flake -> fix-forward candidate;
                            rescope -> re-scope-as-new-acceptance (needs --rescope-scenario);
                            hard|auto|<unknown> -> andon (default-safe)
  --recheck-cmd <cmd>       acceptance/gate command that failed; re-run after remediation
  --remediate-cmd <cmd>     fix-forward remediation (default: scripts/regen-all.sh)
  --rescope-scenario <txt>  REQUIRED for rescope: the new acceptance (a Given/When/Then)
  --reason <txt>            andon/rescope reason recorded on the bead
  --beads-dir <dir>         BEADS_DIR (default: $PWD/_beads)
  --dry-run                 print the decision; make NO bead mutation
  -h|--help                 this help

Emits ONE structured terminal line (JSON) on stdout. Never spins, never silently
defers, never closes the bead as done.
EOF
}

BEAD="" FAILURE_KIND="" RECHECK_CMD="" REMEDIATE_CMD="" RESCOPE_SCENARIO="" REASON="" DRY_RUN=0
BEADS_DIR_ARG=""

while [ $# -gt 0 ]; do
  case "$1" in
    --bead) BEAD="${2:-}"; shift 2 ;;
    --failure-kind) FAILURE_KIND="${2:-}"; shift 2 ;;
    --recheck-cmd) RECHECK_CMD="${2:-}"; shift 2 ;;
    --remediate-cmd) REMEDIATE_CMD="${2:-}"; shift 2 ;;
    --rescope-scenario) RESCOPE_SCENARIO="${2:-}"; shift 2 ;;
    --reason) REASON="${2:-}"; shift 2 ;;
    --beads-dir) BEADS_DIR_ARG="${2:-}"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "recovery: unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

[ -n "$BEAD" ] || { echo "recovery: --bead is required" >&2; usage; exit 2; }
[ -n "$FAILURE_KIND" ] || { echo "recovery: --failure-kind is required" >&2; usage; exit 2; }

BEADS_DIR="${BEADS_DIR_ARG:-${BEADS_DIR:-$PWD/_beads}}"
REMEDIATE_CMD="${REMEDIATE_CMD:-scripts/regen-all.sh}"
export BEADS_DIR

# br wrapper — every bead mutation flows through here so the BEADS_DIR scoping is
# uniform and dry-run is honored in exactly one place (no silent-defer escape).
br_do() {
  if [ "$DRY_RUN" -eq 1 ]; then
    echo "recovery[dry-run]: would run: br $*" >&2
    return 0
  fi
  br "$@"
}

# emit_terminal <branch> <terminal_state> <bead_action> [extra_json]
# The single crisp terminal verdict. `spin` is ALWAYS false by construction —
# there is no unbounded loop in this script.
emit_terminal() {
  local branch="$1" terminal_state="$2" bead_action="$3" extra="${4:-}"
  printf '{"bead":"%s","branch":"%s","terminal_state":"%s","bead_action":"%s","spin":false%s}\n' \
    "$BEAD" "$branch" "$terminal_state" "$bead_action" "${extra:+,$extra}"
}

# ---------------------------------------------------------------------------
# ANDON — pull the cord. Loud, labeled, commented; NOT closed, NOT deferred,
# NOT retried. Default-safe sink for hard/unknown/ambiguous failures, and the
# escalation target when fix-forward exhausts its budget.
# ---------------------------------------------------------------------------
andon() {
  local reason="${1:-unrecoverable failure}"
  echo "recovery: ANDON on $BEAD — $reason" >&2
  br_do update "$BEAD" --add-label andon >/dev/null 2>&1 || true
  br_do comments add "$BEAD" "ANDON (recovery state-machine): $reason. Run halted; needs a human or a fresh run. Bead left open (not closed, not deferred)." >/dev/null 2>&1 || true
  emit_terminal "andon" "andon" "labeled-andon"
  exit 3
}

# ---------------------------------------------------------------------------
# FIX-FORWARD — ported policy: run the remediation, then re-run the recheck,
# bounded by FIX_FORWARD_BUDGET. Green -> recovered (bead stays progressable,
# NOT closed). Still red after the budget -> escalate to andon (a fix-forward
# that cannot fix IS a hard failure). The bounded loop is the "no spin" guard.
# ---------------------------------------------------------------------------
fix_forward() {
  local attempt=0
  while [ "$attempt" -le "$FIX_FORWARD_BUDGET" ]; do
    if [ "$attempt" -gt 0 ]; then
      echo "recovery: fix-forward remediation (attempt $attempt/$FIX_FORWARD_BUDGET): $REMEDIATE_CMD" >&2
      if [ "$DRY_RUN" -eq 0 ]; then bash -c "$REMEDIATE_CMD" >&2 || true; fi
    fi
    if [ -z "$RECHECK_CMD" ]; then
      # No recheck given: a single remediation is the whole fix-forward; treat the
      # post-remediation state as recovered only after we've actually remediated.
      if [ "$attempt" -gt 0 ]; then break; fi
      attempt=$((attempt + 1)); continue
    fi
    if [ "$DRY_RUN" -eq 1 ]; then
      echo "recovery[dry-run]: would recheck: $RECHECK_CMD" >&2
      break
    fi
    if bash -c "$RECHECK_CMD" >&2; then
      br_do comments add "$BEAD" "fix-forward (recovery state-machine): recovered via \`$REMEDIATE_CMD\`; recheck green. Bead progressable (acceptance still owned by the merge/pawl door)." >/dev/null 2>&1 || true
      emit_terminal "fix-forward" "recovered" "comment-recovered"
      exit 0
    fi
    attempt=$((attempt + 1))
  done

  if [ "$DRY_RUN" -eq 1 ]; then
    emit_terminal "fix-forward" "recovered" "dry-run-no-recheck"
    exit 0
  fi
  if [ -z "$RECHECK_CMD" ]; then
    # Remediation ran, nothing to verify against — record and let the run proceed.
    br_do comments add "$BEAD" "fix-forward (recovery state-machine): remediation \`$REMEDIATE_CMD\` applied; no recheck command supplied. Bead progressable." >/dev/null 2>&1 || true
    emit_terminal "fix-forward" "recovered" "comment-remediated"
    exit 0
  fi
  # Budget exhausted, recheck still red -> escalate (never spin).
  andon "fix-forward exhausted (budget=$FIX_FORWARD_BUDGET): recheck \`$RECHECK_CMD\` still failing after \`$REMEDIATE_CMD\`"
}

# ---------------------------------------------------------------------------
# RE-SCOPE-AS-NEW-ACCEPTANCE — the failure becomes a NEW acceptance. File a
# follow-up bead whose body IS the new scenario, make the original depend on it
# (original blocked-by-new), label + comment the original. The original is NOT
# closed (it isn't done) and NOT left spinning — crisp terminal `rescoped`.
# Missing scenario -> andon, never a silent defer.
# ---------------------------------------------------------------------------
rescope() {
  [ -n "$RESCOPE_SCENARIO" ] || andon "re-scope requested without --rescope-scenario (cannot silently defer an acceptance change)"

  local body new_id
  body="Re-scoped from $BEAD by the recovery state-machine. The failure became a new acceptance.

## Scenarios
$RESCOPE_SCENARIO

Reason: ${REASON:-original acceptance unreachable as written}"

  if [ "$DRY_RUN" -eq 1 ]; then
    echo "recovery[dry-run]: would create follow-up acceptance bead and block $BEAD on it" >&2
    emit_terminal "rescope" "rescoped" "dry-run-new-acceptance" '"new_acceptance":"(dry-run)"'
    exit 0
  fi

  # Capture inside the `if !` condition so a non-zero `br create` (locked DB,
  # stale-DB refusal, bad --deps, daemon contention) escalates to andon instead
  # of aborting the script under `set -e` BEFORE the guard — that abort would be
  # a silent defer (exit 1, no terminal line, no bead mutation). br stderr passes
  # through as diagnostics; only stdout (the id) is captured.
  if ! new_id="$(br create "Re-scoped acceptance from $BEAD" -t task -p 1 --silent --deps "blocks:$BEAD" --body "$body")"; then
    andon "re-scope failed: br create errored (non-zero exit) for $BEAD"
  fi
  new_id="$(printf '%s' "$new_id" | tr -d '[:space:]')"
  [ -n "$new_id" ] || andon "re-scope failed: br create returned no id"

  br_do update "$BEAD" --add-label rescoped >/dev/null 2>&1 || true
  br_do comments add "$BEAD" "RE-SCOPED (recovery state-machine): acceptance moved to $new_id (which now blocks this bead). Original left open, not closed." >/dev/null 2>&1 || true
  emit_terminal "rescope" "rescoped" "new-acceptance-filed" "\"new_acceptance\":\"$new_id\""
  exit 0
}

# ---------------------------------------------------------------------------
# CLASSIFY -> dispatch ONE branch. Default-safe: anything not explicitly a
# fix-forward or a rescope falls to andon (never mis-close on ambiguity).
# ---------------------------------------------------------------------------
case "$FAILURE_KIND" in
  drift|flake) fix_forward ;;
  rescope)     rescope ;;
  hard|auto|*) andon "${REASON:-failure-kind=$FAILURE_KIND classified as unrecoverable}" ;;
esac
