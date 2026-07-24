from __future__ import annotations

import json
import hashlib
import importlib.util
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
CHECKER = ROOT / "scripts" / "check-proof-contract.py"
KERNEL_PATH = ROOT / "skills" / "validate" / "scripts" / "kernel_v3.py"
KERNEL_SPEC = importlib.util.spec_from_file_location(
    "test_check_proof_contract_kernel",
    KERNEL_PATH,
)
assert KERNEL_SPEC and KERNEL_SPEC.loader
KERNEL = importlib.util.module_from_spec(KERNEL_SPEC)
KERNEL_SPEC.loader.exec_module(KERNEL)
FROZEN_PATHS = [
    "docs/contracts/proof-contracts/active.json",
    "docs/contracts/proof-contracts/epoch-0/descriptor.json",
    "docs/contracts/proof-contracts/epoch-0b/descriptor.json",
    "docs/contracts/proof-contracts/bootstrap-root-replacements/2026-07-24-epoch-0-to-0b.json",
    "docs/evidence/proof-epochs/epoch-0/verdicts/bf865e3233c1e19e6346d37403db775e9fb0fa6b252d14af88e4c9aaa081d804.json",
    "scripts/bootstrap-proof-transition.py",
    "scripts/bootstrap-proof-transition-v2.py",
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

    def write_object(self, path: Path, value: dict) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(
            json.dumps(value, ensure_ascii=False, sort_keys=True, indent=2) + "\n",
            encoding="utf-8",
        )

    def build_epoch_one_fixture(self, repository: Path) -> dict[str, Path]:
        for relative in (
            "scripts/bootstrap-proof-transition.py",
            "skills/validate/scripts/kernel_v3.py",
        ):
            target = repository / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(ROOT / relative, target)

        def bound_file(relative: str, payload: str) -> dict:
            path = repository / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(payload, encoding="utf-8")
            return {
                "ref": relative,
                "digest": hashlib.sha256(path.read_bytes()).hexdigest(),
                "mode": f"{path.stat().st_mode & 0o777:04o}",
            }

        recorder = bound_file("proof/transition-recorder.py", "recorder = 1\n")
        corpus_file = repository / "proof/corpus/case.json"
        corpus_file.parent.mkdir(parents=True, exist_ok=True)
        corpus_file.write_text("{}\n", encoding="utf-8")
        corpus = {
            "algorithm": "sha256-tree-v1",
            "ref": "proof/corpus",
            "digest": KERNEL.tree_digest(corpus_file.parent),
        }

        prior_components = []
        for role in sorted(KERNEL.BASE_PROOF_COMPONENT_ROLES):
            binding = bound_file(
                f"proof/prior/{role}.txt",
                f"{role}\n",
            )
            prior_components.append({"role": role, **binding})
        prior_descriptor = {
            "schema_version": "proof-contract.v1",
            "epoch": 0,
            "components": prior_components,
            "qualification_corpus": corpus,
            "qualification_subject_manifest_digest": None,
            "transition_recorder": recorder,
            "known_gaps": [],
        }
        prior_path = repository / "docs/contracts/proof-contracts/epoch-0/descriptor.json"
        self.write_object(prior_path, prior_descriptor)

        candidate_components = []
        for role in sorted(KERNEL.EPOCH_ONE_PROOF_COMPONENT_ROLES):
            if role == "validator-implementation":
                path = repository / "skills/validate/scripts/kernel_v3.py"
                binding = {
                    "ref": "skills/validate/scripts/kernel_v3.py",
                    "digest": hashlib.sha256(path.read_bytes()).hexdigest(),
                    "mode": f"{path.stat().st_mode & 0o777:04o}",
                }
            else:
                binding = bound_file(
                    f"proof/candidate/{role}.txt",
                    f"{role}\n",
                )
            candidate_components.append({"role": role, **binding})
        subject_entries = []
        for binding in [*candidate_components, recorder]:
            path = repository / binding["ref"]
            subject_entries.append(
                {
                    "path": binding["ref"],
                    "kind": "file",
                    "executable": bool(path.stat().st_mode & 0o111),
                    "digest": binding["digest"],
                }
            )
        subject = {
            "schema_version": "subject-manifest.v1",
            "declared_roots": ["proof/candidate"],
            "exclusions": [],
            "entries": sorted(subject_entries, key=lambda item: item["path"]),
        }
        subject["canonical_manifest_digest"] = KERNEL.digest_value(subject)
        subject_path = repository / "proof/qualification-subject.json"
        self.write_object(subject_path, subject)

        candidate_descriptor = {
            "schema_version": "proof-contract.v1",
            "epoch": 1,
            "components": candidate_components,
            "qualification_corpus": corpus,
            "qualification_subject_manifest_digest": subject[
                "canonical_manifest_digest"
            ],
            "transition_recorder": recorder,
            "known_gaps": [],
        }
        candidate_path = (
            repository / "docs/contracts/proof-contracts/epoch-1/descriptor.json"
        )
        self.write_object(candidate_path, candidate_descriptor)

        verdict = KERNEL.finalize_artifact(
            {
                "schema_version": "verdict.v2",
                "acceptance_digest": "a" * 64,
                "subject_manifest_digest": subject[
                    "canonical_manifest_digest"
                ],
                "author_context_id": "author:one",
                "validator_context_id": "validator:one",
                "freshness_attestation": {
                    "source": "runtime",
                    "attester_identity": "runtime:one",
                },
                "verdict": "PASS",
                "criteria": [
                    {
                        "id": "T1",
                        "result": "PASS",
                        "evidence_refs": ["proof/check.txt"],
                        "reason": "proved",
                    }
                ],
                "findings": [],
                "evidence_refs": ["proof/check.txt"],
                "checked": ["epoch-one binding"],
                "not_checked": [],
                "validated_at": "2026-07-24T12:00:00Z",
            }
        )
        verdict_path = (
            repository
            / ".agents/ao/verdicts"
            / f"{verdict['artifact_digest']}.json"
        )
        verdict_path.parent.mkdir(parents=True, exist_ok=True)
        verdict_path.write_bytes(KERNEL.canonical_bytes(verdict) + b"\n")

        transition = {
            "schema_version": "proof-contract-transition.v1",
            "prior": {
                "epoch": 0,
                "contract_ref": prior_path.relative_to(repository).as_posix(),
                "contract_digest": hashlib.sha256(
                    prior_path.read_bytes()
                ).hexdigest(),
                "activation_transition_digest": None,
            },
            "candidate": {
                "epoch": 1,
                "contract_ref": candidate_path.relative_to(repository).as_posix(),
                "contract_digest": hashlib.sha256(
                    candidate_path.read_bytes()
                ).hexdigest(),
                "subject_manifest_ref": subject_path.relative_to(
                    repository
                ).as_posix(),
                "subject_manifest_digest": subject[
                    "canonical_manifest_digest"
                ],
                "qualification_corpus_ref": corpus["ref"],
                "qualification_corpus_digest": corpus["digest"],
            },
            "qualification_verdict": {
                "ref": verdict_path.relative_to(repository).as_posix(),
                "digest": verdict["artifact_digest"],
            },
            "validator_identity": "validator:one",
            "activated_at": "2026-07-24T12:01:00Z",
        }
        transition_payload = KERNEL.canonical_bytes(transition) + b"\n"
        transition_digest = hashlib.sha256(transition_payload).hexdigest()
        transition_path = (
            repository
            / "docs/contracts/proof-contracts/transitions"
            / f"{transition_digest}.json"
        )
        transition_path.parent.mkdir(parents=True, exist_ok=True)
        transition_path.write_bytes(transition_payload)
        active = {
            "schema_version": "proof-contract-active.v1",
            "epoch": 1,
            "contract_ref": candidate_path.relative_to(repository).as_posix(),
            "contract_digest": hashlib.sha256(candidate_path.read_bytes()).hexdigest(),
            "activation_transition_ref": transition_path.relative_to(
                repository
            ).as_posix(),
            "activation_transition_digest": transition_digest,
        }
        active_path = repository / "docs/contracts/proof-contracts/active.json"
        self.write_object(active_path, active)
        return {
            "active": active_path,
            "prior": prior_path,
            "prior_component": repository / prior_components[0]["ref"],
            "corpus": corpus_file,
            "candidate": candidate_path,
            "transition": transition_path,
            "subject": subject_path,
            "verdict": verdict_path,
        }

    def select_epoch_zero(self, repository: Path, paths: dict[str, Path]) -> None:
        prior = paths["prior"]
        self.write_object(
            paths["active"],
            {
                "schema_version": "proof-contract-active.v1",
                "epoch": 0,
                "contract_ref": prior.relative_to(repository).as_posix(),
                "contract_digest": hashlib.sha256(prior.read_bytes()).hexdigest(),
                "activation_transition_ref": None,
                "activation_transition_digest": None,
            },
        )

    def test_repository_epoch_zero_freeze_is_valid(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            repository = Path(temporary)
            paths = self.build_epoch_one_fixture(repository)
            self.select_epoch_zero(repository, paths)
            result = self.run_checker(repository)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(json.loads(result.stdout)["epoch"], 0)

    def test_seeded_component_mutation_is_detected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            repository = Path(temporary)
            paths = self.build_epoch_one_fixture(repository)
            self.select_epoch_zero(repository, paths)
            target = paths["prior_component"]
            target.write_text(target.read_text() + "\n")
            result = self.run_checker(repository)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("frozen component digest changed", result.stderr)

    def test_seeded_corpus_mutation_is_detected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            repository = Path(temporary)
            paths = self.build_epoch_one_fixture(repository)
            self.select_epoch_zero(repository, paths)
            target = paths["corpus"]
            target.write_text(target.read_text() + "\n")
            result = self.run_checker(repository)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("qualification corpus digest changed", result.stderr)

    def test_epoch_one_qualification_verdict_byte_mutation_is_detected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            repository = Path(temporary)
            paths = self.build_epoch_one_fixture(repository)
            verdict = paths["verdict"]
            verdict.write_bytes(verdict.read_bytes() + b"\n")
            result = self.run_checker(repository)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("qualification verdict binding is invalid", result.stderr)

    def test_epoch_one_activation_chain_is_valid(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            repository = Path(temporary)
            self.build_epoch_one_fixture(repository)
            result = self.run_checker(repository)
            self.assertEqual(result.returncode, 0, result.stderr)
            checked = json.loads(result.stdout)
            self.assertEqual(checked["epoch"], 1)
            self.assertTrue(checked["activation_transition_digest"])

    def test_epoch_one_transition_binding_mutation_is_detected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            repository = Path(temporary)
            paths = self.build_epoch_one_fixture(repository)
            transition = json.loads(paths["transition"].read_text())
            transition["candidate"]["subject_manifest_digest"] = "f" * 64
            payload = KERNEL.canonical_bytes(transition) + b"\n"
            digest = hashlib.sha256(payload).hexdigest()
            transition_path = (
                paths["transition"].parent / f"{digest}.json"
            )
            transition_path.write_bytes(payload)
            active = json.loads(paths["active"].read_text())
            active["activation_transition_ref"] = transition_path.relative_to(
                repository
            ).as_posix()
            active["activation_transition_digest"] = digest
            self.write_object(paths["active"], active)
            result = self.run_checker(repository)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(
                "transition does not bind the active descriptor",
                result.stderr,
            )

    def test_epoch_one_rejects_unbound_always_pass_validator_before_import(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            repository = Path(temporary)
            self.build_epoch_one_fixture(repository)
            kernel_path = repository / "skills/validate/scripts/kernel_v3.py"
            kernel_path.write_text(
                "def load_active_proof(_repository):\n"
                "    return {'epoch': 1}\n",
                encoding="utf-8",
            )
            result = self.run_checker(repository)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("frozen component bytes or mode changed", result.stderr)

    def test_checker_fails_closed_beyond_supported_epoch(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            repository = Path(temporary)
            paths = self.build_epoch_one_fixture(repository)
            active = json.loads(paths["active"].read_text())
            active["epoch"] = 2
            self.write_object(paths["active"], active)
            result = self.run_checker(repository)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("unsupported active proof epoch", result.stderr)


if __name__ == "__main__":
    unittest.main()
