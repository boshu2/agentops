#!/usr/bin/env python3
"""Generate deterministic context-window shards for full-repo reading.

The manifest covers every tracked file present in the working tree (via
`git ls-files`), splitting large text files into line chunks and representing
binary assets as metadata units.
"""

from __future__ import annotations

import argparse
import json
import math
import subprocess
import sys
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


@dataclass
class ReadUnit:
    path: str
    kind: str  # text-full | text-chunk | binary-metadata
    size_bytes: int
    budget_bytes: int
    line_start: int | None = None
    line_end: int | None = None
    chunk_index: int | None = None
    chunk_count: int | None = None


@dataclass
class Shard:
    shard_id: int
    unit_count: int
    budget_bytes: int
    estimated_tokens: int
    units: list[ReadUnit]


def list_tracked_files() -> list[str]:
    # Prefer git for determinism and to avoid depending on ripgrep being installed.
    try:
        out = subprocess.check_output(["git", "ls-files"], text=True)
    except (FileNotFoundError, subprocess.CalledProcessError):
        out = subprocess.check_output(["rg", "--files"], text=True)

    # Some environments use sparse/partial checkouts (skip-worktree). Filter to
    # files that actually exist so the shard generator is runnable everywhere.
    files: list[str] = []
    for line in out.splitlines():
        path = line.strip()
        if not path:
            continue
        if Path(path).is_file():
            files.append(path)

    files.sort()
    return files


def is_binary(path: Path) -> bool:
    try:
        sample = path.read_bytes()[:4096]
    except OSError:
        return False
    return b"\x00" in sample


def count_lines(path: Path) -> int:
    data = path.read_bytes()
    if not data:
        return 1
    return data.count(b"\n") + (0 if data.endswith(b"\n") else 1)


def units_for_file(path: str, chunk_target_bytes: int) -> list[ReadUnit]:
    file_path = Path(path)
    size = file_path.stat().st_size

    if is_binary(file_path):
        return [
            ReadUnit(
                path=path,
                kind="binary-metadata",
                size_bytes=size,
                budget_bytes=min(2048, max(1, size)),
            )
        ]

    lines = count_lines(file_path)
    if size <= chunk_target_bytes:
        return [
            ReadUnit(
                path=path,
                kind="text-full",
                size_bytes=size,
                budget_bytes=max(1, size),
                line_start=1,
                line_end=lines,
                chunk_index=1,
                chunk_count=1,
            )
        ]

    chunk_count = max(2, math.ceil(size / chunk_target_bytes))
    lines_per_chunk = max(1, math.ceil(lines / chunk_count))
    bytes_per_chunk = max(1, math.ceil(size / chunk_count))

    units: list[ReadUnit] = []
    start = 1
    for idx in range(1, chunk_count + 1):
        end = min(lines, start + lines_per_chunk - 1)
        units.append(
            ReadUnit(
                path=path,
                kind="text-chunk",
                size_bytes=size,
                budget_bytes=bytes_per_chunk,
                line_start=start,
                line_end=end,
                chunk_index=idx,
                chunk_count=chunk_count,
            )
        )
        if end >= lines:
            break
        start = end + 1

    return units


def pack_shards(units: list[ReadUnit], max_units: int, max_bytes: int) -> list[Shard]:
    shards: list[Shard] = []
    current: list[ReadUnit] = []
    current_bytes = 0

    def flush() -> None:
        nonlocal current, current_bytes
        if not current:
            return
        shard_id = len(shards) + 1
        shards.append(
            Shard(
                shard_id=shard_id,
                unit_count=len(current),
                budget_bytes=current_bytes,
                estimated_tokens=math.ceil(current_bytes / 4),
                units=current,
            )
        )
        current = []
        current_bytes = 0

    for unit in units:
        exceeds = (
            len(current) >= max_units
            or (current and current_bytes + unit.budget_bytes > max_bytes)
        )
        if exceeds:
            flush()

        current.append(unit)
        current_bytes += unit.budget_bytes

        if len(current) >= max_units or current_bytes >= max_bytes:
            flush()

    flush()
    return shards


