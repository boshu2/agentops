#!/usr/bin/env bash
# check-atomic-write-ratchet.sh (age-ratchet-lib-extraction-bv7d.9) —
# ADVISORY changed-scope ratchet: no NEW hand-rolled tmp+rename atomic write
# outside cli/internal/storage.
#
# WHY
# ---
# The blessed atomic-write lives in cli/internal/storage: AtomicWriteFile
# (write temp, fsync, rename, fsync parent dir) and FsyncDir for sanctioned
# own-rename movers. 25 files across the CLI hand-roll the tmp+rename shape
# instead — most without the fsync, so a crash between rename and the dir
# flush can lose the write. Consolidation without a guard accretes new copies
# (the 07-01 escape re-rolled writeJSONAtomic fsync-less a week after the
# helper landed); three consecutive recon sweeps flagged the adoption gap.
# This ratchet stops NEW hand-rolled sites and applies shrink pressure on the
# grandfathered set. It is the first NEW consumer of scripts/lib/ratchet.sh —
# a gate at the cost of a detector + a pinned file.
#
# SCOPE LIMIT — READ THIS, DO NOT PRETEND COMPLETENESS
# ----------------------------------------------------
# This is a GREP HEURISTIC over source text, file-level, no AST:
#   * FALSE NEGATIVES: the rename and the temp-file signal must be in the SAME
#     file; a helper split across files is not caught; "renameio"-style
#     wrappers are not modeled.
#   * FALSE POSITIVES: a file that legitimately uses os.Rename for a MOVE and
#     separately mentions a temp path will be flagged; relocating an existing
#     rename inside a grandfathered file re-flags it (the added-hunk guard
#     sees a new + line).
# ACCEPTABLE because the gate is ADVISORY PERMANENTLY at this fidelity (same
# rationale as go.jsonl-scanner-ratchet, seed.go): a false positive costs this
# warning, never a blocked push. Graduation to blocking requires a
# function-level/fingerprint detector — a separately-earned arc, not this one.
#
# HEURISTIC (per changed cli/**/*.go file, outside cli/internal/storage/,
# excluding _test.go — test scratch files are not production crash-safety):
#   trips  ⟺  a NON-COMMENT line invokes `os.Rename(`
#             AND a NON-COMMENT line carries `os.CreateTemp(` OR a `".tmp`
#                 literal (two-signal AND; comment mentions NEVER count —
#                 the comment-strip rule, premortem r3)
#             AND (changed-content guard) an ADDED hunk introduces `os.Rename(`.
#   The added-hunk guard fires EVEN when the file is grandfathered: a NEW
#   rename site inside a grandfathered file still trips (premortem FM7 —
#   do not clone the grandfather-skips-first flow). Its ERE skips lines whose
#   os.Rename( sits after a // (comment-only additions never trip); an added
#   line inside a multi-line /* block */ cannot be classified from a diff
#   hunk and may false-trip — advisory, pre-declared.
#
# GRANDFATHER: scripts/.atomic-write-grandfather (FILENAME-pinned, seeded via
# --regenerate, hand-audited at seeding). Shrink-only with intersection
# authority + growth guard ON (a same-diff self-allowlist grants nothing) —
# this is a NEW gate, so it gets the lib's fail-closed default.
#
# USAGE
#   bash scripts/check-atomic-write-ratchet.sh                 # --scope head
#   bash scripts/check-atomic-write-ratchet.sh --scope worktree|staged|auto
#   bash scripts/check-atomic-write-ratchet.sh --regenerate
#
# EXIT: 0 pass/not-applicable · 1 advisory finding (new site / stale entry /
#       grandfather grew) · 2 usage/environment error.

# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"
cd "$REPO_ROOT" || exit 2
# Shared shrink-only ratchet mechanics (parse mode raw).
. "$REPO_ROOT/scripts/lib/ratchet.sh"

GRANDFATHER_FILE="scripts/.atomic-write-grandfather"
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

