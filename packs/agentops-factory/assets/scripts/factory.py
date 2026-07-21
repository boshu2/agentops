#!/usr/bin/env python3
"""Bead-native Fenced Steward reducer and Refinery for Gas City.

Packs define reusable roles and commands. Beads are the durable units of work
and the only factory lifecycle ledger. JSON artifacts are immutable proposals or
evidence referenced by beads; they never replace bead status, dependencies, or
metadata.
"""

from __future__ import annotations

import argparse
import copy
from concurrent.futures import ThreadPoolExecutor, as_completed
import contextlib
from datetime import datetime
import fcntl
from functools import lru_cache
import hashlib
import importlib.util
import json
import os
from pathlib import Path, PurePosixPath
import re
import secrets
import shlex
import shutil
import subprocess
import sys
import tempfile
import time
import tomllib
from typing import Any
from urllib.parse import quote

from jsonschema import Draft202012Validator, FormatChecker


sys.dont_write_bytecode = True

# jsonschema's optional RFC 3339 dependency is not installed in every supported
# local toolchain.  Register the standard-library equivalent on the real
# FormatChecker so a missing extra cannot silently turn off date-time checks.
PAYLOAD_FORMAT_CHECKER = FormatChecker()


@PAYLOAD_FORMAT_CHECKER.checks("date-time")
def _rfc3339_date_time(value: object) -> bool:
    if not isinstance(value, str):
        return True
    if not re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})", value):
        return False
    try:
        parsed = datetime.fromisoformat(value[:-1] + "+00:00" if value.endswith("Z") else value)
    except ValueError:
        return False
    return parsed.tzinfo is not None

PACK_ROOT = Path(__file__).resolve().parents[2]
SCHEMA_ROOT = PACK_ROOT / "assets" / "schemas"
EXECUTOR_ADAPTER = PACK_ROOT.parent / "agentops-executor" / "assets" / "scripts" / "packet.py"
ID_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$")
DIGEST_RE = re.compile(r"^[0-9a-f]{64}$")
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
PROVIDERS = {"codex", "claude"}
VERDICTS = {"PASS", "FAIL", "NOT_PROVEN"}
DELIVERY_MODES = {"pr", "qualify"}
FACTORY_GIT_NAME = "AgentOps Gas City Factory"
FACTORY_GIT_EMAIL = "agentops-factory@localhost.invalid"
# Dynamic rig reconciliation and a busy managed Dolt control plane can make a
# legitimate GC/Beads read exceed the ordinary interactive threshold. Keep the
# operation bounded while matching the executor adapter's production floor.
CONTROL_PLANE_TIMEOUT_SECONDS = 120
FACTORY_ROLE_MODELS = {
    ("implement", "codex"): ("gpt-5.6-terra", "gpt-5.6-terra"),
    ("implement", "claude"): ("opus-4.8", "claude-opus-4-8"),
    ("validate", "codex"): ("gpt-5.6-sol", "gpt-5.6-sol"),
    ("validate", "claude"): ("opus-4.8", "claude-opus-4-8"),
    ("mayor", "codex"): ("gpt-5.6-sol", "gpt-5.6-sol"),
    ("mayor", "claude"): ("opus-4.8", "claude-opus-4-8"),
    ("rescope", "codex"): ("gpt-5.6-sol", "gpt-5.6-sol"),
    ("rescope", "claude"): ("opus-4.8", "claude-opus-4-8"),
    ("plan-review", "codex"): ("gpt-5.6-sol", "gpt-5.6-sol"),
    ("plan-review", "claude"): ("opus-4.8", "claude-opus-4-8"),
    ("refiner", "codex"): ("gpt-5.6-sol", "gpt-5.6-sol"),
    ("refiner", "claude"): ("opus-4.8", "claude-opus-4-8"),
}
# Locators identify canonical pack declarations only. Provider, model, and
# effort are derived at runtime from those TOMLs and deploy/gc/city.toml.
ROLE_TEMPLATES = {
    "mayor": PACK_ROOT / "agents" / "mayor" / "agent.toml",
    "plan": PACK_ROOT / "agents" / "plan-reviewer" / "agent.toml",
    "ambiguity_advice": PACK_ROOT / "agents" / "refiner" / "agent.toml",
    "implementation": PACK_ROOT.parent / "agentops-executor" / "agents" / "implementer" / "agent.toml",
    "implementation_overflow": PACK_ROOT.parent / "agentops-executor" / "agents" / "implementer-claude" / "agent.toml",
    "validation": PACK_ROOT.parent / "agentops-executor" / "agents" / "validator" / "agent.toml",
}


def requested_role_policy(role: str, provider: str) -> dict[str, Any]:
    """Read the abstract policy aliases from the versioned request schema."""
    schema_role = "implementation" if role == "implementation_overflow" else role
    schema = load_object(SCHEMA_ROOT / "factory-role-request.v2.schema.json", "factory role request schema")
    for clause in schema.get("allOf", []):
        selector = clause.get("if", {}).get("properties", {}).get("role", {})
        matches = selector.get("const") == schema_role or schema_role in selector.get("enum", [])
        requested_spec = clause.get("then", {}).get("properties", {}).get("requested", {})
        requested_refs = [requested_spec["$ref"]] if isinstance(requested_spec.get("$ref"), str) else [item["$ref"] for item in requested_spec.get("anyOf", []) if isinstance(item.get("$ref"), str)]
        if not matches:
            continue
        for requested_ref in requested_refs:
            definition = schema["$defs"][requested_ref.rsplit("/", 1)[-1]]
            properties = definition["allOf"][-1]["properties"]
            if properties["provider"]["const"] == provider:
                return {
                    "provider": provider,
                    "model": properties["model"]["const"],
                    "reasoning": properties["reasoning"]["const"],
                    "fallback": {"allowed": False, "used": False, "reason": None},
                }
    raise FactoryError("invalid_role_policy", f"no request policy exists for {role}")
