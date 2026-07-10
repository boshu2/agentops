# shellcheck shell=bash
# scripts/lib/ratchet.sh — shared shrink-only pinned-list ratchet mechanics
# (age-ratchet-lib-extraction-bv7d.1).
#
# WHY: seven in-tree gates each hand-rolled the same grandfather/baseline
# machinery (~300 duplicated lines; lineage explicit — check-jsonl-scanner-
# ratchet.sh says its base-ref logic was "ported from" check-new-scripts-use-
# preamble.sh). This lib is the one implementation: a heuristic detector runs
# over a scope; every CURRENT violator is pinned by name in a checked-in list;
# NEW violations fail the gate; the list only SHRINKS. Only a gate changes
# behavior — this is the repo's own law, extracted.
#
# DESIGN RULES (pre-mortem hardened, .agents/council/2026-07-10-pre-mortem-ratchet-lib.md):
#   * Functions only — no top-level `set -e`/`set -u`, no preamble source
#     (libs are strict-mode-agnostic by convention; see lib/bats-common.bash).
#     Callers own strict mode and REPO_ROOT anchoring.
#   * Parse modes are PER-CONSUMER parameters, never a universal superset:
#     a migration must select the consuming gate's ORIGINAL parsing so its
#     behavior does not change. Modes: raw | cr-strip | strip | trailing-comment.
#   * The growth guard (shrink-only + intersection authority) is ON by default
#     for new gates; a migrated Family-B/C gate that never had it opts out with
#     RATCHET_GROWTH_GUARD=off — flipping a shipped gate's semantics is its own
#     arc, never a migration side effect.
#
# SCOPE LIMIT — READ THIS, DO NOT PRETEND COMPLETENESS
# ----------------------------------------------------
# This is BASH GLUE OVER GIT, not a verified kernel. "Fail-closed" here is a
# BOUNDED claim over the enumerated classes below, each pinned by a case in
# tests/scripts/ratchet-lib.bats — it is NOT a universal guarantee that no
# execution path anywhere can yield an empty result with rc 0. Bash subprocess
# composition cannot express that property; a consumer needing it belongs in
# the Go twin (`ao lint ratchet`, deferred until >=3 migrations per sweep N1).
#
#   FAIL-CLOSED (rc 2, test-pinned):
#     * invalid parse modes and scopes at every entry point;
#     * unresolvable base refs (sole exception: first-parent-of-root, the
#       initial-commit state the source gates accommodate);
#     * a base-ref read failure on a path that EXISTS at that ref;
#     * unreadable pinned files on any guard path (set arithmetic, growth
#       guard, intersection authority);
#     * every CHOSEN collection command inside the changed-scope collectors —
#       explicit per-command `|| return 2` capture (a `set -e` subshell body
#       CANNOT provide this: `-e` is suppressed in tested contexts — see the
#       ERROR POSTURE note at ratchet_changed_files), each class pinned by a
#       shim or edge-state test;
#     * generation failure in regenerate (atomic tmp+mv; never a truncated list).
#   HANDLED GIT-HISTORY SHAPES: root commits (--root) and merge commits
#     (-m --first-parent = "what this commit introduced onto the first-parent
#     line") in head scope.
#   BEST-EFFORT (documented, deliberately out of scope):
#     * I/O failure of the fixed-program parse filters (sed/grep over in-memory
#       streams) — grep rc>=2 is loud, but a sed failure masked by a downstream
#       legal-empty grep is accepted;
#     * auto-scope decision probes are tolerant BY DESIGN (a probe failure
#       selects a branch; the chosen command still carries its explicit
#       fail-loud handler);
#     * added-hunk fallback treats a diff-less path as entirely added and
#       matches whole-file — the fail-closed DIRECTION (over-matching);
#     * git states not enumerated above (e.g. concurrent ref mutation).
# The seven source gates carry this same honesty discipline in their own
# headers ("treat a finding as a prompt to look, not a proof" —
# check-jsonl-scanner-ratchet.sh:26-41). Bounding the claim honestly is a
# standing review convergence in this repo; pretending completeness is not.
#
# CALLER CONTRACT:
#   ratchet_load_base <pinned-file> <base-ref>   # once, before guard/authority fns
#   ...then any of the functions below. All paths are relative to the caller's
#   CWD (gates cd "$REPO_ROOT" first). Offender sets are newline-delimited
#   paths on stdin unless stated otherwise.
#
# Exit vocabulary (per-function): 0 = pass/true · 1 = finding/false · 2 = usage.

