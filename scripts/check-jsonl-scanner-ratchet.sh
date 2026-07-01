#!/usr/bin/env bash
# check-jsonl-scanner-ratchet.sh (age-storage-hardening-roxg.3) —
# ADVISORY changed-scope ratchet: no NEW raw `bufio.NewScanner` over JSONL
# outside cli/internal/storage.
#
# WHY
# ---
# A raw `bufio.NewScanner` defaults to a 64KB per-token buffer and, worse,
# fails SILENTLY: a line over the buffer is truncated (or the scan stops) with
# no error returned. 64 files across the CLI hand-roll scanner loops and 45
# re-decide buffer sizes; any unbumped scanner over a *.jsonl stream silently
# truncates at 64KB. The blessed replacement is the storage package's loud
# helpers:
#     storage.ScanJSONL       — scan an io.Reader with the ErrLineTooLong policy
#     storage.ScanJSONLFile   — same, opening a path
# which raise a LOUD error (storage.ErrLineTooLong) instead of truncating.
# (These names land via sibling bead age-storage-hardening-roxg.1, which
# precedes this ratchet in the landing chain; the ratchet only NAMES them in
# finding text, it does not import them, so it is safe to land in either order.)
#
# The mass migration of the ~44 existing sites is DELIBERATELY NOT happening
# here. This ratchet exists to (a) stop NEW raw-scanner-over-JSONL sites and
# (b) apply shrink pressure on the grandfathered set: a grandfathered file that
# no longer trips the heuristic must be pruned from the list.
#
# SCOPE LIMIT — READ THIS, DO NOT PRETEND COMPLETENESS
# ----------------------------------------------------
# This is a GREP HEURISTIC over source text. It is a FILE-LEVEL APPROXIMATION,
# NOT an AST / dataflow analysis. Concretely:
#   * FALSE NEGATIVES are possible. The scanner and the ".jsonl" mention must be
#     in the SAME file for the heuristic to fire. A scanner in file A reading a
#     path constructed in file B is NOT caught. A path built from a variable, a
#     constant in another package, or ".json"+"l" concatenation is NOT caught.
#   * FALSE POSITIVES are possible. A file that mentions ".jsonl" anywhere (a
#     comment, an unrelated path string, a doc reference) AND separately uses a
#     `bufio.NewScanner` over something that is NOT JSONL will be flagged.
# This heuristic is ACCEPTABLE precisely because the gate is ADVISORY
# (non-blocking): a false positive costs a one-line grandfather entry, never a
# blocked push. We NEVER claim this catches every unsafe scanner — honest scope
# bounding is a standing review convergence in this repo. Treat a finding as a
# prompt to look, not a proof of a bug.
#
# HEURISTIC (per changed cli/**/*.go file, outside cli/internal/storage/):
#   trips  ⟺  the file mentions ".jsonl" ANYWHERE
#             AND the file INVOKES `bufio.NewScanner(` (open-paren form — a bare
#                 mention of the symbol in a comment/doc, e.g. this gate's own
#                 registry row, does NOT trip it; every real site uses the paren)
#             AND (changed-content guard) the ADDED hunk introduces
#                 `bufio.NewScanner(` — so merely editing an already-grandfathered
#                 file without adding a scanner does not re-flag it.
#
# BEHAVIOR
#   1. New/modified file that TRIPS and is NOT grandfathered  -> FAIL (advisory),
#      finding names storage.ScanJSONL / storage.ScanJSONLFile.
#   2. Grandfathered file (in scripts/.jsonl-scanner-grandfather) -> exempt.
#      AUTHORITY IS THE INTERSECTION of the working snapshot and the base-ref
#      snapshot: an entry added in the same diff grants NO protection (a commit
#      cannot self-allowlist its own new site). Initial-snapshot commit (no
#      grandfather at the base ref) stands alone.
#   3. SHRINK-ONLY LIST: any DATA line ADDED to the grandfather relative to the
#      base ref -> FAIL (the list only shrinks; sole exception: the initial
#      snapshot). And a grandfathered file that NO LONGER trips the whole-file
#      heuristic -> FAIL demanding a prune.
#   4. cli/internal/storage/** -> exempt (that is the home of the blessed impl).
#
# USAGE
#   bash scripts/check-jsonl-scanner-ratchet.sh                 # --scope head (default)
#   bash scripts/check-jsonl-scanner-ratchet.sh --scope worktree
#   bash scripts/check-jsonl-scanner-ratchet.sh --scope staged
#   bash scripts/check-jsonl-scanner-ratchet.sh --scope auto
#   bash scripts/check-jsonl-scanner-ratchet.sh --regenerate    # rewrite the grandfather file
#
# EXIT: 0 pass/not-applicable · 1 advisory finding (new site or stale grandfather).

