from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


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
        self.assertEqual(set(command.choices), {"inspect-role-v2", "emit-role-v2", "doctor"})
        source = ADAPTER.read_text(encoding="utf-8")
        for forbidden in ("refinery", "merge_slot", "integration_rig", "delivery_record", "retry_delivery", "subprocess.run"):
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


if __name__ == "__main__":
    unittest.main()
