#!/usr/bin/env python3
"""Exact-byte, fail-closed primitives for the RPI proof kernel.

This module is deliberately runtime-neutral.  It has no Git, tracker, queue,
network, retry, or delivery integration.  verdict.v2 and rpi-report.v1 remain
legacy read formats in validate.py; every writer in this module emits only the
new versioned contracts.
"""

from __future__ import annotations

from datetime import datetime
import fnmatch
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import tempfile
from typing import Any, Iterable


HEX64 = frozenset("0123456789abcdef")
SCHEMAS = {
    "verdict": "schemas/verdict.v3.schema.json",
    "rpi_report": "schemas/rpi-report.v2.schema.json",
    "subject_manifest": "schemas/subject-manifest.v2.schema.json",
    "scope_index": "schemas/scope-index.v1.schema.json",
    "effect_receipt": "schemas/effect-receipt.v1.schema.json",
    "check_receipt": "schemas/check-receipt.v1.schema.json",
}
PROOF_ACTIVE_REF = "docs/contracts/proof-contracts/active.json"
REPOSITORY_OBSERVATION = [{"id": "repository", "includes": ["."]}]
RUNTIME_EXCLUSIONS = frozenset(
    {
        ".git",
        ".agents/ao/intents",
        ".agents/ao/verdicts",
        ".agents/ao/reports",
    }
)
COMPLETE_RUNTIME_EXCLUSIONS = sorted(RUNTIME_EXCLUSIONS)
BASE_PROOF_COMPONENT_ROLES = frozenset(
    {
        "validator-contract",
        "validator-implementation",
        "verdict-schema",
        "rpi-report-schema",
        "subject-manifest-schema",
    }
)
EPOCH_ONE_PROOF_COMPONENT_ROLES = BASE_PROOF_COMPONENT_ROLES | frozenset(
    {
        "validator-cli",
        "scope-index-schema",
        "effect-receipt-schema",
        "check-receipt-schema",
    }
)


class ContractError(ValueError):
    """An artifact does not satisfy the exact kernel contract."""


class TerminalValidation(ContractError):
    """Validation must stop; the current candidate cannot be judged further."""


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(
        value,
        sort_keys=True,
        separators=(",", ":"),
        ensure_ascii=False,
    ).encode("utf-8")


def sha256(payload: bytes) -> str:
    return hashlib.sha256(payload).hexdigest()


def digest_value(value: Any) -> str:
    return sha256(canonical_bytes(value))


def valid_digest(value: Any) -> bool:
    return (
        isinstance(value, str)
        and len(value) == 64
        and all(character in HEX64 for character in value)
    )


def artifact_identity(value: dict[str, Any]) -> dict[str, Any]:
    return {key: item for key, item in value.items() if key != "artifact_digest"}


def finalize_artifact(value: dict[str, Any]) -> dict[str, Any]:
    artifact = dict(value)
    artifact.pop("artifact_digest", None)
    artifact["artifact_digest"] = digest_value(artifact)
    return artifact


def verify_artifact_digest(value: dict[str, Any], label: str) -> None:
    claimed = value.get("artifact_digest")
    if not valid_digest(claimed) or digest_value(artifact_identity(value)) != claimed:
        raise ContractError(f"{label} artifact_digest is invalid")


def _reject_duplicate_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise ContractError(f"duplicate JSON key: {key}")
        result[key] = value
    return result


def load_json_bytes(payload: bytes, label: str) -> dict[str, Any]:
    try:
        text = payload.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ContractError(f"{label} is not UTF-8 JSON") from exc
    decoder = json.JSONDecoder(object_pairs_hook=_reject_duplicate_keys)
    try:
        value, end = decoder.raw_decode(text)
    except json.JSONDecodeError as exc:
        raise ContractError(f"{label} is not valid JSON: {exc}") from exc
    if text[end:].strip():
        raise ContractError(f"{label} contains trailing JSON data")
    if not isinstance(value, dict):
        raise ContractError(f"{label} must be a JSON object")
    return value


def load_json(path: Path, label: str | None = None) -> dict[str, Any]:
    return load_json_bytes(path.read_bytes(), label or str(path))


def require_exact_keys(
    value: dict[str, Any],
    *,
    required: Iterable[str],
    optional: Iterable[str] = (),
    label: str,
) -> None:
    required_set = set(required)
    allowed = required_set | set(optional)
    missing = sorted(required_set - value.keys())
    extra = sorted(value.keys() - allowed)
    if missing:
        raise ContractError(f"{label} missing required fields: {', '.join(missing)}")
    if extra:
        raise ContractError(f"{label} contains unknown fields: {', '.join(extra)}")


def require_id(value: Any, label: str) -> str:
    if not isinstance(value, str) or not value:
        raise ContractError(f"{label} must be a nonempty string")
    allowed = set("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._:-")
    if value[0] not in allowed or any(character not in allowed for character in value):
        raise ContractError(f"{label} contains unsupported characters")
    return value


def require_datetime(value: Any, label: str) -> None:
    if not isinstance(value, str):
        raise ContractError(f"{label} must be an RFC3339 date-time")
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as exc:
        raise ContractError(f"{label} must be an RFC3339 date-time") from exc
    if parsed.tzinfo is None:
        raise ContractError(f"{label} must include a timezone")


def normalize_rel(raw: str) -> str:
    if not isinstance(raw, str) or not raw:
        raise ContractError("path must be a nonempty string")
    if raw == ".":
        return "."
    if "\\" in raw:
        raise ContractError(f"path contains a backslash: {raw}")
    if raw.startswith("/") or raw.startswith("//") or re.match(r"^[A-Za-z]:", raw):
        raise ContractError(f"path is absolute or drive-qualified: {raw}")
    if raw.endswith("/"):
        raise ContractError(f"path contains a trailing separator: {raw}")
    segments = raw.split("/")
    if any(segment in {"", ".", ".."} for segment in segments):
        raise ContractError(f"path contains an empty, dot, or parent segment: {raw}")
    if any(ord(character) < 32 or ord(character) == 127 for character in raw):
        raise ContractError(f"path contains a control character: {raw}")
    path = PurePosixPath(raw)
    if path.is_absolute() or path.as_posix() != raw:
        raise ContractError(f"path is not canonical POSIX-relative form: {raw}")
    return raw


def path_matches(path: str, pattern: str) -> bool:
    pattern = normalize_rel(pattern)
    if pattern == ".":
        return True
    if any(character in pattern for character in "*?["):
        return fnmatch.fnmatchcase(path, pattern)
    return path == pattern or path.startswith(pattern.rstrip("/") + "/")


def _excluded(path: str, exclusions: Iterable[str]) -> bool:
    return any(path_matches(path, exclusion) for exclusion in exclusions)


def _entry(root: Path, relative: str) -> dict[str, Any]:
    full = root if relative == "." else root / relative
    metadata = full.lstat()
    executable = bool(
        metadata.st_mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    )
    if full.is_symlink():
        digest = sha256(os.readlink(full).encode("utf-8"))
        kind = "symlink"
    elif full.is_file():
        digest = sha256(full.read_bytes())
        kind = "file"
    else:
        raise ContractError(f"unsupported manifest entry kind: {relative}")
    return {
        "path": relative,
        "kind": kind,
        "executable": executable,
        "digest": digest,
    }


def _walk(root: Path, include: str, exclusions: list[str]) -> list[dict[str, Any]]:
    full = root if include == "." else root / include
    if not full.exists() and not full.is_symlink():
        return []
    if full.is_file() or full.is_symlink():
        return [] if _excluded(include, exclusions) else [_entry(root, include)]
    entries: list[dict[str, Any]] = []
    def fail_walk(error: OSError) -> None:
        raise ContractError(f"cannot completely observe subject tree: {error}")

    for raw_dir, dirnames, filenames in os.walk(
        full,
        followlinks=False,
        onerror=fail_walk,
    ):
        directory = Path(raw_dir)
        retained: list[str] = []
        for name in sorted(dirnames):
            child = directory / name
            relative = normalize_rel(child.relative_to(root).as_posix())
            if _excluded(relative, exclusions):
                continue
            if child.is_symlink():
                entries.append(_entry(root, relative))
            else:
                retained.append(name)
        dirnames[:] = retained
        for name in sorted(filenames):
            relative = normalize_rel((directory / name).relative_to(root).as_posix())
            if not _excluded(relative, exclusions):
                entries.append(_entry(root, relative))
    return entries