def verify_manifest(manifest: dict[str, Any], files: list[str], max_units: int, max_bytes: int) -> None:
    shard_entries = manifest.get("shards", [])
    if not isinstance(shard_entries, list) or not shard_entries:
        raise ValueError("manifest has no shards")

    coverage = {path: 0 for path in files}

    for shard in shard_entries:
        units = shard.get("units", [])
        if len(units) > max_units:
            raise ValueError(f"shard {shard.get('shard_id')} exceeds max_units={max_units}")
        if int(shard.get("budget_bytes", 0)) > max_bytes:
            raise ValueError(f"shard {shard.get('shard_id')} exceeds max_bytes={max_bytes}")

        for unit in units:
            path = unit.get("path")
            if path not in coverage:
                raise ValueError(f"unknown file in unit: {path}")
            coverage[path] += 1

    missing = [path for path, count in coverage.items() if count == 0]
    if missing:
        raise ValueError(f"coverage gap: {len(missing)} files missing (example: {missing[0]})")


def build_manifest(files: list[str], max_units: int, max_bytes: int) -> dict[str, Any]:
    all_units: list[ReadUnit] = []
    for path in files:
        all_units.extend(units_for_file(path, chunk_target_bytes=max_bytes))

    shards = pack_shards(all_units, max_units=max_units, max_bytes=max_bytes)
    repo_bytes = sum(Path(path).stat().st_size for path in files)
    budget_bytes = sum(shard.budget_bytes for shard in shards)

    return {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "root": str(Path.cwd()),
        "settings": {
            "max_units": max_units,
            "max_bytes": max_bytes,
        },
        "totals": {
            "files": len(files),
            "units": len(all_units),
            "shards": len(shards),
            "repo_bytes": repo_bytes,
            "budget_bytes": budget_bytes,
            "estimated_tokens": math.ceil(budget_bytes / 4),
        },
        "shards": [
            {
                "shard_id": shard.shard_id,
                "unit_count": shard.unit_count,
                "budget_bytes": shard.budget_bytes,
                "estimated_tokens": shard.estimated_tokens,
                "units": [asdict(unit) for unit in shard.units],
            }
            for shard in shards
        ],
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Generate deterministic context shards")
    parser.add_argument("--max-units", type=int, default=80, help="Max units per shard")
    parser.add_argument("--max-bytes", type=int, default=300000, help="Max budget bytes per shard")
    parser.add_argument(
        "--out",
        default=".agents/rpi/context-shards/latest.json",
        help="Output manifest path",
    )
    parser.add_argument("--check", action="store_true", help="Validate manifest limits and coverage")
    parser.add_argument("--quiet", action="store_true", help="Suppress summary output")
    return parser.parse_args()


def main() -> int:
    args = parse_args()

    if args.max_units <= 0:
        print("max-units must be > 0", file=sys.stderr)
        return 2
    if args.max_bytes <= 0:
        print("max-bytes must be > 0", file=sys.stderr)
        return 2

    files = list_tracked_files()
    manifest = build_manifest(files, max_units=args.max_units, max_bytes=args.max_bytes)

    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(manifest, indent=2) + "\n")

    if args.check:
        try:
            verify_manifest(manifest, files, max_units=args.max_units, max_bytes=args.max_bytes)
        except ValueError as exc:
            print(f"FAIL: {exc}", file=sys.stderr)
            return 1

    if not args.quiet:
        totals = manifest["totals"]
        print(
            "PASS: generated context shards "
            f"files={totals['files']} units={totals['units']} shards={totals['shards']} "
            f"budget_bytes={totals['budget_bytes']} estimated_tokens={totals['estimated_tokens']} "
            f"out={out_path}"
        )

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
