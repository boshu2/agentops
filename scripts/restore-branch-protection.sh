#!/usr/bin/env bash
# restore-branch-protection.sh — re-apply the pre-collapse branch protection on
# main (ag-qidx.1 rollback). Reconstructs the PUT body from the captured GET in
# docs/runbooks/branch-protection-backup.json. This is the one-command revert for
# the push-to-main collapse.
#
#   scripts/restore-branch-protection.sh            # apply
#   scripts/restore-branch-protection.sh --dry-run  # print PUT body, do not apply
set -euo pipefail

# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
repo_root="$(resolve_repo_root)"
backup="${repo_root}/scripts/branch-protection-backup.json"
dry_run=false
[[ "${1:-}" == "--dry-run" ]] && dry_run=true

if [[ ! -f "$backup" ]]; then
    echo "ERROR: backup ${backup} not found — cannot restore" >&2
    exit 1
fi

# The GET schema is richer than PUT accepts; build the exact PUT body from it.
put_body="$(python3 - "$backup" <<'PY'
import json, sys
g = json.load(open(sys.argv[1]))

def rsc(g):
    s = g.get("required_status_checks")
    if not s:
        return None
    return {"strict": s.get("strict", False),
            "contexts": s.get("contexts", [])}

def rpr(g):
    r = g.get("required_pull_request_reviews")
    if not r:
        return None
    return {"dismiss_stale_reviews": r.get("dismiss_stale_reviews", False),
            "require_code_owner_reviews": r.get("require_code_owner_reviews", False),
            "required_approving_review_count": r.get("required_approving_review_count", 0)}

body = {
    "required_status_checks": rsc(g),
    "enforce_admins": g.get("enforce_admins", {}).get("enabled", False),
    "required_pull_request_reviews": rpr(g),
    "restrictions": None,
    "required_linear_history": g.get("required_linear_history", {}).get("enabled", False),
    "allow_force_pushes": g.get("allow_force_pushes", {}).get("enabled", False),
    "allow_deletions": g.get("allow_deletions", {}).get("enabled", False),
}
print(json.dumps(body))
PY
)"

if $dry_run; then
    echo "PUT repos/:owner/:repo/branches/main/protection"
    echo "$put_body" | python3 -m json.tool
    exit 0
fi

echo "$put_body" | gh api --method PUT repos/:owner/:repo/branches/main/protection --input - >/dev/null
echo "✓ branch protection on main restored from ${backup}"
