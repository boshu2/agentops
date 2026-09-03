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
#
# ONE RUN DIRECTORY. Every live capture creates $PROBE_RUN
# ($TMPDIR/probe-run.XXXXXX, mode 0700, resolved to its realpath) holding four
# fixed children: home/ (the scratch HOME; home/.codex is CODEX_HOME), ws/ (the
# rep workspace and cwd), tmp/ (the rep's TMPDIR) and dispatch/
# (harness-private: the materialized prompt, the raw codex JSONL, stderr). One
# EXIT trap removes the whole run directory, so a run leaves no probe material
# in the temp hierarchy for a later rep or run to read.
#
# RESET, NOT RELOCATE. The paths are the same for every rep so the seatbelt
# profile is constant and the capture contract binds ONE profile digest. Before
# each rep the harness recreates ws/ and tmp/ empty and rebuilds home/.codex
# from a SANITIZED config plus a COPY of auth.json, and refuses the rep if ws/
# is not empty afterwards. The sanitized config keeps only the operator's
# top-level scalar keys: every table is dropped, so a rep starts none of the
# operator's MCP servers and inherits no [projects] trust entry. auth.json is
# copied rather than symlinked because the real home is read-denied.
#
# THE NETWORK SEAL. The outer profile is `(allow default)` and codex's own
# sandbox is bypassed inside it (seatbelt does not nest), so before this a rep
# could `curl` the canonical SKILL.md off the forge and the filesystem seal
# proved nothing about what it read. Every capture now starts a harness-owned
# CONNECT proxy (scripts/lib/probe-connect-proxy.py) on 127.0.0.1 and an
# ephemeral port; the profile denies `network*` except outbound to that port;
# the rep gets HTTPS_PROXY/HTTP_PROXY/ALL_PROXY pointing at it. The proxy
# allows CONNECT only to the bound host allowlist and refuses everything else
# with 403, logging every attempt to the harness-private dispatch dir. A refused
# CONNECT degrades that rep (`network-egress`), and the per-rep counts plus the
# log digest are bound into the transcript's probe-input event.
#
# THE REP ENVIRONMENT. The rep runs with exactly the variables the seal's
# env_allowlist names, in its own process group, with every non-stdio descriptor
# closed, under a GENERATED config (not the operator's) whose text and digest
# the seal binds. After the rep the harness reaps the process group (a survivor
# degrades the rep as `rep-survivor`) and re-reads the config: the only
# permitted growth is codex's own `[projects."<ws>"]` trust table, anything else
# degrades the rep as `config-mutated`.
#
# THE PROFILE (seatbelt, macOS sandbox-exec; last matching rule wins). It is
# RENDERED from the bound seal block by scripts/lib/probe-fixture-metadata.py,
# and coverage requires the block to rebuild it to the recorded digest, so a
# recorded root is the bytes the kernel enforced rather than a claim beside
# them:
#   * file-write* denied everywhere except run home/, ws/, tmp/ and /dev, and
#     denied again on run dispatch/;
#   * file-read* denied on the real TMPDIR, /tmp, /private/tmp, the real HOME
#     (which subsumes ~/.agents, ~/.claude/skills, ~/.gemini/skills,
#     ~/.codex/skills and every checkout under it), this checkout, the resolved
#     skills dir, the git common directory's parent (the main checkout a linked
#     worktree shares), and each skill root's resolved entry for the skill under
#     test. Seatbelt matches the traversed path, so both the literal and the
#     resolved form of every root is denied;
#   * file-read* re-allowed on run home/, ws/ and tmp/, with file-read-metadata
#     on their ancestors so the rep can cd into its workspace (a denied ancestor
#     breaks getcwd), and file-read-metadata only on run dispatch/ (node stats
#     its stdio files at startup; contents and listing stay denied);
#   * file-link and file-clone denied on run dispatch/ and every denied read
#     root, so a rep cannot launder a denied file into its readable workspace;
#   * file-read* allowed on the producer executable itself when it resolves
#     under a denied root (the codex launcher can live under the real HOME);
#     every such path is recorded in the seal as allowed_read_paths.
# The profile is handed to scripts/lib/codex-exec.sh as the CODEX_EXEC_WRAP
# command prefix `(sandbox-exec -p "<profile>")`.
#
# Fail closed: when sandbox-exec is absent the live dispatch refuses. Only an
# explicit PROBE_SEAL=none runs unsealed; that run prints
# `seal: none (coverage-ineligible)` and records seal_mode=none. The seal record
# (seal.json) is written into the capture stage before the first rep and echoed
# into the scorecard under `seal`; the capture contract binds its mechanism,
# wrap, roots, rep env, config sanitization and profile digest, and tier
# coverage requires every one of them.
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
SEAL_PROFILE=""
SEAL_PROFILE_FILE=""
SEAL_PROFILE_SHA=""
SEAL_SANDBOX_EXEC=""
SEAL_JSON=""
SEAL_DENIED_ROOTS=()
SEAL_DENIED_DATA_ROOTS=()
SEAL_DENIED_LINK_ROOTS=()
SEAL_WRITABLE_ROOTS=()
SEAL_ALLOWED_READ_PATHS=()
SEAL_LAUNCHER_CHAIN=()
SEAL_LAUNCHER_SHA=""
SEAL_CONFIG_KEPT=""
SEAL_CONFIG_SHA=""
SEAL_CONFIG_TEXT=""
SEAL_CONFIG_SOURCE=""
SEAL_AUTH_COPIED=0
SEAL_GIT_COMMON_ROOT=""
SEAL_REAL_TMPDIR=""
SEAL_CACHE_ROOT=""
PROBE_RUN=""
RUN_HOME=""
RUN_WS=""
RUN_TMP=""
RUN_DISPATCH=""
REAL_HOME="${HOME:-}"
REAL_CODEX_HOME="${CODEX_HOME:-$REAL_HOME/.codex}"
REP_HOME="$REAL_HOME"
REP_CODEX_HOME="$REAL_CODEX_HOME"
REP_TMPDIR="${TMPDIR:-/tmp}"
# The devices a sealed rep may write. `/dev` as a whole was allowed before,
# which is a writable tree inside an otherwise read-only filesystem.
SEAL_DEV_WRITE_PATHS=(/dev/null /dev/zero /dev/dtracehelper /dev/tty)
# The variables the rep is launched with, from `env -i`. Anything not named
# here does not reach the producer: the operator's shell environment carried
# proxy settings, tokens, editor hooks and PATH entries the seal never saw.
# PWD, OLDPWD, SHLVL and _ are maintained by the shell itself and cannot be
# suppressed; they are listed so the record discloses them rather than hiding
# names the rep can actually see.
SEAL_ENV_ALLOWLIST=(PATH HOME CODEX_HOME TMPDIR LANG TERM PWD OLDPWD SHLVL _
    HTTPS_PROXY HTTP_PROXY ALL_PROXY NO_PROXY
    REVIEWER REVIEWER_MARKER
    CODEX_EXEC_PROMPT_FILE CODEX_EXEC_DIR CODEX_EXEC_SANDBOX
    CODEX_EXEC_SKIP_GIT_CHECK CODEX_EXEC_TIMEOUT CODEX_EXEC_MODEL
    CODEX_EXEC_OUT_FILE CODEX_EXEC_STDERR_FILE CODEX_EXEC_EXPECT_OUTPUT
    CODEX_EXEC_BIN PROBE_SEAL_PROFILE_FILE SKILL_PROBES_DIR SKILL_PROBE_SKILLS_DIR)
