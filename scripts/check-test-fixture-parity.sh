#!/usr/bin/env bash
# check-test-fixture-parity.sh — pre-push parity gate
#
# Mirrors the CI-side parity assertions that, when broken, surface only on
# push to main. Catches the pattern at developer time:
#   1. New hooks/<x>.sh added without a matching reference in any
#      tests/{hooks,skills,scripts}/test-* file.
#   2. New scripts/<x>.sh referenced by scripts/pre-push-gate.sh but not
#      stubbed in tests/scripts/pre-push-gate.bats setup().
#
# Output: silent PASS (exit 0). FAIL (exit 1) prints one line per category
# of missing entries followed by a suggested-fix line.
#
# Bypass: AGENTOPS_PARITY_GATE_DISABLED=1.
#
# Usage:
#   scripts/check-test-fixture-parity.sh                # use repo root from git
#   scripts/check-test-fixture-parity.sh /path/to/root  # use given root (for fixtures/tests)

set -euo pipefail

if [[ "${AGENTOPS_PARITY_GATE_DISABLED:-}" == "1" ]]; then
    printf 'check-test-fixture-parity: skipped (AGENTOPS_PARITY_GATE_DISABLED=1)\n' >&2
    exit 0
fi

ROOT="${1:-}"
if [[ -z "$ROOT" ]]; then
    ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
fi

HOOKS_DIR="$ROOT/hooks"
GATE_SCRIPT="$ROOT/scripts/pre-push-gate.sh"
BATS_FILE="$ROOT/tests/scripts/pre-push-gate.bats"

# --- Check 1: hook coverage parity ---
# For each hooks/*.sh (non-recursive — examples/ intentionally excluded), the
# basename must appear in at least one of the canonical test files.
missing_hooks=""
if [[ -d "$HOOKS_DIR" ]]; then
    for hook_file in "$HOOKS_DIR"/*.sh; do
        [[ -e "$hook_file" ]] || continue
        hook_name="$(basename "$hook_file" .sh)"
        coverage_files=()
        # Mirror tests/hooks/test-hooks.bats:805-810: every .sh/.bats in
        # tests/hooks/ counts as coverage. Plus per-hook test files in
        # tests/{skills,scripts}/.
        for candidate in \
            "$ROOT/tests/hooks"/*.sh \
            "$ROOT/tests/hooks"/*.bats \
            "$ROOT/tests/skills/test-${hook_name}"*.sh \
            "$ROOT/tests/scripts/test-${hook_name}"*.sh; do
            [[ -e "$candidate" ]] && coverage_files+=("$candidate")
        done
        if [[ ${#coverage_files[@]} -eq 0 ]]; then
            missing_hooks+=" $hook_name"
            continue
        fi
        if ! grep -F -q "$hook_name" "${coverage_files[@]}" 2>/dev/null; then
            missing_hooks+=" $hook_name"
        fi
    done
fi

# --- Check 2: pre-push helper-stub parity ---
# Extract every helper-script reference inside scripts/pre-push-gate.sh using
# the IDENTICAL regex as tests/scripts/pre-push-gate.bats:154 (W1 amendment).
# For each helper, assert tests/scripts/pre-push-gate.bats setup() contains
# a make_stub line for it.
missing_stubs=""
if [[ -r "$GATE_SCRIPT" && -r "$BATS_FILE" ]]; then
    while IFS= read -r helper; do
        # Skip self-reference
        [[ "$helper" == "scripts/pre-push-gate.sh" ]] && continue
        # Stub line in bats setup() looks like:
        #   make_stub "$FAKE_REPO/<helper>"
        # or for the hash-snapshot helper, make_hash_snapshot_stub.
        if ! grep -F -q "$helper" "$BATS_FILE" 2>/dev/null; then
            missing_stubs+=" $helper"
        fi
    done < <(grep -oE '(scripts|tests|skills)/[A-Za-z0-9_./-]+[.]sh' "$GATE_SCRIPT" | sort -u)
fi

# --- Output ---
if [[ -z "$missing_hooks" && -z "$missing_stubs" ]]; then
    exit 0
fi

[[ -n "$missing_hooks" ]] && printf 'hooks with no test coverage:%s\n' "$missing_hooks" >&2
[[ -n "$missing_stubs" ]] && printf 'pre-push helpers without bats stub:%s\n' "$missing_stubs" >&2
printf '\nAdd coverage in tests/hooks/test-hooks.{sh,bats} or tests/{skills,scripts}/test-<name>*.sh,\nand a make_stub line in tests/scripts/pre-push-gate.bats setup().\n' >&2
exit 1
