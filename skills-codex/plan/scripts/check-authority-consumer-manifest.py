#!/usr/bin/env python3
"""Check a migration's authority/consumer inventory and classify slice scopes."""

from __future__ import annotations

import argparse
import json
from pathlib import Path, PurePosixPath
from typing import Any


def normalize_path(value: Any, label: str, reasons: list[str]) -> str | None:
    if not isinstance(value, str) or not value.strip():
        reasons.append(f"{label} must be a nonempty repository-relative path")
        return None
    candidate = PurePosixPath(value)
    if candidate.is_absolute() or ".." in candidate.parts or str(candidate) in {"", "."}:
        reasons.append(f"{label} must be a normalized repository-relative path: {value}")
        return None
    return str(candidate)


def read_inventory_output(path: Path) -> tuple[set[str], list[str]]:
    """Read independently captured newline-delimited repository paths as data."""
    reasons: list[str] = []
    paths: set[str] = set()
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError as exc:
        return set(), [f"cannot read inventory output: {exc}"]
    for index, raw_path in enumerate(lines, start=1):
        if not raw_path.strip():
            continue
        normalized = normalize_path(raw_path.strip(), f"inventory output line {index}", reasons)
        if normalized:
            if normalized in paths:
                reasons.append(f"duplicate inventory output path: {normalized}")
            paths.add(normalized)
    if not paths:
        reasons.append("inventory output must contain at least one repository-relative path")
    return paths, reasons


def exact_keys(value: Any, required: set[str], label: str, reasons: list[str]) -> bool:
    if not isinstance(value, dict):
        reasons.append(f"{label} must be an object")
        return False
    missing = sorted(required - value.keys())
    extra = sorted(value.keys() - required)
    if missing:
        reasons.append(f"{label} missing fields: {', '.join(missing)}")
    if extra:
        reasons.append(f"{label} has unknown fields: {', '.join(extra)}")
    return not missing and not extra


def render(status: str, classification: str, shared: list[str], reasons: list[str]) -> dict[str, Any]:
    return {
        "manifest_status": status,
        "scope_classification": classification,
        "parallel_safe": status == "complete" and classification == "disjoint",
        "shared_paths": sorted(shared),
        "reasons": sorted(set(reasons)),
    }


