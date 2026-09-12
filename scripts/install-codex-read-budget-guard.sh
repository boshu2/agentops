#!/usr/bin/env bash
# Opt-in Codex PreToolUse Bash guard. Installs files and hooks.json only; hook
# trust remains an explicit Codex /hooks review. No default plugin hook changes.
# Usage: scripts/install-codex-read-budget-guard.sh [--project]
# CODEX_HOOKS_FILE overrides the destination hooks.json; CODEX_HOME selects the
# default user config directory (otherwise ~/.codex). Assets live beside it.
# shellcheck disable=SC1091,SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"
umask 077

case "${1:-}" in
  ''|--project) ;;
  --help|-h)
    printf '%s\n' 'Usage: install-codex-read-budget-guard.sh [--project]' \
      'CODEX_HOOKS_FILE overrides hooks.json; CODEX_HOME selects the user directory.' \
      'After installation, review and trust the hook in Codex /hooks.'
    exit 0 ;;
  *) printf 'Unknown argument: %s\n' "$1" >&2; exit 2 ;;
esac
[[ $# -le 1 ]] || { echo 'Expected at most one argument.' >&2; exit 2; }
require_cmd jq

hooks_file="${CODEX_HOOKS_FILE:-}"
if [[ -z "$hooks_file" ]]; then
  if [[ "${1:-}" == --project ]]; then
    # Codex 0.154 loads project hooks.json from the primary checkout even when
    # config.toml comes from a linked worktree. Never silently write that other
    # checkout or install into an undiscovered local hook path.
    if git_dir="$(git rev-parse --absolute-git-dir 2>/dev/null)" &&
       git_common_dir="$(git rev-parse --git-common-dir 2>/dev/null)"; then
      git_dir="$(CDPATH= cd "$git_dir" && pwd -P)"
      git_common_dir="$(CDPATH= cd "$git_common_dir" && pwd -P)"
      if [[ "$git_dir" != "$git_common_dir" ]]; then
        printf '%s\n' \
          'ERROR: Codex 0.154 discovers project hooks from the primary checkout, not this linked worktree.' \
          'Run this installer without --project for personal hooks, or run --project from the primary checkout.' \
          'CODEX_HOOKS_FILE may select an explicit destination; this installer will not write another checkout automatically.' >&2
        exit 2
      fi
    fi
    hooks_file="$PWD/.codex/hooks.json"
  else
    hooks_file="${CODEX_HOME:-${HOME:?HOME or CODEX_HOME is required}/.codex}/hooks.json"
  fi
fi
mkdir -p "$(dirname "$hooks_file")"
config_dir="$(CDPATH= cd "$(dirname "$hooks_file")" && pwd)"
hooks_file="$config_dir/$(basename "$hooks_file")"
assets="$config_dir/hooks/agentops-read-budget"
source_dir="$REPO_ROOT/skills/cc-hooks/hooks"
for name in read-budget-guard.sh codex-read-budget-guard.sh; do
  [[ -f "$source_dir/$name" ]] || { echo "Missing guard source: $source_dir/$name" >&2; exit 1; }
done
dst="$assets/codex-read-budget-guard.sh"
hook_command="$(jq -nr --arg path "$dst" '["bash", $path] | @sh')"
tmp="$(mktemp "${hooks_file}.tmp.XXXXXX")"
trap 'rm -f "$tmp"' EXIT

merge_hooks() {
  jq -e --arg cmd "$hook_command" '
    .hooks //= {} | .hooks.PreToolUse //= [] |
    if any(.hooks.PreToolUse[]?;
      .matcher == "^Bash$" and any(.hooks[]?;
        .type == "command" and .command == $cmd and
        (.async // false) == false and .timeout == 10))
    then .
    else .hooks.PreToolUse += [{matcher:"^Bash$", hooks:[{
      type:"command", command:$cmd, timeout:10
    }]}]
    end
  '
}
if [[ -f "$hooks_file" ]]; then
  merge_hooks < "$hooks_file" > "$tmp"
else
  printf '{}\n' | merge_hooks > "$tmp"
fi

mkdir -p "$assets"
for name in read-budget-guard.sh codex-read-budget-guard.sh; do
  install -m 0755 "$source_dir/$name" "$assets/$name"
done
if ! cmp -s "$hooks_file" "$tmp"; then
  if [[ -f "$hooks_file" ]]; then
    backup="$(mktemp "${hooks_file}.bak.$(date +%Y%m%d%H%M%S).XXXXXX")"
    cp -p "$hooks_file" "$backup"
    printf 'Backed up hooks: %s\n' "$backup"
  fi
  mv "$tmp" "$hooks_file"
else
  rm -f "$tmp"
fi
trap - EXIT

printf 'Configured Codex PreToolUse Bash read-budget guard: %s\n' "$hooks_file"
printf '%s\n' 'Review and trust the exact hook definition in Codex /hooks before it can run.' \
  'For --project, the project .codex config layer must also be trusted.' \
  'New or changed hooks are skipped until trusted; this installer does not grant trust.' \
  'Rule: refuse unbounded cat/head/tail reads above AOP_READ_BUDGET_LINES (default 350).' \
  'Uninstall: remove this matcher from hooks.json, then remove its agentops-read-budget asset directory.'