# is_exempt_path PATH -> 0 (exempt) for anything outside the ratchet's scope:
# not a cli Go file, under cli/internal/storage/ (the blessed home), or a
# _test.go file (test scratch writes are not production crash-safety).
is_exempt_path() {
  local p="$1"
  case "$p" in
    cli/internal/storage/*) return 0 ;;
    *_test.go) return 0 ;;
    cli/*)
      [[ "$p" == *.go ]] && return 1
      return 0
      ;;
    *) return 0 ;;
  esac
}

# strip_go_comments (stdin -> stdout): remove // line tails and /* */ block
# comments before signal-grepping — a comment mention never counts (premortem
# r3; pawl refute extended it from full-line to inline/block forms). NOT a Go
# parser: a "//" or "/*" INSIDE a string literal is treated as a comment
# opener and truncates that line — acceptable for a two-signal grep and
# pre-declared here.
strip_go_comments() {
  awk '
    BEGIN { inblock = 0 }
    {
      line = $0; out = ""; i = 1; n = length(line)
      while (i <= n) {
        if (inblock) {
          if (substr(line, i, 2) == "*/") { inblock = 0; i += 2 } else { i++ }
          continue
        }
        two = substr(line, i, 2)
        if (two == "//") break
        if (two == "/*") { inblock = 1; i += 2; continue }
        out = out substr(line, i, 1); i++
      }
      print out
    }
  '
}

# file_trips PATH -> 0 if the WHOLE FILE trips: a non-comment os.Rename(
# invocation AND a non-comment temp-file signal (os.CreateTemp( or ".tmp
# literal), evaluated AFTER strip_go_comments.
file_trips() {
  local p="$1" stripped
  [[ -f "$p" ]] || return 1
  stripped="$(strip_go_comments < "$p")" || return 1
  printf '%s\n' "$stripped" | grep -q 'os\.Rename(' || return 1
  printf '%s\n' "$stripped" | grep -qE 'os\.CreateTemp\(|"\.tmp' || return 1
  return 0
}

# compute_grandfather_set -> every in-scope cli/**/*.go file currently
# tripping, sorted. Same heuristic as the flagger, so a fresh regenerate is
# authoritative for "what is grandfathered now".
compute_grandfather_set() {
  local f
  while IFS= read -r f; do
    is_exempt_path "$f" && continue
    file_trips "$f" || continue
    printf '%s\n' "$f"
  done < <(grep -rl 'os\.Rename(' cli --include='*.go' 2>/dev/null || true) | LC_ALL=C sort
}

grandfather_header() {
  echo "# scripts/.atomic-write-grandfather — FILENAME-pinned grandfather list."
  echo "#"
  echo "# Every cli/**/*.go file (outside cli/internal/storage/, excluding _test.go)"
  echo "# that currently has BOTH a non-comment os.Rename( invocation AND a"
  echo "# non-comment temp-file signal (os.CreateTemp( or \".tmp literal) — the"
  echo "# hand-rolled atomic-write shape. These predate the ratchet"
  echo "# (age-ratchet-lib-extraction-bv7d.9) and are exempt from the FILE-level"
  echo "# trip; a NEW os.Rename( hunk still trips even in a grandfathered file."
  echo "# The list only SHRINKS: a file that no longer trips must be pruned."
  echo "#"
  echo "# Blessed replacement: storage.AtomicWriteFile (temp, fsync, rename,"
  echo "# fsync-dir) / storage.FsyncDir for sanctioned own-rename movers."
  echo "#"
  echo "# Regenerate with:  bash scripts/check-atomic-write-ratchet.sh --regenerate"
  echo "# (regenerate at LAND time, after the final rebase; hand-audit the list.)"
}

if [[ "$MODE" == "regenerate" ]]; then
  ratchet_regenerate "$GRANDFATHER_FILE" grandfather_header compute_grandfather_set || exit 2
  echo "regenerated $GRANDFATHER_FILE ($(grep -cv '^#' "$GRANDFATHER_FILE" 2>/dev/null || echo 0) files)"
  exit 0
fi

# Load the grandfather set + its base-ref snapshot (intersection authority,
# growth guard ON — the lib's fail-closed default for NEW gates).
grandfather_data="$(ratchet_load_pinned "$GRANDFATHER_FILE" raw)" \
  || { echo "check-atomic-write-ratchet: cannot read $GRANDFATHER_FILE" >&2; exit 2; }
declare -a GRANDFATHER=()
while IFS= read -r line; do
  [[ -n "$line" ]] && GRANDFATHER+=("$line")
done <<< "$grandfather_data"

ratchet_load_base "$GRANDFATHER_FILE" "$(ratchet_base_ref "$SCOPE")" || exit 2

rc=0

# Growth guard first: a same-diff self-allowlist fails independently of the
# per-file scan.
added_entries=""
_shrink_rc=0
added_entries="$(ratchet_assert_shrink_only "$GRANDFATHER_FILE" raw)" || _shrink_rc=$?
if [[ "$_shrink_rc" -eq 2 ]]; then
  exit 2
elif [[ "$_shrink_rc" -ne 0 ]]; then
  rc=1
  echo "FAIL: $GRANDFATHER_FILE gained new entries — the grandfather list only SHRINKS." >&2
  echo "      A new site cannot be allowlisted; use storage.AtomicWriteFile /" >&2
  echo "      storage.FsyncDir instead. Added entries:" >&2
  while IFS= read -r line; do
    echo "        $line" >&2
  done <<< "$added_entries"
fi

new_hits=()
while IFS= read -r f; do
  [[ -z "$f" ]] && continue
  is_exempt_path "$f" && continue
  file_trips "$f" || continue
  # The added-hunk guard applies to EVERY tripping file — grandfathered or
  # not: a new rename site never rides an old exemption (premortem FM7).
  # It fires when the added hunk introduces EITHER half of the signature —
  # the rename OR the temp signal — because file_trips already requires both
  # in the file: adding a temp-writer that reuses an EXISTING rename is a new
  # hand-rolled atomic write too (pawl round 4). The ERE requires the match
  # with NO // earlier on the added line (comment-only additions never trip);
  # an added line inside a multi-line /* block */ cannot be classified from a
  # diff hunk and MAY false-trip — advisory, pre-declared.
  ratchet_added_hunk_matches "$SCOPE" "$f" '^([^/]|/[^/])*(os\.Rename\(|os\.CreateTemp\(|"\.tmp)' || continue
  new_hits+=("$f")
done < <(ratchet_changed_files "$SCOPE" | LC_ALL=C sort -u)

if [[ "${#new_hits[@]}" -gt 0 ]]; then
  rc=1
  echo "ADVISORY: new hand-rolled tmp+rename atomic write introduced outside cli/internal/storage:" >&2
  for f in "${new_hits[@]}"; do
    echo "  - $f" >&2
  done
  echo "  Use storage.AtomicWriteFile (write temp, fsync, rename, fsync parent dir) —" >&2
  echo "  or storage.FsyncDir after an own os.Rename move. A hand-rolled tmp+rename" >&2
  echo "  without the fsyncs can lose the write on crash. See age-ratchet-lib-extraction-bv7d.9." >&2
  echo "  (Advisory file-level heuristic; the grandfather list only shrinks — a new" >&2
  echo "   site cannot be allowlisted. A genuine false positive costs this warning only.)" >&2
fi

# SHRINK RATCHET: a grandfathered file that no longer trips must be pruned.
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
  echo "  Regenerate: bash scripts/check-atomic-write-ratchet.sh --regenerate" >&2
fi

if [[ "$rc" -eq 0 ]]; then
  echo "PASS: atomic-write ratchet (scope=$SCOPE, grandfathered=${#GRANDFATHER[@]}, no new sites, no stale grandfather)"
fi
exit "$rc"
