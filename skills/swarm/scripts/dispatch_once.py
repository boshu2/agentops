#!/usr/bin/env python3
"""One-shot dispatch through one explicitly authorized bounded child command."""

from __future__ import annotations

from collections.abc import Iterable, Mapping
import json
import os
from pathlib import Path, PurePosixPath
import signal
import subprocess
import threading
import time
from typing import Any


MAX_TIMEOUT_SECONDS = 60.0
MAX_MESSAGE_BYTES = 4 * 1024 * 1024
MAX_PACKETS = 256
MAX_SCOPE_PATHS = 256


class ExecutorPolicy:
    """An exact command plus finite limits authorized by the caller."""

    def __init__(
        self,
        *,
        workspace_root: str | os.PathLike[str],
        argv: list[str] | tuple[str, ...],
        cwd: str = ".",
        timeout_seconds: float,
        max_input_bytes: int,
        max_output_bytes: int,
        max_packets: int,
    ) -> None:
        raw_root = Path(workspace_root)
        if not raw_root.is_absolute():
            raise ValueError("workspace_root must be absolute")
        _reject_symlinks(raw_root)
        root = raw_root.resolve(strict=True)
        if not root.is_dir():
            raise ValueError("workspace_root must be a directory")
        if not isinstance(argv, (list, tuple)) or not argv or not all(
            isinstance(item, str) and item for item in argv
        ):
            raise ValueError("argv must be a nonempty string list")
        executable_path = Path(argv[0])
        if not executable_path.is_absolute():
            raise ValueError("executor executable must be absolute")
        if executable_path.is_symlink():
            raise ValueError("executor executable must not be a symlink")
        executable = executable_path.resolve(strict=True)
        if not executable.is_file() or not os.access(executable, os.X_OK):
            raise ValueError("executor executable is not executable")
        relative_cwd = PurePosixPath(cwd.replace("\\", "/"))
        if relative_cwd.is_absolute() or ".." in relative_cwd.parts:
            raise ValueError("executor cwd must stay inside workspace_root")
        candidate_cwd = root.joinpath(*relative_cwd.parts)
        _reject_symlinks(candidate_cwd, stop=root)
        resolved_cwd = candidate_cwd.resolve(strict=True)
        if not resolved_cwd.is_dir() or not _within(resolved_cwd, root):
            raise ValueError("executor cwd must stay inside workspace_root")
        if not (0 < timeout_seconds <= MAX_TIMEOUT_SECONDS):
            raise ValueError("timeout_seconds is outside the supported finite range")
        if not (0 < max_input_bytes <= MAX_MESSAGE_BYTES):
            raise ValueError("max_input_bytes is outside the supported finite range")
        if not (0 < max_output_bytes <= MAX_MESSAGE_BYTES):
            raise ValueError("max_output_bytes is outside the supported finite range")
        if not (0 < max_packets <= MAX_PACKETS):
            raise ValueError("max_packets is outside the supported finite range")

        self.workspace_root = root
        self.argv = (str(executable), *tuple(argv[1:]))
        self.cwd = resolved_cwd
        self.timeout_seconds = float(timeout_seconds)
        self.max_input_bytes = int(max_input_bytes)
        self.max_output_bytes = int(max_output_bytes)
        self.max_packets = int(max_packets)


def _within(path: Path, root: Path) -> bool:
    try:
        path.relative_to(root)
    except ValueError:
        return False
    return True


def _reject_symlinks(path: Path, *, stop: Path | None = None) -> None:
    boundary = stop.resolve(strict=True) if stop is not None else None
    current = path
    while True:
        if current.is_symlink():
            raise ValueError(f"symlink path is not allowed: {path}")
        if boundary is not None and current == boundary:
            return
        if current.parent == current:
            return
        current = current.parent


def _packet_id(packet: Mapping[str, Any]) -> str:
    value = packet.get("packet_id", packet.get("id"))
    if not isinstance(value, str) or not value.strip() or len(value.encode()) > 256:
        raise ValueError("each packet needs a bounded nonempty packet_id")
    return value


def _scope_prefix(value: str) -> PurePosixPath:
    parts: list[str] = []
    for part in PurePosixPath(value).parts:
        if any(character in part for character in "*?["):
            break
        parts.append(part)
    return PurePosixPath(*parts) if parts else PurePosixPath(".")


def _includes(packet: Mapping[str, Any], workspace_root: Path) -> tuple[str, ...]:
    scope = packet.get("write_scope")
    if not isinstance(scope, Mapping):
        raise ValueError(f"{_packet_id(packet)}: write_scope must be an object")
    if scope.get("exclude"):
        raise ValueError(f"{_packet_id(packet)}: write_scope.exclude is not honored")
    raw = scope.get("include")
    if not isinstance(raw, list) or not raw or len(raw) > MAX_SCOPE_PATHS:
        raise ValueError(f"{_packet_id(packet)}: write_scope.include has an invalid size")
    normalized: list[str] = []
    for value in raw:
        if (
            not isinstance(value, str)
            or not value.strip()
            or len(value.encode("utf-8")) > 4096
        ):
            raise ValueError(f"{_packet_id(packet)}: include paths must be bounded strings")
        replaced = value.replace("\\", "/")
        path = PurePosixPath(replaced)
        canonical = path.as_posix().rstrip("/")
        if (
            path.is_absolute()
            or ".." in path.parts
            or canonical in {"", "."}
            or canonical != replaced.rstrip("/")
        ):
            raise ValueError(f"{_packet_id(packet)}: include path must be canonical and workspace-relative")
        prefix = _scope_prefix(canonical)
        candidate = workspace_root.joinpath(*prefix.parts)
        _reject_symlinks(candidate, stop=workspace_root)
        # Resolve the deepest existing ancestor.  This protects future output
        # paths as well as already-existing paths from symlink escapes.
        ancestor = candidate
        while not ancestor.exists() and ancestor != workspace_root:
            ancestor = ancestor.parent
        if not _within(ancestor.resolve(strict=True), workspace_root):
            raise ValueError(f"{_packet_id(packet)}: include path escapes workspace_root")
        normalized.append(canonical)
    return tuple(normalized)