def _normalize_observation_roots(
    observation_roots: list[dict[str, Any]],
) -> list[dict[str, Any]]:
    if not isinstance(observation_roots, list) or not observation_roots:
        raise ContractError("subject-manifest.v2 requires observation_roots")
    normalized: list[dict[str, Any]] = []
    seen_ids: set[str] = set()
    for index, item in enumerate(observation_roots):
        if not isinstance(item, dict):
            raise ContractError(f"observation_roots[{index}] must be an object")
        require_exact_keys(
            item,
            required=("id", "includes"),
            label=f"observation_roots[{index}]",
        )
        identifier = require_id(item["id"], f"observation_roots[{index}].id")
        if identifier in seen_ids:
            raise ContractError(f"duplicate observation root ID: {identifier}")
        seen_ids.add(identifier)
        includes = item["includes"]
        if not isinstance(includes, list) or not includes:
            raise ContractError(f"observation_roots[{index}].includes must be nonempty")
        normalized_includes = sorted({normalize_rel(path) for path in includes})
        if len(normalized_includes) != len(includes):
            raise ContractError(f"observation_roots[{index}].includes contains duplicates")
        normalized.append({"id": identifier, "includes": normalized_includes})
    return sorted(normalized, key=lambda item: item["id"])


def build_manifest_v2(
    root: Path,
    observation_roots: list[dict[str, Any]],
    exclusions: list[str],
) -> dict[str, Any]:
    root = root.resolve()
    if not root.is_dir():
        raise ContractError(f"subject root is not a directory: {root}")
    normalized_roots = _normalize_observation_roots(observation_roots)
    normalized_exclusions = sorted({normalize_rel(item) for item in exclusions})
    if len(normalized_exclusions) != len(exclusions):
        raise ContractError("subject-manifest.v2 exclusions contain duplicates")
    unsupported_exclusions = set(normalized_exclusions) - RUNTIME_EXCLUSIONS
    if unsupported_exclusions:
        raise ContractError(
            "subject-manifest.v2 contains unsupported runtime exclusions: "
            + ", ".join(sorted(unsupported_exclusions))
        )
    entries: dict[str, dict[str, Any]] = {}
    for observation in normalized_roots:
        for include in observation["includes"]:
            for item in _walk(root, include, normalized_exclusions):
                entries[item["path"]] = item
    manifest = {
        "schema_version": "subject-manifest.v2",
        "observation_roots": normalized_roots,
        "exclusions": normalized_exclusions,
        "entries": sorted(entries.values(), key=lambda item: item["path"]),
    }
    manifest["canonical_manifest_digest"] = digest_value(manifest)
    validate_manifest_v2(manifest)
    return manifest


def validate_manifest_v2(manifest: dict[str, Any]) -> None:
    require_exact_keys(
        manifest,
        required=(
            "schema_version",
            "observation_roots",
            "exclusions",
            "entries",
            "canonical_manifest_digest",
        ),
        label="subject-manifest.v2",
    )
    if manifest["schema_version"] != "subject-manifest.v2":
        raise ContractError("subject-manifest.v2 schema_version is invalid")
    roots = _normalize_observation_roots(manifest["observation_roots"])
    if roots != manifest["observation_roots"]:
        raise ContractError("subject-manifest.v2 observation_roots are not canonical")
    exclusions = manifest["exclusions"]
    if (
        not isinstance(exclusions, list)
        or exclusions != sorted(set(exclusions))
        or any(normalize_rel(item) != item for item in exclusions)
    ):
        raise ContractError("subject-manifest.v2 exclusions are not canonical")
    unsupported_exclusions = set(exclusions) - RUNTIME_EXCLUSIONS
    if unsupported_exclusions:
        raise ContractError(
            "subject-manifest.v2 contains unsupported runtime exclusions: "
            + ", ".join(sorted(unsupported_exclusions))
        )
    entries = manifest["entries"]
    if not isinstance(entries, list):
        raise ContractError("subject-manifest.v2 entries must be an array")
    paths: list[str] = []
    for index, item in enumerate(entries):
        if not isinstance(item, dict):
            raise ContractError(f"subject-manifest.v2 entries[{index}] must be an object")
        require_exact_keys(
            item,
            required=("path", "kind", "executable", "digest"),
            label=f"subject-manifest.v2 entries[{index}]",
        )
        path = normalize_rel(item["path"])
        if path != item["path"] or item["kind"] not in {"file", "symlink"}:
            raise ContractError(f"subject-manifest.v2 entries[{index}] is invalid")
        if not isinstance(item["executable"], bool) or not valid_digest(item["digest"]):
            raise ContractError(f"subject-manifest.v2 entries[{index}] is invalid")
        paths.append(path)
    if paths != sorted(set(paths)):
        raise ContractError("subject-manifest.v2 entry paths must be sorted and unique")
    claimed = manifest["canonical_manifest_digest"]
    identity = {
        key: value
        for key, value in manifest.items()
        if key != "canonical_manifest_digest"
    }
    if not valid_digest(claimed) or digest_value(identity) != claimed:
        raise ContractError("subject-manifest.v2 canonical_manifest_digest is invalid")


def verify_manifest_v2(manifest: dict[str, Any], root: Path) -> None:
    validate_manifest_v2(manifest)
    rebuilt = build_manifest_v2(
        root,
        manifest["observation_roots"],
        manifest["exclusions"],
    )
    if canonical_bytes(rebuilt) != canonical_bytes(manifest):
        raise TerminalValidation("candidate mutated after freeze")


def store_exact_bytes(payload: bytes, destination: Path, suffix: str) -> tuple[Path, bool]:
    destination.mkdir(parents=True, exist_ok=True)
    target = destination / f"{sha256(payload)}{suffix}"
    if target.exists():
        if target.read_bytes() == payload:
            return target, True
        raise ContractError(f"content-addressed integrity collision at {target}")
    file_descriptor, temporary = tempfile.mkstemp(
        prefix=".exact-",
        suffix=".tmp",
        dir=destination,
    )
    try:
        with os.fdopen(file_descriptor, "wb") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, target)
        directory_descriptor = os.open(destination, os.O_RDONLY)
        try:
            os.fsync(directory_descriptor)
        finally:
            os.close(directory_descriptor)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)
    return target, False


def atomic_write_bytes(target: Path, payload: bytes) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    file_descriptor, temporary = tempfile.mkstemp(
        prefix=f".{target.name}.",
        suffix=".tmp",
        dir=target.parent,
    )
    try:
        with os.fdopen(file_descriptor, "wb") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, target)
        directory_descriptor = os.open(target.parent, os.O_RDONLY)
        try:
            os.fsync(directory_descriptor)
        finally:
            os.close(directory_descriptor)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def atomic_write_json(target: Path, value: dict[str, Any]) -> None:
    atomic_write_bytes(
        target,
        json.dumps(
            value,
            sort_keys=True,
            indent=2,
            ensure_ascii=False,
        ).encode("utf-8")
        + b"\n",
    )


def mint_intent_snapshot(
    payload: bytes,
    destination: Path,
    *,
    expected_digest: str | None = None,
) -> tuple[Path, str, bool]:
    digest = sha256(payload)
    if expected_digest is not None and digest != expected_digest:
        raise TerminalValidation(
            f"exact intent digest mismatch: expected {expected_digest}, observed {digest}"
        )
    path, existed = store_exact_bytes(payload, destination, ".intent")
    return path, digest, existed


def consume_intent_snapshot(path: Path, expected_digest: str) -> bytes:
    if not valid_digest(expected_digest):
        raise ContractError("expected intent digest must be lowercase SHA-256")
    payload = path.read_bytes()
    observed = sha256(payload)
    if observed != expected_digest:
        raise TerminalValidation(
            f"exact intent digest mismatch: expected {expected_digest}, observed {observed}"
        )
    expected_name = f"{expected_digest}.intent"
    if path.name != expected_name:
        raise ContractError(
            f"intent snapshot filename must bind its digest: expected {expected_name}"
        )
    return payload


