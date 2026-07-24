#!/usr/bin/env python3
"""Closed-world compiler primitives for the shadow skill-contract.v3 rail."""

from __future__ import annotations

import copy
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import shlex
import shutil
from typing import Any, Iterable

from jsonschema import Draft7Validator
import yaml


CONTRACT_SCHEMA_REF = "schemas/skill-contract.v3.schema.json"
COMPILER_REFS = (
    "skills/skill-builder/scripts/contract_v3.py",
    "skills/skill-builder/scripts/compile_contracts.py",
)
CHECKS = (
    "schema",
    "authority",
    "effects",
    "artifacts",
    "triggers",
    "failure",
    "proof",
    "hard_dependencies",
    "source_unchanged",
)
MUTATING_EFFECTS = {
    "filesystem.write",
    "network.write",
    "environment.write",
    "credential.switch",
    "external.mutate",
    "runtime.session",
    "host.configure",
}
RECEIPT_REQUIRED_EFFECTS = MUTATING_EFFECTS | {"process.start"}
CLEANUP_REQUIRED_EFFECTS = {
    "process.start",
    "credential.switch",
    "runtime.session",
}
EXPECTED_ROUTE_RESULTS = {
    "positive": "route",
    "negative": "do_not_route",
    "ambiguity": "clarify",
}


class ContractError(ValueError):
    """A stable, fixture-addressable compiler rejection."""

    def __init__(self, code: str, message: str):
        super().__init__(message)
        self.code = code
        self.message = message

    def as_dict(self) -> dict[str, str]:
        return {"code": self.code, "message": self.message}


class StrictLoader(yaml.SafeLoader):
    """Safe YAML loader that rejects duplicate mapping keys."""


def _construct_mapping(
    loader: StrictLoader,
    node: yaml.nodes.MappingNode,
    deep: bool = False,
) -> dict[Any, Any]:
    result: dict[Any, Any] = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        try:
            duplicate = key in result
        except TypeError as exc:
            raise ContractError(
                "INVALID_YAML_KEY",
                f"mapping key is not hashable at line {key_node.start_mark.line + 1}",
            ) from exc
        if duplicate:
            raise ContractError(
                "DUPLICATE_YAML_KEY",
                f"duplicate YAML key {key!r} at line {key_node.start_mark.line + 1}",
            )
        result[key] = loader.construct_object(value_node, deep=deep)
    return result


StrictLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG,
    _construct_mapping,
)


def canonical_bytes(value: Any) -> bytes:
    return (
        json.dumps(
            value,
            ensure_ascii=False,
            separators=(",", ":"),
            sort_keys=True,
        )
        + "\n"
    ).encode("utf-8")


def canonical_digest(value: Any) -> str:
    return hashlib.sha256(canonical_bytes(value)).hexdigest()


def file_sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def reject_duplicate_json_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise ContractError("DUPLICATE_JSON_KEY", f"duplicate JSON key: {key}")
        result[key] = value
    return result


def load_json(path: Path) -> Any:
    try:
        return json.loads(
            path.read_text(encoding="utf-8"),
            object_pairs_hook=reject_duplicate_json_keys,
        )
    except ContractError:
        raise
    except (OSError, json.JSONDecodeError) as exc:
        raise ContractError("INVALID_JSON", f"{path}: {exc}") from exc


def load_frontmatter(source_path: Path) -> dict[str, Any]:
    try:
        text = source_path.read_text(encoding="utf-8")
    except OSError as exc:
        raise ContractError("SOURCE_UNAVAILABLE", f"{source_path}: {exc}") from exc
    parts = text.split("---", 2)
    if len(parts) != 3 or parts[0].strip():
        raise ContractError(
            "INVALID_FRONTMATTER",
            f"{source_path}: leading YAML frontmatter is missing",
        )
    try:
        value = yaml.load(parts[1], Loader=StrictLoader)
    except ContractError:
        raise
    except yaml.YAMLError as exc:
        problem = str(exc).splitlines()[0]
        raise ContractError(
            "INVALID_FRONTMATTER",
            f"{source_path}: {problem}",
        ) from exc
    if not isinstance(value, dict):
        raise ContractError(
            "INVALID_FRONTMATTER",
            f"{source_path}: frontmatter must be an object",
        )
    return value


