from __future__ import annotations

import json
import importlib.util
from pathlib import Path
import shutil
import sys
import tempfile
import unittest
from unittest import mock

import jsonschema

def find_repository() -> Path:
    for candidate in Path(__file__).resolve().parents:
        if (
            (candidate / "schemas" / "verdict.v3.schema.json").is_file()
            and (candidate / "tests" / "fixtures" / "rpi-kernel-v3").is_dir()
        ):
            return candidate
    raise RuntimeError("cannot locate AgentOps repository fixtures")


REPOSITORY = find_repository()
SPEC = importlib.util.spec_from_file_location(
    "kernel_v3",
    Path(__file__).with_name("kernel_v3.py"),
)
assert SPEC and SPEC.loader
kernel = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(kernel)
sys.modules["kernel_v3"] = kernel
RECORDER_SPEC = importlib.util.spec_from_file_location(
    "record_proof_transition",
    Path(__file__).with_name("record_proof_transition.py"),
)
assert RECORDER_SPEC and RECORDER_SPEC.loader
recorder = importlib.util.module_from_spec(RECORDER_SPEC)
RECORDER_SPEC.loader.exec_module(recorder)


class KernelV3Tests(unittest.TestCase):
    def write_json(self, path: Path, value: dict) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(
            json.dumps(value, sort_keys=True, indent=2) + "\n",
            encoding="utf-8",
        )

    def assert_schema(self, repository: Path, name: str, value: dict) -> None:
        schema = json.loads((repository / "schemas" / name).read_text())
        jsonschema.Draft202012Validator(schema).validate(value)

    def prepare_repository(self, raw: str) -> Path:
        repository = Path(raw)
        for reference in kernel.SCHEMAS.values():
            source = REPOSITORY / reference
            destination = repository / reference
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(source, destination)
        contract = repository / "docs/contracts/proof-contracts/epoch-0/descriptor.json"
        contract.parent.mkdir(parents=True, exist_ok=True)
        component = repository / "proof/component.py"
        recorder = repository / "proof/transition.py"
        corpus_file = repository / "proof/corpus/case.json"
        component.parent.mkdir(parents=True, exist_ok=True)
        corpus_file.parent.mkdir(parents=True, exist_ok=True)
        component.write_text("component = 1\n", encoding="utf-8")
        recorder.write_text("recorder = 1\n", encoding="utf-8")
        corpus_file.write_text("{}\n", encoding="utf-8")
        components = []
        for role in sorted(kernel.EPOCH_ONE_PROOF_COMPONENT_ROLES):
            role_path = (
                component
                if role == "validator-implementation"
                else repository / "proof/components" / f"{role}.json"
            )
            role_path.parent.mkdir(parents=True, exist_ok=True)
            if role_path != component:
                role_path.write_text(
                    json.dumps({"role": role}, sort_keys=True) + "\n",
                    encoding="utf-8",
                )
            components.append(
                {
                    "role": role,
                    "ref": role_path.relative_to(repository).as_posix(),
                    "digest": kernel.sha256(role_path.read_bytes()),
                    "mode": "0644",
                }
            )
        descriptor = {
            "schema_version": "proof-contract.v1",
            "epoch": 0,
            "components": components,
            "qualification_corpus": {
                "algorithm": "sha256-tree-v1",
                "ref": corpus_file.parent.relative_to(repository).as_posix(),
                "digest": kernel.tree_digest(corpus_file.parent),
            },
            "qualification_subject_manifest_digest": None,
            "transition_recorder": {
                "ref": recorder.relative_to(repository).as_posix(),
                "digest": kernel.sha256(recorder.read_bytes()),
                "mode": "0644",
            },
            "known_gaps": [],
        }
        self.write_json(contract, descriptor)
        active = {
            "schema_version": "proof-contract-active.v1",
            "epoch": 0,
            "contract_ref": contract.relative_to(repository).as_posix(),
            "contract_digest": kernel.sha256(contract.read_bytes()),
            "activation_transition_ref": None,
            "activation_transition_digest": None,
        }
        self.write_json(repository / kernel.PROOF_ACTIVE_REF, active)
        (repository / "src").mkdir()
        (repository / "src/input.txt").write_text("before\n", encoding="utf-8")
        (repository / "generated").mkdir()
        (repository / "generated/output.txt").write_text("before\n", encoding="utf-8")
        (repository / "unrelated.txt").write_text("before\n", encoding="utf-8")
        return repository

    def freeze_scope(
        self,
        intent_digest: str,
        patterns: list[str],
        *,
        criteria: list[dict] | None = None,
        exclusions: list[dict] | None = None,
    ) -> dict:
        return kernel.freeze_scope_index(
            intent_digest=intent_digest,
            frozen_at="2026-07-24T12:00:00Z",
            criteria=criteria
            or [{"id": "criterion-1", "required": True, "statement": "works"}],
            scope_classes=[{"id": "candidate", "patterns": patterns}],
            declared_exclusions=exclusions or [],
        )

    def build_subject(
        self,
        repository: Path,
        scope_patterns: list[str],
        *,
        partial: bool = False,
        mutate_unrelated: bool = False,
        mutate_active: bool = False,
    ) -> dict:
        intent_payload = b"acceptance: exact bytes\n"
        intent_path, intent_digest, _ = kernel.mint_intent_snapshot(
            intent_payload,
            repository / ".agents/ao/intents",
        )
        roots = (
            [{"id": "partial", "includes": ["src"]}]
            if partial
            else kernel.REPOSITORY_OBSERVATION
        )
        exclusions = [
            ".git",
            ".agents/ao/intents",
            ".agents/ao/verdicts",
            ".agents/ao/reports",
        ]
        before = kernel.build_manifest_v2(repository, roots, exclusions)
        (repository / "src/input.txt").write_text("after\n", encoding="utf-8")
        if not partial:
            (repository / "generated/output.txt").write_text(
                "after\n",
                encoding="utf-8",
            )
        if mutate_unrelated:
            (repository / "unrelated.txt").write_text("after\n", encoding="utf-8")
        if mutate_active:
            active_path = repository / kernel.PROOF_ACTIVE_REF
            active = json.loads(active_path.read_text())
            active_path.write_text(
                json.dumps(active, sort_keys=False, indent=4) + "\n",
                encoding="utf-8",
            )
        final = kernel.build_manifest_v2(repository, roots, exclusions)
        scope = self.freeze_scope(intent_digest, scope_patterns)
        check = kernel.build_check_receipt(
            "check:focused",
            ["python3", "-m", "unittest"],
            0,
            final["canonical_manifest_digest"],
            b"ok\n",
            b"",
            "2026-07-24T12:01:00Z",
        )
        check_ref = ".agents/ao/reports/check.json"
        effect = kernel.derive_effect_receipt(
            before,
            final,
            scope,
            [{"ref": check_ref, "digest": check["artifact_digest"]}],
        )
        values = {
            "intent": intent_path,
            "intent_digest": intent_digest,
            "before": before,
            "final": final,
            "scope": scope,
            "check": check,
            "effect": effect,
        }
        for name in ("before", "final", "scope", "check", "effect"):
            reference = repository / ".agents/ao/reports" / f"{name}.json"
            self.write_json(reference, values[name])
            values[f"{name}_ref"] = reference.relative_to(repository).as_posix()
        return values

    def activate_synthetic_epoch_one(self, repository: Path) -> dict:
        prior_active = json.loads((repository / kernel.PROOF_ACTIVE_REF).read_text())
        prior_contract = json.loads(
            (repository / prior_active["contract_ref"]).read_text()
        )
        subject = {
            "schema_version": "subject-manifest.v1",
            "declared_roots": ["src"],
            "exclusions": [],
            "entries": [],
        }
        subject["canonical_manifest_digest"] = kernel.digest_value(subject)
        subject_ref = "proof/qualification-subject.json"
        self.write_json(repository / subject_ref, subject)

        verdict = {
            "schema_version": "verdict.v2",
            "acceptance_digest": "a" * 64,
            "verdict": "PASS",
            "subject_manifest_digest": subject["canonical_manifest_digest"],
            "author_context_id": "author:epoch-one",
            "validator_context_id": "validator:epoch-zero",
            "freshness_attestation": {
                "source": "runtime",
                "attester_identity": "runtime:epoch-zero"
            },
            "criteria": [
                {
                    "id": "qualifies",
                    "result": "PASS",
                    "evidence_refs": ["check:qualification"],
                }
            ],
            "findings": [],
            "evidence_refs": ["check:qualification"],
            "checked": ["candidate"],
            "not_checked": [],
            "validated_at": "2026-07-24T12:59:00Z",
        }
        verdict["artifact_digest"] = kernel.digest_value(verdict)
        verdict_ref = f"proof/verdicts/{verdict['artifact_digest']}.json"
        self.write_json(repository / verdict_ref, verdict)

        candidate = {
            **prior_contract,
            "epoch": 1,
            "qualification_subject_manifest_digest": subject[
                "canonical_manifest_digest"
            ],
        }
        candidate_ref = "proof/epoch-1.json"
        self.write_json(repository / candidate_ref, candidate)
        candidate_digest = kernel.sha256((repository / candidate_ref).read_bytes())
        transition = {
            "schema_version": "proof-contract-transition.v1",
            "prior": {
                "epoch": 0,
                "contract_ref": prior_active["contract_ref"],
                "contract_digest": prior_active["contract_digest"],
                "activation_transition_digest": None,
            },
            "candidate": {
                "epoch": 1,
                "contract_ref": candidate_ref,
                "contract_digest": candidate_digest,
                "subject_manifest_ref": subject_ref,
                "subject_manifest_digest": subject["canonical_manifest_digest"],
                "qualification_corpus_ref": candidate["qualification_corpus"]["ref"],
                "qualification_corpus_digest": candidate["qualification_corpus"][
                    "digest"
                ],
            },
            "qualification_verdict": {
                "ref": verdict_ref,
                "digest": verdict["artifact_digest"],
            },
            "validator_identity": "validator:epoch-zero",
            "activated_at": "2026-07-24T13:00:00Z",
        }
        transition_payload = (
            json.dumps(
                transition,
                sort_keys=True,
                separators=(",", ":"),
                ensure_ascii=False,
            ).encode("utf-8")
            + b"\n"
        )
        transition_digest = kernel.sha256(transition_payload)
        transition_ref = f"proof/transitions/{transition_digest}.json"
        kernel.atomic_write_bytes(repository / transition_ref, transition_payload)
        self.write_json(
            repository / kernel.PROOF_ACTIVE_REF,
            {
                "schema_version": "proof-contract-active.v1",
                "epoch": 1,
                "contract_ref": candidate_ref,
                "contract_digest": candidate_digest,
                "activation_transition_ref": transition_ref,
                "activation_transition_digest": transition_digest,
            },
        )
        return {
            "subject_ref": subject_ref,
            "verdict_ref": verdict_ref,
            "transition_ref": transition_ref,
        }

    def prepare_epoch_two_transition(self, repository: Path) -> dict:
        self.activate_synthetic_epoch_one(repository)
        active = kernel.load_active_proof(repository)
        general_recorder = repository / "proof/general-transition.py"
        shutil.copyfile(
            Path(__file__).with_name("record_proof_transition.py"),
            general_recorder,
        )
        general_recorder.chmod(0o755)
        prior_contract = json.loads((repository / active["contract_ref"]).read_text())
        components = [dict(item) for item in prior_contract["components"]]
        candidate_only = repository / "proof/candidate-only.json"
        candidate_only.write_text(
            '{"candidate":"epoch-two"}\n',
            encoding="utf-8",
        )
        components.append(
            {
                "role": "candidate-extension",
                "ref": candidate_only.relative_to(repository).as_posix(),
                "digest": kernel.sha256(candidate_only.read_bytes()),
                "mode": "0644",
            }
        )
        entries = []
        for binding in components + [
            {
                "ref": general_recorder.relative_to(repository).as_posix(),
                "digest": kernel.sha256(general_recorder.read_bytes()),
                "mode": "0755",
            }
        ]:
            entries.append(
                {
                    "path": binding["ref"],
                    "kind": "file",
                    "executable": bool(int(binding["mode"], 8) & 0o111),
                    "digest": binding["digest"],
                }
            )
        entries.sort(key=lambda item: item["path"])
        subject = {
            "schema_version": "subject-manifest.v1",
            "declared_roots": ["proof"],
            "exclusions": [],
            "entries": entries,
        }
        subject["canonical_manifest_digest"] = kernel.digest_value(subject)
        subject_ref = "proof/epoch-2-subject.json"
        self.write_json(repository / subject_ref, subject)

        candidate = {
            **prior_contract,
            "epoch": 2,
            "components": components,
            "qualification_subject_manifest_digest": subject[
                "canonical_manifest_digest"
            ],
            "transition_recorder": {
                "ref": general_recorder.relative_to(repository).as_posix(),
                "digest": kernel.sha256(general_recorder.read_bytes()),
                "mode": "0755",
            },
        }
        candidate_ref = "proof/epoch-2.json"
        self.write_json(repository / candidate_ref, candidate)

        evidence_digest = "e" * 64
        verdict = kernel.finalize_artifact(
            {
                "schema_version": "verdict.v3",
                "invocation_id": "invocation:epoch-two",
                "judgment_id": "judgment:epoch-one",
                "intent_ref": ".agents/ao/intents/" + "a" * 64 + ".intent",
                "intent_digest": "a" * 64,
                "proof_identity": active,
                "schema_digests": {
                    key: "b" * 64 for key in kernel.SCHEMAS
                },
                "before_manifest_ref": "proof/before.json",
                "before_manifest_digest": "c" * 64,
                "final_manifest_ref": subject_ref,
                "final_manifest_digest": subject["canonical_manifest_digest"],
                "scope_index_ref": "proof/scope.json",
                "scope_index_digest": "d" * 64,
                "effect_receipt_ref": "proof/effect.json",
                "effect_receipt_digest": evidence_digest,
                "author_context_id": "author:epoch-two",
                "validator_context_id": "validator:epoch-one",
                "freshness_attestation": {
                    "source": "runtime",
                    "attester_identity": "runtime:epoch-one",
                },
                "verdict": "PASS",
                "criteria": [
                    {
                        "id": "qualifies",
                        "result": "PASS",
                        "evidence_receipt_digests": [evidence_digest],
                        "reason": "candidate qualifies",
                    }
                ],
                "findings": [],
                "checked": ["candidate"],
                "not_checked": [],
                "validated_at": "2026-07-24T14:00:00Z",
            }
        )
        verdict_ref = (
            f".agents/ao/verdicts/{verdict['artifact_digest']}.json"
        )
        self.write_json(repository / verdict_ref, verdict)
        return {
            "active_ref": kernel.PROOF_ACTIVE_REF,
            "candidate_ref": candidate_ref,
            "subject_ref": subject_ref,
            "verdict_ref": verdict_ref,
            "corpus_ref": candidate["qualification_corpus"]["ref"],
            "transitions_ref": "proof/transitions-next",
            "component_ref": candidate_only.relative_to(repository).as_posix(),
            "recorder_ref": general_recorder.relative_to(repository).as_posix(),
        }

    def run_epoch_two_transition(self, repository: Path, refs: dict):
        return recorder.record_transition(
            repository=repository,
            active_pointer=repository / refs["active_ref"],
            candidate_descriptor=repository / refs["candidate_ref"],
            qualification_subject_manifest=repository / refs["subject_ref"],
            qualification_verdict=repository / refs["verdict_ref"],
            qualification_corpus=repository / refs["corpus_ref"],
            transitions_dir=repository / refs["transitions_ref"],
            validator_identity="validator:epoch-one",
            activated_at="2026-07-24T14:01:00Z",
        )

    def pass_draft(self, evidence_digest: str) -> dict:
        return {
            "verdict": "PASS",
            "criteria": [
                {
                    "id": "criterion-1",
                    "result": "PASS",
                    "evidence_receipt_digests": [evidence_digest],
                    "reason": "focused check passed",
                }
            ],
            "findings": [],
            "checked": ["criterion-1"],
            "not_checked": [],
            "validated_at": "2026-07-24T12:02:00Z",
        }

    def store(
        self,
        repository: Path,
        values: dict,
        *,
        judgment_id: str = "judgment:one",
    ):
        return kernel.store_verdict_v3(
            self.pass_draft(values["check"]["artifact_digest"]),
            repository=repository,
            destination=repository / ".agents/ao/verdicts",
            invocation_id="invocation:one",
            judgment_id=judgment_id,
            intent_ref=values["intent"].relative_to(repository).as_posix(),
            expected_intent_digest=values["intent_digest"],
            before_manifest_ref=values["before_ref"],
            final_manifest_ref=values["final_ref"],
            scope_index_ref=values["scope_ref"],
            effect_receipt_ref=values["effect_ref"],
            author_context_id="author:one",
            validator_context_id="validator:one",
            freshness_source="runtime",
            freshness_attester_id="runtime:one",
        )

    def test_exact_bytes_survive_and_living_source_is_never_rederived(self):
        with tempfile.TemporaryDirectory() as raw:
            destination = Path(raw) / "intents"
            original = Path(raw) / "living.txt"
            exact = "café\n".encode("utf-8")
            different = "cafe\u0301 \n".encode("utf-8")
            original.write_bytes(exact)
            snapshot, digest, _ = kernel.mint_intent_snapshot(
                original.read_bytes(),
                destination,
            )
            original.write_bytes(different)
            self.assertEqual(kernel.consume_intent_snapshot(snapshot, digest), exact)
            self.assertNotEqual(kernel.sha256(original.read_bytes()), digest)
            with self.assertRaisesRegex(kernel.TerminalValidation, "digest mismatch"):
                kernel.consume_intent_snapshot(snapshot, kernel.sha256(different))

    def test_repository_observation_includes_generated_companions_and_deletions(self):
        with tempfile.TemporaryDirectory() as raw:
            repository = self.prepare_repository(raw)
            values = self.build_subject(repository, ["src", "generated"])
            effect = values["effect"]
            self.assertEqual(effect["coverage"], "COMPLETE")
            self.assertEqual(
                effect["actual_changed_paths"],
                ["generated/output.txt", "src/input.txt"],
            )
            (repository / "generated/output.txt").unlink()
            deleted = kernel.build_manifest_v2(
                repository,
                kernel.REPOSITORY_OBSERVATION,
                [
                    ".git",
                    ".agents/ao/intents",
                    ".agents/ao/verdicts",
                    ".agents/ao/reports",
                ],
            )
            deletion_effect = kernel.derive_effect_receipt(
                values["final"],
                deleted,
                values["scope"],
                [],
            )
            self.assertEqual(
                deletion_effect["changes"][0]["change_kind"],
                "DELETED",
            )

    def test_mutation_outside_write_scope_is_observed_and_forces_fail(self):
        with tempfile.TemporaryDirectory() as raw:
            repository = self.prepare_repository(raw)
            values = self.build_subject(
                repository,
                ["src", "generated"],
                mutate_unrelated=True,
            )
            self.assertEqual(values["effect"]["coverage"], "COMPLETE")
            self.assertEqual(values["effect"]["undeclared_paths"], ["unrelated.txt"])
            artifact, _path, _ = self.store(repository, values)
            self.assertEqual(artifact["verdict"], "FAIL")
            self.assertEqual(artifact["findings"][-1]["id"], "validate.out-of-scope")

    def test_partial_observation_forces_not_proven(self):
        with tempfile.TemporaryDirectory() as raw:
            repository = self.prepare_repository(raw)
            values = self.build_subject(repository, ["src"], partial=True)
            self.assertEqual(values["effect"]["coverage"], "INCOMPLETE")
            artifact, _path, _ = self.store(repository, values)
            self.assertEqual(artifact["verdict"], "NOT_PROVEN")
            self.assertEqual(
                artifact["findings"][-1]["id"],
                "validate.incomplete-coverage",
            )

    def test_required_criterion_cannot_be_absorbed_by_exclusion(self):
        with self.assertRaisesRegex(kernel.ContractError, "cannot absorb required"):
            self.freeze_scope(
                "a" * 64,
                ["src"],
                exclusions=[
                    {
                        "id": "excluded",
                        "criterion_ids": ["criterion-1"],
                        "reason": "too hard",
                    }
                ],
            )

    def test_duplicate_criterion_ids_are_rejected(self):
        with self.assertRaisesRegex(kernel.ContractError, "criterion IDs must be unique"):
            self.freeze_scope(
                "a" * 64,
                ["src"],
                criteria=[
                    {"id": "same", "required": True, "statement": "one"},
                    {"id": "same", "required": True, "statement": "two"},
                ],
            )

    def test_candidate_mutation_after_freeze_is_terminal(self):
        with tempfile.TemporaryDirectory() as raw:
            repository = self.prepare_repository(raw)
            values = self.build_subject(repository, ["src", "generated"])
            (repository / "src/input.txt").write_text(
                "mutated after freeze\n",
                encoding="utf-8",
            )
            with self.assertRaisesRegex(
                kernel.TerminalValidation,
                "candidate mutated after freeze",
            ):
                self.store(repository, values)

    def test_candidate_mutation_between_validation_recomputations_is_terminal(self):
        with tempfile.TemporaryDirectory() as raw:
            repository = self.prepare_repository(raw)
            values = self.build_subject(repository, ["src", "generated"])
            original = kernel.load_active_proof

            def mutate_between_checks(root: Path):
                proof = original(root)
                (repository / "src/input.txt").write_text(
                    "mutated during validation\n",
                    encoding="utf-8",
                )
                return proof

            with mock.patch.object(
                kernel,
                "load_active_proof",
                side_effect=mutate_between_checks,
            ), self.assertRaisesRegex(
                kernel.TerminalValidation,
                "candidate mutated after freeze",
            ):
                self.store(repository, values)

    def test_duplicate_intent_final_subject_judgment_is_rejected(self):
        with tempfile.TemporaryDirectory() as raw:
            repository = self.prepare_repository(raw)
            values = self.build_subject(repository, ["src", "generated"])
            self.store(repository, values)
            with self.assertRaisesRegex(
                kernel.TerminalValidation,
                "duplicate unlinked judgment",
            ):
                self.store(repository, values, judgment_id="judgment:two")

    def test_candidate_proof_contract_cannot_activate_itself(self):
        with tempfile.TemporaryDirectory() as raw:
            repository = self.prepare_repository(raw)
            values = self.build_subject(
                repository,
                ["src", "generated", kernel.PROOF_ACTIVE_REF],
                mutate_active=True,
            )
            with self.assertRaisesRegex(
                kernel.TerminalValidation,
                "cannot activate itself",
            ):
                self.store(repository, values)

    def test_forged_complete_empty_effect_cannot_hide_real_changes(self):
        with tempfile.TemporaryDirectory() as raw:
            repository = self.prepare_repository(raw)
            values = self.build_subject(
                repository,
                ["src", "generated"],
                mutate_unrelated=True,
            )
            forged = kernel.finalize_artifact(
                {
                    **kernel.artifact_identity(values["effect"]),
                    "coverage": "COMPLETE",
                    "changes": [],
                    "actual_changed_paths": [],
                    "undeclared_paths": [],
                }
            )
            self.write_json(repository / values["effect_ref"], forged)
            with self.assertRaisesRegex(
                kernel.TerminalValidation,
                "runtime-derived",
            ):
                self.store(repository, values)

    def test_complete_requires_exact_runtime_exclusion_set(self):
        with tempfile.TemporaryDirectory() as raw:
            repository = self.prepare_repository(raw)
            intent_path, intent_digest, _ = kernel.mint_intent_snapshot(
                b"intent\n",
                repository / ".agents/ao/intents",
            )
            before = kernel.build_manifest_v2(
                repository,
                kernel.REPOSITORY_OBSERVATION,
                [".agents/ao/intents"],
            )
            (repository / "src/input.txt").write_text("after\n", encoding="utf-8")
            final = kernel.build_manifest_v2(
                repository,
                kernel.REPOSITORY_OBSERVATION,
                [".agents/ao/intents"],
            )
            scope = self.freeze_scope(intent_digest, ["src"])
            effect = kernel.derive_effect_receipt(before, final, scope, [])
            self.assertEqual(effect["coverage"], "INCOMPLETE")
            self.assertTrue(intent_path.exists())

    def test_unreadable_walk_error_cannot_be_complete(self):
        error = PermissionError("denied")

        def broken_walk(*_args, **kwargs):
            kwargs["onerror"](error)
            return iter(())

        with tempfile.TemporaryDirectory() as raw, mock.patch.object(
            kernel.os,
            "walk",
            side_effect=broken_walk,
        ):
            with self.assertRaisesRegex(kernel.ContractError, "cannot completely observe"):
                kernel.build_manifest_v2(
                    Path(raw),
                    kernel.REPOSITORY_OBSERVATION,
                    kernel.COMPLETE_RUNTIME_EXCLUSIONS,
                )

    def test_named_json_output_is_atomic_and_canonical(self):
        with tempfile.TemporaryDirectory() as raw:
            target = Path(raw) / "nested/artifact.json"
            kernel.atomic_write_json(target, {"unicode": "café", "value": 1})
            self.assertEqual(
                target.read_bytes(),
                '{\n  "unicode": "café",\n  "value": 1\n}\n'.encode("utf-8"),
            )
            self.assertEqual(
                [path for path in target.parent.iterdir() if path != target],
                [],
            )

    def test_effect_reader_rejects_noncanonical_lists(self):
        with tempfile.TemporaryDirectory() as raw:
            repository = self.prepare_repository(raw)
            values = self.build_subject(
                repository,
                ["src"],
                mutate_unrelated=True,
            )
            malformed = kernel.finalize_artifact(
                {
                    **kernel.artifact_identity(values["effect"]),
                    "undeclared_paths": ["unrelated.txt", "unrelated.txt"],
                }
            )
            with self.assertRaisesRegex(kernel.ContractError, "undeclared_paths"):
                kernel.validate_effect_receipt(malformed)

    def test_pass_requires_nonempty_checked_evidence(self):
        with tempfile.TemporaryDirectory() as raw:
            repository = self.prepare_repository(raw)
            values = self.build_subject(repository, ["src", "generated"])
            draft = self.pass_draft(values["check"]["artifact_digest"])
            draft["checked"] = []
            with self.assertRaisesRegex(kernel.ContractError, "nonempty checked"):
                kernel.store_verdict_v3(
                    draft,
                    repository=repository,
                    destination=repository / ".agents/ao/verdicts",
                    invocation_id="invocation:one",
                    judgment_id="judgment:one",
                    intent_ref=values["intent"].relative_to(repository).as_posix(),
                    expected_intent_digest=values["intent_digest"],
                    before_manifest_ref=values["before_ref"],
                    final_manifest_ref=values["final_ref"],
                    scope_index_ref=values["scope_ref"],
                    effect_receipt_ref=values["effect_ref"],
                    author_context_id="author:one",
                    validator_context_id="validator:one",
                    freshness_source="runtime",
                    freshness_attester_id="runtime:one",
                )

    def test_hostile_repository_refs_are_rejected_lexically(self):
        hostile = (
            "",
            "/absolute",
            "//server/share",
            "C:/escape",
            "C:\\escape",
            "a\\b",
            "a//b",
            "a/./b",
            "a/../b",
            "./a",
            "a/",
            "a/\u0001b",
        )
        for reference in hostile:
            with self.subTest(reference=reference), self.assertRaises(
                kernel.ContractError
            ):
                kernel.normalize_rel(reference)
        self.assertEqual(kernel.normalize_rel("ordinary/path.json"), "ordinary/path.json")

    def test_active_proof_fails_on_transitive_component_corpus_and_mode_mutation(self):
        mutations = ("component", "component-mode", "corpus", "recorder")
        for mutation in mutations:
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as raw:
                repository = self.prepare_repository(raw)
                if mutation == "component":
                    (repository / "proof/component.py").write_text(
                        "component = 2\n",
                        encoding="utf-8",
                    )
                elif mutation == "component-mode":
                    (repository / "proof/component.py").chmod(0o755)
                elif mutation == "corpus":
                    (repository / "proof/corpus/case.json").write_text(
                        '{"changed":true}\n',
                        encoding="utf-8",
                    )
                else:
                    (repository / "proof/transition.py").write_text(
                        "recorder = 2\n",
                        encoding="utf-8",
                    )
                with self.assertRaisesRegex(
                    kernel.TerminalValidation,
                    "active proof",
                ):
                    kernel.load_active_proof(repository)

    def test_active_epoch_transition_binds_subject_verdict_and_transition_bytes(self):
        for mutation in ("none", "subject", "verdict", "transition"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as raw:
                repository = self.prepare_repository(raw)
                refs = self.activate_synthetic_epoch_one(repository)
                if mutation == "subject":
                    (repository / refs["subject_ref"]).write_text(
                        '{"changed":true}\n',
                        encoding="utf-8",
                    )
                elif mutation == "verdict":
                    (repository / refs["verdict_ref"]).write_text(
                        '{"changed":true}\n',
                        encoding="utf-8",
                    )
                elif mutation == "transition":
                    (repository / refs["transition_ref"]).write_text(
                        '{"changed":true}\n',
                        encoding="utf-8",
                    )
                if mutation == "none":
                    self.assertEqual(kernel.load_active_proof(repository)["epoch"], 1)
                else:
                    with self.assertRaises(
                        (kernel.ContractError, kernel.TerminalValidation)
                    ):
                        kernel.load_active_proof(repository)

    def test_general_recorder_advances_epoch_one_to_two_content_addressed(self):
        with tempfile.TemporaryDirectory() as raw:
            repository = self.prepare_repository(raw)
            refs = self.prepare_epoch_two_transition(repository)
            result = self.run_epoch_two_transition(repository, refs)
            self.assertEqual(result["result"], "ACTIVATED")
            self.assertEqual(result["epoch"], 2)
            transition_path = repository / result["transition_ref"]
            self.assertEqual(
                transition_path.name,
                f"{result['transition_digest']}.json",
            )
            active = json.loads(
                (repository / kernel.PROOF_ACTIVE_REF).read_text()
            )
            self.assertEqual(active["epoch"], 2)
            self.assertEqual(
                kernel.load_active_proof(repository)["contract_digest"],
                result["contract_digest"],
            )

    def test_general_recorder_rejects_wrong_prior_identity_and_v2(self):
        for mutation in ("wrong-prior", "legacy-v2"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as raw:
                repository = self.prepare_repository(raw)
                refs = self.prepare_epoch_two_transition(repository)
                verdict_path = repository / refs["verdict_ref"]
                verdict = json.loads(verdict_path.read_text())
                if mutation == "wrong-prior":
                    verdict["proof_identity"]["contract_digest"] = "f" * 64
                    verdict = kernel.finalize_artifact(verdict)
                else:
                    verdict = {
                        "schema_version": "verdict.v2",
                        "artifact_digest": "f" * 64,
                    }
                replacement = verdict_path.parent / (
                    f"{verdict.get('artifact_digest', 'f' * 64)}.json"
                )
                self.write_json(replacement, verdict)
                refs["verdict_ref"] = replacement.relative_to(repository).as_posix()
                with self.assertRaisesRegex(
                    (kernel.ContractError, kernel.TerminalValidation),
                    "prior proof|requires a verdict.v3",
                ):
                    self.run_epoch_two_transition(repository, refs)

    def test_general_recorder_rejects_component_absent_or_mutated(self):
        for mutation in ("absent", "mutated"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as raw:
                repository = self.prepare_repository(raw)
                refs = self.prepare_epoch_two_transition(repository)
                if mutation == "mutated":
                    (repository / refs["component_ref"]).write_text(
                        "mutated = True\n",
                        encoding="utf-8",
                    )
                else:
                    subject_path = repository / refs["subject_ref"]
                    subject = json.loads(subject_path.read_text())
                    subject["entries"] = [
                        entry
                        for entry in subject["entries"]
                        if entry["path"] != refs["component_ref"]
                    ]
                    subject["canonical_manifest_digest"] = kernel.digest_value(
                        {
                            key: value
                            for key, value in subject.items()
                            if key != "canonical_manifest_digest"
                        }
                    )
                    self.write_json(subject_path, subject)
                    candidate_path = repository / refs["candidate_ref"]
                    candidate = json.loads(candidate_path.read_text())
                    candidate["qualification_subject_manifest_digest"] = subject[
                        "canonical_manifest_digest"
                    ]
                    self.write_json(candidate_path, candidate)
                    verdict_path = repository / refs["verdict_ref"]
                    verdict = json.loads(verdict_path.read_text())
                    verdict["final_manifest_digest"] = subject[
                        "canonical_manifest_digest"
                    ]
                    verdict = kernel.finalize_artifact(verdict)
                    replacement = verdict_path.parent / (
                        f"{verdict['artifact_digest']}.json"
                    )
                    self.write_json(replacement, verdict)
                    refs["verdict_ref"] = replacement.relative_to(
                        repository
                    ).as_posix()
                with self.assertRaisesRegex(
                    kernel.TerminalValidation,
                    "candidate proof",
                ):
                    self.run_epoch_two_transition(repository, refs)

    def test_general_recorder_cas_refuses_races_and_keeps_active_pointer(self):
        for mutation in ("descriptor", "component", "active"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as raw:
                repository = self.prepare_repository(raw)
                refs = self.prepare_epoch_two_transition(repository)
                active_path = repository / kernel.PROOF_ACTIVE_REF
                original_active = active_path.read_bytes()
                original_write = recorder.kernel.atomic_write_bytes
                injected = False

                def inject_after_transition(target: Path, payload: bytes):
                    nonlocal injected
                    original_write(target, payload)
                    if (
                        not injected
                        and target.parent.resolve()
                        == (repository / refs["transitions_ref"]).resolve()
                    ):
                        injected = True
                        if mutation == "descriptor":
                            (repository / refs["candidate_ref"]).write_text(
                                '{"mutated":true}\n',
                                encoding="utf-8",
                            )
                        elif mutation == "component":
                            (repository / refs["component_ref"]).write_text(
                                "mutated = True\n",
                                encoding="utf-8",
                            )
                        else:
                            active_path.write_text(
                                '{"mutated":true}\n',
                                encoding="utf-8",
                            )

                with mock.patch.object(
                    recorder.kernel,
                    "atomic_write_bytes",
                    side_effect=inject_after_transition,
                ), self.assertRaises(
                    (kernel.ContractError, kernel.TerminalValidation)
                ):
                    self.run_epoch_two_transition(repository, refs)
                if mutation != "active":
                    self.assertEqual(active_path.read_bytes(), original_active)

    def test_artifacts_have_strict_schema_readers(self):
        with tempfile.TemporaryDirectory() as raw:
            repository = self.prepare_repository(raw)
            values = self.build_subject(repository, ["src", "generated"])
            artifact, _path, _ = self.store(repository, values)
            self.assert_schema(
                repository,
                "subject-manifest.v2.schema.json",
                values["final"],
            )
            self.assert_schema(
                repository,
                "scope-index.v1.schema.json",
                values["scope"],
            )
            self.assert_schema(
                repository,
                "check-receipt.v1.schema.json",
                values["check"],
            )
            self.assert_schema(
                repository,
                "effect-receipt.v1.schema.json",
                values["effect"],
            )
            self.assert_schema(repository, "verdict.v3.schema.json", artifact)
            kernel.validate_verdict_v3(artifact, scope_index=values["scope"])
            report = kernel.build_rpi_report_v2(
                invocation_id=artifact["invocation_id"],
                correlation={"goal_id": "goal:one"},
                status=artifact["verdict"],
                intent_ref=artifact["intent_ref"],
                intent_digest=artifact["intent_digest"],
                proof_identity=artifact["proof_identity"],
                before_manifest_digest=artifact["before_manifest_digest"],
                final_manifest_digest=artifact["final_manifest_digest"],
                effect_receipt_digest=artifact["effect_receipt_digest"],
                verdict_ref=artifact["final_manifest_ref"],
                verdict_digest=artifact["artifact_digest"],
                checked=artifact["checked"],
                not_checked=artifact["not_checked"],
            )
            self.assert_schema(repository, "rpi-report.v2.schema.json", report)
            kernel.validate_rpi_report_v2(report)


if __name__ == "__main__":
    unittest.main()
