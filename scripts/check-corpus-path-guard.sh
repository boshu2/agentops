#!/usr/bin/env bash
# check-corpus-path-guard.sh — fail-closed pre-push PATH guard (ag-ao0eo, epic ag-k7tq9).
#
# Layer 4 of the private/public corpus seam (council verdict:
# .agents/council/2026-06-15-corpus-private-public-seam-verdict.md). S0 carved the
# corpus into a private nested repo; this guard makes it IMPOSSIBLE for a private
# artifact to be committed/pushed to the PUBLIC boshu2/agentops repo.
#
# PRIVACY-CRITICAL: this MUST fail closed. If the push range cannot be determined,
# we still scan the staged index and the nearest committed range we can compute —
# we never silently pass.
#
# This is a PATH guard. It is DISTINCT from:
#   - scripts/check-corpus-secret-scan.sh — the CONTENT scanner (credential patterns).
#   - the S4 `ao corpus scan` marker registry — the marker allowlist.
# Do NOT add a second marker/content allowlist here. PATHS ONLY.
#
# SCOPE — what this guard does NOT cover (do not credit it for these):
#   - It does NOT protect against `git worktree remove` / `git branch -D` /
#     destructive cleanup. Pre-push runs on what is being PUSHED, not on local
#     worktree/branch deletion. That hazard is a SEPARATE concern and out of
#     scope here. Do not pretend this guard covers it.
set -euo pipefail

# Forbidden top-level paths (anchored at repo root). Any push-range or staged
# path matching these is private and must never reach the public repo:
#   .agents/learnings/  — the private corpus dir (S0)
#   _beads/             — the private tracker (its own nested repo)
#   docs/wiki/          — generated public wiki, but UNTRACEABLE for now:
#                         the promotion manifest (bead S5) does not exist yet, so
#                         we treat ANY docs/wiki/ path as untraceable and reject.
# TODO(S5): replace the blanket docs/wiki/ rejection with a traceability
#           allowlist — accept a docs/wiki/ path only when the S5 promotion
#           manifest proves it was generated from a public-promoted source.
FORBIDDEN='^\.agents/learnings/|^_beads/|^docs/wiki/'

# Base ref override lets the gate/CI pass the exact range; default origin/main..HEAD.
BASE_REF="${CORPUS_PATH_GUARD_BASE:-origin/main}"

# Compute the committed range robustly. We want everything reachable from HEAD
# that is not already on the public base. Prefer merge-base with the base ref;
# fall back so we NEVER silently skip the committed range.
base=""
if git rev-parse --verify -q "$BASE_REF" >/dev/null 2>&1; then
  base="$(git merge-base "$BASE_REF" HEAD 2>/dev/null || echo "")"
fi
if [ -z "$base" ]; then
  # No origin/main (fresh clone, detached, CI without remote). Fall back to the
  # parent of HEAD so we still scan the tip commit; if even that fails (root
  # commit), the empty base makes `git diff` list HEAD's full tree — still scanned.
  base="$(git rev-parse -q --verify 'HEAD~1' 2>/dev/null || echo "")"
fi

committed=""
if [ -n "$base" ]; then
  committed="$(git diff --name-only "$base..HEAD" 2>/dev/null || true)"
else
  # No usable base at all (root commit / no HEAD~1): scan HEAD's full tree.
  committed="$(git ls-tree -r --name-only HEAD 2>/dev/null || true)"
fi

# Staged index too — an already-committed private file would pass a staged-only
# check, but a staged-but-uncommitted one would pass a committed-only check.
# Scan BOTH.
staged="$(git diff --cached --name-only 2>/dev/null || true)"

# Union, de-duplicated.
all_paths="$(printf '%s\n%s\n' "$committed" "$staged" | grep -v '^$' | sort -u || true)"

if [ -z "$all_paths" ]; then
  echo "check-corpus-path-guard: ok (no committed/staged paths in scope)"
  exit 0
fi

offenders="$(printf '%s\n' "$all_paths" | grep -E "$FORBIDDEN" || true)"

if [ -n "$offenders" ]; then
  echo "check-corpus-path-guard: FAIL — private artifact path(s) heading to the PUBLIC repo:" >&2
  printf '%s\n' "$offenders" | while IFS= read -r p; do
    [ -n "$p" ] && echo "  forbidden: $p" >&2
  done
  echo "  fix: private artifacts must stay in their nested repo (_beads/ has its own remote;" >&2
  echo "       .agents/learnings/ is the private corpus). Never commit/push them to boshu2/agentops." >&2
  echo "       docs/wiki/ is rejected until the S5 promotion manifest adds a traceability allowlist." >&2
  exit 1
fi

count="$(printf '%s\n' "$all_paths" | grep -c '' || echo 0)"
echo "check-corpus-path-guard: ok (${count} path(s) scanned, no private artifacts)"
