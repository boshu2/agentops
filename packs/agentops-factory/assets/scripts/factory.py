#!/usr/bin/env python3
"""Bead-native Fenced Steward reducer and Refinery for Gas City.

Packs define reusable roles and commands. Beads are the durable units of work
and the only factory lifecycle ledger. JSON artifacts are immutable proposals or
evidence referenced by beads; they never replace bead status, dependencies, or
metadata.
"""

from __future__ import annotations

import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
import contextlib
import fcntl
import hashlib
import importlib.util
import json
import os
from pathlib import Path, PurePosixPath
import re
import secrets
import shutil
import subprocess
import sys
import tempfile
import time
import tomllib
from typing import Any


sys.dont_write_bytecode = True

PACK_ROOT = Path(__file__).resolve().parents[2]
SCHEMA_ROOT = PACK_ROOT / "assets" / "schemas"
EXECUTOR_ADAPTER = PACK_ROOT.parent / "agentops-executor" / "assets" / "scripts" / "packet.py"
ID_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$")
DIGEST_RE = re.compile(r"^[0-9a-f]{64}$")
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
PROVIDERS = {"codex", "claude"}
VERDICTS = {"PASS", "FAIL", "NOT_PROVEN"}
ROLE_REQUEST_KEYS = {
    "schema_version", "request_id", "role", "provider", "program_id",
    "workspace", "intent_source", "intent_digest", "repository", "base_branch",
    "base_sha", "subject_path", "subject_digest", "mayor_context_id",
    "artifact_path", "result_path",
}
NODE_KEYS = {
    "id", "title", "intent", "acceptance", "non_goals", "depends_on",
    "write_scope", "generated_scope", "subject", "first_check", "provider",
    "validator_provider", "risk", "supersedes",
}


class FactoryError(RuntimeError):
    def __init__(self, code: str, message: str):
        super().__init__(message)
        self.code = code


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode("utf-8")


def digest_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def digest_file(path: Path) -> str:
    return digest_bytes(path.read_bytes())


def load_object(path: Path, label: str) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise FactoryError("missing_file", f"{label} does not exist: {path}") from exc
    except (OSError, json.JSONDecodeError) as exc:
        raise FactoryError("invalid_json", f"cannot read {label} {path}: {exc}") from exc
    if not isinstance(value, dict):
        raise FactoryError("invalid_json", f"{label} must be an object: {path}")
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


def write_text_atomic(path: Path, payload: str) -> None:
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        os.fchmod(fd, path.stat().st_mode & 0o777)
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
        directory_fd = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory_fd)
        finally:
            os.close(directory_fd)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def write_or_verify_json(path: Path, value: dict[str, Any], label: str) -> None:
    if path.exists():
        if load_object(path, label) != value:
            raise FactoryError("identity_mismatch", f"existing {label} differs from the current bead facts: {path}")
        return
    write_json_atomic(path, value)


def write_or_verify_text(path: Path, value: str, label: str) -> None:
    if path.exists():
        if path.read_text(encoding="utf-8") != value:
            raise FactoryError("identity_mismatch", f"existing {label} differs from the current bead facts: {path}")
        return
    write_text_atomic(path, value)


def require_string(value: Any, label: str, pattern: re.Pattern[str] | None = None) -> str:
    if not isinstance(value, str) or not value.strip():
        raise FactoryError("invalid_contract", f"{label} must be a nonempty string")
    if pattern is not None and not pattern.fullmatch(value):
        raise FactoryError("invalid_contract", f"{label} has an invalid shape: {value!r}")
    return value


def require_list(value: Any, label: str, nonempty: bool = False) -> list[Any]:
    if not isinstance(value, list) or (nonempty and not value):
        raise FactoryError("invalid_contract", f"{label} must be a{' nonempty' if nonempty else ''} array")
    return value


def exact_keys(value: dict[str, Any], required: set[str], allowed: set[str], label: str) -> None:
    missing = sorted(required - set(value))
    unknown = sorted(set(value) - allowed)
    if missing or unknown:
        raise FactoryError("invalid_contract", f"{label} fields are not exact; missing={missing} unknown={unknown}")


def absolute_path(value: Any, label: str, exists: bool = False, directory: bool = False) -> Path:
    raw = require_string(value, label)
    path = Path(raw).expanduser()
    if not path.is_absolute():
        raise FactoryError("invalid_path", f"{label} must be absolute: {raw}")
    path = path.resolve(strict=False)
    if exists and not path.exists():
        raise FactoryError("missing_file", f"{label} does not exist: {path}")
    if directory and path.exists() and not path.is_dir():
        raise FactoryError("invalid_path", f"{label} must be a directory: {path}")
    return path


def normalize_path(value: Any, label: str) -> str:
    raw = require_string(value, label)
    path = PurePosixPath(raw.replace("\\", "/"))
    if path.is_absolute() or ".." in path.parts:
        raise FactoryError("invalid_scope", f"{label} escapes the repository: {raw}")
    return path.as_posix().removeprefix("./") or "."


