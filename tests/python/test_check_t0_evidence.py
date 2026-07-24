from __future__ import annotations

import copy
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
            for reference in (
                CHECK_MODULE.ACTIVE_SCHEMA_REF,
                CHECK_MODULE.TRANSITION_SCHEMA_REF,
                CHECK_MODULE.PROOF_SCHEMA_REF,
                CHECK_MODULE.SUBJECT_SCHEMA_REF,
                CHECK_MODULE.VERDICT_SCHEMA_REF,
                "scripts/bootstrap-proof-transition.py",
            ):
                target = repository / reference
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(ROOT / reference, target)

            corpus_ref = "tests/fixtures/t0-epoch1-corpus"
            corpus = repository / corpus_ref
            corpus.mkdir(parents=True)
            (corpus / "case.json").write_text('{"case":"pass"}\n')
            bootstrap = CHECK_MODULE.load_bootstrap_module(repository)
            corpus_digest = bootstrap.tree_digest(corpus)

            components = []
            for role in [
                "validator-contract",
                "validator-implementation",
                "verdict-schema",
                "rpi-report-schema",
                "subject-manifest-schema",
            ]:
                reference = f"components/{role}"
                path = repository / reference
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(f"{role}\n")
                path.chmod(0o644)
                components.append(
                    {
                        "role": role,
                        "ref": reference,
                        "digest": hashlib.sha256(path.read_bytes()).hexdigest(),
                        "mode": "0644",
                    }
                )
            recorder_ref = "scripts/future-transition.py"
            recorder_path = repository / recorder_ref
            recorder_path.write_text("#!/usr/bin/env python3\n")
            recorder_path.chmod(0o755)
            recorder = {
                "ref": recorder_ref,
                "digest": hashlib.sha256(recorder_path.read_bytes()).hexdigest(),
                "mode": "0755",
            }
            frozen_items = components + [recorder]
            subject_ref = "docs/evidence/proof-epochs/epoch-1/subject.json"
            subject_path = repository / subject_ref
            subject_path.parent.mkdir(parents=True, exist_ok=True)
            subject = {
                "schema_version": "subject-manifest.v1",
                "declared_roots": [item["ref"] for item in frozen_items],
                "exclusions": [],
                "entries": [
                    {
                        "path": item["ref"],
                        "kind": "file",
                        "executable": item["mode"] == "0755",
                        "digest": item["digest"],
                    }
                    for item in frozen_items
                ],
            }
            subject["canonical_manifest_digest"] = CHECK_MODULE.canonical_digest(
                subject
            )
            subject_path.write_text(json.dumps(subject, sort_keys=True))

            verdict_ref_prefix = "docs/evidence/proof-epochs/epoch-1/verdicts"
            verdict = {
                "schema_version": "verdict.v2",
                "acceptance_digest": "b" * 64,
                "subject_manifest_digest": subject["canonical_manifest_digest"],
                "author_context_id": "author",
                "validator_context_id": "validator",
                "freshness_attestation": {
                    "source": "runtime",
                    "attester_identity": "validator",
                },
                "verdict": "PASS",
                "criteria": [
                    {
                        "id": "Q-1",
                        "result": "PASS",
                        "evidence_refs": ["fixture"],
                    }
                ],
                "findings": [],
                "evidence_refs": ["fixture"],
                "checked": ["fixture"],
                "not_checked": [],
                "validated_at": "2026-07-24T20:00:00Z",
            }
            verdict["artifact_digest"] = CHECK_MODULE.canonical_digest(verdict)
            verdict_ref = (
                f"{verdict_ref_prefix}/{verdict['artifact_digest']}.json"
            )
            verdict_path = repository / verdict_ref
            verdict_path.parent.mkdir(parents=True, exist_ok=True)
            verdict_path.write_text(json.dumps(verdict, sort_keys=True))

            candidate_ref = "docs/contracts/proof-contracts/epoch-1/descriptor.json"
            candidate = repository / candidate_ref
            candidate.parent.mkdir(parents=True, exist_ok=True)
            candidate_value = {
                "schema_version": "proof-contract.v1",
                "epoch": 1,
                "components": components,
                "qualification_corpus": {
                    "algorithm": "sha256-tree-v1",
                    "ref": corpus_ref,
                    "digest": corpus_digest,
                },
                "qualification_subject_manifest_digest": subject[
                    "canonical_manifest_digest"
                ],
                "transition_recorder": recorder,
                "known_gaps": [],
            }
            candidate.write_text(json.dumps(candidate_value, sort_keys=True))
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
                    "subject_manifest_ref": subject_ref,
                    "subject_manifest_digest": subject[
                        "canonical_manifest_digest"
                    ],
                    "qualification_corpus_ref": corpus_ref,
                    "qualification_corpus_digest": corpus_digest,
                },
                "qualification_verdict": {
                    "ref": verdict_ref,
                    "digest": verdict["artifact_digest"],
                },
                "validator_identity": "validator",
                "activated_at": "2026-07-24T20:01:00Z",
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

            malformed_cases: list[tuple[str, dict[str, object]]] = []
            extra_transition = copy.deepcopy(transition)
            extra_transition["unexpected"] = True
            malformed_cases.append(("extra transition field", extra_transition))
            extra_candidate = copy.deepcopy(transition)
            extra_candidate["candidate"]["unexpected"] = True
            malformed_cases.append(("extra candidate field", extra_candidate))
            escaping_verdict = copy.deepcopy(transition)
            escaping_verdict["qualification_verdict"]["ref"] = "../escape.json"
            malformed_cases.append(("escaping qualification ref", escaping_verdict))
            boolean_epoch = copy.deepcopy(transition)
            boolean_epoch["candidate"]["epoch"] = True
            malformed_cases.append(("boolean candidate epoch", boolean_epoch))
            malformed_time = copy.deepcopy(transition)
            malformed_time["activated_at"] = "not-a-time"
            malformed_cases.append(("malformed activation time", malformed_time))
            for label, malformed_case in malformed_cases:
                with self.subTest(label=label):
                    with self.assertRaisesRegex(
                        CHECK_MODULE.EvidenceError,
                        "active transition violates its schema",
                    ):
                        CHECK_MODULE.validate_schema_instance(
                            repository,
                            schema_ref=CHECK_MODULE.TRANSITION_SCHEMA_REF,
                            schema_digest=CHECK_MODULE.TRANSITION_SCHEMA_DIGEST,
                            instance=malformed_case,
                            label="active transition",
                        )

            malformed = dict(transition)
            malformed.pop("qualification_verdict")
            malformed_payload = (
                json.dumps(malformed, sort_keys=True, separators=(",", ":")) + "\n"
            ).encode()
            malformed_digest = hashlib.sha256(malformed_payload).hexdigest()
            malformed_ref = (
                f"docs/contracts/proof-contracts/transitions/{malformed_digest}.json"
            )
            malformed_path = repository / malformed_ref
            malformed_path.write_bytes(malformed_payload)
            active_path.write_text(
                json.dumps(
                    {
                        "schema_version": "proof-contract-active.v1",
                        "epoch": 1,
                        "contract_ref": candidate_ref,
                        "contract_digest": candidate_digest,
                        "activation_transition_ref": malformed_ref,
                        "activation_transition_digest": malformed_digest,
                    }
                )
            )
            with self.assertRaisesRegex(
                CHECK_MODULE.EvidenceError,
                "active transition violates its schema",
            ):
                CHECK_MODULE.check_pause_state(repository, evidence, pause)

            duplicate_path = repository / "duplicate.json"
            duplicate_path.write_text('{"epoch":1,"epoch":1}\n')
            with self.assertRaisesRegex(
                CHECK_MODULE.EvidenceError,
                "duplicate JSON key",
            ):
                CHECK_MODULE.load(duplicate_path)

            alias_parent = repository / "component-alias"
            alias_parent.symlink_to(repository / "components", target_is_directory=True)
            with self.assertRaisesRegex(
                CHECK_MODULE.EvidenceError,
                "uses a symlinked path component",
            ):
                CHECK_MODULE.resolve_repository_ref(
                    repository,
                    "component-alias/validator-contract",
                    "aliased component",
                )
            direct_alias = repository / "direct-alias"
            direct_alias.symlink_to(repository / "components/validator-contract")
            with self.assertRaisesRegex(
                CHECK_MODULE.EvidenceError,
                "uses a symlinked path component",
            ):
                CHECK_MODULE.resolve_repository_ref(
                    repository,
                    "direct-alias",
                    "direct alias",
                )
            outside = Path(temporary) / "outside"
            outside.write_text("outside\n")
            outside_alias = repository / "outside-alias"
            outside_alias.symlink_to(outside)
            with self.assertRaisesRegex(
                CHECK_MODULE.EvidenceError,
                "uses a symlinked path component",
            ):
                CHECK_MODULE.resolve_repository_ref(
                    repository,
                    "outside-alias",
                    "outside alias",
                )
            for reference in (
                "/absolute",
                "../parent",
                "a/../parent",
                "a/./dot",
                "a//empty",
                "a\\windows",
                "trailing/",
            ):
                with self.subTest(reference=reference):
                    with self.assertRaises(CHECK_MODULE.EvidenceError):
                        CHECK_MODULE.resolve_repository_ref(
                            repository,
                            reference,
                            "invalid reference",
                        )

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
