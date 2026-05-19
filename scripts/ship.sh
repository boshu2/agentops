#!/usr/bin/env bash
# ship.sh — single-command bot-paired fast-lane PR cycle.
#
# Removes the choice to skip the discipline encoded in
# skills/ship-loop/SKILL.md.  In particular, anti-pattern #1 of that skill
# ("Running --fast pre-push on an inventory-touching PR") is enforced
# mechanically here: this script DETECTS inventory-touching changes and
# routes through the FULL pre-push gate (no --fast) automatically.  The
# operator no longer chooses; the script chooses.
#
# What it does:
#   1. Detect inventory touch from working-tree + staged diff
#   2. If inventory: run the regen scripts (sync-skill-counts, codex-hashes,
#      domain-map, context-map, registry, sync-hooks) preemptively so the
#      gates see consistent state
#   3. Run pre-push-gate.sh in full mode (inventory PR) or --fast (routine)
#   4. On green: stage all changes (regens included), exit 0 with a clean
#      "ready to commit + push" next-step report
#   5. On red: exit 2 with a clear reason — same as the gate itself
#
# What it does NOT do (yet):
#   - commit (operator writes the message; tracked separately)
#   - push or open PR (one extra command after green)
#   - close the bead
#
# Usage:
#   scripts/ship.sh                    # auto-detect, auto-regen, auto-gate
#   scripts/ship.sh --dry-run          # show plan, no execution
#   scripts/ship.sh --force-fast       # escape hatch: use --fast even on
#                                       # inventory diff (NOT recommended;
#                                       # accept full responsibility for the
#                                       # registries-drift class if you use it)
#   scripts/ship.sh --no-regen         # skip the regen sweep (useful if you
#                                       # have committed regens already)
#   scripts/ship.sh --gate full|fast   # force the gate mode explicitly
#
# Exit codes:
#   0  Ready to commit + push (or already committed; gate passed)
#   2  Pre-push gate BLOCKED — see gate output for which check failed
#   3  Preconditions failed (on main branch, no remote, etc.)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

DRY_RUN=false
FORCE_GATE=""      # "" | "full" | "fast"
SKIP_REGEN=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)
            sed -nE 's/^# ?(.*)/\1/p' "$0" | head -40
            exit 0
            ;;
        --dry-run) DRY_RUN=true; shift ;;
        --force-fast) FORCE_GATE="fast"; shift ;;
        --no-regen) SKIP_REGEN=true; shift ;;
        --gate)
            case "${2:-}" in
                full|fast) FORCE_GATE="$2"; shift 2 ;;
                *) echo "ship: --gate requires full|fast" >&2; exit 3 ;;
            esac
            ;;
        *) echo "ship: unknown flag: $1" >&2; exit 3 ;;
    esac
done

# --- Preconditions ---------------------------------------------------------

branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
if [[ -z "$branch" || "$branch" == "HEAD" ]]; then
    echo "ship: not on a named branch (detached HEAD?). Create a feature branch first." >&2
    exit 3
fi
if [[ "$branch" == "main" || "$branch" == "master" ]]; then
    echo "ship: on '$branch' — branch off main before shipping." >&2
    exit 3
fi

# Inventory-touch detection patterns.  Each surface here triggers ~1-15
# inventory validators that --fast mode would skip.  Anti-pattern #1 in
# skills/ship-loop catalogs the empirical cost (PR #332 burned ~90 min
# of CI cycles on this class).
INVENTORY_PATTERNS=(
    '^skills/'
    '^skills-codex/'
    '^skills-codex-overrides/'
    '^docs/contracts/'
    '^docs/learnings/'
    '^docs/reference/'
    '^schemas/'
    '^SKILL-TIERS\.md$'
    '^PRODUCT\.md$'
    '^README\.md$'
    '^registry\.json$'
    '^\.github/workflows/'
    '^cli/docs/COMMANDS\.md$'
)

