#!/usr/bin/env bash
# agent-output-validate.sh — the authoritative gate over agent-produced output.
#
# Runs `ao validate --gate` (the umbrella that composes the ratchet / standards /
# scenario validators) against whatever an out-of-session agent produced — a PR
# branch checkout or an artifact bundle — and propagates the PASS/WARN/FAIL
# verdict to this process's exit code. This is the SAME authoritative gate
# interactive work hits in validate.yml; CI is the gate, not a hook (AgentOps
# 3.0 is hookless). Called by .github/workflows/agent-output-validate.yml and
# usable directly by an NTM lead pane or a self-hosted sandbox.
#
# Usage:
#   agent-output-validate.sh [ao validate flags...]
#   agent-output-validate.sh --changes plan.md
#   agent-output-validate.sh --bead ag-123 --strict
#
# Exit codes (mirrors `ao validate --gate`):
#   0  PASS or WARN (gate passes)
#   1  FAIL (gate fails — a violation was found)
#   2  internal/usage error (could not run the gate, e.g. ao not found)
#
# Env:
#   AO_BIN  override the `ao` binary (default: `ao` on PATH). Lets CI point at a
#           freshly built binary and lets tests inject a fixture.
set -euo pipefail

AO_BIN="${AO_BIN:-ao}"

# Resolve the ao binary loudly — a missing gate must never read as a clean pass.
if ! command -v "$AO_BIN" >/dev/null 2>&1 && [ ! -x "$AO_BIN" ]; then
  echo "agent-output-validate: ao binary not found (AO_BIN=$AO_BIN). Build it first (cd cli && go install ./cmd/ao) or set AO_BIN." >&2
  exit 2
fi

echo "agent-output-validate: running the authoritative gate on agent-produced output"
echo "agent-output-validate: \$ $AO_BIN validate --gate $*"

rc=0
"$AO_BIN" validate --gate "$@" || rc=$?

case "$rc" in
  0) echo "agent-output-validate: PASS — agent output cleared the gate" ;;
  1) echo "agent-output-validate: FAIL — agent output violated the gate (not mergeable)" >&2 ;;
  *) echo "agent-output-validate: ERROR — gate could not run (exit $rc)" >&2 ;;
esac

exit "$rc"
