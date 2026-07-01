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
# YIELD_ROOT is where the (gitignored) yield ledger lives for the best-effort
# gate-verdict emit. It honors AGENTOPS_REPO_ROOT (the same override the sibling
# check-pawl-pre-push.sh uses) so tests can isolate the yield side-effect into a
# temp repo. REPO_ROOT stays script-relative for SCHEMA + script assets, which
# must resolve to the real checkout regardless. Production (env unset) =>
# YIELD_ROOT == REPO_ROOT, so behavior is identical. (age-obae)
YIELD_ROOT="${AGENTOPS_REPO_ROOT:-$REPO_ROOT}"

die() { echo "pawl-verdict: ERROR: $*" >&2; exit 2; }

command -v jq >/dev/null 2>&1 || die "jq not on PATH"

# _ao_bin: print the TRUSTED ao binary for the best-effort provenance/yield/membrane
# emits below. SECURITY (age-a9iv.4): on the stranger/embedded path these emits run with
# cwd = the UNTRUSTED repo under review (some explicitly `cd` into it to root the ledger),
# so a bare `ao` resolved via PATH would execute a planted `./ao` if PATH contains `.`/a
# relative entry — reintroducing the RCE this change closes. So when PAWL_UNTRUSTED_REPO=1
# use ONLY the absolute, pinned $AO_BIN (the invoking binary) and NEVER fall back to PATH;
# print nothing + return 1 when none (callers treat that as skip — emits are non-blocking).
# In-checkout (flag unset) keeps the original PATH resolution.
_ao_bin() {
  if [[ -n "${AO_BIN:-}" && -x "${AO_BIN:-}" ]]; then printf '%s\n' "$AO_BIN"; return 0; fi
  [[ "${PAWL_UNTRUSTED_REPO:-0}" == "1" ]] && return 1
  command -v ao 2>/dev/null
}

# ---------------------------------------------------------------------------
# Verified verdict-edge emission + ledger auto-bind (age-wedge-all-in-dyr0.3)
#
# SENSOR (`ao provenance emit-verdict`): fail-OPEN but LOUD. A broken emit never
# blocks the verdict that was already written (the review result stands), but it
# prints a prominent warning naming the exact corrective command plus a nonzero
# SECONDARY-STATUS line — replacing the old silent `|| true` (a silently dead
# sensor is worse than none, because it is trusted).
#
# AUTO-BIND (the ledger commit — kills the manual `chore(provenance): bind …`
# step): fail-CLOSED. It can only ever commit the single path
# docs/provenance/ledger.jsonl (`git add` of that exact path + a pathspec-
# limited `git commit -- <path>`, which leaves unrelated staged work staged and
# OUT of the bind commit), fires only when THIS emit actually appended to the
# ledger, and NEVER creates a commit in pre-push/hook context: git has already
# selected the local_sha being pushed, so mutating HEAD mid-push desyncs the
# pushed ref (see scripts/pawl-land.sh). Gated by PAWL_AUTOBIND (default ON;
# 0/false/no/off opts out). The bind commit's trailing-#trivial subject +
# provenance-only file set satisfies the check-pawl-pre-push.sh waiver
# (age-u43w), so the next push's tip is waived and the cycle terminates.
# ---------------------------------------------------------------------------

# _ledger_root_from_cwd: the directory whose docs/provenance/ledger.jsonl the
# emit will write. MIRRORS ao's resolveLedgerPath() (cli/cmd/ao/provenance_add.go):
# walk up from cwd looking for a dir with docs/ AND schemas/, else a .git entry;
# fall back to cwd. Callers that isolate the ledger do so by cwd (see
# scripts/epic-d16-donetest.sh) — that contract is preserved, so the snapshot
# and the auto-bind always target the SAME ledger the emit wrote.
_ledger_root_from_cwd() {
  local dir="$PWD" i
  for i in 1 2 3 4 5 6 7 8 9 10 11 12; do
    : "$i"
    if [[ -d "$dir/docs" && -d "$dir/schemas" ]]; then printf '%s\n' "$dir"; return 0; fi
    if [[ -e "$dir/.git" ]]; then printf '%s\n' "$dir"; return 0; fi
    [[ "$dir" == "/" ]] && break
    dir="$(dirname "$dir")"
  done
  printf '%s\n' "$PWD"
}