def validate_scope_index(index: dict[str, Any]) -> None:
    require_exact_keys(
        index,
        required=(
            "schema_version",
            "intent_digest",
            "frozen_at",
            "criteria",
            "scope_classes",
            "declared_exclusions",
            "artifact_digest",
        ),
        label="scope-index.v1",
    )
    if index["schema_version"] != "scope-index.v1" or not valid_digest(index["intent_digest"]):
        raise ContractError("scope-index.v1 identity is invalid")
    require_datetime(index["frozen_at"], "scope-index.v1 frozen_at")
    criteria = index["criteria"]
    if not isinstance(criteria, list) or not criteria:
        raise ContractError("scope-index.v1 criteria must be nonempty")
    criterion_ids: list[str] = []
    required_ids: set[str] = set()
    for position, criterion in enumerate(criteria):
        if not isinstance(criterion, dict):
            raise ContractError(f"scope-index.v1 criteria[{position}] must be an object")
        require_exact_keys(
            criterion,
            required=("id", "required", "statement_digest"),
            label=f"scope-index.v1 criteria[{position}]",
        )
        identifier = require_id(criterion["id"], f"criteria[{position}].id")
        if not isinstance(criterion["required"], bool) or not valid_digest(
            criterion["statement_digest"]
        ):
            raise ContractError(f"scope-index.v1 criteria[{position}] is invalid")
        criterion_ids.append(identifier)
        if criterion["required"]:
            required_ids.add(identifier)
    if len(criterion_ids) != len(set(criterion_ids)):
        raise ContractError("scope-index.v1 criterion IDs must be unique")
    scope_classes = index["scope_classes"]
    if not isinstance(scope_classes, list) or not scope_classes:
        raise ContractError("scope-index.v1 scope_classes must be nonempty")
    class_ids: set[str] = set()
    for position, scope_class in enumerate(scope_classes):
        if not isinstance(scope_class, dict):
            raise ContractError(f"scope_classes[{position}] must be an object")
        require_exact_keys(
            scope_class,
            required=("id", "patterns"),
            label=f"scope_classes[{position}]",
        )
        identifier = require_id(scope_class["id"], f"scope_classes[{position}].id")
        if identifier in class_ids:
            raise ContractError("scope-index.v1 scope class IDs must be unique")
        class_ids.add(identifier)
        patterns = scope_class["patterns"]
        if (
            not isinstance(patterns, list)
            or not patterns
            or len(patterns) != len(set(patterns))
        ):
            raise ContractError(f"scope_classes[{position}].patterns is invalid")
        for pattern in patterns:
            normalize_rel(pattern)
    exclusion_ids: set[str] = set()
    known_ids = set(criterion_ids)
    for position, exclusion in enumerate(index["declared_exclusions"]):
        if not isinstance(exclusion, dict):
            raise ContractError(f"declared_exclusions[{position}] must be an object")
        require_exact_keys(
            exclusion,
            required=("id", "criterion_ids", "reason"),
            label=f"declared_exclusions[{position}]",
        )
        identifier = require_id(exclusion["id"], f"declared_exclusions[{position}].id")
        if identifier in exclusion_ids:
            raise ContractError("scope-index.v1 exclusion IDs must be unique")
        exclusion_ids.add(identifier)
        excluded = exclusion["criterion_ids"]
        if (
            not isinstance(excluded, list)
            or not excluded
            or len(excluded) != len(set(excluded))
        ):
            raise ContractError(
                f"declared_exclusions[{position}].criterion_ids is invalid"
            )
        unknown = set(excluded) - known_ids
        if unknown:
            raise ContractError(
                "declared exclusion references unknown criteria: "
                + ", ".join(sorted(unknown))
            )
        absorbed = set(excluded) & required_ids
        if absorbed:
            raise ContractError(
                "declared exclusions cannot absorb required criteria: "
                + ", ".join(sorted(absorbed))
            )
        if not isinstance(exclusion["reason"], str) or not exclusion["reason"]:
            raise ContractError(f"declared_exclusions[{position}].reason is invalid")
    verify_artifact_digest(index, "scope-index.v1")


def freeze_scope_index(
    *,
    intent_digest: str,
    frozen_at: str,
    criteria: list[dict[str, Any]],
    scope_classes: list[dict[str, Any]],
    declared_exclusions: list[dict[str, Any]],
) -> dict[str, Any]:
    if not valid_digest(intent_digest):
        raise ContractError("scope-index.v1 intent_digest is invalid")
    frozen_criteria: list[dict[str, Any]] = []
    for position, criterion in enumerate(criteria):
        if not isinstance(criterion, dict):
            raise ContractError(f"criteria[{position}] must be an object")
        require_exact_keys(
            criterion,
            required=("id", "required", "statement"),
            label=f"criteria[{position}]",
        )
        if not isinstance(criterion["statement"], str) or not criterion["statement"]:
            raise ContractError(f"criteria[{position}].statement must be nonempty")
        frozen_criteria.append(
            {
                "id": criterion["id"],
                "required": criterion["required"],
                "statement_digest": sha256(criterion["statement"].encode("utf-8")),
            }
        )
    index = finalize_artifact(
        {
            "schema_version": "scope-index.v1",
            "intent_digest": intent_digest,
            "frozen_at": frozen_at,
            "criteria": frozen_criteria,
            "scope_classes": scope_classes,
            "declared_exclusions": declared_exclusions,
        }
    )
    validate_scope_index(index)
    return index


def _scope_patterns(scope_index: dict[str, Any]) -> list[str]:
    validate_scope_index(scope_index)
    return [
        pattern
        for scope_class in scope_index["scope_classes"]
        for pattern in scope_class["patterns"]
    ]


def derive_effect_receipt(
    before: dict[str, Any],
    final: dict[str, Any],
    scope_index: dict[str, Any],
    check_receipt_refs: list[dict[str, str]],
) -> dict[str, Any]:
    validate_manifest_v2(before)
    validate_manifest_v2(final)
    if (
        before["observation_roots"] != final["observation_roots"]
        or before["exclusions"] != final["exclusions"]
    ):
        raise TerminalValidation("before/final observation policy changed")
    patterns = _scope_patterns(scope_index)
    prior = {item["path"]: item for item in before["entries"]}
    current = {item["path"]: item for item in final["entries"]}
    changes: list[dict[str, Any]] = []
    for path in sorted(set(prior) | set(current)):
        old = prior.get(path)
        new = current.get(path)
        if old == new:
            continue
        if old is None:
            kind = "ADDED"
        elif new is None:
            kind = "DELETED"
        elif old["kind"] != new["kind"]:
            kind = "TYPE_CHANGED"
        elif old["digest"] == new["digest"] and old["executable"] != new["executable"]:
            kind = "MODE_CHANGED"
        else:
            kind = "MODIFIED"
        changes.append(
            {
                "path": path,
                "change_kind": kind,
                "before_digest": old["digest"] if old else None,
                "after_digest": new["digest"] if new else None,
            }
        )
    actual_paths = [item["path"] for item in changes]
    undeclared = [
        path
        for path in actual_paths
        if not any(path_matches(path, pattern) for pattern in patterns)
    ]
    normalized_refs: list[dict[str, str]] = []
    for position, reference in enumerate(check_receipt_refs):
        if not isinstance(reference, dict):
            raise ContractError(f"check_receipt_refs[{position}] must be an object")
        require_exact_keys(
            reference,
            required=("ref", "digest"),
            label=f"check_receipt_refs[{position}]",
        )
        if (
            not isinstance(reference["ref"], str)
            or not reference["ref"]
            or not valid_digest(reference["digest"])
        ):
            raise ContractError(f"check_receipt_refs[{position}] is invalid")
        normalized_reference = {
            "ref": normalize_rel(reference["ref"]),
            "digest": reference["digest"],
        }
        if normalized_reference["ref"] != reference["ref"]:
            raise ContractError(f"check_receipt_refs[{position}].ref is not canonical")
        normalized_refs.append(normalized_reference)
    normalized_refs.sort(key=lambda item: item["ref"])
    if len({item["ref"] for item in normalized_refs}) != len(normalized_refs):
        raise ContractError("check_receipt_refs contains duplicate refs")
    receipt = {
        "schema_version": "effect-receipt.v1",
        "before_manifest_digest": before["canonical_manifest_digest"],
        "final_manifest_digest": final["canonical_manifest_digest"],
        "coverage": (
            "COMPLETE"
            if before["observation_roots"] == REPOSITORY_OBSERVATION
            and final["observation_roots"] == REPOSITORY_OBSERVATION
            and before["exclusions"] == COMPLETE_RUNTIME_EXCLUSIONS
            and final["exclusions"] == COMPLETE_RUNTIME_EXCLUSIONS
            else "INCOMPLETE"
        ),
        "changes": changes,
        "actual_changed_paths": actual_paths,
        "undeclared_paths": undeclared,
        "check_receipt_refs": normalized_refs,
    }
    return finalize_artifact(receipt)


