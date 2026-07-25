#!/usr/bin/env python3
"""Render or verify the exact 49-row T2 contract migration ledger."""

from __future__ import annotations

import argparse
import os
from pathlib import Path
import sys
import tempfile
from typing import Any

from jsonschema import Draft7Validator

sys.dont_write_bytecode = True

from contract_v3 import (  # noqa: E402
    COMPILER_REFS,
    CONTRACT_SCHEMA_REF,
    ContractError,
    canonical_bytes,
    compile_skill,
    compiler_identity,
    file_set_identity,
    file_sha256,
    live_skill_names,
    load_frontmatter,
    load_json,
)


LEDGER_SCHEMA_REF = "schemas/skill-migration-readiness.v1.schema.json"
DEFAULT_LEDGER_REF = "skills/skill-builder/ledgers/migration-readiness.json"
PROBE_REF = "skills/skill-builder/receipts/skill-builder-contract-v3-probe.json"
PROBE_SCHEMA_REF = "skills/skill-builder/schemas/probe-report.json"
RUNNER_REFS = [
    "skills/skill-builder/scripts/run_contract_probe.py",
    "skills/skill-builder/scripts/probe_runtime.py",
]
TRANCHES = {
    "T1": {"implement", "plan", "rpi", "validate"},
    "T2": {"skill-builder"},
    "T3": {"automation-shape-routing", "craft-goal", "goals", "product"},
    "T4": {
        "cass",
        "codebase-recon",
        "council",
        "domain",
        "idea-genie",
        "postmortem",
        "premortem",
        "reality-check",
        "research",
        "reverse-engineer",
        "scope",
        "security",
        "standards",
    },
    "T5": {"converter", "doc", "refactor", "scaffold", "test", "workflow-builder"},
    "T6": {"learn", "operationalize", "pattern-mining", "toil-mining"},
    "T7a": {
        "agent-mail",
        "agent-native",
        "agy-native",
        "codex-exec",
        "ntm",
        "rch",
        "swarm",
        "using-gc",
    },
    "T7b": {
        "account-rotation",
        "bootstrap",
        "cc-hooks",
        "dcg",
        "handoff",
        "ms",
        "sbh",
        "shared",
        "status",
    },
}


def repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def target_tranche(skill_name: str) -> str:
    matches = [name for name, skills in TRANCHES.items() if skill_name in skills]
    if len(matches) != 1:
        raise ContractError(
            "cv3.check_migration_readiness.target_tranche.01",
            "TRANCHE_MAP_INVALID",
            f"{skill_name}: expected exactly one target tranche, found {matches}",
        )
    return matches[0]


def validate_json_schema(
    value: Any,
    *,
    schema_ref: str,
    root: Path,
    code: str,
) -> None:
    schema = load_json(root / schema_ref)
    errors = sorted(
        Draft7Validator(schema).iter_errors(value),
        key=lambda error: (
            tuple(str(part) for part in error.absolute_path),
            error.validator,
            error.message,
        ),
    )
    if errors:
        error = errors[0]
        path = "$" + "".join(f"[{part!r}]" for part in error.absolute_path)
        raise ContractError(
            "cv3.check_migration_readiness.validate_json_schema.01",
            code,
            f"{path}: {error.message}",
        )