# Strict mode + hijack-proof REPO_ROOT come from the shared preamble
# (scripts/lib/preamble.sh). The lib anchors REPO_ROOT at its OWN directory, so
# running this gate from inside a different checkout cannot hijack the root —
# and the bats suite exercises it against an isolated temp repo by copying the
# script AND the lib into the temp skeleton (the lib then anchors there).
# `CDPATH=` is an intentional env-prefix, not an assignment — hence SC1007.
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"
cd "$REPO_ROOT" || exit 2

GRANDFATHER_FILE="scripts/.jsonl-scanner-grandfather"
SCOPE="head"
MODE="check"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --scope)
      shift
      [[ $# -gt 0 ]] || { echo "--scope requires a value" >&2; exit 2; }
      SCOPE="$1"
      ;;
    --scope=*) SCOPE="${1#--scope=}" ;;
    --regenerate) MODE="regenerate" ;;
    -h|--help) sed -n '2,60p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "Unknown arg: $1" >&2; exit 2 ;;
  esac
  shift
done

case "$SCOPE" in
  head|staged|worktree|auto) ;;
  *) echo "Invalid --scope: $SCOPE (expected head|staged|worktree|auto)" >&2; exit 2 ;;
esac

# is_exempt_path PATH -> 0 (exempt) if the path is OUTSIDE the ratchet's scope:
# not a cli/*.go file at all, OR under cli/internal/storage/ (the blessed home of
# the ScanJSONL impl). Returns 1 (in scope) for any other cli/*.go file at any
# depth. `cli/*` + a `.go` suffix test covers every depth (case `*` is a single
# path segment for the prefix, and *.go matches the whole path suffix).
is_exempt_path() {
  local p="$1"
  case "$p" in
    cli/internal/storage/*) return 0 ;;  # blessed impl home — exempt
    cli/*)
      [[ "$p" == *.go ]] && return 1     # a cli Go file at any depth — in scope
      return 0                           # a cli non-Go file — exempt
      ;;
    *) return 0 ;;                       # not under cli/ — exempt
  esac
}

# file_trips PATH -> 0 if the WHOLE FILE trips the heuristic (mentions .jsonl AND
# INVOKES bufio.NewScanner). Used for the grandfather set and the shrink ratchet.
# The scanner match requires the invocation form `bufio.NewScanner(` (open paren)
# so a bare mention of the symbol in a comment/doc — e.g. this gate's own registry
# row — does not trip it. Every real site uses the paren form.
file_trips() {
  local p="$1"
  [[ -f "$p" ]] || return 1
  grep -q '\.jsonl' "$p" 2>/dev/null || return 1
  grep -q 'bufio\.NewScanner(' "$p" 2>/dev/null || return 1
  return 0
}

# compute_grandfather_set -> emit, sorted, every in-scope cli/**/*.go file that
# currently trips the whole-file heuristic. This is the SAME heuristic used to
# flag, so a fresh regenerate is authoritative for "what is grandfathered now".
compute_grandfather_set() {
  local f
  # -l: names of files INVOKING bufio.NewScanner(; then filter by scope + .jsonl.
  while IFS= read -r f; do
    is_exempt_path "$f" && continue
    grep -q '\.jsonl' "$f" 2>/dev/null || continue
    printf '%s\n' "$f"
  done < <(grep -rl 'bufio\.NewScanner(' cli --include='*.go' 2>/dev/null || true) | LC_ALL=C sort
}

if [[ "$MODE" == "regenerate" ]]; then
  {
    echo "# scripts/.jsonl-scanner-grandfather — FILENAME-pinned grandfather list."
    echo "#"
    echo "# Every cli/**/*.go file (outside cli/internal/storage/) that currently has"
    echo "# BOTH a bufio.NewScanner( invocation AND a .jsonl mention. These predate the ratchet"
    echo "# (age-storage-hardening-roxg.3) and are exempt. The list only SHRINKS: a"
    echo "# grandfathered file that no longer trips the heuristic must be pruned."
    echo "#"
    echo "# Blessed replacement for a raw scanner over JSONL: storage.ScanJSONL /"
    echo "# storage.ScanJSONLFile (loud storage.ErrLineTooLong policy)."
    echo "#"
    echo "# Regenerate with:  bash scripts/check-jsonl-scanner-ratchet.sh --regenerate"
    echo "# (regenerate at LAND time, after the final rebase — sibling adoptions may"
    echo "#  have cleaned sites.)"
    compute_grandfather_set
  } > "$GRANDFATHER_FILE"
  echo "regenerated $GRANDFATHER_FILE ($(grep -cv '^#' "$GRANDFATHER_FILE" 2>/dev/null || echo 0) files)"
  exit 0
fi

# Load the grandfather set (filename per non-comment line).
declare -a GRANDFATHER=()
if [[ -f "$GRANDFATHER_FILE" ]]; then
  while IFS= read -r line; do
    [[ -z "$line" || "$line" == \#* ]] && continue
    GRANDFATHER+=("$line")
  done < "$GRANDFATHER_FILE"
fi

# grandfather_base_ref: the git ref holding the PRE-change grandfather snapshot
# for the active scope — the baseline the shrink-only rule is enforced against.
# Mirrors collect_changed_files' scope semantics (pattern ported from
# scripts/check-new-scripts-use-preamble.sh).
grandfather_base_ref() {
  case "$SCOPE" in
    head) echo "HEAD^" ;;
    staged|worktree) echo "HEAD" ;;
    auto)
      if [[ -n "$(git diff --cached --name-only 2>/dev/null || true)" || -n "$(git diff --name-only 2>/dev/null || true)" ]]; then
        echo "HEAD"
      else
        echo "HEAD^"
      fi
      ;;
  esac
}

