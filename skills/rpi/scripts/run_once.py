#!/usr/bin/env python3
"""Run one RPI traversal through explicitly authorized bounded child commands.

The transport is deliberately data-only.  Phase implementations receive one
JSON object on stdin and must return one JSON value on stdout.  No caller
callable is ever imported or invoked in this process.
"""

from __future__ import annotations

from collections.abc import Mapping
import json
import os
from pathlib import Path, PurePosixPath
import re
import signal
import subprocess
import threading
import time
from types import MappingProxyType
from typing import Any


DIGEST_PATTERN = re.compile(r"^[0-9a-f]{64}$")
PHASES = ("anti_ceremony", "plan", "implement", "validate")
MAX_TIMEOUT_SECONDS = 60.0
MAX_MESSAGE_BYTES = 4 * 1024 * 1024


class PhaseExecutionError(RuntimeError):
    """A bounded phase failed without exposing its potentially sensitive output."""


class ExecutionPolicy:
    """Immutable, caller-authorized command set and finite execution limits."""

    __slots__ = (
        "workspace_root",
        "commands",
        "timeout_seconds",
        "max_input_bytes",
        "max_output_bytes",
        "_frozen",
    )

    def __setattr__(self, name: str, value: Any) -> None:
        if getattr(self, "_frozen", False):
            raise AttributeError("ExecutionPolicy is immutable")
        object.__setattr__(self, name, value)

    def __init__(
        self,
        *,
        workspace_root: str | os.PathLike[str],
        commands: Mapping[str, Mapping[str, Any]],
        timeout_seconds: float,
        max_input_bytes: int,
        max_output_bytes: int,
    ) -> None:
        root_path = Path(workspace_root)
        if not root_path.is_absolute():
            raise ValueError("workspace_root must be absolute")
        _reject_symlink_components(root_path)
        root = root_path.resolve(strict=True)
        if not root.is_dir():
            raise ValueError("workspace_root must be a directory")
        if not (0 < timeout_seconds <= MAX_TIMEOUT_SECONDS):
            raise ValueError(f"timeout_seconds must be in (0, {MAX_TIMEOUT_SECONDS:g}]")
        if not (0 < max_input_bytes <= MAX_MESSAGE_BYTES):
            raise ValueError(f"max_input_bytes must be in [1, {MAX_MESSAGE_BYTES}]")
        if not (0 < max_output_bytes <= MAX_MESSAGE_BYTES):
            raise ValueError(f"max_output_bytes must be in [1, {MAX_MESSAGE_BYTES}]")
        if set(commands) != set(PHASES):
            raise ValueError("commands must define exactly anti_ceremony, plan, implement, validate")

        normalized: dict[str, tuple[tuple[str, ...], Path]] = {}
        for phase in PHASES:
            raw = commands[phase]
            if not isinstance(raw, Mapping):
                raise TypeError(f"{phase} command must be a mapping")
            if set(raw) - {"argv", "cwd"}:
                raise ValueError(f"{phase} command has unsupported fields")
            argv_value = raw.get("argv")
            if (
                not isinstance(argv_value, (list, tuple))
                or not argv_value
                or not all(isinstance(item, str) and item for item in argv_value)
            ):
                raise ValueError(f"{phase} argv must be a nonempty string list")
            executable_path = Path(argv_value[0])
            if not executable_path.is_absolute():
                raise ValueError(f"{phase} executable must be absolute")
            if executable_path.is_symlink():
                raise ValueError(f"{phase} executable must not be a symlink")
            executable = executable_path.resolve(strict=True)
            if not executable.is_file() or not os.access(executable, os.X_OK):
                raise ValueError(f"{phase} executable is not an executable file")

            raw_cwd = raw.get("cwd", ".")
            if not isinstance(raw_cwd, str) or not raw_cwd:
                raise ValueError(f"{phase} cwd must be a workspace-relative string")
            relative = PurePosixPath(raw_cwd.replace("\\", "/"))
            if relative.is_absolute() or ".." in relative.parts:
                raise ValueError(f"{phase} cwd must stay inside workspace_root")
            candidate = root.joinpath(*relative.parts)
            _reject_symlink_components(candidate, stop=root)
            cwd = candidate.resolve(strict=True)
            if not cwd.is_dir() or not _within(cwd, root):
                raise ValueError(f"{phase} cwd must stay inside workspace_root")
            normalized[phase] = ((str(executable), *tuple(argv_value[1:])), cwd)

        self.workspace_root = root
        self.commands = MappingProxyType(normalized)
        self.timeout_seconds = float(timeout_seconds)
        self.max_input_bytes = int(max_input_bytes)
        self.max_output_bytes = int(max_output_bytes)
        self._frozen = True


