#!/usr/bin/env bash
# AgentOps Session Start Hook
# Creates .agents/ directories and injects using-agentops context

# Note: no set -e — hooks must fail open (exit 0), not abort on errors

# Get plugin directory (where this script lives)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
CHAIN_LIB="$SCRIPT_DIR/../lib/chain-parser.sh"
if [ -f "$CHAIN_LIB" ]; then
    # shellcheck source=../lib/chain-parser.sh
    . "$CHAIN_LIB"
fi
PLUGIN_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"
AO_DIR="$ROOT/.agents/ao"
HOOK_ERROR_LOG="$AO_DIR/hook-errors.log"
HOOK_HELPERS_LIB="$SCRIPT_DIR/../lib/hook-helpers.sh"
if [ -f "$HOOK_HELPERS_LIB" ]; then
    # shellcheck source=../lib/hook-helpers.sh
    . "$HOOK_HELPERS_LIB"
fi

# Kill switches
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_SESSION_START_DISABLED:-}" = "1" ] && exit 0
AO_TIMEOUT_BIN="timeout"
command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1 || AO_TIMEOUT_BIN="gtimeout"

log_hook_fail() {
    local message="$1"
    mkdir -p "$AO_DIR" 2>/dev/null || return 0
    echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) HOOK_FAIL: ${message}" >> "$HOOK_ERROR_LOG" 2>/dev/null || true
}

run_ao_quick() {
    local seconds="$1"
    shift
    if command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1; then
        "$AO_TIMEOUT_BIN" "$seconds" ao "$@" 2>/dev/null
        return $?
    fi
    ao "$@" 2>/dev/null
}

# Ensure relative paths and ao commands are rooted to the active repo.
cd "$ROOT" 2>/dev/null || true

# Safety net — canonical setup is 'ao init'. This ensures dirs exist even if init wasn't run.
AGENTS_DIRS=(".agents/research" ".agents/products" ".agents/retros" ".agents/learnings" ".agents/patterns" ".agents/council" ".agents/knowledge/pending" ".agents/plans" ".agents/rpi" ".agents/ao")

for dir in "${AGENTS_DIRS[@]}"; do
    target_dir="$ROOT/$dir"
    if [[ ! -d "$target_dir" ]]; then
        mkdir -p "$target_dir" 2>/dev/null
    fi
done

# Auto-gitignore .agents/ in consumer repos
if [ "${AGENTOPS_GITIGNORE_AUTO:-1}" != "0" ] && [ -d "$ROOT/.git" ]; then
    {
        GITIGNORE="$ROOT/.gitignore"
        if [ -f "$GITIGNORE" ]; then
            if ! grep -q '\.agents/' "$GITIGNORE" 2>/dev/null; then
                printf '\n# AgentOps session artifacts (auto-added)\n.agents/\n' >> "$GITIGNORE"
                echo "Added .agents/ to .gitignore" >&2
            fi
        else
            printf '# AgentOps session artifacts (auto-added)\n.agents/\n' > "$GITIGNORE"
            echo "Created .gitignore with .agents/" >&2
        fi
    } 2>/dev/null || log_hook_fail "gitignore auto-setup"

    # Warn if .agents/ files are already tracked despite .gitignore
    if git -C "$ROOT" ls-files .agents/ 2>/dev/null | head -1 | grep -q .; then
        echo "Warning: .agents/ files are tracked in git despite .gitignore. Run: git rm -r --cached .agents/" >&2
    fi
fi

# Belt-and-suspenders: nested .agents/.gitignore
if [ ! -f "$ROOT/.agents/.gitignore" ]; then
    cat > "$ROOT/.agents/.gitignore" 2>/dev/null <<'NESTED_GITIGNORE'
# Do not commit this directory — session artifacts, absolute paths, sensitive output.
*
!.gitignore
!README.md
NESTED_GITIGNORE
fi

