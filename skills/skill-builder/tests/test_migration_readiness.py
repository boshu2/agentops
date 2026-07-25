#!/usr/bin/env python3
"""Focused checks for the exact T2 migration-readiness ledger."""

from __future__ import annotations

import copy
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

from jsonschema import Draft7Validator


REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPT_DIR = REPO_ROOT / "skills/skill-builder/scripts"
sys.path.insert(0, str(SCRIPT_DIR))

import check_migration_readiness as readiness  # noqa: E402
from contract_v3 import ContractError, compile_skill  # noqa: E402


LEDGER_REF = "skills/skill-builder/ledgers/migration-readiness.json"
SCHEMA_REF = "schemas/skill-migration-readiness.v1.schema.json"
CHECKER_REF = "skills/skill-builder/scripts/check_migration_readiness.py"


class MigrationReadinessTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.ledger = json.loads((REPO_ROOT / LEDGER_REF).read_text(encoding="utf-8"))
        cls.schema = json.loads((REPO_ROOT / SCHEMA_REF).read_text(encoding="utf-8"))

    def run_checker(self, ledger: Path) -> subprocess.CompletedProcess[bytes]:
        return subprocess.run(
            [
                sys.executable,
                CHECKER_REF,
                "check",
                "--ledger",
                str(ledger.relative_to(REPO_ROOT)),
            ],
            cwd=REPO_ROOT,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

    def test_schema_and_exact_inventory(self) -> None:
        errors = list(Draft7Validator(self.schema).iter_errors(self.ledger))
        self.assertEqual([], errors, [error.message for error in errors])
        rows = self.ledger["rows"]
        self.assertEqual(49, len(rows))
        self.assertEqual(
            sorted(path.parent.name for path in (REPO_ROOT / "skills").glob("*/SKILL.md")),
            [row["name"] for row in rows],
        )
        ready = [row["name"] for row in rows if row["cutover_ready"]]
        self.assertEqual(["skill-builder"], ready)
        blocked = [row for row in rows if not row["cutover_ready"]]
        self.assertEqual(48, len(blocked))
        self.assertTrue(all(row["blockers"] for row in blocked))

    def test_checker_is_read_only(self) -> None:
        path = REPO_ROOT / LEDGER_REF
        before = path.read_bytes()
        result = self.run_checker(path)
        self.assertEqual(0, result.returncode, result.stderr.decode())
        self.assertEqual(before, path.read_bytes())

    def test_stale_identity_fails_for_intended_cause(self) -> None:
        hostile = copy.deepcopy(self.ledger)
        hostile["rows"][0]["source_sha256"] = "0" * 64
        with tempfile.TemporaryDirectory(
            prefix=".readiness-hostile-",
            dir=REPO_ROOT / "skills/skill-builder",
        ) as temporary:
            path = Path(temporary) / "ledger.json"
            path.write_text(json.dumps(hostile), encoding="utf-8")
            result = self.run_checker(path)
        self.assertEqual(1, result.returncode)
        self.assertIn(b"[LEDGER_STALE]", result.stderr)

    def test_unknown_ledger_field_fails_closed(self) -> None:
        hostile = copy.deepcopy(self.ledger)
        hostile["rows"][0]["implicit_ready"] = True
        with tempfile.TemporaryDirectory(
            prefix=".readiness-hostile-",
            dir=REPO_ROOT / "skills/skill-builder",
        ) as temporary:
            path = Path(temporary) / "ledger.json"
            path.write_text(json.dumps(hostile), encoding="utf-8")
            result = self.run_checker(path)
        self.assertEqual(1, result.returncode)
        self.assertIn(b"[LEDGER_SCHEMA_INVALID]", result.stderr)

    def test_probe_harness_identity_is_recomputed(self) -> None:
        compiled = compile_skill(REPO_ROOT, "skill-builder")
        receipt = json.loads(
            (REPO_ROOT / readiness.PROBE_REF).read_text(encoding="utf-8")
        )
        hostile = copy.deepcopy(receipt)
        hostile["proof"]["entrypoint"]["sha256"] = "0" * 64
        real_load_json = readiness.load_json

        def load_with_hostile_probe(path: Path) -> object:
            if path.resolve() == (REPO_ROOT / readiness.PROBE_REF).resolve():
                return hostile
            return real_load_json(path)

        with mock.patch.object(
            readiness,
            "load_json",
            side_effect=load_with_hostile_probe,
        ):
            with self.assertRaises(ContractError) as caught:
                readiness.validated_probe(REPO_ROOT, compiled)
        self.assertEqual("PROBE_RECEIPT_INVALID", caught.exception.code)


if __name__ == "__main__":
    unittest.main()
