#!/usr/bin/env bash
set -euo pipefail

# pre-push-proof.sh — records/checks that the fast pre-push gate already
# passed for the current commit, scope, changed-file set, and gate scripts.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

COMMAND="${1:-}"
shift || true

SCOPE="upstream"
MODE="fast"
PROOF_FILE="${AGENTOPS_PRE_PUSH_PROOF:-$REPO_ROOT/.agents/validation/pre-push-success.tsv}"

usage() {
    cat <<'EOF'
Usage: scripts/pre-push-proof.sh {write|check} [--scope auto|upstream|staged|worktree|head] [--mode fast]

Records or checks a local validation proof for scripts/pre-push-gate.sh.
The proof is intentionally local under .agents/validation and is not a
substitute for CI.
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --scope)
            SCOPE="${2:-}"
            shift 2
            ;;
        --mode)
            MODE="${2:-}"
            shift 2
            ;;
        --file)
            PROOF_FILE="${2:-}"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown arg: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

case "$COMMAND" in
    write|check) ;;
    *)
        usage >&2
        exit 2
        ;;
esac

case "$SCOPE" in
    auto|upstream|staged|worktree|head) ;;
    *)
        echo "Invalid --scope: $SCOPE" >&2
        exit 2
        ;;
esac

hash_stdin() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum | awk '{print $1}'
    else
        shasum -a 256 | awk '{print $1}'
    fi
}

checksum_file() {
    local file="$1"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$file"
    else
        shasum -a 256 "$file"
    fi
}

collect_changed() {
    case "$SCOPE" in
        upstream)
            git diff --name-only '@{upstream}...HEAD' 2>/dev/null || true
            ;;
        staged)
            git diff --name-only --cached 2>/dev/null || true
            ;;
        worktree)
            {
                git diff --name-only --cached 2>/dev/null || true
                git diff --name-only 2>/dev/null || true
            } | sed '/^[[:space:]]*$/d' | sort -u
            ;;
        head)
            git show --name-only --pretty=format: HEAD 2>/dev/null || true
            ;;
        auto)
            {
                git diff --name-only '@{upstream}...HEAD' 2>/dev/null || true
                git diff --name-only --cached 2>/dev/null || true
                git diff --name-only 2>/dev/null || true
            } | sed '/^[[:space:]]*$/d' | sort -u
            ;;
    esac
}

current_head() {
    git rev-parse HEAD 2>/dev/null || printf 'unknown\n'
}

current_upstream() {
    git rev-parse '@{upstream}' 2>/dev/null || printf 'no-upstream\n'
}

changed_hash() {
    collect_changed | sed '/^[[:space:]]*$/d' | sort -u | hash_stdin
}

gate_hash() {
    local file
    for file in scripts/pre-push-gate.sh scripts/pre-push-proof.sh .githooks/pre-push; do
        [[ -f "$file" ]] || continue
        checksum_file "$file"
    done | hash_stdin
}

fingerprint() {
    {
        printf 'version=v1\n'
        printf 'mode=%s\n' "$MODE"
        printf 'scope=%s\n' "$SCOPE"
        printf 'head=%s\n' "$(current_head)"
        printf 'upstream=%s\n' "$(current_upstream)"
        printf 'changed_hash=%s\n' "$(changed_hash)"
        printf 'gate_hash=%s\n' "$(gate_hash)"
    } | hash_stdin
}

read_proof_value() {
    local key="$1"
    [[ -f "$PROOF_FILE" ]] || return 1
    grep -E "^${key}=" "$PROOF_FILE" | head -1 | cut -d= -f2-
}

case "$COMMAND" in
    write)
        mkdir -p "$(dirname "$PROOF_FILE")"
        fp="$(fingerprint)"
        {
            printf 'fingerprint=%s\n' "$fp"
            printf 'written_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
            printf 'mode=%s\n' "$MODE"
            printf 'scope=%s\n' "$SCOPE"
            printf 'head=%s\n' "$(current_head)"
        } >"$PROOF_FILE"
        printf 'pre-push validation proof recorded: %s\n' "$PROOF_FILE"
        ;;
    check)
        expected="$(fingerprint)"
        actual="$(read_proof_value fingerprint || true)"
        if [[ -n "$actual" && "$actual" == "$expected" ]]; then
            written_at="$(read_proof_value written_at || true)"
            printf 'pre-push validation proof current'
            if [[ -n "$written_at" ]]; then
                printf ' (%s)' "$written_at"
            fi
            printf '\n'
            exit 0
        fi
        exit 1
        ;;
esac
