#!/usr/bin/env python3
"""Pure subject identity, scope, and verdict.v2 persistence helpers.

The module intentionally has no Git, tracker, queue, network, release, or
delivery integration. It operates only on explicit files and directories.
"""

from __future__ import annotations

import argparse
import fnmatch
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import stat
import sys
import tempfile
from typing import Any, Iterable


HEX64 = set("0123456789abcdef")


class ContractError(ValueError):
    pass


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode("utf-8")


def digest_value(value: Any) -> str:
    return hashlib.sha256(canonical_bytes(value)).hexdigest()


def normalize_rel(raw: str) -> str:
    raw = raw.replace("\\", "/")
    path = PurePosixPath(raw)
    if path.is_absolute() or ".." in path.parts:
        raise ContractError(f"path escapes subject root: {raw}")
    normalized = path.as_posix()
    if normalized in ("", "."):
        return "."
    return normalized.removeprefix("./")


def path_matches(path: str, pattern: str) -> bool:
    pattern = normalize_rel(pattern)
    if pattern == ".":
        return True
    if any(ch in pattern for ch in "*?["):
        return fnmatch.fnmatchcase(path, pattern)
    return path == pattern or path.startswith(pattern.rstrip("/") + "/")


def is_excluded(path: str, exclusions: Iterable[str]) -> bool:
    return any(path_matches(path, pattern) for pattern in exclusions)


def entry_for(root: Path, rel: str) -> dict[str, Any]:
    full = root if rel == "." else root / rel
    info = full.lstat()
    executable = bool(info.st_mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH))
    if full.is_symlink():
        target = os.readlink(full).encode("utf-8")
        return {"path": rel, "kind": "symlink", "executable": executable, "digest": hashlib.sha256(target).hexdigest()}
    if full.is_file():
        return {"path": rel, "kind": "file", "executable": executable, "digest": hashlib.sha256(full.read_bytes()).hexdigest()}
    raise ContractError(f"unsupported subject kind: {rel}")


def walk_declared(root: Path, declared: str, exclusions: list[str]) -> list[dict[str, Any]]:
    full = root if declared == "." else root / declared
    if not full.exists() and not full.is_symlink():
        return []
    if full.is_file() or full.is_symlink():
        return [] if is_excluded(declared, exclusions) else [entry_for(root, declared)]
    entries: list[dict[str, Any]] = []
    for dirpath, dirnames, filenames in os.walk(full, followlinks=False):
        current = Path(dirpath)
        kept_dirs: list[str] = []
        for name in sorted(dirnames):
            child = current / name
            rel = normalize_rel(child.relative_to(root).as_posix())
            if is_excluded(rel, exclusions):
                continue
            if child.is_symlink():
                entries.append(entry_for(root, rel))
            else:
                kept_dirs.append(name)
        dirnames[:] = kept_dirs
        for name in sorted(filenames):
            rel = normalize_rel((current / name).relative_to(root).as_posix())
            if not is_excluded(rel, exclusions):
                entries.append(entry_for(root, rel))
    return entries


def load_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ContractError(f"expected JSON object: {path}")
    return value


def build_manifest(
    root: Path,
    declared_roots: list[str],
    exclusions: list[str],
    base_manifest: dict[str, Any] | None = None,
    git_metadata: dict[str, Any] | None = None,
) -> dict[str, Any]:
    root = root.resolve()
    if not root.is_dir():
        raise ContractError(f"subject root is not a directory: {root}")
    declared = sorted(set(normalize_rel(item) for item in declared_roots))
    if not declared:
        raise ContractError("at least one declared root is required")
    excluded = sorted(set(normalize_rel(item) for item in exclusions))
    by_path: dict[str, dict[str, Any]] = {}
    for item in declared:
        for entry in walk_declared(root, item, excluded):
            by_path[entry["path"]] = entry

    manifest: dict[str, Any] = {
        "schema_version": "subject-manifest.v1",
        "declared_roots": declared,
        "exclusions": excluded,
        "entries": sorted(by_path.values(), key=lambda item: item["path"]),
    }
    if base_manifest is not None:
        base_digest = base_manifest.get("canonical_manifest_digest")
        if not valid_digest(base_digest):
            raise ContractError("base manifest has no valid canonical_manifest_digest")
        manifest["base_manifest_digest"] = base_digest
        current = set(by_path)
        deletions = []
        for prior in base_manifest.get("entries", []):
            path = normalize_rel(str(prior.get("path", "")))
            declared_here = any(path_matches(path, item) for item in declared)
            if declared_here and path not in current and not is_excluded(path, excluded):
                deletions.append({"path": path, "kind": "deletion", "executable": bool(prior.get("executable", False))})
        manifest["entries"] = sorted(manifest["entries"] + deletions, key=lambda item: item["path"])
    if git_metadata:
        manifest["git_metadata"] = git_metadata
    manifest["canonical_manifest_digest"] = digest_value(manifest)
    return manifest