def validated_probe(root: Path, compiled: dict[str, Any]) -> dict[str, Any]:
    receipt = load_json(root / PROBE_REF)
    validate_json_schema(
        receipt,
        schema_ref=PROBE_SCHEMA_REF,
        root=root,
        code="PROBE_RECEIPT_INVALID",
    )
    required = {"skill": "skill-builder", "result": "PASS"}
    for key, expected in required.items():
        if receipt.get(key) != expected:
            raise ContractError(
                "cv3.check_migration_readiness.validated_probe.01",
                "PROBE_RECEIPT_INVALID",
                f"{PROBE_REF}: {key} does not bind the current compiled contract",
            )
    source = receipt["source"]
    if (
        source["ref"] != compiled["source"]["ref"]
        or source["before_sha256"] != compiled["source"]["before_sha256"]
        or source["after_sha256"] != compiled["source"]["after_sha256"]
        or not source["unchanged"]
    ):
        raise ContractError(
            "cv3.check_migration_readiness.validated_probe.02",
            "PROBE_RECEIPT_INVALID",
            f"{PROBE_REF}: source identity is stale or mutated",
        )
    if receipt["contract"] != compiled["contract"]:
        raise ContractError(
            "cv3.check_migration_readiness.validated_probe.03",
            "PROBE_RECEIPT_INVALID",
            f"{PROBE_REF}: contract identity is stale",
        )
    if receipt["compiler"] != compiled["compiler"]:
        raise ContractError(
            "cv3.check_migration_readiness.validated_probe.04",
            "PROBE_RECEIPT_INVALID",
            f"{PROBE_REF}: compiler identity is stale",
        )
    if receipt["proof"] != compiled["proof"]:
        raise ContractError(
            "cv3.check_migration_readiness.validated_probe.05",
            "PROBE_RECEIPT_INVALID",
            f"{PROBE_REF}: proof identity is stale",
        )
    if receipt["runner"] != file_set_identity(root, RUNNER_REFS):
        raise ContractError(
            "cv3.check_migration_readiness.validated_probe.06",
            "PROBE_RECEIPT_INVALID",
            f"{PROBE_REF}: runner identity is stale",
        )
    isolation = receipt["isolation"]
    execution = receipt["execution"]
    cleanup = execution["cleanup"]
    if (
        isolation["kind"] != "disposable_repository_copy"
        or isolation["initial_manifest_sha256"] != isolation["final_manifest_sha256"]
        or isolation["changed_paths"]
        or isolation["out_of_scope_paths"]
        or not isolation["live_root_unchanged"]
        or isolation["live_root_changed_paths"]
        or execution["exit_code"] != 0
        or execution["timed_out"]
        or execution["interrupted"]
        or cleanup["trigger"] != "none"
        or not cleanup["parent_reaped"]
        or not cleanup["process_group_empty"]
        or not cleanup["complete"]
        or receipt["errors"]
    ):
        raise ContractError(
            "cv3.check_migration_readiness.validated_probe.07",
            "PROBE_RECEIPT_INVALID",
            f"{PROBE_REF}: isolated execution facts do not prove PASS",
        )
    return receipt


def expected_ledger(root: Path) -> dict[str, Any]:
    names = live_skill_names(root)
    mapped = set().union(*TRANCHES.values())
    if names != mapped or len(names) != 49:
        missing = sorted(names - mapped)
        stale = sorted(mapped - names)
        raise ContractError(
            "cv3.check_migration_readiness.expected_ledger.01",
            "TRANCHE_MAP_INVALID",
            f"canonical skill set mismatch (unmapped={missing}, stale={stale}, count={len(names)})",
        )
    compiled = compile_skill(root, "skill-builder")
    validated_probe(root, compiled)
    rows: list[dict[str, Any]] = []
    for name in sorted(names):
        source_ref = f"skills/{name}/SKILL.md"
        frontmatter = load_frontmatter(root / source_ref)
        contract = frontmatter.get("metadata", {}).get("contract_v3")
        if name == "skill-builder":
            if contract is None:
                raise ContractError(
                    "cv3.check_migration_readiness.expected_ledger.02",
                    "CONTRACT_V3_ABSENT",
                    "skill-builder must be the sole contract-v3-ready T2 skill",
                )
            row = {
                "name": name,
                "source_ref": source_ref,
                "source_sha256": file_sha256(root / source_ref),
                "target_tranche": target_tranche(name),
                "contract_status": "VALID",
                "compiler_status": "PASS",
                "contract_digest": compiled["contract"]["digest"],
                "probe_status": "PASS",
                "probe_receipt": {
                    "ref": PROBE_REF,
                    "sha256": file_sha256(root / PROBE_REF),
                },
                "proof_identity": None,
                "blockers": [],
                "cutover_ready": True,
            }
        else:
            if contract is not None:
                raise ContractError(
                    "cv3.check_migration_readiness.expected_ledger.03",
                    "EARLY_CONTRACT_V3",
                    f"{source_ref}: T2 may not migrate later-tranche skills",
                )
            row = {
                "name": name,
                "source_ref": source_ref,
                "source_sha256": file_sha256(root / source_ref),
                "target_tranche": target_tranche(name),
                "contract_status": "ABSENT",
                "compiler_status": "SKIPPED_ABSENT",
                "contract_digest": None,
                "probe_status": "UNDECLARED",
                "probe_receipt": None,
                "proof_identity": None,
                "blockers": [
                    "CONTRACT_V3_ABSENT",
                    "PROBE_UNDECLARED",
                ],
                "cutover_ready": False,
            }
        rows.append(row)
    compiler = compiler_identity(root)
    return {
        "schema_version": "skill-migration-readiness.v1",
        "live_authority": {
            "skill_api_version": 1,
            "catalog_version": "3",
        },
        "shadow": {
            "contract_schema_ref": CONTRACT_SCHEMA_REF,
            "contract_schema_sha256": file_sha256(root / CONTRACT_SCHEMA_REF),
            "compiler_refs": list(COMPILER_REFS),
            "compiler_digest": compiler["digest"],
        },
        "skill_count": 49,
        "rows": rows,
    }


