#!/usr/bin/env python3
"""Disposable-copy execution with bounded evidence and process-group cleanup."""

from __future__ import annotations

from dataclasses import dataclass, field
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time
from typing import BinaryIO


DEFAULT_TIMEOUT_SECONDS = 120.0
DEFAULT_RETAINED_BYTES = 65_536
DEFAULT_TERM_GRACE_SECONDS = 2.0
DEFAULT_KILL_GRACE_SECONDS = 2.0
EXCLUDED_NAMES = {".git", "__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache"}
EXCLUDED_PREFIXES = (
    "skills/skill-builder/ledgers",
    "skills/skill-builder/receipts",
)


def _excluded(ref: str) -> bool:
    parts = Path(ref).parts
    if any(part in EXCLUDED_NAMES for part in parts):
        return True
    return any(ref == prefix or ref.startswith(f"{prefix}/") for prefix in EXCLUDED_PREFIXES)


def _manifest(root: Path) -> dict[str, dict[str, object]]:
    entries: dict[str, dict[str, object]] = {}
    for path in sorted(root.rglob("*")):
        ref = path.relative_to(root).as_posix()
        if _excluded(ref):
            continue
        mode = path.lstat().st_mode & 0o7777
        if path.is_symlink():
            entries[ref] = {
                "kind": "symlink",
                "mode": mode,
                "target": os.readlink(path),
            }
        elif path.is_file():
            entries[ref] = {
                "kind": "file",
                "mode": mode,
                "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
            }
        elif path.is_dir():
            entries[ref] = {"kind": "directory", "mode": mode}
    return entries


def manifest_digest(manifest: dict[str, dict[str, object]]) -> str:
    payload = json.dumps(
        manifest,
        ensure_ascii=False,
        separators=(",", ":"),
        sort_keys=True,
    ).encode("utf-8")
    return hashlib.sha256(payload).hexdigest()


def changed_paths(
    before: dict[str, dict[str, object]],
    after: dict[str, dict[str, object]],
) -> list[str]:
    return sorted(
        ref
        for ref in set(before) | set(after)
        if before.get(ref) != after.get(ref)
    )


def _validate_symlinks(root: Path) -> None:
    resolved_root = root.resolve()
    for path in sorted(root.rglob("*")):
        ref = path.relative_to(root).as_posix()
        if _excluded(ref) or not path.is_symlink():
            continue
        try:
            path.resolve(strict=True).relative_to(resolved_root)
        except (OSError, ValueError) as exc:
            raise ValueError(f"repository symlink escapes or dangles: {ref}") from exc


COPY_SOURCE_ROOT = Path("/")


def _copy_ignore(directory: str, names: list[str]) -> set[str]:
    base = Path(directory)
    ignored: set[str] = set()
    for name in names:
        if name in EXCLUDED_NAMES:
            ignored.add(name)
            continue
        path = base / name
        try:
            ref = path.relative_to(COPY_SOURCE_ROOT).as_posix()
        except ValueError:
            continue
        if _excluded(ref):
            ignored.add(name)
    return ignored


@dataclass
class BoundedCapture:
    limit: int
    retained: bytearray = field(default_factory=bytearray)
    total_bytes: int = 0
    digest: object = field(default_factory=hashlib.sha256)
    lock: threading.Lock = field(default_factory=threading.Lock)

    def consume(self, chunk: bytes) -> None:
        with self.lock:
            self.total_bytes += len(chunk)
            self.digest.update(chunk)  # type: ignore[union-attr]
            remaining = max(0, self.limit - len(self.retained))
            if remaining:
                self.retained.extend(chunk[:remaining])

    def facts(self) -> dict[str, object]:
        retained = bytes(self.retained)
        return {
            "total_bytes": self.total_bytes,
            "retained_bytes": len(retained),
            "sha256": self.digest.hexdigest(),  # type: ignore[union-attr]
            "retained_sha256": hashlib.sha256(retained).hexdigest(),
            "truncated": self.total_bytes > len(retained),
        }


def _drain(stream: BinaryIO, capture: BoundedCapture) -> None:
    try:
        while True:
            chunk = stream.read(65_536)
            if not chunk:
                return
            capture.consume(chunk)
    finally:
        stream.close()


