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

python3 - <<'PY'
from __future__ import annotations

import sys
from pathlib import Path

import yaml


def fail(msg: str) -> None:
    print(f"FAIL: {msg}", file=sys.stderr)
    raise SystemExit(1)

path = Path("GOALS.yaml")
try:
    data = yaml.safe_load(path.read_text())
except Exception as exc:
    fail(f"Unable to parse GOALS.yaml: {exc}")

if not isinstance(data, dict):
    fail("GOALS.yaml must parse to a mapping")

mission = data.get("mission")
if not isinstance(mission, str) or len(mission.strip()) < 20:
    fail("GOALS.yaml mission must be a descriptive string")

goals = data.get("goals")
if not isinstance(goals, list) or not goals:
    fail("GOALS.yaml must include a non-empty goals list")

print("PASS: goals schema contract")
PY

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
