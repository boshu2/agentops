#!/usr/bin/env bash
# Static token budget validation for skill content
# Ensures no single skill or session injection exceeds context budget limits.
# No Claude CLI needed — runs in milliseconds.
#
# Thresholds:
#   Per-skill SKILL.md: FAIL > 10,000 tokens, WARN > 8,000 tokens
#   SessionStart total:  FAIL > 8,000 tokens
#
# Token estimation: 4 characters per token (conservative average)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SKILL_ROOTS=("$REPO_ROOT/skills" "$REPO_ROOT/skills-codex")

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Thresholds (in estimated tokens)
SKILL_FAIL_LIMIT=10000
SKILL_WARN_LIMIT=8000
SESSION_FAIL_LIMIT=8000
DESC_FAIL_CHARS=180

# Token estimation: bytes / 4
estimate_tokens() {
    local bytes="$1"
    echo $(( bytes / 4 ))
}

passed=0
failed=0
warned=0

echo "=== Token Budget Validation ==="
echo ""

# ─────────────────────────────────────────────────────────
# Check 1: Per-skill SKILL.md token budgets
# ─────────────────────────────────────────────────────────
echo -e "${BLUE}--- Per-Skill SKILL.md Budgets ---${NC}"
echo ""

# Collect all skills with sizes for sorting
declare -a skill_sizes=()

for skills_dir in "${SKILL_ROOTS[@]}"; do
    root_label="${skills_dir#"$REPO_ROOT"/}"
    for skill_dir in "$skills_dir"/*/; do
        [[ -d "$skill_dir" ]] || continue
        skill_md="${skill_dir}SKILL.md"
        [[ -f "$skill_md" ]] || continue

        skill_name="${root_label}/$(basename "$skill_dir")"
        bytes=$(wc -c < "$skill_md" | tr -d ' ')
        tokens=$(estimate_tokens "$bytes")
        skill_sizes+=("$tokens $skill_name")

        if [[ $tokens -gt $SKILL_FAIL_LIMIT ]]; then
            echo -e "  ${RED}[FAIL]${NC} $skill_name: ${tokens} tokens (${bytes} bytes) > ${SKILL_FAIL_LIMIT} limit"
            ((failed++)) || true
        elif [[ $tokens -gt $SKILL_WARN_LIMIT ]]; then
            echo -e "  ${YELLOW}[WARN]${NC} $skill_name: ${tokens} tokens (${bytes} bytes) > ${SKILL_WARN_LIMIT} warning threshold"
            ((warned++)) || true
        else
            ((passed++)) || true
        fi
    done
done

# Print top-5 largest skills
echo ""
echo -e "${BLUE}Top 5 largest skills:${NC}"
printf '%s\n' "${skill_sizes[@]}" | sort -rn | head -5 | while read -r tokens name; do
    pct=$(( tokens * 100 / SKILL_FAIL_LIMIT ))
    echo "  ${tokens} tokens (${pct}% of limit) — ${name}"
done

# ─────────────────────────────────────────────────────────
# Check 2: SessionStart injection budget
# ─────────────────────────────────────────────────────────
echo ""
echo -e "${BLUE}--- SessionStart Injection Budget ---${NC}"
echo ""

session_total_bytes=0

# Component 1: using-agentops SKILL.md
using_agentops="$REPO_ROOT/skills/using-agentops/SKILL.md"
if [[ -f "$using_agentops" ]]; then
    ua_bytes=$(wc -c < "$using_agentops" | tr -d ' ')
    ua_tokens=$(estimate_tokens "$ua_bytes")
    echo "  using-agentops SKILL.md: ${ua_tokens} tokens (${ua_bytes} bytes)"
    session_total_bytes=$((session_total_bytes + ua_bytes))
else
    echo -e "  ${YELLOW}[SKIP]${NC} using-agentops SKILL.md not found"
fi

# Component 2: Hook wrapper overhead (template strings, JSON encoding, status lines)
# Measured from session-start.sh output structure: ~2000 bytes of wrapper
HOOK_WRAPPER_BYTES=2000
hook_tokens=$(estimate_tokens $HOOK_WRAPPER_BYTES)
echo "  Hook wrapper overhead: ${hook_tokens} tokens (${HOOK_WRAPPER_BYTES} bytes, estimated)"
session_total_bytes=$((session_total_bytes + HOOK_WRAPPER_BYTES))

# Component 3: ao lookup / signpost cap (default budget ~4000 bytes)
AO_LOOKUP_BYTES=4000
lookup_tokens=$(estimate_tokens $AO_LOOKUP_BYTES)
echo "  ao lookup/signpost cap: ${lookup_tokens} tokens (${AO_LOOKUP_BYTES} bytes, estimated)"
session_total_bytes=$((session_total_bytes + AO_LOOKUP_BYTES))

session_total_tokens=$(estimate_tokens $session_total_bytes)
echo ""

if [[ $session_total_tokens -gt $SESSION_FAIL_LIMIT ]]; then
    echo -e "  ${RED}[FAIL]${NC} SessionStart total: ${session_total_tokens} tokens > ${SESSION_FAIL_LIMIT} limit"
    ((failed++)) || true
else
    pct=$(( session_total_tokens * 100 / SESSION_FAIL_LIMIT ))
    echo -e "  ${GREEN}[PASS]${NC} SessionStart total: ${session_total_tokens} tokens (${pct}% of ${SESSION_FAIL_LIMIT} limit)"
    ((passed++)) || true
fi

# ─────────────────────────────────────────────────────────
# Check 3: Always-loaded skill catalog description budget
# ─────────────────────────────────────────────────────────
echo ""
echo -e "${BLUE}--- Skill Description Budget ---${NC}"
echo ""

desc_failures=0
while IFS= read -r skill_md; do
    desc_chars=$(awk '
        /^---$/ {
            if (!in_fm) {
                in_fm = 1
                next
            }
            print length(desc)
            found = 1
            exit
        }
        in_fm && /^description:[[:space:]]*/ {
            desc = $0
            sub(/^description:[[:space:]]*/, "", desc)
            collecting = 1
            next
        }
        in_fm && collecting {
            if ($0 ~ /^[A-Za-z0-9_-]+:/) {
                collecting = 0
                next
            }
            line = $0
            sub(/^[[:space:]]+/, "", line)
            desc = desc " " line
        }
        END {
            if (!found) {
                print length(desc)
            }
        }
    ' "$skill_md")
    if [[ "$desc_chars" -gt "$DESC_FAIL_CHARS" ]]; then
        echo -e "  ${RED}[FAIL]${NC} ${skill_md#"$REPO_ROOT"/}: ${desc_chars} chars > ${DESC_FAIL_CHARS} description limit"
        ((desc_failures++)) || true
    fi
