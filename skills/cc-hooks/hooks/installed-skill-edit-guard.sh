#!/usr/bin/env bash
# installed-skill-edit-guard (PreToolUse / Edit|Write)
# age-workflow-guardrail-hooks-j39.1 — route Edit/Write of an INSTALLED skill copy
# back to the repo source of truth.
#
# The mistake-token: an Edit/Write whose target path is under */.claude/skills/**
# (or .codex/skills, .gemini/skills) has NO legitimate form — those are the
# installed / symlinked copies (overwritten on install; symlinks through to the
# factory checkout). The source of truth is skills/<name>/ in the agentops repo.
#
# Reversible footgun -> ROUTE, not hard-block: exit 2 + a one-line stderr redirect.
#
# Context-budget discipline (hooks are powerful but pollute context — use sparingly):
#   - SILENT on the happy path: any other file_path -> exit 0, zero stdout/stderr.
#   - Fires its one redirect ONLY on an installed-skill-copy edit, at most ONCE
#     per session (sentinel-gated) so it never repeats.
#   - NEVER emits stray stdout on an exit-0 PreToolUse path (stdout there is
#     parsed as JSON). Block via exit 2 + stderr only.
set -uo pipefail

input="$(cat)"
path="$(printf '%s' "$input" | jq -r '.tool_input.file_path // ""')"
sid="$(printf '%s' "$input" | jq -r '.session_id // "nosession"')"

# Match ONLY the file_path: an Edit/Write target under an installed skills dir.
# We match the path segment `.claude/skills/` (or .codex/.gemini) anywhere in the
# path so ~, $HOME, and absolute /Users/*/.claude/skills/** all hit. We match the
# file_path field only — a repo doc whose BODY mentions "claude/skills" lands in
# tool_input.content, never file_path, so prose can never fire this guard.
case "$path" in
  */.claude/skills/*|*/.codex/skills/*|*/.gemini/skills/*)
    : # installed skill copy -> fire
    ;;
  *)
    exit 0  # repo skills/**, any other path -> SILENT happy path
    ;;
esac

dir="${TMPDIR:-/tmp}/claude-installed-skill-edit-guard"
sentinel="$dir/${sid//\//_}"
[ -f "$sentinel" ] && exit 0   # already redirected this session

mkdir -p "$dir" 2>/dev/null || true
: > "$sentinel" 2>/dev/null || true

# Derive the repo-relative target so the redirect is actionable.
name="$(printf '%s' "$path" | sed -n 's#.*/\.\(claude\|codex\|gemini\)/skills/\([^/]*\)/.*#\2#p')"
[ -n "$name" ] || name="$(printf '%s' "$path" | sed -n 's#.*/\.\(claude\|codex\|gemini\)/skills/\([^/]*\)$#\2#p')"
hint="skills/<name>/"
[ -n "$name" ] && hint="skills/${name}/"

cat >&2 <<MSG
⛔ INSTALLED-SKILL EDIT: do not edit installed skill copies.
  ${path}
  is an INSTALLED / symlinked copy — overwritten on install, or symlinked through
  to the factory checkout. Editing it is lost work.
  → Edit ${hint} in the agentops repo (the source of truth) instead.
Fires once per session. Re-run your edit against the repo skills/ path.
MSG
exit 2
