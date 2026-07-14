#!/usr/bin/env python3
"""Admit Validate against the one persistent RPI run budget."""

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


SCHEMA_VERSION = 1
SKILL_DIR = Path(__file__).resolve().parent.parent
REPO_ROOT = SKILL_DIR.parent.parent
SCHEMA_PATH = REPO_ROOT / "schemas" / "validation-budget-receipt.v1.schema.json"
FACTUAL_CHECKER = SKILL_DIR / "scripts" / "validation-request.py"
GOVERNOR = REPO_ROOT / "skills" / "rpi" / "scripts" / "run-governor.py"
METER_OPTIONS = (
    ("reviewer_tokens", "--reviewer-tokens"),
    ("elapsed_seconds", "--elapsed-seconds"),
    ("review_contexts", "--review-contexts"),
    ("deterministic_executions", "--deterministic-executions"),
)


class BudgetError(Exception):
    pass


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode("utf-8")


def digest_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def digest_file(path: Path) -> str:
    return digest_bytes(path.read_bytes())


def load_json(path: Path) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise BudgetError(f"invalid-json:{path}") from error
    if not isinstance(value, dict):
        raise BudgetError(f"invalid-json-object:{path}")
    return value


def load_schema() -> dict[str, Any]:
    schema = load_json(SCHEMA_PATH)
    Draft202012Validator.check_schema(schema)
    return schema


def validate_receipt(receipt: dict[str, Any]) -> None:
    errors = sorted(
        Draft202012Validator(load_schema()).iter_errors(receipt),
        key=lambda error: list(error.absolute_path),
    )
    if errors:
        first = errors[0]
        location = ".".join(str(part) for part in first.absolute_path) or "$"
        raise BudgetError(f"invalid-budget-receipt:{location}:{first.message}")

    authorized = receipt["status"] == "AUTHORIZED"
    if receipt["validator_dispatch_allowed"] is not authorized:
        raise BudgetError("invalid-budget-receipt:dispatch-authority")
    if authorized:
        governor = receipt["governor"]
        if governor is None or governor["authorized"] is not True:
            raise BudgetError("invalid-budget-receipt:governor-authority")
        if not governor["admission_id"].startswith(f"{receipt['run_id']}:"):
            raise BudgetError("invalid-budget-receipt:run-admission-identity")
    else:
        if receipt["next_action"] is not None:
            raise BudgetError("invalid-budget-receipt:nonauthorizing-next-action")
        governor = receipt["governor"]
        if (
            receipt["factual_receipt_availability"] != "present"
            and governor is not None
        ):
            raise BudgetError("invalid-budget-receipt:unavailable-proof-governor")
        if governor is not None and (
            governor["authorized"] is not False
            or governor["admission_id"] is not None
            or governor["reason"] != receipt["reason"]
        ):
            raise BudgetError("invalid-budget-receipt:nonauthorizing-governor")
        if receipt["reason"].startswith("hard-ceiling:") and (
            governor is None
            or governor["disposition"] != "ANDON"
            or governor["helper_allowed"] is not False
        ):
            raise BudgetError("invalid-budget-receipt:hard-ceiling-authority")


def atomic_write(path: Path, value: dict[str, Any]) -> None:
    validate_receipt(value)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary_name = tempfile.mkstemp(
        prefix=f".{path.name}.", suffix=".tmp", dir=path.parent
    )
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(value, handle, sort_keys=True, separators=(",", ":"))
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary_name, path)
        directory_fd = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory_fd)
        finally:
            os.close(directory_fd)
    finally:
        try:
            os.unlink(temporary_name)
        except FileNotFoundError:
            pass


def requested_charge(args: argparse.Namespace) -> dict[str, int | None]:
    return {name: getattr(args, name) for name, _ in METER_OPTIONS}


