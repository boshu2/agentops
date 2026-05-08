#!/usr/bin/env bash
set -euo pipefail

# Enforce: repo-root .agents/ holds local/private agent runtime state and must
# not bleed into git, EXCEPT for an explicit audit-truth allowlist that
# compounds across nightly runs. The allowlist is intentionally narrow —
# baseline/final goal snapshots, evolve cycle history, per-goal attempt
# history, dream probe registry, findings registry, the live next-work
# queue, and the Reconciliation Engine arc's static thesis snapshot +
# operator decision records (.agents/reconcile/wave-0-thesis-snapshot.md,
# .agents/reconcile/thesis-stability-decision.md, and
# .agents/reconcile/promotion-decision.md — committed inputs to
# scripts/check-thesis-stability.sh and the Wave 1E gate-flip workflow,
# not runtime state). These are ~10 KB/day of audit data that nightly +
# evolve + reconcile cite to avoid re-doing work.
#
# Anything else under .agents/ stays untracked. Changes to the allowlist
# require a coordinated update of .gitignore (which carries the matching
# negation patterns) and CLAUDE.md / PROGRAM.md guidance.

ALLOWED_PATHS_REGEX='^\.agents/(nightly/|evolve/cycle-history\.jsonl$|evolve/session-state\.json$|goals/[^/]+/attempts\.jsonl$|findings/registry\.jsonl$|rpi/next-work\.jsonl$|reconcile/wave-0-thesis-snapshot\.md$|reconcile/thesis-stability-decision\.md$|reconcile/promotion-decision\.md$)'

ALLOWED_REINCLUDES_REGEX='^[[:space:]]*!/?\.agents/?[[:space:]]*$|^[[:space:]]*!/?\.agents/(rpi/?|rpi/next-work\.jsonl|nightly/?|nightly/\*\*|evolve/?|evolve/cycle-history\.jsonl|evolve/session-state\.json|goals/?|goals/\*\*/?|goals/\*\*/attempts\.jsonl|findings/?|findings/registry\.jsonl|reconcile/?|reconcile/wave-0-thesis-snapshot\.md|reconcile/thesis-stability-decision\.md|reconcile/promotion-decision\.md)[[:space:]]*$'

if [[ -n "${NO_TRACKED_AGENTS_REPO_ROOT:-}" ]]; then
  REPO_ROOT="$(cd "$NO_TRACKED_AGENTS_REPO_ROOT" && pwd)"
else
  REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
fi

filter_disallowed() {
  # Drop allowlisted paths. Treat blank input as no findings.
  local input="$1"
  [[ -n "$input" ]] || return 0
  printf '%s\n' "$input" | grep -Ev "$ALLOWED_PATHS_REGEX" || true
}

tracked_all="$(git -C "$REPO_ROOT" ls-files -- .agents 2>/dev/null || true)"
staged_all="$(git -C "$REPO_ROOT" diff --cached --name-only --diff-filter=ACMR -- .agents 2>/dev/null || true)"
tracked="$(filter_disallowed "$tracked_all")"
staged="$(filter_disallowed "$staged_all")"
errors=0

if [[ -n "$tracked" ]]; then
  echo "ERROR: repo-root .agents paths are tracked outside the audit-truth allowlist." >&2
  echo "Remove them from the index, or extend the allowlist if they truly compound across runs:" >&2
  echo "  git rm -r --cached <path>" >&2
  echo "$tracked" | sed 's/^/  - /' >&2
  errors=1
fi

if [[ -n "$staged" ]]; then
  echo "ERROR: repo-root .agents paths are staged outside the audit-truth allowlist." >&2
  echo "These files look like local agent runtime state, not audit truth:" >&2
  echo "$staged" | sed 's/^/  - /' >&2
  errors=1
fi

if [[ ! -f "$REPO_ROOT/.gitignore" ]]; then
  echo "ERROR: root .gitignore missing; cannot enforce /.agents/ ignore policy." >&2
  errors=1
else
  if ! grep -Eq '^[[:space:]]*/\.agents/(\*|\*\*/\*)?[[:space:]]*($|#)' "$REPO_ROOT/.gitignore"; then
    echo "ERROR: root .gitignore must contain an explicit '/.agents/' (or '/.agents/*') ignore rule." >&2
    errors=1
  fi
  # Use `grep -n` so we can show line numbers in diagnostics, then strip the
  # "LINENO:" prefix before matching against the allowlist regex (otherwise
  # the leading digits prevent the `^` anchor from matching).
  reinclude_lines="$(grep -nE '^[[:space:]]*!/?\.agents(/|$)' "$REPO_ROOT/.gitignore" || true)"
  disallowed_reincludes=""
  if [[ -n "$reinclude_lines" ]]; then
    while IFS= read -r line; do
      content="${line#*:}"
      if [[ ! "$content" =~ $ALLOWED_REINCLUDES_REGEX ]]; then
        disallowed_reincludes+="${line}"$'\n'
      fi
    done <<<"$reinclude_lines"
  fi
  if [[ -n "$disallowed_reincludes" ]]; then
    echo "ERROR: root .gitignore re-includes repo-root .agents paths outside the audit-truth allowlist:" >&2
    printf '%s' "$disallowed_reincludes" | sed 's/^/  /' >&2
    errors=1
  fi
fi

if [[ "$errors" -ne 0 ]]; then
  exit 1
fi

echo "no disallowed tracked repo-root .agents state"