# Grandfather AUTHORITY: an entry grants protection only if it is present in
# BOTH the working snapshot AND the base-ref snapshot (intersection). The
# working file alone is attacker-controlled — a commit could add a new tripping
# file AND append its path to the grandfather, self-granting protection (the
# bypass check_grandfather_shrink_only also rejects; the intersection makes the
# exemption independently fail-closed). If the base ref has no snapshot at all
# (the initial-snapshot commit, or a root commit), the working file is the
# cutoff and stands alone.
BASE_GF_EXISTS=0
BASE_GF_LINES=""
load_base_grandfather() {
  local base_ref
  base_ref="$(grandfather_base_ref)"
  if BASE_GF_LINES="$(git show "$base_ref:$GRANDFATHER_FILE" 2>/dev/null)"; then
    BASE_GF_EXISTS=1
  fi
}
load_base_grandfather

is_grandfathered() {
  local p="$1" g found=1
  for g in "${GRANDFATHER[@]}"; do
    [[ "$g" == "$p" ]] && { found=0; break; }
  done
  [[ "$found" -eq 0 ]] || return 1
  if [[ "$BASE_GF_EXISTS" -eq 1 ]]; then
    printf '%s\n' "$BASE_GF_LINES" | grep -qxF -- "$p" || return 1
  fi
  return 0
}

# check_grandfather_shrink_only -> fail if any DATA line was ADDED to the
# grandfather snapshot relative to the base ref. This closes the bypass where a
# change ships a new raw-scanner site AND appends its path to the grandfather in
# the same diff — the list may only SHRINK. The one legal addition is the
# INITIAL snapshot: if the file does not exist at the base ref, there is nothing
# to ratchet against yet and the whole snapshot is the cutoff.
check_grandfather_shrink_only() {
  local added
  [[ -f "$GRANDFATHER_FILE" ]] || return 0
  [[ "$BASE_GF_EXISTS" -eq 1 ]] || return 0
  added="$(comm -13 \
    <(printf '%s\n' "$BASE_GF_LINES" | grep -vE '^#|^[[:space:]]*$' | LC_ALL=C sort -u) \
    <(grep -vE '^#|^[[:space:]]*$' "$GRANDFATHER_FILE" | LC_ALL=C sort -u))"
  if [[ -n "$added" ]]; then
    echo "FAIL: $GRANDFATHER_FILE gained new entries — the grandfather list only SHRINKS." >&2
    echo "      A new site cannot be allowlisted; use storage.ScanJSONL / storage.ScanJSONLFile" >&2
    echo "      (loud storage.ErrLineTooLong policy) instead. Added entries:" >&2
    while IFS= read -r line; do
      echo "        $line" >&2
    done <<< "$added"
    return 1
  fi
  return 0
}

# --- collect changed files for the requested scope -------------------------
collect_changed_files() {
  case "$SCOPE" in
    head)     git diff-tree --no-commit-id --name-only -r HEAD 2>/dev/null || true ;;
    staged)   git diff --cached --name-only 2>/dev/null || true ;;
    worktree) git diff --name-only 2>/dev/null || true
              git ls-files --others --exclude-standard 2>/dev/null || true ;;
    auto)
      if [[ -n "$(git diff --cached --name-only 2>/dev/null || true)" ]]; then
        git diff --cached --name-only 2>/dev/null || true
      elif [[ -n "$(git diff --name-only 2>/dev/null || true)" ]]; then
        git diff --name-only 2>/dev/null || true
        git ls-files --others --exclude-standard 2>/dev/null || true
      else
        git diff-tree --no-commit-id --name-only -r HEAD 2>/dev/null || true
      fi
      ;;
  esac
}