def validate_effect_receipt(receipt: dict[str, Any]) -> None:
    require_exact_keys(
        receipt,
        required=(
            "schema_version",
            "before_manifest_digest",
            "final_manifest_digest",
            "coverage",
            "changes",
            "actual_changed_paths",
            "undeclared_paths",
            "check_receipt_refs",
            "artifact_digest",
        ),
        label="effect-receipt.v1",
    )
    if receipt["schema_version"] != "effect-receipt.v1":
        raise ContractError("effect-receipt.v1 schema_version is invalid")
    if not valid_digest(receipt["before_manifest_digest"]) or not valid_digest(
        receipt["final_manifest_digest"]
    ):
        raise ContractError("effect-receipt.v1 manifest digests are invalid")
    if receipt["coverage"] not in {"COMPLETE", "INCOMPLETE"}:
        raise ContractError("effect-receipt.v1 coverage is invalid")
    changes = receipt["changes"]
    if not isinstance(changes, list):
        raise ContractError("effect-receipt.v1 changes must be an array")
    changed_paths: list[str] = []
    for position, change in enumerate(changes):
        if not isinstance(change, dict):
            raise ContractError(f"changes[{position}] must be an object")
        require_exact_keys(
            change,
            required=("path", "change_kind", "before_digest", "after_digest"),
            label=f"changes[{position}]",
        )
        path = normalize_rel(change["path"])
        changed_paths.append(path)
        if change["change_kind"] not in {
            "ADDED",
            "MODIFIED",
            "DELETED",
            "MODE_CHANGED",
            "TYPE_CHANGED",
        }:
            raise ContractError(f"changes[{position}].change_kind is invalid")
        for field in ("before_digest", "after_digest"):
            if change[field] is not None and not valid_digest(change[field]):
                raise ContractError(f"changes[{position}].{field} is invalid")
    if changed_paths != sorted(set(changed_paths)):
        raise ContractError("effect-receipt.v1 changes must be sorted and unique")
    if receipt["actual_changed_paths"] != changed_paths:
        raise ContractError("effect-receipt.v1 actual_changed_paths do not match changes")
    undeclared = receipt["undeclared_paths"]
    if (
        not isinstance(undeclared, list)
        or undeclared != sorted(set(undeclared))
        or not set(undeclared).issubset(changed_paths)
    ):
        raise ContractError("effect-receipt.v1 undeclared_paths are not actual changes")
    references = receipt["check_receipt_refs"]
    if not isinstance(references, list):
        raise ContractError("effect-receipt.v1 check_receipt_refs must be an array")
    reference_names: list[str] = []
    for position, reference in enumerate(references):
        if not isinstance(reference, dict):
            raise ContractError(f"check_receipt_refs[{position}] must be an object")
        require_exact_keys(
            reference,
            required=("ref", "digest"),
            label=f"check_receipt_refs[{position}]",
        )
        if (
            normalize_rel(reference["ref"]) != reference["ref"]
            or not valid_digest(reference["digest"])
        ):
            raise ContractError(f"check_receipt_refs[{position}] is invalid")
        reference_names.append(reference["ref"])
    if reference_names != sorted(set(reference_names)):
        raise ContractError("effect-receipt.v1 check_receipt_refs are not canonical")
    verify_artifact_digest(receipt, "effect-receipt.v1")


def build_check_receipt(
    receipt_id: str,
    command: list[str],
    exit_code: int,
    subject_manifest_digest: str,
    stdout: bytes,
    stderr: bytes,
    observed_at: str,
) -> dict[str, Any]:
    require_id(receipt_id, "check receipt ID")
    if not isinstance(command, list) or not command or any(
        not isinstance(item, str) for item in command
    ):
        raise ContractError("check receipt command must be a nonempty string array")
    if not isinstance(exit_code, int) or not valid_digest(subject_manifest_digest):
        raise ContractError("check receipt runtime facts are invalid")
    require_datetime(observed_at, "check receipt observed_at")
    result = "PASS" if exit_code == 0 else "FAIL"
    return finalize_artifact(
        {
            "schema_version": "check-receipt.v1",
            "receipt_id": receipt_id,
            "command": command,
            "result": result,
            "exit_code": exit_code,
            "subject_manifest_digest": subject_manifest_digest,
            "stdout_digest": sha256(stdout),
            "stderr_digest": sha256(stderr),
            "observed_at": observed_at,
        }
    )


def validate_check_receipt(receipt: dict[str, Any]) -> None:
    require_exact_keys(
        receipt,
        required=(
            "schema_version",
            "receipt_id",
            "command",
            "result",
            "exit_code",
            "subject_manifest_digest",
            "stdout_digest",
            "stderr_digest",
            "observed_at",
            "artifact_digest",
        ),
        label="check-receipt.v1",
    )
    if receipt["schema_version"] != "check-receipt.v1":
        raise ContractError("check-receipt.v1 schema_version is invalid")
    require_id(receipt["receipt_id"], "check-receipt.v1 receipt_id")
    command = receipt["command"]
    if not isinstance(command, list) or not command or any(
        not isinstance(item, str) for item in command
    ):
        raise ContractError("check-receipt.v1 command is invalid")
    if receipt["result"] not in {"PASS", "FAIL", "ERROR"} or not isinstance(
        receipt["exit_code"], int
    ):
        raise ContractError("check-receipt.v1 result is invalid")
    if receipt["result"] == "PASS" and receipt["exit_code"] != 0:
        raise ContractError("check-receipt.v1 PASS requires exit_code 0")
    if receipt["result"] == "FAIL" and receipt["exit_code"] == 0:
        raise ContractError("check-receipt.v1 FAIL requires nonzero exit_code")
    for field in ("subject_manifest_digest", "stdout_digest", "stderr_digest"):
        if not valid_digest(receipt[field]):
            raise ContractError(f"check-receipt.v1 {field} is invalid")
    require_datetime(receipt["observed_at"], "check-receipt.v1 observed_at")
    verify_artifact_digest(receipt, "check-receipt.v1")


def schema_digests(repository: Path) -> dict[str, str]:
    result: dict[str, str] = {}
    for role, reference in SCHEMAS.items():
        result[role] = sha256((repository / reference).read_bytes())
    return result


def resolve_repository_ref(repository: Path, reference: str) -> Path:
    relative = normalize_rel(reference)
    repository = repository.resolve()
    candidate = repository / relative
    cursor = repository
    for segment in relative.split("/"):
        cursor = cursor / segment
        if cursor.is_symlink():
            raise ContractError(f"repository reference traverses a symlink: {reference}")
    try:
        candidate.resolve(strict=False).relative_to(repository)
    except ValueError as exc:
        raise ContractError(f"repository reference escapes root: {reference}") from exc
    return candidate


def tree_digest(root: Path) -> str:
    if not root.is_dir():
        raise ContractError(f"qualification corpus is not a directory: {root}")
    entries: list[dict[str, Any]] = []
    for path in sorted(root.rglob("*")):
        if path.is_dir() and not path.is_symlink():
            continue
        relative = path.relative_to(root).as_posix()
        normalize_rel(relative)
        metadata = path.lstat()
        executable = bool(
            metadata.st_mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
        )
        if path.is_symlink():
            kind = "symlink"
            content = os.readlink(path).encode("utf-8")
        elif path.is_file():
            kind = "file"
            content = path.read_bytes()
        else:
            raise ContractError(f"unsupported qualification artifact: {path}")
        entries.append(
            {
                "path": relative,
                "kind": kind,
                "executable": executable,
                "sha256": sha256(content),
            }
        )
    if not entries:
        raise ContractError("qualification corpus must contain at least one file")
    return digest_value(entries)


def _mode(path: Path) -> str:
    return f"{path.lstat().st_mode & 0o777:04o}"


def _validate_legacy_qualification_verdict(
    verdict: dict[str, Any],
    expected_digest: str,
) -> None:
    required = {
        "schema_version",
        "acceptance_digest",
        "subject_manifest_digest",
        "author_context_id",
        "validator_context_id",
        "freshness_attestation",
        "verdict",
        "criteria",
        "findings",
        "evidence_refs",
        "checked",
        "not_checked",
        "validated_at",
        "artifact_digest",
    }
    if set(verdict) != required or verdict.get("schema_version") != "verdict.v2":
        raise ContractError("qualification verdict is not verdict.v2 or verdict.v3")
    claimed = verdict.get("artifact_digest")
    if (
        claimed != expected_digest
        or not valid_digest(claimed)
        or digest_value(artifact_identity(verdict)) != claimed
    ):
        raise ContractError("qualification verdict digest is invalid")
    if verdict.get("verdict") != "PASS":
        raise ContractError("qualification verdict did not PASS")
    author = verdict.get("author_context_id")
    validator = verdict.get("validator_context_id")
    freshness = verdict.get("freshness_attestation")
    if (
        not isinstance(author, str)
        or not author
        or not isinstance(validator, str)
        or not validator
        or author == validator
        or not isinstance(freshness, dict)
        or freshness.get("source") not in {"runtime", "caller"}
        or not isinstance(freshness.get("attester_identity"), str)
        or not freshness["attester_identity"]
    ):
        raise ContractError(
            "qualification verdict lacks distinct identities or freshness"
        )
    if not valid_digest(verdict["acceptance_digest"]) or not valid_digest(
        verdict["subject_manifest_digest"]
    ):
        raise ContractError("qualification verdict subject or intent digest is invalid")
    criteria = verdict["criteria"]
    if (
        not isinstance(criteria, list)
        or not criteria
        or any(
            not isinstance(item, dict)
            or set(item) - {"id", "result", "evidence_refs", "reason"}
            or not {"id", "result", "evidence_refs"}.issubset(item)
            or not isinstance(item["id"], str)
            or not item["id"]
            or item["result"] != "PASS"
            or not isinstance(item["evidence_refs"], list)
            or not item["evidence_refs"]
            or any(
                not isinstance(reference, str) or not reference
                for reference in item["evidence_refs"]
            )
            for item in criteria
        )
    ):
        raise ContractError("qualification verdict criteria are not evidence-backed PASS")
    criterion_ids = [item["id"] for item in criteria]
    if len(criterion_ids) != len(set(criterion_ids)):
        raise ContractError("qualification verdict criterion IDs are duplicated")
    if (
        not isinstance(verdict["evidence_refs"], list)
        or not verdict["evidence_refs"]
        or not isinstance(verdict["checked"], list)
        or not verdict["checked"]
        or verdict["not_checked"] != []
    ):
        raise ContractError("qualification verdict lacks complete checked evidence")
    require_datetime(verdict["validated_at"], "qualification verdict validated_at")


