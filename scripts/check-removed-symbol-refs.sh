#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MODE="fixed"
SYMBOLS=()
EXCLUDES=()

usage() {
    cat <<'USAGE'
Usage: scripts/check-removed-symbol-refs.sh [options] -- <removed-symbol>...

Check that removed CLI commands, flags, symbols, or cross-language surfaces no
longer have tracked callsites. This intentionally searches beyond the source
language so shell scripts, GitHub workflow YAML, docs, skills, and tests cannot
silently keep dead references alive.

Options:
  --regex             Treat symbols as extended regular expressions
  --exclude GLOB      Additional git pathspec glob to exclude
  -h, --help          Show this help

Examples:
  scripts/check-removed-symbol-refs.sh -- --oscillation-sweep
  scripts/check-removed-symbol-refs.sh -- 'ao defrag old-command'
USAGE
}

die() {
    echo "ERROR: $*" >&2
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --regex)
            MODE="regex"
            shift
            ;;
        --exclude)
            [[ -n "${2:-}" ]] || die "--exclude requires a git pathspec glob"
            EXCLUDES+=("$2")
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        --)
            shift
            while [[ $# -gt 0 ]]; do
                SYMBOLS+=("$1")
                shift
            done
            ;;
        -*)
            die "unknown option or flag-like symbol before --: $1"
            ;;
        *)
            SYMBOLS+=("$1")
            shift
            ;;
    esac
done

[[ "${#SYMBOLS[@]}" -gt 0 ]] || {
    usage
    exit 1
}

cd "$REPO_ROOT"

if ! git rev-parse --show-toplevel >/dev/null 2>&1; then
    die "not inside a git repository: $REPO_ROOT"
fi

PATHS=(
    "."
    ":(exclude)CHANGELOG.md"
    ":(exclude)docs/CHANGELOG.md"
    ":(exclude)docs/releases/**"
    ":(exclude).agents/**"
    ":(exclude)worktrees/**"
    ":(exclude).worktrees/**"
)

for exclude in "${EXCLUDES[@]}"; do
    PATHS+=(":(exclude)$exclude")
done

failures=0

for symbol in "${SYMBOLS[@]}"; do
    echo "==> checking removed symbol: $symbol"
    tmp="$(mktemp)"
    set +e
    if [[ "$MODE" == "regex" ]]; then
        git grep -n -E -- "$symbol" -- "${PATHS[@]}" >"$tmp"
    else
        git grep -n -F -- "$symbol" -- "${PATHS[@]}" >"$tmp"
    fi
    grep_status=$?
    set -e

    if [[ "$grep_status" -gt 1 ]]; then
        rm -f "$tmp"
        die "git grep failed for removed symbol: $symbol"
    fi

    if [[ -s "$tmp" ]]; then
        echo "FAIL removed-symbol-refs: remaining references to '$symbol'" >&2
        cat "$tmp" >&2
        failures=$((failures + 1))
    else
        echo "OK removed-symbol-refs: no remaining references to '$symbol'"
    fi
    rm -f "$tmp"
done

if [[ "$failures" -gt 0 ]]; then
    exit 1
fi

exit 0