def live_skill_names(repo_root: Path) -> set[str]:
    return {
        path.parent.name
        for path in (repo_root / "skills").glob("*/SKILL.md")
        if path.is_file() and not path.parent.is_symlink()
    }


def _format_json_path(parts: Iterable[Any]) -> str:
    rendered = "$"
    for part in parts:
        if isinstance(part, int):
            rendered += f"[{part}]"
        else:
            rendered += f".{part}"
    return rendered


def _schema_error_code(error: Any) -> str:
    path = list(error.absolute_path)
    root = str(path[0]) if path else ""
    if error.validator == "additionalProperties":
        return "UNKNOWN_FIELD"
    if error.validator == "uniqueItems":
        return "DUPLICATE_MEMBER"
    if root == "authority":
        return "AUTHORITY_INVALID"
    if root == "effects":
        return "EFFECT_INVALID"
    if root == "artifacts":
        return "ARTIFACT_INVALID"
    if root == "triggers":
        if error.validator in {"required", "minItems"}:
            return "TRIGGER_INCOMPLETE"
        return "TRIGGER_INVALID"
    if root == "failure":
        if error.validator == "required":
            return "FAILURE_INCOMPLETE"
        return "FAILURE_INVALID"
    if root == "proof":
        return "PROOF_INVALID"
    if error.validator == "required":
        return "CONTRACT_INCOMPLETE"
    return "SCHEMA_INVALID"


def _raise_first_schema_error(contract: Any, schema: dict[str, Any]) -> None:
    validator = Draft7Validator(schema)
    errors = sorted(
        validator.iter_errors(contract),
        key=lambda error: (
            tuple(str(part) for part in error.absolute_path),
            error.validator,
            error.message,
        ),
    )
    if not errors:
        return
    error = errors[0]
    path = _format_json_path(error.absolute_path)
    raise ContractError(
        _schema_error_code(error),
        f"{path}: {error.message}",
    )


def _validate_repo_ref(
    value: str,
    *,
    repo_root: Path,
    code: str,
    label: str,
) -> Path:
    if "\\" in value:
        raise ContractError(code, f"{label} must use repository-relative POSIX syntax")
    ref = PurePosixPath(value)
    if ref.is_absolute() or not ref.parts or ".." in ref.parts or "." in ref.parts:
        raise ContractError(code, f"{label} is not a contained repository-relative path")
    path = repo_root.joinpath(*ref.parts)
    try:
        resolved = path.resolve(strict=True)
        resolved.relative_to(repo_root.resolve())
    except (OSError, ValueError) as exc:
        raise ContractError(code, f"{label} does not resolve inside the repository: {value}") from exc
    if not resolved.is_file():
        raise ContractError(code, f"{label} is not a regular file: {value}")
    return resolved


def _require_unique_ids(
    values: Iterable[tuple[str, str]],
    *,
    code: str,
) -> None:
    seen: dict[str, str] = {}
    for identifier, location in values:
        if identifier in seen:
            raise ContractError(
                code,
                f"duplicate id {identifier!r} at {seen[identifier]} and {location}",
            )
        seen[identifier] = location


def _normalize_prompt(value: str) -> str:
    return re.sub(r"\s+", " ", value.strip().casefold())


