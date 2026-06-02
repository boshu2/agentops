#!/usr/bin/env python3
"""Phase 2 — typed memory wrapper over bd (bead .11).

Implements the DATA-MODEL contract on top of bd memory keys: a fenced
``<!--mem ... -->`` header carries structured fields; the key prefix carries the
type. Pure parse/render/rank helpers (no I/O) plus an injectable ``BdStore``
adapter so callers — importer (.10), recall (.12), write-path (.13), MEMORY.md
generator (.14) — share one implementation, and tests run without the bd CLI.
"""

from __future__ import annotations

import math
import subprocess
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Callable

VALID_TYPES = ("fact", "feedback", "project", "episodic", "procedural")

# Per-type decay half-life in days (DATA-MODEL.md).
TYPE_HALF_LIFE: dict[str, float] = {
    "fact": 90.0,
    "feedback": 365.0,
    "project": 120.0,
    "episodic": 14.0,
    "procedural": 365.0,
}

# Per-type recall weight (procedural/feedback rank above episodic).
TYPE_WEIGHT: dict[str, float] = {
    "procedural": 1.3,
    "feedback": 1.2,
    "fact": 1.0,
    "project": 1.0,
    "episodic": 0.7,
}

_HEADER_OPEN = "<!--mem"
_HEADER_CLOSE = "-->"


@dataclass
class MemHeader:
    """Structured fields stored in the fenced ``<!--mem-->`` block."""

    type: str = "fact"
    source: str = ""
    utility: float = 0.0
    access_count: int = 0
    created_at: str = ""
    last_accessed: str = ""
    maturity: str = "provisional"
    superseded_by: str | None = None


@dataclass
class Memory:
    """A typed memory: key + header + body text."""

    key: str
    header: MemHeader = field(default_factory=MemHeader)
    text: str = ""


def type_of(key: str) -> str:
    """Return the type prefix of a key, or 'fact' if untyped."""
    prefix = key.split(":", 1)[0] if ":" in key else ""
    return prefix if prefix in VALID_TYPES else "fact"


def make_key(mem_type: str, slug: str) -> str:
    """Build a `<type>:<slug>` key, validating the type."""
    if mem_type not in VALID_TYPES:
        raise ValueError(f"unknown memory type {mem_type!r}; expected one of {VALID_TYPES}")
    return f"{mem_type}:{slug}"


def _coerce(header: MemHeader) -> None:
    """Normalize header field types after parsing string values."""
    try:
        header.utility = float(header.utility)
    except (TypeError, ValueError):
        header.utility = 0.0
    try:
        header.access_count = int(header.access_count)
    except (TypeError, ValueError):
        header.access_count = 0
    if header.type not in VALID_TYPES:
        header.type = "fact"
    if header.superseded_by in ("", "null", "None"):
        header.superseded_by = None


def parse(body: str) -> tuple[MemHeader, str]:
    """Split a stored body into (header, text). Missing block ⇒ defaults."""
    stripped = body.lstrip()
    if not stripped.startswith(_HEADER_OPEN):
        return MemHeader(), body
    lines = stripped.splitlines()
    header = MemHeader()
    end = 0
    for i, line in enumerate(lines[1:], start=1):
        if line.strip() == _HEADER_CLOSE:
            end = i
            break
        if ":" in line:
            field_name, _, value = line.partition(":")
            field_name = field_name.strip()
            if field_name in MemHeader.__dataclass_fields__:
                setattr(header, field_name, value.strip())
    _coerce(header)
    text = "\n".join(lines[end + 1:]).lstrip("\n")
    return header, text


def render(header: MemHeader, text: str) -> str:
    """Render (header, text) back into a stored body with the fenced block."""
    sup = "null" if header.superseded_by is None else header.superseded_by
    block = [
        _HEADER_OPEN,
        f"type: {header.type}",
        f"source: {header.source}",
        f"utility: {header.utility}",
        f"access_count: {header.access_count}",
        f"created_at: {header.created_at}",
        f"last_accessed: {header.last_accessed}",
        f"maturity: {header.maturity}",
        f"superseded_by: {sup}",
        _HEADER_CLOSE,
    ]
    return "\n".join(block) + "\n" + text


