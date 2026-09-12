#!/usr/bin/env bash
# Opt-in personal/project installation of the source-owned Codex role templates.
# Runtime-resolved shared library, following repository installer convention.
# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

agent_dir="${CODEX_HOME:-$HOME/.codex}/agents"
case "${1:-}" in
  '') ;;
  --project) agent_dir="$PWD/.codex/agents"; shift ;;
  --help|-h)
    echo 'Usage: scripts/install-codex-context-agents.sh [--project]'
    printf 'Default: %s. Restart Codex after installation.\n' "$agent_dir"
    exit 0 ;;
  *) echo "Unknown argument: $1" >&2; exit 2 ;;
esac
[ "$#" -eq 0 ] || { echo 'Unexpected arguments' >&2; exit 2; }

# Consume the same generated bundle shipped by the Codex plugin. Source owners
# are skills/agent-native/agents/*.toml; scripts/regen-all.sh owns this projection.
source_dir="$REPO_ROOT/skills-codex/agent-native/agents"
for role in bulk-reader code-writer; do
  [ -f "$source_dir/$role.toml" ] || {
    echo "Missing generated role $role; run bash scripts/regen-all.sh" >&2; exit 1;
  }
done
mkdir -p "$agent_dir"
for role in bulk-reader code-writer; do
  target="$agent_dir/$role.toml"
  if [ -e "$target" ] || [ -L "$target" ]; then
    if cmp -s "$source_dir/$role.toml" "$target"; then continue; fi
    # Never follow an existing role symlink while replacing its destination.
    backup="$(mktemp "$target.bak.XXXXXX")"
    if ! cp -p "$target" "$backup"; then
      rm -f "$backup"
      echo "Cannot back up $target" >&2; exit 1
    fi
  fi
  staging="$(mktemp "$agent_dir/.${role}.XXXXXX")"
  if ! cp "$source_dir/$role.toml" "$staging" || ! chmod 644 "$staging" || ! mv -f "$staging" "$target"; then
    rm -f "$staging"
    echo "Cannot install $target" >&2; exit 1
  fi
done
printf 'Installed bulk-reader and code-writer in %s. Restart Codex to discover them.\n' "$agent_dir"
printf 'Roles use gpt-5.6-luna. The read-budget hook is a separate opt-in installation.\n'
