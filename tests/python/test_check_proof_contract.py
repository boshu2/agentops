from __future__ import annotations

import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
CHECKER = ROOT / "scripts" / "check-proof-contract.py"
FROZEN_PATHS = [
    "docs/contracts/proof-contracts/active.json",
    "docs/contracts/proof-contracts/epoch-0/descriptor.json",
    "scripts/bootstrap-proof-transition.py",
    "skills/validate/SKILL.md",
    "skills/validate/scripts/validate.py",
    "skills/validate/scripts/check_contract_corpus.py",
    "schemas/verdict.v2.schema.json",
    "schemas/rpi-report.v1.schema.json",
    "schemas/subject-manifest.v1.schema.json",
    "cli/internal/verdictcheck/verdictcheck.go",
]


class ProofContractCheckTest(unittest.TestCase):
    def run_checker(self, repository: Path) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [
                sys.executable,
                str(CHECKER),
                "--repository",
                str(repository),
            ],
            text=True,
            capture_output=True,
            check=False,
        )

    def build_fixture(self, destination: Path) -> None:
        for relative in FROZEN_PATHS:
            source = ROOT / relative
            target = destination / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(source, target)
        corpus_source = ROOT / "tests" / "fixtures" / "verdict-contract" / "cases"
        shutil.copytree(
            corpus_source,
            destination / "tests" / "fixtures" / "verdict-contract" / "cases",
        )

    def test_repository_epoch_zero_freeze_is_valid(self) -> None:
        result = self.run_checker(ROOT)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout)["epoch"], 0)

    def test_seeded_component_mutation_is_detected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            repository = Path(temporary)
            self.build_fixture(repository)
            target = repository / "schemas" / "verdict.v2.schema.json"
            target.write_text(target.read_text() + "\n")
            result = self.run_checker(repository)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("frozen component digest changed", result.stderr)

    def test_seeded_corpus_mutation_is_detected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            repository = Path(temporary)
            self.build_fixture(repository)
            target = next(
                (
                    repository
                    / "tests"
                    / "fixtures"
                    / "verdict-contract"
                    / "cases"
                ).glob("*.json")
            )
            target.write_text(target.read_text() + "\n")
            result = self.run_checker(repository)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("qualification corpus digest changed", result.stderr)


if __name__ == "__main__":
    unittest.main()
