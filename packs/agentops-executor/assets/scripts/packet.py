#!/usr/bin/env python3
"""Strict, single-dispatch Gas City adapter for AgentOps execution packets.

This helper owns transport validation and factual runtime receipts only. It has
no retry, planning, semantic-verdict, Git, integration, release, or delivery
authority.
"""

from __future__ import annotations

import argparse
import fcntl
import hashlib
import importlib.util
import json
import os
from pathlib import Path, PurePosixPath
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
import time
from typing import Any


sys.dont_write_bytecode = True

PACK_ROOT = Path(__file__).resolve().parents[2]
ENVELOPE_SCHEMA = PACK_ROOT / "assets" / "schemas" / "gc-execution-envelope.v1.schema.json"
PROJECTION_MANIFEST = PACK_ROOT / "assets" / "generated-skill-manifest.json"
VALIDATE_HELPER = PACK_ROOT / "agents" / "validator" / "skills" / "validate" / "scripts" / "validate.py"
PACKET_ID_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$")
DIGEST_RE = re.compile(r"^[0-9a-f]{64}$")
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
COMMON_EXCLUDES = [
    ".git",
    "**/.git",
    ".gc",
    "**/.gc",
    ".beads",
    "**/.beads",
    ".agents",
    "**/.agents",
    ".codex",
    "**/.codex",
    ".claude",
    "**/.claude",
    "__pycache__",
    "**/__pycache__",
    "*.pyc",
]

# Dynamic rig registration and reload can legitimately leave GC's durable
# inventory reads waiting on the managed Dolt control plane for longer than the
# CLI's ordinary interactive latency.  Keep these calls bounded, but do not
# reject an otherwise healthy packet at the old 30-second threshold.
CONTROL_PLANE_TIMEOUT_SECONDS = 120
MINIMUM_GC_VERSION = (1, 3, 5)
ALLOWED_KEYS = {
    "schema_version",
    "packet_id",
    "role",
    "provider",
    "intent_source",
    "intent_digest",
    "workspace",
    "subject",
    "write_scope",
    "evidence_dir",
    "result_path",
    "baseline_manifest",
    "subject_manifest",
    "scope_receipt",
    "author_context_id",
}
RESPONSE_KEYS = {
    "schema_version",
    "packet_id",
    "role",
    "outcome",
    "transport_bead_id",
    "session_context_id",
    "session_name",
    "template",
    "artifacts",
    "message",
}
ARTIFACT_KEYS = {"path", "sha256"}
ROLE_MODELS = {
    ("implement", "codex"): ("gpt-5.6-terra", "gpt-5.6-terra"),
    ("validate", "codex"): ("gpt-5.6-sol", "gpt-5.6-sol"),
    ("implement", "claude"): ("opus-4.8", "claude-opus-4-8"),
    ("validate", "claude"): ("opus-4.8", "claude-opus-4-8"),
}


class PacketError(RuntimeError):
    """A fail-closed envelope or transport error."""

    def __init__(self, code: str, message: str):
        super().__init__(message)
        self.code = code


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode("utf-8")


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: Path) -> str:
    return sha256_bytes(path.read_bytes())


def load_object(path: Path, label: str) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise PacketError("missing_file", f"{label} does not exist: {path}") from exc
    except (OSError, json.JSONDecodeError) as exc:
        raise PacketError("invalid_json", f"cannot read {label} {path}: {exc}") from exc
    if not isinstance(value, dict):
        raise PacketError("invalid_json", f"{label} must be a JSON object: {path}")
    return value


