#!/usr/bin/env bash
# check-new-scripts-use-preamble.sh — preamble adoption ratchet (age-gate-the-ungated-egwt.10).
#
# WHY: scripts/lib/preamble.sh centralizes strict mode + a CWD-hijack-proof
# REPO_ROOT + portable stat/find + with_tmpdir/require_cmd — each helper is an
# already-paid-for bug class. Yet it had ZERO adopters and every script added
# after the opportunistic-adoption decision re-hand-rolled the preamble. A doc
# instruction is measured-inert here; only a gate changes behavior.
#
# WHAT (zero-churn ratchet — never rewrites the 300+ existing scripts):
#   For each ADDED or MODIFIED top-level `scripts/*.sh` in the changed scope
#   that is NOT on the grandfather snapshot, require it to either
#     * source scripts/lib/preamble.sh (any recognizable form), OR
#     * carry a `# preamble-exempt: <reason>` line (non-empty reason).
#   Grandfathered scripts are exempt UNTIL they adopt preamble.sh — at which
#   point they must be PRUNED from the snapshot (the allowlist only shrinks).
#
# SCOPE: top-level `scripts/*.sh` only. `scripts/lib/**` and other `scripts/<sub>/**`
# are sourced library fragments / sub-tool entrypoints, not the top-level entry
# scripts this ratchet governs — and the preamble is itself a lib (it cannot
# source itself). Libs are EXEMPT AS A CLASS by construction, so they are never
# scanned and never appear on the grandfather list.
#
# Usage:
#   bash scripts/check-new-scripts-use-preamble.sh                 # scope: auto
#   bash scripts/check-new-scripts-use-preamble.sh --scope head    # push scope
#   bash scripts/check-new-scripts-use-preamble.sh --scope worktree
#
# Exit codes:
#   0 - pass / not applicable
#   1 - a governed script neither sources preamble nor is exempt/grandfathered,
#       OR a grandfathered script now sources preamble (snapshot must shrink),
#       OR the grandfather snapshot GAINED entries vs. its base-ref version
#       (allowlist additions are rejected — only the initial snapshot may add)

# Dogfood: this gate sources the very preamble it enforces (strict mode +
# CWD-hijack-proof REPO_ROOT). That is also why it is NOT on the grandfather list.
# `CDPATH=` is an intentional env-prefix (clears CDPATH for that one cd), not an
# assignment — hence the SC1007 disable, matching scripts/lib/preamble.sh.
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

cd "$REPO_ROOT" || exit 1

# Shared shrink-only ratchet mechanics (this gate's grandfather machinery was
# the ORIGINAL the jsonl gate ported; both now delegate to one lib) —
# age-ratchet-lib-extraction-bv7d.4. Parse mode raw = original entry parsing.
. "$REPO_ROOT/scripts/lib/ratchet.sh"

GRANDFATHER="scripts/.preamble-grandfather"
SCOPE="auto"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --scope)
      shift
      [[ $# -gt 0 ]] || { echo "--scope requires a value" >&2; exit 2; }
      SCOPE="$1"
      ;;
    --scope=*) SCOPE="${1#--scope=}" ;;
    -h|--help) sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "Unknown arg: $1" >&2; exit 2 ;;
  esac
  shift
done

case "$SCOPE" in
  head|staged|worktree|upstream|auto) ;;
  *) echo "Invalid --scope: $SCOPE (want head|staged|worktree|upstream|auto)" >&2; exit 2 ;;
esac

# collect_changed_files: emit "<STATUS>\t<path>" lines for the requested scope
# (shared lib; same derivation regen-changed-scope.sh uses). STATUS is git's
# name-status letter (A/M/R/...); we treat A and M (and R's new path) as "in
# scope" — a deletion (D) is never governed (the file is gone).
collect_changed_files() {
  ratchet_changed_files_status "$SCOPE"
}

# Base-ref snapshot, intersection authority, and the shrink-only guard come
# from scripts/lib/ratchet.sh (this gate's hand-rolled versions were the
# original the lib preserves). The wrapper keeps this gate's message shape.
check_grandfather_shrink_only() {
  local added rc_=0
  added="$(ratchet_assert_shrink_only "$GRANDFATHER" raw)" || rc_=$?
  [[ "$rc_" -eq 2 ]] && exit 2
  if [[ "$rc_" -ne 0 ]]; then
    echo "FAIL: $GRANDFATHER gained new entries — the grandfather list only SHRINKS." >&2
    echo "      A new script cannot be allowlisted; it must source scripts/lib/preamble.sh" >&2
    echo "      or carry a '# preamble-exempt: <reason>' line. Added entries:" >&2
    while IFS= read -r line; do
      echo "        $line" >&2
    done <<< "$added"
    return 1
  fi
  return 0
}

is_grandfathered() {
  ratchet_is_pinned "$1" "$GRANDFATHER" raw
}

# sources_preamble PATH → 0 if the script sources scripts/lib/preamble.sh in any
# recognizable form. All forms — absolutized `$(... )/lib/preamble.sh`,
# `$SCRIPT_DIR/lib/preamble.sh`, a literal `scripts/lib/preamble.sh` — end in
# `lib/preamble.sh` on a `.`/`source` line, so we match that, ignoring comments.
sources_preamble() {
  local path="$1"
  [[ -f "$path" ]] || return 1
  # Strip leading whitespace; a source line begins with `.` or `source` and
  # references lib/preamble.sh. Exclude commented-out lines (leading `#`).
  grep -Eq '^[[:space:]]*(\.|source)[[:space:]]+.*lib/preamble\.sh' "$path"
}

