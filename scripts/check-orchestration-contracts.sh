#!/usr/bin/env bash
# check-orchestration-contracts.sh — orchestration tools yaml presence + structure
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
tools="$repo_root/docs/contracts/orchestration-tools.yaml"
profiles="$repo_root/docs/contracts/orchestration-profiles.yaml"
schema="$repo_root/schemas/orchestration-instrument.v1.schema.json"

status=0
fail() { echo "FAIL: $*" >&2; status=1; }

for f in "$tools" "$profiles" "$schema"; do
  if [[ ! -f "$f" ]]; then
    fail "missing contract file: $f"
  fi
done

if ! grep -q '^tools:' "$tools"; then
  fail "orchestration-tools.yaml missing tools section"
fi
if ! grep -q 'version_floors:' "$tools"; then
  fail "orchestration-tools.yaml missing version_floors"
fi
if ! grep -q 'spawn_argv:' "$profiles"; then
  fail "orchestration-profiles.yaml missing structured spawn_argv"
fi
if grep -q 'spawn_flags:' "$profiles"; then
  fail "orchestration-profiles.yaml must not use shell spawn_flags strings"
fi

if [[ $status -eq 0 ]]; then
  echo "OK: orchestration contracts"
fi
exit $status
