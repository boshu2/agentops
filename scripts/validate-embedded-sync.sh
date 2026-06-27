#!/usr/bin/env bash
# validate-embedded-sync.sh — Verify embedded copies match source lib/skills.
# Exits non-zero if any embedded file is stale (doesn't match source).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
EMBEDDED="$REPO_ROOT/cli/embedded"
ERRORS=0

check_file() {
    local src="$1" dst="$2"
    if [[ ! -f "$dst" ]]; then
        echo "MISSING: $dst (source: $src)"
        ERRORS=$((ERRORS + 1))
        return
    fi
    if ! diff -q "$src" "$dst" >/dev/null 2>&1; then
        echo "STALE: $dst differs from $src"
        ERRORS=$((ERRORS + 1))
    fi
}

# Check skills/standards/references/*
for f in "$REPO_ROOT"/skills/standards/references/*; do
    basename=$(basename "$f")
    check_file "$f" "$EMBEDDED/skills/standards/references/$basename"
done

# Check skills/compile runtime script
check_file "$REPO_ROOT/skills/compile/scripts/compile.sh" "$EMBEDDED/skills/compile/scripts/compile.sh"

# Check the pawl bundle: scripts + verdict schema embedded so `ao pawl review` runs
# zero-config on a stranger's repo (no AgentOps checkout). The scripts/ + schemas/
# sibling layout must be preserved (pawl-verdict.sh reads its schema script-relative).
for s in pawl-review.sh pawl-verdict.sh pawl.sh; do
    check_file "$REPO_ROOT/scripts/$s" "$EMBEDDED/pawl/scripts/$s"
done
check_file "$REPO_ROOT/schemas/pawl-verdict.v1.schema.json" "$EMBEDDED/pawl/schemas/pawl-verdict.v1.schema.json"

if [[ $ERRORS -gt 0 ]]; then
    echo ""
    echo "ERROR: $ERRORS embedded file(s) are out of sync."
    echo "Run 'cd cli && make sync-hooks' to fix."
    exit 1
fi

echo "All embedded files are in sync."