def _age_days(ts: str, now: datetime) -> float:
    """Whole days between an ISO-8601 timestamp and ``now`` (0 if unparseable)."""
    if not ts:
        return 0.0
    try:
        when = datetime.fromisoformat(ts.replace("Z", "+00:00"))
    except ValueError:
        return 0.0
    if when.tzinfo is None:
        when = when.replace(tzinfo=timezone.utc)
    return max(0.0, (now - when).total_seconds() / 86400.0)


def _estimate_tokens(text: str) -> int:
    """Rough token estimate (~0.75 words/token) for budgeting recall."""
    return max(1, int(len(text.split()) / 0.75))


def score(mem: Memory, now: datetime) -> float:
    """Decay-weighted recall score per DATA-MODEL.md."""
    h = mem.header
    half_life = TYPE_HALF_LIFE.get(h.type, 90.0)
    ref = h.last_accessed or h.created_at
    recency = 0.5 ** (_age_days(ref, now) / half_life)
    weight = TYPE_WEIGHT.get(h.type, 1.0)
    return weight * recency * (1.0 + math.log1p(max(0, h.access_count))) * (0.5 + h.utility)


def rank(memories: list[Memory], now: datetime, max_tokens: int | None = None) -> list[Memory]:
    """Rank memories by decay score; truncate to a token budget if given."""
    live = [m for m in memories if m.header.superseded_by is None]
    ordered = sorted(live, key=lambda m: score(m, now), reverse=True)
    if max_tokens is None:
        return ordered
    out: list[Memory] = []
    spent = 0
    for m in ordered:
        cost = _estimate_tokens(m.text)
        if spent + cost > max_tokens and out:
            break
        out.append(m)
        spent += cost
    return out


class BdStore:
    """Injectable adapter over the bd memory CLI.

    ``runner`` is a callable taking an argv list and returning stdout text;
    the default shells out to ``bd``. Tests inject a fake runner (or use the
    in-memory ``FakeStore``) so they never touch the real database.
    """

    def __init__(self, runner: Callable[[list[str]], str] | None = None) -> None:
        self._run = runner or self._default_runner

    @staticmethod
    def _default_runner(argv: list[str]) -> str:
        proc = subprocess.run(argv, capture_output=True, text=True, timeout=120, check=False)
        return proc.stdout

    def remember(self, key: str, body: str) -> None:
        """Upsert a memory (bd updates in place by key)."""
        self._run(["bd", "remember", "--key", key, body])

    def forget(self, key: str) -> None:
        """Remove a memory by key."""
        self._run(["bd", "forget", key])

    def list_memories(self) -> list[Memory]:
        """List all memories, parsing `bd memories` '### key / body' blocks."""
        return parse_listing(self._run(["bd", "memories"]))


def parse_listing(text: str) -> list[Memory]:
    """Parse `bd memories` output into Memory objects (### key, then body)."""
    out: list[Memory] = []
    key: str | None = None
    buf: list[str] = []

    def flush() -> None:
        if key is not None:
            header, body_text = parse("\n".join(buf).strip())
            out.append(Memory(key=key, header=header, text=body_text))

    for line in text.splitlines():
        if line.startswith("### "):
            flush()
            key = line[4:].strip()
            buf = []
        elif key is not None:
            buf.append(line)
    flush()
    return out


class FakeStore(BdStore):
    """In-memory store for tests; records remember/forget calls."""

    def __init__(self) -> None:  # noqa: D107 - see class docstring
        super().__init__(runner=lambda argv: "")
        self.data: dict[str, str] = {}

    def remember(self, key: str, body: str) -> None:
        self.data[key] = body

    def forget(self, key: str) -> None:
        self.data.pop(key, None)

    def list_memories(self) -> list[Memory]:
        mems: list[Memory] = []
        for key, body in self.data.items():
            header, text = parse(body)
            mems.append(Memory(key=key, header=header, text=text))
        return mems