def check(
    repo: Path,
    manifest: Any,
    inventory_output_paths: set[str],
    inventory_output_reasons: list[str],
) -> tuple[dict[str, Any], int]:
    reasons = list(inventory_output_reasons)
    top_fields = {"schema_version", "migration_id", "authorities", "inventory", "consumers", "slices"}
    if not exact_keys(manifest, top_fields, "manifest", reasons):
        return render("incomplete", "incomplete", [], reasons), 1
    if manifest.get("schema_version") != 1:
        reasons.append("schema_version must be 1")
    if not isinstance(manifest.get("migration_id"), str) or not manifest["migration_id"].strip():
        reasons.append("migration_id must be a nonempty string")

    authority_paths: set[str] = set()
    authorities = manifest.get("authorities")
    if not isinstance(authorities, list) or not authorities:
        reasons.append("authorities must be a nonempty array")
        authorities = []
    for index, authority in enumerate(authorities):
        if not exact_keys(authority, {"path", "symbols"}, f"authorities[{index}]", reasons):
            continue
        path = normalize_path(authority.get("path"), f"authorities[{index}].path", reasons)
        symbols = authority.get("symbols")
        if not isinstance(symbols, list) or not symbols or any(not isinstance(item, str) or not item.strip() for item in symbols):
            reasons.append(f"authorities[{index}].symbols must be a nonempty string array")
        if path:
            if path in authority_paths:
                reasons.append(f"duplicate authority path: {path}")
            authority_paths.add(path)

    inventory = manifest.get("inventory")
    observed_paths: set[str] = set()
    if exact_keys(inventory, {"command", "observed_paths", "complete"}, "inventory", reasons):
        if not isinstance(inventory.get("command"), str) or not inventory["command"].strip():
            reasons.append("inventory.command must record the nonempty command used")
        if inventory.get("complete") is not True:
            reasons.append("inventory was not checked complete")
        observed = inventory.get("observed_paths")
        if not isinstance(observed, list) or not observed:
            reasons.append("inventory.observed_paths must be a nonempty array")
        else:
            for index, raw_path in enumerate(observed):
                path = normalize_path(raw_path, f"inventory.observed_paths[{index}]", reasons)
                if path:
                    if path in observed_paths:
                        reasons.append(f"duplicate observed path: {path}")
                    observed_paths.add(path)

    for path in sorted(inventory_output_paths - observed_paths):
        reasons.append(f"inventory output path omitted from observed_paths: {path}")
    for path in sorted(observed_paths - inventory_output_paths):
        reasons.append(f"observed_paths entry absent from inventory output: {path}")

    consumer_paths: set[str] = set()
    consumers = manifest.get("consumers")
    if not isinstance(consumers, list):
        reasons.append("consumers must be an array")
        consumers = []
    valid_kinds = {"runtime", "test", "docs", "generated", "schema", "fixture"}
    for index, consumer in enumerate(consumers):
        fields = {"path", "authority_path", "kind"}
        if not exact_keys(consumer, fields, f"consumers[{index}]", reasons):
            continue
        path = normalize_path(consumer.get("path"), f"consumers[{index}].path", reasons)
        authority = normalize_path(consumer.get("authority_path"), f"consumers[{index}].authority_path", reasons)
        if consumer.get("kind") not in valid_kinds:
            reasons.append(f"consumers[{index}].kind is not one of {sorted(valid_kinds)}")
        if authority and authority not in authority_paths:
            reasons.append(f"consumer authority is not declared: {authority}")
        if path:
            if path in consumer_paths or path in authority_paths:
                reasons.append(f"path is classified more than once: {path}")
            consumer_paths.add(path)

    classified_paths = authority_paths | consumer_paths
    for path in sorted(observed_paths - classified_paths):
        reasons.append(f"observed path is not classified: {path}")
    for path in sorted(classified_paths - observed_paths):
        reasons.append(f"classified path was not observed: {path}")

    owners: dict[str, list[str]] = {}
    slices = manifest.get("slices")
    if not isinstance(slices, list) or not slices:
        reasons.append("slices must be a nonempty array")
        slices = []
    slice_ids: set[str] = set()
    for index, slice_entry in enumerate(slices):
        if not exact_keys(slice_entry, {"id", "write_scope"}, f"slices[{index}]", reasons):
            continue
        slice_id = slice_entry.get("id")
        if not isinstance(slice_id, str) or not slice_id.strip():
            reasons.append(f"slices[{index}].id must be a nonempty string")
            continue
        if slice_id in slice_ids:
            reasons.append(f"duplicate slice id: {slice_id}")
        slice_ids.add(slice_id)
        scope = slice_entry.get("write_scope")
        if not isinstance(scope, list) or not scope:
            reasons.append(f"slices[{index}].write_scope must be a nonempty array")
            continue
        for path_index, raw_path in enumerate(scope):
            path = normalize_path(raw_path, f"slices[{index}].write_scope[{path_index}]", reasons)
            if path:
                owners.setdefault(path, []).append(slice_id)

    for path in sorted(observed_paths - owners.keys()):
        reasons.append(f"observed path has no slice owner: {path}")
    for path in sorted(owners.keys() - observed_paths):
        reasons.append(f"slice-owned path was not observed: {path}")
    for path in sorted(observed_paths):
        if not (repo / path).exists():
            reasons.append(f"path does not exist: {path}")

    shared = [path for path, path_owners in owners.items() if len(set(path_owners)) > 1]
    if reasons:
        return render("incomplete", "incomplete", shared, reasons), 1
    classification = "shared" if shared else "disjoint"
    return render("complete", classification, shared, []), 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    parser.add_argument(
        "--inventory-output",
        type=Path,
        required=True,
        help="newline-delimited repository paths captured by a separately executed safe inventory command",
    )
    args = parser.parse_args()
    try:
        manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        print(json.dumps(render("incomplete", "incomplete", [], [f"cannot read manifest: {exc}"]), sort_keys=True))
        return 2
    inventory_paths, inventory_reasons = read_inventory_output(args.inventory_output)
    result, status = check(args.repo.resolve(), manifest, inventory_paths, inventory_reasons)
    print(json.dumps(result, sort_keys=True))
    return status


if __name__ == "__main__":
    raise SystemExit(main())