# --- entry parsing -----------------------------------------------------------

# ratchet__require_mode <parse-mode> -> rc 2 + message on an unknown mode.
# EVERY mode-taking entry point validates BEFORE doing set arithmetic: an
# invalid mode inside a process substitution otherwise yields empty streams and
# a guard that silently passes — the fail-open class the pawl refuted three
# rounds running (2026-07-10).
ratchet__require_mode() {
  case "${1:-}" in
    raw|cr-strip|strip|trailing-comment) return 0 ;;
  esac
  echo "ratchet: unknown parse-mode '${1:-}' (want raw|cr-strip|strip|trailing-comment)" >&2
  return 2
}

# ratchet_strip_data <parse-mode>  (stdin -> stdout)
# Emits DATA lines only: full-line `#` comments and blank lines are dropped in
# every mode; what happens to the entry text differs per mode (see header).
ratchet_strip_data() {
  local mode="${1:?ratchet_strip_data: parse-mode required}"
  # grep rc 1 (no data lines) is a legal empty result; any harder failure
  # (rc >= 2) is loud — same no-ambient-strict-mode rationale as the
  # changed-scope collectors.
  case "$mode" in
    raw)
      grep -vE '^#|^[[:space:]]*$' || [ $? -eq 1 ] || return 2
      ;;
    cr-strip)
      sed 's/\r$//' | grep -vE '^#|^[[:space:]]*$' || [ $? -eq 1 ] || return 2
      ;;
    strip)
      sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' | grep -vE '^#|^$' || [ $? -eq 1 ] || return 2
      ;;
    trailing-comment)
      # Strip a trailing ` # comment` (and a full-line comment), then trim.
      sed -e 's/[[:space:]]#.*$//' -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' \
        | grep -vE '^#|^$' || [ $? -eq 1 ] || return 2
      ;;
    *)
      ratchet__require_mode "$mode" || return 2
      ;;
  esac
  return 0
}

# ratchet_load_pinned <file> <parse-mode> -> data lines on stdout.
# A missing pinned file emits nothing and succeeds (callers decide whether
# absence is an initial-snapshot state or an error).
ratchet_load_pinned() {
  local file="${1:?ratchet_load_pinned: pinned-file required}"
  local mode="${2:?ratchet_load_pinned: parse-mode required}"
  ratchet__require_mode "$mode" || return 2
  [[ -f "$file" ]] || return 0
  ratchet_strip_data "$mode" < "$file"
}

# --- set arithmetic ----------------------------------------------------------

# ratchet_new_violations <pinned-file> <parse-mode>   (offenders on stdin)
# Emits offenders NOT present in the working pinned file (set difference).
ratchet_new_violations() {
  local file="${1:?ratchet_new_violations: pinned-file required}"
  local mode="${2:?ratchet_new_violations: parse-mode required}"
  ratchet__require_mode "$mode" || return 2
  local pinned
  pinned="$(ratchet_load_pinned "$file" "$mode")" \
    || { echo "ratchet_new_violations: cannot read pinned file '$file' — refusing to certify" >&2; return 2; }
  comm -23 \
    <(LC_ALL=C sort -u) \
    <(printf '%s\n' "$pinned" | LC_ALL=C sort -u | grep -v '^$' || [ $? -eq 1 ])
}

