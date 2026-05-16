#!/usr/bin/env bash
# AgentOps Session Start Hook
# Creates .agents/ directories, consumes handoffs, and prepares runtime state.
# CLAUDE.md owns the operator-facing startup surface; this hook stays silent.
# practices: [wiki-knowledge-surface, pragmatic-programmer]

# Kill switches
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_SESSION_START_DISABLED:-}" = "1" ] && exit 0

# Worker environment sanitization
if [[ "${AGENTOPS_WORKER_SESSION:-}" == "1" ]]; then
    # Reset aliases to prevent interference
    unalias -a 2>/dev/null || true
fi

# shellcheck disable=SC2034 # available for helper sourcing
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"
AO_DIR="$ROOT/.agents/ao"
FRESH_REPO=0
if [ ! -e "$ROOT/.agents" ]; then
    FRESH_REPO=1
fi

HOOK_ERROR_LOG="$AO_DIR/hook-errors.log"
if [ -f "$SCRIPT_DIR/../lib/hook-helpers.sh" ]; then
    # shellcheck source=../lib/hook-helpers.sh
    . "$SCRIPT_DIR/../lib/hook-helpers.sh"
elif [ -f "$SCRIPT_DIR/hook-helpers.sh" ]; then
    # shellcheck source=../lib/hook-helpers.sh
    . "$SCRIPT_DIR/hook-helpers.sh"
fi

# shellcheck disable=SC2329 # utility available for future use
log_hook_fail() {
    mkdir -p "$AO_DIR" 2>/dev/null || return 0
    echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) HOOK_FAIL: $1" >> "$HOOK_ERROR_LOG" 2>/dev/null || true
}

migrate_codex_hooks_feature_flag() {
    [ -n "${CODEX_THREAD_ID:-}" ] || [ -n "${CODEX_HOME:-}" ] || return 0

    local codex_home config_file tmp_file
    codex_home="${CODEX_HOME:-$HOME/.codex}"
    config_file="$codex_home/config.toml"
    [ -f "$config_file" ] || return 0
    grep -qE '^[[:space:]]*codex_hooks[[:space:]]*=' "$config_file" 2>/dev/null || return 0

    tmp_file="$(mktemp)" || return 0
    if awk '
        function is_section(line) {
            return line ~ /^[[:space:]]*\[[^]]+\][[:space:]]*$/
        }
        function trim(value) {
            sub(/^[[:space:]]+/, "", value)
            sub(/[[:space:]]+$/, "", value)
            return value
        }
        function emit_pending_hooks() {
            if (in_features && !features_has_hooks && legacy_value != "") {
                print "hooks = " legacy_value
            }
        }
        FNR == NR {
            if (is_section($0)) {
                in_features = ($0 ~ /^[[:space:]]*\[features\][[:space:]]*$/)
                next
            }
            if (!in_features) {
                next
            }
            if ($0 ~ /^[[:space:]]*hooks[[:space:]]*=/) {
                features_has_hooks = 1
                next
            }
            if (legacy_value == "" && $0 ~ /^[[:space:]]*codex_hooks[[:space:]]*=/) {
                legacy_value = $0
                sub(/^[^=]*=/, "", legacy_value)
                legacy_value = trim(legacy_value)
            }
            next
        }
        FNR == 1 {
            in_features = 0
        }
        is_section($0) {
            emit_pending_hooks()
            in_features = ($0 ~ /^[[:space:]]*\[features\][[:space:]]*$/)
            print
            next
        }
        in_features && $0 ~ /^[[:space:]]*codex_hooks[[:space:]]*=/ {
            next
        }
        { print }
        END {
            emit_pending_hooks()
        }
    ' "$config_file" "$config_file" > "$tmp_file"; then
        mv "$tmp_file" "$config_file" 2>/dev/null || rm -f "$tmp_file" 2>/dev/null
    else
        rm -f "$tmp_file" 2>/dev/null
    fi
}

migrate_codex_hooks_feature_flag

# Stale/manual installs can miss helper updates. SessionStart must never create
# operator-facing noise, so fail open if the helper layer is unavailable.
if ! declare -F session_write_environment_manifest >/dev/null 2>&1; then
    exit 0
fi

cd "$ROOT" 2>/dev/null || true

# Ensure global .agents/ directories exist (cross-repo knowledge)
mkdir -p "$HOME/.agents/learnings" "$HOME/.agents/patterns" 2>/dev/null

# Ensure local .agents/ directories exist
for dir in .agents/research .agents/products .agents/retros .agents/learnings \
           .agents/patterns .agents/council .agents/knowledge/pending \
           .agents/plans .agents/rpi .agents/ao .agents/handoff \
           .agents/findings .agents/planning-rules .agents/pre-mortem-checks \
           .agents/constraints; do
    mkdir -p "$ROOT/$dir" 2>/dev/null
done

session_write_environment_manifest "$ROOT" "$AO_DIR"

rm -f "$ROOT/.agents/ao/.factory-router-fired" \
      "$ROOT/.agents/ao/.factory-intake-needed" \
      "$ROOT/.agents/ao/factory-goal.txt" \
      "$ROOT/.agents/ao/factory-briefing.txt" 2>/dev/null

# Auto-cleanup stale RPI runs (lightweight, <1s, dry-run only)
if command -v ao &>/dev/null; then
    ao rpi cleanup --all --stale-after 24h --dry-run >/dev/null 2>&1 || true
fi

# Auto-promote pending forge candidates (Tier 0 → Tier 1)
# Opt-in only: corpus-mutating maintenance must not run on every session start.
# Set AGENTOPS_STARTUP_CLOSE_LOOP=1 to enable. See learning explosion postmortem
# (mol-qwx4): unconditional startup close-loop + non-idempotent pending lifecycle
# turned a 47-file batch into 11k+ duplicate artifacts.
if [ "${AGENTOPS_STARTUP_CLOSE_LOOP:-0}" = "1" ] && command -v ao &>/dev/null; then
    ao flywheel close-loop --quiet >/dev/null 2>&1 || true