def _overlap(left: str, right: str) -> bool:
    left_prefix = _scope_prefix(left).as_posix().casefold()
    right_prefix = _scope_prefix(right).as_posix().casefold()
    if left_prefix == "." or right_prefix == ".":
        return True
    return (
        left_prefix == right_prefix
        or left_prefix.startswith(right_prefix + "/")
        or right_prefix.startswith(left_prefix + "/")
    )


def _kill_group(process: subprocess.Popen[bytes]) -> None:
    try:
        os.killpg(process.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass


def _execute(packet: Mapping[str, Any], policy: ExecutorPolicy) -> Any:
    encoded = json.dumps(packet, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
    if len(encoded) > policy.max_input_bytes:
        raise ValueError("packet input exceeds max_input_bytes")
    process = subprocess.Popen(
        policy.argv,
        cwd=policy.cwd,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        start_new_session=True,
        env={"PATH": os.defpath, "LANG": "C", "LC_ALL": "C"},
    )
    assert process.stdin and process.stdout and process.stderr
    stdout = bytearray()
    total = 0
    lock = threading.Lock()
    overflow = threading.Event()

    def feed() -> None:
        try:
            process.stdin.write(encoded)
            process.stdin.close()
        except (BrokenPipeError, OSError):
            pass

    def drain(stream: Any, *, retain: bool) -> None:
        nonlocal total
        while chunk := stream.read(8192):
            with lock:
                total += len(chunk)
                if total > policy.max_output_bytes:
                    overflow.set()
                if retain and len(stdout) <= policy.max_output_bytes:
                    stdout.extend(chunk[: max(0, policy.max_output_bytes + 1 - len(stdout))])

    threads = [
        threading.Thread(target=feed, daemon=True),
        threading.Thread(target=drain, args=(process.stdout,), kwargs={"retain": True}, daemon=True),
        threading.Thread(target=drain, args=(process.stderr,), kwargs={"retain": False}, daemon=True),
    ]
    for thread in threads:
        thread.start()
    deadline = time.monotonic() + policy.timeout_seconds
    timed_out = False
    while process.poll() is None:
        if overflow.is_set():
            break
        if time.monotonic() >= deadline:
            timed_out = True
            break
        time.sleep(0.005)
    if process.poll() is None or overflow.is_set():
        _kill_group(process)
    try:
        return_code = process.wait(timeout=1)
    except subprocess.TimeoutExpired:
        _kill_group(process)
        return_code = process.wait(timeout=1)
    _kill_group(process)
    for thread in threads:
        thread.join(timeout=1)
    for stream in (process.stdout, process.stderr):
        stream.close()
    if timed_out:
        raise TimeoutError("executor exceeded timeout_seconds")
    if overflow.is_set():
        raise ValueError("executor output exceeds max_output_bytes")
    if return_code != 0:
        raise RuntimeError(f"executor exited with status {return_code}")
    try:
        return json.loads(bytes(stdout).decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise RuntimeError("executor did not return one JSON value") from exc


def dispatch_once(
    packets: Iterable[Mapping[str, Any]],
    policy: ExecutorPolicy,
) -> list[dict[str, Any]]:
    """Validate the complete batch, then execute each packet exactly once."""
    if not isinstance(policy, ExecutorPolicy):
        raise TypeError("dispatch_once requires an ExecutorPolicy; callbacks are not accepted")
    batch = list(packets)
    if len(batch) > policy.max_packets:
        raise ValueError("packet batch exceeds max_packets")
    identities: list[str] = []
    scopes: list[tuple[str, ...]] = []
    for packet in batch:
        if not isinstance(packet, Mapping):
            raise ValueError("each packet must be an object")
        identity = _packet_id(packet)
        if identity in identities:
            raise ValueError(f"duplicate packet_id: {identity}")
        identities.append(identity)
        scopes.append(_includes(packet, policy.workspace_root))
        encoded = json.dumps(packet, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
        if len(encoded) > policy.max_input_bytes:
            raise ValueError(f"{identity}: packet input exceeds max_input_bytes")

    for left_index, left_scope in enumerate(scopes):
        for right_index in range(left_index + 1, len(scopes)):
            for left in left_scope:
                for right in scopes[right_index]:
                    if _overlap(left, right):
                        raise ValueError(
                            f"write scopes overlap: {identities[left_index]}:{left} and "
                            f"{identities[right_index]}:{right}"
                        )

    results: list[dict[str, Any]] = []
    for identity, packet in zip(identities, batch, strict=True):
        try:
            results.append({"packet_id": identity, "result": _execute(packet, policy)})
        except (RuntimeError, TimeoutError, ValueError) as exc:
            # The error is factual but executor stdout/stderr is deliberately
            # not retained; it may contain credentials or unrelated paths.
            results.append(
                {
                    "packet_id": identity,
                    "error": {"type": type(exc).__name__, "message": str(exc)},
                }
            )
    return results
