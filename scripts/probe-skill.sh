#!/usr/bin/env bash
# probe-skill.sh — skill BEHAVIORAL probe harness (age-e508.1).
#
# ============================ HONESTY HEADER =================================
# A probe measures BEHAVIOR-CHANGE, NOT quality-uplift. It answers exactly one
# question: when the declared treatment source is injected (treatment) versus
# omitted (control), does the agent actually DO the thing differently — a tool call made, an
# artifact produced, a sequence followed? It NEVER scores whether the text
# mentions the skill, and it NEVER claims the skill makes output better. A
# BEHAVIORAL verdict means the treatment increased the scored behavior; INERT
# means equal rates; REGRESSIVE means the treatment reduced it. The historical 2026-06-30 graphify report (0/2
# treatment responses obeyed the guidance) predates immutable capture metadata
# and remains LEGACY-UNVERIFIED, not a current harness result. Small N (default
# 2-3) is DIRECTIONAL, not statistical. Do not overclaim (ADR-0011 discipline).
# ============================================================================
#
# A PROBE is a directory under evals/skill-probes/<id>/ carrying:
#   probe.json            metadata: id, skill, reps, behavior, discriminator
#   question.md           the scenario question — IDENTICAL for both arms
#   treatment-prelude.md  an optional distilled treatment injected ONLY when
#                         probe.json declares treatment_source=injected-prelude;
#                         this mode measures the bound prelude, not full-skill
#                         activation, and does not qualify as skill-tier coverage
#   discriminator.sh      a DETERMINISTIC check over a prompt-free response envelope:
#                         exit 0 = behavior PRESENT, 1 = ABSENT, 2 = infra error
#   fixtures/             recorded transcripts control-<n>.txt / treatment-<n>.txt
#                         as one bound prompt event followed by native Codex JSONL
#   fixtures/capture-contract.json
#                         pre-dispatch binding over exact prompt bytes, producer
#                         request/runtime executable identity, schedule, and scoring
#   fixtures/fixture-set.json
#                         immutable capture metadata: exact transcripts and thread
#                         ids, evaluation-input and canonical SKILL.md inventories,
#                         evaluator identity, per-file SHA-256 digests, and one
#                         binding digest over the complete capture contract
#
# MODES:
#   live (default)  dispatch a cross-family worker (codex exec — the sanctioned
#                   headless path; NEVER claude -p, LAW 0) for each arm x rep,
#                   capture the transcript, run the discriminator, and publish
#                   a new immutable fixture set so its bound classification is
#                   replayable. Existing fixture sets are never overwritten.
#   --replay        skip dispatch; verify immutable capture metadata and then run
#                   the discriminator over the bound transcripts. Legacy fixture
#                   sets without metadata fail closed; they are not retroactively
#                   blessed as verified captures.
#
# VERDICT: BEHAVIORAL iff treatment_rate > control_rate; REGRESSIVE iff lower;
# INERT iff equal; UNMEASURED iff either arm has no usable reps or a live capture
# is incomplete (a durable delta needs two measured arms and replayable evidence).
#
# Usage:
#   bash scripts/probe-skill.sh --probe rpi --replay
#   bash scripts/probe-skill.sh --probe rpi --replay --fixtures fixtures-xhigh-2026-08-04
#   bash scripts/probe-skill.sh --probe rpi --reps 2 --output out.json
#   bash scripts/probe-skill.sh --probe rpi --live --capture
#   bash scripts/probe-skill.sh --probe rpi --live --model gpt-5-mini
#
# Flags: --probe <id> (required) · --replay | --live · --capture · --reps N ·
#        --fixtures <directory-name> ·
#        --output <path> · --timeout <secs> · --model <id> (weaker producer, the
#        ratchet when a frontier producer aces both arms) · --effort <level>
#        (low|medium|high|xhigh — sets codex model_reasoning_effort; the SECOND
#        ratchet: when even a weaker model id aces both arms at the config
#        default effort, lower the effort to surface headroom. 2026-08-04 wave-1
#        finding: gpt-5.6-luna at xhigh aced 4/6 control arms).
#
# Env overrides (test seams): SKILL_PROBES_DIR (default $REPO_ROOT/evals/skill-probes),
# SKILL_PROBE_SKILLS_DIR (default $REPO_ROOT/skills), PROBE_FIXTURE_SET
# (default fixtures), PROBE_SEAL (seatbelt|none, default seatbelt)
#
# FILESYSTEM SEAL (move 2): a live rep must not be able to read the skill it
# is being measured on. Without a seal a rep inherits the operator's skill
# roots — codex walks $HOME/.agents/skills regardless of CODEX_HOME, and
# ~/.agents/skills, ~/.codex/skills, ~/.claude/skills and ~/.gemini/skills all
# symlink into this checkout — and 2026-08-28 control-arm reps read
# skills/<skill>/SKILL.md off disk by absolute path (skill-read-contamination).
# Every live dispatch therefore runs the rep:
#   * under a scratch HOME/CODEX_HOME ($SEAL_HOME, auth.json and config.toml
#     symlinked from the real CODEX_HOME so codex can authenticate; no
#     $SEAL_HOME/.agents/skills exists), from the live workspace as cwd;
#   * inside a seatbelt (macOS sandbox-exec) profile that denies file-read* on
#     the checkout root, the resolved skills dir, $REAL_HOME/.agents,
#     $REAL_HOME/.claude/skills, $REAL_HOME/.gemini/skills and the real
#     ~/.codex/skills, and denies file-write* everywhere except the live
#     workspace, $SEAL_HOME, /private/tmp, /private/var/folders and $TMPDIR,
#     so a rep whose own codex sandbox is bypassed stays read-only against the
#     real filesystem. The profile is handed to scripts/lib/codex-exec.sh as
#     the CODEX_EXEC_WRAP command prefix `(sandbox-exec -p "<profile>")`.
# Fail closed: when sandbox-exec is absent the live dispatch refuses. Only an
# explicit PROBE_SEAL=none runs unsealed; that run prints
# `seal: none (coverage-ineligible)` and records seal_mode=none. The seal record
# (seal.json: seal_mode, the profile text, the denied roots) is written into the
# capture stage before the first rep and echoed into the scorecard under `seal`.
#
# practices: [measurement-over-assertion, ab-testing]
# shellcheck source=scripts/lib/preamble.sh disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"
# shellcheck source=scripts/lib/codex-exec.sh disable=SC1091
. "$REPO_ROOT/scripts/lib/codex-exec.sh"

