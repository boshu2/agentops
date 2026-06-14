#!/usr/bin/env bash
#
# pawl-verdict.sh — write / check the machine-checkable cross-family pawl verdict
# that authorizes a mutate-shared-trunk merge (docs/contracts/pawls.md).
#
# This is the executable enforcement behind the prose "merge-to-main requires
# green CI AND the cross-family pawl gate". The /pre-land-refuters (or /council)
# panel WRITES a verdict here; scripts/reconcile-pr.sh READS it before merging.
#
# Fail-closed by construction. `check` exits 0 (authorize the merge) ONLY when:
#   - the verdict file exists and parses,
#   - it is tied to THIS bead AND this PR,
#   - disposition == CONFIRMED (REFUTED / ESCALATE / HOLD => refuse),
#   - >= 2 refuters, ALL of them CONFIRMED,
#   - >= 2 DISTINCT model families across the refuters.
# Anything else (missing, wrong bead/PR, non-CONFIRMED, single-family) => non-zero.
#
# Schema: schemas/pawl-verdict.v1.schema.json
#
# Usage:
#   pawl-verdict.sh check <bead-id> <pr> [--dir <verdicts-dir>]
#       -> exit 0 = CONFIRMED cross-family verdict present; merge authorized.
#       -> exit 1 = present but does NOT authorize (REFUTED/ESCALATE/HOLD,
#                   single-family, a non-CONFIRMED refuter, wrong PR/bead).
#       -> exit 2 = absent / unreadable / malformed (fail-closed: treat as no verdict).
#
#   pawl-verdict.sh write <bead-id> <pr> --disposition <D> \
#       --refuter <family>:<CONFIRMED|REFUTED> [--refuter ...] \
#       [--dir <verdicts-dir>] [--head <sha>] [--council <path>] [--attempt N]
#       -> writes .agents/pawl-verdicts/<bead-id>.json (atomic temp+rename).
#
# Dependency: jq.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DEFAULT_DIR="$REPO_ROOT/.agents/pawl-verdicts"

die() { echo "pawl-verdict: ERROR: $*" >&2; exit 2; }

command -v jq >/dev/null 2>&1 || die "jq not on PATH"

cmd="${1:-}"; shift || true

# --- shared arg parse helpers ------------------------------------------------
VDIR="$DEFAULT_DIR"

# ---------------------------------------------------------------------------
# check <bead> <pr> [--dir D]
# ---------------------------------------------------------------------------
do_check() {
  local bead="${1:-}" pr="${2:-}"; shift 2 || true
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --dir) VDIR="${2:-}"; shift 2 ;;
      *) shift ;;
    esac
  done
  [[ -n "$bead" && -n "$pr" ]] || { echo "pawl-verdict check: need <bead> <pr>" >&2; return 2; }

  local f="$VDIR/$bead.json"
  if [[ ! -f "$f" ]]; then
    echo "PAWL-GATE: no cross-family verdict at $f — fail-closed, merge refused" >&2
    return 2
  fi
  if ! jq -e . "$f" >/dev/null 2>&1; then
    echo "PAWL-GATE: verdict $f is unreadable/malformed — fail-closed" >&2
    return 2
  fi

  # All structural + content gates evaluated in jq; prints "OK" iff authorized,
  # else "BAD: <reason>". Fail-closed on any jq error (empty => refuse).
  local result
  result="$(jq -r --arg bead "$bead" --arg pr "$pr" '
    def fam_count: ([.refuters[].family] | unique | length);
    if (.schema_version != "pawl-verdict.v1") then "BAD: schema_version != pawl-verdict.v1"
    elif (.bead_id != $bead) then "BAD: bead mismatch (verdict.bead_id=\(.bead_id) wanted \($bead))"
    elif ((.pr|tostring) != $pr) then "BAD: pr mismatch (verdict.pr=\(.pr) wanted \($pr))"
    elif (.disposition != "CONFIRMED") then "BAD: disposition=\(.disposition) (only CONFIRMED authorizes; REFUTED/ESCALATE/HOLD refuse)"
    elif ((.refuters | length) < 2) then "BAD: <2 refuters"
    elif (any(.refuters[]; .verdict != "CONFIRMED")) then "BAD: a refuter is not CONFIRMED"
    elif (fam_count < 2) then "BAD: <2 distinct model families (single-family is not cross-family)"
    else "OK"
    end
  ' "$f" 2>/dev/null)"

  if [[ "$result" == "OK" ]]; then
    echo "PAWL-GATE: CONFIRMED cross-family verdict for $bead (PR $pr) — merge authorized" >&2
    return 0
  fi
  echo "PAWL-GATE: ${result:-BAD: unreadable verdict} — fail-closed, merge refused" >&2
  return 1
}

