#!/usr/bin/env python3
"""Phase 2 — decay-ranked, token-budgeted recall (bead .12).

Replaces substring `bd memories <kw>` with relevance: rank live memories by the
DATA-MODEL decay score (recency × access × type-weight × utility), optionally
filter by type or query substring, and emit the top results within a token
budget. Bumps access_count on the returned memories (the read-path signal that
feeds utility/promotion). Intended for SessionStart injection.

Usage:
    recall.py [--query STR] [--type T] [--max-tokens N] [--no-bump] [--json]
"""

from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

_HERE = Path(__file__).resolve().parent
if str(_HERE) not in sys.path:
    sys.path.insert(0, str(_HERE))

import mem  # noqa: E402


def recall(
    store: mem.BdStore,
    now: datetime,
    query: str = "",
    mem_type: str = "",
    max_tokens: int | None = None,
    bump: bool = True,
) -> list[mem.Memory]:
    """Return decay-ranked memories matching the filters, within budget."""
    candidates = store.list_memories()
    if mem_type:
        candidates = [m for m in candidates if m.header.type == mem_type]
    if query:
        q = query.lower()
        candidates = [m for m in candidates if q in m.key.lower() or q in m.text.lower()]
    ranked = mem.rank(candidates, now, max_tokens)
    if bump:
        stamp = now.isoformat()
        for m in ranked:
            m.header.access_count += 1
            m.header.last_accessed = stamp
            store.remember(m.key, mem.render(m.header, m.text))
    return ranked


def main(argv: list[str] | None = None) -> int:
    """CLI entry point. Returns process exit code."""
    parser = argparse.ArgumentParser(description="Decay-ranked, token-budgeted memory recall.")
    parser.add_argument("--query", default="", help="substring filter on key or body")
    parser.add_argument("--type", default="", dest="mem_type", help="filter to one memory type")
    parser.add_argument("--max-tokens", type=int, default=None, help="token budget for results")
    parser.add_argument("--no-bump", action="store_true", help="do not bump access_count")
    parser.add_argument("--json", action="store_true", help="emit JSON instead of text")
    args = parser.parse_args(argv)

    now = datetime.now(timezone.utc)
    results = recall(
        mem.BdStore(),
        now,
        query=args.query,
        mem_type=args.mem_type,
        max_tokens=args.max_tokens,
        bump=not args.no_bump,
    )
    if args.json:
        rows = [{"key": m.key, "type": m.header.type, "text": m.text} for m in results]
        print(json.dumps(rows, indent=2))
    else:
        for m in results:
            print(f"[{m.header.type}] {m.key}\n  {m.text}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
