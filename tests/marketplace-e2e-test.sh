#!/bin/bash
# Comprehensive E2E test for AgentOps marketplace
# Tests: plugin structure, skill validation, cross-references, commands
# Usage: ./tests/marketplace-e2e-test.sh [--verbose]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERBOSE="${1:-}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

errors=0
warnings=0
tests_passed=0

log() { echo -e "${BLUE}[TEST]${NC} $1"; }
pass() { echo -e "${GREEN}  ✓${NC} $1"; tests_passed=$((tests_passed + 1)); }
fail() { echo -e "${RED}  ✗${NC} $1"; errors=$((errors + 1)); }
warn() { echo -e "${YELLOW}  !${NC} $1"; warnings=$((warnings + 1)); }
section() { echo -e "\n${CYAN}━━━ $1 ━━━${NC}"; }

cd "$REPO_ROOT"

# =============================================================================
section "1. JSON Validation"
# =============================================================================

log "Validating all JSON files..."

json_files=(
    ".claude-plugin/marketplace.json"
    ".claude-plugin/plugin.json"
)

for plugin_dir in plugins/*/; do
    [[ -f "${plugin_dir}.claude-plugin/plugin.json" ]] && json_files+=("${plugin_dir}.claude-plugin/plugin.json")
done

for jf in "${json_files[@]}"; do
    if [[ ! -f "$jf" ]]; then
        fail "$jf - file not found"
        continue
    fi
    if python3 -m json.tool "$jf" > /dev/null 2>&1; then
        pass "$jf"
    else
        fail "$jf - invalid JSON"
    fi
done

# =============================================================================
section "2. Marketplace Schema Validation"
# =============================================================================

log "Validating marketplace structure..."

# Check marketplace has required fields
marketplace_valid=$(python3 << 'PYEOF'
import json
import sys

with open('.claude-plugin/marketplace.json') as f:
    mp = json.load(f)

errors = []

# Required top-level fields
for field in ['name', 'description', 'plugins']:
    if field not in mp:
        errors.append(f"Missing required field: {field}")

# Each plugin must have name, description, source
for i, plugin in enumerate(mp.get('plugins', [])):
    for field in ['name', 'description', 'source']:
        if field not in plugin:
            errors.append(f"Plugin {i}: missing {field}")

if errors:
    for e in errors:
        print(f"ERROR: {e}")
    sys.exit(1)
else:
    print(f"OK: {len(mp.get('plugins', []))} plugins defined")
    sys.exit(0)
PYEOF
) && pass "Marketplace schema valid - $marketplace_valid" || fail "Marketplace schema invalid"

# =============================================================================
section "3. Plugin References"
# =============================================================================

log "Validating plugin sources exist..."

while IFS='|' read -r name source; do
    [[ -z "$name" ]] && continue

    if [[ "$source" == "." ]]; then
        pj=".claude-plugin/plugin.json"
    else
        pj="${source}/.claude-plugin/plugin.json"
    fi

    if [[ -f "$pj" ]]; then
        pass "$name -> $source"
    else
        fail "$name: source $source has no plugin.json"
    fi
done < <(python3 -c "
import json
with open('.claude-plugin/marketplace.json') as f:
    mp = json.load(f)
for p in mp.get('plugins', []):
    src = p['source'].lstrip('./')
    if not src: src = '.'
    print(f\"{p['name']}|{src}\")
")

# =============================================================================
section "4. Skill YAML Frontmatter Validation"
# =============================================================================

log "Validating skill frontmatter..."

skill_count=0
skill_errors=0

for plugin_dir in plugins/*/; do
    skills_dir="${plugin_dir}skills"
    [[ ! -d "$skills_dir" ]] && continue
    plugin_name=$(basename "$plugin_dir")

    for skill_dir in "$skills_dir"/*/; do
        [[ ! -d "$skill_dir" ]] && continue
        skill_file="${skill_dir}SKILL.md"
        skill_name=$(basename "$skill_dir")

        if [[ ! -f "$skill_file" ]]; then
            fail "$plugin_name/$skill_name: SKILL.md missing"
            skill_errors=$((skill_errors + 1))
            continue
        fi

        skill_count=$((skill_count + 1))

        # Validate frontmatter
        result=$(python3 << PYEOF
import re
import sys

with open('$skill_file') as f:
    content = f.read()

# Check for frontmatter
match = re.match(r'^---\s*\n(.*?)\n---', content, re.DOTALL)
if not match:
    print("ERROR: No YAML frontmatter")
    sys.exit(1)

yaml_content = match.group(1)
errors = []

# Check required fields
required = ['name', 'description', 'version']
for field in required:
    if f'{field}:' not in yaml_content:
        errors.append(f"Missing: {field}")

# Check version format
import re as regex
version_match = regex.search(r'version:\s*["\']?(\d+\.\d+\.\d+)', yaml_content)
if not version_match:
    version_match = regex.search(r'version:\s*(\d+\.\d+\.\d+)', yaml_content)
    if not version_match:
        errors.append("Invalid version format")

if errors:
    print("WARN: " + "; ".join(errors))
    sys.exit(2)
else:
    print("OK")
    sys.exit(0)
PYEOF
        )
        exit_code=$?

        if [[ $exit_code -eq 0 ]]; then
            [[ "$VERBOSE" == "--verbose" ]] && pass "$plugin_name/$skill_name"
        elif [[ $exit_code -eq 1 ]]; then
            fail "$plugin_name/$skill_name: $result"
            skill_errors=$((skill_errors + 1))
        else
            warn "$plugin_name/$skill_name: $result"
        fi
    done
done

if [[ $skill_errors -eq 0 ]]; then
    pass "All $skill_count skills have valid frontmatter"
else
    fail "$skill_errors skills with invalid frontmatter"
fi

# =============================================================================
section "5. Skill Cross-Reference Validation"
# =============================================================================

log "Validating skill dependencies..."

# Build skill inventory and check references
crossref_result=$(python3 << 'PYEOF'
import os
import re
from pathlib import Path

plugins_dir = Path('plugins')
all_skills = set()
skill_deps = {}

# Collect all skills
for plugin_dir in plugins_dir.iterdir():
    if not plugin_dir.is_dir():
        continue
    skills_dir = plugin_dir / 'skills'
    if not skills_dir.exists():
        continue
    for skill_dir in skills_dir.iterdir():
        if skill_dir.is_dir() and (skill_dir / 'SKILL.md').exists():
            skill_name = skill_dir.name
            all_skills.add(skill_name)

            # Extract dependencies
            with open(skill_dir / 'SKILL.md') as f:
                content = f.read()
            match = re.match(r'^---\s*\n(.*?)\n---', content, re.DOTALL)
            if match:
                yaml_content = match.group(1)
                in_skills = False
                deps = []
                for line in yaml_content.split('\n'):
                    if line.strip().startswith('skills:'):
                        in_skills = True
                        continue
                    if in_skills:
                        if line.startswith('  - '):
                            deps.append(line.strip('  - ').strip().strip('"').strip("'"))
                        elif line and not line.startswith(' '):
                            in_skills = False
                if deps:
                    skill_deps[skill_name] = deps

# Check for broken references
errors = []
for skill, deps in skill_deps.items():
    for dep in deps:
        if dep not in all_skills:
            errors.append(f"{skill} -> {dep} (not found)")

if errors:
    for e in errors:
        print(f"ERROR: {e}")
    exit(1)
else:
    print(f"OK: {len(skill_deps)} skills with {sum(len(d) for d in skill_deps.values())} total dependencies")
    exit(0)
PYEOF
) && pass "Cross-references valid - $crossref_result" || fail "Invalid cross-references: $crossref_result"

# =============================================================================
section "6. Commands Validation"
# =============================================================================

log "Validating commands..."

cmd_count=0
[[ -d "commands" ]] && cmd_count=$(find commands -name "*.md" -type f 2>/dev/null | wc -l | tr -d ' ')

if [[ $cmd_count -gt 0 ]]; then
    pass "Found $cmd_count commands in commands/"
else
    warn "No commands found in commands/"
fi

# Check INDEX.md exists
if [[ -f "commands/INDEX.md" ]]; then
    pass "commands/INDEX.md exists"
else
    warn "commands/INDEX.md missing"
fi

# =============================================================================
section "7. Content Quality Checks"
# =============================================================================

log "Running content quality checks..."

# Check for placeholder patterns (actual placeholders, not docs about them)
placeholder_count=$( (grep -rn "\[your-email\]\|\[your-name\]" --include="*.md" . 2>/dev/null || true) | wc -l | tr -d '[:space:]')
placeholder_count=${placeholder_count:-0}
if [[ $placeholder_count -gt 0 ]]; then
    fail "$placeholder_count files with placeholder patterns"
else
    pass "No placeholder patterns"
fi

# Check for broken internal links (simple check)
broken_links=$( (grep -rhoE '\[.*\]\(\./[^)]+\)' --include="*.md" . 2>/dev/null || true) | \
    sed 's/.*(\.\///' | sed 's/).*//' | \
    while read -r link; do
        [[ ! -e "$link" ]] && echo "$link"
    done | wc -l | tr -d '[:space:]')