PROBES_DIR="${SKILL_PROBES_DIR:-$REPO_ROOT/evals/skill-probes}"
SKILLS_DIR="${SKILL_PROBE_SKILLS_DIR:-$REPO_ROOT/skills}"
PROBE=""
REPLAY=0
CAPTURE=0
REPS=""
REPS_EXPLICIT=0
OUTPUT=""
TIMEOUT="${PROBE_TIMEOUT:-240}"
MODEL="${PROBE_MODEL:-}"
EFFORT="${PROBE_EFFORT:-}"
MODEL_CONSTRAINT=0
EFFORT_CONSTRAINT=0
if [[ -n "$MODEL" ]]; then MODEL_CONSTRAINT=1; fi
if [[ -n "$EFFORT" ]]; then EFFORT_CONSTRAINT=1; fi
FIXTURE_SET="${PROBE_FIXTURE_SET:-fixtures}"

usage() { grep '^#' "$0" | sed 's/^# \?//'; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --probe)   PROBE="${2:-}"; shift 2;;
        --replay)  REPLAY=1; shift;;
        --live)    REPLAY=0; shift;;
        --capture) CAPTURE=1; shift;;
        --reps)    REPS="${2:-}"; REPS_EXPLICIT=1; shift 2;;
        --fixtures|--fixture-set) FIXTURE_SET="${2:-}"; shift 2;;
        --output)  OUTPUT="${2:-}"; shift 2;;
        --timeout) TIMEOUT="${2:-}"; shift 2;;
        --model)   MODEL="${2:-}"; MODEL_CONSTRAINT=1; shift 2;;
        --effort)  EFFORT="${2:-}"; EFFORT_CONSTRAINT=1; shift 2;;
        -h|--help) usage; exit 0;;
        *) echo "Unknown flag: $1" >&2; exit 2;;
    esac
done

[[ -n "$PROBE" ]] || { echo "error: --probe <id> required" >&2; exit 2; }
[[ "$PROBE" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]] \
    || { echo "error: unsafe probe id: $PROBE" >&2; exit 2; }
[[ "$FIXTURE_SET" =~ ^fixtures([_-][A-Za-z0-9][A-Za-z0-9._-]*)?$ ]] \
    || { echo "error: --fixtures must name a fixtures directory inside the probe: $FIXTURE_SET" >&2; exit 2; }
PROBE_DIR="$PROBES_DIR/$PROBE"
[[ -d "$PROBE_DIR" ]] || { echo "error: probe not found: $PROBE_DIR" >&2; exit 2; }
DISC="$PROBE_DIR/discriminator.sh"
QUESTION="$PROBE_DIR/question.md"
META="$PROBE_DIR/probe.json"
if [[ $REPLAY -eq 0 ]]; then
    for f in "$DISC" "$QUESTION" "$META"; do
        [[ -f "$f" && ! -L "$f" ]] || { echo "error: probe file missing or unsafe: $f" >&2; exit 2; }
    done
fi

# Read probe metadata (python3, no jq dependency).
json_get() { python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d.get(sys.argv[2],""))' "$META" "$1"; }

FIXDIR="$PROBE_DIR/$FIXTURE_SET"
FIXTURE_META_TOOL="$REPO_ROOT/scripts/lib/probe-fixture-metadata.py"
[[ -f "$FIXTURE_META_TOOL" ]] || { echo "error: fixture metadata helper missing: $FIXTURE_META_TOOL" >&2; exit 2; }
HARNESS_PATH="$REPO_ROOT/scripts/probe-skill.sh"
PREAMBLE_PATH="$REPO_ROOT/scripts/lib/preamble.sh"
DISPATCH_HELPER_PATH="$REPO_ROOT/scripts/lib/codex-exec.sh"

if [[ -n "$OUTPUT" ]]; then
    OUTPUT_DIR="$(dirname "$OUTPUT")"
    [[ -d "$OUTPUT_DIR" ]] || { echo "error: scorecard output directory does not exist: $OUTPUT_DIR" >&2; exit 2; }
    [[ ! -e "$OUTPUT" && ! -L "$OUTPUT" ]] \
        || { echo "error: refusing to overwrite immutable scorecard output: $OUTPUT" >&2; exit 2; }
fi

summary_get() {
    local summary="$1" path="$2"
    python3 -c '
import json, sys
value = json.loads(sys.argv[1])
for part in sys.argv[2].split("."):
    if not isinstance(value, dict) or part not in value:
        value = None
        break
    value = value[part]
print("" if value is None else value)
' "$summary" "$path"
}

summary_json() {
    local summary="$1" path="$2"
    python3 -c '
import json, sys
value = json.loads(sys.argv[1])
for part in sys.argv[2].split("."):
    value = value[part]
print(json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")))
' "$summary" "$path"
}

SKILL=""
TREATMENT_SOURCE=""
if [[ $REPLAY -eq 0 ]]; then
    if ! PROBE_CONTRACT="$(python3 "$FIXTURE_META_TOOL" probe-contract \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS_DIR" \
        --probe "$PROBE")"; then
        echo "error: probe contract is incomplete or unsafe" >&2
        exit 2
    fi
    SKILL="$(summary_get "$PROBE_CONTRACT" canonical_skill.name)"
    CONTRACT_REPS="$(summary_get "$PROBE_CONTRACT" reps)"
    TREATMENT_SOURCE="$(summary_get "$PROBE_CONTRACT" treatment_source)"
fi

PRODUCER_MODEL=""
PRODUCER_EFFORT=""
PRODUCER_JSON=""
CAPTURE_REQUESTED_MODEL="$MODEL"
CAPTURE_REQUESTED_EFFORT="$EFFORT"
FIXTURE_BINDING=""
FIXTURE_SCHEMA=""
CAPTURE_EVALUATOR=""
CURRENT_EVALUATOR="$(python3 "$FIXTURE_META_TOOL" identity \
    --harness "$HARNESS_PATH" \
    --preamble "$PREAMBLE_PATH" \
    --dispatch-helper "$DISPATCH_HELPER_PATH")"

