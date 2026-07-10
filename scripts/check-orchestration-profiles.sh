#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
profiles="$repo_root/docs/contracts/orchestration-profiles.yaml"
lifecycle="$repo_root/skills/agent-native/references/agent-lifecycle.md"

status=0
fail() { echo "FAIL: $*" >&2; status=1; }
[ -f "$profiles" ] || fail "missing $profiles"
[ -f "$lifecycle" ] || fail "missing $lifecycle"

for token in '--robot-spawn=${session}' '--spawn-cod=1' '--spawn-cod=2' '--spawn-no-user' '--spawn-wait' '--spawn-dir=${worktree}' '--spawn-cc=1' '--spawn-agy=1'; do
  grep -F -- "$token" "$profiles" >/dev/null || fail "profiles missing live NTM token $token"
done
for role in Orchestrator Worker Verifier Scribe Heartbeat; do
  grep -q "$role" "$profiles" || fail "profiles missing role $role"
done
for id in ntm-workers ntm-pawl dual-pane tri-vendor; do
  grep -q "profile_id: $id" "$profiles" || fail "profiles missing $id"
done
grep -q 'suspect' "$lifecycle" || fail "agent lifecycle missing suspicion state"
grep -q 'nudged' "$lifecycle" || fail "agent lifecycle missing bounded nudge state"

[ "$status" -ne 0 ] || echo "OK: orchestration profiles drift gate"
exit "$status"