# Environment manifest — capture tool presence and git state for council legibility
{
  ENV_JSON="$ROOT/.agents/ao/environment.json"

  # Tool presence checks (command -v only, no version extraction)
  ao_present=false; command -v ao &>/dev/null && ao_present=true
  bd_present=false; command -v bd &>/dev/null && bd_present=true
  codex_present=false; command -v codex &>/dev/null && codex_present=true
  gt_present=false; command -v gt &>/dev/null && gt_present=true
  gh_present=false; command -v gh &>/dev/null && gh_present=true
  jq_present=false; command -v jq &>/dev/null && jq_present=true

  # Git state
  git_branch="$(git branch --show-current 2>/dev/null || echo 'unknown')"
  git_head="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
  git_dirty=false; [ -n "$(git status --porcelain 2>/dev/null)" ] && git_dirty=true

  # Missing tools list
  missing_tools=""
  $ao_present || missing_tools="${missing_tools}\"ao\","
  $bd_present || missing_tools="${missing_tools}\"bd\","
  $codex_present || missing_tools="${missing_tools}\"codex\","
  $gt_present || missing_tools="${missing_tools}\"gt\","
  $gh_present || missing_tools="${missing_tools}\"gh\","
  $jq_present || missing_tools="${missing_tools}\"jq\","
  missing_tools="[${missing_tools%,}]"

  # Platform
  platform="$(uname -s | tr '[:upper:]' '[:lower:]')"

  cat > "$ENV_JSON" <<ENVEOF
{
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "platform": "$platform",
  "tools": {"ao": $ao_present, "bd": $bd_present, "codex": $codex_present, "gt": $gt_present, "gh": $gh_present, "jq": $jq_present},
  "missing_tools": $missing_tools,
  "git": {"branch": "$git_branch", "head": "$git_head", "dirty": $git_dirty}
}
ENVEOF
} 2>/dev/null || log_hook_fail "environment manifest write failed"

# Clean up stale nudge dedup flag from previous session
rm -f "$ROOT/.agents/ao/.ratchet-advance-fired" 2>/dev/null

# Process pending extraction queue (closes forge → extract loop)
if command -v ao &>/dev/null; then
    run_ao_quick "${AGENTOPS_SESSION_START_EXTRACT_TIMEOUT:-5}" extract || true
fi

# Get flywheel status (brief one-liner for visibility)
flywheel_status=""
if command -v ao &>/dev/null; then
    # Try new structured command first
    if run_ao_quick "${AGENTOPS_SESSION_START_STATUS_TIMEOUT:-3}" flywheel nudge --help >/dev/null 2>&1; then
        nudge_json=$(run_ao_quick "${AGENTOPS_SESSION_START_STATUS_TIMEOUT:-3}" flywheel nudge -o json) || {
            log_hook_fail "ao flywheel nudge"
            nudge_json=""
        }
        if [ -n "$nudge_json" ] && command -v jq >/dev/null 2>&1; then
            status_line=$(echo "$nudge_json" | jq -r '.status // ""')
            velocity=$(echo "$nudge_json" | jq -r '.velocity // 0')
            sessions=$(echo "$nudge_json" | jq -r '.sessions_count // 0')
            learnings_count=$(echo "$nudge_json" | jq -r '.learnings_count // 0')
            pool_pending=$(echo "$nudge_json" | jq -r '.pool_pending // 0')
            if [[ -n "$status_line" ]]; then
                flywheel_status="**Flywheel:** [${status_line}] | ${sessions} sessions | ${learnings_count} learnings | ${pool_pending} pending | velocity: ${velocity}/week"
            fi
        fi
    fi

    # Fallback: old grep/tr parsing if new command unavailable or failed
    if [[ -z "$flywheel_status" ]]; then
        flywheel_output=$(run_ao_quick "${AGENTOPS_SESSION_START_STATUS_TIMEOUT:-3}" flywheel status) || {
            log_hook_fail "ao flywheel status"
            flywheel_output=""
        }
        if [[ -n "$flywheel_output" ]]; then
            # Parse the status line and velocity (tr -d removes newlines)
            status_line=$(echo "$flywheel_output" | grep -o '\[.*\]' | head -1 | tr -d '\n' || echo "[UNKNOWN]")
            velocity=$(echo "$flywheel_output" | grep "velocity:" | grep -o '[+-][0-9.]*' | tr -d '\n' || echo "?")
            sessions=$(run_ao_quick "${AGENTOPS_SESSION_START_STATUS_TIMEOUT:-3}" status | grep "^Sessions:" | awk '{print $2}' | head -1 | tr -d '\n' || echo "?")
            learnings_count=$(find "$ROOT"/.agents/learnings -maxdepth 1 -name '*.md' -type f 2>/dev/null | wc -l | tr -d ' \n' || echo "0")
            flywheel_status="**Flywheel:** ${status_line} | ${sessions} sessions | ${learnings_count} learnings | velocity: ${velocity}/week"
        fi
    fi