if [[ $REPLAY -eq 1 ]]; then
    [[ -d "$FIXDIR" && ! -L "$FIXDIR" ]] \
        || { echo "error: replay fixture set not found or unsafe: $FIXDIR" >&2; exit 2; }
    if ! FIXTURE_METADATA="$(python3 "$FIXTURE_META_TOOL" verify --fixture-dir "$FIXDIR" --probe-dir "$PROBE_DIR" --skills-dir "$SKILLS_DIR" --probe "$PROBE")"; then
        echo "error: replay refused: fixture metadata is missing or failed verification" >&2
        exit 2
    fi
    MANIFEST_REPS="$(summary_get "$FIXTURE_METADATA" reps)"
    if [[ $REPS_EXPLICIT -eq 1 && "$REPS" != "$MANIFEST_REPS" ]]; then
        echo "error: --reps $REPS does not match fixture metadata reps $MANIFEST_REPS" >&2
        exit 2
    fi
    REPS="$MANIFEST_REPS"
    PRODUCER_MODEL="$(summary_get "$FIXTURE_METADATA" producer.model)"
    PRODUCER_EFFORT="$(summary_get "$FIXTURE_METADATA" producer.effort)"
    PRODUCER_JSON="$(summary_json "$FIXTURE_METADATA" producer)"
    CAPTURE_REQUESTED_MODEL="$(summary_get "$FIXTURE_METADATA" requested_producer.model)"
    CAPTURE_REQUESTED_EFFORT="$(summary_get "$FIXTURE_METADATA" requested_producer.effort)"
    FIXTURE_BINDING="$(summary_get "$FIXTURE_METADATA" binding_sha256)"
    FIXTURE_SCHEMA="$(summary_get "$FIXTURE_METADATA" schema)"
    CAPTURE_EVALUATOR="$(python3 -c 'import json,sys; print(json.dumps(json.loads(sys.argv[1])["capture_evaluator"],sort_keys=True,separators=(",",":")))' "$FIXTURE_METADATA")"
    TREATMENT_SOURCE="$(summary_get "$FIXTURE_METADATA" treatment_source)"
    SKILL="$(summary_get "$FIXTURE_METADATA" canonical_skill.name)"
    if [[ -z "$SKILL" && -f "$META" && ! -L "$META" ]]; then
        SKILL="$(json_get skill)"
    fi
    [[ -n "$SKILL" ]] || { echo "error: verified fixture does not identify a skill" >&2; exit 2; }
    if [[ $MODEL_CONSTRAINT -eq 1 && "$MODEL" != "$PRODUCER_MODEL" ]]; then
        echo "error: replay --model $MODEL does not match bound fixture producer request $PRODUCER_MODEL" >&2
        exit 2
    fi
    if [[ $EFFORT_CONSTRAINT -eq 1 && "$EFFORT" != "$PRODUCER_EFFORT" ]]; then
        echo "error: replay --effort $EFFORT does not match bound fixture producer request $PRODUCER_EFFORT" >&2
        exit 2
    fi
else
    [[ -n "$REPS" ]] || REPS="$CONTRACT_REPS"
    if [[ "$REPS" != "$CONTRACT_REPS" ]]; then
        echo "error: --reps $REPS does not match bound probe.json reps $CONTRACT_REPS" >&2
        exit 2
    fi
    [[ ! -e "$FIXDIR" && ! -L "$FIXDIR" ]] || {
        echo "error: refusing to overwrite immutable fixture set: $FIXDIR" >&2
        echo "  choose a new --fixtures name for this live capture" >&2
        exit 2
    }
fi

# --- filesystem seal ---------------------------------------------------------
PROBE_SEAL="${PROBE_SEAL:-seatbelt}"
SEAL_MODE=""
SEAL_HOME=""
SEAL_PROFILE=""
SEAL_PROFILE_FILE=""
SEAL_PROFILE_SHA=""
SEAL_SANDBOX_EXEC=""
SEAL_JSON=""
SEAL_DENIED_ROOTS=()
SEAL_WRITABLE_ROOTS=()
SEAL_AUTH_LINKS=()
REAL_HOME="${HOME:-}"
REAL_CODEX_HOME="${CODEX_HOME:-$REAL_HOME/.codex}"
REP_HOME="$REAL_HOME"
REP_CODEX_HOME="$REAL_CODEX_HOME"
# shellcheck disable=SC2034 # consumed by codex_exec_guarded in the sourced library
CODEX_EXEC_WRAP=()

# resolve_seal_mode — decide seatbelt|none BEFORE any stage or workspace exists,
# so a refused seal leaves nothing behind. Absent sandbox-exec fails closed.
resolve_seal_mode() {
    case "$PROBE_SEAL" in
        none)
            SEAL_MODE=none
            echo "seal: none (coverage-ineligible)" >&2
            ;;
        seatbelt)
            if ! SEAL_SANDBOX_EXEC="$(command -v sandbox-exec 2>/dev/null)"; then
                echo "error: filesystem seal unavailable: sandbox-exec is not on PATH" >&2
                echo "  an unsealed rep inherits the operator's skill roots (skill-read-contamination);" >&2
                echo "  set PROBE_SEAL=none explicitly to run unsealed (the capture is coverage-ineligible)" >&2
                exit 2
            fi
            SEAL_MODE=seatbelt
            ;;
        *)
            echo "error: PROBE_SEAL must be seatbelt|none, got: $PROBE_SEAL" >&2
            exit 2
            ;;
    esac
}

seal_realpath() {
    python3 -c 'import os, sys; print(os.path.realpath(sys.argv[1]))' "$1"
}