def valid_digest(value: Any) -> bool:
    return isinstance(value, str) and len(value) == 64 and all(ch in HEX64 for ch in value)


def verify_manifest(manifest: dict[str, Any], root: Path, base_manifest: dict[str, Any] | None) -> tuple[bool, str]:
    claimed = manifest.get("canonical_manifest_digest")
    unsigned = {key: value for key, value in manifest.items() if key != "canonical_manifest_digest"}
    if not valid_digest(claimed) or digest_value(unsigned) != claimed:
        return False, "manifest canonical digest is invalid"
    rebuilt = build_manifest(
        root,
        list(manifest.get("declared_roots", [])),
        list(manifest.get("exclusions", [])),
        base_manifest,
        manifest.get("git_metadata"),
    )
    if canonical_bytes(rebuilt) != canonical_bytes(manifest):
        return False, "subject content no longer matches manifest"
    return True, "manifest matches subject"


def plan_digest(plan: dict[str, Any]) -> str:
    return digest_value(plan)


def scope_result(plan: dict[str, Any], candidate: dict[str, Any]) -> dict[str, Any]:
    reasons: list[str] = []
    if candidate.get("plan_packet_digest") != plan_digest(plan):
        return {"result": "NOT_PROVEN", "out_of_scope": [], "reasons": ["PlanPacket digest mismatch"]}
    if candidate.get("acceptance_digest") != plan.get("acceptance_digest"):
        return {"result": "NOT_PROVEN", "out_of_scope": [], "reasons": ["acceptance digest mismatch"]}
    if not candidate.get("changed_path_coverage_complete"):
        return {"result": "NOT_PROVEN", "out_of_scope": [], "reasons": ["complete changed-path coverage was not established"]}
    write_scope = plan.get("write_scope") or {}
    includes = list(write_scope.get("include") or [])
    excludes = list(write_scope.get("exclude") or [])
    if not includes:
        return {"result": "NOT_PROVEN", "out_of_scope": [], "reasons": ["PlanPacket has no write_scope.include"]}
    out = []
    for raw in candidate.get("actual_changed_paths") or []:
        try:
            path = normalize_rel(str(raw))
        except ContractError as exc:
            reasons.append(str(exc))
            continue
        allowed = any(path_matches(path, pattern) for pattern in includes)
        denied = is_excluded(path, excludes)
        if not allowed or denied:
            out.append(path)
    if reasons:
        return {"result": "NOT_PROVEN", "out_of_scope": sorted(set(out)), "reasons": reasons}
    if out:
        return {"result": "FAIL", "out_of_scope": sorted(set(out)), "reasons": ["proven change outside Plan write scope"]}
    return {"result": "PASS", "out_of_scope": [], "reasons": []}


def add_integrity_finding(draft: dict[str, Any], summary: str) -> dict[str, Any]:
    changed = dict(draft)
    changed["verdict"] = "NOT_PROVEN"
    findings = list(changed.get("findings") or [])
    findings.append({"id": "validate.integrity", "summary": summary, "evidence_refs": ["verdict-store"]})
    changed["findings"] = findings
    return changed


def enforce_identity(draft: dict[str, Any]) -> dict[str, Any]:
    draft = dict(draft)
    draft.setdefault("author_context_id", None)
    draft.setdefault("validator_context_id", None)
    draft.setdefault("freshness_attestation", None)
    author = draft.get("author_context_id")
    validator = draft.get("validator_context_id")
    freshness = draft.get("freshness_attestation")
    problems = []
    if not isinstance(author, str) or not author.strip():
        problems.append("author context ID is missing")
    if not isinstance(validator, str) or not validator.strip():
        problems.append("validator context ID is missing")
    if author and validator and author == validator:
        problems.append("author and validator context IDs collide")
    if not isinstance(freshness, dict) or freshness.get("source") not in ("runtime", "caller") or not freshness.get("attester_identity"):
        problems.append("freshness attestation is missing or invalid")
    if draft.get("verdict") == "PASS" and (draft.get("not_checked") or []):
        problems.append("PASS cannot contain not_checked items")
    criteria = draft.get("criteria")
    if draft.get("verdict") == "PASS" and (
        not isinstance(criteria, list)
        or not criteria
        or any(not isinstance(item, dict) or item.get("result") != "PASS" for item in criteria)
    ):
        problems.append("PASS requires at least one criterion and every criterion must PASS")
    if problems:
        return add_integrity_finding(draft, "; ".join(problems))
    return draft


