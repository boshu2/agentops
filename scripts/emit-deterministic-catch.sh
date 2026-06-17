#!/usr/bin/env bash
# emit-deterministic-catch.sh — the DETERMINISTIC membrane tier (age-srl).
#
# When a pre-push Go gate BLOCKS a push, the membrane just caught a false-done:
# the agent tried to land changes it implicitly claimed ready, and a deterministic
# ground-truth gate (build / test / a check-*.sh) rejected them. The in-situ
# catch-rate gauge (age-t3f, `ao yield gauge`) only counted the cross-family
# refuter/merge tier (reconcile-pr.sh auto-emits); these deterministic catches
# were dropped on the floor, so the gauge UNDERCOUNTS. This records them.
#
# Called from the FAIL branch of the cockpit pre-push gate
# (scripts/hooks/pre-push.local), symmetric to the PASS-branch SENSOR feed
# (emit-landed-provenance.sh). One emit per blocked push attempt — the genuine
# "claimed-done" door — not per local `ao gate check` iteration.
#
# Design constraints (mirror emit-landed-provenance.sh — honest, low blast radius):
#   - NON-BLOCKING: every failure warns and exits 0. Recording a catch must never
#     itself block (or un-block) a push.
#   - SKIPPABLE: AGENTOPS_DETERMINISTIC_CATCH_SKIP=1 disables it.
#   - DETERMINISTIC TIER: disposition=REFUTED, mode=deterministic, cross_family=false
#     so it raises the overall catch_rate WITHOUT touching catch_rate_cross_family.
#
# Env knobs:
#   AO_BIN                  explicit ao binary (else cli/bin/ao)
#   AO_YIELD_RUN_ID         run bucket (default: deterministic-pre-push)
#   AO_YIELD_BEAD           bead id (default: parsed from HEAD subject, else
#                           pre-push-deterministic)
#   AO_YIELD_DIFFICULTY     difficulty weight (default: 1)
set -uo pipefail

if [[ "${AGENTOPS_DETERMINISTIC_CATCH_SKIP:-0}" == "1" ]]; then
    exit 0
fi

warn() { echo "deterministic-catch: $*" >&2; }

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { warn "not in a git repo; skipping"; exit 0; }
cd "$ROOT" || exit 0

# Resolve the ao binary: explicit AO_BIN, then the conventional build output.
AO="${AO_BIN:-}"
if [[ -z "$AO" && -x "cli/bin/ao" ]]; then
    AO="$PWD/cli/bin/ao"
fi
if [[ -z "$AO" || ! -x "$AO" ]]; then
    warn "ao binary not found (set AO_BIN or build cli/bin/ao); skipping emit"
    exit 0
fi

run_id="${AO_YIELD_RUN_ID:-deterministic-pre-push}"
difficulty="${AO_YIELD_DIFFICULTY:-1}"
sha="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"

# Bead id: explicit override, else a bead ref in the HEAD commit subject
# (age-xxx / ag-xxx), else a stable sentinel for unattributed deterministic catches.
bead="${AO_YIELD_BEAD:-}"
if [[ -z "$bead" ]]; then
    subject="$(git log -1 --format=%s 2>/dev/null || true)"
    bead="$(printf '%s' "$subject" | grep -oE '\b(age|ag)-[a-z0-9]+' | head -1 || true)"
fi
[[ -n "$bead" ]] || bead="pre-push-deterministic"

# Deterministic-tier gate-verdict body. REFUTED = a false-done the gate caught.
# No refuter families and cross_family=false: this is ground-truth, not a panel,
# so it must NOT inflate catch_rate_cross_family. author_ne_reviewer=true: a
# deterministic gate is never the author. evidence_present=true: the gate output
# IS the evidence. Schema is closed (DisallowUnknownFields) — keep these keys exact.
body="$(printf '{"difficulty":%s,"pawl_verdict_ref":{"bead_id":"%s","head_sha":"%s"},"disposition":"REFUTED","head_sha":"%s","attempt":1,"mode":"deterministic","author_context_id":"pre-push-gate","refuter_families":[],"author_family":"deterministic-gate","cross_family":false,"author_ne_reviewer":true,"evidence_present":true}' \
    "$difficulty" "$bead" "$sha" "$sha")"

if "$AO" yield emit gate-verdict --bead "$bead" --run "$run_id" --json "$body" >/dev/null 2>&1; then
    echo "deterministic-catch: recorded REFUTED catch for $bead@$sha (run $run_id)"
else
    warn "ao yield emit failed; skipping (non-blocking)"
fi
exit 0