def _validate_authority(
    contract: dict[str, Any],
    *,
    skill_name: str,
) -> None:
    authority = set(contract["authority"])
    forbidden = (
        ("refine_intent", skill_name != "plan", "only plan may refine intent"),
        ("dispatch_phase", skill_name != "rpi", "only rpi may dispatch phases"),
        ("write_verdict", skill_name != "validate", "only validate may write verdicts"),
        (
            "transport",
            contract["primary_layer"] != "runtime",
            "transport requires the runtime primary layer",
        ),
    )
    for verb, rejected, message in forbidden:
        if verb in authority and rejected:
            raise ContractError("FORBIDDEN_AUTHORITY", f"{skill_name}: {message}")
    if skill_name == "rpi" and "dispatch_phase" not in authority:
        raise ContractError(
            "FORBIDDEN_AUTHORITY",
            "rpi must declare dispatch_phase to represent its bounded phase dispatch",
        )


def _validate_effects(contract: dict[str, Any]) -> None:
    effects = contract["effects"]
    _require_unique_ids(
        ((effect["id"], f"effects[{index}]") for index, effect in enumerate(effects)),
        code="DUPLICATE_MEMBER",
    )
    for effect in effects:
        if effect["kind"] in RECEIPT_REQUIRED_EFFECTS and effect["receipt"] != "required":
            raise ContractError(
                "EFFECT_RECEIPT_REQUIRED",
                f"effect {effect['id']!r} ({effect['kind']}) requires receipt=required",
            )
        if effect["kind"] in CLEANUP_REQUIRED_EFFECTS and effect["cleanup"] != "required":
            raise ContractError(
                "EFFECT_CLEANUP_REQUIRED",
                f"effect {effect['id']!r} ({effect['kind']}) requires cleanup=required",
            )
    if "mutate_subject" in contract["authority"]:
        authorized_mutation = any(
            effect["kind"] in MUTATING_EFFECTS
            and effect["authorization"] in {"caller", "implement"}
            for effect in effects
        )
        if not authorized_mutation:
            raise ContractError(
                "MUTATION_EFFECT_REQUIRED",
                "mutate_subject requires a declared mutating effect authorized by caller or implement",
            )
    elif any(effect["kind"] in MUTATING_EFFECTS for effect in effects):
        raise ContractError(
            "MUTATION_AUTHORITY_REQUIRED",
            "a mutating effect requires mutate_subject authority",
        )


def _validate_artifacts(
    contract: dict[str, Any],
    *,
    repo_root: Path,
) -> None:
    artifact_ids: list[tuple[str, str]] = []
    for direction in ("consumes", "produces"):
        for index, artifact in enumerate(contract["artifacts"][direction]):
            artifact_ids.append((artifact["name"], f"artifacts.{direction}[{index}]"))
            for field in ("schema_ref", "validator"):
                value = artifact[field]
                if value is not None:
                    _validate_repo_ref(
                        value,
                        repo_root=repo_root,
                        code="ARTIFACT_INVALID",
                        label=f"artifact {artifact['name']} {field}",
                    )
            if (
                direction == "produces"
                and artifact["semantics"] == "binding"
                and (artifact["schema_ref"] is None or artifact["validator"] is None)
            ):
                raise ContractError(
                    "BINDING_ARTIFACT_UNVALIDATED",
                    f"binding output {artifact['name']!r} requires schema_ref and validator",
                )
    _require_unique_ids(artifact_ids, code="DUPLICATE_MEMBER")


