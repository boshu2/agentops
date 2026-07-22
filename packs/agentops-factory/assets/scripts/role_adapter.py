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
import tomllib
from typing import Any


PACK_ROOT = Path(__file__).resolve().parents[2]
SCHEMA_ROOT = PACK_ROOT / "assets" / "schemas"
ROLE_TEMPLATES = {
    "mayor": PACK_ROOT / "agents" / "mayor" / "agent.toml",
    "plan": PACK_ROOT / "agents" / "plan-reviewer" / "agent.toml",
    "ambiguity_advice": PACK_ROOT / "agents" / "refiner" / "agent.toml",
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


def role_artifact_contract_v2(role: str) -> dict[str, Any]:
    schemas = {
        "mayor": "program-graph.v2.schema.json",
        "plan": "plan-review.v1.schema.json",
        "ambiguity_advice": "ambiguity-advice.v1.schema.json",
    }
    if role not in schemas:
        raise RoleAdapterError(f"role is not enabled for role-v2: {role!r}")
    path = SCHEMA_ROOT / schemas[role]
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
    context_id = os.environ.get("GC_SESSION_ID")
    if not context_id or context_id == request.get("prior_context_id"):
        raise RoleAdapterError("role emission lacks a fresh session identity")
    artifact_value = load_object(artifact, "role artifact")
    contract = role_artifact_contract_v2(request["role"])
    if (artifact_value.get("schema_version") != contract["schema_version"]
            or set(artifact_value) != set(contract["allowed_top_level"])
            or not set(contract["required_top_level"]).issubset(artifact_value)):
        raise RoleAdapterError("role artifact schema does not match exact request role")
    if request["role"] == "mayor" and (artifact_value.get("program_id") != request["program_id"] or artifact_value.get("intent_digest") != request["intent_digest"]):
        raise RoleAdapterError("mayor artifact does not bind the exact program and intent")
    if request["role"] == "plan" and artifact_value.get("program_id") != request["program_id"]:
        raise RoleAdapterError("plan artifact does not bind the exact program")
    if request["role"] == "ambiguity_advice" and (artifact_value.get("nonbinding") is not True or artifact_value.get("mutates_artifacts") is not False):
        raise RoleAdapterError("ambiguity advice must be nonbinding and no-byte")
    response = {"schema_version": "factory-role-response.v2", "request_id": request["request_id"],
                "request_digest": digest_file(paths["request"]), "role": request["role"],
                "semantic_bead_id": request["semantic_bead_id"], "session_context_id": context_id,
                "requested": request["requested"], "actual": {**request["requested"], "effort": request["requested"]["reasoning"]},
                "artifact_path": str(artifact), "artifact_digest": digest_file(artifact)}
    paths["result"].write_text(json.dumps(response, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(response, sort_keys=True))
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
        qualified = f"agentops.{role}" if scope == "city" else f"rig/agentops.{role}"
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
    emit = commands.add_parser("emit-role-v2"); emit.add_argument("--request", required=True); emit.add_argument("--artifact", required=True)
    commands.add_parser("doctor")
    return result


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    try:
        if args.command == "inspect-role-v2": return command_inspect_role_v2(args)
        if args.command == "emit-role-v2": return command_emit_role_v2(args)
        return command_doctor()
    except RoleAdapterError as exc:
        print(json.dumps({"ok": False, "error": str(exc)}, sort_keys=True))
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