# ---------------------------------------------------------------------------
# write <bead> <pr> --disposition D --refuter fam:verdict [...] [opts]
# ---------------------------------------------------------------------------
do_write() {
  local bead="${1:-}" pr="${2:-}"; shift 2 || true
  local disposition="" head="" council="" attempt="1"
  local refuters=()
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --dir)         VDIR="${2:-}"; shift 2 ;;
      --disposition) disposition="${2:-}"; shift 2 ;;
      --refuter)     refuters+=("${2:-}"); shift 2 ;;
      --head)        head="${2:-}"; shift 2 ;;
      --council)     council="${2:-}"; shift 2 ;;
      --attempt)     attempt="${2:-}"; shift 2 ;;
      *)             echo "pawl-verdict write: unknown flag $1" >&2; return 2 ;;
    esac
  done
  [[ -n "$bead" && -n "$pr" && -n "$disposition" ]] || { echo "pawl-verdict write: need <bead> <pr> --disposition" >&2; return 2; }
  [[ ${#refuters[@]} -ge 1 ]] || { echo "pawl-verdict write: need >=1 --refuter fam:verdict" >&2; return 2; }

  # Build the refuters JSON array from fam:verdict tokens.
  local refs_json="[]"
  local tok fam vd
  for tok in "${refuters[@]}"; do
    fam="${tok%%:*}"; vd="${tok##*:}"
    refs_json="$(jq -c --arg f "$fam" --arg v "$vd" '. + [{"family":$f,"verdict":$v}]' <<<"$refs_json")"
  done

  mkdir -p "$VDIR"
  local out="$VDIR/$bead.json" tmp
  tmp="$(mktemp "$VDIR/.$bead.XXXXXX")"
  jq -n \
    --arg bead "$bead" \
    --argjson pr "$pr" \
    --arg disposition "$disposition" \
    --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --argjson attempt "$attempt" \
    --argjson refuters "$refs_json" \
    --arg head "$head" \
    --arg council "$council" \
    '{
       schema_version: "pawl-verdict.v1",
       bead_id: $bead,
       pr: $pr,
       disposition: $disposition,
       generated_at: $generated_at,
       attempt: $attempt,
       refuters: $refuters
     }
     + (if $head    != "" then {head_sha: $head} else {} end)
     + (if $council != "" then {council_artifact: $council} else {} end)' \
    > "$tmp" || { rm -f "$tmp"; die "failed to render verdict json"; }
  mv "$tmp" "$out"
  echo "pawl-verdict: wrote $out (disposition=$disposition)" >&2
}

case "$cmd" in
  check) do_check "$@" ;;
  write) do_write "$@" ;;
  -h|--help|"") cat >&2 <<'H'
Usage:
  pawl-verdict.sh check <bead-id> <pr> [--dir D]
  pawl-verdict.sh write <bead-id> <pr> --disposition <CONFIRMED|REFUTED|ESCALATE|HOLD> \
      --refuter <family>:<CONFIRMED|REFUTED> [--refuter ...] [--dir D] [--head SHA] [--council PATH] [--attempt N]
H
    exit 2 ;;
  *) die "unknown subcommand: $cmd" ;;
esac