def _within(path: Path, root: Path) -> bool:
    try:
        path.relative_to(root)
    except ValueError:
        return False
    return True


def _reject_symlink_components(path: Path, *, stop: Path | None = None) -> None:
    """Reject an existing symlink in the inspected path, including its leaf."""
    current = path
    boundary = stop.resolve(strict=True) if stop is not None else None
    while True:
        if current.exists() or current.is_symlink():
            if current.is_symlink():
                raise ValueError(f"symlink path is not allowed: {path}")
        if boundary is not None and current == boundary:
            return
        if current.parent == current:
            return
        current = current.parent


def valid_digest(value: Any) -> bool:
    return isinstance(value, str) and bool(DIGEST_PATTERN.fullmatch(value))


def valid_string_list(value: Any) -> bool:
    return isinstance(value, list) and all(
        isinstance(item, str) and bool(item.strip()) for item in value
    )


def guard_result(value: Any) -> dict[str, Any]:
    if not isinstance(value, Mapping):
        raise ValueError("anti-ceremony guard must return a mapping")
    result = dict(value)
    expected = {
        "decision",
        "reason",
        "frozen_outcome",
        "parked_process_work",
        "remaining_proof",
        "stop_condition",
    }
    if set(result) != expected:
        raise ValueError("anti-ceremony guard returned the wrong fields")
    if result["decision"] not in {"CONTINUE", "STOP"}:
        raise ValueError("anti-ceremony decision must be CONTINUE or STOP")
    reason = result["reason"]
    if (
        not isinstance(reason, str)
        or not reason.strip()
        or "\n" in reason
        or reason[-1] not in ".!?"
        or sum(reason.count(mark) for mark in ".!?") != 1
    ):
        raise ValueError("anti-ceremony reason must be exactly one sentence")
    if not isinstance(result["frozen_outcome"], str) or not result["frozen_outcome"].strip():
        raise ValueError("anti-ceremony frozen_outcome must be a nonempty string")
    if not valid_string_list(result["parked_process_work"]):
        raise ValueError("anti-ceremony parked_process_work must be a string list")
    if not valid_string_list(result["remaining_proof"]):
        raise ValueError("anti-ceremony remaining_proof must be a string list")
    if not isinstance(result["stop_condition"], str) or not result["stop_condition"].strip():
        raise ValueError("anti-ceremony stop_condition must be a nonempty string")
    return result


def report(
    status: str,
    *,
    intent_ref: str | None = None,
    acceptance_digest: str | None = None,
    subject_digest: str | None = None,
    verdict_ref: str | None = None,
    verdict_digest: str | None = None,
    checked: list[str] | None = None,
    not_checked: list[str] | None = None,
) -> dict[str, Any]:
    return {
        "schema_version": "rpi-report.v1",
        "status": status,
        "intent_ref": intent_ref,
        "acceptance_digest": acceptance_digest,
        "subject_manifest_digest": subject_digest,
        "verdict_ref": verdict_ref,
        "verdict_digest": verdict_digest,
        "checked": checked or [],
        "not_checked": not_checked or [],
    }


