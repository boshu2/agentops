#!/usr/bin/env bash
#
# pawl-verdict.sh — write / check the machine-checkable, EVIDENCE-BOUND and
# COMMIT-BOUND pawl verdict (fresh-context default; multi-model opt-in) that
# authorizes a mutate-shared-trunk merge (docs/contracts/pawls.md).
#
# This is the executable enforcement behind the prose "merge-to-main requires
# green CI AND a real review of THIS commit" by reviewer(s) meeting the pawl's
# diversity mode. The /pre-land-refuters (or /council) panel WRITES a verdict
# here; scripts/reconcile-pr.sh READS it before merging.
#
# THREAT MODEL — this defends against a SLOPPY agent that skips the real review
# and self-stamps CONFIRMED. It does NOT defend against a hostile forger: there
# are no signatures, no peercred, no OS-level writer separation. Cryptographic
# un-forgeability is INTENTIONALLY OUT OF SCOPE (single-operator trusted loop;
# the cut cathedral). What it DOES guarantee: a review actually RAN (evidence
# files exist + non-empty), against the CURRENT commit (head_sha matches the PR
# head), by reviewer(s) meeting the pawl's diversity mode — fresh-context
# (>=1 fresh red-team, model-agnostic) by default, or multi-model (>=2 REAL
# roster-validated families) where a pawl is opted up. Honest framing: this is
# an evidence-bound, commit-bound verdict that requires real reviewer runs —
# NOT "un-forgeable / trusted provenance".
#
# Fail-closed by construction. `check` exits 0 (authorize the merge) ONLY when:
#   - the verdict file exists, parses, AND validates against the v1 schema,
#   - it is tied to THIS bead AND this PR,
#   - head_sha is present, --head is provided (empty/absent => fail-closed),
#     and head_sha equals the PR's CURRENT head,
#   - disposition == CONFIRMED (REFUTED / ESCALATE / HOLD => refuse),
#   - >= 1 refuter, ALL of them CONFIRMED, ALL families roster-validated,
#   - the per-MODE diversity floor is met:
#       fresh-context (DEFAULT) -> >= 1 refuter whose context_id !=
#         author_context_id (a genuine fresh red-team; MODEL-AGNOSTIC),
#       multi-model (OPT-IN)    -> >= 2 DISTINCT KNOWN-roster canonical families.
#   - real reviewer evidence is present + non-empty (per-refuter evidence file,
#     or the verdict's council_artifact file).
# Anything else => non-zero.
#
# DIVERSITY IS MODE-BASED. A fresh-CONTEXT reviewer (separate invocation, no
# shared accumulated context) catches the author's tunnel-vision/context-drift
# errors — that is the default and is model-agnostic. multi-MODEL (>=2 families)
# ALSO catches a model's systematic blind spots — stronger, opt-in per pawl
# (operator-tunable, like the circuit-breaker thresholds). Only the diversity
# requirement changes with mode; ALL other hardening (head_sha, evidence,
# schema-validity, terminal CI, roster validation) is mode-INDEPENDENT.
#
# Schema: schemas/pawl-verdict.v1.schema.json
#
# Usage:
#   pawl-verdict.sh check <bead-id> <pr> [--dir <verdicts-dir>] [--head <current-sha>]
#       -> exit 0 = CONFIRMED, evidence-bound, commit-current pawl verdict
#                   (diversity-mode floor met) present; merge authorized.
#       -> exit 1 = present but does NOT authorize (REFUTED/ESCALATE/HOLD,
#                   per-mode diversity floor not met, a non-CONFIRMED refuter,
#                   wrong PR/bead, STALE head_sha, missing reviewer evidence,
#                   fake family, unknown mode).
#       -> exit 2 = absent / unreadable / malformed / schema-invalid
#                   (fail-closed: treat as no verdict).
#
#   pawl-verdict.sh write <bead-id> <pr> --disposition <D> --head <sha> \
#       --author-context <id> [--mode <fresh-context|multi-model>] \
#       --refuter <family>:<CONFIRMED|REFUTED>:<context_id>[:<evidence-path>] [--refuter ...] \
#       [--dir <verdicts-dir>] [--council <path>] [--attempt N]
#       -> writes .agents/pawl-verdicts/<bead-id>.json (atomic temp+rename).
#       --author-context is REQUIRED (fresh-context is verified against it).
#       --mode defaults to fresh-context when omitted. head_sha and per-refuter
#       evidence (or --council) are what make `check` authorize: head_sha is
#       required, and a CONFIRMED merge needs real, non-empty reviewer evidence.
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
    and (.author_context_id? | is_str) and ((.author_context_id|length) > 0)
    and ((.mode? // null) | (. == null) or ((["fresh-context","multi-model"]) | index(.) != null))
    and (.refuters? | type == "array") and ((.refuters|length) >= 1)
    # no unknown top-level keys
    and ((keys - ["schema_version","bead_id","pr","head_sha","disposition","generated_at","mode","author_context_id","attempt","refuters","council_artifact"]) | length == 0)
    # each refuter: required family+verdict+context_id, correct types/enums, no
    # extras, family within the known alias set (canonical-roster check is separate)
    and (all(.refuters[];
          (.family? | is_str) and ((.family|ascii_downcase) as $f | (known_families | index($f)) != null)
          and (.verdict? | is_str) and ((["CONFIRMED","REFUTED"]) | index(.verdict) != null)
          and (.context_id? | is_str) and ((.context_id|length) > 0)
          and ((.evidence? // null) | (. == null) or is_str)
          and ((.reviewer? // null) | (. == null) or is_str)
          and ((keys - ["family","verdict","context_id","evidence","reviewer"]) | length == 0)
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
    echo "PAWL-GATE: no pawl verdict at $f — fail-closed, merge refused" >&2
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
    elif ((.refuters | length) < 1) then "BAD: <1 refuter"
    elif (any(.refuters[]; .verdict != "CONFIRMED")) then "BAD: a refuter is not CONFIRMED"
    elif ((.author_context_id // "") == "") then "BAD: missing author_context_id"
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
  # Defense-in-depth: an empty/absent --head means the caller could not resolve
  # the PR's current head, so we cannot prove the verdict is commit-current.
  # Refuse rather than skip the staleness comparison (which would let a stale
  # verdict authorize a merge). Either this layer or reconcile-pr.sh's pre-check
  # closes the hole; both run for defense-in-depth.
  if [[ -z "$cur_head" ]]; then
    echo "PAWL-GATE: no current PR head provided (--head empty/absent) — cannot prove the verdict is commit-current — fail-closed, merge refused" >&2
    return 1
  fi
  if [[ "$verdict_head" != "$cur_head" ]]; then
    echo "PAWL-GATE: STALE verdict — head_sha=$verdict_head != current PR head=$cur_head; a new commit was pushed after review — fail-closed, merge refused" >&2
    return 1
  fi

  # --- roster validation (MODE-INDEPENDENT): every family label must be a
  #     known roster member. A fake/junk label is refused regardless of mode, so
  #     neither the fresh-context floor nor the multi-model floor can be gamed
  #     with a bogus string. -----------------------------------------------------
  local fams=() raw norm
  while IFS= read -r raw; do
    [[ -z "$raw" ]] && continue
    norm="$(normalize_family "$raw")"
    if [[ -z "$norm" ]]; then
      echo "PAWL-GATE: unknown/fake reviewer family '$raw' (not in roster claude|gpt|gemini) — fail-closed, merge refused" >&2
      return 1
    fi
    fams+=("$norm")
  done < <(jq -r '.refuters[].family' "$f" 2>/dev/null)

  # --- diversity (PER MODE) ----------------------------------------------------
  # mode comes from the verdict (reflecting the pawl's configured mode); absent
  # => the cheap default 'fresh-context'. Everything else in this gate is mode-
  # independent — only this requirement changes.
  local mode author_ctx
  mode="$(jq -r '.mode // "fresh-context"' "$f" 2>/dev/null)"
  author_ctx="$(jq -r '.author_context_id // ""' "$f" 2>/dev/null)"
  case "$mode" in
    multi-model)
      # OPT-IN, strongest door: >=2 distinct CANONICAL families (catches a
      # model's systematic blind spots, not just the author's context drift).
      local distinct
      distinct="$(printf '%s\n' "${fams[@]}" | sort -u | grep -c .)"
      if [[ "${distinct:-0}" -lt 2 ]]; then
        echo "PAWL-GATE: mode=multi-model needs >=2 distinct KNOWN model families (got: ${fams[*]:-none}) — single-family is not cross-family — fail-closed" >&2
        return 1
      fi
      ;;
    fresh-context|"")
      # DEFAULT, cheap door: >=1 refuter whose context_id != author_context_id —
      # a genuine fresh red-team (separate invocation, no shared accumulated
      # context), MODEL-AGNOSTIC. Same family in a fresh context still counts;
      # a refuter that ran in the author's own context does NOT.
      local fresh_count rctx
      fresh_count=0
      while IFS= read -r rctx; do
        [[ -z "$rctx" ]] && continue
        if [[ "$rctx" != "$author_ctx" ]]; then
          fresh_count=$((fresh_count + 1))
        fi
      done < <(jq -r '.refuters[].context_id // ""' "$f" 2>/dev/null)
      if [[ "$fresh_count" -lt 1 ]]; then
        echo "PAWL-GATE: mode=fresh-context needs >=1 refuter whose context_id != author_context_id ($author_ctx) — a refuter that ran in the author's own context is not a fresh red-team — fail-closed" >&2
        return 1
      fi
      ;;
    *)
      echo "PAWL-GATE: unknown mode '$mode' (expected fresh-context|multi-model) — fail-closed, merge refused" >&2
      return 1
      ;;
  esac

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

  echo "PAWL-GATE: CONFIRMED, evidence-bound, commit-current pawl verdict (mode=$mode) for $bead (PR $pr, head ${verdict_head:0:12}) — merge authorized" >&2
  return 0
}

# ---------------------------------------------------------------------------
# write <bead> <pr> --disposition D --refuter fam:verdict [...] [opts]
# ---------------------------------------------------------------------------
do_write() {
  local bead="${1:-}" pr="${2:-}"; shift 2 || true
  local disposition="" head="" council="" attempt="1" mode="" author_ctx=""
  local refuters=()
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --dir)            VDIR="${2:-}"; shift 2 ;;
      --disposition)    disposition="${2:-}"; shift 2 ;;
      --refuter)        refuters+=("${2:-}"); shift 2 ;;
      --head)           head="${2:-}"; shift 2 ;;
      --council)        council="${2:-}"; shift 2 ;;
      --attempt)        attempt="${2:-}"; shift 2 ;;
      --mode)           mode="${2:-}"; shift 2 ;;
      --author-context) author_ctx="${2:-}"; shift 2 ;;
      *)                echo "pawl-verdict write: unknown flag $1" >&2; return 2 ;;
    esac
  done
  [[ -n "$bead" && -n "$pr" && -n "$disposition" ]] || { echo "pawl-verdict write: need <bead> <pr> --disposition" >&2; return 2; }
  [[ -n "$author_ctx" ]] || { echo "pawl-verdict write: need --author-context <id> (the authoring session id; fresh-context is verified against it)" >&2; return 2; }
  [[ ${#refuters[@]} -ge 1 ]] || { echo "pawl-verdict write: need >=1 --refuter fam:verdict:context_id[:evidence]" >&2; return 2; }

  # Build the refuters JSON array from `fam:verdict:context_id[:evidence-path]`
  # tokens. context_id (3rd field, REQUIRED) records the invocation the refuter
  # ran in — fresh-context mode requires it to differ from --author-context.
  # The optional 4th field is the reviewer's evidence path (its real run output),
  # which `check` requires to exist + be non-empty. Evidence is LAST so a path
  # containing ':' stays intact.
  local refs_json="[]"
  local tok fam rest vd ctx ev
  for tok in "${refuters[@]}"; do
    fam="${tok%%:*}"; rest="${tok#*:}"
    vd="${rest%%:*}"
    if [[ "$rest" != *:* ]]; then
      echo "pawl-verdict write: refuter '$tok' missing context_id (need fam:verdict:context_id[:evidence])" >&2; return 2
    fi
    rest="${rest#*:}"          # now context_id[:evidence]
    ctx="${rest%%:*}"
    if [[ "$rest" == *:* ]]; then ev="${rest#*:}"; else ev=""; fi
    [[ -n "$ctx" ]] || { echo "pawl-verdict write: refuter '$tok' has empty context_id" >&2; return 2; }
    if [[ -n "$ev" ]]; then
      refs_json="$(jq -c --arg f "$fam" --arg v "$vd" --arg c "$ctx" --arg e "$ev" '. + [{"family":$f,"verdict":$v,"context_id":$c,"evidence":$e}]' <<<"$refs_json")"
    else
      refs_json="$(jq -c --arg f "$fam" --arg v "$vd" --arg c "$ctx" '. + [{"family":$f,"verdict":$v,"context_id":$c}]' <<<"$refs_json")"
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
    --arg mode "$mode" \
    --arg author_ctx "$author_ctx" \
    '{
       schema_version: "pawl-verdict.v1",
       bead_id: $bead,
       pr: $pr,
       disposition: $disposition,
       generated_at: $generated_at,
       author_context_id: $author_ctx,
       attempt: $attempt,
       refuters: $refuters
     }
     + (if $mode    != "" then {mode: $mode} else {} end)
     + (if $head    != "" then {head_sha: $head} else {} end)
     + (if $council != "" then {council_artifact: $council} else {} end)' \
    > "$tmp" || { rm -f "$tmp"; die "failed to render verdict json"; }
  mv "$tmp" "$out"
  echo "pawl-verdict: wrote $out (disposition=$disposition)" >&2

  # Emit verdict→commit provenance edge (non-blocking, ag-cm8nd sensor).
  if command -v ao >/dev/null 2>&1; then
    ao provenance emit-verdict --file "$out" 2>/dev/null || true
  fi
}

case "$cmd" in
  check) do_check "$@" ;;
  write) do_write "$@" ;;
  -h|--help|"") cat >&2 <<'H'
