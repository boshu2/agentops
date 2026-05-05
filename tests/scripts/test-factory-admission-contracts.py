#!/usr/bin/env python3
"""Validate factory admission schemas and fixtures."""

from __future__ import annotations

import copy
import json
from pathlib import Path

from jsonschema import Draft202012Validator, FormatChecker


ROOT = Path(__file__).resolve().parents[2]
FIXTURES = ROOT / "tests" / "fixtures" / "factory-admission"


def load(path: Path) -> dict:
    with path.open() as fh:
        return json.load(fh)


def assert_valid(schema: dict, instance: dict, name: str) -> None:
    validator = Draft202012Validator(schema, format_checker=FormatChecker())
    errors = sorted(validator.iter_errors(instance), key=lambda err: err.path)
    if errors:
        joined = "\n".join(error.message for error in errors)
        raise AssertionError(f"{name} should be valid:\n{joined}")


def assert_invalid(schema: dict, instance: dict, name: str) -> None:
    validator = Draft202012Validator(schema, format_checker=FormatChecker())
    if not list(validator.iter_errors(instance)):
        raise AssertionError(f"{name} should be invalid")


def main() -> None:
    work_order_schema = load(ROOT / "schemas" / "factory-work-order.v1.schema.json")
    decision_schema = load(ROOT / "schemas" / "factory-admission.v1.schema.json")
    work_order = load(FIXTURES / "valid-work-order.json")
    decision = load(FIXTURES / "valid-admission-decision.json")

    assert_valid(work_order_schema, work_order, "valid work order")
    assert_valid(decision_schema, decision, "valid admission decision")

    for field in ["target", "generated_at", "expires_at", "validation_commands", "digest_policy"]:
        invalid = copy.deepcopy(work_order)
        invalid.pop(field)
        assert_invalid(work_order_schema, invalid, f"work order missing {field}")

    invalid = copy.deepcopy(work_order)
    invalid["landing_policy"] = "auto_merge"
    assert_invalid(work_order_schema, invalid, "work order with auto_merge")

    invalid = copy.deepcopy(work_order)
    invalid["allowed_files"] = ["/tmp/outside"]
    assert_invalid(work_order_schema, invalid, "work order with absolute path")

    invalid = copy.deepcopy(decision)
    invalid["allowed"] = False
    invalid["reasons"] = []
    assert_invalid(decision_schema, invalid, "blocked decision without reasons")

    print("factory-admission-contracts: PASS")


if __name__ == "__main__":
    main()