detect_inventory_touch() {
    local files
    files="$(
        {
            # Modified tracked files (staged + working tree)
            git diff --name-only HEAD 2>/dev/null
            git diff --name-only --cached 2>/dev/null
            # New untracked files (e.g., skills/new-skill/SKILL.md before first add)
            git ls-files --others --exclude-standard 2>/dev/null
            # Commits on this branch not yet on origin/main
            git diff --name-only "$(git merge-base HEAD origin/main 2>/dev/null || echo HEAD)..HEAD" 2>/dev/null
        } | sort -u | sed '/^[[:space:]]*$/d'
    )"
    if [[ -z "$files" ]]; then
        return 1   # no changes at all
    fi
    local pattern
    for pattern in "${INVENTORY_PATTERNS[@]}"; do
        if echo "$files" | grep -qE "$pattern"; then
            return 0
        fi
    done
    return 1
}

# --- Mode selection --------------------------------------------------------

case "$FORCE_GATE" in
    full)
        gate_mode="full"
        reason="--gate full (operator override)"
        ;;
    fast)
        gate_mode="fast"
        reason="--force-fast or --gate fast (operator override; ANTI-PATTERN #1 if inventory)"
        ;;
    *)
        if detect_inventory_touch; then
            gate_mode="full"
            reason="inventory-touching diff detected (per ship-loop anti-pattern #1)"
        else
            gate_mode="fast"
            reason="routine diff (no inventory surfaces touched)"
        fi
        ;;
esac

echo "ship: branch=$branch  gate=$gate_mode  ($reason)"

if [[ "$DRY_RUN" == "true" ]]; then
    regen_msg="$([[ "$gate_mode" == "full" ]] && echo "yes" || echo "no")"
    gate_msg="$([[ "$gate_mode" == "fast" ]] && echo "pre-push-gate.sh --fast" || echo "pre-push-gate.sh (full)")"
    echo "ship: --dry-run — would run regen sweep: $regen_msg; then $gate_msg"
    exit 0
fi

# --- Regen sweep (inventory PRs only) --------------------------------------

if [[ "$gate_mode" == "full" && "$SKIP_REGEN" != "true" ]]; then
    echo "ship: running regen sweep (inventory PRs need synced derivatives)"

    run_regen() {
        local name="$1" cmd="$2"
        if eval "$cmd" >/dev/null 2>&1; then
            echo "  ok:   $name"
        else
            echo "  WARN: $name failed (non-fatal; gate may catch it)"
        fi
    }

    [[ -x scripts/sync-skill-counts.sh ]] && \
        run_regen "sync-skill-counts" "bash scripts/sync-skill-counts.sh"
    [[ -x scripts/regen-codex-hashes.sh ]] && \
        run_regen "regen-codex-hashes" "bash scripts/regen-codex-hashes.sh"
    [[ -x scripts/generate-skill-domain-map.sh ]] && \
        run_regen "generate-skill-domain-map" "bash scripts/generate-skill-domain-map.sh"
    [[ -x scripts/generate-context-map.sh ]] && \
        run_regen "generate-context-map" "bash scripts/generate-context-map.sh"
    [[ -x scripts/generate-registry.sh ]] && \
        run_regen "generate-registry" "bash scripts/generate-registry.sh"
    if [[ -f cli/Makefile ]] && grep -q '^sync-hooks:' cli/Makefile; then
        run_regen "cli sync-hooks" "(cd cli && make sync-hooks)"
    fi
fi

# --- Pre-push gate ---------------------------------------------------------

gate_args=()
if [[ "$gate_mode" == "fast" ]]; then
    gate_args=(--fast)
fi

echo "ship: running pre-push-gate.sh ${gate_args[*]:-(full)}"
if bash scripts/pre-push-gate.sh "${gate_args[@]}"; then
    echo ""
    echo "ship: pre-push gate PASSED."
    echo ""
    echo "Next steps (operator):"
    echo "  1. Review changes:           git status && git diff"
    echo "  2. Commit:                   git add -A && git commit -m '...'"
    echo "  3. Push + auto-merge:        git push && gh pr create + gh pr merge <num> --squash --auto"
    echo "  4. Close bead on merge:      bd close <bead-id> --reason 'Merged via PR #<num>'"
    exit 0
else
    rc=$?
    echo "" >&2
    echo "ship: pre-push gate BLOCKED. Address the failing check(s) above and re-run." >&2
    echo "ship: in inventory-PR mode, --fast skips ~15 validators — DO NOT use --force-fast to bypass." >&2
    exit "$rc"
fi
