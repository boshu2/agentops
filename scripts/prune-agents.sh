#!/usr/bin/env bash
# prune-agents.sh — Enforce .agents/ retention policies
#
# Usage:
#   ./scripts/prune-agents.sh              # Dry run (default) — show what would be deleted
#   ./scripts/prune-agents.sh --execute    # Actually delete files
#   ./scripts/prune-agents.sh --quiet      # Suppress per-file output (summary only)
#   ./scripts/prune-agents.sh --execute --quiet  # Auto-prune with minimal output
#
# Policies defined in .agents/README.md ## Pruning section.
# Never touches: learnings/, patterns/, plans/, research/, retros/ (knowledge assets)

set -euo pipefail

# Anchor to repo root to avoid pruning wrong .agents/ when cwd differs
# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
REPO_ROOT="$(resolve_repo_root)"
AGENTS_DIR="${REPO_ROOT}/.agents"
DRY_RUN=true
TOTAL_FILES=0
TOTAL_BYTES=0

QUIET=false
for arg in "$@"; do
    case "$arg" in
        --execute) DRY_RUN=false ;;
        --quiet) QUIET=true ;;
    esac
done

if [[ "$QUIET" == false ]]; then
    if [[ "$DRY_RUN" == true ]]; then
        echo "=== DRY RUN — no files will be deleted (pass --execute to delete) ==="
    else
        echo "=== EXECUTE MODE — files will be deleted ==="
    fi
    echo ""
fi

# Helper: list files to prune, sorted oldest first
prune_keep_newest() {
    local dir="$1"
    local keep="$2"
    local label="$3"

    if [[ ! -d "$dir" ]]; then
        return
    fi

    local count
    count=$(find "$dir" -maxdepth 1 -type f 2>/dev/null | wc -l | tr -d ' ')

    if [[ "$count" -le "$keep" ]]; then
        [[ "$QUIET" == false ]] && echo "[$label] $count files — within limit ($keep). Nothing to prune."
        return
    fi

    local to_delete=$((count - keep))
    [[ "$QUIET" == false ]] && echo "[$label] $count files — keeping newest $keep, pruning $to_delete"

    # List oldest files first (by modification time)
    find "$dir" -maxdepth 1 -type f -print0 2>/dev/null \
        | xargs -0 ls -t 2>/dev/null \
        | tail -n "$to_delete" \
        | while read -r f; do
            local size
            size=$(stat -f%z "$f" 2>/dev/null || stat --format=%s "$f" 2>/dev/null || echo 0)
            TOTAL_BYTES=$((TOTAL_BYTES + size))
            TOTAL_FILES=$((TOTAL_FILES + 1))
            if [[ "$DRY_RUN" == true ]]; then
                [[ "$QUIET" == false ]] && echo "  would delete: $f ($(numfmt_size "$size"))"
            else
                rm -f "$f"
                [[ "$QUIET" == false ]] && echo "  deleted: $f ($(numfmt_size "$size"))"
            fi
        done
}

prune_older_than() {
    local dir="$1"
    local days="$2"
    local pattern="$3"
    local label="$4"

    if [[ ! -d "$dir" ]]; then
        return
    fi

    local found
    found=$(find "$dir" -maxdepth 1 -name "$pattern" -type f -mtime +"$days" 2>/dev/null | wc -l | tr -d ' ')

    if [[ "$found" -eq 0 ]]; then
        [[ "$QUIET" == false ]] && echo "[$label] No files older than ${days}d matching '$pattern'. Nothing to prune."
        return
    fi

    [[ "$QUIET" == false ]] && echo "[$label] $found files older than ${days}d"

    find "$dir" -maxdepth 1 -name "$pattern" -type f -mtime +"$days" -print0 2>/dev/null \
        | while IFS= read -r -d '' f; do
            local size
            size=$(stat -f%z "$f" 2>/dev/null || stat --format=%s "$f" 2>/dev/null || echo 0)
            TOTAL_BYTES=$((TOTAL_BYTES + size))
            TOTAL_FILES=$((TOTAL_FILES + 1))
            if [[ "$DRY_RUN" == true ]]; then
                [[ "$QUIET" == false ]] && echo "  would delete: $f ($(numfmt_size "$size"))"
            else
                rm -f "$f"
                [[ "$QUIET" == false ]] && echo "  deleted: $f ($(numfmt_size "$size"))"
            fi
        done
}

