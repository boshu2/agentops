#!/usr/bin/env python3
"""Freeze and execute fail-closed factual validation requests."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any

from jsonschema import Draft202012Validator
from referencing import Registry, Resource


SCHEMA_VERSION = 1
SPEC_KEYS = {
    "schema_version",
    "request_id",
    "base_sha",
    "candidate_sha",
    "subtree_paths",
    "owned_paths",
    "acceptance_path",
    "evidence_paths",
    "claim_dependency_paths",
    "gate_registry_path",
    "toolchain_paths",
    "selected_gate_ids",
    "author_id",
    "validator",
}
REQUIRED_SPEC_KEYS = SPEC_KEYS
HEX40 = set("0123456789abcdef")


class RequestError(Exception):
    """A deterministic request or repository identity failure."""

    def __init__(self, code: str, detail: str):
        super().__init__(f"{code}: {detail}")
        self.code = code
        self.detail = detail


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(
        value, sort_keys=True, separators=(",", ":"), ensure_ascii=False
    ).encode("utf-8")


def digest_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def digest_file(path: Path) -> str:
    return digest_bytes(path.read_bytes())


def schema_dir() -> Path:
    return Path(__file__).resolve().parents[3] / "schemas"


def validate_schema(name: str, value: Any) -> None:
    root = schema_dir()
    path = root / name
    schema = json.loads(path.read_text(encoding="utf-8"))
    resources: list[tuple[str, Resource[Any]]] = []
    for candidate in root.glob("validation-*.schema.json"):
        try:
            document = json.loads(candidate.read_text(encoding="utf-8"))
        except json.JSONDecodeError:
            continue
        if "$id" in document:
            resources.append((document["$id"], Resource.from_contents(document)))
    registry = Registry().with_resources(resources)
    errors = sorted(
        Draft202012Validator(schema, registry=registry).iter_errors(value),
        key=lambda error: tuple(str(item) for item in error.absolute_path),
    )
    if errors:
        error = errors[0]
        location = ".".join(str(item) for item in error.absolute_path) or "$"
        raise RequestError("schema_error", f"{name}:{location}: {error.message}")


def load_json(path: Path, code: str = "invalid_json") -> Any:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise RequestError(code, f"{path}: {error}") from error


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as stream:
            json.dump(value, stream, indent=2, sort_keys=True)
            stream.write("\n")
        os.replace(temporary, path)
    except BaseException:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


def write_exclusive_json(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    try:
        descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    except FileExistsError as error:
        raise RequestError("duplicate_run", str(path)) from error
    try:
        with os.fdopen(descriptor, "wb") as stream:
            stream.write(canonical_bytes(value) + b"\n")
            stream.flush()
            os.fsync(stream.fileno())
        directory = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    except BaseException:
        # The exclusive file is deliberately retained as a fail-closed HOLD.
        raise


def reserve_run(repo: Path, output: Path, request_id: str, request_digest: str) -> Path:
    """Durably claim one canonical request identity before any factual execution."""
    common_raw = git(
        repo, "rev-parse", "--path-format=absolute", "--git-common-dir"
    ).stdout.strip()
    common_dir = Path(common_raw)
    if not common_dir.is_absolute():
        common_dir = (repo / common_dir).resolve()
    run_id = digest_bytes(
        canonical_bytes({"request_id": request_id, "request_sha256": request_digest})
    )
    claim_path = common_dir / "agentops-validation-runs" / f"{run_id}.json"
    reservation = {
        "schema_version": SCHEMA_VERSION,
        "state": "RESERVED",
        "run_id": run_id,
        "request_id": request_id,
        "request_sha256": request_digest,
        "receipt_path": str(output),
    }
    write_exclusive_json(claim_path, reservation)
    output_reservation = dict(reservation)
    output_reservation["run_claim_path"] = str(claim_path)
    write_exclusive_json(output, output_reservation)
    return claim_path


def complete_run(claim_path: Path, output: Path, receipt: dict[str, Any]) -> None:
    claim = load_json(claim_path, "invalid_run_claim")
    claim.update(
        {
            "state": "COMPLETED",
            "receipt_path": str(output),
            "receipt_sha256": digest_file(output),
            "disposition": receipt["disposition"],
        }
    )
    write_json(claim_path, claim)


def git(repo: Path, *args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(
        ["git", "-C", str(repo), *args],
        check=False,
        capture_output=True,
        text=True,
    )
    if check and result.returncode != 0:
        detail = result.stderr.strip() or result.stdout.strip() or "git command failed"
        raise RequestError("git_error", detail)
    return result


def oid(value: Any, label: str, length: int = 40) -> str:
    if (
        not isinstance(value, str)
        or len(value) != length
        or any(char not in HEX40 for char in value)
    ):
        raise RequestError(
            "invalid_identity", f"{label} must be a lowercase {length}-hex identity"
        )
    return value


def safe_path(repo: Path, value: Any, label: str) -> tuple[str, Path]:
    if (
        not isinstance(value, str)
        or not value
        or any(ord(character) < 32 for character in value)
        or Path(value).is_absolute()
    ):
        raise RequestError(
            "invalid_path", f"{label} must be a nonempty repository-relative path"
        )
    normalized = Path(value)
    if ".." in normalized.parts:
        raise RequestError("invalid_path", f"{label} escapes the repository")
    resolved = (repo / normalized).resolve()
    try:
        resolved.relative_to(repo.resolve())
    except ValueError as error:
        raise RequestError("invalid_path", f"{label} escapes the repository") from error
    return normalized.as_posix(), resolved


def file_identity(repo: Path, value: Any, label: str) -> dict[str, str]:
    relative, path = safe_path(repo, value, label)
    if not path.is_file():
        raise RequestError(f"missing_{label}", relative)
    return {"path": relative, "sha256": digest_file(path)}


def resolve_commit(repo: Path, value: Any, label: str) -> str:
    expected = oid(value, label)
    result = git(repo, "rev-parse", "--verify", f"{expected}^{{commit}}")
    actual = result.stdout.strip()
    if actual != expected:
        raise RequestError(
            "identity_mismatch", f"{label}: expected {expected}, resolved {actual}"
        )
    return actual


def require_clean_candidate(repo: Path, candidate: str) -> None:
    head = git(repo, "rev-parse", "HEAD").stdout.strip()
    status = git(repo, "status", "--porcelain=v1", "--untracked-files=all").stdout
    if head != candidate or status:
        raise RequestError(
            "candidate_mutated", f"expected clean HEAD {candidate}, observed {head}"
        )


def require_ancestor(repo: Path, base: str, candidate: str) -> None:
    result = git(repo, "merge-base", "--is-ancestor", base, candidate, check=False)
    if result.returncode == 1:
        raise RequestError(
            "base_not_ancestor", f"{base} is not an ancestor of {candidate}"
        )
    if result.returncode != 0:
        detail = result.stderr.strip() or "merge-base failed"
        raise RequestError("git_error", detail)


def changed_surfaces(repo: Path, base: str, candidate: str) -> list[dict[str, str]]:
    output = git(
        repo, "diff", "--name-status", "--no-renames", f"{base}..{candidate}"
    ).stdout
    surfaces: list[dict[str, str]] = []
    for line in output.splitlines():
        if not line:
            continue
        status, path = line.split("\t", 1)
        safe_path(repo, path, "changed_surface")
        surfaces.append({"path": path, "status": status})
    return sorted(surfaces, key=lambda item: (item["path"], item["status"]))


def subtrees(repo: Path, candidate: str, paths: Any) -> list[dict[str, str]]:
    if (
        not isinstance(paths, list)
        or not paths
        or any(not isinstance(path, str) or not path for path in paths)
        or len(set(paths)) != len(paths)
    ):
        raise RequestError(
            "invalid_spec", "subtree_paths must be a nonempty unique array"
        )
    result: list[dict[str, str]] = []
    for index, raw_path in enumerate(paths):
        path, _ = safe_path(repo, raw_path, f"subtree_paths[{index}]")
        tree = git(repo, "rev-parse", f"{candidate}:{path}").stdout.strip()
        kind = git(repo, "cat-file", "-t", tree).stdout.strip()
        if kind != "tree":
            raise RequestError(
                "invalid_subtree", f"{path} resolves to {kind}, not tree"
            )
        result.append({"path": path, "tree_sha": oid(tree, f"subtree {path}")})
    return sorted(result, key=lambda item: item["path"])


def object_at(repo: Path, commit: str, path: str) -> tuple[str, str] | None:
    result = git(repo, "rev-parse", f"{commit}:{path}", check=False)
    if result.returncode != 0:
        return None
    object_id = result.stdout.strip()
    kind = git(repo, "cat-file", "-t", object_id).stdout.strip()
    return object_id, kind


def owned_paths(
    repo: Path, base: str, candidate: str, paths: Any
) -> list[dict[str, Any]]:
    if (
        not isinstance(paths, list)
        or not paths
        or any(not isinstance(path, str) or not path for path in paths)
        or len(set(paths)) != len(paths)
    ):
        raise RequestError(
            "invalid_spec", "owned_paths must be a nonempty unique array"
        )
    result: list[dict[str, Any]] = []
    for index, raw_path in enumerate(paths):
        path, _ = safe_path(repo, raw_path, f"owned_paths[{index}]")
        current = object_at(repo, candidate, path)
        if current is not None:
            object_id, kind = current
            if kind != "blob":
                raise RequestError(
                    "invalid_owned_path", f"{path} resolves to {kind}, not blob"
                )
            result.append(
                {"path": path, "blob_oid": oid(object_id, f"owned path {path}")}
            )
            continue
        if object_at(repo, base, path) is None:
            raise RequestError(
                "invalid_owned_path", f"{path} exists in neither base nor candidate"
            )
        result.append({"path": path, "deleted": True})
    return sorted(result, key=lambda item: item["path"])


def semantic_id(
    owned: list[dict[str, Any]],
    acceptance: dict[str, str],
    claim_dependencies: list[str],
    evidence: list[dict[str, str]],
) -> str:
    identity = {
        "owned_paths": sorted(owned, key=lambda item: item["path"]),
        "acceptance_digest": acceptance["sha256"],
        "claim_dependency_digests": sorted(claim_dependencies),
        "evidence_dependency_digests": sorted(item["sha256"] for item in evidence),
    }
    return digest_bytes(canonical_bytes(identity))


def defect_class(code: str) -> str:
    """Name the owning integrity boundary for a fail-closed preflight defect."""
    if code in {
        "missing_gate_registry",
        "invalid_gate_registry",
        "unknown_gate",
        "duplicate_gate",
        "gate_not_factual_json",
        "registry_entry_changed",
        "missing_registry_backing",
        "missing_mandatory_gate",
    }:
        return "registry_integrity"
    if code in {"candidate_mutated", "base_not_ancestor"}:
        return "candidate_integrity"
    if code in {
        "missing_acceptance",
        "missing_claim_dependency",
        "stale_claim_dependency",
        "missing_evidence",
        "claim_dependency_digest_mismatch",
        "semantic_identity_changed",
    }:
        return "evidence_integrity"
    if code == "missing_toolchain":
        return "toolchain_integrity"
    if code in {"validator_not_independent", "invalid_validator_route"}:
        return "validator_integrity"
    return "request_integrity"


def validate_registry(value: Any) -> dict[str, dict[str, Any]]:
    validate_schema("validation-gate-registry.v1.schema.json", value)
    by_id: dict[str, dict[str, Any]] = {}
    for gate_entry in value["gates"]:
        gate_id = gate_entry["id"]
        if gate_id in by_id:
            raise RequestError("duplicate_gate", gate_id)
        if "--json" not in gate_entry["argv"]:
            raise RequestError("gate_not_factual_json", gate_id)
        by_id[gate_id] = gate_entry
    return by_id


def validate_spec(spec: Any) -> None:
    if not isinstance(spec, dict):
        raise RequestError("invalid_spec", "spec must be an object")
    unexpected = sorted(set(spec) - SPEC_KEYS)
    missing = sorted(REQUIRED_SPEC_KEYS - set(spec))
    if unexpected:
        raise RequestError(
            "invalid_spec", f"unexpected fields: {', '.join(unexpected)}"
        )
    if missing:
        raise RequestError("invalid_spec", f"missing fields: {', '.join(missing)}")
    if spec["schema_version"] != SCHEMA_VERSION:
        raise RequestError("invalid_spec", "schema_version must be 1")
    for key in ("request_id", "author_id"):
        if not isinstance(spec[key], str) or not spec[key]:
            raise RequestError("invalid_spec", f"{key} must be a nonempty string")
    validator = spec["validator"]
    if not isinstance(validator, dict) or set(validator) != {
        "validator_id",
        "fresh",
        "route",
    }:
        raise RequestError(
            "invalid_spec", "validator must contain only validator_id, fresh, and route"
        )
    if validator["fresh"] is not True or validator["route"] != "single_fresh":
        raise RequestError(
            "invalid_validator_route", "default validation requires one fresh validator"
        )
    if validator["validator_id"] == spec["author_id"]:
        raise RequestError(
            "validator_not_independent", "author_id and validator_id must differ"
        )


def freeze(repo: Path, spec_path: Path, output: Path) -> int:
    spec = load_json(spec_path)
    validate_spec(spec)
    base = resolve_commit(repo, spec["base_sha"], "base_sha")
    candidate = resolve_commit(repo, spec["candidate_sha"], "candidate_sha")
    require_clean_candidate(repo, candidate)
    require_ancestor(repo, base, candidate)

    acceptance = file_identity(repo, spec["acceptance_path"], "acceptance")
    if not isinstance(spec["evidence_paths"], list) or not spec["evidence_paths"]:
        raise RequestError("invalid_spec", "evidence_paths must be a nonempty array")
    evidence = [
        file_identity(repo, path, "evidence") for path in spec["evidence_paths"]
    ]
    claim_paths = spec["claim_dependency_paths"]
    if (
        not isinstance(claim_paths, list)
        or not claim_paths
        or any(not isinstance(path, str) or not path for path in claim_paths)
        or len(set(claim_paths)) != len(claim_paths)
    ):
        raise RequestError(
            "invalid_spec", "claim_dependency_paths must be a nonempty unique array"
        )
    claim_dependencies = [
        file_identity(repo, path, "claim_dependency") for path in claim_paths
    ]
    claim_dependencies = sorted(claim_dependencies, key=lambda item: item["path"])
    claim_digests = sorted(identity["sha256"] for identity in claim_dependencies)
    registry_identity = file_identity(repo, spec["gate_registry_path"], "gate_registry")
    registry = load_json(repo / registry_identity["path"], "invalid_gate_registry")
    registry_by_id = validate_registry(registry)
    if not isinstance(spec["toolchain_paths"], list) or not spec["toolchain_paths"]:
        raise RequestError("invalid_spec", "toolchain_paths must be a nonempty array")
    toolchain = [
        file_identity(repo, path, "toolchain") for path in spec["toolchain_paths"]
    ]
    selected_ids = spec["selected_gate_ids"]
    if (
        not isinstance(selected_ids, list)
        or not selected_ids
        or any(not isinstance(gate_id, str) or not gate_id for gate_id in selected_ids)
        or len(set(selected_ids)) != len(selected_ids)
    ):
        raise RequestError(
            "invalid_spec", "selected_gate_ids must be a nonempty unique array"
        )
    selected: list[dict[str, str]] = []
    for gate_id in selected_ids:
        if gate_id not in registry_by_id:
            raise RequestError("unknown_gate", str(gate_id))
        entry = registry_by_id[gate_id]
        selected.append(
            {
                "id": gate_id,
                "lane": entry["lane"],
                "proof_kind": entry["proof_kind"],
                "entry_sha256": digest_bytes(canonical_bytes(entry)),
            }
        )

    surfaces = changed_surfaces(repo, base, candidate)
    owned_spec = spec["owned_paths"]
    if (
        not isinstance(owned_spec, list)
        or not owned_spec
        or any(not isinstance(path, str) or not path for path in owned_spec)
        or len(set(owned_spec)) != len(owned_spec)
    ):
        raise RequestError(
            "invalid_spec", "owned_paths must be a nonempty unique array"
        )
    declared_owned = sorted(owned_spec)
    changed_paths = sorted(item["path"] for item in surfaces)
    if declared_owned != changed_paths:
        missing = sorted(set(changed_paths) - set(declared_owned))
        extra = sorted(set(declared_owned) - set(changed_paths))
        raise RequestError(
            "owned_paths_mismatch",
            f"missing={missing}; extra={extra}",
        )
    owned = owned_paths(repo, base, candidate, declared_owned)
    request = {
        "schema_version": SCHEMA_VERSION,
        "request_id": spec["request_id"],
        "candidate": {
            "semantic_id": semantic_id(owned, acceptance, claim_digests, evidence),
            "delivery_sha": candidate,
            "base_sha": base,
            "tree_sha": oid(
                git(repo, "rev-parse", f"{candidate}^{{tree}}").stdout.strip(),
                "tree_sha",
            ),
            "subtrees": subtrees(repo, candidate, spec["subtree_paths"]),
            "changed_surfaces": surfaces,
            "owned_paths": owned,
        },
        "acceptance": acceptance,
        "evidence": sorted(evidence, key=lambda item: item["path"]),
        "claim_dependencies": claim_dependencies,
        "claim_dependency_digests": claim_digests,
        "gate_registry": registry_identity,
        "toolchain": sorted(toolchain, key=lambda item: item["path"]),
        "selected_gates": selected,
        "author_id": spec["author_id"],
        "validator": spec["validator"],
    }
    validate_schema("validation-request.v1.schema.json", request)
    write_json(output, request)
    print(
        json.dumps(
            {"status": "FROZEN", "request_id": request["request_id"]}, sort_keys=True
        )
    )
    return 0


def identity_error(
    repo: Path,
    identity: dict[str, str],
    missing_code: str,
    stale_code: str | None = None,
) -> RequestError | None:
    try:
        _, path = safe_path(repo, identity.get("path"), missing_code)
    except RequestError as error:
        return RequestError(missing_code, error.detail)
    if not path.is_file():
        return RequestError(missing_code, identity["path"])
    actual = digest_file(path)
    if actual != identity.get("sha256"):
        return RequestError(
            stale_code or missing_code,
            f"{identity['path']}: expected {identity.get('sha256')}, observed {actual}",
        )
    return None


def candidate_error(repo: Path, candidate: dict[str, Any]) -> RequestError | None:
    try:
        delivery = resolve_commit(repo, candidate["delivery_sha"], "delivery_sha")
        base = resolve_commit(repo, candidate["base_sha"], "base_sha")
        require_clean_candidate(repo, delivery)
        require_ancestor(repo, base, delivery)
        actual_tree = git(repo, "rev-parse", f"{delivery}^{{tree}}").stdout.strip()
        if actual_tree != candidate["tree_sha"]:
            return RequestError("candidate_mutated", "candidate tree identity changed")
        actual_subtrees = subtrees(
            repo, delivery, [item["path"] for item in candidate["subtrees"]]
        )
        if actual_subtrees != candidate["subtrees"]:
            return RequestError("candidate_mutated", "subtree identity changed")
        actual_surfaces = changed_surfaces(repo, base, delivery)
        if actual_surfaces != candidate["changed_surfaces"]:
            return RequestError("candidate_mutated", "changed-surface identity changed")
        candidate_owned_paths = [item["path"] for item in candidate["owned_paths"]]
        if sorted(candidate_owned_paths) != sorted(
            item["path"] for item in actual_surfaces
        ):
            return RequestError(
                "candidate_mutated", "owned paths do not cover changed surfaces"
            )
        actual_owned = owned_paths(repo, base, delivery, candidate_owned_paths)
        if actual_owned != candidate["owned_paths"]:
            return RequestError("candidate_mutated", "owned-path identity changed")
    except RequestError as error:
        if error.code in {"candidate_mutated", "base_not_ancestor"}:
            return RequestError("candidate_mutated", error.detail)
        return error
    return None


def factual_result(argv: list[str], repo: Path) -> dict[str, Any]:
    try:
        completed = subprocess.run(
            argv, cwd=repo, check=False, capture_output=True, text=True
        )
    except (OSError, ValueError) as error:
        encoded = str(error).encode("utf-8")
        return {
            "status": "ERROR",
            "exit_code": 127,
            "output_sha256": digest_bytes(encoded),
            "facts": {},
        }
    output = completed.stdout.encode("utf-8")
    result: dict[str, Any] = {
        "status": "ERROR",
        "exit_code": completed.returncode,
        "output_sha256": digest_bytes(output),
        "facts": {},
    }
    try:
        parsed = json.loads(completed.stdout)
    except json.JSONDecodeError:
        return result
    if not isinstance(parsed, dict) or parsed.get("status") not in {
        "PASS",
        "FAIL",
        "ERROR",
        "UNKNOWN",
    }:
        return result
    facts = parsed.get("facts", {})
    if not isinstance(facts, dict):
        return result
    status = parsed["status"]
    if status == "PASS" and completed.returncode != 0:
        return result
    result["status"] = status
    result["facts"] = facts
    return result


def pinned_result(
    repo: Path, commit: str, argv: list[str], prefix: str
) -> dict[str, Any]:
    with tempfile.TemporaryDirectory(prefix=prefix) as parent:
        worktree = Path(parent) / "repo"
        added = git(
            repo,
            "worktree",
            "add",
            "--detach",
            "--quiet",
            str(worktree),
            commit,
            check=False,
        )
        if added.returncode != 0:
            return {
                "status": "ERROR",
                "exit_code": added.returncode,
                "output_sha256": digest_bytes(added.stderr.encode("utf-8")),
                "facts": {},
            }
        try:
            return factual_result(argv, worktree)
        finally:
            git(repo, "worktree", "remove", "--force", str(worktree), check=False)


def blocked_receipt(
    request: dict[str, Any], request_digest: str, error: RequestError
) -> dict[str, Any]:
    return {
        "schema_version": SCHEMA_VERSION,
        "request_id": request["request_id"],
        "request_sha256": request_digest,
        "candidate": request["candidate"],
        "validator_route": request["validator"]["route"],
        "preflight_errors": [
            {
                "code": error.code,
                "defect_class": defect_class(error.code),
                "detail": error.detail,
            }
        ],
        "gate_executions": [],
        "model_spend_allowed": False,
        "disposition": "BLOCK",
        "next_action": None,
    }


def validate_receipt_invariants(
    request: dict[str, Any],
    request_digest: str,
    receipt: dict[str, Any],
) -> None:
    """Reject schema-valid receipts that contradict the frozen request."""
    validate_schema("validation-receipt.v1.schema.json", receipt)
    bindings = (
        ("request_id", request["request_id"]),
        ("request_sha256", request_digest),
        ("candidate", request["candidate"]),
        ("validator_route", request["validator"]["route"]),
    )
    for field, expected in bindings:
        if receipt[field] != expected:
            raise RequestError("receipt_invariant", f"{field} does not match request")

    errors = receipt["preflight_errors"]
    executions = receipt["gate_executions"]
    if errors:
        if (
            executions
            or receipt["disposition"] != "BLOCK"
            or receipt["model_spend_allowed"] is not False
            or receipt["next_action"] is not None
        ):
            raise RequestError(
                "receipt_invariant", "preflight errors must be nonauthorizing BLOCK"
            )
        return

    expected_gates = [
        (item["id"], item["lane"], item["proof_kind"])
        for item in request["selected_gates"]
    ]
    actual_gates = [
        (item["id"], item["lane"], item["proof_kind"]) for item in executions
    ]
    if actual_gates != expected_gates:
        raise RequestError(
            "receipt_invariant", "gate executions do not exactly match selected gates"
        )
    if not executions:
        raise RequestError(
            "receipt_invariant", "a post-preflight receipt requires gate executions"
        )
    if not any(item["lane"] == "mandatory" for item in executions):
        raise RequestError(
            "receipt_invariant", "a post-preflight receipt requires a mandatory gate"
        )

    saw_block = False
    saw_candidate_introduced = False
    saw_pre_existing = False
    for execution in executions:
        candidate_status = execution["candidate"]["status"]
        attribution = execution["attribution"]
        baseline = execution.get("baseline")
        if candidate_status == "FAIL":
            if baseline is None:
                raise RequestError(
                    "receipt_invariant", f"{execution['id']}: FAIL lacks baseline"
                )
            baseline_status = baseline["status"]
            if baseline_status == "PASS":
                expected_attribution = "candidate_introduced"
                if execution["lane"] == "mandatory":
                    saw_candidate_introduced = True
            elif baseline_status == "FAIL":
                expected_attribution = "pre_existing"
                if execution["lane"] == "mandatory":
                    saw_pre_existing = True
            else:
                expected_attribution = "not_applicable"
                saw_block = True
            if attribution != expected_attribution:
                raise RequestError(
                    "receipt_invariant",
                    f"{execution['id']}: attribution contradicts baseline",
                )
        else:
            if baseline is not None or attribution != "not_applicable":
                raise RequestError(
                    "receipt_invariant",
                    f"{execution['id']}: non-FAIL cannot have baseline attribution",
                )
            if candidate_status in {"ERROR", "UNKNOWN"}:
                saw_block = True

    if saw_block:
        expected_disposition = "BLOCK"
        expected_action = None
    elif saw_candidate_introduced:
        expected_disposition = "REPAIR"
        expected_action = "REPAIR_CANDIDATE"
    elif saw_pre_existing:
        expected_disposition = "NOTE"
        expected_action = "RETURN_PRE_EXISTING_FAILURE"
    else:
        expected_disposition = "READY"
        expected_action = "VALIDATE_SINGLE_FRESH"
    expected_spend = expected_disposition == "READY"
    if (
        receipt["disposition"] != expected_disposition
        or receipt["next_action"] != expected_action
        or receipt["model_spend_allowed"] is not expected_spend
    ):
        raise RequestError(
            "receipt_invariant", "receipt authority contradicts factual evidence"
        )


def execute(repo: Path, request_path: Path, output: Path) -> int:
    request = load_json(request_path, "invalid_request")
    validate_schema("validation-request.v1.schema.json", request)
    request_digest = digest_bytes(canonical_bytes(request))
    claim_path = reserve_run(repo, output, request["request_id"], request_digest)
    if os.environ.get("AGENTOPS_VALIDATION_TEST_CRASH_AFTER_RESERVATION") == "1":
        os._exit(75)

    error = candidate_error(repo, request["candidate"])
    if error is None:
        error = identity_error(repo, request["acceptance"], "missing_acceptance")
    if error is None:
        for identity in request["claim_dependencies"]:
            error = identity_error(
                repo,
                identity,
                "missing_claim_dependency",
                "stale_claim_dependency",
            )
            if error is not None:
                break
    if error is None:
        for identity in request["evidence"]:
            error = identity_error(repo, identity, "missing_evidence")
            if error is not None:
                break
    if error is None:
        error = identity_error(repo, request["gate_registry"], "missing_gate_registry")
    if error is None:
        for identity in request["toolchain"]:
            error = identity_error(repo, identity, "missing_toolchain")
            if error is not None:
                break

    registry_by_id: dict[str, dict[str, Any]] = {}
    if error is None:
        try:
            registry = load_json(
                repo / request["gate_registry"]["path"], "invalid_gate_registry"
            )
            registry_by_id = validate_registry(registry)
            for selected in request["selected_gates"]:
                entry = registry_by_id.get(selected["id"])
                if entry is None:
                    raise RequestError("unknown_gate", selected["id"])
                if (
                    entry["lane"] != selected["lane"]
                    or entry["proof_kind"] != selected["proof_kind"]
                    or digest_bytes(canonical_bytes(entry)) != selected["entry_sha256"]
                ):
                    raise RequestError("registry_entry_changed", selected["id"])
                for backing in entry["backing"]:
                    backing_error = identity_error(
                        repo, backing, "missing_registry_backing"
                    )
                    if backing_error is not None:
                        raise backing_error
            if not any(
                selected["lane"] == "mandatory"
                for selected in request["selected_gates"]
            ):
                raise RequestError(
                    "missing_mandatory_gate", "selected gates contain no mandatory lane"
                )
        except RequestError as caught:
            error = caught

    if error is None and request["author_id"] == request["validator"]["validator_id"]:
        error = RequestError(
            "validator_not_independent", "author and validator identities match"
        )
    if error is None and (
        request["validator"]["fresh"] is not True
        or request["validator"]["route"] != "single_fresh"
    ):
        error = RequestError(
            "invalid_validator_route", "one fresh validator is required"
        )
    if error is None:
        claim_digests = sorted(
            identity["sha256"] for identity in request["claim_dependencies"]
        )
        if claim_digests != request["claim_dependency_digests"]:
            error = RequestError(
                "claim_dependency_digest_mismatch",
                "claim dependency references and digest projection differ",
            )
    if error is None:
        actual_semantic = semantic_id(
            request["candidate"]["owned_paths"],
            request["acceptance"],
            request["claim_dependency_digests"],
            request["evidence"],
        )
        if actual_semantic != request["candidate"]["semantic_id"]:
            error = RequestError(
                "semantic_identity_changed",
                "request semantic identity does not match its inputs",
            )

    if error is not None:
        receipt = blocked_receipt(request, request_digest, error)
        validate_receipt_invariants(request, request_digest, receipt)
        write_json(output, receipt)
        complete_run(claim_path, output, receipt)
        print(json.dumps({"status": "BLOCK", "code": error.code}, sort_keys=True))
        return 1

    executions: list[dict[str, Any]] = []
    saw_block = False
    saw_candidate_introduced = False
    saw_pre_existing = False
    for selected in request["selected_gates"]:
        entry = registry_by_id[selected["id"]]
        candidate_result = pinned_result(
            repo,
            request["candidate"]["delivery_sha"],
            entry["argv"],
            "validation-candidate-",
        )
        execution: dict[str, Any] = {
            "id": selected["id"],
            "lane": selected["lane"],
            "proof_kind": selected["proof_kind"],
            "candidate": candidate_result,
            "attribution": "not_applicable",
        }
        if candidate_result["status"] == "FAIL":
            baseline = pinned_result(
                repo,
                request["candidate"]["base_sha"],
                entry["argv"],
                "validation-baseline-",
            )
            execution["baseline"] = baseline
            if baseline["status"] == "PASS":
                execution["attribution"] = "candidate_introduced"
                if selected["lane"] == "mandatory":
                    saw_candidate_introduced = True
            elif baseline["status"] == "FAIL":
                execution["attribution"] = "pre_existing"
                if selected["lane"] == "mandatory":
                    saw_pre_existing = True
            else:
                saw_block = True
        elif candidate_result["status"] in {"ERROR", "UNKNOWN"}:
            saw_block = True
        executions.append(execution)

    if saw_block:
        disposition = "BLOCK"
        next_action: str | None = None
    elif saw_candidate_introduced:
        disposition = "REPAIR"
        next_action = "REPAIR_CANDIDATE"
    elif saw_pre_existing:
        disposition = "NOTE"
        next_action = "RETURN_PRE_EXISTING_FAILURE"
    else:
        disposition = "READY"
        next_action = "VALIDATE_SINGLE_FRESH"
    model_spend_allowed = disposition == "READY"
    receipt = {
        "schema_version": SCHEMA_VERSION,
        "request_id": request["request_id"],
        "request_sha256": request_digest,
        "candidate": request["candidate"],
        "validator_route": request["validator"]["route"],
        "preflight_errors": [],
        "gate_executions": executions,
        "model_spend_allowed": model_spend_allowed,
        "disposition": disposition,
        "next_action": next_action,
    }
    validate_receipt_invariants(request, request_digest, receipt)
    write_json(output, receipt)
    complete_run(claim_path, output, receipt)
    print(
        json.dumps(
            {"status": disposition, "request_id": request["request_id"]}, sort_keys=True
        )
    )
    return 0 if disposition == "READY" else 1


def check_receipt(request_path: Path, receipt_path: Path) -> int:
    request = load_json(request_path, "invalid_request")
    validate_schema("validation-request.v1.schema.json", request)
    receipt = load_json(receipt_path, "invalid_receipt")
    validate_receipt_invariants(
        request, digest_bytes(canonical_bytes(request)), receipt
    )
    print(
        json.dumps(
            {"status": "VALID", "request_id": request["request_id"]}, sort_keys=True
        )
    )
    return 0


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    subparsers = root.add_subparsers(dest="command", required=True)
    freeze_parser = subparsers.add_parser(
        "freeze", help="freeze a validation request from explicit identities"
    )
    freeze_parser.add_argument("--repo", required=True, type=Path)
    freeze_parser.add_argument("--spec", required=True, type=Path)
    freeze_parser.add_argument("--output", required=True, type=Path)
    run_parser = subparsers.add_parser(
        "run", help="preflight and execute each selected factual gate once"
    )
    run_parser.add_argument("--repo", required=True, type=Path)
    run_parser.add_argument("--request", required=True, type=Path)
    run_parser.add_argument("--output", required=True, type=Path)
    receipt_parser = subparsers.add_parser(
        "check-receipt",
        help="validate receipt schema, request binding, and authorization invariants",
    )
    receipt_parser.add_argument("--request", required=True, type=Path)
    receipt_parser.add_argument("--receipt", required=True, type=Path)
    return root


def main() -> int:
    args = parser().parse_args()
    try:
        if args.command == "check-receipt":
            return check_receipt(args.request.resolve(), args.receipt.resolve())
        repo = args.repo.resolve()
        if args.command == "freeze":
            return freeze(repo, args.spec.resolve(), args.output.resolve())
        return execute(repo, args.request.resolve(), args.output.resolve())
    except RequestError as error:
        print(f"{error.code}: {error.detail}", file=sys.stderr)
        return 2 if error.code == "duplicate_run" else 1
    except OSError as error:
        print(f"io_error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
