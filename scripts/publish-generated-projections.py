#!/usr/bin/env python3
"""Render and publish every owner-mapped generated projection transactionally.

The publisher never uses Git for classification or recovery. It renders in a
disposable repository copy, snapshots exact live filesystem state, writes each
owned target from staging, and commits the publication manifest last.
"""

from __future__ import annotations

import argparse
from contextlib import contextmanager
from datetime import datetime, timezone
import fcntl
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import time
from typing import Any, Iterator
import uuid


OWNER_MAP_SCHEMA = "generated-projection-owner-map.v1"
PUBLICATION_SCHEMA = "generated-projection-publication.v1"
RECEIPT_SCHEMA = "publication-receipt.v1"
CLASSIFICATIONS = {
    "CLEAN_CURRENT",
    "DIRTY_PRESERVE",
    "MISSING",
    "UNOWNED",
}
HEX64 = frozenset("0123456789abcdef")


class PublicationError(RuntimeError):
    """Publication cannot continue without violating the contract."""


class PublicationInterrupted(PublicationError):
    """A handled process signal interrupted publication."""


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(
        value,
        sort_keys=True,
        separators=(",", ":"),
        ensure_ascii=False,
    ).encode("utf-8")


def digest_value(value: Any) -> str:
    return hashlib.sha256(canonical_bytes(value)).hexdigest()


def sha256(payload: bytes) -> str:
    return hashlib.sha256(payload).hexdigest()


def valid_digest(value: Any) -> bool:
    return (
        isinstance(value, str)
        and len(value) == 64
        and all(character in HEX64 for character in value)
    )


def finalize_artifact(value: dict[str, Any]) -> dict[str, Any]:
    artifact = dict(value)
    artifact.pop("artifact_digest", None)
    artifact["artifact_digest"] = digest_value(artifact)
    return artifact


def verify_artifact(value: dict[str, Any], label: str) -> None:
    claimed = value.get("artifact_digest")
    identity = {key: item for key, item in value.items() if key != "artifact_digest"}
    if not valid_digest(claimed) or digest_value(identity) != claimed:
        raise PublicationError(f"{label} artifact_digest is invalid")


def reject_duplicate_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise PublicationError(f"duplicate JSON key: {key}")
        result[key] = value
    return result


def load_json(path: Path, label: str | None = None) -> dict[str, Any]:
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as exc:
        raise PublicationError(f"cannot read {label or path}: {exc}") from exc
    decoder = json.JSONDecoder(object_pairs_hook=reject_duplicate_keys)
    try:
        value, end = decoder.raw_decode(text)
    except json.JSONDecodeError as exc:
        raise PublicationError(f"{label or path} is not valid JSON: {exc}") from exc
    if text[end:].strip():
        raise PublicationError(f"{label or path} contains trailing JSON data")
    if not isinstance(value, dict):
        raise PublicationError(f"{label or path} must be a JSON object")
    return value


def exact_keys(
    value: dict[str, Any],
    required: set[str],
    label: str,
) -> None:
    missing = sorted(required - value.keys())
    extra = sorted(value.keys() - required)
    if missing:
        raise PublicationError(f"{label} missing fields: {', '.join(missing)}")
    if extra:
        raise PublicationError(f"{label} has unknown fields: {', '.join(extra)}")


def normalize_ref(raw: Any) -> str:
    if not isinstance(raw, str) or not raw:
        raise PublicationError("repository reference must be a nonempty string")
    if raw == "." or "\\" in raw or raw.startswith("/") or raw.endswith("/"):
        raise PublicationError(f"unsafe repository reference: {raw!r}")
    if len(raw) >= 2 and raw[0].isalpha() and raw[1] == ":":
        raise PublicationError(f"drive-qualified repository reference: {raw!r}")
    if any(ord(character) < 32 or ord(character) == 127 for character in raw):
        raise PublicationError(f"control character in repository reference: {raw!r}")
    parts = raw.split("/")
    if any(part in {"", ".", ".."} for part in parts):
        raise PublicationError(f"non-canonical repository reference: {raw!r}")
    if PurePosixPath(raw).as_posix() != raw:
        raise PublicationError(f"non-POSIX repository reference: {raw!r}")
    return raw


def inside(path: str, root: str) -> bool:
    return path == root or path.startswith(root + "/")


def resolve_ref(
    repository: Path,
    reference: str,
    *,
    allow_leaf_symlink: bool = True,
) -> Path:
    reference = normalize_ref(reference)
    repository = repository.resolve()
    current = repository
    parts = reference.split("/")
    for index, part in enumerate(parts):
        current = current / part
        if current.is_symlink() and (index < len(parts) - 1 or not allow_leaf_symlink):
            raise PublicationError(
                f"repository reference traverses a symlink: {reference}"
            )
    return repository / reference