# exempt_reason PATH → prints the non-empty reason if the script carries a
# `# preamble-exempt: <reason>` line, else prints nothing. A bare marker with an
# empty/whitespace-only reason is NOT accepted (anti cargo-cult).
exempt_reason() {
  local path="$1" line reason
  [[ -f "$path" ]] || return 0
  line="$(grep -m1 -E '^[[:space:]]*#[[:space:]]*preamble-exempt:' "$path" || true)"
  [[ -n "$line" ]] || return 0
  reason="${line#*preamble-exempt:}"
  # trim leading/trailing whitespace
  reason="${reason#"${reason%%[![:space:]]*}"}"
  reason="${reason%"${reason##*[![:space:]]}"}"
  printf '%s' "$reason"
}

# The exact source line we tell authors to paste. $SCRIPT_DIR-relative so it
# works from any CWD — the same hardening the lib's own header documents.
suggested_source_line='. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"'

tmp_changed="$(mktemp -d "${TMPDIR:-/tmp}/preamble-ratchet.XXXXXX")"
trap 'rm -rf "$tmp_changed"' EXIT

collect_changed_files > "$tmp_changed/raw" 2>/dev/null || true

missing=0        # governed scripts that neither source nor are exempt
shrink=0         # grandfathered scripts that now source preamble (must prune)
grow=0           # grandfather snapshot gained entries (allowlist only shrinks)

ratchet_load_base "$GRANDFATHER" "$(ratchet_base_ref "$SCOPE")" || exit 2
if ! check_grandfather_shrink_only; then
  grow=1
fi

while IFS=$'\t' read -r status path rest; do
  [[ -n "${status:-}" && -n "${path:-}" ]] || continue
  # Renames arrive as "R<score>\told\tnew"; git puts the NEW path in `rest`.
  if [[ "$status" == R* && -n "${rest:-}" ]]; then
    path="$rest"
    status="A"   # the new path is effectively an addition to govern
  fi
  # Only ADDED or MODIFIED files are governed. Deletions (D) are skipped.
  case "$status" in
    A|M) ;;
    *) continue ;;
  esac
  # Only TOP-LEVEL scripts/*.sh — never scripts/lib/** or scripts/<sub>/**.
  case "$path" in
    scripts/*.sh) ;;
    *) continue ;;
  esac
  # Reject nested paths that slipped through the glob (scripts/x/y.sh).
  [[ "$path" == scripts/*/* ]] && continue
  # The file may have been deleted in the working tree even if status says A/M
  # (edge: staged-then-deleted). Skip anything not present.
  [[ -f "$path" ]] || continue

  if is_grandfathered "$path"; then
    # Shrink ratchet: a grandfathered script that NOW sources preamble must be
    # pruned from the snapshot — the allowlist only shrinks.
    if sources_preamble "$path"; then
      echo "FAIL: $path is on $GRANDFATHER but now sources scripts/lib/preamble.sh." >&2
      echo "      The grandfather list only SHRINKS. Remove this line from $GRANDFATHER:" >&2
      echo "        $path" >&2
      shrink=$((shrink + 1))
    fi
    # Grandfathered + not-yet-sourcing → still allowed, no action.
    continue
  fi

  # Not grandfathered → must source preamble OR carry a non-empty exempt reason.
  if sources_preamble "$path"; then
    continue
  fi
  reason="$(exempt_reason "$path")"
  if [[ -n "$reason" ]]; then
    continue
  fi

  # Distinguish "marker present but empty reason" from "no marker" for a precise
  # message, but both fail.
  if grep -Eq '^[[:space:]]*#[[:space:]]*preamble-exempt:' "$path"; then
    echo "FAIL: $path has a '# preamble-exempt:' marker with an EMPTY reason." >&2
    echo "      A non-empty reason is required (anti cargo-cult). Either give a real reason:" >&2
    echo "        # preamble-exempt: <why this script cannot source the preamble>" >&2
    echo "      or source the hardened preamble by pasting this near the top:" >&2
    echo "        $suggested_source_line" >&2
  else
    echo "FAIL: $path (new/changed) does not source scripts/lib/preamble.sh." >&2
    echo "      Paste this near the top of the script:" >&2
    echo "        $suggested_source_line" >&2
    echo "      or, if it genuinely cannot (e.g. a standalone bootstrap with no repo context), add:" >&2
    echo "        # preamble-exempt: <reason>" >&2
  fi
  missing=$((missing + 1))
done < "$tmp_changed/raw"

if [[ "$missing" -gt 0 || "$shrink" -gt 0 || "$grow" -gt 0 ]]; then
  echo "" >&2
  echo "preamble-ratchet: $missing script(s) need preamble/exemption; $shrink grandfather entry/entries must be pruned; snapshot-grew=$grow." >&2
  exit 1
fi

echo "PASS: preamble ratchet — all new/changed top-level scripts/*.sh source the preamble, are exempt, or are grandfathered."
exit 0