ROLE_REQUEST_KEYS = {
    "schema_version", "request_id", "role", "provider", "program_id",
    "workspace", "intent_source", "intent_digest", "repository", "base_branch",
    "base_sha", "subject_path", "subject_digest", "mayor_context_id",
    "artifact_path", "result_path",
}
ROLE_REQUEST_V2_KEYS = {
    "schema_version", "request_id", "program_id", "semantic_bead_id",
    "workspace", "intent_source", "intent_digest", "subject_path",
    "subject_digest", "evidence_refs", "prior_context_id", "role", "requested",
    "artifact_path", "result_path",
}
ROLE_RESPONSE_V2_KEYS = {
    "schema_version", "request_id", "request_digest", "role",
    "semantic_bead_id", "session_context_id", "requested", "actual",
    "artifact_path", "artifact_digest",
}
V2_ROUTABLE_ROLES = {"mayor", "plan", "ambiguity_advice"}
V2_ROLE_TEMPLATES = {
    "mayor": "mayor",
    "plan": "plan-reviewer",
    "ambiguity_advice": "refiner",
}
NODE_KEYS = {
    "id", "title", "intent", "acceptance", "non_goals", "depends_on",
    "write_scope", "generated_scope", "subject", "first_check", "provider",
    "validator_provider", "execution_role", "worker_model_policy",
    "validator_model_policy", "risk", "supersedes",
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
    path.parent.mkdir(parents=True, exist_ok=True)
    mode = path.stat().st_mode & 0o777 if path.exists() else 0o644
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        os.fchmod(fd, mode)
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


def require_provider(value: Any, label: str) -> str:
    provider = require_string(value, label)
    if provider not in PROVIDERS:
        raise FactoryError("invalid_contract", f"{label} must be one of {sorted(PROVIDERS)}")
    return provider


def opposite_provider(provider: str) -> str:
    selected = require_provider(provider, "provider")
    return "claude" if selected == "codex" else "codex"


def lifecycle_agent_name(role: str, provider: str) -> str:
    selected = require_provider(provider, f"{role} provider")
    physical = {"mayor": "mayor", "plan-review": "plan-reviewer", "refiner": "refiner"}
    if role not in physical:
        raise FactoryError("invalid_contract", f"unsupported lifecycle role: {role}")
    configured = tomllib.loads((PACK_ROOT / "agents" / physical[role] / "agent.toml").read_text(encoding="utf-8"))
    if configured.get("provider") != selected:
        raise FactoryError("unroutable_role", f"{role} is only composed for {configured.get('provider')}")
    return physical[role]


def lifecycle_target(role: str, provider: str, rig: str, binding: str) -> str:
    local = lifecycle_agent_name(role, provider)
    return f"{binding}.{local}" if role == "mayor" else f"{rig}/{binding}.{local}"


def attest_role_runtime(role: str, requested: dict[str, Any], actual: dict[str, Any],
                        context_id: str, fallback_observed: bool) -> dict[str, Any]:
    """Fail closed unless a resolved 3.3 launch exactly matches its role policy.

    This adapter is intentionally pure so composition tests can exercise it
    without starting GC, tmux, or a provider process.
    """
    template = ROLE_TEMPLATES.get(role)
    if template is None:
        raise FactoryError("unroutable_role", f"role is not enabled: {role}")
    config = tomllib.loads(template.read_text(encoding="utf-8"))
    defaults = config.get("option_defaults", {})
    provider = config.get("provider")
    expected_requested = requested_role_policy(role, provider)
    if requested != expected_requested:
        raise FactoryError("requested_policy_mismatch", f"{role} requested policy is not exact")
    city_path = PACK_ROOT.parents[1] / "deploy" / "gc" / "city.toml"
    city = tomllib.loads(city_path.read_text(encoding="utf-8"))
    provider_config = city["providers"][provider]
    choices = {entry["key"]: entry["choices"] for entry in provider_config["options_schema"]}

    def launch_value(key: str, option: Any) -> Any:
        try:
            selected = next(choice for choice in choices[key] if choice["value"] == option)
        except (KeyError, StopIteration) as exc:
            raise FactoryError("runtime_identity_missing", f"{key} option has no declared launch choice") from exc
        args = selected.get("flag_args", [])
        if key == "model":
            try:
                return args[args.index("--model") + 1]
            except (ValueError, IndexError) as exc:
                raise FactoryError("runtime_identity_missing", "model option has no --model launch value") from exc
        return option if args else None

    expected_actual = {
        "provider": provider,
        "model": launch_value("model", defaults.get("model")),
        "reasoning": expected_requested["reasoning"],
        "effort": launch_value("effort", defaults.get("effort")),
        "fallback": expected_requested["fallback"],
    }
    if actual != expected_actual:
        raise FactoryError("runtime_identity_mismatch", f"{role} resolved launch is not exact")
    if not isinstance(context_id, str) or not context_id.strip():
        raise FactoryError("runtime_identity_missing", "session context identity is required")
    if fallback_observed:
        raise FactoryError("not_proven", "fallback was observed")
    return {
        "role": role,
        "requested": requested,
        "actual": actual,
        "context_id": context_id,
        "fallback_observed": False,
    }


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
    portable = raw.replace("\\", "/")
    if re.match(r"^[A-Za-z]:($|/)", portable):
        raise FactoryError("invalid_scope", f"{label} uses a drive path: {raw}")
    path = PurePosixPath(portable)
    if path.is_absolute() or ".." in path.parts:
        raise FactoryError("invalid_scope", f"{label} escapes the repository: {raw}")
    normalized = path.as_posix()
    while normalized.startswith("./"):
        normalized = normalized[2:]
    if normalized in {"", "."}:
        raise FactoryError("invalid_scope", f"{label} normalizes to an empty path")
    return normalized


def canonical_v2_scope_paths(values: Any, label: str, *, nonempty: bool) -> list[str]:
    raw_paths = require_list(values, label, nonempty=nonempty)
    normalized = [normalize_path(item, f"{label}[{index}]") for index, item in enumerate(raw_paths)]
    if len(set(normalized)) != len(normalized):
        raise FactoryError("invalid_scope", f"{label} contains duplicate normalized paths")
    return normalized


# GC33-4 is deliberately a proof kernel.  These helpers consume an immutable
# ready-set snapshot; they do not create Beads, sessions, locks, or a queue.
def _writer_paths(writer: dict[str, Any]) -> set[str]:
    paths = canonical_v2_scope_paths(writer.get("write_scope", []), "writer.write_scope", nonempty=True)
    generated = canonical_v2_scope_paths(writer.get("generated_companions", []), "writer.generated_companions", nonempty=False)
    return set(paths + generated)


def _path_overlaps(left: str, right: str) -> bool:
    return left == right or left.startswith(f"{right}/") or right.startswith(f"{left}/")


def writer_conflicts(left: dict[str, Any], right: dict[str, Any]) -> bool:
    """Return whether two declared candidate envelopes must serialize."""
    left_paths, right_paths = _writer_paths(left), _writer_paths(right)
    if any(_path_overlaps(a, b) for a in left_paths for b in right_paths):
        return True
    left_group = left.get("atomic_group_id", left.get("delivery_group_id"))
    right_group = right.get("atomic_group_id", right.get("delivery_group_id"))
    return isinstance(left_group, str) and bool(left_group) and left_group == right_group


def candidate_identity_receipt(candidate: dict[str, Any]) -> dict[str, str | int]:
    """Validate the independent identity facts a bounded inert runner records."""
    required = ("worktree", "branch", "git_index", "lease_token", "fence_epoch")
    if any(not candidate.get(field) for field in required):
        raise FactoryError("candidate_identity_invalid", "candidate identity is incomplete")
    worktree = absolute_path(candidate["worktree"], "candidate worktree", directory=True)
    branch = output(["git", "-C", str(worktree), "branch", "--show-current"])
    expected_branch = require_string(candidate["branch"], "candidate branch")
    if branch != expected_branch:
        raise FactoryError("candidate_identity_invalid", "candidate branch does not match its live worktree")
    raw_index = output(["git", "-C", str(worktree), "rev-parse", "--git-path", "index"])
    actual_index = Path(raw_index)
    if not actual_index.is_absolute():
        actual_index = (worktree / actual_index).resolve(strict=False)
    git_index = Path(require_string(candidate["git_index"], "candidate git index")).resolve(strict=False)
    if git_index != actual_index or not git_index.is_file():
        raise FactoryError("candidate_identity_invalid", "candidate Git index is not the live worktree index")
    if not isinstance(candidate["fence_epoch"], int) or isinstance(candidate["fence_epoch"], bool) or candidate["fence_epoch"] <= 0:
        raise FactoryError("candidate_identity_invalid", "candidate fence epoch must be positive")
    return {
        "worktree": str(worktree), "branch": expected_branch,
        "git_index": str(git_index), "lease_token": require_string(candidate["lease_token"], "candidate lease token"),
        "fence_epoch": candidate["fence_epoch"],
    }


def lease_receipt_is_current(bead_id: str, receipt: dict[str, Any], current: dict[str, Any]) -> bool:
    """Check one semantic bead's token/generation pair; no global epoch exists."""
    try:
        semantic_bead_id = require_string(bead_id, "semantic bead id")
        return (require_string(receipt.get("semantic_bead_id"), "receipt semantic bead id") == semantic_bead_id
                and require_string(current.get("semantic_bead_id"), "current semantic bead id") == semantic_bead_id
                and require_string(receipt.get("lease_token"), "receipt lease token")
                == require_string(current.get("lease_token"), "current lease token")
                and int(receipt.get("fence_epoch", 0)) == int(current.get("fence_epoch", 0))
                and int(current.get("fence_epoch", 0)) > 0)
    except (FactoryError, TypeError, ValueError):
        return False


def _eligible_writers(ready: list[dict[str, Any]]) -> list[dict[str, Any]]:
    eligible = [item for item in ready if item.get("ready", True) and item.get("dependencies_ready", True)]
    for item in eligible:
        if item.get("bead_class") not in {"product", "delivery_repair"}:
            raise FactoryError("invalid_writer_class", "admission accepts only product or delivery_repair writers")
        require_string(item.get("id"), "writer id")
        _writer_paths(item)
    return sorted(eligible, key=lambda item: (item.get("admitted_at", 0), str(item["id"])))


def admit_semantic_writers(ready: list[dict[str, Any]], *, capacity: int, repair_streak: int) -> dict[str, Any]:
    """Apply the GC33-4 fixed-width fairness policy without side effects."""
    if capacity not in {1, 2, 3, 4}:
        raise FactoryError("invalid_capacity", "writer capacity must be in 1..4")
    if repair_streak < 0:
        raise FactoryError("invalid_repair_streak", "repair streak cannot be negative")
    eligible = _eligible_writers(ready)
    product = [item for item in eligible if item["bead_class"] == "product"]
    repair = [item for item in eligible if item["bead_class"] == "delivery_repair"]
    if capacity == 1:
        preferred = (product + repair) if product and repair and repair_streak >= 2 else (repair + product if repair else product)
    else:
        repair_slots = {2: 1, 3: 2, 4: 3}[capacity]
        product_slots = 1
        preferred = product[:product_slots] + repair[:repair_slots]
        used = {id(item) for item in preferred}
        preferred += [item for item in eligible if id(item) not in used]
    selected: list[dict[str, Any]] = []
    for writer in preferred:
        if len(selected) == capacity:
            break
        if not any(writer_conflicts(writer, admitted) for admitted in selected):
            selected.append(writer)
    result_selected = []
    for writer in selected:
        rendered = dict(writer)
        candidate = writer.get("candidate")
        if candidate is not None:
            rendered["candidate"] = candidate_identity_receipt(candidate)
        else:
            rendered.pop("candidate", None)
        result_selected.append(rendered)
    if len(result_selected) == 2 and all(isinstance(item.get("candidate"), dict) for item in result_selected):
        identities = ("worktree", "branch", "git_index", "lease_token")
        for identity in identities:
            if result_selected[0]["candidate"][identity] == result_selected[1]["candidate"][identity]:
                raise FactoryError("candidate_identity_invalid", f"candidate identities collide: {identity}")
        # Epochs are per-bead lease generations, not a repository-wide clock.
        # Two first leases may therefore both be generation one.  What must
        # never recur is a complete lease identity for the same semantic bead.
        complete = {
            (str(item["id"]), item["candidate"]["lease_token"], item["candidate"]["fence_epoch"])
            for item in result_selected
        }
        if len(complete) != len(result_selected):
            raise FactoryError("candidate_identity_invalid", "duplicate complete per-bead lease identity")
    consecutive = repair_streak + 1 if selected and selected[0]["bead_class"] == "delivery_repair" else 0
    return {"selected": result_selected, "repair_streak": consecutive, "eligible": [item["id"] for item in eligible]}


def _payload(work: dict[str, Any]) -> dict[str, Any]:
    return work.get("payload", work)


@lru_cache(maxsize=2)
def _payload_validator(schema_version: str) -> Draft202012Validator:
    """Compile the checked-in payload schema; it is the sole payload authority."""
    name = {"delivery.v1": "delivery.v1.schema.json", "ambiguity-request.v1": "ambiguity-request.v1.schema.json"}.get(schema_version)
    if name is None:
        raise FactoryError("invalid_payload", f"unsupported payload schema version: {schema_version!r}")
    schema = load_object(SCHEMA_ROOT / name, name)
    try:
        Draft202012Validator.check_schema(schema)
        return Draft202012Validator(schema, format_checker=PAYLOAD_FORMAT_CHECKER)
    except Exception as exc:  # jsonschema reports several concrete schema errors.
        raise FactoryError("invalid_schema", f"cannot compile {name}: {exc}") from exc


def payload_shape_valid(payload: dict[str, Any]) -> bool:
    """Validate payloads with Draft 2020-12 and its registered format checks."""
    if not isinstance(payload, dict):
        return False
    version = payload.get("schema_version")
    if not isinstance(version, str):
        return False
    try:
        return not any(_payload_validator(version).iter_errors(payload))
    except FactoryError:
        return False


def _published_delivery(work: dict[str, Any], delivery_route: str) -> bool:
    payload = _payload(work)
    return (payload.get("schema_version") == "delivery.v1" and payload.get("kind") == "delivery"
            and payload.get("publication") == "published" and work.get("state") == "ready"
            and work.get("assignee") is None and work.get("delivery_route") == delivery_route
            and not work.get("gc.routed_to"))


def _published_ambiguity(work: dict[str, Any]) -> bool:
    payload = _payload(work)
    return (payload.get("schema_version") == "ambiguity-request.v1" and payload.get("kind") == "ambiguity_request"
            and payload.get("publication") == "published")


def stock_model_predicate(work: dict[str, Any], qualified_route: str) -> bool:
    """Pinned v1.3.5 characterization: assigned tiers or exact ready sling."""
    if work.get("state") in {"in_progress", "ready"} and work.get("assignee") == qualified_route:
        return True
    return work.get("state") == "ready" and work.get("assignee") is None and work.get("gc.routed_to") == qualified_route


def refiner_predicate(work: dict[str, Any], refiner_route: str) -> bool:
    return stock_model_predicate(work, refiner_route) and _published_ambiguity(work)


def delivery_predicate(work: dict[str, Any], delivery_route: str) -> bool:
    return _published_delivery(work, delivery_route)


def _delivery_payload() -> dict[str, Any]:
    return {"schema_version": "delivery.v1", "kind": "delivery", "handoff_id": "a" * 64,
            "semantic_bead_id": "semantic", "semantic_terminal_ref": "beads:semantic",
            "admission_certificate_digest": "b" * 64, "delivery_bead_id": "delivery",
            "external_ref": "handoff", "epoch": 1, "predecessor_receipt_digest": None,
            "mode": "auto", "state": "queued", "publication": "non_routable",
            "deadline": "2026-07-22T00:00:00Z", "effect_gate": None, "successor_bead_id": None}


def _ambiguity_payload() -> dict[str, Any]:
    return {"schema_version": "ambiguity-request.v1", "kind": "ambiguity_request",
            "delivery_bead_id": "delivery", "question": "fact?", "facts": ["fact"],
            "deadline": "2026-07-22T00:00:00Z", "publication": "non_routable"}


def _route_spec(spec: Any) -> dict[str, Any]:
    if isinstance(spec, str):
        return {"qualified_name": spec, "effective_work_query": "stock", "effective_sling_query": f"gc.routed_to={spec}"}
    if isinstance(spec, dict):
        route = spec.get("qualified_name", spec.get("route"))
        if not isinstance(route, str):
            raise FactoryError("invalid_route", "model route has no qualified name")
        return {"qualified_name": route,
                "effective_work_query": spec.get("effective_work_query", spec.get("work_query", "stock")),
                "effective_sling_query": spec.get("effective_sling_query", spec.get("sling_query", f"gc.routed_to={route}"))}
    raise FactoryError("invalid_route", "model route is invalid")


def model_consumers(work: dict[str, Any], routes: dict[str, Any]) -> list[str]:
    """Evaluate every effective model work query, never only the Refiner route."""
    consumers: list[str] = []
    for role, raw_spec in routes.items():
        spec = _route_spec(raw_spec)
        if spec["effective_work_query"] == "ready+unassigned":
            matches = work.get("state") == "ready" and work.get("assignee") is None
        elif spec["effective_work_query"] == "stock":
            matches = stock_model_predicate(work, spec["qualified_name"])
        else:
            matches = False
        if matches:
            consumers.append(role)
    return consumers


def consumers_for_work(work: dict[str, Any], *, delivery_route: str, routes: dict[str, Any]) -> list[str]:
    consumers = ["delivery"] if delivery_predicate(work, delivery_route) else []
    consumers.extend(model_consumers(work, routes))
    return consumers


class FakePublicationStore:
    """The sole inert boundary for staged typed work and fake reconciliation."""
    def __init__(self) -> None:
        self.records: dict[str, dict[str, Any]] = {}
        self.provider_records: dict[str, dict[str, list[dict[str, str]]]] = {}
        self.delivery_selections: list[dict[str, str]] = []
        self.claim_expirations: list[dict[str, str]] = []

    def create(self, identity: str, kind: str) -> None:
        if kind not in {"delivery", "ambiguity"} or identity in self.records:
            raise FactoryError("invalid_store_create", "fake work record identity or kind is invalid")
        self.records[identity] = {"kind": kind, "payload": {}, "state": "blocked", "assignee": None,
                                  "claims": {}, "delivery_selection": None, "advice_results": {}}

    def apply(self, identity: str, field: str, value: Any, *, ready: bool | None = None) -> None:
        record = self.records[identity]
        if record.get("delivery_route") or record.get("gc.routed_to") or record["payload"].get("publication") == "published":
            raise FactoryError("construction_visible", "construction writes are forbidden after publication")
        record["payload"][field] = copy.deepcopy(value)
        if ready is not None:
            record["state"] = "ready" if ready else "blocked"

    def snapshot(self, identity: str) -> dict[str, Any]:
        return copy.deepcopy(self.records[identity])

    def publish(self, identity: str, *, delivery_route: str, refiner_route: str) -> dict[str, Any]:
        """Validate first, then make payload/readiness/one selector visible atomically."""
        record = self.records[identity]
        payload = copy.deepcopy(record["payload"])
        expected_version = "delivery.v1" if record["kind"] == "delivery" else "ambiguity-request.v1"
        if payload.get("schema_version") != expected_version or not payload_shape_valid(payload):
            raise FactoryError("premature_publication", "incomplete or invalid construction payload cannot publish")
        published = dict(payload, publication="published")
        if not payload_shape_valid(published):
            raise FactoryError("premature_publication", "published payload violates its checked-in schema")
        final = copy.deepcopy(record)
        final["payload"] = published
        final["state"] = "ready"
        final["assignee"] = None
        if record["kind"] == "delivery":
            final["delivery_route"] = delivery_route
            final.pop("gc.routed_to", None)
        else:
            final["gc.routed_to"] = refiner_route
            final.pop("delivery_route", None)
        self.records[identity] = final
        return self.snapshot(identity)

    def select_delivery(self, identity: str) -> None:
        record = self.records[identity]
        selection = f"fake-delivery:{identity}"
        if record["delivery_selection"] is None:
            record["delivery_selection"] = selection
            self.delivery_selections.append({"record": identity, "selection": selection})

    def acquire_model_claim(self, identity: str, role: str) -> bool:
        record = self.records[identity]
        prior = record["claims"].get(role)
        if prior == "active":
            return False
        record["claims"][role] = "active"
        ledger = self.provider_records.setdefault(role, {"claims": [], "starts": [], "results": []})
        ledger["claims"].append({"record": identity, "claim_identity": f"fake-{role}:{identity}"})
        return True

    def expire_model_claim(self, identity: str, role: str) -> None:
        record = self.records[identity]
        if record["claims"].get(role) == "active":
            record["claims"][role] = "expired"
            self.claim_expirations.append({"record": identity, "role": role})

    def record_nonbinding_advice(self, identity: str, role: str) -> None:
        record = self.records[identity]
        advice_identity = f"fake-{role}:{identity}"
        if advice_identity in record["advice_results"]:
            return
        record["advice_results"][advice_identity] = "complete"
        ledger = self.provider_records.setdefault(role, {"claims": [], "starts": [], "results": []})
        ledger["starts"].append({"record": identity, "start_identity": advice_identity})
        ledger["results"].append({"record": identity, "result_identity": advice_identity, "nonbinding": "true"})


def routing_truth_table(*, refiner_route: str | None = None, delivery_route: str,
                        model_routes: dict[str, Any] | None = None) -> list[dict[str, Any]]:
    inventory = composed_route_doctor()["inventory"] if model_routes is None else model_routes
    routes = inventory if all(isinstance(value, dict) and "qualified_name" in value for value in inventory.values()) else inventory
    refiner_route = refiner_route or _route_spec(routes["refiner"])["qualified_name"]
    store = FakePublicationStore()
    rows: list[dict[str, Any]] = []
    for name, kind, payload in (("delivery", "delivery", _delivery_payload()), ("ambiguity", "ambiguity", _ambiguity_payload())):
        store.create(name, kind)
        for field, value in payload.items():
            store.apply(name, field, value, ready=True)
        staged = store.snapshot(name)
        rows.append({"name": f"staged_{name}", "work": staged,
                     "consumers": consumers_for_work(staged, delivery_route=delivery_route, routes=routes)})
        published = store.publish(name, delivery_route=delivery_route, refiner_route=refiner_route)
        rows.append({"name": f"published_{name}", "work": published,
                     "consumers": consumers_for_work(published, delivery_route=delivery_route, routes=routes)})
    return rows


def validate_routing_truth_table(rows: list[dict[str, Any]]) -> bool:
    required = {"staged_delivery": [], "published_delivery": ["delivery"], "staged_ambiguity": [], "published_ambiguity": ["refiner"]}
    observed = {str(row.get("name")): row.get("consumers") for row in rows}
    if observed != required or any(len(consumers) > 1 for consumers in observed.values() if isinstance(consumers, list)):
        return False
    return True


def publishable_route(work: dict[str, Any], refiner_route: str, delivery_route: str) -> bool:
    """Reject construction rows that expose any composed consumer before publish."""
    staged = _payload(work).get("publication") == "non_routable"
    payload = _payload(work)
    if not payload_shape_valid(payload):
        return False
    if (payload.get("schema_version") == "ambiguity-request.v1" and payload.get("kind") != "ambiguity_request") or (payload.get("schema_version") == "delivery.v1" and payload.get("kind") != "delivery"):
        return False
    if not staged:
        is_delivery = payload.get("schema_version") == "delivery.v1"
        has_delivery_selector = work.get("delivery_route") == delivery_route
        has_model_selector = isinstance(work.get("gc.routed_to"), str) and bool(work.get("gc.routed_to"))
        if (is_delivery and (not has_delivery_selector or has_model_selector)) or (
            not is_delivery and (has_delivery_selector or not has_model_selector)
        ):
            return False
        if not is_delivery and work.get("gc.routed_to") != refiner_route:
            return False
    routes = composed_route_doctor()["inventory"]
    if staged and model_consumers(work, routes):
        return False
    consumers = len(consumers_for_work(work, delivery_route=delivery_route, routes=routes))
    return consumers == 0 if staged else consumers == 1


class _CountingFakeProvider:
    """A capability-free fake provider whose observable facts are store-owned."""
    __slots__ = ("role",)
    def __init__(self, role: str) -> None:
        self.role = role
    def reconcile(self, store: FakePublicationStore, identity: str) -> None:
        if store.acquire_model_claim(identity, self.role):
            store.record_nonbinding_advice(identity, self.role)


def run_inert_routing_harness() -> dict[str, Any]:
    """Store-derived loss/duplicate/expiry proof with no live providers or effects."""
    composition = composed_route_doctor()
    if not composition["ok"]:
        raise FactoryError("composition_failed", composition["reason"] or "invalid composition")
    routes = composition["inventory"]
    store = FakePublicationStore()
    for identity, kind, payload in (("delivery", "delivery", _delivery_payload()), ("ambiguity", "ambiguity", _ambiguity_payload())):
        store.create(identity, kind)
        for field, value in payload.items(): store.apply(identity, field, value)
    refiner_route = routes["refiner"]["qualified_name"]
    store.publish("delivery", delivery_route="agentops.delivery", refiner_route=refiner_route)
    providers = {role: _CountingFakeProvider(role) for role in routes}
    def reconcile() -> None:
        for identity in sorted(store.records):
            record = store.snapshot(identity)
            consumers = consumers_for_work(record, delivery_route="agentops.delivery", routes=routes)
            if consumers == ["delivery"]:
                store.select_delivery(identity)
            elif len(consumers) == 1 and consumers[0] in providers:
                providers[consumers[0]].reconcile(store, identity)
            elif consumers:
                raise FactoryError("routing_firewall_failed", f"multiple or unknown consumers: {consumers}")
    for _event in ("notification_lost", "duplicate"):
        reconcile()
    # A valid ambiguity goes through the identical creation/validation/publication boundary.
    store.publish("ambiguity", delivery_route="agentops.delivery", refiner_route=refiner_route)
    reconcile(); store.expire_model_claim("ambiguity", "refiner"); reconcile()
    provider_records = copy.deepcopy(store.provider_records)
    refiner_records = provider_records.get("refiner", {"claims": [], "starts": [], "results": []})
    routine_claims = [entry for ledger in provider_records.values() for entry in ledger["claims"] if entry["record"] == "delivery"]
    routine_starts = [entry for ledger in provider_records.values() for entry in ledger["starts"] if entry["record"] == "delivery"]
    routine_refiner_starts = [entry for entry in refiner_records["starts"] if entry["record"] == "delivery"]
    unique_claims = {entry["claim_identity"] for ledger in provider_records.values() for entry in ledger["claims"]}
    return {"refiner_starts": len(refiner_records["starts"]), "routine_refiner_starts": len(routine_refiner_starts),
            "model_claims": len(unique_claims), "claim_acquisitions": sum(len(ledger["claims"]) for ledger in provider_records.values()),
            "claim_expirations": len(store.claim_expirations),
            "routine_model_claims": len(routine_claims), "routine_model_starts": len(routine_starts),
            "delivery_selections": len(store.delivery_selections), "ambiguity_starts": len(refiner_records["starts"]),
            "nonbinding_only": len(refiner_records["results"]) == 1 and all(result.get("nonbinding") == "true" for result in refiner_records["results"]),
            "identities": {"delivery": store.delivery_selections[0]["selection"] if store.delivery_selections else None,
                           "ambiguity": refiner_records["starts"][0]["start_identity"] if refiner_records["starts"] else None},
            "providers": provider_records}


_COMPOSED_ROLE_SOURCES = {
    "mayor": PACK_ROOT / "agents" / "mayor" / "agent.toml",
    "plan-reviewer": PACK_ROOT / "agents" / "plan-reviewer" / "agent.toml",
    "refiner": PACK_ROOT / "agents" / "refiner" / "agent.toml",
    "implementer": PACK_ROOT.parent / "agentops-executor" / "agents" / "implementer" / "agent.toml",
    "implementer-claude": PACK_ROOT.parent / "agentops-executor" / "agents" / "implementer-claude" / "agent.toml",
    "validator": PACK_ROOT.parent / "agentops-executor" / "agents" / "validator" / "agent.toml",
}

# Pinned one-rig `gc agent list --json` projection.  The proof deliberately
# keeps this inert fixture beside its source/config composer; GC33-10 owns a
# live clean-city comparison.
_PINNED_INERT_AGENT_FIXTURE = {
    "mayor": ("claude", "city"),
    "plan-reviewer": ("codex", "rig"),
    "refiner": ("claude", "rig"),
    "implementer": ("codex", "rig"),
    "implementer-claude": ("claude", "rig"),
    "validator": ("codex", "rig"),
}


def composed_inventory_parity(inventory: dict[str, dict[str, Any]], *, rig: str = "rig",
                              binding: str = "agentops") -> bool:
    """Compare every active-route fact to the pinned inert agent-list fixture."""
    if set(inventory) != set(_PINNED_INERT_AGENT_FIXTURE):
        return False
    for role, (provider, scope) in _PINNED_INERT_AGENT_FIXTURE.items():
        qualified = f"{binding}.{role}" if scope == "city" else f"{rig}/{binding}.{role}"
        spec = inventory[role]
        if {"qualified_name": spec.get("qualified_name"), "provider": spec.get("provider"),
            "scope": spec.get("scope"), "binding": spec.get("binding"), "suspended": spec.get("suspended"),
            "configured_work_query": spec.get("configured_work_query"), "configured_sling_query": spec.get("configured_sling_query"),
            "effective_work_query": spec.get("effective_work_query"), "effective_sling_query": spec.get("effective_sling_query")} != {
                "qualified_name": qualified, "provider": provider, "scope": scope, "binding": binding, "suspended": False,
                "configured_work_query": None, "configured_sling_query": None,
                "effective_work_query": "stock", "effective_sling_query": f"gc.routed_to={qualified}"}:
            return False
    return True


def _stock_work_query(qualified_name: str) -> str:
    return f"assigned(in_progress|ready,{qualified_name}) || ready+unassigned+gc.routed_to={qualified_name}"


def _managed_city_patches() -> set[str]:
    city = tomllib.loads((PACK_ROOT.parents[1] / "deploy" / "gc" / "city.toml").read_text(encoding="utf-8"))
    patches = city.get("patches", {}).get("agent", [])
    if not isinstance(patches, list):
        return set()
    return {patch.get("name") for patch in patches if isinstance(patch, dict) and patch.get("suspended") is True and isinstance(patch.get("name"), str)}


def composed_route_doctor(overrides: dict[str, Any] | None = None, *, delivery_route: str = "agentops.delivery",
                          rig: str = "rig", binding: str = "agentops") -> dict[str, Any]:
    """Purely compose the factory/executor import graph and managed city patches.

    It intentionally models the one-rig inert inventory rather than invoking GC.
    The role TOMLs own provider/scope while the city template owns generic-role
    suspension; native work/sling facts are rendered from the pinned stock model.
    """
    problems: list[str] = []
    try:
        pack = tomllib.loads((PACK_ROOT / "pack.toml").read_text(encoding="utf-8"))
        if pack.get("imports", {}).get("executor", {}).get("source") != "../agentops-executor":
            problems.append("factory executor import graph is not pinned")
        required_patches = {"bd.dog", "core.control-dispatcher", "codex", "claude"}
        if not required_patches <= _managed_city_patches():
            problems.append("managed generic/provider roles are not all suspended")
    except (OSError, tomllib.TOMLDecodeError) as exc:
        problems.append(f"cannot compose city/import configuration: {exc}")
    inventory: dict[str, dict[str, Any]] = {}
    for role, path in _COMPOSED_ROLE_SOURCES.items():
        try:
            config = tomllib.loads(path.read_text(encoding="utf-8"))
        except (OSError, tomllib.TOMLDecodeError) as exc:
            problems.append(f"cannot load {role}: {exc}")
            continue
        provider, scope = config.get("provider"), config.get("scope")
        if provider not in PROVIDERS or scope not in {"city", "rig"}:
            problems.append(f"{role} has invalid provider or scope")
            continue
        qualified = f"{binding}.{role}" if scope == "city" else f"{rig}/{binding}.{role}"
        inventory[role] = {"qualified_name": qualified, "route": qualified, "provider": provider,
                           "scope": scope, "binding": binding, "suspended": False,
                           "configured_work_query": config.get("work_query"),
                           "configured_sling_query": config.get("sling_query"),
                           "effective_work_query": "stock",
                           "effective_work_query_text": _stock_work_query(qualified),
                           "effective_sling_query": f"gc.routed_to={qualified}"}
    if set(inventory) != set(_COMPOSED_ROLE_SOURCES):
        problems.append("composed inventory does not contain exactly six selected roles")
    if overrides:
        for role, mutation in overrides.items():
            if role not in inventory:
                problems.append(f"unknown inventory mutation: {role}")
                continue
            if isinstance(mutation, str):
                inventory[role]["effective_work_query"] = "ready+unassigned"
                inventory[role]["effective_work_query_text"] = mutation
            elif isinstance(mutation, dict):
                inventory[role].update(mutation)
            else:
                problems.append(f"invalid inventory mutation for {role}")
    for role, spec in inventory.items():
        qualified = spec["qualified_name"]
        config = tomllib.loads(_COMPOSED_ROLE_SOURCES[role].read_text(encoding="utf-8"))
        expected_scope = config.get("scope")
        expected_qualified = f"{binding}.{role}" if expected_scope == "city" else f"{rig}/{binding}.{role}"
        if spec["suspended"] is not False:
            problems.append(f"selected role is suspended: {role}")
        if spec["scope"] != expected_scope or spec["binding"] != binding or qualified != expected_qualified:
            problems.append(f"selected role has invalid scope, binding, or qualified name: {role}")
        if spec["configured_work_query"] is not None or spec["configured_sling_query"] is not None:
            problems.append(f"selected role configures a custom query: {role}")
        if spec["effective_work_query"] != "stock" or spec["effective_work_query_text"] != _stock_work_query(qualified):
            problems.append(f"selected role has a broadened effective work query: {role}")
        if spec["effective_sling_query"] != f"gc.routed_to={qualified}":
            problems.append(f"selected role has a redirected effective sling: {role}")
    if delivery_route in {spec["qualified_name"] for spec in inventory.values()} or any(
        spec["effective_sling_query"] == f"gc.routed_to={delivery_route}" for spec in inventory.values()
    ):
        problems.append("deterministic delivery selector collides with a model route")
    if not composed_inventory_parity(inventory, rig=rig, binding=binding):
        problems.append("composed inventory differs from pinned inert agent-list fixture")
    return {"ok": not problems, "routes": {role: spec["qualified_name"] for role, spec in inventory.items()},
            "inventory": inventory, "roles": sorted(inventory), "custom_query_roles": [
                role for role, spec in inventory.items() if spec["configured_work_query"] is not None or spec["configured_sling_query"] is not None],
            "reason": "; ".join(problems) if problems else None}


def exhaustive_construction_proof(*, delivery_route: str = "agentops.delivery") -> dict[str, int]:
    """Traverse every payload-field subset × readiness state through one store boundary."""
    composition = composed_route_doctor(delivery_route=delivery_route)
    if not composition["ok"]:
        raise FactoryError("composition_failed", composition["reason"] or "invalid route composition")
    routes = composition["inventory"]
    refiner_route = routes["refiner"]["qualified_name"]
    checked = 0
    published = 0
    for kind, payload in (("delivery", _delivery_payload()), ("ambiguity", _ambiguity_payload())):
        fields = tuple(payload)
        full_mask = (1 << len(fields)) - 1
        for mask in range(1 << len(fields)):
            for ready in (False, True):
                store = FakePublicationStore(); identity = f"{kind}-{mask}-{int(ready)}"; store.create(identity, kind)
                for index, field in enumerate(fields):
                    if mask & (1 << index): store.apply(identity, field, payload[field], ready=ready)
                staged = store.snapshot(identity)
                if consumers_for_work(staged, delivery_route=delivery_route, routes=routes):
                    raise FactoryError("routing_firewall_failed", f"construction state is visible: {identity}")
                before = canonical_bytes(staged)
                if mask != full_mask:
                    try:
                        store.publish(identity, delivery_route=delivery_route, refiner_route=refiner_route)
                    except FactoryError:
                        if canonical_bytes(store.snapshot(identity)) != before:
                            raise FactoryError("non_atomic_publication", "rejected publication changed fake store")
                    else:
                        raise FactoryError("premature_publication", "incomplete construction payload published")
                else:
                    final = store.publish(identity, delivery_route=delivery_route, refiner_route=refiner_route)
                    expected = ["delivery"] if kind == "delivery" else ["refiner"]
                    if consumers_for_work(final, delivery_route=delivery_route, routes=routes) != expected:
                        raise FactoryError("routing_firewall_failed", f"published {kind} lacks exactly one consumer")
                    published += 1
                checked += 1
    return {"construction_states": checked, "published_states": published, "model_routes": len(routes)}


def fable_adviser_launch_contract() -> dict[str, Any]:
    """Describe the deliberately credential-free, still-unisolated Fable launch."""
    return {
        "credential_environment": [],
        "forbidden_environment": ["GITHUB_TOKEN", "GH_TOKEN", "GIT_ASKPASS", "SSH_ASKPASS", "SSH_AUTH_SOCK"],
        "stripped_environment": ["GITHUB_TOKEN", "GH_TOKEN", "GIT_ASKPASS", "SSH_ASKPASS", "SSH_AUTH_SOCK"],
        "interactive_unrestricted": True,
        "isolation": "gc33-4-required",
    }


def adviser_dispatch_decision() -> dict[str, Any]:
    return {"status": "adviser_isolation_unproven", "eligible": False}


def require_adviser_isolation() -> None:
    decision = adviser_dispatch_decision()
    raise FactoryError(decision["status"], "adviser_isolation_unproven: GC33-4 process isolation is required before Fable dispatch")


def protected_root_manifest(root: Path, allowed_paths: set[Path]) -> dict[str, str]:
    """Hash every protected file, including .git, except exact transport outputs."""
    manifest: dict[str, str] = {}
    for path in sorted(root.rglob("*")):
        if path.is_file() and path.resolve() not in allowed_paths:
            manifest[str(path.relative_to(root))] = digest_file(path)
    return manifest


def guard_ambiguity_advice_transport(paths: dict[str, Path], environment: dict[str, str], before: dict[str, str], after: dict[str, str]) -> None:
    forbidden = set(fable_adviser_launch_contract()["forbidden_environment"]) & set(environment)
    if forbidden:
        raise FactoryError("credential_exposed", f"Fable adviser launch exposes credentials: {sorted(forbidden)}")
    allowed = {paths["artifact"].resolve(), paths["result"].resolve()}
    if before != after:
        raise FactoryError("protected_root_mutated", "Fable adviser changed protected workspace or Git state")
    workspace = paths["workspace"].resolve()
    if paths["artifact"].resolve() == paths["result"].resolve() or not all(is_within(path, workspace) for path in allowed):
        raise FactoryError("invalid_path", "Fable adviser transport paths are not exact workspace outputs")


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


def factory_git_command(*args: str) -> list[str]:
    """Build a Git command for a commit owned by the factory runtime.

    Disposable rigs must not depend on a caller's global or repository-local
    author configuration. The explicit identity applies only to this command;
    it does not mutate either Git config surface.
    """
    return [
        "git",
        "-c", f"user.name={FACTORY_GIT_NAME}",
        "-c", f"user.email={FACTORY_GIT_EMAIL}",
        *args,
    ]


def abort_interrupted_factory_cherry_pick(worktree: Path) -> bool:
    """Abort only a Git-proven interrupted factory cherry-pick transaction."""
    marker = run_process(
        ["git", "rev-parse", "--verify", "CHERRY_PICK_HEAD"],
        cwd=worktree,
        check=False,
    )
    if marker.returncode != 0:
        return False
    prior_head = git_head(worktree)
    run_process(factory_git_command("cherry-pick", "--abort"), cwd=worktree, timeout=120)
    if git_head(worktree) != prior_head:
        raise FactoryError("assembly_recovery_failed", "aborting interrupted cherry-pick moved the integration head")
    if output(["git", "status", "--porcelain", "--untracked-files=no"], cwd=worktree):
        raise FactoryError("assembly_recovery_failed", "aborting interrupted cherry-pick left tracked changes")
    return True


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
    # Some native Beads commands currently print human progress before a
    # pretty-printed JSON object even with --json. Accept one complete trailing
    # JSON value without weakening the requirement that nothing follows it.
    decoder = json.JSONDecoder()
    for index in range(len(raw) - 1, -1, -1):
        if raw[index] not in "[{":
            continue
        try:
            value, end = decoder.raw_decode(raw[index:])
        except json.JSONDecodeError:
            continue
        if not raw[index + end:].strip():
            return value
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

    def run(
        self,
        *args: str,
        check: bool = True,
        timeout: float = CONTROL_PLANE_TIMEOUT_SECONDS,
    ) -> subprocess.CompletedProcess[str]:
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

    def close(self, bead_id: str, reason: str, work_outcome: str,
              commit: str | None = None, branch: str | None = None,
              work_dir: str | Path | None = None) -> None:
        if work_outcome not in {"shipped", "no-op", "blocked", "abandoned"}:
            raise FactoryError("invalid_work_outcome", f"invalid gc.work_outcome {work_outcome!r}")
        fields: dict[str, Any] = {
            "gc.outcome": "fail" if work_outcome in {"blocked", "abandoned"} else "pass",
            "gc.work_outcome": work_outcome,
        }
        if work_outcome == "shipped":
            fields["gc.work_commit"] = require_string(commit, "gc.work_commit", SHA_RE)
            fields["gc.work_branch"] = require_string(branch, "gc.work_branch")
            if work_dir is not None:
                fields["gc.work_dir"] = str(Path(work_dir).resolve(strict=False))
        elif commit is not None or branch is not None:
            raise FactoryError("invalid_work_outcome", f"{work_outcome} must not carry commit/branch evidence")
        self.update_metadata(bead_id, fields)
        self.run("close", bead_id, "--reason", reason)

    def defer(self, bead_id: str, reason: str) -> None:
        self.run("defer", bead_id, "--reason", reason)

    def hold_delivery(self, bead_id: str) -> None:
        """Atomically park delivery and release every GC runtime identity.

        Gas City deliberately projects Beads' non-terminal statuses (including
        ``deferred``) onto its smaller ``open`` work-state vocabulary.  A
        deferred bead that remains routed or assigned can therefore keep a
        Refiner pool hot forever.  A delivery hold is not runnable work: retain
        the durable hold facts on the bead, but remove its route, assignee, and
        ephemeral session workspace in the same Beads update that defers it.
        """
        self.run(
            "update", bead_id,
            "--status", "deferred",
            "--assignee", "",
            "--unset-metadata", "gc.routed_to",
            "--unset-metadata", "gc.session_name",
            "--unset-metadata", "gc.work_dir",
        )

    def retry_delivery(self, bead_id: str, route: str) -> None:
        route = require_string(route, "Refiner retry route")
        self.run(
            "update", bead_id,
            "--status", "open",
            "--assignee", "",
            "--set-metadata", f"gc.routed_to={route}",
            "--unset-metadata", "factory.delivery_hold",
            "--unset-metadata", "factory.delivery_hold_code",
            "--unset-metadata", "factory.delivery_hold_reason",
            "--unset-metadata", "factory.refiner_context_id",
            "--unset-metadata", "factory.refiner_model",
            "--unset-metadata", "factory.refiner_model_policy",
            "--unset-metadata", "factory.refiner_model_source",
            "--unset-metadata", "gc.session_name",
            "--unset-metadata", "gc.work_branch",
            "--unset-metadata", "gc.work_dir",
        )

    def dep_add(self, blocked: str, blocker: str, dep_type: str = "blocks") -> None:
        self.run("dep", "add", blocked, blocker, "--type", dep_type)

    def acquire_merge_slot(self, holder: str, timeout: float) -> str:
        """Acquire this rig's native Beads merge slot with bounded waiting."""
        holder = require_string(holder, "merge slot holder")
        if timeout <= 0:
            raise FactoryError("invalid_argument", "merge slot timeout must be positive")
        created = parse_json_output(
            self.run("merge-slot", "create", "--json", timeout=120).stdout,
            "bd merge-slot create",
        )
        slot_id = require_string(
            created.get("id") if isinstance(created, dict) else None,
            "merge slot id",
        )
        deadline = time.monotonic() + timeout
        while True:
            completed = self.run(
                "merge-slot", "acquire", "--holder", holder, "--wait", "--json",
                check=False, timeout=min(60, max(5, timeout)),
            )
            value = parse_json_output(completed.stdout, "bd merge-slot acquire")
            if not isinstance(value, dict):
                raise FactoryError("merge_slot_invalid", "bd merge-slot acquire returned no object")
            if value.get("id") != slot_id:
                raise FactoryError("merge_slot_invalid", "Beads changed merge slot identity while acquiring")
            # Beads returns exit 1/acquired:false when the same stable holder
            # retries after process recovery. Its holder field is still the
            # authoritative ownership proof.
            if value.get("acquired") is True or value.get("holder") == holder:
                return slot_id
            if value.get("waiting") is not True:
                detail = completed.stderr.strip() or repr(value)
                raise FactoryError("merge_slot_failed", f"cannot queue for native merge slot: {detail}")
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise FactoryError(
                    "merge_slot_timeout",
                    f"native merge slot {slot_id} remained held by {value.get('holder')!r}",
                )
            time.sleep(min(2.0, remaining))

    def release_merge_slot(self, holder: str, slot_id: str) -> None:
        holder = require_string(holder, "merge slot holder")
        slot_id = require_string(slot_id, "merge slot id")
        completed = self.run(
            "merge-slot", "release", "--holder", holder, "--json",
            check=False, timeout=120,
        )
        value = parse_json_output(completed.stdout, "bd merge-slot release")
        if (
            completed.returncode != 0
            or not isinstance(value, dict)
            or value.get("id") != slot_id
            or value.get("released") is not True
        ):
            detail = completed.stderr.strip() or repr(value)
            raise FactoryError("merge_slot_release_failed", f"cannot release native merge slot: {detail}")


@contextlib.contextmanager
def refinery_merge_slot(beads: Beads, refinery_bead: str, timeout: float) -> Any:
    """Serialize only candidate assembly; fresh validation runs outside it."""
    holder = f"factory-refinery:{require_string(refinery_bead, 'refinery bead')}"
    slot_id = beads.acquire_merge_slot(holder, timeout)
    try:
        yield {"id": slot_id, "holder": holder}
    finally:
        beads.release_merge_slot(holder, slot_id)


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

    def field_contracts(properties: dict[str, Any]) -> dict[str, dict[str, Any]]:
        visible = ("type", "const", "enum", "pattern", "minLength", "minItems", "maxItems", "uniqueItems")
        return {
            name: {key: definition[key] for key in visible if key in definition}
            for name, definition in properties.items()
        }

    def model_policy_contract() -> dict[str, Any]:
        return {
            "worker": {
                provider: FACTORY_ROLE_MODELS[("implement", provider)][0]
                for provider in sorted(PROVIDERS)
            },
            "validator": {
                provider: FACTORY_ROLE_MODELS[("validate", provider)][0]
                for provider in sorted(PROVIDERS)
            },
            "lifecycle": {
                role: {
                    provider: FACTORY_ROLE_MODELS[(role, provider)][0]
                    for provider in sorted(PROVIDERS)
                }
                for role in ("mayor", "plan-review", "refiner")
            },
        }

    if request["role"] == "plan-review":
        schema_path = SCHEMA_ROOT / "plan-review.v1.schema.json"
        schema = load_object(schema_path, "plan-review schema")
        rendered["artifact_contract"] = {
            "schema_path": str(schema_path.resolve()),
            "required_top_level": schema["required"],
            "allowed_top_level": sorted(schema["properties"]),
            "criterion_required": schema["properties"]["criteria"]["items"]["required"],
            "criterion_allowed": sorted(schema["properties"]["criteria"]["items"]["properties"]),
            "finding_required": schema["properties"]["findings"]["items"]["required"],
            "finding_allowed": sorted(schema["properties"]["findings"]["items"]["properties"]),
            "verdict_values": sorted(VERDICTS),
            "top_level_fields": field_contracts(schema["properties"]),
            "criterion_fields": field_contracts(schema["properties"]["criteria"]["items"]["properties"]),
            "finding_fields": field_contracts(schema["properties"]["findings"]["items"]["properties"]),
            "model_policy": model_policy_contract(),
        }
    else:
        schema_path = SCHEMA_ROOT / "program-graph.v1.schema.json"
        schema = load_object(schema_path, "program-graph schema")
        node = schema["properties"]["nodes"]["items"]
        rendered["artifact_contract"] = {
            "schema_path": str(schema_path.resolve()),
            "artifact_kind": "program-graph" if request["role"] == "mayor" else "successor-node",
            "required_top_level": schema["required"] if request["role"] == "mayor" else node["required"],
            "allowed_top_level": (
                sorted(schema["properties"])
                if request["role"] == "mayor"
                else sorted(node["properties"])
            ),
            "node_required": node["required"],
            "node_allowed": sorted(node["properties"]),
            "provider_values": sorted(PROVIDERS),
            "execution_role": "implementation",
            "model_policy": model_policy_contract(),
            "risk_values": ["high", "routine"],
            "first_check_semantics": (
                "A post-implementation shell command run by the factory in the candidate worktree; "
                "it must exit 0 for a correct candidate. It is not a RED precondition or an assertion "
                "that the requested product is absent. It must be scoped to the node's product paths "
                "and acceptance, independent of factory/runtime scaffolding (.gc/**, .claude/**, "
                ".codex/**), sibling worktrees, and unrelated pre-existing caller changes."
            ),
            "top_level_fields": field_contracts(schema["properties"]),
            "node_fields": field_contracts(node["properties"]),
            "subject_fields": field_contracts(node["properties"]["subject"]["properties"]),
        }
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
    lifecycle_role = "mayor" if request["role"] == "rescope" else request["role"]
    expected_suffix = f".{lifecycle_agent_name(lifecycle_role, provider)}"
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
            request["mayor_context_id"], request["provider"],
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


def validate_role_request_v2(path: Path) -> tuple[dict[str, Any], dict[str, Path]]:
    """Validate the composed 3.3 request without changing legacy v1 routing."""
    request = load_object(path, "factory role v2 request")
    exact_keys(request, ROLE_REQUEST_V2_KEYS, ROLE_REQUEST_V2_KEYS, "factory role v2 request")
    if request.get("schema_version") != "factory-role-request.v2":
        raise FactoryError("invalid_role_request", "schema_version must be factory-role-request.v2")
    for field in ("request_id", "program_id", "semantic_bead_id"):
        require_string(request.get(field), field)
    require_string(request.get("intent_digest"), "intent_digest", DIGEST_RE)
    role = request.get("role")
    if role == "support":
        raise FactoryError("unroutable_role", "support/Luna remains fail-closed and unroutable")
    if role not in V2_ROUTABLE_ROLES:
        raise FactoryError("unroutable_role", f"role is not enabled for the v2 adapter: {role!r}")
    if role == "mayor":
        if request.get("prior_context_id") is not None:
            raise FactoryError("invalid_role_request", "Mayor v2 request must not name a prior context")
    else:
        require_string(request.get("prior_context_id"), "prior_context_id")
    workspace = absolute_path(request.get("workspace"), "workspace", True, True)
    intent = absolute_path(request.get("intent_source"), "intent_source", True)
    subject = absolute_path(request.get("subject_path"), "subject_path", True)
    if not is_within(intent, workspace) or not is_within(subject, workspace):
        raise FactoryError("invalid_path", "intent and subject paths must stay inside workspace")
    if request["intent_digest"] != digest_file(intent):
        raise FactoryError("identity_mismatch", "request intent digest is stale")
    require_string(request.get("subject_digest"), "subject_digest", DIGEST_RE)
    if request["subject_digest"] != digest_file(subject):
        raise FactoryError("identity_mismatch", "request subject digest is stale")
    evidence = require_list(request.get("evidence_refs"), "evidence_refs", nonempty=True)
    for index, reference in enumerate(evidence):
        if not isinstance(reference, dict):
            raise FactoryError("invalid_contract", f"evidence_refs[{index}] must be an object")
        exact_keys(reference, {"path", "digest"}, {"path", "digest"}, f"evidence_refs[{index}]")
        evidence_path = absolute_path(reference.get("path"), f"evidence_refs[{index}].path", True)
        if not is_within(evidence_path, workspace):
            raise FactoryError("invalid_path", f"evidence_refs[{index}] escapes workspace")
        digest = require_string(reference.get("digest"), f"evidence_refs[{index}].digest", DIGEST_RE)
        if digest != digest_file(evidence_path):
            raise FactoryError("identity_mismatch", f"evidence_refs[{index}] digest is stale")
    configured = tomllib.loads(ROLE_TEMPLATES[role].read_text(encoding="utf-8"))
    expected = requested_role_policy(role, configured["provider"])
    if request.get("requested") != expected:
        raise FactoryError("requested_policy_mismatch", f"{role} requested policy is not exact")
    artifact = absolute_path(request.get("artifact_path"), "artifact_path")
    result = absolute_path(request.get("result_path"), "result_path")
    if not is_within(artifact, workspace) or not is_within(result, workspace):
        raise FactoryError("invalid_path", "role artifacts must stay inside workspace")
    if len({path, artifact, result}) != 3:
        raise FactoryError("invalid_role_request", "request, artifact, and result paths must differ")
    return request, {
        "request": path, "workspace": workspace, "intent": intent, "subject": subject,
        "artifact": artifact, "result": result,
    }


def validate_program_graph_v2(value: dict[str, Any], intent_digest: str) -> dict[str, Any]:
    """Strictly enforce the intentionally small 3.3 graph contract."""
    fields = {"schema_version", "program_id", "intent_digest", "nodes", "max_parallel", "role_policy", "delivery_group_id", "prefix_safety"}
    exact_keys(value, fields, fields, "program graph v2")
    if value.get("schema_version") != "program-graph.v2":
        raise FactoryError("invalid_graph", "schema_version must be program-graph.v2")
    require_string(value.get("program_id"), "program_id")
    if value.get("intent_digest") != intent_digest:
        raise FactoryError("identity_mismatch", "program graph intent digest is stale")
    if not isinstance(value.get("max_parallel"), int) or isinstance(value["max_parallel"], bool) or value["max_parallel"] < 1:
        raise FactoryError("invalid_graph", "max_parallel must be a positive integer")
    require_string(value.get("delivery_group_id"), "delivery_group_id")
    if value.get("prefix_safety") not in {"safe", "atomic_group", "externally_gated"}:
        raise FactoryError("invalid_graph", "prefix_safety is invalid")
    no_fallback = {"allowed": False, "used": False, "reason": None}

    def runtime(policy: Any, name: str, model: str, reasoning: str, provider: str, extra: dict[str, Any] | None = None) -> None:
        required = {"model", "reasoning", "provider", "fallback", *(extra or {})}
        if not isinstance(policy, dict):
            raise FactoryError("invalid_graph", f"role_policy.{name} must be an object")
        exact_keys(policy, required, required, f"role_policy.{name}")
        expected = {"model": model, "reasoning": reasoning, "provider": provider, "fallback": no_fallback, **(extra or {})}
        if policy != expected:
            raise FactoryError("invalid_graph", f"role_policy.{name} is not exact")

    role_policy = value.get("role_policy")
    role_keys = {"mayor", "planner", "worker_pool", "validator", "refiner", "luna"}
    if not isinstance(role_policy, dict):
        raise FactoryError("invalid_graph", "role_policy must be an object")
    exact_keys(role_policy, role_keys, role_keys, "role_policy")
    runtime(role_policy["mayor"], "mayor", "fable", "adaptive", "claude")
    runtime(role_policy["planner"], "planner", "sol", "high", "codex")
    runtime(role_policy["validator"], "validator", "sol", "high", "codex")
    runtime(role_policy["refiner"], "refiner", "fable", "adaptive", "claude", {"ambiguity_only": True})
    runtime(role_policy["luna"], "luna", "luna", "high", "codex", {"support_only": True})
    workers = role_policy["worker_pool"]
    if not isinstance(workers, dict):
        raise FactoryError("invalid_graph", "role_policy.worker_pool must be an object")
    exact_keys(workers, {"default", "overflow", "fallback"}, {"default", "overflow", "fallback"}, "role_policy.worker_pool")
    runtime(workers["default"], "worker_pool.default", "terra", "high", "codex")
    runtime(workers["overflow"], "worker_pool.overflow", "opus", "medium", "claude")
    if workers["fallback"] != no_fallback:
        raise FactoryError("invalid_graph", "role_policy.worker_pool fallback is not exact")
    nodes = require_list(value.get("nodes"), "nodes", nonempty=True)
    ids: set[str] = set()
    dependencies: dict[str, list[str]] = {}
    node_fields = {"id", "bead_class", "intent_digest", "depends_on", "write_scope", "generated_companions", "role", "model", "reasoning", "provider", "fallback"}
    for index, node in enumerate(nodes):
        if not isinstance(node, dict):
            raise FactoryError("invalid_graph", f"nodes[{index}] must be an object")
        exact_keys(node, node_fields, node_fields, f"nodes[{index}]")
        node_id = require_string(node.get("id"), f"nodes[{index}].id")
        if node_id in ids:
            raise FactoryError("invalid_graph", f"duplicate node id {node_id}")
        ids.add(node_id)
        if node.get("bead_class") not in {"product", "delivery_repair"} or node.get("intent_digest") != intent_digest:
            raise FactoryError("invalid_graph", f"nodes[{index}] has an invalid class or intent digest")
        if node.get("role") != "implementation" or node.get("fallback") != no_fallback:
            raise FactoryError("invalid_graph", f"nodes[{index}] role or fallback is invalid")
        pair = (node.get("model"), node.get("reasoning"), node.get("provider"))
        if pair not in {("terra", "high", "codex"), ("opus", "medium", "claude")}:
            raise FactoryError("invalid_graph", f"nodes[{index}] model/provider/reasoning is invalid")
        depends = require_list(node.get("depends_on"), f"nodes[{index}].depends_on")
        if any(not isinstance(dep, str) or not dep for dep in depends) or len(set(depends)) != len(depends):
            raise FactoryError("invalid_graph", f"nodes[{index}] dependencies are invalid")
        if node_id in depends:
            raise FactoryError("invalid_graph", f"nodes[{index}] cannot depend on itself")
        dependencies[node_id] = depends
        write_scope = canonical_v2_scope_paths(node.get("write_scope"), f"nodes[{index}].write_scope", nonempty=True)
        generated = canonical_v2_scope_paths(node.get("generated_companions"), f"nodes[{index}].generated_companions", nonempty=False)
        if set(write_scope) & set(generated):
            raise FactoryError("invalid_scope", f"nodes[{index}] reuses a normalized path across scope classes")
        node["write_scope"] = write_scope
        node["generated_companions"] = generated
    if any(dep not in ids for deps in dependencies.values() for dep in deps):
        raise FactoryError("invalid_graph", "node dependency is not in the graph")
    pending = {node_id: set(deps) for node_id, deps in dependencies.items()}
    while pending:
        ready = {node_id for node_id, deps in pending.items() if not deps}
        if not ready:
            raise FactoryError("invalid_graph", "program graph contains a dependency cycle")
        for node_id in ready:
            pending.pop(node_id)
        for deps in pending.values():
            deps.difference_update(ready)
    return value


def role_artifact_contract_v2(role: str) -> dict[str, Any]:
    if role not in V2_ROUTABLE_ROLES:
        raise FactoryError("unroutable_role", f"role is not enabled for the v2 adapter: {role!r}")
    schema_names = {
        "mayor": "program-graph.v2.schema.json",
        "plan": "plan-review.v1.schema.json",
        "ambiguity_advice": "ambiguity-advice.v1.schema.json",
    }
    schema_path = SCHEMA_ROOT / schema_names[role]
    schema = load_object(schema_path, f"{role} artifact schema")
    return {
        "schema_path": str(schema_path.resolve()),
        "schema_version": schema["properties"]["schema_version"]["const"],
        "required_top_level": schema["required"],
        "allowed_top_level": sorted(schema["properties"]),
        "schema": schema,
        "nonbinding_no_byte": role == "ambiguity_advice",
    }


def command_inspect_role_v2(args: argparse.Namespace) -> int:
    path = absolute_path(args.request, "request", True)
    request, paths = validate_role_request_v2(path)
    rendered = dict(request)
    rendered["request_path"] = str(paths["request"])
    rendered["request_digest"] = digest_file(paths["request"])
    rendered["artifact_contract"] = role_artifact_contract_v2(request["role"])
    print(json.dumps(rendered, sort_keys=True))
    return 0


def validate_v2_artifact(request: dict[str, Any], artifact: Path, context_id: str) -> None:
    role = request["role"]
    value = load_object(artifact, f"{role} artifact")
    if role == "ambiguity_advice":
        exact_keys(
            value,
            {"schema_version", "request_id", "context_id", "finding", "nonbinding", "mutates_artifacts"},
            {"schema_version", "request_id", "context_id", "finding", "nonbinding", "mutates_artifacts"},
            f"{role} artifact",
        )
        expected_schema = role_artifact_contract_v2(role)["schema_version"]
        if value.get("schema_version") != expected_schema or value.get("request_id") != request["request_id"]:
            raise FactoryError("artifact_mismatch", f"{role} artifact does not bind the exact request")
        if value.get("context_id") != context_id:
            raise FactoryError("runtime_identity_mismatch", f"{role} artifact context differs from the runtime session")
        if value.get("nonbinding") is not True or value.get("mutates_artifacts") is not False:
            raise FactoryError("invalid_contract", f"{role} artifact must be nonbinding and no-byte")
        require_string(value.get("finding"), f"{role} finding")
    elif role == "mayor":
        graph = validate_program_graph_v2(value, request["intent_digest"])
        if graph["program_id"] != request["program_id"]:
            raise FactoryError("artifact_mismatch", "Mayor artifact does not bind the requested program")
    else:
        graph = validate_program_graph_v2(load_object(Path(request["subject_path"]), "plan subject graph"), request["intent_digest"])
        if graph["program_id"] != request["program_id"]:
            raise FactoryError("artifact_mismatch", "plan subject graph does not bind the requested program")
        validate_review(value, graph, request["subject_digest"], request["prior_context_id"], "codex")
        if value["reviewer_context_id"] != context_id:
            raise FactoryError("runtime_identity_mismatch", "plan artifact context differs from the runtime session")


def command_emit_role_v2(args: argparse.Namespace) -> int:
    request_path = absolute_path(args.request, "request", True)
    request, paths = validate_role_request_v2(request_path)
    artifact = absolute_path(args.artifact, "artifact", True)
    if artifact != paths["artifact"]:
        raise FactoryError("artifact_mismatch", "emitted artifact differs from requested artifact_path")
    if request["role"] == "ambiguity_advice":
        before = protected_root_manifest(paths["workspace"], {paths["artifact"].resolve(), paths["result"].resolve()})
        guard_ambiguity_advice_transport(paths, dict(os.environ), before, before)
        require_adviser_isolation()
    context_id = require_string(os.environ.get("GC_SESSION_ID"), "GC_SESSION_ID")
    if request["role"] != "mayor" and context_id == request["prior_context_id"]:
        raise FactoryError("freshness_collision", "v2 role context collides with its prior context")
    session = runtime_session(context_id)
    expected_template = f".{V2_ROLE_TEMPLATES[request['role']]}"
    if session.get("template", "").endswith(expected_template) is False:
        raise FactoryError("template_mismatch", f"runtime template must end with {expected_template}")
    if session.get("provider") != request["requested"]["provider"]:
        raise FactoryError("provider_mismatch", "runtime provider differs from role request")
    if session.get("live_model_observed") is not True:
        raise FactoryError("runtime_identity_missing", "v2 emit requires a live session model observation")
    fallback = {"allowed": False, "used": False, "reason": None}
    actual = {
        "provider": session["provider"], "model": session["model"],
        "reasoning": request["requested"]["reasoning"],
        "effort": command_effort(session["command"]), "fallback": fallback,
    }
    attest_role_runtime(request["role"], request["requested"], actual, context_id, False)
    validate_v2_artifact(request, artifact, context_id)
    response = {
        "schema_version": "factory-role-response.v2", "request_id": request["request_id"],
        "request_digest": digest_file(request_path), "role": request["role"],
        "semantic_bead_id": request["semantic_bead_id"], "session_context_id": context_id,
        "requested": request["requested"], "actual": actual, "artifact_path": str(artifact),
        "artifact_digest": digest_file(artifact),
    }
    exact_keys(response, ROLE_RESPONSE_V2_KEYS, ROLE_RESPONSE_V2_KEYS, "factory role v2 response")
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
    if node.get("execution_role") != "implementation":
        raise FactoryError("invalid_graph", f"node {node_id} execution_role must be implementation")
    if node.get("provider") not in PROVIDERS or node.get("validator_provider") not in PROVIDERS:
        raise FactoryError("invalid_graph", f"node {node_id} providers must be codex or claude")
    if node["provider"] == node["validator_provider"]:
        raise FactoryError("invalid_graph", f"node {node_id} must use an opposite-family Validator")
    expected_worker = FACTORY_ROLE_MODELS[("implement", node["provider"])][0]
    expected_validator = FACTORY_ROLE_MODELS[("validate", node["validator_provider"])][0]
    if node.get("worker_model_policy") != expected_worker:
        raise FactoryError(
            "invalid_graph",
            f"node {node_id} worker_model_policy must be {expected_worker} for {node['provider']}",
        )
    if node.get("validator_model_policy") != expected_validator:
        raise FactoryError(
            "invalid_graph",
            f"node {node_id} validator_model_policy must be {expected_validator} for {node['validator_provider']}",
        )
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
                    mayor_context: str, expected_provider: str | None = None) -> dict[str, Any]:
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
    if expected_provider is not None and value["provider"] != expected_provider:
        raise FactoryError(
            "provider_mismatch",
            f"plan review provider must be {expected_provider!r}, got {value['provider']!r}",
        )
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


