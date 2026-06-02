#!/usr/bin/env python3
"""Phase 2 — unified write entrypoint (bead .13).

The single `remember` interface every harness routes to, so memory has one
durable home (bd) instead of one store per harness. Stamps type, provenance, and
timestamps per DATA-MODEL, then upserts via the shared wrapper. Wire each harness
to call this instead of its own private store:

  - Claude auto-memory hook  → remember.py --type ... --source session:<id> "<body>"
  - Codex / OpenClaw memory  → same CLI

Usage:
    remember.py --type T --slug S [--source SRC] [--maturity M] "body text"
"""

from __future__ import annotations

import argparse
import sys
from datetime import datetime, timezone
from pathlib import Path

_HERE = Path(__file__).resolve().parent
if str(_HERE) not in sys.path:
    sys.path.insert(0, str(_HERE))

import mem  # noqa: E402


def write_memory(
    store: mem.BdStore,
    mem_type: str,
    slug: str,
    body: str,
    source: str,
    maturity: str,
    now: datetime,
) -> str:
    """Upsert a typed memory and return its key."""
    key = mem.make_key(mem_type, slug)
    now_iso = now.isoformat()
    header = mem.MemHeader(
        type=mem_type,
        source=source,
        created_at=now_iso,
        last_accessed=now_iso,
        maturity=maturity,
    )
    store.remember(key, mem.render(header, body.strip()))
    return key


def main(argv: list[str] | None = None) -> int:
    """CLI entry point. Returns process exit code."""
    parser = argparse.ArgumentParser(description="Unified typed memory write (all harnesses → bd).")
    parser.add_argument("--type", required=True, dest="mem_type", choices=mem.VALID_TYPES)
    parser.add_argument("--slug", required=True, help="kebab slug for the key")
    parser.add_argument("--source", default="", help="provenance, e.g. session:<id>")
    parser.add_argument("--maturity", default="provisional", help="maturity tier")
    parser.add_argument("body", help="the memory body text")
    args = parser.parse_args(argv)

    key = write_memory(
        mem.BdStore(),
        args.mem_type,
        args.slug,
        args.body,
        args.source,
        args.maturity,
        datetime.now(timezone.utc),
    )
    print(f"remembered {key}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
