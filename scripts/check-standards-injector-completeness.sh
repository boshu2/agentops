#!/usr/bin/env bash
# check-standards-injector-completeness.sh
#
# Asserts that every <lang> mapped by hooks/standards-injector.sh has a
# corresponding skills/standards/references/<lang>.md file. The hook fails
# open when a reference is missing — that's how `.js` lost standards inject
# for weeks until the 2026-04-26 nightly retro caught it. This gate makes
# the omission loud at lint time.
#
# Exit 0: every mapped lang has a reference file.
# Exit 1: at least one mapped lang is missing its reference file.
# Exit 2: parser failed (case statement not where expected).

set -euo pipefail

# Resolve repo root from script location.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Allow override for tests (so the bats harness can point at a fake repo).
HOOK_FILE="${STANDARDS_INJECTOR_HOOK:-$REPO_ROOT/hooks/standards-injector.sh}"
REFS_DIR="${STANDARDS_REFERENCES_DIR:-$REPO_ROOT/skills/standards/references}"

if [[ ! -f "$HOOK_FILE" ]]; then
    echo "ERROR: hook file not found: $HOOK_FILE" >&2
    exit 2
fi
if [[ ! -d "$REFS_DIR" ]]; then
    echo "ERROR: references dir not found: $REFS_DIR" >&2
    exit 2
fi

# Parse case statement of form:
#   case "$EXT" in
#       py)        LANG="python" ;;
#       ts|tsx)    LANG="typescript" ;;
#       *)         exit 0 ;;
#   esac
#
# Strategy: extract lines containing LANG="..." and pull the language name
# out of the right side. The case-pattern on the left can have | alternation,
# but we only care about the LANG mapping (one entry per pattern → one ref
# file). The wildcard pattern *) does not assign LANG so it is skipped.
LANGS=$(awk '
    /case[[:space:]]+"\$EXT"[[:space:]]+in/ { in_case = 1; next }
    in_case && /^[[:space:]]*esac/ { in_case = 0; next }
    in_case && /LANG=/ {
        # Extract the value inside LANG="..."
        if (match($0, /LANG="[^"]+"/)) {
            s = substr($0, RSTART + 6, RLENGTH - 7)
            print s
        }
    }
' "$HOOK_FILE" | sort -u)

if [[ -z "$LANGS" ]]; then
    echo "ERROR: parser found no LANG= assignments inside case block of $HOOK_FILE" >&2
    echo "       This is a parser failure, not a missing-language failure." >&2
    exit 2
fi

missing=()
for lang in $LANGS; do
    if [[ ! -f "$REFS_DIR/$lang.md" ]]; then
        missing+=("$lang")
    fi
done

if [[ ${#missing[@]} -gt 0 ]]; then
    echo "FAIL: standards-injector maps language(s) with no reference file:" >&2
    for lang in "${missing[@]}"; do
        echo "  - $lang  (expected: $REFS_DIR/$lang.md)" >&2
    done
    echo >&2
    echo "Hint: either add the missing reference file under skills/standards/references/," >&2
    echo "      or remove the language from the case statement in $HOOK_FILE." >&2
    exit 1
fi

echo "OK: all $(echo "$LANGS" | wc -l | tr -d ' ') languages mapped by standards-injector.sh have reference files."
exit 0