def _validate_triggers(
    contract: dict[str, Any],
    *,
    skill_name: str,
    skill_names: set[str],
) -> None:
    triggers = contract["triggers"]
    trigger_ids: list[tuple[str, str]] = []
    prompts: dict[str, str] = {}
    for family, expected in EXPECTED_ROUTE_RESULTS.items():
        for index, case in enumerate(triggers[family]):
            location = f"triggers.{family}[{index}]"
            trigger_ids.append((case["id"], location))
            if case["expected"] != expected:
                raise ContractError(
                    "TRIGGER_EXPECTATION_INVALID",
                    f"{location} must expect {expected!r}",
                )
            normalized = _normalize_prompt(case["prompt"])
            if normalized in prompts:
                raise ContractError(
                    "TRIGGER_COLLISION",
                    f"prompt collision between {prompts[normalized]} and {location}",
                )
            prompts[normalized] = location
    for index, case in enumerate(triggers["aliases"]):
        location = f"triggers.aliases[{index}]"
        trigger_ids.append((case["id"], location))
        if case["canonical_skill"] not in skill_names:
            raise ContractError(
                "TRIGGER_REFERENCE_INVALID",
                f"{location} references unknown canonical skill {case['canonical_skill']!r}",
            )
        if case["canonical_skill"] != skill_name:
            raise ContractError(
                "TRIGGER_REFERENCE_INVALID",
                f"{location} must resolve to its owning skill {skill_name!r}",
            )
        normalized = _normalize_prompt(case["alias"])
        if normalized in prompts:
            raise ContractError(
                "TRIGGER_COLLISION",
                f"alias collision between {prompts[normalized]} and {location}",
            )
        prompts[normalized] = location
    for index, case in enumerate(triggers["nearest_neighbors"]):
        location = f"triggers.nearest_neighbors[{index}]"
        trigger_ids.append((case["id"], location))
        if case["skill"] not in skill_names or case["skill"] == skill_name:
            raise ContractError(
                "TRIGGER_REFERENCE_INVALID",
                f"{location} must name a different live skill",
            )
    _require_unique_ids(trigger_ids, code="DUPLICATE_MEMBER")


def _validate_proof(contract: dict[str, Any], *, repo_root: Path) -> None:
    proof = contract["proof"]
    command = proof["command"]
    if "\n" in command or "\r" in command or command != command.strip():
        raise ContractError(
            "PROOF_INVALID",
            "proof.command must be one trimmed command line",
        )
    try:
        tokens = shlex.split(command)
    except ValueError as exc:
        raise ContractError("PROOF_INVALID", f"proof.command is malformed: {exc}") from exc
    if not tokens:
        raise ContractError("PROOF_INVALID", "proof.command must not be empty")
    executable = tokens[0]
    if "/" in executable:
        executable_path = _validate_repo_ref(
            executable,
            repo_root=repo_root,
            code="PROOF_INVALID",
            label="proof executable",
        )
        if not executable_path.stat().st_mode & 0o111:
            raise ContractError("PROOF_INVALID", "proof executable is not executable")
    elif shutil.which(executable) is None:
        raise ContractError(
            "PROOF_INVALID",
            f"proof executable is unavailable: {executable}",
        )
    if executable in {"bash", "sh", "python", "python3"} and len(tokens) > 1:
        script = tokens[1]
        if not script.startswith("-"):
            _validate_repo_ref(
                script,
                repo_root=repo_root,
                code="PROOF_INVALID",
                label="proof script",
            )
    for fixture_ref in proof["fixture_refs"]:
        _validate_repo_ref(
            fixture_ref,
            repo_root=repo_root,
            code="PROOF_INVALID",
            label="proof fixture",
        )


def _validate_hard_dependencies(
    *,
    skill_name: str,
    dependencies: Any,
) -> None:
    if not isinstance(dependencies, list) or not all(
        isinstance(value, str) for value in dependencies
    ):
        raise ContractError(
            "HARD_DEPENDENCY_INVALID",
            f"{skill_name}: metadata.dependencies must be a string array",
        )
    if len(set(dependencies)) != len(dependencies):
        raise ContractError(
            "DUPLICATE_MEMBER",
            f"{skill_name}: metadata.dependencies contains duplicates",
        )
    expected = {"plan", "implement", "validate"} if skill_name == "rpi" else set()
    if set(dependencies) != expected:
        if skill_name == "rpi":
            message = "rpi hard dependencies must be exactly plan, implement, and validate"
        else:
            message = f"{skill_name} may not declare hard skill dependencies"
        raise ContractError("HARD_DEPENDENCY_FORBIDDEN", message)