numfmt_size() {
    local bytes="$1"
    if [[ "$bytes" -ge 1073741824 ]]; then
        echo "$(( bytes / 1073741824 ))GB"
    elif [[ "$bytes" -ge 1048576 ]]; then
        echo "$(( bytes / 1048576 ))MB"
    elif [[ "$bytes" -ge 1024 ]]; then
        echo "$(( bytes / 1024 ))KB"
    else
        echo "${bytes}B"
    fi
}

# --- Policy: council/ — keep last 30 ---
prune_keep_newest "$AGENTS_DIR/council" 30 "council"
[[ "$QUIET" == false ]] && echo ""

# --- tooling/ and security/ no longer live in .agents/ (moved to $TMPDIR) ---
# Clean up any legacy directories left from older versions
for legacy_dir in "$AGENTS_DIR/tooling" "$AGENTS_DIR/security"; do
    if [[ -d "$legacy_dir" ]]; then
        legacy_count=$(find "$legacy_dir" -type f 2>/dev/null | wc -l | tr -d ' ')
        if [[ "$legacy_count" -gt 0 ]]; then
            [[ "$QUIET" == false ]] && echo "[legacy] $legacy_dir has $legacy_count files (scanner output moved to \$TMPDIR)"
            if [[ "$DRY_RUN" == true ]]; then
                [[ "$QUIET" == false ]] && echo "  would delete: $legacy_dir/ ($legacy_count files)"
            else
                rm -rf "$legacy_dir"
                mkdir -p "$legacy_dir"
                [[ "$QUIET" == false ]] && echo "  deleted: $legacy_dir/ ($legacy_count files)"
            fi
        fi
    fi
done
[[ "$QUIET" == false ]] && echo ""

# --- Policy: knowledge/pending/ — older than 14 days ---
prune_older_than "$AGENTS_DIR/knowledge/pending" 14 "*.md" "knowledge/pending"
[[ "$QUIET" == false ]] && echo ""

# --- Policy: rpi/ phase summaries — older than 30 days ---
prune_older_than "$AGENTS_DIR/rpi" 30 "phase-*-summary-*" "rpi/phase-summaries"
[[ "$QUIET" == false ]] && echo ""

# --- Policy: ao/sessions/ — keep last 50 ---
prune_keep_newest "$AGENTS_DIR/ao/sessions" 50 "ao/sessions"
[[ "$QUIET" == false ]] && echo ""

# --- Policy: handoff/ — keep last 10 ---
prune_keep_newest "$AGENTS_DIR/handoff" 10 "handoff"
[[ "$QUIET" == false ]] && echo ""

# --- Policy: opencode-tests/ — logs older than 7 days ---
prune_older_than "$AGENTS_DIR/opencode-tests" 7 "*.log" "opencode-tests"
prune_older_than "$AGENTS_DIR/opencode-tests" 7 "*.txt" "opencode-tests/summaries"
[[ "$QUIET" == false ]] && echo ""

# --- Policy: ao/subagent-outputs/ — keep last 50 ---
prune_keep_newest "$AGENTS_DIR/ao/subagent-outputs" 50 "ao/subagent-outputs"
[[ "$QUIET" == false ]] && echo ""

# --- Policy: releases/local-ci/ — keep last 3 runs ---
# Local CI validation runs dump ~2GB each (scanner output, SBOMs, etc.)
if [[ -d "$AGENTS_DIR/releases/local-ci" ]]; then
    ci_runs=$(find "$AGENTS_DIR/releases/local-ci" -maxdepth 1 -mindepth 1 -type d 2>/dev/null | wc -l | tr -d ' ')
    keep_ci=3
    if [[ "$ci_runs" -gt "$keep_ci" ]]; then
        to_delete_ci=$((ci_runs - keep_ci))
        [[ "$QUIET" == false ]] && echo "[releases/local-ci] $ci_runs runs — keeping newest $keep_ci, pruning $to_delete_ci"
        find "$AGENTS_DIR/releases/local-ci" -maxdepth 1 -mindepth 1 -type d -print0 2>/dev/null \
            | xargs -0 ls -dt 2>/dev/null \
            | tail -n "$to_delete_ci" \
            | while read -r d; do
                local_size=$(du -sk "$d" 2>/dev/null | cut -f1 || echo 0)
                TOTAL_FILES=$((TOTAL_FILES + 1))
                if [[ "$DRY_RUN" == true ]]; then
                    [[ "$QUIET" == false ]] && echo "  would delete: $d (~${local_size}KB)"
                else
                    rm -rf "$d"
                    [[ "$QUIET" == false ]] && echo "  deleted: $d (~${local_size}KB)"
                fi
            done
    else
        [[ "$QUIET" == false ]] && echo "[releases/local-ci] $ci_runs runs — within limit ($keep_ci). Nothing to prune."
    fi
