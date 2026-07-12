#!/usr/bin/env python3
"""Fail-closed validator for an RPI execution packet and receipt extension."""

from __future__ import annotations

import json
import sys
from pathlib import Path

from jsonschema import Draft202012Validator, FormatChecker


def fail(message: str) -> None:
    raise ValueError(message)


def require_nonempty(value: object, label: str) -> str:
    if not isinstance(value, str) or not value.strip():
        fail(f"{label} must be a nonempty string")
    return value


def find_repo_root() -> Path:
    """Resolve by the declared schema surface, never by directory depth."""
    for candidate in Path(__file__).resolve().parents:
        if (candidate / "schemas" / "execution-packet.schema.json").is_file():
            return candidate
    fail("cannot locate schemas/execution-packet.schema.json from validator path")


def resolve_artifact(repo_root: Path, value: object, label: str) -> Path:
    artifact = require_nonempty(value, label)
    relative = Path(artifact)
    if relative.is_absolute():
        fail(f"{label} must be repository-relative")
    resolved = (repo_root / relative).resolve()
    try:
        resolved.relative_to(repo_root.resolve())
    except ValueError:
        fail(f"{label} escapes the repository")
    if not resolved.is_file() or resolved.stat().st_size == 0:
        fail(f"{label} must name an existing nonempty file")
    return resolved


def validate_receipts(packet: dict[str, object], repo_root: Path) -> None:
    loaded = packet.get("skills_loaded")
    if not isinstance(loaded, list) or not loaded:
        fail("skills_loaded must be a nonempty array")

    loaded_names: set[str] = set()
    for index, entry in enumerate(loaded):
        if not isinstance(entry, dict):
            fail(f"skills_loaded[{index}] must be an object")
        loaded_names.add(require_nonempty(entry.get("name"), f"skills_loaded[{index}].name"))
        require_nonempty(entry.get("reason"), f"skills_loaded[{index}].reason")
    if "rpi" not in loaded_names:
        fail("skills_loaded must include rpi")

    receipts = packet.get("phase_receipts")
    if not isinstance(receipts, list) or not receipts:
        fail("phase_receipts must be a nonempty array")
    allowed = {"DONE", "PARTIAL", "BLOCKED", "PASS", "WARN", "FAIL", "REFUTED"}
    observed_phases: set[str] = set()
    final_status_by_phase: dict[str, str] = {}
    for index, receipt in enumerate(receipts):
        if not isinstance(receipt, dict):
            fail(f"phase_receipts[{index}] must be an object")
        phase = require_nonempty(receipt.get("phase"), f"phase_receipts[{index}].phase")
        observed_phases.add(phase)
        skill = require_nonempty(receipt.get("skill"), f"phase_receipts[{index}].skill")
        if skill not in loaded_names:
            fail(f"phase_receipts[{index}].skill is absent from skills_loaded")
        status = require_nonempty(receipt.get("status"), f"phase_receipts[{index}].status")
        if status not in allowed:
            fail(f"phase_receipts[{index}].status is not a recognized verdict")
        final_status_by_phase[phase] = status
        resolve_artifact(repo_root, receipt.get("artifact"), f"phase_receipts[{index}].artifact")

    required_phases = {"discovery", "implementation", "validation"}
    missing = sorted(required_phases - observed_phases)
    if missing:
        fail(f"phase_receipts missing required lifecycle phases: {', '.join(missing)}")

    successful_statuses = {
        "discovery": {"DONE"},
        "implementation": {"DONE"},
        "validation": {"PASS"},
    }
    for phase, successful in successful_statuses.items():
        status = final_status_by_phase[phase]
        if status not in successful:
            fail(f"final {phase} receipt status {status} is not successful")


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: validate-execution-packet.py <execution-packet.json>", file=sys.stderr)
        return 2

    packet_path = Path(sys.argv[1])
    repo_root = find_repo_root()
    schema_path = repo_root / "schemas" / "execution-packet.schema.json"
    try:
        packet = json.loads(packet_path.read_text(encoding="utf-8"))
        schema = json.loads(schema_path.read_text(encoding="utf-8"))
        if not isinstance(packet, dict):
            fail("execution packet must be a JSON object")
        validate_receipts(packet, repo_root)
        core_packet = {
            key: value
            for key, value in packet.items()
            if key not in {"skills_loaded", "phase_receipts"}
        }
        Draft202012Validator(schema, format_checker=FormatChecker()).validate(core_packet)
    except (OSError, json.JSONDecodeError, ValueError) as exc:
        print(f"invalid execution packet: {exc}", file=sys.stderr)
        return 1
    except Exception as exc:  # jsonschema emits several concrete validation errors
        print(f"invalid execution packet: {exc}", file=sys.stderr)
        return 1

    print(f"valid execution packet: {packet_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
