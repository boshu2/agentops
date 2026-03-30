#!/usr/bin/env bash
# AgentOps Session Start Hook (manual mode)
# Creates .agents/ directories, consumes handoffs, injects skill context.
# MEMORY.md is auto-loaded by Claude Code — no extract/inject needed.

# Kill switches
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_SESSION_START_DISABLED:-}" = "1" ] && exit 0

# Worker environment sanitization
if [[ "${AGENTOPS_WORKER_SESSION:-}" == "1" ]]; then
    # Reset aliases to prevent interference
    unalias -a 2>/dev/null || true
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
PLUGIN_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"
AO_DIR="$ROOT/.agents/ao"

HOOK_ERROR_LOG="$AO_DIR/hook-errors.log"

log_hook_fail() {
    mkdir -p "$AO_DIR" 2>/dev/null || return 0
    echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) HOOK_FAIL: $1" >> "$HOOK_ERROR_LOG" 2>/dev/null || true
}

trim_lookup_text() {
    printf '%s' "${1:-}" \
        | tr '\n' ' ' \
        | tr -s '[:space:]' ' ' \
        | sed 's/^ //; s/ $//'
}

derive_lookup_query() {
    if [ -n "${AGENTOPS_SESSION_LOOKUP_QUERY:-}" ]; then
        trim_lookup_text "$AGENTOPS_SESSION_LOOKUP_QUERY"
        return 0
    fi
    if [ -n "${H_GOAL:-}" ]; then
        trim_lookup_text "$H_GOAL"
        return 0
    fi
    if [ -n "${H_SUMMARY:-}" ]; then
        trim_lookup_text "$H_SUMMARY"
        return 0
    fi
    return 0
}

detect_lookup_bead() {
    if ! command -v bd >/dev/null 2>&1; then
        return 0
    fi
    timeout 1 bd current 2>/dev/null | head -1 | tr -d '\r' | sed 's/^ //; s/ $//'
}