def validate_owner_map(value: dict[str, Any], repository: Path) -> None:
    exact_keys(
        value,
        {"schema_version", "publication", "owners", "artifact_digest"},
        OWNER_MAP_SCHEMA,
    )
    if value["schema_version"] != OWNER_MAP_SCHEMA:
        raise PublicationError("owner map schema_version is invalid")
    verify_artifact(value, OWNER_MAP_SCHEMA)
    publication = value["publication"]
    if not isinstance(publication, dict):
        raise PublicationError("owner map publication must be an object")
    exact_keys(
        publication,
        {"lock_ref", "manifest_ref", "receipt_dir_ref", "transaction_dir_ref"},
        "owner map publication",
    )
    publication_refs = [
        normalize_ref(publication[field])
        for field in (
            "lock_ref",
            "manifest_ref",
            "receipt_dir_ref",
            "transaction_dir_ref",
        )
    ]
    if len(set(publication_refs)) != len(publication_refs):
        raise PublicationError("owner map publication references must be unique")
    owners = value["owners"]
    if not isinstance(owners, list) or not owners:
        raise PublicationError("owner map owners must be a nonempty array")
    owner_ids: set[str] = set()
    target_paths: list[str] = []
    for owner_index, owner in enumerate(owners):
        label = f"owners[{owner_index}]"
        if not isinstance(owner, dict):
            raise PublicationError(f"{label} must be an object")
        exact_keys(
            owner,
            {"id", "generator_refs", "source_refs", "commands", "targets"},
            label,
        )
        owner_id = owner["id"]
        if (
            not isinstance(owner_id, str)
            or not owner_id
            or owner_id in owner_ids
            or not owner_id[0].isalpha()
            or any(
                not (character.islower() or character.isdigit() or character == "-")
                for character in owner_id
            )
        ):
            raise PublicationError(f"{label}.id is invalid or duplicated")
        owner_ids.add(owner_id)
        for field in ("generator_refs", "source_refs"):
            references = owner[field]
            if (
                not isinstance(references, list)
                or not references
                or len(references) != len(set(references))
            ):
                raise PublicationError(f"{label}.{field} must be nonempty and unique")
            for reference in references:
                path = resolve_ref(
                    repository,
                    normalize_ref(reference),
                    allow_leaf_symlink=False,
                )
                if not os.path.lexists(path):
                    raise PublicationError(
                        f"{label}.{field} reference is missing: {reference}"
                    )
        commands = owner["commands"]
        if not isinstance(commands, list) or not commands:
            raise PublicationError(f"{label}.commands must be nonempty")
        for command_index, command in enumerate(commands):
            if (
                not isinstance(command, list)
                or not command
                or any(not isinstance(arg, str) or not arg for arg in command)
            ):
                raise PublicationError(
                    f"{label}.commands[{command_index}] must be a nonempty argv"
                )
        targets = owner["targets"]
        if not isinstance(targets, list) or not targets:
            raise PublicationError(f"{label}.targets must be nonempty")
        for target_index, target in enumerate(targets):
            target_label = f"{label}.targets[{target_index}]"
            if not isinstance(target, dict):
                raise PublicationError(f"{target_label} must be an object")
            exact_keys(target, {"path", "kind", "allow_missing"}, target_label)
            target_path = normalize_ref(target["path"])
            if target["kind"] not in {"file", "tree", "symlink", "any"}:
                raise PublicationError(f"{target_label}.kind is invalid")
            if not isinstance(target["allow_missing"], bool):
                raise PublicationError(f"{target_label}.allow_missing must be boolean")
            target_paths.append(target_path)
    if len(target_paths) != len(set(target_paths)):
        raise PublicationError("projection targets must have exactly one owner")
    for index, left in enumerate(target_paths):
        for right in target_paths[index + 1 :]:
            if inside(left, right) or inside(right, left):
                raise PublicationError(
                    f"projection targets overlap: {left} and {right}"
                )
    for reference in publication_refs:
        if any(
            inside(reference, target) or inside(target, reference)
            for target in target_paths
        ):
            raise PublicationError(
                f"publication state overlaps generated target: {reference}"
            )


def all_targets(owner_map: dict[str, Any]) -> list[dict[str, Any]]:
    targets = []
    for owner in owner_map["owners"]:
        for target in owner["targets"]:
            targets.append({"owner": owner["id"], **target})
    return sorted(targets, key=lambda item: item["path"])


def state_identity(state: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in state.items() if key != "state_digest"}


def finalize_state(state: dict[str, Any]) -> dict[str, Any]:
    result = dict(state)
    result["state_digest"] = digest_value(result)
    return result


def validate_state(state: Any, label: str) -> None:
    if not isinstance(state, dict):
        raise PublicationError(f"{label} must be an object")
    kind = state.get("kind")
    fields = {
        "missing": {"kind", "state_digest"},
        "file": {"kind", "mode", "content_digest", "state_digest"},
        "symlink": {
            "kind",
            "mode",
            "link_target",
            "content_digest",
            "state_digest",
        },
        "directory": {"kind", "mode", "entries", "state_digest"},
    }
    if kind not in fields:
        raise PublicationError(f"{label}.kind is invalid")
    exact_keys(state, fields[kind], label)
    if not valid_digest(state["state_digest"]):
        raise PublicationError(f"{label}.state_digest is invalid")
    if digest_value(state_identity(state)) != state["state_digest"]:
        raise PublicationError(f"{label}.state_digest does not bind state")
    if kind == "missing":
        return
    mode = state["mode"]
    if (
        not isinstance(mode, str)
        or len(mode) != 4
        or any(character not in "01234567" for character in mode)
    ):
        raise PublicationError(f"{label}.mode is invalid")
    if kind in {"file", "symlink"} and not valid_digest(state["content_digest"]):
        raise PublicationError(f"{label}.content_digest is invalid")
    if kind == "symlink" and not isinstance(state["link_target"], str):
        raise PublicationError(f"{label}.link_target is invalid")
    if kind != "directory":
        return
    entries = state["entries"]
    if not isinstance(entries, list):
        raise PublicationError(f"{label}.entries must be an array")
    paths: list[str] = []
    for index, entry in enumerate(entries):
        entry_label = f"{label}.entries[{index}]"
        if not isinstance(entry, dict):
            raise PublicationError(f"{entry_label} must be an object")
        entry_kind = entry.get("kind")
        entry_fields = {
            "directory": {"path", "kind", "mode"},
            "file": {"path", "kind", "mode", "content_digest"},
            "symlink": {
                "path",
                "kind",
                "mode",
                "link_target",
                "content_digest",
            },
        }
        if entry_kind not in entry_fields:
            raise PublicationError(f"{entry_label}.kind is invalid")
        exact_keys(entry, entry_fields[entry_kind], entry_label)
        paths.append(normalize_ref(entry["path"]))
        if (
            not isinstance(entry["mode"], str)
            or len(entry["mode"]) != 4
            or any(character not in "01234567" for character in entry["mode"])
        ):
            raise PublicationError(f"{entry_label}.mode is invalid")
        if entry_kind in {"file", "symlink"} and not valid_digest(
            entry["content_digest"]
        ):
            raise PublicationError(f"{entry_label}.content_digest is invalid")
        if entry_kind == "symlink" and not isinstance(entry["link_target"], str):
            raise PublicationError(f"{entry_label}.link_target is invalid")
    if paths != sorted(set(paths)):
        raise PublicationError(f"{label}.entries must be sorted and unique")