# Test seams: every exported PROBE_* name reaches the rep, so a stub producer
# can be told where to look. They are expanded to concrete names at seal time
# and recorded in the seal's env_allowlist, so the record discloses exactly what
# the rep was launched with rather than a pattern.
SEAL_ENV_SEAM_PREFIX="PROBE_"
# --- network seal ------------------------------------------------------------
# The hosts codex-cli 0.145 actually reached on this operator, observed by
# running reps through the proxy in discovery mode on 2026-09-03: chatgpt.com
# (the turn itself), ab.chatgpt.com (feature flags), and the OpenAI content
# hosts under oaiusercontent.com, which a real prompt needs and a trivial one
# does not. Those carry a rotating region prefix (sdmntprsouthcentralus,
# sdmntprcentralus, sdmntprwestcentralus were all seen in one capture), so they
# are allowed as one named domain suffix rather than a list that goes stale and
# nulls a capture. api.openai.com and auth.openai.com are kept for an API-key
# producer; they were NOT observed on a ChatGPT-auth account. None of these can
# serve this repository's SKILL.md.
PROBE_NETWORK_HOSTS_DEFAULT="chatgpt.com,ab.chatgpt.com,.oaiusercontent.com,api.openai.com,auth.openai.com"
PROBE_NETWORK_HOSTS="${PROBE_NETWORK_HOSTS:-$PROBE_NETWORK_HOSTS_DEFAULT}"
# No unix socket was needed: the proxy resolves DNS, so the rep never talks to
# mDNSResponder. Kept configurable because that is a platform detail.
PROBE_NETWORK_UNIX_SOCKETS="${PROBE_NETWORK_UNIX_SOCKETS:-}"
PROXY_SCRIPT="$REPO_ROOT/scripts/lib/probe-connect-proxy.py"
PROXY_PID=""
PROXY_PORT=""
PROXY_LOG=""
PROXY_REP_FILE=""
NETWORK_HOST_LIST=()
NETWORK_SOCKET_LIST=()
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
            # The SYSTEM binary by absolute path, never a PATH lookup: a stub
            # named sandbox-exec earlier on PATH would have run instead while
            # the record still claimed /usr/bin/sandbox-exec. The override is a
            # test seam and is itself required to be executable, so the
            # fail-closed path stays reachable.
            if [[ -n "${PROBE_SEAL_SANDBOX_EXEC:-}" ]]; then
                SEAL_SANDBOX_EXEC="$PROBE_SEAL_SANDBOX_EXEC"
                if [[ ! -x "$SEAL_SANDBOX_EXEC" ]]; then
                    echo "error: PROBE_SEAL_SANDBOX_EXEC is not executable: $SEAL_SANDBOX_EXEC" >&2
                    exit 2
                fi
            elif [[ -x /usr/bin/sandbox-exec ]]; then
                SEAL_SANDBOX_EXEC=/usr/bin/sandbox-exec
            elif ! SEAL_SANDBOX_EXEC="$(command -v sandbox-exec 2>/dev/null)"; then
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
# A trailing slash is stripped: `(subpath "/x/")` matches nothing, and it also
# broke the prefix tests that decide whether a path sits under a denied root.
seal_add_root() {
    local -n roots="$1"
    local candidate resolved existing present raw
    raw="$2"
    [[ "$raw" == "/" ]] || raw="${raw%/}"
    resolved="$(seal_realpath "$raw")"
    for candidate in "$raw" "$resolved"; do
        seal_path_ok "$candidate" || { echo "error: seal cannot quote path: $candidate" >&2; exit 2; }
        present=0
        for existing in "${roots[@]}"; do
            [[ "$existing" != "$candidate" ]] || { present=1; break; }
        done
        [[ $present -eq 1 ]] || roots+=("$candidate")
    done
}