def contained_path(root: Path, raw: str) -> Path:
    candidate = Path(raw)
    path = candidate if candidate.is_absolute() else root / candidate
    try:
        parent = path.parent.resolve(strict=True)
        parent.relative_to(root.resolve())
    except (OSError, ValueError) as exc:
        raise ContractError(
            "cv3.check_migration_readiness.contained_path.01",
            "OUTPUT_PATH_INVALID",
            f"output parent is not inside the repository: {raw}",
        ) from exc
    if path.exists() and (path.is_symlink() or not path.is_file()):
        raise ContractError(
            "cv3.check_migration_readiness.contained_path.02",
            "OUTPUT_PATH_INVALID",
            f"output must be a regular file or missing: {raw}",
        )
    return path


def atomic_write(path: Path, payload: bytes) -> None:
    descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    except BaseException:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Render or verify the exact T2 migration-readiness ledger",
    )
    subparsers = parser.add_subparsers(dest="mode", required=True)
    render = subparsers.add_parser("render", help="render expected bytes to stdout")
    render.set_defaults(ledger=DEFAULT_LEDGER_REF)
    record = subparsers.add_parser("record", help="record expected bytes")
    record.add_argument("--output", default=DEFAULT_LEDGER_REF)
    check = subparsers.add_parser("check", help="verify the committed ledger")
    check.add_argument("--ledger", default=DEFAULT_LEDGER_REF)
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    root = repo_root()
    try:
        expected = expected_ledger(root)
        validate_json_schema(
            expected,
            schema_ref=LEDGER_SCHEMA_REF,
            root=root,
            code="LEDGER_SCHEMA_INVALID",
        )
        payload = canonical_bytes(expected)
        if args.mode == "render":
            sys.stdout.buffer.write(payload)
        elif args.mode == "record":
            output = contained_path(root, args.output)
            atomic_write(output, payload)
            print(output.relative_to(root).as_posix())
        else:
            ledger_path = contained_path(root, args.ledger)
            actual = load_json(ledger_path)
            validate_json_schema(
                actual,
                schema_ref=LEDGER_SCHEMA_REF,
                root=root,
                code="LEDGER_SCHEMA_INVALID",
            )
            if (
                canonical_bytes(actual) != payload
                or ledger_path.read_bytes() != payload
            ):
                raise ContractError(
                    "cv3.check_migration_readiness.main.01",
                    "LEDGER_STALE",
                    f"{args.ledger}: rows, identities, ordering, or canonical bytes are stale",
                )
            print(
                "skill migration readiness: PASS (49 rows; 1 ready; 48 explicit blockers)"
            )
        return 0
    except ContractError as exc:
        print(f"[{exc.code}] {exc.message}", file=sys.stderr)
        return 1
    except OSError as exc:
        print(f"[IO_ERROR] {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