# ratchet_stale_entries <pinned-file> <parse-mode>   (offenders on stdin)
# Emits pinned entries NOT present in the current offender set — the default
# stale predicate ("no longer trips the detector"), used by 6 of 7 gates.
ratchet_stale_entries() {
  local file="${1:?ratchet_stale_entries: pinned-file required}"
  local mode="${2:?ratchet_stale_entries: parse-mode required}"
  ratchet__require_mode "$mode" || return 2
  local pinned
  pinned="$(ratchet_load_pinned "$file" "$mode")" \
    || { echo "ratchet_stale_entries: cannot read pinned file '$file' — refusing to certify" >&2; return 2; }
  comm -13 \
    <(LC_ALL=C sort -u) \
    <(printf '%s\n' "$pinned" | LC_ALL=C sort -u | grep -v '^$' || [ $? -eq 1 ])
}

# ratchet_stale_entries_by <predicate-fn> <pinned-file> <parse-mode>
# Emits pinned entries for which <predicate-fn ENTRY> returns nonzero — the
# parameterized stale predicate (scenario-linkage keeps exists-only semantics).
ratchet_stale_entries_by() {
  local pred="${1:?ratchet_stale_entries_by: predicate-fn required}"
  local file="${2:?ratchet_stale_entries_by: pinned-file required}"
  local mode="${3:?ratchet_stale_entries_by: parse-mode required}"
  ratchet__require_mode "$mode" || return 2
  local pinned entry
  pinned="$(ratchet_load_pinned "$file" "$mode")" \
    || { echo "ratchet_stale_entries_by: cannot read pinned file '$file' — refusing to certify" >&2; return 2; }
  while IFS= read -r entry; do
    [[ -n "$entry" ]] || continue
    "$pred" "$entry" || printf '%s\n' "$entry"
  done <<< "$pinned"
  return 0
}

# --- base-ref snapshot + growth guard + intersection authority ----------------

# Module state set by ratchet_load_base (mirrors the Family-A globals).
RATCHET_BASE_EXISTS=0
RATCHET_BASE_LINES=""

# ratchet_load_base <pinned-file> <base-ref>
# Loads the base-ref snapshot of the pinned file (git show <ref>:<file>).
# Legal no-base states (guard stands down): the ref resolves but the file is
# absent there (initial snapshot), or the ref is the first parent of a ROOT
# commit (e.g. HEAD^ on the first commit). An UNRESOLVABLE ref otherwise is a
# loud rc=2 — silently standing down the growth guard on a caller typo would
# disable the fail-closed property (pawl refute, 2026-07-10).
ratchet_load_base() {
  local file="${1:?ratchet_load_base: pinned-file required}"
  local ref="${2:?ratchet_load_base: base-ref required}"
  RATCHET_BASE_EXISTS=0
  RATCHET_BASE_LINES=""
  if git rev-parse --verify --quiet "$ref^{commit}" >/dev/null 2>&1; then
    # Distinguish "file absent at the base ref" (legal initial snapshot) from
    # any other read failure (corruption, I/O): only the former may stand the
    # guard down (pawl refute round 2, 2026-07-10).
    local ls_out
    if ! ls_out="$(git ls-tree "$ref" -- "$file" 2>&1)"; then
      echo "ratchet_load_base: cannot read '$file' at base ref '$ref' (${ls_out}) — refusing to stand down the growth guard" >&2
      return 2
    fi
    if [[ -z "$ls_out" ]]; then
      return 0
    fi
    if ! RATCHET_BASE_LINES="$(git show "$ref:$file" 2>/dev/null)"; then
      echo "ratchet_load_base: '$file' exists at base ref '$ref' but cannot be read — refusing to stand down the growth guard" >&2
      return 2
    fi
    RATCHET_BASE_EXISTS=1
    return 0
  fi
  # Ref does not resolve. The one legal shape: first-parent-of-root (<stem>^ or
  # <stem>~1 where <stem> resolves to a parentless commit) — the initial-commit
  # state the source gates accommodate.
  local stem="$ref"
  stem="${stem%^}"
  stem="${stem%~1}"
  if [[ "$stem" != "$ref" ]] && git rev-parse --verify --quiet "$stem^{commit}" >/dev/null 2>&1; then
    local parents
    parents="$(git rev-list --parents -n1 "$stem" 2>/dev/null | awk '{print $2}')"
    if [[ -z "$parents" ]]; then
      return 0
    fi
  fi
  echo "ratchet_load_base: base ref '$ref' does not resolve — refusing to stand down the growth guard" >&2
  return 2
}

