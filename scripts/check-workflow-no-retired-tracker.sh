#!/usr/bin/env bash
# check-workflow-no-retired-tracker.sh — workflow tracker-drift guard (age-u1u6).
#
# THIS repo's own tracker is `br` (beads_rust): BEADS_DIR="$(ao beads dir)" br ...
# A Claude Workflow script that instructs an agent to run a `bd` command for
# AgentOps' own issue tracking is sending it at the wrong tracker — br is correct
# here.
#
# Two-store nuance (age-gc-adoption-u0he): bd/dolt is NOT retired globally — it is
# the gascity SUBSTRATE store (first-class, embraced, what a gas-city factory runs
# on natively). That is a different LAYER from this repo's br tracker. This gate
# protects the agentops-tracking context; a workflow line that legitimately
# documents the substrate store must carry the `gascity-substrate` marker to be
# exempt (see EXEMPT_MARKER below).
#
# This shipped uncaught: operating-loop.js — the single most-viewed content
# artifact on the public repo (GitHub traffic) — carried a prompt telling agents
# to "Run `bd ready`". Nothing flagged it. This gate keeps the viewed workflow
# surface from silently drifting off br.
#
# Scans ONLY repo-tracked .claude/workflows/*.js for bd COMMAND tokens (a `bd`
# word boundary followed by a subcommand or flag). It deliberately does NOT match
# the word "bead", "bdd-foundry", or any `br` command.
#
# Exit 0: no workflow drifts to a bd tracker command (substrate-marked lines OK).
# Exit 1: at least one does (with file:line and the offending text).
#
# Repo root from cwd git. No GNU-only shell constructs.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"

# bd command surface. Word-boundary `bd` + space + subcommand/flag. The leading
# \b also catches the backtick-command form (`\`bd ready\``) in prompts.
pattern='\bbd (ready|update|close|show|create|dep|list|ping|context|init|sync|--json|-)'

# Substrate carve-out (age-gc-adoption-u0he): a line documenting the gascity
# SUBSTRATE store (bd/dolt as the factory's native store, a different layer from
# this repo's br tracker) and explicitly annotated with this marker is legitimate,
# not tracker drift, and is exempt from the bd-command check.
EXEMPT_MARKER='gascity-substrate'

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
      # Substrate carve-out: skip lines that explicitly document the gascity
      # substrate store — bd/dolt there is the factory's native store, not this
      # repo's tracker drifting off br.
      case "$line" in
        *"$EXEMPT_MARKER"*) continue ;;
      esac
      findings="${findings}  ${f}:${line}"$'\n'
      status=1
    done <<< "$hits"
  fi
done <<< "$tracked"

if [ "$status" -ne 0 ]; then
  echo "FAIL: bd tracker command(s) in Claude workflow(s) — this repo tracks with br:" >&2
  printf '%s' "$findings" >&2
  echo "" >&2
  echo "This repo's tracker is br: BEADS_DIR=\"\$(ao beads dir)\" br <cmd>." >&2
  echo "bd/dolt is the gascity SUBSTRATE store (a different layer), not this repo's tracker." >&2
  echo "Repoint the workflow prompt/comment to br — or, if it genuinely documents the" >&2
  echo "substrate store, mark that line 'gascity-substrate'." >&2
  exit 1
fi

echo "OK: no Claude workflow drifts off br to a bd tracker command."
