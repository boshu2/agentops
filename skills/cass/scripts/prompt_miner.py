#!/usr/bin/env python3
"""Bounded prompt clustering over explicit, root-confined session files."""

from __future__ import annotations

import argparse
from collections import defaultdict
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
from typing import Any


MAX_FILES = 256
MAX_FILE_BYTES = 16 * 1024 * 1024
MAX_LINE_BYTES = 1024 * 1024
MAX_PROMPTS = 100_000
MAX_TOP = 200
MAX_EXCERPT = 512
SECRET_PATTERNS = (
    (re.compile(r"(?i)\b(api[_-]?key|token|password|secret|authorization)\s*[:=]\s*\S+"), r"\1=[REDACTED]"),
    (re.compile(r"(?i)\bBearer\s+\S+"), "Bearer [REDACTED]"),
    (re.compile(r"(?<![\w@])(?:/[^\s/:]+){2,}"), "[REDACTED-PATH]"),
    (re.compile(r"\b[A-Fa-f0-9]{32,}\b"), "[REDACTED-TOKEN]"),
)


def normalize(value: str) -> str:
    return re.sub(r"\s+", " ", value.strip())


def redact(value: str, limit: int) -> str:
    result = value
    for pattern, replacement in SECRET_PATTERNS:
        result = pattern.sub(replacement, result)
    if len(result) > limit:
        result = result[:limit].rstrip() + "…[TRUNCATED]"
    return result


def parse_iso(value: Any) -> datetime:
    if not isinstance(value, str) or not value:
        return datetime(1970, 1, 1, tzinfo=timezone.utc)
    candidate = value[:-1] + "+00:00" if value.endswith("Z") else value
    try:
        parsed = datetime.fromisoformat(candidate)
    except ValueError:
        return datetime(1970, 1, 1, tzinfo=timezone.utc)
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return parsed.astimezone(timezone.utc)


def extract_text(content: Any) -> str:
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        return " ".join(extract_text(item) for item in content if isinstance(item, (str, dict)))
    if isinstance(content, dict):
        return extract_text(content.get("text", content.get("content", "")))
    return ""


def prompt_from_record(record: Any) -> tuple[str, datetime, str] | None:
    if not isinstance(record, dict):
        return None
    if record.get("type") == "user" and isinstance(record.get("message"), dict):
        text = extract_text(record["message"].get("content", ""))
        return (text, parse_iso(record.get("timestamp")), "claude_code") if text.strip() else None
    if record.get("role") == "user" and "content" in record:
        text = extract_text(record["content"])
        return (text, parse_iso(record.get("timestamp") or record.get("created_at")), "codex") if text.strip() else None
    return None


def resolve_sources(root: Path, supplied: list[str], max_files: int, max_file_bytes: int) -> list[tuple[str, Path]]:
    if len(supplied) > max_files:
        raise ValueError("session count exceeds --max-files")
    sources: dict[str, Path] = {}
    for raw in supplied:
        relative = Path(raw)
        if relative.is_absolute() or ".." in relative.parts or relative.as_posix() != raw:
            raise ValueError(f"session path must be canonical and input-root-relative: {raw}")
        candidate = root / relative
        current = candidate
        while current != root:
            if current.is_symlink():
                raise ValueError(f"session path contains a symlink: {raw}")
            current = current.parent
        resolved = candidate.resolve(strict=True)
        resolved.relative_to(root)
        mode = resolved.stat().st_mode
        if not stat.S_ISREG(mode):
            raise ValueError(f"session path is not a regular file: {raw}")
        if resolved.stat().st_size > max_file_bytes:
            raise ValueError(f"session exceeds --max-file-bytes: {raw}")
        sources[raw] = resolved
    return [(name, sources[name]) for name in sorted(sources)]


def mine(
    sources: list[tuple[str, Path]],
    *,
    max_line_bytes: int,
    max_prompts: int,
    agent_filter: str | None,
) -> list[tuple[datetime, str, str, str]]:
    items: list[tuple[datetime, str, str, str]] = []
    for source, path in sources:
        with path.open("rb") as handle:
            for line_number, raw in enumerate(handle, 1):
                if len(raw) > max_line_bytes:
                    raise ValueError(f"input line exceeds --max-line-bytes at {source}:{line_number}")
                try:
                    record = json.loads(raw.decode("utf-8"))
                except (UnicodeDecodeError, json.JSONDecodeError):
                    continue
                extracted = prompt_from_record(record)
                if extracted is None:
                    continue
                text, timestamp, agent = extracted
                if agent_filter and agent != agent_filter:
                    continue
                items.append((timestamp, source, text.strip(), agent))
                if len(items) > max_prompts:
                    raise ValueError("prompt count exceeds --max-prompts")
    items.sort(key=lambda item: (item[0], item[1]))
    return items


