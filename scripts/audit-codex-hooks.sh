#!/usr/bin/env bash
# audit-codex-hooks.sh — Audit active Codex hooks for non-AgentOps context injectors.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

CODEX_HOME="${CODEX_HOME:-$HOME/.codex}"
HOOKS_FILE=""
PRUNE_FOREIGN="false"
REPAIR_SHAPE="false"
STRICT="false"
JSON_OUTPUT="false"

usage() {
  cat <<'EOF'
audit-codex-hooks.sh

Audit ~/.codex/hooks.json for AgentOps-managed and foreign hook handlers.

Options:
  --codex-home <dir>   Codex home to inspect (default: $CODEX_HOME or ~/.codex)
  --hooks-file <file>   Hook manifest to inspect (default: <codex-home>/hooks.json)
  --repair-shape        Replace missing/legacy-flat hooks.json with the native AgentOps event-map manifest
  --prune-foreign      Remove non-AgentOps hook handlers after writing a backup
  --strict             Exit non-zero when foreign hook handlers are present
  --json               Emit machine-readable JSON
  --help               Show this help

Examples:
  bash scripts/audit-codex-hooks.sh
  bash scripts/audit-codex-hooks.sh --strict
  bash scripts/audit-codex-hooks.sh --prune-foreign
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --codex-home)
      CODEX_HOME="${2:-}"
      shift 2
      ;;
    --hooks-file)
      HOOKS_FILE="${2:-}"
      shift 2
      ;;
    --prune-foreign)
      PRUNE_FOREIGN="true"
      shift
      ;;
    --repair-shape)
      REPAIR_SHAPE="true"
      shift
      ;;
    --strict)
      STRICT="true"
      shift
      ;;
    --json)
      JSON_OUTPUT="true"
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown arg: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$CODEX_HOME" ]]; then
  echo "ERROR: --codex-home cannot be empty" >&2
  exit 2
fi

if [[ -z "$HOOKS_FILE" ]]; then
  HOOKS_FILE="$CODEX_HOME/hooks.json"
fi

build_agentops_scripts_json() {
  local hooks_dir="$REPO_ROOT/hooks"
  if [[ ! -d "$hooks_dir" ]]; then
    printf '%s\n' '["session-start.sh","ao-flywheel-close.sh","quality-signals.sh","go-test-precommit.sh","commit-review-gate.sh","ratchet-advance.sh"]'
    return 0
  fi

  find "$hooks_dir" -maxdepth 1 -type f -name '*.sh' -exec basename {} \; \
    | sort \
    | jq -R . \
    | jq -s .
}

AGENTOPS_SCRIPTS_JSON="$(build_agentops_scripts_json)"

require_jq() {
  command -v jq >/dev/null 2>&1 || {
    echo "ERROR: jq is required" >&2
    exit 2
  }
}

require_hooks_file() {
  [[ -f "$HOOKS_FILE" ]] || {
    echo "ERROR: Codex hooks file not found: $HOOKS_FILE" >&2
    exit 2
  }
}

validate_hooks_shape() {
  jq -e '.hooks | type == "object"' "$HOOKS_FILE" >/dev/null 2>&1 || {
    echo "ERROR: $HOOKS_FILE must use the Codex event-map schema under .hooks" >&2
    exit 2
  }
}

active_plugin_root() {
  local install_meta="$CODEX_HOME/.agentops-codex-install.json"
  local plugin_root=""

  if [[ -f "$install_meta" ]]; then
    plugin_root="$(jq -r '.plugin_root // empty' "$install_meta" 2>/dev/null || true)"
  fi

  if [[ -z "$plugin_root" ]]; then
    plugin_root="$CODEX_HOME/plugins/cache/agentops-marketplace/agentops/local"
  fi

  printf '%s\n' "$plugin_root"
}

repair_hooks_shape() {
  local source_file="$REPO_ROOT/hooks/codex-hooks.json"
  local tmp_file backup_file plugin_root

  [[ -f "$source_file" ]] || {
    echo "ERROR: canonical Codex hooks manifest not found: $source_file" >&2
    exit 2
  }

  if [[ -f "$HOOKS_FILE" ]] && jq -e '.hooks | type == "object"' "$HOOKS_FILE" >/dev/null 2>&1; then
    return 0
  fi

  mkdir -p "$(dirname "$HOOKS_FILE")"
  tmp_file="$(mktemp)"
  plugin_root="$(active_plugin_root)"

  jq --arg root "$plugin_root" '
    def replace_root:
      if type == "object" then
        with_entries(.value |= replace_root)
      elif type == "array" then
        map(replace_root)
      elif type == "string" then
        gsub("\\$\\{AGENTOPS_PLUGIN_ROOT:-~/.codex/plugins/cache/agentops\\}"; $root)
      else
        .
      end;
    replace_root
  ' "$source_file" > "$tmp_file"

  if [[ -f "$HOOKS_FILE" ]]; then
    backup_file="${HOOKS_FILE}.bak.$(date -u +%Y%m%dT%H%M%SZ)"
    cp "$HOOKS_FILE" "$backup_file"
  fi

  mv "$tmp_file" "$HOOKS_FILE"
}

