#!/usr/bin/env bash
# verify.sh — self-check for the Gemini/AGY CORE image bundle (cp-7uih).
#
# Confirms:
#   1. plugin.json / mcp_config.json / hooks.json / hooks/hooks.json are valid JSON
#      and expose the expected manifest shape (modeled on the proven .agy-plugin
#      package at agentops ed8f573e6).
#   2. each bundled slug (CORE + AGY operator; see arrays below) resolves to
#      skills/<slug>/SKILL.md inside the bundle AND is byte-identical to the
#      canonical source skills/<slug>/SKILL.md
#      (the KEY FINDING: zero content conversion — only the wrapper differs).
#   3. agents/ and rules/ each carry >=2 AGY-native control templates.
#   4. if the `agy` CLI is present, `agy plugin validate` passes; otherwise the
#      check is skipped (noted) and we rely on JSON-validity + slug-presence.
#
# Exit 0 = bundle valid. Run from anywhere.
set -euo pipefail

PLUGIN_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# repo root = images/gemini -> images -> <repo root>
REPO_ROOT="$(cd "$PLUGIN_DIR/../.." && pwd)"

# The 32 CORE slugs — the original IMAGE-CORE.md §1 list resolved through the
# skill-consolidation ledger (docs/contracts/skill-dispositions.yaml historical
# merged-into chains + caam->account-rotation, refreshed 2026-07-04, age-085q).
# Retired-with-no-successor slugs (ssh, gcloud, gh-cli, gh-actions, ...) dropped.
# 2026-07-07 retire wave (age-skills-audit-fable-l6ic.12): red-team, curate,
# compile, flywheel, recover, review dropped — merged into validate /
# postmortem / status per docs/audits/skills-audit-2026-07-06.md.
core_skills=(
  rpi discovery research plan implement crank swarm validate
  council premortem postmortem
  goals evolve bootstrap handoff
  operationalize push scope status test
  skill-builder heal-skill
  beads-br beads-bv agent-mail ntm cass dcg
  rch sbh cc-hooks account-rotation
)

# The Gemini/AGY operator surface (was 4 skills; agy-rules-workflows,
# agy-mcp-plugins, agy-headless-evidence merged into agy-native per the
# dispositions ledger). Same packaging recipe: direct, byte-identical
# SKILL.md copies, zero conversion.
operator_skills=(
  agy-native
)

# Full bundled set = CORE + AGY operator.
all_skills=( "${core_skills[@]}" "${operator_skills[@]}" )

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
pass() { printf 'PASS: %s\n' "$*"; }

command -v jq >/dev/null || fail "jq is required"

# --- 1. manifest files exist + valid JSON + expected shape -------------------
[[ -f "$PLUGIN_DIR/plugin.json" ]]      || fail "missing plugin.json"
[[ -f "$PLUGIN_DIR/mcp_config.json" ]]  || fail "missing mcp_config.json"
[[ -f "$PLUGIN_DIR/hooks.json" ]]       || fail "missing hooks.json"
[[ -f "$PLUGIN_DIR/hooks/hooks.json" ]] || fail "missing hooks/hooks.json"
[[ -d "$PLUGIN_DIR/agents" ]]           || fail "missing agents/"
[[ -d "$PLUGIN_DIR/rules" ]]            || fail "missing rules/"

for f in plugin.json mcp_config.json hooks.json hooks/hooks.json; do
  jq -e . "$PLUGIN_DIR/$f" >/dev/null || fail "$f is not valid JSON"
done
pass "all manifest files are valid JSON"

jq -e '
  .name == "agentops-core-gemini"
  and .skills == "./skills"
  and .agents == "./agents"
  and .rules == "./rules"
  and .hooks == "./hooks/hooks.json"
  and .mcpServers["agent-mail"].command == "am"
  and .mcpServers["agent-mail"].args == ["serve-stdio"]
' "$PLUGIN_DIR/plugin.json" >/dev/null \
  || fail "plugin.json does not expose the expected skills/agents/rules/hooks + Agent Mail MCP"

jq -e '
  .mcpServers["agent-mail"].command == "am"
  and .mcpServers["agent-mail"].args == ["serve-stdio"]
' "$PLUGIN_DIR/mcp_config.json" >/dev/null \
  || fail "mcp_config.json does not expose Agent Mail over stdio"

jq -e '
  .["agentops-dcg"].PreToolUse[0].matcher == "run_command"
  and .["agentops-dcg"].PreToolUse[0].hooks[0].command == "dcg"
  and .["agentops-evidence-surface"].Stop[0].command == "ao handoff --help >/dev/null 2>&1 || true"
' "$PLUGIN_DIR/hooks.json" >/dev/null \
  || fail "hooks.json does not expose the expected guard/evidence hooks"

cmp -s "$PLUGIN_DIR/hooks.json" "$PLUGIN_DIR/hooks/hooks.json" \
  || fail "root hooks.json and hooks/hooks.json drifted"
pass "plugin.json / mcp_config.json / hooks parity OK"

# --- 2. agents + rules count -------------------------------------------------
agent_count="$(find "$PLUGIN_DIR/agents" -maxdepth 1 -type f -name '*.md' | wc -l | tr -d ' ')"
[[ "$agent_count" -ge 2 ]] || fail "expected >=2 AGY agent templates, found $agent_count"
rule_count="$(find "$PLUGIN_DIR/rules" -maxdepth 1 -type f -name '*.md' | wc -l | tr -d ' ')"
[[ "$rule_count" -ge 2 ]] || fail "expected >=2 AGY rules, found $rule_count"
pass "agents ($agent_count) + rules ($rule_count) present"

# --- 3. slug presence + byte-identity to source (CORE + AGY operator) ---------
bundled_count="$(find "$PLUGIN_DIR/skills" -mindepth 2 -maxdepth 2 -name SKILL.md | wc -l | tr -d ' ')"
expected_count="${#all_skills[@]}"
[[ "$bundled_count" == "$expected_count" ]] \
  || fail "expected $expected_count bundled skills, found $bundled_count"

for skill in "${all_skills[@]}"; do
  bundled="$PLUGIN_DIR/skills/$skill/SKILL.md"
  source_file="$REPO_ROOT/skills/$skill/SKILL.md"
  [[ -f "$bundled" ]]     || fail "missing bundled skill: $skill"
  [[ -f "$source_file" ]] || fail "missing canonical source skill: skills/$skill/SKILL.md"
  cmp -s "$bundled" "$source_file" \
    || fail "bundled skill drifted from source (NOT zero-conversion): $skill"
done
pass "all $expected_count slugs (${#core_skills[@]} CORE + ${#operator_skills[@]} AGY operator) resolve to skills/<slug>/SKILL.md and match source byte-for-byte"

# --- 4. agy plugin validate (if available) -----------------------------------
if command -v agy >/dev/null; then
  if agy plugin validate "$PLUGIN_DIR"; then
    pass "agy plugin validate succeeded"
  else
    fail "agy plugin validate failed"
  fi
else
  # shellcheck disable=SC2016  # backticks below are literal prose, not command substitution
  printf 'NOTE: agy CLI not found — skipping `agy plugin validate`; relied on JSON-validity + slug-presence.\n'
fi

printf 'OK: Gemini/AGY CORE image bundle is valid (%s skills = %s CORE + %s AGY operator, direct+wrapped, zero conversion)\n' "$expected_count" "${#core_skills[@]}" "${#operator_skills[@]}"