fi

# Get ratchet status (brief one-liner for visibility)
ratchet_status=""
ratchet_output=""
if command -v ao &>/dev/null; then
    if command -v jq >/dev/null 2>&1; then
        ratchet_json=$(run_ao_quick "${AGENTOPS_SESSION_START_STATUS_TIMEOUT:-3}" ratchet status -o json) || {
            log_hook_fail "ao ratchet status"
            ratchet_json=""
        }
        if [ -n "$ratchet_json" ]; then
            ratchet_output=$(echo "$ratchet_json" | jq -r '
                [.steps[] | "\(.step):\(.status)"] | join(" → ")
            ' 2>/dev/null)
        fi
    else
        ratchet_output=$(run_ao_quick "${AGENTOPS_SESSION_START_STATUS_TIMEOUT:-3}" ratchet status -o table | head -3) || {
            log_hook_fail "ao ratchet status"
            ratchet_output=""
        }
    fi
    if [[ -n "$ratchet_output" ]]; then
        ratchet_status="**Ratchet:** ${ratchet_output}"
    fi
fi

# Ratchet resume directive: suggest next RPI step if chain.jsonl exists
resume_directive=""
if [ "${AGENTOPS_AUTOCHAIN:-}" != "0" ] && command -v jq >/dev/null 2>&1; then
    # Try new structured command first
    if run_ao_quick "${AGENTOPS_SESSION_START_STATUS_TIMEOUT:-3}" ratchet next --help >/dev/null 2>&1; then
        next_json=$(run_ao_quick "${AGENTOPS_SESSION_START_STATUS_TIMEOUT:-3}" ratchet next -o json)
        if [ -n "$next_json" ]; then
            next_step=$(echo "$next_json" | jq -r '.next // ""')
            last_step=$(echo "$next_json" | jq -r '.last_step // ""')
            last_artifact=$(echo "$next_json" | jq -r '.last_artifact // ""')
            skill=$(echo "$next_json" | jq -r '.skill // ""')
            complete=$(echo "$next_json" | jq -r '.complete // false')

            if [ "$complete" = "true" ]; then
                resume_directive="RPI cycle complete. Run /post-mortem to extract learnings."
            elif [ -n "$next_step" ] && [ -n "$skill" ]; then
                artifact_arg=""
                if [ -n "$last_artifact" ]; then
                    artifact_arg=" $last_artifact"
                fi
                resume_directive="RESUMING FLYWHEEL: ${last_step} completed. Suggested next: ${skill}${artifact_arg}. Say SKIP to bypass."
            fi
        fi
    fi

    # Fallback: existing chain.jsonl walking code if new command unavailable or failed
    if [ -z "$resume_directive" ]; then
        CHAIN_FILE="$ROOT/.agents/ao/chain.jsonl"
        if [ -f "$CHAIN_FILE" ]; then
            # RPI step sequence
            RPI_STEPS="research plan pre-mortem implement vibe post-mortem"

            # Find the latest completed/locked step, handling both old and new schema
            # Old schema: {"gate":"<step>","status":"locked"} or {"status":"skipped"}
            # New schema: {"step":"<step>","locked":true}
            last_step=""
            last_timestamp=""
            last_artifact=""

            # Read chain.jsonl and find last completed entry
            while IFS= read -r line; do
                # Try new schema first: "step" field + "locked":true
                step_name=$(echo "$line" | jq -r 'if .step then .step else .gate // empty end' 2>/dev/null)
                is_done=$(echo "$line" | jq -r '
                    if .locked == true then "yes"
                    elif .status == "locked" then "yes"
                    elif .status == "skipped" then "yes"
                    else "no"
                    end
                ' 2>/dev/null)

                if [ "$is_done" = "yes" ] && [ -n "$step_name" ]; then
                    last_step="$step_name"
                    last_timestamp=$(echo "$line" | jq -r '.timestamp // .ts // empty' 2>/dev/null)
                    # Extract artifact path: "artifact" (old) or "output" (new)
                    raw_artifact=$(echo "$line" | jq -r '.artifact // .output // empty' 2>/dev/null)
                    # Sanitize: relative only, under .agents/, no ".."
                    if [ -n "$raw_artifact" ]; then
                        case "$raw_artifact" in
                            /*|*..*)  raw_artifact="" ;;  # reject absolute or traversal
                            .agents/*) last_artifact="$raw_artifact" ;;
                            *)         raw_artifact="" ;;  # reject paths not under .agents/
                        esac
                    fi
                fi
            done < "$CHAIN_FILE"

            # Determine next pending step
            if [ -n "$last_step" ]; then
                next_step=""
                found_last=false
                for s in $RPI_STEPS; do
                    if $found_last; then
                        next_step="$s"
                        break
                    fi
                    if [ "$s" = "$last_step" ]; then
                        found_last=true
                    fi
                done

                if [ -n "$next_step" ]; then
                    # Map step names to skill commands
                    skill_cmd="/$next_step"
                    artifact_arg=""
                    if [ -n "$last_artifact" ]; then
                        artifact_arg=" $last_artifact"
                    fi
                    ts_display="${last_timestamp:-unknown}"
                    resume_directive="RESUMING FLYWHEEL: ${last_step} completed at ${ts_display}. Suggested next: ${skill_cmd}${artifact_arg}. Say SKIP to bypass."
                fi
            fi
        fi
    fi
fi

# Check for auto-handoff recovery
# Priority: packet queue (v1) -> pending markers -> legacy auto-*.md.
handoff_section=""
HANDOFF_ROOT="$ROOT/.agents/handoff"
HANDOFF_PENDING_DIR="$HANDOFF_ROOT/pending"
HANDOFF_CONSUMED_DIR="$HANDOFF_ROOT/consumed"
HANDOFF_FILE=""
PACKET_ROOT="$ROOT/.agents/ao/packets"
PACKET_PENDING_DIR="$PACKET_ROOT/pending"
PACKET_CONSUMED_DIR="$PACKET_ROOT/consumed"
PACKET_QUARANTINE_DIR="$PACKET_ROOT/quarantine"

resolve_handoff_path() {
    local rel_path="$1"
    case "$rel_path" in
        .agents/*) printf '%s\n' "$ROOT/$rel_path" ;;
        /*)        printf '%s\n' "$rel_path" ;;
        *)         printf '%s\n' "$ROOT/$rel_path" ;;
    esac
}

# 1) Packet-first one-shot consumption (preferred)
if [ -d "$PACKET_PENDING_DIR" ]; then
    PACKET_FILE=$(find "$PACKET_PENDING_DIR" -maxdepth 1 -name '*.json' -print 2>/dev/null | sort -r | head -1)
    if [[ -n "$PACKET_FILE" && -f "$PACKET_FILE" ]]; then
        packet_valid=false
        if command -v validate_memory_packet_file >/dev/null 2>&1; then
            if validate_memory_packet_file "$PACKET_FILE"; then
                packet_valid=true
            fi
        elif command -v jq >/dev/null 2>&1; then
            if jq -e '.schema_version == 1 and (.packet_type | type == "string") and (.payload | type == "object")' "$PACKET_FILE" >/dev/null 2>&1; then
                packet_valid=true
            fi
        fi

        if $packet_valid; then
            packet_type="packet"
            packet_handoff_path=""
            packet_summary=""
            handoff_content=""

            if command -v jq >/dev/null 2>&1; then
                packet_type=$(jq -r '.packet_type // "packet"' "$PACKET_FILE" 2>/dev/null)
                rel_path=$(jq -r '.handoff_file // ""' "$PACKET_FILE" 2>/dev/null)
                packet_summary=$(jq -r '.payload.summary // .payload.last_assistant_message // ""' "$PACKET_FILE" 2>/dev/null)
                if [[ -n "$rel_path" && "$rel_path" != "null" ]]; then
                    packet_handoff_path=$(resolve_handoff_path "$rel_path")
                fi
            fi

            if [[ -n "$packet_handoff_path" && -f "$packet_handoff_path" ]]; then
                handoff_content=$(cat "$packet_handoff_path" 2>/dev/null || echo "")
            fi
            if [[ -z "$handoff_content" && -n "$packet_summary" ]]; then
                handoff_content="$packet_summary"
            fi
            if [ -z "$handoff_content" ] && command -v jq >/dev/null 2>&1; then
                handoff_content=$(jq -r '.payload | tostring' "$PACKET_FILE" 2>/dev/null)
            fi

            if [[ -n "$handoff_content" ]]; then
                handoff_section="

---
## 🔄 Recovery: Memory Packet (${packet_type})

${handoff_content}
---
"
            fi

            mkdir -p "$PACKET_CONSUMED_DIR" 2>/dev/null || log_hook_fail "packet consumed dir create failed"
            consumed_packet="$PACKET_CONSUMED_DIR/$(basename "$PACKET_FILE")"
            if command -v jq >/dev/null 2>&1; then
                consumed_ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
                jq --arg ts "$consumed_ts" '.consumed = true | .consumed_at = $ts' "$PACKET_FILE" > "$consumed_packet" 2>/dev/null \
                    || cp "$PACKET_FILE" "$consumed_packet" 2>/dev/null
                rm -f "$PACKET_FILE" 2>/dev/null
            else
                mv "$PACKET_FILE" "$consumed_packet" 2>/dev/null || true
            fi
        else
            mkdir -p "$PACKET_QUARANTINE_DIR" 2>/dev/null || log_hook_fail "packet quarantine dir create failed"
            quarantined_packet="$PACKET_QUARANTINE_DIR/$(basename "$PACKET_FILE")"
            if command -v jq >/dev/null 2>&1; then
                quarantined_ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
                jq --arg ts "$quarantined_ts" --arg reason "schema_validation_failed" \
                    '.quarantined = true | .quarantined_at = $ts | .quarantine_reason = $reason' \
                    "$PACKET_FILE" > "$quarantined_packet" 2>/dev/null || mv "$PACKET_FILE" "$quarantined_packet" 2>/dev/null
            else
                mv "$PACKET_FILE" "$quarantined_packet" 2>/dev/null || true
            fi
            rm -f "$PACKET_FILE" 2>/dev/null || true
            log_hook_fail "malformed packet quarantined: $(basename "$PACKET_FILE")"
        fi
    fi
fi

# 2) Marker-based one-shot consumption
if [[ -z "$handoff_section" && -d "$HANDOFF_PENDING_DIR" ]]; then
    HANDOFF_MARKER=$(find "$HANDOFF_PENDING_DIR" -maxdepth 1 -name '*.json' -print 2>/dev/null | sort -r | head -1)
    if [[ -n "$HANDOFF_MARKER" && -f "$HANDOFF_MARKER" ]]; then
        HANDOFF_PATH=""
        if command -v jq >/dev/null 2>&1; then
            rel_path=$(jq -r '.handoff_file // ""' "$HANDOFF_MARKER" 2>/dev/null)
            if [[ -n "$rel_path" && "$rel_path" != "null" ]]; then
                HANDOFF_PATH=$(resolve_handoff_path "$rel_path")
            fi
        else
            rel_path=$(grep -o '"handoff_file"[[:space:]]*:[[:space:]]*"[^"]*"' "$HANDOFF_MARKER" | head -1 | sed 's/.*"handoff_file"[[:space:]]*:[[:space:]]*"//;s/"$//')
            if [[ -n "$rel_path" ]]; then
                HANDOFF_PATH=$(resolve_handoff_path "$rel_path")
            fi
        fi

        if [[ -n "$HANDOFF_PATH" && -f "$HANDOFF_PATH" ]]; then
            handoff_content=$(cat "$HANDOFF_PATH" 2>/dev/null || echo "")
            if [[ -n "$handoff_content" ]]; then
                handoff_section="

---
## 🔄 Recovery: Auto-Handoff

${handoff_content}
---
"

                mkdir -p "$HANDOFF_CONSUMED_DIR" 2>/dev/null || log_hook_fail "handoff consumed dir create failed"
                consumed_marker="$HANDOFF_CONSUMED_DIR/$(basename "$HANDOFF_MARKER")"
                if command -v jq >/dev/null 2>&1; then
                    consumed_ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
                    jq --arg ts "$consumed_ts" '.consumed = true | .consumed_at = $ts' "$HANDOFF_MARKER" > "$consumed_marker" 2>/dev/null \
                        || cp "$HANDOFF_MARKER" "$consumed_marker" 2>/dev/null
                    rm -f "$HANDOFF_MARKER" 2>/dev/null
                else
                    mv "$HANDOFF_MARKER" "$consumed_marker" 2>/dev/null || true
                fi
            fi
        fi
    fi
fi

# 3) Legacy fallback: consume latest auto-*.md from precompact
if [[ -z "$handoff_section" ]]; then
    HANDOFF_FILE=$(find "$HANDOFF_ROOT/" -maxdepth 1 -name 'auto-*.md' -print 2>/dev/null | sort -r | head -1)
    if [[ -n "$HANDOFF_FILE" && -f "$HANDOFF_FILE" ]]; then
        handoff_content=$(cat "$HANDOFF_FILE" 2>/dev/null || echo "")
        if [[ -n "$handoff_content" ]]; then
            handoff_section="

---
## 🔄 Recovery: Auto-Handoff from Pre-Compaction

${handoff_content}
---
"
            # Legacy behavior: delete after one read
            rm -f "$HANDOFF_FILE" 2>/dev/null
        fi
    fi
fi

# Prune check — auto-execute when AGENTOPS_PRUNE_AUTO=1, otherwise dry-run only
if [ "${AGENTOPS_HOOKS_DISABLED:-}" != "1" ] && [ -x "$PLUGIN_ROOT/scripts/prune-agents.sh" ]; then
    FCOUNT=$(find "$ROOT/.agents" -type f 2>/dev/null | wc -l | tr -d ' ')
    if [ "${FCOUNT:-0}" -gt 500 ]; then
        if [ "${AGENTOPS_PRUNE_AUTO:-}" = "1" ]; then
            "$PLUGIN_ROOT/scripts/prune-agents.sh" --execute --quiet > "$AO_DIR/prune-auto.log" 2>&1 || true
            echo "🗑️ .agents/ had $FCOUNT files. Auto-pruned. Log: $AO_DIR/prune-auto.log" >&2
            echo "   Disable with: AGENTOPS_PRUNE_AUTO=0" >&2
        else
            "$PLUGIN_ROOT/scripts/prune-agents.sh" > "$AO_DIR/prune-dry-run.log" 2>&1 || true
            echo "⚠️ .agents/ has $FCOUNT files. Prune preview: $AO_DIR/prune-dry-run.log" >&2
            echo "   Enable auto-prune: AGENTOPS_PRUNE_AUTO=1" >&2
        fi
    fi
fi

# Detect and read AGENTS.md if present (competitor adoption: AGENTS.md standard)
agents_md_content=""
if [[ -f "$ROOT/AGENTS.md" ]]; then
    agents_md_content=$(cat "$ROOT/AGENTS.md" 2>/dev/null || echo "")
fi

# Read the using-agentops skill content (with safe error handling)
SKILL_FILE="${PLUGIN_ROOT}/skills/using-agentops/SKILL.md"
if [[ -f "$SKILL_FILE" ]]; then
    using_agentops_content=$(cat "$SKILL_FILE")
else
    # Generic fallback - don't leak path information
    using_agentops_content="(AgentOps skill content unavailable)"
fi

# escape_for_json: Escape string for safe JSON embedding
# Handles: backslash, quotes, newlines, carriage returns, tabs
# Parameters: $1 = input string
# Output: Escaped string suitable for JSON string value (without surrounding quotes)
# Note: This is used when jq is not available; prefer jq when possible
escape_for_json() {
    local input="$1"
    local output=""
    local i char
    for (( i=0; i<${#input}; i++ )); do
        char="${input:$i:1}"
        # shellcheck disable=SC1003
        case "$char" in
            '\'*) output+='\\\\' ;;
            '"') output+='\\"' ;;
            $'\n') output+='\\n' ;;
            $'\r') output+='\\r' ;;
            $'\t') output+='\\t' ;;
            *) output+="$char" ;;
        esac
    done
    printf '%s' "$output"
}

# Build AGENTS.md section if content exists
agents_md_section=""
if [[ -n "$agents_md_content" ]]; then
    agents_md_section="\n\n## Project Agent Instructions (AGENTS.md)\n\n${agents_md_content}"
fi

# Build flywheel status section if available
flywheel_section=""
if [[ -n "$flywheel_status" || -n "$ratchet_status" || -n "$resume_directive" ]]; then
    flywheel_section="

---
${flywheel_status}"
    if [[ -n "$ratchet_status" ]]; then
        flywheel_section="${flywheel_section}
${ratchet_status}"
    fi
    if [[ -n "$resume_directive" ]]; then
        flywheel_section="${flywheel_section}
**${resume_directive}**"
    fi
    flywheel_section="${flywheel_section}

**Quick commands:** \`ao search <query>\` | \`ao flywheel status\` | \`ao trace <artifact>\`
---
"
fi

# Combine all content for context injection
full_content="<EXTREMELY_IMPORTANT>
You have AgentOps superpowers.
${flywheel_section}${handoff_section}
**Below is the full content of your 'agentops:using-agentops' skill - your introduction to using AgentOps skills. For all other skills, use the 'Skill' tool:**

${using_agentops_content}${agents_md_section}
</EXTREMELY_IMPORTANT>"

# Output context injection as JSON using jq for safe encoding (preferred)
# Falls back to manual escaping if jq unavailable
if command -v jq &>/dev/null; then
    # Use jq to safely encode the entire content as a JSON string
    additional_context=$(printf '%s' "$full_content" | jq -Rs '.')
    cat <<EOF
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": ${additional_context}
  }
}
EOF
else
    # Fallback: manual escaping (less safe but functional)
    escaped_content=$(escape_for_json "$full_content")
    cat <<EOF
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "${escaped_content}"
  }
}
EOF
fi

exit 0