def mode_of(path: Path) -> str:
    return f"{stat.S_IMODE(path.lstat().st_mode):04o}"


def leaf_state(path: Path, relative: str) -> dict[str, Any]:
    metadata = path.lstat()
    mode = f"{stat.S_IMODE(metadata.st_mode):04o}"
    if path.is_symlink():
        target = os.readlink(path)
        return {
            "path": relative,
            "kind": "symlink",
            "mode": mode,
            "link_target": target,
            "content_digest": sha256(target.encode("utf-8")),
        }
    if path.is_file():
        return {
            "path": relative,
            "kind": "file",
            "mode": mode,
            "content_digest": sha256(path.read_bytes()),
        }
    if path.is_dir():
        return {
            "path": relative,
            "kind": "directory",
            "mode": mode,
        }
    raise PublicationError(f"unsupported filesystem kind: {path}")


def snapshot_path(path: Path) -> dict[str, Any]:
    if not os.path.lexists(path):
        return finalize_state({"kind": "missing"})
    if path.is_symlink():
        target = os.readlink(path)
        return finalize_state(
            {
                "kind": "symlink",
                "mode": mode_of(path),
                "link_target": target,
                "content_digest": sha256(target.encode("utf-8")),
            }
        )
    if path.is_file():
        return finalize_state(
            {
                "kind": "file",
                "mode": mode_of(path),
                "content_digest": sha256(path.read_bytes()),
            }
        )
    if not path.is_dir():
        raise PublicationError(f"unsupported filesystem kind: {path}")
    entries: list[dict[str, Any]] = []
    for child in sorted(path.rglob("*")):
        relative = child.relative_to(path).as_posix()
        entries.append(leaf_state(child, relative))
        if child.is_symlink():
            continue
    return finalize_state(
        {
            "kind": "directory",
            "mode": mode_of(path),
            "entries": entries,
        }
    )


def aggregate_states(
    rows: list[dict[str, Any]],
    field: str,
) -> str:
    return digest_value(
        [
            {
                "owner": row["owner"],
                "path": row["path"],
                "state": row[field],
            }
            for row in sorted(rows, key=lambda item: item["path"])
        ]
    )


def expected_kind_matches(target: dict[str, Any], state: dict[str, Any]) -> bool:
    if state["kind"] == "missing":
        return bool(target["allow_missing"])
    expected = target["kind"]
    return (
        expected == "any"
        or (expected == "file" and state["kind"] == "file")
        or (expected == "tree" and state["kind"] == "directory")
        or (expected == "symlink" and state["kind"] == "symlink")
    )


def prior_owned_entries(
    prior_manifest: dict[str, Any] | None,
    target: dict[str, Any],
) -> set[str]:
    if prior_manifest is None:
        return set()
    for row in prior_manifest.get("targets", []):
        if row.get("owner") == target["owner"] and row.get("path") == target["path"]:
            state = row.get("state") or {}
            if state.get("kind") == "directory":
                return {
                    entry["path"]
                    for entry in state.get("entries", [])
                    if isinstance(entry, dict) and isinstance(entry.get("path"), str)
                }
            return {"."}
    return set()


def classify_target(
    target: dict[str, Any],
    before: dict[str, Any],
    rendered: dict[str, Any],
    prior_manifest: dict[str, Any] | None,
) -> str:
    if before["kind"] == "missing":
        return "MISSING"
    if before["state_digest"] == rendered["state_digest"]:
        return "CLEAN_CURRENT"
    prior_entries = prior_owned_entries(prior_manifest, target)
    if before["kind"] != rendered["kind"] and not prior_entries:
        return "UNOWNED"
    if before["kind"] == "directory":
        before_entries = {entry["path"] for entry in before["entries"]}
        rendered_entries = (
            {entry["path"] for entry in rendered["entries"]}
            if rendered["kind"] == "directory"
            else set()
        )
        unowned = (before_entries - rendered_entries) - prior_entries
        if unowned:
            return "UNOWNED"
    return "DIRTY_PRESERVE"


def scan_tree(
    root: Path,
    *,
    excluded_roots: list[str] | None = None,
) -> dict[str, dict[str, Any]]:
    excluded_roots = excluded_roots or []
    result: dict[str, dict[str, Any]] = {}
    for path in sorted(root.rglob("*")):
        relative = path.relative_to(root).as_posix()
        if (
            inside(relative, ".git")
            or inside(relative, ".agents")
            or "__pycache__" in path.parts
            or path.suffix in {".pyc", ".pyo"}
            or any(inside(relative, excluded) for excluded in excluded_roots)
        ):
            continue
        result[relative] = leaf_state(path, relative)
    return result