def command_model(command: str) -> str:
    try:
        tokens = shlex.split(command)
    except ValueError as exc:
        raise FactoryError("runtime_identity_missing", f"GC runtime command cannot be parsed: {exc}") from exc
    values = [tokens[index + 1] for index, token in enumerate(tokens[:-1]) if token in {"--model", "-m"}]
    if len(values) != 1 or not values[0].strip():
        raise FactoryError("runtime_identity_missing", "GC runtime command must contain exactly one explicit model")
    return values[0].strip()


def command_effort(command: str) -> str | None:
    """Read the concrete effort launch option; Fable adaptive has no flag."""
    try:
        tokens = shlex.split(command)
    except ValueError as exc:
        raise FactoryError("runtime_identity_missing", f"GC runtime command cannot be parsed: {exc}") from exc
    values: list[str] = []
    for index, token in enumerate(tokens[:-1]):
        if token == "--effort":
            values.append(tokens[index + 1])
        elif token == "-c" and tokens[index + 1].startswith("model_reasoning_effort="):
            values.append(tokens[index + 1].split("=", 1)[1])
    if len(values) > 1 or any(not value.strip() for value in values):
        raise FactoryError("runtime_identity_missing", "GC runtime command has ambiguous effort")
    return values[0] if values else None


def runtime_session(context_id: str) -> dict[str, Any]:
    value = parse_json_output(
        output([gc_binary(), "--city", city_path(), "session", "list", "--state", "all", "--json"]),
        "gc session list",
    )
    sessions = value.get("sessions") if isinstance(value, dict) else None
    if not isinstance(sessions, list):
        raise FactoryError("runtime_identity_missing", "gc session list returned no sessions")
    matches = [item for item in sessions if isinstance(item, dict) and str(item.get("id", "")) == context_id]
    if len(matches) > 1:
        raise FactoryError("runtime_identity_missing", f"expected one runtime session {context_id}, found {len(matches)}")
    live = matches[0] if matches else None

    # The durable city-scoped session bead owns the exact launch command. Read
    # it even while the live projection exists so model policy is attested from
    # the process GC actually launched, not from ambient CLI defaults.
    try:
        record = Beads(None).show(context_id)
    except FactoryError as exc:
        raise FactoryError(
            "runtime_identity_missing",
            f"runtime session {context_id} is absent from both the live projection and city session beads",
        ) from exc
    if str(record.get("id", "")) != context_id or record.get("issue_type") != "session":
        raise FactoryError("runtime_identity_missing", f"city bead {context_id} is not the requested session identity")
    if live is None and record.get("status") != "closed":
        raise FactoryError("runtime_identity_missing", f"runtime session {context_id} is not live and its bead is not closed")
    meta = metadata(record)
    fields: dict[str, str] = {}
    for field in ("provider", "template", "session_name", "state"):
        value = (live or {}).get(field) or meta.get(field)
        fields[field] = require_string(value, f"session {field}")
    launched_model = command_model(str(meta.get("command", "")))
    observed_model = (live or {}).get("model")
    if isinstance(observed_model, str) and observed_model.strip() and observed_model.strip() != launched_model:
        raise FactoryError(
            "runtime_model_mismatch",
            f"GC transcript model {observed_model.strip()} does not match launched model {launched_model}",
        )
    return {
        "id": context_id,
        **fields,
        "status": record.get("status"),
        "model": launched_model,
        "model_source": "launch_command",
        "command": require_string(meta.get("command"), "session command"),
        "live_model_observed": isinstance(observed_model, str) and bool(observed_model.strip()),
    }