# make_run_dir — the one run directory of this capture. Created before the seal
# so every rep-facing path is fixed and the profile can bind them, and removed
# whole by the EXIT trap so no probe material survives the run.
make_run_dir() {
    local created
    created="$(mktemp -d "${TMPDIR:-/tmp}/probe-run.XXXXXX")" \
        || { echo "error: could not create the probe run directory" >&2; exit 2; }
    # Publish the path to the trap BEFORE anything that can fail: a chmod or a
    # realpath that died left the directory behind when the trap was armed after
    # them.
    PROBE_RUN="$created"
    chmod 0700 "$created"
    PROBE_RUN="$(seal_realpath "$created")"
    RUN_HOME="$PROBE_RUN/home"
    RUN_WS="$PROBE_RUN/ws"
    RUN_TMP="$PROBE_RUN/tmp"
    RUN_DISPATCH="$PROBE_RUN/dispatch"
    mkdir -p "$RUN_HOME" "$RUN_WS" "$RUN_TMP" "$RUN_DISPATCH" \
        || { echo "error: could not populate the probe run directory" >&2; exit 2; }
    chmod 0700 "$RUN_HOME" "$RUN_WS" "$RUN_TMP" "$RUN_DISPATCH"
    REP_TMPDIR="$RUN_TMP"
}

# seal_generate_config — ONE generated config for the whole capture, kept
# immutable under the dispatch dir and copied into each rep's scratch home. It
# is generated, never derived from the operator's file: even table-stripped,
# the operator's config carried `web_search` live (a second egress path), a
# `notify` hook naming an operator program, and a key set that moves under the
# harness. A generated file has one text and one digest to bind.
seal_generate_config() {
    local summary
    SEAL_CONFIG_SOURCE="$RUN_DISPATCH/config.toml"
    local args=(probe-config --target "$SEAL_CONFIG_SOURCE")
    if [[ -n "$EFFORT" ]]; then args+=(--effort "$EFFORT"); fi
    if ! summary="$(python3 "$FIXTURE_META_TOOL" "${args[@]}")"; then
        echo "error: could not generate the sealed rep config" >&2
        exit 2
    fi
    SEAL_CONFIG_KEPT="$(summary_json "$summary" keys)"
    SEAL_CONFIG_SHA="$(summary_get "$summary" sha256)"
    # The record reads the text back from the generated file itself: a shell
    # substitution would drop the trailing newline and the digest would no
    # longer match the bytes the rep was given.
    SEAL_CONFIG_TEXT="$SEAL_CONFIG_SOURCE"
}

# seal_install_rep_home — rebuild the scratch HOME/CODEX_HOME from scratch: a
# COPY of auth.json (the real home is read-denied, so a symlink cannot resolve)
# and a copy of the generated config.
seal_install_rep_home() {
    rm -rf -- "$RUN_HOME"
    mkdir -p "$RUN_HOME/.codex" \
        || { echo "error: could not create the scratch CODEX_HOME" >&2; exit 2; }
    chmod 0700 "$RUN_HOME" "$RUN_HOME/.codex"
    if [[ -f "$REAL_CODEX_HOME/auth.json" ]]; then
        cp "$REAL_CODEX_HOME/auth.json" "$RUN_HOME/.codex/auth.json" \
            || { echo "error: could not copy auth.json into the scratch CODEX_HOME" >&2; exit 2; }
        chmod 0600 "$RUN_HOME/.codex/auth.json"
        SEAL_AUTH_COPIED=1
    else
        echo "seal: no auth.json under $REAL_CODEX_HOME; a real producer cannot authenticate" >&2
    fi
    if [[ -n "$SEAL_CONFIG_SOURCE" && -f "$SEAL_CONFIG_SOURCE" ]]; then
        cp "$SEAL_CONFIG_SOURCE" "$RUN_HOME/.codex/config.toml" \
            || { echo "error: could not install the generated rep config" >&2; exit 2; }
        chmod 0600 "$RUN_HOME/.codex/config.toml"
    fi
    REP_HOME="$RUN_HOME"
    REP_CODEX_HOME="$RUN_HOME/.codex"
    # The profile file lives in the scratch home, so it is rewritten with it.
    if [[ -n "$SEAL_PROFILE" ]]; then
        printf '%s\n' "$SEAL_PROFILE" > "$SEAL_PROFILE_FILE" \
            || { echo "error: could not write the seal profile file" >&2; exit 2; }
    fi
}