# ratchet_base_ref <scope> -> the git ref holding the PRE-change snapshot for
# the scope, mirroring the Family-A gates' scope semantics exactly.
ratchet_base_ref() {
  local scope="${1:?ratchet_base_ref: scope required}"
  case "$scope" in
    head) echo "HEAD^" ;;
    staged|worktree) echo "HEAD" ;;
    upstream)
      local upstream_ref base
      upstream_ref="$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null || true)"
      if [[ -n "$upstream_ref" ]]; then
        if ! base="$(git merge-base HEAD "$upstream_ref" 2>/dev/null)" || [[ -z "$base" ]]; then
          echo "ratchet_base_ref: no merge-base with upstream '$upstream_ref'" >&2
          return 2
        fi
        printf '%s\n' "$base"
      else
        echo "HEAD^"
      fi
      ;;
    auto)
      if [[ -n "$(git diff --cached --name-only 2>/dev/null || true)" || -n "$(git diff --name-only 2>/dev/null || true)" ]]; then
        echo "HEAD"
      else
        echo "HEAD^"
      fi
      ;;
    *)
      echo "ratchet_base_ref: unknown scope '$scope' (want head|staged|worktree|upstream|auto)" >&2
      return 2
      ;;
  esac
}

# ratchet_assert_shrink_only <pinned-file> <parse-mode>
# FAILS (rc 1) when a DATA line was ADDED to the pinned file relative to the
# base-ref snapshot loaded by ratchet_load_base — the list only SHRINKS. The
# one legal addition is the INITIAL snapshot (no file at the base ref).
# Added entries are echoed to stdout for the caller's message.
# RATCHET_GROWTH_GUARD=off disables the check (migrated-gate escape ONLY).
ratchet_assert_shrink_only() {
  local file="${1:?ratchet_assert_shrink_only: pinned-file required}"
  local mode="${2:?ratchet_assert_shrink_only: parse-mode required}"
  ratchet__require_mode "$mode" || return 2
  [[ "${RATCHET_GROWTH_GUARD:-on}" == "off" ]] && return 0
  [[ -f "$file" ]] || return 0
  [[ "$RATCHET_BASE_EXISTS" -eq 1 ]] || return 0
  local base_data work_data added
  base_data="$(printf '%s\n' "$RATCHET_BASE_LINES" | ratchet_strip_data "$mode")" \
    || { echo "ratchet_assert_shrink_only: cannot parse base snapshot — refusing to certify" >&2; return 2; }
  work_data="$(ratchet_load_pinned "$file" "$mode")" \
    || { echo "ratchet_assert_shrink_only: cannot read pinned file '$file' — refusing to certify" >&2; return 2; }
  added="$(comm -13 \
    <(printf '%s\n' "$base_data" | LC_ALL=C sort -u | grep -v '^$' || [ $? -eq 1 ]) \
    <(printf '%s\n' "$work_data" | LC_ALL=C sort -u | grep -v '^$' || [ $? -eq 1 ]))"
  if [[ -n "$added" ]]; then
    printf '%s\n' "$added"
    return 1
  fi
  return 0
}

