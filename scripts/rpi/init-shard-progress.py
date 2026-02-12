#!/usr/bin/env python3
"""Initialize and validate context-shard progress state."""

from __future__ import annotations

import argparse
import json
from datetime import datetime, timezone
from pathlib import Path


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def load_json(path: Path) -> dict:
    return json.loads(path.read_text())


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Initialize/validate shard progress")
    parser.add_argument("--manifest", default=".agents/rpi/context-shards/latest.json")
    parser.add_argument("--progress", default=".agents/rpi/context-shards/progress.json")
    parser.add_argument("--check", action="store_true", help="Fail on mismatch instead of auto-heal")
    parser.add_argument("--quiet", action="store_true", help="Suppress summary output")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    manifest_path = Path(args.manifest)
    progress_path = Path(args.progress)

    if not manifest_path.exists():
        raise SystemExit(f"manifest not found: {manifest_path}")

    manifest = load_json(manifest_path)
    shard_ids = [int(shard["shard_id"]) for shard in manifest.get("shards", [])]
    if not shard_ids:
        raise SystemExit("manifest has no shards")

    if not progress_path.exists():
        progress_path.parent.mkdir(parents=True, exist_ok=True)
        payload = {
            "created_at": now_iso(),
            "manifest": str(manifest_path),
            "status": [
                {
                    "shard_id": shard_id,
                    "state": "todo",  # todo | in_progress | done
                    "updated_at": None,
                    "notes": "",
                }
                for shard_id in shard_ids
            ],
        }
        progress_path.write_text(json.dumps(payload, indent=2) + "\n")

    progress = load_json(progress_path)
    status = progress.get("status", [])
    by_id = {int(item.get("shard_id")): item for item in status if "shard_id" in item}

    missing = [shard_id for shard_id in shard_ids if shard_id not in by_id]
    extra = [shard_id for shard_id in by_id if shard_id not in set(shard_ids)]

    if missing or extra:
        if args.check:
            raise SystemExit(f"progress mismatch: missing={missing[:5]} extra={extra[:5]}")

        merged = []
        for shard_id in shard_ids:
            merged.append(
                by_id.get(
                    shard_id,
                    {
                        "shard_id": shard_id,
                        "state": "todo",
                        "updated_at": None,
                        "notes": "",
                    },
                )
            )
        progress["status"] = merged
        progress_path.write_text(json.dumps(progress, indent=2) + "\n")

    if not args.quiet:
        todo = sum(1 for item in progress["status"] if item.get("state") == "todo")
        in_progress = sum(1 for item in progress["status"] if item.get("state") == "in_progress")
        done = sum(1 for item in progress["status"] if item.get("state") == "done")
        print(
            f"PASS: shard progress ready total={len(shard_ids)} "
            f"todo={todo} in_progress={in_progress} done={done} path={progress_path}"
        )

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
