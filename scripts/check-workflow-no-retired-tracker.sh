#!/usr/bin/env bash
# check-workflow-no-retired-tracker.sh — workflow staleness guard (age-u1u6).
#
# The tracker is `br` (beads_rust): BEADS_DIR="$(ao beads dir)" br ...
# bd/Dolt is RETIRED legacy (single-host SPOF, no offline lane). A Claude
# Workflow script that instructs an agent to run a `bd` command sends it at a
# tool that no longer exists.
#
# This shipped uncaught: operating-loop.js — the single most-viewed content
# artifact on the public repo (GitHub traffic) — carried a prompt telling agents
# to "Run `bd ready`". Nothing flagged it. This gate keeps the viewed workflow
# surface from silently re-staling.
#
# Scans ONLY repo-tracked .claude/workflows/*.js for retired bd COMMAND tokens
# (a `bd` word boundary followed by a subcommand or flag). It deliberately does
# NOT match the word "bead", "bdd-foundry", or any `br` command.
#
# Exit 0: no workflow references a retired bd command.
# Exit 1: at least one does (with file:line and the offending text).
#
# Repo root from cwd git. No GNU-only shell constructs.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"

# Retired bd command surface. Word-boundary `bd` + space + subcommand/flag. The
# leading \b also catches the backtick-command form (`\`bd ready\``) in prompts.
pattern='\bbd (ready|update|close|show|create|dep|list|ping|context|init|sync|--json|-)'

# Drive the scan off the TRACKED set, not directory existence: a sparse/broken
# checkout missing .claude/workflows/ must NOT report green for a surface that is
# tracked. If git tracks zero workflows, there is genuinely nothing to protect.
tracked="$(git -C "$repo_root" ls-files '.claude/workflows/*.js')"
if [ -z "$tracked" ]; then
  echo "check-workflow-no-retired-tracker: no tracked .claude/workflows/*.js — nothing to scan."
  exit 0
fi

status=0
findings=""
while IFS= read -r f; do
  [ -n "$f" ] || continue
  path="$repo_root/$f"
  # Fail CLOSED if a tracked workflow we must scan is absent/unreadable in the
  # worktree — never silently skip it (that would let the gate pass unscanned).
  if [ ! -r "$path" ]; then
    echo "FAIL: tracked workflow not readable in worktree: $f" >&2
    exit 1
  fi
  # grep exit: 0=match, 1=no match, 2=error. Distinguish 2 (read/scan error) and
  # fail CLOSED on it; treating it as 1 (the old `2>/dev/null` + plain `if`) was a
  # fail-open. Guard set -e around the expected non-zero (exit 1 = no match).
  set +e
  hits="$(grep -nE "$pattern" "$path")"
  rc=$?
  set -e
  if [ "$rc" -eq 2 ]; then
    echo "FAIL: grep error scanning tracked workflow: $f" >&2
    exit 1
  fi
  if [ "$rc" -eq 0 ]; then
    while IFS= read -r line; do
      [ -n "$line" ] || continue
      findings="${findings}  ${f}:${line}"$'\n'
      status=1
    done <<< "$hits"
  fi
done <<< "$tracked"

if [ "$status" -ne 0 ]; then
  echo "FAIL: retired bd/Dolt tracker command(s) in Claude workflow(s):" >&2
  printf '%s' "$findings" >&2
  echo "" >&2
  echo "The tracker is br: BEADS_DIR=\"\$(ao beads dir)\" br <cmd>. bd/Dolt is retired." >&2
  echo "Repoint the workflow prompt/comment to br." >&2
  exit 1
fi

echo "OK: no Claude workflow references a retired bd command."
