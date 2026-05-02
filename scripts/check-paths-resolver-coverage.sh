#!/usr/bin/env bash
# check-paths-resolver-coverage.sh — Warn-only ratchet for soc-irg1.5.
#
# Counts how many executable-code surfaces still hardcode `.agents/` paths
# instead of sourcing the canonical state-path resolver:
#   - lib/ao-paths.sh           (shell side, soc-irg1.1)
#   - cli/internal/paths/*.go   (Go side, soc-irg1.1)
#
# Surfaces measured (executable code that COMPUTES paths):
#   cli/cmd/ao   — non-test ao subcommands
#   cli/internal — non-test internal Go packages
#   hooks        — bash hook scripts
#   lib          — bash helper libraries
#   scripts      — release / validation / maintenance scripts
#
# Surfaces deliberately NOT measured (path strings here are usually content,
# not constructions, or are auto-generated):
#   *_test.go   — fixture paths legitimately reference .agents/
#   skills/, skills-codex/, docs/   — markdown prose and examples
#   cli/embedded/                   — auto-synced from hooks/ + skills/
#
# Exit code: ALWAYS 0 (warn-only, per warn-then-fail-ratchet pattern).
# Flip to blocking is a separate decision after 2+ weeks of baseline data —
# see GOALS.md ("Fitness gate: state-path-resolver-coverage") and
# .agents/patterns/2026-05-01-state-path-resolver.md for context.
#
# Pattern: see .agents/patterns/pre-tag-ci-validation.md and
# warn-then-fail-ratchet for the lifecycle convention.

set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$REPO_ROOT" 2>/dev/null || exit 0

SURFACES=("cli/cmd/ao" "cli/internal" "hooks" "lib" "scripts")

count_surface() {
    # Counts string-literal occurrences of "\.agents/ — i.e. each line that
    # constructs or references a hardcoded .agents/ path string. Per-line
    # (not per-file) so chipping away at long files is visible in the metric.
    local surface="$1"
    [[ -d "$surface" ]] || { echo 0; return; }
    case "$surface" in
        cli/cmd/ao|cli/internal)
            # Go: skip *_test.go (fixture paths are intentional).
            grep -rn '"\.agents/' "$surface/" 2>/dev/null \
                | grep -v ':.*_test\.go:' \
                | grep -v '^[^:]*_test\.go:' \
                | wc -l
            ;;
        *)
            grep -rn '"\.agents/' "$surface/" 2>/dev/null | wc -l
            ;;
    esac
}

declare -A counts
total=0
for surface in "${SURFACES[@]}"; do
    n=$(count_surface "$surface")
    counts[$surface]=$n
    total=$((total + n))
done

# Compact one-line metric (parseable by tooling) + pretty per-surface line.
printf 'state-path-resolver-coverage  total=%d  by-surface:' "$total"
for surface in "${SURFACES[@]}"; do
    printf ' %s=%d' "$surface" "${counts[$surface]}"
done
printf '\n'

# Long form, one surface per line, for human review when run manually.
if [[ "${VERBOSE:-0}" == "1" ]] || [[ -t 1 ]]; then
    printf '\n'
    printf '  %-15s  %5s\n' "surface" "files"
    printf '  %-15s  %5s\n' "---------------" "-----"
    for surface in "${SURFACES[@]}"; do
        printf '  %-15s  %5d\n' "$surface" "${counts[$surface]}"
    done
    printf '  %-15s  %5d\n' "TOTAL" "$total"
    printf '\n'
    printf '  Status: warn-only (gate exit 0 always).\n'
    printf '  Migration target: 0 hardcoded ".agents/" sites in executable surfaces.\n'
    printf '  See .agents/patterns/2026-05-01-state-path-resolver.md.\n'
fi

# Warn-only — never fail the gate during the baseline window.
exit 0
