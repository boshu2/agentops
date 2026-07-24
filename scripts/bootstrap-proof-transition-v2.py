#!/usr/bin/env python3
"""Strict epoch-0 bootstrap recorder with candidate-component binding.

The rejected v1 recorder remains immutable historical evidence.  This wrapper
loads that frozen transition engine, replaces its descriptor validation inside
the engine's active-pointer lock, and additionally proves that every candidate
component and its future transition recorder:

* exists beneath the repository root;
* matches the declared exact bytes and mode; and
* is present with the same identity in the judged subject manifest.
"""

from __future__ import annotations

import hashlib
import importlib.util
import json
import os
from pathlib import Path
import stat
import sys
from typing import Any


BASE_RECORDER = Path(__file__).with_name("bootstrap-proof-transition.py")


def load_base():
    specification = importlib.util.spec_from_file_location(
        "agentops_bootstrap_transition_v1", BASE_RECORDER
    )
    if specification is None or specification.loader is None:
        raise RuntimeError(f"cannot load frozen base recorder: {BASE_RECORDER}")
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


def exact_artifact(path: Path) -> tuple[str, str, bool, str]:
    information = path.lstat()
    executable = bool(
        information.st_mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    )
    mode = f"{stat.S_IMODE(information.st_mode):04o}"
    if path.is_symlink():
        kind = "symlink"
        payload = os.readlink(path).encode("utf-8")
    elif path.is_file():
        kind = "file"
        payload = path.read_bytes()
    else:
        raise ValueError(f"candidate component is not a file or symlink: {path}")
    return kind, hashlib.sha256(payload).hexdigest(), executable, mode


def strict_candidate_validator(
    base: Any,
    repository: Path,
    manifest: dict[str, Any],
    original_validator: Any,
):
    claimed_manifest_digest = base.manifest_digest(manifest)
    entries = manifest.get("entries")
    if not isinstance(entries, list):
        raise base.TransitionError("qualification subject manifest entries are invalid")
    by_path: dict[str, dict[str, Any]] = {}
    for index, entry in enumerate(entries):
        if not isinstance(entry, dict) or not isinstance(entry.get("path"), str):
            raise base.TransitionError(
                f"qualification subject manifest entries[{index}] is invalid"
            )
        if entry["path"] in by_path:
            raise base.TransitionError(
                f"qualification subject manifest duplicates {entry['path']}"
            )
        by_path[entry["path"]] = entry

    def validate(value: dict[str, Any], *, expected_epoch: int) -> None:
        original_validator(value, expected_epoch=expected_epoch)
        if value["qualification_subject_manifest_digest"] != claimed_manifest_digest:
            raise base.TransitionError(
                "candidate descriptor does not bind the supplied subject manifest"
            )
        artifacts = list(value["components"])
        artifacts.append(
            {
                "role": "transition-recorder",
                **value["transition_recorder"],
            }
        )
        for artifact in artifacts:
            role = artifact["role"]
            reference = artifact["ref"]
            target = repository / reference
            normalized = base.normalize_ref(repository, target)
            if normalized != reference:
                raise base.TransitionError(
                    f"candidate component ref is not canonical: {reference}"
                )
            try:
                kind, digest, executable, mode = exact_artifact(target)
            except (OSError, ValueError) as error:
                raise base.TransitionError(
                    f"candidate component is unavailable: {role}: {error}"
                ) from error
            if digest != artifact["digest"]:
                raise base.TransitionError(
                    f"candidate component digest changed: {role}"
                )
            if mode != artifact["mode"]:
                raise base.TransitionError(
                    f"candidate component mode changed: {role}"
                )
            entry = by_path.get(reference)
            if entry is None:
                raise base.TransitionError(
                    f"candidate component is absent from judged subject: {role}"
                )
            if (
                entry.get("kind") != kind
                or entry.get("digest") != digest
                or entry.get("executable") is not executable
            ):
                raise base.TransitionError(
                    f"candidate component disagrees with judged subject: {role}"
                )

    return validate


def main() -> int:
    base = load_base()
    args = base.parse_args()
    repository = Path(args.repository).resolve()
    manifest_path = Path(args.candidate_manifest).resolve()
    try:
        manifest = base.load_object(manifest_path)
        original_validator = base.validate_descriptor
        base.validate_descriptor = strict_candidate_validator(
            base,
            repository,
            manifest,
            original_validator,
        )
        result = base.activate(args)
    except (OSError, RuntimeError, ValueError) as error:
        print(f"bootstrap-proof-transition-v2: {error}", file=sys.stderr)
        return 2
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