def _manifest_digest(manifest: dict[str, Any]) -> str:
    if manifest.get("schema_version") == "subject-manifest.v2":
        validate_manifest_v2(manifest)
        return manifest["canonical_manifest_digest"]
    if manifest.get("schema_version") != "subject-manifest.v1":
        raise ContractError("qualification subject manifest version is invalid")
    claimed = manifest.get("canonical_manifest_digest")
    identity = {
        key: value
        for key, value in manifest.items()
        if key not in {"canonical_manifest_digest", "git_metadata"}
    }
    if not valid_digest(claimed) or digest_value(identity) != claimed:
        raise ContractError("qualification subject manifest digest is invalid")
    return claimed


def load_active_proof(repository: Path) -> dict[str, Any]:
    repository = repository.resolve()
    active_path = resolve_repository_ref(repository, PROOF_ACTIVE_REF)
    active = load_json(active_path, "active proof identity")
    require_exact_keys(
        active,
        required=(
            "schema_version",
            "epoch",
            "contract_ref",
            "contract_digest",
            "activation_transition_ref",
            "activation_transition_digest",
        ),
        label="proof-contract-active.v1",
    )
    if active["schema_version"] != "proof-contract-active.v1":
        raise ContractError("active proof schema_version is invalid")
    contract_ref = normalize_rel(active["contract_ref"])
    if contract_ref != active["contract_ref"]:
        raise ContractError("active proof contract_ref is not canonical")
    contract_path = resolve_repository_ref(repository, contract_ref)
    if sha256(contract_path.read_bytes()) != active["contract_digest"]:
        raise TerminalValidation("active proof contract bytes do not match active identity")
    contract = load_json(contract_path, "active proof contract")
    validate_proof_contract_v1(contract)
    if contract["epoch"] != active["epoch"]:
        raise TerminalValidation("active proof epoch differs from its descriptor")
    for component in contract["components"]:
        component_path = resolve_repository_ref(repository, component["ref"])
        if (
            sha256(component_path.read_bytes()) != component["digest"]
            or _mode(component_path) != component["mode"]
        ):
            raise TerminalValidation(
                f"active proof component bytes or mode changed: {component['ref']}"
            )
    recorder = contract["transition_recorder"]
    recorder_path = resolve_repository_ref(repository, recorder["ref"])
    if (
        sha256(recorder_path.read_bytes()) != recorder["digest"]
        or _mode(recorder_path) != recorder["mode"]
    ):
        raise TerminalValidation("active proof transition recorder bytes or mode changed")
    corpus = contract["qualification_corpus"]
    corpus_path = resolve_repository_ref(repository, corpus["ref"])
    if tree_digest(corpus_path) != corpus["digest"]:
        raise TerminalValidation("active proof qualification corpus changed")
    if active["activation_transition_ref"] is None:
        if active["activation_transition_digest"] is not None:
            raise ContractError("active proof transition digest has no reference")
        if active["epoch"] != 0:
            raise TerminalValidation("non-bootstrap proof epoch lacks activation transition")
    else:
        transition_ref = normalize_rel(active["activation_transition_ref"])
        if transition_ref != active["activation_transition_ref"]:
            raise ContractError("active proof transition ref is not canonical")
        transition_path = resolve_repository_ref(repository, transition_ref)
        if (
            sha256(transition_path.read_bytes())
            != active["activation_transition_digest"]
        ):
            raise TerminalValidation("active proof transition bytes do not match identity")
        transition = load_json(transition_path, "active proof transition")
        validate_proof_transition_v1(transition)
        prior = transition["prior"]
        prior_contract_path = resolve_repository_ref(
            repository,
            prior["contract_ref"],
        )
        if sha256(prior_contract_path.read_bytes()) != prior["contract_digest"]:
            raise TerminalValidation(
                "active proof transition prior contract binding changed"
            )
        prior_contract = load_json(prior_contract_path, "prior proof contract")
        validate_proof_contract_v1(prior_contract)
        if prior_contract["epoch"] != prior["epoch"]:
            raise TerminalValidation("active proof transition prior epoch is invalid")
        candidate = transition["candidate"]
        if (
            candidate["epoch"] != active["epoch"]
            or candidate["contract_ref"] != active["contract_ref"]
            or candidate["contract_digest"] != active["contract_digest"]
            or candidate["qualification_corpus_ref"] != corpus["ref"]
            or candidate["qualification_corpus_digest"] != corpus["digest"]
            or candidate["subject_manifest_digest"]
            != contract["qualification_subject_manifest_digest"]
        ):
            raise TerminalValidation(
                "active proof transition does not bind the active descriptor"
            )
        subject_path = resolve_repository_ref(
            repository,
            candidate["subject_manifest_ref"],
        )
        if _manifest_digest(load_json(subject_path)) != candidate["subject_manifest_digest"]:
            raise TerminalValidation("active proof qualification subject binding changed")
        verdict_binding = transition["qualification_verdict"]
        verdict_path = resolve_repository_ref(repository, verdict_binding["ref"])
        verdict = load_json(verdict_path, "active proof qualification verdict")
        if verdict_path.name != f"{verdict_binding['digest']}.json":
            raise TerminalValidation("qualification verdict filename is unbound")
        if verdict.get("schema_version") == "verdict.v3":
            validate_verdict_v3(verdict)
            if (
                verdict["artifact_digest"] != verdict_binding["digest"]
                or verdict["verdict"] != "PASS"
            ):
                raise TerminalValidation("qualification verdict binding is invalid")
            expected_proof = {
                "epoch": prior["epoch"],
                "contract_ref": prior["contract_ref"],
                "contract_digest": prior["contract_digest"],
                "activation_transition_digest": prior[
                    "activation_transition_digest"
                ],
            }
            if verdict["proof_identity"] != expected_proof:
                raise TerminalValidation(
                    "qualification verdict was not issued under prior proof identity"
                )
        else:
            if prior["epoch"] != 0:
                raise TerminalValidation(
                    "legacy verdict.v2 can qualify only the bootstrap transition"
                )
            _validate_legacy_qualification_verdict(
                verdict,
                verdict_binding["digest"],
            )
        judged_subject = verdict.get("subject_manifest_digest")
        if judged_subject is None:
            judged_subject = verdict.get("final_manifest_digest")
        if judged_subject != candidate["subject_manifest_digest"]:
            raise TerminalValidation("qualification verdict judged a different subject")
    return {
        "epoch": active["epoch"],
        "contract_ref": contract_ref,
        "contract_digest": active["contract_digest"],
        "activation_transition_digest": active["activation_transition_digest"],
    }


def _validate_ref_digest(
    repository: Path,
    reference: str,
    digest: str,
    label: str,
) -> None:
    relative = normalize_rel(reference)
    if relative != reference or not valid_digest(digest):
        raise ContractError(f"{label} reference or digest is invalid")
    if sha256((repository / relative).read_bytes()) != digest:
        raise TerminalValidation(f"{label} bytes do not match the bound digest")


def _required_criterion_ids(scope_index: dict[str, Any]) -> set[str]:
    return {
        item["id"]
        for item in scope_index["criteria"]
        if item["required"]
    }


