#!/usr/bin/env bash
# validate-embedded-sync.sh — Verify embedded copies match source hooks/scripts.
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

# Check hooks/*.sh and hooks.json
for f in "$REPO_ROOT"/hooks/*.sh "$REPO_ROOT"/hooks/hooks.json; do
    basename=$(basename "$f")
    check_file "$f" "$EMBEDDED/hooks/$basename"
done

# Check lib/hook-helpers.sh
check_file "$REPO_ROOT/lib/hook-helpers.sh" "$EMBEDDED/lib/hook-helpers.sh"

# Check skills/standards/references/*
for f in "$REPO_ROOT"/skills/standards/references/*; do
    basename=$(basename "$f")
    check_file "$f" "$EMBEDDED/skills/standards/references/$basename"
done

# Check skills/using-agentops/SKILL.md
check_file "$REPO_ROOT/skills/using-agentops/SKILL.md" "$EMBEDDED/skills/using-agentops/SKILL.md"

# Check skills/compile runtime script
check_file "$REPO_ROOT/skills/compile/scripts/compile.sh" "$EMBEDDED/skills/compile/scripts/compile.sh"

if [[ $ERRORS -gt 0 ]]; then
    echo ""
    echo "ERROR: $ERRORS embedded file(s) are out of sync."
    echo "Run 'cd cli && make sync-hooks' to fix."
    exit 1
fi

echo "All embedded files are in sync."