def validate_contract(
    contract: Any,
    *,
    skill_name: str,
    dependencies: Any,
    repo_root: Path,
    schema: dict[str, Any] | None = None,
    skill_names: set[str] | None = None,
) -> dict[str, Any]:
    """Validate one contract and return a defensive copy on success."""

    if schema is None:
        schema = load_json(repo_root / CONTRACT_SCHEMA_REF)
    _raise_first_schema_error(contract, schema)
    assert isinstance(contract, dict)
    names = live_skill_names(repo_root) if skill_names is None else skill_names
    _validate_authority(contract, skill_name=skill_name)
    _validate_effects(contract)
    _validate_artifacts(contract, repo_root=repo_root)
    _validate_triggers(
        contract,
        skill_name=skill_name,
        skill_names=names,
    )
    _validate_proof(contract, repo_root=repo_root)
    _validate_hard_dependencies(
        skill_name=skill_name,
        dependencies=dependencies,
    )
    return copy.deepcopy(contract)


def compiler_identity(repo_root: Path) -> dict[str, Any]:
    sources = [
        {"ref": ref, "sha256": file_sha256(repo_root / ref)}
        for ref in COMPILER_REFS
    ]
    return {"sources": sources, "digest": canonical_digest(sources)}


def fixture_identity(repo_root: Path, fixture_refs: list[str]) -> dict[str, Any]:
    refs = sorted(fixture_refs)
    identities = [
        {"ref": ref, "sha256": file_sha256(repo_root / ref)}
        for ref in refs
    ]
    return {"refs": refs, "digest": canonical_digest(identities)}


def compile_skill(repo_root: Path, skill_name: str) -> dict[str, Any]:
    """Compile one source without mutation and return a deterministic receipt."""

    if not re.fullmatch(r"[a-z][a-z0-9]*(?:-[a-z0-9]+)*", skill_name):
        raise ContractError("SKILL_NAME_INVALID", f"invalid skill name: {skill_name!r}")
    source_ref = f"skills/{skill_name}/SKILL.md"
    source_path = repo_root / source_ref
    if not source_path.is_file() or source_path.is_symlink():
        raise ContractError("SOURCE_UNAVAILABLE", f"missing regular source: {source_ref}")
    before = file_sha256(source_path)
    frontmatter = load_frontmatter(source_path)
    if frontmatter.get("name") != skill_name:
        raise ContractError(
            "SKILL_NAME_MISMATCH",
            f"{source_ref}: frontmatter name must be {skill_name!r}",
        )
    metadata = frontmatter.get("metadata")
    if not isinstance(metadata, dict):
        raise ContractError(
            "CONTRACT_V3_ABSENT",
            f"{source_ref}: metadata.contract_v3 is absent",
        )
    contract = metadata.get("contract_v3")
    if contract is None:
        raise ContractError(
            "CONTRACT_V3_ABSENT",
            f"{source_ref}: metadata.contract_v3 is absent",
        )
    schema_path = repo_root / CONTRACT_SCHEMA_REF
    schema = load_json(schema_path)
    dependencies = metadata.get("dependencies")
    compiled = validate_contract(
        contract,
        skill_name=skill_name,
        dependencies=dependencies,
        repo_root=repo_root,
        schema=schema,
    )
    after = file_sha256(source_path)
    if before != after:
        raise ContractError(
            "SOURCE_MUTATED_DURING_CHECK",
            f"{source_ref}: source bytes changed while compiling",
        )
    return {
        "schema_version": "skill-contract-compile-receipt.v1",
        "source": {
            "ref": source_ref,
            "before_sha256": before,
            "after_sha256": after,
            "unchanged": True,
        },
        "contract": {
            "schema_ref": CONTRACT_SCHEMA_REF,
            "schema_sha256": file_sha256(schema_path),
            "digest": canonical_digest(compiled),
        },
        "compiler": compiler_identity(repo_root),
        "fixtures": fixture_identity(repo_root, compiled["proof"]["fixture_refs"]),
        "checks": list(CHECKS),
        "result": "PASS",
        "errors": [],
    }
