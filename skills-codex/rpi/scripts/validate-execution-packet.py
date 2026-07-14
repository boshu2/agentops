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


def validate_versioned_alias(
    container: dict[str, object],
    version: int,
    legacy_key: str,
    canonical_key: str,
    prefix: str = "",
    allow_equal_dual: bool = True,
) -> None:
    legacy_present = legacy_key in container
    canonical_present = canonical_key in container
    if legacy_present and canonical_present:
        if not allow_equal_dual or container[legacy_key] != container[canonical_key]:
            fail(
                "conflicting execution packet fields "
                f"{prefix}{legacy_key} and {prefix}{canonical_key}"
            )
        return
    if version < 3 and canonical_present:
        fail(f"schema_version {version} does not own field {prefix}{canonical_key}")
    if version == 3 and legacy_present:
        fail(f"schema_version {version} does not own field {prefix}{legacy_key}")


def validate_mortem_aliases(packet: dict[str, object]) -> None:
    version = packet.get("schema_version")
    if not isinstance(version, int) or isinstance(version, bool) or version not in {1, 2, 3}:
        return  # The published schema owns the base type/range diagnostic.
    validate_versioned_alias(
        packet, version, "pre_mortem_verdict", "premortem_verdict"
    )
    artifacts = packet.get("artifacts")
    if isinstance(artifacts, dict):
        validate_versioned_alias(
            artifacts,
            version,
            "pre_mortem_path",
            "premortem_path",
            "artifacts.",
            False,
        )


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
    required = [
        ("discovery", "discovery", "DONE"),
        ("crank", "crank", "DONE"),
        ("validate", "validate", "PASS"),
        ("learn", "learn", "DONE"),
    ]
    packet_state = packet.get("packet_state", "terminal")
    if packet_state not in {"prospective", "terminal"}:
        fail("packet_state must be prospective or terminal")
    if len(receipts) != len(required):
        fail("phase_receipts must contain discovery, crank, validate, learn in order")
    for index, (receipt, expected) in enumerate(zip(receipts, required)):
        if not isinstance(receipt, dict):
            fail(f"phase_receipts[{index}] must be an object")
        phase = require_nonempty(receipt.get("phase"), f"phase_receipts[{index}].phase")
        skill = require_nonempty(receipt.get("skill"), f"phase_receipts[{index}].skill")
        status = require_nonempty(receipt.get("status"), f"phase_receipts[{index}].status")
        expected_phase, expected_skill, expected_status = expected
        if (phase, skill) != (expected_phase, expected_skill):
            fail(
                f"phase_receipts[{index}] must be phase {expected_phase} "
                f"with skill {expected_skill}"
            )
        if packet_state == "terminal":
            if skill not in loaded_names:
                fail(f"phase_receipts[{index}].skill is absent from skills_loaded")
            if status != expected_status:
                fail(
                    "terminal phase_receipts must be successful; "
                    f"phase_receipts[{index}].status {status}, want {expected_status}"
                )
            resolve_artifact(repo_root, receipt.get("artifact"), f"phase_receipts[{index}].artifact")
            continue

        prospective_statuses = ["DONE", "pending", "not_checked", "not_checked"]
        expected_prospective = prospective_statuses[index]
        if status != expected_prospective:
            fail(
                "prospective phase_receipts must record only Discovery success; "
                f"phase_receipts[{index}].status {status}, want {expected_prospective}"
            )
        if index == 0:
            if skill not in loaded_names:
                fail(f"phase_receipts[{index}].skill is absent from skills_loaded")
            resolve_artifact(repo_root, receipt.get("artifact"), f"phase_receipts[{index}].artifact")
        else:
            if skill in loaded_names:
                fail(f"prospective skills_loaded must omit unrun phase skill: {skill}")
            if "artifact" in receipt:
                fail(f"prospective phase_receipts[{index}] must omit artifact for an unrun phase")


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
        validate_mortem_aliases(packet)
        validate_receipts(packet, repo_root)
        Draft202012Validator(schema, format_checker=FormatChecker()).validate(packet)
    except (OSError, json.JSONDecodeError, ValueError) as exc:
        print(f"invalid execution packet: {exc}", file=sys.stderr)
        return 1
    except Exception as exc:  # jsonschema emits several concrete validation errors
        print(f"invalid execution packet: {exc}", file=sys.stderr)
        return 1

    packet_state = packet.get("packet_state", "terminal")
    print(f"valid {packet_state} execution packet: {packet_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
