#!/usr/bin/env bash
# shellcheck shell=bash
# pawl-evidence-dir.sh — sourceable: resolve where pawl review evidence lives
# (age-pawl-evidence-worktree-loss-np1e).
#
# THE loss class (2026-07-09): reviews run from a disposable land worktree wrote
# transcripts to <worktree>/.agents/pawl-evidence/ and the landed provenance
# ledger recorded that absolute path; `git worktree remove` after the push
# destroyed the refute transcripts (the membrane's most valuable output) and
# left every landed evidence_path dangling. Evidence must live in the CANONICAL
# checkout — the git-common-dir's parent — which is the same directory for a
# normal checkout and the main checkout for any linked worktree.
#
# pawl_evidence_dir <repo_root>
#   prints the evidence directory (no trailing slash), returns 0.
#   Honors AGENTOPS_PAWL_EVIDENCE_DIR as an explicit override (tests, alt rigs).
#   A non-git <repo_root> falls back to <repo_root>/.agents/pawl-evidence.
pawl_evidence_dir() {
  local repo="${1:?repo_root}"
  if [[ -n "${AGENTOPS_PAWL_EVIDENCE_DIR:-}" ]]; then
    printf '%s\n' "$AGENTOPS_PAWL_EVIDENCE_DIR"
    return 0
  fi
  printf '%s\n' "$(pawl_canonical_root "$repo")/.agents/pawl-evidence"
}

# pawl_canonical_root <repo_root>
#   prints the canonical checkout root: the git-common-dir's parent — the same
#   directory for a normal checkout, the MAIN checkout for a linked worktree.
#   A non-git <repo_root> prints itself. Used for every per-review artifact
#   that must outlive a disposable worktree (evidence transcripts, membrane
#   catches).
pawl_canonical_root() {
  local repo="${1:?repo_root}" common
  common="$(git -C "$repo" rev-parse --path-format=absolute --git-common-dir 2>/dev/null || true)"
  if [[ -n "$common" ]]; then
    dirname "$common"
  else
    printf '%s\n' "$repo"
  fi
}
