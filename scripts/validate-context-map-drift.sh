#!/usr/bin/env bash
# validate-context-map-drift.sh — detect drift between SKILL.md frontmatter and
# the generated docs/contracts/context-map.md.
#
# Per DDD+Hexagonal v1 plan Issue #5 (Fix 3): the context map is a generated
# artifact whose source of truth is `skills/*/SKILL.md` frontmatter. Drift
# happens when a SKILL.md is edited (hexagonal_role/consumes/produces/context_rel
# changes) without regenerating the map. This gate forces the regeneration to
# be committed alongside the SKILL.md edit.
#
# Behaviour:
#   - Exit 0 if regenerating yields the same bytes as the current on-disk
#     context-map.
#   - Exit 1 if drift is detected, with a helpful "how to fix" message on stderr.
#
# Safety: the script never leaves the working tree dirty. The current
# context-map.md is backed up to a temp file and restored on exit (trap),
# whether the script exits cleanly or via error.
set -euo pipefail

CONTEXT_MAP="${CONTEXT_MAP:-docs/contracts/context-map.md}"
GENERATOR="${GENERATOR:-scripts/generate-context-map.sh}"

if [[ ! -f "$CONTEXT_MAP" ]]; then
    echo "validate-context-map-drift: missing $CONTEXT_MAP" >&2
    exit 1
fi

if [[ ! -x "$GENERATOR" && ! -f "$GENERATOR" ]]; then
    echo "validate-context-map-drift: missing $GENERATOR" >&2
    exit 1
fi

TMP_BACKUP="$(mktemp -t context-map-backup.XXXXXX.md)"
cp -f "$CONTEXT_MAP" "$TMP_BACKUP"

# shellcheck disable=SC2329 # invoked indirectly via trap
cleanup() {
    # Always restore the original committed context-map so partial failures
    # (or a real drift detection) never leave the working tree dirty.
    if [[ -f "$TMP_BACKUP" ]]; then
        cp -f "$TMP_BACKUP" "$CONTEXT_MAP"
        rm -f "$TMP_BACKUP"
    fi
}
trap cleanup EXIT

bash "$GENERATOR" >/dev/null

if cmp -s "$CONTEXT_MAP" "$TMP_BACKUP"; then
    exit 0
fi

cat >&2 <<'EOF'
Context map drift detected. To fix:
  bash scripts/generate-context-map.sh
  git add docs/contracts/context-map.md
Then commit.
EOF

exit 1