def changed_paths(
    before: dict[str, dict[str, Any]],
    after: dict[str, dict[str, Any]],
) -> list[str]:
    return sorted(
        path for path in set(before) | set(after) if before.get(path) != after.get(path)
    )


def stage_ignore(directory: str, names: list[str]) -> set[str]:
    ignored = {"__pycache__"}
    if Path(directory).name == "":
        ignored.update({".git", ".agents"})
    ignored.update(name for name in names if name.endswith((".pyc", ".pyo")))
    if ".git" in names:
        ignored.add(".git")
    if ".agents" in names:
        ignored.add(".agents")
    return ignored


def run_owner(
    owner: dict[str, Any],
    stage: Path,
    environment: dict[str, str],
) -> None:
    allowed = [target["path"] for target in owner["targets"]]
    before = scan_tree(stage)
    for command in owner["commands"]:
        result = subprocess.run(
            command,
            cwd=stage,
            env=environment,
            check=False,
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            detail = (result.stderr or result.stdout).strip()
            raise PublicationError(
                f"owner {owner['id']} render failed ({result.returncode}): {detail}"
            )
    after = scan_tree(stage)
    outside = [
        path
        for path in changed_paths(before, after)
        if not any(inside(path, target) for target in allowed)
    ]
    if outside:
        raise PublicationError(
            f"owner {owner['id']} mutated undeclared staged paths: "
            + ", ".join(outside)
        )


def render_complete(
    repository: Path,
    owner_map: dict[str, Any],
) -> tuple[tempfile.TemporaryDirectory[str], Path, list[dict[str, Any]]]:
    temporary = tempfile.TemporaryDirectory(prefix="ao-projection-stage-")
    stage = Path(temporary.name) / "repository"
    shutil.copytree(
        repository,
        stage,
        symlinks=True,
        ignore=stage_ignore,
    )
    environment = dict(os.environ)
    environment.update(
        {
            "AGENTOPS_PUBLICATION_STAGE_ROOT": str(stage),
            "AGENTOPS_PUBLICATION": "1",
            "PYTHONDONTWRITEBYTECODE": "1",
        }
    )
    for owner in owner_map["owners"]:
        run_owner(owner, stage, environment)
    targets = []
    for target in all_targets(owner_map):
        state = snapshot_path(resolve_ref(stage, target["path"]))
        if not expected_kind_matches(target, state):
            raise PublicationError(
                f"owner {target['owner']} rendered unexpected kind for "
                f"{target['path']}: {state['kind']}"
            )
        targets.append({**target, "rendered": state})
    first_digest = aggregate_states(targets, "rendered")
    for owner in owner_map["owners"]:
        run_owner(owner, stage, environment)
    second = []
    for target in all_targets(owner_map):
        state = snapshot_path(resolve_ref(stage, target["path"]))
        second.append({**target, "rendered": state})
    second_digest = aggregate_states(second, "rendered")
    if first_digest != second_digest:
        raise PublicationError("generated projections are not byte-and-mode idempotent")
    return temporary, stage, targets


def atomic_write_bytes(path: Path, payload: bytes, mode: int = 0o644) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(
        prefix=f".{path.name}.",
        suffix=".tmp",
        dir=path.parent,
    )
    try:
        os.fchmod(descriptor, mode)
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
        fsync_directory(path.parent)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def atomic_write_json(path: Path, value: dict[str, Any]) -> None:
    atomic_write_bytes(path, canonical_bytes(value) + b"\n")


def fsync_directory(path: Path) -> None:
    descriptor = os.open(path, os.O_RDONLY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def fsync_tree(path: Path) -> None:
    if path.is_symlink():
        fsync_directory(path.parent)
        return
    if path.is_file():
        descriptor = os.open(path, os.O_RDONLY)
        try:
            os.fsync(descriptor)
        finally:
            os.close(descriptor)
        fsync_directory(path.parent)
        return
    if path.is_dir():
        for child in sorted(path.rglob("*"), reverse=True):
            if child.is_symlink():
                continue
            if child.is_file():
                descriptor = os.open(child, os.O_RDONLY)
                try:
                    os.fsync(descriptor)
                finally:
                    os.close(descriptor)
            elif child.is_dir():
                fsync_directory(child)
        fsync_directory(path)


def remove_path(path: Path) -> None:
    if path.is_symlink() or path.is_file():
        path.unlink()
    elif path.is_dir():
        shutil.rmtree(path)


def copy_path_exact(source: Path, destination: Path) -> None:
    if os.path.lexists(destination):
        remove_path(destination)
    destination.parent.mkdir(parents=True, exist_ok=True)
    if source.is_symlink():
        os.symlink(os.readlink(source), destination)
    elif source.is_file():
        shutil.copy2(source, destination, follow_symlinks=False)
    elif source.is_dir():
        shutil.copytree(
            source,
            destination,
            symlinks=True,
            copy_function=shutil.copy2,
        )
        shutil.copystat(source, destination, follow_symlinks=False)
    else:
        raise PublicationError(f"cannot copy unsupported path: {source}")


def build_recovery_bundle(
    repository: Path,
    transaction_root: Path,
    transaction_id: str,
    rows: list[dict[str, Any]],
    manifest_ref: str,
    owner_map_digest: str,
) -> tuple[Path, dict[str, Any]]:
    bundle = transaction_root / transaction_id
    bundle.mkdir(parents=True, exist_ok=False)
    recovery_rows = []
    for index, row in enumerate(rows):
        source = resolve_ref(repository, row["path"])
        slot_ref = f"targets/{index}"
        slot = bundle / slot_ref
        if row["before"]["kind"] != "missing":
            copy_path_exact(source, slot)
            fsync_tree(slot)
            if snapshot_path(slot)["state_digest"] != row["before"]["state_digest"]:
                raise PublicationError(f"recovery snapshot mismatch for {row['path']}")
        recovery_rows.append(
            {
                "path": row["path"],
                "slot_ref": slot_ref,
                "before": row["before"],
            }
        )
    journal = finalize_artifact(
        {
            "schema_version": "generated-projection-recovery.v1",
            "transaction_id": transaction_id,
            "owner_map_digest": owner_map_digest,
            "manifest_ref": manifest_ref,
            "targets": recovery_rows,
        }
    )
    atomic_write_json(bundle / "recovery.json", journal)
    fsync_directory(bundle)
    fsync_directory(transaction_root)
    return bundle, journal


def restore_bundle(
    repository: Path,
    bundle: Path,
    journal: dict[str, Any],
) -> None:
    for row in journal["targets"]:
        target = resolve_ref(repository, row["path"])
        if os.path.lexists(target):
            remove_path(target)
        if row["before"]["kind"] != "missing":
            slot = bundle / normalize_ref(row["slot_ref"])
            copy_path_exact(slot, target)
            fsync_tree(target)
        fsync_directory(target.parent)
        if snapshot_path(target)["state_digest"] != row["before"]["state_digest"]:
            raise PublicationError(f"recovery did not restore {row['path']}")
    transaction_id = journal["transaction_id"]
    parents = {
        resolve_ref(repository, row["path"]).parent for row in journal["targets"]
    }
    for parent in parents:
        for prefix in (
            f".ao-publish-old-{transaction_id}-",
            f".ao-publish-new-{transaction_id}-",
        ):
            for leftover in parent.glob(prefix + "*"):
                remove_path(leftover)
        fsync_directory(parent)


def load_publication_manifest(path: Path) -> dict[str, Any] | None:
    if not path.is_file():
        return None
    value = load_json(path, "publication manifest")
    exact_keys(
        value,
        {
            "schema_version",
            "transaction_id",
            "owner_map_digest",
            "input_digest",
            "rendered_digest",
            "targets",
            "published_at",
            "artifact_digest",
        },
        PUBLICATION_SCHEMA,
    )
    if value["schema_version"] != PUBLICATION_SCHEMA:
        raise PublicationError("publication manifest schema_version is invalid")
    verify_artifact(value, PUBLICATION_SCHEMA)
    for field in ("owner_map_digest", "input_digest", "rendered_digest"):
        if not valid_digest(value[field]):
            raise PublicationError(f"publication manifest {field} is invalid")
    if (
        not isinstance(value["transaction_id"], str)
        or not value["transaction_id"]
        or not isinstance(value["published_at"], str)
    ):
        raise PublicationError("publication manifest identity is invalid")
    targets = value["targets"]
    if not isinstance(targets, list) or not targets:
        raise PublicationError("publication manifest targets must be nonempty")
    paths: list[str] = []
    for index, row in enumerate(targets):
        label = f"publication manifest targets[{index}]"
        if not isinstance(row, dict):
            raise PublicationError(f"{label} must be an object")
        exact_keys(row, {"owner", "path", "declared_kind", "state"}, label)
        if not isinstance(row["owner"], str) or not row["owner"]:
            raise PublicationError(f"{label}.owner is invalid")
        paths.append(normalize_ref(row["path"]))
        if row["declared_kind"] not in {"file", "tree", "symlink", "any"}:
            raise PublicationError(f"{label}.declared_kind is invalid")
        validate_state(row["state"], f"{label}.state")
    if paths != sorted(set(paths)):
        raise PublicationError(
            "publication manifest target paths must be sorted and unique"
        )
    return value


def cleanup_bundle(bundle: Path) -> None:
    shutil.rmtree(bundle)
    fsync_directory(bundle.parent)


def recover_pending(
    repository: Path,
    transaction_root: Path,
    owner_map: dict[str, Any],
    *,
    allow_mutation: bool,
) -> tuple[int, int]:
    if not transaction_root.exists():
        return 0, 0
    bundles = sorted(path for path in transaction_root.iterdir() if path.is_dir())
    if bundles and not allow_mutation:
        raise PublicationError(
            "pending publication recovery exists; check mode is read-only, "
            "run --recover-only or --write"
        )
    restored = 0
    for bundle in bundles:
        journal = load_json(bundle / "recovery.json", "publication recovery journal")
        exact_keys(
            journal,
            {
                "schema_version",
                "transaction_id",
                "owner_map_digest",
                "manifest_ref",
                "targets",
                "artifact_digest",
            },
            "publication recovery journal",
        )
        if journal["schema_version"] != "generated-projection-recovery.v1":
            raise PublicationError("recovery journal schema_version is invalid")
        verify_artifact(journal, "publication recovery journal")
        expected_paths = [target["path"] for target in all_targets(owner_map)]
        journal_paths = [row.get("path") for row in journal["targets"]]
        if (
            journal["owner_map_digest"] != owner_map["artifact_digest"]
            or journal_paths != expected_paths
        ):
            raise PublicationError(
                "pending recovery does not bind the current owner map and targets"
            )
        manifest_path = resolve_ref(
            repository,
            journal["manifest_ref"],
            allow_leaf_symlink=False,
        )
        manifest = load_publication_manifest(manifest_path)
        committed = (
            manifest is not None
            and manifest["transaction_id"] == journal["transaction_id"]
        )
        if committed:
            manifest_states = {
                row["path"]: row["state"]["state_digest"] for row in manifest["targets"]
            }
            committed = all(
                snapshot_path(resolve_ref(repository, row["path"]))["state_digest"]
                == manifest_states.get(row["path"])
                for row in journal["targets"]
            )
        if not committed:
            restore_bundle(repository, bundle, journal)
            restored += 1
        cleanup_bundle(bundle)
    return len(bundles), restored


@contextmanager
def publication_lock(path: Path, timeout_seconds: float) -> Iterator[None]:
    if not path.is_file() or path.is_symlink():
        raise PublicationError(
            "publication lock_ref must name an existing regular contract file"
        )
    descriptor = os.open(path, os.O_RDONLY)
    deadline = time.monotonic() + timeout_seconds
    try:
        while True:
            try:
                fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
                break
            except BlockingIOError:
                if time.monotonic() >= deadline:
                    raise PublicationError(
                        f"publication lock timed out after {timeout_seconds:g}s"
                    )
                time.sleep(min(0.05, max(0.0, deadline - time.monotonic())))
        yield
    finally:
        try:
            fcntl.flock(descriptor, fcntl.LOCK_UN)
        finally:
            os.close(descriptor)


@contextmanager
def handled_signals() -> Iterator[None]:
    previous: dict[int, Any] = {}

    def interrupt(signum: int, _frame: Any) -> None:
        raise PublicationInterrupted(f"publication interrupted by signal {signum}")

    for signum in (signal.SIGINT, signal.SIGTERM):
        previous[signum] = signal.getsignal(signum)
        signal.signal(signum, interrupt)
    try:
        yield
    finally:
        for signum, handler in previous.items():
            signal.signal(signum, handler)


def prepare_target(
    stage: Path,
    repository: Path,
    target: dict[str, Any],
    transaction_id: str,
    index: int,
) -> Path | None:
    rendered = resolve_ref(stage, target["path"])
    if not os.path.lexists(rendered):
        return None
    live = resolve_ref(repository, target["path"])
    prepared = live.parent / f".ao-publish-new-{transaction_id}-{index}"
    if os.path.lexists(prepared):
        remove_path(prepared)
    copy_path_exact(rendered, prepared)
    fsync_tree(prepared)
    return prepared


def replace_target(
    live: Path,
    prepared: Path | None,
    transaction_id: str,
    index: int,
) -> Path | None:
    old = live.parent / f".ao-publish-old-{transaction_id}-{index}"
    if os.path.lexists(old):
        remove_path(old)
    if os.path.lexists(live):
        os.replace(live, old)
        fsync_directory(live.parent)
    if prepared is not None:
        os.replace(prepared, live)
        fsync_directory(live.parent)
    return old if os.path.lexists(old) else None


def remove_asides(paths: list[Path]) -> None:
    for path in paths:
        if os.path.lexists(path):
            remove_path(path)
            fsync_directory(path.parent)


def repository_input_digest(repository: Path, targets: list[dict[str, Any]]) -> str:
    excluded = [target["path"] for target in targets]
    return digest_value(scan_tree(repository, excluded_roots=excluded))


def receipt(
    *,
    mode: str,
    result: str,
    owner_map_ref: str,
    owner_map_digest: str,
    input_digest: str,
    before_digest: str,
    rendered_digest: str,
    after_digest: str,
    rows: list[dict[str, Any]],
    manifest_ref: str | None,
    manifest_digest: str | None,
    recovery_found: int,
    recovery_restored: int,
) -> dict[str, Any]:
    return finalize_artifact(
        {
            "schema_version": RECEIPT_SCHEMA,
            "mode": mode,
            "result": result,
            "owner_map_ref": owner_map_ref,
            "owner_map_digest": owner_map_digest,
            "input_digest": input_digest,
            "before_digest": before_digest,
            "rendered_digest": rendered_digest,
            "after_digest": after_digest,
            "classifications": [
                {
                    "owner": row["owner"],
                    "path": row["path"],
                    "classification": row["classification"],
                    "before_state_digest": row["before"]["state_digest"],
                    "rendered_state_digest": row["rendered"]["state_digest"],
                }
                for row in rows
            ],
            "publication_manifest_ref": manifest_ref,
            "publication_manifest_digest": manifest_digest,
            "recovery": {
                "pending_transactions_found": recovery_found,
                "restored_transactions": recovery_restored,
            },
            "checked": [
                "exclusive publication lock",
                "strict owner-map identity and non-overlap",
                "complete isolated staged render",
                "declared-owner staged mutation containment",
                "second-render byte and mode idempotence",
                "exact before, rendered, and after filesystem identities",
                "unowned collision refusal before live mutation",
                "filesystem recovery bundle independent of Git",
                "publication manifest written after owned targets",
            ],
            "observed_at": datetime.now(timezone.utc)
            .isoformat()
            .replace("+00:00", "Z"),
        }
    )


def store_receipt(receipt_value: dict[str, Any], destination: Path) -> Path:
    destination.mkdir(parents=True, exist_ok=True)
    target = destination / f"{receipt_value['artifact_digest']}.json"
    payload = canonical_bytes(receipt_value) + b"\n"
    if target.exists():
        if target.read_bytes() != payload:
            raise PublicationError(f"receipt integrity collision: {target}")
        return target
    atomic_write_bytes(target, payload)
    return target


def publish(
    repository: Path,
    owner_map_ref: str,
    owner_map: dict[str, Any],
    *,
    mode: str,
    fail_after_target: int | None,
    abrupt_after_target: int | None,
) -> tuple[dict[str, Any], Path | None]:
    targets = all_targets(owner_map)
    publication = owner_map["publication"]
    manifest_ref = publication["manifest_ref"]
    manifest_path = resolve_ref(
        repository,
        manifest_ref,
        allow_leaf_symlink=False,
    )
    transaction_root = resolve_ref(
        repository,
        publication["transaction_dir_ref"],
        allow_leaf_symlink=False,
    )
    receipt_root = resolve_ref(
        repository,
        publication["receipt_dir_ref"],
        allow_leaf_symlink=False,
    )
    recovery_found, recovery_restored = recover_pending(
        repository,
        transaction_root,
        owner_map,
        allow_mutation=mode in {"WRITE", "RECOVER"},
    )
    if mode == "RECOVER":
        empty_digest = digest_value([])
        value = receipt(
            mode="RECOVER",
            result="RECOVERED",
            owner_map_ref=owner_map_ref,
            owner_map_digest=owner_map["artifact_digest"],
            input_digest=repository_input_digest(repository, targets),
            before_digest=empty_digest,
            rendered_digest=empty_digest,
            after_digest=empty_digest,
            rows=[],
            manifest_ref=manifest_ref if manifest_path.exists() else None,
            manifest_digest=(
                load_publication_manifest(manifest_path)["artifact_digest"]
                if manifest_path.exists()
                else None
            ),
            recovery_found=recovery_found,
            recovery_restored=recovery_restored,
        )
        path = store_receipt(value, receipt_root)
        return value, path

    prior_manifest = load_publication_manifest(manifest_path)
    if prior_manifest is not None:
        expected_bindings = [
            (target["owner"], target["path"], target["kind"]) for target in targets
        ]
        observed_bindings = [
            (row["owner"], row["path"], row["declared_kind"])
            for row in prior_manifest["targets"]
        ]
        if (
            prior_manifest["owner_map_digest"] != owner_map["artifact_digest"]
            or observed_bindings != expected_bindings
        ):
            raise PublicationError(
                "publication manifest does not bind the current owner map"
            )
    input_digest = repository_input_digest(repository, targets)
    temporary, stage, rendered_rows = render_complete(repository, owner_map)
    try:
        rows = []
        for rendered_row in rendered_rows:
            before = snapshot_path(resolve_ref(repository, rendered_row["path"]))
            classification = classify_target(
                rendered_row,
                before,
                rendered_row["rendered"],
                prior_manifest,
            )
            if classification not in CLASSIFICATIONS:
                raise PublicationError("internal classification error")
            rows.append(
                {
                    **rendered_row,
                    "before": before,
                    "classification": classification,
                }
            )
        before_digest = aggregate_states(rows, "before")
        rendered_digest = aggregate_states(rows, "rendered")
        if any(row["classification"] == "UNOWNED" for row in rows):
            collisions = [
                row["path"] for row in rows if row["classification"] == "UNOWNED"
            ]
            raise PublicationError(
                "unowned projection collision before mutation: " + ", ".join(collisions)
            )
        clean = all(
            row["before"]["state_digest"] == row["rendered"]["state_digest"]
            for row in rows
        )
        if mode == "CHECK":
            value = receipt(
                mode="CHECK",
                result="CLEAN" if clean else "DRIFT",
                owner_map_ref=owner_map_ref,
                owner_map_digest=owner_map["artifact_digest"],
                input_digest=input_digest,
                before_digest=before_digest,
                rendered_digest=rendered_digest,
                after_digest=before_digest,
                rows=rows,
                manifest_ref=manifest_ref if prior_manifest else None,
                manifest_digest=(
                    prior_manifest["artifact_digest"] if prior_manifest else None
                ),
                recovery_found=recovery_found,
                recovery_restored=recovery_restored,
            )
            return value, None
        if (
            clean
            and prior_manifest is not None
            and prior_manifest["owner_map_digest"] == owner_map["artifact_digest"]
            and prior_manifest["input_digest"] == input_digest
            and prior_manifest["rendered_digest"] == rendered_digest
        ):
            value = receipt(
                mode="WRITE",
                result="IDEMPOTENT",
                owner_map_ref=owner_map_ref,
                owner_map_digest=owner_map["artifact_digest"],
                input_digest=input_digest,
                before_digest=before_digest,
                rendered_digest=rendered_digest,
                after_digest=before_digest,
                rows=rows,
                manifest_ref=manifest_ref,
                manifest_digest=prior_manifest["artifact_digest"],
                recovery_found=recovery_found,
                recovery_restored=recovery_restored,
            )
            return value, store_receipt(value, receipt_root)

        transaction_id = uuid.uuid4().hex
        bundle, journal = build_recovery_bundle(
            repository,
            transaction_root,
            transaction_id,
            rows,
            manifest_ref,
            owner_map["artifact_digest"],
        )
        asides: list[Path] = []
        committed = False
        try:
            with handled_signals():
                for index, row in enumerate(rows, start=1):
                    prepared = prepare_target(
                        stage,
                        repository,
                        row,
                        transaction_id,
                        index,
                    )
                    aside = replace_target(
                        resolve_ref(repository, row["path"]),
                        prepared,
                        transaction_id,
                        index,
                    )
                    if aside is not None:
                        asides.append(aside)
                    if fail_after_target == index:
                        raise PublicationError(f"injected failure after target {index}")
                    if abrupt_after_target == index:
                        os._exit(97)
                after_rows = [
                    {
                        **row,
                        "after": snapshot_path(resolve_ref(repository, row["path"])),
                    }
                    for row in rows
                ]
                for row in after_rows:
                    if row["after"]["state_digest"] != row["rendered"]["state_digest"]:
                        raise PublicationError(
                            f"published bytes differ from staging: {row['path']}"
                        )
                manifest = finalize_artifact(
                    {
                        "schema_version": PUBLICATION_SCHEMA,
                        "transaction_id": transaction_id,
                        "owner_map_digest": owner_map["artifact_digest"],
                        "input_digest": input_digest,
                        "rendered_digest": rendered_digest,
                        "targets": [
                            {
                                "owner": row["owner"],
                                "path": row["path"],
                                "declared_kind": row["kind"],
                                "state": row["after"],
                            }
                            for row in after_rows
                        ],
                        "published_at": datetime.now(timezone.utc)
                        .isoformat()
                        .replace("+00:00", "Z"),
                    }
                )
                atomic_write_json(manifest_path, manifest)
                committed = True
            remove_asides(asides)
            cleanup_bundle(bundle)
        except BaseException:
            if not committed:
                restore_bundle(repository, bundle, journal)
                remove_asides(asides)
                cleanup_bundle(bundle)
            raise
        final_rows = [
            {
                **row,
                "after": snapshot_path(resolve_ref(repository, row["path"])),
            }
            for row in rows
        ]
        after_digest = aggregate_states(final_rows, "after")
        manifest = load_publication_manifest(manifest_path)
        assert manifest is not None
        value = receipt(
            mode="WRITE",
            result="PUBLISHED",
            owner_map_ref=owner_map_ref,
            owner_map_digest=owner_map["artifact_digest"],
            input_digest=input_digest,
            before_digest=before_digest,
            rendered_digest=rendered_digest,
            after_digest=after_digest,
            rows=rows,
            manifest_ref=manifest_ref,
            manifest_digest=manifest["artifact_digest"],
            recovery_found=recovery_found,
            recovery_restored=recovery_restored,
        )
        return value, store_receipt(value, receipt_root)
    finally:
        temporary.cleanup()


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--repository",
        default=".",
        help="repository root containing the owner map and generator inputs",
    )
    parser.add_argument(
        "--owner-map",
        default="docs/contracts/generated-projection-owners.v1.json",
    )
    modes = parser.add_mutually_exclusive_group(required=True)
    modes.add_argument("--check", action="store_true")
    modes.add_argument("--write", action="store_true")
    modes.add_argument("--recover-only", action="store_true")
    modes.add_argument("--validate-owner-map", action="store_true")
    parser.add_argument("--lock-timeout", type=float, default=30.0)
    parser.add_argument("--fail-after-target", type=int)
    parser.add_argument("--abrupt-after-target", type=int)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    repository = Path(args.repository).resolve()
    try:
        if not repository.is_dir():
            raise PublicationError(f"repository is not a directory: {repository}")
        owner_map_ref = normalize_ref(args.owner_map)
        owner_map_path = resolve_ref(repository, owner_map_ref)
        owner_map = load_json(owner_map_path, OWNER_MAP_SCHEMA)
        validate_owner_map(owner_map, repository)
        if args.validate_owner_map:
            print(
                json.dumps(
                    {
                        "owner_map_digest": owner_map["artifact_digest"],
                        "owners": len(owner_map["owners"]),
                        "result": "PASS",
                        "targets": len(all_targets(owner_map)),
                    },
                    sort_keys=True,
                )
            )
            return 0
        if args.lock_timeout < 0:
            raise PublicationError("--lock-timeout must be nonnegative")
        mode = "CHECK" if args.check else "RECOVER" if args.recover_only else "WRITE"
        lock_path = resolve_ref(
            repository,
            owner_map["publication"]["lock_ref"],
            allow_leaf_symlink=False,
        )
        with publication_lock(lock_path, args.lock_timeout):
            locked_owner_map = load_json(owner_map_path, OWNER_MAP_SCHEMA)
            validate_owner_map(locked_owner_map, repository)
            if locked_owner_map["artifact_digest"] != owner_map["artifact_digest"]:
                raise PublicationError(
                    "owner map changed while acquiring publication lock"
                )
            owner_map = locked_owner_map
            value, receipt_path = publish(
                repository,
                owner_map_ref,
                owner_map,
                mode=mode,
                fail_after_target=args.fail_after_target,
                abrupt_after_target=args.abrupt_after_target,
            )
        classification_counts = {
            classification: sum(
                1
                for row in value["classifications"]
                if row["classification"] == classification
            )
            for classification in sorted(CLASSIFICATIONS)
        }
        output = {
            "artifact_digest": value["artifact_digest"],
            "classification_counts": classification_counts,
            "drift_paths": [
                row["path"]
                for row in value["classifications"]
                if row["before_state_digest"] != row["rendered_state_digest"]
            ],
            "mode": value["mode"],
            "publication_manifest_digest": value["publication_manifest_digest"],
            "receipt_path": (
                receipt_path.relative_to(repository).as_posix()
                if receipt_path is not None
                else None
            ),
            "rendered_digest": value["rendered_digest"],
            "result": value["result"],
        }
        print(json.dumps(output, sort_keys=True))
        if mode == "CHECK" and value["result"] != "CLEAN":
            return 1
        return 0
    except (OSError, PublicationError, subprocess.SubprocessError) as exc:
        print(f"publish-generated-projections: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
