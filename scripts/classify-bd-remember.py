#!/usr/bin/env python3
"""Classify bd remember memories into a lineage-preserving migration manifest."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any


BEAD_RE = re.compile(r"\b(?:ag|soc)-[a-z0-9]+(?:\.[a-z0-9]+)*\b", re.IGNORECASE)
STALE_RE = re.compile(
    r"\b("
    r"deprecated|obsolete|superseded|stale|retired|dead command|"
    r"no longer|do not use|wrong|false premise|invalidated"
    r")\b",
    re.IGNORECASE,
)
ONE_OFF_RE = re.compile(
    r"\b(this bead|current bead|this pr|current pr|one[- ]off|single[- ]use|"
    r"session[- ]local|release[- ]only|incident[- ]only)\b",
    re.IGNORECASE,
)


@dataclass(frozen=True)
class Memory:
    key: str
    body: str
    created_at: str
    updated_at: str
    raw: dict[str, Any]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Classify bd remember entries into bead, pull-learning, or discard "
            "without mutating the tracker or corpus."
        )
    )
    parser.add_argument(
        "--input",
        type=Path,
        help="JSON file from `bd memories --json`. If omitted, runs bd memories --json.",
    )
    parser.add_argument(
        "--output",
        type=Path,
        help="Write the manifest to this path instead of stdout.",
    )
    parser.add_argument(
        "--format",
        choices=("json", "markdown"),
        default="json",
        help="Manifest output format.",
    )
    parser.add_argument(
        "--expected-count",
        type=int,
        help="Fail if the input memory count differs from this value.",
    )
    parser.add_argument(
        "--generated-at",
        help="Deterministic ISO timestamp for tests. Defaults to current UTC.",
    )
    return parser.parse_args()


def load_payload(input_path: Path | None) -> tuple[Any, str]:
    if input_path is not None:
        return json.loads(input_path.read_text(encoding="utf-8")), str(input_path)

    proc = subprocess.run(
        ["bd", "memories", "--json"],
        check=False,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if proc.returncode != 0:
        sys.stderr.write(proc.stderr)
        raise SystemExit(proc.returncode)
    return json.loads(proc.stdout), "bd memories --json"


def as_memory_list(payload: Any) -> list[dict[str, Any]]:
    if isinstance(payload, list):
        return [item for item in payload if isinstance(item, dict)]
    if isinstance(payload, dict):
        for key in ("memories", "items", "result", "data"):
            value = payload.get(key)
            if isinstance(value, list):
                return [item for item in value if isinstance(item, dict)]
    raise SystemExit("classify-bd-remember: expected JSON list or object containing memories/items/result/data")


def first_string(record: dict[str, Any], *keys: str) -> str:
    for key in keys:
        value = record.get(key)
        if isinstance(value, str) and value.strip():
            return value.strip()
    return ""


def normalize_memory(record: dict[str, Any], index: int) -> Memory:
    key = first_string(record, "key", "id", "name", "slug")
    body = first_string(record, "content", "body", "rule", "text", "value", "memory")
    if not key:
        key = f"unkeyed-{index + 1:04d}"
    return Memory(
        key=key,
        body=body,
        created_at=first_string(record, "created_at", "createdAt", "created"),
        updated_at=first_string(record, "updated_at", "updatedAt", "updated"),
        raw=record,
    )


def slugify(value: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")
    return slug or "unkeyed-memory"


def classify(memory: Memory) -> tuple[str, str, float]:
    text = f"{memory.key}\n{memory.body}"
    if not memory.body:
        return "discard", "empty body cannot preserve an actionable rule", 0.95
    if STALE_RE.search(text):
        return "discard", "stale/deprecated marker requires discard unless manually rescued", 0.78
    bead_refs = sorted(set(BEAD_RE.findall(text)))
    if bead_refs:
        return "bead", "mentions work item(s): " + ", ".join(bead_refs), 0.82
    if ONE_OFF_RE.search(text):
        return "bead", "one-off/session-local marker makes this bead-scoped context", 0.7
    return (
        "pull-learning",
        "general rule remains potentially useful but must re-earn maturity through corpus gates",
        0.62,
    )


def build_item(memory: Memory, index: int) -> dict[str, Any]:
    disposition, rationale, confidence = classify(memory)
    item: dict[str, Any] = {
        "index": index,
        "key": memory.key,
        "disposition": disposition,
        "rationale": rationale,
        "confidence": confidence,
        "lineage": {
            "source": "bd remember",
            "key": memory.key,
            "body": memory.body,
            "created_at": memory.created_at,
            "updated_at": memory.updated_at,
            "raw": memory.raw,
        },
    }
    if disposition == "pull-learning":
        item["proposed_learning"] = {
            "id": slugify(memory.key),
            "reach": "pull",
            "maturity": "provisional",
            "source": "migrated-bd-remember",
            "source_key": memory.key,
            "rule": memory.body,
        }
    if disposition == "bead":
        item["bead_refs"] = sorted(set(BEAD_RE.findall(f"{memory.key}\n{memory.body}")))
    return item


def build_manifest(records: list[dict[str, Any]], source: str, generated_at: str) -> dict[str, Any]:
    items = [build_item(normalize_memory(record, index), index) for index, record in enumerate(records)]
    counts = {"bead": 0, "pull-learning": 0, "discard": 0}
    for item in items:
        counts[item["disposition"]] += 1
    return {
        "schema_version": "bd-remember-migration-manifest.v1",
        "source": source,
        "generated_at": generated_at,
        "total": len(items),
        "classified": len(items),
        "unclassified": 0,
        "counts": counts,
        "items": items,
    }


def render_markdown(manifest: dict[str, Any]) -> str:
    lines = [
        "# bd remember migration manifest",
        "",
        f"- Schema: `{manifest['schema_version']}`",
        f"- Source: `{manifest['source']}`",
        f"- Generated at: `{manifest['generated_at']}`",
        f"- Total: **{manifest['total']}**",
        f"- Classified: **{manifest['classified']}**",
        f"- Unclassified: **{manifest['unclassified']}**",
        "",
        "## Counts",
        "",
    ]
    for disposition in ("bead", "pull-learning", "discard"):
        lines.append(f"- `{disposition}`: **{manifest['counts'][disposition]}**")
    lines.extend(["", "## Items", ""])
    for item in manifest["items"]:
        lines.append(
            f"- `{item['key']}` -> `{item['disposition']}` "
            f"({item['confidence']:.2f}): {item['rationale']}"
        )
    lines.append("")
    return "\n".join(lines)


def write_output(text: str, output_path: Path | None) -> None:
    if output_path is None:
        sys.stdout.write(text)
        if not text.endswith("\n"):
            sys.stdout.write("\n")
        return
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(text, encoding="utf-8")


def main() -> int:
    args = parse_args()
    payload, source = load_payload(args.input)
    records = as_memory_list(payload)
    if args.expected_count is not None and len(records) != args.expected_count:
        sys.stderr.write(
            f"classify-bd-remember: expected {args.expected_count} memories, got {len(records)}\n"
        )
        return 1

    generated_at = args.generated_at or dt.datetime.now(dt.timezone.utc).isoformat()
    manifest = build_manifest(records, source, generated_at)
    if args.format == "markdown":
        write_output(render_markdown(manifest), args.output)
    else:
        write_output(json.dumps(manifest, indent=2, sort_keys=True), args.output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
