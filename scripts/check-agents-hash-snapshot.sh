#!/usr/bin/env bash
set -euo pipefail

# check-agents-hash-snapshot.sh — content-hash gate for ~/.agents/ mutation
# during test runs.
#
# Complements scripts/check-home-isolation.sh which only checks for the
# t.Setenv("HOME", ...) pattern statically via grep. This gate catches any
# mutation of the protected ~/.agents/ subtrees, including:
#   - Tests that write to ~/.agents/ without HOME isolation
#   - Tests that write then use os.Chtimes to hide the mtime (the
#     mtime-bypass attack; demonstrated in cli/internal/overnight/recovery_test.go:285
#     for legitimate recovery fixture setup, but the same API can hide
#     test-side poisoning)
#
# Usage:
#   SNAP=$(./scripts/check-agents-hash-snapshot.sh capture)
#   go test ./...
#   ./scripts/check-agents-hash-snapshot.sh diff "$SNAP"
#
# Scoped to the subtrees Dream and flywheel tests are most likely to
# corrupt: learnings, patterns, findings, rpi/next-work.jsonl.

# shasum presence check — skip gracefully on minimal containers
if ! command -v shasum >/dev/null 2>&1; then
    echo "WARN: shasum not available; ~/.agents hash gate skipped" >&2
    exit 0
fi

AGENTS_HUB="${AGENTS_HUB_OVERRIDE:-$HOME/.agents}"
SCOPES=(learnings patterns findings rpi/next-work.jsonl)

# --ignore-untracked: exclude git-untracked files from the scoped subtrees
# so local scratch files (docs/blog/, *_test.txt, etc.) do not trip the gate.
# Env override: HASH_GATE_IGNORE_UNTRACKED=1 is equivalent to passing the flag.
# The pre-push wrapper also honors HASH_GATE_TIMEOUT_SECONDS for bounded local
# runtime when shared ~/.agents state is busy.
IGNORE_UNTRACKED="${HASH_GATE_IGNORE_UNTRACKED:-0}"

args=()
for arg in "$@"; do
    case "$arg" in
        --ignore-untracked) IGNORE_UNTRACKED=1 ;;
        *) args+=("$arg") ;;
    esac
done
set -- "${args[@]+"${args[@]}"}"

# is_untracked <path> — returns 0 if the path exists inside a git repo and is
# untracked per `git status`. Returns 1 otherwise (tracked, ignored, or not in
# a repo). Safe to call with absolute paths; resolves the repo from the path's
# parent directory.
is_untracked() {
    local p="$1"
    [[ "$IGNORE_UNTRACKED" == "1" ]] || return 1
    command -v git >/dev/null 2>&1 || return 1
    local dir
    dir="$(dirname "$p")"
    [[ -d "$dir" ]] || return 1
    # --error-unmatch is noisy; use ls-files with --error-unmatch exit code.
    if ( cd "$dir" && git rev-parse --is-inside-work-tree >/dev/null 2>&1 ); then
        if ( cd "$dir" && git ls-files --error-unmatch -- "$p" >/dev/null 2>&1 ); then
            return 1 # tracked
        fi
        return 0 # inside repo, not tracked
    fi
    return 1
}

snapshot() {
    for scope in "${SCOPES[@]}"; do
        local target="$AGENTS_HUB/$scope"
        if [[ -f "$target" ]]; then
            if is_untracked "$target"; then
                continue
            fi
            shasum -a 256 "$target" 2>/dev/null || true
        elif [[ -d "$target" ]]; then
            while IFS= read -r -d '' f; do
                if is_untracked "$f"; then
                    continue
                fi
                shasum -a 256 "$f" 2>/dev/null || true
            done < <(find "$target" -type f -print0 2>/dev/null) \
                | sort
        fi
    done
}

cmd="${1:-}"
case "$cmd" in
    capture)
        out="${AGENTOPS_HASH_SNAPSHOT:-$(mktemp -t agents-snap.XXXXXX)}"
        snapshot > "$out"
        echo "$out"
        ;;
    diff)
        snap_file="${2:-}"
        if [[ -z "$snap_file" || ! -f "$snap_file" ]]; then
            echo "ERROR: diff requires a valid snapshot file path" >&2
            exit 2
        fi
        current=$(mktemp -t agents-snap.XXXXXX)
        snapshot > "$current"
        if ! diff -q "$snap_file" "$current" >/dev/null 2>&1; then
            echo "FAIL: ~/.agents mutation detected during tests" >&2
            echo "--- snapshot before (path: $snap_file)" >&2
            echo "+++ snapshot after" >&2
            diff "$snap_file" "$current" | head -40 >&2
            rm -f "$current"
            exit 1
        fi
        rm -f "$current"
        ;;
    *)
        echo "usage: $0 [--ignore-untracked] {capture|diff <snapshot-file>}" >&2
        echo "       HASH_GATE_IGNORE_UNTRACKED=1 $0 capture  # env form" >&2
        exit 2
        ;;
esac