# ratchet_is_pinned <entry> <pinned-file> <parse-mode>
# Intersection AUTHORITY: the entry protects only if present in BOTH the
# working pinned file AND the base-ref snapshot (when one exists). The working
# file alone is attacker-controlled — a same-diff self-allowlist grants NO
# protection. With no base snapshot, the working file is the cutoff and stands
# alone.
ratchet_is_pinned() {
  local entry="${1:?ratchet_is_pinned: entry required}"
  local file="${2:?ratchet_is_pinned: pinned-file required}"
  local mode="${3:?ratchet_is_pinned: parse-mode required}"
  ratchet__require_mode "$mode" || return 2
  local work_data base_data
  work_data="$(ratchet_load_pinned "$file" "$mode")" \
    || { echo "ratchet_is_pinned: cannot read pinned file '$file'" >&2; return 2; }
  printf '%s\n' "$work_data" | grep -qxF -- "$entry" || return 1
  if [[ "$RATCHET_BASE_EXISTS" -eq 1 ]]; then
    base_data="$(printf '%s\n' "$RATCHET_BASE_LINES" | ratchet_strip_data "$mode")" \
      || { echo "ratchet_is_pinned: cannot parse base snapshot" >&2; return 2; }
    printf '%s\n' "$base_data" | grep -qxF -- "$entry" || return 1
  fi
  return 0
}

# --- changed-scope collection + added-hunk guard -------------------------------

# ratchet_changed_files <scope> -> changed paths (name-only) for the scope.
# worktree includes untracked files (brand-new additions). auto includes
# untracked ONLY on the tracked-diff branch — a worktree with ONLY untracked
# changes falls back to HEAD and the untracked files are not listed. That edge
# is INHERITED BY PARITY from the source gates (preamble :95-104, jsonl
# :263-272): migrations must not change shipped-gate semantics, and the edge is
# benign in gate usage (an untracked script is governed at head scope on
# commit). New gates wanting untracked visibility use scope=worktree.
# ERROR POSTURE (pawl rounds 4-8): the source gates run under `set -euo
# pipefail`, so their `|| true` escape valves were sound — any unexpected git
# failure aborted the whole script. This lib runs in the CALLER's shell with no
# ambient strict mode, so a swallowed git failure here would return an EMPTY
# changeset and let a gate certify an unchecked diff. Every CHOSEN collection
# command therefore carries an explicit `|| return 2` handler; only the
# auto-scope decision PROBES stay tolerant (a probe failure just picks a
# branch — the chosen command still fails loud).
# WHY NOT a `(set -euo pipefail; …)` body with one boundary handler: bash
# suppresses `-e` inside ANY command in a tested context, and the `(...) ||
# handler` shape IS a tested context — the strict-mode body silently degrades
# back to fail-open (verified live: the unrelated-upstream bats case went red
# on exactly that rewrite, 2026-07-10). Explicit per-command handlers are the
# only shape bash honors here; the bats shim/edge cases pin each one.
ratchet_changed_files() {
  local scope="${1:?ratchet_changed_files: scope required}"
  case "$scope" in
    head)     git diff-tree --root -m --first-parent --no-commit-id --name-only -r HEAD 2>/dev/null \
                || { echo "ratchet_changed_files: git diff-tree HEAD failed — refusing to certify an unchecked change set" >&2; return 2; } ;;
    staged)   git diff --cached --name-only 2>/dev/null \
                || { echo "ratchet_changed_files: git diff --cached failed" >&2; return 2; } ;;
    worktree) git diff --name-only 2>/dev/null \
                || { echo "ratchet_changed_files: git diff failed" >&2; return 2; }
              git ls-files --others --exclude-standard 2>/dev/null \
                || { echo "ratchet_changed_files: git ls-files failed" >&2; return 2; } ;;
    upstream)
      local upstream_ref base
      upstream_ref="$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null || true)"
      if [[ -n "$upstream_ref" ]]; then
        if ! base="$(git merge-base HEAD "$upstream_ref" 2>/dev/null)" || [[ -z "$base" ]]; then
          echo "ratchet_changed_files: no merge-base with upstream '$upstream_ref' — refusing to certify an unchecked change set" >&2
          return 2
        fi
        git diff --name-only "$base"...HEAD 2>/dev/null \
          || { echo "ratchet_changed_files: git diff vs upstream merge-base failed" >&2; return 2; }
      else
        git diff-tree --root -m --first-parent --no-commit-id --name-only -r HEAD 2>/dev/null \
          || { echo "ratchet_changed_files: git diff-tree HEAD failed" >&2; return 2; }
      fi
      ;;
    auto)
      if [[ -n "$(git diff --cached --name-only 2>/dev/null || true)" ]]; then
        git diff --cached --name-only 2>/dev/null \
          || { echo "ratchet_changed_files: git diff --cached failed" >&2; return 2; }
      elif [[ -n "$(git diff --name-only 2>/dev/null || true)" ]]; then
        git diff --name-only 2>/dev/null \
          || { echo "ratchet_changed_files: git diff failed" >&2; return 2; }
        git ls-files --others --exclude-standard 2>/dev/null \
          || { echo "ratchet_changed_files: git ls-files failed" >&2; return 2; }
      else
        git diff-tree --root -m --first-parent --no-commit-id --name-only -r HEAD 2>/dev/null \
          || { echo "ratchet_changed_files: git diff-tree HEAD failed" >&2; return 2; }
      fi
      ;;
    *)
      echo "ratchet_changed_files: unknown scope '$scope'" >&2
      return 2
      ;;
  esac
  return 0
}