def validate_verdict_v3(
    artifact: dict[str, Any],
    *,
    scope_index: dict[str, Any] | None = None,
) -> None:
    required = (
        "schema_version",
        "invocation_id",
        "judgment_id",
        "intent_ref",
        "intent_digest",
        "proof_identity",
        "schema_digests",
        "before_manifest_ref",
        "before_manifest_digest",
        "final_manifest_ref",
        "final_manifest_digest",
        "scope_index_ref",
        "scope_index_digest",
        "effect_receipt_ref",
        "effect_receipt_digest",
        "author_context_id",
        "validator_context_id",
        "freshness_attestation",
        "verdict",
        "criteria",
        "findings",
        "checked",
        "not_checked",
        "validated_at",
        "artifact_digest",
    )
    require_exact_keys(artifact, required=required, label="verdict.v3")
    if artifact["schema_version"] != "verdict.v3":
        raise ContractError("verdict.v3 schema_version is invalid")
    require_id(artifact["invocation_id"], "verdict.v3 invocation_id")
    require_id(artifact["judgment_id"], "verdict.v3 judgment_id")
    if artifact["invocation_id"] == artifact["judgment_id"]:
        raise ContractError("invocation and judgment identities must be distinct")
    for field in (
        "intent_digest",
        "before_manifest_digest",
        "final_manifest_digest",
        "scope_index_digest",
        "effect_receipt_digest",
    ):
        if not valid_digest(artifact[field]):
            raise ContractError(f"verdict.v3 {field} is invalid")
    for field in (
        "intent_ref",
        "before_manifest_ref",
        "final_manifest_ref",
        "scope_index_ref",
        "effect_receipt_ref",
    ):
        if normalize_rel(artifact[field]) != artifact[field]:
            raise ContractError(f"verdict.v3 {field} is not canonical")
    proof = artifact["proof_identity"]
    if not isinstance(proof, dict):
        raise ContractError("verdict.v3 proof_identity must be an object")
    require_exact_keys(
        proof,
        required=(
            "epoch",
            "contract_ref",
            "contract_digest",
            "activation_transition_digest",
        ),
        label="verdict.v3 proof_identity",
    )
    if (
        not isinstance(proof["epoch"], int)
        or proof["epoch"] < 0
        or normalize_rel(proof["contract_ref"]) != proof["contract_ref"]
        or not valid_digest(proof["contract_digest"])
        or (
            proof["activation_transition_digest"] is not None
            and not valid_digest(proof["activation_transition_digest"])
        )
    ):
        raise ContractError("verdict.v3 proof_identity is invalid")
    schemas = artifact["schema_digests"]
    if not isinstance(schemas, dict) or set(schemas) != set(SCHEMAS):
        raise ContractError("verdict.v3 schema_digests has invalid fields")
    if any(not valid_digest(value) for value in schemas.values()):
        raise ContractError("verdict.v3 schema_digests is invalid")
    for field in ("author_context_id", "validator_context_id"):
        require_id(artifact[field], f"verdict.v3 {field}")
    if artifact["author_context_id"] == artifact["validator_context_id"]:
        raise ContractError("verdict.v3 author and validator contexts collide")
    freshness = artifact["freshness_attestation"]
    if not isinstance(freshness, dict):
        raise ContractError("verdict.v3 freshness_attestation must be an object")
    require_exact_keys(
        freshness,
        required=("source", "attester_identity"),
        label="verdict.v3 freshness_attestation",
    )
    if freshness["source"] not in {"runtime", "caller"}:
        raise ContractError("verdict.v3 freshness source is invalid")
    require_id(
        freshness["attester_identity"],
        "verdict.v3 freshness attester_identity",
    )
    if artifact["verdict"] not in {"PASS", "FAIL", "NOT_PROVEN"}:
        raise ContractError("verdict.v3 verdict is invalid")
    criteria = artifact["criteria"]
    if not isinstance(criteria, list) or not criteria:
        raise ContractError("verdict.v3 criteria must be nonempty")
    criterion_ids: list[str] = []
    for position, criterion in enumerate(criteria):
        if not isinstance(criterion, dict):
            raise ContractError(f"verdict.v3 criteria[{position}] must be an object")
        require_exact_keys(
            criterion,
            required=("id", "result", "evidence_receipt_digests", "reason"),
            label=f"verdict.v3 criteria[{position}]",
        )
        identifier = require_id(criterion["id"], f"verdict.v3 criteria[{position}].id")
        criterion_ids.append(identifier)
        if criterion["result"] not in {"PASS", "FAIL", "NOT_PROVEN", "EXCLUDED"}:
            raise ContractError(f"verdict.v3 criteria[{position}].result is invalid")
        evidence = criterion["evidence_receipt_digests"]
        if (
            not isinstance(evidence, list)
            or len(evidence) != len(set(evidence))
            or any(not valid_digest(item) for item in evidence)
        ):
            raise ContractError(
                f"verdict.v3 criteria[{position}].evidence_receipt_digests is invalid"
            )
        if not isinstance(criterion["reason"], str):
            raise ContractError(f"verdict.v3 criteria[{position}].reason is invalid")
    if len(criterion_ids) != len(set(criterion_ids)):
        raise ContractError("verdict.v3 criterion IDs must be unique")
    if scope_index is not None:
        validate_scope_index(scope_index)
        frozen_ids = {item["id"] for item in scope_index["criteria"]}
        if set(criterion_ids) != frozen_ids:
            raise ContractError("verdict.v3 criteria do not equal the frozen criterion IDs")
        exclusions = {
            identifier
            for exclusion in scope_index["declared_exclusions"]
            for identifier in exclusion["criterion_ids"]
        }
        by_id = {item["id"]: item for item in criteria}
        if {
            identifier
            for identifier, item in by_id.items()
            if item["result"] == "EXCLUDED"
        } != exclusions:
            raise ContractError("verdict.v3 exclusions differ from the frozen scope index")
        for identifier in _required_criterion_ids(scope_index):
            if by_id[identifier]["result"] == "EXCLUDED":
                raise ContractError("required criteria cannot be excluded")
    if artifact["verdict"] == "PASS":
        excluded = {
            item["id"] for item in criteria if item["result"] == "EXCLUDED"
        }
        for criterion in criteria:
            if criterion["id"] in excluded:
                continue
            if criterion["result"] != "PASS" or not criterion[
                "evidence_receipt_digests"
            ]:
                raise ContractError(
                    "verdict.v3 PASS requires evidence-backed PASS for every "
                    "non-excluded criterion"
                )
        if artifact["not_checked"]:
            raise ContractError("verdict.v3 PASS cannot contain not_checked items")
        if not artifact["checked"]:
            raise ContractError("verdict.v3 PASS requires nonempty checked evidence")
    if not isinstance(artifact["findings"], list):
        raise ContractError("verdict.v3 findings must be an array")
    finding_ids: set[str] = set()
    for position, finding in enumerate(artifact["findings"]):
        if not isinstance(finding, dict):
            raise ContractError(f"verdict.v3 findings[{position}] must be an object")
        require_exact_keys(
            finding,
            required=("id", "summary", "evidence_receipt_digests"),
            label=f"verdict.v3 findings[{position}]",
        )
        identifier = require_id(
            finding["id"],
            f"verdict.v3 findings[{position}].id",
        )
        if identifier in finding_ids:
            raise ContractError("verdict.v3 finding IDs must be unique")
        finding_ids.add(identifier)
        if not isinstance(finding["summary"], str) or not finding["summary"]:
            raise ContractError(f"verdict.v3 findings[{position}].summary is invalid")
        evidence = finding["evidence_receipt_digests"]
        if (
            not isinstance(evidence, list)
            or not evidence
            or len(evidence) != len(set(evidence))
            or any(not valid_digest(item) for item in evidence)
        ):
            raise ContractError(
                f"verdict.v3 findings[{position}].evidence_receipt_digests is invalid"
            )
    for field in ("checked", "not_checked"):
        if not isinstance(artifact[field], list) or any(
            not isinstance(item, str) or not item for item in artifact[field]
        ):
            raise ContractError(f"verdict.v3 {field} is invalid")
    require_datetime(artifact["validated_at"], "verdict.v3 validated_at")
    verify_artifact_digest(artifact, "verdict.v3")


def _find_duplicate_judgment(
    destination: Path,
    artifact: dict[str, Any],
) -> Path | None:
    if not destination.exists():
        return None
    for path in sorted(destination.glob("*.json")):
        existing = load_json(path, f"stored verdict {path.name}")
        if existing.get("schema_version") != "verdict.v3":
            continue
        validate_verdict_v3(existing)
        same_invocation = existing["invocation_id"] == artifact["invocation_id"]
        same_judgment = existing["judgment_id"] == artifact["judgment_id"]
        same_subject = (
            existing["intent_digest"] == artifact["intent_digest"]
            and existing["final_manifest_digest"] == artifact["final_manifest_digest"]
        )
        if same_invocation or same_judgment or same_subject:
            if canonical_bytes(existing) == canonical_bytes(artifact):
                return path
            raise TerminalValidation(
                "duplicate unlinked judgment for one invocation or intent/subject"
            )
    return None


def atomic_store_artifact(
    artifact: dict[str, Any],
    destination: Path,
    *,
    prefix: str,
) -> tuple[Path, bool]:
    destination.mkdir(parents=True, exist_ok=True)
    payload = canonical_bytes(artifact) + b"\n"
    target = destination / f"{artifact['artifact_digest']}.json"
    if target.exists():
        if target.read_bytes() == payload:
            return target, True
        raise ContractError(f"{prefix} integrity collision at {target}")
    file_descriptor, temporary = tempfile.mkstemp(
        prefix=f".{prefix}-",
        suffix=".tmp",
        dir=destination,
    )
    try:
        with os.fdopen(file_descriptor, "wb") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, target)
        directory_descriptor = os.open(destination, os.O_RDONLY)
        try:
            os.fsync(directory_descriptor)
        finally:
            os.close(directory_descriptor)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)
    return target, False


