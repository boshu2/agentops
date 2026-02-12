#!/usr/bin/env bash
set -euo pipefail

# Contract: deterministic context-window tooling is available for large-repo
# /rpi workflows and can initialize + traverse bounded shards.

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if [ ! -f GOALS.yaml ]; then
  echo "FAIL: GOALS.yaml not found" >&2
  exit 1
fi

mission_line="$(rg -n '^[[:space:]]*mission:[[:space:]]*' GOALS.yaml | head -n 1 || true)"
if [ -z "$mission_line" ]; then
  echo "FAIL: GOALS.yaml mission field is required" >&2
  exit 1
fi

mission_value="${mission_line#*:}"
mission_value="$(printf '%s' "$mission_value" | sed -E "s/^[[:space:]]+//; s/[[:space:]]+$//; s/^['\"]//; s/['\"]$//")"
if [ "${#mission_value}" -lt 20 ]; then
  echo "FAIL: GOALS.yaml mission must be at least 20 chars" >&2
  exit 1
fi

if ! rg -q '^[[:space:]]*goals:[[:space:]]*$' GOALS.yaml; then
  echo "FAIL: GOALS.yaml goals list is required" >&2
  exit 1
fi

goal_count="$(rg -c '^[[:space:]]*-[[:space:]]*id:[[:space:]]*[^[:space:]]+' GOALS.yaml || true)"
if [ "${goal_count:-0}" -lt 1 ]; then
  echo "FAIL: GOALS.yaml goals list must include at least one id entry" >&2
  exit 1
fi

echo "PASS: goals schema contract"

scripts/rpi/generate-context-shards.py \
  --max-units 80 \
  --max-bytes 300000 \
  --out .agents/rpi/context-shards/latest.json \
  --check \
  --quiet

scripts/rpi/init-shard-progress.py \
  --manifest .agents/rpi/context-shards/latest.json \
  --progress .agents/rpi/context-shards/progress.json \
  --check \
  --quiet

scripts/rpi/run-shard.py \
  --manifest .agents/rpi/context-shards/latest.json \
  --progress .agents/rpi/context-shards/progress.json \
  --shard-id 1 \
  --limit 1 >/dev/null

echo "PASS: context-window contract"