def _kill_group(process: subprocess.Popen[bytes]) -> None:
    try:
        os.killpg(process.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    except PermissionError:
        try:
            process.kill()
        except ProcessLookupError:
            pass


def _run_phase(policy: ExecutionPolicy, phase: str, payload: Mapping[str, Any]) -> Any:
    try:
        encoded = json.dumps(payload, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
    except (TypeError, ValueError) as exc:
        raise TypeError(f"{phase} input must be JSON-serializable") from exc
    if len(encoded) > policy.max_input_bytes:
        raise ValueError(f"{phase} input exceeds max_input_bytes")

    argv, cwd = policy.commands[phase]
    try:
        process = subprocess.Popen(
            argv,
            cwd=cwd,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            start_new_session=True,
            env={"PATH": os.defpath, "LANG": "C", "LC_ALL": "C"},
        )
    except OSError as exc:
        raise PhaseExecutionError(f"{phase} could not start") from exc
    assert process.stdin and process.stdout and process.stderr
    stdout = bytearray()
    total = 0
    output_lock = threading.Lock()
    overflow = threading.Event()

    def feed() -> None:
        try:
            process.stdin.write(encoded)
        except (BrokenPipeError, OSError):
            pass
        finally:
            process.stdin.close()

    def drain(stream: Any, *, retain: bool) -> None:
        nonlocal total
        while chunk := stream.read(8192):
            with output_lock:
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
    # A phase that exits after spawning children is still finished.  Kill the
    # entire dedicated process group so inherited pipe handles cannot linger.
    _kill_group(process)
    for thread in threads:
        thread.join(timeout=1)
    process.stdout.close()
    process.stderr.close()

    if timed_out:
        raise TimeoutError(f"{phase} exceeded timeout_seconds")
    if overflow.is_set():
        raise ValueError(f"{phase} output exceeds max_output_bytes")
    if return_code != 0:
        raise PhaseExecutionError(f"{phase} exited with status {return_code}")
    try:
        return json.loads(bytes(stdout).decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise PhaseExecutionError(f"{phase} did not return one JSON value") from exc


def invoke_once(intent: Any, policy: ExecutionPolicy) -> dict[str, Any]:
    """Invoke each explicitly authorized child phase at most once."""
    if not isinstance(policy, ExecutionPolicy):
        raise TypeError("invoke_once requires an ExecutionPolicy; callables are not accepted")

    admission = guard_result(_run_phase(policy, "anti_ceremony", {"intent": intent}))
    if admission["decision"] == "STOP":
        return report(
            "NOT_PLANNED",
            checked=[f"anti-ceremony guard: STOP — {admission['reason']}"],
            not_checked=["plan", "implement", "validate"],
        )

    planned = _run_phase(policy, "plan", {"intent": intent})
    if planned is None:
        return report("NOT_PLANNED", not_checked=["implement", "validate"])
    if not isinstance(planned, Mapping):
        raise ValueError("Plan must return a mapping or null")
    resolved_intent = dict(planned)
    intent_ref = resolved_intent.get("intent_ref")
    if not isinstance(intent_ref, str) or not intent_ref:
        intent_ref = "caller"
    acceptance_digest = resolved_intent.get("acceptance_digest")
    if not valid_digest(acceptance_digest):
        raise ValueError(
            "Plan must declare acceptance_digest as the SHA-256 of the exact resolved intent bytes"
        )

    built = _run_phase(policy, "implement", {"resolved_intent": resolved_intent})
    if built is None:
        return report(
            "NOT_BUILT",
            intent_ref=intent_ref,
            acceptance_digest=acceptance_digest,
            checked=["plan"],
            not_checked=["validate"],
        )
    if not isinstance(built, Mapping):
        raise ValueError("Implement must return a mapping or null")
    subject = dict(built)

    raw_validation = _run_phase(
        policy,
        "validate",
        {"resolved_intent": resolved_intent, "subject": subject},
    )
    if not isinstance(raw_validation, Mapping):
        raise ValueError("Validate must return a mapping")
    validation = dict(raw_validation)
    status = validation.get("verdict")
    if status not in {"PASS", "FAIL", "NOT_PROVEN"}:
        raise ValueError("Validate must return PASS, FAIL, or NOT_PROVEN")
    if validation.get("acceptance_digest") != acceptance_digest:
        raise ValueError("Validate verdict does not match the resolved intent digest")
    subject_digest = validation.get("subject_manifest_digest")
    if not valid_digest(subject_digest):
        raise ValueError("Validate must return the exact subject manifest digest")
    candidate_digest = subject.get("subject_manifest_digest")
    if candidate_digest is not None and subject_digest != candidate_digest:
        raise ValueError("Validate result does not match the implemented subject digest")
    author_context_id = validation.get("author_context_id")
    validator_context_id = validation.get("validator_context_id")
    freshness = validation.get("freshness_attestation")
    if (
        not isinstance(author_context_id, str)
        or not author_context_id
        or not isinstance(validator_context_id, str)
        or not validator_context_id
        or author_context_id == validator_context_id
        or not isinstance(freshness, Mapping)
        or freshness.get("source") not in {"runtime", "caller"}
        or not isinstance(freshness.get("attester_identity"), str)
        or not freshness.get("attester_identity")
    ):
        raise ValueError("Validate must return distinct context identities and explicit freshness")
    verdict_digest = validation.get("verdict_digest")
    verdict_ref = validation.get("verdict_ref")
    if (verdict_digest is None) != (verdict_ref is None):
        raise ValueError("Validate must return both verdict_ref and verdict_digest when persisted")
    if verdict_ref is not None and (
        not isinstance(verdict_ref, str)
        or not verdict_ref
        or not valid_digest(verdict_digest)
    ):
        raise ValueError("Persisted verdict identity is invalid")
    return report(
        status,
        intent_ref=intent_ref,
        acceptance_digest=acceptance_digest,
        subject_digest=subject_digest,
        verdict_ref=verdict_ref,
        verdict_digest=verdict_digest,
        checked=list(validation.get("checked") or []),
        not_checked=list(validation.get("not_checked") or []),
    )
