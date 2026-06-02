#!/usr/bin/env python3
"""Phase 2 — salvage importer (bead .10).

Imports the KEEP items from a Phase-1 manifest into bd as typed memories with
provenance. Idempotent: keys are derived deterministically, and bd updates in
place — re-running imports zero duplicates. Records an import-ledger (source path
→ bd key) so Phase-3 retire can verify every keeper landed before deleting
originals. ``--dry-run`` writes nothing.

PACING: bd auto-commits every write to Dolt; a burst can OOM a memory-capped
server (see memory bd-dolt-write-burst-oom). This importer sleeps between writes.

Usage:
    import_memories.py --manifest M.json [--dry-run] [--ledger L.json] [--sleep S]
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
import time
from datetime import datetime, timezone
from pathlib import Path

_HERE = Path(__file__).resolve().parent
if str(_HERE) not in sys.path:
    sys.path.insert(0, str(_HERE))

import mem  # noqa: E402

# Map source layer → memory type (authored knowledge is durable fact/feedback).
_LAYER_TYPE = {
    "agents_knowledge": "fact",
    "agents_learnings": "feedback",
    "claude_memory": "feedback",
}

_SLUG_RE = re.compile(r"[^a-z0-9]+")


def slug_for(path: str) -> str:
    """Deterministic kebab slug from a source path."""
    source = Path(path).as_posix()
    stem = Path(path).stem.lower()
    digest = hashlib.sha256(source.encode()).hexdigest()[:8]
    slug = _SLUG_RE.sub("-", stem).strip("-")
    return f"{slug or 'untitled'}-{digest}"


def memory_for(item: dict[str, object], now_iso: str) -> mem.Memory:
    """Build a typed Memory from a manifest KEEP item (reads the file body)."""
    layer = str(item["layer"])
    path = str(item["path"])
    mem_type = _LAYER_TYPE.get(layer, "fact")
    key = mem.make_key(mem_type, slug_for(path))
    try:
        text = Path(path).read_text(errors="replace").strip()
    except OSError as exc:
        text = f"[import: unreadable source {path}: {exc}]"
    header = mem.MemHeader(
        type=mem_type,
        source=f"file:{path}",
        utility=0.3,
        created_at=now_iso,
        last_accessed=now_iso,
        maturity="established",
    )
    return mem.Memory(key=key, header=header, text=text)


def run_import(
    manifest: dict[str, object],
    store: mem.BdStore,
    dry_run: bool,
    sleep_s: float,
    now: datetime,
) -> dict[str, object]:
    """Import KEEP items; return an import-ledger (source → key)."""
    now_iso = now.isoformat()
    ledger: list[dict[str, str]] = []
    seen_keys: set[str] = set()
    for item in manifest.get("keep", []):  # type: ignore[union-attr]
        m = memory_for(item, now_iso)
        # collision-safe: disambiguate identical slugs from different paths
        key = m.key
        n = 2
        while key in seen_keys:
            key = f"{m.key}-{n}"
            n += 1
        seen_keys.add(key)
        if not dry_run:
            store.remember(key, mem.render(m.header, m.text))
            if sleep_s:
                time.sleep(sleep_s)
        ledger.append({"source": m.header.source, "key": key})
    return {
        "schema": "bd-first-memory.ledger.v1",
        "dry_run": dry_run,
        "imported": len(ledger),
        "entries": ledger,
    }


def main(argv: list[str] | None = None) -> int:
    """CLI entry point. Returns process exit code."""
    parser = argparse.ArgumentParser(description="Import manifest keepers into bd (idempotent).")
    parser.add_argument("--manifest", required=True, help="Phase-1 manifest JSON")
    parser.add_argument("--ledger", default="", help="write import-ledger JSON here")
    parser.add_argument("--dry-run", action="store_true", help="write nothing; emit ledger only")
    parser.add_argument("--sleep", type=float, default=0.2, help="seconds between writes (pacing)")
    args = parser.parse_args(argv)

    manifest = json.loads(Path(args.manifest).expanduser().read_text())
    ledger = run_import(
        manifest, mem.BdStore(), args.dry_run, args.sleep, datetime.now(timezone.utc)
    )
    payload = json.dumps(ledger, indent=2)
    if args.ledger:
        Path(args.ledger).expanduser().write_text(payload)
    mode = "DRY-RUN" if args.dry_run else "imported"
    print(f"{mode}: {ledger['imported']} keepers")
    if not args.ledger:
        print(payload)
    return 0


if __name__ == "__main__":
    sys.exit(main())