write_environment_manifest() {
    local env_file="$AO_DIR/environment.json"
    local tmp_file git_branch head_sha git_dirty tools_json manifest_json

    git_branch="$(git -C "$ROOT" branch --show-current 2>/dev/null || echo "")"
    head_sha="$(git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo "")"
    if git -C "$ROOT" diff --quiet 2>/dev/null && git -C "$ROOT" diff --cached --quiet 2>/dev/null; then
        if [ -z "$(git -C "$ROOT" ls-files --others --exclude-standard 2>/dev/null)" ]; then
            git_dirty=false
        else
            git_dirty=true
        fi
    else
        git_dirty=true
    fi

    if command -v jq &>/dev/null; then
        tools_json=$(jq -n \
            --arg ao "$(command -v ao 2>/dev/null || true)" \
            --arg git "$(command -v git 2>/dev/null || true)" \
            --arg jqbin "$(command -v jq 2>/dev/null || true)" '
            {
                ao: ($ao != ""),
                git: ($git != ""),
                jq: ($jqbin != "")
            }
        ')
        manifest_json=$(jq -n \
            --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
            --arg os "$(uname -s 2>/dev/null || echo unknown)" \
            --arg arch "$(uname -m 2>/dev/null || echo unknown)" \
            --arg root "$ROOT" \
            --arg branch "$git_branch" \
            --arg head_sha "$head_sha" \
            --argjson git_dirty "$git_dirty" \
            --argjson tools "$tools_json" '
            {
                timestamp: $ts,
                platform: {
                    os: $os,
                    arch: $arch
                },
                tools: $tools,
                git: {
                    repo_root: $root,
                    branch: $branch,
                    head_sha: $head_sha,
                    dirty: $git_dirty
                }
            }
        ')
        tmp_file="${env_file}.tmp"
        printf '%s\n' "$manifest_json" > "$tmp_file" 2>/dev/null && mv "$tmp_file" "$env_file" 2>/dev/null || true
    fi
}

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

write_environment_manifest

# Clear stale dedup flags from prior sessions (prevents cross-session suppression)
rm -f "$ROOT/.agents/ao/.intent-echo-fired" 2>/dev/null

# Auto-cleanup stale RPI runs (lightweight, <1s, dry-run only)
if command -v ao &>/dev/null; then
    ao rpi cleanup --all --stale-after 24h --dry-run >/dev/null 2>&1 || true
fi

# Auto-promote pending forge candidates (Tier 0 → Tier 1)
# Closes the gap where forge extracts knowledge at session end but promotion
# to .agents/learnings/ only happens when explicitly triggered.
if command -v ao &>/dev/null; then
    ao flywheel close-loop --quiet >/dev/null 2>&1 || true
fi

# Auto-gitignore .agents/
if [ "${AGENTOPS_GITIGNORE_AUTO:-1}" != "0" ] && [ -d "$ROOT/.git" ]; then
    GITIGNORE="$ROOT/.gitignore"
    if [ -f "$GITIGNORE" ]; then
        grep -q '\.agents/' "$GITIGNORE" 2>/dev/null || \
            printf '\n# AgentOps session artifacts\n.agents/\n' >> "$GITIGNORE" 2>/dev/null
    else
        printf '# AgentOps session artifacts\n.agents/\n' > "$GITIGNORE" 2>/dev/null
    fi
fi
if [ ! -f "$ROOT/.agents/.gitignore" ]; then
    cat > "$ROOT/.agents/.gitignore" 2>/dev/null <<'EOF'
# Deny all by default — session artifacts must not leak to git.
*

# Allow this file and navigation signposts
!.gitignore
!README.md
!AGENTS.md

# Allow AGENTS.md signposts in subdirectories
!council/
council/*
!council/AGENTS.md

!defrag/
defrag/*

!evolve/
evolve/*
!evolve/cycle-history.jsonl
!evolve/fitness-0-baseline.json
!evolve/active-baseline.txt
!evolve/baselines/
evolve/baselines/*
!evolve/baselines/index.jsonl
!evolve/baselines/*.json

!handoff/
handoff/*
!handoff/AGENTS.md

!learnings/
learnings/*
!learnings/AGENTS.md

!patterns/
patterns/*
!patterns/AGENTS.md

!plans/
plans/*
!plans/AGENTS.md

!plugins/
plugins/*
!plugins/marketplace.json

!releases/
releases/*
!releases/*.md
!releases/evidence-only-closures/
releases/evidence-only-closures/*
!releases/evidence-only-closures/*.json

!research/
research/*

!retro/
retro/*

!retros/
retros/*

!rpi/
rpi/*
!rpi/AGENTS.md
!rpi/next-work.schema.md
EOF
fi

# Flywheel behavior
INJECTED_KNOWLEDGE=""

# Derive MEMORY.md path once (used for stale nudge)
MEMORY_DIR="$HOME/.claude/projects"
PROJECT_PATH=$(printf '%s' "$ROOT" | tr '/' '-')
MEMORY_FILE="$MEMORY_DIR/$PROJECT_PATH/memory/MEMORY.md"
MEMORY_AGE_DAYS=-1
if [ -f "$MEMORY_FILE" ]; then
    MTIME=$(stat -f %m "$MEMORY_FILE" 2>/dev/null || stat -c %Y "$MEMORY_FILE" 2>/dev/null || echo "")
    if [ -n "$MTIME" ]; then
        MEMORY_AGE_DAYS=$(( ($(date +%s) - MTIME) / 86400 ))
    fi
fi

# Structured handoff consumption (ao handoff JSON artifacts)
HANDOFF_CONTEXT=""
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
            H_CONTINUATION=$(jq -r '.continuation // empty' "$CONSUMING" 2>/dev/null)
            H_TYPE=$(jq -r '.type // "manual"' "$CONSUMING" 2>/dev/null)
            # Finalize: write consumed metadata and rename to .consumed.json
            CONSUMED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
            jq --arg t "$CONSUMED_AT" '.consumed=true | .consumed_at=$t' \
                "$CONSUMING" > "${CONSUMING}.tmp" 2>/dev/null \
                && mv "${CONSUMING}.tmp" "${HANDOFF_JSON%.json}.consumed.json" 2>/dev/null \
                && rm -f "$CONSUMING" 2>/dev/null
            # Build injection context
            HANDOFF_CONTEXT="### Handoff Context (${H_TYPE})"
            [ -n "$H_GOAL" ] && HANDOFF_CONTEXT="${HANDOFF_CONTEXT}
- **Goal:** ${H_GOAL}"
            [ -n "$H_SUMMARY" ] && HANDOFF_CONTEXT="${HANDOFF_CONTEXT}
- **Summary:** ${H_SUMMARY}"
            [ -n "$H_CONTINUATION" ] && HANDOFF_CONTEXT="${HANDOFF_CONTEXT}
- **Continue:** ${H_CONTINUATION}"
            HANDOFF_CONTEXT="${HANDOFF_CONTEXT}
- **Source:** ${HANDOFF_JSON}"
        fi
    fi
fi

# Predecessor handoff discovery
PREDECESSOR_FILE="${GT_PREDECESSOR_HANDOFF:-}"
if [ -z "$PREDECESSOR_FILE" ] && [ -d "$ROOT/.agents/handoff" ]; then
    PREDECESSOR_FILE=$(ls -t "$ROOT/.agents/handoff/"*.md 2>/dev/null | head -1)
fi

# Build injection context (manual mode — MEMORY.md auto-loaded by Claude Code)
INJECTED_KNOWLEDGE="MEMORY.md is auto-loaded by Claude Code for this project.
For on-demand retrieval: \`ao search \"<query>\"\` or \`ao lookup --query \"<query>\"\`"
if [ -n "$HANDOFF_CONTEXT" ]; then
    INJECTED_KNOWLEDGE="${INJECTED_KNOWLEDGE}

${HANDOFF_CONTEXT}"
fi
if [ -n "$PREDECESSOR_FILE" ] && [ -f "$PREDECESSOR_FILE" ]; then
    INJECTED_KNOWLEDGE="${INJECTED_KNOWLEDGE}
Predecessor handoff: ${PREDECESSOR_FILE}"
fi

# Point to knowledge signpost (agents search on demand)
if [ -f "${ROOT:-.}/.agents/AGENTS.md" ]; then
    INJECTED_KNOWLEDGE="${INJECTED_KNOWLEDGE}

Knowledge artifacts are in \`.agents/\`. See \`.agents/AGENTS.md\` for navigation.
Use \`ao lookup --query \"topic\"\` for learnings retrieval, or \`Grep\` in \`.agents/learnings/\` as fallback."
else
    INJECTED_KNOWLEDGE="${INJECTED_KNOWLEDGE}

Knowledge artifacts are in \`.agents/\` (if populated).
Use \`ao lookup --query \"topic\"\` for on-demand learnings retrieval."
fi

# Auto-retrieve and cite top learnings for this session (closes citation gap)
if command -v ao &>/dev/null; then
    LOOKUP_QUERY="$(derive_lookup_query)"
    LOOKUP_BEAD="$(detect_lookup_bead)"
    LOOKUP_CONTEXT=""
    LOOKUP_ARGS=(lookup --limit 5)
    if [ -n "$LOOKUP_QUERY" ]; then
        LOOKUP_ARGS+=(--query "$LOOKUP_QUERY")
        LOOKUP_CONTEXT="query=\"$LOOKUP_QUERY\""
    fi
    if [ -n "$LOOKUP_BEAD" ]; then
        LOOKUP_ARGS+=(--bead "$LOOKUP_BEAD")
        if [ -n "$LOOKUP_CONTEXT" ]; then
            LOOKUP_CONTEXT="${LOOKUP_CONTEXT}, "
        fi
        LOOKUP_CONTEXT="${LOOKUP_CONTEXT}bead=${LOOKUP_BEAD}"
    fi
    if [ -z "$LOOKUP_CONTEXT" ]; then
        LOOKUP_CONTEXT="task context unavailable"
    fi
    if [ -n "$LOOKUP_QUERY" ] || [ -n "$LOOKUP_BEAD" ]; then
        FLYWHEEL_KNOWLEDGE=$(timeout 5 ao "${LOOKUP_ARGS[@]}" 2>/dev/null || true)
    else
        FLYWHEEL_KNOWLEDGE=""
    fi
    if [ -n "$FLYWHEEL_KNOWLEDGE" ]; then
        INJECTED_KNOWLEDGE="${INJECTED_KNOWLEDGE}

### Prior Learnings (auto-retrieved: ${LOOKUP_CONTEXT})
${FLYWHEEL_KNOWLEDGE}"
    fi
fi

# Keep startup context lean: inject only fresh flywheel knowledge and a short skill pointer.
SKILL_FILE="${PLUGIN_ROOT}/skills/using-agentops/SKILL.md"
if [ -f "$SKILL_FILE" ]; then
    using_agentops_hint="AgentOps workflow context is available. Use the Skill tool to read ${SKILL_FILE} when needed."
else
    using_agentops_hint="(AgentOps skill content unavailable at ${SKILL_FILE})"
fi

# Truncation (static cap — no notebook mode logic needed)
MAX_INJECT_CHARS=4000
if [ -n "$INJECTED_KNOWLEDGE" ] && [ "${#INJECTED_KNOWLEDGE}" -gt "$MAX_INJECT_CHARS" ]; then
    # Truncate at last newline within budget (never mid-line)
    trimmed="${INJECTED_KNOWLEDGE:0:$MAX_INJECT_CHARS}"
    INJECTED_KNOWLEDGE="${trimmed%
*}

*[truncated by session-start hook]*"
fi

# Nudge agent if MEMORY.md is stale or missing
if [ -n "${MEMORY_FILE:-}" ] && [ -f "$MEMORY_FILE" ] && [ "${MEMORY_AGE_DAYS:-0}" -gt 7 ]; then
    INJECTED_KNOWLEDGE="${INJECTED_KNOWLEDGE}

*Note: Your MEMORY.md hasn't been updated in ${MEMORY_AGE_DAYS} days. Consider running \`ao notebook update\` or updating it manually.*"
fi

if [ -n "$INJECTED_KNOWLEDGE" ]; then
    full_content=$(cat <<HOOKCTX
<AGENTOPS_CONTEXT>
${INJECTED_KNOWLEDGE}

${using_agentops_hint}
</AGENTOPS_CONTEXT>
HOOKCTX
)
else
    full_content=$(cat <<HOOKCTX
<AGENTOPS_CONTEXT>
No prior flywheel knowledge was injected for this session.
${using_agentops_hint}
</AGENTOPS_CONTEXT>
HOOKCTX
)
fi

if command -v jq &>/dev/null; then
    additional_context=$(printf '%s' "$full_content" | jq -Rs '.')
    cat <<HOOKEOF
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": ${additional_context}
  }
}
HOOKEOF
else
    # Minimal fallback — escape newlines and quotes
    escaped=$(printf '%s' "$full_content" | sed 's/\\/\\\\/g; s/"/\\"/g' | tr '\n' ' ')
    cat <<HOOKEOF
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "${escaped}"
  }
}
HOOKEOF
fi

exit 0