def _group_alive(process_group: int) -> bool:
    try:
        os.killpg(process_group, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    return True


def _wait_group_empty(process_group: int, grace_seconds: float) -> bool:
    deadline = time.monotonic() + grace_seconds
    while time.monotonic() < deadline:
        if not _group_alive(process_group):
            return True
        time.sleep(0.02)
    return not _group_alive(process_group)


def _cleanup_group(
    process: subprocess.Popen[bytes],
    process_group: int,
    *,
    trigger: str,
    term_grace_seconds: float,
    kill_grace_seconds: float,
) -> dict[str, object]:
    term_sent = False
    kill_sent = False
    parent_reaped = process.returncode is not None
    if _group_alive(process_group):
        try:
            os.killpg(process_group, signal.SIGTERM)
            term_sent = True
        except ProcessLookupError:
            pass
    if not parent_reaped:
        try:
            process.wait(timeout=term_grace_seconds)
            parent_reaped = True
        except subprocess.TimeoutExpired:
            pass
    group_empty = _wait_group_empty(process_group, term_grace_seconds)
    if not group_empty:
        try:
            os.killpg(process_group, signal.SIGKILL)
            kill_sent = True
        except ProcessLookupError:
            pass
        if not parent_reaped:
            try:
                process.wait(timeout=kill_grace_seconds)
                parent_reaped = True
            except subprocess.TimeoutExpired:
                pass
        group_empty = _wait_group_empty(process_group, kill_grace_seconds)
    return {
        "trigger": trigger,
        "term_sent": term_sent,
        "kill_sent": kill_sent,
        "parent_reaped": parent_reaped,
        "process_group_empty": group_empty,
        "complete": parent_reaped and group_empty,
    }


def _normal_cleanup(process: subprocess.Popen[bytes], process_group: int) -> dict[str, object]:
    try:
        process.wait(timeout=0)
        parent_reaped = True
    except subprocess.TimeoutExpired:
        parent_reaped = False
    group_empty = not _group_alive(process_group)
    return {
        "trigger": "none",
        "term_sent": False,
        "kill_sent": False,
        "parent_reaped": parent_reaped,
        "process_group_empty": group_empty,
        "complete": parent_reaped and group_empty,
    }


def _execute(
    command: list[str],
    *,
    cwd: Path,
    environment: dict[str, str],
    timeout_seconds: float,
    retained_bytes: int,
    term_grace_seconds: float,
    kill_grace_seconds: float,
) -> tuple[dict[str, object], bytes, bytes]:
    stdout_capture = BoundedCapture(retained_bytes)
    stderr_capture = BoundedCapture(retained_bytes)
    process = subprocess.Popen(
        command,
        cwd=cwd,
        env=environment,
        stdin=subprocess.DEVNULL,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        start_new_session=True,
    )
    assert process.stdout is not None
    assert process.stderr is not None
    stdout_thread = threading.Thread(
        target=_drain,
        args=(process.stdout, stdout_capture),
        daemon=True,
    )
    stderr_thread = threading.Thread(
        target=_drain,
        args=(process.stderr, stderr_capture),
        daemon=True,
    )
    stdout_thread.start()
    stderr_thread.start()
    process_group = process.pid
    timed_out = False
    interrupted = False
    deadline = time.monotonic() + timeout_seconds
    trigger = "none"
    try:
        while process.poll() is None:
            if time.monotonic() >= deadline:
                timed_out = True
                trigger = "timeout"
                break
            time.sleep(0.02)
    except KeyboardInterrupt:
        interrupted = True
        trigger = "interrupted"
    if trigger == "none" and _group_alive(process_group):
        trigger = "descendants"
    cleanup = (
        _normal_cleanup(process, process_group)
        if trigger == "none"
        else _cleanup_group(
            process,
            process_group,
            trigger=trigger,
            term_grace_seconds=term_grace_seconds,
            kill_grace_seconds=kill_grace_seconds,
        )
    )
    stdout_thread.join(timeout=kill_grace_seconds)
    stderr_thread.join(timeout=kill_grace_seconds)
    if stdout_thread.is_alive() or stderr_thread.is_alive():
        cleanup["complete"] = False
    returncode = process.returncode
    exit_code = returncode if returncode is not None else 124 if timed_out else 130
    facts = {
        "exit_code": exit_code,
        "timed_out": timed_out,
        "interrupted": interrupted,
        "timeout_seconds": timeout_seconds,
        "stdout": stdout_capture.facts(),
        "stderr": stderr_capture.facts(),
        "cleanup": cleanup,
    }
    return facts, bytes(stdout_capture.retained), bytes(stderr_capture.retained)


def run_isolated_command(
    repo_root: Path,
    command: list[str],
    *,
    timeout_seconds: float = DEFAULT_TIMEOUT_SECONDS,
    retained_bytes: int = DEFAULT_RETAINED_BYTES,
    term_grace_seconds: float = DEFAULT_TERM_GRACE_SECONDS,
    kill_grace_seconds: float = DEFAULT_KILL_GRACE_SECONDS,
) -> dict[str, object]:
    """Run a command in a disposable repository copy and return exact facts."""

    root = repo_root.resolve(strict=True)
    _validate_symlinks(root)
    live_before = _manifest(root)
    with tempfile.TemporaryDirectory(prefix="skill-contract-probe-") as temporary:
        temporary_root = Path(temporary)
        isolated_root = temporary_root / "repo"
        global COPY_SOURCE_ROOT
        COPY_SOURCE_ROOT = root
        shutil.copytree(
            root,
            isolated_root,
            symlinks=True,
            ignore=_copy_ignore,
        )
        initial = _manifest(isolated_root)
        home = temporary_root / "home"
        tmpdir = temporary_root / "tmp"
        home.mkdir()
        tmpdir.mkdir()
        environment = os.environ.copy()
        environment.update(
            {
                "HOME": str(home),
                "TMPDIR": str(tmpdir),
                "PYTHONDONTWRITEBYTECODE": "1",
                "PYTHONPATH": os.pathsep.join(path for path in sys.path if path),
            }
        )
        execution, stdout, stderr = _execute(
            command,
            cwd=isolated_root,
            environment=environment,
            timeout_seconds=timeout_seconds,
            retained_bytes=retained_bytes,
            term_grace_seconds=term_grace_seconds,
            kill_grace_seconds=kill_grace_seconds,
        )
        final = _manifest(isolated_root)
    live_after = _manifest(root)
    changed = changed_paths(initial, final)
    live_changed = changed_paths(live_before, live_after)
    return {
        "execution": execution,
        "isolation": {
            "kind": "disposable_repository_copy",
            "initial_manifest_sha256": manifest_digest(initial),
            "final_manifest_sha256": manifest_digest(final),
            "changed_paths": changed,
            "out_of_scope_paths": changed,
            "live_root_unchanged": not live_changed,
            "live_root_changed_paths": live_changed,
        },
        "stdout_retained": stdout,
        "stderr_retained": stderr,
    }