# _in_prepush_context: true when a git push (or any git hook) is in flight, i.e.
# creating a commit now would desync the ref git already selected for push.
# PRIMARY: the explicit PAWL_PREPUSH marker exported by the pre-push gate entry
# (scripts/check-pawl-pre-push.sh). SECONDARY (heuristic): GIT_PREFIX / GIT_DIR
# SET-ness — git exports these into hook subprocesses (probed: git 2.54 pre-push
# exports GIT_PREFIX; older gits export GIT_DIR) and nothing sets them in a
# normal interactive shell. Failure direction is safe by construction: a false
# positive merely parks the row and prints the bind one-liner.
_in_prepush_context() {
  case "$(printf '%s' "${PAWL_PREPUSH:-}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
  esac
  [[ -n "${GIT_PREFIX+set}" || -n "${GIT_DIR+set}" ]]
}

# _warn_emit_failed <why> <verdict-file-abs> [detail]: the LOUD fail-open sensor
# warning — names the exact corrective commands and prints a nonzero
# SECONDARY-STATUS marker WITHOUT changing the caller's exit semantics.
_warn_emit_failed() {
  local why="$1" vfile="$2" detail="${3:-}"
  {
    echo "pawl-verdict: =================================================================="
    echo "pawl-verdict: WARNING — provenance verdict-edge emit FAILED: $why"
    if [[ -n "$detail" ]]; then
      printf '%s\n' "$detail" | sed 's/^/pawl-verdict:   | /'
    fi
    echo "pawl-verdict: The verdict itself is UNAFFECTED (fail-open sensor), but the"
    echo "pawl-verdict: provenance ledger did NOT gain this verdict edge."
    echo "pawl-verdict: FIX: rebuild ao (cd <agentops>/cli && make build), then retry:"
    echo "pawl-verdict:   ao provenance emit-verdict --file $vfile"
    echo "pawl-verdict: =================================================================="
    echo "pawl-verdict: SECONDARY-STATUS: provenance-emit=1 (verdict outcome unchanged)"
  } >&2
}