def repeated(
    items: list[tuple[datetime, str, str, str]],
    *,
    top: int,
    minimum: int,
    excerpt_chars: int,
) -> list[dict[str, Any]]:
    groups: dict[str, dict[str, Any]] = defaultdict(
        lambda: {"count": 0, "agents": set(), "first": None, "last": None, "sources": [], "text": ""}
    )
    for timestamp, source, text, agent in items:
        key = normalize(text)
        data = groups[hashlib.sha256(key.encode()).hexdigest()]
        data["text"] = data["text"] or text
        data["count"] += 1
        data["agents"].add(agent)
        data["first"] = min(data["first"] or timestamp, timestamp)
        data["last"] = max(data["last"] or timestamp, timestamp)
        if source not in data["sources"] and len(data["sources"]) < 3:
            data["sources"].append(source)
    result = []
    for digest, data in groups.items():
        if data["count"] < minimum:
            continue
        result.append({
            "prompt_sha256": digest,
            "redacted_excerpt": redact(data["text"], excerpt_chars),
            "count": data["count"],
            "agents": sorted(data["agents"]),
            "first_seen": data["first"].isoformat(),
            "last_seen": data["last"].isoformat(),
            "example_sources": data["sources"],
            "is_ritual": data["count"] >= 10,
        })
    result.sort(key=lambda item: (-item["count"], item["prompt_sha256"]))
    return result[:top]


def bounded(parser: argparse.ArgumentParser, label: str, value: int, ceiling: int) -> None:
    if not 0 < value <= ceiling:
        parser.error(f"{label} must be in [1, {ceiling}]")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Mine repeated prompts from explicit bounded session files")
    parser.add_argument("--input-root", required=True)
    parser.add_argument("--max-files", required=True, type=int)
    parser.add_argument("--max-file-bytes", required=True, type=int)
    parser.add_argument("--max-line-bytes", required=True, type=int)
    parser.add_argument("--max-prompts", required=True, type=int)
    parser.add_argument("--max-excerpt-chars", required=True, type=int)
    parser.add_argument("--top", type=int, default=30)
    parser.add_argument("--min-count", type=int, default=2)
    parser.add_argument("--agent", choices=["claude_code", "codex"])
    parser.add_argument("--rituals-only", action="store_true")
    parser.add_argument("--json", action="store_true")
    parser.add_argument("sessions", nargs="+")
    args = parser.parse_args(argv)

    for label, value, ceiling in (
        ("--max-files", args.max_files, MAX_FILES),
        ("--max-file-bytes", args.max_file_bytes, MAX_FILE_BYTES),
        ("--max-line-bytes", args.max_line_bytes, MAX_LINE_BYTES),
        ("--max-prompts", args.max_prompts, MAX_PROMPTS),
        ("--max-excerpt-chars", args.max_excerpt_chars, MAX_EXCERPT),
        ("--top", args.top, MAX_TOP),
        ("--min-count", args.min_count, MAX_PROMPTS),
    ):
        bounded(parser, label, value, ceiling)

    root_arg = Path(args.input_root)
    if not root_arg.is_absolute() or root_arg.is_symlink():
        parser.error("--input-root must be an absolute non-symlink directory")
    try:
        root = root_arg.resolve(strict=True)
        if not root.is_dir():
            raise ValueError("not a directory")
        sources = resolve_sources(root, args.sessions, args.max_files, args.max_file_bytes)
        items = mine(
            sources,
            max_line_bytes=args.max_line_bytes,
            max_prompts=args.max_prompts,
            agent_filter=args.agent,
        )
        minimum = max(args.min_count, 10) if args.rituals_only else args.min_count
        results = repeated(items, top=args.top, minimum=minimum, excerpt_chars=args.max_excerpt_chars)
    except (OSError, ValueError) as exc:
        parser.error(str(exc))

    report = {
        "input_sources": [name for name, _path in sources],
        "total_prompts": len(items),
        "repeated_prompts": results,
        "bounds": {
            "max_files": args.max_files,
            "max_file_bytes": args.max_file_bytes,
            "max_line_bytes": args.max_line_bytes,
            "max_prompts": args.max_prompts,
            "max_excerpt_chars": args.max_excerpt_chars,
        },
    }
    if args.json:
        json.dump(report, sys.stdout, indent=2, ensure_ascii=False)
        sys.stdout.write("\n")
    else:
        print(f"Found {len(items)} user prompts in {len(sources)} explicit sources")
        for item in results:
            print(f"{item['count']:3d}x ({', '.join(item['agents'])}): {item['redacted_excerpt']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