collect_handlers() {
  jq -c --argjson scripts "$AGENTOPS_SCRIPTS_JSON" '
    def is_agentops_command($cmd):
      ($cmd | type == "string") and (
        ($cmd | contains("ao "))
        or ($cmd | contains("${CLAUDE_PLUGIN_ROOT}/hooks/"))
        or any($scripts[]; . as $script | ($cmd | test("(^|[[:space:]])[^[:space:]]*/hooks/" + $script + "([[:space:]]|$)")))
        or
        any($scripts[]; . as $script | ($cmd | contains("${AGENTOPS_PLUGIN_ROOT") and ($cmd | contains("/hooks/" + $script))))
      );
    def model_visible_event($event):
      ($event == "SessionStart" or
       $event == "UserPromptSubmit" or
       $event == "PreToolUse" or
       $event == "PostToolUse" or
       $event == "PreCompact");
    [
      .hooks | to_entries[]?
      | .key as $event
      | .value[]? as $group
      | ($group.matcher // "") as $matcher
      | $group.hooks[]?
      | {
          event: $event,
          matcher: $matcher,
          command: (.command // ""),
          timeout: (.timeout // null),
          agentops: is_agentops_command(.command // ""),
          model_visible_event: model_visible_event($event)
        }
    ]
  ' "$HOOKS_FILE"
}

prune_foreign_handlers() {
  local tmp_file backup_file
  tmp_file="$(mktemp)"
  backup_file="${HOOKS_FILE}.bak.$(date -u +%Y%m%dT%H%M%SZ)"

  jq --argjson scripts "$AGENTOPS_SCRIPTS_JSON" '
    def is_agentops_command($cmd):
      ($cmd | type == "string") and (
        ($cmd | contains("ao "))
        or ($cmd | contains("${CLAUDE_PLUGIN_ROOT}/hooks/"))
        or any($scripts[]; . as $script | ($cmd | test("(^|[[:space:]])[^[:space:]]*/hooks/" + $script + "([[:space:]]|$)")))
        or
        any($scripts[]; . as $script | ($cmd | contains("${AGENTOPS_PLUGIN_ROOT") and ($cmd | contains("/hooks/" + $script))))
      );
    .hooks |= with_entries(
      .value |= [
        .[]?
        | .hooks = [
            .hooks[]?
            | select(is_agentops_command(.command // ""))
          ]
        | select((.hooks | length) > 0)
      ]
    )
    | .hooks |= with_entries(select((.value | length) > 0))
  ' "$HOOKS_FILE" > "$tmp_file"

  cp "$HOOKS_FILE" "$backup_file"
  mv "$tmp_file" "$HOOKS_FILE"
  printf '%s\n' "$backup_file"
}

print_text_report() {
  local handlers_json="$1"
  local backup_file="${2:-}"
  local total agentops foreign visible_foreign

  total="$(jq 'length' <<< "$handlers_json")"
  agentops="$(jq '[.[] | select(.agentops)] | length' <<< "$handlers_json")"
  foreign="$(jq '[.[] | select(.agentops | not)] | length' <<< "$handlers_json")"
  visible_foreign="$(jq '[.[] | select((.agentops | not) and .model_visible_event)] | length' <<< "$handlers_json")"

  echo "Codex hooks audit: $HOOKS_FILE"
  echo "Total handlers: $total"
  echo "AgentOps handlers: $agentops"
  echo "Foreign handlers: $foreign"
  echo "Foreign handlers on model-visible events: $visible_foreign"

  if [[ "$foreign" -gt 0 ]]; then
    echo
    echo "Foreign handlers:"
    jq -r '
      .[]
      | select(.agentops | not)
      | "- " + .event + (if .matcher != "" then " [" + .matcher + "]" else "" end) + ": " + .command
    ' <<< "$handlers_json"
  fi

  if [[ -n "$backup_file" ]]; then
    echo
    echo "Pruned foreign handlers."
    echo "Backup: $backup_file"
  fi
}

print_json_report() {
  local handlers_json="$1"
  local backup_file="${2:-}"

  jq -n \
    --arg hooks_file "$HOOKS_FILE" \
    --arg backup_file "$backup_file" \
    --argjson handlers "$handlers_json" \
    '{
      hooks_file: $hooks_file,
      backup_file: (if $backup_file == "" then null else $backup_file end),
      total_handlers: ($handlers | length),
      agentops_handlers: ($handlers | map(select(.agentops)) | length),
      foreign_handlers: ($handlers | map(select(.agentops | not)) | length),
      foreign_model_visible_handlers: ($handlers | map(select((.agentops | not) and .model_visible_event)) | length),
      handlers: $handlers
    }'
}

require_jq
if [[ "$REPAIR_SHAPE" == "true" ]]; then
  repair_hooks_shape
fi
require_hooks_file
validate_hooks_shape

handlers_before="$(collect_handlers)"
foreign_count="$(jq '[.[] | select(.agentops | not)] | length' <<< "$handlers_before")"
backup_path=""

if [[ "$PRUNE_FOREIGN" == "true" && "$foreign_count" -gt 0 ]]; then
  backup_path="$(prune_foreign_handlers)"
  handlers_after="$(collect_handlers)"
else
  handlers_after="$handlers_before"
fi

if [[ "$JSON_OUTPUT" == "true" ]]; then
  print_json_report "$handlers_after" "$backup_path"
else
  print_text_report "$handlers_after" "$backup_path"
fi

remaining_foreign="$(jq '[.[] | select(.agentops | not)] | length' <<< "$handlers_after")"
if [[ "$STRICT" == "true" && "$remaining_foreign" -gt 0 ]]; then
  exit 1
fi
