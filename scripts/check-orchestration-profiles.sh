#!/usr/bin/env bash
# check-orchestration-profiles.sh — drift gate for profiles yaml ↔ dual-pane-atm checklist
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
profiles="$repo_root/docs/contracts/orchestration-profiles.yaml"
checklist="$repo_root/skills/dual-pane-atm/references/spawn-checklist.md"

if [[ ! -f "$profiles" ]]; then
  echo "FAIL: missing $profiles" >&2
  exit 1
fi
if [[ ! -f "$checklist" ]]; then
  echo "FAIL: missing $checklist" >&2
  exit 1
fi

status=0
fail() { echo "FAIL: $*" >&2; status=1; }

for token in "--no-user" "--cc=1:opus" "--cod=1:gpt-5.5" "--agy=1"; do
  if ! grep -F -- "$token" "$profiles" >/dev/null; then
    fail "profiles yaml missing spawn token $token"
  fi
done

for marker in "ao orchestrate preflight" "ao orchestrate verify"; do
  if ! grep -qF "$marker" "$checklist"; then
    fail "spawn-checklist missing instrument lane marker: $marker"
  fi
done

for id in dual-pane tri-vendor; do
  if ! grep -q "profile_id: $id" "$profiles"; then
    fail "profiles yaml missing profile_id $id"
  fi
done

if [[ $status -eq 0 ]]; then
  echo "OK: orchestration profiles drift gate"
fi
exit $status
