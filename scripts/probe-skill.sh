#!/usr/bin/env bash
# probe-skill.sh — skill BEHAVIORAL probe harness (age-e508.1).
#
# ============================ HONESTY HEADER =================================
# A probe measures BEHAVIOR-CHANGE, NOT quality-uplift. It answers exactly one
# question: when the skill is LOADED (treatment) versus NOT loaded (control),
# does the agent actually DO the thing differently — a tool call made, an
# artifact produced, a sequence followed? It NEVER scores whether the text
# mentions the skill, and it NEVER claims the skill makes output better. A
# BEHAVIORAL verdict means "loading it changed what the agent did"; an INERT
# verdict means "it didn't" (the 2026-06-30 graphify result: a doc-instruction
# skill 0/2 treatment agents obeyed). Small N (default 2-3) is DIRECTIONAL, not
# statistical. Do not overclaim (ADR-0011 discipline).
# ============================================================================
#
# A PROBE is a directory under evals/skill-probes/<id>/ carrying:
#   probe.json            metadata: id, skill, reps, behavior, discriminator
#   question.md           the scenario question — IDENTICAL for both arms
#   treatment-prelude.md  the skill guidance injected ONLY in the treatment arm
#                         (the sole variable: control = question; treatment =
#                         prelude + question)
#   discriminator.sh      a DETERMINISTIC behavioral check over one transcript:
#                         exit 0 = behavior PRESENT, 1 = ABSENT, 2 = infra error
#   fixtures/             recorded transcripts control-<n>.txt / treatment-<n>.txt
#                         (used by --replay for deterministic calibration + a
#                         committed, reproducible evidence run)
#
# MODES:
#   live (default)  dispatch a cross-family worker (codex exec — the sanctioned
#                   headless path; NEVER claude -p, LAW 0) for each arm x rep,
#                   capture the transcript, run the discriminator. Writes the
#                   transcripts into fixtures/ so the run is reproducible.
#   --replay        skip dispatch; run the discriminator over the committed
#                   fixtures. Deterministic — this is what calibration and CI use.
#
# VERDICT: BEHAVIORAL iff treatment_rate > control_rate; INERT iff not; UNMEASURED
# iff no usable treatment reps (all degraded / missing).
#
# Usage:
#   bash scripts/probe-skill.sh --probe crank --replay
#   bash scripts/probe-skill.sh --probe crank --reps 2 --output out.json
#   bash scripts/probe-skill.sh --probe crank --live --capture   # record fixtures
#   bash scripts/probe-skill.sh --probe crank --live --model gpt-5-mini  # weak producer
#
# Flags: --probe <id> (required) · --replay | --live · --capture · --reps N ·
#        --output <path> · --timeout <secs> · --model <id> (weaker producer, the
#        ratchet when a frontier producer aces both arms).
#
# Env overrides (test seams): SKILL_PROBES_DIR (default $REPO_ROOT/evals/skill-probes)
#
# practices: [measurement-over-assertion, ab-testing]
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"
# shellcheck source=scripts/lib/codex-exec.sh disable=SC1091
. "$REPO_ROOT/scripts/lib/codex-exec.sh"

PROBES_DIR="${SKILL_PROBES_DIR:-$REPO_ROOT/evals/skill-probes}"
PROBE=""
REPLAY=0
CAPTURE=0
REPS=""
OUTPUT=""
TIMEOUT="${PROBE_TIMEOUT:-240}"
MODEL="${PROBE_MODEL:-}"

usage() { grep '^#' "$0" | sed 's/^# \?//'; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --probe)   PROBE="${2:-}"; shift 2;;
        --replay)  REPLAY=1; shift;;
        --live)    REPLAY=0; shift;;
        --capture) CAPTURE=1; shift;;
        --reps)    REPS="${2:-}"; shift 2;;
        --output)  OUTPUT="${2:-}"; shift 2;;
        --timeout) TIMEOUT="${2:-}"; shift 2;;
        --model)   MODEL="${2:-}"; shift 2;;
        -h|--help) usage; exit 0;;
        *) echo "Unknown flag: $1" >&2; exit 2;;
    esac
done

[[ -n "$PROBE" ]] || { echo "error: --probe <id> required" >&2; exit 2; }
PROBE_DIR="$PROBES_DIR/$PROBE"
[[ -d "$PROBE_DIR" ]] || { echo "error: probe not found: $PROBE_DIR" >&2; exit 2; }
DISC="$PROBE_DIR/discriminator.sh"
QUESTION="$PROBE_DIR/question.md"
PRELUDE="$PROBE_DIR/treatment-prelude.md"
META="$PROBE_DIR/probe.json"
for f in "$DISC" "$QUESTION" "$PRELUDE" "$META"; do
    [[ -f "$f" ]] || { echo "error: probe file missing: $f" >&2; exit 2; }
done

# Read reps + skill from probe.json (python3, no jq dependency).
json_get() { python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d.get(sys.argv[2],""))' "$META" "$1"; }
[[ -n "$REPS" ]] || REPS="$(json_get reps)"
[[ -n "$REPS" ]] || REPS=2
SKILL="$(json_get skill)"

FIXDIR="$PROBE_DIR/fixtures"
mkdir -p "$FIXDIR"

