#!/usr/bin/env python3
"""Behavioral proof for the closed shadow skill-contract.v3 compiler."""

from __future__ import annotations

import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

from jsonschema import Draft7Validator


REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPT_DIR = REPO_ROOT / "skills/skill-builder/scripts"
sys.path.insert(0, str(SCRIPT_DIR))

from contract_v3 import (  # noqa: E402
    ContractError,
    canonical_bytes,
    compile_skill,
    load_frontmatter,
    load_json,
    validate_contract,
)


FIXTURE_REF = "skills/skill-builder/fixtures/contract-v3/cases.json"
SOURCE_REF = "skills/skill-builder/SKILL.md"
CLI_REF = "skills/skill-builder/scripts/compile_contracts.py"
REPORT_SCHEMA_REF = "skills/skill-builder/schemas/compile-report.json"


def pointer_parent(value: object, pointer: str) -> tuple[object, str]:
    parts = pointer.lstrip("/").split("/")
    current = value
    for raw in parts[:-1]:
        part = raw.replace("~1", "/").replace("~0", "~")
        if isinstance(current, list):
            current = current[int(part)]
        else:
            assert isinstance(current, dict)
            current = current[part]
    return current, parts[-1].replace("~1", "/").replace("~0", "~")


def pointer_get(value: object, pointer: str) -> object:
    parent, key = pointer_parent(value, pointer)
    if isinstance(parent, list):
        return parent[int(key)]
    assert isinstance(parent, dict)
    return parent[key]


def apply_mutation(value: object, mutation: dict[str, object]) -> None:
    parent, key = pointer_parent(value, str(mutation["path"]))
    operation = mutation["op"]
    if operation == "set":
        if isinstance(parent, list):
            parent[int(key)] = copy.deepcopy(mutation["value"])
        else:
            assert isinstance(parent, dict)
            parent[key] = copy.deepcopy(mutation["value"])
    elif operation == "delete":
        if isinstance(parent, list):
            del parent[int(key)]
        else:
            assert isinstance(parent, dict)
            del parent[key]
    elif operation == "append":
        target = pointer_get(value, str(mutation["path"]))
        assert isinstance(target, list)
        target.append(copy.deepcopy(mutation["value"]))
    elif operation == "copy_append":
        target = pointer_get(value, str(mutation["path"]))
        source = pointer_get(value, str(mutation["source"]))
        assert isinstance(target, list)
        target.append(copy.deepcopy(source))
    else:
        raise AssertionError(f"unknown fixture mutation: {operation}")


def byte_snapshot(root: Path) -> dict[str, str]:
    return {
        path.relative_to(root).as_posix(): hashlib.sha256(path.read_bytes()).hexdigest()
        for path in sorted(root.rglob("*"))
        if path.is_file() and "__pycache__" not in path.parts
    }


class ContractV3Tests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        frontmatter = load_frontmatter(REPO_ROOT / SOURCE_REF)
        cls.base_contract = frontmatter["metadata"]["contract_v3"]
        cls.base_dependencies = frontmatter["metadata"]["dependencies"]
        cls.fixture_set = load_json(REPO_ROOT / FIXTURE_REF)

    def test_skill_builder_contract_compiles(self) -> None:
        receipt = compile_skill(REPO_ROOT, "skill-builder")
        self.assertEqual(receipt["result"], "PASS")
        self.assertTrue(receipt["source"]["unchanged"])
        self.assertEqual(receipt["source"]["before_sha256"], receipt["source"]["after_sha256"])
        self.assertEqual(receipt["contract"]["schema_ref"], "schemas/skill-contract.v3.schema.json")
        schema = load_json(REPO_ROOT / REPORT_SCHEMA_REF)
        errors = list(Draft7Validator(schema).iter_errors(receipt))
        self.assertEqual([], errors, [error.message for error in errors])

    def test_hostile_fixture_corpus_rejects_for_intended_cause(self) -> None:
        cases = self.fixture_set["cases"]
        self.assertGreaterEqual(len(cases), 35)
        self.assertEqual(len(cases), len({case["id"] for case in cases}))
        for case in cases:
            with self.subTest(case=case["id"]):
                contract = copy.deepcopy(self.base_contract)
                for mutation in case["mutations"]:
                    apply_mutation(contract, mutation)
                with self.assertRaises(ContractError) as caught:
                    validate_contract(
                        contract,
                        skill_name=case.get("skill_name", "skill-builder"),
                        dependencies=case.get("dependencies", self.base_dependencies),
                        repo_root=REPO_ROOT,
                    )
                self.assertEqual(
                    case["expected_code"],
                    caught.exception.code,
                    caught.exception.message,
                )

    def test_duplicate_yaml_keys_fail_closed(self) -> None:
        with tempfile.TemporaryDirectory(
            prefix=".contract-v3-yaml-",
            dir=REPO_ROOT / "skills/skill-builder",
        ) as temporary:
            source = Path(temporary) / "SKILL.md"
            source.write_text(
                "---\n"
                "name: first\n"
                "name: second\n"
                "metadata: {}\n"
                "---\n"
                "# body\n",
                encoding="utf-8",
            )
            with self.assertRaises(ContractError) as caught:
                load_frontmatter(source)
            self.assertEqual("DUPLICATE_YAML_KEY", caught.exception.code)

    def test_check_is_read_only_and_record_bytes_match(self) -> None:
        source = REPO_ROOT / SOURCE_REF
        before = source.read_bytes()
        package_before = byte_snapshot(REPO_ROOT / "skills/skill-builder")
        environment = os.environ.copy()
        environment["PYTHONDONTWRITEBYTECODE"] = "1"
        check = subprocess.run(
            [
                sys.executable,
                CLI_REF,
                "check",
                "--skill",
                "skill-builder",
            ],
            cwd=REPO_ROOT,
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=environment,
        )
        self.assertEqual(before, source.read_bytes())
        self.assertEqual(
            package_before,
            byte_snapshot(REPO_ROOT / "skills/skill-builder"),
        )
        self.assertEqual(canonical_bytes(json.loads(check.stdout)), check.stdout)
        with tempfile.TemporaryDirectory(
            prefix=".contract-v3-record-",
            dir=REPO_ROOT / "skills/skill-builder",
        ) as temporary:
            output = Path(temporary) / "receipt.json"
            subprocess.run(
                [
                    sys.executable,
                    CLI_REF,
                    "record",
                    "--skill",
                    "skill-builder",
                    "--output",
                    str(output.relative_to(REPO_ROOT)),
                ],
                cwd=REPO_ROOT,
                check=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                env=environment,
            )
            self.assertEqual(check.stdout, output.read_bytes())
        self.assertEqual(before, source.read_bytes())

    def test_check_failure_does_not_create_output(self) -> None:
        result = subprocess.run(
            [
                sys.executable,
                CLI_REF,
                "check",
                "--skill",
                "plan",
            ],
            cwd=REPO_ROOT,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env={**os.environ, "PYTHONDONTWRITEBYTECODE": "1"},
        )
        self.assertEqual(1, result.returncode)
        self.assertIn(b"[CONTRACT_V3_ABSENT]", result.stderr)


if __name__ == "__main__":
    unittest.main()