broken_links=${broken_links:-0}

if [[ "$broken_links" -gt 0 ]]; then
    warn "$broken_links potentially broken internal links"
else
    pass "No obvious broken internal links"
fi

# Check skill content is substantive (> 500 chars after frontmatter)
thin_skills=0
for skill_file in plugins/*/skills/*/SKILL.md; do
    [[ ! -f "$skill_file" ]] && continue
    content_after_frontmatter=$(sed '1,/^---$/d; 1,/^---$/d' "$skill_file" | wc -c)
    if [[ $content_after_frontmatter -lt 500 ]]; then
        [[ "$VERBOSE" == "--verbose" ]] && warn "$skill_file: only $content_after_frontmatter chars of content"
        thin_skills=$((thin_skills + 1))
    fi
done

if [[ $thin_skills -gt 0 ]]; then
    warn "$thin_skills skills with thin content (< 500 chars)"
else
    pass "All skills have substantive content"
fi

# =============================================================================
section "8. Security Checks"
# =============================================================================

log "Running security checks..."

# Check for potential secrets (simplified gitleaks-style)
# Exclude: examples, placeholders, variables ($VAR), environment var patterns
secret_patterns='(password|api[_-]?key|secret|token)\s*[:=]\s*["\x27][^\s"$]+["\x27]'
secret_count=$( (grep -riE "$secret_patterns" --include="*.md" --include="*.json" . 2>/dev/null || true) | \
    (grep -v 'example\|placeholder\|your-\|<\|>\|\$[A-Z_]\|from-literal\|standards' || true) | wc -l | tr -d '[:space:]')
