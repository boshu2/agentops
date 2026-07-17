#!/usr/bin/env python3
"""Project selected generated AgentOps skills into the Gas City role pack."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import sys
import tempfile


SCHEMA_VERSION = "agentops.gc-skill-projection.v1"
MANIFEST_REL = Path("packs/agentops-executor/assets/generated-skill-manifest.json")
PROJECTIONS = (
    (
        "implement-codex",
        Path("skills/implement"),
        Path("packs/agentops-executor/agents/implementer/skills/implement"),
    ),
    (
        "implement-claude",
        Path("skills/implement"),
        Path("packs/agentops-executor/agents/implementer-claude/skills/implement"),
    ),
    (
        "using-gc",
        Path("skills/using-gc"),
        Path("packs/agentops-executor/skills/using-gc"),
    ),
    (
        "validate-codex",
        Path("skills/validate"),
        Path("packs/agentops-executor/agents/validator/skills/validate"),
    ),
    (
        "validate-claude",
        Path("skills/validate"),
        Path("packs/agentops-executor/agents/validator-claude/skills/validate"),
    ),
)
EXCLUDED_NAMES = {
    ".DS_Store",
    ".agentops-generated.json",
    ".agentops-manifest.json",
    "prompt.md",
}
EXCLUDED_SUFFIXES = {".pyc", ".pyo"}
EXCLUDED_DIRS = {"__pycache__"}


class ProjectionError(RuntimeError):
    pass


@dataclass(frozen=True)
class ProjectedFile:
    relative: Path
    payload: bytes
    mode: int

    @property
    def sha256(self) -> str:
        return hashlib.sha256(self.payload).hexdigest()


def is_source_bookkeeping(relative: Path) -> bool:
    return (
        any(part in EXCLUDED_DIRS for part in relative.parts)
        or relative.name in EXCLUDED_NAMES
        or relative.suffix in EXCLUDED_SUFFIXES
    )


def read_tree(root: Path, *, exclude_bookkeeping: bool) -> dict[str, ProjectedFile]:
    if not root.is_dir():
        raise ProjectionError(f"missing projection directory: {root}")
    files: dict[str, ProjectedFile] = {}
    for path in sorted(root.rglob("*")):
        relative = path.relative_to(root)
        if exclude_bookkeeping and is_source_bookkeeping(relative):
            continue
        if path.is_symlink():
            raise ProjectionError(f"unsupported symlink in projection: {path}")
        if path.is_dir():
            continue
        if not path.is_file():
            raise ProjectionError(f"unsupported projection entry: {path}")
        key = relative.as_posix()
        files[key] = ProjectedFile(
            relative=relative,
            payload=path.read_bytes(),
            mode=stat.S_IMODE(path.stat().st_mode),
        )
    if not files:
        raise ProjectionError(f"projection source has no runtime files: {root}")
    return files


def manifest_bytes(
    sources: dict[str, dict[str, ProjectedFile]],
) -> bytes:
    rows: list[dict[str, str]] = []
    projection_by_skill = {skill: (source, destination) for skill, source, destination in PROJECTIONS}
    for skill, files in sources.items():
        source_root, destination_root = projection_by_skill[skill]
        for relative, projected in files.items():
            source = (source_root / relative).as_posix()
            destination = (destination_root / relative).as_posix()
            rows.append(
                {
                    "source": source,
                    "destination": destination,
                    "source_sha256": projected.sha256,
                    "sha256": projected.sha256,
                    "mode": f"{projected.mode:04o}",
                }
            )
    rows.sort(key=lambda row: row["destination"])
    manifest = {
        "schema_version": SCHEMA_VERSION,
        "source": "skills",
        "generator": "scripts/sync-gc-pack.py",
        "files": rows,
    }
    return (json.dumps(manifest, indent=2, sort_keys=False) + "\n").encode("utf-8")


def compare_tree(
    skill: str,
    expected: dict[str, ProjectedFile],
    destination: Path,
) -> list[str]:
    if not destination.is_dir():
        return [f"{skill}: missing destination {destination}"]
    try:
        actual = read_tree(destination, exclude_bookkeeping=False)
    except ProjectionError as exc:
        return [f"{skill}: {exc}"]

    errors: list[str] = []
    expected_paths = set(expected)
    actual_paths = set(actual)
    for relative in sorted(expected_paths - actual_paths):
        errors.append(f"{skill}: missing generated file {relative}")
    for relative in sorted(actual_paths - expected_paths):
        errors.append(f"{skill}: stale generated file {relative}")
    for relative in sorted(expected_paths & actual_paths):
        if actual[relative].payload != expected[relative].payload:
            errors.append(f"{skill}: content drift {relative}")
        if actual[relative].mode != expected[relative].mode:
            errors.append(f"{skill}: mode drift {relative}")
    return errors


def replace_tree(destination: Path, files: dict[str, ProjectedFile]) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    stage_root = Path(tempfile.mkdtemp(prefix=f".{destination.name}.gc-sync-", dir=destination.parent))
    stage = stage_root / destination.name
    try:
        stage.mkdir()
        for relative in sorted(files):
            projected = files[relative]
            target = stage / projected.relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(projected.payload)
            target.chmod(projected.mode)
        if destination.exists():
            shutil.rmtree(destination)
        os.replace(stage, destination)
    finally:
        shutil.rmtree(stage_root, ignore_errors=True)


def atomic_write(path: Path, payload: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary_name = tempfile.mkstemp(prefix=f".{path.name}.", suffix=".tmp", dir=path.parent)
    temporary = Path(temporary_name)
    try:
        with os.fdopen(fd, "wb") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)


def sync(repo_root: Path, *, check: bool) -> int:
    repo_root = repo_root.resolve()
    sources: dict[str, dict[str, ProjectedFile]] = {}
    try:
        for skill, source, _destination in PROJECTIONS:
            sources[skill] = read_tree(repo_root / source, exclude_bookkeeping=True)
    except (OSError, ProjectionError) as exc:
        print(f"sync-gc-pack: {exc}", file=sys.stderr)
        return 2

    expected_manifest = manifest_bytes(sources)
    if check:
        errors: list[str] = []
        for skill, _source, destination in PROJECTIONS:
            errors.extend(compare_tree(skill, sources[skill], repo_root / destination))
        manifest_path = repo_root / MANIFEST_REL
        try:
            actual_manifest = manifest_path.read_bytes()
        except FileNotFoundError:
            errors.append(f"manifest: missing {manifest_path}")
        except OSError as exc:
            errors.append(f"manifest: cannot read {manifest_path}: {exc}")
        else:
            if actual_manifest != expected_manifest:
                errors.append(f"manifest: generated content drift {manifest_path}")
        if errors:
            for error in errors:
                print(f"sync-gc-pack: {error}", file=sys.stderr)
            return 1
        print("sync-gc-pack: GC skill projection is current")
        return 0

    try:
        for skill, _source, destination in PROJECTIONS:
            replace_tree(repo_root / destination, sources[skill])
        atomic_write(repo_root / MANIFEST_REL, expected_manifest)
    except OSError as exc:
        print(f"sync-gc-pack: {exc}", file=sys.stderr)
        return 2
    print("sync-gc-pack: projected implement, using-gc, validate")
    return 0


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="report drift without writing")
    parser.add_argument(
        "--repo-root",
        type=Path,
        default=Path(__file__).resolve().parents[1],
        help=argparse.SUPPRESS,
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    return sync(args.repo_root, check=args.check)


if __name__ == "__main__":
    raise SystemExit(main())
