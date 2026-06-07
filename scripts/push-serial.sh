#!/usr/bin/env bash
# push-serial.sh — serialize concurrent pushes to main for SWARM sessions
# (ag-qidx P1.4). Solo work does NOT need this — use `git push`; only when
# multiple agents on this host push concurrently does the lock matter.
#
# Two layers of serialization:
#   1. within-host: a portable advisory lock (mkdir is atomic; `flock` is absent
#      on macOS) so two local agents can't gate+push at the same instant and
#      corrupt a shared generated surface (registry.json etc.).
#   2. cross-host (Mac + bushido): git's own atomic non-fast-forward rejection +
#      rebase-on-reject retry — no shared lock exists across hosts, so we rely on
#      git and converge by rebasing.
#
# The pre-push gate still runs in the hook on the actual push. Usage:
#   scripts/push-serial.sh [remote] [branch]   # default: origin main
set -euo pipefail

REMOTE="${1:-origin}"
BRANCH="${2:-main}"
LOCK_DIR="${TMPDIR:-/tmp}/agentops-push.lock"
TIMEOUT="${PUSH_LOCK_TIMEOUT:-300}"
MAX_REBASE="${PUSH_MAX_REBASE:-5}"

# ---- layer 1: within-host advisory lock (mkdir, portable) ----
waited=0
while ! mkdir "$LOCK_DIR" 2>/dev/null; do
    if [ "$waited" -ge "$TIMEOUT" ]; then
        echo "push-serial: timed out after ${TIMEOUT}s waiting for $LOCK_DIR" >&2
        echo "  (another local push is in progress; or remove a stale lock dir)" >&2
        exit 1
    fi
    sleep 2
    waited=$((waited + 2))
done
trap 'rmdir "$LOCK_DIR" 2>/dev/null || true' EXIT

# ---- keep base fresh before pushing ----
git fetch "$REMOTE" "$BRANCH" --quiet
git rebase "$REMOTE/$BRANCH"

# ---- layer 2: push with rebase-on-reject retry (cross-host convergence) ----
err_file="$(mktemp)"
trap 'rmdir "$LOCK_DIR" 2>/dev/null || true; rm -f "$err_file"' EXIT
attempts=0
while ! git push "$REMOTE" "HEAD:$BRANCH" 2>"$err_file"; do
    if grep -qiE 'non-fast-forward|fetch first|rejected' "$err_file"; then
        attempts=$((attempts + 1))
        if [ "$attempts" -ge "$MAX_REBASE" ]; then
            echo "push-serial: still rejected after ${MAX_REBASE} rebase attempts" >&2
            cat "$err_file" >&2
            exit 1
        fi
        echo "push-serial: main moved — rebasing (attempt ${attempts}/${MAX_REBASE})" >&2
        git fetch "$REMOTE" "$BRANCH" --quiet
        git rebase "$REMOTE/$BRANCH"
    else
        cat "$err_file" >&2
        exit 1
    fi
done
echo "push-serial: pushed HEAD -> ${REMOTE}/${BRANCH}"