def dispatch_role(request_path: Path, rig: str, binding: str, timeout: float,
                  existing_work_bead: str | None = None,
                  linked_bead: str | None = None) -> dict[str, Any]:
    request, paths = validate_role_request(request_path)
    beads = Beads(None if request["role"] in {"mayor", "rescope"} else rig)
    kind = {"mayor": "planning", "plan-review": "plan-review", "rescope": "rescope-planning"}[request["role"]]
    request_digest = digest_file(request_path)
    if request["role"] in {"mayor", "rescope"}:
        target = lifecycle_target("mayor", request["provider"], rig, binding)
    else:
        target = lifecycle_target("plan-review", request["provider"], rig, binding)
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
        record = beads.show(bead_id)
        if record.get("status") == "closed" and paths["result"].is_file():
            # The bead close is the role's commit point. A response emitted by
            # a process that crashes before closure is provisional and may be
            # replaced when the same session/bead recovers. Loading it early
            # would cache stale evidence across that legitimate recovery.
            response = load_object(paths["result"], "factory role response")
            break
        time.sleep(1)
    else:
        if paths["result"].is_file():
            raise FactoryError("role_timeout", f"role bead {bead_id} emitted but did not close")
        raise FactoryError("role_timeout", f"role bead {bead_id} produced no response")
    assert response is not None and record is not None
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
    configured_model, launched_model = FACTORY_ROLE_MODELS[(request["role"], request["provider"])]
    if session.get("model") != launched_model:
        raise FactoryError(
            "runtime_model_mismatch",
            f"GC runtime model must be {configured_model} ({launched_model}) for {request['role']}/{request['provider']}",
        )
    if record.get("assignee") != response["session_name"]:
        raise FactoryError("runtime_identity_mismatch", "role bead assignee differs from the response session")
    beads.update_metadata(bead_id, {
        "factory.status": "completed",
        "factory.target": target,
        "factory.session_context_id": session_id,
        "factory.model": session["model"],
        "factory.model_policy": configured_model,
        "factory.model_source": session["model_source"],
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
        f"First post-implementation GREEN check: {node['first_check']}\n"
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
        "factory.execution_role": node["execution_role"],
        "factory.worker_model_policy": node["worker_model_policy"],
        "factory.validator_provider": node["validator_provider"],
        "factory.validator_model_policy": node["validator_model_policy"],
        "factory.write_scope": json.dumps(node["write_scope"], separators=(",", ":")),
        "factory.generated_scope": json.dumps(node["generated_scope"], separators=(",", ":")),
        "factory.subject": json.dumps(node["subject"], sort_keys=True, separators=(",", ":")),
        "factory.first_check": node["first_check"],
        "factory.risk": node["risk"],
        "factory.spec": json.dumps(node, sort_keys=True, separators=(",", ":")),
    }


def compile_bead_plan(graph: dict[str, Any], graph_digest: str, review_digest: str,
                      intent_source: Path, mayor_context: str, reviewer_context: str,
                      mayor_provider: str = "codex", reviewer_provider: str = "claude",
                      refiner_provider: str = "codex",
                      integration_validator_provider: str = "claude") -> dict[str, Any]:
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
            "factory.mayor_provider": mayor_provider,
            "factory.reviewer_context_id": reviewer_context,
            "factory.plan_reviewer_provider": reviewer_provider,
            "factory.refiner_provider": refiner_provider,
            "factory.integration_validator_provider": integration_validator_provider,
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
            "factory.mayor_provider": mayor_provider,
            "factory.refiner_provider": refiner_provider,
            "factory.integration_validator_provider": integration_validator_provider,
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
                  mayor_context: str, rig: str, binding: str = "factory",
                  delivery_mode: str = "pr", mayor_provider: str = "codex",
                  reviewer_provider: str = "claude", refiner_provider: str = "codex",
                  integration_validator_provider: str = "claude") -> dict[str, Any]:
    if delivery_mode not in DELIVERY_MODES:
        raise FactoryError("invalid_contract", f"delivery_mode must be one of {sorted(DELIVERY_MODES)}")
    mayor_provider = require_provider(mayor_provider, "mayor_provider")
    reviewer_provider = require_provider(reviewer_provider, "reviewer_provider")
    refiner_provider = require_provider(refiner_provider, "refiner_provider")
    integration_validator_provider = require_provider(
        integration_validator_provider, "integration_validator_provider",
    )
    if mayor_provider == reviewer_provider:
        raise FactoryError("provider_collision", "plan review must use the opposite provider from the Mayor")
    if refiner_provider == integration_validator_provider:
        raise FactoryError(
            "provider_collision",
            "integration validation must use the opposite provider from the Refiner",
        )
    graph = validate_graph(load_object(graph_path, "program graph"), digest_file(intent))
    mayor = require_string(mayor_context, "mayor context")
    review = validate_review(
        load_object(review_path, "plan review"), graph, digest_file(graph_path),
        mayor, reviewer_provider,
    )
    if review["verdict"] != "PASS":
        raise FactoryError("plan_rejected", f"plan review is {review['verdict']}; no beads were admitted")
    plan = compile_bead_plan(
        graph, digest_file(graph_path), digest_file(review_path), intent, mayor,
        review["reviewer_context_id"], mayor_provider, reviewer_provider,
        refiner_provider, integration_validator_provider,
    )
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
        expected_policy = {
            "factory.mayor_provider": mayor_provider,
            "factory.refiner_provider": refiner_provider,
            "factory.integration_validator_provider": integration_validator_provider,
        }
        for item in (*programs, *refineries):
            item_meta = metadata(item)
            for key, wanted in expected_policy.items():
                if item_meta.get(key) != wanted:
                    raise FactoryError("admission_collision", f"existing program lifecycle policy {key} differs")
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
    for key in ("program", "refinery"):
        bead_id = ids[key]
        current_mode = metadata(beads.show(bead_id)).get("factory.delivery_mode")
        if current_mode is not None and current_mode != delivery_mode:
            raise FactoryError("admission_collision", f"existing {key} bead has delivery mode {current_mode!r}")
        beads.update_metadata(bead_id, {"factory.delivery_mode": delivery_mode})
    return {
        "schema_version": "factory-admission-result.v1",
        "program_id": graph["program_id"],
        "program_bead": ids["program"],
        "refinery_bead": ids["refinery"],
        "experiment_beads": {node["id"]: ids[f"experiment-{node['id']}"] for node in graph["nodes"]},
        "graph_digest": digest_file(graph_path),
        "review_digest": digest_file(review_path),
        "delivery_mode": delivery_mode,
        "mayor_provider": mayor_provider,
        "reviewer_provider": reviewer_provider,
        "refiner_provider": refiner_provider,
        "integration_validator_provider": integration_validator_provider,
    }


