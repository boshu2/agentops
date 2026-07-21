#!/usr/bin/env python3
"""Deterministic inventory, cleanup admission, and worker Git auditing for AgentOps GC.

Discovery is read-only. Cleanup is manifest-driven and refuses names/globs,
ambiguous ownership, dirty destructive targets without recoverable evidence,
and process identity drift.
"""

from __future__ import annotations

import argparse
from collections import Counter
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import plistlib
import secrets
import shutil
import signal
import stat
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
    "release_relevance",
}
EVIDENCE_EXCLUDED_PARTS = {".git", ".gc", ".beads", ".codex", ".claude"}
PROCESS_TOOL_NAMES = {"asm", "cgo", "compile", "go", "link"}


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
    head = run(
        ["git", "-C", str(top), "rev-parse", "--verify", "HEAD"], check=False
    ).strip()
    return {
        "top": str(top),
        "head": head or None,
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


def safe_dolt_state(path: Path) -> dict[str, Any] | None:
    state = path / ".gc" / "runtime" / "packs" / "dolt" / "dolt-state.json"
    if not state.is_file():
        return None
    try:
        payload = json.loads(state.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        return {"path": str(state), "error": str(exc)}
    allowed = {
        "database",
        "host",
        "pid",
        "port",
        "running",
        "started_at",
        "version",
    }
    result = {key: payload[key] for key in sorted(allowed) if key in payload}
    result["path"] = str(state)
    return result


def path_record(path: Path) -> dict[str, Any]:
    resolved = path.resolve()
    name = path.name
    marker = safe_marker(path)
    if marker is not None and "error" not in marker:
        ownership = "proven_agentops_marker"
    elif name.startswith(AGENTOPS_GC_TEMP_PREFIX):
        ownership = "proven_agentops_tool_prefix"
    elif name.startswith(AGENTOPS_GC_PREFIXES):
        ownership = "name_only_agentops_gc"
    elif name in AMBIGUOUS_GC_NAMES:
        ownership = "ambiguous_gc_named"
    else:
        ownership = "unclassified"
    return {
        "path": str(path),
        "realpath": str(resolved),
        "name": name,
        "exists": path.exists(),
        "ownership": ownership,
        "agentops_marker": marker,
        "dolt_state": safe_dolt_state(path),
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
                if path.exists():
                    raw_index = run(["git", "-C", str(path), "rev-parse", "--git-path", "index"]).strip()
                    index = Path(raw_index)
                    current["git_index"] = str(index if index.is_absolute() else (path / index).resolve())
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


def git_repository_record(repo: Path) -> dict[str, Any]:
    repo = repo.resolve()
    status = git_status(repo)
    if status is None:
        return {
            "path": str(repo),
            "exists": repo.exists(),
            "error": "not a Git worktree",
        }
    remotes = []
    for line in run(["git", "-C", str(repo), "remote", "-v"]).splitlines():
        fields = line.split()
        if len(fields) == 3:
            remotes.append(
                {
                    "name": fields[0],
                    "url": fields[1],
                    "direction": fields[2].strip("()"),
                }
            )
    branches = []
    branch_output = run(
        [
            "git",
            "-C",
            str(repo),
            "for-each-ref",
            "--format=%(refname:short)%09%(objectname)%09%(upstream:short)",
            "refs/heads",
        ]
    )
    for line in branch_output.splitlines():
        name, head, upstream = (line.split("\t") + ["", ""])[:3]
        branches.append({"name": name, "head": head, "upstream": upstream or None})
    return {
        "path": str(repo),
        "exists": True,
        "git": status,
        "remotes": remotes,
        "branches": branches,
        "worktrees": git_worktrees(repo),
    }


def parse_tmux_sessions(raw: str) -> list[dict[str, Any]]:
    sessions = []
    for line in raw.splitlines():
        fields = line.split("\t")
        if len(fields) != 4:
            raise ReliabilityError(f"invalid tmux session record: {line!r}")
        name, windows, pid, attached = fields
        if not windows.isdigit() or not pid.isdigit() or attached not in {"0", "1"}:
            raise ReliabilityError(f"invalid tmux session record: {line!r}")
        sessions.append(
            {
                "name": name,
                "windows": int(windows),
                "pid": int(pid),
                "attached": attached == "1",
            }
        )
    return sessions


def tmux_records(socket_root: Path) -> list[dict[str, Any]]:
    if not socket_root.is_dir() or shutil.which("tmux") is None:
        return []
    records = []
    for socket in sorted(socket_root.iterdir(), key=lambda item: item.name):
        try:
            mode = socket.lstat().st_mode
        except OSError:
            continue
        if not stat.S_ISSOCK(mode):
            continue
        result = subprocess.run(
            [
                "tmux",
                "-S",
                str(socket),
                "list-sessions",
                "-F",
                "#{session_name}\t#{session_windows}\t#{pid}\t#{session_attached}",
            ],
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=5,
            check=False,
        )
        sessions = parse_tmux_sessions(result.stdout) if result.returncode == 0 else []
        records.append(
            {
                "socket": str(socket),
                "active": result.returncode == 0,
                "sessions": sessions,
                "error": result.stderr.strip() or None,
            }
        )
    return records


def path_ownership_is_proven(ownership: Any) -> bool:
    return ownership in {"proven_agentops_marker", "proven_agentops_tool_prefix"}


def process_path_ownership(paths_by_realpath: dict[str, dict[str, Any]], owner: str | None) -> str:
    if owner is None:
        return "ambiguous"
    record = paths_by_realpath.get(owner)
    if record is not None and path_ownership_is_proven(record.get("ownership")):
        return "proven_path"
    return "ambiguous_named_path"


def launchd_records(paths: list[dict[str, Any]]) -> list[dict[str, Any]]:
    launchd_root = Path.home() / "Library" / "LaunchAgents"
    paths_by_realpath = {item["realpath"]: item for item in paths}
    known = sorted(paths_by_realpath, key=len, reverse=True)
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
        ownership = process_path_ownership(paths_by_realpath, owner)
        if owner is None and isinstance(gc_home, str):
            experiment = agentops_experiment_ancestor(Path(gc_home))
            if experiment is not None:
                owner = str(experiment)
                ownership = process_path_ownership(paths_by_realpath, owner)
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
    paths_by_realpath = {item["realpath"]: item for item in paths}
    known = sorted(paths_by_realpath, key=len, reverse=True)
    launchd_by_pid = {item.get("pid"): item for item in launchd if item.get("pid")}
    records = []
    for raw in output.splitlines():
        fields = raw.strip().split(maxsplit=2)
        if len(fields) != 3:
            continue
        pid_text, ppid_text, command = fields
        executable = Path(command.split(maxsplit=1)[0]).name
        if "gc supervisor run" in command:
            category = "gc_supervisor"
        elif "dolt sql-server" in command:
            category = "dolt_server"
        elif "__gc-managed-dolt-scope-watchdog" in command:
            category = "dolt_watchdog"
        elif executable in PROCESS_TOOL_NAMES:
            category = "go_toolchain"
        else:
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
        ownership = process_path_ownership(paths_by_realpath, owner)
        if launchd_item is not None and launchd_item.get("ownership") == "proven_path":
            ownership = "proven_path"
        records.append(
            {
                "pid": int(pid_text),
                "ppid": int(ppid_text),
                "command": command,
                "category": category,
                "owned_path": owner,
                "ownership": ownership,
                "launchd_label": launchd_item.get("label") if launchd_item else None,
            }
        )
    return records


def binary_records(
    dev_root: Path, gascity_repo: Path, selected_paths: list[Path] | None = None
) -> list[dict[str, Any]]:
    candidates: dict[Path, set[str]] = {}

    def add(path: Path, source: str) -> None:
        candidates.setdefault(path, set()).add(source)

    for command in ("gc", "bd"):
        output = run(["which", "-a", command], check=False)
        for line in output.splitlines():
            if line.strip():
                add(Path(line), "ambient")
    add(gascity_repo / "bin" / "gc", "gascity_repo")
    for path in dev_root.glob("*toolchain*/bin/gc"):
        add(path, "dev_root")
    for path in selected_paths or []:
        add(path, "selected")
    records = []
    for path in sorted(candidates, key=str):
        if not path.is_file():
            continue
        resolved = path.resolve()
        command = "bd" if path.name == "bd" else "gc"
        records.append(
            {
                "path": str(path),
                "realpath": str(resolved),
                "command": command,
                "sha256": sha256_file(resolved),
                "version": None,
                "version_source": "not_executed",
                "sources": sorted(candidates[path]),
                "selected": "selected" in candidates[path],
            }
        )
    return records


def selected_toolchain(records: list[dict[str, Any]]) -> dict[str, Any]:
    identities: dict[str, set[tuple[str, str]]] = {"gc": set(), "bd": set()}
    for item in records:
        command = item.get("command")
        if item.get("selected") and command in identities:
            identities[command].add(
                (str(item.get("realpath")), str(item.get("sha256")))
            )
    counts = {command: len(values) for command, values in identities.items()}
    duplicate = any(count > 1 for count in counts.values())
    if duplicate:
        status = "ambiguous"
    elif counts == {"gc": 1, "bd": 1}:
        status = "exact"
    elif counts == {"gc": 0, "bd": 0}:
        status = "not_selected"
    else:
        status = "incomplete"
    return {
        "status": status,
        "duplicate_active_identity": duplicate,
        "identity_counts": counts,
        "identities": {
            command: [
                {"realpath": realpath, "sha256": sha256}
                for realpath, sha256 in sorted(values)
            ]
            for command, values in identities.items()
        },
    }


def inventory(
    dev_root: Path,
    gascity_repo: Path,
    generated_at: str | None,
    *,
    scan_roots: list[Path] | None = None,
    explicit_paths: list[Path] | None = None,
    git_repos: list[Path] | None = None,
    selected_binaries: list[Path] | None = None,
    tmux_root: Path | None = None,
) -> dict[str, Any]:
    dev_root = dev_root.resolve()
    gascity_repo = gascity_repo.resolve()
    roots = [dev_root, *(scan_roots or [])]
    candidates: dict[str, Path] = {}
    for root in roots:
        root = root.resolve()
        if not root.is_dir():
            continue
        for candidate in sorted(root.iterdir(), key=lambda item: item.name):
            if not candidate.is_dir():
                continue
            if (
                candidate.name.startswith(AGENTOPS_GC_PREFIXES)
                or candidate.name in AMBIGUOUS_GC_NAMES
            ):
                candidates[str(candidate.resolve())] = candidate
    temp_root = Path("/tmp").resolve()
    for candidate in sorted(
        temp_root.glob(f"{AGENTOPS_GC_TEMP_PREFIX}*"), key=lambda item: item.name
    ):
        candidates[str(candidate.resolve())] = candidate
    for candidate in explicit_paths or []:
        candidates[str(candidate.resolve())] = candidate
    paths = [path_record(candidates[key]) for key in sorted(candidates)]
    launchd = launchd_records(paths)
    processes = process_records(paths, launchd)
    binaries = binary_records(dev_root, gascity_repo, selected_binaries)
    repositories = [git_repository_record(path) for path in git_repos or []]
    socket_root = tmux_root or Path("/private/tmp") / f"tmux-{os.getuid()}"
    payload = {
        "schema_version": SCHEMA_VERSION,
        "generated_at": generated_at or dt.datetime.now(dt.UTC).isoformat(),
        "dev_root": str(dev_root),
        "scan_roots": sorted({str(path.resolve()) for path in roots}),
        "gascity_repo": str(gascity_repo),
        "experiment_paths": paths,
        "gascity_worktrees": git_worktrees(gascity_repo),
        "git_repositories": repositories,
        "processes": processes,
        "dolt": {
            "declared_states": [
                {"experiment": item["realpath"], **item["dolt_state"]}
                for item in paths
                if item.get("dolt_state")
            ],
            "live_processes": [
                item for item in processes if item["category"].startswith("dolt_")
            ],
        },
        "tmux": {"socket_root": str(socket_root), "servers": tmux_records(socket_root)},
        "binaries": binaries,
        "selected_toolchain": selected_toolchain(binaries),
        "launchd": launchd,
    }
    stable = dict(payload)
    stable.pop("generated_at")
    payload["inventory_digest"] = digest(stable)
    return payload


def validate_inventory_payload(payload: dict[str, Any]) -> dict[str, Any]:
    required = {
        "schema_version",
        "generated_at",
        "inventory_digest",
        "experiment_paths",
        "processes",
        "dolt",
        "tmux",
        "binaries",
        "selected_toolchain",
        "git_repositories",
    }
    if payload.get("schema_version") != SCHEMA_VERSION or not required.issubset(payload):
        raise ReliabilityError("inventory payload is missing required fields")
    stable = dict(payload)
    stable.pop("generated_at")
    recorded_digest = stable.pop("inventory_digest")
    if recorded_digest != digest(stable):
        raise ReliabilityError("inventory digest does not match payload")
    paths = payload["experiment_paths"]
    processes = payload["processes"]
    if not isinstance(paths, list) or not isinstance(processes, list):
        raise ReliabilityError("inventory paths and processes must be lists")
    allowed_ownership = {
        "proven_agentops_marker",
        "proven_agentops_tool_prefix",
        "name_only_agentops_gc",
        "ambiguous_gc_named",
        "unclassified",
    }
    seen_paths: set[str] = set()
    for index, item in enumerate(paths):
        realpath = item.get("realpath") if isinstance(item, dict) else None
        if not isinstance(realpath, str) or realpath in seen_paths:
            raise ReliabilityError(f"duplicate experiment path identity at index {index}")
        seen_paths.add(realpath)
        if item.get("ownership") not in allowed_ownership:
            raise ReliabilityError(f"invalid experiment ownership at index {index}")
    seen_pids: set[int] = set()
    for index, item in enumerate(processes):
        pid = item.get("pid") if isinstance(item, dict) else None
        if not isinstance(pid, int) or pid in seen_pids or not isinstance(item.get("category"), str):
            raise ReliabilityError(f"invalid process identity at index {index}")
        seen_pids.add(pid)
    tmux = payload["tmux"]
    dolt = payload["dolt"]
    if not isinstance(tmux, dict) or not isinstance(tmux.get("servers"), list):
        raise ReliabilityError("inventory tmux section is invalid")
    if not isinstance(dolt, dict) or not isinstance(dolt.get("declared_states"), list):
        raise ReliabilityError("inventory Dolt section is invalid")
    selected = payload["selected_toolchain"]
    if not isinstance(selected, dict) or selected.get("duplicate_active_identity") is True:
        raise ReliabilityError("inventory selected toolchain is ambiguous")
    return {
        "inventory_digest": recorded_digest,
        "experiment_paths": len(paths),
        "processes": len(processes),
        "tmux_servers": len(tmux["servers"]),
        "dolt_states": len(dolt["declared_states"]),
        "selected_toolchain_status": selected.get("status"),
    }


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


def valid_git_sha(value: Any) -> bool:
    return (
        isinstance(value, str)
        and len(value) == 40
        and all(character in "0123456789abcdef" for character in value)
    )


def validate_fork_manifest(payload: dict[str, Any]) -> dict[str, Any]:
    required = {
        "schema_version",
        "observed_at",
        "upstream",
        "fork",
        "upstream_main",
        "fork_main",
        "local_pre_push",
        "retained_branches",
        "removed_merged_branches",
    }
    if set(payload) != required or payload.get("schema_version") != SCHEMA_VERSION:
        raise ReliabilityError("fork manifest fields are invalid")
    if not valid_git_sha(payload["upstream_main"]) or not valid_git_sha(
        payload["fork_main"]
    ):
        raise ReliabilityError("fork manifest requires exact Git SHAs")
    if payload["upstream_main"] != payload["fork_main"]:
        raise ReliabilityError("fork main does not equal observed upstream main")
    pre_push = payload.get("local_pre_push")
    if not isinstance(pre_push, dict) or set(pre_push) != {
        "result",
        "subject_sha",
        "log_sha256",
        "bypass_used",
        "reason",
    }:
        raise ReliabilityError("fork manifest local pre-push evidence is invalid")
    if pre_push["result"] not in {
        "passed",
        "failed_on_observed_official_main",
        "prior_official_main_failure_reused",
    }:
        raise ReliabilityError("fork manifest local pre-push result is invalid")
    if not valid_git_sha(pre_push["subject_sha"]):
        raise ReliabilityError("fork manifest local pre-push subject is invalid")
    if (
        pre_push["result"] in {"passed", "failed_on_observed_official_main"}
        and pre_push["subject_sha"] != payload["fork_main"]
    ):
        raise ReliabilityError("fork manifest local pre-push subject does not match fork main")
    log_sha256 = pre_push["log_sha256"]
    if (
        not isinstance(log_sha256, str)
        or len(log_sha256) != 64
        or not all(character in "0123456789abcdef" for character in log_sha256)
    ):
        raise ReliabilityError("fork manifest local pre-push digest is invalid")
    if pre_push["bypass_used"] is True and pre_push["result"] not in {
        "failed_on_observed_official_main",
        "prior_official_main_failure_reused",
    }:
        raise ReliabilityError("fork manifest pre-push bypass lacks a failed official-main gate")
    if (
        pre_push["result"] == "prior_official_main_failure_reused"
        and pre_push["bypass_used"] is not True
    ):
        raise ReliabilityError("fork manifest prior failure reuse requires a bypass")
    if not isinstance(pre_push["reason"], str) or not pre_push["reason"].strip():
        raise ReliabilityError("fork manifest pre-push bypass lacks a reason")
    retained = payload.get("retained_branches")
    removed = payload.get("removed_merged_branches")
    if not isinstance(retained, list) or not isinstance(removed, list):
        raise ReliabilityError("fork manifest branch collections must be lists")
    seen: set[str] = {"main"}
    for index, entry in enumerate(retained):
        branch = entry.get("branch") if isinstance(entry, dict) else None
        if not isinstance(branch, str) or not branch or branch in seen:
            raise ReliabilityError(f"invalid retained fork branch at index {index}")
        seen.add(branch)
        if not valid_git_sha(entry.get("head")):
            raise ReliabilityError(f"retained branch {branch} requires exact head")
        if entry.get("upstream_kind") not in {"issue", "pr"}:
            raise ReliabilityError(f"retained branch {branch} lacks upstream kind")
        if entry.get("state") != "open" or not isinstance(
            entry.get("upstream_number"), int
        ):
            raise ReliabilityError(f"retained branch {branch} lacks open upstream work")
        if not str(entry.get("upstream_url", "")).startswith("https://github.com/"):
            raise ReliabilityError(f"retained branch {branch} lacks upstream URL")
    for index, entry in enumerate(removed):
        branch = entry.get("branch") if isinstance(entry, dict) else None
        if not isinstance(branch, str) or not branch or branch in seen:
            raise ReliabilityError(f"invalid removed fork branch at index {index}")
        seen.add(branch)
        if not valid_git_sha(entry.get("head")) or entry.get("state") != "merged":
            raise ReliabilityError(f"removed branch {branch} lacks merged evidence")
        if not isinstance(entry.get("upstream_number"), int) or not str(
            entry.get("upstream_url", "")
        ).startswith("https://github.com/"):
            raise ReliabilityError(f"removed branch {branch} lacks upstream evidence")
    return {
        "retained_branches": len(retained),
        "removed_merged_branches": len(removed),
        "manifest_digest": digest(payload),
    }


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


def _status_paths(status: list[str]) -> list[str]:
    paths: list[str] = []
    for line in status:
        if not isinstance(line, str) or len(line) < 4:
            paths.append("<invalid-status>")
            continue
        paths.extend(part for part in line[3:].split(" -> ") if part)
    return paths


def _declared_path(path: str, declared: list[str]) -> bool:
    for item in declared:
        root = item.rstrip("/") if item.endswith("/") else item
        if root and (path == root or path.startswith(f"{root}/")):
            return True
    return False


def _reflog_additions(before: list[Any], after: list[Any]) -> list[str]:
    """Return added reflog entries without trusting their presentation order."""
    remaining = Counter(item for item in before if isinstance(item, str))
    additions: list[str] = []
    for item in after:
        if not isinstance(item, str):
            additions.append("<invalid-reflog-entry>")
        elif remaining[item]:
            remaining[item] -= 1
        else:
            additions.append(item)
    return additions


def _candidate_reflog_action(entry: str, *, branch: str, index: str) -> str | None:
    """Classify one candidate reflog action, or None when it belongs elsewhere.

    The candidate branch and its linked-worktree HEAD are the only reflogs it
    may change.  Ref ownership alone is insufficient: only ordinary commit
    actions are allowed, and the snapshot audit separately proves the ref is a
    forward move whose diff remains in declared scope.
    """
    fields = entry.split("|", 2)
    if len(fields) != 3:
        return None
    _object_id, selector, subject = fields
    worktree_head = f"worktrees/{Path(index).parent.name}/HEAD@{{"
    if not (selector.startswith(f"refs/heads/{branch}@{{") or selector.startswith(worktree_head)):
        return None
    return subject.partition(":")[0]


def _valid_process_receipt(receipt: Any) -> bool:
    fields = {
        "schema_version", "isolation_token", "runner_pgid", "completed",
        "outcome", "timeout", "leak_detected", "cleanup_required",
        "cleanup_complete", "surviving_pids",
    }
    if not isinstance(receipt, dict) or set(receipt) != fields:
        return False
    token = receipt.get("isolation_token")
    return (
        receipt.get("schema_version") == "candidate-process-receipt.v1"
        and isinstance(token, str)
        and len(token) == 64
        and all(character in "0123456789abcdef" for character in token)
        and isinstance(receipt.get("runner_pgid"), int)
        and receipt["runner_pgid"] > 0
        and all(isinstance(receipt.get(key), bool) for key in (
            "completed", "timeout", "leak_detected", "cleanup_required", "cleanup_complete",
        ))
        and isinstance(receipt.get("outcome"), str)
        and isinstance(receipt.get("surviving_pids"), list)
        and all(isinstance(pid, int) and pid > 0 for pid in receipt["surviving_pids"])
    )


def candidate_git_audit(before: dict[str, Any], after: dict[str, Any], candidate: dict[str, Any], *, process_receipt: dict[str, Any] | None) -> dict[str, Any]:
    """Audit one candidate while allowing only its branch, tree, index and paths.

    This is intentionally snapshot-only: it never scans or kills by process
    name.  The bounded runner supplies the exact isolation-token receipt.
    """
    findings: list[str] = []
    worktree, branch, index = candidate.get("worktree"), candidate.get("branch"), candidate.get("index")
    declared = candidate.get("declared_paths", [])
    generated = candidate.get("generated_paths", [])
    if not all(isinstance(value, str) and value for value in (worktree, branch, index)) or not isinstance(declared, list) or not isinstance(generated, list) or not all(isinstance(item, str) and item for item in [*declared, *generated]):
        raise ReliabilityError("candidate audit requires exact worktree, branch, index, and declared paths")
    allowed_paths = [*declared, *generated]
    if before.get("repo") != after.get("repo") or before.get("common_dir") != after.get("common_dir"):
        findings.append("repository identity changed")
    if before.get("stash") != after.get("stash"):
        findings.append("stash state changed")
    before_trees = {str(Path(item["path"]).resolve()): item for item in before.get("worktrees", [])}
    after_trees = {str(Path(item["path"]).resolve()): item for item in after.get("worktrees", [])}
    worktree = str(Path(worktree).resolve())
    if set(before_trees) != set(after_trees):
        findings.append("worktree set changed")
    if worktree not in before_trees or worktree not in after_trees:
        findings.append("candidate worktree missing from snapshot")
    for path in sorted(set(before_trees) & set(after_trees)):
        left, right = before_trees[path], after_trees[path]
        if path != worktree:
            if left.get("head") != right.get("head") or left.get("branch") != right.get("branch") or (left.get("git") or {}).get("status", []) != (right.get("git") or {}).get("status", []):
                findings.append(f"non-candidate worktree changed: {path}")
            continue
        if left.get("branch") != branch or right.get("branch") != branch or left.get("git_index") != index or right.get("git_index") != index:
            findings.append("candidate identity changed")
        for snapshot in (left, right):
            for changed in _status_paths((snapshot.get("git") or {}).get("status", [])):
                if not _declared_path(changed, allowed_paths):
                    findings.append(f"undeclared candidate write: {changed}")
    before_refs, after_refs = set(before.get("refs", [])), set(after.get("refs", []))
    changed_refs = before_refs ^ after_refs
    candidate_ref = f"refs/heads/{branch}|"
    if any(not ref.startswith(candidate_ref) for ref in changed_refs):
        findings.append("non-candidate ref changed")
    before_reflog, after_reflog = before.get("reflog", []), after.get("reflog", [])
    if not isinstance(before_reflog, list) or not isinstance(after_reflog, list):
        findings.append("reflog snapshot is invalid")
    else:
        additions = _reflog_additions(before_reflog, after_reflog)
        if len(after_reflog) < len(before_reflog) or any(not isinstance(item, str) for item in before_reflog):
            findings.append("reflog history removed or invalid")
        for entry in additions:
            action = _candidate_reflog_action(entry, branch=branch, index=index)
            if action is None:
                findings.append("non-candidate reflog changed")
            elif action != "commit":
                findings.append("candidate reflog action is not an allowed forward commit")
    if worktree in before_trees and worktree in after_trees and before_trees[worktree].get("head") != after_trees[worktree].get("head") and Path(worktree).is_dir():
        ancestor = subprocess.run(["git", "-C", worktree, "merge-base", "--is-ancestor", str(before_trees[worktree].get("head")), str(after_trees[worktree].get("head"))], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
        if ancestor.returncode:
            findings.append("candidate identity rewrote history")
        diff = subprocess.run(["git", "-C", worktree, "diff", "--name-status", "--no-renames", str(before_trees[worktree].get("head")), str(after_trees[worktree].get("head"))], text=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=False)
        if diff.returncode:
            findings.append("candidate committed diff unavailable")
        else:
            for line in diff.stdout.splitlines():
                fields = line.split("\t")
                for changed in fields[1:]:
                    if not _declared_path(changed, allowed_paths):
                        findings.append(f"undeclared committed candidate write: {changed}")
    receipt_proven = False
    if process_receipt is None:
        findings.append("isolation receipt is not proven")
    elif not _valid_process_receipt(process_receipt):
        findings.append("isolation receipt is invalid")
    elif process_receipt.get("completed") is True and process_receipt.get("outcome") == "clean" and process_receipt.get("timeout") is False and process_receipt.get("leak_detected") is False and process_receipt.get("cleanup_required") is False and process_receipt.get("cleanup_complete") is True and not process_receipt["surviving_pids"]:
        receipt_proven = True
    else:
        findings.append("process isolation reported non-clean outcome")
    git_findings = [item for item in findings if not item.startswith("isolation receipt")]
    return {"result": "PASS" if not findings else ("NOT_PROVEN" if not git_findings else "FAIL"), "git_result": "PASS" if not git_findings else "FAIL", "findings": findings,
            "candidate": {"worktree": worktree, "branch": branch, "index": index, "declared_paths": declared, "generated_paths": generated},
            "process_receipt": process_receipt, "process_receipt_proven": receipt_proven}


def _process_group_pids(pgid: int) -> list[int]:
    """List only a known child session's process group; never match names."""
    result = subprocess.run(["ps", "-axo", "pid=,pgid="], text=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=False)
    if result.returncode:
        return []
    return sorted({int(fields[0]) for line in result.stdout.splitlines() if len(fields := line.split()) == 2 and fields[0].isdigit() and fields[1].isdigit() and int(fields[1]) == pgid})


def run_bounded_isolation(argv: list[str], *, timeout_seconds: float) -> dict[str, Any]:
    """Run one command in a fresh owned process group and return its receipt.

    The token correlates this invocation; accounting and signalling are only by
    the exact fresh PGID.  This deliberately makes no hostile-detachment claim.
    """
    if not argv or timeout_seconds <= 0:
        raise ReliabilityError("bounded isolation requires argv and a positive timeout")
    token = secrets.token_hex(32)
    child = subprocess.Popen(argv, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, env={**os.environ, "AGENTOPS_GC_ISOLATION_TOKEN": token}, start_new_session=True)
    pgid = child.pid
    completed = False
    timed_out = False
    try:
        child.wait(timeout=timeout_seconds)
        completed = True
    except subprocess.TimeoutExpired:
        timed_out = True
    time.sleep(0.03)
    survivors = _process_group_pids(pgid)
    leaked = bool(survivors)
    cleanup_required = timed_out or leaked
    cleanup_complete = not cleanup_required
    if cleanup_required:
        try:
            os.killpg(pgid, signal.SIGTERM)
        except (ProcessLookupError, PermissionError):
            pass
        try:
            child.wait(timeout=0.25)
        except subprocess.TimeoutExpired:
            pass
        deadline = time.monotonic() + 0.5
        while _process_group_pids(pgid) and time.monotonic() < deadline:
            time.sleep(0.02)
        if _process_group_pids(pgid):
            try:
                os.killpg(pgid, signal.SIGKILL)
            except (ProcessLookupError, PermissionError):
                pass
            try:
                child.wait(timeout=0.25)
            except subprocess.TimeoutExpired:
                pass
            time.sleep(0.03)
        cleanup_complete = not _process_group_pids(pgid)
    outcome = "clean" if not cleanup_required else ("timeout_cleanup_required" if timed_out else "leak_cleanup_required")
    if not cleanup_complete:
        outcome = "cleanup_failed"
    return {"schema_version": "candidate-process-receipt.v1", "isolation_token": token, "runner_pgid": pgid,
            "completed": completed, "outcome": outcome, "timeout": timed_out, "leak_detected": leaked,
            "cleanup_required": cleanup_required, "cleanup_complete": cleanup_complete,
            "surviving_pids": _process_group_pids(pgid)}


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
                raise ReliabilityError(
                    f"action {index}: target absent from inventory: {target}"
                )
            if item.get("ownership") not in {
                "proven_agentops_marker",
                "proven_agentops_tool_prefix",
            }:
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
            if matches[0].get("ownership") != "proven_path":
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
            if matches[0].get("ownership") != "proven_path":
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
    inv.add_argument("--scan-root", action="append", default=[], type=Path)
    inv.add_argument("--path", action="append", default=[], type=Path)
    inv.add_argument("--git-repo", action="append", default=[], type=Path)
    inv.add_argument("--selected-binary", action="append", default=[], type=Path)
    inv.add_argument("--tmux-root", type=Path)
    inv.add_argument("--generated-at")
    inv.add_argument("--output", required=True, type=Path)

    validate_inv = sub.add_parser("validate-inventory")
    validate_inv.add_argument("--inventory", required=True, type=Path)

    registry = sub.add_parser("validate-registry")
    registry.add_argument("--registry", required=True, type=Path)

    fork = sub.add_parser("validate-fork-manifest")
    fork.add_argument("--manifest", required=True, type=Path)

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
            payload = inventory(
                args.dev_root,
                args.gascity_repo,
                args.generated_at,
                scan_roots=args.scan_root,
                explicit_paths=args.path,
                git_repos=args.git_repo,
                selected_binaries=args.selected_binary,
                tmux_root=args.tmux_root,
            )
            atomic_json(args.output, payload)
            result: dict[str, Any] = {
                "inventory": str(args.output),
                "inventory_digest": payload["inventory_digest"],
            }
        elif args.command == "validate-inventory":
            payload = json.loads(args.inventory.read_text(encoding="utf-8"))
            result = validate_inventory_payload(payload)
        elif args.command == "validate-registry":
            result = validate_registry(args.registry)
        elif args.command == "validate-fork-manifest":
            manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
            result = validate_fork_manifest(manifest)
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
