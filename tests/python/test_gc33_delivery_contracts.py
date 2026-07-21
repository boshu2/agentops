from __future__ import annotations

import json
from copy import deepcopy
from pathlib import Path
import unittest

from jsonschema import Draft202012Validator, FormatChecker


ROOT = Path(__file__).resolve().parents[2]
SCHEMAS = ROOT / "packs" / "agentops-factory" / "assets" / "schemas"
DIGEST = "a" * 64


class GC33DeliveryContractTest(unittest.TestCase):
    def load(self, name: str) -> dict[str, object]:
        return json.loads((SCHEMAS / name).read_text(encoding="utf-8"))

    def validator(self, name: str) -> Draft202012Validator:
        schema = self.load(name)
        Draft202012Validator.check_schema(schema)
        return Draft202012Validator(schema, format_checker=FormatChecker())

    def prepared(self) -> dict[str, object]:
        return {"schema_version": "handoff-prepared.v1", "handoff_id": DIGEST, "semantic_bead_id": "semantic", "semantic_terminal_ref": "beads:semantic#gc33.terminal", "admission_certificate_ref": f"certificate:sha256:{DIGEST}", "admission_certificate_digest": "b" * 64, "expected_delivery_bead_id": "delivery", "expected_external_ref": f"handoff:{DIGEST}:epoch:1", "epoch": 1, "mode": "auto", "state": "queued", "deadline": "2026-07-22T00:00:00Z", "prepared_at": "2026-07-21T00:00:00Z"}

    def delivery(self, publication: str = "non_routable") -> dict[str, object]:
        prepared = self.prepared()
        return {"schema_version": "delivery.v1", "kind": "delivery", "handoff_id": DIGEST, "semantic_bead_id": "semantic", "semantic_terminal_ref": "beads:semantic#gc33.terminal", "admission_certificate_digest": "b" * 64, "delivery_bead_id": "delivery", "external_ref": prepared["expected_external_ref"], "epoch": 1, "predecessor_receipt_digest": None, "mode": "auto", "state": "queued", "publication": publication, "deadline": "2026-07-22T00:00:00Z", "effect_gate": None, "successor_bead_id": None}

    def committed(self) -> dict[str, object]:
        return {"schema_version": "handoff-committed.v1", "handoff_id": DIGEST, "prepared_digest": "c" * 64, "semantic_bead_id": "semantic", "semantic_terminal_verdict": "PASS", "semantic_terminal_ref": "beads:semantic#gc33.terminal", "admission_certificate_digest": "b" * 64, "delivery_bead_id": "delivery", "expected_external_ref": f"handoff:{DIGEST}:epoch:1", "epoch": 1, "delivery_payload_ref": "delivery.published.json", "delivery_payload_digest": "d" * 64, "mode": "auto", "state": "queued", "deadline": "2026-07-22T00:00:00Z", "committed_at": "2026-07-21T00:00:03Z"}

    def fallback(self, *, allowed: bool = False, used: bool = False, reason: object = None) -> dict[str, object]:
        return {"allowed": allowed, "used": used, "reason": reason}

    def test_every_33_schema_is_a_valid_draft_2020_12_schema(self) -> None:
        names = ("program-graph.v2.schema.json", "admission-certificate.v2.schema.json", "handoff-prepared.v1.schema.json", "handoff-committed.v1.schema.json", "delivery.v1.schema.json", "ambiguity-request.v1.schema.json", "epoch-receipt.v1.schema.json", "effect-receipt.v1.schema.json", "factory-role-request.v2.schema.json", "factory-role-response.v2.schema.json")
        for name in names:
            with self.subTest(name=name):
                Draft202012Validator.check_schema(self.load(name))

    def test_actual_marker_artifacts_are_schema_conformant(self) -> None:
        cases = (("handoff-prepared.v1.schema.json", self.prepared()), ("delivery.v1.schema.json", self.delivery()), ("delivery.v1.schema.json", self.delivery("published")), ("handoff-committed.v1.schema.json", self.committed()))
        for name, payload in cases:
            with self.subTest(name=name):
                self.assertTrue(self.validator(name).is_valid(payload))

    def test_actual_marker_artifacts_reject_missing_and_conflicting_facts(self) -> None:
        cases = (("handoff-prepared.v1.schema.json", self.prepared(), "expected_external_ref"), ("delivery.v1.schema.json", self.delivery(), "external_ref"), ("handoff-committed.v1.schema.json", self.committed(), "expected_external_ref"), ("handoff-committed.v1.schema.json", self.committed(), "epoch"))
        for name, payload, required in cases:
            with self.subTest(name=name, failure="missing"):
                broken = deepcopy(payload); del broken[required]
                self.assertFalse(self.validator(name).is_valid(broken))
            with self.subTest(name=name, failure="extra"):
                broken = deepcopy(payload); broken["invented"] = True
                self.assertFalse(self.validator(name).is_valid(broken))
        broken = self.committed(); broken["semantic_terminal_verdict"] = "FAIL"
        self.assertFalse(self.validator("handoff-committed.v1.schema.json").is_valid(broken))

    def test_delivery_state_machine_rejects_illegal_combinations(self) -> None:
        validator = self.validator("delivery.v1.schema.json")
        valid = self.delivery("published")
        valid.update({"state": "merge_requested", "effect_gate": {"committed_handoff_digest": "c" * 64, "base_sha": "d" * 40, "expected_remote_head": "e" * 40}})
        self.assertTrue(validator.is_valid(valid))
        cases = {
            "manual_merge_requested": {"mode": "manual"},
            "auto_manual_review": {"state": "manual_review", "effect_gate": {"committed_handoff_digest": "c" * 64, "base_sha": "d" * 40, "expected_remote_head": "e" * 40}},
            "manual_review_nonroutable": {"state": "manual_review", "mode": "manual", "publication": "non_routable", "effect_gate": None},
            "manual_review_no_gate": {"state": "manual_review", "mode": "manual", "effect_gate": None},
            "successor_required_missing": {"state": "successor_required", "successor_bead_id": None},
            "ordinary_successor": {"state": "queued", "successor_bead_id": "unexpected"},
            "nonroutable_gate": {"publication": "non_routable", "effect_gate": {"committed_handoff_digest": "c" * 64, "base_sha": "d" * 40, "expected_remote_head": "e" * 40}},
        }
        for name, update in cases.items():
            with self.subTest(name=name):
                candidate = deepcopy(valid); candidate.update(update)
                self.assertFalse(validator.is_valid(candidate))
        successor = deepcopy(valid); successor.update({"state": "successor_required", "effect_gate": None, "successor_bead_id": "delivery-2"})
        self.assertTrue(validator.is_valid(successor))

    def test_epoch_receipt_uses_the_delivery_state_enum(self) -> None:
        validator = self.validator("epoch-receipt.v1.schema.json")
        valid = {"schema_version": "epoch-receipt.v1", "handoff_id": DIGEST, "epoch": 1, "predecessor_receipt_digest": None, "effect_receipt_digest": "b" * 64, "state": "landed"}
        self.assertTrue(validator.is_valid(valid))
        invalid = deepcopy(valid); invalid["state"] = "anything-at-all"
        self.assertFalse(validator.is_valid(invalid))

    def test_program_graph_role_policy_and_fallback_are_exact(self) -> None:
        graph = {"schema_version": "program-graph.v2", "program_id": "gc33-program", "intent_digest": DIGEST, "max_parallel": 1, "delivery_group_id": "group", "prefix_safety": "safe", "role_policy": {"mayor": {"model": "fable", "reasoning": "adaptive", "provider": "claude", "fallback": self.fallback()}, "planner": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": self.fallback()}, "validator": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": self.fallback()}, "worker_pool": {"default": {"model": "terra", "reasoning": "high", "provider": "codex", "fallback": self.fallback()}, "overflow": {"model": "opus", "reasoning": "medium", "provider": "claude", "fallback": self.fallback()}, "fallback": self.fallback()}, "refiner": {"model": "fable", "reasoning": "adaptive", "provider": "claude", "fallback": self.fallback(), "ambiguity_only": True}, "luna": {"model": "luna", "reasoning": "high", "provider": "codex", "fallback": self.fallback(), "support_only": True}}, "nodes": [{"id": "semantic", "bead_class": "product", "intent_digest": DIGEST, "depends_on": [], "write_scope": ["deploy/gc"], "generated_companions": [], "role": "implementation", "model": "terra", "reasoning": "high", "provider": "codex", "fallback": self.fallback()}]}
        validator = self.validator("program-graph.v2.schema.json")
        self.assertTrue(validator.is_valid(graph))
        for path, value in ((["role_policy", "worker_pool", "default", "reasoning"], "medium"), (["role_policy", "luna", "support_only"], False), (["nodes", 0, "fallback"], self.fallback(allowed=False, used=True, reason="no"))):
            candidate = deepcopy(graph); target: object = candidate
            for step in path[:-1]: target = target[step]  # type: ignore[index]
            target[path[-1]] = value  # type: ignore[index]
            self.assertFalse(validator.is_valid(candidate))
        fallback_paths = ((["role_policy", "mayor", "fallback"], self.fallback(allowed=True)), (["role_policy", "planner", "fallback"], self.fallback(used=True, reason="outage")), (["role_policy", "validator", "fallback"], self.fallback(allowed=True)), (["role_policy", "worker_pool", "default", "fallback"], self.fallback(allowed=True)), (["role_policy", "worker_pool", "overflow", "fallback"], self.fallback(allowed=True)), (["role_policy", "worker_pool", "fallback"], self.fallback(allowed=True)), (["role_policy", "refiner", "fallback"], self.fallback(allowed=True)), (["role_policy", "luna", "fallback"], self.fallback(allowed=True)), (["nodes", 0, "fallback"], self.fallback(allowed=True)))
        for path, value in fallback_paths:
            with self.subTest(path=path):
                candidate = deepcopy(graph); target: object = candidate
                for step in path[:-1]: target = target[step]  # type: ignore[index]
                target[path[-1]] = value  # type: ignore[index]
                self.assertFalse(validator.is_valid(candidate))

    def test_admission_and_role_runtime_fallback_facts_are_consistent(self) -> None:
        terra_author = {"context_id": "author", "requested_model": "terra", "requested_reasoning": "high", "requested_provider": "codex", "actual_model": "terra", "actual_reasoning": "high", "actual_provider": "codex", "fallback": self.fallback()}
        sol_validator = {"context_id": "validator", "requested_model": "sol", "requested_reasoning": "high", "requested_provider": "codex", "actual_model": "sol", "actual_reasoning": "high", "actual_provider": "codex", "fallback": self.fallback()}
        admission = {"schema_version": "admission-certificate.v2", "semantic_bead_id": "semantic", "intent_digest": DIGEST, "verdict": "PASS", "candidate": {"commit": "a" * 40, "tree": "b" * 40, "content_digest": "c" * 64}, "store": {"identity": "beads", "digest": "d" * 64}, "changed_path_manifest": "e" * 64, "verdict_digest": "f" * 64, "evidence_digest": "0" * 64, "attestations": {"author": terra_author, "validator": sol_validator}, "delivery_group_id": "group", "prefix_safety": "safe"}
        validator = self.validator("admission-certificate.v2.schema.json")
        self.assertTrue(validator.is_valid(admission))
        opus_author = deepcopy(terra_author); opus_author.update({"requested_model": "opus", "requested_reasoning": "medium", "requested_provider": "claude", "actual_model": "opus", "actual_reasoning": "medium", "actual_provider": "claude"})
        opus_admission = deepcopy(admission); opus_admission["attestations"]["author"] = opus_author
        self.assertTrue(validator.is_valid(opus_admission))
        empty_store = deepcopy(admission); empty_store["store"]["identity"] = ""
        self.assertFalse(validator.is_valid(empty_store))
        invalid = deepcopy(admission); invalid["attestations"]["author"]["fallback"] = self.fallback(allowed=False, used=True, reason="forbidden")
        self.assertFalse(validator.is_valid(invalid))
        wrong_author = deepcopy(admission); wrong_author["attestations"]["author"]["actual_model"] = "opus"
        self.assertFalse(validator.is_valid(wrong_author))
        wrong_validator = deepcopy(admission); wrong_validator["attestations"]["validator"]["actual_model"] = "terra"
        self.assertFalse(validator.is_valid(wrong_validator))
        request = {"schema_version": "factory-role-request.v2", "request_id": "request", "program_id": "program", "semantic_bead_id": "semantic", "intent_digest": DIGEST, "role": "validation", "requested": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": self.fallback()}, "artifact_path": "artifact", "result_path": "result"}
        request_validator = self.validator("factory-role-request.v2.schema.json")
        self.assertTrue(request_validator.is_valid(request))
        wrong = deepcopy(request); wrong["requested"]["model"] = "terra"
        self.assertFalse(request_validator.is_valid(wrong))
        allowed_request_fallback = deepcopy(request); allowed_request_fallback["requested"]["fallback"] = self.fallback(allowed=True)
        self.assertFalse(request_validator.is_valid(allowed_request_fallback))
        response = {"schema_version": "factory-role-response.v2", "request_id": "request", "request_digest": DIGEST, "role": "validation", "semantic_bead_id": "semantic", "session_context_id": "ctx", "requested": request["requested"], "actual": request["requested"], "artifact_path": "artifact", "artifact_digest": "b" * 64}
        response_validator = self.validator("factory-role-response.v2.schema.json")
        self.assertTrue(response_validator.is_valid(response))
        silent_downgrade = deepcopy(response); silent_downgrade["actual"] = {"model": "terra", "reasoning": "low", "provider": "codex", "fallback": self.fallback()}
        self.assertFalse(response_validator.is_valid(silent_downgrade))
        implementation = deepcopy(response); implementation["role"] = "implementation"; implementation["requested"] = {"model": "terra", "reasoning": "high", "provider": "codex", "fallback": self.fallback()}; implementation["actual"] = {"model": "opus", "reasoning": "medium", "provider": "claude", "fallback": self.fallback()}
        self.assertFalse(response_validator.is_valid(implementation))
        bad_response = deepcopy(response); bad_response["actual"]["fallback"] = self.fallback(allowed=True, used=False, reason="must be null")
        self.assertFalse(response_validator.is_valid(bad_response))

    def test_effect_receipts_bind_only_actual_landed_heads(self) -> None:
        validator = self.validator("effect-receipt.v1.schema.json")
        base = {"schema_version": "effect-receipt.v1", "effect_id": "effect", "handoff_id": DIGEST, "epoch": 1, "expected_remote_head": "a" * 40, "outcome": "applied", "resulting_sha": "b" * 40}
        self.assertTrue(validator.is_valid(base))
        already = deepcopy(base); already["outcome"] = "already_applied"
        self.assertTrue(validator.is_valid(already))
        missing = deepcopy(base); missing["resulting_sha"] = None
        self.assertFalse(validator.is_valid(missing))
        refused = deepcopy(base); refused["outcome"] = "refused"; refused["resulting_sha"] = None
        self.assertTrue(validator.is_valid(refused))
        false_receipt = deepcopy(refused); false_receipt["resulting_sha"] = "c" * 40
        self.assertFalse(validator.is_valid(false_receipt))

    def test_legacy_contracts_are_preserved_but_not_33_authority(self) -> None:
        factory = (ROOT / "docs/architecture/gas-city-factory.md").read_text(encoding="utf-8")
        adr = (ROOT / "docs/adr/ADR-0015-gas-city-fenced-steward.md").read_text(encoding="utf-8")
        self.assertNotIn("The binding decision is", factory)
        self.assertIn("Superseded for 3.3", adr)
        self.assertIn("Luna-high is support-only", factory)
        self.assertIn("factory-role-request.v2", factory)
        self.assertIn("non-admissible for any 3.3", factory)
        self.assertIn("Claim\nholder death is therefore not a relied-on store property", factory)
        self.assertIn("exactly one\nserialized creator/sweep authority", factory)
        self.assertIn("resulting SHA for\n`applied` and `already_applied`", factory)
        for name in ("factory-role-request.v1.schema.json", "factory-role-response.v1.schema.json", "delivery-record.v1.schema.json"):
            self.assertIn("Legacy 3.2", self.load(name)["title"])


if __name__ == "__main__":
    unittest.main()
