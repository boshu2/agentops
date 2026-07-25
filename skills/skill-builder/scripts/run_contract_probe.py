#!/usr/bin/env python3
"""Run one content-bound contract proof in a disposable repository copy."""

from __future__ import annotations

import argparse
from pathlib import Path
import shlex
import sys

sys.dont_write_bytecode = True

from compile_contracts import atomic_write, contained_output  # noqa: E402
from contract_v3 import (  # noqa: E402
    ContractError,
    canonical_bytes,
    compile_skill,
    file_set_identity,
    file_sha256,
)
from probe_runtime import run_isolated_command  # noqa: E402


RUNNER_REFS = [
    "skills/skill-builder/scripts/run_contract_probe.py",
    "skills/skill-builder/scripts/probe_runtime.py",
]


def repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Execute one declared skill-contract.v3 proof",
    )
    subparsers = parser.add_subparsers(dest="mode", required=True)
    check = subparsers.add_parser("check", help="run and render the receipt")
    check.add_argument("--skill", required=True)
    record = subparsers.add_parser("record", help="run and record the receipt")
    record.add_argument("--skill", required=True)
    record.add_argument("--output", required=True)
    return parser.parse_args(argv)


def _probe_errors(outcome: dict[str, object]) -> list[dict[str, str]]:
    execution = outcome["execution"]
    isolation = outcome["isolation"]
    assert isinstance(execution, dict)
    assert isinstance(isolation, dict)
    errors: list[dict[str, str]] = []
    confinement = execution["confinement"]
    assert isinstance(confinement, dict)
    if not confinement["proven"]:
        errors.append(
            {
                "code": "PROOF_CONFINEMENT_UNAVAILABLE",
                "message": "no proven OS write-confinement backend was available; proof command was not run",
            }
        )
    if execution["timed_out"]:
        errors.append({"code": "PROOF_TIMEOUT", "message": "proof exceeded its hard timeout"})
    if execution["interrupted"]:
        errors.append({"code": "PROOF_INTERRUPTED", "message": "proof was interrupted by the caller"})
    cleanup = execution["cleanup"]
    assert isinstance(cleanup, dict)
    if not cleanup["complete"]:
        errors.append(
            {
                "code": "PROOF_CLEANUP_INCOMPLETE",
                "message": "proof process group was not fully terminated and reaped",
            }
        )
    elif cleanup["trigger"] == "descendants":
        errors.append(
            {
                "code": "PROOF_DESCENDANTS",
                "message": "proof left a live descendant after its entrypoint exited",
            }
        )
    if not isolation["live_root_unchanged"]:
        errors.append(
            {
                "code": "LIVE_ROOT_MUTATED",
                "message": "live repository bytes changed during isolated proof execution",
            }
        )
    if isolation["out_of_scope_paths"]:
        errors.append(
            {
                "code": "PROOF_SCOPE_VIOLATION",
                "message": "proof changed paths outside its empty allowed-write scope",
            }
        )
    if execution["exit_code"] != 0 and not execution["timed_out"] and not execution["interrupted"]:
        errors.append(
            {
                "code": "PROOF_EXIT_NONZERO",
                "message": f"proof entrypoint exited {execution['exit_code']}",
            }
        )
    return errors


def run_probe(root: Path, skill_name: str) -> dict[str, object]:
    runner_before = file_set_identity(root, RUNNER_REFS)

    def prepare_snapshot(snapshot: Path) -> tuple[list[str], dict[str, object]]:
        compiled_snapshot = compile_skill(snapshot, skill_name)
        snapshot_runner = file_set_identity(snapshot, RUNNER_REFS)
        if snapshot_runner != runner_before:
            raise ContractError(
                "SNAPSHOT_IDENTITY_MISMATCH",
                "disposable snapshot runner bytes differ from the loaded runner source",
            )
        return shlex.split(compiled_snapshot["proof"]["command"]), {
            "compiled": compiled_snapshot,
            "runner": snapshot_runner,
        }

    outcome = run_isolated_command(root, None, prepare=prepare_snapshot)
    preparation = outcome.pop("preparation")
    assert isinstance(preparation, dict)
    compiled = preparation["compiled"]
    runner = preparation["runner"]
    assert isinstance(compiled, dict)
    assert isinstance(runner, dict)
    runner_after = file_set_identity(root, RUNNER_REFS)
    after = file_sha256(root / compiled["source"]["ref"])
    source_unchanged = after == compiled["source"]["before_sha256"]
    if not source_unchanged or runner_after != runner_before:
        isolation = outcome["isolation"]
        assert isinstance(isolation, dict)
        isolation["live_root_unchanged"] = False
        changed = isolation["live_root_changed_paths"]
        assert isinstance(changed, list)
        if not source_unchanged and compiled["source"]["ref"] not in changed:
            changed.append(compiled["source"]["ref"])
        if runner_after != runner_before:
            for runner_ref in RUNNER_REFS:
                if runner_ref not in changed:
                    changed.append(runner_ref)
        changed.sort()
    errors = _probe_errors(outcome)
    execution = outcome["execution"]
    assert isinstance(execution, dict)
    cleanup = execution["cleanup"]
    assert isinstance(cleanup, dict)
    proof_failed = execution["exit_code"] != 0
    not_proven = (
        not execution["confinement"]["proven"]  # type: ignore[index]
        or not execution["confinement"]["command_executed"]  # type: ignore[index]
        or execution["timed_out"]
        or execution["interrupted"]
        or not cleanup["complete"]
        or cleanup["trigger"] != "none"
        or not outcome["isolation"]["live_root_unchanged"]  # type: ignore[index]
    )
    result = "NOT_PROVEN" if not_proven else "FAIL" if proof_failed or errors else "PASS"
    return {
        "schema_version": "skill-contract-probe-receipt.v1",
        "skill": skill_name,
        "source": {
            "ref": compiled["source"]["ref"],
            "before_sha256": compiled["source"]["before_sha256"],
            "after_sha256": after,
            "unchanged": source_unchanged,
        },
        "contract": compiled["contract"],
        "compiler": compiled["compiler"],
        "runner": runner,
        "proof": compiled["proof"],
        "isolation": outcome["isolation"],
        "execution": outcome["execution"],
        "result": result,
        "errors": errors,
    }


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    root = repo_root()
    try:
        receipt = run_probe(root, args.skill)
        payload = canonical_bytes(receipt)
        if args.mode == "check":
            sys.stdout.buffer.write(payload)
        else:
            output = contained_output(root, args.output)
            atomic_write(output, payload)
            print(output.relative_to(root).as_posix())
        if receipt["result"] != "PASS":
            for error in receipt["errors"]:
                print(f"[{error['code']}] {error['message']}", file=sys.stderr)
            return 1
        return 0
    except ContractError as exc:
        print(f"[{exc.code}] {exc.message}", file=sys.stderr)
        return 1
    except (OSError, ValueError) as exc:
        print(f"[PROBE_SETUP_ERROR] {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
