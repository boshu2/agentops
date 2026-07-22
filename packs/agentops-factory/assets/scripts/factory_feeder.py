#!/usr/bin/env python3
"""Bounded GC33 admission/check bridge.

This program deliberately has no watch mode, database, queue, or retry loop.
The controller invokes its check mode for one checked Formula v2 step; an
operator or the checked plan invokes ``admit`` once to turn a byte-exact
program graph into one Beads graph.  The immutable receipt is the sole record
owned here; Beads remains the workflow and delivery source of truth.
"""
from __future__ import annotations

import argparse
from datetime import datetime, timedelta, timezone
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
from types import SimpleNamespace
from typing import Any

from jsonschema import Draft202012Validator, FormatChecker


SCRIPT_ROOT = Path(__file__).resolve().parent
SCHEMA_CANDIDATES = (
    SCRIPT_ROOT.parent / "schemas",
    SCRIPT_ROOT.parent / "schemas" / "agentops-factory",
)
SCHEMA_ROOT = next(
    (path for path in SCHEMA_CANDIDATES if (path / "program-graph.v2.schema.json").is_file()),
    SCHEMA_CANDIDATES[0],
)
SAFE_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$")
SAFE_RIG_ID = re.compile(r"^[A-Za-z0-9_-]+$")


class FeederError(ValueError):
    pass


class PhaseFailure(FeederError):
    def __init__(self, code: str, message: str):
        super().__init__(message)
        self.code = code


def sealed_rig_id(value: object, label: str = "rig_id") -> str:
    if not isinstance(value, str) or SAFE_RIG_ID.fullmatch(value) is None:
        raise FeederError(f"{label} has an unsafe identity")
    return value


def formula_routes(rig_id: object) -> dict[str, str]:
    """Derive Formula-cook targets from the digest-bound native rig only."""
    rig = sealed_rig_id(rig_id)
    return {
        "mayor": "agentops.mayor",
        "plan": f"{rig}/agentops.plan-reviewer",
        "implementer": f"{rig}/agentops.implementer",
        "implementer_claude": f"{rig}/agentops.implementer-claude",
        "validator": f"{rig}/agentops.validator",
    }


def validate_build_context(context: dict[str, Any]) -> None:
    validate_schema(context, "factory-build-context.v1.schema.json", "build context")
    if context.get("routes") != formula_routes(context.get("rig_id")):
        raise FeederError("build context routes differ from its sealed rig_id")


def canonical(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=True, separators=(",", ":"), sort_keys=True).encode() + b"\n"


def digest(raw: bytes) -> str:
    return hashlib.sha256(raw).hexdigest()


def factory_evidence_root(root: Path) -> Path:
    return root / ".gc" / "agentops" / "factory" / "evidence"


def read_exact(path: Path) -> tuple[dict[str, Any], bytes]:
    raw = path.read_bytes()
    try:
        value = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise FeederError(f"invalid JSON: {path}") from exc
    if not isinstance(value, dict) or canonical(value) != raw:
        raise FeederError(f"non-canonical object: {path}")
    return value, raw


def require_fields(value: dict[str, Any], fields: set[str], label: str) -> None:
    if set(value) != fields:
        raise FeederError(f"{label} has unknown, missing, or conflicting fields")


def validate_schema(value: dict[str, Any], name: str, label: str) -> None:
    schema_path = SCHEMA_ROOT / name
    if not schema_path.is_file() or schema_path.is_symlink():
        raise FeederError(f"trusted schema is missing: {schema_path}")
    try:
        schema = json.loads(schema_path.read_bytes())
    except (OSError, json.JSONDecodeError) as exc:
        raise FeederError(f"trusted schema is invalid: {schema_path}") from exc
    try:
        Draft202012Validator(schema, format_checker=FormatChecker()).validate(value)
    except Exception as exc:
        raise FeederError(f"{label} does not satisfy {name}: {exc}") from exc


def lower_digest(value: object, label: str) -> str:
    if not isinstance(value, str) or len(value) != 64 or any(c not in "0123456789abcdef" for c in value):
        raise FeederError(f"{label} is not a lowercase sha256")
    return value


