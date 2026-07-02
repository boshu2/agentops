#!/usr/bin/env python3
"""ms-adoption-probe.py — measure ms (meta_skill) adoption vs hand-grepping.

READ-ONLY. Scans Claude Code session transcripts (``~/.claude/projects/**/*.jsonl``)
over a date window and counts two competing behaviours:

  (a) ms-search events   — the agent reached for the skill index:
        * ``mcp__ms__search`` tool_use blocks
        * Bash commands matching ``\\bms search\\b``
  (b) hand-grep events   — the agent hand-searched skill files instead:
        * ``Grep`` / ``Glob`` tool_use whose pattern/path/glob targets a ``skills/`` dir
        * Bash commands running ``grep``/``rg`` over a ``skills/`` path

It emits a small JSON summary ``{window, ms_calls, grep_calls, ratio, ...}``.
Run a pre-announcement window for a BASELINE, then re-measure after adoption to
see whether the outcome ritual / "ms search first" encoding actually moved
behaviour (external validation beats self-report).

Usage:
    scripts/ms-adoption-probe.py --days 14
    scripts/ms-adoption-probe.py --since 2026-06-18 --out docs/evals/ms-adoption-baseline.json
    scripts/ms-adoption-probe.py --projects-dir /path/to/projects --days 14

Nothing is written unless --out is given. Never mutates transcripts.
"""

from __future__ import annotations

import argparse
import datetime as _dt
import json
import os
import re
import sys
from pathlib import Path
from typing import Iterable

# --- classifiers -----------------------------------------------------------

# Bash command matches an actual `ms search ...` invocation (spec heuristic).
_MS_SEARCH_BASH = re.compile(r"\bms search\b")
# Bash command that hand-greps a skills/ path.
_GREP_TOKEN = re.compile(r"\b(grep|rg)\b")
_SKILLS_PATH = re.compile(r"(^|[\s'\"/(=])skills/")


def _targets_skills(*values: object) -> bool:
    """True if any provided value (str) references a ``skills/`` path or dir."""
    for v in values:
        if isinstance(v, str) and _SKILLS_PATH.search(v):
            return True
    return False


def classify_tool_use(name: str, tool_input: dict) -> str | None:
    """Return 'ms', 'grep', or None for a single tool_use block.

    Pure + deterministic so it is unit-testable in isolation.
    """
    if not name:
        return None
    if name == "mcp__ms__search":
        return "ms"
    if name in ("Grep", "Glob"):
        if _targets_skills(
            tool_input.get("path"),
            tool_input.get("glob"),
            tool_input.get("pattern"),
        ):
            return "grep"
        return None
    if name == "Bash":
        cmd = tool_input.get("command") or ""
        if _MS_SEARCH_BASH.search(cmd):
            return "ms"
        if _GREP_TOKEN.search(cmd) and _SKILLS_PATH.search(cmd):
            return "grep"
        return None
    return None


def _parse_ts(raw: object) -> _dt.datetime | None:
    if not isinstance(raw, str) or not raw:
        return None
    try:
        return _dt.datetime.fromisoformat(raw.replace("Z", "+00:00"))
    except ValueError:
        return None


def count_records(records: Iterable[dict], cutoff: _dt.datetime | None) -> dict:
    """Count ms vs grep events over records, honouring the ``cutoff`` window.

    A record with no parseable timestamp is skipped when a cutoff is set (we
    cannot place it in the window honestly). ``records`` is any iterable of the
    parsed transcript-line dicts.
    """
    ms = 0
    grep = 0
    for rec in records:
        if not isinstance(rec, dict):
            continue
        if cutoff is not None:
            ts = _parse_ts(rec.get("timestamp"))
            if ts is None or ts < cutoff:
                continue
        msg = rec.get("message")
        if not isinstance(msg, dict):
            continue
        content = msg.get("content")
        if not isinstance(content, list):
            continue
        for block in content:
            if not (isinstance(block, dict) and block.get("type") == "tool_use"):
                continue
            kind = classify_tool_use(block.get("name", ""), block.get("input") or {})
            if kind == "ms":
                ms += 1
            elif kind == "grep":
                grep += 1
    return {"ms_calls": ms, "grep_calls": grep}


def _iter_lines(path: Path) -> Iterable[dict]:
    try:
        fh = path.open("r", encoding="utf-8", errors="replace")
    except OSError:
        return
    with fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            try:
                yield json.loads(line)
            except json.JSONDecodeError:
                continue


def run_probe(projects_dir: Path, cutoff: _dt.datetime | None) -> dict:
    ms = 0
    grep = 0
    sessions = 0
    for jsonl in sorted(projects_dir.glob("*/*.jsonl")):
        sessions += 1
        c = count_records(_iter_lines(jsonl), cutoff)
        ms += c["ms_calls"]
        grep += c["grep_calls"]
    ratio = round(ms / grep, 3) if grep else None
    return {"ms_calls": ms, "grep_calls": grep, "ratio": ratio, "sessions_scanned": sessions}


def _resolve_cutoff(args) -> _dt.datetime | None:
    now = _dt.datetime.now(_dt.timezone.utc)
    if args.since:
        cut = _parse_ts(args.since)
        if cut is None:
            # bare YYYY-MM-DD
            try:
                cut = _dt.datetime.strptime(args.since, "%Y-%m-%d").replace(
                    tzinfo=_dt.timezone.utc
                )
            except ValueError:
                sys.exit(f"error: --since not an ISO date/datetime: {args.since!r}")
        if cut.tzinfo is None:
            cut = cut.replace(tzinfo=_dt.timezone.utc)
        return cut
    if args.days is not None:
        return now - _dt.timedelta(days=args.days)
    return None


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description="Measure ms adoption vs hand-grepping.")
    p.add_argument("--since", help="Window start: YYYY-MM-DD or ISO datetime.")
    p.add_argument("--days", type=int, help="Window = last N days (alternative to --since).")
    p.add_argument(
        "--projects-dir",
        default=os.path.expanduser("~/.claude/projects"),
        help="Claude transcripts root (default: ~/.claude/projects).",
    )
    p.add_argument("--out", help="Write JSON here instead of stdout.")
    args = p.parse_args(argv)
    if args.since is None and args.days is None:
        args.days = 14  # default baseline window

    cutoff = _resolve_cutoff(args)
    now = _dt.datetime.now(_dt.timezone.utc)
    projects_dir = Path(args.projects_dir)
    if not projects_dir.is_dir():
        sys.exit(f"error: projects dir not found: {projects_dir}")

    result = run_probe(projects_dir, cutoff)
    result = {
        "window": {
            "since": cutoff.isoformat() if cutoff else None,
            "until": now.isoformat(),
        },
        **result,
        "generated_at": now.isoformat(),
        "projects_dir": str(projects_dir),
    }
    payload = json.dumps(result, indent=2)
    if args.out:
        Path(args.out).write_text(payload + "\n", encoding="utf-8")
        print(f"wrote {args.out}", file=sys.stderr)
    print(payload)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
