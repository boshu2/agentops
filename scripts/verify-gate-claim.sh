#!/usr/bin/env bash
# verify-gate-claim.sh — mechanical enforcement of anti-pattern #7
#
# Anti-pattern #7 (skills/ship-loop/references/anti-patterns.md): claiming a
# gate fix landed without re-running the gate at canonical HEAD. This script
# makes that mechanical. Given a ref (PR number or branch name, used for
# logging only) and a claimed output line, it either:
#   - runs the gate at the current HEAD and greps the output (active mode), or
#   - greps a provided log without running anything (passive mode, --log).
#
# Active mode is the post-merge use case (run on main HEAD after a fix lands).
# Passive mode is the in-gate use case (called from pre-push-gate's tee'd log
# at the end of a run, so no recursion).
#
# Usage:
#   scripts/verify-gate-claim.sh <ref> "<claim>"
#   scripts/verify-gate-claim.sh --fast <ref> "<claim>"
#   scripts/verify-gate-claim.sh --log /tmp/gate.log <ref> "<claim>"
#   scripts/verify-gate-claim.sh --gate 'bats tests/scripts/foo.bats' <ref> "<claim>"
#
# Exit codes:
#   0 — claim matched verbatim
#   1 — claim absent (the AP#7 violation)
#   2 — usage error
#   3 — gate execution or log read failed (cannot verify either way)

set -euo pipefail

usage() {
    cat >&2 <<'USAGE'
Usage: scripts/verify-gate-claim.sh [options] <ref> "<claim>"

Mechanical enforcement of ship-loop anti-pattern #7: a PR claim about gate
output must reproduce verbatim when the gate runs at HEAD.

Required positional (after options):
  <ref>     PR number (e.g., 354) or branch name. Used for logging only;
            the gate always runs at the current HEAD.
  <claim>   Literal output line to search for (substring match, fgrep).

Options:
  --fast            Pass --fast to scripts/pre-push-gate.sh (default: full).
  --gate <cmd>      Use <cmd> instead of pre-push-gate.sh (e.g., a bats run).
  --log <file>      Passive mode: skip running anything; grep <file>.
  -h, --help        Show this help.

Exit codes:
  0 = verified (claim present)
  1 = AP#7 violation (claim absent)
  2 = usage error
  3 = gate execution / log read failed
USAGE
}

ref=""
claim=""
mode_fast=false
gate_cmd=""
log_file=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --fast)
            mode_fast=true
            shift
            ;;
        --gate)
            if [[ $# -lt 2 ]]; then usage; exit 2; fi
            gate_cmd="$2"
            shift 2
            ;;
        --log)
            if [[ $# -lt 2 ]]; then usage; exit 2; fi
            log_file="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        --)
            shift
            break
            ;;
        -*)
            echo "verify-gate-claim: unknown flag: $1" >&2
            usage
            exit 2
            ;;
        *)
            if [[ -z "$ref" ]]; then
                ref="$1"
            elif [[ -z "$claim" ]]; then
                claim="$1"
            else
                echo "verify-gate-claim: unexpected positional arg: $1" >&2
                usage
                exit 2
            fi
            shift
            ;;
    esac
done

# Allow remaining positionals after --
while [[ $# -gt 0 ]]; do
    if [[ -z "$ref" ]]; then
        ref="$1"
    elif [[ -z "$claim" ]]; then
        claim="$1"
    else
        echo "verify-gate-claim: unexpected positional arg: $1" >&2
        usage
        exit 2
    fi
    shift
done

if [[ -z "$ref" || -z "$claim" ]]; then
    echo "verify-gate-claim: missing required arguments (<ref> and <claim>)" >&2
    usage
    exit 2
fi

if [[ -n "$gate_cmd" && -n "$log_file" ]]; then
    echo "verify-gate-claim: --gate and --log are mutually exclusive" >&2
    exit 2
fi

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd -P)"
cd "$repo_root"

head_sha="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
echo "verify-gate-claim: ref=$ref head=$head_sha"
echo "verify-gate-claim: claim=$claim"

if [[ -n "$log_file" ]]; then
    if [[ ! -f "$log_file" ]]; then
        echo "verify-gate-claim: log file not found: $log_file" >&2
        exit 3
    fi
    source_descr="log:$log_file"
    # Passive mode — just grep
    if grep -qF -- "$claim" "$log_file"; then
        echo "verify-gate-claim: PASS — claim found verbatim in $source_descr"
        exit 0
    fi
    echo "verify-gate-claim: FAIL — claim NOT found in $source_descr" >&2
    echo "verify-gate-claim: expected line (substring): $claim" >&2
    exit 1
fi

# Active mode — run the gate
if [[ -z "$gate_cmd" ]]; then
    if $mode_fast; then
        gate_cmd="scripts/pre-push-gate.sh --fast"
    else
        gate_cmd="scripts/pre-push-gate.sh"
    fi
fi

source_descr="gate:$gate_cmd"
echo "verify-gate-claim: running $gate_cmd"

# The gate may legitimately exit non-zero (real failures). We still need its
# output for claim verification; capture-or-true, then make the verdict from
# the grep result alone. If the gate itself crashes (e.g., missing binary),
# we surface that as exit 3 — verification was impossible.
gate_log="$(mktemp -t verify-gate-claim.XXXXXX.log)"
trap 'rm -f "$gate_log"' EXIT

set +e
# shellcheck disable=SC2086  # gate_cmd is intentionally word-split
$gate_cmd >"$gate_log" 2>&1
gate_status=$?
set -e

# Exit 127 (command not found) or 126 (not executable) = cannot verify
if [[ $gate_status -eq 126 || $gate_status -eq 127 ]]; then
    echo "verify-gate-claim: gate command failed to execute (status=$gate_status)" >&2
    echo "verify-gate-claim: gate output follows (tail):" >&2
    tail -20 "$gate_log" >&2 || true
    exit 3
fi

if grep -qF -- "$claim" "$gate_log"; then
    echo "verify-gate-claim: PASS — claim found verbatim in $source_descr"
    echo "verify-gate-claim: gate exit status was $gate_status (informational; verdict is from claim match)"
    exit 0
fi

echo "verify-gate-claim: FAIL — claim NOT found in $source_descr" >&2
echo "verify-gate-claim: expected line (substring): $claim" >&2
echo "verify-gate-claim: gate exit status was $gate_status" >&2
echo "verify-gate-claim: gate output (last 40 lines):" >&2
tail -40 "$gate_log" >&2 || true
exit 1
