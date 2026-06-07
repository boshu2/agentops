#!/usr/bin/env bash
# scope-guard.sh — BeforeTool guard enforcing Flywheel Rule 4 (scoped commits).
#
# Wired into ~/.gemini/settings.json as a BeforeTool/run_shell_command hook. It
# inspects an incoming `git add` / `git commit` and rejects paths outside the
# claimed bead's declared write scope. This is a POLICY STUB: it is wired and
# safe (fails open / no-op) until you implement reading the live tracker's write
# scope for the currently-claimed bead. Implement the marked section against your
# tracker (br/bd) so it ENFORCES rather than no-ops.
#
# Hook contract (AGY): the tool command is passed on stdin or argv depending on
# the harness; this stub reads both. Exit 0 = allow, non-zero = block.
set -euo pipefail

cmd="${*:-}"
if [ -z "$cmd" ] && [ ! -t 0 ]; then cmd="$(cat 2>/dev/null || true)"; fi

# Only act on git add/commit; everything else passes through.
case "$cmd" in
  *"git add"*|*"git commit"*) : ;;
  *) exit 0 ;;
esac

# --- IMPLEMENT: read the claimed bead's write scope and verify staged paths ----
# Example shape (uncomment + adapt to your tracker):
#   bead="$(cat .agents/ratchet/current-bead 2>/dev/null || true)"
#   [ -n "$bead" ] || exit 0   # no claimed bead -> nothing to scope against
#   scope="$(br show "$bead" --json 2>/dev/null | jq -r '.write_scope[]?' || true)"
#   [ -n "$scope" ] || exit 0
#   for p in $(git diff --cached --name-only); do
#     ok=0
#     for s in $scope; do case "$p" in $s*) ok=1;; esac; done
#     [ "$ok" -eq 1 ] || { echo "scope-guard: $p outside bead $bead write scope" >&2; exit 1; }
#   done
# -----------------------------------------------------------------------------

# Stub default: allow (no-op) until the scope read above is implemented.
exit 0
