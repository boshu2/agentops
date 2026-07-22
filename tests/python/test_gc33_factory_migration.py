from __future__ import annotations

import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
ADAPTER = ROOT / "packs/agentops-factory/assets/scripts/role_adapter.py"


class GC33FactoryMigrationTest(unittest.TestCase):
    def adapter(self):
        spec = importlib.util.spec_from_file_location("gc33_role_adapter", ADAPTER)
        assert spec and spec.loader
        adapter = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(adapter)
        return adapter

    def test_only_role_v2_and_routing_doctor_remain_after_factory_retirement(self) -> None:
        self.assertTrue(ADAPTER.is_file(), "role-v2 adapter is missing")
        self.assertFalse((ROOT / "packs/agentops-factory/assets/scripts/factory.py").exists())
        adapter = self.adapter()
        command = next(action for action in adapter.parser()._actions if action.dest == "command")
        self.assertEqual(set(command.choices), {"inspect-role-v2", "emit-role-v2", "verify-role-v2", "doctor"})
        source = ADAPTER.read_text(encoding="utf-8")
        for forbidden in ("refinery", "merge_slot", "integration_rig", "delivery_record", "retry_delivery"):
            self.assertNotIn(forbidden, source)
        retired = ("admission-certificate.v1.schema.json", "program-graph.v1.schema.json", "rescope-context.v1.schema.json", "factory-role-request.v1.schema.json", "factory-role-response.v1.schema.json", "delivery-record.v1.schema.json")
        for name in retired:
            self.assertFalse((ROOT / "packs/agentops-factory/assets/schemas" / name).exists(), name)

    def test_gc33_5a_differential_fixture_pins_all_native_sources(self) -> None:
        fixture = json.loads((ROOT / "tests/fixtures/gc33/gc33-5a-differential.json").read_text(encoding="utf-8"))
        self.assertEqual(fixture["schema_version"], "gc33-5a-differential.v1")
        self.assertEqual(fixture["sources"]["gas_city"]["commit"], "8ffc009ded781a2ada2077f3a29bd712b2def0bf")
        self.assertEqual(fixture["sources"]["beads"]["commit"], "8e4e59d39f3459a43cf21a3236a13eca4dd874f7")
        self.assertEqual(fixture["sources"]["embedded_packs"]["commit"], "33d3a430a67d1782ad364556cb566bdb01d0afe3")
        self.assertEqual(fixture["forge_auto_merge"]["replay"], "marker_first_observe_only_no_resend")

    def test_composed_doctor_reads_all_role_tomls_and_rejects_model_overlap(self) -> None:
        adapter = self.adapter()
        composition = adapter.composed_route_doctor()
        self.assertTrue(composition["ok"], composition["reason"])
        self.assertEqual(set(composition["inventory"]), {"mayor", "plan-reviewer", "refiner", "implementer", "implementer-claude", "validator"})
        self.assertFalse(adapter.composed_route_doctor({"refiner": {"work_query": "ready+unassigned"}})["ok"])
        self.assertFalse(adapter.composed_route_doctor(delivery_route="rig/agentops.refiner")["ok"])

    def test_request_contract_rejects_policy_or_identity_drift(self) -> None:
        adapter = self.adapter()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            intent = root / "intent"; intent.write_text("intent", encoding="utf-8")
            digest = adapter.digest_file(intent)
            request = {"schema_version": "factory-role-request.v2", "request_id": "r", "program_id": "p", "semantic_bead_id": "s", "workspace": str(root), "intent_source": str(intent), "intent_digest": digest, "subject_path": str(intent), "subject_digest": digest, "evidence_refs": [{"path": str(intent), "digest": digest}], "prior_context_id": None, "role": "mayor", "requested": adapter.requested_role_policy("mayor"), "artifact_path": str(root / "artifact"), "result_path": str(root / "result")}
            path = root / "request.json"; path.write_text(json.dumps(request), encoding="utf-8")
            self.assertEqual(adapter.validate_request(path)[0]["request_id"], "r")
            request["requested"]["fallback"]["used"] = True
            path.write_text(json.dumps(request), encoding="utf-8")
            with self.assertRaises(adapter.RoleAdapterError):
                adapter.validate_request(path)

    def test_role_emission_attests_fable_launch_and_claimed_bead(self) -> None:
        adapter = self.adapter()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            intent = root / "intent"; intent.write_text("intent", encoding="utf-8")
            digest = adapter.digest_file(intent)
            artifact = root / "graph.json"
            artifact.write_text(json.dumps({
                "schema_version": "program-graph.v2", "program_id": "p",
                "intent_digest": digest, "repository_dir": "/repo",
                "base_ref": "main", "base_oid": "a" * 40,
                "workspace_root": "/worktrees", "packet_root": "/packets",
                "max_parallel": 1, "delivery_group_id": "delivery",
                "prefix_safety": "safe",
                "role_policy": {
                    "mayor": adapter.requested_role_policy("mayor"),
                    "planner": adapter.requested_role_policy("plan"),
                    "validator": adapter.requested_role_policy("validation"),
                    "refiner": adapter.requested_role_policy("mayor") | {"ambiguity_only": True},
                    "luna": {"model": "luna", "reasoning": "high", "provider": "codex", "fallback": {"allowed": False, "used": False, "reason": None}, "support_only": True},
                    "worker_pool": {"default": adapter.requested_role_policy("implementation"), "overflow": {"model": "opus", "reasoning": "medium", "provider": "claude", "fallback": {"allowed": False, "used": False, "reason": None}}, "fallback": {"allowed": False, "used": False, "reason": None}},
                },
                "nodes": [{"id": "n", "title": "n", "intent": "do n", "acceptance": ["works"], "non_goals": [], "subject": {"includes": ["src"], "excludes": []}, "first_check": "true", "bead_class": "product", "intent_digest": digest, "depends_on": [], "write_scope": ["src"], "generated_companions": [], "role": "implementation", "model": "terra", "reasoning": "high", "provider": "codex", "fallback": {"allowed": False, "used": False, "reason": None}}],
            }, sort_keys=True), encoding="utf-8")
            request = {"schema_version": "factory-role-request.v2", "request_id": "r", "program_id": "p", "semantic_bead_id": "mayor-step", "workspace": str(root), "intent_source": str(intent), "intent_digest": digest, "subject_path": str(intent), "subject_digest": digest, "evidence_refs": [{"path": str(intent), "digest": digest}], "prior_context_id": None, "role": "mayor", "requested": adapter.requested_role_policy("mayor"), "artifact_path": str(artifact), "result_path": str(root / "result.json")}
            request_path = root / "request.json"; request_path.write_text(json.dumps(request), encoding="utf-8")

            def fake_gc(arguments):
                if arguments[:3] == ["session", "list", "--state"]:
                    return {"sessions": [{"id": "session-1", "provider": "claude", "template": "agentops.mayor", "session_name": "mayor-1", "state": "working", "model": "claude-fable-5"}]}
                if arguments[:2] == ["bd", "show"] and arguments[2] == "session-1":
                    return {"id": "session-1", "issue_type": "session", "status": "open", "metadata": {"provider": "claude", "template": "agentops.mayor", "session_name": "mayor-1", "state": "working", "command": "claude --model claude-fable-5"}}
                if arguments[:2] == ["bd", "show"] and arguments[2] == "mayor-step":
                    return {"id": "mayor-step", "assignee": "mayor-1", "metadata": {"gc.routed_to": "agentops.mayor", "work_dir": str(root)}}
                raise AssertionError(arguments)

            environment = {"GC_SESSION_ID": "session-1", "GC_SESSION_NAME": "mayor-1", "GC_PROVIDER": "claude", "GC_TEMPLATE": "agentops.mayor", "GC_ARTIFACT_DIR": str(root / "attempt")}
            args = type("Args", (), {"request": str(request_path), "artifact": str(artifact), "bead": "mayor-step"})()
            with mock.patch.dict(os.environ, environment, clear=False), mock.patch.object(adapter, "run_gc_json", side_effect=fake_gc):
                self.assertEqual(adapter.command_emit_role_v2(args), 0)
            response = json.loads((root / "result.json").read_text(encoding="utf-8"))
            self.assertEqual(response["actual"], {"model": "claude-fable-5", "reasoning": "adaptive", "provider": "claude", "effort": None, "fallback": {"allowed": False, "used": False, "reason": None}})
            self.assertEqual(json.loads((root / "attempt/agentops-factory-check-request.json").read_text())["semantic_bead_id"], "mayor-step")

    def test_role_emission_rejects_silent_sol_downgrade(self) -> None:
        adapter = self.adapter()
        request = {"role": "plan", "requested": adapter.requested_role_policy("plan"), "workspace": Path("/work")}
        session = {"id": "session", "provider": "codex", "template": "rig/agentops.plan-reviewer", "session_name": "plan-1", "state": "working", "model": "gpt-5.6-terra", "effort": "high", "reasoning": "high", "fallback": {"allowed": False, "used": False, "reason": None}}
        with self.assertRaises(adapter.RoleAdapterError):
            adapter.validate_actual_identity(request, session)


if __name__ == "__main__":
    unittest.main()