# ratchet_changed_files_status <scope> -> "<STATUS>\t<path>" lines (A/M/R...),
# untracked emitted as "A\t<path>" — the name-status variant Family A's
# preamble gate consumes (renames keep git's R<score>\told\tnew shape).
# Same inherited auto/only-untracked edge as ratchet_changed_files above.
ratchet_changed_files_status() {
  local scope="${1:?ratchet_changed_files_status: scope required}"
  case "$scope" in
    head)     git diff-tree --root -m --first-parent --no-commit-id --name-status -r HEAD 2>/dev/null \
                || { echo "ratchet_changed_files_status: git diff-tree HEAD failed — refusing to certify an unchecked change set" >&2; return 2; } ;;
    staged)   git diff --cached --name-status 2>/dev/null \
                || { echo "ratchet_changed_files_status: git diff --cached failed" >&2; return 2; } ;;
    worktree) git diff --name-status 2>/dev/null \
                || { echo "ratchet_changed_files_status: git diff failed" >&2; return 2; }
              local untracked_wt
              untracked_wt="$(git ls-files --others --exclude-standard 2>/dev/null)" \
                || { echo "ratchet_changed_files_status: git ls-files failed" >&2; return 2; }
              if [[ -n "$untracked_wt" ]]; then
                printf '%s\n' "$untracked_wt" | awk '{ printf "A\t%s\n", $0 }'
              fi ;;
    upstream)
      local upstream_ref base
      upstream_ref="$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null || true)"
      if [[ -n "$upstream_ref" ]]; then
        if ! base="$(git merge-base HEAD "$upstream_ref" 2>/dev/null)" || [[ -z "$base" ]]; then
          echo "ratchet_changed_files_status: no merge-base with upstream '$upstream_ref' — refusing to certify an unchecked change set" >&2
          return 2
        fi
        git diff --name-status "$base"...HEAD 2>/dev/null \
          || { echo "ratchet_changed_files_status: git diff vs upstream merge-base failed" >&2; return 2; }
      else
        git diff-tree --root -m --first-parent --no-commit-id --name-status -r HEAD 2>/dev/null \
          || { echo "ratchet_changed_files_status: git diff-tree HEAD failed" >&2; return 2; }
      fi
      ;;
    auto)
      if [[ -n "$(git diff --cached --name-only 2>/dev/null || true)" ]]; then
        git diff --cached --name-status 2>/dev/null \
          || { echo "ratchet_changed_files_status: git diff --cached failed" >&2; return 2; }
      elif [[ -n "$(git diff --name-only 2>/dev/null || true)" ]]; then
        git diff --name-status 2>/dev/null \
          || { echo "ratchet_changed_files_status: git diff failed" >&2; return 2; }
        local untracked_auto
        untracked_auto="$(git ls-files --others --exclude-standard 2>/dev/null)" \
          || { echo "ratchet_changed_files_status: git ls-files failed" >&2; return 2; }
        if [[ -n "$untracked_auto" ]]; then
          printf '%s\n' "$untracked_auto" | awk '{ printf "A\t%s\n", $0 }'
        fi
      else
        git diff-tree --root -m --first-parent --no-commit-id --name-status -r HEAD 2>/dev/null \
          || { echo "ratchet_changed_files_status: git diff-tree HEAD failed" >&2; return 2; }
      fi
      ;;
    *)
      echo "ratchet_changed_files_status: unknown scope '$scope'" >&2
      return 2
      ;;
  esac
  return 0
}

