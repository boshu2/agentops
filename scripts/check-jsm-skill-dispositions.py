#!/usr/bin/env python3
"""Validate the frozen JSM package inventory against one disposition per name.

The release path reads checked-in files only. ``--refresh`` is an explicit,
advisory maintenance action: it fetches remote names into a candidate, proves
the audit has exactly one decision for every name, and only then replaces the
frozen snapshot atomically.
"""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
from typing import Iterable


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_SNAPSHOT = ROOT / "docs/audits/manifests/external-skill-jsm-frozen.json"
DEFAULT_AUDIT = ROOT / "docs/audits/external-skill-corpus-2026-07-09.md"
DEFAULT_LEGACY_MANIFEST = ROOT / "docs/audits/manifests/external-skill-official-2026-07-09.txt"
REMOTE_ARGS = ["list", "--remote", "--jeffreys", "--json"]
ROW_RE = re.compile(r"^\|\s*\d+\s*\|\s*`([^`]+)`\s*\|\s*(.*?)\s*\|\s*$")


class ValidationError(RuntimeError):
    pass


def normalized_names(values: Iterable[object], *, source: str) -> list[str]:
    names: list[str] = []
    for value in values:
        if not isinstance(value, str) or not value.strip():
            raise ValidationError(f"{source}: every package name must be a nonempty string")
        names.append(value.strip())
    duplicates = sorted({name for name in names if names.count(name) > 1})
    if duplicates:
        raise ValidationError(f"{source}: duplicate package names: {duplicates}")
    return sorted(names)


def load_snapshot(path: Path) -> list[str]:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise ValidationError(f"frozen snapshot unreadable: {path}: {exc}") from exc
    if not isinstance(payload, dict) or payload.get("schema_version") != 1:
        raise ValidationError(f"frozen snapshot must be a schema_version 1 object: {path}")
    names = normalized_names(payload.get("names", []), source="frozen snapshot")
    if payload.get("names") != names:
        raise ValidationError("frozen snapshot names must be sorted")
    return names


def parse_legacy_manifest(path: Path) -> list[str]:
    return normalized_names(
        [
            line.strip()
            for line in path.read_text(encoding="utf-8").splitlines()
            if line.strip() and not line.lstrip().startswith("#")
        ],
        source="legacy manifest",
    )


def parse_official_dispositions(path: Path) -> dict[str, list[str]]:
    text = path.read_text(encoding="utf-8")
    start = text.find("## Official 100")
    end = text.find("## Local/community", start + 1) if start >= 0 else -1
    region = text[start:end] if start >= 0 and end > start else text
    decisions: dict[str, list[str]] = {}
    for line in region.splitlines():
        match = ROW_RE.match(line)
        if not match:
            continue
        name, decision = match.groups()
        if not decision.strip():
            raise ValidationError(f"empty disposition for {name}")
        decisions.setdefault(name, []).append(decision.strip())
    return decisions


def validate_one_to_one(names: list[str], audit_path: Path) -> None:
    decisions = parse_official_dispositions(audit_path)
    duplicate = sorted(name for name, rows in decisions.items() if len(rows) != 1)
    expected = set(names)
    actual = set(decisions)
    missing = sorted(expected - actual)
    extra = sorted(actual - expected)
    problems: list[str] = []
    if duplicate:
        problems.append(f"duplicate decisions={duplicate}")
    if missing:
        problems.append(f"missing decisions={missing}")
    if extra:
        problems.append(f"extra decisions={extra}")
    if problems:
        raise ValidationError("; ".join(problems))


def names_sha256(names: list[str]) -> str:
    return hashlib.sha256(("\n".join(names) + "\n").encode()).hexdigest()


def write_candidate(path: Path, names: list[str]) -> Path:
    candidate = Path(f"{path}.candidate")
    payload = {
        "schema_version": 1,
        "source_command": "jsm list --remote --jeffreys --json",
        "refreshed_at": dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z"),
        "sorted_name_sha256": names_sha256(names),
        "names": names,
    }
    candidate.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return candidate


def check(snapshot: Path, audit: Path, explicit_manifest: Path | None) -> list[str]:
    names = load_snapshot(snapshot)
    if explicit_manifest is not None:
        legacy = parse_legacy_manifest(explicit_manifest)
        if legacy != names:
            raise ValidationError("frozen snapshot and checked manifest names differ")
    validate_one_to_one(names, audit)
    print(f"PASS: {len(names)} frozen JSM packages; {len(names)} one-to-one dispositions")
    return names


def refresh(snapshot: Path, audit: Path, jsm_bin: str) -> None:
    candidate = Path(f"{snapshot}.candidate")
    try:
        proc = subprocess.run(
            [jsm_bin, *REMOTE_ARGS], text=True, capture_output=True, check=False
        )
        if proc.returncode != 0:
            detail = proc.stderr.strip() or f"exit {proc.returncode}"
            raise ValidationError(f"remote command failed: {detail}")
        try:
            payload = json.loads(proc.stdout)
            skills = payload["skills"]
            if not isinstance(skills, list):
                raise TypeError(".skills is not an array")
            names = normalized_names((item.get("name") for item in skills), source="remote JSM")
        except (json.JSONDecodeError, KeyError, TypeError, AttributeError) as exc:
            raise ValidationError(f"remote JSM output has no valid .skills[].name inventory: {exc}") from exc
        validate_one_to_one(names, audit)
        snapshot.parent.mkdir(parents=True, exist_ok=True)
        candidate = write_candidate(snapshot, names)
        load_snapshot(candidate)
        os.replace(candidate, snapshot)
        print(f"REFRESHED: {len(names)} frozen JSM packages in {snapshot}")
    except (OSError, ValidationError) as exc:
        candidate.unlink(missing_ok=True)
        command = f"{jsm_bin} {' '.join(REMOTE_ARGS)}"
        print(
            f"refresh failed; prior frozen snapshot preserved: {exc}\n"
            f"retry explicitly with: python3 scripts/check-jsm-skill-dispositions.py --refresh\n"
            f"remote command: {command}",
            file=sys.stderr,
        )
        raise SystemExit(1) from exc


def main() -> int:
    parser = argparse.ArgumentParser()
    action = parser.add_mutually_exclusive_group(required=True)
    action.add_argument("--check", action="store_true")
    action.add_argument("--refresh", action="store_true")
    args = parser.parse_args()

    snapshot = Path(os.environ.get("JSM_DISPOSITIONS_SNAPSHOT", DEFAULT_SNAPSHOT))
    audit = Path(os.environ.get("JSM_DISPOSITIONS_AUDIT", DEFAULT_AUDIT))
    explicit_manifest = os.environ.get("JSM_DISPOSITIONS_MANIFEST")
    manifest = Path(explicit_manifest) if explicit_manifest else None

    try:
        if args.refresh:
            refresh(snapshot, audit, os.environ.get("JSM_BIN", "jsm"))
        else:
            check(snapshot, audit, manifest)
    except (OSError, ValidationError) as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