fi

# Always gitignore repo-root .agents/. It is local agent runtime state and must
# not be committed. AGENTOPS_GITIGNORE_AUTO is retained only as a legacy no-op.
if [ -d "$ROOT/.git" ]; then
    GITIGNORE="$ROOT/.gitignore"
    if [ -f "$GITIGNORE" ]; then
        # Match the modern anchored shapes (/.agents/, /.agents/*,
        # /.agents/**/*) AND any allowlist re-include (!/.agents/...).
        # The original `^/\.agents/$` was too narrow and double-appended
        # whenever the gitignore used the directory-glob shape.
        grep -qE '^!?/?\.agents(/|$)' "$GITIGNORE" 2>/dev/null || \
            printf '\n# AgentOps session artifacts\n/.agents/\n' >> "$GITIGNORE" 2>/dev/null
    else
        printf '# AgentOps session artifacts\n/.agents/\n' > "$GITIGNORE" 2>/dev/null
    fi
fi
# Reconcile the deny-all child .gitignore with the root .gitignore allowlist.
# AgentOps itself force-tracks audit-truth artifacts (.agents/findings/registry.jsonl,
# .agents/nightly/<date>/baseline-goals.json, etc.) via `!/.agents/...` re-includes;
# a blanket `*` deny-all child would override every parent re-include because
# gitignore's last-match-wins rule applies to the deeper file.
#
# - Root has `!/.agents/` allowlist → REMOVE any deny-all child that would
#   sabotage the allowlist (audit truth wins; this catches stale child files
#   from older hook versions or external tooling).
# - Root has no allowlist → INSTALL the deny-all child (external repos that
#   embed AgentOps still get the safety belt against runtime-state leaks).
if grep -qE '^!/?\.agents/' "$ROOT/.gitignore" 2>/dev/null; then
    if [ -f "$ROOT/.agents/.gitignore" ] && grep -qxE '\*' "$ROOT/.agents/.gitignore" 2>/dev/null; then
        rm -f "$ROOT/.agents/.gitignore" 2>/dev/null
    fi
elif [ ! -f "$ROOT/.agents/.gitignore" ]; then
    cat > "$ROOT/.agents/.gitignore" 2>/dev/null <<'EOF'
# Deny all by default — session artifacts must not leak to git.
*

# Allow only this file for local deny-by-default semantics.
!.gitignore
EOF
fi

STARTUP_CONTEXT_MODE="$(session_resolve_startup_context_mode)"
FACTORY_GOAL=""
FACTORY_BRIEFING_PATH=""

# Structured handoff consumption (ao handoff JSON artifacts)
if [ -d "$ROOT/.agents/handoff" ] && command -v jq &>/dev/null; then
    # Find newest unconsumed .json handoff (exclude .consumed.json and .consuming.json)
    HANDOFF_JSON=$(find "$ROOT/.agents/handoff" -maxdepth 1 -name 'handoff-*.json' \
        -not -name '*.consumed.json' -not -name '*.consuming.json' 2>/dev/null \
        | sort -r | head -1)
    if [ -n "$HANDOFF_JSON" ] && [ -f "$HANDOFF_JSON" ]; then
        # Atomic claim: mv to .consuming prevents concurrent session race
        CONSUMING="${HANDOFF_JSON%.json}.consuming.json"
        if mv "$HANDOFF_JSON" "$CONSUMING" 2>/dev/null; then
            H_GOAL=$(jq -r '.goal // empty' "$CONSUMING" 2>/dev/null)
            H_SUMMARY=$(jq -r '.summary // empty' "$CONSUMING" 2>/dev/null)
            # shellcheck disable=SC2034 # reserved for future handoff expansion
            H_CONTINUATION=$(jq -r '.continuation // empty' "$CONSUMING" 2>/dev/null)
            # shellcheck disable=SC2034 # reserved for handoff type routing
            H_TYPE=$(jq -r '.type // "manual"' "$CONSUMING" 2>/dev/null)
            # Finalize: write consumed metadata and rename to .consumed.json
            CONSUMED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
            jq --arg t "$CONSUMED_AT" '.consumed=true | .consumed_at=$t' \
                "$CONSUMING" > "${CONSUMING}.tmp" 2>/dev/null \
                && mv "${CONSUMING}.tmp" "${HANDOFF_JSON%.json}.consumed.json" 2>/dev/null \
                && rm -f "$CONSUMING" 2>/dev/null
        fi
    fi
fi

if [ "$STARTUP_CONTEXT_MODE" = "factory" ]; then
    FACTORY_GOAL="$(session_derive_lookup_query "${H_GOAL:-}" "${H_SUMMARY:-}")"
    if [ -n "$FACTORY_GOAL" ]; then
        printf '%s' "$FACTORY_GOAL" > "$ROOT/.agents/ao/factory-goal.txt" 2>/dev/null || true
        FACTORY_BRIEFING_PATH="$(session_build_factory_briefing "$FACTORY_GOAL")"
        if [ -n "$FACTORY_BRIEFING_PATH" ]; then
            printf '%s' "$FACTORY_BRIEFING_PATH" > "$ROOT/.agents/ao/factory-briefing.txt" 2>/dev/null || true
        else
            rm -f "$ROOT/.agents/ao/factory-briefing.txt" 2>/dev/null || true
        fi
    else
        : > "$ROOT/.agents/ao/.factory-intake-needed" 2>/dev/null || true
    fi
fi

exit 0