def base_receipt(
    args: argparse.Namespace,
    request: dict[str, Any],
    factual_receipt_sha256: str | None,
    factual_receipt_availability: str,
) -> dict[str, Any]:
    return {
        "schema_version": SCHEMA_VERSION,
        "status": "NONAUTHORIZING",
        "reason": "not-admitted",
        "run_id": args.run_id,
        "request_id": request["request_id"],
        "request_sha256": digest_bytes(canonical_bytes(request)),
        "factual_receipt_availability": factual_receipt_availability,
        "factual_receipt_sha256": factual_receipt_sha256,
        "action": "semantic-review",
        "charge": requested_charge(args),
        "governor": None,
        "validator_dispatch_allowed": False,
        "next_action": None,
    }


def write_refusal(output: Path, receipt: dict[str, Any], reason: str) -> int:
    receipt["status"] = "NONAUTHORIZING"
    receipt["reason"] = reason
    receipt["validator_dispatch_allowed"] = False
    receipt["next_action"] = None
    atomic_write(output, receipt)
    print(json.dumps({"status": "NONAUTHORIZING", "reason": reason}, sort_keys=True))
    return 1


def check_factual_receipt(request_path: Path, receipt_path: Path) -> bool:
    completed = subprocess.run(
        [
            sys.executable,
            str(FACTUAL_CHECKER),
            "check-receipt",
            "--request",
            str(request_path),
            "--receipt",
            str(receipt_path),
        ],
        check=False,
        capture_output=True,
        text=True,
    )
    return completed.returncode == 0


def nonready_reason(factual: dict[str, Any]) -> str:
    for execution in factual.get("gate_executions", []):
        status = execution.get("candidate", {}).get("status")
        if status in {"FAIL", "ERROR", "UNKNOWN"}:
            return f"factual-proof-not-ready:{status}"
    return f"factual-proof-not-ready:{factual.get('disposition', 'UNKNOWN')}"


def governor_evidence(payload: dict[str, Any]) -> dict[str, Any]:
    admission_id = None
    if payload.get("authorized") is True:
        admissions = payload.get("admissions")
        if isinstance(admissions, list) and admissions:
            admission_id = admissions[-1].get("id")
    helper = payload.get("helper")
    helper_allowed = helper.get("allowed", False) if isinstance(helper, dict) else False
    usage = payload.get("usage")
    if not isinstance(usage, dict):
        usage = None
    return {
        "admission_id": admission_id,
        "disposition": payload.get("disposition", "NOTE"),
        "reason": payload.get("reason", "invalid-governor-receipt"),
        "authorized": payload.get("authorized") is True,
        "helper_allowed": helper_allowed,
        "usage": usage,
        "receipt_sha256": digest_bytes(canonical_bytes(payload)),
    }


def authorized_admission_matches(
    payload: dict[str, Any], run_id: str, charge: dict[str, int | None]
) -> bool:
    if (
        payload.get("run_id") != run_id
        or payload.get("authorized") is not True
        or payload.get("disposition") != "NOTE"
        or payload.get("reason") != "admitted-before-dispatch"
    ):
        return False
    admissions = payload.get("admissions")
    if not isinstance(admissions, list) or not admissions:
        return False
    admission = admissions[-1]
    expected_charge = {"waves": 0, **charge}
    return (
        admission.get("id") == f"{run_id}:{len(admissions)}"
        and admission.get("sequence") == len(admissions)
        and admission.get("action") == "semantic-review"
        and admission.get("status") == "recorded"
        and admission.get("charge") == expected_charge
    )


