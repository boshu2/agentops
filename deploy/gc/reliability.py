#!/usr/bin/env python3
"""Deterministic inventory, cleanup admission, and worker Git auditing for AgentOps GC.

Discovery is read-only. Cleanup is manifest-driven and refuses names/globs,
ambiguous ownership, dirty destructive targets without recoverable evidence,
and process identity drift.
"""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import plistlib
import shutil
import signal
import subprocess
import sys
import tarfile
import time
from typing import Any


SCHEMA_VERSION = 1
AGENTOPS_GC_PREFIXES = ("gc-agentops", "agentops-gc")
AGENTOPS_GC_TEMP_PREFIX = "agentops-gc-reliability-"
AMBIGUOUS_GC_NAMES = {"gc-city", "gc-mvp", "gc-role-duel"}
REGISTRY_REQUIRED = {
    "id",
    "kind",
    "observed",
    "location",
    "identities",
    "symptom",
    "reproduction",
    "owner",
    "fix_location",
    "build",
    "deterministic_test",
    "live_check",
    "upstream",
    "disposition",
}
EVIDENCE_EXCLUDED_PARTS = {".git", ".gc", ".beads", ".codex", ".claude"}


class ReliabilityError(RuntimeError):
    pass


def canonical_json(value: Any) -> bytes:
    return (json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n").encode()


def digest(value: Any) -> str:
    return hashlib.sha256(canonical_json(value)).hexdigest()


def atomic_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(f".{path.name}.{os.getpid()}.tmp")
    with temporary.open("wb") as handle:
        handle.write(json.dumps(value, indent=2, sort_keys=True).encode())
        handle.write(b"\n")
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, path)


def atomic_bytes(path: Path, value: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(f".{path.name}.{os.getpid()}.tmp")
    with temporary.open("wb") as handle:
        handle.write(value)
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, path)


def run(argv: list[str], *, cwd: Path | None = None, check: bool = True) -> str:
    result = subprocess.run(
        argv,
        cwd=cwd,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        timeout=20,
        check=False,
    )
    if check and result.returncode:
        raise ReliabilityError(
            f"command failed ({result.returncode}): {' '.join(argv)}\n{result.stderr.strip()}"
        )
    return result.stdout


def sha256_file(path: Path) -> str:
    hasher = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            hasher.update(chunk)
    return hasher.hexdigest()


def git_status(path: Path) -> dict[str, Any] | None:
    probe = subprocess.run(
        ["git", "-C", str(path), "rev-parse", "--show-toplevel"],
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        check=False,
    )
    if probe.returncode:
        return None
    top = Path(probe.stdout.strip()).resolve()
    status = run(["git", "-C", str(top), "status", "--porcelain=v1"], check=True)
    return {
        "top": str(top),
        "head": run(["git", "-C", str(top), "rev-parse", "HEAD"]).strip(),
        "branch": run(
            ["git", "-C", str(top), "symbolic-ref", "--quiet", "--short", "HEAD"],
            check=False,
        ).strip()
        or None,
        "dirty": bool(status),
        "status": status.splitlines(),
    }


