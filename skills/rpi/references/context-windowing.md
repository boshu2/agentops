# Large-Repo Context Windowing

Use this mode when the repo is too large for stable single-window analysis.

## Why

Trying to read everything in one pass causes context collapse and unstable decisions.
Deterministic shards let `/rpi` process all files incrementally with bounded load.

## Setup Contract

```bash
scripts/rpi/context-window-contract.sh
```

This verifies:
- `GOALS.yaml` is valid
- shard manifest generation works
- shard progress state initializes and validates
- shard runner can traverse shard 1

## Generate Shards

```bash
scripts/rpi/generate-context-shards.py \
  --max-units 80 \
  --max-bytes 300000 \
  --out .agents/rpi/context-shards/latest.json \
  --check
```

## Initialize Progress State

```bash
scripts/rpi/init-shard-progress.py \
  --manifest .agents/rpi/context-shards/latest.json \
  --progress .agents/rpi/context-shards/progress.json \
  --check
```

## Run One Shard (Bounded)

```bash
scripts/rpi/run-shard.py \
  --manifest .agents/rpi/context-shards/latest.json \
  --progress .agents/rpi/context-shards/progress.json \
  --shard-id 1 \
  --limit 20 \
  --mark in_progress \
  --notes "phase-1 analysis start"
```

## Operating Pattern

1. Generate shard manifest once per material repo change.
2. Process one shard at a time.
3. Write concise shard summaries to `.agents/rpi/phase-*.md`.
4. Mark shard status (`todo`, `in_progress`, `done`).
5. Continue until all shards are `done`.

This keeps CPU and context budgets bounded while preserving full-file coverage.