# seal_path_ok PATH — a seatbelt profile quotes paths with double quotes; refuse
# any path that could break out of the literal.
seal_path_ok() {
    [[ -n "$1" && "$1" != *[\"\\]* && "$1" != *$'\n'* ]]
}

# seal_add_root ARRAY_NAME PATH — append PATH and its resolved form (deduped).
seal_add_root() {
    local -n roots="$1"
    local candidate resolved existing present
    resolved="$(seal_realpath "$2")"
    for candidate in "$2" "$resolved"; do
        seal_path_ok "$candidate" || { echo "error: seal cannot quote path: $candidate" >&2; exit 2; }
        present=0
        for existing in "${roots[@]}"; do
            [[ "$existing" != "$candidate" ]] || { present=1; break; }
        done
        [[ $present -eq 1 ]] || roots+=("$candidate")
    done
}

# build_seal — materialize the scratch HOME, the seatbelt profile, and the
# CODEX_EXEC_WRAP prefix. Needs LIVE_WORKSPACE.
build_seal() {
    local checkout root name
    SEAL_HOME="$(mktemp -d "${TMPDIR:-/tmp}/probe-seal.XXXXXX")"
    chmod 0700 "$SEAL_HOME"
    mkdir -p "$SEAL_HOME/.codex"
    for name in auth.json config.toml; do
        if [[ -f "$REAL_CODEX_HOME/$name" ]]; then
            ln -s "$REAL_CODEX_HOME/$name" "$SEAL_HOME/.codex/$name"
            SEAL_AUTH_LINKS+=("$name")
        fi
    done
    if [[ ! -f "$REAL_CODEX_HOME/auth.json" ]]; then
        echo "seal: no auth.json under $REAL_CODEX_HOME; a real producer cannot authenticate" >&2
    fi
    REP_HOME="$SEAL_HOME"
    REP_CODEX_HOME="$SEAL_HOME/.codex"

    checkout="$(seal_realpath "$REPO_ROOT")"
    for root in "$(seal_realpath "$LIVE_WORKSPACE")" "$(seal_realpath "$SEAL_HOME")"; do
        if [[ "$root" == "$checkout" || "$root" == "$checkout"/* ]]; then
            echo "error: seal cannot place a writable root inside the checkout: $root" >&2
            exit 2
        fi
    done
    seal_add_root SEAL_DENIED_ROOTS "$REPO_ROOT"
    seal_add_root SEAL_DENIED_ROOTS "$SKILLS_DIR"
    seal_add_root SEAL_DENIED_ROOTS "$REAL_HOME/.agents"
    seal_add_root SEAL_DENIED_ROOTS "$REAL_HOME/.claude/skills"
    seal_add_root SEAL_DENIED_ROOTS "$REAL_HOME/.gemini/skills"
    seal_add_root SEAL_DENIED_ROOTS "$REAL_HOME/.codex/skills"
    seal_add_root SEAL_DENIED_ROOTS "$REAL_CODEX_HOME/skills"
    seal_add_root SEAL_WRITABLE_ROOTS "$LIVE_WORKSPACE"
    seal_add_root SEAL_WRITABLE_ROOTS "$SEAL_HOME"
    seal_add_root SEAL_WRITABLE_ROOTS /private/tmp
    seal_add_root SEAL_WRITABLE_ROOTS /private/var/folders
    seal_add_root SEAL_WRITABLE_ROOTS "${TMPDIR:-/tmp}"
    seal_add_root SEAL_WRITABLE_ROOTS /dev

    # Seatbelt: the last matching rule wins, so the write allow-list follows the
    # blanket write deny, and the read denies come last.
    SEAL_PROFILE='(version 1)'$'\n''(allow default)'$'\n''(deny file-write*)'$'\n''(allow file-write*'
    for root in "${SEAL_WRITABLE_ROOTS[@]}"; do SEAL_PROFILE+=$'\n'"  (subpath \"$root\")"; done
    SEAL_PROFILE+=')'$'\n''(deny file-read*'
    for root in "${SEAL_DENIED_ROOTS[@]}"; do SEAL_PROFILE+=$'\n'"  (subpath \"$root\")"; done
    SEAL_PROFILE+=')'
    SEAL_PROFILE_FILE="$SEAL_HOME/seal.sb"
    printf '%s\n' "$SEAL_PROFILE" > "$SEAL_PROFILE_FILE"
    SEAL_PROFILE_SHA="sha256:$(python3 -c 'import hashlib, sys; print(hashlib.sha256(sys.argv[1].encode()).hexdigest())' "$SEAL_PROFILE")"
    # shellcheck disable=SC2034 # consumed by codex_exec_guarded in the sourced library
    CODEX_EXEC_WRAP=(sandbox-exec -p "$SEAL_PROFILE")
}

# write_seal_record PATH — the seal.json record (also echoed into the scorecard).
write_seal_record() {
    local wrap_json
    wrap_json="$(python3 -c 'import json, sys; print(json.dumps(sys.argv[1:]))' "${CODEX_EXEC_WRAP[@]}")"
    SEAL_JSON="$(python3 - "$1" "$SEAL_MODE" "$SEAL_SANDBOX_EXEC" "$SEAL_PROFILE" \
        "$SEAL_PROFILE_FILE" "$SEAL_PROFILE_SHA" "$wrap_json" \
        "$(printf '%s\n' "${SEAL_DENIED_ROOTS[@]}")" \
        "$(printf '%s\n' "${SEAL_WRITABLE_ROOTS[@]}")" \
        "$REP_HOME" "$REP_CODEX_HOME" "$REAL_HOME" "$REAL_CODEX_HOME" \
        "$(printf '%s\n' "${SEAL_AUTH_LINKS[@]}")" "$(uname -s)" <<'PY'
import json
import sys

(
    path, mode, sandbox_exec, profile, profile_file, profile_sha, wrap_json,
    denied, writable, rep_home, rep_codex_home, real_home, real_codex_home,
    auth_links, platform,
) = sys.argv[1:]
sealed = mode == "seatbelt"
record = {
    "schema": "agentops-skill-probe-seal.v1",
    "seal_mode": mode,
    "coverage_eligible": sealed,
    "mechanism": "sandbox-exec" if sealed else None,
    "sandbox_exec": sandbox_exec or None,
    "platform": platform,
    "wrap": json.loads(wrap_json),
    "profile": profile or None,
    "profile_file": profile_file or None,
    "profile_sha256": profile_sha or None,
    "denied_read_roots": [line for line in denied.split("\n") if line],
    "writable_roots": [line for line in writable.split("\n") if line],
    "rep_env": {"HOME": rep_home, "CODEX_HOME": rep_codex_home},
    "real_home": real_home,
    "real_codex_home": real_codex_home,
    "auth_links": [line for line in auth_links.split("\n") if line],
}
encoded = json.dumps(record, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
with open(path, "x", encoding="utf-8") as handle:
    handle.write(encoded)
print(json.dumps(record, ensure_ascii=False, sort_keys=True, separators=(",", ":")))
PY
)" || { echo "error: could not write seal record: $1" >&2; exit 2; }
}

if [[ $REPLAY -eq 0 ]]; then
    resolve_seal_mode
fi

[[ "$REPS" =~ ^[1-9][0-9]*$ ]] || { echo "error: --reps must be a positive integer, got: $REPS" >&2; exit 2; }
[[ "$REPS" -le 20 ]] || { echo "error: --reps must not exceed 20, got: $REPS" >&2; exit 2; }
[[ "$TIMEOUT" =~ ^[0-9]+$ ]] || { echo "error: --timeout must be a non-negative integer, got: $TIMEOUT" >&2; exit 2; }

# Structured JSONL is mandatory for new captures. --effort plumbs through the
# codex-exec lib's CODEX_EXEC_EXTRA_ARGS array
# (arrays cannot cross a process boundary, so the flag lives here, in the same
# shell that sources the lib). Applied to BOTH arms — the producer config must
# stay symmetric or the delta is confounded.
if [[ $REPLAY -eq 0 ]]; then
    # shellcheck disable=SC2034 # consumed by codex_exec_guarded in the sourced library
    CODEX_EXEC_EXTRA_ARGS=(--json --ephemeral)
fi
if [[ $REPLAY -eq 0 && -n "$EFFORT" ]]; then
    # shellcheck disable=SC2034 # CODEX_EXEC_EXTRA_ARGS is consumed by codex_exec_guarded in the sourced codex-exec.sh
    case "$EFFORT" in
        low|medium|high|xhigh) CODEX_EXEC_EXTRA_ARGS+=(-c "model_reasoning_effort=\"$EFFORT\"");;
        *) echo "error: --effort must be low|medium|high|xhigh, got: $EFFORT" >&2; exit 2;;
    esac
fi

# dispatch_live ARM REP TRANSCRIPT_OUT -> bind the exact prompt event, capture
# native Codex JSONL, and populate TRANSCRIPT_OUT with one structured envelope.
dispatch_live() {
    local arm="$1" rep="$2" transcript="$3" receipt_name="$4" result_name="$5"
    local rc=0 receipt="" outcome="DEGRADED" had_noclobber=0
    local prompt_file="$LIVE_WORKSPACE/$arm-$rep.prompt"
    local runtime_file="$LIVE_WORKSPACE/$arm-$rep.codex.jsonl"
    local stderr_file="$LIVE_WORKSPACE/$arm-$rep.codex.stderr"
    if ! python3 "$FIXTURE_META_TOOL" capture-file \
        --fixture-dir "$LIVE_STAGE" --probe "$PROBE" \
        --name "prompt-$arm" >"$prompt_file"; then
        echo "probe-skill: could not materialize bound $arm-$rep prompt" >&2
        printf -v "$receipt_name" '%s' ""
        printf -v "$result_name" '%s' "$outcome"
        return 1
    fi
    if [[ -o noclobber ]]; then
        had_noclobber=1
    else
        set -o noclobber
    fi
    if ! exec 9> "$transcript"; then
        [[ $had_noclobber -eq 1 ]] || set +o noclobber
        echo "probe-skill: $arm dispatch refused unsafe/existing transcript sink" >&2
        printf -v "$receipt_name" '%s' ""
        printf -v "$result_name" '%s' "$outcome"
        return 1
    fi
    [[ $had_noclobber -eq 1 ]] || set +o noclobber
    # read-only sandbox: the probe only wants the agent's PLAN text, no mutation.
    # --model routes a WEAKER producer (e.g. gpt-5-mini) — the ratchet for
    # surfacing a skill's behavioral value when a frontier producer aces both arms
    # (the membrane-eval-too-easy lesson). Empty => the codex default (frontier).
    # The rep runs with the sealed HOME/CODEX_HOME and the live workspace as cwd:
    # a cwd inside the denied checkout makes every shell child print a getcwd
    # error on stderr, which degrades the rep. CODEX_EXEC_WRAP (the seatbelt
    # prefix) reaches the library through the shared shell, arrays never cross
    # a process boundary.
    (
        cd "$LIVE_WORKSPACE" || exit 70
        REVIEWER=codex \
        REVIEWER_MARKER=turn.completed \
        CODEX_EXEC_PROMPT_FILE="$prompt_file" \
        CODEX_EXEC_DIR="$LIVE_WORKSPACE" \
        CODEX_EXEC_SANDBOX=read-only \
        CODEX_EXEC_SKIP_GIT_CHECK=1 \
        CODEX_EXEC_TIMEOUT="$TIMEOUT" \
        CODEX_EXEC_MODEL="$MODEL" \
        CODEX_EXEC_OUT_FILE="$runtime_file" \
        CODEX_EXEC_STDERR_FILE="$stderr_file" \
        CODEX_EXEC_EXPECT_OUTPUT=1 \
        HOME="$REP_HOME" \
        CODEX_HOME="$REP_CODEX_HOME" \
        PROBE_SEAL_PROFILE_FILE="$SEAL_PROFILE_FILE" \
            codex_exec_guarded >/dev/null
    ) || rc=$?
    # Producer stderr fails the rep closed. ONE literal is excluded: codex-cli
    # >= 0.14 announces stdin prompt delivery with "Reading prompt from
    # stdin...", and THIS harness chose stdin delivery ten lines above
    # (CODEX_EXEC_PROMPT_FILE), so that line is the harness hearing its own
    # echo, not a producer diagnostic — on codex-cli 0.145.0 it accompanies a
    # zero exit and a complete JSONL stream, which degraded 100% of live reps
    # and made every live capture UNMEASURED. Every other byte the producer
    # writes still degrades the rep, and the full stderr is still echoed. This
    # admits a dispatch; it can never turn ABSENT into PRESENT, and the bound
    # prompt event, transcript inventory, and discriminator still decide the
    # rep.
    if [[ -s "$stderr_file" ]]; then
        cat "$stderr_file" >&2
        if grep -qvxF 'Reading prompt from stdin...' "$stderr_file"; then
            [[ "$rc" -ne 0 ]] || rc=2
        fi
    fi
    if [[ "$rc" -eq 0 ]]; then
        python3 "$FIXTURE_META_TOOL" assemble-transcript \
            --runtime-file "$runtime_file" --prompt-file "$prompt_file" \
            --fixture-dir "$LIVE_STAGE" --probe "$PROBE" \
            --arm "$arm" --rep "$rep" >&9 || rc=$?
    fi
    if [[ "$rc" -eq 0 ]]; then
        receipt="$(python3 "$FIXTURE_META_TOOL" classify-open \
            --fd 0 --path "$transcript" --fixture-dir "$LIVE_STAGE" \
            --probe "$PROBE" --arm "$arm" --rep "$rep" <&9)" || rc=$?
    fi
    exec 9>&-
    if [[ "$rc" -ne 0 || -z "$receipt" ]]; then
        echo "probe-skill: $arm dispatch degraded (rc=$rc); no transcript accepted" >&2
        printf -v "$receipt_name" '%s' ""
        printf -v "$result_name" '%s' "$outcome"
        return 1
    fi
    outcome="$(summary_get "$receipt" outcome)"
    printf -v "$receipt_name" '%s' "$receipt"
    printf -v "$result_name" '%s' "$outcome"
    return 0
}

LIVE_STAGE=""
LIVE_ALL_DISPATCH_OK=1

publish_fixture_set() {
    local stage="$1" target="$2"
    python3 "$FIXTURE_META_TOOL" publish \
        --stage-dir "$stage" \
        --target-dir "$target" \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS_DIR" \
        --probe "$PROBE" >/dev/null || return 1
    LIVE_STAGE=""
    return 0
}

if [[ $REPLAY -eq 0 ]]; then
    LIVE_STAGE="$(mktemp -d "$PROBE_DIR/.${FIXTURE_SET}.capture.XXXXXX")"
    chmod 0700 "$LIVE_STAGE"
    SNAPSHOT_ARGS=(
        snapshot
        --fixture-dir "$LIVE_STAGE"
        --probe-dir "$PROBE_DIR"
        --skills-dir "$SKILLS_DIR"
        --probe "$PROBE"
    )
    if [[ -n "$MODEL" ]]; then SNAPSHOT_ARGS+=(--requested-model "$MODEL"); fi
    if [[ -n "$EFFORT" ]]; then SNAPSHOT_ARGS+=(--requested-effort "$EFFORT"); fi
    if [[ -n "${CODEX_EXEC_BIN:-}" ]]; then
        SNAPSHOT_ARGS+=(--producer-override-bin "$CODEX_EXEC_BIN")
    fi
    if ! python3 "$FIXTURE_META_TOOL" "${SNAPSHOT_ARGS[@]}" >/dev/null; then
        echo "error: live capture inputs changed while creating the pre-dispatch snapshot" >&2
        exit 2
    fi
    if ! CURRENT_CONTRACT="$(python3 "$FIXTURE_META_TOOL" probe-contract \
        --probe-dir "$PROBE_DIR" --skills-dir "$SKILLS_DIR" \
        --probe "$PROBE")" || [[ "$CURRENT_CONTRACT" != "$PROBE_CONTRACT" ]]; then
        echo "error: live capture inputs changed while creating the pre-dispatch snapshot" >&2
        exit 2
    fi
    LIVE_WORKSPACE="$(mktemp -d "${TMPDIR:-/tmp}/probe-ws.XXXXXX")"
    chmod 0700 "$LIVE_WORKSPACE"
    if [[ "$SEAL_MODE" == "seatbelt" ]]; then
        build_seal
    fi
    # The seal record lands in the capture stage before the first rep runs.
    write_seal_record "$LIVE_STAGE/seal.json"
    if [[ "$SEAL_MODE" == "seatbelt" ]]; then
        echo "seal: seatbelt ($SEAL_SANDBOX_EXEC; ${#SEAL_DENIED_ROOTS[@]} denied read roots)" >&2
    fi
fi

C_PRESENT=0; C_USABLE=0; T_PRESENT=0; T_USABLE=0
PER_REP_JSON=""
declare -A LIVE_RESULTS=()
declare -A LIVE_DIGESTS=()

if [[ $REPLAY -eq 0 ]]; then
    while IFS=$'\t' read -r arm rep; do
        key="$arm-$rep"
        transcript="$LIVE_STAGE/$key.txt"
        dispatch_receipt=""
        dispatch_result="DEGRADED"
        if dispatch_live "$arm" "$rep" "$transcript" dispatch_receipt dispatch_result; then
            model="$(summary_get "$dispatch_receipt" producer.model)"
            effort="$(summary_get "$dispatch_receipt" producer.effort)"
            PRODUCER_MODEL="$model"
            PRODUCER_EFFORT="$effort"
            LIVE_DIGESTS["$key"]="$(summary_get "$dispatch_receipt" sha256)"
        else
            LIVE_ALL_DISPATCH_OK=0
        fi
        LIVE_RESULTS["$key"]="$dispatch_result"
    done < <(python3 -c '
import json, sys
for entry in json.loads(sys.argv[1])["schedule"]:
    print(f"{entry['"'"'arm'"'"']}\t{entry['"'"'rep'"'"']}")
' "$PROBE_CONTRACT")

    for ((n=1; n<=REPS; n++)); do
        c_res="${LIVE_RESULTS[control-$n]:-DEGRADED}"
        t_res="${LIVE_RESULTS[treatment-$n]:-DEGRADED}"
        if [[ "$c_res" != "DEGRADED" ]]; then C_USABLE=$((C_USABLE + 1)); fi
        if [[ "$c_res" == "PRESENT" ]]; then C_PRESENT=$((C_PRESENT + 1)); fi
        if [[ "$t_res" != "DEGRADED" ]]; then T_USABLE=$((T_USABLE + 1)); fi
        if [[ "$t_res" == "PRESENT" ]]; then T_PRESENT=$((T_PRESENT + 1)); fi
        entry="$(printf '{"rep":%d,"control":"%s","treatment":"%s"}' "$n" "$c_res" "$t_res")"
        PER_REP_JSON="${PER_REP_JSON:+$PER_REP_JSON,}$entry"
    done
else
    if ! SCORE_SUMMARY="$(python3 "$FIXTURE_META_TOOL" score \
        --fixture-dir "$FIXDIR" --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS_DIR" --probe "$PROBE")"; then
        echo "error: replay scoring failed; no scorecard will be emitted" >&2
        exit 2
    fi
    C_PRESENT="$(summary_get "$SCORE_SUMMARY" control.present)"
    C_USABLE="$(summary_get "$SCORE_SUMMARY" control.usable)"
    T_PRESENT="$(summary_get "$SCORE_SUMMARY" treatment.present)"
    T_USABLE="$(summary_get "$SCORE_SUMMARY" treatment.usable)"
    PER_REP_JSON="$(python3 -c '
import json, sys
print(",".join(json.dumps(item,separators=(",",":")) for item in json.loads(sys.argv[1])["per_rep"]))
' "$SCORE_SUMMARY")"
fi

if [[ $REPLAY -eq 0 && "$LIVE_ALL_DISPATCH_OK" -eq 1 ]]; then
    if ! CURRENT_CONTRACT="$(python3 "$FIXTURE_META_TOOL" probe-contract \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS_DIR" \
        --probe "$PROBE")" || [[ "$CURRENT_CONTRACT" != "$PROBE_CONTRACT" ]]; then
        echo "error: live capture inputs changed during dispatch; fixture set not published" >&2
        exit 2
    fi
    # The stage inventory is exact today (probe-fixture-metadata.py create and
    # verify reject any file beyond the transcripts and capture-contract.json),
    # so the seal record leaves the stage before binding and survives in the
    # live workspace and the scorecard. Lane M-C: once the metadata tool binds
    # seal.json into the capture contract, delete this relocation.
    if [[ -f "$LIVE_STAGE/seal.json" ]]; then
        mv "$LIVE_STAGE/seal.json" "$LIVE_WORKSPACE/seal.json"
    fi
    CREATE_ARGS=(
        create
        --fixture-dir "$LIVE_STAGE"
        --probe-dir "$PROBE_DIR"
        --skills-dir "$SKILLS_DIR"
        --harness "$HARNESS_PATH"
        --preamble "$PREAMBLE_PATH"
        --dispatch-helper "$DISPATCH_HELPER_PATH"
        --probe "$PROBE"
        --reps "$REPS"
    )
    if [[ -n "$MODEL" ]]; then CREATE_ARGS+=(--requested-model "$MODEL"); fi
    if [[ -n "$EFFORT" ]]; then CREATE_ARGS+=(--requested-effort "$EFFORT"); fi
    if ! FIXTURE_METADATA="$(python3 "$FIXTURE_META_TOOL" "${CREATE_ARGS[@]}")"; then
        echo "error: live capture refused: structured transcripts or bound producer identity failed verification" >&2
        exit 2
    fi
    FIXTURE_BINDING="$(summary_get "$FIXTURE_METADATA" binding_sha256)"
    FIXTURE_SCHEMA="$(summary_get "$FIXTURE_METADATA" schema)"
    CAPTURE_EVALUATOR="$(python3 -c 'import json,sys; print(json.dumps(json.loads(sys.argv[1])["capture_evaluator"],sort_keys=True,separators=(",",":")))' "$FIXTURE_METADATA")"
    if [[ "$CAPTURE_EVALUATOR" != "$CURRENT_EVALUATOR" ]]; then
        echo "error: capture evaluator changed during live dispatch; fixture set not published" >&2
        exit 2
    fi
    TREATMENT_SOURCE="$(summary_get "$FIXTURE_METADATA" treatment_source)"
    PRODUCER_MODEL="$(summary_get "$FIXTURE_METADATA" producer.model)"
    PRODUCER_EFFORT="$(summary_get "$FIXTURE_METADATA" producer.effort)"
    PRODUCER_JSON="$(summary_json "$FIXTURE_METADATA" producer)"
    for ((n=1; n<=REPS; n++)); do
        for arm in control treatment; do
            key="$arm-$n"
            bound_digest="$(python3 -c '
import json, sys
records=json.loads(sys.argv[1])["transcripts"]
print(next(item["sha256"] for item in records if item["path"] == sys.argv[2] + ".txt"))
' "$FIXTURE_METADATA" "$key")"
            if [[ "$bound_digest" != "${LIVE_DIGESTS[$key]:-}" ]]; then
                echo "error: live transcript identity changed after scoring: $key" >&2
                exit 2
            fi
        done
    done
    if ! python3 "$FIXTURE_META_TOOL" verify \
        --fixture-dir "$LIVE_STAGE" \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS_DIR" \
        --probe "$PROBE" >/dev/null; then
        echo "error: staged fixture set failed post-capture verification" >&2
        exit 2
    fi
    if ! CURRENT_CONTRACT="$(python3 "$FIXTURE_META_TOOL" probe-contract \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS_DIR" \
        --probe "$PROBE")" || [[ "$CURRENT_CONTRACT" != "$PROBE_CONTRACT" ]]; then
        echo "error: live capture inputs changed before publish; fixture set not published" >&2
        exit 2
    fi
    if ! SCORE_SUMMARY="$(python3 "$FIXTURE_META_TOOL" score \
        --fixture-dir "$LIVE_STAGE" --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS_DIR" --probe "$PROBE")"; then
        echo "error: staged fixture set failed bound response-only scoring" >&2
        exit 2
    fi
    if ! python3 -c '
import json, sys
raise SystemExit(0 if json.loads("[" + sys.argv[1] + "]") == json.loads(sys.argv[2])["per_rep"] else 1)
' "$PER_REP_JSON" "$SCORE_SUMMARY"; then
        echo "error: bound scoring disagrees with live transcript receipts" >&2
        exit 2
    fi
    C_PRESENT="$(summary_get "$SCORE_SUMMARY" control.present)"
    C_USABLE="$(summary_get "$SCORE_SUMMARY" control.usable)"
    T_PRESENT="$(summary_get "$SCORE_SUMMARY" treatment.present)"
    T_USABLE="$(summary_get "$SCORE_SUMMARY" treatment.usable)"
    if ! publish_fixture_set "$LIVE_STAGE" "$FIXDIR"; then
        echo "error: failed to publish fixture set atomically: $FIXDIR" >&2
        exit 1
    fi
    if ! PUBLISHED_METADATA="$(python3 "$FIXTURE_META_TOOL" verify \
        --fixture-dir "$FIXDIR" \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS_DIR" \
        --probe "$PROBE")" || \
        [[ "$(summary_get "$PUBLISHED_METADATA" binding_sha256)" != "$FIXTURE_BINDING" ]]; then
        echo "error: published fixture target failed exact binding verification: $FIXDIR" >&2
        exit 1
    fi
elif [[ $REPLAY -eq 0 ]]; then
    echo "probe-skill: incomplete live run; fixture set not published" >&2
fi

# --- verdict ------------------------------------------------------------------
rate() { local n="$1" d="$2"; [[ "$d" -eq 0 ]] && { echo "null"; return; }; python3 -c "print(round($n/$d,4))"; }
C_RATE="$(rate "$C_PRESENT" "$C_USABLE")"
T_RATE="$(rate "$T_PRESENT" "$T_USABLE")"

if [[ "$C_USABLE" -eq 0 || "$T_USABLE" -eq 0 || ( $REPLAY -eq 0 && "$LIVE_ALL_DISPATCH_OK" -ne 1 ) ]]; then
    VERDICT="UNMEASURED"
else
    # Direction is part of the result; a lower treatment rate is not a null.
    cmp_res="$(python3 -c "
tu=$T_USABLE; cu=$C_USABLE
tr=$T_PRESENT/tu
cr=$C_PRESENT/cu
print('BEHAVIORAL' if tr>cr else 'REGRESSIVE' if tr<cr else 'INERT')")"
    VERDICT="$cmp_res"
fi

MODE="live"
if [[ $REPLAY -eq 1 ]]; then MODE="replay"; fi
GEN_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
if [[ -n "$FIXTURE_BINDING" ]]; then
    BOUND_SCHEDULE="$(summary_json "$FIXTURE_METADATA" schedule)"
    BOUND_SCORING="$(summary_json "$FIXTURE_METADATA" scoring)"
else
    BOUND_SCHEDULE="$(summary_json "$PROBE_CONTRACT" schedule)"
    BOUND_SCORING="$(summary_json "$PROBE_CONTRACT" scoring)"
fi

SCORECARD="$(python3 - \
    "$PROBE" "$SKILL" "$MODE" "$GEN_AT" "$REPS" \
    "$PRODUCER_MODEL" "$PRODUCER_EFFORT" "$PRODUCER_JSON" \
    "$CAPTURE_REQUESTED_MODEL" "$CAPTURE_REQUESTED_EFFORT" \
    "$FIXTURE_SET" "$FIXTURE_BINDING" "$FIXTURE_SCHEMA" "$TREATMENT_SOURCE" \
    "$BOUND_SCHEDULE" "$BOUND_SCORING" \
    "$C_PRESENT" "$C_USABLE" "$C_RATE" \
    "$T_PRESENT" "$T_USABLE" "$T_RATE" \
    "$VERDICT" "$PER_REP_JSON" \
    "$CURRENT_EVALUATOR" "$CAPTURE_EVALUATOR" "$SEAL_JSON" <<'PY'
import json
import sys

(
    probe, skill, mode, generated_at, reps,
    producer_model, producer_effort, producer_json,
    requested_model, requested_effort,
    fixture_name, fixture_binding, fixture_schema, treatment_source,
    schedule_json, scoring_json,
    control_present, control_usable, control_rate,
    treatment_present, treatment_usable, treatment_rate,
    verdict, per_rep_json,
    current_evaluator_json, capture_evaluator_json, seal_json,
) = sys.argv[1:]

current_evaluator = json.loads(current_evaluator_json)
capture_evaluator = json.loads(capture_evaluator_json) if capture_evaluator_json else None
producer = (
    json.loads(producer_json)
    if producer_json
    else {
        "adapter": "codex",
        "model": producer_model or None,
        "effort": producer_effort or None,
    }
)

def nullable_rate(value):
    return None if value == "null" else float(value)

scorecard = {
    "schema": "agentops-skill-probe.v3",
    "probe": probe,
    "skill": skill,
    "mode": mode,
    "generated_at": generated_at,
    "reps": int(reps),
    "producer": producer,
    "requested_producer": {
        "model": requested_model or None,
        "effort": requested_effort or None,
    },
    "fixture_set": {
        "name": fixture_name,
        "metadata": "fixture-set.json" if fixture_binding else None,
        "binding_sha256": fixture_binding or None,
        "schema": fixture_schema or None,
    },
    "treatment_source": treatment_source,
    "seal": json.loads(seal_json) if seal_json else None,
    "evaluator": current_evaluator,
    "capture_evaluator": capture_evaluator,
    "evaluator_matches_capture": (
        current_evaluator == capture_evaluator if capture_evaluator is not None else None
    ),
    "honesty": (
        "UNMEASURED live attempt; no immutable fixture set was published"
        if not fixture_binding
        else "measures response-shape BEHAVIOR-CHANGE under the exact bound canonical "
        "SKILL.md treatment, NOT quality-uplift; small N is directional (ADR-0011)"
        if treatment_source == "canonical-skill"
        else "measures response-shape BEHAVIOR-CHANGE under the exact hash-bound injected "
        "prelude named by bound probe metadata; canonical SKILL.md is not bound; this is "
        "NOT full-skill activation or quality-uplift; small N is directional "
        "(ADR-0011)"
        if fixture_schema == "agentops-skill-probe-fixture-set.v1"
        else "measures response-shape BEHAVIOR-CHANGE under the exact bound injected "
        "prelude associated with the bound canonical skill, NOT full-skill activation "
        "or quality-uplift; small N is directional (ADR-0011)"
    ),
    "schedule": json.loads(schedule_json),
    "scoring": json.loads(scoring_json),
    "control": {
        "present": int(control_present),
        "usable": int(control_usable),
        "rate": nullable_rate(control_rate),
    },
    "treatment": {
        "present": int(treatment_present),
        "usable": int(treatment_usable),
        "rate": nullable_rate(treatment_rate),
    },
    "verdict": verdict,
    "per_rep": json.loads("[" + per_rep_json + "]"),
}
print(json.dumps(scorecard, ensure_ascii=False, indent=2, sort_keys=False))
PY
)" || { echo "error: failed to serialize scorecard JSON" >&2; exit 1; }

if [[ -n "$OUTPUT" ]]; then
    if ! printf '%s\n' "$SCORECARD" | python3 "$FIXTURE_META_TOOL" \
        write-output --path "$OUTPUT" >/dev/null; then
        echo "error: refusing to overwrite immutable scorecard output: $OUTPUT" >&2
        exit 2
    fi
    echo "scorecard written: $OUTPUT" >&2
else
    printf '%s\n' "$SCORECARD"
fi

# In --capture we keep the fixtures (default for live). In pure --replay we never
# wrote them. Nothing else to do.
if [[ $CAPTURE -eq 1 && -n "$FIXTURE_BINDING" ]]; then
    echo "fixtures captured under: $FIXDIR" >&2
fi
exit 0