def safe_marker(path: Path) -> dict[str, Any] | None:
    marker = path / ".gc" / "agentops-bootstrap.json"
    if not marker.is_file():
        return None
    try:
        payload = json.loads(marker.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        return {"error": str(exc)}
    allowed = {
        "schema_version",
        "state",
        "city",
        "rig",
        "pack",
        "rig_name",
        "binding",
        "gc_bin",
        "max_active_sessions",
    }
    return {key: payload[key] for key in sorted(allowed) if key in payload}


def path_record(path: Path) -> dict[str, Any]:
    resolved = path.resolve()
    name = path.name
    if name.startswith(AGENTOPS_GC_PREFIXES):
        ownership = "agentops_gc_named"
    elif name in AMBIGUOUS_GC_NAMES:
        ownership = "ambiguous_gc_named"
    else:
        ownership = "unclassified"
    return {
        "path": str(path),
        "realpath": str(resolved),
        "name": name,
        "ownership": ownership,
        "agentops_marker": safe_marker(path),
        "gc_site": (path / ".gc" / "site.toml").is_file(),
        "git": git_status(path),
    }


def agentops_experiment_ancestor(path: Path) -> Path | None:
    """Return the exact AgentOps GC experiment root containing *path*, if any."""
    for candidate in (path, *path.parents):
        if candidate.name.startswith(AGENTOPS_GC_PREFIXES):
            return candidate.resolve()
    return None


def path_aliases(path: str) -> set[str]:
    """Return stable aliases for matching macOS /tmp and /private/tmp paths."""
    candidate = Path(path)
    aliases = {str(candidate), str(candidate.resolve())}
    resolved = str(candidate.resolve())
    if resolved.startswith("/private/tmp/"):
        aliases.add(resolved.removeprefix("/private"))
    return aliases


def git_worktrees(repo: Path) -> list[dict[str, Any]]:
    output = run(["git", "-C", str(repo), "worktree", "list", "--porcelain"])
    records: list[dict[str, Any]] = []
    current: dict[str, Any] = {}
    for line in output.splitlines() + [""]:
        if not line:
            if current:
                path = Path(current["path"])
                current["git"] = git_status(path) if path.exists() else None
                records.append(current)
                current = {}
            continue
        key, _, value = line.partition(" ")
        if key == "worktree":
            current["path"] = value
        elif key == "HEAD":
            current["head"] = value
        elif key == "branch":
            current["branch"] = value.removeprefix("refs/heads/")
        elif key in {"detached", "bare", "prunable", "locked"}:
            current[key] = value or True
    return records


def launchd_records(paths: list[dict[str, Any]]) -> list[dict[str, Any]]:
    launchd_root = Path.home() / "Library" / "LaunchAgents"
    known = sorted((item["realpath"] for item in paths), key=len, reverse=True)
    loaded: dict[str, int | None] = {}
    for line in run(["launchctl", "list"], check=False).splitlines()[1:]:
        fields = line.split(maxsplit=2)
        if len(fields) != 3:
            continue
        pid_text, _, label = fields
        loaded[label] = int(pid_text) if pid_text.isdigit() else None
    records = []
    if not launchd_root.is_dir():
        return records
    for path in sorted(launchd_root.glob("*gascity*"), key=str):
        try:
            with path.open("rb") as handle:
                payload = plistlib.load(handle)
        except (OSError, plistlib.InvalidFileException) as exc:
            records.append({"path": str(path), "sha256": sha256_file(path), "error": str(exc)})
            continue
        env = payload.get("EnvironmentVariables") or {}
        gc_home = env.get("GC_HOME") if isinstance(env, dict) else None
        arguments = payload.get("ProgramArguments") or []
        candidates = [str(gc_home)] if isinstance(gc_home, str) else []
        candidates.extend(str(item) for item in arguments if isinstance(item, str))
        owner = next(
            (
                candidate
                for candidate in known
                if any(alias in value for alias in path_aliases(candidate) for value in candidates)
            ),
            None,
        )
        ownership = "proven_path" if owner else "ambiguous"
        if owner is None and isinstance(gc_home, str):
            experiment = agentops_experiment_ancestor(Path(gc_home))
            if experiment is not None:
                owner = str(experiment)
                ownership = "proven_named_gc_home"
        label = payload.get("Label")
        records.append(
            {
                "path": str(path),
                "sha256": sha256_file(path),
                "label": label,
                "pid": loaded.get(label) if isinstance(label, str) else None,
                "program_arguments": arguments,
                "gc_home": gc_home,
                "owned_path": owner,
                "ownership": ownership,
            }
        )
    return records


def process_records(
    paths: list[dict[str, Any]], launchd: list[dict[str, Any]]
) -> list[dict[str, Any]]:
    output = run(["ps", "-axo", "pid=,ppid=,command="])
    known = sorted((item["realpath"] for item in paths), key=len, reverse=True)
    launchd_by_pid = {item.get("pid"): item for item in launchd if item.get("pid")}
    records = []
    for raw in output.splitlines():
        fields = raw.strip().split(maxsplit=2)
        if len(fields) != 3:
            continue
        pid_text, ppid_text, command = fields
        if not any(
            marker in command
            for marker in ("gc supervisor run", "dolt sql-server", "__gc-managed-dolt-scope-watchdog")
        ):
            continue
        launchd_item = launchd_by_pid.get(int(pid_text))
        owner = next(
            (
                candidate
                for candidate in known
                if any(alias in command for alias in path_aliases(candidate))
            ),
            None,
        )
        if owner is None:
            for field in command.split():
                experiment = agentops_experiment_ancestor(Path(field))
                if experiment is not None:
                    owner = str(experiment)
                    break
        if owner is None and launchd_item is not None:
            owner = launchd_item.get("owned_path")
        records.append(
            {
                "pid": int(pid_text),
                "ppid": int(ppid_text),
                "command": command,
                "owned_path": owner,
                "ownership": launchd_item.get("ownership") if launchd_item and owner else (
                    "proven_path" if owner else "ambiguous"
                ),
                "launchd_label": launchd_item.get("label") if launchd_item else None,
            }
        )
    return records


def binary_records(dev_root: Path, gascity_repo: Path) -> list[dict[str, Any]]:
    candidates: set[Path] = set()
    for command in ("gc", "bd"):
        output = run(["which", "-a", command], check=False)
        candidates.update(Path(line) for line in output.splitlines() if line.strip())
    candidates.add(gascity_repo / "bin" / "gc")
    for path in dev_root.glob("*toolchain*/bin/gc"):
        candidates.add(path)
    records = []
    for path in sorted(candidates, key=str):
        if not path.is_file():
            continue
        resolved = path.resolve()
        command = "bd" if path.name == "bd" else "gc"
        version = run([str(path), "version"], check=False).strip().splitlines()
        records.append(
            {
                "path": str(path),
                "realpath": str(resolved),
                "command": command,
                "sha256": sha256_file(resolved),
                "version": version[:4],
            }
        )
    return records


def inventory(dev_root: Path, gascity_repo: Path, generated_at: str | None) -> dict[str, Any]:
    dev_root = dev_root.resolve()
    gascity_repo = gascity_repo.resolve()
    paths = []
    for candidate in sorted(dev_root.iterdir(), key=lambda item: item.name):
        if not candidate.is_dir():
            continue
        if candidate.name.startswith(AGENTOPS_GC_PREFIXES) or candidate.name in AMBIGUOUS_GC_NAMES:
            paths.append(path_record(candidate))
    temp_root = Path("/tmp").resolve()
    for candidate in sorted(temp_root.glob(f"{AGENTOPS_GC_TEMP_PREFIX}*"), key=lambda item: item.name):
        paths.append(path_record(candidate))
    launchd = launchd_records(paths)
    payload = {
        "schema_version": SCHEMA_VERSION,
        "generated_at": generated_at or dt.datetime.now(dt.UTC).isoformat(),
        "dev_root": str(dev_root),
        "gascity_repo": str(gascity_repo),
        "experiment_paths": paths,
        "gascity_worktrees": git_worktrees(gascity_repo),
        "processes": process_records(paths, launchd),
        "binaries": binary_records(dev_root, gascity_repo),
        "launchd": launchd,
    }
    stable = dict(payload)
    stable.pop("generated_at")
    payload["inventory_digest"] = digest(stable)
    return payload


def validate_registry(path: Path) -> dict[str, Any]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    entries = payload.get("entries")
    if payload.get("schema_version") != SCHEMA_VERSION or not isinstance(entries, list) or not entries:
        raise ReliabilityError("registry requires schema_version=1 and nonempty entries")
    seen: set[str] = set()
    for index, entry in enumerate(entries):
        missing = sorted(REGISTRY_REQUIRED - set(entry))
        if missing:
            raise ReliabilityError(f"registry entry {index} missing: {', '.join(missing)}")
        if entry["id"] in seen:
            raise ReliabilityError(f"duplicate registry id: {entry['id']}")
        seen.add(entry["id"])
        if entry["owner"] not in {"agentops", "gascity", "beads", "environment", "unknown"}:
            raise ReliabilityError(f"invalid owner for {entry['id']}: {entry['owner']}")
    return {"entries": len(entries), "registry_digest": digest(payload)}


def evidence_safe_untracked(path: str) -> bool:
    candidate = Path(path)
    return not candidate.is_absolute() and ".." not in candidate.parts and not (
        set(candidate.parts) & EVIDENCE_EXCLUDED_PARTS
    )


def preserve_git(path: Path, output: Path) -> dict[str, Any]:
    path = path.resolve()
    status = git_status(path)
    if status is None:
        raise ReliabilityError(f"not a Git worktree: {path}")
    top = Path(status["top"])
    tracked = run(["git", "-C", str(top), "diff", "--binary", "HEAD"]).encode()
    staged = run(["git", "-C", str(top), "diff", "--binary", "--cached", "HEAD"]).encode()
    untracked = [
        item
        for item in run(
            ["git", "-C", str(top), "ls-files", "--others", "--exclude-standard", "-z"]
        ).split("\0")
        if item and evidence_safe_untracked(item)
    ]
    output.mkdir(parents=True, exist_ok=True)
    tracked_path = output / "tracked.patch"
    staged_path = output / "staged.patch"
    untracked_path = output / "untracked.tar.gz"
    atomic_bytes(tracked_path, tracked)
    atomic_bytes(staged_path, staged)
    temporary_tar = untracked_path.with_name(f".{untracked_path.name}.{os.getpid()}.tmp")
    with tarfile.open(temporary_tar, "w:gz") as archive:
        for relative in sorted(untracked):
            source = top / relative
            if source.exists() or source.is_symlink():
                archive.add(source, arcname=relative, recursive=False)
    os.replace(temporary_tar, untracked_path)
    metadata = {
        "schema_version": SCHEMA_VERSION,
        "source": str(top),
        "head": status["head"],
        "branch": status["branch"],
        "status": status["status"],
        "tracked_patch_sha256": sha256_file(tracked_path),
        "staged_patch_sha256": sha256_file(staged_path),
        "untracked_archive_sha256": sha256_file(untracked_path),
        "untracked_files": sorted(untracked),
        "excluded_runtime_parts": sorted(EVIDENCE_EXCLUDED_PARTS),
    }
    atomic_json(output / "metadata.json", metadata)
    return metadata


def git_snapshot(repo: Path) -> dict[str, Any]:
    repo = repo.resolve()
    common = run(["git", "-C", str(repo), "rev-parse", "--git-common-dir"]).strip()
    common_path = (repo / common).resolve() if not Path(common).is_absolute() else Path(common).resolve()
    worktrees = git_worktrees(repo)
    refs = run(["git", "-C", str(repo), "for-each-ref", "--format=%(refname)|%(objectname)"])
    reflog = run(
        ["git", "-C", str(repo), "reflog", "show", "--all", "--date=raw", "--format=%H|%gD|%gs"],
        check=False,
    )
    stash = run(["git", "-C", str(repo), "stash", "list", "--format=%gd|%H|%gs"], check=False)
    return {
        "schema_version": SCHEMA_VERSION,
        "repo": str(repo),
        "common_dir": str(common_path),
        "worktrees": worktrees,
        "refs": refs.splitlines(),
        "reflog": reflog.splitlines(),
        "stash": stash.splitlines(),
    }


def git_audit(before: dict[str, Any], after: dict[str, Any]) -> dict[str, Any]:
    findings = []
    if before.get("repo") != after.get("repo") or before.get("common_dir") != after.get("common_dir"):
        findings.append("repository identity changed")
    for key, label in (("stash", "stash state changed"), ("refs", "ref state changed")):
        if before.get(key) != after.get(key):
            findings.append(label)
    before_reflog = before.get("reflog", [])
    after_reflog = after.get("reflog", [])
    if before_reflog != after_reflog:
        findings.append("reflog changed")
    before_trees = {item["path"]: item for item in before.get("worktrees", [])}
    after_trees = {item["path"]: item for item in after.get("worktrees", [])}
    if set(before_trees) != set(after_trees):
        findings.append("worktree set changed")
    for path in sorted(set(before_trees) & set(after_trees)):
        left = before_trees[path]
        right = after_trees[path]
        if left.get("head") != right.get("head") or left.get("branch") != right.get("branch"):
            findings.append(f"HEAD or branch changed: {path}")
        left_status = (left.get("git") or {}).get("status", [])
        right_status = (right.get("git") or {}).get("status", [])
        if left_status != right_status:
            findings.append(f"content/status changed: {path}")
    return {"result": "PASS" if not findings else "FAIL", "findings": findings}


def find_path(inventory_payload: dict[str, Any], target: str) -> dict[str, Any] | None:
    target_path = str(Path(target).resolve())
    for item in inventory_payload.get("experiment_paths", []):
        if item.get("realpath") == target_path:
            return item
    return None


def validate_cleanup_plan(inventory_payload: dict[str, Any], plan: dict[str, Any]) -> dict[str, Any]:
    if plan.get("schema_version") != SCHEMA_VERSION or not isinstance(plan.get("actions"), list):
        raise ReliabilityError("cleanup plan requires schema_version=1 and actions[]")
    admitted = []
    for index, action in enumerate(plan["actions"]):
        operation = action.get("operation")
        if operation == "move_path":
            target = action.get("target")
            archive = action.get("archive")
            if not isinstance(target, str) or not Path(target).is_absolute():
                raise ReliabilityError(f"action {index}: target must be an exact absolute path")
            if not isinstance(archive, str) or not Path(archive).is_absolute():
                raise ReliabilityError(f"action {index}: archive must be an exact absolute path")
            item = find_path(inventory_payload, target)
            if item is None:
                raise ReliabilityError(f"action {index}: target absent from inventory: {target}")
            if item.get("ownership") != "agentops_gc_named":
                raise ReliabilityError(f"action {index}: ambiguous ownership: {target}")
            dirty = bool((item.get("git") or {}).get("dirty"))
            evidence = action.get("evidence_ref")
            if dirty and (not isinstance(evidence, str) or not Path(evidence).is_file()):
                raise ReliabilityError(f"action {index}: dirty target lacks recoverable evidence: {target}")
        elif operation == "stop_process":
            pid = action.get("pid")
            expected = action.get("expected_command")
            matches = [item for item in inventory_payload.get("processes", []) if item.get("pid") == pid]
            if len(matches) != 1 or matches[0].get("command") != expected:
                raise ReliabilityError(f"action {index}: process identity not exact")
            if matches[0].get("ownership") not in {"proven_path", "proven_named_gc_home"}:
                raise ReliabilityError(f"action {index}: process ownership is ambiguous")
        elif operation == "git_worktree_remove":
            target = action.get("target")
            matches = [item for item in inventory_payload.get("gascity_worktrees", []) if item.get("path") == target]
            if len(matches) != 1:
                raise ReliabilityError(f"action {index}: worktree absent from inventory")
            if (matches[0].get("git") or {}).get("dirty"):
                raise ReliabilityError(f"action {index}: dirty worktree requires archival, not removal")
        elif operation == "git_worktree_archive":
            target = action.get("target")
            matches = [item for item in inventory_payload.get("gascity_worktrees", []) if item.get("path") == target]
            if len(matches) != 1:
                raise ReliabilityError(f"action {index}: worktree absent from inventory")
            evidence = action.get("evidence_ref")
            if not isinstance(evidence, str) or not Path(evidence).is_file():
                raise ReliabilityError(f"action {index}: archived worktree lacks recoverable evidence")
        elif operation == "launchd_remove":
            target = action.get("target")
            label = action.get("label")
            expected_sha256 = action.get("expected_sha256")
            matches = [
                item
                for item in inventory_payload.get("launchd", [])
                if item.get("path") == target
                and item.get("label") == label
                and item.get("sha256") == expected_sha256
            ]
            if len(matches) != 1:
                raise ReliabilityError(f"action {index}: launchd identity not exact")
            if matches[0].get("ownership") not in {"proven_path", "proven_named_gc_home"}:
                raise ReliabilityError(f"action {index}: launchd ownership is ambiguous")
            archive = action.get("archive")
            if not isinstance(archive, str) or not Path(archive).is_absolute():
                raise ReliabilityError(f"action {index}: launchd archive must be absolute")
        else:
            raise ReliabilityError(f"action {index}: unsupported operation: {operation!r}")
        admitted.append(index)
    stable = {"schema_version": plan["schema_version"], "actions": plan["actions"]}
    return {"admitted_actions": admitted, "plan_digest": digest(stable)}


def current_process_command(pid: int) -> str | None:
    output = run(["ps", "-p", str(pid), "-o", "command="], check=False).strip()
    return output or None


def apply_cleanup_plan(
    inventory_payload: dict[str, Any], plan: dict[str, Any], *, execute: bool, confirmation: str | None
) -> dict[str, Any]:
    validation = validate_cleanup_plan(inventory_payload, plan)
    plan_digest = validation["plan_digest"]
    if execute and confirmation != plan_digest:
        raise ReliabilityError("--confirm must equal the validated plan digest")
    results = []
    for action in plan["actions"]:
        operation = action["operation"]
        if not execute:
            results.append({"operation": operation, "result": "would_apply"})
            continue
        if operation == "move_path":
            target = Path(action["target"])
            archive = Path(action["archive"])
            if not target.exists() and archive.exists():
                result = "already_applied"
            elif not target.exists():
                raise ReliabilityError(f"move target disappeared without archive: {target}")
            elif archive.exists():
                raise ReliabilityError(f"archive target already exists: {archive}")
            else:
                archive.parent.mkdir(parents=True, exist_ok=True)
                target.rename(archive)
                result = "applied"
            results.append({"operation": operation, "target": str(target), "result": result})
        elif operation == "stop_process":
            pid = int(action["pid"])
            expected = action["expected_command"]
            command = current_process_command(pid)
            if command is None:
                result = "already_applied"
            elif command != expected:
                raise ReliabilityError(f"process identity drift for PID {pid}")
            else:
                os.kill(pid, signal.SIGTERM)
                deadline = time.monotonic() + 10
                while time.monotonic() < deadline and current_process_command(pid) is not None:
                    time.sleep(0.1)
                if current_process_command(pid) is not None:
                    raise ReliabilityError(f"PID {pid} did not exit after SIGTERM")
                result = "applied"
            results.append({"operation": operation, "pid": pid, "result": result})
        elif operation == "git_worktree_remove":
            target = Path(action["target"])
            repo = Path(inventory_payload["gascity_repo"])
            listed = {item["path"] for item in git_worktrees(repo)}
            if str(target) not in listed and not target.exists():
                result = "already_applied"
            else:
                run(["git", "-C", str(repo), "worktree", "remove", str(target)])
                result = "applied"
            results.append({"operation": operation, "target": str(target), "result": result})
        elif operation == "git_worktree_archive":
            target = Path(action["target"])
            repo = Path(inventory_payload["gascity_repo"])
            listed = {item["path"] for item in git_worktrees(repo)}
            if str(target) not in listed and not target.exists():
                result = "already_applied"
            else:
                run(["git", "-C", str(repo), "worktree", "remove", "--force", str(target)])
                result = "applied"
            results.append({"operation": operation, "target": str(target), "result": result})
        elif operation == "launchd_remove":
            target = Path(action["target"])
            archive = Path(action["archive"])
            label = action["label"]
            run(["launchctl", "bootout", f"gui/{os.getuid()}/{label}"], check=False)
            if not target.exists() and archive.exists():
                result = "already_applied"
            elif not target.exists():
                raise ReliabilityError(f"launchd plist disappeared without archive: {target}")
            elif archive.exists():
                raise ReliabilityError(f"launchd archive target already exists: {archive}")
            else:
                archive.parent.mkdir(parents=True, exist_ok=True)
                target.rename(archive)
                result = "applied"
            results.append({"operation": operation, "target": str(target), "label": label, "result": result})
    return {"plan_digest": plan_digest, "execute": execute, "results": results}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)

    inv = sub.add_parser("inventory")
    inv.add_argument("--dev-root", required=True, type=Path)
    inv.add_argument("--gascity-repo", required=True, type=Path)
    inv.add_argument("--generated-at")
    inv.add_argument("--output", required=True, type=Path)

    registry = sub.add_parser("validate-registry")
    registry.add_argument("--registry", required=True, type=Path)

    preserve = sub.add_parser("preserve-git")
    preserve.add_argument("--path", required=True, type=Path)
    preserve.add_argument("--output", required=True, type=Path)

    snap = sub.add_parser("git-snapshot")
    snap.add_argument("--repo", required=True, type=Path)
    snap.add_argument("--output", required=True, type=Path)

    audit = sub.add_parser("git-audit")
    audit.add_argument("--before", required=True, type=Path)
    audit.add_argument("--after", required=True, type=Path)
    audit.add_argument("--output", type=Path)

    plan = sub.add_parser("validate-cleanup")
    plan.add_argument("--inventory", required=True, type=Path)
    plan.add_argument("--plan", required=True, type=Path)

    apply = sub.add_parser("apply-cleanup")
    apply.add_argument("--inventory", required=True, type=Path)
    apply.add_argument("--plan", required=True, type=Path)
    apply.add_argument("--execute", action="store_true")
    apply.add_argument("--confirm")
    apply.add_argument("--output", type=Path)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        if args.command == "inventory":
            payload = inventory(args.dev_root, args.gascity_repo, args.generated_at)
            atomic_json(args.output, payload)
            result: dict[str, Any] = {
                "inventory": str(args.output),
                "inventory_digest": payload["inventory_digest"],
            }
        elif args.command == "validate-registry":
            result = validate_registry(args.registry)
        elif args.command == "preserve-git":
            result = preserve_git(args.path, args.output)
        elif args.command == "git-snapshot":
            result = git_snapshot(args.repo)
            atomic_json(args.output, result)
            result = {"snapshot": str(args.output), "snapshot_digest": digest(result)}
        elif args.command == "git-audit":
            before = json.loads(args.before.read_text(encoding="utf-8"))
            after = json.loads(args.after.read_text(encoding="utf-8"))
            result = git_audit(before, after)
            if args.output:
                atomic_json(args.output, result)
        elif args.command == "validate-cleanup":
            inv = json.loads(args.inventory.read_text(encoding="utf-8"))
            plan = json.loads(args.plan.read_text(encoding="utf-8"))
            result = validate_cleanup_plan(inv, plan)
        else:
            inv = json.loads(args.inventory.read_text(encoding="utf-8"))
            plan = json.loads(args.plan.read_text(encoding="utf-8"))
            result = apply_cleanup_plan(inv, plan, execute=args.execute, confirmation=args.confirm)
            if args.output:
                atomic_json(args.output, result)
        print(json.dumps(result, indent=2, sort_keys=True))
        return 1 if args.command == "git-audit" and result["result"] != "PASS" else 0
    except (OSError, ValueError, ReliabilityError, subprocess.TimeoutExpired) as exc:
        print(f"gc reliability: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