# start_network_proxy — the harness-owned CONNECT proxy the rep's only egress
# runs through. Without it the outer profile is `(allow default)` for the
# network and codex's own sandbox is bypassed inside it, so a rep could fetch
# the canonical SKILL.md straight off the forge over HTTPS and the filesystem
# seal proved nothing about what it read.
start_network_proxy() {
    local host args=()
    [[ -f "$PROXY_SCRIPT" ]] || { echo "error: probe proxy missing: $PROXY_SCRIPT" >&2; exit 2; }
    PROXY_LOG="$RUN_DISPATCH/network.log"
    PROXY_REP_FILE="$RUN_DISPATCH/network.rep"
    : > "$PROXY_LOG"
    : > "$PROXY_REP_FILE"
    NETWORK_HOST_LIST=()
    # The trailing entry has no newline after it, so the read that returns it
    # also reports EOF: without the `|| [[ -n ... ]]` guard the last host in the
    # allowlist is silently dropped.
    while IFS= read -r host || [[ -n "$host" ]]; do
        [[ -n "$host" ]] || continue
        NETWORK_HOST_LIST+=("$host")
        args+=(--allow-host "$host")
    done < <(printf '%s' "$PROBE_NETWORK_HOSTS" | tr ',' '\n')
    [[ ${#NETWORK_HOST_LIST[@]} -gt 0 ]] \
        || { echo "error: the network allowlist is empty" >&2; exit 2; }
    NETWORK_SOCKET_LIST=()
    while IFS= read -r host || [[ -n "$host" ]]; do
        [[ -n "$host" ]] || continue
        NETWORK_SOCKET_LIST+=("$host")
    done < <(printf '%s' "$PROBE_NETWORK_UNIX_SOCKETS" | tr ',' '\n')
    local port_file="$RUN_DISPATCH/network.port"
    python3 "$PROXY_SCRIPT" "${args[@]}" \
        --log "$PROXY_LOG" --port-file "$port_file" --rep-file "$PROXY_REP_FILE" \
        >"$RUN_DISPATCH/network.out" 2>"$RUN_DISPATCH/network.err" &
    PROXY_PID=$!
    local waited=0
    while [[ ! -s "$port_file" ]]; do
        if ! kill -0 "$PROXY_PID" 2>/dev/null; then
            echo "error: the probe network proxy exited before it bound a port" >&2
            cat "$RUN_DISPATCH/network.err" >&2 || true
            exit 2
        fi
        waited=$((waited + 1))
        [[ $waited -lt 100 ]] || { echo "error: the probe network proxy did not bind" >&2; exit 2; }
        sleep 0.1
    done
    PROXY_PORT="$(tr -d '[:space:]' < "$port_file")"
    [[ "$PROXY_PORT" =~ ^[1-9][0-9]*$ ]] \
        || { echo "error: the probe network proxy reported no port" >&2; exit 2; }
}

stop_network_proxy() {
    [[ -n "$PROXY_PID" ]] || return 0
    kill "$PROXY_PID" 2>/dev/null || true
    wait "$PROXY_PID" 2>/dev/null || true
    PROXY_PID=""
}

# seal_add_producer_path — the producer executable is resolved by the unsealed
# harness but exec'd inside the profile, so the sealed rep must be able to read
# it, and it can sit under a denied root (a codex launcher under the real HOME).
# The exception is the ONE hole in the read denies, so it is not a free-form
# list: the whole symlink chain from `command -v codex` to the real binary is
# recorded as launcher_chain, the allowance is exactly the links a denied root
# would otherwise cover, and coverage re-derives it. Without that tie, any HOME
# path holding a SKILL.md could be listed as an allowed "launcher".
seal_add_producer_path() {
    local bin candidate next
    bin="${CODEX_EXEC_BIN:-codex}"
    if [[ "$bin" != */* ]]; then
        bin="$(command -v "$bin" 2>/dev/null || true)"
    fi
    [[ -n "$bin" && -e "$bin" ]] || return 0
    candidate="$bin"
    local guard=0 form resolved_form present existing
    while :; do
        seal_path_ok "$candidate" || { echo "error: seal cannot quote path: $candidate" >&2; exit 2; }
        # Both traversal forms: seatbelt matches the path the kernel resolved, and
        # a temp path reaches the same file as /var/... and /private/var/... .
        resolved_form="$(seal_realpath "$candidate")"
        for form in "$candidate" "$resolved_form"; do
            present=0
            for existing in "${SEAL_LAUNCHER_CHAIN[@]}"; do
                [[ "$existing" != "$form" ]] || { present=1; break; }
            done
            [[ $present -eq 1 ]] || SEAL_LAUNCHER_CHAIN+=("$form")
        done
        [[ -L "$candidate" ]] || break
        next="$(python3 -c '
import os, sys
target = os.readlink(sys.argv[1])
print(target if os.path.isabs(target) else os.path.normpath(
    os.path.join(os.path.dirname(sys.argv[1]), target)))
' "$candidate")"
        [[ -n "$next" && "$next" != "$candidate" ]] || break
        candidate="$next"
        guard=$((guard + 1))
        [[ $guard -lt 16 ]] || break
    done
    SEAL_LAUNCHER_SHA="sha256:$(python3 -c '
import hashlib, sys
print(hashlib.sha256(open(sys.argv[1], "rb").read()).hexdigest())
' "${SEAL_LAUNCHER_CHAIN[-1]}")"
    local root covered
    for candidate in "${SEAL_LAUNCHER_CHAIN[@]}"; do
        covered=0
        for root in "${SEAL_DENIED_ROOTS[@]}"; do
            if [[ "$candidate" == "$root" || "$candidate" == "$root"/* ]]; then
                covered=1
                break
            fi
        done
        [[ $covered -eq 1 ]] && SEAL_ALLOWED_READ_PATHS+=("$candidate")
    done
    return 0
}

# seal_git_common_root — the parent of the git common directory. In a linked
# worktree that is the MAIN checkout, which holds the same canonical SKILL.md
# bytes under a different path; the checkout deny alone does not cover it.
# Falls back to the checkout so the recorded root is never empty.
seal_git_common_root() {
    local common
    if common="$(git -C "$REPO_ROOT" rev-parse --path-format=absolute --git-common-dir 2>/dev/null)" \
        && [[ -n "$common" ]]; then
        seal_realpath "$(dirname "$common")"
        return 0
    fi
    seal_realpath "$REPO_ROOT"
}

# build_seal — the facts of this capture. The PROFILE is rendered from them by
# scripts/lib/probe-fixture-metadata.py, which is also what a verifier uses to
# rebuild the profile from the bound block: one renderer, so a recorded root and
# the bytes the kernel enforced cannot drift apart.
build_seal() {
    local checkout root name entry
    seal_generate_config
    start_network_proxy
    seal_install_rep_home

    checkout="$(seal_realpath "$REPO_ROOT")"
    for root in "$RUN_HOME" "$RUN_WS" "$RUN_TMP"; do
        if [[ "$root" == "$checkout" || "$root" == "$checkout"/* ]]; then
            echo "error: seal cannot place a writable root inside the checkout: $root" >&2
            exit 2
        fi
    done
    SEAL_REAL_TMPDIR="$(seal_realpath "${TMPDIR:-/tmp}")"
    SEAL_GIT_COMMON_ROOT="$(seal_git_common_root)"
    # The Darwin per-user cache directory, sibling of the per-user T dir. It is
    # not skill material, but it is an operator-writable tree the rep had full
    # read of; node and codex start with it denied (verified 2026-09-03).
    SEAL_CACHE_ROOT="$(getconf DARWIN_USER_CACHE_DIR 2>/dev/null || true)"
    if [[ -n "$SEAL_CACHE_ROOT" ]]; then
        SEAL_CACHE_ROOT="$(seal_realpath "${SEAL_CACHE_ROOT%/}")"
    fi
    # The whole per-user temp hierarchy: this run's own directory is allowed
    # back below, so what stays denied is every other run's debris.
    seal_add_root SEAL_DENIED_ROOTS "${TMPDIR:-/tmp}"
    seal_add_root SEAL_DENIED_ROOTS /private/tmp
    seal_add_root SEAL_DENIED_ROOTS /tmp
    [[ -z "$SEAL_CACHE_ROOT" ]] || seal_add_root SEAL_DENIED_ROOTS "$SEAL_CACHE_ROOT"
    # The real HOME subsumes the four skill roots, ~/.codex sessions (which
    # carry canonical text) and every checkout under it; the explicit roots stay
    # so the record names what it protects even when HOME moves. The real
    # CODEX_HOME is denied WHOLE, not only its skills dir: its sessions and
    # rollouts hold canonical text and its config names other checkouts, and it
    # can be configured outside HOME entirely.
    seal_add_root SEAL_DENIED_ROOTS "$REAL_HOME"
    seal_add_root SEAL_DENIED_ROOTS "$REAL_CODEX_HOME"
    seal_add_root SEAL_DENIED_ROOTS "$REPO_ROOT"
    seal_add_root SEAL_DENIED_ROOTS "$SKILLS_DIR"
    seal_add_root SEAL_DENIED_ROOTS "$SEAL_GIT_COMMON_ROOT"
    seal_add_root SEAL_DENIED_ROOTS "$REAL_HOME/.agents"
    seal_add_root SEAL_DENIED_ROOTS "$REAL_HOME/.claude/skills"
    seal_add_root SEAL_DENIED_ROOTS "$REAL_HOME/.gemini/skills"
    seal_add_root SEAL_DENIED_ROOTS "$REAL_HOME/.codex/skills"
    # A skill root entry is a symlink INTO some checkout, and seatbelt matches
    # the traversed path, so deny the target the entry resolves to as well.
    if [[ -n "$SKILL" ]]; then
        for name in "$REAL_HOME/.agents/skills" "$REAL_HOME/.claude/skills" \
            "$REAL_HOME/.gemini/skills" "$REAL_HOME/.codex/skills" \
            "$REAL_CODEX_HOME/skills"; do
            entry="$name/$SKILL"
            [[ -e "$entry" || -L "$entry" ]] || continue
            seal_add_root SEAL_DENIED_ROOTS "$(dirname "$(seal_realpath "$entry")")"
        done
    fi
    # The harness dispatch dir (prompt files, raw JSONL, stderr, the proxy log
    # and the generated config of every rep): metadata only, so node can stat
    # its stdio, with contents, listing and writes denied.
    seal_add_root SEAL_DENIED_DATA_ROOTS "$RUN_DISPATCH"
    seal_add_root SEAL_WRITABLE_ROOTS "$RUN_HOME"
    seal_add_root SEAL_WRITABLE_ROOTS "$RUN_WS"
    seal_add_root SEAL_WRITABLE_ROOTS "$RUN_TMP"
    # Laundering roots: a hard link or a clone turns a denied file into a
    # readable one inside the workspace, so link and clone are denied wherever
    # reads are.
    for root in "${SEAL_DENIED_ROOTS[@]}" "${SEAL_DENIED_DATA_ROOTS[@]}"; do
        seal_add_root SEAL_DENIED_LINK_ROOTS "$root"
    done
    seal_add_producer_path
    SEAL_PROFILE_FILE="$RUN_HOME/seal.sb"
}

# seal_expand_env_allowlist — add the exported test-seam names to the allowlist
# so the record lists concrete variables, never a pattern.
seal_expand_env_allowlist() {
    local name existing present
    while IFS= read -r name; do
        [[ "$name" == "$SEAL_ENV_SEAM_PREFIX"* ]] || continue
        present=0
        for existing in "${SEAL_ENV_ALLOWLIST[@]}"; do
            [[ "$existing" != "$name" ]] || { present=1; break; }
        done
        [[ $present -eq 1 ]] || SEAL_ENV_ALLOWLIST+=("$name")
    done < <(compgen -e || true)
}

# seal_payload_file PATH — the JSON the renderer turns into a profile and a
# record. Every field the contract binds comes from here, so the harness never
# writes profile text of its own.
seal_payload_file() {
    python3 - "$1" "$SEAL_MODE" "$SEAL_SANDBOX_EXEC" "$(uname -s)" \
        "$SEAL_PROFILE_FILE" \
        "$(printf '%s\n' ${SEAL_DENIED_ROOTS[@]+"${SEAL_DENIED_ROOTS[@]}"})" \
        "$(printf '%s\n' ${SEAL_DENIED_DATA_ROOTS[@]+"${SEAL_DENIED_DATA_ROOTS[@]}"})" \
        "$(printf '%s\n' ${SEAL_DENIED_LINK_ROOTS[@]+"${SEAL_DENIED_LINK_ROOTS[@]}"})" \
        "$(printf '%s\n' ${SEAL_WRITABLE_ROOTS[@]+"${SEAL_WRITABLE_ROOTS[@]}"})" \
        "$(printf '%s\n' ${SEAL_DEV_WRITE_PATHS[@]+"${SEAL_DEV_WRITE_PATHS[@]}"})" \
        "$(printf '%s\n' ${SEAL_ALLOWED_READ_PATHS[@]+"${SEAL_ALLOWED_READ_PATHS[@]}"})" \
        "$(printf '%s\n' ${SEAL_LAUNCHER_CHAIN[@]+"${SEAL_LAUNCHER_CHAIN[@]}"})" \
        "$SEAL_LAUNCHER_SHA" \
        "$(printf '%s\n' ${SEAL_ENV_ALLOWLIST[@]+"${SEAL_ENV_ALLOWLIST[@]}"})" \
        "$REP_HOME" "$REP_CODEX_HOME" "$REP_TMPDIR" \
        "$REAL_HOME" "$REAL_CODEX_HOME" "$SEAL_REAL_TMPDIR" "$SEAL_CACHE_ROOT" \
        "$SEAL_GIT_COMMON_ROOT" "$PROBE_RUN" "$RUN_WS" "$RUN_DISPATCH" \
        "$(printf '%s\n' ${NETWORK_HOST_LIST[@]+"${NETWORK_HOST_LIST[@]}"})" \
        "$(printf '%s\n' ${NETWORK_SOCKET_LIST[@]+"${NETWORK_SOCKET_LIST[@]}"})" \
        "$PROXY_PORT" "$SEAL_CONFIG_KEPT" "$SEAL_CONFIG_SHA" "$SEAL_CONFIG_TEXT" \
        "$SEAL_AUTH_COPIED" <<'PY'
import json
import sys

(
    path, mode, sandbox_exec, platform, profile_file,
    denied, denied_data, denied_link, writable, dev_write, allowed_read,
    launcher_chain, launcher_sha, env_allowlist,
    rep_home, rep_codex_home, rep_tmpdir,
    real_home, real_codex_home, real_tmpdir, cache_root, git_common_root,
    run_root, workspace_root, dispatch_root,
    hosts, sockets, proxy_port, config_keys, config_sha, config_text,
    auth_copied,
) = sys.argv[1:]
# config_text arrives as the generated file's path (empty when unsealed);
# read the exact bytes so the digest matches what the rep was given.
if config_text:
    with open(config_text, encoding="utf-8") as handle:
        config_text = handle.read()


def lines(value):
    return [line for line in value.split("\n") if line]


sealed = mode == "seatbelt"
payload = {
    "seal_mode": mode,
    "sandbox_exec": sandbox_exec,
    "platform": platform,
    "profile_file": profile_file,
    "denied_read_roots": lines(denied),
    "denied_read_data_roots": lines(denied_data),
    "denied_link_roots": lines(denied_link),
    "writable_roots": lines(writable),
    "dev_write_paths": lines(dev_write) if sealed else [],
    "allowed_read_paths": lines(allowed_read),
    "launcher_chain": lines(launcher_chain),
    "launcher_sha256": launcher_sha,
    "env_allowlist": lines(env_allowlist),
    "rep_env": {
        "HOME": rep_home,
        "CODEX_HOME": rep_codex_home,
        "TMPDIR": rep_tmpdir,
    },
    "real_home": real_home,
    "real_codex_home": real_codex_home,
    "real_tmpdir": real_tmpdir,
    "cache_root": cache_root,
    "git_common_root": git_common_root,
    "run_root": run_root,
    "workspace_root": workspace_root,
    "dispatch_root": dispatch_root,
    "network": {
        "mode": "proxy-allowlist" if sealed else "open",
        "hosts": sorted(lines(hosts)) if sealed else [],
        "proxy": f"127.0.0.1:{proxy_port}" if sealed and proxy_port else None,
        "unix_sockets": sorted(lines(sockets)) if sealed else [],
    },
    "config_sanitized": json.loads(config_keys) if config_keys else None,
    "config_sha256": config_sha,
    "config_text": config_text,
    "auth_copied": auth_copied == "1",
}
with open(path, "w", encoding="utf-8") as handle:
    json.dump(payload, handle, ensure_ascii=False, sort_keys=True)
PY
}

# reset_rep_environment — same paths every rep, emptied between reps. The
# scratch CODEX_HOME is rebuilt too: codex writes its session rollout there, and
# a rollout holds the prompt bytes of the rep that wrote it.
reset_rep_environment() {
    rm -rf -- "$RUN_WS" "$RUN_TMP" || return 1
    mkdir -p "$RUN_WS" "$RUN_TMP" || return 1
    chmod 0700 "$RUN_WS" "$RUN_TMP" || return 1
    if [[ "$SEAL_MODE" == "seatbelt" ]]; then
        seal_install_rep_home
    fi
    [[ -z "$(ls -A "$RUN_WS")" ]] || return 1
    return 0
}

# write_seal_record PATH — hand the payload to the renderer, which writes
# seal.json with the rendered profile, its digest and the wrap. The harness
# reads the profile back so the dispatch and the record cannot disagree.
write_seal_record() {
    local payload="$RUN_DISPATCH/seal-payload.json"
    seal_payload_file "$payload" \
        || { echo "error: could not assemble the seal payload" >&2; exit 2; }
    if ! SEAL_JSON="$(python3 "$FIXTURE_META_TOOL" seal-record \
        --payload "$payload" --output "$1")"; then
        echo "error: could not write seal record: $1" >&2
        exit 2
    fi
    if [[ "$SEAL_MODE" == "seatbelt" ]]; then
        SEAL_PROFILE="$(python3 -c '
import json, sys
print(json.loads(sys.argv[1])["profile"], end="")
' "$SEAL_JSON")"
        # shellcheck disable=SC2034 # reported in the seal notice below
        SEAL_PROFILE_SHA="$(summary_get "$SEAL_JSON" profile_sha256)"
        printf '%s\n' "$SEAL_PROFILE" > "$SEAL_PROFILE_FILE" \
            || { echo "error: could not write the seal profile file" >&2; exit 2; }
        # shellcheck disable=SC2034 # consumed by codex_exec_guarded in the sourced library
        CODEX_EXEC_WRAP=("$SEAL_SANDBOX_EXEC" -p "$SEAL_PROFILE")
    fi
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

# rep_environment CMD... — run CMD with EXACTLY the variables
# SEAL_ENV_ALLOWLIST names and nothing else. The operator's ambient environment
# reached the producer before this: proxy settings, tokens, editor hooks and
# PATH entries the seal never saw and the record never disclosed. The dispatch
# entry point is a shell function from the sourced library, so `env -i` cannot
# exec it; the environment is emptied in this subshell instead, which leaves the
# producer with the same set either way.
rep_environment() {
    local -A wanted=()
    local name value
    # Resolve every allowlisted value BEFORE clearing, or the ambient ones
    # (CODEX_EXEC_BIN, PATH, LANG) are gone by the time they are read back.
    for name in "${SEAL_ENV_ALLOWLIST[@]}"; do
        case "$name" in
            HOME) value="$REP_HOME";;
            CODEX_HOME) value="$REP_CODEX_HOME";;
            TMPDIR) value="$REP_TMPDIR";;
            HTTPS_PROXY|HTTP_PROXY|ALL_PROXY)
                [[ -n "$PROXY_PORT" ]] || continue
                value="http://127.0.0.1:$PROXY_PORT";;
            NO_PROXY)
                [[ -n "$PROXY_PORT" ]] || continue
                value="";;
            REVIEWER) value="codex";;
            REVIEWER_MARKER) value="turn.completed";;
            CODEX_EXEC_SANDBOX) value="read-only";;
            CODEX_EXEC_SKIP_GIT_CHECK) value="1";;
            CODEX_EXEC_TIMEOUT) value="$TIMEOUT";;
            CODEX_EXEC_MODEL) value="$MODEL";;
            CODEX_EXEC_EXPECT_OUTPUT) value="1";;
            PROBE_SEAL_PROFILE_FILE) value="$SEAL_PROFILE_FILE";;
            *)
                [[ -n "${!name+set}" ]] || continue
                value="${!name}";;
        esac
        wanted["$name"]="$value"
    done
    while IFS= read -r name; do
        [[ -n "${wanted[$name]+set}" ]] && continue
        unset -v "$name" 2>/dev/null || true
    done < <(compgen -e || true)
    # Exported FUNCTIONS travel in the environment too (as BASH_FUNC_name%%) and
    # `compgen -e` does not list them, so a caller's shell helpers reached the
    # producer. Nothing the rep runs needs an inherited function.
    local declaration
    while IFS= read -r declaration; do
        name="${declaration##* }"
        [[ -n "$name" ]] || continue
        unset -f "$name" 2>/dev/null || true
    done < <(declare -Fx 2>/dev/null || true)
    for name in "${!wanted[@]}"; do
        export "$name=${wanted[$name]}"
    done
    "$@"
}

# rep_group_members PGID — how many live processes are still in the group.
# `pgrep -g` is not usable here: on macOS an unmatched group id makes it list
# every process, which would report a survivor after every clean rep.
rep_group_members() {
    ps -A -o pgid=,pid= 2>/dev/null \
        | awk -v want="$1" '$1 == want { count += 1 } END { print count + 0 }'
}

# reap_rep_group PGID — signal the rep's process group, wait, and prove it is
# empty. A survivor would see the next rep's workspace.
reap_rep_group() {
    local pgid="$1" waited=0
    kill -TERM -"$pgid" 2>/dev/null || true
    while [[ "$(rep_group_members "$pgid")" != "0" ]]; do
        waited=$((waited + 1))
        if [[ $waited -eq 20 ]]; then
            kill -KILL -"$pgid" 2>/dev/null || true
        fi
        [[ $waited -lt 40 ]] || return 1
        sleep 0.1
    done
    return 0
}

# dispatch_live ARM REP TRANSCRIPT_OUT -> bind the exact prompt event, capture
# native Codex JSONL, and populate TRANSCRIPT_OUT with one structured envelope.
dispatch_live() {
    local arm="$1" rep="$2" transcript="$3" receipt_name="$4" result_name="$5"
    local rc=0 receipt="" outcome="DEGRADED" had_noclobber=0 workspace=""
    # Harness-private per-rep files stay in the read-denied dispatch dir; the
    # rep's cwd is the run directory's one workspace, emptied before this rep.
    local prompt_file="$RUN_DISPATCH/$arm-$rep.prompt"
    local runtime_file="$RUN_DISPATCH/$arm-$rep.codex.jsonl"
    local stderr_file="$RUN_DISPATCH/$arm-$rep.codex.stderr"
    # Same path every rep so the profile stays constant; emptied, and the
    # scratch CODEX_HOME rebuilt, so this rep starts from nothing.
    if ! reset_rep_environment; then
        echo "probe-skill: could not reset a fresh empty $arm-$rep workspace" >&2
        printf -v "$receipt_name" '%s' ""
        printf -v "$result_name" '%s' "$outcome"
        return 1
    fi
    workspace="$RUN_WS"
    if [[ -n "$PROXY_REP_FILE" ]]; then
        printf '%s\n' "$arm-$rep" > "$PROXY_REP_FILE" \
            || { echo "probe-skill: could not tag the proxy log for $arm-$rep" >&2; }
    fi
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
    # The rep runs with the sealed HOME/CODEX_HOME and its own fresh workspace
    # as cwd: a cwd inside the denied checkout makes every shell child print a
    # getcwd error on stderr, which degrades the rep. The prompt file is fed on
    # stdin by the (unsealed) harness shell; the rep never sees it on disk.
    # CODEX_EXEC_WRAP (the seatbelt prefix) reaches the library through the
    # shared shell, arrays never cross a process boundary.
    # The rep runs in its OWN process group so a forked survivor cannot outlive
    # the rep and read the next rep's workspace: `set -m` gives the background
    # job its own pgid, and the group is signalled and reaped below.
    local rep_pgid=""
    local had_monitor=0
    case "$-" in *m*) had_monitor=1;; esac
    set -m
    (
        cd "$workspace" || exit 70
        # Close every descriptor the harness owns before the producer starts:
        # FD 9 is the open transcript sink, and the dispatch handles belong to
        # the harness, not the rep.
        exec 9>&-
        local entry fd
        for entry in /dev/fd/*; do
            fd="${entry##*/}"
            case "$fd" in 0|1|2|\*) continue;; esac
            [[ "$fd" =~ ^[0-9]+$ ]] || continue
            eval "exec ${fd}>&-" 2>/dev/null || true
        done
        CODEX_EXEC_PROMPT_FILE="$prompt_file"
        CODEX_EXEC_DIR="$workspace"
        CODEX_EXEC_OUT_FILE="$runtime_file"
        CODEX_EXEC_STDERR_FILE="$stderr_file"
        export CODEX_EXEC_PROMPT_FILE CODEX_EXEC_DIR CODEX_EXEC_OUT_FILE CODEX_EXEC_STDERR_FILE
        rep_environment codex_exec_guarded >/dev/null
    ) &
    rep_pgid=$!
    [[ $had_monitor -eq 1 ]] || set +m
    wait "$rep_pgid" || rc=$?
    if ! reap_rep_group "$rep_pgid"; then
        echo "probe-skill: $arm-$rep left a running process behind (rep-survivor)" >&2
        [[ "$rc" -ne 0 ]] || rc=3
    fi
    printf '%s\n' "" > "$PROXY_REP_FILE" 2>/dev/null || true
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
    # What this rep asked the proxy for. A refused CONNECT is a rep reaching for
    # a destination the capture does not permit, so it degrades: `network-egress`.
    local egress="" egress_json=""
    if [[ "$SEAL_MODE" == "seatbelt" ]]; then
        if ! egress="$(python3 "$FIXTURE_META_TOOL" proxy-egress \
            --log "$PROXY_LOG" --rep "$arm-$rep")"; then
            echo "probe-skill: could not read the $arm-$rep proxy log" >&2
            [[ "$rc" -ne 0 ]] || rc=4
        else
            if [[ "$(summary_get "$egress" refused)" != "0" ]]; then
                echo "probe-skill: rep DEGRADED (network-egress): $arm-$rep was refused $(summary_get "$egress" refused) connection(s): $(summary_json "$egress" detail)" >&2
                [[ "$rc" -ne 0 ]] || rc=4
            fi
            egress_json="$(python3 -c '
import json, sys
record = json.loads(sys.argv[1])
print(json.dumps({key: record[key] for key in ("allowed", "refused", "log_sha256")},
                 sort_keys=True, separators=(",", ":")))
' "$egress")"
        fi
        # The generated config is immutable except for the one trust table codex
        # writes for its own cwd. Anything else means the rep edited the file it
        # was measured under: `config-mutated`.
        local drift
        if drift="$(python3 "$FIXTURE_META_TOOL" config-drift \
            --path "$REP_CODEX_HOME/config.toml" \
            --expected-file "$SEAL_CONFIG_SOURCE" \
            --workspace "$workspace")"; then
            if [[ "$(summary_json "$drift" findings)" != "[]" ]]; then
                echo "probe-skill: rep DEGRADED (config-mutated): $arm-$rep $(summary_json "$drift" findings)" >&2
                [[ "$rc" -ne 0 ]] || rc=5
            fi
        else
            echo "probe-skill: could not check the $arm-$rep config for drift" >&2
            [[ "$rc" -ne 0 ]] || rc=5
        fi
    fi
    if [[ "$rc" -eq 0 ]]; then
        local -a assemble_args=(
            assemble-transcript
            --runtime-file "$runtime_file" --prompt-file "$prompt_file"
            --fixture-dir "$LIVE_STAGE" --probe "$PROBE"
            --arm "$arm" --rep "$rep" --workspace "$workspace"
            --workspace-reset
        )
        if [[ -n "$egress_json" ]]; then
            assemble_args+=(--network-egress "$egress_json")
        fi
        python3 "$FIXTURE_META_TOOL" "${assemble_args[@]}" >&9 || rc=$?
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

# cleanup_capture — ONE guarded trap over everything a live capture creates: the
# run directory, the unpublished capture stage, and the network proxy. The stage
# is released (LIVE_STAGE emptied) only by a successful atomic publish, so a
# failure anywhere leaves no half-written fixture set behind.
cleanup_capture() {
    local status=$?
    stop_network_proxy
    if [[ -n "$PROBE_RUN" && -d "$PROBE_RUN" ]]; then
        rm -rf -- "$PROBE_RUN"
    fi
    if [[ -n "$LIVE_STAGE" && -d "$LIVE_STAGE" ]]; then
        rm -rf -- "$LIVE_STAGE"
    fi
    return "$status"
}

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
    trap cleanup_capture EXIT
    LIVE_STAGE="$(mktemp -d "$PROBE_DIR/.${FIXTURE_SET}.capture.XXXXXX")"
    chmod 0700 "$LIVE_STAGE"
    # One run directory for the whole capture: home/, ws/, tmp/ and the
    # harness-private dispatch/ (prompt copies and raw producer streams). It
    # lives OUTSIDE the checkout because node aborts at startup when its stdio
    # files sit under a file-read* denied tree (it stats them), so dispatch/ is
    # denied file-read-DATA and writes, never metadata. The EXIT trap takes the
    # whole directory, so nothing from this run outlives it.
    make_run_dir
    seal_expand_env_allowlist
    if [[ "$SEAL_MODE" == "seatbelt" ]]; then
        build_seal
    fi
    # The seal record lands in the capture stage BEFORE the snapshot binds the
    # capture contract, so the contract's seal block is the record itself.
    write_seal_record "$LIVE_STAGE/seal.json"
    if [[ "$SEAL_MODE" == "seatbelt" ]]; then
        echo "seal: seatbelt ($SEAL_SANDBOX_EXEC; ${#SEAL_DENIED_ROOTS[@]} denied read roots; network proxy-allowlist via 127.0.0.1:$PROXY_PORT for ${#NETWORK_HOST_LIST[@]} hosts)" >&2
    fi
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
    # An incomplete live dispatch is a FAILED run, not a null result: emitting an
    # UNMEASURED scorecard for it let a caller read a partial capture as an
    # honest measurement and exit zero.
    echo "error: incomplete live run; fixture set not published and no scorecard written" >&2
    exit 1
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