# _autobind_ledger_edge <bead> <disposition> <ledger-root>: commit the freshly
# appended ledger edge as the established #trivial provenance-only shape.
# Fail-closed on scope (only docs/provenance/ledger.jsonl can enter the commit),
# fail-open on outcome (any failure warns loudly and returns 0 — the verdict and
# the emitted row both stand).
_autobind_ledger_edge() {
  local bead="$1" disposition="$2" root="$3"
  local rel="docs/provenance/ledger.jsonl"
  local msg="chore(provenance): bind pawl $disposition verdict for $bead #trivial"
  local bind_cmd="git -C $root add -- $rel && git -C $root commit -m \"$msg\" -- $rel"

  case "$(printf '%s' "${PAWL_AUTOBIND:-1}" | tr '[:upper:]' '[:lower:]')" in
    0|false|no|off)
      {
        echo "pawl-verdict: auto-bind OFF (PAWL_AUTOBIND=${PAWL_AUTOBIND:-0}) — ledger edge left uncommitted; bind with:"
        echo "  $bind_cmd"
      } >&2
      return 0 ;;
  esac

  # Never create commits in a repo we do not own (stranger/embedded review path).
  if [[ "${PAWL_UNTRUSTED_REPO:-0}" == "1" ]]; then
    echo "pawl-verdict: auto-bind skipped (PAWL_UNTRUSTED_REPO=1) — not committing in a reviewed stranger repo" >&2
    return 0
  fi

  # Bind ONLY when the ledger root is itself the toplevel of a git work tree —
  # never let a stray PARENT repo swallow a commit aimed at an isolated root.
  # Compare physical paths (macOS /tmp vs /private/tmp).
  local top top_phys root_phys
  top="$(git -C "$root" rev-parse --show-toplevel 2>/dev/null)" || top=""
  top_phys=""; root_phys=""
  [[ -n "$top" ]] && top_phys="$(cd "$top" 2>/dev/null && pwd -P)"
  root_phys="$(cd "$root" 2>/dev/null && pwd -P)"
  if [[ -z "$top_phys" || "$top_phys" != "$root_phys" ]]; then
    echo "pawl-verdict: auto-bind skipped ($root is not a git work-tree toplevel) — ledger edge left uncommitted" >&2
    return 0
  fi

  # Anything to commit on the ledger path? (The append-gate in the caller means
  # we normally get here only after a real append.)
  [[ -n "$(git -C "$root" status --porcelain -- "$rel" 2>/dev/null)" ]] || return 0

  # RE-ENTRANCY (critical): NEVER mutate HEAD while a push/hook is in flight —
  # git already selected the local_sha being pushed (scripts/pawl-land.sh).
  # Park the row and print the exact post-push one-liner instead.
  if _in_prepush_context; then
    {
      echo "pawl-verdict: PRE-PUSH context detected — ledger edge written but NOT committed (a commit here would desync the pushed ref)."
      echo "pawl-verdict: after the push completes, bind it with:"
      echo "  $bind_cmd"
    } >&2
    return 0
  fi

  # Path-scoped bind commit: stage ONLY the ledger path (never -A), then a
  # pathspec-limited commit — unrelated staged work stays staged and excluded.
  if ! git -C "$root" add -- "$rel" 2>/dev/null; then
    {
      echo "pawl-verdict: WARNING — auto-bind could not stage $rel; ledger edge left uncommitted. Bind manually:"
      echo "  $bind_cmd"
      echo "pawl-verdict: SECONDARY-STATUS: autobind-commit=1 (verdict outcome unchanged)"
    } >&2
    return 0
  fi
  local commit_out commit_rc=0
  commit_out="$(git -C "$root" commit -m "$msg" -- "$rel" 2>&1)" || commit_rc=$?
  if [[ "$commit_rc" -ne 0 ]]; then
    {
      echo "pawl-verdict: WARNING — auto-bind commit FAILED (rc=$commit_rc); ledger edge left uncommitted."
      printf '%s\n' "$commit_out" | sed 's/^/pawl-verdict:   | /'
      echo "pawl-verdict: bind manually: $bind_cmd"
      echo "pawl-verdict: SECONDARY-STATUS: autobind-commit=1 (verdict outcome unchanged)"
    } >&2
    return 0
  fi

  # Fail-closed post-assert: the bind commit must contain the ledger path ONLY.
  local sha files
  sha="$(git -C "$root" rev-parse HEAD 2>/dev/null)"
  files="$(git -C "$root" diff-tree --no-commit-id --name-only -r "$sha" 2>/dev/null)"
  if [[ "$files" != "$rel" ]]; then
    {
      echo "pawl-verdict: WARNING — auto-bind commit ${sha:0:12} contains unexpected paths (wanted ONLY $rel):"
      printf '%s\n' "$files" | sed 's/^/pawl-verdict:   | /'
      echo "pawl-verdict: inspect it: git -C $root show --stat ${sha:0:12}"
      echo "pawl-verdict: SECONDARY-STATUS: autobind-commit=1 (verdict outcome unchanged)"
    } >&2
    return 0
  fi
  echo "pawl-verdict: auto-bound verdict ledger edge at ${sha:0:12} ($msg)" >&2
}

