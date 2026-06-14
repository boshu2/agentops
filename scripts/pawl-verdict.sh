#!/usr/bin/env bash
#
# pawl-verdict.sh — write / check the machine-checkable, EVIDENCE-BOUND and
# COMMIT-BOUND cross-family pawl verdict that authorizes a mutate-shared-trunk
# merge (docs/contracts/pawls.md).
#
# This is the executable enforcement behind the prose "merge-to-main requires
# green CI AND a real cross-family review of THIS commit". The /pre-land-refuters
# (or /council) panel WRITES a verdict here; scripts/reconcile-pr.sh READS it
# before merging.
#
# THREAT MODEL — this defends against a SLOPPY agent that skips the real review
# and self-stamps CONFIRMED. It does NOT defend against a hostile forger: there
# are no signatures, no peercred, no OS-level writer separation. Cryptographic
# un-forgeability is INTENTIONALLY OUT OF SCOPE (single-operator trusted loop;
# the cut cathedral). What it DOES guarantee: a review actually RAN (evidence
# files exist + non-empty), against the CURRENT commit (head_sha matches the PR
# head), by two REAL families (roster-validated labels). Honest framing: this is
# an evidence-bound, commit-bound cross-family verdict that requires real
# reviewer runs — NOT "un-forgeable / trusted provenance".
#
# Fail-closed by construction. `check` exits 0 (authorize the merge) ONLY when:
#   - the verdict file exists, parses, AND validates against the v1 schema,
#   - it is tied to THIS bead AND this PR,
#   - head_sha is present and (when --head given) equals the PR's CURRENT head,
#   - disposition == CONFIRMED (REFUTED / ESCALATE / HOLD => refuse),
#   - >= 2 refuters, ALL of them CONFIRMED,
#   - >= 2 DISTINCT KNOWN-roster model families across the refuters,
#   - real reviewer evidence is present + non-empty (per-refuter evidence file,
#     or the verdict's council_artifact file).
# Anything else => non-zero.
#
# Schema: schemas/pawl-verdict.v1.schema.json
#
# Usage:
#   pawl-verdict.sh check <bead-id> <pr> [--dir <verdicts-dir>] [--head <current-sha>]
#       -> exit 0 = CONFIRMED, evidence-bound, commit-current cross-family
#                   verdict present; merge authorized.
#       -> exit 1 = present but does NOT authorize (REFUTED/ESCALATE/HOLD,
#                   single-family, a non-CONFIRMED refuter, wrong PR/bead,
#                   STALE head_sha, missing reviewer evidence, fake family).
#       -> exit 2 = absent / unreadable / malformed / schema-invalid
#                   (fail-closed: treat as no verdict).
#
#   pawl-verdict.sh write <bead-id> <pr> --disposition <D> --head <sha> \
#       --refuter <family>:<CONFIRMED|REFUTED>[:<evidence-path>] [--refuter ...] \
#       [--dir <verdicts-dir>] [--council <path>] [--attempt N]
#       -> writes .agents/pawl-verdicts/<bead-id>.json (atomic temp+rename).
#       head_sha and per-refuter evidence (or --council) are what make `check`
#       authorize: head_sha is required, and a CONFIRMED merge needs real,
#       non-empty reviewer evidence.
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
SCHEMA="$REPO_ROOT/schemas/pawl-verdict.v1.schema.json"

# Canonical model-family roster. Aliases collapse to a canonical label; anything
# off-roster is rejected so "≥2 distinct families" can't be gamed with junk.
# claude <- claude|fable|anthropic ; gpt <- gpt|codex|openai ; gemini <- gemini|agy|google
normalize_family() {
  local raw="${1:-}"
  raw="$(printf '%s' "$raw" | tr '[:upper:]' '[:lower:]')"
  case "$raw" in
    claude|fable|anthropic) echo "claude" ;;
    gpt|codex|openai)       echo "gpt" ;;
    gemini|agy|google)      echo "gemini" ;;
    *)                      echo "" ;;   # unknown => empty => rejected
  esac
}