def store_verdict_v3(
    draft: dict[str, Any],
    *,
    repository: Path,
    destination: Path,
    invocation_id: str,
    judgment_id: str,
    intent_ref: str,
    expected_intent_digest: str,
    before_manifest_ref: str,
    final_manifest_ref: str,
    scope_index_ref: str,
    effect_receipt_ref: str,
    author_context_id: str,
    validator_context_id: str,
    freshness_source: str,
    freshness_attester_id: str,
) -> tuple[dict[str, Any], Path, bool]:
    repository = repository.resolve()
    intent_path = resolve_repository_ref(repository, intent_ref)
    consume_intent_snapshot(intent_path, expected_intent_digest)
    before = load_json(resolve_repository_ref(repository, before_manifest_ref))
    final = load_json(resolve_repository_ref(repository, final_manifest_ref))
    scope_index = load_json(resolve_repository_ref(repository, scope_index_ref))
    effect = load_json(resolve_repository_ref(repository, effect_receipt_ref))
    validate_manifest_v2(before)
    validate_manifest_v2(final)
    verify_manifest_v2(final, repository)
    validate_scope_index(scope_index)
    validate_effect_receipt(effect)
    expected_effect = derive_effect_receipt(
        before,
        final,
        scope_index,
        effect["check_receipt_refs"],
    )
    if canonical_bytes(expected_effect) != canonical_bytes(effect):
        raise TerminalValidation(
            "effect receipt does not equal runtime-derived before/final changes"
        )
    check_receipt_digests: set[str] = set()
    for reference in effect["check_receipt_refs"]:
        receipt = load_json(
            resolve_repository_ref(repository, reference["ref"]),
            f"check receipt {reference['ref']}",
        )
        validate_check_receipt(receipt)
        if receipt["artifact_digest"] != reference["digest"]:
            raise TerminalValidation(
                "effect receipt check reference does not bind the typed receipt"
            )
        if receipt["subject_manifest_digest"] != final["canonical_manifest_digest"]:
            raise TerminalValidation(
                "check receipt is bound to a different final subject"
            )
        check_receipt_digests.add(receipt["artifact_digest"])
    if scope_index["intent_digest"] != expected_intent_digest:
        raise TerminalValidation("scope index is bound to a different intent")
    if effect["before_manifest_digest"] != before["canonical_manifest_digest"]:
        raise TerminalValidation("effect receipt is bound to a different before manifest")
    if effect["final_manifest_digest"] != final["canonical_manifest_digest"]:
        raise TerminalValidation("effect receipt is bound to a different final manifest")
    if effect["coverage"] != "COMPLETE":
        draft = dict(draft)
        draft["verdict"] = "NOT_PROVEN"
        draft["findings"] = list(draft.get("findings") or []) + [
            {
                "id": "validate.incomplete-coverage",
                "summary": "repository-wide changed-path coverage is incomplete",
                "evidence_receipt_digests": [effect["artifact_digest"]],
            }
        ]
        draft["not_checked"] = list(draft.get("not_checked") or []) + [
            "repository-wide changed-path coverage"
        ]
    elif effect["undeclared_paths"]:
        draft = dict(draft)
        draft["verdict"] = "FAIL"
        draft["findings"] = list(draft.get("findings") or []) + [
            {
                "id": "validate.out-of-scope",
                "summary": "repository changes fall outside frozen scope classes",
                "evidence_receipt_digests": [effect["artifact_digest"]],
            }
        ]
    if PROOF_ACTIVE_REF in effect["actual_changed_paths"]:
        raise TerminalValidation("a candidate proof contract cannot activate itself")
    proof_identity = load_active_proof(repository)
    runtime_fields = {
        "schema_version": "verdict.v3",
        "invocation_id": invocation_id,
        "judgment_id": judgment_id,
        "intent_ref": normalize_rel(intent_ref),
        "intent_digest": expected_intent_digest,
        "proof_identity": proof_identity,
        "schema_digests": schema_digests(repository),
        "before_manifest_ref": normalize_rel(before_manifest_ref),
        "before_manifest_digest": before["canonical_manifest_digest"],
        "final_manifest_ref": normalize_rel(final_manifest_ref),
        "final_manifest_digest": final["canonical_manifest_digest"],
        "scope_index_ref": normalize_rel(scope_index_ref),
        "scope_index_digest": scope_index["artifact_digest"],
        "effect_receipt_ref": normalize_rel(effect_receipt_ref),
        "effect_receipt_digest": effect["artifact_digest"],
        "author_context_id": author_context_id,
        "validator_context_id": validator_context_id,
        "freshness_attestation": {
            "source": freshness_source,
            "attester_identity": freshness_attester_id,
        },
    }
    semantic_keys = {"verdict", "criteria", "findings", "checked", "not_checked", "validated_at"}
    if set(draft) != semantic_keys:
        raise ContractError(
            "verdict.v3 draft fields must be exactly: "
            + ", ".join(sorted(semantic_keys))
        )
    claimed_criterion_evidence = {
        digest
        for criterion in draft["criteria"]
        for digest in criterion.get("evidence_receipt_digests", [])
    }
    if not claimed_criterion_evidence.issubset(check_receipt_digests):
        raise TerminalValidation(
            "criterion evidence is not backed by supplied typed check receipts"
        )
    artifact = finalize_artifact({**runtime_fields, **draft})
    validate_verdict_v3(artifact, scope_index=scope_index)
    duplicate = _find_duplicate_judgment(destination, artifact)
    verify_manifest_v2(final, repository)
    if duplicate is not None:
        return artifact, duplicate, True
    path, existed = atomic_store_artifact(
        artifact,
        destination,
        prefix="verdict-v3",
    )
    return artifact, path, existed


def build_rpi_report_v2(
    *,
    invocation_id: str,
    correlation: dict[str, str] | None,
    status: str,
    intent_ref: str | None,
    intent_digest: str | None,
    proof_identity: dict[str, Any] | None,
    before_manifest_digest: str | None,
    final_manifest_digest: str | None,
    effect_receipt_digest: str | None,
    verdict_ref: str | None,
    verdict_digest: str | None,
    checked: list[str],
    not_checked: list[str],
) -> dict[str, Any]:
    require_id(invocation_id, "rpi-report.v2 invocation_id")
    if status not in {"PASS", "FAIL", "NOT_PROVEN", "NOT_PLANNED", "NOT_BUILT"}:
        raise ContractError("rpi-report.v2 status is invalid")
    report = {
        "schema_version": "rpi-report.v2",
        "invocation_id": invocation_id,
        "correlation": correlation,
        "status": status,
        "intent_ref": intent_ref,
        "intent_digest": intent_digest,
        "proof_identity": proof_identity,
        "before_manifest_digest": before_manifest_digest,
        "final_manifest_digest": final_manifest_digest,
        "effect_receipt_digest": effect_receipt_digest,
        "verdict_ref": verdict_ref,
        "verdict_digest": verdict_digest,
        "checked": checked,
        "not_checked": not_checked,
    }
    artifact = finalize_artifact(report)
    validate_rpi_report_v2(artifact)
    return artifact


def validate_rpi_report_v2(report: dict[str, Any]) -> None:
    require_exact_keys(
        report,
        required=(
            "schema_version",
            "invocation_id",
            "correlation",
            "status",
            "intent_ref",
            "intent_digest",
            "proof_identity",
            "before_manifest_digest",
            "final_manifest_digest",
            "effect_receipt_digest",
            "verdict_ref",
            "verdict_digest",
            "checked",
            "not_checked",
            "artifact_digest",
        ),
        label="rpi-report.v2",
    )
    if report["schema_version"] != "rpi-report.v2":
        raise ContractError("rpi-report.v2 schema_version is invalid")
    require_id(report["invocation_id"], "rpi-report.v2 invocation_id")
    correlation = report["correlation"]
    if correlation is not None:
        if not isinstance(correlation, dict) or len(correlation) > 8:
            raise ContractError("rpi-report.v2 correlation is not a bounded object")
        for key, value in correlation.items():
            require_id(key, "rpi-report.v2 correlation key")
            if (
                len(key) > 64
                or not isinstance(value, str)
                or len(value) > 256
            ):
                raise ContractError("rpi-report.v2 correlation entry exceeds bounds")
        if len(canonical_bytes(correlation)) > 2048:
            raise ContractError("rpi-report.v2 correlation exceeds byte bound")
    status = report["status"]
    if status not in {"PASS", "FAIL", "NOT_PROVEN", "NOT_PLANNED", "NOT_BUILT"}:
        raise ContractError("rpi-report.v2 status is invalid")
    nullable_digests = (
        "intent_digest",
        "before_manifest_digest",
        "final_manifest_digest",
        "effect_receipt_digest",
        "verdict_digest",
    )
    if any(
        report[field] is not None and not valid_digest(report[field])
        for field in nullable_digests
    ):
        raise ContractError("rpi-report.v2 contains an invalid digest")
    for field in ("intent_ref", "verdict_ref"):
        if report[field] is not None and normalize_rel(report[field]) != report[field]:
            raise ContractError(f"rpi-report.v2 {field} is not canonical")
    for field in ("checked", "not_checked"):
        if not isinstance(report[field], list) or any(
            not isinstance(item, str) or not item for item in report[field]
        ):
            raise ContractError(f"rpi-report.v2 {field} is invalid")
    semantic = status in {"PASS", "FAIL", "NOT_PROVEN"}
    semantic_fields = (
        "intent_ref",
        "intent_digest",
        "proof_identity",
        "before_manifest_digest",
        "final_manifest_digest",
        "effect_receipt_digest",
        "verdict_ref",
        "verdict_digest",
    )
    if semantic and any(report[field] is None for field in semantic_fields):
        raise ContractError("semantic rpi-report.v2 requires complete durable bindings")
    if not semantic and any(
        report[field] is not None
        for field in (
            "proof_identity",
            "before_manifest_digest",
            "final_manifest_digest",
            "effect_receipt_digest",
            "verdict_ref",
            "verdict_digest",
        )
    ):
        raise ContractError("report-only rpi-report.v2 cannot claim subject or verdict proof")
    if report["proof_identity"] is not None:
        proof = report["proof_identity"]
        if not isinstance(proof, dict):
            raise ContractError("rpi-report.v2 proof_identity must be an object")
        require_exact_keys(
            proof,
            required=(
                "epoch",
                "contract_ref",
                "contract_digest",
                "activation_transition_digest",
            ),
            label="rpi-report.v2 proof_identity",
        )
        if (
            not isinstance(proof["epoch"], int)
            or proof["epoch"] < 0
            or normalize_rel(proof["contract_ref"]) != proof["contract_ref"]
            or not valid_digest(proof["contract_digest"])
            or (
                proof["activation_transition_digest"] is not None
                and not valid_digest(proof["activation_transition_digest"])
            )
        ):
            raise ContractError("rpi-report.v2 proof_identity is invalid")
    verify_artifact_digest(report, "rpi-report.v2")