def admit(args: argparse.Namespace) -> int:
    request_path = args.request.resolve()
    factual_receipt_path = args.factual_receipt.resolve()
    output = args.output.resolve()
    try:
        request = load_json(request_path)
    except BudgetError as error:
        print(str(error), file=sys.stderr)
        return 1

    if not factual_receipt_path.is_file():
        receipt = base_receipt(args, request, None, "absent")
        return write_refusal(output, receipt, "missing-factual-proof")
    factual_digest = digest_file(factual_receipt_path)
    try:
        factual = load_json(factual_receipt_path)
    except BudgetError:
        receipt = base_receipt(args, request, factual_digest, "invalid_json")
        return write_refusal(output, receipt, "invalid-factual-proof-json")

    receipt = base_receipt(args, request, factual_digest, "present")

    if not check_factual_receipt(request_path, factual_receipt_path):
        return write_refusal(output, receipt, "invalid-factual-proof")
    if (
        factual.get("disposition") != "READY"
        or factual.get("model_spend_allowed") is not True
        or factual.get("next_action") != "VALIDATE_SINGLE_FRESH"
    ):
        return write_refusal(output, receipt, nonready_reason(factual))

    charge = requested_charge(args)
    for name, _ in METER_OPTIONS:
        value = charge[name]
        if value is None:
            return write_refusal(output, receipt, f"missing-meter:{name}")
        if isinstance(value, bool) or value < 0:
            return write_refusal(output, receipt, f"invalid-meter:{name}")

    command = [
        sys.executable,
        str(GOVERNOR),
        "admit",
        "--state-dir",
        str(args.state_dir.resolve()),
        "--run-id",
        args.run_id,
        "--action",
        "semantic-review",
    ]
    for name, option in METER_OPTIONS:
        command.extend((option, str(charge[name])))
    completed = subprocess.run(command, check=False, capture_output=True, text=True)
    try:
        governor_payload = json.loads(completed.stdout)
    except json.JSONDecodeError:
        return write_refusal(output, receipt, "invalid-governor-receipt")
    if not isinstance(governor_payload, dict):
        return write_refusal(output, receipt, "invalid-governor-receipt")

    receipt["governor"] = governor_evidence(governor_payload)
    if completed.returncode == 0 and authorized_admission_matches(
        governor_payload, args.run_id, charge
    ):
        receipt["status"] = "AUTHORIZED"
        receipt["reason"] = "admitted-before-dispatch"
        receipt["validator_dispatch_allowed"] = True
        receipt["next_action"] = "VALIDATE_SINGLE_FRESH"
        atomic_write(output, receipt)
        print(
            json.dumps(
                {
                    "status": "AUTHORIZED",
                    "admission_id": receipt["governor"]["admission_id"],
                },
                sort_keys=True,
            )
        )
        return 0

    reason = governor_payload.get("reason")
    if not isinstance(reason, str) or not reason:
        reason = "governor-refused"
    return write_refusal(output, receipt, reason)


def check(args: argparse.Namespace) -> int:
    try:
        validate_receipt(load_json(args.receipt.resolve()))
    except BudgetError as error:
        print(str(error), file=sys.stderr)
        return 1
    print(json.dumps({"status": "VALID"}, sort_keys=True))
    return 0


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    commands = root.add_subparsers(dest="command", required=True)
    admit_parser = commands.add_parser(
        "admit", help="record the one semantic-review admission before dispatch"
    )
    admit_parser.add_argument("--state-dir", required=True, type=Path)
    admit_parser.add_argument("--run-id", required=True)
    admit_parser.add_argument("--request", required=True, type=Path)
    admit_parser.add_argument("--factual-receipt", required=True, type=Path)
    admit_parser.add_argument("--output", required=True, type=Path)
    for name, option in METER_OPTIONS:
        admit_parser.add_argument(option, dest=name, type=int)
    admit_parser.set_defaults(handler=admit)

    check_parser = commands.add_parser(
        "check-receipt", help="validate the S2 receipt and authority invariants"
    )
    check_parser.add_argument("--receipt", required=True, type=Path)
    check_parser.set_defaults(handler=check)
    return root


def main() -> int:
    args = parser().parse_args()
    try:
        return args.handler(args)
    except (BudgetError, OSError, ValueError) as error:
        print(str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