done < <(find "${SKILL_ROOTS[@]}" -maxdepth 2 -name SKILL.md -type f | sort)

if [[ "$desc_failures" -eq 0 ]]; then
    echo -e "  ${GREEN}[PASS]${NC} all skill descriptions <= ${DESC_FAIL_CHARS} chars"
    ((passed++)) || true
else
    failed=$((failed + desc_failures))
fi

# ─────────────────────────────────────────────────────────
# Summary
# ─────────────────────────────────────────────────────────
echo ""
echo -e "${BLUE}═══════════════════════════════════════════${NC}"
echo "Token Budget Summary:"
echo -e "  ${GREEN}Passed:${NC}  $passed"
echo -e "  ${RED}Failed:${NC}  $failed"
echo -e "  ${YELLOW}Warned:${NC}  $warned"
echo -e "${BLUE}═══════════════════════════════════════════${NC}"

if [[ $failed -gt 0 ]]; then
    echo ""
    echo -e "${RED}FAIL: $failed budget(s) exceeded${NC}"
    echo ""
    echo "Remediation:"
    echo "  1. Move content from SKILL.md to references/ (loaded JIT)"
    echo "  2. Check SessionStart hook output volume"
    echo "  3. Reduce ao lookup / signpost token budget"
    exit 1
fi

echo ""
echo -e "${GREEN}PASS: All token budgets within limits${NC}"
exit 0