# ratchet_added_hunk_matches <scope> <path> <ere>
# 0 iff an ADDED diff line for <path> in <scope> matches <ere> — the
# changed-content guard: editing a file without adding a matching line does
# not re-flag it. A path with NO diff (untracked in worktree scope, missing
# HEAD~1) is treated as entirely added and matched whole-file.
ratchet_added_hunk_matches() {
  local scope="${1:?ratchet_added_hunk_matches: scope required}"
  local p="${2:?ratchet_added_hunk_matches: path required}"
  local ere="${3:?ratchet_added_hunk_matches: ere required}"
  local diffcmd
  case "$scope" in
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
    *)
      echo "ratchet_added_hunk_matches: unknown scope '$scope'" >&2
      return 2
      ;;
  esac
  local diff_out
  diff_out="$("${diffcmd[@]}" 2>/dev/null || true)"
  if [[ -z "$diff_out" ]]; then
    grep -Eq -- "$ere" "$p" 2>/dev/null && return 0
    return 1
  fi
  # True iff an ADDED line (single leading '+', not the '+++' header) matches.
  # awk sidesteps the BSD-grep '+'-quantifier portability trap. The ERE rides
  # ENVIRON, not -v: -v applies C-escape processing that corrupts patterns
  # like 'os\.Rename\(' (the backslashes are load-bearing).
  printf '%s\n' "$diff_out" | RATCHET_HUNK_ERE="$ere" awk '
    BEGIN { ere = ENVIRON["RATCHET_HUNK_ERE"] }
    /^\+\+\+/ { next }
    /^\+/ { if (substr($0, 2) ~ ere) found = 1 }
    END { exit found ? 0 : 1 }
  '
}

# --- regenerate ----------------------------------------------------------------

# ratchet_regenerate <pinned-file> <header-fn> <entries-fn>
# Rewrites the pinned file: <header-fn> prints the `#` header (purpose, blessed
# replacement, bead id, regenerate command, the "list only SHRINKS" line);
# <entries-fn> prints the current offender set (any order — the lib sorts
# LC_ALL=C, the regenerate-at-land-time doctrine's canonical order).
# Fail-closed + atomic: both generator functions must exist and succeed, and
# the pinned file is replaced via tmp+mv so a generation failure can never
# leave a truncated list behind (a truncated grandfather would spuriously
# fail-or-pass every consumer of it).
ratchet_regenerate() {
  local file="${1:?ratchet_regenerate: pinned-file required}"
  local header_fn="${2:?ratchet_regenerate: header-fn required}"
  local entries_fn="${3:?ratchet_regenerate: entries-fn required}"
  local fn
  for fn in "$header_fn" "$entries_fn"; do
    if ! declare -F "$fn" >/dev/null 2>&1; then
      echo "ratchet_regenerate: '$fn' is not a function — pinned file left untouched" >&2
      return 2
    fi
  done
  local hdr entries tmp
  if ! hdr="$("$header_fn")"; then
    echo "ratchet_regenerate: header-fn '$header_fn' failed — pinned file left untouched" >&2
    return 2
  fi
  if ! entries="$("$entries_fn")"; then
    echo "ratchet_regenerate: entries-fn '$entries_fn' failed — pinned file left untouched" >&2
    return 2
  fi
  tmp="$(mktemp "${file}.XXXXXX")" || return 2
  {
    printf '%s\n' "$hdr"
    if [[ -n "$entries" ]]; then
      printf '%s\n' "$entries" | LC_ALL=C sort
    fi
  } > "$tmp" || { rm -f "$tmp"; return 2; }
  mv "$tmp" "$file"
}
