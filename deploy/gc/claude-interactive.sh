#!/usr/bin/env bash
# City-local wrapper for GC-owned interactive Claude sessions.
set -u

claude_bin="__GC_AGENTOPS_CLAUDE_BIN__"
script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
diagnostics_dir="$script_dir/../runtime/claude-diagnostics"
mkdir -p "$diagnostics_dir"
chmod 700 "$diagnostics_dir"

invocation="claude-$(date -u '+%Y%m%dT%H%M%SZ')-$$"
debug_file="$diagnostics_dir/$invocation.debug.log"
exit_file="$diagnostics_dir/$invocation.exit"

# Keep the provider interactive. --debug-file only redirects Claude's own
# diagnostic stream; prompts and responses remain attached to the GC tmux pane.
"$claude_bin" --debug-file "$debug_file" "$@"
status=$?
printf 'exit_code=%s\nfinished_at=%s\n' \
  "$status" "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" >"$exit_file"
chmod 600 "$debug_file" "$exit_file" 2>/dev/null || true
exit "$status"
