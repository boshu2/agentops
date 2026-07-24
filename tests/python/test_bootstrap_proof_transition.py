from __future__ import annotations

import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
RECORDER = ROOT / "scripts" / "bootstrap-proof-transition-v2.py"


def canonical(value: object) -> bytes:
    return json.dumps(
        value,
        ensure_ascii=False,
        separators=(",", ":"),
        sort_keys=True,
    ).encode("utf-8")


def digest(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def write_json(path: Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(canonical(value) + b"\n")


def tree_digest(root: Path) -> str:
    entries = []
    for path in sorted(root.rglob("*")):
        if path.is_file():
            entries.append(
                {
                    "path": path.relative_to(root).as_posix(),
                    "kind": "file",
                    "executable": False,
                    "sha256": digest(path.read_bytes()),
                }
            )
    return digest(canonical(entries))


class BootstrapTransitionTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.repository = Path(self.temporary.name)
        self.active = self.repository / "proof" / "active.json"
        self.prior = self.repository / "proof" / "epoch-0.json"
        self.candidate = self.repository / "proof" / "epoch-1.json"
        self.manifest = self.repository / "proof" / "candidate-manifest.json"
        self.corpus = self.repository / "proof" / "corpus"
        self.verdicts = self.repository / "proof" / "verdicts"
        self.transitions = self.repository / "proof" / "transitions"
        self.corpus.mkdir(parents=True)
        (self.corpus / "case.json").write_text('{"case":"exact bytes"}\n')

        write_json(
            self.prior,
            {
                "schema_version": "proof-contract.v1",
                "epoch": 0,
                "components": [],
                "qualification_corpus": {
                    "algorithm": "sha256-tree-v1",
                    "ref": "proof/corpus",
                    "digest": tree_digest(self.corpus),
                },
                "qualification_subject_manifest_digest": None,
                "transition_recorder": {
                    "ref": "scripts/bootstrap-proof-transition.py",
                    "digest": digest(RECORDER.read_bytes()),
                    "mode": "0644",
                },
                "known_gaps": ["bootstrap test fixture"],
            },
        )
        self.prior_digest = digest(self.prior.read_bytes())
        write_json(
            self.active,
            {
                "schema_version": "proof-contract-active.v1",
                "epoch": 0,
                "contract_ref": "proof/epoch-0.json",
                "contract_digest": self.prior_digest,
                "activation_transition_ref": None,
                "activation_transition_digest": None,
            },
        )

        required_roles = sorted(
            {
                "validator-contract",
                "validator-implementation",
                "verdict-schema",
                "rpi-report-schema",
                "subject-manifest-schema",
            }
        )
        component_root = self.repository / "proof" / "components"
        component_root.mkdir()
        components = []
        entries = []
        declared_roots = []
        for role in required_roles:
            component = component_root / role
            component.write_text(f"{role}\n")
            reference = component.relative_to(self.repository).as_posix()
            component_digest = digest(component.read_bytes())
            components.append(
                {
                    "role": role,
                    "ref": reference,
                    "digest": component_digest,
                    "mode": "0644",
                }
            )
            entries.append(
                {
                    "path": reference,
                    "kind": "file",
                    "executable": False,
                    "digest": component_digest,
                }
            )
            declared_roots.append(reference)
        future_recorder = component_root / "future-transition-recorder.py"
        future_recorder.write_text("# future recorder\n")
        future_recorder_ref = future_recorder.relative_to(self.repository).as_posix()
        future_recorder_digest = digest(future_recorder.read_bytes())
        entries.append(
            {
                "path": future_recorder_ref,
                "kind": "file",
                "executable": False,
                "digest": future_recorder_digest,
            }
        )
        declared_roots.append(future_recorder_ref)

        manifest = {
            "schema_version": "subject-manifest.v1",
            "declared_roots": sorted(declared_roots),
            "exclusions": [],
            "entries": sorted(entries, key=lambda item: item["path"]),
        }
        manifest["canonical_manifest_digest"] = digest(canonical(manifest))
        write_json(self.manifest, manifest)

        write_json(
            self.candidate,
            {
                "schema_version": "proof-contract.v1",
                "epoch": 1,
                "components": components,
                "qualification_corpus": {
                    "algorithm": "sha256-tree-v1",
                    "ref": "proof/corpus",
                    "digest": tree_digest(self.corpus),
                },
                "qualification_subject_manifest_digest": manifest[
                    "canonical_manifest_digest"
                ],
                "transition_recorder": {
                    "ref": future_recorder_ref,
                    "digest": future_recorder_digest,
                    "mode": "0644",
                },
                "known_gaps": [],
            },
        )

        verdict = {
            "schema_version": "verdict.v2",
            "acceptance_digest": "3" * 64,
            "subject_manifest_digest": manifest["canonical_manifest_digest"],
            "author_context_id": "candidate-author",
            "validator_context_id": "fresh-validator",
            "freshness_attestation": {
                "source": "runtime",
                "attester_identity": "test-runtime",
            },
            "verdict": "PASS",
            "criteria": [
                {"id": "t1", "result": "PASS", "evidence_refs": ["fixture"]}
            ],
            "findings": [],
            "evidence_refs": ["fixture"],
            "checked": ["candidate"],
            "not_checked": [],
            "validated_at": "2026-07-24T00:00:00Z",
        }
        verdict["artifact_digest"] = digest(canonical(verdict))
        self.verdict = self.verdicts / f"{verdict['artifact_digest']}.json"
        write_json(self.verdict, verdict)

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def run_recorder(
        self, *, expected: str | None = None
    ) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [
                sys.executable,
                str(RECORDER),
                "--repository",
                str(self.repository),
                "--active-pointer",
                str(self.active),
                "--candidate-descriptor",
                str(self.candidate),
                "--candidate-manifest",
                str(self.manifest),
                "--qualification-corpus",
                str(self.corpus),
                "--qualification-verdict",
                str(self.verdict),
                "--transitions-dir",
                str(self.transitions),
                "--expected-prior-digest",
                expected or self.prior_digest,
                "--validator-id",
                "fresh-validator",
                "--activated-at",
                "2026-07-24T00:00:01Z",
            ],
            text=True,
            capture_output=True,
            check=False,
        )

    def test_activates_once_and_records_exact_transition(self) -> None:
        result = self.run_recorder()
        self.assertEqual(result.returncode, 0, result.stderr)
        output = json.loads(result.stdout)
        active = json.loads(self.active.read_text())
        self.assertEqual(output["result"], "ACTIVATED")
        self.assertEqual(active["epoch"], 1)
        transition = self.repository / output["transition_ref"]
        self.assertEqual(digest(transition.read_bytes()), output["transition_digest"])
        second = self.run_recorder()
        self.assertNotEqual(second.returncode, 0)

    def test_stale_prior_compare_and_swap_is_refused(self) -> None:
        result = self.run_recorder(expected="f" * 64)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("CAS refused", result.stderr)
        self.assertEqual(json.loads(self.active.read_text())["epoch"], 0)

    def test_corpus_mutation_is_refused(self) -> None:
        (self.corpus / "case.json").write_text('{"case":"mutated"}\n')
        result = self.run_recorder()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("corpus digest does not match", result.stderr)
        self.assertEqual(json.loads(self.active.read_text())["epoch"], 0)

    def test_missing_candidate_component_is_refused(self) -> None:
        candidate = json.loads(self.candidate.read_text())
        target = self.repository / candidate["components"][0]["ref"]
        target.unlink()
        result = self.run_recorder()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("candidate component is unavailable", result.stderr)
        self.assertEqual(json.loads(self.active.read_text())["epoch"], 0)

    def test_candidate_component_byte_mutation_is_refused(self) -> None:
        candidate = json.loads(self.candidate.read_text())
        target = self.repository / candidate["components"][0]["ref"]
        target.write_text("mutated component\n")
        result = self.run_recorder()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("candidate component digest changed", result.stderr)
        self.assertEqual(json.loads(self.active.read_text())["epoch"], 0)

    def test_candidate_component_mode_mutation_is_refused(self) -> None:
        candidate = json.loads(self.candidate.read_text())
        target = self.repository / candidate["components"][0]["ref"]
        target.chmod(0o755)
        result = self.run_recorder()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("candidate component mode changed", result.stderr)
        self.assertEqual(json.loads(self.active.read_text())["epoch"], 0)

    def test_component_absent_from_judged_subject_is_refused(self) -> None:
        unbound = self.repository / "proof" / "components" / "unbound"
        unbound.write_text("unbound\n")
        candidate = json.loads(self.candidate.read_text())
        candidate["components"].append(
            {
                "role": "extra-unbound-component",
                "ref": unbound.relative_to(self.repository).as_posix(),
                "digest": digest(unbound.read_bytes()),
                "mode": "0644",
            }
        )
        write_json(self.candidate, candidate)
        result = self.run_recorder()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("absent from judged subject", result.stderr)
        self.assertEqual(json.loads(self.active.read_text())["epoch"], 0)

    def test_missing_future_transition_recorder_is_refused(self) -> None:
        candidate = json.loads(self.candidate.read_text())
        target = self.repository / candidate["transition_recorder"]["ref"]
        target.unlink()
        result = self.run_recorder()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(
            "candidate component is unavailable: transition-recorder",
            result.stderr,
        )
        self.assertEqual(json.loads(self.active.read_text())["epoch"], 0)

    def test_subject_mismatch_is_refused(self) -> None:
        verdict = json.loads(self.verdict.read_text())
        verdict["subject_manifest_digest"] = "a" * 64
        verdict["artifact_digest"] = digest(
            canonical(
                {
                    key: value
                    for key, value in verdict.items()
                    if key != "artifact_digest"
                }
            )
        )
        replacement = self.verdict.with_name(f"{verdict['artifact_digest']}.json")
        write_json(replacement, verdict)
        self.verdict = replacement
        result = self.run_recorder()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("different subject", result.stderr)

    def test_non_pass_or_identity_collision_is_refused(self) -> None:
        verdict = json.loads(self.verdict.read_text())
        verdict["author_context_id"] = verdict["validator_context_id"]
        verdict["artifact_digest"] = digest(
            canonical(
                {
                    key: value
                    for key, value in verdict.items()
                    if key != "artifact_digest"
                }
            )
        )
        replacement = self.verdict.with_name(f"{verdict['artifact_digest']}.json")
        write_json(replacement, verdict)
        self.verdict = replacement
        result = self.run_recorder()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("distinct identities", result.stderr)


if __name__ == "__main__":
    unittest.main()
