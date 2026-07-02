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

# The diff-identity signature (commit_patch_id / commit_content_lines / commit_content_sig) is
# the SINGLE source of truth shared with pawl-review.sh (age-rk3r.9). Sourced script-relative
# from lib/ so the in-checkout run uses scripts/lib/ and the embedded (stranger) path uses the
# extracted bundle's scripts/lib/ — the SAME pattern pawl-review.sh uses for codex-exec.sh.
# shellcheck source=scripts/lib/diff-identity.sh
. "$SCRIPT_DIR/lib/diff-identity.sh"

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

# _ledger_foreign_added_lines <root> <rel>: print the ledger's PRE-existing
# uncommitted ADDED lines — content the pending path-scoped bind commit would
# sweep in (the commit captures the whole working-tree-vs-HEAD delta of the path,
# NOT just this run's appended edge), but which THIS run did not emit. Mirrors
# that delta via `git diff HEAD` (captures both staged and unstaged mods, exactly
# what `git add <rel> && git commit -- <rel>` will include). Handles the
# untracked-ledger edge — where git cannot diff the file at all — by treating the
# whole working-tree file as foreign. (age-7krl)
_ledger_foreign_added_lines() {
  local root="$1" rel="$2" diff
  diff="$(git -C "$root" diff HEAD --no-color -- "$rel" 2>/dev/null)"
  if [[ -n "$diff" ]]; then
    printf '%s\n' "$diff" | awk '/^\+/ && !/^\+\+\+/ { print substr($0, 2) }'
    return 0
  fi
  # No diff vs HEAD => either clean-tracked (nothing foreign) or fully untracked
  # (git cannot diff it). Distinguish: an untracked path is absent from the index.
  if [[ -f "$root/$rel" ]] && ! git -C "$root" ls-files --error-unmatch -- "$rel" >/dev/null 2>&1; then
    cat -- "$root/$rel"
  fi
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

# _warn_foreign_ledger_rows <root> <rel> <foreign-lines>: the LOUD warn-only
# sensor for the foreign-row sweep — counts and NAMES the pre-existing ledger
# rows the pending bind commit will include but this run did NOT emit (renders
# from_id -> to_id [relation] when the row parses as JSON, else the raw line).
# WARN ONLY: it never blocks, never changes what is committed, and never reorders
# rows — hash-chained ledger rows must not be rewritten. (age-7krl)
_warn_foreign_ledger_rows() {
  local root="$1" rel="$2" foreign="$3"
  local n; n="$(printf '%s\n' "$foreign" | grep -c .)"
  [[ "${n:-0}" -ge 1 ]] || return 0
  {
    echo "pawl-verdict: =================================================================="
    echo "pawl-verdict: WARNING — the auto-bind commit will SWEEP IN $n pre-existing ledger"
    echo "pawl-verdict: row(s) this pawl run did NOT emit — found uncommitted in $rel"
    echo "pawl-verdict: BEFORE this run's verdict edge (another lane's leftovers or an"
    echo "pawl-verdict: aborted run):"
    local line desc
    while IFS= read -r line; do
      [[ -n "$line" ]] || continue
      if desc="$(printf '%s' "$line" | jq -er 'select(.from_id or .to_id) | "\(.from_id // "?") -> \(.to_id // "?")  [\(.relation // "?")]"' 2>/dev/null)" && [[ -n "$desc" ]]; then
        echo "pawl-verdict:   | $desc"
      else
        echo "pawl-verdict:   | ${line:0:200}"
      fi
    done < <(printf '%s\n' "$foreign")
    echo "pawl-verdict: These foreign rows are committed AS-IS — hash-chained ledger rows"
    echo "pawl-verdict: are never reordered or rewritten (warn-first). If they are not yours,"
    echo "pawl-verdict: review the bind commit after it lands: git -C $root show HEAD"
    echo "pawl-verdict: SECONDARY-STATUS: autobind-foreign-rows=$n (verdict outcome unchanged)"
    echo "pawl-verdict: =================================================================="
  } >&2
}

# _autobind_ledger_edge <bead> <disposition> <ledger-root>: commit the freshly
# appended ledger edge as the established #trivial provenance-only shape.
# Fail-closed on scope (only docs/provenance/ledger.jsonl can enter the commit),
# fail-open on outcome (any failure warns loudly and returns 0 — the verdict and
# the emitted row both stand).
_autobind_ledger_edge() {
  local bead="$1" disposition="$2" root="$3" foreign="${4:-}"
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

  # WARN (warn-only — never blocks, never alters what is committed): if the
  # ledger already carried uncommitted rows BEFORE this run's emit, the path-
  # scoped bind commit will sweep those foreign rows in too. Name them loudly so
  # no foreign row can ride in silently; they are still committed as-is (hash-
  # chained rows must not be reordered/rewritten). (age-7krl)
  if [[ -n "$foreign" ]]; then
    _warn_foreign_ledger_rows "$root" "$rel" "$foreign"
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

# _assert_review_head_unmoved: HIJACK GUARD (age-sylz). pawl-review.sh exports
# PAWL_REVIEW_START_HEAD = the HEAD sha it snapshotted when it resolved the diff for
# review. A cross-family review runs for MINUTES, and a concurrent lane can `git reset`
# a SHARED landing worktree mid-review — which would bind THIS verdict's edge onto the
# OTHER lane's commit (a real incident 2026-07-02). The `check` stale-guard
# (verdict.head_sha vs --head) CANNOT catch it: it passes when both moved together. This
# guard compares the LIVE worktree HEAD against the review-start snapshot and refuses the
# emit/bind on any difference, naming BOTH shas + the failure mode. The compared repo is
# the ledger root the emit/auto-bind would target (_ledger_root_from_cwd), so it protects
# the exact tree the edge would be committed into. Returns 0 = snapshot absent (older
# caller — prior behavior) OR live HEAD equals the snapshot (proceed); nonzero = the
# worktree HEAD moved (refuse).
_assert_review_head_unmoved() {
  local snap="${PAWL_REVIEW_START_HEAD:-}"
  [[ -n "$snap" ]] || return 0          # older caller: no snapshot => prior behavior (bind)
  local root now
  root="$(_ledger_root_from_cwd)"
  now="$(git -C "$root" rev-parse HEAD 2>/dev/null || true)"
  [[ -n "$now" ]] || return 0           # no resolvable live HEAD => cannot prove a move; do not block
  [[ "$now" == "$snap" ]] && return 0   # unmoved => bind
  {
    echo "pawl-verdict: =================================================================="
    echo "pawl-verdict: REFUSING to emit/bind the verdict edge — worktree HEAD moved during"
    echo "pawl-verdict: review — possible concurrent-lane hijack; re-run the review."
    echo "pawl-verdict:   review-start HEAD (snapshot) : $snap"
    echo "pawl-verdict:   worktree HEAD now (bind-time): $now"
    echo "pawl-verdict: A shared landing worktree was reset/advanced by another lane while the"
    echo "pawl-verdict: cross-family review ran, so this verdict would bind onto the WRONG commit."
    echo "pawl-verdict: =================================================================="
  } >&2
  return 1
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

  # Snapshot any PRE-emit uncommitted ledger rows (another lane's leftovers or an
  # aborted run). The path-scoped bind commit sweeps the whole working-tree-vs-
  # HEAD delta of the ledger path, so these foreign rows would ride into the bind
  # commit even though this run never emitted them — capture them NOW so the
  # auto-bind can WARN (warn-only; the rows are still committed as-is). (age-7krl)
  local foreign_rows
  foreign_rows="$(_ledger_foreign_added_lines "$ledger_root" "docs/provenance/ledger.jsonl")"

  local emit_out emit_rc=0
  emit_out="$("$_ao" provenance emit-verdict --file "$vfile_abs" 2>&1)" || emit_rc=$?
  if [[ "$emit_rc" -ne 0 ]]; then
    _warn_emit_failed "ao provenance emit-verdict exited $emit_rc" "$vfile_abs" "$emit_out"
    return 0
  fi
  echo "pawl-verdict: provenance verdict edge emitted for $bead ($disposition)" >&2

  [[ -f "$ledger" ]] && after="$(wc -c < "$ledger" | tr -d '[:space:]')"
  [[ "$after" != "$before" ]] || return 0   # nothing appended -> nothing to bind
  _autobind_ledger_edge "$bead" "$disposition" "$ledger_root" "$foreign_rows"
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

# commit_patch_id / commit_content_lines / commit_content_sig now live in the SHARED
# scripts/lib/diff-identity.sh (sourced above) — the SINGLE diff-identity signature shared
# with pawl-review.sh's --converge lineage, so the two can never drift (age-rk3r.9).

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
    and (.disposition as $d | (["CONFIRMED","REFUTED","ESCALATE","HOLD","REBOUND"] | index($d)) != null)
    and (.generated_at? | is_str) and ((.generated_at|length) > 0)
    and (.author_context_id? | is_str) and ((.author_context_id|length) > 0)
    and ((.mode? // null) | (. == null) or (. as $m | (["fresh-context","multi-model"] | index($m)) != null))
    and (.refuters? | type == "array") and ((.refuters|length) >= 1)
    # no unknown top-level keys
    and ((keys - ["schema_version","bead_id","pr","head_sha","disposition","generated_at","mode","author_context_id","attempt","refuters","council_artifact","degraded","rebound_from_verdict","rebound_from_sha","patch_id_proof"]) | length == 0)
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
# EVIDENCE-QUALITY FLOOR (age-rk3r.11) — ADVISORY-FIRST.
#
# The evidence-binding gate in `check` proves a review FILE exists + is non-empty.
# It does NOT prove the review carried SUBSTANCE: a 155-byte "no blocking defects"
# stub passed `check` on 2026-07-01. This floor raises the bar — a CONFIRMED must
# carry EITHER (a) at least one file:line-shaped finding/observation, OR (b) an
# explicit reviewed-scope attestation (a files-reviewed count) — PLUS, for every
# refuter attributable to a cold reviewer adapter, that adapter's genuine-run marker
# in its OWN evidence (codex="tokens used", agy="VERDICT:", MIRRORING the .1 adapter
# contract in scripts/lib/codex-exec.sh reviewer_adapter_marker).
#
# HONESTY (load-bearing): this floor measures review SUBSTANCE — that a real, specific
# review actually ran over named code — NOT correctness. A substantive review can
# still be WRONG; the floor cannot and does not certify the verdict is right.
#
# ROLLOUT: ADVISORY-FIRST, like the repo's other advisory gates. Until FLOOR_ENFORCE_AFTER
# the floor only MEASURES + WARNS — it NEVER changes the authorize/refuse decision — so the
# false-positive rate on REAL reviews can be observed for one cycle before it fail-closes.
# On/after the flip date a floor violation HOLDs (fail-closed).
#   FLIP MECHANISM (both overridable so the flip is a one-line change, not a code edit):
#     - FLOOR_ENFORCE_AFTER (const; env override PAWL_FLOOR_ENFORCE_AFTER) — the UTC date
#       (YYYY-MM-DD) the floor starts BLOCKING. ISO-8601 dates are zero-padded, so the
#       lexical `[[ < ]]` compare below is a correct chronological order.
#     - PAWL_FLOOR_ENFORCE=1|0 — force enforce|advisory regardless of the date (operator
#       kill-switch + how tests pin behavior clock-independently).
#   AT FLIP TIME: bump/remove FLOOR_ENFORCE_AFTER AND update any stub-evidence behavior-lock
#   suites (their thin fixtures deliberately carry no substance and will begin to HOLD).
# ---------------------------------------------------------------------------
FLOOR_ENFORCE_AFTER="${PAWL_FLOOR_ENFORCE_AFTER:-2026-07-16}"

# _floor_enforcing — 0 (true) when the floor should BLOCK on a violation; 1 (false) when
# it is still advisory (measure + warn only). PAWL_FLOOR_ENFORCE overrides the date both
# ways; otherwise enforcing once today (UTC) >= FLOOR_ENFORCE_AFTER.
_floor_enforcing() {
  case "$(printf '%s' "${PAWL_FLOOR_ENFORCE:-}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on)  return 0 ;;
    0|false|no|off) return 1 ;;
  esac
  local today; today="$(date -u +%Y-%m-%d)"
  [[ "$today" < "$FLOOR_ENFORCE_AFTER" ]] && return 1
  return 0
}

# _evidence_has_substance <file> — 0 (true) when the evidence carries review SUBSTANCE:
# a file:line-shaped finding/observation (or a bare "line N" code reference), OR an
# explicit reviewed-scope attestation (a files-reviewed count). Deliberately GENEROUS:
# a false "has substance" merely authorizes as `check` does today, whereas the HARMFUL
# direction is a false HOLD on a genuine review — so any recognizable substance shape
# suffices and thin "no blocking defects" stubs (no location, no count) are what fail.
_evidence_has_substance() {
  local f="$1"
  [[ -s "$f" ]] || return 1
  # (a) file:line — path.ext:NNN (a finding/observation that cites concrete code) OR a
  #     bare "line N" / "lines N-M" reference (reviewers cite locations both ways).
  grep -qE '[[:alnum:]_./-]+\.[[:alnum:]]+:[0-9]+' "$f" 2>/dev/null && return 0
  grep -qiE '\blines?[[:space:]]+[0-9]+' "$f" 2>/dev/null && return 0
  # (b) reviewed-scope attestation — a files-reviewed count (the clean-review escape for
  #     a review that legitimately found nothing concrete to cite).
  grep -qiE '(files[[:space:]_-]*reviewed|reviewed[[:space:]_-]*(files|scope))[^0-9]{0,40}[0-9]+' "$f" 2>/dev/null && return 0
  return 1
}

# _family_genuine_marker <canonical-family> — the adapter genuine-run marker EXPECTED in a
# refuter's evidence for that family, MIRRORING the .1 reviewer-adapter contract
# (scripts/lib/codex-exec.sh reviewer_adapter_marker): gpt→codex adapter ("tokens used"),
# gemini→agy adapter ("VERDICT:"). Empty for a family with no cold-adapter marker defined
# yet (claude = the warm reviewer) → advisory skip, never a hard fail (duel amendment 1).
# Drift-locked against codex-exec.sh by tests/scripts/pawl-verdict-evidence-floor.bats.
_family_genuine_marker() {
  case "$1" in
    gpt)    printf 'tokens used' ;;   # codex adapter
    gemini) printf 'VERDICT:' ;;      # agy (cold) adapter
    *)      printf '' ;;              # claude / unknown: no cold-adapter marker (advisory)
  esac
}

# _resolve_ev_path <path> — resolve a RELATIVE evidence path against YIELD_ROOT (the repo
# being gated: AGENTOPS_REPO_ROOT on the embedded/stranger path = the USER's repo; ==
# REPO_ROOT in-checkout). Absolute paths stay. Using YIELD_ROOT (not the script-relative
# REPO_ROOT) is what makes evidence written repo-relative resolvable on the embedded path —
# where REPO_ROOT is the extracted-bundle temp dir, not the user's repo. (age-rk3r.9)
_resolve_ev_path() {
  local p="$1"
  [[ -z "$p" ]] && { printf ''; return; }
  case "$p" in /*) printf '%s' "$p" ;; *) printf '%s/%s' "$YIELD_ROOT" "$p" ;; esac
}

# pawl_evidence_floor <verdict-file> — evaluate the evidence-quality floor for a CONFIRMED
# (or, per duel amendment 2, a degraded-fallback) verdict. Prints the honesty caveat + any
# WARN/HOLD findings to stderr. RETURNS 0 when the caller may AUTHORIZE (floor satisfied, or
# still in the advisory window), 1 ONLY when enforcing AND the floor is violated (caller must
# HOLD). Reusable by the degraded-fallback path (.2): a degraded verdict must meet this floor
# before a failover can be trusted. Assumes the caller already ran the evidence-binding gate
# (the referenced evidence files exist + are non-empty).
pawl_evidence_floor() {
  local f="$1"
  local enforcing=0; _floor_enforcing || enforcing=1   # 0 = enforce, 1 = advisory
  local mode_label; [[ "$enforcing" -eq 0 ]] && mode_label="ENFORCING" || mode_label="ADVISORY"

  # Honesty caveat — ALWAYS printed when the floor runs.
  echo "PAWL-FLOOR ($mode_label): measures review SUBSTANCE (that a real, specific review ran over named code), NOT correctness — a substantive review can still be wrong." >&2

  local held=0

  # (1) SUBSTANCE — at least ONE evidence surface (a per-refuter evidence file, or the
  #     top-level council_artifact) must carry a file:line finding OR a reviewed-scope
  #     attestation.
  local any_substance=0 ev epath council cpath
  council="$(jq -r '.council_artifact // ""' "$f" 2>/dev/null)"
  if [[ -n "$council" ]]; then
    cpath="$(_resolve_ev_path "$council")"
    [[ -s "$cpath" ]] && _evidence_has_substance "$cpath" && any_substance=1
  fi
  if [[ "$any_substance" -ne 1 ]]; then
    while IFS= read -r ev; do
      [[ -z "$ev" ]] && continue
      epath="$(_resolve_ev_path "$ev")"
      if [[ -s "$epath" ]] && _evidence_has_substance "$epath"; then any_substance=1; break; fi
    done < <(jq -r '.refuters[].evidence // empty' "$f" 2>/dev/null)
  fi
  if [[ "$any_substance" -ne 1 ]]; then
    if [[ "$enforcing" -eq 0 ]]; then
      echo "PAWL-FLOOR: HOLD (evidence-quality floor) — CONFIRMED verdict carries NO review substance: no file:line finding and no reviewed-scope attestation (a files-reviewed count) in any evidence surface. A thin 'no blocking defects' stub is not a review." >&2
      held=1
    else
      echo "PAWL-FLOOR: WARN (advisory; evidence-quality floor) — CONFIRMED verdict carries no file:line finding and no reviewed-scope attestation; a thin stub would HOLD once the floor enforces (on/after $FLOOR_ENFORCE_AFTER). Still authorizing (advisory window)." >&2
    fi
  fi

  # (2) PER-ADAPTER GENUINE-RUN MARKER — each refuter attributable to a cold adapter
  #     (gpt=codex / gemini=agy) must carry that adapter's genuine-run marker in its OWN
  #     evidence, so a real cross-family review can be told apart from a lazy stub/echo. A
  #     family with no cold-adapter marker yet (claude) is advisory-skipped (amendment 1);
  #     a refuter with no evidence path is skipped (the substance gate already requires a
  #     real evidence surface to exist somewhere).
  local rfam rnorm rev repath marker
  while IFS=$'\t' read -r rfam rev; do
    [[ -z "$rfam" ]] && continue
    rnorm="$(normalize_family "$rfam")"
    [[ -z "$rnorm" ]] && continue
    marker="$(_family_genuine_marker "$rnorm")"
    [[ -z "$marker" ]] && continue          # no cold-adapter marker for this family — advisory skip
    [[ -z "$rev" ]] && continue             # no evidence path recorded — skip (substance gate covers presence)
    repath="$(_resolve_ev_path "$rev")"
    [[ -s "$repath" ]] || continue
    if ! grep -qiF -- "$marker" "$repath" 2>/dev/null; then
      if [[ "$enforcing" -eq 0 ]]; then
        echo "PAWL-FLOOR: HOLD (adapter genuine-run floor) — refuter family '$rnorm' (adapter marker '$marker') is MISSING its genuine-run marker in evidence '$rev' — cannot prove a real '$rnorm' review ran rather than an echo/stub." >&2
        held=1
      else
        echo "PAWL-FLOOR: WARN (advisory; adapter genuine-run floor) — refuter family '$rnorm' evidence '$rev' lacks the adapter genuine-run marker '$marker'; would HOLD once the floor enforces (on/after $FLOOR_ENFORCE_AFTER). Still authorizing (advisory window)." >&2
      fi
    fi
  done < <(jq -r '.refuters[] | [(.family // ""), (.evidence // "")] | @tsv' "$f" 2>/dev/null)

  [[ "$enforcing" -eq 0 && "$held" -eq 1 ]] && return 1
  return 0
}

# ---------------------------------------------------------------------------
# check <bead> <pr> [--dir D] [--head CURRENT_SHA]
# ---------------------------------------------------------------------------
do_check() {
  local bead="${1:-}" pr="${2:-}"; shift 2 || true
  local cur_head="" verdict_file=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --dir)          VDIR="${2:-}"; shift 2 ;;
      --head)         cur_head="${2:-}"; shift 2 ;;
      # --verdict-file overrides the default <dir>/<bead>.json resolution so the caller can
      # point check at a verdict file whose NAME is not <bead>.json (the REBOUND lineage
      # recursion targets an archived '<bead>.confirmed-<sha>.json'). The file's own bead_id
      # still must equal <bead> (the structural gate enforces it).
      --verdict-file) verdict_file="${2:-}"; shift 2 ;;
      *) shift ;;
    esac
  done
  [[ -n "$bead" && -n "$pr" ]] || { echo "pawl-verdict check: need <bead> <pr>" >&2; return 2; }

  local f="${verdict_file:-$VDIR/$bead.json}"
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
  #
  # AUTHORIZING dispositions: CONFIRMED (a real review of THIS commit) OR REBOUND (a
  # prior CONFIRMED review re-bound onto a byte-identical commit — see the REBOUND
  # lineage gate below, which is what actually earns a REBOUND its authorization).
  # A REBOUND that fails its lineage gate is refused there; REFUTED/ESCALATE/HOLD
  # refuse here. The refuter-CONFIRMED + author_context_id checks apply to both:
  # a REBOUND carries the ORIGINAL review's refuter panel (rebind-verified copies it
  # verbatim), so it must still be all-CONFIRMED with an author context.
  local result
  result="$(jq -r --arg bead "$bead" --arg pr "$pr" '
    if (.schema_version != "pawl-verdict.v1") then "BAD: schema_version != pawl-verdict.v1"
    elif (.bead_id != $bead) then "BAD: bead mismatch (verdict.bead_id=\(.bead_id) wanted \($bead))"
    elif ((.pr|tostring) != $pr) then "BAD: pr mismatch (verdict.pr=\(.pr) wanted \($pr))"
    elif ((.disposition != "CONFIRMED") and (.disposition != "REBOUND")) then "BAD: disposition=\(.disposition) (only CONFIRMED or REBOUND authorize; REFUTED/ESCALATE/HOLD refuse)"
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

  local disposition
  disposition="$(jq -r '.disposition // ""' "$f" 2>/dev/null)"

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

  # --- REBOUND lineage gate (age-rk3r.9): a REBOUND verdict authorizes the merge ONLY
  # when it descends from a FULLY-VALID authorizing CONFIRMED and the diff is proven
  # unchanged (patch-id + whitespace-significant content). It is NEVER forgeable as a fresh
  # CONFIRMED and is NO EASIER to authorize than the CONFIRMED it inherits (DEFECT 2 fix).
  # This block enforces the REBOUND-SPECIFIC requirements and then FALLS THROUGH to the
  # SHARED gate battery below (roster, fresh-context/diversity floor, evidence-binding, the
  # .11 evidence-quality floor) — the REBOUND carries the original panel verbatim, so it
  # must PASS those exact gates too; it does NOT short-circuit before them. Fail-closed
  # unless ALL hold:
  #   (1) rebound_from_verdict is present, resolvable, and is itself a FULLY-VALID
  #       authorizing CONFIRMED — validated by running the SAME check battery against it
  #       (recursive `check` at its own reviewed sha), not merely `.disposition=="CONFIRMED"`;
  #   (2) patch_id_proof is present AND matches the patch-id RE-DERIVED from THIS tip
  #       (recomputed from git — a stale/forged string cannot slip through); AND
  #   (3) the whitespace-significant +/- CONTENT LINES of the current tip EQUAL those of the
  #       reviewed commit (rebound_from_sha) — closing the patch-id whitespace hole (DEFECT 1),
  #       so a whitespace-only change (e.g. Python indentation) is REFUSED even though its
  #       patch-id matches.
  # CAVEAT (verbatim, load-bearing): a matching patch-id proves the change is rebase-stable
  # but is WHITESPACE-INSENSITIVE, so it does NOT prove the diff BYTES are unchanged, and even
  # byte-identical content on a new base can still break (semantic conflict). REBOUND therefore
  # requires (a) the same rebase-stable patch-id AND (b) byte-identical +/- content lines
  # (whitespace-significant) AND (c) a green full gate on the new tip. (c) is enforced at
  # WRITE time (rebind-verified refuses on a red gate); this `check` enforces (a), (b), and
  # the full lineage validity.
  if [[ "$disposition" == "REBOUND" ]]; then
    local reb_from reb_proof reb_from_sha
    reb_from="$(jq -r '.rebound_from_verdict // ""' "$f" 2>/dev/null)"
    reb_proof="$(jq -r '.patch_id_proof // ""' "$f" 2>/dev/null)"
    reb_from_sha="$(jq -r '.rebound_from_sha // ""' "$f" 2>/dev/null)"
    if [[ -z "$reb_from" ]]; then
      echo "PAWL-GATE: REBOUND verdict missing rebound_from_verdict lineage — fail-closed, merge refused (a REBOUND must name the CONFIRMED review it descends from)" >&2
      return 1
    fi
    if [[ -z "$reb_proof" ]]; then
      echo "PAWL-GATE: REBOUND verdict missing patch_id_proof — fail-closed, merge refused (no proof the diff is byte-identical to the reviewed one)" >&2
      return 1
    fi
    if [[ -z "$reb_from_sha" || ${#reb_from_sha} -lt 7 ]]; then
      echo "PAWL-GATE: REBOUND verdict missing/short rebound_from_sha — fail-closed, merge refused (cannot identify the reviewed commit to prove content identity)" >&2
      return 1
    fi
    # Resolve the lineage verdict path: absolute stays; a bare basename or relative path
    # resolves against the SAME verdicts dir this check is reading, then the repo root — so
    # lineage bound in-tree ('.agents/pawl-verdicts/<bead>.json') or by dir is found.
    local reb_path=""
    case "$reb_from" in
      /*) reb_path="$reb_from" ;;
      *)
        if [[ -f "$VDIR/$reb_from" ]]; then reb_path="$VDIR/$reb_from"
        elif [[ -f "$REPO_ROOT/$reb_from" ]]; then reb_path="$REPO_ROOT/$reb_from"
        else reb_path="$reb_from"; fi
        ;;
    esac
    if [[ ! -f "$reb_path" ]]; then
      echo "PAWL-GATE: REBOUND lineage verdict '$reb_from' not found (looked at $reb_path) — fail-closed, merge refused" >&2
      return 1
    fi
    # (1) LINEAGE MUST BE A FULLY-VALID AUTHORIZING CONFIRMED (DEFECT 2 fix). Not just
    # `.disposition=="CONFIRMED"` — run the ENTIRE check battery against the lineage verdict
    # at ITS OWN reviewed sha, so a thin/self-stamped/evidence-less "CONFIRMED" that would
    # NOT itself authorize a merge cannot be laundered into an authorizing REBOUND. The
    # recursive check re-validates schema + roster + fresh-context/diversity floor +
    # evidence-binding + the .11 evidence-quality floor on the lineage. It must be a plain
    # CONFIRMED (guarded below) so it takes the CONFIRMED path (no REBOUND recursion).
    local reb_disp reb_bead reb_pr
    reb_disp="$(jq -r '.disposition // ""' "$reb_path" 2>/dev/null)"
    if [[ "$reb_disp" != "CONFIRMED" ]]; then
      echo "PAWL-GATE: REBOUND lineage verdict '$reb_from' is disposition=$reb_disp, not CONFIRMED — a REBOUND may only descend from a genuine CONFIRMED review — fail-closed, merge refused" >&2
      return 1
    fi
    reb_bead="$(jq -r '.bead_id // ""' "$reb_path" 2>/dev/null)"
    reb_pr="$(jq -r '.pr // 0' "$reb_path" 2>/dev/null)"
    # Recurse: validate the lineage as a full authorizing CONFIRMED at its reviewed sha.
    # Point check at the lineage file DIRECTLY (--verdict-file) since its name is not
    # <bead>.json (the archived '<bead>.confirmed-<sha>.json'). PAWL_REBIND_LINEAGE_CHECK
    # guards against any accidental infinite recursion (the lineage is a plain CONFIRMED so
    # it won't re-enter this REBOUND block, but the guard is belt-and-suspenders). Evidence
    # paths in the lineage resolve against REPO_ROOT as usual.
    if [[ -z "${PAWL_REBIND_LINEAGE_CHECK:-}" ]]; then
      local lineage_out lineage_rc=0
      lineage_out="$(PAWL_REBIND_LINEAGE_CHECK=1 AGENTOPS_REPO_ROOT="$YIELD_ROOT" \
        bash "$0" check "$reb_bead" "$reb_pr" --verdict-file "$reb_path" --head "$reb_from_sha" 2>&1)" || lineage_rc=$?
      if [[ "$lineage_rc" -ne 0 ]]; then
        {
          echo "PAWL-GATE: REBOUND lineage verdict '$reb_from' is NOT a fully-valid authorizing CONFIRMED (it fails the same check battery a merge-authorizing CONFIRMED must pass — roster / fresh-context / evidence-binding / evidence-quality floor). A REBOUND may not launder a thin or invalid verdict into an authorizing one — fail-closed, merge refused."
          printf '%s\n' "$lineage_out" | sed 's/^/  lineage| /'
        } >&2
        return 1
      fi
    fi
    # (2)+(3) EQUIVALENCE PROVEN BETWEEN rebound_from_sha AND THE TIP (age-rk3r.9) — the
    # reviewed CONFIRMED applies to the tip ONLY if the tip's diff is TRULY EQUIVALENT to the
    # reviewed commit's. RE-DERIVE from git, for BOTH commits, BOTH keys:
    #   patch-id (rebase-stable) AND the BYTE-EXACT content signature (commit_content_lines:
    #   every diff byte significant EXCEPT text-hunk blob ids + @@ positions — so whitespace,
    #   file mode, binary content, and trailing-newline are all significant), and require the
    #   reviewed values to EQUAL the tip values. Comparing reviewed-vs-tip (not just
    # stored-proof-vs-tip) is what closes the forged-proof hole: a patch_id_proof set to the
    # tip's own patch-id can no longer bypass, because the authoritative comparison recomputes
    # the REVIEWED commit's keys and requires the match. The stored patch_id_proof is ALSO
    # required to equal the re-derived tip patch-id as a defense-in-depth consistency check (a
    # lineage whose recorded proof disagrees with git is tampered/stale). Compute against
    # YIELD_ROOT (the repo being gated).
    local tip_pid reviewed_pid tip_content reviewed_content
    tip_pid="$(commit_patch_id "$cur_head" "$YIELD_ROOT")"
    reviewed_pid="$(commit_patch_id "$reb_from_sha" "$YIELD_ROOT")"
    tip_content="$(commit_content_lines "$cur_head" "$YIELD_ROOT")"
    reviewed_content="$(commit_content_lines "$reb_from_sha" "$YIELD_ROOT")"
    if [[ -z "$tip_pid" || -z "$reviewed_pid" ]]; then
      echo "PAWL-GATE: REBOUND — could not compute git patch-id for the current head ($cur_head) or the reviewed commit ($reb_from_sha) (unknown sha / empty diff / git error) — fail-closed, merge refused" >&2
      return 1
    fi
    if [[ -z "$tip_content" || -z "$reviewed_content" ]]; then
      echo "PAWL-GATE: REBOUND — could not extract the byte-exact content signature for the current tip ($cur_head) or the reviewed commit ($reb_from_sha) — cannot prove the change is identical — fail-closed, merge refused" >&2
      return 1
    fi
    # AUTHORITATIVE: the reviewed commit's patch-id must equal the tip's.
    if [[ "$reviewed_pid" != "$tip_pid" ]]; then
      echo "PAWL-GATE: REBOUND — the reviewed commit ($reb_from_sha) patch-id=$reviewed_pid != the current tip ($cur_head) patch-id=$tip_pid; the tip is NOT the same change as the reviewed one — fail-closed, merge refused" >&2
      return 1
    fi
    # AUTHORITATIVE: the reviewed commit's BYTE-EXACT content signature must equal the tip's —
    # catches EVERY diff-byte difference patch-id misses: a whitespace-only change (Python
    # indentation), a FILE-MODE change (e.g. a data file made EXECUTABLE), a BINARY content
    # change, or a trailing-newline flip. The signature is byte-exact except text-hunk blob ids
    # + @@ positions, so it is complete by construction (denylist, not a leaky allowlist).
    if [[ "$reviewed_content" != "$tip_content" ]]; then
      echo "PAWL-GATE: REBOUND — the tip's byte-exact content signature differs from the reviewed commit's (patch-id matched but the change is NOT identical — a whitespace edit, a file-mode flip like a data file made executable, a binary-content change, or a trailing-newline flip is semantically load-bearing) — fail-closed, merge refused" >&2
      return 1
    fi
    # DEFENSE-IN-DEPTH: the recorded patch_id_proof must agree with git (a stale/forged proof
    # that disagrees with the re-derived tip patch-id is rejected).
    if [[ "$reb_proof" != "$tip_pid" ]]; then
      echo "PAWL-GATE: REBOUND patch_id_proof=$reb_proof disagrees with the re-derived current tip patch-id=$tip_pid — the recorded proof is stale or forged — fail-closed, merge refused" >&2
      return 1
    fi
    echo "PAWL-GATE: REBOUND lineage OK — descends from a fully-valid CONFIRMED '$reb_from'; the reviewed commit ($reb_from_sha) and the tip ($cur_head) are equivalent (patch-id $tip_pid + byte-exact content signature match); now validating the REBOUND's own panel through the shared gate battery…" >&2
    # FALL THROUGH — the REBOUND's own panel must pass the SAME roster/diversity/evidence/
    # floor gates below (DEFECT 2: no short-circuit). It authorizes only if BOTH the lineage
    # proof above AND the shared battery below pass.
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
  # Resolve RELATIVE evidence/council paths against YIELD_ROOT (the repo being gated =
  # AGENTOPS_REPO_ROOT on the embedded path = the user's repo; == REPO_ROOT in-checkout).
  # Script-relative REPO_ROOT would be the extracted-bundle temp dir on the embedded path,
  # so evidence written repo-relative would falsely read as missing there. (age-rk3r.9)
  resolve_path() {
    local p="$1"
    [[ -z "$p" ]] && { echo ""; return; }
    case "$p" in /*) echo "$p" ;; *) echo "$YIELD_ROOT/$p" ;; esac
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

  # --- evidence-QUALITY floor (age-rk3r.11): ADVISORY-FIRST. Evidence-binding above
  #     proves the review files EXIST + are non-empty; this floor proves they carry
  #     SUBSTANCE (a file:line finding or a reviewed-scope attestation) + each cold-adapter
  #     refuter carries its genuine-run marker. Warn-only until FLOOR_ENFORCE_AFTER; a
  #     violation then HOLDs. It measures review substance, NOT correctness (caveat printed).
  if ! pawl_evidence_floor "$f"; then
    echo "PAWL-GATE: evidence-quality floor HOLD (see PAWL-FLOOR above) — fail-closed, merge refused" >&2
    return 1
  fi

  echo "PAWL-GATE: $disposition, evidence-bound, commit-current pawl verdict (mode=$mode) for $bead (PR $pr, head ${verdict_head:0:12}) — merge authorized" >&2
  return 0
}

# ---------------------------------------------------------------------------
# write <bead> <pr> --disposition D --refuter fam:verdict [...] [opts]
# ---------------------------------------------------------------------------
do_write() {
  local bead="${1:-}" pr="${2:-}"; shift 2 || true
  local disposition="" head="" council="" attempt="1" mode="" author_ctx=""
  local difficulty="1" author_family="unknown" domain="" reason="" degraded=""
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
      # age-rk3r.2: honest-degradation flag (the .16-accepted `degraded` field). TRUE when the
      # panel could not run at its configured strength (a reviewer family was unavailable and a
      # fall-over/reduced-diversity path produced the verdict) but a real review still ran. Only
      # true|false accepted; absent => the field is OMITTED (nominal, byte-identical default).
      --degraded)       degraded="${2:-}"; shift 2 ;;
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
  # age-rk3r.2: validate --degraded (the .16 boolean field). Only true|false; anything else is a
  # caller bug — fail-closed rather than write a non-boolean the schema would then reject at check.
  if [[ -n "$degraded" ]]; then
    case "$degraded" in
      true|false) ;;
      *) echo "pawl-verdict write: --degraded must be 'true' or 'false' (got '$degraded')" >&2; return 2 ;;
    esac
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
    --arg degraded "$degraded" \
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
     + (if $council != "" then {council_artifact: $council} else {} end)
     + (if $degraded != "" then {degraded: ($degraded == "true")} else {} end)' \
    > "$tmp" || { rm -f "$tmp"; die "failed to render verdict json"; }
  mv "$tmp" "$out"
  echo "pawl-verdict: wrote $out (disposition=$disposition)" >&2

  # HIJACK GUARD (age-sylz): before emitting/binding the verdict edge, confirm the
  # worktree HEAD still equals the sha pawl-review.sh snapshotted at review start
  # (PAWL_REVIEW_START_HEAD). If a concurrent lane reset a SHARED landing worktree
  # mid-review, binding here would attach this verdict onto the OTHER lane's commit
  # — refuse fail-closed (exit nonzero, no edge). Absent snapshot => prior behavior.
  if ! _assert_review_head_unmoved; then
    return 1
  fi

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

# ---------------------------------------------------------------------------
# rebind-verified <bead> <pr> --from-verdict <path> --head NEWSHA [--dir D] [--repo-root R]
#
# The SAFE, patch-id-gated re-bind (age-rk3r.9): stop paying a full re-review for a
# byte-identical rebase. Given a PRIOR CONFIRMED verdict and a NEW tip proven to be the
# SAME change as the reviewed one, AND with the full local gate GREEN on the new tip,
# WRITE a DISTINCT REBOUND verdict binding the new tip and carrying lineage
# (rebound_from_verdict / rebound_from_sha / patch_id_proof). This REUSES a real review
# across a no-op rebase instead of re-running it — the reviewer would read the same diff
# bytes, so a second full review is pure cost with zero added assurance. A REBOUND is
# NEVER forgeable as a fresh CONFIRMED and is NO EASIER to authorize than the CONFIRMED it
# inherits: `check` authorizes it only when its lineage is a FULLY-VALID CONFIRMED, its
# patch_id_proof matches the new tip, and the tip's whitespace-significant content matches.
#
# This is a DISTINCT verb from `rebind` (do_rebind): `rebind` is pawl-land.sh's
# deterministic clean-land restamp (keeps CONFIRMED, no patch-id or gate check) and its
# behavior is locked; `rebind-verified` is the patch-id+content+gate-gated REBOUND writer.
#
# Permitted ONLY when ALL THREE hold:
#   (a) git patch-id --stable of the reviewed diff EQUALS the new tip's — the REBASE-STABLE
#       key (it normalizes @@ hunk positions that legitimately shift across a rebase), AND
#   (b) the +/- CONTENT LINES are byte-identical (WHITESPACE-SIGNIFICANT) between the
#       reviewed diff and the new-tip diff — because git patch-id is whitespace-INSENSITIVE,
#       so (a) alone would authorize a whitespace-only change (e.g. Python indentation,
#       semantically load-bearing); (b) closes that hole, AND
#   (c) the FULL local gate is green on the new tip (ao gate check --scope head — the gate
#       re-runs the tests against the new base, which catches a semantic conflict that even
#       a byte-identical diff on a new base can cause).
# CAVEAT (verbatim, load-bearing): a matching patch-id proves the change is rebase-stable
# but is WHITESPACE-INSENSITIVE, so it does NOT prove the diff bytes are unchanged, and even
# byte-identical content on a new base can still break (semantic conflict). REBOUND requires
# (a) the same rebase-stable patch-id AND (b) byte-identical +/- content lines (whitespace-
# significant) AND (c) a green full gate on the new tip.
#
# Gate command is overridable via PAWL_REBIND_GATE_CMD (for tests / a documented
# alternate gate); default "ao gate check --scope head" run with cwd = the repo root.
# Set PAWL_REBIND_SKIP_GATE=1 ONLY in a test that pins the patch-id/content/lineage logic in
# isolation (it prints a loud warning; never use it in production — it removes the
# behavior half of the safety argument).
do_rebind_verified() {
  local bead="${1:-}" pr="${2:-}"; shift 2 2>/dev/null || true
  local newhead="" from_verdict="" repo_root="$REPO_ROOT"
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --head)          newhead="${2:-}"; shift 2 ;;
      --from-verdict)  from_verdict="${2:-}"; shift 2 ;;
      --dir)           VDIR="${2:-}"; shift 2 ;;
      --repo-root)     repo_root="${2:-}"; shift 2 ;;
      *) echo "pawl-verdict rebind-verified: unknown flag $1" >&2; return 2 ;;
    esac
  done
  [[ -n "$bead" && -n "$pr" ]] || { echo "pawl-verdict rebind-verified: need <bead> <pr> --head NEWSHA [--from-verdict PATH]" >&2; return 2; }
  [[ -n "$newhead" && ${#newhead} -ge 7 ]] || { echo "pawl-verdict rebind-verified: need --head NEWSHA (>=7 chars)" >&2; return 2; }

  # The prior verdict defaults to this bead's own verdict file (the common case: the
  # verdict that reviewed the pre-rebase commit is <dir>/<bead>.json). --from-verdict
  # overrides for cross-bead lineage. It MUST exist and be CONFIRMED (never a REBOUND
  # or a REFUTED — a REBOUND may only descend from a genuine authorizing review).
  local prior="${from_verdict:-$VDIR/$bead.json}"
  [[ -f "$prior" ]] || { echo "pawl-verdict rebind-verified: prior verdict '$prior' not found — nothing to re-bind" >&2; return 2; }
  local prior_disp prior_sha
  prior_disp="$(jq -r '.disposition // ""' "$prior" 2>/dev/null)"
  prior_sha="$(jq -r '.head_sha // ""' "$prior" 2>/dev/null)"
  if [[ "$prior_disp" != "CONFIRMED" ]]; then
    echo "pawl-verdict rebind-verified: prior verdict '$prior' is disposition=$prior_disp, not CONFIRMED — a REBOUND may only descend from a CONFIRMED review — refusing" >&2
    return 2
  fi
  if [[ -z "$prior_sha" || ${#prior_sha} -lt 7 ]]; then
    echo "pawl-verdict rebind-verified: prior verdict '$prior' has no/short head_sha — cannot identify the reviewed commit — refusing" >&2
    return 2
  fi

  # LINEAGE VALIDITY (DEFECT 2 fix, WRITE-TIME half) — the prior CONFIRMED must be a
  # FULLY-VALID authorizing CONFIRMED, not just disposition=="CONFIRMED". Run the SAME check
  # battery against it at ITS reviewed sha (roster / fresh-context / evidence-binding / the
  # .11 evidence-quality floor) so rebind-verified REFUSES to build a REBOUND from a thin,
  # self-stamped, or evidence-less CONFIRMED — the laundering path. (Defense-in-depth: `check`
  # re-validates the lineage again at merge time; catching it here gives a clear write-time
  # error.) Guarded by PAWL_REBIND_LINEAGE_CHECK against any accidental recursion.
  if [[ -z "${PAWL_REBIND_LINEAGE_CHECK:-}" ]]; then
    local prior_bead prior_pr lin_out lin_rc=0
    prior_bead="$(jq -r '.bead_id // ""' "$prior" 2>/dev/null)"
    prior_pr="$(jq -r '.pr // 0' "$prior" 2>/dev/null)"
    lin_out="$(PAWL_REBIND_LINEAGE_CHECK=1 AGENTOPS_REPO_ROOT="$repo_root" \
      bash "$0" check "$prior_bead" "$prior_pr" --verdict-file "$prior" --head "$prior_sha" 2>&1)" || lin_rc=$?
    if [[ "$lin_rc" -ne 0 ]]; then
      {
        echo "pawl-verdict rebind-verified: prior verdict '$prior' is NOT a fully-valid authorizing CONFIRMED — it fails the same check battery a merge-authorizing CONFIRMED must pass (roster / fresh-context / evidence-binding / evidence-quality floor). A REBOUND may not launder a thin/invalid verdict into an authorizing one — refusing."
        printf '%s\n' "$lin_out" | sed 's/^/  lineage| /'
      } >&2
      return 2
    fi
  fi

  # (a) PATCH-ID MATCH — the rebase-stable key. Compute both stable patch-ids from git;
  # refuse (fail-closed) on any difference, naming the FIRST differing file so the operator
  # sees WHY it is not a no-op rebase. An empty patch-id (unknown sha / empty diff / git
  # error) is treated as "cannot prove identity" and refuses. NOTE (DEFECT 1): patch-id is
  # WHITESPACE-INSENSITIVE, so this match alone does NOT prove the diff bytes are unchanged
  # — the (b) content-line check below is what closes that hole.
  local prior_pid new_pid
  prior_pid="$(commit_patch_id "$prior_sha" "$repo_root")"
  new_pid="$(commit_patch_id "$newhead" "$repo_root")"
  if [[ -z "$prior_pid" ]]; then
    echo "pawl-verdict rebind-verified: could not compute patch-id for the reviewed commit $prior_sha (unknown sha / empty diff / git error) — refusing" >&2
    return 2
  fi
  if [[ -z "$new_pid" ]]; then
    echo "pawl-verdict rebind-verified: could not compute patch-id for the new tip $newhead (unknown sha / empty diff / git error) — refusing" >&2
    return 2
  fi
  if [[ "$prior_pid" != "$new_pid" ]]; then
    # Name the first differing file — the diff CHANGED, so this is NOT a no-op rebase
    # and a REBOUND is not permitted; the change needs a real re-review.
    local first_diff
    first_diff="$(git -c core.fsmonitor= -C "$repo_root" diff --no-ext-diff --no-textconv --name-only "$prior_sha" "$newhead" 2>/dev/null | head -1)"
    {
      echo "pawl-verdict rebind-verified: patch-id MISMATCH — the diff CHANGED since the review, so this is NOT a no-op rebase — refusing (a REBOUND is not permitted; re-run the full review)."
      echo "  reviewed commit ${prior_sha:0:12} patch-id: $prior_pid"
      echo "  new tip         ${newhead:0:12} patch-id: $new_pid"
      if [[ -n "$first_diff" ]]; then
        echo "  first differing file (reviewed..new): $first_diff"
      else
        echo "  (the patch-ids differ; the two commits' diffs are not byte-identical)"
      fi
    } >&2
    return 2
  fi

  # (b) CONTENT-LINE MATCH (DEFECT 1 fix) — patch-id is whitespace-insensitive, so require
  # the +/- change content to be IDENTICAL BYTE-FOR-BYTE (leading whitespace included),
  # comparing ONLY the stable structural headers + content lines (index/@@ dropped, since
  # they legitimately shift across a rebase). This is what makes the "byte-identical diff"
  # safety claim TRUE: a whitespace-only change (e.g. Python indentation) has the same
  # patch-id but a DIFFERENT content-line set and is REFUSED here. Refuse fail-closed on any
  # difference (or an empty set = cannot prove content identity), naming the difference.
  local prior_content new_content
  prior_content="$(commit_content_lines "$prior_sha" "$repo_root")"
  new_content="$(commit_content_lines "$newhead" "$repo_root")"
  if [[ -z "$prior_content" || -z "$new_content" ]]; then
    echo "pawl-verdict rebind-verified: could not extract whitespace-significant content lines for the reviewed commit or the new tip — cannot prove the diff bytes are identical — refusing" >&2
    return 2
  fi
  if [[ "$prior_content" != "$new_content" ]]; then
    {
      echo "pawl-verdict rebind-verified: CONTENT-LINE MISMATCH — the patch-ids match but the +/- content BYTES differ (patch-id is whitespace-INSENSITIVE; a whitespace-only change is semantically load-bearing, e.g. Python indentation) — refusing (a REBOUND is not permitted; re-run the full review)."
      echo "  reviewed commit ${prior_sha:0:12} vs new tip ${newhead:0:12} — first differing content line:"
      # Show the first line where the whitespace-significant content sets diverge.
      diff <(printf '%s\n' "$prior_content") <(printf '%s\n' "$new_content") 2>/dev/null | grep -E '^[<>]' | head -4 | sed 's/^/    /'
    } >&2
    return 2
  fi

  # (c) FULL GATE GREEN ON THE NEW TIP — the behavior check. A byte-identical diff on a
  # new base can still break (semantic conflict); the gate re-runs the tests against the
  # new base. Run with cwd = repo root so the gate resolves the checkout it is gating.
  #
  # HEAD==newhead GUARD (required for the gate contract to hold): the gate runs against the
  # worktree's CURRENT HEAD (`cd "$repo_root" && <gate>`), NOT against $newhead directly. If
  # the worktree is checked out at some OTHER commit while --head names $newhead, the gate
  # would validate the WRONG tree yet we would stamp a REBOUND for $newhead — the "gate is
  # green on the NEW TIP" claim would be false. So REQUIRE the worktree HEAD to resolve to
  # $newhead before the gate runs; if not, REFUSE fail-closed and instruct (never auto-check
  # out — that mutates the user's worktree). This guarantees the behavior check ran against
  # the very commit being rebound. Bypassed only when the gate itself is skipped (test-only).
  case "$(printf '%s' "${PAWL_REBIND_SKIP_GATE:-}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) : ;;   # gate skipped (test-only) — the HEAD guard is moot without a gate run
    *)
      local worktree_head
      worktree_head="$(git -c core.fsmonitor= -C "$repo_root" rev-parse HEAD 2>/dev/null)"
      # Resolve $newhead to a full sha in this repo so a short/ref --head compares correctly.
      local newhead_full
      newhead_full="$(git -c core.fsmonitor= -C "$repo_root" rev-parse --verify "${newhead}^{commit}" 2>/dev/null)"
      if [[ -z "$worktree_head" || -z "$newhead_full" || "$worktree_head" != "$newhead_full" ]]; then
        local wt_disp="${worktree_head:0:12}"; [[ -z "$wt_disp" ]] && wt_disp="<none>"
        {
          echo "pawl-verdict rebind-verified: the gate must run against the rebound tip, but the worktree HEAD ($wt_disp) is not the rebound tip ${newhead:0:12} — refusing (the gate would validate the wrong commit while stamping a REBOUND for $newhead)."
          echo "  check out $newhead first (git -C $repo_root checkout $newhead), or omit --head to rebind onto the current HEAD."
        } >&2
        return 2
      fi
      ;;
  esac

  local gate_cmd="${PAWL_REBIND_GATE_CMD:-ao gate check --scope head}"
  case "$(printf '%s' "${PAWL_REBIND_SKIP_GATE:-}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on)
      echo "pawl-verdict rebind-verified: WARNING — PAWL_REBIND_SKIP_GATE set; SKIPPING the green-gate behavior check (TEST-ONLY; the patch-id match alone does NOT prove behavior is unchanged)" >&2
      ;;
    *)
      echo "pawl-verdict rebind-verified: running the full gate on the new tip ($gate_cmd) — the behavior check a byte-identical diff cannot give…" >&2
      local gate_out gate_rc=0
      # gate_cmd is an operator/default command LINE (e.g. "ao gate check --scope head"),
      # so it is intentionally run through the shell via eval to honor its own word-split
      # + flags. It is NOT untrusted repo input: production default is a fixed literal, and
      # the only override is PAWL_REBIND_GATE_CMD, an operator/env-set value.
      gate_out="$( cd "$repo_root" && eval "$gate_cmd" 2>&1 )" || gate_rc=$?
      if [[ "$gate_rc" -ne 0 ]]; then
        {
          echo "pawl-verdict rebind-verified: the FULL gate is RED on the new tip $newhead (rc=$gate_rc) — refusing to write a REBOUND. A byte-identical diff on a new base can still break (semantic conflict); the red gate is that check firing."
          printf '%s\n' "$gate_out" | tail -n 20 | sed 's/^/  | /'
        } >&2
        return 2
      fi
      echo "pawl-verdict rebind-verified: gate GREEN on the new tip." >&2
      ;;
  esac

  # BOTH conditions hold — write the DISTINCT REBOUND verdict binding the new tip.
  # Preserve the prior review's refuters/mode/author_context/attempt VERBATIM (the
  # review did not re-run), flip disposition to REBOUND, set the new head + timestamp,
  # and record the three lineage fields. rebound_from_verdict points at the prior
  # verdict PATH so `check` can re-verify the lineage root was CONFIRMED.
  mkdir -p "$VDIR"
  local out="$VDIR/$bead.json" tmp
  tmp="$(mktemp "$VDIR/.$bead.XXXXXX")"

  # LINEAGE PRESERVATION (critical): the common case defaults the prior verdict to this
  # bead's OWN file ($VDIR/$bead.json), which is ALSO $out — overwriting it with the
  # REBOUND would make rebound_from_verdict point at a file that is now REBOUND, not
  # CONFIRMED, and `check`'s lineage gate (which re-reads it and requires CONFIRMED)
  # would refuse. So when prior IS out, ARCHIVE the original CONFIRMED verdict to a
  # distinct, auditable path FIRST and point the lineage there. Compare physical paths
  # (macOS /tmp vs /private/tmp). An explicit --from-verdict at a distinct path is used
  # as-is (no archive needed — it survives).
  local lineage_ref="$prior"
  local prior_phys out_phys
  prior_phys="$(cd "$(dirname "$prior")" 2>/dev/null && printf '%s/%s' "$(pwd -P)" "$(basename "$prior")")"
  out_phys="$(cd "$(dirname "$out")" 2>/dev/null && printf '%s/%s' "$(pwd -P)" "$(basename "$out")")"
  if [[ -n "$prior_phys" && "$prior_phys" == "$out_phys" ]]; then
    local archive="$VDIR/$bead.confirmed-${prior_sha:0:12}.json"
    if ! cp "$prior" "$archive" 2>/dev/null; then
      rm -f "$tmp"; die "rebind-verified: could not archive the prior CONFIRMED verdict to $archive — refusing (lineage would be lost)"
    fi
    lineage_ref="$archive"
    echo "pawl-verdict rebind-verified: archived the prior CONFIRMED verdict to $archive (lineage root preserved for check)" >&2
  fi

  jq --arg h "$newhead" \
     --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
     --arg from "$lineage_ref" \
     --arg fromsha "$prior_sha" \
     --arg proof "$new_pid" \
     '.disposition = "REBOUND"
      | .head_sha = $h
      | .generated_at = $ts
      | .rebound_from_verdict = $from
      | .rebound_from_sha = $fromsha
      | .patch_id_proof = $proof' \
     "$prior" > "$tmp" || { rm -f "$tmp"; die "rebind-verified: failed to render REBOUND verdict json"; }
  mv "$tmp" "$out"
  echo "pawl-verdict: wrote REBOUND $out -> head ${newhead:0:12} (lineage: ${lineage_ref} @ ${prior_sha:0:12}, patch-id $new_pid)" >&2

  # HIJACK GUARD (age-sylz): as with write, refuse to emit/bind the edge if a
  # concurrent lane moved a shared worktree HEAD out from under this run.
  if ! _assert_review_head_unmoved; then
    return 1
  fi
  # LIMITATION (age-rk3r.18): REBOUND is honored by pawl-verdict check (the reconcile/merge
  # path — .9's stated acceptance); the portable `ao verify init` pre-push gate + CI
  # (cli/cmd/ao/verify_prepush.go, scripts/check-tip-verdict-ci.sh) honor only CONFIRMED
  # today, so this committed REBOUND edge is safely REFUSED there (fail-closed) until
  # age-rk3r.18 wires Go-side REBOUND lineage+proof re-validation.
  # Re-fire the verdict sensor for the REBOUND edge (CHECKED + auto-bind, same as write).
  emit_verdict_edge_checked "$out" "$bead" "REBOUND"
}

case "$cmd" in
  check) do_check "$@" ;;
  write) do_write "$@" ;;
  rebind) do_rebind "$@" ;;
  rebind-verified) do_rebind_verified "$@" ;;
  -h|--help|"") cat >&2 <<'H'
Usage:
  pawl-verdict.sh check <bead-id> <pr> [--dir D] [--head CURRENT_SHA]
  pawl-verdict.sh write <bead-id> <pr> --disposition <CONFIRMED|REFUTED|ESCALATE|HOLD> --head SHA \
      --author-context <id> [--mode <fresh-context|multi-model>] \
      --refuter <family>:<CONFIRMED|REFUTED>:<context_id>[:<evidence-path>] [--refuter ...] \
      [--dir D] [--council PATH] [--attempt N] [--degraded true|false]
      --degraded true marks an honestly-DEGRADED review posture (a fall-over reviewer produced
        this verdict after an outage on the configured family). Absent => nominal (field omitted).
  pawl-verdict.sh rebind <bead-id> <pr> --head NEWSHA [--dir D]
      Re-stamp an existing CONFIRMED verdict onto a new head (refuters preserved) — the
      deterministic clean-land step after the verdict's ledger edge is committed.
  pawl-verdict.sh rebind-verified <bead-id> <pr> --head NEWSHA [--from-verdict PATH] [--dir D] [--repo-root R]
      The SAFE, patch-id-gated re-bind: authorize a rebase that is the SAME change WITHOUT a
      full re-review by writing a DISTINCT REBOUND verdict (lineage: rebound_from_verdict /
      rebound_from_sha / patch_id_proof). Permitted ONLY when ALL THREE hold: (a) git patch-id
      --stable of the reviewed diff EQUALS the new tip's (the rebase-stable key), AND (b) the
      +/- content lines are byte-identical WHITESPACE-SIGNIFICANT (patch-id is whitespace-
      INSENSITIVE, so (a) alone would pass a whitespace-only change like Python indentation),
      AND (c) the full local gate is GREEN on the new tip (ao gate check --scope head; override
      via PAWL_REBIND_GATE_CMD).
      CAVEAT: a matching patch-id proves the change is rebase-stable but is WHITESPACE-
      INSENSITIVE, so it does NOT prove the diff bytes are unchanged, and even byte-identical
      content on a new base can still break (semantic conflict). REBOUND requires (a) the same
      rebase-stable patch-id AND (b) byte-identical +/- content lines (whitespace-significant)
      AND (c) a green full gate on the new tip. `check` authorizes a REBOUND only when its
      lineage is a FULLY-VALID CONFIRMED (it re-runs the same roster/fresh-context/evidence/
      floor gates against the lineage), its patch_id_proof matches the tip, and the tip's
      content bytes match — never forgeable, and no easier to authorize than the CONFIRMED it
      inherits.

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