def write_json_atomic(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = json.dumps(value, indent=2, sort_keys=True) + "\n"
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def normalize_relative(raw: Any, label: str) -> str:
    if not isinstance(raw, str) or not raw:
        raise PacketError("invalid_envelope", f"{label} entries must be nonempty strings")
    path = PurePosixPath(raw.replace("\\", "/"))
    if path.is_absolute() or ".." in path.parts:
        raise PacketError("invalid_envelope", f"{label} path escapes the workspace: {raw}")
    normalized = path.as_posix().removeprefix("./")
    return normalized or "."


def absolute_path(raw: Any, label: str, *, must_exist: bool = False, directory: bool = False) -> Path:
    if not isinstance(raw, str) or not raw:
        raise PacketError("invalid_envelope", f"{label} must be a nonempty absolute path")
    path = Path(raw).expanduser()
    if not path.is_absolute():
        raise PacketError("invalid_envelope", f"{label} must be absolute: {raw}")
    path = path.resolve(strict=False)
    if must_exist and not path.exists():
        raise PacketError("missing_file", f"{label} does not exist: {path}")
    if directory and path.exists() and not path.is_dir():
        raise PacketError("invalid_envelope", f"{label} must be a directory: {path}")
    return path


def is_within(path: Path, root: Path) -> bool:
    try:
        path.relative_to(root)
        return True
    except ValueError:
        return False


def string_list(value: Any, label: str, *, nonempty: bool = False) -> list[str]:
    if not isinstance(value, list) or (nonempty and not value):
        qualifier = "nonempty " if nonempty else ""
        raise PacketError("invalid_envelope", f"{label} must be a {qualifier}array")
    normalized = [normalize_relative(item, label) for item in value]
    if len(normalized) != len(set(normalized)):
        raise PacketError("invalid_envelope", f"{label} contains duplicate paths")
    return normalized


def validate_envelope(path: Path, allow_existing_result: bool = False) -> tuple[dict[str, Any], dict[str, Path]]:
    packet = load_object(path, "packet")
    unknown = sorted(set(packet) - ALLOWED_KEYS)
    if unknown:
        raise PacketError("invalid_envelope", f"unknown envelope fields: {', '.join(unknown)}")
    required = {
        "schema_version",
        "packet_id",
        "role",
        "provider",
        "intent_source",
        "intent_digest",
        "workspace",
        "subject",
        "write_scope",
        "evidence_dir",
        "result_path",
    }
    missing = sorted(required - set(packet))
    if missing:
        raise PacketError("invalid_envelope", f"missing envelope fields: {', '.join(missing)}")
    if packet["schema_version"] != "gc-execution-envelope.v1":
        raise PacketError("invalid_envelope", "schema_version must be gc-execution-envelope.v1")
    packet_id = packet.get("packet_id")
    if not isinstance(packet_id, str) or not PACKET_ID_RE.fullmatch(packet_id):
        raise PacketError("invalid_envelope", "packet_id has an invalid shape")
    role = packet.get("role")
    if role not in {"implement", "validate"}:
        raise PacketError("invalid_envelope", "role must be implement or validate")
    provider = packet.get("provider")
    if provider not in {"codex", "claude"}:
        raise PacketError("invalid_envelope", "provider must be codex or claude")
    digest = packet.get("intent_digest")
    if not isinstance(digest, str) or not DIGEST_RE.fullmatch(digest):
        raise PacketError("invalid_envelope", "intent_digest must be a lowercase SHA-256 digest")

    workspace = absolute_path(packet["workspace"], "workspace", must_exist=True, directory=True)
    intent = absolute_path(packet["intent_source"], "intent_source", must_exist=True)
    evidence = absolute_path(packet["evidence_dir"], "evidence_dir", directory=True)
    result = absolute_path(packet["result_path"], "result_path")
    packet_resolved = path.resolve(strict=True)
    for candidate, label in ((intent, "intent_source"), (evidence, "evidence_dir"), (result, "result_path"), (packet_resolved, "packet")):
        if not is_within(candidate, workspace):
            raise PacketError("invalid_envelope", f"{label} must stay within workspace {workspace}: {candidate}")
    if not is_within(result, evidence):
        raise PacketError("invalid_envelope", "result_path must stay within evidence_dir")
    # Codex workspace-write protects every `.agents` directory as read-only,
    # including directories beneath an explicitly writable additional root.
    # Keep GC adapter artifacts in the already-excluded, gitignored `.gc`
    # plane so both supported providers can durably emit their response.
    expected_evidence = workspace / ".gc" / "agentops" / packet_id
    if evidence != expected_evidence:
        raise PacketError(
            "invalid_envelope",
            f"evidence_dir must be the canonical packet directory {expected_evidence}",
        )
    if result.exists() and not allow_existing_result:
        raise PacketError("stale_result", f"result_path already exists: {result}")
    actual_intent_digest = sha256_file(intent)
    if actual_intent_digest != digest:
        raise PacketError(
            "intent_digest_mismatch",
            f"intent digest mismatch: packet={digest} actual={actual_intent_digest}",
        )

    subject = packet.get("subject")
    if not isinstance(subject, dict) or set(subject) - {"includes", "excludes"}:
        raise PacketError("invalid_envelope", "subject must contain only includes and excludes")
    includes = string_list(subject.get("includes"), "subject.includes", nonempty=True)
    excludes = string_list(subject.get("excludes"), "subject.excludes")
    write_scope = string_list(packet.get("write_scope"), "write_scope", nonempty=(role == "implement"))
    if role == "validate" and write_scope:
        raise PacketError("invalid_envelope", "validate packet write_scope must be empty")
    packet["subject"] = {"includes": includes, "excludes": excludes}
    packet["write_scope"] = write_scope

    paths: dict[str, Path] = {
        "packet": packet_resolved,
        "workspace": workspace,
        "intent": intent,
        "evidence": evidence,
        "result": result,
    }
    role_fields = {"baseline_manifest", "subject_manifest", "scope_receipt", "author_context_id"}
    if role == "implement":
        present = sorted(role_fields & set(packet))
        if present:
            raise PacketError("invalid_envelope", f"implement packet cannot supply: {', '.join(present)}")
    else:
        missing_role = sorted(role_fields - set(packet))
        if missing_role:
            raise PacketError("invalid_envelope", f"validate packet missing: {', '.join(missing_role)}")
        author = packet.get("author_context_id")
        if not isinstance(author, str) or not author.strip():
            raise PacketError("invalid_envelope", "author_context_id must be nonempty")
        manifest_values: dict[str, dict[str, Any]] = {}
        for field in ("baseline_manifest", "subject_manifest"):
            manifest = absolute_path(packet[field], field, must_exist=True)
            if not is_within(manifest, workspace):
                raise PacketError("invalid_envelope", f"{field} must stay within workspace")
            value = load_object(manifest, field)
            expected_roots = sorted(packet["subject"]["includes"])
            expected_exclusions = sorted(set(packet["subject"]["excludes"] + COMMON_EXCLUDES))
            if sorted(value.get("declared_roots", [])) != expected_roots:
                raise PacketError(
                    "subject_declaration_mismatch",
                    f"{field} declared_roots do not match packet subject.includes",
                )
            if sorted(value.get("exclusions", [])) != expected_exclusions:
                raise PacketError(
                    "subject_declaration_mismatch",
                    f"{field} exclusions do not match packet subject.excludes plus runtime exclusions",
                )
            paths[field] = manifest
            manifest_values[field] = value
        scope_path = absolute_path(packet["scope_receipt"], "scope_receipt", must_exist=True)
        if not is_within(scope_path, workspace):
            raise PacketError("invalid_envelope", "scope_receipt must stay within workspace")
        receipt = load_object(scope_path, "scope_receipt")
        schema_version = receipt.get("schema_version")
        v1_fields = {
            "schema_version", "packet_id", "role", "status", "write_scope",
            "actual_changed_paths", "outside_scope",
        }
        v2_fields = v1_fields | {
            "workspace_base_sha", "subject_changed_paths", "outside_subject",
        }
        expected_fields = v2_fields if schema_version == "gc-scope-receipt.v2" else v1_fields
        if set(receipt) != expected_fields:
            raise PacketError("invalid_scope_receipt", "scope_receipt fields are not exact")
        if schema_version not in {"gc-scope-receipt.v1", "gc-scope-receipt.v2"} or receipt.get("role") != "implement":
            raise PacketError("invalid_scope_receipt", "scope_receipt identity is invalid")
        receipt_scope = string_list(receipt.get("write_scope"), "scope_receipt.write_scope", nonempty=True)
        subject_changes = changed_paths(manifest_values["baseline_manifest"], manifest_values["subject_manifest"])
        receipt_packet = {
            "packet_id": receipt.get("packet_id"), "role": "implement",
            "write_scope": receipt_scope, "subject": packet["subject"],
        }
        if schema_version == "gc-scope-receipt.v2":
            base_sha = str(receipt.get("workspace_base_sha", ""))
            if not SHA_RE.fullmatch(base_sha):
                raise PacketError("invalid_scope_receipt", "scope_receipt workspace_base_sha is invalid")
            expected_receipt = make_scope_receipt(
                receipt_packet,
                subject_changes,
                workspace_changes=git_workspace_changes(workspace, base_sha),
                workspace_base_sha=base_sha,
            )
        else:
            expected_receipt = make_scope_receipt(receipt_packet, subject_changes)
        if receipt != expected_receipt:
            raise PacketError("invalid_scope_receipt", "scope_receipt does not match the supplied manifests and write scope")
        paths["scope_receipt"] = scope_path
    return packet, paths


def run_process(argv: list[str], *, input_text: str | None = None, timeout: float | None = None) -> subprocess.CompletedProcess[str]:
    try:
        return subprocess.run(
            argv,
            input=input_text,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=timeout,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise PacketError("process_error", f"cannot run {' '.join(argv)}: {exc}") from exc


def require_process(argv: list[str], *, input_text: str | None = None, timeout: float | None = None) -> str:
    completed = run_process(argv, input_text=input_text, timeout=timeout)
    if completed.returncode != 0:
        detail = completed.stderr.strip() or completed.stdout.strip() or f"exit {completed.returncode}"
        raise PacketError("command_failed", f"{' '.join(argv)}: {detail}")
    return completed.stdout


def manifest_args(packet: dict[str, Any], paths: dict[str, Path], output: Path, base: Path | None = None) -> list[str]:
    if not VALIDATE_HELPER.is_file():
        raise PacketError("projection_missing", f"generated validate helper is missing: {VALIDATE_HELPER}")
    args = [sys.executable, str(VALIDATE_HELPER), "manifest", "--root", str(paths["workspace"])]
    for include in packet["subject"]["includes"]:
        args += ["--include", include]
    excludes = sorted(set(packet["subject"]["excludes"] + COMMON_EXCLUDES))
    for exclude in excludes:
        args += ["--exclude", exclude]
    if base is not None:
        args += ["--base-manifest", str(base)]
    args += ["--output", str(output)]
    return args


def build_manifest(packet: dict[str, Any], paths: dict[str, Path], output: Path, base: Path | None = None) -> dict[str, Any]:
    require_process(manifest_args(packet, paths, output, base), timeout=120)
    return load_object(output, "subject manifest")


def verify_manifest(paths: dict[str, Path]) -> tuple[bool, str]:
    args = [
        sys.executable,
        str(VALIDATE_HELPER),
        "verify-manifest",
        "--root",
        str(paths["workspace"]),
        "--manifest",
        str(paths["subject_manifest"]),
        "--base-manifest",
        str(paths["baseline_manifest"]),
    ]
    completed = run_process(args, timeout=120)
    message = (completed.stdout.strip() or completed.stderr.strip() or f"exit {completed.returncode}")
    return completed.returncode == 0, message


def entry_identity(entry: dict[str, Any]) -> tuple[Any, ...]:
    return (entry.get("kind"), entry.get("executable"), entry.get("digest"))


def changed_paths(baseline: dict[str, Any], subject: dict[str, Any]) -> list[str]:
    before = {str(item.get("path")): item for item in baseline.get("entries", []) if isinstance(item, dict)}
    changed: set[str] = set()
    for item in subject.get("entries", []):
        if not isinstance(item, dict) or not item.get("path"):
            continue
        path = str(item["path"])
        if item.get("kind") == "deletion" or path not in before or entry_identity(item) != entry_identity(before[path]):
            changed.add(path)
    after_paths = {str(item.get("path")) for item in subject.get("entries", []) if isinstance(item, dict)}
    changed.update(path for path in before if path not in after_paths)
    return sorted(changed)


def path_matches(path: str, pattern: str) -> bool:
    import fnmatch

    if pattern == ".":
        return True
    if any(character in pattern for character in "*?["):
        return fnmatch.fnmatchcase(path, pattern)
    return path == pattern or path.startswith(pattern.rstrip("/") + "/")


def git_workspace_head(workspace: Path) -> str:
    completed = run_process(["git", "-C", str(workspace), "rev-parse", "HEAD"], timeout=30)
    value = completed.stdout.strip()
    if completed.returncode != 0 or not SHA_RE.fullmatch(value):
        detail = completed.stderr.strip() or completed.stdout.strip()
        raise PacketError("git_identity", f"cannot resolve workspace HEAD: {detail}")
    return value


def git_workspace_changes(workspace: Path, base_sha: str) -> list[str]:
    if not SHA_RE.fullmatch(base_sha):
        raise PacketError("git_identity", f"invalid workspace base SHA: {base_sha!r}")
    commands = (
        ["git", "-C", str(workspace), "diff", "--name-only", "-z", base_sha, "--"],
        ["git", "-C", str(workspace), "ls-files", "--others", "--exclude-standard", "-z"],
    )
    changes: set[str] = set()
    for command in commands:
        completed = run_process(command, timeout=120)
        if completed.returncode != 0:
            detail = completed.stderr.strip() or completed.stdout.strip()
            raise PacketError("git_identity", f"cannot inventory workspace changes: {detail}")
        changes.update(path for path in completed.stdout.split("\0") if path)
    return sorted(
        path for path in changes
        if not any(path_matches(path, pattern) for pattern in COMMON_EXCLUDES)
    )


def subject_matches(packet: dict[str, Any], path: str) -> bool:
    subject = packet.get("subject")
    if not isinstance(subject, dict):
        return True
    includes = subject.get("includes", [])
    excludes = subject.get("excludes", [])
    return (
        any(path_matches(path, pattern) for pattern in includes)
        and not any(path_matches(path, pattern) for pattern in excludes)
    )


def make_scope_receipt(packet: dict[str, Any], changes: list[str], *,
                       workspace_changes: list[str] | None = None,
                       workspace_base_sha: str | None = None) -> dict[str, Any]:
    actual = sorted(set(workspace_changes if workspace_changes is not None else changes))
    outside = sorted(path for path in actual if not any(path_matches(path, item) for item in packet["write_scope"]))
    outside_subject = sorted(path for path in actual if not subject_matches(packet, path))
    if outside:
        status = "FAIL"
    elif workspace_changes is not None and outside_subject:
        status = "FAIL"
    elif packet["role"] == "implement" and not changes:
        status = "NOT_PROVEN"
    elif packet["role"] == "validate" and actual:
        status = "FAIL"
    else:
        status = "PASS"
    receipt = {
        "schema_version": "gc-scope-receipt.v1",
        "packet_id": packet["packet_id"],
        "role": packet["role"],
        "status": status,
        "write_scope": packet["write_scope"],
        "actual_changed_paths": actual,
        "outside_scope": outside,
    }
    if workspace_changes is not None:
        if workspace_base_sha is None or not SHA_RE.fullmatch(workspace_base_sha):
            raise PacketError("git_identity", "workspace scope receipt requires a valid base SHA")
        receipt.update({
            "schema_version": "gc-scope-receipt.v2",
            "workspace_base_sha": workspace_base_sha,
            "subject_changed_paths": sorted(set(changes)),
            "outside_subject": outside_subject,
        })
    return receipt


def parse_json_output(raw: str, label: str) -> Any:
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        pass
    candidates = [line for line in raw.splitlines() if line.strip()]
    for line in reversed(candidates):
        try:
            return json.loads(line)
        except json.JSONDecodeError:
            continue
    raise PacketError("invalid_runtime_json", f"{label} did not return JSON: {raw[:400]}")


def bootstrap_gc_identity() -> tuple[Path, str | None] | None:
    city = os.environ.get("GC_CITY_PATH") or os.environ.get("GC_CITY")
    if not city:
        return None
    marker = Path(city) / ".gc" / "agentops-bootstrap.json"
    if not marker.is_file():
        return None
    value = load_object(marker, "AgentOps GC bootstrap marker")
    schema_version = value.get("schema_version")
    digest: str | None = None
    if schema_version == 2:
        raw_path = value.get("gc_bin", "")
    elif schema_version == 3:
        gc = value.get("toolchain", {}).get("gc", {})
        raw_path = gc.get("path", "")
        digest = gc.get("sha256")
        if not isinstance(digest, str) or not DIGEST_RE.fullmatch(digest):
            raise PacketError("gc_identity", "bootstrap marker GC digest is invalid")
    else:
        raise PacketError(
            "gc_identity",
            f"bootstrap marker schema_version is unsupported: {schema_version!r}",
        )
    candidate = Path(str(raw_path)).expanduser()
    if not candidate.is_file() or not os.access(candidate, os.X_OK):
        raise PacketError("gc_missing", f"bootstrap marker GC binary is not executable: {candidate}")
    return candidate.resolve(), digest


def gc_binary() -> str:
    managed_identity = bootstrap_gc_identity()
    configured = os.environ.get("GC_BIN")
    if configured:
        candidate = Path(configured).expanduser()
        if not candidate.is_file() or not os.access(candidate, os.X_OK):
            raise PacketError("gc_missing", f"configured GC_BIN is not executable: {candidate}")
        candidate = candidate.resolve()
        if managed_identity is not None and candidate != managed_identity[0]:
            raise PacketError(
                "gc_identity",
                f"configured GC_BIN does not match managed marker: {candidate} != {managed_identity[0]}",
            )
        if managed_identity is not None and managed_identity[1] is not None:
            actual = sha256_file(candidate)
            if actual != managed_identity[1]:
                raise PacketError(
                    "gc_identity",
                    f"configured GC_BIN digest does not match managed marker: {actual} != {managed_identity[1]}",
                )
        return str(candidate)
    if managed_identity is not None:
        candidate, digest = managed_identity
        if digest is not None:
            actual = sha256_file(candidate)
            if actual != digest:
                raise PacketError(
                    "gc_identity",
                    f"bootstrap marker GC digest mismatch: {actual} != {digest}",
                )
        return str(candidate)
    found = shutil.which("gc")
    if not found:
        raise PacketError("gc_missing", "gc binary is not available")
    return found


def local_agent_name(packet: dict[str, Any]) -> str:
    return {
        ("implement", "codex"): "implementer",
        ("validate", "codex"): "validator",
        ("implement", "claude"): "implementer-claude",
        ("validate", "claude"): "validator-claude",
    }[(packet["role"], packet["provider"])]


def selected_rig_record(rig: str) -> dict[str, Any]:
    city = os.environ.get("GC_CITY_PATH") or os.environ.get("GC_CITY")
    if not city:
        raise PacketError("city_missing", "GC_CITY_PATH is not set by the pack command runtime")
    requested_path = Path(rig).expanduser().resolve(strict=False) if Path(rig).is_absolute() else None
    deadline = time.monotonic() + CONTROL_PLANE_TIMEOUT_SECONDS
    last_matches: list[dict[str, Any]] = []
    first_attempt = True
    while True:
        remaining = deadline - time.monotonic()
        attempt_timeout = CONTROL_PLANE_TIMEOUT_SECONDS if first_attempt else max(1.0, remaining)
        first_attempt = False
        value = parse_json_output(
            require_process(
                [gc_binary(), "--city", city, "rig", "list", "--json"],
                timeout=attempt_timeout,
            ),
            "gc rig list",
        )
        records = value.get("rigs") if isinstance(value, dict) else None
        if not isinstance(records, list):
            raise PacketError("rig_missing", "gc rig list returned no rigs array")
        matches: list[dict[str, Any]] = []
        for record in records:
            if not isinstance(record, dict) or not isinstance(record.get("path"), str):
                continue
            root = Path(record["path"]).expanduser().resolve(strict=False)
            if record.get("name") == rig or (requested_path is not None and root == requested_path):
                matches.append({**record, "resolved_path": root})
        if len(matches) == 1:
            prefix = matches[0].get("prefix")
            if not isinstance(prefix, str) or not prefix:
                raise PacketError("rig_missing", f"configured rig {rig!r} has no bead prefix")
            return matches[0]
        last_matches = matches
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise PacketError(
                "rig_missing",
                f"expected exactly one configured rig matching {rig!r}; observed {len(last_matches)}",
            )
        time.sleep(min(2.0, remaining))


def packet_transport_description(packet: dict[str, Any], paths: dict[str, Path]) -> str:
    return "\n".join(
        [
            f"execution_provider={packet['provider']}",
            "GC transport only; handle exactly once and do not infer semantic completion.",
            f"adapter_path={Path(__file__).resolve()}",
            f"packet_path={paths['packet']}",
            f"packet_digest={sha256_file(paths['packet'])}",
            f"intent_digest={packet['intent_digest']}",
            f"result_path={paths['result']}",
        ]
    )


def validate_transport_identity(record: dict[str, Any], bead_id: str, title: str, description: str,
                                identity: dict[str, str]) -> None:
    if record.get("id") != bead_id or record.get("title") != title or record.get("description") != description:
        raise PacketError("transport_identity_mismatch", f"transport bead {bead_id} belongs to different work")
    meta = record.get("metadata")
    if not isinstance(meta, dict) or any(meta.get(key) != value for key, value in identity.items()):
        raise PacketError("transport_identity_mismatch", f"transport bead {bead_id} metadata changed")


def prepare_packet_transport(packet: dict[str, Any], paths: dict[str, Path], rig: str,
                             binding: str) -> tuple[str, str]:
    """Create or reconcile the deterministic packet bead without routing it."""
    city = os.environ.get("GC_CITY_PATH") or os.environ.get("GC_CITY")
    if not city:
        raise PacketError("city_missing", "GC_CITY_PATH is not set by the pack command runtime")
    rig_record = selected_rig_record(rig)
    rig_root = rig_record["resolved_path"]
    if paths["workspace"] != rig_root:
        raise PacketError(
            "workspace_rig_mismatch",
            f"packet workspace must equal configured rig root {rig_root}: {paths['workspace']}",
        )
    local_agent = local_agent_name(packet)
    target = f"{rig}/{binding}.{local_agent}"
    packet_digest = sha256_file(paths["packet"])
    bead_id = f"{rig_record['prefix']}-pkt-{packet_digest[:16]}"
    title = f"AgentOps {packet['role']} packet {packet['packet_id']}"
    description = packet_transport_description(packet, paths)
    identity = {
        "agentops.packet_id": packet["packet_id"],
        "agentops.packet_digest": packet_digest,
        "agentops.intent_digest": packet["intent_digest"],
        "agentops.target": target,
        "agentops.result_path": str(paths["result"]),
        # The dynamic-rig policy sets the native executor work_dir base to the
        # candidate's parent. Gas City's pack-workspace route then resolves the
        # ready bead directly into the already-isolated candidate root instead
        # of deriving a second bead-named sub-worktree.
        "gc.pack_workspace": paths["workspace"].name,
    }
    record = transport_record(rig, bead_id)
    if record is None:
        args = [
            gc_binary(),
            "--city",
            city,
            "bd",
            "--rig",
            rig,
            "create",
            title,
            "--id",
            bead_id,
            "--force",
            "--description",
            description,
            "--labels",
            "agentops-packet,gc-transport",
            "--metadata",
            json.dumps(identity, sort_keys=True, separators=(",", ":")),
            "--json",
        ]
        require_process(args, timeout=120)
        # Creation output is a command projection whose envelope has changed
        # across GC/bd versions. The selected rig's durable bead is the
        # authority: re-read it and verify every identity field below.
        record = transport_record(rig, bead_id)
        if record is None:
            raise PacketError("dispatch_failed", f"gc bd create succeeded but transport bead is unreadable: {bead_id}")
    validate_transport_identity(record, bead_id, title, description, identity)
    return bead_id, target


def ensure_transport_routed(rig: str, bead_id: str, target: str) -> None:
    """Route a prepared bead once, or reconcile an already-routed bead."""
    city = os.environ.get("GC_CITY_PATH") or os.environ.get("GC_CITY")
    if not city:
        raise PacketError("city_missing", "GC_CITY_PATH is not set by the pack command runtime")
    record = transport_record(rig, bead_id)
    if record is None:
        raise PacketError("transport_missing", f"prepared transport bead disappeared: {bead_id}")
    meta = record.get("metadata")
    meta = meta if isinstance(meta, dict) else {}
    routed_to = meta.get("gc.routed_to")
    if routed_to is not None:
        if routed_to != target:
            raise PacketError("transport_identity_mismatch", f"transport bead {bead_id} routed to {routed_to!r}")
        return
    if record.get("status") != "open":
        raise PacketError("transport_state_invalid", f"unrouted transport bead {bead_id} is not open")
    args = [
        gc_binary(),
        "--city",
        city,
        "sling",
        target,
        bead_id,
        "--no-formula",
        "--no-convoy",
        "--json",
    ]
    value = parse_json_output(require_process(args, timeout=120), "gc sling")
    if (
        not isinstance(value, dict)
        or not value.get("success")
        or str(value.get("bead_id", "")) != bead_id
    ):
        raise PacketError("dispatch_failed", f"gc sling did not route the packet: {value!r}")


def bead_record(value: Any, bead_id: str) -> dict[str, Any] | None:
    if isinstance(value, list):
        for item in value:
            found = bead_record(item, bead_id)
            if found:
                return found
        return None
    if not isinstance(value, dict):
        return None
    if str(value.get("id", "")) == bead_id:
        return value
    for key in ("issue", "bead", "issues", "beads", "result", "data"):
        if key in value:
            found = bead_record(value[key], bead_id)
            if found:
                return found
    return None


def transport_record(rig: str, bead_id: str) -> dict[str, Any] | None:
    city = os.environ.get("GC_CITY_PATH") or os.environ.get("GC_CITY") or ""
    args = [gc_binary(), "--city", city, "bd", "--rig", rig, "show", bead_id, "--json"]
    completed = run_process(args, timeout=CONTROL_PLANE_TIMEOUT_SECONDS)
    if completed.returncode != 0:
        return None
    try:
        value = parse_json_output(completed.stdout, "gc bd show")
    except PacketError:
        return None
    return bead_record(value, bead_id)


def wait_for_response(result_path: Path, rig: str, bead_id: str, deadline: float) -> tuple[dict[str, Any], dict[str, Any]]:
    response: dict[str, Any] | None = None
    while time.monotonic() < deadline:
        if response is None and result_path.is_file():
            response = load_object(result_path, "agent response")
        if response is not None:
            transport = transport_record(rig, bead_id)
            if transport is not None and transport.get("status") == "closed":
                return response, transport
        time.sleep(1)
    if response is None:
        raise PacketError("packet_timeout", f"packet {bead_id} produced no response before the deadline")
    raise PacketError("transport_timeout", f"packet {bead_id} wrote a response but did not close its transport bead")


def command_model(command: str) -> str:
    try:
        tokens = shlex.split(command)
    except ValueError as exc:
        raise PacketError("runtime_identity_missing", f"GC runtime command cannot be parsed: {exc}") from exc
    values = [tokens[index + 1] for index, token in enumerate(tokens[:-1]) if token in {"--model", "-m"}]
    if len(values) != 1 or not values[0].strip():
        raise PacketError("runtime_identity_missing", "GC runtime command must contain exactly one explicit model")
    return values[0].strip()


def runtime_session(context_id: str) -> dict[str, Any]:
    city = os.environ.get("GC_CITY_PATH") or os.environ.get("GC_CITY") or ""
    raw = require_process(
        [gc_binary(), "--city", city, "session", "list", "--state", "all", "--json"],
        timeout=CONTROL_PLANE_TIMEOUT_SECONDS,
    )
    value = parse_json_output(raw, "gc session list")
    sessions = value.get("sessions") if isinstance(value, dict) else None
    if not isinstance(sessions, list):
        raise PacketError("runtime_identity_missing", "gc session list returned no sessions array")
    matches = [item for item in sessions if isinstance(item, dict) and str(item.get("id", "")) == context_id]
    if len(matches) > 1:
        raise PacketError("runtime_identity_missing", f"expected one GC runtime session {context_id}, found {len(matches)}")
    live = matches[0] if matches else None

    # The durable city session bead owns the exact launch command. Use it for
    # model attestation even while the live projection exists; transcript model
    # discovery is optional and may disappear as soon as a fresh pool drains.
    raw = require_process(
        [gc_binary(), "--city", city, "bd", "show", context_id, "--json"],
        timeout=CONTROL_PLANE_TIMEOUT_SECONDS,
    )
    record = bead_record(parse_json_output(raw, "gc bd show session"), context_id)
    if (
        record is None
        or record.get("issue_type") != "session"
        or (live is None and record.get("status") != "closed")
    ):
        raise PacketError("runtime_identity_missing", f"durable city session bead is missing for {context_id}")
    meta = record.get("metadata")
    if not isinstance(meta, dict):
        raise PacketError("runtime_identity_missing", f"city session bead {context_id} has no metadata")
    fields: dict[str, str] = {}
    for field in ("provider", "template", "session_name", "state"):
        value = (live or {}).get(field) or meta.get(field)
        if not isinstance(value, str) or not value.strip():
            raise PacketError("runtime_identity_missing", f"city session bead {context_id} has no {field}")
        fields[field] = value
    launched_model = command_model(str(meta.get("command", "")))
    observed_model = (live or {}).get("model")
    if isinstance(observed_model, str) and observed_model.strip() and observed_model.strip() != launched_model:
        raise PacketError(
            "runtime_model_mismatch",
            f"GC transcript model {observed_model.strip()} does not match launched model {launched_model}",
        )
    return {
        "id": context_id,
        **fields,
        "status": record.get("status"),
        "model": launched_model,
        "model_source": "launch_command",
    }


def load_validate_module() -> Any:
    spec = importlib.util.spec_from_file_location("agentops_gc_validate_contract", VALIDATE_HELPER)
    if spec is None or spec.loader is None:
        raise PacketError("projection_missing", f"cannot load generated validate helper: {VALIDATE_HELPER}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def validate_response_artifacts(packet: dict[str, Any], paths: dict[str, Path], response: dict[str, Any]) -> None:
    artifacts = response.get("artifacts")
    if not isinstance(artifacts, list):
        raise PacketError("invalid_agent_response", "agent response artifacts must be an array")
    if response.get("outcome") != "error" and not artifacts:
        raise PacketError("invalid_agent_response", "successful agent response must contain at least one artifact")
    seen: set[Path] = set()
    loaded: list[dict[str, Any]] = []
    for index, item in enumerate(artifacts):
        if not isinstance(item, dict) or set(item) != ARTIFACT_KEYS:
            raise PacketError("invalid_agent_response", f"artifact {index} must contain exactly path and sha256")
        artifact = absolute_path(item.get("path"), f"artifact {index}", must_exist=True)
        if not is_within(artifact, paths["evidence"]):
            raise PacketError("invalid_agent_response", f"artifact is outside evidence_dir: {artifact}")
        if artifact in seen:
            raise PacketError("invalid_agent_response", f"duplicate artifact path: {artifact}")
        seen.add(artifact)
        if item.get("sha256") != sha256_file(artifact):
            raise PacketError("artifact_digest_mismatch", f"artifact digest does not match current content: {artifact}")
        loaded.append(load_object(artifact, f"artifact {index}") if packet["role"] == "validate" and response.get("outcome") == "evidence" else {})
    if packet["role"] != "validate" or response.get("outcome") != "evidence":
        return
    if len(artifacts) != 1:
        raise PacketError("invalid_agent_response", "validate evidence response must reference exactly one verdict.v2 artifact")
    verdict = loaded[0]
    try:
        load_validate_module().validate_verdict_v2(verdict)
    except Exception as exc:
        raise PacketError("invalid_verdict_artifact", f"validator artifact is not valid verdict.v2: {exc}") from exc
    session_id = response["session_context_id"]
    subject_manifest = load_object(paths["subject_manifest"], "subject_manifest")
    expected = {
        "acceptance_digest": packet["intent_digest"],
        "subject_manifest_digest": subject_manifest.get("canonical_manifest_digest"),
        "author_context_id": packet["author_context_id"],
        "validator_context_id": session_id,
    }
    for field, value in expected.items():
        if verdict.get(field) != value:
            raise PacketError("verdict_binding_mismatch", f"verdict.v2 {field} does not match runtime packet facts")
    if verdict.get("freshness_attestation") != {"source": "runtime", "attester_identity": session_id}:
        raise PacketError("verdict_binding_mismatch", "verdict.v2 freshness attestation is not bound to the GC validator session")
    scope_status = load_object(paths["scope_receipt"], "scope_receipt").get("status")
    if scope_status == "FAIL" and verdict.get("verdict") != "FAIL":
        raise PacketError("verdict_binding_mismatch", "verdict.v2 must be FAIL when runtime scope failed")
    if scope_status == "NOT_PROVEN" and verdict.get("verdict") != "NOT_PROVEN":
        raise PacketError("verdict_binding_mismatch", "verdict.v2 must be NOT_PROVEN when runtime scope is not proven")


def validate_agent_response(
    packet: dict[str, Any],
    paths: dict[str, Path],
    response: dict[str, Any],
    bead_id: str,
    transport: dict[str, Any],
    session: dict[str, Any],
    expected_template: str,
) -> None:
    expected = "candidate" if packet["role"] == "implement" else "evidence"
    if set(response) != RESPONSE_KEYS:
        unknown = sorted(set(response) - RESPONSE_KEYS)
        missing = sorted(RESPONSE_KEYS - set(response))
        raise PacketError(
            "invalid_agent_response",
            f"agent response fields are not exact; missing={missing} unknown={unknown}",
        )
    if response.get("schema_version") != "gc-agent-response.v1":
        raise PacketError("invalid_agent_response", "agent response schema_version is invalid")
    if response.get("packet_id") != packet["packet_id"] or response.get("role") != packet["role"]:
        raise PacketError("invalid_agent_response", "agent response identity does not match the packet")
    if response.get("transport_bead_id") != bead_id:
        raise PacketError("invalid_agent_response", "agent response transport bead does not match dispatch")
    if response.get("outcome") not in {expected, "error"}:
        raise PacketError("invalid_agent_response", f"{packet['role']} response must be {expected} or error")
    if not isinstance(response.get("session_context_id"), str) or not response["session_context_id"].strip():
        raise PacketError("invalid_agent_response", "agent response has no runtime session context ID")
    if packet["role"] == "validate" and response["session_context_id"] == packet.get("author_context_id"):
        raise PacketError("freshness_collision", "validator and author context IDs collide")
    if not isinstance(response.get("session_name"), str) or not response["session_name"].strip():
        raise PacketError("runtime_identity_missing", "agent response has no runtime session name")
    if transport.get("assignee") != response["session_name"]:
        raise PacketError("runtime_identity_mismatch", "transport assignee does not match agent response session name")
    if response.get("template") != expected_template:
        raise PacketError("runtime_identity_mismatch", "agent response template does not match routed target")
    runtime_expected = {
        "id": response["session_context_id"],
        "session_name": response["session_name"],
        "template": response["template"],
        "provider": packet["provider"],
    }
    for field, value in runtime_expected.items():
        if session.get(field) != value:
            raise PacketError("runtime_identity_mismatch", f"GC runtime session {field} does not match agent response")
    configured_model, launched_model = ROLE_MODELS[(packet["role"], packet["provider"])]
    if session.get("model") != launched_model:
        raise PacketError(
            "runtime_model_mismatch",
            f"GC runtime model must be {configured_model} ({launched_model}) for {packet['role']}/{packet['provider']}",
        )
    validate_response_artifacts(packet, paths, response)


def success_result(
    packet: dict[str, Any],
    paths: dict[str, Path],
    response: dict[str, Any],
    bead_id: str,
    target: str,
    evidence: dict[str, Any],
) -> dict[str, Any]:
    return {
        "schema_version": "gc-execution-result.v1",
        "ok": True,
        "command": "agentops run-packet",
        "action": "handled",
        "packet_id": packet["packet_id"],
        "role": packet["role"],
        "provider": packet["provider"],
        "outcome": response["outcome"],
        "intent_digest": packet["intent_digest"],
        "workspace": str(paths["workspace"]),
        "transport": {
            "bead_id": bead_id,
            "target": target,
            "closed": True,
            "session_context_id": response["session_context_id"],
            "session_name": response.get("session_name", ""),
        },
        "agent_response": response,
        "runtime_evidence": evidence,
    }


def error_result(error: PacketError, packet: dict[str, Any] | None = None) -> dict[str, Any]:
    return {
        "schema_version": "gc-execution-result.v1",
        "ok": False,
        "command": "agentops run-packet",
        "action": "error",
        "packet_id": str((packet or {}).get("packet_id", "unknown")),
        "role": str((packet or {}).get("role", "unknown")) if (packet or {}).get("role") in {"implement", "validate"} else "unknown",
        "provider": str((packet or {}).get("provider", "unknown")) if (packet or {}).get("provider") in {"codex", "claude"} else "unknown",
        "outcome": "error",
        "error": {"code": error.code, "message": str(error)},
    }


def load_transport_state(path: Path, packet: dict[str, Any], paths: dict[str, Path],
                         rig: str, binding: str) -> dict[str, Any]:
    state = load_object(path, "runtime transport state")
    expected_keys = {
        "schema_version", "packet_id", "packet_digest", "intent_digest",
        "rig", "binding", "bead_id", "target", "dispatch_phase", "manifest_file_digests",
    }
    if packet["role"] == "implement":
        expected_keys.add("workspace_base_sha")
    if set(state) != expected_keys or state.get("schema_version") != "gc-packet-transport.v1":
        raise PacketError("transport_state_invalid", "runtime transport state fields are invalid")
    expected = {
        "packet_id": packet["packet_id"],
        "packet_digest": sha256_file(paths["packet"]),
        "intent_digest": sha256_file(paths["intent"]),
        "rig": rig,
        "binding": binding,
    }
    for field, wanted in expected.items():
        if state.get(field) != wanted:
            raise PacketError("transport_state_mismatch", f"runtime transport {field} changed")
    if state["intent_digest"] != packet["intent_digest"]:
        raise PacketError("transport_state_mismatch", "runtime transport intent no longer matches the packet")
    if not isinstance(state.get("bead_id"), str) or not state["bead_id"]:
        raise PacketError("transport_state_invalid", "runtime transport has no bead ID")
    if not isinstance(state.get("target"), str) or not state["target"]:
        raise PacketError("transport_state_invalid", "runtime transport has no target")
    if state.get("dispatch_phase") not in {"prepared", "routed"}:
        raise PacketError("transport_state_invalid", "runtime transport has an invalid dispatch phase")
    digests = state.get("manifest_file_digests")
    if not isinstance(digests, dict) or not digests:
        raise PacketError("transport_state_invalid", "runtime transport has no manifest digests")
    for field, digest in digests.items():
        manifest_path = Path(field)
        if not manifest_path.is_file() or sha256_file(manifest_path) != digest:
            raise PacketError("manifest_mutated", f"{field} changed after transport dispatch")
    return state


def recover_runtime_result(path: Path, packet: dict[str, Any], paths: dict[str, Path],
                           rig: str) -> dict[str, Any]:
    result = load_object(path, "runtime execution result")
    if result.get("schema_version") != "gc-execution-result.v1" or result.get("ok") is not True:
        raise PacketError("runtime_result_invalid", "cached runtime result is not a successful execution result")
    expected = {
        "packet_id": packet["packet_id"], "role": packet["role"],
        "provider": packet["provider"], "intent_digest": packet["intent_digest"],
        "workspace": str(paths["workspace"]),
    }
    for field, wanted in expected.items():
        if result.get(field) != wanted:
            raise PacketError("runtime_result_mismatch", f"cached runtime result {field} changed")
    evidence = result.get("runtime_evidence")
    if not isinstance(evidence, dict) or evidence.get("packet_digest") != sha256_file(paths["packet"]):
        raise PacketError("runtime_result_mismatch", "cached runtime result packet digest changed")
    if sha256_file(paths["intent"]) != packet["intent_digest"]:
        raise PacketError("runtime_result_mismatch", "cached runtime result intent changed")
    response = load_object(paths["result"], "agent response")
    if response != result.get("agent_response"):
        raise PacketError("runtime_result_mismatch", "cached agent response differs from the durable response")
    transport_summary = result.get("transport")
    if not isinstance(transport_summary, dict):
        raise PacketError("runtime_result_invalid", "cached runtime result has no transport summary")
    bead_id = str(transport_summary.get("bead_id", ""))
    target = str(transport_summary.get("target", ""))
    transport = transport_record(rig, bead_id)
    if transport is None or transport.get("status") != "closed":
        raise PacketError("transport_state_invalid", "cached transport bead is not closed")
    session = runtime_session(str(response.get("session_context_id", "")))
    validate_agent_response(packet, paths, response, bead_id, transport, session, target)
    if packet["role"] == "implement":
        baseline_path = paths["evidence"] / "runtime-baseline-manifest.json"
        baseline = load_object(baseline_path, "runtime baseline manifest")
        recovered_subject = build_manifest(
            packet, paths, paths["evidence"] / "runtime-recovery-subject-manifest.json",
            baseline_path,
        )
        subject_changes = changed_paths(baseline, recovered_subject)
        receipt_path = absolute_path(evidence.get("scope_receipt"), "scope receipt", must_exist=True)
        stored_receipt = load_object(receipt_path, "scope receipt")
        base_sha = str(stored_receipt.get("workspace_base_sha", ""))
        receipt = make_scope_receipt(
            packet,
            subject_changes,
            workspace_changes=git_workspace_changes(paths["workspace"], base_sha),
            workspace_base_sha=base_sha,
        )
        if stored_receipt != receipt:
            raise PacketError("runtime_result_mismatch", "cached scope receipt differs from the current workspace")
        if evidence.get("actual_changed_paths") != receipt["actual_changed_paths"]:
            raise PacketError("runtime_result_mismatch", "cached changed paths differ from the current subject")
        if evidence.get("subject_manifest_digest") != recovered_subject.get("canonical_manifest_digest"):
            raise PacketError("runtime_result_mismatch", "cached subject digest differs from the current subject")
        if evidence.get("scope_status") != receipt.get("status"):
            raise PacketError("runtime_result_mismatch", "cached scope status differs from the current subject")
    else:
        matched, detail = verify_manifest(paths)
        if not matched:
            raise PacketError("runtime_result_mismatch", f"cached validation subject moved: {detail}")
    return result


def command_run(args: argparse.Namespace) -> int:
    packet: dict[str, Any] | None = None
    try:
        packet_path = absolute_path(args.packet, "packet", must_exist=True)
        raw_packet = load_object(packet_path, "packet")
        raw_evidence = absolute_path(raw_packet.get("evidence_dir"), "evidence_dir", directory=True)
        runtime_result_path = raw_evidence / "runtime-result.json"
        transport_state_path = raw_evidence / "runtime-transport.json"
        recovering = runtime_result_path.is_file() or transport_state_path.is_file()
        packet, paths = validate_envelope(packet_path, allow_existing_result=recovering)
        paths["evidence"].mkdir(parents=True, exist_ok=True)
        if runtime_result_path.is_file():
            print(json.dumps(recover_runtime_result(runtime_result_path, packet, paths, args.rig), sort_keys=True, separators=(",", ":")))
            return 0
        initial_packet_digest = sha256_file(packet_path)
        initial_intent_digest = sha256_file(paths["intent"])
        runtime_evidence: dict[str, Any] = {"packet_digest": initial_packet_digest}
        transport_state = (
            load_transport_state(transport_state_path, packet, paths, args.rig, args.binding)
            if transport_state_path.is_file() else None
        )

        if packet["role"] == "implement":
            baseline_path = paths["evidence"] / "runtime-baseline-manifest.json"
            if transport_state:
                workspace_base_sha = str(transport_state.get("workspace_base_sha", ""))
                if not SHA_RE.fullmatch(workspace_base_sha):
                    raise PacketError("transport_state_invalid", "transport state has no valid workspace base SHA")
            else:
                workspace_base_sha = git_workspace_head(paths["workspace"])
            if transport_state:
                baseline = load_object(baseline_path, "runtime baseline manifest")
            else:
                baseline = build_manifest(packet, paths, baseline_path)
            initial_manifest_digests = {"baseline_manifest": sha256_file(baseline_path)}
            runtime_evidence["baseline_manifest"] = str(baseline_path)
            runtime_evidence["baseline_manifest_digest"] = baseline["canonical_manifest_digest"]
        else:
            initial_manifest_digests = {
                "baseline_manifest": sha256_file(paths["baseline_manifest"]),
                "subject_manifest": sha256_file(paths["subject_manifest"]),
                "scope_receipt": sha256_file(paths["scope_receipt"]),
            }
            before_ok, before_message = verify_manifest(paths)
            runtime_evidence["subject_matched_before"] = before_ok
            runtime_evidence["subject_match_before_detail"] = before_message
            if not before_ok:
                raise PacketError("subject_mismatch", f"subject does not match supplied manifest before dispatch: {before_message}")

        if transport_state:
            bead_id = transport_state["bead_id"]
            target = transport_state["target"]
        else:
            bead_id, target = prepare_packet_transport(packet, paths, args.rig, args.binding)
            manifest_file_digests = {
                str(path): digest
                for field, digest in initial_manifest_digests.items()
                for path in [baseline_path if field == "baseline_manifest" and packet["role"] == "implement" else paths[field]]
            }
            transport_state = {
                "schema_version": "gc-packet-transport.v1",
                "packet_id": packet["packet_id"],
                "packet_digest": initial_packet_digest,
                "intent_digest": initial_intent_digest,
                "rig": args.rig,
                "binding": args.binding,
                "bead_id": bead_id,
                "target": target,
                "dispatch_phase": "prepared",
                "manifest_file_digests": manifest_file_digests,
                **({"workspace_base_sha": workspace_base_sha} if packet["role"] == "implement" else {}),
            }
            write_json_atomic(transport_state_path, transport_state)
        ensure_transport_routed(args.rig, bead_id, target)
        if transport_state["dispatch_phase"] != "routed":
            transport_state["dispatch_phase"] = "routed"
            write_json_atomic(transport_state_path, transport_state)
        deadline = time.monotonic() + args.timeout
        response, transport = wait_for_response(paths["result"], args.rig, bead_id, deadline)
        response_digest = sha256_file(paths["result"])
        session = runtime_session(str(response.get("session_context_id", "")))
        expected_template = target
        validate_agent_response(packet, paths, response, bead_id, transport, session, expected_template)
        runtime_evidence["session"] = {
            "id": session.get("id"),
            "session_name": session.get("session_name"),
            "template": session.get("template"),
            "provider": session.get("provider"),
            "model": session.get("model"),
            "model_source": session.get("model_source"),
            "model_policy": ROLE_MODELS[(packet["role"], packet["provider"])][0],
        }

        if packet["role"] == "implement":
            if sha256_file(baseline_path) != initial_manifest_digests["baseline_manifest"]:
                raise PacketError("manifest_mutated", "runtime baseline manifest changed while implementer was running")
            subject_path = paths["evidence"] / "runtime-subject-manifest.json"
            subject = build_manifest(packet, paths, subject_path, baseline_path)
            subject_changes = changed_paths(baseline, subject)
            workspace_changes = git_workspace_changes(paths["workspace"], workspace_base_sha)
            receipt = make_scope_receipt(
                packet,
                subject_changes,
                workspace_changes=workspace_changes,
                workspace_base_sha=workspace_base_sha,
            )
            receipt_path = paths["evidence"] / "runtime-scope-receipt.json"
            write_json_atomic(receipt_path, receipt)
            runtime_evidence.update(
                {
                    "subject_manifest": str(subject_path),
                    "subject_manifest_digest": subject["canonical_manifest_digest"],
                    "scope_receipt": str(receipt_path),
                    "scope_status": receipt["status"],
                    "actual_changed_paths": workspace_changes,
                    "subject_changed_paths": subject_changes,
                }
            )
        else:
            after_ok, after_message = verify_manifest(paths)
            runtime_evidence.update(
                {
                    "subject_matched_after": after_ok,
                    "subject_match_after_detail": after_message,
                    "subject_stable": after_ok,
                }
            )
            if not after_ok:
                raise PacketError("subject_mutated", f"subject changed while validator was running: {after_message}")
            for field, digest in initial_manifest_digests.items():
                if sha256_file(paths[field]) != digest:
                    raise PacketError("manifest_mutated", f"{field} changed while validator was running")
        if sha256_file(packet_path) != initial_packet_digest:
            raise PacketError("packet_mutated", "packet content changed while the GC role was running")
        if sha256_file(paths["intent"]) != initial_intent_digest or initial_intent_digest != packet["intent_digest"]:
            raise PacketError("intent_mutated", "intent content changed while the GC role was running")
        if sha256_file(paths["result"]) != response_digest:
            raise PacketError("agent_response_mutated", "agent response changed before runtime evidence was finalized")
        validate_response_artifacts(packet, paths, response)
        runtime_evidence["packet_stable"] = True
        runtime_evidence["intent_stable"] = True
        runtime_evidence["agent_response_stable"] = True
        runtime_evidence["agent_response_digest"] = response_digest
        runtime_evidence["manifests_stable"] = True
        result = success_result(packet, paths, response, bead_id, target, runtime_evidence)
        write_json_atomic(runtime_result_path, result)
        print(json.dumps(result, sort_keys=True, separators=(",", ":")))
        return 0
    except PacketError as exc:
        result = error_result(exc, packet)
        print(json.dumps(result, sort_keys=True, separators=(",", ":")))
        return 2


def command_inspect(args: argparse.Namespace) -> int:
    packet_path = absolute_path(args.packet, "packet", must_exist=True)
    packet, paths = validate_envelope(packet_path)
    summary = dict(packet)
    summary["packet_digest"] = sha256_file(paths["packet"])
    success_outcome = "candidate" if packet["role"] == "implement" else "evidence"
    response_message = f"one bounded {packet['role']} packet handled"
    summary["response_emit"] = {
        "command_template": (
            f"python3 {shlex.quote(str(Path(__file__).resolve()))} emit "
            f"--packet {shlex.quote(str(paths['packet']))} "
            f"--bead <transport-bead-id> --outcome {success_outcome} "
            "--artifact <absolute-evidence-artifact> "
            f"--message {shlex.quote(response_message)}"
        ),
        "success_outcome": success_outcome,
        "error_outcome": "error",
        "artifact_rules": [
            "successful responses require at least one existing artifact",
            f"every artifact must be inside {paths['evidence']}",
            "the bead argument is the exact transport bead returned by gc hook --claim",
        ],
    }
    if packet["role"] == "validate":
        summary["validate_helper"] = str(VALIDATE_HELPER.resolve())
        draft_path = paths["evidence"] / "verdict-draft.json"
        summary["verdict_store"] = {
            "command": (
                f"python3 {shlex.quote(str(Path(__file__).resolve()))} store-verdict "
                f"--packet {shlex.quote(str(paths['packet']))} "
                f"--draft {shlex.quote(str(draft_path))}"
            ),
            "draft_path": str(draft_path),
            "verdict_dir": str(paths["evidence"] / "verdicts"),
            "runtime_owned": [
                "acceptance_digest", "subject_manifest_digest",
                "author_context_id", "validator_context_id",
                "freshness_attestation", "artifact_digest",
            ],
        }
        summary["verdict_draft_contract"] = {
            "required_top_level": [
                "verdict", "criteria", "findings", "evidence_refs",
                "checked", "not_checked", "validated_at",
            ],
            "criterion_fields": ["id", "result", "evidence_refs", "reason"],
            "criterion_required": ["id", "result", "evidence_refs"],
            "finding_fields": ["id", "summary", "evidence_refs"],
            "result_values": ["PASS", "FAIL", "NOT_PROVEN"],
            "field_types": {
                "verdict": "string enum",
                "criteria": "nonempty array of criterion objects",
                "findings": "array of finding objects",
                "evidence_refs": "array of nonempty strings",
                "checked": "array of nonempty strings",
                "not_checked": "array of nonempty strings",
                "validated_at": "RFC3339 date-time string with timezone",
                "criterion.id": "nonempty string",
                "criterion.result": "string enum",
                "criterion.evidence_refs": "array of nonempty strings",
                "criterion.reason": "optional string",
                "finding.id": "nonempty string",
                "finding.summary": "nonempty string",
                "finding.evidence_refs": "nonempty array of nonempty strings",
            },
            "pass_requirements": [
                "not_checked must be an empty array",
                "criteria must be nonempty and every criterion result must be PASS",
                "every criterion evidence_refs must be nonempty",
                "top-level evidence_refs must be nonempty",
                "checked must be nonempty",
            ],
            "runtime_injected": [
                "schema_version", "acceptance_digest", "subject_manifest_digest",
                "author_context_id", "validator_context_id",
                "freshness_attestation", "artifact_digest",
            ],
        }
    print(json.dumps(summary, indent=2, sort_keys=True))
    return 0


def command_store_verdict(args: argparse.Namespace) -> int:
    packet_path = absolute_path(args.packet, "packet", must_exist=True)
    packet, paths = validate_envelope(packet_path)
    if packet["role"] != "validate":
        raise PacketError("invalid_envelope", "store-verdict requires a validate packet")
    draft = absolute_path(args.draft, "draft", must_exist=True)
    if not is_within(draft, paths["evidence"]):
        raise PacketError("invalid_verdict_artifact", "verdict draft must stay inside evidence_dir")
    session_id = os.environ.get("GC_SESSION_ID", "").strip()
    if not session_id:
        raise PacketError("session_identity_missing", "GC_SESSION_ID is required to store a verdict")
    if session_id == packet["author_context_id"]:
        raise PacketError("freshness_collision", "validator and author context IDs collide")
    if os.environ.get("GC_PROVIDER", "").strip() != packet["provider"]:
        raise PacketError("runtime_identity_mismatch", "GC_PROVIDER does not match the validate packet")
    template = os.environ.get("GC_TEMPLATE", "").strip()
    if not template or template.rsplit(".", 1)[-1] != local_agent_name(packet):
        raise PacketError("runtime_identity_mismatch", "GC_TEMPLATE does not match the validate packet")
    scope_status = load_object(paths["scope_receipt"], "scope_receipt").get("status")
    if scope_status not in {"PASS", "FAIL", "NOT_PROVEN"}:
        raise PacketError("invalid_scope_receipt", "scope receipt has no valid status")
    validator = load_validate_module()
    try:
        intent_bytes = paths["intent"].read_bytes()
        manifest = load_object(paths["subject_manifest"], "subject_manifest")
        draft_value = load_object(draft, "verdict draft")
        verdict_dir = paths["evidence"] / "verdicts"
        verdict_dir.mkdir(parents=True, exist_ok=True)
        lock_path = paths["evidence"] / ".verdict-store.lock"
        with lock_path.open("a+b") as lock:
            fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
            snapshot, snapshot_existed = validator.snapshot_intent(
                intent_bytes,
                paths["evidence"] / ".agents" / "ao" / "intents" / "sha256",
            )
            candidate = validator.bind_runtime_facts(
                draft_value,
                intent_bytes,
                manifest,
                packet["author_context_id"],
                scope_status,
                session_id,
                "runtime",
                session_id,
            )
            candidate = validator.enforce_identity(candidate)
            candidate["schema_version"] = "verdict.v2"
            expected_artifact, expected_payload = validator.artifact_bytes(candidate)
            validator.validate_verdict_v2(expected_artifact)
            existing_paths = sorted(verdict_dir.glob("*.json"))
            if len(existing_paths) > 1:
                raise PacketError(
                    "verdict_invariant_broken",
                    f"packet already has multiple durable verdicts: {[str(path) for path in existing_paths]}",
                )
            if existing_paths:
                verdict_path = existing_paths[0]
                if (
                    verdict_path.name != f"{expected_artifact['artifact_digest']}.json"
                    or verdict_path.read_bytes() != expected_payload
                ):
                    raise PacketError(
                        "verdict_already_recorded",
                        f"packet already has a different terminal verdict: {verdict_path}",
                    )
                artifact = expected_artifact
                existed = True
            else:
                artifact, verdict_path, existed = validator.store_verdict(
                    draft_value,
                    verdict_dir,
                    intent_bytes,
                    manifest,
                    packet["author_context_id"],
                    scope_status,
                    session_id,
                    "runtime",
                    session_id,
                )
    except (OSError, validator.ContractError) as exc:
        raise PacketError("invalid_verdict_artifact", f"cannot store verdict: {exc}") from exc
    result = {
        "acceptance_digest": sha256_bytes(intent_bytes),
        "artifact_digest": artifact["artifact_digest"],
        "idempotent": existed,
        "intent_ref": str(snapshot),
        "intent_snapshot_idempotent": snapshot_existed,
        "path": str(verdict_path),
        "verdict": artifact["verdict"],
    }
    print(json.dumps(result, indent=2, sort_keys=True))
    return 0


def command_emit(args: argparse.Namespace) -> int:
    packet_path = absolute_path(args.packet, "packet", must_exist=True)
    packet, paths = validate_envelope_for_emit(packet_path)
    allowed = {"candidate", "error"} if packet["role"] == "implement" else {"evidence", "error"}
    if args.outcome not in allowed:
        raise PacketError("invalid_agent_response", f"outcome {args.outcome} is not valid for role {packet['role']}")
    session_id = os.environ.get("GC_SESSION_ID", "").strip()
    if not session_id:
        raise PacketError("session_identity_missing", "GC_SESSION_ID is required to emit a role response")
    if packet["role"] == "validate" and session_id == packet.get("author_context_id"):
        raise PacketError("freshness_collision", "validator and author context IDs collide")
    runtime_provider = os.environ.get("GC_PROVIDER", "").strip()
    if runtime_provider != packet["provider"]:
        raise PacketError("runtime_identity_mismatch", "GC_PROVIDER does not match the packet provider")
    runtime_template = os.environ.get("GC_TEMPLATE", "").strip()
    if not runtime_template or runtime_template.rsplit(".", 1)[-1] != local_agent_name(packet):
        raise PacketError("runtime_identity_mismatch", "GC_TEMPLATE does not match the packet role and provider")
    if not args.bead.strip():
        raise PacketError("invalid_agent_response", "transport bead ID must be nonempty")
    artifacts = []
    seen: set[Path] = set()
    for raw in args.artifact:
        artifact = absolute_path(raw, "artifact", must_exist=True)
        if not is_within(artifact, paths["evidence"]):
            raise PacketError("invalid_agent_response", f"artifact is outside evidence_dir: {artifact}")
        if artifact in seen:
            raise PacketError("invalid_agent_response", f"duplicate artifact path: {artifact}")
        seen.add(artifact)
        artifacts.append({"path": str(artifact), "sha256": sha256_file(artifact)})
    if args.outcome != "error" and not artifacts:
        raise PacketError("invalid_agent_response", "successful agent response requires at least one --artifact")
    response = {
        "schema_version": "gc-agent-response.v1",
        "packet_id": packet["packet_id"],
        "role": packet["role"],
        "outcome": args.outcome,
        "transport_bead_id": args.bead,
        "session_context_id": session_id,
        "session_name": os.environ.get("GC_SESSION_NAME", ""),
        "template": runtime_template,
        "artifacts": artifacts,
        "message": args.message,
    }
    validate_response_artifacts(packet, paths, response)
    write_json_atomic(paths["result"], response)
    print(str(paths["result"]))
    return 0


def validate_envelope_for_emit(path: Path) -> tuple[dict[str, Any], dict[str, Path]]:
    """Validate an in-flight packet while allowing its just-created result."""
    packet = load_object(path, "packet")
    result = Path(str(packet.get("result_path", ""))).expanduser()
    if result.is_file():
        raise PacketError("stale_result", f"result_path already exists: {result}")
    return validate_envelope(path)


def command_doctor_contract() -> int:
    problems = []
    for path in (
        ENVELOPE_SCHEMA,
        PACK_ROOT / "commands" / "run-packet" / "schemas" / "result.schema.json",
        PACK_ROOT / "commands" / "run-packet" / "schemas" / "failure.schema.json",
        VALIDATE_HELPER,
    ):
        if not path.is_file():
            problems.append(f"missing {path.relative_to(PACK_ROOT)}")
    if ENVELOPE_SCHEMA.is_file():
        schema = load_object(ENVELOPE_SCHEMA, "envelope schema")
        if schema.get("additionalProperties") is not False:
            problems.append("envelope schema must reject additional properties")
        text = ENVELOPE_SCHEMA.read_text(encoding="utf-8")
        for forbidden in ("retry", "attempt", "budget", "next_action", "release", "delivery"):
            if f'"{forbidden}"' in text:
                problems.append(f"envelope schema contains forbidden lifecycle field {forbidden}")
    if os.environ.get("AGENTOPS_GC_SKIP_VERSION_CHECK") != "1":
        try:
            version_output = require_process([gc_binary(), "version"], timeout=10).strip()
            version_match = re.search(r"(\d+)\.(\d+)\.(\d+)", version_output)
            if version_match:
                if tuple(int(part) for part in version_match.groups()) < MINIMUM_GC_VERSION:
                    problems.append(f"gc >= 1.3.5 is required; found {version_output}")
            elif version_output == "dev":
                for probe in ([gc_binary(), "lint", "--help"], [gc_binary(), "runtime", "drain-ack", "--help"]):
                    completed = run_process(probe, timeout=10)
                    if completed.returncode != 0:
                        problems.append(f"development gc build lacks required surface: {' '.join(probe[1:])}")
            else:
                problems.append(f"gc >= 1.3.5 is required; found {version_output or 'unknown'}")
        except PacketError as exc:
            problems.append(str(exc))
    if problems:
        print(f"packet contract has {len(problems)} error(s)")
        for problem in problems:
            print(problem)
        return 2
    print("packet schema and runtime helper are present and fail closed")
    return 0


def parse_simple_toml(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        values[key.strip()] = value.strip()
    return values


def command_doctor_roles() -> int:
    agents_root = PACK_ROOT / "agents"
    actual = sorted(path.name for path in agents_root.iterdir() if path.is_dir() and not path.name.startswith("_"))
    problems = []
    providers = {
        "implementer": "codex",
        "validator": "codex",
        "implementer-claude": "claude",
        "validator-claude": "claude",
    }
    permission_defaults = {"codex": "auto-edit", "claude": "unrestricted"}
    model_defaults = {
        "implementer": "gpt-5.6-terra",
        "validator": "gpt-5.6-sol",
        "implementer-claude": "opus-4.8",
        "validator-claude": "opus-4.8",
    }
    expected_agents = sorted(providers)
    if actual != expected_agents:
        problems.append(f"expected bounded Codex and Claude implementer/validator roles only, found {actual}")
    for name, provider in providers.items():
        config = agents_root / name / "agent.toml"
        if not config.is_file():
            problems.append(f"missing agents/{name}/agent.toml")
            continue
        values = parse_simple_toml(config)
        expected = {
            "scope": '"rig"',
            "provider": f'"{provider}"',
            "session": '"tmux"',
            "wake_mode": '"fresh"',
            "min_active_sessions": "0",
            "max_active_sessions": "1",
            "inject_assigned_skills": "true",
        }
        if provider == "claude":
            expected["lifecycle"] = '"one_shot"'
        for key, value in expected.items():
            if values.get(key) != value:
                problems.append(f"agents/{name}/agent.toml: {key} must be {value}")
        expected_permission = permission_defaults[provider]
        if expected_permission not in values.get("option_defaults", ""):
            problems.append(
                f"agents/{name}/agent.toml: permission_mode must be {expected_permission}"
            )
        expected_model = model_defaults[name]
        if f'model = "{expected_model}"' not in values.get("option_defaults", ""):
            problems.append(
                f"agents/{name}/agent.toml: model must be {expected_model}"
            )
        for forbidden in ("args", "args_append", "start_command"):
            if forbidden in values:
                problems.append(f"agents/{name}/agent.toml: {forbidden} is forbidden")
        if "default_sling_formula" in values:
            problems.append(f"agents/{name}/agent.toml: default_sling_formula is forbidden")
    for forbidden_dir in ("formulas", "orders"):
        if (PACK_ROOT / forbidden_dir).exists():
            problems.append(f"top-level {forbidden_dir}/ is forbidden for the single-pass adapter")
    if problems:
        print(f"role boundary has {len(problems)} error(s)")
        for problem in problems:
            print(problem)
        return 2
    print("Codex Terra workers, Codex Sol validators, and bounded one-shot Claude Opus 4.8 roles are fresh zero-minimum pools")
    return 0


def command_doctor_projection() -> int:
    if not PROJECTION_MANIFEST.is_file():
        print("generated skill projection manifest is missing")
        return 2
    try:
        manifest = load_object(PROJECTION_MANIFEST, "skill projection manifest")
    except PacketError as exc:
        print(str(exc))
        return 2
    problems = []
    if manifest.get("source") != "skills" or not isinstance(manifest.get("files"), list):
        problems.append("projection manifest source/files contract is invalid")
    for item in manifest.get("files", []):
        if not isinstance(item, dict) or not isinstance(item.get("destination"), str) or not DIGEST_RE.fullmatch(str(item.get("sha256", ""))):
            problems.append(f"invalid projection manifest entry: {item!r}")
            continue
        destination = PACK_ROOT.parent.parent / item["destination"]
        if not destination.is_file():
            problems.append(f"projected skill file is missing: {item['destination']}")
        elif sha256_file(destination) != item["sha256"]:
            problems.append(f"projected skill file drifted: {item['destination']}")
    if problems:
        print(f"skill projection has {len(problems)} error(s)")
        for problem in problems:
            print(problem)
        return 2
    print(f"generated skill projection matches {len(manifest['files'])} recorded files")
    return 0


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    sub = root.add_subparsers(dest="command", required=True)
    run = sub.add_parser("run")
    run.add_argument("--packet", required=True)
    run.add_argument("--rig", required=True)
    run.add_argument("--binding", default="agentops")
    run.add_argument("--timeout", type=float, default=1800)
    inspect = sub.add_parser("inspect")
    inspect.add_argument("--packet", required=True)
    store_verdict = sub.add_parser("store-verdict")
    store_verdict.add_argument("--packet", required=True)
    store_verdict.add_argument("--draft", required=True)
    emit = sub.add_parser("emit")
    emit.add_argument("--packet", required=True)
    emit.add_argument("--bead", required=True)
    emit.add_argument("--outcome", required=True)
    emit.add_argument("--artifact", action="append", default=[])
    emit.add_argument("--message", default="")
    sub.add_parser("doctor-contract")
    sub.add_parser("doctor-roles")
    sub.add_parser("doctor-projection")
    return root


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    try:
        if args.command == "run":
            if args.timeout <= 0:
                raise PacketError("invalid_argument", "--timeout must be positive")
            if not PACKET_ID_RE.fullmatch(args.rig) or not PACKET_ID_RE.fullmatch(args.binding):
                raise PacketError("invalid_argument", "--rig and --binding must be simple GC identifiers")
            return command_run(args)
        if args.command == "inspect":
            return command_inspect(args)
        if args.command == "store-verdict":
            return command_store_verdict(args)
        if args.command == "emit":
            return command_emit(args)
        if args.command == "doctor-contract":
            return command_doctor_contract()
        if args.command == "doctor-roles":
            return command_doctor_roles()
        if args.command == "doctor-projection":
            return command_doctor_projection()
    except PacketError as exc:
        print(f"packet: {exc}", file=sys.stderr)
        return 2
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