# added_hunk_has_scanner PATH -> 0 if the ADDED content for PATH in this scope
# introduces a bufio.NewScanner. Untracked files (worktree scope) have no diff,
# so treat the whole file as "added". This is the changed-content guard: editing
# an already-grandfathered file without adding a scanner does not re-flag it.
added_hunk_has_scanner() {
  local p="$1" diffcmd
  case "$SCOPE" in
    head)     diffcmd=(git diff --no-color HEAD~1 HEAD -- "$p") ;;
    staged)   diffcmd=(git diff --no-color --cached -- "$p") ;;
    worktree) diffcmd=(git diff --no-color -- "$p") ;;
    auto)
      if ! git diff --cached --name-only 2>/dev/null | grep -qxF "$p"; then
        if git diff --name-only 2>/dev/null | grep -qxF "$p"; then
          diffcmd=(git diff --no-color -- "$p")
        else
          diffcmd=(git diff --no-color HEAD~1 HEAD -- "$p")
        fi
      else
        diffcmd=(git diff --no-color --cached -- "$p")
      fi
      ;;
  esac
  local diff_out
  diff_out="$("${diffcmd[@]}" 2>/dev/null || true)"
  if [[ -z "$diff_out" ]]; then
    # No diff (e.g. untracked file in worktree scope, or HEAD~1 missing) —
    # treat the whole file as added.
    grep -q 'bufio\.NewScanner(' "$p" 2>/dev/null && return 0
    return 1
  fi
  # True iff an ADDED diff line (starts with a single '+', i.e. NOT the '+++'
  # file header) introduces a bufio.NewScanner( invocation. awk avoids the ERE
  # '+'-quantifier portability trap that a `grep '^\+\+\+'` form hits on BSD grep.
  printf '%s\n' "$diff_out" | awk '
    /^\+\+\+/ { next }                        # skip the +++ file header
    /^\+/ && /bufio\.NewScanner\(/ { found=1 } # an added line invoking the scanner
    END { exit found ? 0 : 1 }
  '
}

rc=0
new_hits=()

# Grandfather growth guard first: a same-diff self-allowlist must fail even
# before (and independently of) the per-file scan.
if ! check_grandfather_shrink_only; then
  rc=1
fi

while IFS= read -r f; do
  [[ -z "$f" ]] && continue
  is_exempt_path "$f" && continue
  is_grandfathered "$f" && continue
  file_trips "$f" || continue
  added_hunk_has_scanner "$f" || continue
  new_hits+=("$f")
done < <(collect_changed_files | LC_ALL=C sort -u)

if [[ "${#new_hits[@]}" -gt 0 ]]; then
  rc=1
  echo "ADVISORY: new raw bufio.NewScanner over JSONL introduced outside cli/internal/storage:" >&2
  for f in "${new_hits[@]}"; do
    echo "  - $f" >&2
  done
  echo "  Use storage.ScanJSONL / storage.ScanJSONLFile (loud storage.ErrLineTooLong policy)" >&2
  echo "  instead of a raw bufio.NewScanner over a JSONL stream — a raw scanner silently" >&2
  echo "  truncates lines at the 64KB default buffer. See age-storage-hardening-roxg.3." >&2
  echo "  (Advisory heuristic — file-level, no AST — and the grandfather list only shrinks:" >&2
  echo "   a new site cannot be allowlisted. A genuine false positive costs this warning" >&2
  echo "   only — the gate is non-blocking by design.)" >&2
fi

# SHRINK RATCHET: a grandfathered file that no longer trips the whole-file
# heuristic (migrated to storage.ScanJSONL, or deleted) must be pruned.
stale=()
for g in "${GRANDFATHER[@]}"; do
  if ! file_trips "$g"; then
    stale+=("$g")
  fi
done
if [[ "${#stale[@]}" -gt 0 ]]; then
  rc=1
  echo "FAIL: grandfathered files no longer trip the heuristic — prune them (the list only shrinks):" >&2
  for g in "${stale[@]}"; do
    echo "  - $g" >&2
  done
  echo "  Regenerate: bash scripts/check-jsonl-scanner-ratchet.sh --regenerate" >&2
fi

if [[ "$rc" -eq 0 ]]; then
  echo "PASS: jsonl-scanner ratchet (scope=$SCOPE, grandfathered=${#GRANDFATHER[@]}, no new sites, no stale grandfather)"
fi
exit "$rc"