def command_admit(args: argparse.Namespace) -> int:
    intent = absolute_path(args.intent, "intent", True)
    graph_path = absolute_path(args.graph, "graph", True)
    review_path = absolute_path(args.review, "review", True)
    result = admit_program(
        intent, graph_path, review_path, args.mayor_context, args.rig, args.binding,
        getattr(args, "delivery_mode", "pr"),
        getattr(args, "mayor_provider", "codex"),
        getattr(args, "reviewer_provider", "claude"),
        getattr(args, "refiner_provider", "codex"),
        getattr(args, "integration_validator_provider", "claude"),
    )
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
    mayor_provider = require_provider(args.mayor_provider, "mayor_provider")
    reviewer_provider = require_provider(
        args.reviewer_provider or opposite_provider(mayor_provider),
        "reviewer_provider",
    )
    refiner_provider = require_provider(args.refiner_provider, "refiner_provider")
    integration_validator_provider = require_provider(
        args.integration_validator_provider or opposite_provider(refiner_provider),
        "integration_validator_provider",
    )
    if mayor_provider == reviewer_provider:
        raise FactoryError("provider_collision", "plan review must use the opposite provider from the Mayor")
    if refiner_provider == integration_validator_provider:
        raise FactoryError(
            "provider_collision",
            "integration validation must use the opposite provider from the Refiner",
        )
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
        "provider": mayor_provider,
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
    admission = admit_program(
        intent, graph_path, review_path, mayor["session_context_id"], args.rig, args.binding,
        getattr(args, "delivery_mode", "pr"),
        mayor_provider, reviewer_provider, refiner_provider,
        integration_validator_provider,
    )
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
        "mayor_provider": mayor_provider,
        "reviewer_provider": reviewer_provider,
        "refiner_provider": refiner_provider,
        "integration_validator_provider": integration_validator_provider,
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
            run_process(factory_git_command("cherry-pick", predecessor_sha), cwd=worktree, timeout=120)
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


def remove_duplicate_factory_imports(rig_name: str, binding: str) -> list[str]:
    """Remove rig-local aliases for the city-inherited factory pack.

    `gc rig add` discovers packs in a worktree and may add the same pack under
    its manifest name even though the city already exposes it under the
    program's selected binding.  Both aliases materialize the same agents and
    skills, so suspension patches cannot make the resulting city healthy.
    """
    require_string(rig_name, "rig name", ID_RE)
    require_string(binding, "factory binding", ID_RE)
    city_config = Path(city_path()) / "city.toml"
    try:
        config = tomllib.loads(city_config.read_text(encoding="utf-8"))
    except (OSError, tomllib.TOMLDecodeError) as exc:
        raise FactoryError("city_config_invalid", f"cannot read city config {city_config}: {exc}") from exc
    rigs = [
        item for item in config.get("rigs", [])
        if isinstance(item, dict) and item.get("name") == rig_name
    ]
    if len(rigs) != 1:
        raise FactoryError("rig_missing", f"dynamic rig {rig_name!r} is not uniquely configured")

    canonical_pack = PACK_ROOT.resolve(strict=False)
    duplicate_bindings: list[str] = []
    imports = rigs[0].get("imports", {})
    if not isinstance(imports, dict):
        raise FactoryError("city_config_invalid", f"dynamic rig {rig_name!r} has invalid imports")
    for import_binding, import_spec in imports.items():
        if import_binding == binding or not isinstance(import_binding, str) or not isinstance(import_spec, dict):
            continue
        source = import_spec.get("source")
        if not isinstance(source, str) or not source:
            continue
        source_path = Path(source).expanduser()
        if not source_path.is_absolute():
            source_path = city_config.parent / source_path
        if source_path.resolve(strict=False) == canonical_pack:
            duplicate_bindings.append(import_binding)

    for duplicate in sorted(duplicate_bindings):
        output([
            gc_binary(), "--city", city_path(), "--rig", rig_name,
            "import", "remove", duplicate,
        ])
    return sorted(duplicate_bindings)


def enforce_rig_agent_policy(rig_name: str, binding: str, allowed_roles: set[str],
                             rig_root: Path) -> bool:
    """Make a dedicated worktree rig expose only the bead-selected role routes.

    Gas City composes city imports into every rig and injects provider targets.
    A dynamic candidate rig must therefore receive explicit rig-scoped patches;
    `gc agent suspend` cannot write patches for pack-derived agents.
    """
    require_string(rig_name, "rig name", ID_RE)
    require_string(binding, "factory binding", ID_RE)
    if not allowed_roles or not allowed_roles.issubset(
        {"implementer", "implementer-claude", "validator"}
    ):
        raise FactoryError("invalid_agent_policy", f"invalid allowed role set: {sorted(allowed_roles)}")
    rig_root = rig_root.resolve(strict=False)
    # Routed packet beads set gc.pack_workspace to the candidate directory
    # name. Gas City resolves that workspace relative to the configured agent
    # base, so patch the base to the candidate's parent. Using rig_root here
    # would launch every worker at <candidate>/<candidate>.
    session_work_dir = str(rig_root.parent)
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
        local_name = qualified.removeprefix(prefix)
        matches = [
            patch for patch in patches
            if isinstance(patch, dict)
            and patch.get("dir") == rig_name
            and patch.get("name") == local_name
        ]
        if qualified in expected:
            if matches:
                if (
                    len(matches) != 1
                    or matches[0].get("suspended") is True
                    or matches[0].get("work_dir") != session_work_dir
                ):
                    raise FactoryError(
                        "agent_policy_collision",
                        f"invalid workspace patch for bead-selected route {qualified}",
                    )
                continue
            additions.append(
                "[[patches.agent]]\n"
                f"dir = {json.dumps(rig_name)}\n"
                f"name = {json.dumps(local_name)}\n"
                f"work_dir = {json.dumps(session_work_dir)}"
            )
            continue
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
    wrong_work_dirs = {
        str(item.get("qualified_name", "")): item.get("work_dir")
        for item in refreshed
        if item.get("qualified_name") in expected and item.get("work_dir") != session_work_dir
    }
    if wrong_work_dirs:
        raise FactoryError(
            "agent_policy_mismatch",
            f"worktree rig executor roots differ from {session_work_dir!r}: {wrong_work_dirs!r}",
        )
    return bool(additions)


def controller_rig_agents(rig_name: str) -> list[dict[str, Any]]:
    """Read one rig's live controller projection through the native GC API."""
    completed = run_process([
        gc_binary(), "--city", city_path(), "rig", "status", rig_name, "--json",
    ], timeout=45, check=False)
    if completed.returncode != 0:
        detail = completed.stderr.strip() or completed.stdout.strip()
        raise FactoryError("rig_status_failed", f"cannot read live rig status for {rig_name}: {detail}")
    value = parse_json_output(completed.stdout, "gc rig status")
    agents = value.get("agents") if isinstance(value, dict) else None
    if not isinstance(agents, list):
        raise FactoryError("rig_status_failed", f"gc rig status returned no agents for {rig_name}")
    return [item for item in agents if isinstance(item, dict)]


def reload_city_config(rig_name: str, binding: str, allowed_roles: set[str], timeout: float = 120) -> None:
    """Publish a dynamic-rig policy and wait on GC's live status projection.

    ``gc rig add`` already performs its own controller reload.  The factory
    edits only the later pack-derived suspension patches, so request that
    second reload asynchronously and use ``gc rig status`` as the native
    completion signal.  This avoids coupling the program driver to all work a
    synchronous controller tick happens to perform under host pressure.
    """
    require_string(rig_name, "rig name", ID_RE)
    require_string(binding, "factory binding", ID_RE)
    expected = {f"{rig_name}/{binding}.{role}" for role in allowed_roles}
    deadline = time.monotonic() + timeout
    last_active: set[str] = set()
    last_error = ""
    polls_since_request = 5
    while time.monotonic() < deadline:
        try:
            agents = controller_rig_agents(rig_name)
            last_active = {
                str(item.get("qualified_name", ""))
                for item in agents
                if item.get("suspended") is not True
            }
            if last_active == expected:
                return
        except FactoryError as exc:
            last_error = str(exc)
        if polls_since_request >= 5:
            completed = run_process([
                gc_binary(), "--city", city_path(), "reload", "--async", "--json",
            ], timeout=45, check=False)
            value: Any = {}
            if completed.stdout.strip():
                value = parse_json_output(completed.stdout, "gc reload --async")
            outcome = value.get("outcome") if isinstance(value, dict) else None
            busy = outcome == "busy" or "another reload is already in progress" in completed.stderr
            accepted = (
                completed.returncode == 0
                and isinstance(value, dict)
                and value.get("ok") is True
                and outcome in {"accepted", "applied", "no_change"}
            )
            if not accepted and not busy:
                detail = completed.stderr.strip() or repr(value)
                raise FactoryError("city_reload_failed", f"dynamic rig config reload was not accepted: {detail}")
            polls_since_request = 0
        time.sleep(2.0)
        polls_since_request += 1
    detail = f"expected={sorted(expected)} active={sorted(last_active)}"
    if last_error:
        detail += f" last_error={last_error}"
    raise FactoryError("city_reload_failed", f"dynamic rig policy did not become live: {detail}")


def git_blob(ref: str, path: str, worktree: Path) -> bytes | None:
    completed = subprocess.run(
        ["git", "show", f"{ref}:{path}"], cwd=worktree,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False,
    )
    if completed.returncode == 0:
        return completed.stdout
    return None


def is_canonical_beads_gitignore_append(before: bytes, after: bytes) -> bool:
    header = b"# Beads / Dolt files (added by bd init)\n"
    variants = (
        (b".dolt/", b"*.db", b".beads-credential-key"),
        (b".dolt/", b"*.db", b".beads-credential-key", b".beads/proxieddb/"),
    )
    existing = {line.strip() for line in before.splitlines()}
    for patterns in variants:
        missing = [pattern for pattern in patterns if pattern not in existing]
        if not missing:
            continue
        expected = before
        if expected and not expected.endswith(b"\n"):
            expected += b"\n"
        expected += b"\n" + header + b"".join(pattern + b"\n" for pattern in missing)
        if after == expected:
            return True
    return False


def install_worktree_runtime_excludes(worktree: Path) -> Path:
    """Hide an ephemeral rig store without changing the product tree.

    Gas City intentionally writes a project ``.gitignore`` for ordinary rigs.
    Factory rigs are disposable product worktrees, so use Git's native
    worktree-local config instead. This preserves any previously configured
    excludes while keeping every ``.beads`` runtime file out of worker diffs.
    """
    existing = run_process(
        ["git", "config", "--get", "core.excludesFile"],
        cwd=worktree, check=False,
    ).stdout.strip()
    run_process(["git", "config", "extensions.worktreeConfig", "true"], cwd=worktree)
    git_dir = Path(output(["git", "rev-parse", "--absolute-git-dir"], cwd=worktree))
    exclude_path = git_dir / "agentops-factory-exclude"
    inherited = b""
    if existing:
        inherited_path = Path(os.path.expanduser(existing))
        if not inherited_path.is_absolute():
            inherited_path = worktree / inherited_path
        if inherited_path.resolve(strict=False) != exclude_path.resolve(strict=False):
            try:
                inherited = inherited_path.read_bytes()
            except FileNotFoundError:
                inherited = b""
    rendered = inherited
    if rendered and not rendered.endswith(b"\n"):
        rendered += b"\n"
    if b".beads/" not in {line.strip() for line in rendered.splitlines()}:
        rendered += b".beads/\n"
    write_text_atomic(exclude_path, rendered.decode("utf-8"))
    run_process(
        ["git", "config", "--worktree", "core.excludesFile", str(exclude_path)],
        cwd=worktree,
    )
    return exclude_path


def discard_transient_rig_init_commits(worktree: Path, expected_head: str) -> list[str]:
    """Rewind only canonical Git changes created by ``gc rig add``."""
    base = require_string(expected_head, "candidate base SHA", SHA_RE)
    current = git_head(worktree)
    before_gitignore = git_blob(base, ".gitignore", worktree) or b""
    changed: list[str] = []
    if current != base:
        ancestor = run_process(
            ["git", "merge-base", "--is-ancestor", base, current],
            cwd=worktree, check=False,
        )
        if ancestor.returncode != 0:
            raise FactoryError("rig_init_commit_unexpected", "gc rig add moved the candidate away from its base ancestry")
        subjects = output(["git", "log", "--format=%s", f"{base}..{current}"], cwd=worktree).splitlines()
        changed = output(["git", "diff", "--name-only", base, current], cwd=worktree).splitlines()
        allowed_paths = all(path == ".gitignore" or path.startswith(".beads/") for path in changed)
        if not subjects or any(subject != "bd init: initialize beads issue tracking" for subject in subjects) or not allowed_paths:
            raise FactoryError(
                "rig_init_commit_unexpected",
                f"gc rig add created unexpected commits or paths: subjects={subjects!r} paths={changed!r}",
            )
        after_gitignore = git_blob(current, ".gitignore", worktree)
        if ".gitignore" in changed and not is_canonical_beads_gitignore_append(
            before_gitignore, after_gitignore or b"",
        ):
            raise FactoryError("rig_init_commit_unexpected", "bd init changed .gitignore outside its canonical append-only stanza")
        run_process(["git", "update-ref", "-m", "gc rig add: discard transient bd init commit", "HEAD", base, current], cwd=worktree)
        run_process(["git", "read-tree", f"{base}^{{tree}}"], cwd=worktree)

    staged = output(["git", "diff", "--cached", "--name-only"], cwd=worktree).splitlines()
    allowed_staged = all(path == ".gitignore" or path.startswith(".beads/") for path in staged)
    if not allowed_staged:
        raise FactoryError("rig_init_index_unexpected", f"gc rig add staged unexpected paths: {staged!r}")
    if ".gitignore" in staged:
        staged_gitignore = run_process(
            ["git", "show", ":.gitignore"], cwd=worktree, check=False,
        )
        if staged_gitignore.returncode != 0 or not is_canonical_beads_gitignore_append(
            before_gitignore, staged_gitignore.stdout.encode(),
        ):
            raise FactoryError("rig_init_index_unexpected", "bd init staged .gitignore outside its canonical append-only stanza")
    if staged:
        run_process(["git", "read-tree", f"{base}^{{tree}}"], cwd=worktree)

    project_gitignore = worktree / ".gitignore"
    if git_blob(base, ".gitignore", worktree) is None:
        project_gitignore.unlink(missing_ok=True)
    else:
        project_gitignore.write_bytes(before_gitignore)
    install_worktree_runtime_excludes(worktree)
    tracked_beads = [
        path for path in run_process(["git", "ls-files", "-z", "--", ".beads"], cwd=worktree).stdout.split("\0")
        if path
    ]
    if tracked_beads:
        run_process(["git", "update-index", "--skip-worktree", "--", *tracked_beads], cwd=worktree)
    return sorted(set(changed + staged))


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


def beads_config_prefix(path: Path) -> str | None:
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except FileNotFoundError:
        return None
    except OSError as exc:
        raise FactoryError("beads_identity_invalid", f"cannot read {path}: {exc}") from exc
    values: list[str] = []
    for line in lines:
        match = re.match(r"^\s*issue[-_]prefix\s*:\s*([^#]+?)\s*$", line)
        if match:
            value = match.group(1).strip().strip("\"'").lower()
            if value:
                values.append(value)
    if not values:
        return None
    if len(set(values)) != 1:
        raise FactoryError("beads_identity_invalid", f"conflicting issue prefixes in {path}")
    return values[0]


