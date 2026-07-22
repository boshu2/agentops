#!/usr/bin/env python3
"""Small role-v2 inspection/emission and routing-doctor adapter.

This file intentionally owns no bead graph, worktree, Git, forge, retry, or
delivery lifecycle.  Those deterministic transitions belong to the optional Go
delivery reducer and native Gas City/Beads formulas.
"""
from __future__ import annotations

import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import tempfile
import tomllib
from typing import Any

from jsonschema import Draft202012Validator, FormatChecker


PACK_ROOT = Path(__file__).resolve().parents[2]
SCHEMA_ROOT = PACK_ROOT / "assets" / "schemas"
ROLE_TEMPLATES = {
    "mayor": PACK_ROOT / "agents" / "mayor" / "agent.toml",
    "plan": PACK_ROOT / "agents" / "plan-reviewer" / "agent.toml",
    "ambiguity_advice": PACK_ROOT / "agents" / "refiner" / "agent.toml",
    "implementation": PACK_ROOT.parent / "agentops-executor" / "agents" / "implementer" / "agent.toml",
    "validation": PACK_ROOT.parent / "agentops-executor" / "agents" / "validator" / "agent.toml",
}
COMPOSED_ROLE_SOURCES = {
    "mayor": PACK_ROOT / "agents" / "mayor" / "agent.toml",
    "plan-reviewer": PACK_ROOT / "agents" / "plan-reviewer" / "agent.toml",
    "refiner": PACK_ROOT / "agents" / "refiner" / "agent.toml",
    "implementer": PACK_ROOT.parent / "agentops-executor" / "agents" / "implementer" / "agent.toml",
    "implementer-claude": PACK_ROOT.parent / "agentops-executor" / "agents" / "implementer-claude" / "agent.toml",
    "validator": PACK_ROOT.parent / "agentops-executor" / "agents" / "validator" / "agent.toml",
}
ROLE_POLICY = {
    "mayor": {"model": "fable", "reasoning": "adaptive", "provider": "claude", "fallback": {"allowed": False, "used": False, "reason": None}},
    "plan": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": {"allowed": False, "used": False, "reason": None}},
    "ambiguity_advice": {"model": "fable", "reasoning": "adaptive", "provider": "claude", "fallback": {"allowed": False, "used": False, "reason": None}},
    "implementation": {"model": "terra", "reasoning": "high", "provider": "codex", "fallback": {"allowed": False, "used": False, "reason": None}},
    "validation": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": {"allowed": False, "used": False, "reason": None}},
}
EXPECTED_LAUNCH_IDENTITY = {
    "mayor": {"model": "claude-fable-5", "reasoning": "adaptive", "provider": "claude", "effort": None},
    "plan": {"model": "gpt-5.6-sol", "reasoning": "high", "provider": "codex", "effort": "high"},
    "ambiguity_advice": {"model": "claude-fable-5", "reasoning": "adaptive", "provider": "claude", "effort": None},
    "implementation": {"model": "gpt-5.6-terra", "reasoning": "high", "provider": "codex", "effort": "high"},
    "validation": {"model": "gpt-5.6-sol", "reasoning": "high", "provider": "codex", "effort": "high"},
}


class RoleAdapterError(ValueError):
    pass