def artifact_bytes(draft: dict[str, Any]) -> tuple[dict[str, Any], bytes]:
    unsigned = {key: value for key, value in draft.items() if key != "artifact_digest"}
    digest = digest_value(unsigned)
    artifact = dict(unsigned)
    artifact["artifact_digest"] = digest
    return artifact, canonical_bytes(artifact) + b"\n"


def atomic_store(artifact: dict[str, Any], payload: bytes, destination: Path) -> tuple[Path, bool]:
    destination.mkdir(parents=True, exist_ok=True)
    target = destination / f"{artifact['artifact_digest']}.json"
    if target.exists():
        if target.read_bytes() == payload:
            return target, True
        raise ContractError(f"integrity collision at {target}")
    fd, temporary = tempfile.mkstemp(prefix=".verdict-", suffix=".tmp", dir=destination)
    try:
        with os.fdopen(fd, "wb") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, target)
        dir_fd = os.open(destination, os.O_RDONLY)
        try:
            os.fsync(dir_fd)
        finally:
            os.close(dir_fd)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)
    return target, False


def store_verdict(draft: dict[str, Any], destination: Path) -> tuple[dict[str, Any], Path, bool]:
    draft = enforce_identity(draft)
    draft["schema_version"] = "verdict.v2"
    artifact, payload = artifact_bytes(draft)
    try:
        path, existed = atomic_store(artifact, payload, destination)
    except ContractError as exc:
        artifact, payload = artifact_bytes(add_integrity_finding(draft, str(exc)))
        path, existed = atomic_store(artifact, payload, destination)
    return artifact, path, existed


def write_json(value: dict[str, Any], output: str | None) -> None:
    payload = json.dumps(value, sort_keys=True, indent=2, ensure_ascii=False) + "\n"
    if output:
        Path(output).write_text(payload, encoding="utf-8")
    else:
        sys.stdout.write(payload)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    manifest = sub.add_parser("manifest", help="compute subject-manifest.v1 without Git")
    manifest.add_argument("--root", required=True)
    manifest.add_argument("--include", action="append", required=True)
    manifest.add_argument("--exclude", action="append", default=[])
    manifest.add_argument("--base-manifest")
    manifest.add_argument("--git-metadata-json")
    manifest.add_argument("--output")
    verify = sub.add_parser("verify-manifest", help="recompute and compare a manifest")
    verify.add_argument("--root", required=True)
    verify.add_argument("--manifest", required=True)
    verify.add_argument("--base-manifest")
    scope = sub.add_parser("scope", help="compare Candidate changed paths to Plan write scope")
    scope.add_argument("--plan", required=True)
    scope.add_argument("--candidate", required=True)
    scope.add_argument("--output")
    digest = sub.add_parser("digest", help="print a canonical JSON digest")
    digest.add_argument("json_file")
    store = sub.add_parser("store-verdict", help="atomically persist verdict.v2")
    store.add_argument("--draft", required=True)
    store.add_argument("--workspace", default=".")
    store.add_argument("--verdict-dir")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        if args.command == "manifest":
            base = load_json(Path(args.base_manifest)) if args.base_manifest else None
            metadata = json.loads(args.git_metadata_json) if args.git_metadata_json else None
            write_json(build_manifest(Path(args.root), args.include, args.exclude, base, metadata), args.output)
        elif args.command == "verify-manifest":
            manifest = load_json(Path(args.manifest))
            base = load_json(Path(args.base_manifest)) if args.base_manifest else None
            ok, reason = verify_manifest(manifest, Path(args.root), base)
            write_json({"result": "PASS" if ok else "NOT_PROVEN", "reason": reason}, None)
            return 0 if ok else 1
        elif args.command == "scope":
            write_json(scope_result(load_json(Path(args.plan)), load_json(Path(args.candidate))), args.output)
        elif args.command == "digest":
            print(digest_value(load_json(Path(args.json_file))))
        elif args.command == "store-verdict":
            destination = Path(args.verdict_dir) if args.verdict_dir else Path(args.workspace) / ".agentops" / "verdicts" / "sha256"
            artifact, path, existed = store_verdict(load_json(Path(args.draft)), destination)
            write_json({"artifact_digest": artifact["artifact_digest"], "path": str(path), "verdict": artifact["verdict"], "idempotent": existed}, None)
        return 0
    except (ContractError, OSError, json.JSONDecodeError) as exc:
        print(f"validate: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
