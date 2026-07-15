#!/usr/bin/env bash
# install-ms-reindex-hook.sh — install a post-merge hook that auto-runs the ms
# reindex law (scripts/ms-reindex.sh) on the CANONICAL agentops checkout (age-22g0).
#
# WHY INSTALL-ON-DEMAND, NOT A TRACKED HOOK:
#   AgentOps does not own repository Git policy. This maintenance hook therefore
#   cannot be a committed `.githooks/` + core.hooksPath default that fires in
#   every contributor clone and linked worktree.
#   Instead we ship the INSTALLER, tracked, and the operator runs it ONCE on the
#   canonical checkout. The hook it writes is heavily GUARDED so it is a no-op
#   anywhere except the canonical checkout on main after a merge that touched
#   skills/**. Idempotent — safe to re-run (e.g. after a fresh clone).
#
# The hook keeps ms's on-disk index in lockstep with skills/** on main and, per
# the reindex law, sweeps every surviving `ms mcp serve` so stale servers can't
# serve pre-wipe orphan ids.
set -euo pipefail

CANONICAL_DIR="${MS_REINDEX_CANONICAL_DIR:-$HOME/dev/agentops}"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"

common="$(git -C "$repo_root" rev-parse --git-common-dir)"
case "$common" in
  /*) ;;
  *)  common="$(cd "$repo_root" && cd "$common" && pwd)" ;;
esac
hooks_dir="${common}/hooks"
mkdir -p "$hooks_dir"
hook="${hooks_dir}/post-merge"

marker="# --- AGENTOPS MS-REINDEX POST-MERGE (managed by install-ms-reindex-hook.sh) ---"
end_marker="# --- END AGENTOPS MS-REINDEX POST-MERGE ---"

# The guarded hook body. Guards (ALL must hold or it no-ops silently):
#   1. toplevel == canonical checkout path
#   2. current branch == main
#   3. the merge that just ran changed skills/** (diff ORIG_HEAD..HEAD)
# Then it runs scripts/ms-reindex.sh from the canonical checkout, best-effort:
# a reindex failure must NOT wedge the merge, so it warns and returns 0.
read -r -d '' block <<EOF || true
${marker}
# Auto-run the ms reindex law when skills/** land on main. See
# scripts/install-ms-reindex-hook.sh for why this is install-on-demand.
_msr_canonical="${CANONICAL_DIR}"
_msr_top="\$(git rev-parse --show-toplevel 2>/dev/null || true)"
_msr_branch="\$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
if [ "\$_msr_top" = "\$_msr_canonical" ] && [ "\$_msr_branch" = "main" ]; then
  if git diff --name-only ORIG_HEAD HEAD -- 'skills/' 2>/dev/null | grep -q .; then
    if [ -x "\$_msr_top/scripts/ms-reindex.sh" ]; then
      echo "[post-merge] skills/** changed on main — running ms-reindex.sh" >&2
      "\$_msr_top/scripts/ms-reindex.sh" || echo "[post-merge] WARN: ms-reindex.sh failed (non-fatal to merge)" >&2
    fi
  fi
fi
${end_marker}
EOF

if [ -f "$hook" ] && grep -qF "$marker" "$hook"; then
  # Replace our existing managed block in place (idempotent update).
  tmp="$(mktemp "${TMPDIR:-/tmp}/msr-hook.XXXXXX")"
  awk -v m="$marker" -v e="$end_marker" '
    $0==m {skip=1}
    skip==0 {print}
    $0==e {skip=0}
  ' "$hook" > "$tmp"
  printf '%s\n' "$block" >> "$tmp"
  install -m 0755 "$tmp" "$hook"
  rm -f "$tmp"
  echo "✓ refreshed ms-reindex post-merge block in ${hook}"
elif [ -f "$hook" ]; then
  # Foreign hook present — append our guarded block, don't clobber it.
  printf '\n%s\n' "$block" >> "$hook"
  chmod +x "$hook"
  echo "✓ appended ms-reindex block to existing ${hook}"
else
  {
    printf '#!/usr/bin/env bash\n'
    printf 'set -euo pipefail\n\n'
    printf '%s\n' "$block"
  } > "$hook"
  chmod +x "$hook"
  echo "✓ installed ${hook}"
fi

echo "  guard: toplevel==${CANONICAL_DIR} && branch==main && skills/** changed in merge"
