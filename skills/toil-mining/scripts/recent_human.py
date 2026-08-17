#!/usr/bin/env python3
"""Extract high-confidence human messages from caller-supplied Codex JSONL."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Iterable


REQUEST_MARKER = "# My request for Codex:"
MAX_FILES_LIMIT = 256
MAX_FILE_BYTES_LIMIT = 16 * 1024 * 1024
MAX_LINE_BYTES_LIMIT = 1024 * 1024
MAX_MESSAGES_LIMIT = 10_000
MAX_TEXT_CHARS_LIMIT = 4096

SENSITIVE_PATTERNS: tuple[tuple[re.Pattern[str], str], ...] = (
    (
        re.compile(
            r"(?i)\b(api[_-]?key|access[_-]?token|auth(?:orization)?|password|passwd|secret)"
            r"\s*[:=]\s*([^\s,;]+)"
        ),
        r"\1=[REDACTED]",
    ),
    (re.compile(r"(?i)\bBearer\s+[A-Za-z0-9._~+\-/]+=*"), "Bearer [REDACTED]"),
    (re.compile(r"-----BEGIN [^-\n]+-----[\s\S]*?-----END [^-\n]+-----"), "[REDACTED-PEM]"),
    (re.compile(r"(?<![\w@])(?:/[^\s/:]+){2,}"), "[REDACTED-PATH]"),
    (re.compile(r"(?<!\w)~/(?:[^\s]+)"), "[REDACTED-PATH]"),
    (re.compile(r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b"), "[REDACTED-EMAIL]"),
    (re.compile(r"\b(?:[A-Fa-f0-9]{32,}|[A-Za-z0-9+/]{40,}={0,2})\b"), "[REDACTED-TOKEN]"),
)

# These are deliberately narrow. The client_id check below is the primary
# machine-injection boundary; these patterns catch known generated envelopes
# even if a future producer happens to attach a client id.
MACHINE_ECHO_PATTERNS: tuple[tuple[str, re.Pattern[str]], ...] = (
    (
        "internal_context",
        re.compile(
            r"^\s*<(?:codex_internal_context|environment_context|permissions|"
            r"skills_instructions|apps_instructions|plugins_instructions)\b",
            re.IGNORECASE,
        ),
    ),
    (
        "fresh_context_refuter",
        re.compile(
            r"^\s*You are (?:a|the) fresh-context, cross-family reviewer\b",
            re.IGNORECASE,
        ),
    ),
    (
        "agent_message_envelope",
        re.compile(
            r"^\s*Message Type:\s*(?:NEW_TASK|MESSAGE|FINAL_ANSWER)\b",
            re.IGNORECASE,
        ),
    ),
)


@dataclass(frozen=True)
class Candidate:
    source_id: str
    line: int
    timestamp: datetime
    timestamp_text: str
    redacted_text: str
    text_digest: str
    client_id: str


def parse_instant(value: str, label: str) -> datetime:
    candidate = value.strip()
    if candidate.endswith(("Z", "z")):
        candidate = f"{candidate[:-1]}+00:00"
    try:
        instant = datetime.fromisoformat(candidate)
    except ValueError as exc:
        raise argparse.ArgumentTypeError(f"{label} is not ISO-8601: {value!r}") from exc
    if instant.tzinfo is None:
        raise argparse.ArgumentTypeError(f"{label} must include a timezone: {value!r}")
    return instant.astimezone(timezone.utc)


def canonical_instant(instant: datetime) -> str:
    return (
        instant.astimezone(timezone.utc)
        .isoformat(timespec="milliseconds")
        .replace("+00:00", "Z")
    )


def normalize_request(message: str) -> str:
    """Remove Codex attachment/IDE headers while retaining the explicit request."""
    normalized = message.replace("\r\n", "\n").replace("\r", "\n")
    if REQUEST_MARKER in normalized:
        normalized = normalized.split(REQUEST_MARKER, 1)[1]
    lines = [line.rstrip() for line in normalized.strip().split("\n")]
    return "\n".join(lines).strip()


def redact_request(message: str, max_chars: int) -> str:
    """Return a bounded, redacted excerpt; raw human text is never emitted."""
    redacted = message
    for pattern, replacement in SENSITIVE_PATTERNS:
        redacted = pattern.sub(replacement, redacted)
    if len(redacted) > max_chars:
        redacted = redacted[:max_chars].rstrip() + "…[TRUNCATED]"
    return redacted


def machine_echo_reason(text: str) -> str | None:
    for name, pattern in MACHINE_ECHO_PATTERNS:
        if pattern.search(text):
            return name
    return None


def is_user_message(record: Any) -> bool:
    return (
        isinstance(record, dict)
        and record.get("type") == "event_msg"
        and isinstance(record.get("payload"), dict)
        and record["payload"].get("type") == "user_message"
    )


def iter_records(path: Path, max_line_bytes: int) -> Iterable[tuple[int, bytes]]:
    with path.open("rb") as handle:
        for line_number, raw_line in enumerate(handle, start=1):
            if len(raw_line) > max_line_bytes:
                raise ValueError(f"input line exceeds max_line_bytes at source line {line_number}")
            yield line_number, raw_line


def extract(
    sources: list[tuple[str, str, Path]],
    since: datetime,
    until: datetime,
    *,
    max_line_bytes: int,
    max_messages: int,
    max_text_chars: int,
) -> dict[str, Any]:
    exclusions: dict[str, Any] = {
        "invalid_json": 0,
        "non_user_message": 0,
        "invalid_timestamp": 0,
        "outside_window": 0,
        "missing_client_id": 0,
        "machine_echo": 0,
        "machine_echo_by_pattern": {name: 0 for name, _ in MACHINE_ECHO_PATTERNS},
        "empty_after_normalization": 0,
        "duplicate_client_id": 0,
        "duplicate_client_id_text_conflict": 0,
    }
    input_lines = 0
    parsed_records = 0
    candidate_user_messages = 0
    candidates: list[Candidate] = []

    for source_id, _relative_path, path in sources:
        for line_number, raw_line in iter_records(path, max_line_bytes):
            input_lines += 1
            try:
                record = json.loads(raw_line.decode("utf-8"))
            except (json.JSONDecodeError, UnicodeDecodeError):
                exclusions["invalid_json"] += 1
                continue
            parsed_records += 1
            if not is_user_message(record):
                exclusions["non_user_message"] += 1
                continue

            candidate_user_messages += 1
            timestamp_value = record.get("timestamp")
            if not isinstance(timestamp_value, str):
                exclusions["invalid_timestamp"] += 1
                continue
            try:
                timestamp = parse_instant(timestamp_value, "record timestamp")
            except argparse.ArgumentTypeError:
                exclusions["invalid_timestamp"] += 1
                continue
            if not since <= timestamp < until:
                exclusions["outside_window"] += 1
                continue

            payload = record["payload"]
            message = payload.get("message")
            if not isinstance(message, str):
                exclusions["empty_after_normalization"] += 1
                continue
            text = normalize_request(message)
            if not text:
                exclusions["empty_after_normalization"] += 1
                continue
            echo_reason = machine_echo_reason(text)
            if echo_reason is not None:
                exclusions["machine_echo"] += 1
                exclusions["machine_echo_by_pattern"][echo_reason] += 1
                continue

            client_id = payload.get("client_id")
            if not isinstance(client_id, str) or not client_id.strip():
                exclusions["missing_client_id"] += 1
                continue

            candidates.append(
                Candidate(
                    source_id=source_id,
                    line=line_number,
                    timestamp=timestamp,
                    timestamp_text=canonical_instant(timestamp),
                    redacted_text=redact_request(text, max_text_chars),
                    text_digest=hashlib.sha256(text.encode("utf-8")).hexdigest(),
                    client_id=client_id.strip(),
                )
            )
            if len(candidates) > max_messages:
                raise ValueError("candidate messages exceed max_messages")

    # Sorting before selection makes output independent of caller path order and
    # chooses the earliest source occurrence when restored/forked sessions repeat
    # the same UI client event.
    candidates.sort(
        key=lambda item: (item.timestamp, item.source_id, item.line, item.client_id)
    )
    retained: list[Candidate] = []
    by_client_id: dict[str, Candidate] = {}
    for candidate in candidates:
        previous = by_client_id.get(candidate.client_id)
        if previous is not None:
            exclusions["duplicate_client_id"] += 1
            if previous.text_digest != candidate.text_digest:
                exclusions["duplicate_client_id_text_conflict"] += 1
            continue
        by_client_id[candidate.client_id] = candidate
        retained.append(candidate)

    return {
        "schema_version": 1,
        "window": {
            "since_inclusive": canonical_instant(since),
            "until_exclusive": canonical_instant(until),
        },
        "inputs": [
            {"source_id": source_id, "relative_path": relative_path}
            for source_id, relative_path, _path in sources
        ],
        "counts": {
            "input_files": len(sources),
            "input_lines": input_lines,
            "parsed_records": parsed_records,
            "candidate_user_messages": candidate_user_messages,
            "emitted_messages": len(retained),
            "excluded": exclusions,
        },
        "messages": [
            {
                "source_id": item.source_id,
                "line": item.line,
                "timestamp": item.timestamp_text,
                "redacted_text": item.redacted_text,
                "text_sha256": item.text_digest,
            }
            for item in retained
        ],
        "checked": [
            "Only regular, non-symlink JSONL files beneath the explicit input root were read.",
            "The time window was applied since-inclusive and until-exclusive in UTC.",
            "Only event_msg/user_message records with a nonempty client_id were retained.",
            "Codex attachment wrappers were reduced to the text after '# My request for Codex:'.",
            "Known generated envelopes were excluded and restored/forked copies were deduplicated by client_id.",
            "Emitted text was bounded and redacted; raw human text and absolute paths were not emitted.",
        ],
        "not_checked": [
            "Attachment file contents and any session paths not explicitly supplied were not read.",
            "A client_id is strong UI-origin evidence, not proof of a particular human author.",
            "Messages without client_id were not classified as human, including any directly typed by a client that omits that field.",
            "No semantic clustering, pain scoring, recommendation, tracker mutation, or automation scheduling was performed.",
        ],
    }


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description=(
            "Extract high-confidence recent human messages from caller-supplied "
            "Codex JSONL sessions. Writes one JSON report to stdout."
        )
    )
    parser.add_argument("--since", required=True, help="inclusive ISO-8601 timestamp")
    parser.add_argument("--until", required=True, help="exclusive ISO-8601 timestamp")
    parser.add_argument("--input-root", required=True, help="absolute root containing every session")
    parser.add_argument("--max-files", required=True, type=int)
    parser.add_argument("--max-file-bytes", required=True, type=int)
    parser.add_argument("--max-line-bytes", required=True, type=int)
    parser.add_argument("--max-messages", required=True, type=int)
    parser.add_argument("--max-text-chars", required=True, type=int)
    parser.add_argument(
        "sessions", nargs="+", help="explicit input-root-relative Codex JSONL paths"
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        since = parse_instant(args.since, "--since")
        until = parse_instant(args.until, "--until")
    except argparse.ArgumentTypeError as exc:
        parser.error(str(exc))
    if since >= until:
        parser.error("--since must be earlier than --until")

    bounds = (
        ("--max-files", args.max_files, MAX_FILES_LIMIT),
        ("--max-file-bytes", args.max_file_bytes, MAX_FILE_BYTES_LIMIT),
        ("--max-line-bytes", args.max_line_bytes, MAX_LINE_BYTES_LIMIT),
        ("--max-messages", args.max_messages, MAX_MESSAGES_LIMIT),
        ("--max-text-chars", args.max_text_chars, MAX_TEXT_CHARS_LIMIT),
    )
    for label, value, ceiling in bounds:
        if not 0 < value <= ceiling:
            parser.error(f"{label} must be in [1, {ceiling}]")

    root_arg = Path(args.input_root)
    if not root_arg.is_absolute() or root_arg.is_symlink():
        parser.error("--input-root must be an absolute non-symlink directory")
    try:
        input_root = root_arg.resolve(strict=True)
    except OSError as exc:
        parser.error(f"--input-root cannot be resolved: {exc}")
    if not input_root.is_dir():
        parser.error("--input-root must be a directory")

    unique_paths: dict[str, tuple[str, str, Path]] = {}
    for supplied in args.sessions:
        relative = Path(supplied)
        if relative.is_absolute() or ".." in relative.parts or relative.as_posix() != supplied:
            parser.error(f"session path must be canonical and input-root-relative: {supplied}")
        path = input_root / relative
        current = path
        while current != input_root:
            if current.is_symlink():
                parser.error(f"session path must not contain symlinks: {supplied}")
            current = current.parent
        try:
            resolved = path.resolve(strict=True)
            resolved.relative_to(input_root)
        except (OSError, ValueError):
            parser.error(f"session path escapes or does not exist: {supplied}")
        if not resolved.is_file() or not os.path.isfile(resolved):
            parser.error(f"session path is not a regular file: {supplied}")
        if resolved.stat().st_size > args.max_file_bytes:
            parser.error(f"session exceeds --max-file-bytes: {supplied}")
        source_id = "source-" + hashlib.sha256(supplied.encode("utf-8")).hexdigest()[:16]
        unique_paths[supplied] = (source_id, supplied, resolved)
    if len(unique_paths) > args.max_files:
        parser.error("session count exceeds --max-files")
    sources = [unique_paths[key] for key in sorted(unique_paths)]

    try:
        report = extract(
            sources,
            since,
            until,
            max_line_bytes=args.max_line_bytes,
            max_messages=args.max_messages,
            max_text_chars=args.max_text_chars,
        )
    except ValueError as exc:
        parser.error(str(exc))
    json.dump(report, sys.stdout, indent=2, ensure_ascii=False)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