secret_count=${secret_count:-0}

if [[ $secret_count -gt 0 ]]; then
    fail "$secret_count potential secrets found"
    [[ "$VERBOSE" == "--verbose" ]] && grep -riE "$secret_patterns" --include="*.md" --include="*.json" . 2>/dev/null | grep -v "example\|placeholder" | head -3
else
    pass "No potential secrets detected"
fi

# Check for dangerous shell patterns
dangerous_patterns='rm\s+-rf\s+/|curl.*\|\s*bash|wget.*\|\s*sh'
dangerous_count=$( (grep -riE "$dangerous_patterns" --include="*.md" --include="*.sh" . 2>/dev/null || true) | wc -l | tr -d '[:space:]')
dangerous_count=${dangerous_count:-0}

if [[ "$dangerous_count" -gt 0 ]]; then
    warn "$dangerous_count potentially dangerous shell patterns"
else
    pass "No dangerous shell patterns"
fi

# =============================================================================
# Summary
# =============================================================================

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}                    E2E TEST SUMMARY                        ${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "  Tests passed:  ${GREEN}$tests_passed${NC}"
echo -e "  Warnings:      ${YELLOW}$warnings${NC}"
echo -e "  Errors:        ${RED}$errors${NC}"
echo ""

if [[ $errors -gt 0 ]]; then
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}  FAILED - $errors errors found                              ${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    exit 1
elif [[ $warnings -gt 0 ]]; then
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}  PASSED WITH WARNINGS                                      ${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    exit 0
else
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  ALL E2E TESTS PASSED                                      ${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    exit 0
fi