# run_discriminator TRANSCRIPT -> echoes PRESENT|ABSENT|DEGRADED
run_discriminator() {
    local transcript="$1" rc=0
    [[ -s "$transcript" ]] || { echo "DEGRADED"; return; }
    bash "$DISC" "$transcript" >/dev/null 2>&1 || rc=$?
    case "$rc" in
        0) echo "PRESENT";;
        1) echo "ABSENT";;
        *) echo "DEGRADED";;
    esac
}

# dispatch_live ARM REP TRANSCRIPT_OUT -> populate TRANSCRIPT_OUT via codex exec.
# ARM is control|treatment. Control prompt = question; treatment = prelude+question.
dispatch_live() {
    local arm="$1" transcript="$2" prompt work
    if [[ "$arm" == "treatment" ]]; then
        prompt="$(cat "$PRELUDE")"$'\n\n---\n\n'"$(cat "$QUESTION")"
    else
        prompt="$(cat "$QUESTION")"
    fi
    work="$(mktemp -d "${TMPDIR:-/tmp}/probe-ws.XXXXXX")"
    # read-only sandbox: the probe only wants the agent's PLAN text, no mutation.
    # --model routes a WEAKER producer (e.g. gpt-5-mini) — the ratchet for
    # surfacing a skill's behavioral value when a frontier producer aces both arms
    # (the membrane-eval-too-easy lesson). Empty => the codex default (frontier).
    CODEX_EXEC_PROMPT_ARG="$prompt" \
    CODEX_EXEC_DIR="$work" \
    CODEX_EXEC_SANDBOX=read-only \
    CODEX_EXEC_SKIP_GIT_CHECK=1 \
    CODEX_EXEC_TIMEOUT="$TIMEOUT" \
    CODEX_EXEC_MODEL="$MODEL" \
    CODEX_EXEC_OUT_FILE="$transcript" \
    CODEX_EXEC_EXPECT_OUTPUT=1 \
        codex_exec_guarded >/dev/null 2>&1 || true
    rm -rf "$work"
}

C_PRESENT=0; C_USABLE=0; T_PRESENT=0; T_USABLE=0
PER_REP_JSON=""

for ((n=1; n<=REPS; n++)); do
    c_tx="$FIXDIR/control-$n.txt"
    t_tx="$FIXDIR/treatment-$n.txt"
    if [[ $REPLAY -eq 0 ]]; then
        dispatch_live control "$c_tx"
        dispatch_live treatment "$t_tx"
    fi
    c_res="$(run_discriminator "$c_tx")"
    t_res="$(run_discriminator "$t_tx")"

    [[ "$c_res" != "DEGRADED" ]] && C_USABLE=$((C_USABLE + 1))
    [[ "$c_res" == "PRESENT" ]] && C_PRESENT=$((C_PRESENT + 1))
    [[ "$t_res" != "DEGRADED" ]] && T_USABLE=$((T_USABLE + 1))
    [[ "$t_res" == "PRESENT" ]] && T_PRESENT=$((T_PRESENT + 1))

    entry="$(printf '{"rep":%d,"control":"%s","treatment":"%s"}' "$n" "$c_res" "$t_res")"
    PER_REP_JSON="${PER_REP_JSON:+$PER_REP_JSON,}$entry"
done

# --- verdict ------------------------------------------------------------------
rate() { local n="$1" d="$2"; [[ "$d" -eq 0 ]] && { echo "null"; return; }; python3 -c "print(round($n/$d,4))"; }
C_RATE="$(rate "$C_PRESENT" "$C_USABLE")"
T_RATE="$(rate "$T_PRESENT" "$T_USABLE")"

if [[ "$T_USABLE" -eq 0 ]]; then
    VERDICT="UNMEASURED"
else
    # BEHAVIORAL iff treatment did the thing strictly more than control.
    cmp_res="$(python3 -c "
tu=$T_USABLE; cu=$C_USABLE
tr=$T_PRESENT/tu
cr=($C_PRESENT/cu) if cu>0 else 0.0
print('BEHAVIORAL' if tr>cr else 'INERT')")"
    VERDICT="$cmp_res"
fi

MODE="live"; [[ $REPLAY -eq 1 ]] && MODE="replay"
GEN_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

SCORECARD="$(cat <<EOF
{
  "schema": "agentops-skill-probe.v1",
  "probe": "$PROBE",
  "skill": "$SKILL",
  "mode": "$MODE",
  "generated_at": "$GEN_AT",
  "reps": $REPS,
  "honesty": "measures BEHAVIOR-CHANGE (did the loaded skill change what the agent DID), NOT quality-uplift; small N is directional (ADR-0011)",
  "control": {"present": $C_PRESENT, "usable": $C_USABLE, "rate": $C_RATE},
  "treatment": {"present": $T_PRESENT, "usable": $T_USABLE, "rate": $T_RATE},
  "verdict": "$VERDICT",
  "per_rep": [$PER_REP_JSON]
}
EOF
)"

printf '%s' "$SCORECARD" | python3 -c 'import sys,json; json.load(sys.stdin)' \
    || { echo "error: produced malformed scorecard JSON" >&2; exit 1; }

if [[ -n "$OUTPUT" ]]; then
    printf '%s\n' "$SCORECARD" > "$OUTPUT"
    echo "scorecard written: $OUTPUT" >&2
else
    printf '%s\n' "$SCORECARD"
fi

# In --capture we keep the fixtures (default for live). In pure --replay we never
# wrote them. Nothing else to do.
[[ $CAPTURE -eq 1 ]] && echo "fixtures captured under: $FIXDIR" >&2
exit 0