def atomic_write(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    raw = canonical(value)
    if path.exists():
        existing, existing_raw = read_exact(path)
        if existing != value or existing_raw != raw:
            raise FeederError(f"immutable receipt conflicts: {path}")
        return
    handle, temporary = tempfile.mkstemp(prefix=".factory-feeder.", dir=path.parent)
    try:
        with os.fdopen(handle, "wb") as out:
            out.write(raw)
            out.flush()
            os.fsync(out.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def atomic_bytes(path: Path, raw: bytes, label: str) -> None:
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    if path.exists():
        if path.is_symlink() or path.read_bytes() != raw:
            raise FeederError(f"immutable {label} conflicts: {path}")
        return
    handle, temporary = tempfile.mkstemp(prefix=".factory-feeder.", dir=path.parent)
    try:
        os.fchmod(handle, 0o600)
        with os.fdopen(handle, "wb") as out:
            out.write(raw)
            out.flush()
            os.fsync(out.fileno())
        try:
            os.link(temporary, path)
        except FileExistsError:
            if path.is_symlink() or path.read_bytes() != raw:
                raise FeederError(f"racing immutable {label} conflicts: {path}")
    finally:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass


def parse_json(raw: bytes, label: str) -> Any:
    try:
        return json.loads(raw)
    except json.JSONDecodeError as exc:
        raise FeederError(f"{label} did not return JSON") from exc


def all_beads(bd_bin: str, root: Path) -> list[dict[str, Any]]:
    value = parse_json(run_checked([bd_bin, "list", "--all", "--include-infra", "--include-gates", "--limit", "0", "--json"], root), "bd list")
    if not isinstance(value, list) or not all(isinstance(item, dict) for item in value):
        raise FeederError("bd list returned a non-object array")
    return value


def workflow_rows(bd_bin: str, root: Path, root_id: str, formula: str,
                  persisted_to_canonical: dict[str, str]) -> tuple[dict[str, str], dict[str, dict[str, Any]]]:
    rows = all_beads(bd_bin, root)
    expected_refs = set(persisted_to_canonical.values())
    if (persisted_to_canonical.get(formula) != formula
            or len(expected_refs) != len(persisted_to_canonical)):
        raise FeederError("workflow reference map is not an exact one-to-one stable contract")
    matches: dict[str, list[dict[str, Any]]] = {reference: [] for reference in expected_refs}
    selected: list[dict[str, Any]] = []
    for row in rows:
        metadata = row.get("metadata")
        if row.get("id") != root_id and (not isinstance(metadata, dict) or metadata.get("gc.root_bead_id") != root_id):
            continue
        selected.append(row)
    identifiers = [row.get("id") for row in selected]
    if (not selected or any(not isinstance(identifier, str) or not identifier for identifier in identifiers)
            or len(set(identifiers)) != len(identifiers)):
        raise FeederError(f"workflow {root_id} has missing or duplicate row ids")
    roots = [row for row in selected if row["id"] == root_id]
    if len(roots) != 1:
        raise FeederError(f"workflow {root_id} has no unique root row")
    root_metadata = roots[0].get("metadata")
    if (not isinstance(root_metadata, dict) or "gc.step_ref" in root_metadata
            or "gc.root_bead_id" in root_metadata
            or root_metadata.get("gc.kind") != "workflow"
            or root_metadata.get("gc.formula_contract") != "graph.v2"):
        raise FeederError(f"workflow {root_id} root is not an exact graph.v2 workflow")
    matches[formula].append(roots[0])
    for row in selected:
        if row["id"] == root_id:
            continue
        metadata = row.get("metadata")
        if not isinstance(metadata, dict) or metadata.get("gc.root_bead_id") != root_id:
            raise FeederError(f"workflow {root_id} nonroot lacks exact root identity")
        persisted = metadata.get("gc.step_ref")
        if not isinstance(persisted, str) or persisted not in persisted_to_canonical:
            raise FeederError(f"workflow {root_id} contains unknown persisted step_ref {persisted!r}")
        canonical_ref = persisted_to_canonical[persisted]
        if canonical_ref == formula:
            raise FeederError(f"workflow {root_id} nonroot claims the root reference")
        matches[canonical_ref].append(row)
    if any(len(items) != 1 for items in matches.values()):
        counts = {key: len(items) for key, items in matches.items()}
        raise FeederError(f"workflow {root_id} is partial or duplicate: {counts}")
    records = {key: items[0] for key, items in matches.items()}
    if formula == "agentops-build":
        roles = ("mayor", "plan")
    elif formula == "agentops-experiment":
        roles = ("implement", "validate")
    else:
        raise FeederError(f"unsupported workflow formula {formula!r}")
    for role in roles:
        control = records[f"{formula}.{role}"]
        spec = records[f"{formula}.{role}.spec"]
        attempt = records[f"{formula}.{role}.iteration.1"]
        control_meta = control["metadata"]
        spec_meta = spec["metadata"]
        attempt_meta = attempt["metadata"]
        if control_meta.get("gc.kind") != "ralph" or control_meta.get("gc.step_id") != role:
            raise FeederError(f"workflow {root_id} {role} control metadata differs from stable Ralph contract")
        if (spec_meta.get("gc.kind") != "spec" or spec_meta.get("gc.spec_for") != role
                or spec_meta.get("gc.spec_for_ref") != role):
            raise FeederError(f"workflow {root_id} {role} spec metadata differs from stable source-spec contract")
        if (attempt_meta.get("gc.step_id") != role or attempt_meta.get("gc.ralph_step_id") != role
                or attempt_meta.get("gc.attempt") != "1" or attempt_meta.get("gc.logical_bead_id") != control["id"]):
            raise FeederError(f"workflow {root_id} {role} attempt metadata differs from stable Ralph contract")
    return {key: str(row["id"]) for key, row in records.items()}, records


BUILD_STEP_REFS = {
    "agentops-build", "agentops-build.admission", "agentops-build.mayor",
    "agentops-build.mayor.spec", "agentops-build.mayor.iteration.1",
    "agentops-build.plan", "agentops-build.plan.spec",
    "agentops-build.plan.iteration.1", "agentops-build.workflow-finalize",
}
EXPERIMENT_STEP_REFS = {
    "agentops-experiment", "agentops-experiment.admission", "agentops-experiment.implement",
    "agentops-experiment.implement.spec", "agentops-experiment.implement.iteration.1",
    "agentops-experiment.validate", "agentops-experiment.validate.spec",
    "agentops-experiment.validate.iteration.1", "agentops-experiment.workflow-finalize",
}

BUILD_STEP_REF_MAP = {
    "agentops-build": "agentops-build",
    "agentops-build.admission": "agentops-build.admission",
    "agentops-build.mayor": "agentops-build.mayor",
    "agentops-build.mayor.spec": "agentops-build.mayor.spec",
    "mayor.iteration.1": "agentops-build.mayor.iteration.1",
    "agentops-build.plan": "agentops-build.plan",
    "agentops-build.plan.spec": "agentops-build.plan.spec",
    "plan.iteration.1": "agentops-build.plan.iteration.1",
    "agentops-build.workflow-finalize": "agentops-build.workflow-finalize",
}
EXPERIMENT_STEP_REF_MAP = {
    "agentops-experiment": "agentops-experiment",
    "agentops-experiment.admission": "agentops-experiment.admission",
    "agentops-experiment.implement": "agentops-experiment.implement",
    "agentops-experiment.implement.spec": "agentops-experiment.implement.spec",
    "implement.iteration.1": "agentops-experiment.implement.iteration.1",
    "agentops-experiment.validate": "agentops-experiment.validate",
    "agentops-experiment.validate.spec": "agentops-experiment.validate.spec",
    "validate.iteration.1": "agentops-experiment.validate.iteration.1",
    "agentops-experiment.workflow-finalize": "agentops-experiment.workflow-finalize",
}


def executable(path: object, label: str) -> str:
    if not isinstance(path, str):
        raise FeederError(f"{label} is not a path")
    resolved = Path(path).resolve(strict=True)
    if not resolved.is_file() or not os.access(resolved, os.X_OK):
        raise FeederError(f"{label} is not an executable regular file: {resolved}")
    return str(resolved)


def regular_file(path: object, label: str) -> str:
    if not isinstance(path, str):
        raise FeederError(f"{label} is not a path")
    requested = Path(path)
    if requested.is_symlink():
        raise FeederError(f"{label} must not be a symlink: {requested}")
    resolved = requested.resolve(strict=True)
    if not resolved.is_file():
        raise FeederError(f"{label} is not a regular file: {resolved}")
    return str(resolved)


def delivery_configuration(args: argparse.Namespace, root: Path, repository: Path, base_ref: str,
                           gc_bin: str, bd_bin: str, git_bin: str) -> tuple[dict[str, Any], str]:
    def selected(attribute: str, environment: str) -> str:
        value = getattr(args, attribute, None) or os.environ.get(environment)
        if not isinstance(value, str) or not value:
            raise FeederError(f"{environment} is required for PASS-only delivery")
        return value

    native_path = Path(selected("delivery_native_context", "AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT")).resolve(strict=True)
    native, native_raw = read_exact(native_path)
    validate_schema(native, "gc-delivery-native-context.v1.schema.json", "native delivery context")
    claimed_digest = getattr(args, "delivery_native_context_digest", None) or os.environ.get("AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT_DIGEST")
    native_digest = digest(native_raw)
    if claimed_digest != native_digest:
        raise FeederError("native delivery context digest differs from exact bytes")
    evidence_root = Path(selected("delivery_root", "AGENTOPS_GC_DELIVERY_ROOT")).resolve(strict=True)
    if not evidence_root.is_dir() or evidence_root != factory_evidence_root(root) / "delivery":
        raise FeederError("delivery evidence root is not the factory-owned GC33 namespace")
    mode = getattr(args, "delivery_mode", None) or os.environ.get("AGENTOPS_GC_DELIVERY_MODE", "auto")
    if mode not in {"auto", "manual"}:
        raise FeederError("delivery mode must be auto or manual")
    raw_deadline = getattr(args, "delivery_deadline_seconds", None) or os.environ.get("AGENTOPS_GC_DELIVERY_DEADLINE_SECONDS", "86400")
    try:
        deadline_seconds = int(raw_deadline)
    except (TypeError, ValueError) as exc:
        raise FeederError("delivery deadline must be integer seconds") from exc
    if not 300 <= deadline_seconds <= 604800:
        raise FeederError("delivery deadline must be between 300 and 604800 seconds")
    exact = {"repository_dir": str(repository), "base_ref": base_ref}
    if any(native.get(key) != value for key, value in exact.items()):
        raise FeederError("native delivery repository or base differs from factory request")
    expected_bins = {"gc": gc_bin, "bd": bd_bin, "git": git_bin}
    for name, path in expected_bins.items():
        binding = native["executables"].get(name)
        if not isinstance(binding, dict) or binding.get("path") != path or binding.get("digest") != digest(Path(path).read_bytes()):
            raise FeederError(f"native delivery {name} binding differs from factory executable")
    for name in ("gh", "bash", "agentops-gc-delivery"):
        binding = native["executables"][name]
        path = executable(binding["path"], f"native delivery {name}")
        if binding["digest"] != digest(Path(path).read_bytes()):
            raise FeederError(f"native delivery {name} digest differs from executable bytes")
    delivery = {
        "native_context_path": str(native_path), "native_context_digest": native_digest,
        "evidence_root": str(evidence_root), "mode": mode, "deadline_seconds": deadline_seconds,
    }
    return delivery, sealed_rig_id(native.get("rig_id"), "native delivery rig_id")


def close_inert_step(bd_bin: str, root: Path, record: dict[str, Any], reason: str) -> None:
    status = record.get("status")
    if status == "closed":
        return
    if status != "open" or record.get("assignee") not in (None, ""):
        raise FeederError("inert admission step is no longer unassigned and open")
    run_checked([bd_bin, "close", str(record["id"]), "--reason", reason, "--json"], root)


def start(args: argparse.Namespace) -> int:
    root = Path(args.root).resolve(strict=True)
    repository = Path(args.repository).resolve(strict=True)
    if not repository.is_dir():
        raise FeederError("repository is not a directory")
    if root != repository:
        raise FeederError("factory root must be the selected rig repository")
    requested_rig_id = sealed_rig_id(getattr(args, "rig_id", None), "--rig-id")
    intent_source = Path(args.intent).resolve(strict=True)
    if not intent_source.is_file() or intent_source.is_symlink():
        raise FeederError("intent source must be a regular file")
    intent_raw = intent_source.read_bytes()
    intent_digest = digest(intent_raw)
    git_bin = executable(args.git_bin, "git binary")
    bd_bin = executable(args.bd_bin, "bd binary")
    gc_bin = executable(args.gc_bin, "gc binary")
    role_adapter = regular_file(args.role_adapter, "role adapter")
    packet_adapter = regular_file(args.packet_adapter, "packet adapter")
    factory_check = executable(args.factory_check, "factory check")
    delivery, native_rig_id = delivery_configuration(args, root, repository, args.base_ref, gc_bin, bd_bin, git_bin)
    if requested_rig_id != native_rig_id:
        raise FeederError("--rig-id differs from the digest-bound native delivery rig_id")
    routes = formula_routes(requested_rig_id)
    stable_identity = {"source_bead_id": args.source_bead, "intent_digest": intent_digest, "repository_dir": str(repository), "base_ref": args.base_ref, "rig_id": requested_rig_id, "routes": routes, "delivery": delivery}
    program_id = "gc33-" + digest(canonical(stable_identity))[:24]
    workspace = root / ".gc" / "agentops" / "factory" / "builds" / program_id
    candidate_workspace_root = repository.parent / (repository.name + "-gc33-workers")
    packet_root = root / ".gc" / "agentops" / "factory" / "packets" / program_id
    workspace.mkdir(mode=0o700, parents=True, exist_ok=True)
    intent_path = workspace / "intent-source"
    atomic_bytes(intent_path, intent_raw, "intent snapshot")
    graph_path = workspace / "program-graph.v2.json"
    mayor_request_path = workspace / "mayor-request.v2.json"
    mayor_result_path = workspace / "mayor-response.v2.json"
    plan_request_path = workspace / "plan-request.v2.json"
    plan_result_path = workspace / "plan-response.v2.json"
    plan_artifact_path = workspace / "plan-review.v1.json"
    context_path = workspace / "build-context.v1.json"

    context: dict[str, Any] | None = None
    records: dict[str, dict[str, Any]]
    if context_path.exists():
        context, _ = read_exact(context_path)
        validate_build_context(context)
        if (any(context.get(key) != value for key, value in stable_identity.items())
                or context.get("program_id") != program_id or context.get("max_parallel") != args.max_parallel):
            raise FeederError("existing build context belongs to different inputs")
        base_oid = context["base_oid"]
        steps, records = workflow_rows(bd_bin, root, str(context["workflow_root_id"]), "agentops-build", BUILD_STEP_REF_MAP)
        if steps != context["workflow_steps"]:
            raise FeederError("existing build workflow identity changed")
    else:
        base_oid = run_checked([git_bin, "-C", str(repository), "rev-parse", f"{args.base_ref}^{{commit}}"], repository).decode().strip()
        if not re.fullmatch(r"[0-9a-f]{40}", base_oid):
            raise FeederError("base ref does not resolve to one commit")
        identity = {**stable_identity, "base_oid": base_oid}
        cook = parse_json(run_checked([
            gc_bin, "formula", "cook", "agentops-build", "--attach", args.source_bead,
            "--var", "work_dir=" + str(workspace), "--var", "role_adapter=" + role_adapter,
            "--var", "mayor_request=" + str(mayor_request_path), "--var", "plan_request=" + str(plan_request_path),
            "--var", "mayor_target=" + routes["mayor"], "--var", "plan_target=" + routes["plan"],
            "--var", "factory_check=" + factory_check, "--json",
        ], root), "gc formula cook agentops-build")
        if not isinstance(cook, dict) or cook.get("ok") is not True or cook.get("formula") != "agentops-build" or cook.get("attach_bead_id") != args.source_bead:
            raise FeederError("gc formula cook returned a conflicting build workflow")
        workflow_root_id = cook.get("workflow_root_id") or cook.get("root_id")
        if not isinstance(workflow_root_id, str) or not workflow_root_id:
            raise FeederError("gc formula cook returned no workflow root")
        steps, records = workflow_rows(bd_bin, root, workflow_root_id, "agentops-build", BUILD_STEP_REF_MAP)
        context = {
            "schema_version": "factory-build-context.v1", "program_id": program_id,
            **identity, "intent_path": str(intent_path), "root": str(root), "workspace": str(workspace),
            "candidate_workspace_root": str(candidate_workspace_root), "packet_root": str(packet_root),
            "max_parallel": args.max_parallel,
            "graph_path": str(graph_path), "mayor_request_path": str(mayor_request_path),
            "mayor_result_path": str(mayor_result_path), "plan_request_path": str(plan_request_path),
            "plan_result_path": str(plan_result_path), "plan_artifact_path": str(plan_artifact_path),
            "workflow_root_id": workflow_root_id, "workflow_steps": steps,
            "role_adapter": role_adapter, "packet_adapter": packet_adapter, "factory_check": factory_check,
            "bd_bin": bd_bin, "gc_bin": gc_bin, "git_bin": git_bin, "created_at": args.created_at,
        }
        validate_build_context(context)
        atomic_write(context_path, context)

    mayor_request = {
        "schema_version": "factory-role-request.v2", "request_id": program_id + ".mayor",
        "program_id": program_id, "semantic_bead_id": context["workflow_steps"]["agentops-build.mayor.iteration.1"],
        "workspace": str(workspace), "intent_source": str(intent_path), "intent_digest": intent_digest,
        "subject_path": str(intent_path), "subject_digest": intent_digest,
        "evidence_refs": [{"path": str(intent_path), "digest": intent_digest}, {"path": str(context_path), "digest": digest(context_path.read_bytes())}], "prior_context_id": None,
        "role": "mayor", "requested": {"model": "fable", "reasoning": "adaptive", "provider": "claude", "fallback": {"allowed": False, "used": False, "reason": None}},
        "artifact_path": str(graph_path), "result_path": str(mayor_result_path),
    }
    validate_schema(mayor_request, "factory-role-request.v2.schema.json", "Mayor request")
    atomic_write(mayor_request_path, mayor_request)
    mayor_row = records["agentops-build.mayor.iteration.1"]
    mayor_meta = mayor_row.get("metadata") if isinstance(mayor_row.get("metadata"), dict) else {}
    if (mayor_meta.get("gc.run_target") != context["routes"]["mayor"]
            or mayor_meta.get("gc.routed_to") != context["routes"]["mayor"]
            or mayor_meta.get("work_dir") != str(workspace)
            or str(mayor_request_path) not in str(mayor_row.get("description", ""))):
        raise FeederError("effective Mayor step does not bind request, target, and work_dir")
    close_inert_step(bd_bin, root, records["agentops-build.admission"], "AgentOps checked build request admitted")
    print(json.dumps(context, sort_keys=True))
    return 0


ROLE_CHECK_FIELDS = {
    "schema_version", "kind", "phase", "role", "checked_bead_id", "semantic_bead_id",
    "session_context_id", "rig", "request_path", "result_path", "request_digest",
    "result_digest", "artifact_path", "artifact_digest",
}


def object_bytes(path: Path, label: str) -> tuple[dict[str, Any], bytes]:
    try:
        raw = path.read_bytes()
        value = json.loads(raw)
    except (OSError, json.JSONDecodeError) as exc:
        raise FeederError(f"invalid {label}: {path}") from exc
    if not isinstance(value, dict):
        raise FeederError(f"{label} is not an object: {path}")
    return value, raw


def nested_record(value: Any, identifier: str) -> dict[str, Any] | None:
    if isinstance(value, list):
        matches = [found for item in value if (found := nested_record(item, identifier)) is not None]
        if len(matches) > 1:
            raise FeederError(f"duplicate Beads records for {identifier}")
        return matches[0] if matches else None
    if not isinstance(value, dict):
        return None
    if str(value.get("id", "")) == identifier:
        return value
    for key in ("issue", "bead", "issues", "beads", "result", "data"):
        if key in value and (found := nested_record(value[key], identifier)) is not None:
            return found
    return None


def expected_role_policy() -> dict[str, Any]:
    no = {"allowed": False, "used": False, "reason": None}
    return {
        "mayor": {"model": "fable", "reasoning": "adaptive", "provider": "claude", "fallback": no},
        "planner": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": no},
        "worker_pool": {
            "default": {"model": "terra", "reasoning": "high", "provider": "codex", "fallback": no},
            "overflow": {"model": "opus", "reasoning": "medium", "provider": "claude", "fallback": no},
            "fallback": no,
        },
        "validator": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": no},
        "refiner": {"model": "fable", "reasoning": "adaptive", "provider": "claude", "fallback": no, "ambiguity_only": True},
        "luna": {"model": "luna", "reasoning": "high", "provider": "codex", "fallback": no, "support_only": True},
    }


def load_build_context(role_request: dict[str, Any]) -> tuple[dict[str, Any], Path]:
    workspace = Path(str(role_request.get("workspace", ""))).resolve(strict=True)
    context_path = workspace / "build-context.v1.json"
    context, _ = read_exact(context_path)
    validate_build_context(context)
    if context.get("workspace") != str(workspace) or context.get("program_id") != role_request.get("program_id"):
        raise FeederError("role request does not bind its immutable build context")
    if context.get("intent_digest") != role_request.get("intent_digest") or context.get("intent_path") != role_request.get("intent_source"):
        raise FeederError("role request intent differs from immutable build context")
    return context, context_path


def verify_role_check(pointer: dict[str, Any]) -> tuple[dict[str, Any], dict[str, Any], dict[str, Any], dict[str, Any]]:
    require_fields(pointer, ROLE_CHECK_FIELDS, "role check pointer")
    if pointer.get("schema_version") != "agentops-factory-check.v1" or pointer.get("kind") != "role" or pointer.get("phase") not in {"mayor", "plan"} or pointer.get("role") != pointer.get("phase"):
        raise FeederError("role check pointer has an unsupported phase")
    request_path = Path(str(pointer["request_path"])).resolve(strict=True)
    result_path = Path(str(pointer["result_path"])).resolve(strict=True)
    artifact_path = Path(str(pointer["artifact_path"])).resolve(strict=True)
    role_request, request_raw = read_exact(request_path)
    response, response_raw = read_exact(result_path)
    artifact, artifact_raw = object_bytes(artifact_path, "role artifact")
    if (pointer["request_digest"] != digest(request_raw) or pointer["result_digest"] != digest(response_raw)
            or pointer["artifact_digest"] != digest(artifact_raw)):
        raise FeederError("role check pointer contains stale bytes")
    validate_schema(role_request, "factory-role-request.v2.schema.json", "role request")
    validate_schema(response, "factory-role-response.v2.schema.json", "role response")
    artifact_schema = "program-graph.v2.schema.json" if pointer["phase"] == "mayor" else "plan-review.v1.schema.json"
    validate_schema(artifact, artifact_schema, "role artifact")
    bindings = {
        "role": pointer["phase"], "semantic_bead_id": pointer["checked_bead_id"],
        "session_context_id": pointer["session_context_id"], "request_digest": pointer["request_digest"],
        "artifact_path": str(artifact_path), "artifact_digest": pointer["artifact_digest"],
    }
    if pointer["semantic_bead_id"] != pointer["checked_bead_id"] or any(response.get(key) != value for key, value in bindings.items()) or role_request.get("role") != pointer["phase"] or role_request.get("semantic_bead_id") != pointer["checked_bead_id"]:
        raise FeederError("role request/response/pointer identity does not bind the checked attempt")
    context, _ = load_build_context(role_request)
    expected_attempt = context["workflow_steps"][f"agentops-build.{pointer['phase']}.iteration.1"]
    expected_control = context["workflow_steps"][f"agentops-build.{pointer['phase']}"]
    if pointer["checked_bead_id"] != expected_attempt or os.environ.get("GC_BEAD_ID", "") not in ("", expected_control):
        raise FeederError("role check ran for the wrong Formula control/attempt")
    verify = [sys.executable, context["role_adapter"], "verify-role-v2", "--request", str(request_path),
              "--response", str(result_path), "--bead", pointer["checked_bead_id"], "--session", pointer["session_context_id"]]
    if pointer["rig"] is not None:
        if not isinstance(pointer["rig"], str) or not SAFE_ID.fullmatch(pointer["rig"]):
            raise FeederError("role check pointer rig is invalid")
        verify += ["--rig", pointer["rig"]]
    verified = parse_json(run_checked(verify, Path(context["root"])), "role adapter verification")
    if not isinstance(verified, dict) or verified.get("ok") is not True or verified.get("response_digest") != pointer["result_digest"]:
        raise FeederError("role adapter could not re-attest the durable GC launch")
    return role_request, response, artifact, context


def validate_mayor_graph(graph: dict[str, Any], context: dict[str, Any]) -> None:
    validate_executable_graph(graph)
    exact = {
        "program_id": context["program_id"], "intent_digest": context["intent_digest"],
        "repository_dir": context["repository_dir"], "base_ref": context["base_ref"],
        "base_oid": context["base_oid"], "workspace_root": context["candidate_workspace_root"],
        "packet_root": context["packet_root"], "delivery_group_id": context["program_id"],
        "prefix_safety": "safe", "role_policy": expected_role_policy(),
    }
    if any(graph.get(key) != value for key, value in exact.items()):
        raise FeederError("Mayor graph changes immutable program, repository, path, or role policy")
    if graph.get("max_parallel") > context["max_parallel"]:
        raise FeederError("Mayor graph exceeds controller maximum parallelism")


def prepare_plan_request(role_request: dict[str, Any], response: dict[str, Any], graph: dict[str, Any], context: dict[str, Any]) -> None:
    validate_mayor_graph(graph, context)
    graph_path = Path(context["graph_path"])
    graph_digest = digest(graph_path.read_bytes())
    context_path = Path(context["workspace"]) / "build-context.v1.json"
    plan_request = {
        "schema_version": "factory-role-request.v2", "request_id": context["program_id"] + ".plan",
        "program_id": context["program_id"], "semantic_bead_id": context["workflow_steps"]["agentops-build.plan.iteration.1"],
        "workspace": context["workspace"], "intent_source": context["intent_path"], "intent_digest": context["intent_digest"],
        "subject_path": context["graph_path"], "subject_digest": graph_digest,
        "evidence_refs": [
            {"path": context["intent_path"], "digest": context["intent_digest"]},
            {"path": str(context_path), "digest": digest(context_path.read_bytes())},
            {"path": context["graph_path"], "digest": graph_digest},
            {"path": context["mayor_result_path"], "digest": digest(Path(context["mayor_result_path"]).read_bytes())},
        ],
        "prior_context_id": response["session_context_id"], "role": "plan",
        "requested": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": {"allowed": False, "used": False, "reason": None}},
        "artifact_path": context["plan_artifact_path"], "result_path": context["plan_result_path"],
    }
    validate_schema(plan_request, "factory-role-request.v2.schema.json", "Plan request")
    atomic_write(Path(context["plan_request_path"]), plan_request)
    raw = run_checked([context["bd_bin"], "show", plan_request["semantic_bead_id"], "--json"], Path(context["root"]))
    row = nested_record(parse_json(raw, "bd show Plan attempt"), plan_request["semantic_bead_id"])
    metadata = row.get("metadata") if isinstance(row, dict) and isinstance(row.get("metadata"), dict) else {}
    if (row is None or metadata.get("gc.run_target") != context["routes"]["plan"]
            or metadata.get("gc.routed_to") != context["routes"]["plan"]
            or metadata.get("work_dir") != context["workspace"]
            or context["plan_request_path"] not in str(row.get("description", ""))):
        raise FeederError("effective Plan attempt does not bind its future request, target, and work_dir")


PACKET_CHECK_FIELDS = {
    "schema_version", "kind", "phase", "checked_bead_id", "session_context_id",
    "packet_path", "packet_digest", "result_path", "result_digest",
}


def packet_binding(pointer: dict[str, Any]) -> dict[str, Any]:
    require_fields(pointer, PACKET_CHECK_FIELDS, "packet check pointer")
    phase = pointer.get("phase")
    if pointer.get("schema_version") != "agentops-factory-check.v1" or pointer.get("kind") != "packet" or phase not in {"implement", "validate"}:
        raise PhaseFailure("invalid_pointer", "packet check pointer has an unsupported phase")
    packet_path = Path(str(pointer["packet_path"])).resolve(strict=True)
    result_path = Path(str(pointer["result_path"])).resolve(strict=True)
    packet, packet_raw = read_exact(packet_path)
    response, response_raw = object_bytes(result_path, "agent response")
    if digest(packet_raw) != pointer["packet_digest"] or digest(response_raw) != pointer["result_digest"]:
        raise PhaseFailure("stale_pointer", "packet check pointer contains stale bytes")
    if packet.get("role") != phase or packet.get("result_path") != str(result_path):
        raise PhaseFailure("packet_binding", "packet phase or result path differs from the checked pointer")
    receipt_path = Path(str(packet.get("factory_admission_receipt", ""))).resolve(strict=True)
    receipt, receipt_raw = read_exact(receipt_path)
    validate_schema(receipt, "graph-admission-receipt.v1.schema.json", "graph admission receipt")
    node_id = packet.get("factory_node_id")
    recorded = receipt.get("nodes", {}).get(node_id)
    if not isinstance(node_id, str) or not isinstance(recorded, dict):
        raise PhaseFailure("packet_binding", "packet does not select one admitted graph node")
    expected_packet = recorded["implement_packet"] if phase == "implement" else recorded["validate_packet"]
    if str(packet_path) != expected_packet:
        raise PhaseFailure("packet_binding", "packet path differs from immutable graph admission")
    if phase == "implement" and pointer["packet_digest"] != recorded["implement_packet_digest"]:
        raise PhaseFailure("packet_binding", "implement packet digest differs from immutable graph admission")
    context_path = Path(receipt["context_path"]).resolve(strict=True)
    context, context_raw = read_exact(context_path)
    validate_build_context(context)
    if digest(context_raw) != receipt["context_digest"] or digest(receipt_raw) != digest(receipt_path.read_bytes()):
        raise PhaseFailure("context_binding", "admission context or receipt changed")
    graph, graph_raw = read_exact(Path(context["graph_path"]).resolve(strict=True))
    validate_schema(graph, "program-graph.v2.schema.json", "program graph")
    if digest(graph_raw) != receipt["program_digest"] or graph.get("program_id") != receipt["program_id"]:
        raise PhaseFailure("context_binding", "program graph differs from admitted digest")
    nodes = [item for item in graph["nodes"] if item.get("id") == node_id]
    if len(nodes) != 1 or recorded.get("semantic_bead_id") != receipt["graph_bead_ids"].get(node_id):
        raise PhaseFailure("context_binding", "admitted node or semantic bead identity is ambiguous")
    node = nodes[0]
    expected_attempt = recorded["workflow_steps"][f"agentops-experiment.{phase}.iteration.1"]
    expected_control = recorded["workflow_steps"][f"agentops-experiment.{phase}"]
    if pointer["checked_bead_id"] != expected_attempt or response.get("transport_bead_id") != expected_attempt:
        raise PhaseFailure("attempt_binding", "packet response does not bind the checked Formula attempt")
    if os.environ.get("GC_BEAD_ID", "") not in ("", expected_control):
        raise PhaseFailure("attempt_binding", "controller check ran for the wrong Formula control")
    if response.get("session_context_id") != pointer["session_context_id"]:
        raise PhaseFailure("session_binding", "packet pointer and response session identities differ")
    packet_module = load_python(context["packet_adapter"], "agentops_gc_packet_for_factory_check")
    try:
        validated, paths = packet_module.validate_envelope(packet_path, allow_existing_result=True)
    except Exception as exc:
        raise PhaseFailure("invalid_envelope", f"checked packet violates selected adapter: {exc}") from exc
    attempt = show_bead(context["bd_bin"], Path(context["root"]), expected_attempt, f"{phase} Formula attempt")
    metadata = bead_metadata(attempt, f"{phase} Formula attempt")
    target = metadata.get("gc.run_target")
    if (attempt.get("status") != "closed" or attempt.get("assignee") != response.get("session_name")
            or metadata.get("work_dir") != recorded["work_dir"] or metadata.get("gc.packet_path") != str(packet_path)
            or not isinstance(target, str) or response.get("template") != target):
        raise PhaseFailure("attempt_binding", "closed Formula attempt does not bind response, target, packet, and work_dir")
    try:
        session = packet_module.runtime_session(pointer["session_context_id"])
        packet_module.validate_agent_response(validated, paths, response, expected_attempt, attempt, session, target)
    except Exception as exc:
        raise PhaseFailure("runtime_binding", f"actual GC session or response does not match admitted packet: {exc}") from exc
    return {
        "packet_module": packet_module, "packet": validated, "packet_raw": packet_raw, "paths": paths,
        "response": response, "response_raw": response_raw, "session": session, "attempt": attempt,
        "target": target, "receipt": receipt, "receipt_path": receipt_path, "node": node,
        "node_id": node_id, "recorded": recorded, "context": context, "graph": graph,
    }


def command_env(argv: list[str], cwd: Path, additions: dict[str, str], input_bytes: bytes | None = None) -> bytes:
    environment = os.environ.copy()
    environment.update(additions)
    completed = subprocess.run(argv, cwd=cwd, env=environment, input=input_bytes, check=False, capture_output=True)
    if completed.returncode:
        detail = (completed.stderr or completed.stdout).decode(errors="replace").strip()
        raise PhaseFailure("git_freeze", detail or "candidate freeze command failed")
    return completed.stdout


def git_changes(git_bin: str, workspace: Path, base_oid: str, packet_module: Any) -> list[str]:
    changes: set[str] = set()
    for argv in (
        [git_bin, "-C", str(workspace), "diff", "--name-only", "-z", base_oid, "--"],
        [git_bin, "-C", str(workspace), "ls-files", "--others", "--exclude-standard", "-z"],
    ):
        raw = command_env(argv, workspace, {})
        changes.update(item.decode() for item in raw.split(b"\0") if item)
    return sorted(
        item for item in changes
        if not any(packet_module.path_matches(item, pattern) for pattern in packet_module.COMMON_EXCLUDES)
    )


def freeze_git_candidate(binding: dict[str, Any], changed_paths: list[str], subject_digest: str) -> dict[str, str]:
    context, graph = binding["context"], binding["graph"]
    workspace = Path(binding["recorded"]["work_dir"]).resolve(strict=True)
    git_bin = context["git_bin"]
    head = run_checked([git_bin, "-C", str(workspace), "rev-parse", "HEAD"], workspace).decode().strip()
    if head != graph["base_oid"]:
        raise PhaseFailure("git_freeze", "candidate HEAD moved away from the reviewed base")
    index_fd, index_name = tempfile.mkstemp(prefix=".gc33-index-", dir=workspace / ".gc")
    os.close(index_fd)
    os.unlink(index_name)
    index_path = Path(index_name)
    additions = {"GIT_INDEX_FILE": str(index_path)}
    try:
        command_env([git_bin, "-C", str(workspace), "read-tree", graph["base_oid"]], workspace, additions)
        command_env([git_bin, "-C", str(workspace), "add", "-A", "--", *changed_paths], workspace, additions)
        tree = command_env([git_bin, "-C", str(workspace), "write-tree"], workspace, additions).decode().strip()
        if not re.fullmatch(r"[0-9a-f]{40}", tree):
            raise PhaseFailure("git_freeze", "temporary index did not produce one tree")
        name = run_checked([git_bin, "-C", str(workspace), "config", "user.name"], workspace).decode().strip()
        email = run_checked([git_bin, "-C", str(workspace), "config", "user.email"], workspace).decode().strip()
        if not name or not email:
            raise PhaseFailure("git_identity", "default Git user.name and user.email are required")
        identity = {
            **additions, "GIT_AUTHOR_NAME": name, "GIT_AUTHOR_EMAIL": email,
            "GIT_COMMITTER_NAME": name, "GIT_COMMITTER_EMAIL": email,
            "GIT_AUTHOR_DATE": context["created_at"], "GIT_COMMITTER_DATE": context["created_at"],
        }
        message = f"AgentOps semantic candidate {graph['program_id']}/{binding['node_id']}\n".encode()
        commit = command_env(
            [git_bin, "-C", str(workspace), "commit-tree", tree, "-p", graph["base_oid"]],
            workspace, identity, message,
        ).decode().strip()
        if not re.fullmatch(r"[0-9a-f]{40}", commit):
            raise PhaseFailure("git_freeze", "commit-tree did not produce one candidate commit")
        observed_tree = run_checked([git_bin, "-C", str(workspace), "rev-parse", commit + "^{tree}"], workspace).decode().strip()
        frozen = command_env(
            [git_bin, "-C", str(workspace), "diff-tree", "--no-commit-id", "--name-only", "-r", "-z", graph["base_oid"], commit],
            workspace, {},
        )
        frozen_paths = sorted(item.decode() for item in frozen.split(b"\0") if item)
        if observed_tree != tree or frozen_paths != changed_paths:
            raise PhaseFailure("git_freeze", "candidate commit does not exactly cover runtime changed paths")
        return {"commit": commit, "tree": tree, "content_digest": digest(canonical({"tree": tree, "subject_manifest_digest": subject_digest}))}
    finally:
        index_path.unlink(missing_ok=True)
        Path(str(index_path) + ".lock").unlink(missing_ok=True)


def changed_path_manifest(module: Any, baseline: dict[str, Any], subject: dict[str, Any],
                          changed: list[str]) -> dict[str, Any]:
    """Derive the exact parent-to-candidate inventory consumed by delivery."""
    entries = {str(item.get("path")): item for item in subject.get("entries", []) if isinstance(item, dict)}
    if set(entries) & set(changed) != set(changed):
        raise PhaseFailure("manifest_identity", "subject manifest does not describe every changed path")
    value = {
        "schema_version": "subject-manifest.v1",
        "declared_roots": sorted(subject["declared_roots"]),
        "exclusions": sorted(subject["exclusions"]),
        "base_manifest_digest": baseline["canonical_manifest_digest"],
        "entries": [entries[path] for path in changed],
    }
    identity = json.dumps(value, ensure_ascii=False, separators=(",", ":"), sort_keys=True).encode()
    value["canonical_manifest_digest"] = digest(identity)
    if module.changed_paths(baseline, subject) != changed:
        raise PhaseFailure("manifest_identity", "changed-path manifest input is stale")
    return value


def runtime_attestation(node: dict[str, Any], session: dict[str, Any], role: str) -> dict[str, Any]:
    if role == "author":
        requested_model, requested_reasoning, requested_provider = node["model"], node["reasoning"], node["provider"]
    else:
        requested_model, requested_reasoning, requested_provider = "sol", "high", "codex"
    return {
        "context_id": session["id"], "requested_model": requested_model,
        "requested_reasoning": requested_reasoning, "requested_provider": requested_provider,
        "actual_model": session["model"], "actual_reasoning": session["reasoning"],
        "actual_provider": session["provider"], "actual_effort": session["effort"],
        "fallback": session["fallback"],
    }


def write_runtime_result(binding: dict[str, Any], evidence: dict[str, Any]) -> tuple[Path, dict[str, Any]]:
    module = binding["packet_module"]
    result = module.success_result(
        binding["packet"], binding["paths"], binding["response"],
        binding["attempt"]["id"], binding["target"], evidence,
    )
    path = binding["paths"]["evidence"] / "runtime-result.json"
    atomic_write(path, result)
    return path, result


def implement_transition(binding: dict[str, Any]) -> dict[str, Any]:
    if binding["response"].get("outcome") != "candidate":
        raise PhaseFailure("not_built", binding["response"].get("message") or "implementer produced no candidate")
    module, packet, paths = binding["packet_module"], binding["packet"], binding["paths"]
    baseline_path = paths["evidence"] / "runtime-baseline-manifest.json"
    baseline = module.load_object(baseline_path, "runtime baseline manifest")
    temporary = paths["evidence"] / ".runtime-subject-manifest.json"
    try:
        subject = module.build_manifest(packet, paths, temporary, baseline_path)
        atomic_write(paths["evidence"] / "runtime-subject-manifest.json", subject)
    finally:
        temporary.unlink(missing_ok=True)
    subject_path = paths["evidence"] / "runtime-subject-manifest.json"
    subject_changes = module.changed_paths(baseline, subject)
    workspace_changes = git_changes(binding["context"]["git_bin"], paths["workspace"], binding["graph"]["base_oid"], module)
    if not workspace_changes:
        raise PhaseFailure("not_built", "implementation produced no changed paths")
    scope = module.make_scope_receipt(
        packet, subject_changes, workspace_changes=workspace_changes, workspace_base_sha=binding["graph"]["base_oid"],
    )
    scope_path = paths["evidence"] / "runtime-scope-receipt.json"
    atomic_write(scope_path, scope)
    if scope["status"] != "PASS":
        raise PhaseFailure("scope_" + scope["status"].lower(), "implementation scope is not an exact PASS")
    delivery_manifest = changed_path_manifest(module, baseline, subject, workspace_changes)
    delivery_manifest_path = paths["evidence"] / "delivery-subject-manifest.v1.json"
    atomic_write(delivery_manifest_path, delivery_manifest)
    candidate = freeze_git_candidate(binding, workspace_changes, subject["canonical_manifest_digest"])
    freeze = {
        "schema_version": "candidate-freeze.v1", "program_id": binding["graph"]["program_id"],
        "program_digest": binding["receipt"]["program_digest"], "node_id": binding["node_id"],
        "semantic_bead_id": binding["recorded"]["semantic_bead_id"],
        "implement_packet_digest": digest(binding["packet_raw"]), "intent_digest": packet["intent_digest"],
        "base_oid": binding["graph"]["base_oid"], "author_context_id": binding["session"]["id"],
        "baseline_manifest": str(baseline_path), "baseline_manifest_digest": digest(baseline_path.read_bytes()),
        "subject_manifest": str(subject_path), "subject_manifest_digest": digest(subject_path.read_bytes()),
        "delivery_manifest": str(delivery_manifest_path),
        "delivery_manifest_digest": digest(delivery_manifest_path.read_bytes()),
        "delivery_manifest_content_digest": delivery_manifest["canonical_manifest_digest"],
        "scope_receipt": str(scope_path), "scope_receipt_digest": digest(scope_path.read_bytes()),
        "changed_paths": workspace_changes, "candidate": candidate, "created_at": binding["context"]["created_at"],
    }
    validate_schema(freeze, "candidate-freeze.v1.schema.json", "candidate freeze")
    freeze_path = paths["evidence"] / "candidate-freeze.v1.json"
    atomic_write(freeze_path, freeze)
    runtime_evidence = {
        "packet_digest": digest(binding["packet_raw"]), "agent_response_digest": digest(binding["response_raw"]),
        "session": binding["session"], "baseline_manifest": str(baseline_path),
        "baseline_manifest_digest": baseline["canonical_manifest_digest"], "subject_manifest": str(subject_path),
        "subject_manifest_digest": subject["canonical_manifest_digest"], "scope_receipt": str(scope_path),
        "scope_status": "PASS", "actual_changed_paths": workspace_changes, "subject_changed_paths": subject_changes,
        "packet_stable": True, "intent_stable": True, "agent_response_stable": True, "manifests_stable": True,
    }
    runtime_result_path, _ = write_runtime_result(binding, runtime_evidence)
    validate_packet_id = packet["packet_id"] + "-validate"
    validate_evidence = paths["workspace"] / ".gc" / "agentops" / validate_packet_id
    validate_packet = {
        "schema_version": "gc-execution-envelope.v1", "packet_id": validate_packet_id,
        "role": "validate", "provider": "codex", "intent_source": packet["intent_source"],
        "intent_digest": packet["intent_digest"], "workspace": packet["workspace"], "subject": packet["subject"],
        "write_scope": [], "evidence_dir": str(validate_evidence),
        "result_path": str(validate_evidence / "agent-response.v1.json"),
        "baseline_manifest": str(baseline_path), "subject_manifest": str(subject_path),
        "scope_receipt": str(scope_path), "author_context_id": binding["session"]["id"],
        "factory_admission_receipt": str(binding["receipt_path"]), "factory_node_id": binding["node_id"],
    }
    atomic_write(Path(binding["recorded"]["validate_packet"]), validate_packet)
    try:
        module.validate_envelope(Path(binding["recorded"]["validate_packet"]))
    except Exception as exc:
        raise PhaseFailure("validate_envelope", f"derived Validate packet is invalid: {exc}") from exc
    return {
        "freeze": freeze, "freeze_path": freeze_path, "runtime_result_path": runtime_result_path,
        "author_attestation": runtime_attestation(binding["node"], binding["session"], "author"),
    }


def terminal_path(binding: dict[str, Any]) -> Path:
    return factory_evidence_root(Path(binding["context"]["root"])) / "semantic-terminals" / binding["receipt"]["program_digest"] / binding["node_id"] / "terminal.json"


def store_semantic_terminal(binding: dict[str, Any], terminal: dict[str, Any], certificate_digest: str) -> Path:
    validate_schema(terminal, "semantic-terminal.v1.schema.json", "semantic terminal")
    path = terminal_path(binding)
    atomic_write(path, terminal)
    bead_id = binding["recorded"]["semantic_bead_id"]
    root = Path(binding["context"]["root"])
    bd_bin = binding["context"]["bd_bin"]
    terminal_metadata = canonical({
        "verdict": terminal["status"], "terminal_digest": digest(path.read_bytes()),
        "certificate_digest": certificate_digest,
    }).decode().strip()
    row = show_bead(bd_bin, root, bead_id, "semantic terminal bead")
    metadata = bead_metadata(row, "semantic terminal bead")
    expected_metadata = {"gc33.terminal": terminal_metadata, "gc33.terminal_receipt": str(path)}
    present = {key: metadata.get(key) for key in expected_metadata if key in metadata}
    if present and present != expected_metadata:
        raise PhaseFailure("semantic_terminal_conflict", "semantic bead already has different terminal metadata")
    if present != expected_metadata:
        run_checked([
            bd_bin, "update", bead_id,
            "--set-metadata", "gc33.terminal=" + terminal_metadata,
            "--set-metadata", "gc33.terminal_receipt=" + str(path), "--json",
        ], root)
        row = show_bead(bd_bin, root, bead_id, "semantic terminal bead")
        metadata = bead_metadata(row, "semantic terminal bead")
        if any(metadata.get(key) != value for key, value in expected_metadata.items()):
            raise PhaseFailure("semantic_terminal_conflict", "semantic terminal metadata was not stored exactly")
    if row.get("status") != "closed":
        if row.get("status") not in {"open", "in_progress", "blocked", "deferred"}:
            raise PhaseFailure("semantic_terminal_conflict", "semantic bead has an unsupported nonterminal status")
        run_checked([bd_bin, "close", bead_id, "--reason", "AgentOps semantic terminal: " + terminal["status"], "--json"], root)
        row = show_bead(bd_bin, root, bead_id, "semantic terminal bead")
        if row.get("status") != "closed":
            raise PhaseFailure("semantic_terminal_conflict", "semantic terminal bead did not close")
    return path


def not_built_terminal(binding: dict[str, Any], failure: PhaseFailure) -> dict[str, Any]:
    terminal = {
        "schema_version": "semantic-terminal.v1", "program_id": binding["graph"]["program_id"],
        "program_digest": binding["receipt"]["program_digest"], "node_id": binding["node_id"],
        "semantic_bead_id": binding["recorded"]["semantic_bead_id"],
        "intent_digest": binding["graph"]["intent_digest"], "status": "NOT_BUILT",
        "candidate_freeze_ref": None, "candidate_freeze_digest": None,
        "verdict_ref": None, "verdict_digest": None,
        "author_context_id": binding["session"].get("id"), "validator_context_id": None,
        "error": {"code": failure.code, "message": str(failure)}, "created_at": binding["context"]["created_at"],
    }
    store_semantic_terminal(binding, terminal, "")
    return terminal


def not_proven_terminal(binding: dict[str, Any], failure: PhaseFailure) -> dict[str, Any]:
    try:
        freeze, freeze_path = candidate_freeze_for_validate(binding)
    except (FeederError, OSError):
        freeze = None
        freeze_path = None
    terminal = {
        "schema_version": "semantic-terminal.v1", "program_id": binding["graph"]["program_id"],
        "program_digest": binding["receipt"]["program_digest"], "node_id": binding["node_id"],
        "semantic_bead_id": binding["recorded"]["semantic_bead_id"],
        "intent_digest": binding["graph"]["intent_digest"], "status": "NOT_PROVEN",
        "candidate_freeze_ref": str(freeze_path) if freeze_path is not None else None,
        "candidate_freeze_digest": digest(freeze_path.read_bytes()) if freeze_path is not None else None,
        "verdict_ref": None, "verdict_digest": None,
        "author_context_id": freeze["author_context_id"] if freeze is not None else binding["packet"]["author_context_id"],
        "validator_context_id": binding["session"]["id"],
        "error": {"code": failure.code, "message": str(failure)}, "created_at": binding["context"]["created_at"],
    }
    store_semantic_terminal(binding, terminal, "")
    return terminal


def candidate_freeze_for_validate(binding: dict[str, Any]) -> tuple[dict[str, Any], Path]:
    implement_packet = Path(binding["recorded"]["implement_packet"])
    implement, _ = read_exact(implement_packet)
    freeze_path = Path(implement["evidence_dir"]) / "candidate-freeze.v1.json"
    freeze, freeze_raw = read_exact(freeze_path)
    validate_schema(freeze, "candidate-freeze.v1.schema.json", "candidate freeze")
    exact = {
        "program_id": binding["graph"]["program_id"], "program_digest": binding["receipt"]["program_digest"],
        "node_id": binding["node_id"], "semantic_bead_id": binding["recorded"]["semantic_bead_id"],
        "intent_digest": binding["graph"]["intent_digest"], "base_oid": binding["graph"]["base_oid"],
        "author_context_id": binding["packet"]["author_context_id"],
    }
    if any(freeze.get(key) != value for key, value in exact.items()):
        raise PhaseFailure("candidate_freeze", "Validate packet differs from immutable candidate freeze")
    if (binding["packet"].get("baseline_manifest") != freeze["baseline_manifest"]
            or binding["packet"].get("subject_manifest") != freeze["subject_manifest"]
            or binding["packet"].get("scope_receipt") != freeze["scope_receipt"]):
        raise PhaseFailure("candidate_freeze", "Validate packet manifest paths differ from candidate freeze")
    if freeze.get("implement_packet_digest") != digest(implement_packet.read_bytes()):
        raise PhaseFailure("candidate_freeze", "candidate freeze implement packet changed")
    exact_files = {
        "baseline_manifest": "baseline_manifest_digest", "subject_manifest": "subject_manifest_digest",
        "delivery_manifest": "delivery_manifest_digest", "scope_receipt": "scope_receipt_digest",
    }
    for field, digest_field in exact_files.items():
        path = Path(freeze[field]).resolve(strict=True)
        if digest(path.read_bytes()) != freeze[digest_field]:
            raise PhaseFailure("candidate_freeze", f"candidate freeze {field} bytes changed")
    delivery_manifest, _ = read_exact(Path(freeze["delivery_manifest"]))
    if delivery_manifest.get("canonical_manifest_digest") != freeze["delivery_manifest_content_digest"]:
        raise PhaseFailure("candidate_freeze", "candidate freeze delivery manifest identity changed")
    return freeze, freeze_path


def admission_certificate(binding: dict[str, Any], freeze: dict[str, Any], freeze_path: Path,
                          verdict_path: Path, validate_result_path: Path) -> tuple[dict[str, Any], Path, str]:
    verdict_raw = verdict_path.read_bytes()
    verdict = json.loads(verdict_raw)
    subject = binding["packet_module"].load_object(Path(freeze["subject_manifest"]), "subject manifest")
    evidence = {
        "candidate_freeze_digest": digest(freeze_path.read_bytes()), "verdict_digest": digest(verdict_raw),
        "scope_receipt_digest": freeze["scope_receipt_digest"],
        "validate_result_digest": digest(validate_result_path.read_bytes()),
    }
    store_identity = "beads:" + str(Path(os.environ.get("BEADS_DIR", Path(binding["context"]["root"]) / ".beads")).resolve(strict=False))
    certificate = {
        "schema_version": "admission-certificate.v2", "semantic_bead_id": binding["recorded"]["semantic_bead_id"],
        "intent_digest": binding["graph"]["intent_digest"], "verdict": "PASS", "candidate": freeze["candidate"],
        "store": {"identity": store_identity, "digest": digest(canonical({"identity": store_identity, "semantic_bead_id": binding["recorded"]["semantic_bead_id"], "program_digest": binding["receipt"]["program_digest"]}))},
        "changed_path_manifest": freeze["delivery_manifest_content_digest"], "verdict_digest": digest(verdict_raw),
        "evidence_digest": digest(canonical(evidence)),
        "attestations": {
            "author": runtime_attestation(binding["node"], {"id": freeze["author_context_id"], **binding["author_session"]}, "author"),
            "validator": runtime_attestation(binding["node"], binding["session"], "validator"),
        },
        "delivery_group_id": binding["graph"]["delivery_group_id"], "prefix_safety": binding["graph"]["prefix_safety"],
    }
    validate_schema(certificate, "admission-certificate.v2.schema.json", "admission certificate")
    path = factory_evidence_root(Path(binding["context"]["root"])) / "admission-certificates" / binding["receipt"]["program_digest"] / binding["node_id"] / "certificate.json"
    atomic_write(path, certificate)
    return certificate, path, digest(path.read_bytes())


def validate_transition(binding: dict[str, Any]) -> dict[str, Any]:
    freeze, freeze_path = candidate_freeze_for_validate(binding)
    module, paths = binding["packet_module"], binding["paths"]
    matched, detail = module.verify_manifest(paths)
    if not matched:
        raise PhaseFailure("subject_mutated", "candidate differs from frozen subject: " + detail)
    if binding["response"].get("outcome") != "evidence":
        raise PhaseFailure("not_proven", binding["response"].get("message") or "validator produced no verdict evidence")
    artifacts = binding["response"].get("artifacts", [])
    if len(artifacts) != 1:
        raise PhaseFailure("invalid_verdict", "validator response must contain exactly one verdict")
    verdict_path = Path(artifacts[0]["path"]).resolve(strict=True)
    verdict = module.load_object(verdict_path, "verdict.v2")
    verdict_status = verdict.get("verdict")
    if verdict_status not in {"PASS", "FAIL", "NOT_PROVEN"}:
        raise PhaseFailure("invalid_verdict", "validator artifact has no terminal verdict")
    runtime_evidence = {
        "packet_digest": digest(binding["packet_raw"]), "agent_response_digest": digest(binding["response_raw"]),
        "session": binding["session"], "subject_matched_before": True, "subject_matched_after": True,
        "subject_stable": True, "packet_stable": True, "intent_stable": True,
        "agent_response_stable": True, "manifests_stable": True,
    }
    validate_result_path, _ = write_runtime_result(binding, runtime_evidence)
    implement_packet, _ = read_exact(Path(binding["recorded"]["implement_packet"]))
    implement_result_path = Path(implement_packet["evidence_dir"]) / "runtime-result.json"
    implement_result = json.loads(implement_result_path.read_bytes())
    author_session = implement_result.get("runtime_evidence", {}).get("session")
    if not isinstance(author_session, dict) or author_session.get("id") != freeze["author_context_id"]:
        raise PhaseFailure("author_binding", "implement runtime result does not bind candidate author")
    binding["author_session"] = author_session
    certificate = certificate_path = certificate_digest = None
    if verdict_status == "PASS":
        certificate, certificate_path, certificate_digest = admission_certificate(
            binding, freeze, freeze_path, verdict_path, validate_result_path,
        )
    terminal = {
        "schema_version": "semantic-terminal.v1", "program_id": binding["graph"]["program_id"],
        "program_digest": binding["receipt"]["program_digest"], "node_id": binding["node_id"],
        "semantic_bead_id": binding["recorded"]["semantic_bead_id"], "intent_digest": binding["graph"]["intent_digest"],
        "status": verdict_status, "candidate_freeze_ref": str(freeze_path),
        "candidate_freeze_digest": digest(freeze_path.read_bytes()), "verdict_ref": str(verdict_path),
        "verdict_digest": digest(verdict_path.read_bytes()), "author_context_id": freeze["author_context_id"],
        "validator_context_id": binding["session"]["id"], "error": None, "created_at": binding["context"]["created_at"],
    }
    stored_terminal = store_semantic_terminal(binding, terminal, certificate_digest or "")
    return {
        "terminal": terminal, "terminal_path": stored_terminal, "certificate": certificate,
        "certificate_path": certificate_path, "certificate_digest": certificate_digest,
        "freeze": freeze,
    }


def verified_terminal_material(root: Path, identifier: str, program_digest: str, node_id: str,
                               intent_digest: str, receipt: str, summary: dict[str, Any]) -> dict[str, Any]:
    if set(summary) != {"verdict", "terminal_digest", "certificate_digest"}:
        raise PhaseFailure("semantic_terminal_conflict", "semantic terminal summary fields are invalid")
    if (not isinstance(summary["verdict"], str) or not isinstance(summary["terminal_digest"], str)
            or not isinstance(summary["certificate_digest"], str)):
        raise PhaseFailure("semantic_terminal_conflict", "semantic terminal summary values are invalid")
    expected_terminal = factory_evidence_root(root) / "semantic-terminals" / program_digest / node_id / "terminal.json"
    actual_terminal = Path(receipt).resolve(strict=True)
    if actual_terminal != expected_terminal.resolve(strict=True):
        raise PhaseFailure("semantic_terminal_conflict", "semantic terminal receipt is outside its bound evidence path")
    terminal, terminal_raw = read_exact(actual_terminal)
    validate_schema(terminal, "semantic-terminal.v1.schema.json", "semantic terminal")
    exact = {
        "program_digest": program_digest, "node_id": node_id, "semantic_bead_id": identifier,
        "intent_digest": intent_digest, "status": summary["verdict"],
    }
    if digest(terminal_raw) != summary["terminal_digest"] or any(terminal.get(key) != value for key, value in exact.items()):
        raise PhaseFailure("semantic_terminal_conflict", "semantic terminal bytes or identity differ from its summary")
    certificate_digest = summary["certificate_digest"]
    if terminal["status"] == "PASS":
        if not re.fullmatch(r"[0-9a-f]{64}", certificate_digest):
            raise PhaseFailure("semantic_terminal_conflict", "PASS semantic terminal has no certificate digest")
        certificate_path = factory_evidence_root(root) / "admission-certificates" / program_digest / node_id / "certificate.json"
        certificate, certificate_raw = read_exact(certificate_path.resolve(strict=True))
        validate_schema(certificate, "admission-certificate.v2.schema.json", "semantic admission certificate")
        if (digest(certificate_raw) != certificate_digest or certificate.get("verdict") != "PASS"
                or certificate.get("semantic_bead_id") != identifier or certificate.get("intent_digest") != intent_digest):
            raise PhaseFailure("semantic_terminal_conflict", "PASS certificate differs from terminal identity")
    elif certificate_digest != "":
        raise PhaseFailure("semantic_terminal_conflict", "non-PASS semantic terminal carries a certificate")
    return terminal


def semantic_terminal_status(bd_bin: str, root: Path, identifier: str, program_digest: str,
                             node_id: str, intent_digest: str) -> str | None:
    row = show_bead(bd_bin, root, identifier, "semantic predecessor")
    metadata = bead_metadata(row, "semantic predecessor")
    encoded = metadata.get("gc33.terminal")
    receipt = metadata.get("gc33.terminal_receipt")
    if encoded is None and receipt is None:
        return None
    if not isinstance(encoded, str) or not isinstance(receipt, str):
        raise PhaseFailure("semantic_terminal_conflict", "semantic predecessor has partial terminal metadata")
    try:
        summary = json.loads(encoded)
    except json.JSONDecodeError as exc:
        raise PhaseFailure("semantic_terminal_conflict", "semantic predecessor terminal metadata is invalid") from exc
    terminal = verified_terminal_material(root, identifier, program_digest, node_id, intent_digest, receipt, summary)
    if row.get("status") != "closed":
        raise PhaseFailure("semantic_terminal_conflict", "semantic predecessor terminal does not bind its closed Bead")
    return str(terminal["status"])


def release_ready_admissions(receipt: dict[str, Any], graph: dict[str, Any], bd_bin: str, root: Path) -> list[str]:
    """Fill at most the reviewed semantic width; delivery state is irrelevant."""
    nodes = {node["id"]: node for node in graph["nodes"]}
    status = {
        node_id: semantic_terminal_status(
            bd_bin, root, recorded["semantic_bead_id"], receipt["program_digest"], node_id, graph["intent_digest"],
        )
        for node_id, recorded in receipt["nodes"].items()
    }
    active = 0
    candidates: list[str] = []
    for node_id in sorted(nodes):
        recorded = receipt["nodes"][node_id]
        admission_id = recorded["workflow_steps"]["agentops-experiment.admission"]
        admission = show_bead(bd_bin, root, admission_id, "experiment admission")
        if admission.get("status") == "closed" and status[node_id] is None:
            active += 1
            continue
        if admission.get("status") == "closed" or status[node_id] is not None:
            continue
        predecessors = nodes[node_id]["depends_on"]
        if all(status[parent] == "PASS" for parent in predecessors):
            candidates.append(node_id)
    released: list[str] = []
    for node_id in candidates[:max(0, graph["max_parallel"] - active)]:
        recorded = receipt["nodes"][node_id]
        admission_id = recorded["workflow_steps"]["agentops-experiment.admission"]
        admission = show_bead(bd_bin, root, admission_id, "ready experiment admission")
        close_inert_step(bd_bin, root, admission, "AgentOps checked semantic experiment admitted")
        released.append(admission_id)
    return released


def delivery_identity(binding: dict[str, Any], result: dict[str, Any]) -> dict[str, Any]:
    delivery = binding["context"]["delivery"]
    native, native_raw = read_exact(Path(delivery["native_context_path"]).resolve(strict=True))
    validate_schema(native, "gc-delivery-native-context.v1.schema.json", "native delivery context")
    if digest(native_raw) != delivery["native_context_digest"]:
        raise PhaseFailure("delivery_context", "native delivery context bytes changed")
    semantic = binding["recorded"]["semantic_bead_id"]
    terminal_ref = "beads:" + semantic + "#gc33.terminal"
    parts = [
        result["certificate_digest"], semantic, terminal_ref, native["rig_id"], native["repository"],
        native["remote"], native["base_ref"], delivery["mode"],
    ]
    handoff = digest("\0".join(parts).encode())
    created = datetime.fromisoformat(binding["context"]["created_at"].replace("Z", "+00:00"))
    if created.tzinfo is None:
        raise PhaseFailure("delivery_context", "factory created_at is not timezone-aware")
    deadline = (created.astimezone(timezone.utc) + timedelta(seconds=delivery["deadline_seconds"])).isoformat(timespec="seconds").replace("+00:00", "Z")
    return {
        "native": native, "semantic": semantic, "terminal_ref": terminal_ref, "handoff_id": handoff,
        "delivery_bead_id": "delivery-" + handoff[:20] + "-e000001",
        "external_ref": "handoff:" + handoff + ":epoch:1", "deadline": deadline,
    }


def initial_delivery_receipt_path(binding: dict[str, Any]) -> Path:
    return (factory_evidence_root(Path(binding["context"]["root"])) / "initial-deliveries"
            / binding["receipt"]["program_digest"] / binding["node_id"] / "receipt.json")


def adopt_initial_delivery(binding: dict[str, Any], result: dict[str, Any], identity: dict[str, Any]) -> dict[str, Any] | None:
    rows = [row for row in all_beads(binding["context"]["bd_bin"], Path(binding["context"]["root"]))
            if row.get("id") == identity["delivery_bead_id"]]
    if not rows:
        return None
    if len(rows) != 1:
        raise PhaseFailure("delivery_conflict", "initial delivery Bead identity is ambiguous")
    row = rows[0]
    metadata = bead_metadata(row, "initial delivery Bead")
    encoded = metadata.get("gc.delivery.v1")
    request_ref = metadata.get("gc.delivery_request")
    if metadata.get("gc.kind") != "delivery" or not isinstance(encoded, str) or not isinstance(request_ref, str):
        raise PhaseFailure("delivery_conflict", "initial delivery Bead lacks exact delivery metadata")
    try:
        record, reference = json.loads(encoded), json.loads(request_ref)
    except json.JSONDecodeError as exc:
        raise PhaseFailure("delivery_conflict", "initial delivery metadata is invalid JSON") from exc
    if canonical(record).decode().strip() != encoded or canonical(reference).decode().strip() != request_ref:
        raise PhaseFailure("delivery_conflict", "initial delivery metadata is not canonical")
    freeze = result["freeze"]
    exact_record = {
        "schema_version": "gc.delivery.v1", "handoff_id": identity["handoff_id"],
        "semantic_bead": identity["semantic"], "terminal_ref": identity["terminal_ref"],
        "certificate": result["certificate_digest"], "mode": binding["context"]["delivery"]["mode"],
        "candidate": freeze["candidate"]["commit"], "manifest": freeze["delivery_manifest_content_digest"],
        "ready_at": binding["context"]["created_at"], "deadline": identity["deadline"],
    }
    if any(record.get(key) != value for key, value in exact_record.items()):
        raise PhaseFailure("delivery_conflict", "initial delivery record differs from semantic PASS handoff")
    epoch = record.get("epoch")
    if (not isinstance(epoch, dict) or epoch.get("number") != 1 or epoch.get("base_ref") != binding["graph"]["base_ref"]
            or row.get("external_ref") != identity["external_ref"]):
        raise PhaseFailure("delivery_conflict", "initial delivery epoch or external reference changed")
    if set(reference) != {"schema_version", "path", "digest"} or reference.get("schema_version") != "gc.delivery.request-ref.v1":
        raise PhaseFailure("delivery_conflict", "initial delivery request reference is invalid")
    relative = reference.get("path")
    if not isinstance(relative, str) or not relative_path(relative):
        raise PhaseFailure("delivery_conflict", "initial delivery request reference escapes evidence root")
    request_path = Path(binding["context"]["delivery"]["evidence_root"]) / relative
    if not request_path.is_file() or digest(request_path.read_bytes()) != reference.get("digest"):
        raise PhaseFailure("delivery_conflict", "initial delivery request bytes do not match Beads reference")
    receipt = {
        "schema_version": "initial-delivery-receipt.v1", "program_digest": binding["receipt"]["program_digest"],
        "node_id": binding["node_id"], "semantic_bead_id": identity["semantic"],
        "terminal_digest": digest(Path(result["terminal_path"]).read_bytes()),
        "certificate_digest": result["certificate_digest"],
        "delivery_manifest_content_digest": freeze["delivery_manifest_content_digest"],
        "native_context_digest": binding["context"]["delivery"]["native_context_digest"],
        "handoff_id": identity["handoff_id"], "delivery_bead_id": identity["delivery_bead_id"],
        "external_ref": identity["external_ref"], "mode": binding["context"]["delivery"]["mode"],
        "created_at": binding["context"]["created_at"],
    }
    validate_schema(receipt, "initial-delivery-receipt.v1.schema.json", "initial delivery receipt")
    atomic_write(initial_delivery_receipt_path(binding), receipt)
    return receipt


def run_delivery_step(argv: list[str], cwd: Path) -> dict[str, Any]:
    try:
        completed = subprocess.run(argv, cwd=cwd, check=False, capture_output=True, timeout=75)
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise PhaseFailure("delivery_step", f"bounded initial delivery step failed: {exc}") from exc
    if completed.returncode:
        detail = (completed.stderr or completed.stdout).decode(errors="replace").strip()
        raise PhaseFailure("delivery_step", detail or "bounded initial delivery step failed")
    value = parse_json(completed.stdout, "initial delivery step")
    if not isinstance(value, dict) or set(value) - {"status", "effect", "reason"}:
        raise PhaseFailure("delivery_step", "initial delivery step returned an invalid result")
    return value


def drive_initial_delivery(binding: dict[str, Any], result: dict[str, Any]) -> dict[str, Any]:
    if result["terminal"]["status"] != "PASS" or not result.get("certificate_path") or not result.get("certificate_digest"):
        raise PhaseFailure("delivery_gate", "only exact semantic PASS may enter delivery")
    identity = delivery_identity(binding, result)
    receipt_path = initial_delivery_receipt_path(binding)
    if receipt_path.exists():
        existing, _ = read_exact(receipt_path)
        validate_schema(existing, "initial-delivery-receipt.v1.schema.json", "initial delivery receipt")
        adopted = adopt_initial_delivery(binding, result, identity)
        if adopted != existing:
            raise PhaseFailure("delivery_conflict", "initial delivery receipt no longer matches Beads")
        return existing
    adopted = adopt_initial_delivery(binding, result, identity)
    if adopted is not None:
        return adopted
    native = identity["native"]
    executables = native["executables"]
    freeze = result["freeze"]
    command = [
        executables["agentops-gc-delivery"]["path"], "step",
        "--root", binding["context"]["delivery"]["evidence_root"],
        "--certificate", str(result["certificate_path"]),
        "--subject-manifest", freeze["delivery_manifest"],
        "--subject-manifest-digest", freeze["delivery_manifest_content_digest"],
        "--native-context", binding["context"]["delivery"]["native_context_path"],
        "--native-context-digest", binding["context"]["delivery"]["native_context_digest"],
        "--semantic-bead", identity["semantic"], "--terminal-ref", identity["terminal_ref"],
        "--rig", native["rig_id"], "--repository", native["repository"], "--remote", native["remote"],
        "--epoch", "1", "--mode", binding["context"]["delivery"]["mode"],
        "--deadline", identity["deadline"], "--prepared-at", binding["context"]["created_at"],
        "--committed-at", binding["context"]["created_at"], "--base-ref", binding["graph"]["base_ref"],
        "--base-oid", binding["graph"]["base_oid"], "--gc-bin", executables["gc"]["path"],
        "--beads-bin", executables["bd"]["path"], "--git-bin", executables["git"]["path"],
        "--gh-bin", executables["gh"]["path"], "--bash-bin", executables["bash"]["path"],
    ]
    for _ in range(2):
        try:
            observed = run_delivery_step(command, Path(binding["context"]["repository_dir"]))
        except PhaseFailure:
            adopted = adopt_initial_delivery(binding, result, identity)
            if adopted is not None:
                return adopted
            raise
        adopted = adopt_initial_delivery(binding, result, identity)
        if adopted is not None:
            return adopted
        if observed.get("status") != "prepared":
            raise PhaseFailure("delivery_step", "initial delivery did not reach prepared or successor_created")
    raise PhaseFailure("delivery_step", "initial delivery exceeded two bounded reducer steps")


def check(args: argparse.Namespace) -> int:
    pointer, _ = read_exact(Path(args.request))
    if pointer.get("kind") == "role":
        role_request, response, artifact, context = verify_role_check(pointer)
        if pointer["phase"] == "mayor":
            prepare_plan_request(role_request, response, artifact, context)
            return 0
        # graph_request rejects non-PASS before admit can make any mutation.
        graph, graph_raw = object_bytes(Path(context["graph_path"]), "program graph")
        graph_request(graph, graph_raw, artifact, Path(context["plan_artifact_path"]).read_bytes())
        admit(SimpleNamespace(
            root=context["root"], graph=context["graph_path"], plan=context["plan_artifact_path"],
            bd_bin=context["bd_bin"], gc_bin=context["gc_bin"], git_bin=context["git_bin"],
            created_at=context["created_at"], intent=context["intent_path"], packet_adapter=context["packet_adapter"],
            factory_check=context["factory_check"], context=str(Path(context["workspace"]) / "build-context.v1.json"),
        ))
        return 0
    if pointer.get("kind") == "packet":
        binding = packet_binding(pointer)
        if pointer["phase"] == "implement":
            try:
                implement_transition(binding)
            except PhaseFailure as failure:
                not_built_terminal(binding, failure)
                raise
            return 0
        try:
            result = validate_transition(binding)
        except PhaseFailure as failure:
            not_proven_terminal(binding, failure)
            release_ready_admissions(binding["receipt"], binding["graph"], binding["context"]["bd_bin"], Path(binding["context"]["root"]))
            return 0
        release_ready_admissions(binding["receipt"], binding["graph"], binding["context"]["bd_bin"], Path(binding["context"]["root"]))
        # PASS-only delivery is deliberately a second, independent loop. Its
        # bounded initial handoff is attached after semantic terminalization.
        if result["terminal"]["status"] == "PASS":
            drive_initial_delivery(binding, result)
        return 0
    raise FeederError("controller check request has an unknown kind")


def graph_request(graph: dict[str, Any], graph_raw: bytes, plan: dict[str, Any], plan_raw: bytes) -> tuple[str, str, str]:
    validate_schema(graph, "program-graph.v2.schema.json", "program graph")
    validate_schema(plan, "plan-review.v1.schema.json", "plan review")
    if graph.get("schema_version") != "program-graph.v2" or not isinstance(graph.get("program_id"), str):
        raise FeederError("program graph is not program-graph.v2")
    validate_executable_graph(graph)
    require_fields(plan, {"schema_version", "program_id", "intent_digest", "graph_digest", "mayor_context_id", "reviewer_context_id", "provider", "verdict", "criteria", "findings"}, "plan review")
    if plan.get("schema_version") != "plan-review.v1" or plan.get("program_id") != graph["program_id"]:
        raise FeederError("plan review does not bind program graph")
    if plan.get("verdict") != "PASS":
        raise FeederError("non-PASS plan review may not mutate graph")
    if any(item.get("result") != "PASS" for item in plan.get("criteria", [])) or any(item.get("severity") == "blocking" for item in plan.get("findings", [])):
        raise FeederError("PASS plan review has a non-PASS criterion or blocking finding")
    if plan.get("mayor_context_id") == plan.get("reviewer_context_id") or plan.get("provider") != "codex":
        raise FeederError("plan review is not fresh Sol/Codex binding evidence")
    program_digest = digest(graph_raw)
    if plan.get("graph_digest") != program_digest:
        raise FeederError("plan review graph digest is stale")
    if graph.get("intent_digest") != plan.get("intent_digest"):
        raise FeederError("plan and graph intent digests conflict")
    return graph["program_id"], program_digest, digest(plan_raw)


def relative_path(value: object) -> bool:
    return isinstance(value, str) and value and not value.startswith(("/", "\\")) and ":" not in value.split("/", 1)[0] and all(part not in {"", ".", ".."} for part in value.split("/"))


def scopes_overlap(left: str, right: str) -> bool:
    return left == right or left.startswith(right + "/") or right.startswith(left + "/")


def validate_executable_graph(graph: dict[str, Any]) -> None:
    """Reject graphs that would make a worker infer a bead contract.

    This is intentionally only deterministic graph hygiene; it does not own a
    lifecycle or substitute another factory for Beads' graph mutation.
    """
    if not SAFE_ID.fullmatch(str(graph.get("program_id", ""))) or not SAFE_ID.fullmatch(str(graph.get("delivery_group_id", ""))):
        raise FeederError("program graph has unsafe program or delivery identity")
    if not all(isinstance(graph.get(key), str) and graph[key].startswith("/") for key in ("repository_dir", "workspace_root", "packet_root")) or not isinstance(graph.get("base_ref"), str) or not graph["base_ref"] or not isinstance(graph.get("base_oid"), str) or len(graph["base_oid"]) != 40:
        raise FeederError("program graph lacks an exact repository/base/workspace binding")
    nodes = graph.get("nodes")
    if not isinstance(nodes, list) or not nodes:
        raise FeederError("program graph has no nodes")
    ids: set[str] = set()
    for node in nodes:
        if not isinstance(node, dict):
            raise FeederError("program graph node is not an object")
        identifier = node.get("id")
        required = {"id", "title", "intent", "acceptance", "non_goals", "subject", "first_check", "depends_on", "write_scope"}
        if not isinstance(identifier, str) or not SAFE_ID.fullmatch(identifier) or identifier in ids or not required <= set(node):
            raise FeederError("program graph node lacks an executable unique contract")
        ids.add(identifier)
        if not all(isinstance(node[key], str) and node[key] for key in ("title", "intent", "first_check")):
            raise FeederError("program graph node has empty executable text")
        if not isinstance(node["acceptance"], list) or not node["acceptance"] or not all(isinstance(item, str) and item for item in node["acceptance"]):
            raise FeederError("program graph node has no exact acceptance")
        if not isinstance(node["non_goals"], list) or not all(isinstance(item, str) and item for item in node["non_goals"]):
            raise FeederError("program graph node has malformed non-goals")
        subject = node["subject"]
        if not isinstance(subject, dict) or set(subject) != {"includes", "excludes"} or not isinstance(subject["includes"], list) or not subject["includes"] or not isinstance(subject["excludes"], list) or not all(relative_path(item) for item in subject["includes"] + subject["excludes"]):
            raise FeederError("program graph node has malformed subject scope")
        scopes = node["write_scope"]
        if not isinstance(scopes, list) or not scopes or not all(relative_path(item) for item in scopes):
            raise FeederError("program graph node has malformed write scope")
        companions = node.get("generated_companions")
        if not isinstance(companions, list) or not all(relative_path(item) for item in companions):
            raise FeederError("program graph node has malformed generated companions")
        if node.get("intent_digest") != graph.get("intent_digest"):
            raise FeederError("program graph node intent digest conflicts with program intent")
        if node.get("role") != "implementation" or node.get("fallback") != {"allowed": False, "used": False, "reason": None}:
            raise FeederError("program graph node violates implementation/no-fallback policy")
        if node.get("model") == "terra" and (node.get("provider"), node.get("reasoning")) != ("codex", "high"):
            raise FeederError("Terra node does not bind Codex/high")
        if node.get("model") == "opus" and (node.get("provider"), node.get("reasoning")) != ("claude", "medium"):
            raise FeederError("Opus node does not bind Claude/medium")
        if any(not any(scopes_overlap(companion, scope) for scope in scopes) for companion in companions):
            raise FeederError("generated companion escapes declared write scope")
    by_id = {node["id"]: node for node in nodes}
    for node in nodes:
        dependencies = node["depends_on"]
        if not isinstance(dependencies, list) or len(set(dependencies)) != len(dependencies) or any(not isinstance(item, str) or item not in ids or item == node["id"] for item in dependencies):
            raise FeederError("program graph has invalid dependencies")
    # A DFS proves the graph is acyclic.  Overlapping write scopes must be
    # serialized by a dependency path in either direction.
    visiting: set[str] = set(); visited: set[str] = set()
    def reaches(start: str, want: str) -> bool:
        if start == want:
            return True
        if start in visiting:
            return False
        visiting.add(start)
        answer = any(reaches(parent, want) for parent in by_id[start]["depends_on"])
        visiting.remove(start)
        return answer
    for identifier in ids:
        if reaches(identifier, identifier) and by_id[identifier]["depends_on"]:
            # Explicitly run a conventional cycle walk instead of relying on
            # self reachability (which is true for every starting node).
            def visit(item: str) -> None:
                if item in visiting:
                    raise FeederError("program graph dependency cycle")
                if item in visited:
                    return
                visiting.add(item)
                for parent in by_id[item]["depends_on"]:
                    visit(parent)
                visiting.remove(item); visited.add(item)
            visit(identifier)
    if graph.get("max_parallel") > len(nodes):
        raise FeederError("program max_parallel exceeds admitted node count")
    for index, left in enumerate(nodes):
        for right in nodes[index + 1:]:
            if any(scopes_overlap(a, b) for a in left["write_scope"] for b in right["write_scope"]):
                if not (reaches(left["id"], right["id"]) or reaches(right["id"], left["id"])):
                    raise FeederError("parallel program graph nodes overlap write scope")


def run_checked(argv: list[str], cwd: Path) -> bytes:
    completed = subprocess.run(argv, cwd=cwd, check=False, capture_output=True)
    if completed.returncode:
        raise FeederError((completed.stderr or completed.stdout).decode(errors="replace").strip() or "native command failed")
    return completed.stdout


def load_python(path: str, name: str) -> Any:
    spec = importlib.util.spec_from_file_location(name, path)
    if spec is None or spec.loader is None:
        raise FeederError(f"cannot load selected adapter: {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def node_packet_paths(graph: dict[str, Any], node: dict[str, Any], program_digest: str) -> tuple[Path, Path, Path, Path]:
    workspace = Path(graph["workspace_root"]) / program_digest / node["id"]
    packet_id = f"{graph['program_id']}-{node['id']}"
    evidence = workspace / ".gc" / "agentops" / packet_id
    return workspace, evidence / "implement-envelope.v1.json", evidence / "validate-envelope.v1.json", evidence


def prepare_node_workspace(git_bin: str, graph: dict[str, Any], node: dict[str, Any], program_digest: str,
                           intent_source: str, packet_adapter: str, admission_receipt: Path) -> tuple[str, str, str]:
    """Allocate/adopt one clean linked worktree and only its valid implement packet."""
    workspace, implement_packet, validate_packet, evidence = node_packet_paths(graph, node, program_digest)
    if workspace.exists():
        head = run_checked([git_bin, "-C", str(workspace), "rev-parse", "HEAD"], workspace).decode().strip()
        if head != graph["base_oid"]:
            raise FeederError("existing candidate workspace has the wrong base identity")
    else:
        workspace.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        run_checked([git_bin, "-C", graph["repository_dir"], "worktree", "add", "--detach", str(workspace), graph["base_oid"]], Path(graph["repository_dir"]))
    packet_module = load_python(packet_adapter, "agentops_gc_packet_for_admission")
    try:
        base = packet_module.authorize_linked_worktree(workspace, Path(graph["repository_dir"]))
    except Exception as exc:
        raise FeederError(f"candidate worktree is not an isolated clean linked checkout: {exc}") from exc
    if base != graph["base_oid"]:
        raise FeederError("candidate linked worktree base differs from reviewed graph")
    evidence.mkdir(mode=0o700, parents=True, exist_ok=True)
    intent_target = evidence / "intent-source"
    intent_raw = Path(intent_source).read_bytes()
    if digest(intent_raw) != graph["intent_digest"]:
        raise FeederError("admission intent snapshot is stale")
    atomic_bytes(intent_target, intent_raw, "node intent snapshot")
    packet = {
        "schema_version": "gc-execution-envelope.v1", "packet_id": f"{graph['program_id']}-{node['id']}",
        "role": "implement", "provider": node["provider"], "intent_source": str(intent_target),
        "intent_digest": graph["intent_digest"], "workspace": str(workspace), "subject": node["subject"],
        "write_scope": sorted(set(node["write_scope"] + node["generated_companions"])),
        "evidence_dir": str(evidence), "result_path": str(evidence / "agent-response.v1.json"),
        "factory_admission_receipt": str(admission_receipt), "factory_node_id": node["id"],
    }
    atomic_write(implement_packet, packet)
    try:
        validated, paths = packet_module.validate_envelope(implement_packet)
    except Exception as exc:
        raise FeederError(f"prepared implement packet violates selected adapter: {exc}") from exc
    baseline_path = evidence / "runtime-baseline-manifest.json"
    if baseline_path.exists():
        before = packet_module.load_object(baseline_path, "runtime baseline manifest")
        verify_path = evidence / ".runtime-baseline-verify.json"
        after = packet_module.build_manifest(validated, paths, verify_path)
        try:
            if before != after:
                raise FeederError("candidate changed after its admitted baseline")
        finally:
            verify_path.unlink(missing_ok=True)
    else:
        packet_module.build_manifest(validated, paths, baseline_path)
    if validate_packet.exists():
        raise FeederError("Validate envelope exists before implementation evidence")
    return str(workspace), str(implement_packet), str(validate_packet)


def compile_graph_apply_plan(graph: dict[str, Any], program_digest: str) -> dict[str, Any]:
    """Compile the executable graph contract into Beads v1.1.0's exact plan."""
    graph_digest = program_digest  # hash of the exact, independently reviewed graph bytes.
    nodes: list[dict[str, Any]] = []
    edges: list[dict[str, str]] = []
    for item in graph["nodes"]:
        description = "\n".join((
            "Intent:\n" + item["intent"],
            "Acceptance:\n" + "\n".join("- " + value for value in item["acceptance"]),
            "Non-goals:\n" + "\n".join("- " + value for value in item["non_goals"]),
            "Subject includes:\n" + "\n".join("- " + value for value in item["subject"]["includes"]),
            "Subject excludes:\n" + "\n".join("- " + value for value in item["subject"]["excludes"]),
            "First deterministic check:\n" + item["first_check"],
        ))
        workspace, packet_path, _, _ = node_packet_paths(graph, item, program_digest)
        packet = str(packet_path)
        nodes.append({"key": item["id"], "title": item["title"], "type": "task", "description": description, "priority": 2,
                      "labels": ["agentops", "semantic"], "metadata": {"gc.kind": "semantic", "gc.program_id": graph["program_id"], "gc.program_digest": program_digest, "gc.graph_digest": graph_digest, "gc.intent_digest": item["intent_digest"], "gc.node_id": item["id"], "gc.role": item["role"], "gc.model": item["model"], "gc.reasoning": item["reasoning"], "gc.provider": item["provider"], "gc.repository_dir": graph["repository_dir"], "gc.base_ref": graph["base_ref"], "gc.base_oid": graph["base_oid"], "gc.candidate_workspace": str(workspace), "gc.packet_path": packet}})
        edges.extend({"from_key": item["id"], "to_key": parent, "type": "blocks"} for parent in item["depends_on"])
    return {"commit_message": "agentops: admit " + graph["program_id"], "nodes": nodes, "edges": edges}


def bead_metadata(row: dict[str, Any], label: str) -> dict[str, Any]:
    value = row.get("metadata")
    if isinstance(value, str):
        try:
            value = json.loads(value)
        except json.JSONDecodeError as exc:
            raise FeederError(f"{label} has invalid metadata JSON") from exc
    if not isinstance(value, dict):
        raise FeederError(f"{label} has no object metadata")
    return value


def show_bead(bd_bin: str, root: Path, identifier: str, label: str) -> dict[str, Any]:
    value = parse_json(run_checked([bd_bin, "show", identifier, "--json"], root), f"bd show {label}")
    row = nested_record(value, identifier)
    if row is None:
        raise FeederError(f"bd show did not return exact {label} {identifier}")
    return row


def exact_graph_record(bd_bin: str, root: Path, identifier: str, expected: dict[str, Any],
                       expected_dependencies: set[str]) -> None:
    """Re-read every durable GraphApply field and edge, not just our stamps."""
    row = show_bead(bd_bin, root, identifier, f"semantic node {expected['key']}")
    actual_labels = row.get("labels", [])
    if not isinstance(actual_labels, list) or sorted(actual_labels) != sorted(expected["labels"]):
        raise FeederError(f"semantic node {expected['key']} labels differ from reviewed graph")
    exact = {
        "title": expected["title"], "description": expected["description"],
        "issue_type": expected["type"], "priority": expected["priority"],
    }
    if any(row.get(key) != value for key, value in exact.items()):
        raise FeederError(f"semantic node {expected['key']} fields differ from reviewed graph")
    metadata = bead_metadata(row, f"semantic node {expected['key']}")
    extras = set(metadata) - set(expected["metadata"])
    if any(metadata.get(key) != value for key, value in expected["metadata"].items()):
        raise FeederError(f"semantic node {expected['key']} metadata differs from reviewed graph")
    if extras:
        allowed = {"gc33.terminal", "gc33.terminal_receipt"}
        if extras != allowed:
            raise FeederError(f"semantic node {expected['key']} has unreviewed metadata")
        encoded, receipt = metadata["gc33.terminal"], metadata["gc33.terminal_receipt"]
        if not isinstance(encoded, str) or not isinstance(receipt, str):
            raise FeederError(f"semantic node {expected['key']} has malformed terminal metadata")
        try:
            summary = json.loads(encoded)
        except json.JSONDecodeError as exc:
            raise FeederError(f"semantic node {expected['key']} has invalid terminal summary") from exc
        try:
            verified_terminal_material(
                root, identifier, expected["metadata"]["gc.program_digest"], expected["key"],
                expected["metadata"]["gc.intent_digest"], receipt, summary,
            )
        except PhaseFailure as exc:
            raise FeederError(f"semantic node {expected['key']} terminal metadata differs from its receipt") from exc
        if row.get("status") != "closed":
            raise FeederError(f"semantic node {expected['key']} terminal metadata differs from its receipt")
    dependencies = row.get("dependencies", [])
    if not isinstance(dependencies, list):
        raise FeederError(f"semantic node {expected['key']} dependencies are malformed")
    actual_dependencies: set[tuple[str, str]] = set()
    for item in dependencies:
        if not isinstance(item, dict) or not isinstance(item.get("id"), str) or not isinstance(item.get("dependency_type"), str):
            raise FeederError(f"semantic node {expected['key']} has malformed dependency evidence")
        pair = (item["id"], item["dependency_type"])
        if pair in actual_dependencies:
            raise FeederError(f"semantic node {expected['key']} has duplicate dependency evidence")
        actual_dependencies.add(pair)
    wanted = {(item, "blocks") for item in expected_dependencies}
    if actual_dependencies != wanted:
        raise FeederError(f"semantic node {expected['key']} edges differ from reviewed graph")


def adopted_graph_ids(bd_bin: str, root: Path, graph: dict[str, Any], program_digest: str) -> dict[str, str] | None:
    """Return the complete exact symbolic ID map or fail closed on partiality."""
    rows = all_beads(bd_bin, root)
    matches: dict[str, list[str]] = {node["id"]: [] for node in graph["nodes"]}
    for row in rows:
        if not isinstance(row, dict):
            raise FeederError("bd list returned a non-object record")
        metadata = row.get("metadata")
        if not isinstance(metadata, dict):
            continue
        if metadata.get("gc.program_digest") != program_digest:
            continue
        if metadata.get("gc.graph_digest") != program_digest:
            raise FeederError("existing program graph has conflicting graph digest")
        node_id = metadata.get("gc.node_id")
        if node_id not in matches:
            raise FeederError("existing program graph has an unknown semantic node")
        identifier = row.get("id")
        if not isinstance(identifier, str) or not identifier:
            raise FeederError("existing program graph has no bead id")
        matches[node_id].append(identifier)
    present = {key: values for key, values in matches.items() if values}
    if not present:
        return None
    if set(present) != set(matches):
        raise FeederError("existing program graph is partial")
    if any(len(values) != 1 for values in matches.values()):
        raise FeederError("existing program graph has duplicate semantic nodes")
    identifiers = {key: values[0] for key, values in matches.items()}
    compiled = compile_graph_apply_plan(graph, program_digest)
    expected_nodes = {node["key"]: node for node in compiled["nodes"]}
    dependencies = {node["id"]: {identifiers[parent] for parent in node["depends_on"]} for node in graph["nodes"]}
    for key, identifier in identifiers.items():
        exact_graph_record(bd_bin, root, identifier, expected_nodes[key], dependencies[key])
    return identifiers


def experiment_binding(records: dict[str, dict[str, Any]], steps: dict[str, str], node: dict[str, Any],
                       work_dir: str, implement_packet: str, validate_packet: str, routes: dict[str, str]) -> None:
    implement = records["agentops-experiment.implement.iteration.1"]
    validate = records["agentops-experiment.validate.iteration.1"]
    implement_metadata = bead_metadata(implement, "effective Implement attempt")
    validate_metadata = bead_metadata(validate, "effective Validate attempt")
    implement_target = routes["implementer"] if node["model"] == "terra" else routes["implementer_claude"]
    if (implement_metadata.get("gc.run_target") != implement_target
            or implement_metadata.get("gc.routed_to") != implement_target
            or implement_metadata.get("work_dir") != work_dir
            or implement_metadata.get("gc.packet_path") != implement_packet
            or implement_packet not in str(implement.get("description", ""))):
        raise FeederError(f"effective Implement attempt does not bind node {node['id']}")
    if (validate_metadata.get("gc.run_target") != routes["validator"]
            or validate_metadata.get("gc.routed_to") != routes["validator"]
            or validate_metadata.get("work_dir") != work_dir
            or validate_metadata.get("gc.packet_path") != validate_packet
            or validate_packet not in str(validate.get("description", ""))):
        raise FeederError(f"effective Validate attempt does not bind node {node['id']}")
    if records["agentops-experiment.admission"].get("assignee") not in (None, ""):
        raise FeederError(f"experiment admission for {node['id']} is not inert")
    if set(steps) != EXPERIMENT_STEP_REFS:
        raise FeederError(f"experiment workflow for {node['id']} is partial")


def blocking_dependencies(bd_bin: str, root: Path, identifier: str) -> set[str]:
    value = parse_json(run_checked([bd_bin, "dep", "list", identifier, "--type", "blocks", "--json"], root), "bd dep list")
    if not isinstance(value, list):
        raise FeederError("bd dep list did not return an array")
    dependencies: set[str] = set()
    for row in value:
        if not isinstance(row, dict):
            raise FeederError("bd dep list returned a non-object record")
        target = row.get("id", row.get("depends_on_id"))
        dependency_type = row.get("dependency_type", row.get("type"))
        if not isinstance(target, str) or dependency_type != "blocks" or target in dependencies:
            raise FeederError("bd dep list returned malformed or duplicate blocking evidence")
        dependencies.add(target)
    return dependencies


def wire_predecessors(bd_bin: str, root: Path, admission_id: str, predecessors: set[str]) -> None:
    actual = blocking_dependencies(bd_bin, root, admission_id)
    extras = actual - predecessors
    if extras:
        raise FeederError("experiment admission has unreviewed blocking dependencies")
    for predecessor in sorted(predecessors - actual):
        run_checked([bd_bin, "dep", "add", admission_id, predecessor, "--type", "blocks", "--json"], root)
    if blocking_dependencies(bd_bin, root, admission_id) != predecessors:
        raise FeederError("experiment admission predecessor wiring is incomplete")


def reconcile_admission(receipt: dict[str, Any], args: argparse.Namespace, graph: dict[str, Any],
                        program_digest: str, plan_digest: str) -> None:
    validate_schema(receipt, "graph-admission-receipt.v1.schema.json", "graph admission receipt")
    context, _ = read_exact(Path(args.context).resolve(strict=True))
    validate_build_context(context)
    exact = {
        "program_id": graph["program_id"], "program_digest": program_digest,
        "plan_digest": plan_digest, "graph_digest": program_digest,
        "context_path": str(Path(args.context).resolve(strict=True)),
        "context_digest": digest(Path(args.context).read_bytes()),
        "intent_path": str(Path(args.intent).resolve(strict=True)),
        "intent_digest": graph["intent_digest"],
        "packet_adapter": str(Path(args.packet_adapter).resolve(strict=True)),
        "factory_check": str(Path(args.factory_check).resolve(strict=True)),
    }
    if any(receipt.get(key) != value for key, value in exact.items()):
        raise FeederError("existing graph admission receipt conflicts with exact admission inputs")
    identifiers = adopted_graph_ids(args.bd_bin, Path(args.root).resolve(), graph, program_digest)
    if identifiers != receipt["graph_bead_ids"]:
        raise FeederError("existing graph admission receipt no longer binds the exact semantic graph")
    node_by_id = {node["id"]: node for node in graph["nodes"]}
    if set(receipt["nodes"]) != set(node_by_id):
        raise FeederError("existing graph admission receipt has the wrong node set")
    for node_id, recorded in receipt["nodes"].items():
        node = node_by_id[node_id]
        if recorded["semantic_bead_id"] != identifiers[node_id]:
            raise FeederError("recorded semantic bead identity changed")
        packet, packet_raw = read_exact(Path(recorded["implement_packet"]))
        if digest(packet_raw) != recorded["implement_packet_digest"] or packet.get("workspace") != recorded["work_dir"]:
            raise FeederError("recorded implement packet identity changed")
        steps, records = workflow_rows(args.bd_bin, Path(args.root).resolve(), recorded["workflow_root_id"], "agentops-experiment", EXPERIMENT_STEP_REF_MAP)
        if steps != recorded["workflow_steps"]:
            raise FeederError("recorded experiment workflow identity changed")
        experiment_binding(records, steps, node, recorded["work_dir"], recorded["implement_packet"], recorded["validate_packet"], context["routes"])
        predecessors = {identifiers[parent] for parent in node["depends_on"]}
        if sorted(predecessors) != recorded["predecessor_semantic_bead_ids"]:
            raise FeederError("recorded predecessor identity changed")
        if blocking_dependencies(args.bd_bin, Path(args.root).resolve(), steps["agentops-experiment.admission"]) != predecessors:
            raise FeederError("recorded experiment predecessor wiring changed")


def release_initial_admissions(receipt: dict[str, Any], bd_bin: str, root: Path) -> None:
    for identifier in receipt["initial_admission_ids"]:
        row = show_bead(bd_bin, root, identifier, "initial experiment admission")
        close_inert_step(bd_bin, root, row, "AgentOps checked semantic experiment admitted")


def admit(args: argparse.Namespace) -> int:
    root = Path(args.root).resolve(strict=True)
    graph, graph_raw = object_bytes(Path(args.graph), "program graph")
    plan, plan_raw = object_bytes(Path(args.plan), "plan review")
    program_id, program_digest, plan_digest = graph_request(graph, graph_raw, plan, plan_raw)
    context, context_raw = read_exact(Path(args.context).resolve(strict=True))
    validate_build_context(context)
    exact_context = {
        "root": str(root), "program_id": program_id, "graph_path": str(Path(args.graph).resolve(strict=True)),
        "plan_artifact_path": str(Path(args.plan).resolve(strict=True)), "intent_path": str(Path(args.intent).resolve(strict=True)),
        "intent_digest": graph["intent_digest"], "repository_dir": graph["repository_dir"],
        "base_ref": graph["base_ref"], "base_oid": graph["base_oid"], "candidate_workspace_root": graph["workspace_root"],
        "packet_root": graph["packet_root"], "bd_bin": str(Path(args.bd_bin).resolve(strict=True)),
        "gc_bin": str(Path(args.gc_bin).resolve(strict=True)), "git_bin": str(Path(args.git_bin).resolve(strict=True)),
        "packet_adapter": str(Path(args.packet_adapter).resolve(strict=True)),
        "factory_check": str(Path(args.factory_check).resolve(strict=True)),
    }
    if any(context.get(key) != value for key, value in exact_context.items()):
        raise FeederError("admission inputs differ from the immutable checked build context")
    if digest(Path(args.intent).read_bytes()) != graph["intent_digest"]:
        raise FeederError("admission intent bytes differ from reviewed graph")
    receipt_path = factory_evidence_root(root) / "graph-admissions" / f"{program_digest}.json"
    if receipt_path.exists():
        receipt, _ = read_exact(receipt_path)
        reconcile_admission(receipt, args, graph, program_digest, plan_digest)
        release_initial_admissions(receipt, args.bd_bin, root)
        print(json.dumps(receipt, sort_keys=True))
        return 0
    # Compile the exact pinned Beads GraphApplyPlan; the only mutation is its
    # single atomic create --graph call, and every semantic node is stamped.
    plan_value = compile_graph_apply_plan(graph, program_digest)
    plan_path = factory_evidence_root(root) / "graph-plans" / f"{program_digest}.json"
    atomic_write(plan_path, plan_value)
    graph_bead_ids = adopted_graph_ids(args.bd_bin, root, graph, program_digest)
    if graph_bead_ids is None:
        output = run_checked([args.bd_bin, "create", "--graph", str(plan_path), "--json"], root)
        try:
            created = json.loads(output)
        except json.JSONDecodeError as exc:
            raise FeederError("bd create --graph did not return JSON") from exc
        graph_bead_ids = created.get("ids")
        expected_keys = {node["id"] for node in graph["nodes"]}
        if not isinstance(graph_bead_ids, dict) or set(graph_bead_ids) != expected_keys or not all(isinstance(value, str) and value for value in graph_bead_ids.values()):
            raise FeederError("bd create --graph did not report the exact symbolic ids map")
        # Re-read proves a returned success was a complete exact graph rather
        # than trusting a transport reply.  A restart performs this adoption
        # read before it can make a second create call.
        adopted = adopted_graph_ids(args.bd_bin, root, graph, program_digest)
        if adopted != graph_bead_ids:
            raise FeederError("created graph cannot be re-read as exact program graph")
    # Attachments and predecessor wiring are explicit post-admission mutations.
    # Every one is durable and re-read before any source admission is released.
    admitted_nodes: dict[str, dict[str, Any]] = {}
    for node in graph["nodes"]:
        work_dir, implement_packet, validate_packet = prepare_node_workspace(
            args.git_bin, graph, node, program_digest, args.intent, args.packet_adapter, receipt_path,
        )
        routes = context["routes"]
        target = routes["implementer"] if node["model"] == "terra" else routes["implementer_claude"]
        cooked = parse_json(run_checked([
            args.gc_bin, "formula", "cook", "agentops-experiment", "--attach", graph_bead_ids[node["id"]],
            "--var", "work_dir=" + work_dir, "--var", "implement_packet=" + implement_packet,
            "--var", "validate_packet=" + validate_packet, "--var", "implement_target=" + target,
            "--var", "validate_target=" + routes["validator"],
            "--var", "packet_adapter=" + str(Path(args.packet_adapter).resolve(strict=True)),
            "--var", "factory_check=" + str(Path(args.factory_check).resolve(strict=True)), "--json",
        ], root), f"gc formula cook agentops-experiment {node['id']}")
        if (not isinstance(cooked, dict) or cooked.get("ok") is not True
                or cooked.get("formula") != "agentops-experiment"
                or cooked.get("attach_bead_id") != graph_bead_ids[node["id"]]):
            raise FeederError(f"gc formula cook returned a conflicting experiment for {node['id']}")
        workflow_root = cooked.get("workflow_root_id") or cooked.get("root_id")
        if not isinstance(workflow_root, str) or not workflow_root:
            raise FeederError(f"gc formula cook returned no workflow root for {node['id']}")
        steps, records = workflow_rows(args.bd_bin, root, workflow_root, "agentops-experiment", EXPERIMENT_STEP_REF_MAP)
        experiment_binding(records, steps, node, work_dir, implement_packet, validate_packet, routes)
        packet_raw = Path(implement_packet).read_bytes()
        admitted_nodes[node["id"]] = {
            "semantic_bead_id": graph_bead_ids[node["id"]], "workflow_root_id": workflow_root,
            "workflow_steps": steps, "work_dir": work_dir, "implement_packet": implement_packet,
            "implement_packet_digest": digest(packet_raw), "validate_packet": validate_packet,
            "predecessor_semantic_bead_ids": sorted(graph_bead_ids[parent] for parent in node["depends_on"]),
        }
    for node in graph["nodes"]:
        admitted = admitted_nodes[node["id"]]
        wire_predecessors(
            args.bd_bin, root, admitted["workflow_steps"]["agentops-experiment.admission"],
            set(admitted["predecessor_semantic_bead_ids"]),
        )
    initial_admissions = sorted(
        admitted_nodes[node["id"]]["workflow_steps"]["agentops-experiment.admission"]
        for node in graph["nodes"] if not node["depends_on"]
    )[:graph["max_parallel"]]
    receipt = {
        "schema_version": "graph-admission-receipt.v1", "program_id": program_id,
        "program_digest": program_digest, "plan_digest": plan_digest,
        "graph_digest": program_digest, "graph_bead_ids": graph_bead_ids,
        "context_path": str(Path(args.context).resolve(strict=True)), "context_digest": digest(context_raw),
        "intent_path": str(Path(args.intent).resolve(strict=True)), "intent_digest": graph["intent_digest"],
        "packet_adapter": str(Path(args.packet_adapter).resolve(strict=True)),
        "factory_check": str(Path(args.factory_check).resolve(strict=True)),
        "attached_formulas": ["agentops-experiment"], "nodes": admitted_nodes,
        "initial_admission_ids": initial_admissions, "created_at": args.created_at,
    }
    validate_schema(receipt, "graph-admission-receipt.v1.schema.json", "graph admission receipt")
    atomic_write(receipt_path, receipt)
    reconcile_admission(receipt, args, graph, program_digest, plan_digest)
    release_initial_admissions(receipt, args.bd_bin, root)
    print(json.dumps(receipt, sort_keys=True))
    return 0


def parser() -> argparse.ArgumentParser:
    value = argparse.ArgumentParser(description=__doc__)
    commands = value.add_subparsers(dest="command", required=True)
    item = commands.add_parser("start")
    item.add_argument("--root", required=True)
    item.add_argument("--source-bead", required=True)
    item.add_argument("--intent", required=True)
    item.add_argument("--repository", required=True)
    item.add_argument("--base-ref", required=True)
    item.add_argument("--max-parallel", type=int, default=2)
    item.add_argument("--bd-bin", required=True)
    item.add_argument("--gc-bin", required=True)
    item.add_argument("--git-bin", required=True)
    item.add_argument("--role-adapter", required=True)
    item.add_argument("--packet-adapter", required=True)
    item.add_argument("--factory-check", required=True)
    item.add_argument("--rig-id", required=True)
    item.add_argument("--delivery-native-context")
    item.add_argument("--delivery-native-context-digest")
    item.add_argument("--delivery-root")
    item.add_argument("--delivery-mode", choices=("auto", "manual"))
    item.add_argument("--delivery-deadline-seconds", type=int)
    item.add_argument("--created-at", required=True)
    item.set_defaults(fn=start)
    item = commands.add_parser("check")
    item.add_argument("--request", required=True)
    item.set_defaults(fn=check)
    item = commands.add_parser("admit")
    item.add_argument("--root", required=True)
    item.add_argument("--graph", required=True)
    item.add_argument("--plan", required=True)
    item.add_argument("--bd-bin", required=True)
    item.add_argument("--gc-bin", required=True)
    item.add_argument("--git-bin", required=True)
    item.add_argument("--intent", required=True)
    item.add_argument("--packet-adapter", required=True)
    item.add_argument("--factory-check", required=True)
    item.add_argument("--context", required=True)
    item.add_argument("--created-at", required=True)
    item.set_defaults(fn=admit)
    return value


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    try:
        return args.fn(args)
    except (FeederError, OSError) as exc:
        print(f"agentops factory feeder: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
