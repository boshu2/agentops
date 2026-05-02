#!/usr/bin/env bash
# ao-paths.sh — Canonical state-path resolver for AgentOps (shell side)
#
# Usage:
#   eval "$(./lib/ao-paths.sh)"
#
# Emits `export NAME=value` lines on stdout for every resolved root.
# Idempotent: re-running with identical environment produces identical output.
#
# Env precedence (highest first):
#   AO_HOME              — explicit override (treated as the .agents directory itself)
#   CLAUDE_PLUGIN_DATA   — Claude plugin data dir (resolves AO_HOME=$CLAUDE_PLUGIN_DATA/.agents)
#   default              — $REPO_ROOT/.agents (git rev-parse) or ${PWD}/.agents
#
# Per-subdir overrides (read after the home resolves, win over the default layout):
#   AO_AGENTS_DIR, AO_KNOWLEDGE_ROOT, AO_HOOKS_DIR, AO_SCOPE_LOCK,
#   AO_RPI_DIR, AO_FINDINGS_DIR, AO_PLANS_DIR, AO_COUNCIL_DIR,
#   AO_LEARNINGS_DIR, AO_PATTERNS_DIR, AO_DECISIONS_DIR
#
# Knowledge separation note:
#   AO_KNOWLEDGE_ROOT defaults to ${AO_AGENTS_DIR}/wiki — the *internal*
#   compiled wiki under .agents/. The external knowledge tree (raw + wiki at
#   the vault root) is intentionally NOT modeled here; agentops keeps internal
#   .agents/ separate from external knowledge by design.
#
# Debug:
#   AO_PATHS_DEBUG=1 emits a `# debug:` comment to stderr per resolved root.

set -euo pipefail

_ao_paths_debug() {
    if [[ "${AO_PATHS_DEBUG:-}" == "1" ]]; then
        printf '# debug: %s\n' "$*" >&2
    fi
}

# Resolve AO_HOME via the documented precedence.
_ao_home=""
if [[ -n "${AO_HOME:-}" ]]; then
    _ao_home="${AO_HOME}"
    _ao_paths_debug "AO_HOME from env: $_ao_home"
elif [[ -n "${CLAUDE_PLUGIN_DATA:-}" ]]; then
    _ao_home="${CLAUDE_PLUGIN_DATA%/}/.agents"
    _ao_paths_debug "AO_HOME from CLAUDE_PLUGIN_DATA: $_ao_home"
else
    _ao_repo_root=""
    if command -v git >/dev/null 2>&1; then
        _ao_repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
    fi
    if [[ -z "$_ao_repo_root" ]]; then
        _ao_repo_root="${PWD}"
    fi
    _ao_home="${_ao_repo_root%/}/.agents"
    _ao_paths_debug "AO_HOME from default (repo root or pwd): $_ao_home"
fi

# Layout defaults — every per-subdir variable can be overridden via env.
_ao_agents_dir="${AO_AGENTS_DIR:-$_ao_home}"
_ao_knowledge_root="${AO_KNOWLEDGE_ROOT:-$_ao_agents_dir/wiki}"
_ao_hooks_dir="${AO_HOOKS_DIR:-$_ao_agents_dir/hooks}"
_ao_scope_lock="${AO_SCOPE_LOCK:-$_ao_agents_dir/scope.lock}"
_ao_rpi_dir="${AO_RPI_DIR:-$_ao_agents_dir/rpi}"
_ao_findings_dir="${AO_FINDINGS_DIR:-$_ao_agents_dir/findings}"
_ao_plans_dir="${AO_PLANS_DIR:-$_ao_agents_dir/plans}"
_ao_council_dir="${AO_COUNCIL_DIR:-$_ao_agents_dir/council}"
_ao_learnings_dir="${AO_LEARNINGS_DIR:-$_ao_agents_dir/learnings}"
_ao_patterns_dir="${AO_PATTERNS_DIR:-$_ao_agents_dir/patterns}"
_ao_decisions_dir="${AO_DECISIONS_DIR:-$_ao_agents_dir/decisions}"

# Emit. Sorted output keeps re-runs byte-identical for the same env.
printf 'export AO_HOME=%q\n' "$_ao_home"
printf 'export AO_AGENTS_DIR=%q\n' "$_ao_agents_dir"
printf 'export AO_KNOWLEDGE_ROOT=%q\n' "$_ao_knowledge_root"
printf 'export AO_HOOKS_DIR=%q\n' "$_ao_hooks_dir"
printf 'export AO_SCOPE_LOCK=%q\n' "$_ao_scope_lock"
printf 'export AO_RPI_DIR=%q\n' "$_ao_rpi_dir"
printf 'export AO_FINDINGS_DIR=%q\n' "$_ao_findings_dir"
printf 'export AO_PLANS_DIR=%q\n' "$_ao_plans_dir"
printf 'export AO_COUNCIL_DIR=%q\n' "$_ao_council_dir"
printf 'export AO_LEARNINGS_DIR=%q\n' "$_ao_learnings_dir"
printf 'export AO_PATTERNS_DIR=%q\n' "$_ao_patterns_dir"
printf 'export AO_DECISIONS_DIR=%q\n' "$_ao_decisions_dir"