def prepare_worktree_beads_identity(worktree: Path, requested_prefix: str) -> bool:
    """Fence a linked worktree from its tracked parent Beads identity.

    Git copies tracked `.beads` control files into every linked worktree. The
    identity markers must not be adopted by a factory candidate rig, and GC's
    local interaction log must not make the candidate Git subject dirty. Hide
    every tracked `.beads` path in this worktree's private index, then remove
    only the two identity-marker working copies so `gc rig add` can initialize
    a distinct store. On a retry, matching marker bytes prove that the
    candidate store itself can be adopted.
    """
    requested = require_string(requested_prefix, "candidate beads prefix", ID_RE).lower()
    marker_paths = [Path(".beads/config.yaml"), Path(".beads/metadata.json")]
    markers = [worktree / relative for relative in marker_paths]
    present = [path.is_file() for path in markers]
    current_prefix = beads_config_prefix(markers[0])
    marker_tracked = [
        run_process(
            ["git", "ls-files", "--error-unmatch", "--", relative.as_posix()],
            cwd=worktree, check=False,
        ).returncode == 0
        for relative in marker_paths
    ]
    if all(marker_tracked):
        listed = run_process(
            ["git", "ls-files", "-z", "--", ".beads"],
            cwd=worktree,
        ).stdout
        tracked_beads = sorted(path for path in listed.split("\0") if path)
        if not tracked_beads:
            raise FactoryError("beads_identity_invalid", "tracked parent Beads identity has no index paths")
        run_process(
            ["git", "update-index", "--skip-worktree", "--", *tracked_beads],
            cwd=worktree,
        )
        if all(present) and current_prefix == requested:
            return True
        for marker in markers:
            try:
                marker.unlink()
            except FileNotFoundError:
                pass
        return False

    if all(present) and current_prefix == requested:
        return True

    if any(present):
        found = current_prefix or "unknown"
        raise FactoryError(
            "beads_identity_collision",
            f"candidate worktree has an unowned Beads identity {found!r}; expected {requested!r}",
        )
    return False


def add_gc_rig_without_forge_origin(worktree: Path, argv: list[str]) -> Any:
    """Initialize per-worktree beads without synthesizing a Dolt remote.

    Git worktrees share remote configuration. Registration is serialized, so
    the complete origin section is parked only for the duration of `gc rig add`,
    then restored. The reserved section also makes an interrupted registration
    recoverable on the next attempt. The caller supplies ``--default-branch``
    explicitly; a local filesystem URL is not a valid substitute because bd may
    adopt it as a Dolt remote and misinterpret it as a DoltHub repository
    identifier.
    """
    common_dir_raw = output(["git", "rev-parse", "--git-common-dir"], cwd=worktree)
    common_dir = Path(common_dir_raw)
    if not common_dir.is_absolute():
        common_dir = (worktree / common_dir).resolve(strict=False)
    config_path = common_dir / "config"
    parked_remote = "agentops-factory-origin"

    def remote_section_exists(name: str) -> bool:
        probe = run_process(
            [
                "git", "config", "--file", str(config_path), "--get-regexp",
                rf"^remote\.{re.escape(name)}\.",
            ],
            cwd=worktree, check=False,
        )
        return probe.returncode == 0

    parked = remote_section_exists(parked_remote)
    origin = remote_section_exists("origin")
    if parked and origin:
        raise FactoryError(
            "origin_recovery_ambiguous",
            f"repository has both remote.origin and reserved remote.{parked_remote}",
        )
    if parked:
        run_process([
            "git", "config", "--file", str(config_path), "--rename-section",
            f"remote.{parked_remote}", "remote.origin",
        ], cwd=worktree)
        origin = True
    try:
        prefix_index = argv.index("--prefix")
        requested_prefix = argv[prefix_index + 1]
        name_index = argv.index("--name")
        rig_name = argv[name_index + 1]
    except (ValueError, IndexError) as exc:
        raise FactoryError("invalid_contract", "dynamic rig registration requires explicit --name and --prefix") from exc
    adopt = prepare_worktree_beads_identity(worktree, requested_prefix)
    command = [*argv, "--adopt"] if adopt and "--adopt" not in argv else argv
    origin_parked = False
    if origin:
        run_process([
            "git", "config", "--file", str(config_path), "--rename-section",
            "remote.origin", f"remote.{parked_remote}",
        ], cwd=worktree)
        origin_parked = True
    try:
        raw = output(
            command,
            env={"BD_DOLT_SYNC_CLI_REMOTES": "false", "BEADS_DOLT_SYNC_CLI_REMOTES": "false"},
        )
        value = parse_json_output(raw, "gc rig add")
        # gc rig add can return before the rig's store is first opened. Force
        # that first open while the forge origin is still hidden, then fence
        # the ephemeral rig to local-only Dolt state.
        enforce_dynamic_rig_local_only(rig_name)
        return value
    finally:
        if origin_parked:
            if remote_section_exists("origin"):
                raise FactoryError(
                    "origin_restore_collision",
                    "gc rig add created remote.origin while the original section was parked",
                )
            run_process([
                "git", "config", "--file", str(config_path), "--rename-section",
                f"remote.{parked_remote}", "remote.origin",
            ], cwd=worktree)


def enforce_dynamic_rig_local_only(rig_name: str) -> None:
    """Force-init an ephemeral rig and remove every synthesized Dolt remote."""
    rig = require_string(rig_name, "dynamic rig", ID_RE)
    env = {"BD_DOLT_SYNC_CLI_REMOTES": "false", "BEADS_DOLT_SYNC_CLI_REMOTES": "false"}
    prefix = [gc_binary(), "--city", city_path(), "bd", "--rig", rig]
    output([*prefix, "list", "--json"], env=env)
    listed = parse_json_output(
        output([*prefix, "dolt", "remote", "list", "--json"], env=env),
        "dynamic rig dolt remote list",
    )
    if not isinstance(listed, list):
        raise FactoryError("invalid_runtime_json", "dynamic rig Dolt remote list is not an array")
    names: list[str] = []
    for item in listed:
        if not isinstance(item, dict):
            raise FactoryError("invalid_runtime_json", "dynamic rig Dolt remote entry is not an object")
        names.append(require_string(item.get("name"), "dynamic rig Dolt remote name", ID_RE))
    for name in names:
        output([*prefix, "dolt", "remote", "remove", name, "--json"], env=env)
    remaining = parse_json_output(
        output([*prefix, "dolt", "remote", "list", "--json"], env=env),
        "dynamic rig Dolt remote verification",
    )
    if remaining != []:
        raise FactoryError("dolt_remote_leak", f"dynamic rig retained Dolt remotes: {remaining!r}")


def candidate_rig_prefix(rig_name: str) -> str:
    """Return a short Beads prefix unique to the full candidate rig identity."""
    return safe_identifier(
        "fx", "candidate",
        require_string(rig_name, "candidate rig", ID_RE),
        limit=16,
    )


def register_candidate_rig(lease: dict[str, Any], record: dict[str, Any]) -> tuple[str, str]:
    meta = metadata(record)
    rig_name = safe_identifier("fx", meta["factory.program_id"], meta["factory.node_id"], str(meta.get("factory.attempt", "1")))
    recorded_rig = meta.get("factory.candidate_rig")
    if recorded_rig is not None and recorded_rig != rig_name:
        raise FactoryError(
            "identity_mismatch",
            f"experiment records candidate rig {recorded_rig!r}, not derived rig {rig_name!r}",
        )
    registration_previously_completed = recorded_rig == rig_name
    worktree = Path(lease["worktree"])
    binding = require_string(meta.get("factory.binding"), "factory.binding", ID_RE)
    with city_config_lock():
        matches = [item for item in configured_rigs() if item.get("name") == rig_name]
        rig_added = False
        if matches:
            if len(matches) != 1 or Path(str(matches[0].get("path", ""))).resolve(strict=False) != worktree:
                raise FactoryError("rig_collision", f"candidate rig name {rig_name} is already bound elsewhere")
            enforce_dynamic_rig_local_only(rig_name)
        else:
            value = add_gc_rig_without_forge_origin(
                worktree,
                [
                    gc_binary(), "--city", city_path(), "rig", "add", str(worktree),
                    "--name", rig_name, "--prefix", candidate_rig_prefix(rig_name),
                    "--default-branch", lease["branch"], "--json",
                ],
            )
            if isinstance(value, dict) and value.get("ok") is False:
                raise FactoryError("rig_add_failed", f"candidate rig add failed: {value!r}")
            rig_added = True
        provider = require_string(meta.get("factory.provider"), "factory.provider")
        validator_provider = require_string(meta.get("factory.validator_provider"), "factory.validator_provider")
        worker_role = "implementer-claude" if provider == "claude" else "implementer"
        if validator_provider != "codex":
            raise FactoryError("unroutable_role", "3.3 candidate validation is Sol-high/Codex only")
        validator_role = "validator"
        duplicate_imports = remove_duplicate_factory_imports(rig_name, binding)
        policy_changed = enforce_rig_agent_policy(
            rig_name, binding, {worker_role, validator_role}, worktree,
        )
        # The bead records factory.candidate_rig only after a successful reload.
        # Until that proof exists, reload even if recovery finds the expected
        # bytes already on disk. Once proven, avoid a redundant synchronous
        # reload unless registration or policy actually changed.
        if not registration_previously_completed or rig_added or duplicate_imports or policy_changed:
            reload_city_config(rig_name, binding, {worker_role, validator_role})
    # The caller writes factory.candidate_rig only after this cleanup returns.
    # Its exact value is therefore the durable proof that first registration
    # completed. On recovery, preserve legitimate staged/committed candidate
    # bytes instead of misclassifying them as fresh rig scaffolding.
    if not registration_previously_completed:
        discard_transient_rig_init_commits(worktree, require_string(lease.get("candidate_base_sha"), "candidate base SHA", SHA_RE))
        restore_rig_scaffolding(worktree)
    return rig_name, binding


def integration_rig_prefix(refinery_bead: str, epoch: str) -> str:
    """Return a short Beads prefix unique to one Refinery integration epoch."""
    return safe_identifier(
        "fx", "ref",
        require_string(refinery_bead, "Refinery bead", ID_RE),
        require_string(epoch, "integration branch epoch", ID_RE),
        limit=16,
    )


def register_integration_rig(worktree: Path, refinery_bead: str, branch: str, binding: str,
                             recorded_rig: str | None = None) -> tuple[str, str]:
    epoch = require_string(branch.rsplit("/", 1)[-1], "integration branch epoch", ID_RE)
    rig_name = safe_identifier("fx", "refinery", refinery_bead, epoch)
    if recorded_rig is not None and recorded_rig != rig_name:
        raise FactoryError(
            "identity_mismatch",
            f"Refinery records integration rig {recorded_rig!r}, not derived rig {rig_name!r}",
        )
    binding = require_string(binding, "factory.binding", ID_RE)
    pre_registration_head = git_head(worktree)
    with city_config_lock():
        matches = [item for item in configured_rigs() if item.get("name") == rig_name]
        rig_added = False
        if matches:
            if len(matches) != 1 or Path(str(matches[0].get("path", ""))).resolve(strict=False) != worktree:
                raise FactoryError("rig_collision", f"integration rig name {rig_name} is already bound elsewhere")
            enforce_dynamic_rig_local_only(rig_name)
        else:
            value = add_gc_rig_without_forge_origin(worktree, [
                gc_binary(), "--city", city_path(), "rig", "add", str(worktree),
                "--name", rig_name, "--prefix", integration_rig_prefix(refinery_bead, epoch),
                "--default-branch", branch, "--json",
            ])
            if isinstance(value, dict) and value.get("ok") is False:
                raise FactoryError("rig_add_failed", f"integration rig add failed: {value!r}")
            rig_added = True
        duplicate_imports = remove_duplicate_factory_imports(rig_name, binding)
        policy_changed = enforce_rig_agent_policy(
            rig_name, binding, {"validator"}, worktree,
        )
        # factory.integration_rig is written only after the first successful
        # reload. A later validation pass can therefore verify the rig and skip
        # the second five-minute controller round trip when nothing changed.
        if recorded_rig is None or rig_added or duplicate_imports or policy_changed:
            reload_city_config(rig_name, binding, {"validator"})
    discard_transient_rig_init_commits(worktree, pre_registration_head)
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


