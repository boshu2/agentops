#!/usr/bin/env bash
# impl.sh — PLACEHOLDER implementation for quest {{QUEST}}.
#
# Deliberately UNIMPLEMENTED: every entrypoint returns NOT_IMPLEMENTED (nonzero,
# no stdout) so the acceptance harness (./test.sh) starts RED and stays red until
# a real build lands. The BUILDER replaces this file's body inside its own
# worktree; the PLANNER never touches it (RBAC: the planner's one write surface
# is scaffolding a quest from the template, never editing impl code).
set -uo pipefail

echo "NOT_IMPLEMENTED: ${1:-<no-arg>}" >&2
exit 1
