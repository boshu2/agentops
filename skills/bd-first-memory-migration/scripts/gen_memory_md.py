#!/usr/bin/env python3
"""Phase 2 — regenerate the thin MEMORY.md index from bd (bead .14).

MEMORY.md becomes a GENERATED, read-only cache: one line per live memory, grouped
by type, rebuilt from bd. This permanently fixes the bloat/corruption mode (a
201-line index full of pasted session blobs) because the file is never hand-edited
— the bd memory is the source of truth. Intended to run at SessionStart.

Usage:
    gen_memory_md.py [--out PATH] [--max-lines N]
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

_HERE = Path(__file__).resolve().parent
if str(_HERE) not in sys.path:
    sys.path.insert(0, str(_HERE))

import mem  # noqa: E402

_GENERATED_BANNER = "<!-- GENERATED from bd by bd-first-memory-migration; do not hand-edit -->"
_TYPE_ORDER = ("feedback", "project", "procedural", "fact", "episodic")


def _first_line(text: str) -> str:
    """Collapse a memory body to a single-line summary."""
    for line in text.splitlines():
        line = line.strip()
        if line:
            return line[:160]
    return ""


def render_index(memories: list[mem.Memory], max_lines: int) -> str:
    """Render the thin one-line-per-memory index, grouped by type."""
    live = [m for m in memories if m.header.superseded_by is None]
    by_type: dict[str, list[mem.Memory]] = {}
    for m in live:
        by_type.setdefault(m.header.type, []).append(m)

    lines = [_GENERATED_BANNER, ""]
    emitted = 0
    for mem_type in _TYPE_ORDER:
        group = sorted(by_type.get(mem_type, []), key=lambda m: m.key)
        if not group:
            continue
        lines.append(f"## {mem_type}")
        for m in group:
            if emitted >= max_lines:
                lines.append(f"- _… {len(live) - emitted} more memories truncated at {max_lines}_")
                return "\n".join(lines) + "\n"
            lines.append(f"- `{m.key}` — {_first_line(m.text)}")
            emitted += 1
        lines.append("")
    return "\n".join(lines).rstrip() + "\n"


def main(argv: list[str] | None = None) -> int:
    """CLI entry point. Returns process exit code."""
    parser = argparse.ArgumentParser(description="Regenerate the thin MEMORY.md index from bd.")
    parser.add_argument("--out", default="", help="write index to this path (default: stdout)")
    parser.add_argument("--max-lines", type=int, default=180, help="cap index entries (<200 limit)")
    args = parser.parse_args(argv)

    index = render_index(mem.BdStore().list_memories(), args.max_lines)
    if args.out:
        Path(args.out).expanduser().write_text(index)
        print(f"MEMORY.md -> {args.out} ({index.count(chr(10)) + 1} lines)")
    else:
        print(index)
    return 0


if __name__ == "__main__":
    sys.exit(main())