def executor_session_attestation(result: dict[str, Any], role: str, provider: str) -> dict[str, str]:
    """Validate and return the exact executor runtime/model identity."""
    expected_policy, expected_model = FACTORY_ROLE_MODELS[(role, provider)]
    session = result.get("runtime_evidence", {}).get("session")
    if not isinstance(session, dict):
        raise FactoryError("runtime_identity_missing", f"{role} result has no runtime session attestation")
    context_id = require_string(
        result.get("transport", {}).get("session_context_id"),
        f"{role} session context",
    )
    expected = {
        "id": context_id,
        "provider": provider,
        "model": expected_model,
        "model_policy": expected_policy,
        "model_source": "launch_command",
    }
    for field, wanted in expected.items():
        if session.get(field) != wanted:
            raise FactoryError(
                "runtime_model_mismatch" if field in {"model", "model_policy", "model_source"} else "runtime_identity_mismatch",
                f"{role} runtime {field} must be {wanted!r}, got {session.get(field)!r}",
            )
    return {field: str(value) for field, value in expected.items()}


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
    implement_attestation = executor_session_attestation(
        implement_result, "implement", require_string(meta.get("factory.provider"), "factory.provider"),
    )
    implement_result_path = evidence / "implement-runtime-result.json"
    write_or_verify_json(implement_result_path, implement_result, "implement runtime result")
    runtime = implement_result.get("runtime_evidence", {})
    scope_status = runtime.get("scope_status")
    if scope_status not in VERDICTS:
        raise FactoryError("scope_failed", f"implementer scope is {scope_status}")
    changed = runtime.get("actual_changed_paths")
    if not isinstance(changed, list) or not all(isinstance(path, str) and path for path in changed):
        raise FactoryError("invalid_candidate", "implementer changed-path evidence is malformed")
    first_check = require_string(meta.get("factory.first_check"), "factory.first_check")
    check = run_process(["/bin/sh", "-lc", first_check], cwd=worktree, timeout=600, check=False)
    check_path = evidence / "first-check.json"
    write_json_atomic(check_path, {
        "command": first_check, "exit_code": check.returncode,
        "stdout": check.stdout[-20000:], "stderr": check.stderr[-20000:],
    })
    beads.update_metadata(bead_id, {
        "factory.first_check_path": str(check_path),
        "factory.first_check_digest": digest_file(check_path),
        "factory.first_check_exit_code": str(check.returncode),
    })
    candidate_gate_status = scope_status if check.returncode == 0 else "FAIL"
    current_head = git_head(worktree)
    candidate_base_sha = require_string(meta.get("factory.candidate_base_sha"), "factory.candidate_base_sha", SHA_RE)
    if current_head == candidate_base_sha:
        if changed:
            run_process(["git", "add", "-A", "--", *changed], cwd=worktree)
            staged = output(["git", "diff", "--cached", "--name-only"], cwd=worktree).splitlines()
            if sorted(staged) != sorted(changed):
                raise FactoryError("stage_mismatch", f"staged paths differ from runtime receipt: staged={staged} changed={changed}")
            run_process(
                factory_git_command("commit", "-m", f"factory({meta['factory.node_id']}): frozen candidate"),
                cwd=worktree,
                timeout=120,
            )
        elif output(["git", "status", "--porcelain", "--untracked-files=no"], cwd=worktree):
            raise FactoryError("candidate_dirty", "empty changed-path receipt disagrees with tracked worktree state")
    else:
        commit_count = output(["git", "rev-list", "--count", f"{candidate_base_sha}..HEAD"], cwd=worktree)
        committed = output(["git", "diff", "--name-only", f"{candidate_base_sha}..HEAD"], cwd=worktree).splitlines()
        if commit_count != "1" or sorted(committed) != sorted(changed):
            raise FactoryError("candidate_recovery_mismatch", "existing candidate commit does not match the runtime changed-path receipt")
        if output(["git", "status", "--porcelain", "--untracked-files=no"], cwd=worktree):
            raise FactoryError("candidate_dirty", "recovered candidate worktree has tracked changes")
    candidate_sha = git_head(worktree)
    author_context = implement_attestation["id"]
    beads.update_metadata(bead_id, {
        "factory.execution_phase": "validation_pending",
        "factory.candidate_sha": candidate_sha,
        "factory.implement_result": str(implement_result_path),
        "factory.runtime_scope_status": scope_status,
        "factory.implementation_scope_status": candidate_gate_status,
        "factory.author_context_id": author_context,
        "factory.author_model": implement_attestation["model"],
        "factory.author_model_policy": implement_attestation["model_policy"],
        "factory.author_model_source": implement_attestation["model_source"],
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
    validate_attestation = executor_session_attestation(
        validate_result, "validate", require_string(meta.get("factory.validator_provider"), "factory.validator_provider"),
    )
    validate_result_path = evidence / "validate-runtime-result.json"
    write_or_verify_json(validate_result_path, validate_result, "validate runtime result")
    artifacts = validate_result.get("agent_response", {}).get("artifacts")
    if not isinstance(artifacts, list) or len(artifacts) != 1 or not isinstance(artifacts[0], dict):
        raise FactoryError("verdict_missing", "Validator returned no exact verdict artifact")
    verdict_path = absolute_path(artifacts[0].get("path"), "verdict artifact", True)
    verdict_artifact = load_object(verdict_path, "verdict artifact")
    verdict_value = verdict_artifact.get("verdict")
    if verdict_artifact.get("validator_context_id") != validate_attestation["id"]:
        raise FactoryError(
            "runtime_identity_mismatch",
            "verdict validator context does not match the attested Validator runtime",
        )
    if candidate_gate_status != "PASS" and verdict_value == "PASS":
        raise FactoryError(
            "verdict_binding_mismatch",
            "Validator cannot PASS a candidate whose deterministic candidate gate "
            f"is {candidate_gate_status} (scope={scope_status}, first_check={check.returncode})",
        )
    beads.update_metadata(bead_id, {
        "factory.validator_context_id": validate_attestation["id"],
        "factory.validator_model": validate_attestation["model"],
        "factory.validator_model_policy": validate_attestation["model_policy"],
        "factory.validator_model_source": validate_attestation["model_source"],
    })
    verdict_args = argparse.Namespace(
        rig=base_rig, bead=bead_id, lease_token=lease["lease_token"],
        fence_epoch=lease["fence_epoch"], candidate_sha=candidate_sha,
        subject_manifest=runtime["subject_manifest"], author_context=author_context,
        verdict=str(verdict_path),
    )
    # This reducer runs inside command_execute's worker pool. Redirecting
    # sys.stdout here is process-global, so overlapping lanes can restore a
    # sibling's already-closed sink and make the parent result print fail.
    command_record_verdict(verdict_args, emit=False)
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
        "validator_context_id": validate_attestation["id"],
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


def route_ready_refinery(beads: Beads, rig: str, program_meta: dict[str, Any]) -> str | None:
    """Keep the salvaged v1 executor from waking any model for delivery.

    GC33-3 has no model-facing delivery lane.  The crash-only delivery reducer
    is introduced by later beads through a new entry point; this legacy helper
    deliberately performs no Beads, GC, or session operation.
    """
    del beads, rig, program_meta
    return None


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
    refinery_ready = program_meta.get("factory.refinery_bead") in beads.ready_ids()
    refinery_route = route_ready_refinery(beads, args.rig, program_meta) if refinery_ready else None
    result = {
        "schema_version": "factory-execution-result.v1",
        "program_bead": args.program_bead,
        "waves": waves,
        "executed": sum(len(wave) for wave in waves),
        "reconciled": recoveries,
        "refinery_ready": refinery_ready,
        "refinery_route": refinery_route,
    }
    if not waves and not recoveries and not refinery_route:
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
    program_bead = require_string(meta.get("factory.program_bead"), "factory.program_bead")
    program = beads.show(program_bead)
    program_meta = metadata(program)
    refinery_bead = require_string(program_meta.get("factory.refinery_bead"), "factory.refinery_bead")
    refinery_ready = refinery_bead in beads.ready_ids()
    refinery_route = route_ready_refinery(beads, args.rig, program_meta) if refinery_ready else None
    result["refinery_ready"] = refinery_ready
    result["refinery_route"] = refinery_route
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


def validate_recorded_executor_attestations(meta: dict[str, Any], author_context: str,
                                            validator_context: str) -> None:
    for role, prefix, provider_key, context in (
        ("implement", "author", "factory.provider", author_context),
        ("validate", "validator", "factory.validator_provider", validator_context),
    ):
        provider = require_provider(meta.get(provider_key), provider_key)
        expected_policy, expected_model = FACTORY_ROLE_MODELS[(role, provider)]
        expected = {
            f"factory.{prefix}_context_id": context,
            f"factory.{prefix}_model": expected_model,
            f"factory.{prefix}_model_policy": expected_policy,
            f"factory.{prefix}_model_source": "launch_command",
        }
        for field, wanted in expected.items():
            if meta.get(field) != wanted:
                raise FactoryError(
                    "runtime_attestation_missing",
                    f"durable {role} attestation {field} must be {wanted!r}, got {meta.get(field)!r}",
                )


def command_record_verdict(args: argparse.Namespace, *, emit: bool = True) -> int:
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
    validate_recorded_executor_attestations(meta, args.author_context, validator)
    if result == "PASS":
        if meta.get("factory.implementation_scope_status") != "PASS":
            raise FactoryError(
                "admission_gate_failed",
                "PASS requires a durable implementation_scope_status=PASS",
            )
        first_check_exit_code = meta.get("factory.first_check_exit_code")
        first_check_passed = (
            first_check_exit_code == "0"
            or (
                isinstance(first_check_exit_code, int)
                and not isinstance(first_check_exit_code, bool)
                and first_check_exit_code == 0
            )
        )
        if not first_check_passed:
            raise FactoryError(
                "admission_gate_failed",
                "PASS requires a durable first_check_exit_code=0",
            )
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
            beads.close(
                args.bead, "Factory experiment PASS: exact candidate admitted to Refinery",
                "shipped", candidate_sha,
                require_string(meta.get("factory.branch"), "factory.branch"), worktree,
            )
        refinery = require_string(meta.get("factory.refinery_bead"), "factory.refinery_bead")
        if refinery in beads.ready_ids():
            beads.update_metadata(refinery, {"factory.status": "ready"})
        if emit:
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
        beads.close(
            args.bead, f"Factory experiment {result}: returned to Mayor as {rescope}",
            "blocked",
        )
    if emit:
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
            command_successor(argparse.Namespace(
                rig=rig,
                rejected_bead=rescope_meta["factory.rejected_bead"],
                rescope_bead=rescope_bead,
                proposal=str(proposal_path),
            ), emit=False)
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
        command_successor(argparse.Namespace(
            rig=rig,
            rejected_bead=rescope_meta["factory.rejected_bead"],
            rescope_bead=rescope_bead,
            proposal=str(proposal_path),
        ), emit=False)
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
        "provider": require_provider(program_meta.get("factory.mayor_provider"), "factory.mayor_provider"),
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
    command_successor(argparse.Namespace(
        rig=rig, rejected_bead=rejected_bead, rescope_bead=rescope_bead,
        proposal=str(proposal_path),
    ), emit=False)
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


def command_successor(args: argparse.Namespace, *, emit: bool = True) -> int:
    beads = Beads(args.rig)
    rejected = beads.show(args.rejected_bead)
    rejected_meta = metadata(rejected)
    rescope = beads.show(args.rescope_bead)
    rescope_meta = metadata(rescope)
    if (
        rejected_meta.get("factory.status") != "rejected"
        or rejected_meta.get("factory.verdict") not in {"FAIL", "NOT_PROVEN"}
        or rejected.get("status") != "closed"
    ):
        raise FactoryError("invalid_successor", "successor source is not a terminal closed rejected experiment")
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
        "first_check", "provider", "worker_model_policy", "validator_provider",
        "validator_model_policy",
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
        beads.close(args.rescope_bead, f"Mayor successor admitted as {successor}", "no-op")
    if emit:
        print(json.dumps({"rejected_bead": args.rejected_bead, "rescope_bead": args.rescope_bead, "successor_bead": successor}, sort_keys=True))
    return 0


def attest_refiner_claim(beads: Beads, rig: str, refinery_bead: str,
                         refinery: dict[str, Any], meta: dict[str, Any]) -> dict[str, Any]:
    """Require and attest the routed Refiner that owns a delivery transition."""
    if meta.get("factory.kind") != "refinery" or refinery.get("status") != "in_progress":
        raise FactoryError("refinery_claim_invalid", "delivery requires a Refiner-claimed Refinery bead")
    refiner_provider = require_provider(meta.get("factory.refiner_provider"), "factory.refiner_provider")
    binding = require_string(meta.get("factory.binding"), "factory.binding", ID_RE)
    expected_route = lifecycle_target("refiner", refiner_provider, rig, binding)
    session_name = require_string(os.environ.get("GC_SESSION_NAME"), "GC_SESSION_NAME")
    recorded_session_name = meta.get("gc.session_name")
    if (
        meta.get("gc.routed_to") != expected_route
        or refinery.get("assignee") != session_name
        or (recorded_session_name is not None and recorded_session_name != session_name)
    ):
        raise FactoryError("refinery_claim_invalid", "Refinery bead is not owned by its routed Refiner session")
    session_id = require_string(os.environ.get("GC_SESSION_ID"), "GC_SESSION_ID")
    session = runtime_session(session_id)
    configured_model, launched_model = FACTORY_ROLE_MODELS[("refiner", refiner_provider)]
    for field, wanted in {
        "session_name": session_name,
        "template": expected_route,
        "provider": refiner_provider,
        "model": launched_model,
    }.items():
        if session.get(field) != wanted:
            raise FactoryError(
                "runtime_identity_mismatch" if field != "model" else "runtime_model_mismatch",
                f"Refiner runtime {field} must be {wanted!r}, got {session.get(field)!r}",
            )
    attestation = {
        "factory.refiner_context_id": session_id,
        "factory.refiner_model": session["model"],
        "factory.refiner_model_policy": configured_model,
        "factory.refiner_model_source": session["model_source"],
    }
    for field, value in attestation.items():
        if field in meta and meta[field] != value:
            raise FactoryError("runtime_identity_mismatch", f"Refinery {field} changed after attestation")
    beads.update_metadata(refinery_bead, attestation)
    return {**meta, **attestation}


def command_refinery_assemble(args: argparse.Namespace) -> int:
    beads = Beads(args.rig)
    refinery = beads.show(args.refinery_bead)
    meta = metadata(refinery)
    if meta.get("factory.kind") != "refinery":
        raise FactoryError("not_refinery", "only a Refinery bead may assemble")
    factory_phase = meta.get("factory.status")
    if factory_phase not in {"blocked", "ready", "assembling", "reassembly_required"}:
        raise FactoryError("invalid_transition", f"Refinery cannot assemble from phase {factory_phase!r}")
    refiner_provider = require_provider(meta.get("factory.refiner_provider"), "factory.refiner_provider")
    integration_validator_provider = require_provider(
        meta.get("factory.integration_validator_provider"),
        "factory.integration_validator_provider",
    )
    if refiner_provider == integration_validator_provider:
        raise FactoryError("provider_collision", "Refiner and integration Validator must use opposite providers")
    unresolved = [
        str(item.get("id"))
        for item in refinery.get("dependencies", [])
        if isinstance(item, dict)
        and item.get("dependency_type", "blocks") in {"blocks", "waits-for", "conditional-blocks"}
        and item.get("status") != "closed"
    ]
    if unresolved:
        raise FactoryError("refinery_blocked", f"Refinery bead has unresolved dependencies: {sorted(unresolved)}")
    meta = attest_refiner_claim(beads, args.rig, args.refinery_bead, refinery, meta)
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
    if preparing:
        abort_interrupted_factory_cherry_pick(worktree)
    if output(["git", "status", "--porcelain", "--untracked-files=no"], cwd=worktree):
        raise FactoryError("integration_dirty", "preparing integration worktree has tracked changes")
    binding = require_string(meta.get("factory.binding"), "factory.binding", ID_RE)
    integration_rig, binding = register_integration_rig(
        worktree, args.refinery_bead, branch, binding,
        meta.get("factory.integration_rig") if preparing else None,
    )
    recorded_scaffold = meta.get("factory.integration_scaffold_sha") if preparing else None
    if recorded_scaffold is not None:
        scaffold_sha = require_string(recorded_scaffold, "factory.integration_scaffold_sha", SHA_RE)
        if run_process(
            ["git", "merge-base", "--is-ancestor", scaffold_sha, "HEAD"],
            cwd=worktree, check=False,
        ).returncode != 0:
            raise FactoryError("integration_moved", "integration rig scaffolding is not an ancestor of the recovering branch")
    else:
        scaffold_sha = git_head(worktree)
    beads.update_metadata(args.refinery_bead, {
        "factory.integration_rig": integration_rig,
        "factory.executor_binding": binding,
        "factory.integration_scaffold_sha": scaffold_sha,
    })
    meta.update({
        "factory.integration_rig": integration_rig,
        "factory.executor_binding": binding,
        "factory.integration_scaffold_sha": scaffold_sha,
    })
    evidence = worktree / ".gc" / "agentops-factory" / args.refinery_bead
    evidence.mkdir(parents=True, exist_ok=True)
    executor = load_executor_adapter()
    # Node-local exclusions prevent a candidate from touching a sibling's
    # product while experiments run in isolation. Once certified candidates are
    # assembled, any path included by one node is part of the integrated
    # subject even if another node excluded it locally. Carry forward only
    # exclusions that are not an integration include, or the combined manifest
    # can erase every independently-owned product and become NOT_PROVEN.
    integration_excludes = subject_excludes.difference(subject_includes)
    manifest_packet = {
        "packet_id": safe_identifier(args.refinery_bead, "integration", limit=120),
        "role": "implement",
        "subject": {"includes": sorted(subject_includes), "excludes": sorted(integration_excludes)},
        "write_scope": sorted(integration_scope),
    }
    baseline_path = evidence / "integration-baseline-manifest.json"
    if baseline_path.is_file():
        baseline = load_object(baseline_path, "integration baseline manifest")
        stored_baseline_digest = meta.get("factory.integration_baseline_digest")
        if stored_baseline_digest and digest_file(baseline_path) != stored_baseline_digest:
            raise FactoryError("manifest_mutated", "integration baseline manifest changed during assembly")
    else:
        if git_head(worktree) != scaffold_sha:
            raise FactoryError("assembly_recovery_missing", "integration moved before its baseline manifest was persisted")
        baseline = executor.build_manifest(manifest_packet, {"workspace": worktree}, baseline_path)
    beads.update_metadata(args.refinery_bead, {
        "factory.integration_baseline_manifest": str(baseline_path),
        "factory.integration_baseline_digest": digest_file(baseline_path),
    })
    with refinery_merge_slot(beads, args.refinery_bead, args.merge_slot_timeout) as slot:
        for _bead_id, candidate_sha in candidates:
            if not commit_patch_present(worktree, candidate_sha):
                run_process(factory_git_command("cherry-pick", candidate_sha), cwd=worktree, timeout=120)
        integration_sha = git_head(worktree)
        subject_path = evidence / "integration-subject-manifest.json"
        subject_manifest = executor.build_manifest(
            manifest_packet, {"workspace": worktree}, subject_path, baseline_path,
        )
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
            "factory.merge_slot_id": slot["id"],
            "factory.merge_slot_holder": slot["holder"],
        })
    beads.update_metadata(args.refinery_bead, {"factory.merge_slot_released": "true"})
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
    configured_provider = require_provider(
        meta.get("factory.integration_validator_provider"),
        "factory.integration_validator_provider",
    )
    refiner_provider = require_provider(meta.get("factory.refiner_provider"), "factory.refiner_provider")
    if refiner_provider == configured_provider:
        raise FactoryError("provider_collision", "Refiner and integration Validator must use opposite providers")
    provider = require_provider(args.provider or configured_provider, "integration validator provider")
    if provider != configured_provider:
        raise FactoryError(
            "provider_mismatch",
            f"integration validator provider must remain {configured_provider!r}",
        )
    epoch = int(meta.get("factory.fence_epoch", "0"))
    token = require_string(meta.get("factory.fence_token"), "factory.fence_token")
    integration_sha = require_string(meta.get("factory.integration_sha"), "factory.integration_sha", SHA_RE)
    worktree = check_fence(meta, epoch, token, integration_sha)
    branch = require_string(meta.get("factory.integration_branch"), "factory.integration_branch")
    binding = require_string(meta.get("factory.binding"), "factory.binding", ID_RE)
    candidate_rig, binding = register_integration_rig(
        worktree, args.refinery_bead, branch, binding,
        meta.get("factory.integration_rig"),
    )
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
        "provider": provider,
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
    validation_attestation = executor_session_attestation(validation, "validate", provider)
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
        "factory.integration_validator_context_id": validation_attestation["id"],
        "factory.integration_validator_model": validation_attestation["model"],
        "factory.integration_validator_model_policy": validation_attestation["model_policy"],
        "factory.integration_validator_model_source": validation_attestation["model_source"],
    })
    result = {
        "refinery_bead": args.refinery_bead,
        "integration_sha": integration_sha,
        "verdict": load_object(verdict_path, "integration verdict").get("verdict"),
        "validator_context_id": validation_attestation["id"],
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


def protected_merge_identity(worktree: Path, base_branch: str) -> dict[str, Any]:
    protection = gh_json(
        worktree, "api",
        f"repos/{{owner}}/{{repo}}/branches/{quote(base_branch, safe='')}/protection",
    )
    required = protection.get("required_status_checks")
    if not isinstance(required, dict) or not isinstance(required.get("strict"), bool):
        raise FactoryError("merge_policy_unproven", "target branch has no exact required-status-check policy")
    raw_checks = required.get("checks")
    if not isinstance(raw_checks, list) or not raw_checks:
        raise FactoryError("merge_policy_unproven", "target branch has no required hosted checks")
    checks: list[dict[str, Any]] = []
    for item in raw_checks:
        if not isinstance(item, dict):
            raise FactoryError("merge_policy_unproven", "required hosted-check identity is malformed")
        context = require_string(item.get("context"), "required check context")
        app_id = item.get("app_id")
        if isinstance(app_id, bool) or not isinstance(app_id, int) or app_id <= 0:
            raise FactoryError("merge_policy_unproven", f"required check {context!r} is not bound to one hosted app")
        checks.append({"context": context, "app_id": app_id})
    checks.sort(key=lambda item: (item["context"], item["app_id"]))
    if len({(item["context"], item["app_id"]) for item in checks}) != len(checks):
        raise FactoryError("merge_policy_unproven", "required hosted-check identities are duplicated")
    return {
        "protection_digest": digest_bytes(canonical_bytes(protection)),
        "strict": required["strict"],
        "checks": checks,
    }


def recorded_merge_arm_outcome(meta: dict[str, Any], marker_digest: str, marker_path: Path) -> str | None:
    outcome = meta.get("factory.merge_arm_outcome")
    if outcome is None:
        return None
    if outcome not in {"armed", "refused"}:
        raise FactoryError("merge_arm_result_invalid", f"unknown merge arm outcome: {outcome!r}")
    result_path = absolute_path(meta.get("factory.merge_arm_result_path"), "factory.merge_arm_result_path", True)
    if result_path != marker_path.with_name("merge-arm-result.v1.json"):
        raise FactoryError("merge_arm_result_invalid", "merge arm result evidence path changed")
    if digest_file(result_path) != meta.get("factory.merge_arm_result_digest"):
        raise FactoryError("merge_arm_result_invalid", "merge arm result evidence changed")
    result = load_object(result_path, "merge arm result")
    if set(result) != {"schema_version", "marker_digest", "outcome", "returncode", "response_digest"}:
        raise FactoryError("merge_arm_result_invalid", "merge arm result evidence has unexpected fields")
    returncode = result.get("returncode")
    if (
        result.get("schema_version") != "merge-arm-result.v1"
        or result.get("marker_digest") != marker_digest
        or result.get("outcome") != outcome
        or not isinstance(returncode, int)
        or (outcome == "armed" and returncode != 0)
        or (outcome == "refused" and returncode <= 0)
        or not isinstance(result.get("response_digest"), str)
        or not DIGEST_RE.fullmatch(result["response_digest"])
    ):
        raise FactoryError("merge_arm_result_invalid", "merge arm result evidence is inconsistent")
    return outcome


def delivery_record(meta: dict[str, Any], status: str, pr: dict[str, Any] | None,
                    landed_sha: str | None) -> dict[str, Any]:
    validation = None
    if meta.get("factory.integration_verdict"):
        validation = {
            "verdict": meta.get("factory.integration_verdict"),
            "verdict_digest": meta.get("factory.integration_verdict_digest"),
            "subject_manifest_digest": meta.get("factory.integration_subject_digest"),
            "validator_context_id": meta.get("factory.integration_validator_context_id"),
            "validator_model": meta.get("factory.integration_validator_model"),
            "validator_model_policy": meta.get("factory.integration_validator_model_policy"),
            "validator_model_source": meta.get("factory.integration_validator_model_source"),
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


def command_refinery_qualify(args: argparse.Namespace) -> int:
    """Complete a no-publish canary after the exact integration PASS gate."""
    beads = Beads(args.rig)
    refinery = beads.show(args.refinery_bead)
    meta = metadata(refinery)
    if meta.get("factory.kind") != "refinery" or meta.get("factory.delivery_mode") != "qualify":
        raise FactoryError("invalid_transition", "qualification requires a qualify-mode Refinery bead")
    if meta.get("factory.status") == "qualified":
        record_path = absolute_path(meta.get("factory.delivery_record"), "factory.delivery_record", True)
        if digest_file(record_path) != meta.get("factory.delivery_record_digest"):
            raise FactoryError("identity_mismatch", "qualification record changed")
    else:
        if meta.get("factory.status") != "validated" or meta.get("factory.integration_verdict") != "PASS":
            raise FactoryError("invalid_transition", "qualification requires an exact PASSed integration")
        epoch = int(meta.get("factory.fence_epoch", "0"))
        token = require_string(meta.get("factory.fence_token"), "factory.fence_token")
        integration_sha = require_string(meta.get("factory.integration_sha"), "factory.integration_sha", SHA_RE)
        check_fence(meta, epoch, token, integration_sha)
        record_path = persist_delivery_record(beads, args.refinery_bead, meta, "qualified", None, None)
        beads.update_metadata(args.refinery_bead, {
            "factory.status": "qualified",
            "factory.qualified_sha": integration_sha,
            "factory.qualification_record": str(record_path),
            "factory.qualification_record_digest": digest_file(record_path),
        })
        meta = metadata(beads.show(args.refinery_bead))
    program_bead = require_string(meta.get("factory.program_bead"), "factory.program_bead")
    program = beads.show(program_bead)
    beads.update_metadata(program_bead, {
        "factory.status": "qualified",
        "factory.qualified_sha": meta["factory.integration_sha"],
        "factory.qualification_record": str(record_path),
        "factory.qualification_record_digest": digest_file(record_path),
    })
    if program.get("status") != "closed":
        beads.close(
            program_bead,
            f"Refinery qualification passed at {meta['factory.integration_sha']} without publish or merge",
            "shipped", meta["factory.integration_sha"], meta["factory.integration_branch"],
            meta["factory.integration_worktree"],
        )
    refinery = beads.show(args.refinery_bead)
    if refinery.get("status") != "closed":
        beads.close(
            args.refinery_bead,
            f"Qualification-only integration PASS at {meta['factory.integration_sha']}",
            "shipped", meta["factory.integration_sha"], meta["factory.integration_branch"],
            meta["factory.integration_worktree"],
        )
    result = {
        "refinery_bead": args.refinery_bead,
        "program_bead": program_bead,
        "status": "qualified",
        "integration_sha": meta["factory.integration_sha"],
        "qualification_record": str(record_path),
    }
    print(json.dumps(result, sort_keys=True))
    return 0


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
    landed_tree = require_string(meta.get("factory.landed_tree"), "factory.landed_tree", SHA_RE)
    landed_base_parent = require_string(
        meta.get("factory.landed_base_parent"), "factory.landed_base_parent", SHA_RE,
    )
    delivery_path = absolute_path(meta.get("factory.delivery_record"), "factory.delivery_record", True)
    if digest_file(delivery_path) != meta.get("factory.delivery_record_digest"):
        raise FactoryError("identity_mismatch", "landed delivery record changed")
    program_bead = require_string(meta.get("factory.program_bead"), "factory.program_bead")
    program = beads.show(program_bead)
    beads.update_metadata(program_bead, {
        "factory.status": "landed",
        "factory.landed_sha": landed_sha,
        "factory.landed_tree": landed_tree,
        "factory.landed_base_parent": landed_base_parent,
        "factory.pr_url": require_string(meta.get("factory.pr_url"), "factory.pr_url"),
        "factory.delivery_record": str(delivery_path),
    })
    if refinery.get("status") != "closed":
        beads.close(
            refinery_bead, f"Factory delivery landed at {landed_sha}",
            "shipped", landed_sha, meta["factory.base_branch"], meta["factory.integration_worktree"],
        )
    if program.get("status") != "closed":
        beads.close(
            program_bead,
            f"Factory program landed through {meta['factory.pr_url']} at {landed_sha}",
            "shipped", landed_sha, meta["factory.base_branch"], meta["factory.integration_worktree"],
        )
    return {
        "refinery_bead": refinery_bead,
        "program_bead": program_bead,
        "pr_url": meta["factory.pr_url"],
        "landed_sha": landed_sha,
        "landed_tree": landed_tree,
        "landed_base_parent": landed_base_parent,
        "delivery_record": str(delivery_path),
    }


def command_refinery_land(args: argparse.Namespace) -> int:
    beads = Beads(args.rig)
    record = beads.show(args.refinery_bead)
    meta = metadata(record)
    if meta.get("factory.kind") != "refinery" or meta.get("factory.status") not in {"published", "merge_armed"}:
        raise FactoryError("invalid_transition", "only a published or merge_armed Refinery bead may land")
    integration_sha = require_string(meta.get("factory.integration_sha"), "factory.integration_sha", SHA_RE)
    expected_base_sha = require_string(meta.get("factory.delivery_base_sha"), "factory.delivery_base_sha", SHA_RE)
    base_branch = require_string(meta.get("factory.base_branch"), "factory.base_branch")
    worktree = check_fence(
        meta, int(meta["factory.fence_epoch"]),
        require_string(meta.get("factory.fence_token"), "factory.fence_token"), integration_sha,
    )
    pr_url = require_string(meta.get("factory.pr_url"), "factory.pr_url")
    pr = gh_json(
        worktree, "pr", "view", pr_url, "--json",
        "url,id,number,state,isDraft,headRefOid,headRefName,baseRefName,baseRefOid,mergeStateStatus,autoMergeRequest,mergedAt,mergeCommit",
    )
    if (
        pr.get("headRefOid") != integration_sha
        or pr.get("baseRefName") != base_branch
        or pr.get("baseRefOid") != expected_base_sha
    ):
        raise FactoryError("pr_binding_mismatch", "PR head, base branch, or base OID moved after publication")
    state = pr.get("state")
    if state not in {"OPEN", "MERGED"}:
        raise FactoryError("merge_admission_unproven", "exact PR must remain open until the forge lands it")
    if meta.get("factory.status") == "published":
        if state == "MERGED":
            raise FactoryError("merge_arm_unproven", "published PR merged without this reducer's arm evidence")
        if pr.get("isDraft") or pr.get("mergeStateStatus") != "BLOCKED":
            raise FactoryError("merge_admission_unproven", "exact non-draft PR must be BLOCKED before auto-merge arm")
        if pr.get("autoMergeRequest") is not None:
            raise FactoryError("merge_arm_unproven", "published PR already has an unowned auto-merge request")
    protection = protected_merge_identity(worktree, base_branch)
    marker = {
        "schema_version": "merge-arm.v1",
        "actor": "forge_native_auto_merge",
        "refinery_bead": args.refinery_bead,
        "pr_id": require_string(pr.get("id"), "PR node id"),
        "pr_url": pr_url,
        "head": integration_sha,
        "base_branch": base_branch,
        "base_sha": expected_base_sha,
        "method": "SQUASH",
        "protection": protection,
    }
    marker_path = worktree / ".gc" / "agentops-factory" / args.refinery_bead / "merge-arm.v1.json"
    marker_digest: str
    arm_outcome: str | None = None
    if meta.get("factory.status") == "merge_armed":
        recorded_marker_path = absolute_path(
            meta.get("factory.merge_marker_path"), "factory.merge_marker_path", True,
        )
        if recorded_marker_path != marker_path:
            raise FactoryError("merge_marker_invalid", "merge arm evidence path changed")
        if (
            digest_file(recorded_marker_path) != meta.get("factory.merge_marker_digest")
            or load_object(recorded_marker_path, "merge arm") != marker
            or meta.get("factory.merge_policy_digest") != protection["protection_digest"]
        ):
            raise FactoryError("merge_marker_invalid", "merge arm evidence changed")
        marker_digest = require_string(meta.get("factory.merge_marker_digest"), "factory.merge_marker_digest", DIGEST_RE)
        arm_outcome = recorded_merge_arm_outcome(meta, marker_digest, marker_path)
        if arm_outcome == "refused":
            raise FactoryError("merge_refused_terminal", "forge definitively refused this exact merge arm")
    else:
        write_or_verify_json(marker_path, marker, "merge arm")
        marker_digest = digest_file(marker_path)
        beads.update_metadata(args.refinery_bead, {
            "factory.status": "merge_armed",
            "factory.merge_marker_path": str(marker_path),
            "factory.merge_marker_digest": marker_digest,
            "factory.merge_policy_digest": protection["protection_digest"],
        })
        mutation = "mutation($pullRequestId:ID!,$expectedHeadOid:GitObjectID!,$mergeMethod:PullRequestMergeMethod!){enablePullRequestAutoMerge(input:{pullRequestId:$pullRequestId,expectedHeadOid:$expectedHeadOid,mergeMethod:$mergeMethod}){pullRequest{id}}}"
        armed = run_process([gh_binary(), "api", "graphql", "-f", f"query={mutation}", "-f", f"pullRequestId={marker['pr_id']}", "-f", f"expectedHeadOid={integration_sha}", "-f", "mergeMethod=SQUASH"], cwd=worktree, timeout=120, check=False)
        arm_outcome = "armed" if armed.returncode == 0 else "refused"
        response_digest = digest_bytes((armed.stdout + "\0" + armed.stderr).encode("utf-8"))
        result = {
            "schema_version": "merge-arm-result.v1",
            "marker_digest": marker_digest,
            "outcome": arm_outcome,
            "returncode": armed.returncode,
            "response_digest": response_digest,
        }
        result_path = marker_path.with_name("merge-arm-result.v1.json")
        write_or_verify_json(result_path, result, "merge arm result")
        beads.update_metadata(args.refinery_bead, {
            "factory.merge_arm_outcome": arm_outcome,
            "factory.merge_arm_result_path": str(result_path),
            "factory.merge_arm_result_digest": digest_file(result_path),
        })
        if arm_outcome == "refused":
            detail = armed.stderr.strip() or armed.stdout.strip() or "forge rejected auto-merge admission"
            raise FactoryError("merge_refused_terminal", detail)
        print(json.dumps({"refinery_bead": args.refinery_bead, "status": "merge_armed", "marker": str(marker_path)}, sort_keys=True))
        return 0
    if state == "OPEN":
        request = pr.get("autoMergeRequest")
        if not isinstance(request, dict) or request.get("mergeMethod") != "SQUASH":
            raise FactoryError("merge_arm_unproven", "exact auto-merge request is absent or mismatched")
        print(json.dumps({"refinery_bead": args.refinery_bead, "status": "merge_armed", "pending": True}, sort_keys=True))
        return 0
    merge_commit = pr.get("mergeCommit")
    landed_sha = merge_commit.get("oid") if isinstance(merge_commit, dict) else None
    require_string(landed_sha, "landed SHA", SHA_RE)
    remote_commit = gh_json(worktree, "api", f"repos/{{owner}}/{{repo}}/git/commits/{landed_sha}")
    remote_tree = remote_commit.get("tree")
    landed_tree = remote_tree.get("sha") if isinstance(remote_tree, dict) else None
    require_string(landed_tree, "landed tree", SHA_RE)
    parents = remote_commit.get("parents")
    parent_shas = [item.get("sha") for item in parents if isinstance(item, dict)] if isinstance(parents, list) else []
    expected_tree = output(["git", "rev-parse", f"{integration_sha}^{{tree}}"], cwd=worktree)
    if landed_tree != expected_tree or parent_shas != [expected_base_sha]:
        raise FactoryError("landed_identity_mismatch", "landed tree or base parent differs from the exact admitted epoch")
    pr_summary = {
        "url": pr.get("url"), "number": pr.get("number"), "state": state,
        "head_sha": pr.get("headRefOid"), "base_branch": pr.get("baseRefName"),
        "base_sha": expected_base_sha, "merged_at": pr.get("mergedAt"),
        "merge_commit": landed_sha, "landed_tree": landed_tree,
        "merge_policy_digest": protection["protection_digest"],
    }
    refreshed = metadata(beads.show(args.refinery_bead))
    delivery_path = persist_delivery_record(beads, args.refinery_bead, refreshed, "landed", pr_summary, landed_sha)
    beads.update_metadata(args.refinery_bead, {
        "factory.status": "landed",
        "factory.landed_sha": landed_sha,
        "factory.landed_tree": landed_tree,
        "factory.landed_base_parent": expected_base_sha,
        "factory.merged_at": str(pr.get("mergedAt", "")),
        "factory.delivery_record": str(delivery_path),
        "factory.delivery_record_digest": digest_file(delivery_path),
    })
    result = reconcile_landed_delivery(beads, args.refinery_bead)
    print(json.dumps(result, sort_keys=True))
    return 0


def command_refinery_deliver(args: argparse.Namespace) -> int:
    beads = Beads(args.rig)
    try:
        refinery = beads.show(args.refinery_bead)
        meta = attest_refiner_claim(beads, args.rig, args.refinery_bead, refinery, metadata(refinery))
        phase = meta.get("factory.status")
        with open(os.devnull, "w", encoding="utf-8") as sink, contextlib.redirect_stdout(sink):
            if phase in {"blocked", "ready", "assembling", "reassembly_required"}:
                command_refinery_assemble(argparse.Namespace(
                    rig=args.rig, refinery_bead=args.refinery_bead,
                    worktree_root=args.worktree_root, max_candidates=args.max_candidates,
                    remote=args.remote, merge_slot_timeout=args.merge_slot_timeout,
                ))
                phase = metadata(beads.show(args.refinery_bead)).get("factory.status")
            if phase == "validation_required":
                command_refinery_run_validation(argparse.Namespace(
                    rig=args.rig, refinery_bead=args.refinery_bead,
                    provider=args.provider, timeout=args.timeout,
                ))
                phase = metadata(beads.show(args.refinery_bead)).get("factory.status")
            delivery_mode = metadata(beads.show(args.refinery_bead)).get("factory.delivery_mode", "pr")
            if phase == "validated" and delivery_mode == "qualify":
                command_refinery_qualify(argparse.Namespace(
                    rig=args.rig, refinery_bead=args.refinery_bead,
                ))
                phase = metadata(beads.show(args.refinery_bead)).get("factory.status")
            if phase == "validated":
                command_refinery_publish(argparse.Namespace(
                    rig=args.rig, refinery_bead=args.refinery_bead, remote=args.remote,
                    title=args.title, draft=args.draft,
                ))
                phase = metadata(beads.show(args.refinery_bead)).get("factory.status")
            if phase in {"published", "merge_armed"}:
                command_refinery_land(argparse.Namespace(
                    rig=args.rig, refinery_bead=args.refinery_bead,
                    merge_method=args.merge_method, timeout=args.timeout,
                ))
                phase = metadata(beads.show(args.refinery_bead)).get("factory.status")
        if phase == "qualified":
            qualified = metadata(beads.show(args.refinery_bead))
            result = {
                "refinery_bead": args.refinery_bead,
                "program_bead": qualified.get("factory.program_bead"),
                "status": "qualified",
                "integration_sha": qualified.get("factory.integration_sha"),
                "qualification_record": qualified.get("factory.qualification_record"),
            }
            print(json.dumps(result, sort_keys=True))
            return 0
        if phase == "merge_armed":
            print(json.dumps({
                "refinery_bead": args.refinery_bead,
                "program_bead": metadata(beads.show(args.refinery_bead)).get("factory.program_bead"),
                "status": "merge_armed",
                "pending": True,
            }, sort_keys=True))
            return 0
        if phase == "integration_rejected":
            rejected = metadata(beads.show(args.refinery_bead))
            raise FactoryError("integration_rejected", f"integration verdict is {rejected.get('factory.integration_verdict')}")
        if phase != "landed":
            raise FactoryError("invalid_transition", f"Refinery delivery cannot resume from phase {phase!r}")
    except FactoryError as exc:
        hold_fields: dict[str, Any] = {
            "factory.delivery_hold": "true",
            "factory.delivery_hold_code": exc.code,
            "factory.delivery_hold_reason": str(exc)[:4000],
        }
        if exc.code == "integration_moved":
            hold_fields["factory.status"] = "reassembly_required"
        beads.update_metadata(args.refinery_bead, hold_fields)
        beads.hold_delivery(args.refinery_bead)
        raise
    result = reconcile_landed_delivery(Beads(args.rig), args.refinery_bead)
    result["status"] = "landed"
    print(json.dumps(result, sort_keys=True))
    return 0


def command_refinery_retry(args: argparse.Namespace) -> int:
    """Return an explicitly held delivery to its canonical Refiner route."""
    beads = Beads(args.rig)
    refinery = beads.show(args.refinery_bead)
    meta = metadata(refinery)
    if meta.get("factory.kind") != "refinery":
        raise FactoryError("not_refinery", "only a Refinery bead may retry delivery")
    if refinery.get("status") != "deferred" or meta.get("factory.delivery_hold") not in (True, "true"):
        raise FactoryError("invalid_transition", "retry requires a deferred Refinery delivery hold")
    phase = meta.get("factory.status")
    if phase in {"integration_rejected", "qualified", "landed"}:
        raise FactoryError("invalid_transition", f"Refinery phase {phase!r} is not retryable")
    binding = require_string(meta.get("factory.binding"), "factory.binding", ID_RE)
    refiner_provider = require_provider(meta.get("factory.refiner_provider"), "factory.refiner_provider")
    expected_route = lifecycle_target("refiner", refiner_provider, args.rig, binding)
    held_route = meta.get("gc.routed_to")
    if held_route not in (None, "", expected_route):
        raise FactoryError(
            "identity_mismatch",
            f"held Refinery route must be absent or {expected_route!r}, got {held_route!r}",
        )
    beads.retry_delivery(args.refinery_bead, expected_route)
    print(json.dumps({
        "refinery_bead": args.refinery_bead,
        "status": "open",
        "factory_status": phase,
        "route": expected_route,
    }, sort_keys=True))
    return 0


def command_doctor() -> int:
    problems: list[str] = []
    pack = (PACK_ROOT / "pack.toml").read_text(encoding="utf-8")
    if "[imports.executor]" not in pack:
        problems.append("factory pack must import the thin executor")
    agents = {
        path.name for path in (PACK_ROOT / "agents").iterdir()
        if path.is_dir() and (path / "agent.toml").is_file()
    }
    if agents != {"mayor", "plan-reviewer", "refiner"}:
        problems.append(f"unexpected semantic roles: {sorted(agents)}")
    role_models = {
        "mayor": ("claude", "fable-5", "adaptive", "city"),
        "plan-reviewer": ("codex", "gpt-5.6-sol", "high", "rig"),
        "refiner": ("claude", "fable-5", "adaptive", "rig"),
    }
    for role, (provider, model, effort, scope) in role_models.items():
        path = PACK_ROOT / "agents" / role / "agent.toml"
        if not path.is_file():
            problems.append(f"missing agents/{role}/agent.toml")
            continue
        config = tomllib.loads(path.read_text(encoding="utf-8"))
        if config.get("provider") != provider:
            problems.append(f"agents/{role}/agent.toml: provider must be {provider}")
        if provider == "claude" and config.get("session") != "tmux":
            problems.append(f"agents/{role}/agent.toml: Claude must use an interactive GC session")
        if config.get("option_defaults", {}).get("model") != model:
            problems.append(f"agents/{role}/agent.toml: model must be {model}")
        if config.get("option_defaults", {}).get("effort") != effort:
            problems.append(f"agents/{role}/agent.toml: effort must be {effort}")
        if config.get("scope") != scope:
            problems.append(f"agents/{role}/agent.toml: scope must be {scope}")
        if config.get("wake_mode") != "fresh" or config.get("lifecycle") != "one_shot":
            problems.append(f"agents/{role}/agent.toml: wake_mode/lifecycle must be fresh/one_shot")
        if config.get("min_active_sessions") != 0 or config.get("max_active_sessions") != 1:
            problems.append(f"agents/{role}/agent.toml: session bounds must be min 0 / max 1")
        if "work_query" in config or "sling_query" in config:
            problems.append(f"agents/{role}/agent.toml: native GC demand/claim must not be replaced")
        expected_work_dir = f".gc/agents/{role}"
        if config.get("work_dir") != expected_work_dir:
            problems.append(f"agents/{role}/agent.toml: work_dir must be {expected_work_dir}")
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
    # These are ordinary doctor checks, not helper-only tests: a green doctor
    # attests the composed six-role firewall and every finite staged state.
    try:
        composition = composed_route_doctor()
        if not composition["ok"]:
            problems.append(f"composed routing firewall: {composition['reason']}")
        else:
            proof = exhaustive_construction_proof()
            if proof["construction_states"] <= 0 or proof["model_routes"] != 6:
                problems.append("exhaustive construction proof returned an incomplete receipt")
            harness = run_inert_routing_harness()
            if (harness["routine_model_claims"] != 0 or harness["routine_model_starts"] != 0
                    or harness["routine_refiner_starts"] != 0 or harness["refiner_starts"] != 1
                    or harness["delivery_selections"] != 1 or harness["ambiguity_starts"] != 1):
                problems.append("fake-store reconciliation counts violate the delivery firewall")
    except FactoryError as exc:
        problems.append(f"routing proof failed: {exc}")
    if problems:
        print(f"agentops-factory doctor found {len(problems)} problem(s)")
        for problem in problems:
            print(problem)
        return 2
    print("agentops-factory proves inert routing only; live Fable remains closed pending supported confinement and GC33-11")
    return 0


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    sub = root.add_subparsers(dest="command", required=True)
    inspect_role_v2 = sub.add_parser("inspect-role-v2")
    inspect_role_v2.add_argument("--request", required=True)
    emit_role_v2 = sub.add_parser("emit-role-v2")
    emit_role_v2.add_argument("--request", required=True)
    emit_role_v2.add_argument("--artifact", required=True)
    sub.add_parser("doctor")
    return root


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    try:
        if args.command == "inspect-role-v2":
            return command_inspect_role_v2(args)
        if args.command == "emit-role-v2":
            return command_emit_role_v2(args)
        if args.command == "doctor":
            return command_doctor()
    except FactoryError as exc:
        print(json.dumps({"ok": False, "error": {"code": exc.code, "message": str(exc)}}, sort_keys=True))
        return 2
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