def run_process(argv: list[str], *, cwd: Path | None = None, input_text: str | None = None,
                timeout: float | None = None, check: bool = True,
                env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
    try:
        completed = subprocess.run(
            argv, cwd=cwd, input=input_text, text=True, stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, timeout=timeout, check=False,
            env={**os.environ, **(env or {})},
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise FactoryError("process_error", f"cannot run {' '.join(argv)}: {exc}") from exc
    if check and completed.returncode != 0:
        detail = completed.stderr.strip() or completed.stdout.strip() or f"exit {completed.returncode}"
        raise FactoryError("command_failed", f"{' '.join(argv)}: {detail}")
    return completed


def output(argv: list[str], **kwargs: Any) -> str:
    return run_process(argv, **kwargs).stdout.strip()


def parse_json_output(raw: str, label: str) -> Any:
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        pass
    for line in reversed([line for line in raw.splitlines() if line.strip()]):
        try:
            return json.loads(line)
        except json.JSONDecodeError:
            continue
    raise FactoryError("invalid_runtime_json", f"{label} did not return JSON: {raw[:400]}")


def gc_binary() -> str:
    configured = os.environ.get("GC_BIN")
    if configured:
        path = Path(configured).expanduser()
        if path.is_file() and os.access(path, os.X_OK):
            return str(path.resolve())
        raise FactoryError("gc_missing", f"configured GC_BIN is not executable: {path}")
    found = shutil.which("gc")
    if not found:
        raise FactoryError("gc_missing", "gc binary is unavailable")
    return found


def city_path() -> str:
    city = os.environ.get("GC_CITY_PATH") or os.environ.get("GC_CITY")
    if not city:
        raise FactoryError("city_missing", "GC_CITY_PATH is required")
    return str(Path(city).expanduser().resolve(strict=False))


def is_within(path: Path, root: Path) -> bool:
    try:
        path.relative_to(root)
        return True
    except ValueError:
        return False


class Beads:
    """Thin adapter to GC's selected-rig Beads store."""

    def __init__(self, rig: str | None):
        self.rig = require_string(rig, "rig") if rig is not None else None

    def run(self, *args: str, check: bool = True, timeout: float = 60) -> subprocess.CompletedProcess[str]:
        command = [gc_binary(), "--city", city_path(), "bd"]
        if self.rig is not None:
            command += ["--rig", self.rig]
        return run_process(
            [*command, *args],
            check=check, timeout=timeout,
        )

    def graph_create(self, plan: dict[str, Any]) -> dict[str, str]:
        with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False, encoding="utf-8") as handle:
            json.dump(plan, handle, sort_keys=True)
            handle.write("\n")
            plan_path = handle.name
        try:
            value = parse_json_output(self.run("create", "--graph", plan_path, "--json", timeout=120).stdout, "bd create --graph")
        finally:
            os.unlink(plan_path)
        ids = value.get("ids") if isinstance(value, dict) else None
        if not isinstance(ids, dict) or not ids or not all(isinstance(key, str) and isinstance(item, str) for key, item in ids.items()):
            raise FactoryError("graph_apply_failed", f"bd graph result has no symbolic IDs: {value!r}")
        return ids

    def show(self, bead_id: str) -> dict[str, Any]:
        value = parse_json_output(self.run("show", bead_id, "--json").stdout, "bd show")
        records = value if isinstance(value, list) else value.get("issues", []) if isinstance(value, dict) else []
        matches = [item for item in records if isinstance(item, dict) and str(item.get("id")) == bead_id]
        if len(matches) != 1:
            raise FactoryError("bead_missing", f"expected one bead {bead_id}, found {len(matches)}")
        return matches[0]

    def list_program(self, program_bead: str) -> list[dict[str, Any]]:
        value = parse_json_output(
            self.run("list", "--metadata-field", f"factory.program_bead={program_bead}", "--all", "--json", "--limit", "0").stdout,
            "bd list",
        )
        records = value if isinstance(value, list) else value.get("issues", []) if isinstance(value, dict) else []
        return [item for item in records if isinstance(item, dict)]

    def list_by_metadata(self, key: str, value: str) -> list[dict[str, Any]]:
        result = parse_json_output(
            self.run("list", "--metadata-field", f"{key}={value}", "--all", "--json", "--limit", "0").stdout,
            "bd list",
        )
        records = result if isinstance(result, list) else result.get("issues", []) if isinstance(result, dict) else []
        return [item for item in records if isinstance(item, dict)]

    def ready_ids(self) -> set[str]:
        value = parse_json_output(self.run("ready", "--json", "--limit", "0").stdout, "bd ready")
        records = value if isinstance(value, list) else value.get("issues", []) if isinstance(value, dict) else []
        return {str(item["id"]) for item in records if isinstance(item, dict) and item.get("id")}

    def create(self, title: str, description: str, metadata: dict[str, str], labels: list[str] | None = None) -> str:
        args = ["create", title, "--description", description, "--metadata", json.dumps(metadata, sort_keys=True), "--json"]
        if labels:
            args += ["--labels", ",".join(labels)]
        value = parse_json_output(self.run(*args).stdout, "bd create")
        if isinstance(value, list) and value:
            value = value[0]
        bead_id = value.get("id") if isinstance(value, dict) else None
        if not isinstance(bead_id, str) or not bead_id:
            raise FactoryError("bead_create_failed", f"bd create returned no id: {value!r}")
        return bead_id

    def update_metadata(self, bead_id: str, fields: dict[str, Any]) -> None:
        args = ["update", bead_id]
        for key, value in sorted(fields.items()):
            rendered = value if isinstance(value, str) else json.dumps(value, sort_keys=True, separators=(",", ":"))
            args += ["--set-metadata", f"{key}={rendered}"]
        self.run(*args)

    def close(self, bead_id: str, reason: str) -> None:
        self.run("close", bead_id, "--reason", reason)

    def dep_add(self, blocked: str, blocker: str, dep_type: str = "blocks") -> None:
        self.run("dep", "add", blocked, blocker, "--type", dep_type)


def ensure_dependency(beads: Beads, blocked: str, blocker: str,
                      dep_type: str = "blocks") -> None:
    record = beads.show(blocked)
    for item in record.get("dependencies", []):
        if not isinstance(item, dict):
            continue
        item_id = item.get("id") or item.get("depends_on_id") or item.get("dependency_id")
        item_type = item.get("dependency_type") or item.get("type") or "blocks"
        if str(item_id) == blocker and item_type == dep_type:
            return
    beads.dep_add(blocked, blocker, dep_type)


def validate_role_request(path: Path) -> tuple[dict[str, Any], dict[str, Path]]:
    request = load_object(path, "factory role request")
    exact_keys(request, ROLE_REQUEST_KEYS, ROLE_REQUEST_KEYS, "factory role request")
    if request.get("schema_version") != "factory-role-request.v1":
        raise FactoryError("invalid_role_request", "schema_version must be factory-role-request.v1")
    require_string(request.get("request_id"), "request_id")
    role = request.get("role")
    provider = request.get("provider")
    if role not in {"mayor", "plan-review", "rescope"} or provider not in PROVIDERS:
        raise FactoryError("invalid_role_request", "role or provider is invalid")
    if role in {"mayor", "rescope"} and provider != "codex":
        raise FactoryError("invalid_role_request", "the v1 Mayor and rescope roles use the Codex city role")
    require_string(request.get("program_id"), "program_id", ID_RE)
    workspace = absolute_path(request.get("workspace"), "workspace", True, True)
    repository = absolute_path(request.get("repository"), "repository", True, True)
    if workspace != repository:
        raise FactoryError("invalid_role_request", "workspace must equal repository")
    intent = absolute_path(request.get("intent_source"), "intent_source", True)
    if request.get("intent_digest") != digest_file(intent):
        raise FactoryError("identity_mismatch", "role request intent digest is stale")
    require_string(request.get("intent_digest"), "intent_digest", DIGEST_RE)
    require_string(request.get("base_branch"), "base_branch")
    require_string(request.get("base_sha"), "base_sha", SHA_RE)
    artifact = absolute_path(request.get("artifact_path"), "artifact_path")
    result = absolute_path(request.get("result_path"), "result_path")
    if not is_within(artifact, repository) or not is_within(result, repository):
        raise FactoryError("invalid_role_request", "role artifacts must stay inside the repository")
    if len({path, artifact, result}) != 3:
        raise FactoryError("invalid_role_request", "request, artifact, and result paths must differ")
    subject: Path | None = None
    if role == "mayor":
        if any(request.get(field) is not None for field in ("subject_path", "subject_digest", "mayor_context_id")):
            raise FactoryError("invalid_role_request", "Mayor request must not claim a prior graph or Mayor context")
    else:
        subject = absolute_path(request.get("subject_path"), "subject_path", True)
        require_string(request.get("subject_digest"), "subject_digest", DIGEST_RE)
        require_string(request.get("mayor_context_id"), "mayor_context_id")
        if request["subject_digest"] != digest_file(subject):
            raise FactoryError("identity_mismatch", f"{role} subject digest is stale")
    return request, {
        "request": path,
        "workspace": workspace,
        "repository": repository,
        "intent": intent,
        "artifact": artifact,
        "result": result,
        **({"subject": subject} if subject is not None else {}),
    }


def command_inspect_role(args: argparse.Namespace) -> int:
    path = absolute_path(args.request, "request", True)
    request, paths = validate_role_request(path)
    rendered = dict(request)
    rendered["request_path"] = str(paths["request"])
    print(json.dumps(rendered, sort_keys=True))
    return 0


def command_emit_role(args: argparse.Namespace) -> int:
    request_path = absolute_path(args.request, "request", True)
    request, paths = validate_role_request(request_path)
    artifact = absolute_path(args.artifact, "artifact", True)
    if artifact != paths["artifact"]:
        raise FactoryError("artifact_mismatch", "emitted artifact differs from requested artifact_path")
    session_id = require_string(os.environ.get("GC_SESSION_ID"), "GC_SESSION_ID")
    session_name = require_string(os.environ.get("GC_SESSION_NAME"), "GC_SESSION_NAME")
    template = require_string(os.environ.get("GC_TEMPLATE"), "GC_TEMPLATE")
    provider = require_string(os.environ.get("GC_PROVIDER"), "GC_PROVIDER")
    if provider != request["provider"]:
        raise FactoryError("provider_mismatch", "runtime provider differs from role request")
    expected_suffix = ".mayor" if request["role"] in {"mayor", "rescope"} else (
        ".plan-reviewer-claude" if provider == "claude" else ".plan-reviewer"
    )
    if not template.endswith(expected_suffix):
        raise FactoryError("template_mismatch", f"runtime template must end with {expected_suffix}")
    if request["role"] == "mayor":
        validate_graph(
            load_object(artifact, "program graph"), request["intent_digest"],
            paths["repository"], request["base_sha"],
        )
    elif request["role"] == "plan-review":
        graph = validate_graph(
            load_object(paths["subject"], "program graph"), request["intent_digest"],
            paths["repository"], request["base_sha"],
        )
        validate_review(
            load_object(artifact, "plan review"), graph, request["subject_digest"],
            request["mayor_context_id"],
        )
    else:
        context = validate_rescope_context(load_object(paths["subject"], "rescope context"))
        if context["program_id"] != request["program_id"] or context["canonical_intent_digest"] != request["intent_digest"]:
            raise FactoryError("identity_mismatch", "rescope request does not match its immutable context")
        if session_id == request["mayor_context_id"]:
            raise FactoryError("freshness_collision", "rescope reused the original Mayor context")
        successor = validate_node(load_object(artifact, "successor proposal"), "successor")
        if successor.get("supersedes") != context["rejected_node_id"]:
            raise FactoryError("invalid_successor", "Mayor successor does not supersede the rejected node")
        if successor["id"] == context["rejected_node_id"]:
            raise FactoryError("invalid_successor", "Mayor successor must use a fresh node identity")
        if successor["acceptance"] != context["rejected_spec"]["acceptance"] or successor["non_goals"] != context["rejected_spec"]["non_goals"]:
            raise FactoryError("acceptance_changed", "Mayor successor changed acceptance or non-goals")
    response = {
        "schema_version": "factory-role-response.v1",
        "request_id": request["request_id"],
        "role": request["role"],
        "provider": provider,
        "bead_id": require_string(args.bead, "bead"),
        "session_context_id": session_id,
        "session_name": session_name,
        "template": template,
        "artifact_path": str(artifact),
        "artifact_digest": digest_file(artifact),
    }
    write_json_atomic(paths["result"], response)
    print(json.dumps(response, sort_keys=True))
    return 0


def metadata(record: dict[str, Any]) -> dict[str, Any]:
    value = record.get("metadata", {})
    if isinstance(value, str):
        try:
            value = json.loads(value)
        except json.JSONDecodeError as exc:
            raise FactoryError("invalid_bead", f"bead metadata is invalid JSON: {exc}") from exc
    if not isinstance(value, dict):
        raise FactoryError("invalid_bead", "bead metadata must be an object")
    return value


def metadata_list(meta: dict[str, Any], key: str) -> list[str]:
    value = meta.get(key, [])
    if isinstance(value, str):
        try:
            value = json.loads(value)
        except json.JSONDecodeError as exc:
            raise FactoryError("invalid_bead", f"bead metadata {key} is invalid JSON") from exc
    if not isinstance(value, list) or not all(isinstance(item, str) and item for item in value):
        raise FactoryError("invalid_bead", f"bead metadata {key} must be a string array")
    return value


def metadata_object(meta: dict[str, Any], key: str) -> dict[str, Any]:
    value = meta.get(key)
    if isinstance(value, str):
        try:
            value = json.loads(value)
        except json.JSONDecodeError as exc:
            raise FactoryError("invalid_bead", f"bead metadata {key} is invalid JSON") from exc
    if not isinstance(value, dict):
        raise FactoryError("invalid_bead", f"bead metadata {key} must be an object")
    return value


def path_prefix(pattern: str) -> str:
    at = min((pattern.find(char) for char in "*?[" if char in pattern), default=len(pattern))
    return pattern[:at].rstrip("/") or "."


def scopes_overlap(left: list[str], right: list[str]) -> bool:
    for raw_left in left:
        for raw_right in right:
            a, b = path_prefix(raw_left), path_prefix(raw_right)
            if a == "." or b == "." or a == b or a.startswith(b + "/") or b.startswith(a + "/"):
                return True
    return False


def topological_order(nodes: list[dict[str, Any]]) -> list[str]:
    by_id = {node["id"]: node for node in nodes}
    indegree = {node_id: 0 for node_id in by_id}
    followers = {node_id: [] for node_id in by_id}
    for node in nodes:
        for dependency in node["depends_on"]:
            if dependency not in by_id:
                raise FactoryError("invalid_graph", f"node {node['id']} depends on unknown node {dependency}")
            indegree[node["id"]] += 1
            followers[dependency].append(node["id"])
    ready = sorted(key for key, count in indegree.items() if count == 0)
    ordered: list[str] = []
    while ready:
        current = ready.pop(0)
        ordered.append(current)
        for follower in sorted(followers[current]):
            indegree[follower] -= 1
            if indegree[follower] == 0:
                ready.append(follower)
                ready.sort()
    if len(ordered) != len(nodes):
        raise FactoryError("invalid_graph", "program graph contains a dependency cycle")
    return ordered


def ancestors(nodes: list[dict[str, Any]]) -> dict[str, set[str]]:
    by_id = {node["id"]: node for node in nodes}
    result = {node_id: set() for node_id in by_id}
    for node_id in topological_order(nodes):
        for dependency in by_id[node_id]["depends_on"]:
            result[node_id].add(dependency)
            result[node_id].update(result[dependency])
    return result


def validate_node(node: dict[str, Any], label: str, allow_supersedes: bool = True) -> dict[str, Any]:
    required = NODE_KEYS - {"supersedes"}
    exact_keys(node, required, NODE_KEYS if allow_supersedes else required, label)
    node_id = require_string(node.get("id"), f"{label}.id", ID_RE)
    require_string(node.get("title"), f"{label}.title")
    require_string(node.get("intent"), f"{label}.intent")
    for field, nonempty in (("acceptance", True), ("non_goals", False), ("depends_on", False)):
        values = [require_string(item, f"{label}.{field}") for item in require_list(node.get(field), f"{label}.{field}", nonempty)]
        if len(values) != len(set(values)):
            raise FactoryError("invalid_graph", f"{label}.{field} contains duplicates")
        node[field] = values
    for field, nonempty in (("write_scope", True), ("generated_scope", False)):
        values = [normalize_path(item, f"{label}.{field}") for item in require_list(node.get(field), f"{label}.{field}", nonempty)]
        if len(values) != len(set(values)):
            raise FactoryError("invalid_graph", f"{label}.{field} contains duplicates")
        node[field] = values
    subject = node.get("subject")
    if not isinstance(subject, dict):
        raise FactoryError("invalid_graph", f"{label}.subject must be an object")
    exact_keys(subject, {"includes", "excludes"}, {"includes", "excludes"}, f"{label}.subject")
    node["subject"] = {
        "includes": [normalize_path(item, f"{label}.subject.includes") for item in require_list(subject.get("includes"), "subject.includes", True)],
        "excludes": [normalize_path(item, f"{label}.subject.excludes") for item in require_list(subject.get("excludes"), "subject.excludes")],
    }
    require_string(node.get("first_check"), f"{label}.first_check")
    if node.get("provider") not in PROVIDERS or node.get("validator_provider") not in PROVIDERS:
        raise FactoryError("invalid_graph", f"node {node_id} providers must be codex or claude")
    if node["provider"] == node["validator_provider"]:
        raise FactoryError("invalid_graph", f"node {node_id} must use an opposite-family Validator")
    if node.get("risk") not in {"routine", "high"}:
        raise FactoryError("invalid_graph", f"node {node_id} risk must be routine or high")
    if "supersedes" in node:
        require_string(node["supersedes"], f"{label}.supersedes")
    return node


RESCOPE_CONTEXT_KEYS = {
    "schema_version", "program_id", "program_bead", "rescope_bead",
    "rejected_bead", "rejected_node_id", "rejected_attempt", "rejected_spec",
    "verdict", "verdict_digest", "verdict_path", "canonical_intent_source",
    "canonical_intent_digest", "dependent_beads",
}


def validate_rescope_context(value: dict[str, Any]) -> dict[str, Any]:
    exact_keys(value, RESCOPE_CONTEXT_KEYS, RESCOPE_CONTEXT_KEYS, "rescope context")
    if value.get("schema_version") != "rescope-context.v1":
        raise FactoryError("invalid_rescope", "rescope context schema_version is invalid")
    require_string(value.get("program_id"), "rescope program_id", ID_RE)
    for field in ("program_bead", "rescope_bead", "rejected_bead", "rejected_node_id"):
        require_string(value.get(field), f"rescope {field}")
    if not isinstance(value.get("rejected_attempt"), int) or value["rejected_attempt"] < 1:
        raise FactoryError("invalid_rescope", "rejected_attempt must be a positive integer")
    rejected_spec = value.get("rejected_spec")
    if not isinstance(rejected_spec, dict):
        raise FactoryError("invalid_rescope", "rejected_spec must be an object")
    validate_node(dict(rejected_spec), "rescope rejected_spec")
    if value.get("verdict") not in {"FAIL", "NOT_PROVEN"}:
        raise FactoryError("invalid_rescope", "rescope verdict must be FAIL or NOT_PROVEN")
    require_string(value.get("verdict_digest"), "rescope verdict_digest", DIGEST_RE)
    absolute_path(value.get("verdict_path"), "rescope verdict_path", True)
    intent = absolute_path(value.get("canonical_intent_source"), "canonical intent source", True)
    require_string(value.get("canonical_intent_digest"), "canonical intent digest", DIGEST_RE)
    if digest_file(intent) != value["canonical_intent_digest"]:
        raise FactoryError("identity_mismatch", "canonical intent changed before Mayor rescope")
    dependent = require_list(value.get("dependent_beads"), "dependent_beads")
    if not all(isinstance(item, str) and item for item in dependent):
        raise FactoryError("invalid_rescope", "dependent_beads must contain bead IDs")
    return value


def validate_graph(value: dict[str, Any], intent_digest: str | None = None,
                   repository: Path | None = None, base_sha: str | None = None) -> dict[str, Any]:
    fields = {"schema_version", "program_id", "intent_digest", "repository", "base_branch", "base_sha", "planning_notes", "nodes"}
    exact_keys(value, fields, fields, "program graph")
    if value.get("schema_version") != "program-graph.v1":
        raise FactoryError("invalid_graph", "schema_version must be program-graph.v1")
    require_string(value.get("program_id"), "program_id", ID_RE)
    require_string(value.get("intent_digest"), "intent_digest", DIGEST_RE)
    repo_value = absolute_path(value.get("repository"), "repository", exists=repository is not None, directory=True)
    require_string(value.get("base_branch"), "base_branch")
    require_string(value.get("base_sha"), "base_sha", SHA_RE)
    if not isinstance(value.get("planning_notes"), str):
        raise FactoryError("invalid_graph", "planning_notes must be a string")
    if intent_digest is not None and value["intent_digest"] != intent_digest:
        raise FactoryError("identity_mismatch", "graph intent digest does not match canonical intent")
    if repository is not None and repo_value != repository:
        raise FactoryError("identity_mismatch", "graph repository does not match requested repository")
    if base_sha is not None and value["base_sha"] != base_sha:
        raise FactoryError("identity_mismatch", "graph base SHA does not match requested base")
    nodes = require_list(value.get("nodes"), "nodes", True)
    if len(nodes) > 64:
        raise FactoryError("invalid_graph", "v1 graph exceeds 64 beads")
    ids: set[str] = set()
    for index, raw in enumerate(nodes):
        if not isinstance(raw, dict):
            raise FactoryError("invalid_graph", f"nodes[{index}] must be an object")
        node = validate_node(raw, f"nodes[{index}]", allow_supersedes=False)
        if node["id"] in ids:
            raise FactoryError("invalid_graph", f"duplicate node id {node['id']}")
        ids.add(node["id"])
    order = topological_order(nodes)
    prior = ancestors(nodes)
    by_id = {node["id"]: node for node in nodes}
    for index, left_id in enumerate(order):
        left = by_id[left_id]
        for right_id in order[index + 1:]:
            right = by_id[right_id]
            if scopes_overlap(left["write_scope"] + left["generated_scope"], right["write_scope"] + right["generated_scope"]):
                if left_id not in prior[right_id] and right_id not in prior[left_id]:
                    raise FactoryError("scope_conflict", f"unordered beads {left_id} and {right_id} overlap")
    return value


def validate_review(value: dict[str, Any], graph: dict[str, Any], graph_digest: str,
                    mayor_context: str) -> dict[str, Any]:
    fields = {"schema_version", "program_id", "intent_digest", "graph_digest", "mayor_context_id",
              "reviewer_context_id", "provider", "verdict", "criteria", "findings"}
    exact_keys(value, fields, fields, "plan review")
    if value.get("schema_version") != "plan-review.v1":
        raise FactoryError("invalid_review", "schema_version must be plan-review.v1")
    expected = {"program_id": graph["program_id"], "intent_digest": graph["intent_digest"],
                "graph_digest": graph_digest, "mayor_context_id": mayor_context}
    for key, wanted in expected.items():
        if value.get(key) != wanted:
            raise FactoryError("identity_mismatch", f"plan review {key} is stale")
    reviewer = require_string(value.get("reviewer_context_id"), "reviewer_context_id")
    if reviewer == mayor_context:
        raise FactoryError("freshness_collision", "plan reviewer context collides with Mayor")
    if value.get("provider") not in PROVIDERS or value.get("verdict") not in VERDICTS:
        raise FactoryError("invalid_review", "plan review provider or verdict is invalid")
    criteria = require_list(value.get("criteria"), "criteria", True)
    nonpass = False
    for item in criteria:
        if not isinstance(item, dict) or set(item) != {"id", "result", "reason"}:
            raise FactoryError("invalid_review", "criterion fields are not exact")
        require_string(item.get("id"), "criterion.id")
        require_string(item.get("reason"), "criterion.reason")
        if item.get("result") not in VERDICTS:
            raise FactoryError("invalid_review", "criterion result is invalid")
        nonpass = nonpass or item["result"] != "PASS"
    blocking = False
    for finding in require_list(value.get("findings"), "findings"):
        if not isinstance(finding, dict) or set(finding) != {"id", "severity", "node_ids", "reason"}:
            raise FactoryError("invalid_review", "finding fields are not exact")
        if finding.get("severity") not in {"blocking", "advisory"}:
            raise FactoryError("invalid_review", "finding severity is invalid")
        blocking = blocking or finding["severity"] == "blocking"
    if value["verdict"] == "PASS" and (nonpass or blocking):
        raise FactoryError("invalid_review", "PASS review contains non-PASS evidence")
    return value


ROLE_RESPONSE_KEYS = {
    "schema_version", "request_id", "role", "provider", "bead_id",
    "session_context_id", "session_name", "template", "artifact_path",
    "artifact_digest",
}


def runtime_session(context_id: str) -> dict[str, Any]:
    value = parse_json_output(
        output([gc_binary(), "--city", city_path(), "session", "list", "--state", "all", "--json"]),
        "gc session list",
    )
    sessions = value.get("sessions") if isinstance(value, dict) else None
    if not isinstance(sessions, list):
        raise FactoryError("runtime_identity_missing", "gc session list returned no sessions")
    matches = [item for item in sessions if isinstance(item, dict) and str(item.get("id", "")) == context_id]
    if len(matches) != 1:
        raise FactoryError("runtime_identity_missing", f"expected one runtime session {context_id}, found {len(matches)}")
    return matches[0]


def dispatch_role(request_path: Path, rig: str, binding: str, timeout: float,
                  existing_work_bead: str | None = None,
                  linked_bead: str | None = None) -> dict[str, Any]:
    request, paths = validate_role_request(request_path)
    beads = Beads(None if request["role"] in {"mayor", "rescope"} else rig)
    kind = {"mayor": "planning", "plan-review": "plan-review", "rescope": "rescope-planning"}[request["role"]]
    request_digest = digest_file(request_path)
    if request["role"] in {"mayor", "rescope"}:
        target = f"{binding}.mayor"
    else:
        local = "plan-reviewer-claude" if request["provider"] == "claude" else "plan-reviewer"
        target = f"{rig}/{binding}.{local}"
    description = "\n".join([
        f"Factory {kind} bead {request['request_id']}",
        "This bead is the durable unit of work; close it only after the requested artifact is emitted.",
        f"adapter_path={Path(__file__).resolve()}",
        f"request_path={request_path}",
        f"request_digest={request_digest}",
        f"intent_digest={request['intent_digest']}",
        f"artifact_path={paths['artifact']}",
        f"result_path={paths['result']}",
    ]) + "\n"
    if not existing_work_bead:
        matches = [
            item for item in beads.list_by_metadata("factory.request_digest", request_digest)
            if metadata(item).get("factory.kind") == kind
        ]
        if len(matches) > 1:
            raise FactoryError("duplicate_role_bead", f"request {request['request_id']} has multiple work beads")
        if matches:
            existing_work_bead = require_string(matches[0].get("id"), "existing role work bead")
        elif paths["result"].exists() or paths["artifact"].exists():
            raise FactoryError("artifact_exists", "role artifact or result exists without its work bead")
    if existing_work_bead:
        bead_id = require_string(existing_work_bead, "existing role work bead")
        existing = beads.show(bead_id)
        existing_meta = metadata(existing)
        if existing_meta.get("factory.request_digest") != request_digest:
            raise FactoryError("identity_mismatch", "existing role work bead names a different request")
        if existing.get("status") not in {"open", "in_progress", "closed"}:
            raise FactoryError("invalid_transition", f"existing role work bead has status {existing.get('status')!r}")
    else:
        bead_id = beads.create(
            f"Factory {kind}: {request['program_id']}",
            description,
            {
                "factory.kind": kind,
                "factory.schema": "fenced-steward.v1",
                "factory.program_id": request["program_id"],
                "factory.status": "prepared",
                "factory.request_path": str(request_path),
                "factory.request_digest": request_digest,
                "factory.intent_digest": request["intent_digest"],
                "factory.provider": request["provider"],
            },
            ["gc-factory", f"factory-{kind}"],
        )
        existing = beads.show(bead_id)
        existing_meta = metadata(existing)
    if linked_bead:
        Beads(rig).update_metadata(linked_bead, {"factory.rescope_transport_bead": bead_id})
    routed_to = existing_meta.get("gc.routed_to")
    if routed_to is not None and routed_to != target:
        raise FactoryError("identity_mismatch", f"role work bead is routed to {routed_to!r}, not {target!r}")
    needs_sling = existing.get("status") == "open" and not routed_to and not existing.get("assignee")
    if needs_sling:
        value = parse_json_output(
            output([
                gc_binary(), "--city", city_path(), "sling", target, bead_id,
                "--no-formula", "--no-convoy", "--json",
            ]),
            "gc sling role bead",
        )
        if not isinstance(value, dict) or not value.get("success") or str(value.get("bead_id")) != bead_id:
            raise FactoryError("dispatch_failed", f"gc sling did not route {bead_id} to {target}: {value!r}")
    if existing_meta.get("factory.status") != "completed":
        beads.update_metadata(bead_id, {"factory.status": "routed", "factory.target": target})
    deadline = time.monotonic() + timeout
    response: dict[str, Any] | None = None
    record: dict[str, Any] | None = None
    while time.monotonic() < deadline:
        if response is None and paths["result"].is_file():
            response = load_object(paths["result"], "factory role response")
        record = beads.show(bead_id)
        if response is not None and record.get("status") == "closed":
            break
        time.sleep(1)
    else:
        if response is None:
            raise FactoryError("role_timeout", f"role bead {bead_id} produced no response")
        raise FactoryError("role_timeout", f"role bead {bead_id} emitted but did not close")
    exact_keys(response, ROLE_RESPONSE_KEYS, ROLE_RESPONSE_KEYS, "factory role response")
    expected = {
        "schema_version": "factory-role-response.v1",
        "request_id": request["request_id"],
        "role": request["role"],
        "provider": request["provider"],
        "bead_id": bead_id,
        "artifact_path": str(paths["artifact"]),
        "artifact_digest": digest_file(paths["artifact"]),
    }
    for key, wanted in expected.items():
        if response.get(key) != wanted:
            raise FactoryError("role_response_mismatch", f"role response {key} is stale")
    session_id = require_string(response.get("session_context_id"), "session_context_id")
    if request["role"] == "rescope" and session_id == request.get("mayor_context_id"):
        raise FactoryError("freshness_collision", "rescope reused the original Mayor context")
    session = runtime_session(session_id)
    for key, wanted in {
        "session_name": response["session_name"],
        "template": target,
        "provider": request["provider"],
    }.items():
        if session.get(key) != wanted:
            raise FactoryError("runtime_identity_mismatch", f"runtime session {key} differs from the response")
    if record.get("assignee") != response["session_name"]:
        raise FactoryError("runtime_identity_mismatch", "role bead assignee differs from the response session")
    beads.update_metadata(bead_id, {
        "factory.status": "completed",
        "factory.target": target,
        "factory.session_context_id": session_id,
        "factory.artifact_path": str(paths["artifact"]),
        "factory.artifact_digest": response["artifact_digest"],
    })
    return {**response, "work_bead": bead_id, "target": target}


def node_description(parent_digest: str, node: dict[str, Any]) -> str:
    acceptance = "\n".join(f"- {item}" for item in node["acceptance"])
    non_goals = "\n".join(f"- {item}" for item in node["non_goals"] or ["None declared."])
    return (
        f"{node['intent'].strip()}\n\nParent intent digest: {parent_digest}\n\n"
        f"Acceptance:\n{acceptance}\n\nNon-goals:\n{non_goals}\n\n"
        f"First deterministic check: {node['first_check']}\n"
    )


def experiment_metadata(graph: dict[str, Any], node: dict[str, Any], graph_digest: str,
                        review_digest: str, intent_source: Path) -> dict[str, str]:
    description = node_description(graph["intent_digest"], node)
    return {
        "factory.kind": "experiment",
        "factory.schema": "fenced-steward.v1",
        "factory.program_id": graph["program_id"],
        "factory.node_id": node["id"],
        "factory.status": "admitted",
        "factory.attempt": "1",
        "factory.intent_digest": digest_bytes(description.encode("utf-8")),
        "factory.parent_intent_digest": graph["intent_digest"],
        "factory.intent_source": str(intent_source),
        "factory.graph_digest": graph_digest,
        "factory.review_digest": review_digest,
        "factory.repository": graph["repository"],
        "factory.base_branch": graph["base_branch"],
        "factory.base_sha": graph["base_sha"],
        "factory.provider": node["provider"],
        "factory.validator_provider": node["validator_provider"],
        "factory.write_scope": json.dumps(node["write_scope"], separators=(",", ":")),
        "factory.generated_scope": json.dumps(node["generated_scope"], separators=(",", ":")),
        "factory.subject": json.dumps(node["subject"], sort_keys=True, separators=(",", ":")),
        "factory.first_check": node["first_check"],
        "factory.risk": node["risk"],
        "factory.spec": json.dumps(node, sort_keys=True, separators=(",", ":")),
    }


def compile_bead_plan(graph: dict[str, Any], graph_digest: str, review_digest: str,
                      intent_source: Path, mayor_context: str, reviewer_context: str) -> dict[str, Any]:
    root_key = "program"
    refinery_key = "refinery"
    root = {
        "key": root_key,
        "title": f"Factory program: {graph['program_id']}",
        "type": "epic",
        "description": graph["planning_notes"] or "Fenced Steward factory program.",
        "labels": ["gc-factory", "factory-program"],
        "metadata": {
            "factory.kind": "program",
            "factory.schema": "fenced-steward.v1",
            "factory.program_id": graph["program_id"],
            "factory.intent_source": str(intent_source),
            "factory.intent_digest": graph["intent_digest"],
            "factory.graph_digest": graph_digest,
            "factory.review_digest": review_digest,
            "factory.mayor_context_id": mayor_context,
            "factory.reviewer_context_id": reviewer_context,
            "factory.repository": graph["repository"],
            "factory.base_branch": graph["base_branch"],
            "factory.base_sha": graph["base_sha"],
            "factory.status": "admitted",
        },
        "metadata_refs": {"factory.refinery_bead": refinery_key},
    }
    nodes = [root]
    edges: list[dict[str, str]] = []
    key_for = {node["id"]: f"experiment-{node['id']}" for node in graph["nodes"]}
    for node in graph["nodes"]:
        node_metadata = experiment_metadata(graph, node, graph_digest, review_digest, intent_source)
        node_metadata["factory.mayor_context_id"] = mayor_context
        nodes.append({
            "key": key_for[node["id"]],
            "title": node["title"],
            "type": "task",
            "description": node_description(graph["intent_digest"], node),
            "labels": ["gc-factory", "factory-experiment", f"provider:{node['provider']}"],
            "metadata": node_metadata,
            "metadata_refs": {"factory.program_bead": root_key, "factory.refinery_bead": refinery_key},
        })
        for dependency in node["depends_on"]:
            edges.append({"from_key": key_for[node["id"]], "to_key": key_for[dependency], "type": "blocks"})
    nodes.append({
        "key": refinery_key,
        "title": f"Refinery delivery: {graph['program_id']}",
        "type": "task",
        "description": "Integrate exact PASSed candidate SHAs, validate the integration head, and deliver through a protected PR.",
        "labels": ["gc-factory", "factory-refinery"],
        "metadata": {
            "factory.kind": "refinery",
            "factory.schema": "fenced-steward.v1",
            "factory.program_id": graph["program_id"],
            "factory.status": "blocked",
            "factory.repository": graph["repository"],
            "factory.base_branch": graph["base_branch"],
            "factory.base_sha": graph["base_sha"],
            "factory.intent_digest": graph["intent_digest"],
            "factory.graph_digest": graph_digest,
        },
        "metadata_refs": {"factory.program_bead": root_key},
    })
    for node in graph["nodes"]:
        edges.append({"from_key": refinery_key, "to_key": key_for[node["id"]], "type": "blocks"})
    return {
        "commit_message": f"factory: admit {graph['program_id']} bead graph",
        "nodes": nodes,
        "edges": edges,
    }


def admit_program(intent: Path, graph_path: Path, review_path: Path,
                  mayor_context: str, rig: str, binding: str = "factory") -> dict[str, Any]:
    graph = validate_graph(load_object(graph_path, "program graph"), digest_file(intent))
    mayor = require_string(mayor_context, "mayor context")
    review = validate_review(load_object(review_path, "plan review"), graph, digest_file(graph_path), mayor)
    if review["verdict"] != "PASS":
        raise FactoryError("plan_rejected", f"plan review is {review['verdict']}; no beads were admitted")
    plan = compile_bead_plan(graph, digest_file(graph_path), digest_file(review_path), intent, mayor, review["reviewer_context_id"])
    beads = Beads(rig)
    expected = {"program", "refinery", *{f"experiment-{node['id']}" for node in graph["nodes"]}}
    existing = [
        item for item in beads.list_by_metadata("factory.program_id", graph["program_id"])
        if metadata(item).get("factory.kind") in {"program", "experiment", "refinery"}
    ]
    if existing:
        programs = [item for item in existing if metadata(item).get("factory.kind") == "program"]
        refineries = [item for item in existing if metadata(item).get("factory.kind") == "refinery"]
        experiments = {
            metadata(item).get("factory.node_id"): item
            for item in existing if metadata(item).get("factory.kind") == "experiment"
        }
        expected_nodes = {node["id"] for node in graph["nodes"]}
        if len(programs) != 1 or len(refineries) != 1 or set(experiments) != expected_nodes or len(existing) != len(expected):
            raise FactoryError("admission_collision", "existing program bead graph is partial or has duplicate identities")
        graph_digest = digest_file(graph_path)
        review_digest = digest_file(review_path)
        for item in existing:
            item_meta = metadata(item)
            if item_meta.get("factory.graph_digest") != graph_digest:
                raise FactoryError("admission_collision", "existing program bead graph has a different graph digest")
            if item_meta.get("factory.kind") in {"program", "experiment"} and item_meta.get("factory.review_digest") != review_digest:
                raise FactoryError("admission_collision", "existing program bead graph has a different review digest")
        ids = {
            "program": require_string(programs[0].get("id"), "program bead id"),
            "refinery": require_string(refineries[0].get("id"), "refinery bead id"),
            **{
                f"experiment-{node_id}": require_string(item.get("id"), f"experiment {node_id} bead id")
                for node_id, item in experiments.items()
            },
        }
    else:
        ids = beads.graph_create(plan)
    if set(ids) != expected:
        raise FactoryError("graph_apply_failed", f"bead graph result keys differ: {sorted(ids)}")
    runtime_fields = {
        "factory.rig": rig,
        "factory.binding": binding,
        "factory.adapter_path": str(Path(__file__).resolve()),
    }
    for bead_id in ids.values():
        beads.update_metadata(bead_id, runtime_fields)
    return {
        "schema_version": "factory-admission-result.v1",
        "program_id": graph["program_id"],
        "program_bead": ids["program"],
        "refinery_bead": ids["refinery"],
        "experiment_beads": {node["id"]: ids[f"experiment-{node['id']}"] for node in graph["nodes"]},
        "graph_digest": digest_file(graph_path),
        "review_digest": digest_file(review_path),
    }


def command_admit(args: argparse.Namespace) -> int:
    intent = absolute_path(args.intent, "intent", True)
    graph_path = absolute_path(args.graph, "graph", True)
    review_path = absolute_path(args.review, "review", True)
    result = admit_program(intent, graph_path, review_path, args.mayor_context, args.rig, args.binding)
    if args.result:
        write_json_atomic(absolute_path(args.result, "result"), result)
    print(json.dumps(result, sort_keys=True))
    return 0


def command_plan(args: argparse.Namespace) -> int:
    intent = absolute_path(args.intent, "intent", True)
    repository = absolute_path(args.repository, "repository", True, True)
    program_id = require_string(args.program_id, "program_id", ID_RE)
    base_branch = require_string(args.base_branch, "base_branch")
    base_sha = output(["git", "rev-parse", base_branch], cwd=repository)
    require_string(base_sha, "base_sha", SHA_RE)
    evidence = absolute_path(
        args.evidence_dir or str(repository / ".gc" / "agentops-factory" / "planning" / program_id),
        "evidence-dir",
    )
    mayor_request_path = evidence / "mayor-request.json"
    if evidence.exists() and any(evidence.iterdir()) and not mayor_request_path.is_file():
        raise FactoryError("evidence_exists", f"planning evidence directory is not empty: {evidence}")
    evidence.mkdir(parents=True, exist_ok=True)
    intent_digest = digest_file(intent)
    mayor_request = {
        "schema_version": "factory-role-request.v1",
        "request_id": f"{program_id}-mayor",
        "role": "mayor",
        "provider": "codex",
        "program_id": program_id,
        "workspace": str(repository),
        "intent_source": str(intent),
        "intent_digest": intent_digest,
        "repository": str(repository),
        "base_branch": base_branch,
        "base_sha": base_sha,
        "subject_path": None,
        "subject_digest": None,
        "mayor_context_id": None,
        "artifact_path": str(evidence / "program-graph.json"),
        "result_path": str(evidence / "mayor-response.json"),
    }
    write_or_verify_json(mayor_request_path, mayor_request, "Mayor role request")
    mayor = dispatch_role(mayor_request_path, args.rig, args.binding, args.timeout)
    graph_path = Path(mayor["artifact_path"])
    reviewer_provider = args.reviewer_provider
    review_request = {
        "schema_version": "factory-role-request.v1",
        "request_id": f"{program_id}-plan-review",
        "role": "plan-review",
        "provider": reviewer_provider,
        "program_id": program_id,
        "workspace": str(repository),
        "intent_source": str(intent),
        "intent_digest": intent_digest,
        "repository": str(repository),
        "base_branch": base_branch,
        "base_sha": base_sha,
        "subject_path": str(graph_path),
        "subject_digest": digest_file(graph_path),
        "mayor_context_id": mayor["session_context_id"],
        "artifact_path": str(evidence / "plan-review.json"),
        "result_path": str(evidence / "plan-review-response.json"),
    }
    review_request_path = evidence / "plan-review-request.json"
    write_or_verify_json(review_request_path, review_request, "plan-review role request")
    reviewer = dispatch_role(review_request_path, args.rig, args.binding, args.timeout)
    review_path = Path(reviewer["artifact_path"])
    admission = admit_program(intent, graph_path, review_path, mayor["session_context_id"], args.rig, args.binding)
    beads = Beads(args.rig)
    for work_bead in (mayor["work_bead"], reviewer["work_bead"]):
        ensure_dependency(beads, admission["program_bead"], work_bead, "discovered-from")
    Beads(None).update_metadata(mayor["work_bead"], {"factory.program_bead": admission["program_bead"]})
    beads.update_metadata(reviewer["work_bead"], {"factory.program_bead": admission["program_bead"]})
    beads.update_metadata(admission["program_bead"], {
        "factory.planning_bead": mayor["work_bead"],
        "factory.plan_review_bead": reviewer["work_bead"],
    })
    result = {
        "schema_version": "factory-plan-result.v1",
        "planning_bead": mayor["work_bead"],
        "plan_review_bead": reviewer["work_bead"],
        "mayor_context_id": mayor["session_context_id"],
        "reviewer_context_id": reviewer["session_context_id"],
        "graph_path": str(graph_path),
        "review_path": str(review_path),
        "admission": admission,
    }
    if args.result:
        write_json_atomic(absolute_path(args.result, "result"), result)
    print(json.dumps(result, sort_keys=True))
    return 0


def git_head(repo: Path) -> str:
    value = output(["git", "rev-parse", "HEAD"], cwd=repo)
    if not SHA_RE.fullmatch(value):
        raise FactoryError("git_identity", f"unexpected Git HEAD: {value}")
    return value


def command_lease(args: argparse.Namespace) -> int:
    root = absolute_path(args.worktree_root, "worktree-root", directory=True)
    root.mkdir(parents=True, exist_ok=True)
    result = lease_experiment(args.rig, args.bead, root)
    print(json.dumps(result, sort_keys=True))
    return 0


def commit_patch_present(worktree: Path, commit: str) -> bool:
    ancestor = run_process(
        ["git", "merge-base", "--is-ancestor", commit, "HEAD"],
        cwd=worktree, check=False,
    )
    if ancestor.returncode == 0:
        return True
    parent = output(["git", "rev-parse", f"{commit}^"], cwd=worktree)
    cherry = output(["git", "cherry", "HEAD", commit, parent], cwd=worktree)
    lines = [line for line in cherry.splitlines() if line.strip()]
    return len(lines) == 1 and lines[0].startswith("-")


def lease_experiment(rig: str, bead_id: str, worktree_root: Path) -> dict[str, Any]:
    beads = Beads(rig)
    record = beads.show(bead_id)
    meta = metadata(record)
    if meta.get("factory.kind") != "experiment" or record.get("status") != "open":
        raise FactoryError("not_experiment", "only an open experiment bead may be leased")
    phase = meta.get("factory.status")
    if phase not in {"admitted", "ready", "lease_preparing"}:
        raise FactoryError("already_leased", f"experiment bead factory status is {phase}")
    if phase != "lease_preparing" and bead_id not in beads.ready_ids():
        raise FactoryError("bead_not_ready", f"experiment bead {bead_id} is blocked")
    repository = absolute_path(meta.get("factory.repository"), "factory.repository", True, True)
    base_sha = require_string(meta.get("factory.base_sha"), "factory.base_sha", SHA_RE)
    program_id = require_string(meta.get("factory.program_id"), "factory.program_id", ID_RE)
    node_id = require_string(meta.get("factory.node_id"), "factory.node_id", ID_RE)
    attempt = int(meta.get("factory.attempt", "1"))
    program_bead = require_string(meta.get("factory.program_bead"), "factory.program_bead")
    program_records = beads.list_program(program_bead)
    by_node = program_by_node(program_records)
    by_bead = {str(item.get("id")): item for item in program_records if item.get("id")}
    ordered_predecessors: list[dict[str, Any]] = []
    visited: set[str] = set()

    def visit_node(dependency_node: str) -> None:
        dependency = by_node.get(dependency_node)
        if dependency is None:
            raise FactoryError("dependency_missing", f"experiment dependency {dependency_node} has no bead")
        dependency_meta = metadata(dependency)
        successor_chain: set[str] = set()
        while dependency_meta.get("factory.verdict") in {"FAIL", "NOT_PROVEN"}:
            rejected_id = require_string(dependency.get("id"), "rejected dependency bead id")
            if rejected_id in successor_chain:
                raise FactoryError("successor_cycle", f"dependency successor chain cycles at {rejected_id}")
            successor_chain.add(rejected_id)
            successor_id = require_string(dependency_meta.get("factory.successor_bead"), "factory.successor_bead")
            dependency = by_bead.get(successor_id) or beads.show(successor_id)
            dependency_meta = metadata(dependency)
        dependency_id = require_string(dependency.get("id"), "dependency bead id")
        if dependency_id in visited:
            return
        dependency_spec = json.loads(require_string(dependency_meta.get("factory.spec"), "dependency factory.spec"))
        for ancestor in dependency_spec.get("depends_on", []):
            visit_node(ancestor)
        if dependency_meta.get("factory.verdict") != "PASS" or dependency.get("status") != "closed":
            raise FactoryError("dependency_not_admitted", f"dependency bead {dependency_id} has no terminal PASS")
        visited.add(dependency_id)
        ordered_predecessors.append(dependency)

    spec = json.loads(require_string(meta.get("factory.spec"), "factory.spec"))
    for dependency_node in spec.get("depends_on", []):
        visit_node(dependency_node)
    branch = f"gc/candidate/{program_id}/{node_id}/{attempt}"
    worktree = (worktree_root / program_id / f"{node_id}-{attempt}").resolve(strict=False)
    predecessor_beads = [require_string(item.get("id"), "predecessor bead id") for item in ordered_predecessors]
    predecessor_shas = [
        require_string(metadata(item).get("factory.candidate_sha"), "predecessor candidate SHA", SHA_RE)
        for item in ordered_predecessors
    ]
    if phase == "lease_preparing":
        expected = {
            "factory.branch": branch,
            "factory.worktree": str(worktree),
            "factory.predecessor_beads": predecessor_beads,
            "factory.predecessor_shas": predecessor_shas,
        }
        for key, wanted in expected.items():
            actual = metadata_list(meta, key) if isinstance(wanted, list) else meta.get(key)
            if actual != wanted:
                raise FactoryError("lease_identity_mismatch", f"preparing lease {key} changed")
        token = require_string(meta.get("factory.lease_token"), "factory.lease_token")
        epoch = int(meta.get("factory.fence_epoch", "0"))
    else:
        token = secrets.token_hex(24)
        epoch = int(meta.get("factory.fence_epoch", "0")) + 1
        beads.update_metadata(bead_id, {
            "factory.status": "lease_preparing",
            "factory.branch": branch,
            "factory.worktree": str(worktree),
            "factory.lease_token": token,
            "factory.fence_epoch": str(epoch),
            "factory.predecessor_beads": predecessor_beads,
            "factory.predecessor_shas": predecessor_shas,
        })
    worktree.parent.mkdir(parents=True, exist_ok=True)
    branch_exists = run_process(
        ["git", "show-ref", "--verify", "--quiet", f"refs/heads/{branch}"],
        cwd=repository, check=False,
    ).returncode == 0
    if worktree.exists():
        current_branch = output(["git", "branch", "--show-current"], cwd=worktree)
        if current_branch != branch:
            raise FactoryError("worktree_collision", f"preparing worktree is on {current_branch!r}, expected {branch!r}")
    elif branch_exists:
        run_process(["git", "worktree", "add", str(worktree), branch], cwd=repository, timeout=120)
    else:
        run_process(["git", "worktree", "add", "-b", branch, str(worktree), base_sha], cwd=repository, timeout=120)
    if output(["git", "status", "--porcelain", "--untracked-files=no"], cwd=worktree):
        raise FactoryError("lease_dirty", "preparing candidate worktree has tracked changes")
    for predecessor_sha in predecessor_shas:
        if not commit_patch_present(worktree, predecessor_sha):
            run_process(["git", "cherry-pick", predecessor_sha], cwd=worktree, timeout=120)
    candidate_base_sha = git_head(worktree)
    index_raw = output(["git", "rev-parse", "--git-path", "index"], cwd=worktree)
    index = Path(index_raw) if Path(index_raw).is_absolute() else (worktree / index_raw).resolve(strict=False)
    fields = {
        "factory.status": "leased",
        "factory.branch": branch,
        "factory.worktree": str(worktree),
        "factory.git_index": str(index),
        "factory.lease_token": token,
        "factory.fence_epoch": str(epoch),
        "factory.candidate_base_sha": candidate_base_sha,
        "factory.predecessor_beads": predecessor_beads,
        "factory.predecessor_shas": predecessor_shas,
        "factory.execution_phase": "implement_pending",
    }
    beads.update_metadata(bead_id, fields)
    return {
        "bead": bead_id, "branch": branch, "worktree": str(worktree),
        "git_index": str(index), "lease_token": token, "fence_epoch": epoch,
        "candidate_base_sha": candidate_base_sha,
        "predecessor_beads": predecessor_beads,
        "predecessor_shas": predecessor_shas,
    }


def safe_identifier(*parts: str, limit: int = 48) -> str:
    raw = "-".join(parts)
    clean = re.sub(r"[^A-Za-z0-9_-]+", "-", raw).strip("-") or "factory"
    if len(clean) <= limit:
        return clean
    suffix = digest_bytes(raw.encode("utf-8"))[:10]
    return f"{clean[:limit - 11].rstrip('-')}-{suffix}"


def configured_rigs() -> list[dict[str, Any]]:
    value = parse_json_output(
        output([gc_binary(), "--city", city_path(), "rig", "list", "--json"]),
        "gc rig list",
    )
    rigs = value.get("rigs") if isinstance(value, dict) else None
    if not isinstance(rigs, list):
        raise FactoryError("rig_missing", "gc rig list returned no rigs")
    return [item for item in rigs if isinstance(item, dict)]


def configured_agents() -> list[dict[str, Any]]:
    agents_value = parse_json_output(
        output([gc_binary(), "--city", city_path(), "agent", "list", "--json"]),
        "gc agent list",
    )
    agents = agents_value.get("agents") if isinstance(agents_value, dict) else None
    if not isinstance(agents, list):
        raise FactoryError("agent_missing", "gc agent list returned no agents")
    return [item for item in agents if isinstance(item, dict)]


@contextlib.contextmanager
def city_config_lock() -> Any:
    """Serialize dynamic rig registration and the policy patch that follows it."""
    lock_path = Path(city_path()) / ".gc" / "agentops-factory-config.lock"
    lock_path.parent.mkdir(parents=True, exist_ok=True)
    descriptor = os.open(lock_path, os.O_CREAT | os.O_RDWR, 0o600)
    try:
        fcntl.flock(descriptor, fcntl.LOCK_EX)
        yield
    finally:
        fcntl.flock(descriptor, fcntl.LOCK_UN)
        os.close(descriptor)


def enforce_rig_agent_policy(rig_name: str, binding: str, allowed_roles: set[str]) -> None:
    """Make a dedicated worktree rig expose only the bead-selected role routes.

    Gas City composes city imports into every rig and injects provider targets.
    A dynamic candidate rig must therefore receive explicit rig-scoped patches;
    `gc agent suspend` cannot write patches for pack-derived agents.
    """
    require_string(rig_name, "rig name", ID_RE)
    require_string(binding, "factory binding", ID_RE)
    if not allowed_roles or not allowed_roles.issubset(
        {"implementer", "implementer-claude", "validator", "validator-claude"}
    ):
        raise FactoryError("invalid_agent_policy", f"invalid allowed role set: {sorted(allowed_roles)}")
    prefix = f"{rig_name}/"
    expected = {f"{prefix}{binding}.{role}" for role in allowed_roles}
    agents = [item for item in configured_agents() if item.get("dir") == rig_name]
    present = {str(item.get("qualified_name", "")) for item in agents}
    missing = sorted(expected - present)
    if missing:
        raise FactoryError("agent_missing", f"worktree rig is missing bead-selected routes: {missing}")

    city_config = Path(city_path()) / "city.toml"
    try:
        text = city_config.read_text(encoding="utf-8")
        config = tomllib.loads(text)
    except (OSError, tomllib.TOMLDecodeError) as exc:
        raise FactoryError("city_config_invalid", f"cannot read city config {city_config}: {exc}") from exc
    patches = config.get("patches", {}).get("agent", [])
    additions: list[str] = []
    for agent in agents:
        qualified = require_string(agent.get("qualified_name"), "qualified agent name")
        if not qualified.startswith(prefix):
            raise FactoryError("agent_identity", f"rig agent escapes its rig prefix: {qualified}")
        if qualified in expected:
            if agent.get("suspended") is True:
                raise FactoryError("agent_suspended", f"bead-selected route is suspended: {qualified}")
            continue
        local_name = qualified.removeprefix(prefix)
        matches = [
            patch for patch in patches
            if isinstance(patch, dict)
            and patch.get("dir") == rig_name
            and patch.get("name") == local_name
        ]
        if matches:
            if len(matches) != 1 or matches[0].get("suspended") is not True:
                raise FactoryError("agent_policy_collision", f"invalid suspension patch for {qualified}")
            continue
        additions.append(
            "[[patches.agent]]\n"
            f"dir = {json.dumps(rig_name)}\n"
            f"name = {json.dumps(local_name)}\n"
            "suspended = true"
        )
    if additions:
        rendered = text.rstrip() + "\n\n" + "\n\n".join(additions) + "\n"
        try:
            tomllib.loads(rendered)
        except tomllib.TOMLDecodeError as exc:
            raise FactoryError("city_config_invalid", f"generated rig policy is invalid TOML: {exc}") from exc
        write_text_atomic(city_config, rendered)
        output([gc_binary(), "--city", city_path(), "config", "show", "--json"])

    refreshed = [item for item in configured_agents() if item.get("dir") == rig_name]
    active = {
        str(item.get("qualified_name", ""))
        for item in refreshed
        if item.get("suspended") is not True
    }
    if active != expected:
        raise FactoryError(
            "agent_policy_mismatch",
            f"worktree rig active routes differ from bead policy; expected={sorted(expected)} actual={sorted(active)}",
        )


def restore_rig_scaffolding(worktree: Path) -> list[str]:
    """Remove only tracked mutations made synchronously by `gc rig add`.

    This runs before an agent is dispatched in a newly created worktree, so no
    worker-authored bytes exist yet. Untracked GC runtime directories remain.
    """
    changed = [line for line in output(["git", "diff", "--name-only"], cwd=worktree).splitlines() if line]
    for relative in changed:
        normalized = normalize_path(relative, "rig scaffolding path")
        blob = subprocess.run(
            ["git", "show", f"HEAD:{normalized}"], cwd=worktree,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False,
        )
        if blob.returncode != 0:
            raise FactoryError("rig_scaffold_changed", f"gc rig add created an unexpected tracked path: {normalized}")
        destination = worktree / normalized
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(blob.stdout)
    if output(["git", "status", "--porcelain", "--untracked-files=no"], cwd=worktree):
        raise FactoryError("rig_scaffold_changed", "candidate worktree remained tracked-dirty after rig setup cleanup")
    return changed


def primary_worktree(worktree: Path) -> Path:
    raw = output(["git", "worktree", "list", "--porcelain"], cwd=worktree)
    for line in raw.splitlines():
        if line.startswith("worktree "):
            return Path(line.removeprefix("worktree ")).resolve(strict=False)
    raise FactoryError("git_identity", f"cannot find primary worktree for {worktree}")


def add_gc_rig_with_local_origin(worktree: Path, argv: list[str]) -> Any:
    """Initialize per-worktree beads without adopting a forge's Dolt refs.

    Git worktrees share remote configuration. Registration is serialized, so
    the forge URL is replaced with the local primary worktree only for the
    duration of `gc rig add`, then restored byte-for-byte as config values.
    """
    common_dir_raw = output(["git", "rev-parse", "--git-common-dir"], cwd=worktree)
    common_dir = Path(common_dir_raw)
    if not common_dir.is_absolute():
        common_dir = (worktree / common_dir).resolve(strict=False)
    config_path = common_dir / "config"
    urls_raw = run_process(
        ["git", "config", "--file", str(config_path), "--get-all", "remote.origin.url"],
        cwd=worktree, check=False,
    )
    urls = [line for line in urls_raw.stdout.splitlines() if line]
    beads_dir = worktree / ".beads"
    has_adoptable_store = all(
        (beads_dir / name).is_file() for name in ("metadata.json", "config.yaml")
    )
    command = [*argv, "--adopt"] if has_adoptable_store and "--adopt" not in argv else argv
    if urls:
        run_process(["git", "config", "--file", str(config_path), "--unset-all", "remote.origin.url"], cwd=worktree, check=False)
        run_process(["git", "config", "--file", str(config_path), "--add", "remote.origin.url", str(primary_worktree(worktree))], cwd=worktree)
    try:
        raw = output(
            command,
            env={"BD_DOLT_SYNC_CLI_REMOTES": "false", "BEADS_DOLT_SYNC_CLI_REMOTES": "false"},
        )
        return parse_json_output(raw, "gc rig add")
    finally:
        if urls:
            run_process(["git", "config", "--file", str(config_path), "--unset-all", "remote.origin.url"], cwd=worktree, check=False)
            for url in urls:
                run_process(["git", "config", "--file", str(config_path), "--add", "remote.origin.url", url], cwd=worktree)


def register_candidate_rig(lease: dict[str, Any], record: dict[str, Any]) -> tuple[str, str]:
    meta = metadata(record)
    rig_name = safe_identifier("fx", meta["factory.program_id"], meta["factory.node_id"], str(meta.get("factory.attempt", "1")))
    worktree = Path(lease["worktree"])
    binding = require_string(meta.get("factory.binding"), "factory.binding", ID_RE)
    with city_config_lock():
        matches = [item for item in configured_rigs() if item.get("name") == rig_name]
        if matches:
            if len(matches) != 1 or Path(str(matches[0].get("path", ""))).resolve(strict=False) != worktree:
                raise FactoryError("rig_collision", f"candidate rig name {rig_name} is already bound elsewhere")
        else:
            value = add_gc_rig_with_local_origin(
                worktree,
                [
                    gc_binary(), "--city", city_path(), "rig", "add", str(worktree),
                    "--name", rig_name, "--prefix", safe_identifier("fx", meta["factory.node_id"], limit=16),
                    "--default-branch", lease["branch"], "--json",
                ],
            )
            if isinstance(value, dict) and value.get("ok") is False:
                raise FactoryError("rig_add_failed", f"candidate rig add failed: {value!r}")
        provider = require_string(meta.get("factory.provider"), "factory.provider")
        validator_provider = require_string(meta.get("factory.validator_provider"), "factory.validator_provider")
        worker_role = "implementer-claude" if provider == "claude" else "implementer"
        validator_role = "validator-claude" if validator_provider == "claude" else "validator"
        enforce_rig_agent_policy(rig_name, binding, {worker_role, validator_role})
    restore_rig_scaffolding(worktree)
    return rig_name, binding


def register_integration_rig(worktree: Path, refinery_bead: str, branch: str, binding: str) -> tuple[str, str]:
    epoch = require_string(branch.rsplit("/", 1)[-1], "integration branch epoch", ID_RE)
    rig_name = safe_identifier("fx", "refinery", refinery_bead, epoch)
    binding = require_string(binding, "factory.binding", ID_RE)
    with city_config_lock():
        matches = [item for item in configured_rigs() if item.get("name") == rig_name]
        if matches:
            if len(matches) != 1 or Path(str(matches[0].get("path", ""))).resolve(strict=False) != worktree:
                raise FactoryError("rig_collision", f"integration rig name {rig_name} is already bound elsewhere")
        else:
            value = add_gc_rig_with_local_origin(worktree, [
                gc_binary(), "--city", city_path(), "rig", "add", str(worktree),
                "--name", rig_name, "--prefix", safe_identifier("fx", "ref", limit=16),
                "--default-branch", branch, "--json",
            ])
            if isinstance(value, dict) and value.get("ok") is False:
                raise FactoryError("rig_add_failed", f"integration rig add failed: {value!r}")
        enforce_rig_agent_policy(rig_name, binding, {"validator", "validator-claude"})
    restore_rig_scaffolding(worktree)
    return rig_name, binding


def run_executor_packet(packet_path: Path, rig: str, binding: str, timeout: float) -> dict[str, Any]:
    if not EXECUTOR_ADAPTER.is_file():
        raise FactoryError("executor_missing", f"thin executor adapter is missing: {EXECUTOR_ADAPTER}")
    completed = run_process([
        sys.executable, str(EXECUTOR_ADAPTER), "run", "--packet", str(packet_path),
        "--rig", rig, "--binding", binding, "--timeout", str(timeout),
    ], timeout=timeout + 120, check=False)
    value = parse_json_output(completed.stdout, "agentops run-packet")
    if completed.returncode != 0 or not isinstance(value, dict) or value.get("ok") is not True:
        raise FactoryError("executor_failed", f"executor packet failed: {value!r} {completed.stderr.strip()}")
    return value


def load_executor_adapter() -> Any:
    spec = importlib.util.spec_from_file_location("agentops_factory_executor", EXECUTOR_ADAPTER)
    if spec is None or spec.loader is None:
        raise FactoryError("executor_missing", f"cannot load thin executor adapter: {EXECUTOR_ADAPTER}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def execute_experiment(base_rig: str, bead_id: str, lease: dict[str, Any],
                       candidate_rig: str, binding: str, timeout: float,
                       max_attempts: int = 3) -> dict[str, Any]:
    beads = Beads(base_rig)
    record = beads.show(bead_id)
    meta = metadata(record)
    stored_max_attempts = meta.get("factory.max_attempts")
    if stored_max_attempts is not None and int(stored_max_attempts) != max_attempts:
        raise FactoryError("attempt_policy_mismatch", "experiment max-attempt policy changed after leasing")
    beads.update_metadata(bead_id, {"factory.max_attempts": str(max_attempts)})
    meta["factory.max_attempts"] = str(max_attempts)
    worktree = Path(lease["worktree"])
    evidence = worktree / ".gc" / "agentops-factory" / bead_id
    evidence.mkdir(parents=True, exist_ok=True)
    intent_path = evidence / "intent.md"
    description = require_string(record.get("description"), "experiment bead description")
    write_or_verify_text(intent_path, description, "experiment intent")
    if digest_file(intent_path) != meta.get("factory.intent_digest"):
        raise FactoryError("identity_mismatch", "experiment bead description no longer matches its admitted intent digest")
    subject = json.loads(require_string(meta.get("factory.subject"), "factory.subject"))
    write_scope = json.loads(require_string(meta.get("factory.write_scope"), "factory.write_scope"))
    generated_scope = json.loads(require_string(meta.get("factory.generated_scope"), "factory.generated_scope"))
    allowed_scope = list(dict.fromkeys([*write_scope, *generated_scope]))
    implement_packet_id = safe_identifier(bead_id, "implement", limit=120)
    implement_dir = worktree / ".gc" / "agentops" / implement_packet_id
    implement_packet_path = implement_dir / "packet.json"
    implement_packet = {
        "schema_version": "gc-execution-envelope.v1",
        "packet_id": implement_packet_id,
        "role": "implement",
        "provider": meta["factory.provider"],
        "intent_source": str(intent_path),
        "intent_digest": meta["factory.intent_digest"],
        "workspace": str(worktree),
        "subject": subject,
        "write_scope": allowed_scope,
        "evidence_dir": str(implement_dir),
        "result_path": str(implement_dir / "agent-response.json"),
    }
    write_or_verify_json(implement_packet_path, implement_packet, "implement packet")
    implement_result = run_executor_packet(implement_packet_path, candidate_rig, binding, timeout)
    implement_result_path = evidence / "implement-runtime-result.json"
    write_or_verify_json(implement_result_path, implement_result, "implement runtime result")
    runtime = implement_result.get("runtime_evidence", {})
    if runtime.get("scope_status") != "PASS":
        raise FactoryError("scope_failed", f"implementer scope is {runtime.get('scope_status')}")
    changed = runtime.get("actual_changed_paths")
    if not isinstance(changed, list) or not changed or not all(isinstance(path, str) and path for path in changed):
        raise FactoryError("empty_candidate", "implementer produced no committable changed paths")
    first_check = require_string(meta.get("factory.first_check"), "factory.first_check")
    check = run_process(["/bin/sh", "-lc", first_check], cwd=worktree, timeout=600, check=False)
    check_path = evidence / "first-check.json"
    write_json_atomic(check_path, {
        "command": first_check, "exit_code": check.returncode,
        "stdout": check.stdout[-20000:], "stderr": check.stderr[-20000:],
    })
    if check.returncode != 0:
        raise FactoryError("first_check_failed", f"first check failed for {bead_id}: {check.stderr.strip() or check.stdout.strip()}")
    current_head = git_head(worktree)
    candidate_base_sha = require_string(meta.get("factory.candidate_base_sha"), "factory.candidate_base_sha", SHA_RE)
    if current_head == candidate_base_sha:
        run_process(["git", "add", "-A", "--", *changed], cwd=worktree)
        staged = output(["git", "diff", "--cached", "--name-only"], cwd=worktree).splitlines()
        if sorted(staged) != sorted(changed):
            raise FactoryError("stage_mismatch", f"staged paths differ from runtime receipt: staged={staged} changed={changed}")
        run_process(["git", "commit", "-m", f"factory({meta['factory.node_id']}): admitted candidate"], cwd=worktree, timeout=120)
    else:
        commit_count = output(["git", "rev-list", "--count", f"{candidate_base_sha}..HEAD"], cwd=worktree)
        committed = output(["git", "diff", "--name-only", f"{candidate_base_sha}..HEAD"], cwd=worktree).splitlines()
        if commit_count != "1" or sorted(committed) != sorted(changed):
            raise FactoryError("candidate_recovery_mismatch", "existing candidate commit does not match the runtime changed-path receipt")
        if output(["git", "status", "--porcelain", "--untracked-files=no"], cwd=worktree):
            raise FactoryError("candidate_dirty", "recovered candidate worktree has tracked changes")
    candidate_sha = git_head(worktree)
    author_context = require_string(
        implement_result.get("transport", {}).get("session_context_id"), "implementer session context",
    )
    beads.update_metadata(bead_id, {
        "factory.execution_phase": "validation_pending",
        "factory.candidate_sha": candidate_sha,
        "factory.implement_result": str(implement_result_path),
        "factory.author_context_id": author_context,
    })
    validate_packet_id = safe_identifier(bead_id, "validate", limit=120)
    validate_dir = worktree / ".gc" / "agentops" / validate_packet_id
    validate_packet_path = validate_dir / "packet.json"
    validate_packet = {
        "schema_version": "gc-execution-envelope.v1",
        "packet_id": validate_packet_id,
        "role": "validate",
        "provider": meta["factory.validator_provider"],
        "intent_source": str(intent_path),
        "intent_digest": meta["factory.intent_digest"],
        "workspace": str(worktree),
        "subject": subject,
        "write_scope": [],
        "evidence_dir": str(validate_dir),
        "result_path": str(validate_dir / "agent-response.json"),
        "baseline_manifest": runtime["baseline_manifest"],
        "subject_manifest": runtime["subject_manifest"],
        "scope_receipt": runtime["scope_receipt"],
        "author_context_id": author_context,
    }
    write_or_verify_json(validate_packet_path, validate_packet, "validate packet")
    validate_result = run_executor_packet(validate_packet_path, candidate_rig, binding, timeout)
    validate_result_path = evidence / "validate-runtime-result.json"
    write_or_verify_json(validate_result_path, validate_result, "validate runtime result")
    artifacts = validate_result.get("agent_response", {}).get("artifacts")
    if not isinstance(artifacts, list) or len(artifacts) != 1 or not isinstance(artifacts[0], dict):
        raise FactoryError("verdict_missing", "Validator returned no exact verdict artifact")
    verdict_path = absolute_path(artifacts[0].get("path"), "verdict artifact", True)
    verdict_args = argparse.Namespace(
        rig=base_rig, bead=bead_id, lease_token=lease["lease_token"],
        fence_epoch=lease["fence_epoch"], candidate_sha=candidate_sha,
        subject_manifest=runtime["subject_manifest"], author_context=author_context,
        verdict=str(verdict_path),
    )
    with open(os.devnull, "w", encoding="utf-8") as sink, contextlib.redirect_stdout(sink):
        command_record_verdict(verdict_args)
    final_record = Beads(base_rig).show(bead_id)
    final_meta = metadata(final_record)
    result = {
        "bead": bead_id,
        "node_id": meta["factory.node_id"],
        "candidate_rig": candidate_rig,
        "branch": lease["branch"],
        "candidate_sha": candidate_sha,
        "verdict": final_meta.get("factory.verdict"),
        "implementer_context_id": author_context,
        "validator_context_id": validate_result["transport"]["session_context_id"],
    }
    if result["verdict"] in {"FAIL", "NOT_PROVEN"}:
        rescope_bead = require_string(final_meta.get("factory.rescope_bead"), "factory.rescope_bead")
        result["rescope_bead"] = rescope_bead
        rescope_phase = metadata(Beads(base_rig).show(rescope_bead)).get("factory.status")
        if rescope_phase in {"mayor_required", "successor_preparing", "successor_admitted"}:
            rescope_result = rescope_rejection(base_rig, rescope_bead, timeout)
            result["successor_bead"] = rescope_result["successor_bead"]
        elif rescope_phase == "hold":
            result["hold"] = True
        else:
            raise FactoryError("invalid_rescope", f"rejected experiment produced rescope phase {rescope_phase!r}")
    return result


def lease_snapshot(bead_id: str, meta: dict[str, Any]) -> dict[str, Any]:
    return {
        "bead": bead_id,
        "branch": require_string(meta.get("factory.branch"), "factory.branch"),
        "worktree": str(absolute_path(meta.get("factory.worktree"), "factory.worktree", True, True)),
        "git_index": require_string(meta.get("factory.git_index"), "factory.git_index"),
        "lease_token": require_string(meta.get("factory.lease_token"), "factory.lease_token"),
        "fence_epoch": int(meta.get("factory.fence_epoch", "0")),
        "candidate_base_sha": require_string(meta.get("factory.candidate_base_sha"), "factory.candidate_base_sha", SHA_RE),
        "predecessor_beads": metadata_list(meta, "factory.predecessor_beads"),
        "predecessor_shas": metadata_list(meta, "factory.predecessor_shas"),
    }


def command_execute(args: argparse.Namespace) -> int:
    worktree_root = absolute_path(
        args.worktree_root or str(Path(city_path()) / ".gc" / "factory-worktrees"),
        "worktree-root", directory=True,
    )
    worktree_root.mkdir(parents=True, exist_ok=True)
    beads = Beads(args.rig)
    program = beads.show(args.program_bead)
    program_meta = metadata(program)
    if program_meta.get("factory.kind") != "program":
        raise FactoryError("not_program", f"{args.program_bead} is not a factory program bead")
    stored_max_attempts = program_meta.get("factory.max_attempts")
    if stored_max_attempts is not None and int(stored_max_attempts) != args.max_attempts:
        raise FactoryError("attempt_policy_mismatch", "program max-attempt policy changed after execution began")
    beads.update_metadata(args.program_bead, {"factory.max_attempts": str(args.max_attempts)})
    program_meta["factory.max_attempts"] = str(args.max_attempts)
    waves: list[list[dict[str, Any]]] = []
    recoveries: list[dict[str, Any]] = []
    selected = set(args.bead or [])
    while True:
        ready = beads.ready_ids()
        records = beads.list_program(args.program_bead)
        pending_rescopes = sorted(
            (
                record for record in records
                if record.get("status") == "open"
                and metadata(record).get("factory.kind") == "rescope"
                and metadata(record).get("factory.status") == "mayor_required"
            ),
            key=lambda item: str(item.get("id")),
        )
        for rescope in pending_rescopes:
            rescope_meta = metadata(rescope)
            rejected_bead = require_string(rescope_meta.get("factory.rejected_bead"), "factory.rejected_bead")
            if selected and rejected_bead not in selected and str(rescope.get("id")) not in selected:
                continue
            rejected_record = beads.show(rejected_bead)
            rejected_meta = metadata(rejected_record)
            if rejected_meta.get("factory.status") != "rejected" or rejected_record.get("status") != "closed":
                if selected and str(rescope.get("id")) in selected:
                    selected.add(rejected_bead)
                continue
            attempt = int(rejected_meta.get("factory.attempt", "1"))
            maximum = int(rejected_meta.get("factory.max_attempts", program_meta["factory.max_attempts"]))
            if attempt >= maximum:
                beads.update_metadata(str(rescope["id"]), {
                    "factory.status": "hold",
                    "factory.hold_reason": f"automatic rescope stopped at attempt {attempt} of {maximum}",
                })
                recoveries.append({"rescope_bead": str(rescope["id"]), "status": "hold"})
                continue
            rescope_result = rescope_rejection(args.rig, str(rescope["id"]), args.timeout)
            recoveries.append({**rescope_result, "status": "successor_admitted"})
            if selected and (rejected_bead in selected or str(rescope.get("id")) in selected):
                selected.add(rescope_result["successor_bead"])
        if pending_rescopes:
            ready = beads.ready_ids()
            records = beads.list_program(args.program_bead)
        runnable = []
        for record in records:
            meta = metadata(record)
            phase = meta.get("factory.status")
            if meta.get("factory.kind") != "experiment" or phase not in {
                "admitted", "ready", "lease_preparing", "leased",
                "passed", "rejection_preparing", "rejected",
            }:
                continue
            if record.get("status") == "closed":
                continue
            if phase in {"admitted", "ready"} and record.get("id") not in ready:
                continue
            if selected and record.get("id") not in selected:
                continue
            runnable.append(record)
        runnable.sort(key=lambda item: str(item.get("id")))
        if not runnable:
            break
        runnable = runnable[:args.max_parallel]
        prepared = []
        for record in runnable:
            bead_id = str(record["id"])
            meta = metadata(record)
            stored = meta.get("factory.max_attempts")
            if stored is not None and int(stored) != args.max_attempts:
                raise FactoryError("attempt_policy_mismatch", f"experiment {bead_id} max-attempt policy changed")
            beads.update_metadata(bead_id, {"factory.max_attempts": str(args.max_attempts)})
            if meta.get("factory.status") == "lease_preparing":
                preparing_worktree = absolute_path(meta.get("factory.worktree"), "factory.worktree")
                try:
                    preparing_root = preparing_worktree.parents[1]
                except IndexError as exc:
                    raise FactoryError("invalid_contract", "preparing worktree has no factory root") from exc
                lease_experiment(args.rig, bead_id, preparing_root)
                record = beads.show(bead_id)
                meta = metadata(record)
            if meta.get("factory.status") in {"leased", "passed", "rejection_preparing", "rejected"}:
                lease = lease_snapshot(bead_id, meta)
            else:
                lease = lease_experiment(args.rig, bead_id, worktree_root)
            candidate_rig, binding = register_candidate_rig(lease, record)
            beads.update_metadata(bead_id, {
                "factory.candidate_rig": candidate_rig,
                "factory.executor_binding": binding,
            })
            prepared.append((bead_id, lease, candidate_rig, binding))
        results: list[dict[str, Any]] = []
        with ThreadPoolExecutor(max_workers=len(prepared), thread_name_prefix="gc-factory") as pool:
            futures = {
                pool.submit(
                    execute_experiment, args.rig, bead_id, lease, candidate_rig,
                    binding, args.timeout, args.max_attempts,
                ): bead_id
                for bead_id, lease, candidate_rig, binding in prepared
            }
            for future in as_completed(futures):
                bead_id = futures[future]
                try:
                    results.append(future.result())
                except Exception as exc:
                    beads.update_metadata(bead_id, {"factory.last_error": str(exc)[:4000]})
                    raise
        results.sort(key=lambda item: item["bead"])
        waves.append(results)
        if selected:
            selected.difference_update(item["bead"] for item in results)
            if not selected:
                break
    result = {
        "schema_version": "factory-execution-result.v1",
        "program_bead": args.program_bead,
        "waves": waves,
        "executed": sum(len(wave) for wave in waves),
        "reconciled": recoveries,
        "refinery_ready": program_meta.get("factory.refinery_bead") in beads.ready_ids(),
    }
    if not waves and not recoveries:
        raise FactoryError("no_ready_experiments", "program has no admitted ready experiment beads")
    if args.result:
        write_json_atomic(absolute_path(args.result, "result"), result)
    print(json.dumps(result, sort_keys=True))
    return 0


def command_resume_experiment(args: argparse.Namespace) -> int:
    beads = Beads(args.rig)
    record = beads.show(args.bead)
    meta = metadata(record)
    if meta.get("factory.kind") != "experiment" or meta.get("factory.status") not in {"lease_preparing", "leased"}:
        raise FactoryError("invalid_transition", "resume requires a preparing or already fenced experiment bead")
    if meta.get("factory.status") == "lease_preparing":
        preparing_worktree = absolute_path(meta.get("factory.worktree"), "factory.worktree")
        try:
            worktree_root = preparing_worktree.parents[1]
        except IndexError as exc:
            raise FactoryError("invalid_contract", "preparing worktree has no factory root") from exc
        lease_experiment(args.rig, args.bead, worktree_root)
        record = beads.show(args.bead)
        meta = metadata(record)
    lease = lease_snapshot(args.bead, meta)
    candidate_rig, binding = register_candidate_rig(lease, record)
    beads.update_metadata(args.bead, {
        "factory.candidate_rig": candidate_rig,
        "factory.executor_binding": binding,
        "factory.last_error": "",
    })
    result = execute_experiment(
        args.rig, args.bead, lease, candidate_rig, binding,
        args.timeout, args.max_attempts,
    )
    if args.result:
        write_json_atomic(absolute_path(args.result, "result"), result)
    print(json.dumps(result, sort_keys=True))
    return 0


def validate_verdict(verdict: dict[str, Any], meta: dict[str, Any], subject_digest: str,
                     author_context: str) -> tuple[str, str]:
    if verdict.get("schema_version") != "verdict.v2" or verdict.get("verdict") not in VERDICTS:
        raise FactoryError("invalid_verdict", "artifact is not verdict.v2")
    expected = {
        "acceptance_digest": meta.get("factory.intent_digest"),
        "subject_manifest_digest": subject_digest,
        "author_context_id": author_context,
    }
    for key, wanted in expected.items():
        if verdict.get(key) != wanted:
            raise FactoryError("verdict_binding_mismatch", f"verdict {key} is stale")
    validator = require_string(verdict.get("validator_context_id"), "validator context")
    if validator == author_context:
        raise FactoryError("freshness_collision", "Validator context equals author context")
    if verdict.get("freshness_attestation") != {"source": "runtime", "attester_identity": validator}:
        raise FactoryError("verdict_binding_mismatch", "freshness attestation is not runtime-bound")
    return verdict["verdict"], validator


def command_record_verdict(args: argparse.Namespace) -> int:
    beads = Beads(args.rig)
    record = beads.show(args.bead)
    meta = metadata(record)
    phase = meta.get("factory.status")
    if meta.get("factory.kind") != "experiment" or phase not in {"leased", "passed", "rejection_preparing", "rejected"}:
        raise FactoryError("invalid_transition", "verdict requires a leased or reconcilable terminal experiment bead")
    if meta.get("factory.lease_token") != args.lease_token or int(meta.get("factory.fence_epoch", "0")) != args.fence_epoch:
        raise FactoryError("stale_fence", "experiment lease token or fence epoch is stale")
    worktree = absolute_path(meta.get("factory.worktree"), "factory.worktree", True, True)
    candidate_sha = require_string(args.candidate_sha, "candidate SHA", SHA_RE)
    if git_head(worktree) != candidate_sha:
        raise FactoryError("candidate_moved", "candidate HEAD differs from the frozen SHA")
    if output(["git", "status", "--porcelain", "--untracked-files=no"], cwd=worktree):
        raise FactoryError("candidate_dirty", "tracked candidate content is not committed")
    manifest_path = absolute_path(args.subject_manifest, "subject manifest", True)
    manifest = load_object(manifest_path, "subject manifest")
    subject_digest = require_string(manifest.get("canonical_manifest_digest"), "subject manifest digest", DIGEST_RE)
    verdict_path = absolute_path(args.verdict, "verdict", True)
    verdict = load_object(verdict_path, "verdict")
    result, validator = validate_verdict(verdict, meta, subject_digest, args.author_context)
    if phase in {"passed", "rejection_preparing", "rejected"}:
        stored_result = meta.get("factory.verdict")
        if stored_result != result or meta.get("factory.verdict_digest") != digest_file(verdict_path):
            raise FactoryError("verdict_binding_mismatch", "replayed verdict differs from the bead's durable verdict")
    superseded_bead = meta.get("factory.supersedes_bead")
    if superseded_bead:
        superseded_meta = metadata(beads.show(require_string(superseded_bead, "factory.supersedes_bead")))
        if superseded_meta.get("factory.author_context_id") == args.author_context:
            raise FactoryError("freshness_collision", "successor reused the rejected experiment's Worker context")
    common = {
        "factory.status": "passed" if result == "PASS" else "rejected",
        "factory.verdict": result,
        "factory.verdict_path": str(verdict_path),
        "factory.verdict_digest": digest_file(verdict_path),
        "factory.candidate_sha": candidate_sha,
        "factory.subject_manifest_digest": subject_digest,
        "factory.author_context_id": args.author_context,
        "factory.validator_context_id": validator,
    }
    if result == "PASS":
        certificate = {
            "schema_version": "admission-certificate.v1",
            "program_id": meta["factory.program_id"],
            "node_id": meta["factory.node_id"],
            "experiment_id": args.bead,
            "intent_digest": meta["factory.intent_digest"],
            "candidate_sha": candidate_sha,
            "candidate_base_sha": require_string(meta.get("factory.candidate_base_sha"), "candidate base SHA", SHA_RE),
            "predecessor_beads": metadata_list(meta, "factory.predecessor_beads"),
            "predecessor_shas": metadata_list(meta, "factory.predecessor_shas"),
            "subject_manifest_digest": subject_digest,
            "author_context_id": args.author_context,
            "validator_context_id": validator,
            "verdict_digest": digest_file(verdict_path),
            "verdict": "PASS",
            "lease_token": args.lease_token,
            "fence_epoch": args.fence_epoch,
        }
        certificate_path = worktree / ".gc" / "agentops-factory" / args.bead / "admission.json"
        write_json_atomic(certificate_path, certificate)
        common.update({
            "factory.admission_path": str(certificate_path),
            "factory.admission_digest": digest_file(certificate_path),
        })
        beads.update_metadata(args.bead, common)
        if record.get("status") != "closed":
            beads.close(args.bead, "Factory experiment PASS: exact candidate admitted to Refinery")
        refinery = require_string(meta.get("factory.refinery_bead"), "factory.refinery_bead")
        if refinery in beads.ready_ids():
            beads.update_metadata(refinery, {"factory.status": "ready"})
        print(json.dumps({"bead": args.bead, "verdict": "PASS", "admission": str(certificate_path)}, sort_keys=True))
        return 0

    refinery = require_string(meta.get("factory.refinery_bead"), "factory.refinery_bead")
    attempt = int(meta.get("factory.attempt", "1"))
    max_attempts = int(meta.get("factory.max_attempts", "3"))
    at_ceiling = attempt >= max_attempts
    rescope_meta = {
        "factory.kind": "rescope",
        "factory.schema": "fenced-steward.v1",
        "factory.program_id": meta["factory.program_id"],
        "factory.program_bead": meta["factory.program_bead"],
        "factory.refinery_bead": refinery,
        "factory.rejected_bead": args.bead,
        "factory.attempt": str(attempt),
        "factory.max_attempts": str(max_attempts),
        "factory.verdict": result,
        "factory.verdict_path": str(verdict_path),
        "factory.verdict_digest": digest_file(verdict_path),
        "factory.status": "hold" if at_ceiling else "mayor_required",
        "factory.rig": require_string(meta.get("factory.rig"), "factory.rig"),
        "factory.binding": require_string(meta.get("factory.binding"), "factory.binding", ID_RE),
        "factory.adapter_path": require_string(meta.get("factory.adapter_path"), "factory.adapter_path"),
    }
    if at_ceiling:
        rescope_meta["factory.hold_reason"] = (
            f"automatic rescope stopped at attempt {attempt} of {max_attempts}"
        )
    preparing = {**common, "factory.status": "rejection_preparing"}
    beads.update_metadata(args.bead, preparing)
    program_records = beads.list_program(meta["factory.program_bead"])
    matching_rescopes = [
        item for item in program_records
        if metadata(item).get("factory.kind") == "rescope"
        and metadata(item).get("factory.rejected_bead") == args.bead
    ]
    if len(matching_rescopes) > 1:
        raise FactoryError("duplicate_rescope", f"rejected experiment {args.bead} has multiple rescope beads")
    if matching_rescopes:
        rescope = require_string(matching_rescopes[0].get("id"), "rescope bead id")
        existing_rescope_meta = metadata(matching_rescopes[0])
        for key in ("factory.program_id", "factory.program_bead", "factory.refinery_bead", "factory.verdict_digest"):
            if existing_rescope_meta.get(key) != rescope_meta[key]:
                raise FactoryError("rescope_identity_mismatch", f"existing rescope {key} differs from the rejected bead")
        if at_ceiling and existing_rescope_meta.get("factory.status") != "hold":
            beads.update_metadata(rescope, {
                "factory.status": "hold",
                "factory.attempt": str(attempt),
                "factory.max_attempts": str(max_attempts),
                "factory.hold_reason": rescope_meta["factory.hold_reason"],
            })
    else:
        rescope = beads.create(
            f"Mayor rescope after {result}: {meta['factory.node_id']}",
            "The exact experiment is terminal. Propose a newly identified successor with fresh scope and Worker; do not repair or resume the rejected candidate.",
            rescope_meta,
            ["gc-factory", "factory-rescope"],
        )
    ensure_dependency(beads, refinery, rescope)
    dependent_beads: list[str] = []
    rejected_node = meta["factory.node_id"]
    for item in beads.list_program(meta["factory.program_bead"]):
        item_meta = metadata(item)
        if item.get("id") == args.bead or item_meta.get("factory.kind") != "experiment" or item.get("status") != "open":
            continue
        try:
            item_spec = json.loads(require_string(item_meta.get("factory.spec"), "factory.spec"))
        except json.JSONDecodeError as exc:
            raise FactoryError("invalid_bead", f"experiment {item.get('id')} has invalid factory.spec: {exc}") from exc
        if rejected_node in item_spec.get("depends_on", []):
            dependent = require_string(item.get("id"), "dependent bead id")
            ensure_dependency(beads, dependent, rescope)
            dependent_beads.append(dependent)
    beads.update_metadata(rescope, {"factory.dependent_beads": dependent_beads})
    common.update({"factory.status": "rejected", "factory.rescope_bead": rescope})
    beads.update_metadata(args.bead, common)
    if record.get("status") != "closed":
        beads.close(args.bead, f"Factory experiment {result}: returned to Mayor as {rescope}")
    print(json.dumps({"bead": args.bead, "verdict": result, "rescope_bead": rescope}, sort_keys=True))
    return 0


def program_by_node(records: list[dict[str, Any]]) -> dict[str, dict[str, Any]]:
    result: dict[str, dict[str, Any]] = {}
    for record in records:
        meta = metadata(record)
        if meta.get("factory.kind") == "experiment" and isinstance(meta.get("factory.node_id"), str):
            result[meta["factory.node_id"]] = record
    return result


def reconcile_rescope_transport(rescope_meta: dict[str, Any], rescope_bead: str,
                                successor_bead: str) -> None:
    transport = rescope_meta.get("factory.rescope_transport_bead")
    if not transport:
        return
    Beads(None).update_metadata(require_string(transport, "factory.rescope_transport_bead"), {
        "factory.program_bead": require_string(rescope_meta.get("factory.program_bead"), "factory.program_bead"),
        "factory.rescope_bead": rescope_bead,
        "factory.rejected_bead": require_string(rescope_meta.get("factory.rejected_bead"), "factory.rejected_bead"),
        "factory.successor_bead": successor_bead,
    })


def rescope_rejection(rig: str, rescope_bead: str, timeout: float) -> dict[str, Any]:
    beads = Beads(rig)
    rescope = beads.show(rescope_bead)
    rescope_meta = metadata(rescope)
    if rescope_meta.get("factory.kind") != "rescope":
        raise FactoryError("invalid_rescope", f"{rescope_bead} is not a rescope bead")
    existing_successor = rescope_meta.get("factory.successor_bead")
    if existing_successor:
        successor_id = require_string(existing_successor, "factory.successor_bead")
        successor = beads.show(successor_id)
        successor_meta = metadata(successor)
        if successor_meta.get("factory.supersedes_bead") != rescope_meta.get("factory.rejected_bead"):
            raise FactoryError("invalid_rescope", "recorded successor does not supersede the rejected experiment")
        proposal_value = rescope_meta.get("factory.successor_proposal")
        if proposal_value:
            proposal_path = absolute_path(proposal_value, "factory.successor_proposal", True)
            if digest_file(proposal_path) != rescope_meta.get("factory.successor_proposal_digest"):
                raise FactoryError("identity_mismatch", "recorded successor proposal changed")
            with open(os.devnull, "w", encoding="utf-8") as sink, contextlib.redirect_stdout(sink):
                command_successor(argparse.Namespace(
                    rig=rig,
                    rejected_bead=rescope_meta["factory.rejected_bead"],
                    rescope_bead=rescope_bead,
                    proposal=str(proposal_path),
                ))
        reconcile_rescope_transport(metadata(beads.show(rescope_bead)), rescope_bead, successor_id)
        return {
            "rescope_bead": rescope_bead,
            "successor_bead": successor_id,
            "transport_bead": rescope_meta.get("factory.rescope_transport_bead"),
            "mayor_context_id": rescope_meta.get("factory.rescope_mayor_context_id"),
        }
    if rescope_meta.get("factory.status") == "successor_preparing":
        proposal_path = absolute_path(
            rescope_meta.get("factory.successor_proposal"), "factory.successor_proposal", True,
        )
        if digest_file(proposal_path) != rescope_meta.get("factory.successor_proposal_digest"):
            raise FactoryError("identity_mismatch", "preparing successor proposal changed")
        with open(os.devnull, "w", encoding="utf-8") as sink, contextlib.redirect_stdout(sink):
            command_successor(argparse.Namespace(
                rig=rig,
                rejected_bead=rescope_meta["factory.rejected_bead"],
                rescope_bead=rescope_bead,
                proposal=str(proposal_path),
            ))
        refreshed = metadata(beads.show(rescope_bead))
        successor_id = require_string(refreshed.get("factory.successor_bead"), "factory.successor_bead")
        reconcile_rescope_transport(refreshed, rescope_bead, successor_id)
        return {
            "rescope_bead": rescope_bead,
            "successor_bead": successor_id,
            "transport_bead": refreshed.get("factory.rescope_transport_bead"),
            "mayor_context_id": refreshed.get("factory.rescope_mayor_context_id"),
        }
    if rescope.get("status") != "open" or rescope_meta.get("factory.status") not in {"mayor_required", "hold"}:
        raise FactoryError("invalid_rescope", "rescope bead is not awaiting the Mayor")
    rejected_bead = require_string(rescope_meta.get("factory.rejected_bead"), "factory.rejected_bead")
    rejected = beads.show(rejected_bead)
    rejected_meta = metadata(rejected)
    if rejected_meta.get("factory.status") != "rejected" or rejected.get("status") != "closed":
        raise FactoryError("invalid_rescope", "rescope source is not a terminal closed rejected experiment")
    program_bead = require_string(rescope_meta.get("factory.program_bead"), "factory.program_bead")
    program_meta = metadata(beads.show(program_bead))
    repository = absolute_path(program_meta.get("factory.repository"), "factory.repository", True, True)
    intent_source = absolute_path(program_meta.get("factory.intent_source"), "factory.intent_source", True)
    intent_digest = require_string(program_meta.get("factory.intent_digest"), "factory.intent_digest", DIGEST_RE)
    if digest_file(intent_source) != intent_digest:
        raise FactoryError("identity_mismatch", "canonical program intent changed before Mayor rescope")
    dependent_beads = metadata_list(rescope_meta, "factory.dependent_beads")
    rejected_spec = json.loads(require_string(rejected_meta.get("factory.spec"), "factory.spec"))
    verdict_path = absolute_path(rejected_meta.get("factory.verdict_path"), "factory.verdict_path", True)
    verdict_digest = require_string(
        rejected_meta.get("factory.verdict_digest"), "factory.verdict_digest", DIGEST_RE,
    )
    if digest_file(verdict_path) != verdict_digest:
        raise FactoryError("identity_mismatch", "rejected verdict changed before Mayor rescope")
    context = {
        "schema_version": "rescope-context.v1",
        "program_id": require_string(rescope_meta.get("factory.program_id"), "factory.program_id", ID_RE),
        "program_bead": program_bead,
        "rescope_bead": rescope_bead,
        "rejected_bead": rejected_bead,
        "rejected_node_id": require_string(rejected_meta.get("factory.node_id"), "factory.node_id", ID_RE),
        "rejected_attempt": int(rejected_meta.get("factory.attempt", "1")),
        "rejected_spec": rejected_spec,
        "verdict": rejected_meta.get("factory.verdict"),
        "verdict_digest": verdict_digest,
        "verdict_path": str(verdict_path),
        "canonical_intent_source": str(intent_source),
        "canonical_intent_digest": intent_digest,
        "dependent_beads": dependent_beads,
    }
    validate_rescope_context(context)
    evidence = repository / ".gc" / "agentops-factory" / "rescope" / safe_identifier(rescope_bead, limit=80)
    evidence.mkdir(parents=True, exist_ok=True)
    context_path = evidence / "context.json"
    proposal_path = evidence / "successor-node.json"
    result_path = evidence / "mayor-response.json"
    request_path = evidence / "mayor-request.json"
    write_or_verify_json(context_path, context, "rescope context")
    request = {
        "schema_version": "factory-role-request.v1",
        "request_id": safe_identifier(rescope_bead, "mayor", limit=120),
        "role": "rescope",
        "provider": "codex",
        "program_id": context["program_id"],
        "workspace": str(repository),
        "intent_source": str(intent_source),
        "intent_digest": intent_digest,
        "repository": str(repository),
        "base_branch": require_string(program_meta.get("factory.base_branch"), "factory.base_branch"),
        "base_sha": require_string(program_meta.get("factory.base_sha"), "factory.base_sha", SHA_RE),
        "subject_path": str(context_path),
        "subject_digest": digest_file(context_path),
        "mayor_context_id": require_string(
            rejected_meta.get("factory.mayor_context_id") or program_meta.get("factory.mayor_context_id"),
            "factory.mayor_context_id",
        ),
        "artifact_path": str(proposal_path),
        "result_path": str(result_path),
    }
    write_or_verify_json(request_path, request, "rescope role request")
    transport_bead = rescope_meta.get("factory.rescope_transport_bead")
    mayor = dispatch_role(
        request_path, rig,
        require_string(rescope_meta.get("factory.binding"), "factory.binding", ID_RE),
        timeout,
        existing_work_bead=str(transport_bead) if transport_bead else None,
        linked_bead=rescope_bead,
    )
    beads.update_metadata(rescope_bead, {
        "factory.rescope_context": str(context_path),
        "factory.rescope_context_digest": digest_file(context_path),
        "factory.rescope_request": str(request_path),
        "factory.rescope_transport_bead": mayor["work_bead"],
        "factory.rescope_mayor_context_id": mayor["session_context_id"],
    })
    with open(os.devnull, "w", encoding="utf-8") as sink, contextlib.redirect_stdout(sink):
        command_successor(argparse.Namespace(
            rig=rig, rejected_bead=rejected_bead, rescope_bead=rescope_bead,
            proposal=str(proposal_path),
        ))
    refreshed = metadata(beads.show(rescope_bead))
    successor = require_string(refreshed.get("factory.successor_bead"), "factory.successor_bead")
    reconcile_rescope_transport(refreshed, rescope_bead, successor)
    return {
        "rescope_bead": rescope_bead,
        "successor_bead": successor,
        "transport_bead": mayor["work_bead"],
        "mayor_context_id": mayor["session_context_id"],
    }


def command_rescope(args: argparse.Namespace) -> int:
    result = rescope_rejection(args.rig, args.rescope_bead, args.timeout)
    print(json.dumps(result, sort_keys=True))
    return 0


def command_successor(args: argparse.Namespace) -> int:
    beads = Beads(args.rig)
    rejected = beads.show(args.rejected_bead)
    rejected_meta = metadata(rejected)
    rescope = beads.show(args.rescope_bead)
    rescope_meta = metadata(rescope)
    if rejected_meta.get("factory.status") != "rejected" or rejected_meta.get("factory.verdict") not in {"FAIL", "NOT_PROVEN"}:
        raise FactoryError("invalid_successor", "successor source is not a rejected experiment")
    if rescope_meta.get("factory.rejected_bead") != args.rejected_bead or rescope.get("status") not in {"open", "closed"}:
        raise FactoryError("invalid_successor", "rescope bead does not own the rejected experiment")
    proposal_path = absolute_path(args.proposal, "proposal", True)
    proposal = validate_node(load_object(proposal_path, "successor proposal"), "successor")
    if proposal.get("supersedes") != rejected_meta.get("factory.node_id"):
        raise FactoryError("invalid_successor", "successor must name the rejected node in supersedes")
    program = rejected_meta["factory.program_bead"]
    program_records = beads.list_program(program)
    existing = program_by_node(program_records)
    rejected_spec = json.loads(require_string(rejected_meta.get("factory.spec"), "factory.spec"))
    if proposal["acceptance"] != rejected_spec["acceptance"] or proposal["non_goals"] != rejected_spec["non_goals"]:
        raise FactoryError("acceptance_changed", "successor changed acceptance or non-goals")
    rescope_fields = (
        "intent", "depends_on", "write_scope", "generated_scope", "subject",
        "first_check", "provider", "validator_provider",
    )
    if all(proposal[field] == rejected_spec[field] for field in rescope_fields):
        raise FactoryError("invalid_successor", "successor must rescope at least one execution field")
    for dep in proposal["depends_on"]:
        if dep not in existing:
            raise FactoryError("invalid_successor", f"successor dependency {dep} has no program bead")
        if dep == rejected_meta["factory.node_id"]:
            raise FactoryError("invalid_successor", "successor cannot depend on the rejected node it supersedes")
    graph_stub = {
        "program_id": rejected_meta["factory.program_id"],
        "intent_digest": rejected_meta["factory.parent_intent_digest"],
        "repository": rejected_meta["factory.repository"],
        "base_branch": rejected_meta["factory.base_branch"],
        "base_sha": rejected_meta["factory.base_sha"],
    }
    graph_digest = rejected_meta["factory.graph_digest"]
    review_digest = rejected_meta["factory.review_digest"]
    intent_source = Path(rejected_meta["factory.intent_source"])
    meta = experiment_metadata(graph_stub, proposal, graph_digest, review_digest, intent_source)
    attempt = int(rejected_meta.get("factory.attempt", "1")) + 1
    meta["factory.attempt"] = str(attempt)
    meta["factory.supersedes_bead"] = args.rejected_bead
    for key in ("factory.rig", "factory.binding", "factory.adapter_path"):
        meta[key] = require_string(rejected_meta.get(key), key)
    meta["factory.mayor_context_id"] = require_string(
        rescope_meta.get("factory.rescope_mayor_context_id"),
        "factory.rescope_mayor_context_id",
    )
    plan = {
        "commit_message": f"factory: successor for {args.rejected_bead}",
        "nodes": [{
            "key": "successor",
            "title": proposal["title"],
            "type": "task",
            "description": node_description(graph_stub["intent_digest"], proposal),
            "labels": ["gc-factory", "factory-experiment", f"provider:{proposal['provider']}"],
            "metadata": meta,
            "metadata_refs": {"factory.program_bead": "program-existing"},
        }],
        "edges": [],
    }
    # graph apply cannot metadata-ref an existing bead. Bind it directly.
    plan["nodes"][0].pop("metadata_refs")
    meta["factory.program_bead"] = program
    meta["factory.refinery_bead"] = rejected_meta["factory.refinery_bead"]
    for dep in proposal["depends_on"]:
        plan["edges"].append({"from_key": "successor", "to_id": existing[dep]["id"], "type": "blocks"})
    plan["edges"].append({"from_key": "successor", "to_id": args.rejected_bead, "type": "supersedes"})
    plan["edges"].append({"from_id": rejected_meta["factory.refinery_bead"], "to_key": "successor", "type": "blocks"})
    dependent_beads = rescope_meta.get("factory.dependent_beads", [])
    if isinstance(dependent_beads, str):
        try:
            dependent_beads = json.loads(dependent_beads)
        except json.JSONDecodeError as exc:
            raise FactoryError("invalid_successor", "rescope dependent bead metadata is invalid") from exc
    if not isinstance(dependent_beads, list) or not all(isinstance(item, str) and item for item in dependent_beads):
        raise FactoryError("invalid_successor", "rescope dependent beads must be an array")
    for dependent in dependent_beads:
        plan["edges"].append({"from_id": dependent, "to_key": "successor", "type": "blocks"})
    existing_successor = existing.get(proposal["id"])
    if existing_successor:
        existing_meta = metadata(existing_successor)
        if (
            existing_meta.get("factory.supersedes_bead") != args.rejected_bead
            or existing_meta.get("factory.spec") != meta["factory.spec"]
        ):
            raise FactoryError("invalid_successor", "existing successor identity belongs to different work")
        successor = require_string(existing_successor.get("id"), "successor bead id")
    else:
        beads.update_metadata(args.rescope_bead, {
            "factory.status": "successor_preparing",
            "factory.successor_proposal": str(proposal_path),
            "factory.successor_proposal_digest": digest_file(proposal_path),
        })
        ids = beads.graph_create(plan)
        successor = ids.get("successor")
        if not successor:
            raise FactoryError("graph_apply_failed", "successor graph returned no bead")
    beads.update_metadata(args.rejected_bead, {"factory.successor_bead": successor})
    beads.update_metadata(args.rescope_bead, {
        "factory.status": "successor_admitted",
        "factory.successor_bead": successor,
        "factory.successor_proposal": str(proposal_path),
        "factory.successor_proposal_digest": digest_file(proposal_path),
    })
    if rescope.get("status") != "closed":
        beads.close(args.rescope_bead, f"Mayor successor admitted as {successor}")
    print(json.dumps({"rejected_bead": args.rejected_bead, "rescope_bead": args.rescope_bead, "successor_bead": successor}, sort_keys=True))
    return 0


def command_refinery_assemble(args: argparse.Namespace) -> int:
    beads = Beads(args.rig)
    refinery = beads.show(args.refinery_bead)
    meta = metadata(refinery)
    status = refinery.get("status")
    if meta.get("factory.kind") != "refinery" or status not in {"open", "in_progress"}:
        raise FactoryError("not_refinery", "only an open or Refiner-claimed Refinery bead may assemble")
    factory_phase = meta.get("factory.status")
    if factory_phase not in {"blocked", "ready", "assembling", "reassembly_required"}:
        raise FactoryError("invalid_transition", f"Refinery cannot assemble from phase {factory_phase!r}")
    unresolved = [
        str(item.get("id"))
        for item in refinery.get("dependencies", [])
        if isinstance(item, dict)
        and item.get("dependency_type", "blocks") in {"blocks", "waits-for", "conditional-blocks"}
        and item.get("status") != "closed"
    ]
    if unresolved:
        raise FactoryError("refinery_blocked", f"Refinery bead has unresolved dependencies: {sorted(unresolved)}")
    if status == "open" and factory_phase in {"blocked", "ready"} and args.refinery_bead not in beads.ready_ids():
        raise FactoryError("refinery_blocked", "Refinery bead is open but not Ready-visible")
    if status == "in_progress":
        expected_route = f"{args.rig}/{meta.get('factory.binding')}.refiner"
        session_name = meta.get("gc.session_name")
        if (
            meta.get("gc.routed_to") != expected_route
            or not isinstance(session_name, str)
            or not session_name
            or refinery.get("assignee") != session_name
        ):
            raise FactoryError("refinery_claim_invalid", "in-progress Refinery bead is not owned by its routed Refiner session")
    if factory_phase in {"blocked", "ready"}:
        beads.update_metadata(args.refinery_bead, {"factory.status": "ready"})
        meta["factory.status"] = "ready"
    records = beads.list_program(meta["factory.program_bead"])
    candidate_map: dict[str, dict[str, Any]] = {}
    subject_includes: set[str] = set()
    subject_excludes: set[str] = set()
    integration_scope: set[str] = set()
    for record in records:
        item = metadata(record)
        if item.get("factory.kind") != "experiment":
            continue
        verdict = item.get("factory.verdict")
        if verdict == "PASS":
            certificate_path = absolute_path(item.get("factory.admission_path"), "admission path", True)
            if digest_file(certificate_path) != item.get("factory.admission_digest"):
                raise FactoryError("admission_moved", f"admission certificate changed for {record['id']}")
            certificate = load_object(certificate_path, "admission certificate")
            if certificate.get("candidate_sha") != item.get("factory.candidate_sha") or certificate.get("verdict") != "PASS":
                raise FactoryError("invalid_admission", f"admission certificate is stale for {record['id']}")
            if certificate.get("candidate_base_sha") != item.get("factory.candidate_base_sha"):
                raise FactoryError("invalid_admission", f"admission certificate candidate_base_sha is stale for {record['id']}")
            for field in ("predecessor_beads", "predecessor_shas"):
                if certificate.get(field) != metadata_list(item, f"factory.{field}"):
                    raise FactoryError("invalid_admission", f"admission certificate {field} is stale for {record['id']}")
            candidate_map[str(record["id"])] = {
                "sha": item["factory.candidate_sha"],
                "predecessors": metadata_list(item, "factory.predecessor_beads"),
            }
            item_subject = json.loads(require_string(item.get("factory.subject"), "factory.subject"))
            subject_includes.update(item_subject.get("includes", []))
            subject_excludes.update(item_subject.get("excludes", []))
            integration_scope.update(json.loads(require_string(item.get("factory.write_scope"), "factory.write_scope")))
            integration_scope.update(json.loads(require_string(item.get("factory.generated_scope"), "factory.generated_scope")))
        elif verdict in {"FAIL", "NOT_PROVEN"} and not item.get("factory.successor_bead"):
            raise FactoryError("rescope_incomplete", f"rejected bead {record['id']} has no admitted successor")
    if not candidate_map:
        raise FactoryError("nothing_to_integrate", "Refinery found no PASSed experiment beads")
    candidates: list[tuple[str, str]] = []
    remaining = dict(candidate_map)
    admitted: set[str] = set()
    while remaining:
        ready = sorted(
            bead_id for bead_id, item in remaining.items()
            if set(item["predecessors"]).issubset(admitted)
        )
        if not ready:
            raise FactoryError("candidate_cycle", f"candidate predecessor graph is cyclic or incomplete: {sorted(remaining)}")
        for bead_id in ready:
            item = remaining.pop(bead_id)
            candidates.append((bead_id, item["sha"]))
            admitted.add(bead_id)
    if len(candidates) > args.max_candidates:
        raise FactoryError("train_too_large", f"train has {len(candidates)} candidates; max is {args.max_candidates}")
    repository = absolute_path(meta["factory.repository"], "factory.repository", True, True)
    preparing = factory_phase == "assembling"
    if preparing:
        base_sha = require_string(meta.get("factory.delivery_base_sha"), "factory.delivery_base_sha", SHA_RE)
        epoch = int(meta.get("factory.fence_epoch", "0"))
        branch = require_string(meta.get("factory.integration_branch"), "factory.integration_branch")
        worktree = absolute_path(meta.get("factory.integration_worktree"), "factory.integration_worktree")
        token = require_string(meta.get("factory.fence_token"), "factory.fence_token")
        if metadata_list(meta, "factory.candidate_beads") != [item[0] for item in candidates] or metadata_list(meta, "factory.candidate_shas") != [item[1] for item in candidates]:
            raise FactoryError("assembly_identity_mismatch", "preparing integration candidate train changed")
    else:
        base_sha = require_string(meta["factory.base_sha"], "factory.base_sha", SHA_RE)
        remote_probe = run_process(["git", "remote", "get-url", args.remote], cwd=repository, check=False)
        if remote_probe.returncode == 0:
            run_process(["git", "fetch", args.remote, meta["factory.base_branch"]], cwd=repository, timeout=300)
            base_sha = output(["git", "rev-parse", f"refs/remotes/{args.remote}/{meta['factory.base_branch']}"], cwd=repository)
            require_string(base_sha, "delivery base SHA", SHA_RE)
        epoch = int(meta.get("factory.fence_epoch", "0")) + 1
        branch = f"gc/integration/{meta['factory.program_id']}/1/{epoch}"
        root = absolute_path(args.worktree_root, "worktree-root", directory=True)
        worktree = (root / meta["factory.program_id"] / f"integration-1-{epoch}").resolve(strict=False)
        token = secrets.token_hex(24)
        beads.update_metadata(args.refinery_bead, {
            "factory.status": "assembling",
            "factory.fence_epoch": str(epoch),
            "factory.fence_token": token,
            "factory.integration_branch": branch,
            "factory.integration_worktree": str(worktree),
            "factory.delivery_remote": args.remote,
            "factory.delivery_base_sha": base_sha,
            "factory.candidate_beads": [item[0] for item in candidates],
            "factory.candidate_shas": [item[1] for item in candidates],
        })
    worktree.parent.mkdir(parents=True, exist_ok=True)
    branch_exists = run_process(
        ["git", "show-ref", "--verify", "--quiet", f"refs/heads/{branch}"],
        cwd=repository, check=False,
    ).returncode == 0
    if worktree.exists():
        if output(["git", "branch", "--show-current"], cwd=worktree) != branch:
            raise FactoryError("worktree_collision", "integration worktree is attached to a different branch")
    elif branch_exists:
        run_process(["git", "worktree", "add", str(worktree), branch], cwd=repository, timeout=120)
    else:
        run_process(["git", "worktree", "add", "-b", branch, str(worktree), base_sha], cwd=repository, timeout=120)
    if output(["git", "status", "--porcelain", "--untracked-files=no"], cwd=worktree):
        raise FactoryError("integration_dirty", "preparing integration worktree has tracked changes")
    evidence = worktree / ".gc" / "agentops-factory" / args.refinery_bead
    evidence.mkdir(parents=True, exist_ok=True)
    executor = load_executor_adapter()
    manifest_packet = {
        "packet_id": safe_identifier(args.refinery_bead, "integration", limit=120),
        "role": "implement",
        "subject": {"includes": sorted(subject_includes), "excludes": sorted(subject_excludes)},
        "write_scope": sorted(integration_scope),
    }
    baseline_path = evidence / "integration-baseline-manifest.json"
    if baseline_path.is_file():
        baseline = load_object(baseline_path, "integration baseline manifest")
        stored_baseline_digest = meta.get("factory.integration_baseline_digest")
        if stored_baseline_digest and digest_file(baseline_path) != stored_baseline_digest:
            raise FactoryError("manifest_mutated", "integration baseline manifest changed during assembly")
    else:
        if git_head(worktree) != base_sha:
            raise FactoryError("assembly_recovery_missing", "integration moved before its baseline manifest was persisted")
        baseline = executor.build_manifest(manifest_packet, {"workspace": worktree}, baseline_path)
    beads.update_metadata(args.refinery_bead, {
        "factory.integration_baseline_manifest": str(baseline_path),
        "factory.integration_baseline_digest": digest_file(baseline_path),
    })
    for _bead_id, candidate_sha in candidates:
        if not commit_patch_present(worktree, candidate_sha):
            run_process(["git", "cherry-pick", candidate_sha], cwd=worktree, timeout=120)
    integration_sha = git_head(worktree)
    subject_path = evidence / "integration-subject-manifest.json"
    subject_manifest = executor.build_manifest(manifest_packet, {"workspace": worktree}, subject_path, baseline_path)
    changed = executor.changed_paths(baseline, subject_manifest)
    scope_receipt = executor.make_scope_receipt(manifest_packet, changed)
    scope_path = evidence / "integration-scope-receipt.json"
    write_json_atomic(scope_path, scope_receipt)
    if scope_receipt.get("status") != "PASS":
        raise FactoryError("integration_scope_failed", f"integration scope is {scope_receipt.get('status')}")
    beads.update_metadata(args.refinery_bead, {
        "factory.status": "validation_required",
        "factory.fence_epoch": str(epoch),
        "factory.fence_token": token,
        "factory.integration_branch": branch,
        "factory.integration_worktree": str(worktree),
        "factory.integration_sha": integration_sha,
        "factory.delivery_remote": args.remote,
        "factory.delivery_base_sha": base_sha,
        "factory.candidate_beads": [item[0] for item in candidates],
        "factory.candidate_shas": [item[1] for item in candidates],
        "factory.integration_subject": manifest_packet["subject"],
        "factory.integration_scope": manifest_packet["write_scope"],
        "factory.integration_baseline_manifest": str(baseline_path),
        "factory.integration_baseline_digest": digest_file(baseline_path),
        "factory.integration_subject_manifest": str(subject_path),
        "factory.integration_scope_receipt": str(scope_path),
    })
    print(json.dumps({"refinery_bead": args.refinery_bead, "epoch": epoch, "fence_token": token, "branch": branch, "worktree": str(worktree), "integration_sha": integration_sha, "candidate_beads": [item[0] for item in candidates]}, sort_keys=True))
    return 0


def check_fence(meta: dict[str, Any], epoch: int, token: str, integration_sha: str) -> Path:
    if int(meta.get("factory.fence_epoch", "0")) != epoch or meta.get("factory.fence_token") != token:
        raise FactoryError("stale_fence", "Refinery epoch or token is stale")
    if meta.get("factory.integration_sha") != integration_sha:
        raise FactoryError("integration_moved", "integration SHA differs from fenced bead metadata")
    worktree = absolute_path(meta.get("factory.integration_worktree"), "integration worktree", True, True)
    if git_head(worktree) != integration_sha:
        raise FactoryError("integration_moved", "integration branch moved after fencing")
    return worktree


def command_refinery_validate(args: argparse.Namespace) -> int:
    beads = Beads(args.rig)
    record = beads.show(args.refinery_bead)
    meta = metadata(record)
    check_fence(meta, args.epoch, args.fence_token, args.integration_sha)
    if meta.get("factory.status") != "validation_required":
        raise FactoryError("invalid_transition", "Refinery is not awaiting integration validation")
    manifest = load_object(absolute_path(args.subject_manifest, "subject manifest", True), "subject manifest")
    subject_digest = require_string(manifest.get("canonical_manifest_digest"), "subject manifest digest", DIGEST_RE)
    verdict_path = absolute_path(args.verdict, "verdict", True)
    verdict = load_object(verdict_path, "integration verdict")
    author = f"refinery:{args.refinery_bead}:{args.epoch}"
    synthetic = {"factory.intent_digest": meta["factory.intent_digest"]}
    result, validator = validate_verdict(verdict, synthetic, subject_digest, author)
    fields = {
        "factory.integration_verdict": result,
        "factory.integration_verdict_digest": digest_file(verdict_path),
        "factory.integration_subject_digest": subject_digest,
        "factory.integration_validator_context_id": validator,
        "factory.status": "validated" if result == "PASS" else "integration_rejected",
    }
    beads.update_metadata(args.refinery_bead, fields)
    print(json.dumps({"refinery_bead": args.refinery_bead, "verdict": result, "status": fields["factory.status"]}, sort_keys=True))
    return 0


def command_refinery_run_validation(args: argparse.Namespace) -> int:
    beads = Beads(args.rig)
    record = beads.show(args.refinery_bead)
    meta = metadata(record)
    if meta.get("factory.kind") != "refinery" or meta.get("factory.status") != "validation_required":
        raise FactoryError("invalid_transition", "Refinery is not awaiting integration validation")
    epoch = int(meta.get("factory.fence_epoch", "0"))
    token = require_string(meta.get("factory.fence_token"), "factory.fence_token")
    integration_sha = require_string(meta.get("factory.integration_sha"), "factory.integration_sha", SHA_RE)
    worktree = check_fence(meta, epoch, token, integration_sha)
    branch = require_string(meta.get("factory.integration_branch"), "factory.integration_branch")
    binding = require_string(meta.get("factory.binding"), "factory.binding", ID_RE)
    candidate_rig, binding = register_integration_rig(worktree, args.refinery_bead, branch, binding)
    program = beads.show(require_string(meta.get("factory.program_bead"), "factory.program_bead"))
    program_meta = metadata(program)
    source_intent = absolute_path(program_meta.get("factory.intent_source"), "factory.intent_source", True)
    intent_digest = require_string(meta.get("factory.intent_digest"), "factory.intent_digest", DIGEST_RE)
    if digest_file(source_intent) != intent_digest:
        raise FactoryError("identity_mismatch", "program intent changed before integration validation")
    intent_source = worktree / ".gc" / "agentops-factory" / args.refinery_bead / "intent.md"
    intent_source.parent.mkdir(parents=True, exist_ok=True)
    intent_source.write_bytes(source_intent.read_bytes())
    if digest_file(intent_source) != intent_digest:
        raise FactoryError("identity_mismatch", "copied integration intent does not match the program digest")
    packet_id = safe_identifier(args.refinery_bead, "integration-validate", limit=120)
    evidence = worktree / ".gc" / "agentops" / packet_id
    packet_path = evidence / "packet.json"
    author = f"refinery:{args.refinery_bead}:{epoch}"
    packet = {
        "schema_version": "gc-execution-envelope.v1",
        "packet_id": packet_id,
        "role": "validate",
        "provider": args.provider,
        "intent_source": str(intent_source),
        "intent_digest": intent_digest,
        "workspace": str(worktree),
        "subject": metadata_object(meta, "factory.integration_subject"),
        "write_scope": [],
        "evidence_dir": str(evidence),
        "result_path": str(evidence / "agent-response.json"),
        "baseline_manifest": require_string(meta.get("factory.integration_baseline_manifest"), "integration baseline manifest"),
        "subject_manifest": require_string(meta.get("factory.integration_subject_manifest"), "integration subject manifest"),
        "scope_receipt": require_string(meta.get("factory.integration_scope_receipt"), "integration scope receipt"),
        "author_context_id": author,
    }
    write_json_atomic(packet_path, packet)
    validation = run_executor_packet(packet_path, candidate_rig, binding, args.timeout)
    artifacts = validation.get("agent_response", {}).get("artifacts")
    if not isinstance(artifacts, list) or len(artifacts) != 1 or not isinstance(artifacts[0], dict):
        raise FactoryError("verdict_missing", "integration Validator returned no exact verdict artifact")
    verdict_path = absolute_path(artifacts[0].get("path"), "integration verdict", True)
    transition = argparse.Namespace(
        rig=args.rig, refinery_bead=args.refinery_bead, epoch=epoch,
        fence_token=token, integration_sha=integration_sha,
        subject_manifest=packet["subject_manifest"], verdict=str(verdict_path),
    )
    with open(os.devnull, "w", encoding="utf-8") as sink, contextlib.redirect_stdout(sink):
        command_refinery_validate(transition)
    beads.update_metadata(args.refinery_bead, {
        "factory.integration_rig": candidate_rig,
        "factory.executor_binding": binding,
        "factory.integration_validation_packet": str(packet_path),
        "factory.integration_validator_context_id": validation["transport"]["session_context_id"],
    })
    result = {
        "refinery_bead": args.refinery_bead,
        "integration_sha": integration_sha,
        "verdict": load_object(verdict_path, "integration verdict").get("verdict"),
        "validator_context_id": validation["transport"]["session_context_id"],
        "verdict_path": str(verdict_path),
    }
    print(json.dumps(result, sort_keys=True))
    return 0


def gh_binary() -> str:
    found = shutil.which("gh")
    if not found:
        raise FactoryError("gh_missing", "GitHub CLI is unavailable")
    return found


def gh_json(worktree: Path, *args: str) -> dict[str, Any]:
    value = parse_json_output(output([gh_binary(), *args], cwd=worktree), "gh")
    if not isinstance(value, dict):
        raise FactoryError("gh_invalid", f"GitHub CLI returned non-object JSON: {value!r}")
    return value


def delivery_record(meta: dict[str, Any], status: str, pr: dict[str, Any] | None,
                    landed_sha: str | None) -> dict[str, Any]:
    validation = None
    if meta.get("factory.integration_verdict"):
        validation = {
            "verdict": meta.get("factory.integration_verdict"),
            "verdict_digest": meta.get("factory.integration_verdict_digest"),
            "subject_manifest_digest": meta.get("factory.integration_subject_digest"),
            "validator_context_id": meta.get("factory.integration_validator_context_id"),
        }
    return {
        "schema_version": "delivery-record.v1",
        "program_id": meta["factory.program_id"],
        "wave": 1,
        "epoch": int(meta["factory.fence_epoch"]),
        "fence_token": meta["factory.fence_token"],
        "base_branch": meta["factory.base_branch"],
        "expected_base_sha": meta["factory.delivery_base_sha"],
        "candidate_shas": metadata_list(meta, "factory.candidate_shas"),
        "integration_branch": meta["factory.integration_branch"],
        "integration_sha": meta["factory.integration_sha"],
        "integration_validation": validation,
        "pr": pr,
        "landed_sha": landed_sha,
        "status": status,
    }


def persist_delivery_record(beads: Beads, refinery_bead: str, meta: dict[str, Any],
                            status: str, pr: dict[str, Any] | None,
                            landed_sha: str | None) -> Path:
    worktree = absolute_path(meta.get("factory.integration_worktree"), "integration worktree", True, True)
    path = worktree / ".gc" / "agentops-factory" / refinery_bead / "delivery.json"
    write_json_atomic(path, delivery_record(meta, status, pr, landed_sha))
    beads.update_metadata(refinery_bead, {
        "factory.delivery_record": str(path),
        "factory.delivery_record_digest": digest_file(path),
    })
    return path


def command_refinery_publish(args: argparse.Namespace) -> int:
    beads = Beads(args.rig)
    record = beads.show(args.refinery_bead)
    meta = metadata(record)
    if meta.get("factory.kind") != "refinery" or meta.get("factory.status") != "validated":
        raise FactoryError("invalid_transition", "only a validated Refinery bead may publish")
    if meta.get("factory.integration_verdict") != "PASS":
        raise FactoryError("integration_rejected", "only a PASSed integration may publish")
    epoch = int(meta["factory.fence_epoch"])
    token = require_string(meta.get("factory.fence_token"), "factory.fence_token")
    integration_sha = require_string(meta.get("factory.integration_sha"), "factory.integration_sha", SHA_RE)
    worktree = check_fence(meta, epoch, token, integration_sha)
    if output(["git", "status", "--porcelain", "--untracked-files=no"], cwd=worktree):
        raise FactoryError("integration_dirty", "integration worktree has tracked changes")
    remote = args.remote or meta.get("factory.delivery_remote", "origin")
    base_branch = require_string(meta.get("factory.base_branch"), "factory.base_branch")
    run_process(["git", "fetch", remote, base_branch], cwd=worktree, timeout=300)
    remote_base = output(["git", "rev-parse", f"refs/remotes/{remote}/{base_branch}"], cwd=worktree)
    if remote_base != meta.get("factory.delivery_base_sha"):
        beads.update_metadata(args.refinery_bead, {
            "factory.status": "reassembly_required",
            "factory.reassembly_reason": f"base moved to {remote_base}",
        })
        raise FactoryError("base_moved", "base branch moved after integration validation; reassemble and revalidate")
    branch = require_string(meta.get("factory.integration_branch"), "factory.integration_branch")
    run_process(["git", "push", "-u", remote, f"HEAD:refs/heads/{branch}"], cwd=worktree, timeout=600)
    existing = run_process(
        [gh_binary(), "pr", "view", branch, "--json", "url,number,state,isDraft,headRefOid,headRefName,baseRefName"],
        cwd=worktree, check=False, timeout=60,
    )
    if existing.returncode == 0:
        pr = parse_json_output(existing.stdout, "gh pr view")
    else:
        title = args.title or f"factory: {meta['factory.program_id']}"
        body = "\n".join([
            f"Gas City factory program bead: `{meta['factory.program_bead']}`",
            f"Refinery bead: `{args.refinery_bead}`",
            f"Integration SHA: `{integration_sha}`",
            f"Candidate beads: {', '.join(metadata_list(meta, 'factory.candidate_beads'))}",
            "",
            "Every candidate and this integration head received a fresh AgentOps PASS verdict.",
        ])
        command = [
            gh_binary(), "pr", "create", "--base", base_branch, "--head", branch,
            "--title", title, "--body", body,
        ]
        if args.draft:
            command.append("--draft")
        run_process(command, cwd=worktree, timeout=120)
        pr = gh_json(worktree, "pr", "view", branch, "--json", "url,number,state,isDraft,headRefOid,headRefName,baseRefName")
    if not isinstance(pr, dict) or pr.get("headRefOid") != integration_sha or pr.get("baseRefName") != base_branch:
        raise FactoryError("pr_binding_mismatch", f"PR does not bind the fenced integration head: {pr!r}")
    pr_summary = {
        "url": pr.get("url"), "number": pr.get("number"), "state": pr.get("state"),
        "draft": pr.get("isDraft"), "head_sha": pr.get("headRefOid"),
        "head_branch": pr.get("headRefName"), "base_branch": pr.get("baseRefName"),
    }
    beads.update_metadata(args.refinery_bead, {
        "factory.status": "published",
        "factory.pr_url": require_string(pr.get("url"), "PR URL"),
        "factory.pr_number": str(pr.get("number")),
        "factory.pr_head_sha": integration_sha,
    })
    refreshed = metadata(beads.show(args.refinery_bead))
    delivery_path = persist_delivery_record(beads, args.refinery_bead, refreshed, "published", pr_summary, None)
    print(json.dumps({"refinery_bead": args.refinery_bead, "pr": pr_summary, "delivery_record": str(delivery_path)}, sort_keys=True))
    return 0


def reconcile_landed_delivery(beads: Beads, refinery_bead: str) -> dict[str, Any]:
    refinery = beads.show(refinery_bead)
    meta = metadata(refinery)
    if meta.get("factory.status") != "landed":
        raise FactoryError("invalid_transition", "delivery reconciliation requires landed Refinery metadata")
    landed_sha = require_string(meta.get("factory.landed_sha"), "factory.landed_sha", SHA_RE)
    delivery_path = absolute_path(meta.get("factory.delivery_record"), "factory.delivery_record", True)
    if digest_file(delivery_path) != meta.get("factory.delivery_record_digest"):
        raise FactoryError("identity_mismatch", "landed delivery record changed")
    program_bead = require_string(meta.get("factory.program_bead"), "factory.program_bead")
    program = beads.show(program_bead)
    beads.update_metadata(program_bead, {
        "factory.status": "landed",
        "factory.landed_sha": landed_sha,
        "factory.pr_url": require_string(meta.get("factory.pr_url"), "factory.pr_url"),
        "factory.delivery_record": str(delivery_path),
    })
    if refinery.get("status") != "closed":
        beads.close(refinery_bead, f"Factory delivery landed at {landed_sha}")
    if program.get("status") != "closed":
        beads.close(program_bead, f"Factory program landed through {meta['factory.pr_url']} at {landed_sha}")
    return {
        "refinery_bead": refinery_bead,
        "program_bead": program_bead,
        "pr_url": meta["factory.pr_url"],
        "landed_sha": landed_sha,
        "delivery_record": str(delivery_path),
    }


def command_refinery_land(args: argparse.Namespace) -> int:
    beads = Beads(args.rig)
    record = beads.show(args.refinery_bead)
    meta = metadata(record)
    if meta.get("factory.kind") != "refinery" or meta.get("factory.status") != "published":
        raise FactoryError("invalid_transition", "only a published Refinery bead may land")
    integration_sha = require_string(meta.get("factory.integration_sha"), "factory.integration_sha", SHA_RE)
    worktree = check_fence(
        meta, int(meta["factory.fence_epoch"]),
        require_string(meta.get("factory.fence_token"), "factory.fence_token"), integration_sha,
    )
    pr_url = require_string(meta.get("factory.pr_url"), "factory.pr_url")
    pr = gh_json(
        worktree, "pr", "view", pr_url, "--json",
        "url,number,state,isDraft,headRefOid,headRefName,baseRefName,statusCheckRollup",
    )
    if pr.get("headRefOid") != integration_sha or pr.get("baseRefName") != meta.get("factory.base_branch"):
        raise FactoryError("pr_binding_mismatch", "PR head or base moved after publication")
    if pr.get("state") != "MERGED":
        if pr.get("isDraft"):
            run_process([gh_binary(), "pr", "ready", pr_url], cwd=worktree, timeout=60)
        checks = pr.get("statusCheckRollup")
        if isinstance(checks, list) and checks:
            watched = run_process(
                [gh_binary(), "pr", "checks", pr_url, "--watch", "--fail-fast", "--interval", "10"],
                cwd=worktree, timeout=args.timeout, check=False,
            )
            if watched.returncode != 0:
                raise FactoryError(
                    "checks_failed",
                    watched.stderr.strip() or watched.stdout.strip() or "PR checks failed",
                )
        method_flag = {"merge": "--merge", "squash": "--squash", "rebase": "--rebase"}[args.merge_method]
        merged = run_process(
            [gh_binary(), "pr", "merge", pr_url, method_flag, "--delete-branch"],
            cwd=worktree, timeout=300, check=False,
        )
        if merged.returncode != 0:
            auto = run_process(
                [gh_binary(), "pr", "merge", pr_url, method_flag, "--auto", "--delete-branch"],
                cwd=worktree, timeout=120, check=False,
            )
            if auto.returncode != 0:
                detail = auto.stderr.strip() or auto.stdout.strip() or merged.stderr.strip() or merged.stdout.strip()
                raise FactoryError("merge_failed", detail)
    deadline = time.monotonic() + args.timeout
    final: dict[str, Any] = {}
    while time.monotonic() < deadline:
        final = gh_json(worktree, "pr", "view", pr_url, "--json", "url,number,state,mergedAt,mergeCommit,headRefOid,baseRefName")
        if final.get("state") == "MERGED":
            break
        time.sleep(10)
    else:
        raise FactoryError("merge_timeout", f"PR did not reach MERGED before timeout: {pr_url}")
    merge_commit = final.get("mergeCommit")
    landed_sha = merge_commit.get("oid") if isinstance(merge_commit, dict) else None
    require_string(landed_sha, "landed SHA", SHA_RE)
    pr_summary = {
        "url": final.get("url"), "number": final.get("number"), "state": final.get("state"),
        "head_sha": final.get("headRefOid"), "base_branch": final.get("baseRefName"),
        "merged_at": final.get("mergedAt"), "merge_commit": landed_sha,
    }
    delivery_path = persist_delivery_record(beads, args.refinery_bead, meta, "landed", pr_summary, landed_sha)
    beads.update_metadata(args.refinery_bead, {
        "factory.status": "landed",
        "factory.landed_sha": landed_sha,
        "factory.merged_at": str(final.get("mergedAt", "")),
        "factory.delivery_record": str(delivery_path),
        "factory.delivery_record_digest": digest_file(delivery_path),
    })
    result = reconcile_landed_delivery(beads, args.refinery_bead)
    print(json.dumps(result, sort_keys=True))
    return 0


def command_refinery_deliver(args: argparse.Namespace) -> int:
    beads = Beads(args.rig)
    phase = metadata(beads.show(args.refinery_bead)).get("factory.status")
    with open(os.devnull, "w", encoding="utf-8") as sink, contextlib.redirect_stdout(sink):
        if phase in {"blocked", "ready", "assembling", "reassembly_required"}:
            command_refinery_assemble(argparse.Namespace(
                rig=args.rig, refinery_bead=args.refinery_bead,
                worktree_root=args.worktree_root, max_candidates=args.max_candidates,
                remote=args.remote,
            ))
            phase = metadata(beads.show(args.refinery_bead)).get("factory.status")
        if phase == "validation_required":
            command_refinery_run_validation(argparse.Namespace(
                rig=args.rig, refinery_bead=args.refinery_bead,
                provider=args.provider, timeout=args.timeout,
            ))
            phase = metadata(beads.show(args.refinery_bead)).get("factory.status")
        if phase == "validated":
            command_refinery_publish(argparse.Namespace(
                rig=args.rig, refinery_bead=args.refinery_bead, remote=args.remote,
                title=args.title, draft=args.draft,
            ))
            phase = metadata(beads.show(args.refinery_bead)).get("factory.status")
        if phase == "published":
            command_refinery_land(argparse.Namespace(
                rig=args.rig, refinery_bead=args.refinery_bead,
                merge_method=args.merge_method, timeout=args.timeout,
            ))
            phase = metadata(beads.show(args.refinery_bead)).get("factory.status")
    if phase == "integration_rejected":
        rejected = metadata(beads.show(args.refinery_bead))
        raise FactoryError("integration_rejected", f"integration verdict is {rejected.get('factory.integration_verdict')}")
    if phase != "landed":
        raise FactoryError("invalid_transition", f"Refinery delivery cannot resume from phase {phase!r}")
    result = reconcile_landed_delivery(Beads(args.rig), args.refinery_bead)
    result["status"] = "landed"
    print(json.dumps(result, sort_keys=True))
    return 0


def command_doctor() -> int:
    problems: list[str] = []
    pack = (PACK_ROOT / "pack.toml").read_text(encoding="utf-8")
    if "[imports.executor]" not in pack:
        problems.append("factory pack must import the thin executor")
    agents = {path.name for path in (PACK_ROOT / "agents").iterdir() if path.is_dir()}
    if agents != {"mayor", "plan-reviewer", "plan-reviewer-claude", "refiner"}:
        problems.append(f"unexpected semantic roles: {sorted(agents)}")
    forbidden_state = [
        str(path.relative_to(PACK_ROOT))
        for path in PACK_ROOT.rglob("*")
        if path.is_file() and path.name in {"factory-program-state.json", "state.json"}
    ]
    if forbidden_state:
        problems.append(f"factory must not create a parallel JSON lifecycle state machine: {forbidden_state}")
    source = Path(__file__).read_text(encoding="utf-8")
    runtime_source = source.split("\ndef command_doctor", 1)[0]
    if '"factory.kind": "experiment"' not in source or "graph_create" not in source:
        problems.append("factory experiment units must materialize through bd create --graph")
    if "factory.rescope_bead" not in source or "factory.successor_bead" not in source:
        problems.append("rejection ratchet is not attached to beads")
    if "enforce_rig_agent_policy" not in runtime_source or '"agent", "suspend"' in runtime_source:
        problems.append("dynamic worktree rigs must use verified rig-scoped patches, not ignored agent suspend calls")
    if '"--include", str(PACK_ROOT)' in runtime_source:
        problems.append("dynamic worktree rigs must use the binding admitted on the bead, not inject a duplicate pack binding")
    for schema in SCHEMA_ROOT.glob("*.json"):
        try:
            load_object(schema, schema.name)
        except FactoryError as exc:
            problems.append(str(exc))
    if problems:
        print(f"agentops-factory doctor found {len(problems)} problem(s)")
        for problem in problems:
            print(problem)
        return 2
    print("agentops-factory is bead-native: roles/methods in the pack, work and lifecycle in the bead graph")
    return 0


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    sub = root.add_subparsers(dest="command", required=True)
    plan = sub.add_parser("plan")
    plan.add_argument("--intent", required=True)
    plan.add_argument("--repository", required=True)
    plan.add_argument("--program-id", required=True)
    plan.add_argument("--base-branch", default="main")
    plan.add_argument("--rig", required=True)
    plan.add_argument("--binding", default="factory")
    plan.add_argument("--reviewer-provider", choices=sorted(PROVIDERS), default="claude")
    plan.add_argument("--evidence-dir")
    plan.add_argument("--timeout", type=float, default=1800)
    plan.add_argument("--result")
    admit = sub.add_parser("admit")
    admit.add_argument("--intent", required=True)
    admit.add_argument("--graph", required=True)
    admit.add_argument("--review", required=True)
    admit.add_argument("--mayor-context", required=True)
    admit.add_argument("--rig", required=True)
    admit.add_argument("--binding", default="factory")
    admit.add_argument("--result")
    lease = sub.add_parser("lease")
    lease.add_argument("--rig", required=True)
    lease.add_argument("--bead", required=True)
    lease.add_argument("--worktree-root", required=True)
    execute = sub.add_parser("execute")
    execute.add_argument("--rig", required=True)
    execute.add_argument("--program-bead", required=True)
    execute.add_argument("--bead", action="append")
    execute.add_argument("--worktree-root")
    execute.add_argument("--max-parallel", type=int, default=4)
    execute.add_argument("--max-attempts", type=int, default=3)
    execute.add_argument("--timeout", type=float, default=1800)
    execute.add_argument("--result")
    resume = sub.add_parser("resume-experiment")
    resume.add_argument("--rig", required=True)
    resume.add_argument("--bead", required=True)
    resume.add_argument("--max-attempts", type=int, default=3)
    resume.add_argument("--timeout", type=float, default=1800)
    resume.add_argument("--result")
    verdict = sub.add_parser("record-verdict")
    verdict.add_argument("--rig", required=True)
    verdict.add_argument("--bead", required=True)
    verdict.add_argument("--lease-token", required=True)
    verdict.add_argument("--fence-epoch", type=int, required=True)
    verdict.add_argument("--candidate-sha", required=True)
    verdict.add_argument("--subject-manifest", required=True)
    verdict.add_argument("--author-context", required=True)
    verdict.add_argument("--verdict", required=True)
    successor = sub.add_parser("successor")
    successor.add_argument("--rig", required=True)
    successor.add_argument("--rejected-bead", required=True)
    successor.add_argument("--rescope-bead", required=True)
    successor.add_argument("--proposal", required=True)
    rescope = sub.add_parser("rescope")
    rescope.add_argument("--rig", required=True)
    rescope.add_argument("--rescope-bead", required=True)
    rescope.add_argument("--timeout", type=float, default=1800)
    refinery = sub.add_parser("refinery")
    refinery_sub = refinery.add_subparsers(dest="refinery_command", required=True)
    assemble = refinery_sub.add_parser("assemble")
    assemble.add_argument("--rig", required=True)
    assemble.add_argument("--refinery-bead", required=True)
    assemble.add_argument("--worktree-root", required=True)
    assemble.add_argument("--max-candidates", type=int, default=5)
    assemble.add_argument("--remote", default="origin")
    validate = refinery_sub.add_parser("validate")
    validate.add_argument("--rig", required=True)
    validate.add_argument("--refinery-bead", required=True)
    validate.add_argument("--epoch", type=int, required=True)
    validate.add_argument("--fence-token", required=True)
    validate.add_argument("--integration-sha", required=True)
    validate.add_argument("--subject-manifest", required=True)
    validate.add_argument("--verdict", required=True)
    run_validation = refinery_sub.add_parser("run-validation")
    run_validation.add_argument("--rig", required=True)
    run_validation.add_argument("--refinery-bead", required=True)
    run_validation.add_argument("--provider", choices=sorted(PROVIDERS), default="claude")
    run_validation.add_argument("--timeout", type=float, default=1800)
    publish = refinery_sub.add_parser("publish")
    publish.add_argument("--rig", required=True)
    publish.add_argument("--refinery-bead", required=True)
    publish.add_argument("--remote", default="origin")
    publish.add_argument("--title")
    publish.add_argument("--draft", action="store_true")
    land = refinery_sub.add_parser("land")
    land.add_argument("--rig", required=True)
    land.add_argument("--refinery-bead", required=True)
    land.add_argument("--merge-method", choices=("merge", "squash", "rebase"), default="squash")
    land.add_argument("--timeout", type=float, default=3600)
    deliver = refinery_sub.add_parser("deliver")
    deliver.add_argument("--rig", required=True)
    deliver.add_argument("--refinery-bead", required=True)
    deliver.add_argument("--worktree-root", required=True)
    deliver.add_argument("--max-candidates", type=int, default=8)
    deliver.add_argument("--provider", choices=sorted(PROVIDERS), default="claude")
    deliver.add_argument("--remote", default="origin")
    deliver.add_argument("--title")
    deliver.add_argument("--draft", action="store_true")
    deliver.add_argument("--merge-method", choices=("merge", "squash", "rebase"), default="squash")
    deliver.add_argument("--timeout", type=float, default=3600)
    inspect_role = sub.add_parser("inspect-role")
    inspect_role.add_argument("--request", required=True)
    emit_role = sub.add_parser("emit-role")
    emit_role.add_argument("--request", required=True)
    emit_role.add_argument("--bead", required=True)
    emit_role.add_argument("--artifact", required=True)
    sub.add_parser("doctor")
    return root


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    try:
        if args.command == "plan":
            return command_plan(args)
        if args.command == "admit":
            return command_admit(args)
        if args.command == "lease":
            return command_lease(args)
        if args.command == "execute":
            if args.max_parallel < 1 or args.max_attempts < 1 or args.timeout <= 0:
                raise FactoryError("invalid_argument", "--max-parallel, --max-attempts, and --timeout must be positive")
            return command_execute(args)
        if args.command == "resume-experiment":
            if args.max_attempts < 1 or args.timeout <= 0:
                raise FactoryError("invalid_argument", "--max-attempts and --timeout must be positive")
            return command_resume_experiment(args)
        if args.command == "record-verdict":
            return command_record_verdict(args)
        if args.command == "successor":
            return command_successor(args)
        if args.command == "rescope":
            if args.timeout <= 0:
                raise FactoryError("invalid_argument", "--timeout must be positive")
            return command_rescope(args)
        if args.command == "inspect-role":
            return command_inspect_role(args)
        if args.command == "emit-role":
            return command_emit_role(args)
        if args.command == "doctor":
            return command_doctor()
        if args.command == "refinery":
            if args.refinery_command == "assemble":
                return command_refinery_assemble(args)
            if args.refinery_command == "validate":
                return command_refinery_validate(args)
            if args.refinery_command == "run-validation":
                if args.timeout <= 0:
                    raise FactoryError("invalid_argument", "--timeout must be positive")
                return command_refinery_run_validation(args)
            if args.refinery_command == "publish":
                return command_refinery_publish(args)
            if args.refinery_command == "land":
                if args.timeout <= 0:
                    raise FactoryError("invalid_argument", "--timeout must be positive")
                return command_refinery_land(args)
            if args.refinery_command == "deliver":
                if args.timeout <= 0 or args.max_candidates < 1:
                    raise FactoryError("invalid_argument", "--timeout and --max-candidates must be positive")
                return command_refinery_deliver(args)
    except FactoryError as exc:
        print(json.dumps({"ok": False, "error": {"code": exc.code, "message": str(exc)}}, sort_keys=True))
        return 2
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
