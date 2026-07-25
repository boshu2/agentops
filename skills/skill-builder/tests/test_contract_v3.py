#!/usr/bin/env python3
"""Behavioral proof for the closed shadow skill-contract.v3 compiler."""

from __future__ import annotations

import ast
import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock
from uuid import uuid4

from jsonschema import Draft7Validator


REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPT_DIR = REPO_ROOT / "skills/skill-builder/scripts"
sys.path.insert(0, str(SCRIPT_DIR))

import contract_v3  # noqa: E402
from contract_v3 import (  # noqa: E402
    ContractError,
    canonical_bytes,
    compile_skill,
    load_frontmatter,
    load_json,
    validate_contract,
)


FIXTURE_REF = "skills/skill-builder/fixtures/contract-v3/cases.json"
INVENTORY_REF = "skills/skill-builder/fixtures/contract-v3/invariants.json"
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
        cls.inventory = load_json(REPO_ROOT / INVENTORY_REF)

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
        self.assertEqual(len(cases), len({case["id"] for case in cases}))
        inventory = self.inventory["invariants"]
        self.assertEqual(contract_v3.invariant_inventory(), self.inventory)
        self.assertEqual(len(inventory), len({item["id"] for item in inventory}))
        self.assertEqual(
            {item["id"]: item["expected_code"] for item in inventory},
            {case["id"]: case["expected_code"] for case in cases},
        )
        for case in cases:
            with self.subTest(case=case["id"]):
                with self.assertRaises(ContractError) as caught:
                    self.run_hostile_case(case)
                self.assertEqual(
                    case["expected_code"],
                    caught.exception.code,
                    caught.exception.message,
                )

    def test_inventory_covers_every_literal_compiler_rejection_code(self) -> None:
        tree = ast.parse((SCRIPT_DIR / "contract_v3.py").read_text(encoding="utf-8"))
        rejection_codes: set[str] = set()
        for node in ast.walk(tree):
            if isinstance(node, ast.Call) and isinstance(node.func, ast.Name):
                if node.func.id == "ContractError" and node.args:
                    rejection_codes.update(
                        value.value
                        for value in ast.walk(node.args[0])
                        if isinstance(value, ast.Constant)
                        and isinstance(value.value, str)
                        and value.value.isupper()
                    )
            if (
                isinstance(node, ast.FunctionDef)
                and node.name == "_schema_error_code"
            ):
                rejection_codes.update(
                    value.value
                    for value in ast.walk(node)
                    if isinstance(value, ast.Constant)
                    and isinstance(value.value, str)
                    and value.value.isupper()
                )
        inventory_codes = {
            item["expected_code"] for item in self.inventory["invariants"]
        }
        self.assertEqual(rejection_codes, inventory_codes)

    def test_invalid_unicode_scalar_is_a_typed_contract_rejection(self) -> None:
        contract = copy.deepcopy(self.base_contract)
        contract["failure"]["timeout"]["detail"] = "\ud800"
        with self.assertRaises(ContractError) as caught:
            validate_contract(
                contract,
                skill_name="skill-builder",
                dependencies=self.base_dependencies,
                repo_root=REPO_ROOT,
            )
        self.assertEqual("INVALID_UNICODE", caught.exception.code)

    def run_hostile_case(self, case: dict[str, object]) -> None:
        witness = case.get("witness")
        if witness == "duplicate_yaml":
            self.run_duplicate_yaml_witness()
            return
        if witness == "duplicate_json":
            self.run_duplicate_json_witness()
            return
        if witness == "source_mutation":
            self.run_source_mutation_witness()
            return
        if witness == "nonexecutable_harness":
            self.run_nonexecutable_harness_witness()
            return
        if witness == "unavailable_interpreter":
            self.run_unavailable_interpreter_witness()
            return
        if witness == "invalid_yaml_key":
            self.run_invalid_yaml_key_witness()
            return
        if witness == "invalid_json":
            self.run_invalid_json_witness()
            return
        if witness == "invalid_frontmatter":
            self.run_invalid_frontmatter_witness()
            return
        if witness == "invalid_skill_name":
            compile_skill(REPO_ROOT, "Invalid")
            return
        if witness == "source_unavailable":
            compile_skill(REPO_ROOT, "missing-contract-fixture")
            return
        if witness == "skill_name_mismatch":
            self.run_early_source_witness(name="other-name", metadata="metadata: {}")
            return
        if witness == "contract_absent":
            self.run_early_source_witness(name="fixture-skill", metadata="metadata: {}")
            return
        if witness == "invalid_unicode":
            contract = copy.deepcopy(self.base_contract)
            contract["failure"]["timeout"]["detail"] = "\ud800"
            validate_contract(
                contract,
                skill_name="skill-builder",
                dependencies=self.base_dependencies,
                repo_root=REPO_ROOT,
            )
            return
        if witness == "io_error":
            with mock.patch.object(
                contract_v3,
                "compile_skill",
                side_effect=OSError("hostile compiler I/O"),
            ):
                receipt = contract_v3.compile_receipt(REPO_ROOT, "skill-builder")
            error = receipt["errors"][0]
            raise ContractError(error["code"], error["message"])
        contract = copy.deepcopy(self.base_contract)
        for mutation in case["mutations"]:
            apply_mutation(contract, mutation)
        validate_contract(
            contract,
            skill_name=case.get("skill_name", "skill-builder"),
            dependencies=case.get("dependencies", self.base_dependencies),
            repo_root=REPO_ROOT,
        )

    def run_duplicate_yaml_witness(self) -> None:
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
            load_frontmatter(source)

    def run_duplicate_json_witness(self) -> None:
        with tempfile.TemporaryDirectory(
            prefix=".contract-v3-json-",
            dir=REPO_ROOT / "skills/skill-builder",
        ) as temporary:
            source = Path(temporary) / "fixture.json"
            source.write_text('{"same": 1, "same": 2}\n', encoding="utf-8")
            load_json(source)

    def run_source_mutation_witness(self) -> None:
        source = (REPO_ROOT / SOURCE_REF).resolve()
        real_file_sha256 = contract_v3.file_sha256
        source_calls = 0

        def changing_digest(path: Path) -> str:
            nonlocal source_calls
            digest = real_file_sha256(path)
            if path.resolve() == source:
                source_calls += 1
                if source_calls == 2:
                    return "0" * 64
            return digest

        with mock.patch.object(contract_v3, "file_sha256", side_effect=changing_digest):
            compile_skill(REPO_ROOT, "skill-builder")

    def run_nonexecutable_harness_witness(self) -> None:
        with tempfile.TemporaryDirectory(
            prefix=".contract-v3-mode-",
            dir=REPO_ROOT / "skills/skill-builder",
        ) as temporary:
            script = Path(temporary) / "proof.sh"
            script.write_text("#!/usr/bin/env bash\nexit 0\n", encoding="utf-8")
            script.chmod(0o600)
            ref = script.relative_to(REPO_ROOT).as_posix()
            contract = copy.deepcopy(self.base_contract)
            contract["proof"]["command"] = ref
            contract["proof"]["harness_refs"].append(ref)
            validate_contract(
                contract,
                skill_name="skill-builder",
                dependencies=self.base_dependencies,
                repo_root=REPO_ROOT,
            )

    def run_unavailable_interpreter_witness(self) -> None:
        contract = copy.deepcopy(self.base_contract)
        with mock.patch.object(contract_v3.shutil, "which", return_value=None):
            validate_contract(
                contract,
                skill_name="skill-builder",
                dependencies=self.base_dependencies,
                repo_root=REPO_ROOT,
            )

    def run_invalid_yaml_key_witness(self) -> None:
        with tempfile.TemporaryDirectory(prefix=".contract-v3-yaml-key-") as temporary:
            source = Path(temporary) / "SKILL.md"
            source.write_text(
                "---\n"
                "? [first, second]\n"
                ": value\n"
                "---\n"
                "# body\n",
                encoding="utf-8",
            )
            load_frontmatter(source)

    def run_invalid_json_witness(self) -> None:
        with tempfile.TemporaryDirectory(prefix=".contract-v3-invalid-json-") as temporary:
            source = Path(temporary) / "fixture.json"
            source.write_text('{"unterminated":\n', encoding="utf-8")
            load_json(source)

    def run_invalid_frontmatter_witness(self) -> None:
        with tempfile.TemporaryDirectory(prefix=".contract-v3-frontmatter-") as temporary:
            source = Path(temporary) / "SKILL.md"
            source.write_text("# no frontmatter\n", encoding="utf-8")
            load_frontmatter(source)

    def run_early_source_witness(self, *, name: str, metadata: str) -> None:
        with tempfile.TemporaryDirectory(prefix=".contract-v3-source-") as temporary:
            root = Path(temporary)
            source = root / "skills/fixture-skill/SKILL.md"
            source.parent.mkdir(parents=True)
            source.write_text(
                f"---\nname: {name}\n{metadata}\n---\n# body\n",
                encoding="utf-8",
            )
            compile_skill(root, "fixture-skill")

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

    def test_check_and_record_failure_emit_the_same_typed_receipt(self) -> None:
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
        receipt = json.loads(result.stdout)
        schema = load_json(REPO_ROOT / REPORT_SCHEMA_REF)
        errors = list(Draft7Validator(schema).iter_errors(receipt))
        self.assertEqual([], errors, [error.message for error in errors])
        self.assertEqual("FAIL", receipt["result"])
        self.assertEqual("plan", receipt["skill"])
        self.assertIsNotNone(receipt["source"]["before_sha256"])
        self.assertIsNone(receipt["contract"]["digest"])
        self.assertIsNone(receipt["proof"])
        self.assertEqual([], receipt["checks"])
        self.assertEqual("CONTRACT_V3_ABSENT", receipt["errors"][0]["code"])
        with tempfile.TemporaryDirectory(
            prefix=".contract-v3-fail-record-",
            dir=REPO_ROOT / "skills/skill-builder",
        ) as temporary:
            output = Path(temporary) / "receipt.json"
            recorded = subprocess.run(
                [
                    sys.executable,
                    CLI_REF,
                    "record",
                    "--skill",
                    "plan",
                    "--output",
                    str(output.relative_to(REPO_ROOT)),
                ],
                cwd=REPO_ROOT,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                env={**os.environ, "PYTHONDONTWRITEBYTECODE": "1"},
            )
            self.assertEqual(1, recorded.returncode)
            self.assertEqual(result.stdout, output.read_bytes())

    def test_check_and_record_invalid_unicode_emit_typed_receipt(self) -> None:
        skill_name = f"unicode-hostile-{uuid4().hex}"
        package = REPO_ROOT / "skills" / skill_name
        package.mkdir()
        try:
            source_text = (REPO_ROOT / SOURCE_REF).read_text(encoding="utf-8")
            source_text = source_text.replace(
                "name: skill-builder",
                f"name: {skill_name}",
                1,
            ).replace(
                "canonical_skill: skill-builder",
                f"canonical_skill: {skill_name}",
                1,
            ).replace(
                "detail: Stop the bounded command and return its timeout without retrying.",
                'detail: "\\uD800"',
                1,
            )
            (package / "SKILL.md").write_text(source_text, encoding="utf-8")
            command = [
                sys.executable,
                CLI_REF,
                "check",
                "--skill",
                skill_name,
            ]
            checked = subprocess.run(
                command,
                cwd=REPO_ROOT,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                env={**os.environ, "PYTHONDONTWRITEBYTECODE": "1"},
            )
            self.assertEqual(1, checked.returncode)
            self.assertIn(b"[INVALID_UNICODE]", checked.stderr)
            receipt = json.loads(checked.stdout)
            schema = load_json(REPO_ROOT / REPORT_SCHEMA_REF)
            errors = list(Draft7Validator(schema).iter_errors(receipt))
            self.assertEqual([], errors, [error.message for error in errors])
            self.assertEqual("FAIL", receipt["result"])
            self.assertEqual("INVALID_UNICODE", receipt["errors"][0]["code"])
            self.assertIsNone(receipt["contract"]["digest"])
            output = package / "receipt.json"
            recorded = subprocess.run(
                [
                    sys.executable,
                    CLI_REF,
                    "record",
                    "--skill",
                    skill_name,
                    "--output",
                    str(output.relative_to(REPO_ROOT)),
                ],
                cwd=REPO_ROOT,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                env={**os.environ, "PYTHONDONTWRITEBYTECODE": "1"},
            )
            self.assertEqual(1, recorded.returncode)
            self.assertEqual(checked.stdout, output.read_bytes())
        finally:
            shutil.rmtree(package)


if __name__ == "__main__":
    unittest.main()