fi
[[ "$QUIET" == false ]] && echo ""

# --- Policy: vibe/ vibecheck/ — keep last 20 ---
prune_keep_newest "$AGENTS_DIR/vibe" 20 "vibe"
prune_keep_newest "$AGENTS_DIR/vibecheck" 20 "vibecheck"
[[ "$QUIET" == false ]] && echo ""

# --- Policy: brainstorm/ — keep last 10 ---
prune_keep_newest "$AGENTS_DIR/brainstorm" 10 "brainstorm"
[[ "$QUIET" == false ]] && echo ""

# --- Policy: compaction-snapshots/ — older than 7 days ---
prune_older_than "$AGENTS_DIR/compaction-snapshots" 7 "*.md" "compaction-snapshots"
[[ "$QUIET" == false ]] && echo ""

# --- Policy: swarm/ — keep last 10 ---
prune_keep_newest "$AGENTS_DIR/swarm" 10 "swarm"
[[ "$QUIET" == false ]] && echo ""

# --- Policy: status dashboards — keep last 5 ---
if [[ -d "$AGENTS_DIR" ]]; then
    dashboard_count=$(find "$AGENTS_DIR" -maxdepth 1 -name "status-dashboard*" -type f 2>/dev/null | wc -l | tr -d ' ')
    if [[ "$dashboard_count" -gt 5 ]]; then
        to_delete_dash=$((dashboard_count - 5))
        [[ "$QUIET" == false ]] && echo "[status-dashboards] $dashboard_count files — keeping newest 5, pruning $to_delete_dash"
        find "$AGENTS_DIR" -maxdepth 1 -name "status-dashboard*" -type f -print0 2>/dev/null \
            | xargs -0 ls -t 2>/dev/null \
            | tail -n "$to_delete_dash" \
            | while read -r f; do
                if [[ "$DRY_RUN" == true ]]; then
                    [[ "$QUIET" == false ]] && echo "  would delete: $f"
                else
                    rm -f "$f"
                    [[ "$QUIET" == false ]] && echo "  deleted: $f"
                fi
                TOTAL_FILES=$((TOTAL_FILES + 1))
            done
    fi
fi
[[ "$QUIET" == false ]] && echo ""

# --- Policy: archived-worktrees/ — older than 7 days ---
if [[ -d "$AGENTS_DIR/archived-worktrees" ]]; then
    old_wt=$(find "$AGENTS_DIR/archived-worktrees" -maxdepth 1 -mindepth 1 -type d -mtime +7 2>/dev/null | wc -l | tr -d ' ')
    if [[ "$old_wt" -gt 0 ]]; then
        [[ "$QUIET" == false ]] && echo "[archived-worktrees] $old_wt directories older than 7d"
        find "$AGENTS_DIR/archived-worktrees" -maxdepth 1 -mindepth 1 -type d -mtime +7 -print0 2>/dev/null \
            | while IFS= read -r -d '' d; do
                if [[ "$DRY_RUN" == true ]]; then
                    [[ "$QUIET" == false ]] && echo "  would delete: $d"
                else
                    rm -rf "$d"
                    [[ "$QUIET" == false ]] && echo "  deleted: $d"
                fi
                TOTAL_FILES=$((TOTAL_FILES + 1))
            done
    else
        [[ "$QUIET" == false ]] && echo "[archived-worktrees] No directories older than 7d. Nothing to prune."
    fi
fi
[[ "$QUIET" == false ]] && echo ""

# --- Summary ---
echo "========================================"
if [[ "$DRY_RUN" == true ]]; then
    echo "DRY RUN COMPLETE"
    echo "Files that would be deleted: $TOTAL_FILES"
else
    echo "PRUNE COMPLETE"
    echo "Files deleted: $TOTAL_FILES"
fi
if [[ "$QUIET" == false ]]; then
    echo ""
    echo "Protected directories (never pruned):"
    echo "  learnings/ patterns/ plans/ research/ retros/"
fi
