from __future__ import annotations

import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
CHECKER = ROOT / "scripts" / "check-t0-evidence.py"
EVIDENCE = ROOT / "docs" / "evidence" / "proof-epochs" / "epoch-0"


class T0EvidenceCheckTest(unittest.TestCase):
    def run_checker(
        self, repository: Path, evidence: Path
    ) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [
                sys.executable,
                str(CHECKER),
                "--repository",
                str(repository),
                "--evidence-root",
                str(evidence),
            ],
            text=True,
            capture_output=True,
            check=False,
        )

    def test_live_t0_ledgers_are_coherent(self) -> None:
        result = self.run_checker(ROOT, EVIDENCE)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout)["skill_count"], 49)

    def test_seeded_skill_digest_mutation_is_detected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            repository = Path(temporary)
            evidence = repository / "evidence"
            shutil.copytree(EVIDENCE, evidence)
            ledger = json.loads((evidence / "t0-skill-ledger.json").read_text())
            row = ledger["skills"][0]
            target = repository / row["path"]
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text("mutated\n")
            result = self.run_checker(repository, evidence)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("skill ledger digest changed", result.stderr)

    def test_seeded_routing_summary_mismatch_is_detected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            shutil.copytree(EVIDENCE, evidence, dirs_exist_ok=True)
            routing_path = evidence / "routing-baseline.json"
            routing = json.loads(routing_path.read_text())
            routing["observation"]["summary"]["acceptable"] += 1
            routing_path.write_text(json.dumps(routing))
            result = self.run_checker(ROOT, evidence)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("routing assessment summary", result.stderr)

    def test_pause_lineage_removal_is_detected_despite_pass_result(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            shutil.copytree(EVIDENCE, evidence, dirs_exist_ok=True)
            pause_path = evidence / "t0-pause-drill.json"
            pause = json.loads(pause_path.read_text())
            pause["lineage"] = {}
            pause_path.write_text(json.dumps(pause))
            result = self.run_checker(ROOT, evidence)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("pause drill lineage", result.stderr)

    def test_pause_rejected_proof_authority_is_detected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            shutil.copytree(EVIDENCE, evidence, dirs_exist_ok=True)
            pause_path = evidence / "t0-pause-drill.json"
            pause = json.loads(pause_path.read_text())
            pause["authority"]["proof_contract_ref"] = (
                "docs/contracts/proof-contracts/epoch-0/descriptor.json"
            )
            pause_path.write_text(json.dumps(pause))
            result = self.run_checker(ROOT, evidence)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("pause drill proof authority", result.stderr)

    def test_pause_false_t1_completion_is_detected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            shutil.copytree(EVIDENCE, evidence, dirs_exist_ok=True)
            pause_path = evidence / "t0-pause-drill.json"
            pause = json.loads(pause_path.read_text())
            pause["in_flight"] = ["T1 complete"]
            pause_path.write_text(json.dumps(pause))
            result = self.run_checker(ROOT, evidence)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("pause drill in-flight state", result.stderr)

    def test_rejected_verdict_byte_mutation_is_detected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            shutil.copytree(EVIDENCE, evidence, dirs_exist_ok=True)
            verdict = next((evidence / "verdicts").glob("*.json"))
            verdict.write_bytes(verdict.read_bytes() + b" ")
            result = self.run_checker(ROOT, evidence)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("failed verdict bytes changed", result.stderr)


if __name__ == "__main__":
    unittest.main()