def store_rpi_report_v2(
    report: dict[str, Any],
    destination: Path,
) -> tuple[Path, bool]:
    validate_rpi_report_v2(report)
    return atomic_store_artifact(report, destination, prefix="rpi-report-v2")


def validate_proof_contract_v1(contract: dict[str, Any]) -> None:
    require_exact_keys(
        contract,
        required=(
            "schema_version",
            "epoch",
            "components",
            "qualification_corpus",
            "qualification_subject_manifest_digest",
            "transition_recorder",
            "known_gaps",
        ),
        label="proof-contract.v1",
    )
    if (
        contract["schema_version"] != "proof-contract.v1"
        or not isinstance(contract["epoch"], int)
        or contract["epoch"] < 0
    ):
        raise ContractError("proof-contract.v1 identity is invalid")
    components = contract["components"]
    if not isinstance(components, list) or not components:
        raise ContractError("proof-contract.v1 components must be nonempty")
    roles: set[str] = set()
    references: set[str] = set()
    for position, component in enumerate(components):
        if not isinstance(component, dict):
            raise ContractError(f"proof components[{position}] must be an object")
        require_exact_keys(
            component,
            required=("role", "ref", "digest", "mode"),
            label=f"proof components[{position}]",
        )
        role = require_id(component["role"], f"proof components[{position}].role")
        reference = normalize_rel(component["ref"])
        if role in roles or reference in references:
            raise ContractError("proof-contract.v1 component roles and refs must be unique")
        roles.add(role)
        references.add(reference)
        if reference != component["ref"] or not valid_digest(component["digest"]):
            raise ContractError(f"proof components[{position}] binding is invalid")
        if (
            not isinstance(component["mode"], str)
            or len(component["mode"]) != 4
            or any(character not in "01234567" for character in component["mode"])
        ):
            raise ContractError(f"proof components[{position}].mode is invalid")
    required_roles = (
        EPOCH_ONE_PROOF_COMPONENT_ROLES
        if contract["epoch"] >= 1
        else BASE_PROOF_COMPONENT_ROLES
    )
    missing_roles = required_roles - roles
    if missing_roles:
        raise ContractError(
            "proof-contract.v1 lacks required component roles: "
            + ", ".join(sorted(missing_roles))
        )
    corpus = contract["qualification_corpus"]
    if not isinstance(corpus, dict):
        raise ContractError("proof-contract.v1 qualification_corpus must be an object")
    require_exact_keys(
        corpus,
        required=("algorithm", "ref", "digest"),
        label="proof-contract.v1 qualification_corpus",
    )
    if (
        corpus["algorithm"] != "sha256-tree-v1"
        or normalize_rel(corpus["ref"]) != corpus["ref"]
        or not valid_digest(corpus["digest"])
    ):
        raise ContractError("proof-contract.v1 qualification_corpus is invalid")
    manifest_digest = contract["qualification_subject_manifest_digest"]
    if manifest_digest is not None and not valid_digest(manifest_digest):
        raise ContractError(
            "proof-contract.v1 qualification_subject_manifest_digest is invalid"
        )
    recorder = contract["transition_recorder"]
    if not isinstance(recorder, dict):
        raise ContractError("proof-contract.v1 transition_recorder must be an object")
    require_exact_keys(
        recorder,
        required=("ref", "digest", "mode"),
        label="proof-contract.v1 transition_recorder",
    )
    if (
        normalize_rel(recorder["ref"]) != recorder["ref"]
        or not valid_digest(recorder["digest"])
        or not isinstance(recorder["mode"], str)
        or len(recorder["mode"]) != 4
        or any(character not in "01234567" for character in recorder["mode"])
    ):
        raise ContractError("proof-contract.v1 transition_recorder is invalid")
    gaps = contract["known_gaps"]
    if not isinstance(gaps, list) or any(
        not isinstance(item, str) or not item for item in gaps
    ):
        raise ContractError("proof-contract.v1 known_gaps is invalid")


def validate_proof_transition_v1(transition: dict[str, Any]) -> None:
    require_exact_keys(
        transition,
        required=(
            "schema_version",
            "prior",
            "candidate",
            "qualification_verdict",
            "validator_identity",
            "activated_at",
        ),
        label="proof-contract-transition.v1",
    )
    if transition["schema_version"] != "proof-contract-transition.v1":
        raise ContractError("proof-contract-transition.v1 schema_version is invalid")
    prior = transition["prior"]
    candidate = transition["candidate"]
    if not isinstance(prior, dict) or not isinstance(candidate, dict):
        raise ContractError("proof transition prior and candidate must be objects")
    require_exact_keys(
        prior,
        required=(
            "epoch",
            "contract_ref",
            "contract_digest",
            "activation_transition_digest",
        ),
        label="proof transition prior",
    )
    require_exact_keys(
        candidate,
        required=(
            "epoch",
            "contract_ref",
            "contract_digest",
            "subject_manifest_ref",
            "subject_manifest_digest",
            "qualification_corpus_ref",
            "qualification_corpus_digest",
        ),
        label="proof transition candidate",
    )
    if (
        not isinstance(prior["epoch"], int)
        or not isinstance(candidate["epoch"], int)
        or candidate["epoch"] != prior["epoch"] + 1
    ):
        raise ContractError("proof transition candidate epoch must follow prior epoch")
    for label, binding, ref_fields, digest_fields in (
        (
            "prior",
            prior,
            ("contract_ref",),
            ("contract_digest",),
        ),
        (
            "candidate",
            candidate,
            (
                "contract_ref",
                "subject_manifest_ref",
                "qualification_corpus_ref",
            ),
            (
                "contract_digest",
                "subject_manifest_digest",
                "qualification_corpus_digest",
            ),
        ),
    ):
        for field in ref_fields:
            if normalize_rel(binding[field]) != binding[field]:
                raise ContractError(f"proof transition {label}.{field} is invalid")
        for field in digest_fields:
            if not valid_digest(binding[field]):
                raise ContractError(f"proof transition {label}.{field} is invalid")
    if (
        prior["activation_transition_digest"] is not None
        and not valid_digest(prior["activation_transition_digest"])
    ):
        raise ContractError("proof transition prior activation digest is invalid")
    if (
        prior["contract_ref"] == candidate["contract_ref"]
        or prior["contract_digest"] == candidate["contract_digest"]
    ):
        raise ContractError("proof transition candidate must differ from prior")
    verdict = transition["qualification_verdict"]
    if not isinstance(verdict, dict):
        raise ContractError("proof transition qualification_verdict must be an object")
    require_exact_keys(
        verdict,
        required=("ref", "digest"),
        label="proof transition qualification_verdict",
    )
    if normalize_rel(verdict["ref"]) != verdict["ref"] or not valid_digest(
        verdict["digest"]
    ):
        raise ContractError("proof transition qualification verdict is invalid")
    require_id(transition["validator_identity"], "proof transition validator_identity")
    require_datetime(transition["activated_at"], "proof transition activated_at")