Usage:
  pawl-verdict.sh check <bead-id> <pr> [--dir D] [--head CURRENT_SHA]
  pawl-verdict.sh write <bead-id> <pr> --disposition <CONFIRMED|REFUTED|ESCALATE|HOLD> --head SHA \
      --author-context <id> [--mode <fresh-context|multi-model>] \
      --refuter <family>:<CONFIRMED|REFUTED>:<context_id>[:<evidence-path>] [--refuter ...] \
      [--dir D] [--council PATH] [--attempt N]

Families normalize to the roster: claude(fable/anthropic) | gpt(codex/openai) | gemini(agy/google).
DIVERSITY IS MODE-BASED:
  fresh-context (DEFAULT) -> >=1 refuter whose context_id != --author-context (a
                             genuine fresh red-team; same family is fine).
  multi-model   (OPT-IN)  -> >=2 distinct roster families.
A CONFIRMED merge ALSO requires (mode-independent): schema-valid, this bead+PR,
head_sha == current PR head, all refuters CONFIRMED, every family roster-valid, and
real non-empty reviewer evidence (per-refuter evidence file, or --council artifact).
Threat model: a sloppy self-stamp, NOT a hostile forger — no signatures/peercred
(intentionally out of scope).
H
    exit 2 ;;
  *) die "unknown subcommand: $cmd" ;;
esac
