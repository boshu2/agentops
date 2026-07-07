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

# Check the flywheel compile runtime script. Canonical home moved to scripts/lib/
# when the compile SKILL retired (2026-07-07 wave, l6ic.12) — `ao compile` still
# ships the embedded copy; the embedded PATH is unchanged (Go embed contract).
check_file "$REPO_ROOT/scripts/lib/flywheel-compile.sh" "$EMBEDDED/skills/compile/scripts/compile.sh"

# Check the pawl bundle: scripts + verdict schema embedded so `ao pawl review` runs
# zero-config on a stranger's repo (no AgentOps checkout). The scripts/ + schemas/
# sibling layout must be preserved (pawl-verdict.sh reads its schema script-relative).
for s in pawl-review.sh pawl-verdict.sh pawl.sh; do
    check_file "$REPO_ROOT/scripts/$s" "$EMBEDDED/pawl/scripts/$s"
done
# The membrane-receipts generator + its freshness check ride along in the bundle so
# `ao verify receipts` renders a repo's own proof page zero-config on the stranger
# path (no AgentOps checkout to resolve them from). (age-rk3r.12)
for s in gen-membrane-receipts.sh check-membrane-receipts-freshness.sh; do
    check_file "$REPO_ROOT/scripts/$s" "$EMBEDDED/pawl/scripts/$s"
done
# The shared fail-closed codex runner (lib/codex-exec.sh) MUST ride along in the pawl
# bundle: pawl-review.sh sources it script-relative ($SCRIPT_DIR/lib/codex-exec.sh), so on
# the stranger/embedded path the extracted bundle needs scripts/lib/codex-exec.sh present
# or the review cannot even start. (age-gate-the-ungated-egwt.13)
check_file "$REPO_ROOT/scripts/lib/codex-exec.sh" "$EMBEDDED/pawl/scripts/lib/codex-exec.sh"
# The per-repo verify-config hook (lib/verify-config.sh) also rides along: pawl-review.sh
# sources it script-relative to honor a stranger repo's checked-in .aoverify.yaml on the
# embedded path (via `ao verify --export-env`). Absent from the bundle => stranger-repo
# config is silently ignored, so keep it byte-identical. (age-rk3r.17)
check_file "$REPO_ROOT/scripts/lib/verify-config.sh" "$EMBEDDED/pawl/scripts/lib/verify-config.sh"
# The shared diff-identity signature (lib/diff-identity.sh) rides along too: BOTH pawl-verdict.sh
# and pawl-review.sh source it script-relative ($SCRIPT_DIR/lib/diff-identity.sh) for the SINGLE
# byte-exact denylist used by REBOUND rebind/check AND --converge lineage. Absent from the bundle
# => the embedded rebind/check/converge cannot resolve the signature (fail-closed at the source),
# so keep it byte-identical. (age-rk3r.9)
check_file "$REPO_ROOT/scripts/lib/diff-identity.sh" "$EMBEDDED/pawl/scripts/lib/diff-identity.sh"
check_file "$REPO_ROOT/schemas/pawl-verdict.v1.schema.json" "$EMBEDDED/pawl/schemas/pawl-verdict.v1.schema.json"

if [[ $ERRORS -gt 0 ]]; then
    echo ""
    echo "ERROR: $ERRORS embedded file(s) are out of sync."
    echo "Run 'cd cli && make sync-hooks' to fix."
    exit 1
fi

echo "All embedded files are in sync."
