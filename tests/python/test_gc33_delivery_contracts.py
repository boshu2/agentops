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

    def handoff_delivery(self, publication: str = "non_routable") -> dict[str, object]:
        prepared = self.prepared()
        return {"schema_version": "delivery.v1", "kind": "delivery", "handoff_id": DIGEST, "semantic_bead_id": "semantic", "semantic_terminal_ref": "beads:semantic#gc33.terminal", "admission_certificate_digest": "b" * 64, "delivery_bead_id": "delivery", "external_ref": prepared["expected_external_ref"], "epoch": 1, "predecessor_receipt_digest": None, "mode": "auto", "state": "queued", "publication": publication, "deadline": "2026-07-22T00:00:00Z", "effect_gate": None, "successor_bead_id": None}

    def delivery_record(self) -> dict[str, object]:
        branch = "gc/delivery/" + DIGEST[:20]
        return {"schema_version": "gc.delivery.v1", "revision": 1, "handoff_id": DIGEST, "epoch": {"number": 1, "base_ref": "main", "base_oid": "a" * 40, "branch": branch}, "pr": {"id": "", "effect_id": "", "repository": "", "base_ref": "", "branch": ""}, "state": "queued", "current_receipt": {"path": "", "digest": ""}, "publication": "pending", "ready_at": "2026-07-21T00:00:00Z", "deadline": "2026-07-22T00:00:00Z", "semantic_bead_id": "semantic", "semantic_terminal_ref": "beads:semantic#gc33.terminal", "admission_certificate_digest": "b" * 64, "committed_handoff_digest": "", "mode": "auto", "rig_id": "rig", "repository": "boshu2/agentops", "remote": "origin", "candidate_oid": "c" * 40, "subject_manifest_digest": "d" * 64, "auto_merge_attempt": {"path": "", "digest": ""}}

    def committed(self) -> dict[str, object]:
        return {"schema_version": "handoff-committed.v1", "handoff_id": DIGEST, "prepared_digest": "c" * 64, "semantic_bead_id": "semantic", "semantic_terminal_verdict": "PASS", "semantic_terminal_ref": "beads:semantic#gc33.terminal", "admission_certificate_digest": "b" * 64, "delivery_bead_id": "delivery", "expected_external_ref": f"handoff:{DIGEST}:epoch:1", "epoch": 1, "delivery_payload_ref": "delivery.published.json", "delivery_payload_digest": "d" * 64, "mode": "auto", "state": "queued", "deadline": "2026-07-22T00:00:00Z", "committed_at": "2026-07-21T00:00:03Z"}

    def fallback(self, *, allowed: bool = False, used: bool = False, reason: object = None) -> dict[str, object]:
        return {"allowed": allowed, "used": used, "reason": reason}

    def test_every_33_schema_is_a_valid_draft_2020_12_schema(self) -> None:
        names = sorted(path.name for path in SCHEMAS.glob("*.schema.json"))
        self.assertIn("graph-admission-receipt.v1.schema.json", names)
        self.assertIn("factory-build-context.v1.schema.json", names)
        for name in names:
            with self.subTest(name=name):
                Draft202012Validator.check_schema(self.load(name))

    def test_branch_and_pr_receipts_bind_exact_target_identity(self) -> None:
        branch_name = "gc/delivery/" + "a" * 20
        branch = {"schema_version": "branch-receipt.v1", "handoff_id": DIGEST, "epoch": 1, "rig_id": "rig", "repository": "boshu2/agentops", "remote": "origin", "branch": branch_name, "base_ref": "main", "base_oid": "a" * 40, "expected_head": "b" * 40, "outcome": "observed", "response_digest": "c" * 64}
        pr = {"schema_version": "pr-open-receipt.v1", "intent_digest": "e" * 64, "handoff_id": DIGEST, "epoch": 1, "rig_id": "rig", "repository": "boshu2/agentops", "remote": "origin", "pr_id": "pr-abc", "branch": branch_name, "base_ref": "main", "expected_base_oid": "a" * 40, "observed_base_oid": "a" * 40, "expected_head": "b" * 40, "observed_head": "b" * 40, "effect_id": "d" * 64, "node_id": "PR_node", "number": "17", "url": "https://example.invalid/pull/17", "state": "open", "draft": False, "outcome": "applied", "response_digest": "c" * 64}
        self.assertTrue(self.validator("branch-receipt.v1.schema.json").is_valid(branch))
        self.assertTrue(self.validator("pr-open-receipt.v1.schema.json").is_valid(pr))
        for schema, payload in (("branch-receipt.v1.schema.json", branch), ("pr-open-receipt.v1.schema.json", pr)):
            broken = deepcopy(payload); broken["invented"] = True
            self.assertFalse(self.validator(schema).is_valid(broken))

    def test_native_formula_and_one_step_order_surfaces_remain_pack_owned(self) -> None:
        pack = ROOT / "packs" / "agentops-factory"
        build = (pack / "formulas" / "agentops-build.toml").read_text(encoding="utf-8")
        experiment = (pack / "formulas" / "agentops-experiment.toml").read_text(encoding="utf-8")
        order = (pack / "orders" / "agentops-delivery-sweep.toml").read_text(encoding="utf-8")
        sweep = (pack / "assets" / "scripts" / "delivery-sweep.sh").read_text(encoding="utf-8")
        feeder = (pack / "assets" / "scripts" / "factory_feeder.py").read_text(encoding="utf-8")
        program = (pack / "assets" / "scripts" / "program_start.py").read_text(encoding="utf-8")
        command = (pack / "commands" / "program-start" / "command.toml").read_text(encoding="utf-8")
        self.assertIn('formula = "agentops-build"', build)
        self.assertIn('id = "mayor"', build)
        self.assertIn('id = "plan"', build)
        self.assertNotIn('drain-experiments', build)
        self.assertIn('max_attempts = 1', build)
        self.assertIn('formula = "agentops-experiment"', experiment)
        self.assertIn('id = "admission"', experiment)
        self.assertIn('id = "implement"', experiment)
        self.assertIn('id = "validate"', experiment)
        self.assertIn('trigger = "cooldown"', order)
        self.assertIn('enabled = true', order)
        self.assertIn('idempotent = false', order)
        self.assertIn('AGENTOPS_GC_DELIVERY_BIN', sweep)
        self.assertIn('sweep', sweep)
        self.assertIn('AGENTOPS_GC_BEADS_BIN', sweep)
        self.assertIn('--subject-manifest', sweep)
        self.assertIn('--native-context', sweep)
        self.assertNotIn('factory.py', sweep)
        self.assertNotIn('FIXTURE_STATE', sweep)
        self.assertIn('create", "--graph"', feeder)
        self.assertNotIn('while True', feeder)
        self.assertIn('command = ["program", "start"]', command)
        self.assertIn('factory-bead-intent.v1', program)
        self.assertNotIn('while True', program)

    def test_actual_marker_artifacts_are_schema_conformant(self) -> None:
        cases = (("handoff-prepared.v1.schema.json", self.prepared()), ("delivery.v1.schema.json", self.handoff_delivery()), ("delivery.v1.schema.json", self.handoff_delivery("published")), ("handoff-committed.v1.schema.json", self.committed()), ("gc.delivery.v1.schema.json", self.delivery_record()))
        for name, payload in cases:
            with self.subTest(name=name):
                self.assertTrue(self.validator(name).is_valid(payload))

    def test_actual_marker_artifacts_reject_missing_and_conflicting_facts(self) -> None:
        cases = (("handoff-prepared.v1.schema.json", self.prepared(), "expected_external_ref"), ("delivery.v1.schema.json", self.handoff_delivery(), "external_ref"), ("handoff-committed.v1.schema.json", self.committed(), "expected_external_ref"), ("handoff-committed.v1.schema.json", self.committed(), "epoch"), ("gc.delivery.v1.schema.json", self.delivery_record(), "current_receipt"))
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
        validator = self.validator("gc.delivery.v1.schema.json")
        valid = self.delivery_record()
        valid.update({"revision": 8, "state": "merge_armed", "publication": "published", "committed_handoff_digest": "e" * 64, "gate_digest": "f" * 64, "arm_id": "0" * 64, "auto_merge_effect_id": "1" * 64, "current_receipt": {"path": f"handoffs/{DIGEST}/epochs/000001/merge-arm.json", "digest": "2" * 64}, "pr": {"id": "pr-stable", "effect_id": "3" * 64, "repository": "boshu2/agentops", "base_ref": "main", "branch": "gc/delivery/" + DIGEST[:20], "node_id": "PR_node", "number": "17", "url": "https://example.invalid/pull/17"}})
        valid["epoch"].update({"head": "4" * 40, "tree": "5" * 40})
        self.assertTrue(validator.is_valid(valid))
        cases = {
            "manual_merge_armed": {"mode": "manual"},
            "auto_manual_review": {"state": "manual_review"},
            "active_pending": {"publication": "pending", "committed_handoff_digest": ""},
            "landed_missing_receipt": {"state": "landed"},
            "epoch_two_missing_predecessor": {"epoch": {"number": 2, "base_ref": "main", "base_oid": "a" * 40, "branch": "gc/delivery/" + DIGEST[:20], "head": "4" * 40, "tree": "5" * 40}},
        }
        for name, update in cases.items():
            with self.subTest(name=name):
                candidate = deepcopy(valid); candidate.update(update)
                self.assertFalse(validator.is_valid(candidate))
        escaped = deepcopy(valid); escaped["auto_merge_attempt"] = {"path": f"handoffs/{DIGEST}/epochs/000001/../auto-merge-attempt.json", "digest": "6" * 64}
        self.assertFalse(validator.is_valid(escaped))

    def test_epoch_receipt_binds_composed_tree_and_path_proof(self) -> None:
        validator = self.validator("epoch-receipt.v1.schema.json")
        valid = {"schema_version": "epoch-receipt.v1", "handoff_id": DIGEST, "epoch": 1, "repository": "boshu2/agentops", "remote": "origin", "base_ref": "main", "observed_base_oid": "b" * 40, "candidate": "c" * 40, "subject_manifest_digest": "d" * 64, "path_proof": "e" * 64, "branch": "gc/delivery/" + "f" * 20, "epoch_head": "f" * 40, "epoch_tree": "0" * 40}
        self.assertTrue(validator.is_valid(valid))
        invalid = deepcopy(valid); invalid["epoch_tree"] = "anything-at-all"
        self.assertFalse(validator.is_valid(invalid))

    def test_program_graph_role_policy_and_fallback_are_exact(self) -> None:
        graph = {"schema_version": "program-graph.v2", "program_id": "gc33-program", "intent_digest": DIGEST, "repository_dir": "/repo", "base_ref": "main", "base_oid": "a" * 40, "workspace_root": "/worktrees", "packet_root": "/packets", "max_parallel": 1, "delivery_group_id": "group", "prefix_safety": "safe", "role_policy": {"mayor": {"model": "fable", "reasoning": "adaptive", "provider": "claude", "fallback": self.fallback()}, "planner": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": self.fallback()}, "validator": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": self.fallback()}, "worker_pool": {"default": {"model": "terra", "reasoning": "high", "provider": "codex", "fallback": self.fallback()}, "overflow": {"model": "opus", "reasoning": "medium", "provider": "claude", "fallback": self.fallback()}, "fallback": self.fallback()}, "refiner": {"model": "fable", "reasoning": "adaptive", "provider": "claude", "fallback": self.fallback(), "ambiguity_only": True}, "luna": {"model": "luna", "reasoning": "high", "provider": "codex", "fallback": self.fallback(), "support_only": True}}, "nodes": [{"id": "semantic", "title": "Semantic unit", "intent": "Make one bounded change.", "acceptance": ["Focused check passes."], "non_goals": ["No unrelated change."], "subject": {"includes": ["deploy/gc"], "excludes": [".git"]}, "first_check": "bash scripts/check-gc-executor.sh", "bead_class": "product", "intent_digest": DIGEST, "depends_on": [], "write_scope": ["deploy/gc"], "generated_companions": [], "role": "implementation", "model": "terra", "reasoning": "high", "provider": "codex", "fallback": self.fallback()}]}
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
        terra_author = {"context_id": "author", "requested_model": "terra", "requested_reasoning": "high", "requested_provider": "codex", "actual_model": "gpt-5.6-terra", "actual_reasoning": "high", "actual_provider": "codex", "actual_effort": "high", "fallback": self.fallback()}
        sol_validator = {"context_id": "validator", "requested_model": "sol", "requested_reasoning": "high", "requested_provider": "codex", "actual_model": "gpt-5.6-sol", "actual_reasoning": "high", "actual_provider": "codex", "actual_effort": "high", "fallback": self.fallback()}
        admission = {"schema_version": "admission-certificate.v2", "semantic_bead_id": "semantic", "intent_digest": DIGEST, "verdict": "PASS", "candidate": {"commit": "a" * 40, "tree": "b" * 40, "content_digest": "c" * 64}, "store": {"identity": "beads", "digest": "d" * 64}, "changed_path_manifest": "e" * 64, "verdict_digest": "f" * 64, "evidence_digest": "0" * 64, "attestations": {"author": terra_author, "validator": sol_validator}, "delivery_group_id": "group", "prefix_safety": "safe"}
        validator = self.validator("admission-certificate.v2.schema.json")
        self.assertTrue(validator.is_valid(admission))
        opus_author = deepcopy(terra_author); opus_author.update({"requested_model": "opus", "requested_reasoning": "medium", "requested_provider": "claude", "actual_model": "claude-opus-4-8", "actual_reasoning": "medium", "actual_provider": "claude", "actual_effort": "medium"})
        opus_admission = deepcopy(admission); opus_admission["attestations"]["author"] = opus_author
        self.assertTrue(validator.is_valid(opus_admission))
        empty_store = deepcopy(admission); empty_store["store"]["identity"] = ""
        self.assertFalse(validator.is_valid(empty_store))
        invalid = deepcopy(admission); invalid["attestations"]["author"]["fallback"] = self.fallback(allowed=False, used=True, reason="forbidden")
        self.assertFalse(validator.is_valid(invalid))
        wrong_author = deepcopy(admission); wrong_author["attestations"]["author"]["actual_model"] = "claude-opus-4-8"
        self.assertFalse(validator.is_valid(wrong_author))
        wrong_validator = deepcopy(admission); wrong_validator["attestations"]["validator"]["actual_model"] = "gpt-5.6-terra"
        self.assertFalse(validator.is_valid(wrong_validator))
        request = {"schema_version": "factory-role-request.v2", "request_id": "request", "program_id": "program", "semantic_bead_id": "semantic", "workspace": "workspace", "intent_source": "intent", "intent_digest": DIGEST, "subject_path": "subject", "subject_digest": "b" * 64, "evidence_refs": [{"path": "evidence", "digest": "c" * 64}], "prior_context_id": None, "role": "validation", "requested": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": self.fallback()}, "artifact_path": "artifact", "result_path": "result"}
        request_validator = self.validator("factory-role-request.v2.schema.json")
        self.assertTrue(request_validator.is_valid(request))
        wrong = deepcopy(request); wrong["requested"]["model"] = "terra"
        self.assertFalse(request_validator.is_valid(wrong))
        allowed_request_fallback = deepcopy(request); allowed_request_fallback["requested"]["fallback"] = self.fallback(allowed=True)
        self.assertFalse(request_validator.is_valid(allowed_request_fallback))
        response = {"schema_version": "factory-role-response.v2", "request_id": "request", "request_digest": DIGEST, "role": "validation", "semantic_bead_id": "semantic", "session_context_id": "ctx", "requested": request["requested"], "actual": {"model": "gpt-5.6-sol", "reasoning": "high", "provider": "codex", "effort": "high", "fallback": self.fallback()}, "artifact_path": "artifact", "artifact_digest": "b" * 64}
        response_validator = self.validator("factory-role-response.v2.schema.json")
        self.assertTrue(response_validator.is_valid(response))
        silent_downgrade = deepcopy(response); silent_downgrade["actual"] = {"model": "terra", "reasoning": "low", "provider": "codex", "fallback": self.fallback()}
        self.assertFalse(response_validator.is_valid(silent_downgrade))
        implementation = deepcopy(response); implementation["role"] = "implementation"; implementation["requested"] = {"model": "terra", "reasoning": "high", "provider": "codex", "fallback": self.fallback()}; implementation["actual"] = {"model": "claude-opus-4-8", "reasoning": "medium", "provider": "claude", "effort": "medium", "fallback": self.fallback()}
        self.assertFalse(response_validator.is_valid(implementation))
        bad_response = deepcopy(response); bad_response["actual"]["fallback"] = self.fallback(allowed=True, used=False, reason="must be null")
        self.assertFalse(response_validator.is_valid(bad_response))
        delivery_policy = deepcopy(response); delivery_policy["role"] = "delivery_policy"
        self.assertFalse(response_validator.is_valid(delivery_policy))

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

    def test_unread_legacy_contracts_are_removed_from_the_release_subject(self) -> None:
        factory = (ROOT / "docs/architecture/gas-city-factory.md").read_text(encoding="utf-8")
        adr = (ROOT / "docs/adr/ADR-0015-gas-city-fenced-steward.md").read_text(encoding="utf-8")
        self.assertNotIn("The binding decision is", factory)
        self.assertIn("Superseded for 3.3", adr)
        self.assertIn("Luna-high is support-only", factory)
        self.assertIn("factory-role-request.v2", factory)
        self.assertIn("removed during the 3.3 migration", factory)
        self.assertIn("Claim\nholder death is therefore not a relied-on store property", factory)
        self.assertIn("exactly one\nserialized creator/sweep authority", factory)
        self.assertIn("resulting SHA for\n`applied` and `already_applied`", factory)
        for name in ("admission-certificate.v1.schema.json", "program-graph.v1.schema.json", "rescope-context.v1.schema.json", "factory-role-request.v1.schema.json", "factory-role-response.v1.schema.json", "delivery-record.v1.schema.json"):
            self.assertFalse((SCHEMAS / name).exists(), name)


if __name__ == "__main__":
    unittest.main()