def digest_file(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load_object(path: Path, label: str) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise RoleAdapterError(f"invalid {label}: {exc}") from exc
    if not isinstance(value, dict):
        raise RoleAdapterError(f"{label} must be a JSON object")
    return value


def validate_schema(value: dict[str, Any], path: Path, label: str) -> None:
    schema = load_object(path, f"{label} schema")
    try:
        Draft202012Validator(schema, format_checker=FormatChecker()).validate(value)
    except Exception as exc:
        raise RoleAdapterError(f"{label} does not satisfy {path.name}: {exc}") from exc


def canonical(value: dict[str, Any]) -> bytes:
    return json.dumps(value, ensure_ascii=True, separators=(",", ":"), sort_keys=True).encode() + b"\n"


def write_once(path: Path, value: dict[str, Any], label: str) -> None:
    """Create one canonical immutable object, or adopt byte-identical bytes."""
    raw = canonical(value)
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    if path.exists():
        if path.is_symlink() or path.read_bytes() != raw:
            raise RoleAdapterError(f"existing {label} conflicts: {path}")
        return
    handle, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        os.fchmod(handle, 0o600)
        with os.fdopen(handle, "wb") as output:
            output.write(raw)
            output.flush()
            os.fsync(output.fileno())
        try:
            os.link(temporary, path)
        except FileExistsError:
            if path.is_symlink() or path.read_bytes() != raw:
                raise RoleAdapterError(f"racing {label} conflicts: {path}")
    finally:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass


def absolute_path(value: object, label: str, exists: bool = False) -> Path:
    if not isinstance(value, str) or not value:
        raise RoleAdapterError(f"{label} must be a nonempty path")
    path = Path(value).expanduser().resolve(strict=False)
    if exists and not path.is_file():
        raise RoleAdapterError(f"{label} does not exist")
    return path


def within(path: Path, root: Path) -> bool:
    try:
        path.resolve(strict=False).relative_to(root.resolve(strict=False))
        return True
    except ValueError:
        return False


def requested_role_policy(role: str, provider: str | None = None) -> dict[str, Any]:
    if role not in ROLE_POLICY:
        raise RoleAdapterError(f"role is not enabled for role-v2: {role!r}")
    policy = copy.deepcopy(ROLE_POLICY[role])
    if provider is not None and policy["provider"] != provider:
        raise RoleAdapterError("role provider is not the pinned policy")
    return policy


def run_gc_json(arguments: list[str]) -> Any:
    binary = os.environ.get("GC_BIN", "gc")
    city = os.environ.get("GC_CITY_PATH") or os.environ.get("GC_CITY")
    command = [binary]
    if city:
        command += ["--city", city]
    completed = subprocess.run(command + arguments + ["--json"], check=False, capture_output=True, text=True, timeout=30)
    if completed.returncode:
        raise RoleAdapterError((completed.stderr or completed.stdout).strip() or "GC identity query failed")
    try:
        return json.loads(completed.stdout)
    except json.JSONDecodeError as exc:
        raise RoleAdapterError("GC identity query did not return JSON") from exc


def nested_record(value: Any, identifier: str) -> dict[str, Any] | None:
    if isinstance(value, list):
        matches = [found for item in value if (found := nested_record(item, identifier)) is not None]
        if len(matches) > 1:
            raise RoleAdapterError(f"GC returned duplicate records for {identifier}")
        return matches[0] if matches else None
    if not isinstance(value, dict):
        return None
    if str(value.get("id", "")) == identifier:
        return value
    for key in ("issue", "bead", "issues", "beads", "result", "data"):
        if key in value and (found := nested_record(value[key], identifier)) is not None:
            return found
    return None


def command_identity(command: object, provider: str, requested_reasoning: str) -> tuple[str, str | None, str]:
    if not isinstance(command, str) or not command.strip():
        raise RoleAdapterError("durable GC session has no launch command")
    try:
        tokens = shlex.split(command)
    except ValueError as exc:
        raise RoleAdapterError(f"durable GC launch command cannot be parsed: {exc}") from exc
    models = [tokens[index + 1] for index, token in enumerate(tokens[:-1]) if token in {"--model", "-m"}]
    if len(models) != 1 or not models[0].strip():
        raise RoleAdapterError("durable GC launch command must contain one explicit model")
    if provider == "codex":
        efforts = [
            candidate.partition("=")[2]
            for candidate in (tokens[index + 1] for index, token in enumerate(tokens[:-1]) if token == "-c")
            if candidate.startswith("model_reasoning_effort=")
        ]
        if len(efforts) != 1 or not efforts[0]:
            raise RoleAdapterError("durable Codex launch command must contain one explicit reasoning effort")
        return models[0], efforts[0], efforts[0]
    if provider == "claude":
        efforts = [tokens[index + 1] for index, token in enumerate(tokens[:-1]) if token == "--effort"]
        if requested_reasoning == "adaptive":
            if efforts:
                raise RoleAdapterError("Fable adaptive launch must not invent a Claude effort flag")
            return models[0], None, "adaptive"
        if len(efforts) != 1 or not efforts[0]:
            raise RoleAdapterError("durable Claude launch command must contain one explicit effort")
        return models[0], efforts[0], efforts[0]
    raise RoleAdapterError(f"unsupported GC launch provider: {provider}")


def expected_launch_identity(request: dict[str, Any]) -> dict[str, Any]:
    if request["role"] == "implementation" and request["requested"]["model"] == "opus":
        return {"model": "claude-opus-4-8", "reasoning": "medium", "provider": "claude", "effort": "medium", "fallback": request["requested"]["fallback"]}
    return {**EXPECTED_LAUNCH_IDENTITY[request["role"]], "fallback": request["requested"]["fallback"]}


def validate_actual_identity(request: dict[str, Any], session: dict[str, Any]) -> dict[str, Any]:
    actual = {
        "model": session.get("model"), "reasoning": session.get("reasoning"),
        "provider": session.get("provider"), "effort": session.get("effort"),
        "fallback": session.get("fallback"),
    }
    if actual != expected_launch_identity(request):
        raise RoleAdapterError(f"actual GC launch identity violates requested role policy: {actual!r}")
    return actual


def durable_runtime_identity(request: dict[str, Any], bead_id: str, *, context_id: str | None = None,
                             rig: str | None = None, verify_process_env: bool = True) -> tuple[str, dict[str, Any]]:
    context_id = (context_id or os.environ.get("GC_SESSION_ID", "")).strip()
    if not context_id or context_id == request.get("prior_context_id"):
        raise RoleAdapterError("role emission lacks a fresh session identity")
    sessions_value = run_gc_json(["session", "list", "--state", "all"])
    sessions = sessions_value.get("sessions") if isinstance(sessions_value, dict) else None
    live_matches = [item for item in sessions or [] if isinstance(item, dict) and str(item.get("id", "")) == context_id]
    if len(live_matches) != 1:
        raise RoleAdapterError(f"expected one live GC session {context_id}, found {len(live_matches)}")
    live = live_matches[0]
    durable = nested_record(run_gc_json(["bd", "show", context_id]), context_id)
    if durable is None or durable.get("issue_type") != "session":
        raise RoleAdapterError(f"durable GC session bead is missing for {context_id}")
    metadata = durable.get("metadata")
    if not isinstance(metadata, dict):
        raise RoleAdapterError("durable GC session has no metadata")
    provider = str(live.get("provider") or metadata.get("provider") or "")
    template = str(live.get("template") or metadata.get("template") or "")
    session_name = str(live.get("session_name") or metadata.get("session_name") or "")
    state = str(live.get("state") or metadata.get("state") or "")
    if not all((provider, template, session_name, state)):
        raise RoleAdapterError("GC runtime session identity is incomplete")
    model, effort, reasoning = command_identity(metadata.get("command"), provider, request["requested"]["reasoning"])
    observed_model = live.get("model")
    if isinstance(observed_model, str) and observed_model.strip() and observed_model.strip() != model:
        raise RoleAdapterError("live GC model disagrees with durable launch command")
    if verify_process_env and any((os.environ.get("GC_PROVIDER", "").strip() != provider,
                                   os.environ.get("GC_TEMPLATE", "").strip() != template,
                                   os.environ.get("GC_SESSION_NAME", "").strip() != session_name)):
        raise RoleAdapterError("role process environment disagrees with durable GC session")
    claimed_args = ["bd"]
    rig = (rig if rig is not None else os.environ.get("GC_RIG", "")).strip()
    if rig:
        claimed_args += ["--rig", rig]
    claimed_args += ["show", bead_id]
    claimed = nested_record(run_gc_json(claimed_args), bead_id)
    claimed_meta = claimed.get("metadata") if isinstance(claimed, dict) else None
    if (claimed is None or claimed.get("assignee") != session_name or not isinstance(claimed_meta, dict)
            or claimed_meta.get("gc.routed_to") != template
            or Path(str(claimed_meta.get("work_dir", ""))).resolve(strict=False) != Path(request["workspace"]).resolve(strict=False)):
        raise RoleAdapterError("claimed Formula bead is not bound to this session, template, and workspace")
    session = {"id": context_id, "provider": provider, "template": template, "session_name": session_name,
               "state": state, "model": model, "effort": effort, "reasoning": reasoning,
               "fallback": {"allowed": False, "used": False, "reason": None}}
    return context_id, validate_actual_identity(request, session)


def role_artifact_contract_v2(role: str) -> dict[str, Any]:
    schemas = {
        "mayor": "program-graph.v2.schema.json",
        "plan": "plan-review.v1.schema.json",
        "ambiguity_advice": "ambiguity-advice.v1.schema.json",
        "implementation": "gc-execution-envelope.v1.schema.json",
        "validation": "verdict.v2.schema.json",
    }
    if role not in schemas:
        raise RoleAdapterError(f"role is not enabled for role-v2: {role!r}")
    path = SCHEMA_ROOT / schemas[role]
    if role == "implementation":
        path = PACK_ROOT.parent / "agentops-executor" / "assets" / "schemas" / schemas[role]
    if role == "validation":
        path = PACK_ROOT.parents[1] / "schemas" / schemas[role]
    schema = load_object(path, "artifact schema")
    properties, required = schema.get("properties"), schema.get("required")
    if not isinstance(properties, dict) or not isinstance(required, list):
        raise RoleAdapterError("artifact schema has no strict object contract")
    version = properties.get("schema_version", {}).get("const")
    if not isinstance(version, str):
        raise RoleAdapterError("artifact schema has no pinned version")
    return {"schema_path": str(path.resolve()), "schema_version": version,
            "required_top_level": required, "allowed_top_level": sorted(properties),
            "nonbinding_no_byte": role == "ambiguity_advice"}


def validate_request(path: Path) -> tuple[dict[str, Any], dict[str, Path]]:
    request = load_object(path, "role request")
    validate_schema(request, SCHEMA_ROOT / "factory-role-request.v2.schema.json", "role request")
    fields = {"schema_version", "request_id", "program_id", "semantic_bead_id", "workspace", "intent_source", "intent_digest", "subject_path", "subject_digest", "evidence_refs", "prior_context_id", "role", "requested", "artifact_path", "result_path"}
    if set(request) != fields or request.get("schema_version") != "factory-role-request.v2":
        raise RoleAdapterError("role request is not the exact v2 contract")
    role = request.get("role")
    if not isinstance(role, str):
        raise RoleAdapterError("role request has no role")
    expected = requested_role_policy(role)
    if request.get("requested") != expected:
        raise RoleAdapterError("role request policy is not exact")
    workspace = absolute_path(request.get("workspace"), "workspace")
    intent, subject = absolute_path(request.get("intent_source"), "intent_source", True), absolute_path(request.get("subject_path"), "subject_path", True)
    artifact, result = absolute_path(request.get("artifact_path"), "artifact_path"), absolute_path(request.get("result_path"), "result_path")
    if not all(within(item, workspace) for item in (intent, subject, artifact, result)):
        raise RoleAdapterError("role request path escapes workspace")
    if request.get("intent_digest") != digest_file(intent) or request.get("subject_digest") != digest_file(subject):
        raise RoleAdapterError("role request contains stale subject identity")
    evidence = request.get("evidence_refs")
    if not isinstance(evidence, list) or not evidence:
        raise RoleAdapterError("role request has no evidence")
    for reference in evidence:
        if not isinstance(reference, dict) or set(reference) != {"path", "digest"}:
            raise RoleAdapterError("role request evidence is malformed")
        evidence_path = absolute_path(reference["path"], "evidence path", True)
        if not within(evidence_path, workspace) or reference["digest"] != digest_file(evidence_path):
            raise RoleAdapterError("role request evidence identity is stale")
    return request, {"request": path, "workspace": workspace, "artifact": artifact, "result": result}


def command_inspect_role_v2(args: argparse.Namespace) -> int:
    request, paths = validate_request(absolute_path(args.request, "request", True))
    rendered = dict(request)
    rendered["request_path"] = str(paths["request"])
    rendered["request_digest"] = digest_file(paths["request"])
    rendered["artifact_contract"] = role_artifact_contract_v2(request["role"])
    print(json.dumps(rendered, sort_keys=True))
    return 0


def command_emit_role_v2(args: argparse.Namespace) -> int:
    request, paths = validate_request(absolute_path(args.request, "request", True))
    artifact = absolute_path(args.artifact, "artifact", True)
    if artifact != paths["artifact"]:
        raise RoleAdapterError("emitted artifact differs from requested artifact path")
    artifact_value = load_object(artifact, "role artifact")
    contract = role_artifact_contract_v2(request["role"])
    validate_schema(artifact_value, Path(contract["schema_path"]), "role artifact")
    if request["role"] == "mayor" and (artifact_value.get("program_id") != request["program_id"] or artifact_value.get("intent_digest") != request["intent_digest"]):
        raise RoleAdapterError("mayor artifact does not bind the exact program and intent")
    if request["role"] == "plan" and artifact_value.get("program_id") != request["program_id"]:
        raise RoleAdapterError("plan artifact does not bind the exact program")
    if request["role"] == "ambiguity_advice" and (artifact_value.get("nonbinding") is not True or artifact_value.get("mutates_artifacts") is not False):
        raise RoleAdapterError("ambiguity advice must be nonbinding and no-byte")
    if args.bead != request["semantic_bead_id"]:
        raise RoleAdapterError("emitted Formula bead differs from requested semantic bead")
    context_id, actual = durable_runtime_identity(request, args.bead)
    response = {"schema_version": "factory-role-response.v2", "request_id": request["request_id"],
                "request_digest": digest_file(paths["request"]), "role": request["role"],
                "semantic_bead_id": request["semantic_bead_id"], "session_context_id": context_id,
                "requested": request["requested"], "actual": actual,
                "artifact_path": str(artifact), "artifact_digest": digest_file(artifact)}
    validate_schema(response, SCHEMA_ROOT / "factory-role-response.v2.schema.json", "role response")
    write_once(paths["result"], response, "role response")
    # Formula v2 checks receive only the checked attempt's GC_ARTIFACT_DIR.
    # Project a canonical pointer there so the controller can bind the exact
    # role request/response without scanning another bead or maintaining state.
    artifact_dir = os.environ.get("GC_ARTIFACT_DIR")
    if artifact_dir:
        directory = absolute_path(artifact_dir, "GC_ARTIFACT_DIR")
        directory.mkdir(mode=0o700, parents=True, exist_ok=True)
        check_request = {
            "schema_version": "agentops-factory-check.v1", "kind": "role", "phase": request["role"],
            "role": request["role"], "checked_bead_id": args.bead,
            "semantic_bead_id": request["semantic_bead_id"], "session_context_id": context_id,
            "rig": os.environ.get("GC_RIG", "").strip() or None,
            "request_path": str(paths["request"]), "result_path": str(paths["result"]),
            "request_digest": digest_file(paths["request"]), "result_digest": digest_file(paths["result"]),
            "artifact_path": str(artifact), "artifact_digest": digest_file(artifact),
        }
        destination = directory / "agentops-factory-check-request.json"
        write_once(destination, check_request, "factory check request")
    print(json.dumps(response, sort_keys=True))
    return 0


def command_verify_role_v2(args: argparse.Namespace) -> int:
    request, paths = validate_request(absolute_path(args.request, "request", True))
    response_path = absolute_path(args.response, "response", True)
    response = load_object(response_path, "role response")
    validate_schema(response, SCHEMA_ROOT / "factory-role-response.v2.schema.json", "role response")
    expected = {
        "request_id": request["request_id"], "request_digest": digest_file(paths["request"]),
        "role": request["role"], "semantic_bead_id": args.bead,
        "session_context_id": args.session,
    }
    if any(response.get(key) != value for key, value in expected.items()):
        raise RoleAdapterError("role response does not bind the exact checked request, bead, and session")
    artifact = absolute_path(response.get("artifact_path"), "artifact", True)
    if artifact != paths["artifact"] or response.get("artifact_digest") != digest_file(artifact):
        raise RoleAdapterError("role response artifact identity is stale")
    artifact_value = load_object(artifact, "role artifact")
    validate_schema(artifact_value, Path(role_artifact_contract_v2(request["role"])["schema_path"]), "role artifact")
    _, actual = durable_runtime_identity(request, args.bead, context_id=args.session, rig=args.rig or "", verify_process_env=False)
    if response.get("requested") != request["requested"] or response.get("actual") != actual:
        raise RoleAdapterError("role response runtime identity is not the current durable GC launch")
    result = {"ok": True, "request_digest": expected["request_digest"], "response_digest": digest_file(response_path),
              "artifact_digest": response["artifact_digest"], "session_context_id": args.session,
              "actual": actual}
    print(json.dumps(result, sort_keys=True))
    return 0


def composed_route_doctor(overrides: dict[str, dict[str, Any]] | None = None,
                          delivery_route: str = "agentops.delivery") -> dict[str, Any]:
    inventory: dict[str, dict[str, Any]] = {}
    problems: list[str] = []
    for role, path in COMPOSED_ROLE_SOURCES.items():
        try:
            config = tomllib.loads(path.read_text(encoding="utf-8"))
        except (OSError, tomllib.TOMLDecodeError) as exc:
            problems.append(f"cannot read {role}: {exc}")
            continue
        provider, scope = config.get("provider"), config.get("scope")
        if provider not in {"codex", "claude"} or scope not in {"city", "rig"}:
            problems.append(f"invalid provider or scope: {role}")
        # This is a static binding-qualified inventory for role inspection.
        # Formula cook receives sealed concrete rig targets from the feeder.
        qualified = f"agentops.{role}"
        inventory[role] = {"qualified_name": qualified, "provider": provider, "scope": scope,
                           "work_query": config.get("work_query"), "sling_query": config.get("sling_query")}
    if overrides:
        for role, changes in overrides.items():
            if role not in inventory:
                problems.append(f"unknown composed role: {role}")
            else:
                inventory[role].update(changes)
    if set(inventory) != set(COMPOSED_ROLE_SOURCES):
        problems.append("incomplete composed role inventory")
    routes = [item["qualified_name"] for item in inventory.values()]
    if len(set(routes)) != len(routes) or delivery_route in routes:
        problems.append("delivery selector overlaps a model route")
    for role, item in inventory.items():
        if item["work_query"] is not None or item["sling_query"] is not None:
            problems.append(f"broadened model selector: {role}")
    return {"ok": not problems, "reason": "; ".join(problems) if problems else None,
            "inventory": inventory, "routes": {name: item["qualified_name"] for name, item in inventory.items()}}


def command_doctor() -> int:
    result = composed_route_doctor()
    if not result["ok"]:
        print(f"agentops role adapter doctor: {result['reason']}")
        return 2
    print("agentops role adapter: role-v2 and static routing inventory only")
    return 0


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(description=__doc__)
    commands = result.add_subparsers(dest="command", required=True)
    inspect = commands.add_parser("inspect-role-v2"); inspect.add_argument("--request", required=True)
    emit = commands.add_parser("emit-role-v2"); emit.add_argument("--request", required=True); emit.add_argument("--artifact", required=True); emit.add_argument("--bead", required=True)
    verify = commands.add_parser("verify-role-v2"); verify.add_argument("--request", required=True); verify.add_argument("--response", required=True); verify.add_argument("--bead", required=True); verify.add_argument("--session", required=True); verify.add_argument("--rig")
    commands.add_parser("doctor")
    return result


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    try:
        if args.command == "inspect-role-v2": return command_inspect_role_v2(args)
        if args.command == "emit-role-v2": return command_emit_role_v2(args)
        if args.command == "verify-role-v2": return command_verify_role_v2(args)
        return command_doctor()
    except RoleAdapterError as exc:
        print(json.dumps({"ok": False, "error": str(exc)}, sort_keys=True))
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
