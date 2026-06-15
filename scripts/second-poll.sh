#!/usr/bin/env bash
# second-poll.sh — run the cross-family "second poll" (multi-model pawl) as ONE
# operator command, from any shell (Claude or Codex runtime). (ag-mg757)
#
# The default pawl poll is a single fresh-context refuter (model-agnostic). The
# OPT-IN "second poll" adds a different model FAMILY at the irreversible doors.
# Today that's a documented manual sequence in the pre-land-refuters skill
# (dispatch a Claude refuter + a `codex exec` refuter, then hand-write
# `pawl-verdict.sh write --mode multi-model`). This collapses the cross-family
# half into one command: it runs the gpt-family (codex) refuter for you, parses
# its verdict, captures evidence, and emits a ready-to-run multi-model
# `pawl-verdict.sh write` with the Claude-family refuter left as a clear
# placeholder for the in-session subagent verdict.
#
# It CONSUMES the stable pawl-verdict.sh write CLI — it does NOT modify the pawl
# mechanism. Cross-runtime by construction: it's a shell script.
#
# Usage:
#   second-poll.sh <bead> <pr> <head_sha> [--claim <text>] [--repo <path>]
#                  [--author-context <id>] [--write]
#
# Test injection: CODEX_BIN overrides the `codex` binary (the bats suite feeds a
# fake that emits a known VERDICT line).
#
# Exit codes:
#   0 = second poll ran; command emitted (or written with --write)
#   1 = REFUTED by the second family (surfaced; no CONFIRMED verdict emitted)
#   2 = bad usage / the second family (codex) is unavailable (a second poll
#       needs the second family — fail loud, don't silently degrade to single)
set -uo pipefail

CODEX_BIN="${CODEX_BIN:-codex}"
REPO="$PWD"
CLAIM=""
AUTHOR_CTX="second-poll-author"
DO_WRITE=0

[[ $# -lt 3 ]] && { echo "usage: second-poll.sh <bead> <pr> <head_sha> [--claim t] [--repo p] [--author-context id] [--write]" >&2; exit 2; }
BEAD="$1"; PR="$2"; HEAD="$3"; shift 3
while [[ $# -gt 0 ]]; do
    case "$1" in
        --claim) CLAIM="$2"; shift 2 ;;
        --repo) REPO="$2"; shift 2 ;;
        --author-context) AUTHOR_CTX="$2"; shift 2 ;;
        --write) DO_WRITE=1; shift ;;
        -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        *) echo "unknown arg: $1" >&2; exit 2 ;;
    esac
done

# A second poll requires the second family. If codex is unavailable, fail loud
# rather than silently degrading to a single-family poll (that would be a fake
# multi-model verdict).
if ! command -v "$CODEX_BIN" >/dev/null 2>&1; then
    echo "second-poll: the gpt-family refuter ($CODEX_BIN) is unavailable — a second poll needs the second family. Install/auth codex or run the single fresh-context poll instead." >&2
    exit 2
fi

# build_refuter_prompt assembles the attack prompt for the cross-family refuter.
build_refuter_prompt() {
    local bead="$1" head="$2" claim="$3"
    cat <<EOF
You are an INDEPENDENT cross-family refuter (Codex/GPT) at a pre-land pawl.
Bead: $bead   Commit under review: $head
Completion claim to attack: ${claim:-"(the bead's stated acceptance is met)"}
Try to REFUTE it. Read the diff/tests read-only. Default to REFUTED if uncertain.
Output EXACTLY one line "VERDICT: CONFIRMED" or "VERDICT: REFUTED", then your reasoning.
EOF
}

# parse_verdict extracts CONFIRMED / REFUTED / UNKNOWN from refuter output.
parse_verdict() {
    local out="$1"
    if grep -qiE 'VERDICT:[[:space:]]*REFUTED' <<<"$out"; then echo REFUTED
    elif grep -qiE 'VERDICT:[[:space:]]*CONFIRMED' <<<"$out"; then echo CONFIRMED
    else echo UNKNOWN; fi
}

prompt="$(build_refuter_prompt "$BEAD" "$HEAD" "$CLAIM")"
codex_out="$("$CODEX_BIN" exec --sandbox read-only -C "$REPO" "$prompt" 2>&1)"
verdict="$(parse_verdict "$codex_out")"

# Persist the gpt-family refuter evidence (real, non-empty — pawl-verdict needs it).
evidence_dir="${REPO}/.agents/pawl-verdicts"
mkdir -p "$evidence_dir" 2>/dev/null || evidence_dir="$(mktemp -d)"
evidence="${evidence_dir}/second-poll-${BEAD}-gpt.md"
{ echo "# Second-poll gpt-family refuter — $BEAD @ $HEAD"; echo; echo "$codex_out"; } > "$evidence"

gpt_ctx="second-poll-codex-${HEAD:0:7}"

if [[ "$verdict" == "REFUTED" ]]; then
    echo "second-poll: ❌ REFUTED by the gpt-family (codex) refuter — do NOT land."
    echo "  evidence: $evidence"
    echo "  Re-crank on the failure as the new acceptance, then re-poll."
    exit 1
fi

if [[ "$verdict" == "UNKNOWN" ]]; then
    echo "second-poll: ⚠ codex refuter returned no parseable VERDICT line — treat as not-yet-confirmed." >&2
    echo "  evidence: $evidence" >&2
    exit 1
fi

# CONFIRMED by the second family. Assemble the multi-model verdict command with
# the Claude-family refuter as a placeholder (the in-session subagent fills it).
cmd="scripts/pawl-verdict.sh write ${BEAD} ${PR} --disposition CONFIRMED --head ${HEAD} \\
  --author-context ${AUTHOR_CTX} --mode multi-model \\
  --refuter gpt:CONFIRMED:${gpt_ctx}:${evidence} \\
  --refuter claude:<CONFIRMED|REFUTED>:<claude-context-id>:<claude-evidence-path>"

echo "second-poll: ✅ gpt-family (codex) refuter CONFIRMED. evidence: $evidence"
echo "Multi-model second poll needs the second family too — supply the Claude-family refuter, then run:"
echo
echo "$cmd"

if [[ "$DO_WRITE" == "1" ]]; then
    echo >&2 "second-poll: --write given, but the Claude-family refuter is still a placeholder — refusing to write a half-filled multi-model verdict. Fill the claude refuter and run the printed command."
    exit 1
fi
exit 0
