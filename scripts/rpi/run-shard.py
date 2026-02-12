#!/usr/bin/env python3
"""Bounded shard runner for incremental context-window execution."""

from __future__ import annotations

import argparse
import json
from datetime import datetime, timezone
from pathlib import Path

VALID_STATES = {"todo", "in_progress", "done"}


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def load_json(path: Path) -> dict:
    return json.loads(path.read_text())


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Inspect/update one context shard")
    parser.add_argument("--manifest", default=".agents/rpi/context-shards/latest.json")
    parser.add_argument("--progress", default=".agents/rpi/context-shards/progress.json")
    parser.add_argument("--shard-id", type=int, required=True)
    parser.add_argument("--limit", type=int, default=0, help="Print first N units (0=all)")
    parser.add_argument("--mark", choices=sorted(VALID_STATES), help="Update shard state")
    parser.add_argument("--notes", default="", help="Optional note when marking")
    return parser.parse_args()


def main() -> int:
    args = parse_args()

    manifest_path = Path(args.manifest)
    progress_path = Path(args.progress)

    if not manifest_path.exists():
        raise SystemExit(f"manifest not found: {manifest_path}")
    if not progress_path.exists():
        raise SystemExit(f"progress not found: {progress_path}; run init-shard-progress.py")

    manifest = load_json(manifest_path)
    progress = load_json(progress_path)

    shards = {int(shard["shard_id"]): shard for shard in manifest.get("shards", [])}
    if args.shard_id not in shards:
        raise SystemExit(f"unknown shard id: {args.shard_id}")

    shard = shards[args.shard_id]
    units = shard.get("units", [])

    print(
        f"SHARD {args.shard_id}: units={shard.get('unit_count')} "
        f"budget_bytes={shard.get('budget_bytes')} estimated_tokens={shard.get('estimated_tokens')}"
    )

    max_items = args.limit if args.limit > 0 else len(units)
    for idx, unit in enumerate(units[:max_items], start=1):
        path = unit.get("path")
        kind = unit.get("kind")
        line_start = unit.get("line_start")
        line_end = unit.get("line_end")
        loc = f"L{line_start}-L{line_end}" if line_start and line_end else "metadata"
        print(f"  {idx:03d}. {path} [{kind}] {loc}")

    if args.mark:
        updated = False
        for entry in progress.get("status", []):
            if int(entry.get("shard_id", -1)) == args.shard_id:
                entry["state"] = args.mark
                entry["updated_at"] = now_iso()
                if args.notes:
                    entry["notes"] = args.notes
                updated = True
                break

        if not updated:
            raise SystemExit(f"shard {args.shard_id} missing in progress status")

        progress_path.write_text(json.dumps(progress, indent=2) + "\n")
        print(f"UPDATED: shard {args.shard_id} -> {args.mark}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
