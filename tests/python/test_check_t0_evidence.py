from __future__ import annotations

import hashlib
import importlib.util
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
SPEC = importlib.util.spec_from_file_location("agentops_check_t0_evidence", CHECKER)
assert SPEC is not None and SPEC.loader is not None
CHECK_MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECK_MODULE)


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
            self.assertIn("pause drill authority", result.stderr)

    def test_pause_false_t1_completion_is_detected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            shutil.copytree(EVIDENCE, evidence, dirs_exist_ok=True)
            pause_path = evidence / "t0-pause-drill.json"
            pause = json.loads(pause_path.read_text())
            pause["progress"]["states"]["T1"] = "COMPLETE"
            pause_path.write_text(json.dumps(pause))
            result = self.run_checker(ROOT, evidence)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("pause drill typed progress", result.stderr)

    def test_pause_semantic_completion_in_safe_stop_claim_is_detected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            shutil.copytree(EVIDENCE, evidence, dirs_exist_ok=True)
            pause_path = evidence / "t0-pause-drill.json"
            pause = json.loads(pause_path.read_text())
            pause["safe_stop_claim"] += " T1 is complete and epoch 1 is active."
            pause_path.write_text(json.dumps(pause))
            result = self.run_checker(ROOT, evidence)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("safe-stop claim", result.stderr)

    def test_pause_semantic_completion_in_extra_gap_is_detected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            shutil.copytree(EVIDENCE, evidence, dirs_exist_ok=True)
            pause_path = evidence / "t0-pause-drill.json"
            pause = json.loads(pause_path.read_text())
            pause["known_gaps"].append("T1 has completed")
            pause_path.write_text(json.dumps(pause))
            result = self.run_checker(ROOT, evidence)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("known gaps", result.stderr)

    def test_future_active_pointer_must_bind_transition_candidate(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            repository = Path(temporary) / "repo"
            evidence = repository / "docs/evidence/proof-epochs/epoch-0"
            epoch0b = repository / "docs/evidence/proof-epochs/epoch-0b"
            shutil.copytree(EVIDENCE, evidence)
            shutil.copytree(
                ROOT / "docs/evidence/proof-epochs/epoch-0b",
                epoch0b,
            )
            descriptor = repository / CHECK_MODULE.PAUSE_PROOF_REF
            descriptor.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(ROOT / CHECK_MODULE.PAUSE_PROOF_REF, descriptor)

            candidate_ref = "docs/contracts/proof-contracts/epoch-1/descriptor.json"
            candidate = repository / candidate_ref
            candidate.parent.mkdir(parents=True, exist_ok=True)
            candidate.write_text("{}\n")
            candidate_digest = hashlib.sha256(candidate.read_bytes()).hexdigest()
            transition = {
                "schema_version": "proof-contract-transition.v1",
                "prior": {
                    "epoch": 0,
                    "contract_ref": CHECK_MODULE.PAUSE_PROOF_REF,
                    "contract_digest": CHECK_MODULE.PAUSE_PROOF_DIGEST,
                    "activation_transition_digest": None,
                },
                "candidate": {
                    "epoch": 1,
                    "contract_ref": candidate_ref,
                    "contract_digest": candidate_digest,
                },
            }
            transition_payload = (
                json.dumps(transition, sort_keys=True, separators=(",", ":")) + "\n"
            ).encode()
            transition_digest = hashlib.sha256(transition_payload).hexdigest()
            transition_ref = (
                f"docs/contracts/proof-contracts/transitions/{transition_digest}.json"
            )
            transition_path = repository / transition_ref
            transition_path.parent.mkdir(parents=True, exist_ok=True)
            transition_path.write_bytes(transition_payload)
            active_path = repository / "docs/contracts/proof-contracts/active.json"
            active_path.write_text(
                json.dumps(
                    {
                        "schema_version": "proof-contract-active.v1",
                        "epoch": 1,
                        "contract_ref": "docs/contracts/proof-contracts/epoch-1/fabricated.json",
                        "contract_digest": "f" * 64,
                        "activation_transition_ref": transition_ref,
                        "activation_transition_digest": transition_digest,
                    }
                )
            )
            pause = json.loads((evidence / "t0-pause-drill.json").read_text())
            with self.assertRaisesRegex(
                CHECK_MODULE.EvidenceError,
                "not bound to the transition candidate",
            ):
                CHECK_MODULE.check_pause_state(repository, evidence, pause)
            active_path.write_text(
                json.dumps(
                    {
                        "schema_version": "proof-contract-active.v1",
                        "epoch": 1,
                        "contract_ref": candidate_ref,
                        "contract_digest": candidate_digest,
                        "activation_transition_ref": transition_ref,
                        "activation_transition_digest": transition_digest,
                    }
                )
            )
            CHECK_MODULE.check_pause_state(repository, evidence, pause)

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