# emit_verdict_edge_checked <verdict-file> <bead> <disposition>: the CHECKED
# replacement for both former best-effort `… || true` emit sites (write +
# rebind). Always returns 0 — the sensor must never change the caller's primary
# exit semantics; failures are loud (see _warn_emit_failed), never silent.
emit_verdict_edge_checked() {
  local vfile="$1" bead="$2" disposition="$3"
  local vfile_abs
  case "$vfile" in /*) vfile_abs="$vfile" ;; *) vfile_abs="$PWD/$vfile" ;; esac

  local _ao
  _ao="$(_ao_bin)" || _ao=""
  if [[ -z "$_ao" ]]; then
    _warn_emit_failed "no trusted ao binary found (PATH / AO_BIN)" "$vfile_abs"
    return 0
  fi

  # Snapshot the ledger size so the auto-bind fires ONLY when THIS emit actually
  # appended: an idempotent no-op re-emit, a failed emit, or a stubbed ao must
  # never trigger a commit — and pre-existing dirt alone never does either.
  local ledger_root ledger before="0" after="0"
  ledger_root="$(_ledger_root_from_cwd)"
  ledger="$ledger_root/docs/provenance/ledger.jsonl"
  [[ -f "$ledger" ]] && before="$(wc -c < "$ledger" | tr -d '[:space:]')"

  local emit_out emit_rc=0
  emit_out="$("$_ao" provenance emit-verdict --file "$vfile_abs" 2>&1)" || emit_rc=$?
  if [[ "$emit_rc" -ne 0 ]]; then
    _warn_emit_failed "ao provenance emit-verdict exited $emit_rc" "$vfile_abs" "$emit_out"
    return 0
  fi
  echo "pawl-verdict: provenance verdict edge emitted for $bead ($disposition)" >&2

  [[ -f "$ledger" ]] && after="$(wc -c < "$ledger" | tr -d '[:space:]')"
  [[ "$after" != "$before" ]] || return 0   # nothing appended -> nothing to bind
  _autobind_ledger_edge "$bead" "$disposition" "$ledger_root"
  return 0
}

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
    and (.disposition as $d | (["CONFIRMED","REFUTED","ESCALATE","HOLD"] | index($d)) != null)
    and (.generated_at? | is_str) and ((.generated_at|length) > 0)
    and (.author_context_id? | is_str) and ((.author_context_id|length) > 0)
    and ((.mode? // null) | (. == null) or (. as $m | (["fresh-context","multi-model"] | index($m)) != null))
    and (.refuters? | type == "array") and ((.refuters|length) >= 1)
    # no unknown top-level keys
    and ((keys - ["schema_version","bead_id","pr","head_sha","disposition","generated_at","mode","author_context_id","attempt","refuters","council_artifact"]) | length == 0)
    # each refuter: required family+verdict+context_id, correct types/enums, no
    # extras, family within the known alias set (canonical-roster check is separate)
    and (all(.refuters[];
          (.family? | is_str) and ((.family|ascii_downcase) as $f | (known_families | index($f)) != null)
          and (.verdict? | is_str) and (.verdict as $v | (["CONFIRMED","REFUTED"] | index($v)) != null)
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
  # multi-model is STRICTLY STRONGER than fresh-context, not a swap (age-les):
  # it ADDS >=2-family diversity ON TOP OF the fresh-context floor. The floor is
  # the foundational independence property (it catches the DOMINANT failure — a
  # worker rubber-stamping its own work because it shares the author's context);
  # opting a pawl UP to multi-model must never WAIVE it. So the mode case only
  # adds the multi-model family requirement, and the fresh-context floor below is
  # enforced UNCONDITIONALLY for every mode. Without this, two distinct families
  # whose refuters BOTH ran in the author's own context would authorize the merge
  # — a self-approval bypass (family-diverse but zero context-independence).
  case "$mode" in
    multi-model)
      # OPT-IN, strongest door: ADDS >=2 distinct CANONICAL families (catches a
      # model's systematic blind spots) on top of the unconditional floor below.
      local distinct
      distinct="$(printf '%s\n' "${fams[@]}" | sort -u | grep -c .)"
      if [[ "${distinct:-0}" -lt 2 ]]; then
        echo "PAWL-GATE: mode=multi-model needs >=2 distinct KNOWN model families (got: ${fams[*]:-none}) — single-family is not cross-family — fail-closed" >&2
        return 1
      fi
      ;;
    fresh-context|"")
      : # no requirement beyond the unconditional fresh-context floor below
      ;;
    *)
      echo "PAWL-GATE: unknown mode '$mode' (expected fresh-context|multi-model) — fail-closed, merge refused" >&2
      return 1
      ;;
  esac

  # Fresh-context floor — UNCONDITIONAL (every mode, incl. multi-model): >=1
  # refuter whose context_id != author_context_id — a genuine fresh red-team
  # (separate invocation, no shared accumulated context), MODEL-AGNOSTIC. Same
  # family in a fresh context still counts; a refuter that ran in the author's
  # own context does NOT. This is what closes the multi-model self-approval
  # bypass (age-les): family diversity without context independence is not a
  # real review.
  local fresh_count rctx
  fresh_count=0
  while IFS= read -r rctx; do
    [[ -z "$rctx" ]] && continue
    if [[ "$rctx" != "$author_ctx" ]]; then
      fresh_count=$((fresh_count + 1))
    fi
  done < <(jq -r '.refuters[].context_id // ""' "$f" 2>/dev/null)
  if [[ "$fresh_count" -lt 1 ]]; then
    echo "PAWL-GATE: mode=$mode needs >=1 refuter whose context_id != author_context_id ($author_ctx) — a refuter that ran in the author's own context is not a fresh red-team (the floor applies to multi-model too) — fail-closed" >&2
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

  echo "PAWL-GATE: CONFIRMED, evidence-bound, commit-current pawl verdict (mode=$mode) for $bead (PR $pr, head ${verdict_head:0:12}) — merge authorized" >&2
  return 0
}

# ---------------------------------------------------------------------------
# write <bead> <pr> --disposition D --refuter fam:verdict [...] [opts]
# ---------------------------------------------------------------------------
do_write() {
  local bead="${1:-}" pr="${2:-}"; shift 2 || true
  local disposition="" head="" council="" attempt="1" mode="" author_ctx=""
  local difficulty="1" author_family="unknown" domain="" reason=""
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
      # yield-ledger enrichment (age-uxva) — all optional; the emit is best-effort.
      # NOTE: deliberately NO --run flag. The run_id MUST resolve identically to
      # reconcile-pr.sh's accept emit (which has no --run either), or gauge A
      # can't join them. The symmetric override both honor is AGENTOPS_RUN_ID.
      --difficulty)     difficulty="${2:-}"; shift 2 ;;
      --author-family)  author_family="${2:-}"; shift 2 ;;
      --domain)         domain="${2:-}"; shift 2 ;;
      --reason)         reason="${2:-}"; shift 2 ;;
      *)                echo "pawl-verdict write: unknown flag $1" >&2; return 2 ;;
    esac
  done
  [[ -n "$bead" && -n "$pr" && -n "$disposition" ]] || { echo "pawl-verdict write: need <bead> <pr> --disposition" >&2; return 2; }
  [[ -n "$author_ctx" ]] || { echo "pawl-verdict write: need --author-context <id> (the authoring session id; fresh-context is verified against it)" >&2; return 2; }
  [[ ${#refuters[@]} -ge 1 ]] || { echo "pawl-verdict write: need >=1 --refuter fam:verdict:context_id[:evidence]" >&2; return 2; }
  # EM.2.1: a REFUTED verdict is a candidate ESCAPE (a membrane catch). The
  # membrane EXPLAINS escapes by reason and ROUTES them by domain, so require a
  # non-empty --reason here (what was missed) on any REFUTED — the conscious
  # `write` is where that belongs. The domain is GUARANTEED downstream by the Go
  # writer's UNCLASSIFIED sentinel (never lost), so it is not hard-required here:
  # the standing-pawl auto-logger cannot always resolve a domain and is fail-open,
  # and an exit here would silently DROP the very catch we most want logged.
  if [[ "$disposition" == "REFUTED" ]]; then
    [[ -n "$reason" ]] || { echo "pawl-verdict write: a REFUTED verdict is a candidate escape — --reason is REQUIRED (what was missed). Classify it." >&2; return 2; }
  fi

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

  # Emit verdict→commit provenance edge (ag-cm8nd sensor) — CHECKED, and on a
  # real append auto-bind the ledger edge (age-wedge-all-in-dyr0.3). Fail-open
  # for the verdict (never blocks), loud on failure, never commits mid-push.
  emit_verdict_edge_checked "$out" "$bead" "$disposition"

  # Emit a yield-ledger gate-verdict (age-uxva): the membrane event log of every
  # PANEL/refuter catch, not just merge-path ones. Best-effort, non-blocking,
  # fail-open — never affects the verdict write. head_sha is the REVIEWED head
  # (rebind, which restamps to the landed head, is a head-update not a review, so
  # it does NOT re-emit). The deterministic tier emits separately.
  # Run-id resolution MUST be byte-identical to reconcile-pr.sh's accept emit, or
  # gauge A cannot join the accept to this CONFIRMED (same-run join) and a real
  # merge counts as unadmitted (cross-family REFUTE). Both: AGENTOPS_RUN_ID >
  # AO_YIELD_RUN_ID > $bead. No per-call override here — that asymmetry would
  # reintroduce the mismatch.
  emit_yield_gate_verdict "$bead" "$disposition" "$head" "$attempt" "$mode" \
    "$author_ctx" "$out" "${AGENTOPS_RUN_ID:-${AO_YIELD_RUN_ID:-$bead}}" "$difficulty" \
    "$author_family" "$domain" "$reason"
}

# emit_yield_gate_verdict appends one gate-verdict event to the yield ledger for
# a panel verdict. Derives refuter_families/cross_family from NORMALIZED families
# (so codex+gpt don't read as cross-family), and is idempotent on
# (bead, head_sha, attempt, disposition, reason) via a best-effort ledger scan.
emit_yield_gate_verdict() {
  local bead="$1" disposition="$2" head="$3" attempt="$4" mode="$5" \
    author_ctx="$6" vfile="$7" run_id="$8" difficulty="$9" author_family="${10}" \
    domain="${11}" reason="${12}"
  local _ao; _ao="$(_ao_bin)" || return 0
  [[ -n "$_ao" ]] || return 0
  command -v jq >/dev/null 2>&1 || return 0
  # head_sha is required (>=7) by the schema; without it, skip (fail-open).
  [[ -n "$head" && ${#head} -ge 7 ]] || return 0
  [[ -s "$vfile" ]] || return 0
  [[ -n "$mode" ]] || mode="fresh-context"

  # Normalized, unique refuter families read FROM the verdict file (reuse the
  # canonical roster so codex+gpt collapse to one family).
  local fams=() fam nf
  while IFS= read -r fam; do
    [[ -n "$fam" ]] || continue
    nf="$(normalize_family "$fam")"
    [[ -n "$nf" ]] && fams+=("$nf")
  done < <(jq -r '.refuters[]?.family // empty' "$vfile" 2>/dev/null)
  local fams_json="[]" cross="false" n_distinct=0
  if [[ ${#fams[@]} -gt 0 ]]; then
    fams_json="$(printf '%s\n' "${fams[@]}" | sort -u | jq -R . | jq -cs '.')"
    n_distinct="$(printf '%s\n' "${fams[@]}" | sort -u | grep -c .)"
  fi
  [[ "$n_distinct" -ge 2 ]] && cross="true"

  # Best-effort idempotency (the ledger is O_APPEND — dedup at emit time, not in
  # the writer). The key INCLUDES run_id: gauge A joins per-run, so a later run
  # re-emitting at the same head/attempt/disposition MUST still log (else its
  # accept is unadmitted — cross-family REFUTE). Only an exact same-RUN dup is
  # suppressed (a literal re-run). The ledger lives at REPO_ROOT; the emit below
  # runs with cwd=REPO_ROOT so ao writes the SAME ledger this scan reads.
  local ledger="$YIELD_ROOT/.agents/yield/yield-ledger.jsonl"
  if [[ -f "$ledger" ]] && jq -e --arg run "$run_id" --arg b "$bead" --arg h "$head" \
      --argjson at "${attempt:-1}" --arg d "$disposition" --arg r "$reason" '
      select(.event=="gate-verdict" and .run_id==$run and .bead_id==$b
        and (.body.head_sha==$h) and (.body.attempt==$at) and (.body.disposition==$d)
        and ((.body.reason // "")==$r))' "$ledger" >/dev/null 2>&1; then
    return 0
  fi

  local body
  body="$(jq -c \
    --arg head "$head" --argjson diff "${difficulty:-1}" --arg af "$author_family" \
    --argjson fams "$fams_json" --argjson cross "$cross" \
    --arg dom "$domain" --arg rsn "$reason" '
    {
      difficulty: $diff,
      pawl_verdict_ref: {bead_id: .bead_id, head_sha: (.head_sha // $head)},
      disposition: .disposition,
      head_sha: (.head_sha // $head),
      attempt: (.attempt // 1),
      mode: (.mode // "fresh-context"),
      author_context_id: .author_context_id,
      refuter_families: $fams,
      author_family: $af,
      cross_family: $cross,
      author_ne_reviewer: (.author_context_id as $a | ([.refuters[]?.context_id] | index($a) | not)),
      evidence_present: ([.refuters[]?.evidence // empty] | length > 0)
    }
    + (if $dom != "" then {domain: $dom} else {} end)
    + (if $rsn != "" then {reason: $rsn} else {} end)' "$vfile" 2>/dev/null)" || return 0
  [[ -n "$body" ]] || return 0
  # Run with cwd=YIELD_ROOT so ao resolves the ledger at the same root the dedup
  # scan reads (and where the verdict file lives), independent of the caller cwd.
  # $_ao is resolved ABOVE (before this cd) to an absolute trusted binary, so cd-ing
  # into a possibly-untrusted YIELD_ROOT can never make a bare `ao` hit a planted ./ao.
  ( cd "$YIELD_ROOT" && "$_ao" yield emit gate-verdict --bead "$bead" --run "$run_id" --json "$body" ) >/dev/null 2>&1 || true
}

# rebind <bead> <pr> --head NEWSHA [--dir D]
# Re-stamp an existing CONFIRMED verdict's head_sha (preserving refuters/mode/evidence/
# attempt) onto a new commit — the deterministic clean-land step. After a verdict's own
# ledger edge is committed (which moves HEAD), the verdict must follow to the new head so
# the head-bound pre-push gate stays satisfied in ONE push, with no manual re-pass of the
# refuter args (the review already happened; only the commit it points at moves). See
# scripts/pawl-land.sh and age-standing-pawl-service-ml8.4. Re-fires the verdict sensor.
do_rebind() {
  local bead="${1:-}" pr="${2:-}"; shift 2 2>/dev/null || true
  local newhead=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --head) newhead="${2:-}"; shift 2 ;;
      --dir)  VDIR="${2:-}"; shift 2 ;;
      *) echo "pawl-verdict rebind: unknown flag $1" >&2; return 2 ;;
    esac
  done
  [[ -n "$bead" && -n "$pr" ]] || { echo "pawl-verdict rebind: need <bead> <pr> --head SHA" >&2; return 2; }
  [[ -n "$newhead" && ${#newhead} -ge 7 ]] || { echo "pawl-verdict rebind: need --head SHA (>=7 chars)" >&2; return 2; }
  local out="$VDIR/$bead.json"
  [[ -f "$out" ]] || { echo "pawl-verdict rebind: no verdict at $out" >&2; return 2; }
  jq -e '.disposition == "CONFIRMED"' "$out" >/dev/null 2>&1 || { echo "pawl-verdict rebind: $out is not CONFIRMED — refusing to rebind" >&2; return 2; }
  local tmp; tmp="$(mktemp "$VDIR/.$bead.XXXXXX")"
  jq --arg h "$newhead" --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
     '.head_sha=$h | .generated_at=$ts' "$out" > "$tmp" || { rm -f "$tmp"; die "rebind: failed to render verdict json"; }
  mv "$tmp" "$out"
  echo "pawl-verdict: rebound $out -> head ${newhead:0:12}" >&2
  # Re-fire the verdict sensor for the new head — CHECKED + auto-bind, same as
  # write (rebind only ever restamps a CONFIRMED verdict; refused above else).
  emit_verdict_edge_checked "$out" "$bead" "CONFIRMED"
}

case "$cmd" in
  check) do_check "$@" ;;
  write) do_write "$@" ;;
  rebind) do_rebind "$@" ;;
  -h|--help|"") cat >&2 <<'H'
Usage:
  pawl-verdict.sh check <bead-id> <pr> [--dir D] [--head CURRENT_SHA]
  pawl-verdict.sh write <bead-id> <pr> --disposition <CONFIRMED|REFUTED|ESCALATE|HOLD> --head SHA \
      --author-context <id> [--mode <fresh-context|multi-model>] \
      --refuter <family>:<CONFIRMED|REFUTED>:<context_id>[:<evidence-path>] [--refuter ...] \
      [--dir D] [--council PATH] [--attempt N]
  pawl-verdict.sh rebind <bead-id> <pr> --head NEWSHA [--dir D]
      Re-stamp an existing CONFIRMED verdict onto a new head (refuters preserved) — the
      deterministic clean-land step after the verdict's ledger edge is committed.

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