# Validate a verdict file against the v1 schema. Prefer a real JSON Schema
# validator (python jsonschema / check-jsonschema); fall back to strict jq
# type/enum/required-field checks. Returns 0 = valid, non-zero = invalid (msg on
# stderr). Fail-closed: an unavailable validator falls through to jq, never skips.
schema_validate() {
  local f="$1"
  if command -v check-jsonschema >/dev/null 2>&1; then
    check-jsonschema --schemafile "$SCHEMA" "$f" >/dev/null 2>&1 && return 0
    echo "schema-invalid (check-jsonschema)" >&2; return 1
  fi
  if command -v python3 >/dev/null 2>&1 && python3 -c 'import jsonschema' >/dev/null 2>&1 \
     && [[ -f "$SCHEMA" ]]; then
    python3 - "$SCHEMA" "$f" <<'PY' >/dev/null 2>&1 && return 0
import json, sys, jsonschema
schema = json.load(open(sys.argv[1]))
inst = json.load(open(sys.argv[2]))
jsonschema.validate(inst, schema)
PY
    echo "schema-invalid (jsonschema)" >&2; return 1
  fi
  # --- strict jq fallback: enforce required fields, types, enums, no-extras ---
  jq -e '
    def is_str: type == "string";
    def known_families: ["claude","fable","anthropic","gpt","codex","openai","gemini","agy","google"];
    # required top-level keys
    (.schema_version? == "pawl-verdict.v1")
    and (.bead_id? | is_str) and ((.bead_id|length) > 0)
    and (.pr? | type == "number")          # MUST be a JSON number, not a string
    and (.head_sha? | is_str) and ((.head_sha|length) >= 7)
    and (.disposition? | is_str)
    and ((["CONFIRMED","REFUTED","ESCALATE","HOLD"]) | index(.disposition) != null)
    and (.generated_at? | is_str) and ((.generated_at|length) > 0)
    and (.refuters? | type == "array") and ((.refuters|length) >= 2)
    # no unknown top-level keys
    and ((keys - ["schema_version","bead_id","pr","head_sha","disposition","generated_at","attempt","refuters","council_artifact"]) | length == 0)
    # each refuter: required family+verdict, correct types/enums, no extras,
    # family within the known alias set (canonical-roster check is separate)
    and (all(.refuters[];
          (.family? | is_str) and ((.family|ascii_downcase) as $f | (known_families | index($f)) != null)
          and (.verdict? | is_str) and ((["CONFIRMED","REFUTED"]) | index(.verdict) != null)
          and ((.evidence? // null) | (. == null) or is_str)
          and ((.reviewer? // null) | (. == null) or is_str)
          and ((keys - ["family","verdict","evidence","reviewer"]) | length == 0)
        ))
  ' "$f" >/dev/null 2>&1 && return 0
  echo "schema-invalid (jq fallback)" >&2; return 1
}

# ---------------------------------------------------------------------------
# check <bead> <pr> [--dir D] [--head CURRENT_SHA]
# ---------------------------------------------------------------------------
do_check() {
  local bead="${1:-}" pr="${2:-}"; shift 2 || true
  local cur_head=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --dir)  VDIR="${2:-}"; shift 2 ;;
      --head) cur_head="${2:-}"; shift 2 ;;
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
    echo "PAWL-GATE: verdict $f is unreadable/malformed JSON — fail-closed" >&2
    return 2
  fi
  # Schema validation — reject missing generated_at, wrong types (string pr),
  # extra properties, object-shaped refuters, fake-shaped JSON. Fail-closed.
  if ! schema_validate "$f"; then
    echo "PAWL-GATE: verdict $f fails schema $SCHEMA — fail-closed, merge refused" >&2
    return 2
  fi

  # Structural + content gates (post-schema). family roster is checked in shell
  # (normalize each, reject empties, count distinct canonical labels).
  local result
  result="$(jq -r --arg bead "$bead" --arg pr "$pr" '
    if (.schema_version != "pawl-verdict.v1") then "BAD: schema_version != pawl-verdict.v1"
    elif (.bead_id != $bead) then "BAD: bead mismatch (verdict.bead_id=\(.bead_id) wanted \($bead))"
    elif ((.pr|tostring) != $pr) then "BAD: pr mismatch (verdict.pr=\(.pr) wanted \($pr))"
    elif (.disposition != "CONFIRMED") then "BAD: disposition=\(.disposition) (only CONFIRMED authorizes; REFUTED/ESCALATE/HOLD refuse)"
    elif ((.refuters | length) < 2) then "BAD: <2 refuters"
    elif (any(.refuters[]; .verdict != "CONFIRMED")) then "BAD: a refuter is not CONFIRMED"
    else "OK"
    end
  ' "$f" 2>/dev/null)"
  if [[ "$result" != "OK" ]]; then
    echo "PAWL-GATE: ${result:-BAD: unreadable verdict} — fail-closed, merge refused" >&2
    return 1
  fi

  # --- commit-binding: head_sha required + (when given) equals current head ---
  local verdict_head
  verdict_head="$(jq -r '.head_sha // ""' "$f" 2>/dev/null)"
  if [[ -z "$verdict_head" || "${#verdict_head}" -lt 7 ]]; then
    echo "PAWL-GATE: verdict has no/short head_sha — fail-closed (commit-binding required)" >&2
    return 1
  fi
  if [[ -n "$cur_head" && "$verdict_head" != "$cur_head" ]]; then
    echo "PAWL-GATE: STALE verdict — head_sha=$verdict_head != current PR head=$cur_head; a new commit was pushed after review — fail-closed, merge refused" >&2
    return 1
  fi

  # --- family roster: normalize each label, reject unknown, ≥2 distinct -------
  local fams=() raw norm distinct
  while IFS= read -r raw; do
    [[ -z "$raw" ]] && continue
    norm="$(normalize_family "$raw")"
    if [[ -z "$norm" ]]; then
      echo "PAWL-GATE: unknown/fake reviewer family '$raw' (not in roster claude|gpt|gemini) — fail-closed, merge refused" >&2
      return 1
    fi
    fams+=("$norm")
  done < <(jq -r '.refuters[].family' "$f" 2>/dev/null)
  distinct="$(printf '%s\n' "${fams[@]}" | sort -u | grep -c .)"
  if [[ "${distinct:-0}" -lt 2 ]]; then
    echo "PAWL-GATE: <2 distinct KNOWN model families (got: ${fams[*]:-none}) — single-family is not cross-family — fail-closed" >&2
    return 1
  fi

  # --- evidence-binding: a real review must have run --------------------------
  # Each refuter SHOULD carry an evidence path; the verdict MAY instead reference
  # a council_artifact. We require at least one real, non-empty evidence surface:
  # every per-refuter evidence path that exists must be non-empty, AND there must
  # be at least one non-empty evidence file OR a non-empty council_artifact.
  # Paths resolve against the repo root (or absolute). Fail-closed if none exist.
  local council ev have_evidence=0
  council="$(jq -r '.council_artifact // ""' "$f" 2>/dev/null)"
  resolve_path() {
    local p="$1"
    [[ -z "$p" ]] && { echo ""; return; }
    case "$p" in /*) echo "$p" ;; *) echo "$REPO_ROOT/$p" ;; esac
  }
  if [[ -n "$council" ]]; then
    local cpath; cpath="$(resolve_path "$council")"
    if [[ -s "$cpath" ]]; then have_evidence=1
    else echo "PAWL-GATE: council_artifact '$council' missing/empty at $cpath — fail-closed" >&2; return 1; fi
  fi
  while IFS= read -r ev; do
    [[ -z "$ev" ]] && continue
    local epath; epath="$(resolve_path "$ev")"
    if [[ -s "$epath" ]]; then
      have_evidence=1
    else
      echo "PAWL-GATE: refuter evidence '$ev' missing/empty at $epath — fail-closed (review did not actually run)" >&2
      return 1
    fi
  done < <(jq -r '.refuters[].evidence // empty' "$f" 2>/dev/null)
  if [[ "$have_evidence" -ne 1 ]]; then
    echo "PAWL-GATE: no reviewer evidence (no refuter evidence paths and no council_artifact) — a self-asserted stamp is not a review — fail-closed" >&2
    return 1
  fi

  echo "PAWL-GATE: CONFIRMED, evidence-bound, commit-current cross-family verdict for $bead (PR $pr, head ${verdict_head:0:12}) — merge authorized" >&2
  return 0
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

  # Build the refuters JSON array from `fam:verdict[:evidence-path]` tokens.
  # The optional third field is the reviewer's evidence path (its real run
  # output), which `check` requires to exist + be non-empty.
  local refs_json="[]"
  local tok fam rest vd ev
  for tok in "${refuters[@]}"; do
    fam="${tok%%:*}"; rest="${tok#*:}"
    vd="${rest%%:*}"
    if [[ "$rest" == *:* ]]; then ev="${rest#*:}"; else ev=""; fi
    if [[ -n "$ev" ]]; then
      refs_json="$(jq -c --arg f "$fam" --arg v "$vd" --arg e "$ev" '. + [{"family":$f,"verdict":$v,"evidence":$e}]' <<<"$refs_json")"
    else
      refs_json="$(jq -c --arg f "$fam" --arg v "$vd" '. + [{"family":$f,"verdict":$v}]' <<<"$refs_json")"
    fi
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
  pawl-verdict.sh check <bead-id> <pr> [--dir D] [--head CURRENT_SHA]
  pawl-verdict.sh write <bead-id> <pr> --disposition <CONFIRMED|REFUTED|ESCALATE|HOLD> --head SHA \
      --refuter <family>:<CONFIRMED|REFUTED>[:<evidence-path>] [--refuter ...] \
      [--dir D] [--council PATH] [--attempt N]

Families normalize to the roster: claude(fable/anthropic) | gpt(codex/openai) | gemini(agy/google).
A CONFIRMED merge requires: schema-valid, this bead+PR, head_sha == current PR head,
both refuters CONFIRMED, >=2 distinct roster families, and real non-empty reviewer
evidence (per-refuter evidence file, or --council artifact). Threat model: a sloppy
self-stamp, NOT a hostile forger — no signatures/peercred (intentionally out of scope).
H
    exit 2 ;;
  *) die "unknown subcommand: $cmd" ;;
esac
