from __future__ import annotations

import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import unittest


ROOT = Path(__file__).resolve().parents[2]
HARNESS = ROOT / "deploy" / "gc" / "beads-capability.py"
STATIC_REFERENCE = ROOT / "deploy" / "gc" / "beads_capability_static_reference.py"


class BeadsCapabilityHarnessTest(unittest.TestCase):
    def receipt(self) -> dict[str, object]:
        return json.loads((ROOT / "deploy/gc/beads-capability-selection.v1.json").read_text(encoding="utf-8"))

    def test_receipt_identity_contract_requires_explicit_real_binaries_only(self) -> None:
        source = HARNESS.read_text(encoding="utf-8")
        for token in ("--bd-bin", "--dolt-bin", "lock_provenance", "lock_sha256", "harness_sha256", "BD_COMMIT", "NOT_EXERCISED"):
            self.assertIn(token, source)
        for forbidden in ("--toolchain-receipt", "toolchain.json", "gascity_source_commit", "shutil.which"):
            self.assertNotIn(forbidden, source)

    def test_committed_receipt_binds_current_harness_and_lock_without_running_dolt(self) -> None:
        receipt = self.receipt()
        self.assertEqual(receipt["schema_version"], "beads-capability-selection.v1")
        self.assertEqual(receipt["selected_representation"], "B-successor-delivery-bead")
        toolchain = receipt["toolchain"]
        self.assertEqual(toolchain["harness_sha256"], hashlib.sha256(HARNESS.read_bytes()).hexdigest())
        self.assertEqual(toolchain["lock_sha256"], hashlib.sha256((ROOT / "deploy/gc/toolchain.lock.json").read_bytes()).hexdigest())
        self.assertEqual(toolchain["bd"]["version"], "bd version 1.1.0 (8e4e59d)")
        self.assertTrue(toolchain["dolt"]["version"].startswith("dolt version 2.1.10"))
        self.assertEqual(toolchain["gc_lock_provenance"]["gc"]["version"], "1.3.5")
        self.assertEqual(toolchain["gc_lock_provenance"]["gc"]["source_commit"], "8ffc009ded781a2ada2077f3a29bd712b2def0bf")
        self.assertEqual(toolchain["gc_lock_provenance"]["bd"]["source_commit"], "8e4e59d39f3459a43cf21a3236a13eca4dd874f7")
        self.assertEqual(toolchain["gc_lock_provenance"]["status"], "LOCK_PROVENANCE_NOT_EXERCISED")
        self.assertEqual(toolchain["gc_runtime"]["status"], "NOT_EXERCISED")
        self.assertEqual(toolchain["harness_sha256"], "b5d6d4490492de047554c984008289282aab82cd3e89bf67d5ae8ea71bcbc48e")

    def test_static_handoff_reference_is_separate_from_live_observation(self) -> None:
        receipt = self.receipt()
        self.assertEqual(hashlib.sha256(HARNESS.read_bytes()).hexdigest(), receipt["toolchain"]["harness_sha256"])
        source = STATIC_REFERENCE.read_text(encoding="utf-8")
        self.assertIn("store-observation provenance", source)
        self.assertNotIn("subprocess", source)
        spec = importlib.util.spec_from_file_location("gc33_static_reference", STATIC_REFERENCE)
        self.assertIsNotNone(spec)
        module = importlib.util.module_from_spec(spec)
        self.assertIsNotNone(spec.loader)
        spec.loader.exec_module(module)
        prepared = {"handoff_id": "a" * 64, "semantic_bead_id": "semantic", "semantic_terminal_ref": "beads:semantic#terminal", "admission_certificate_digest": "b" * 64, "expected_delivery_bead_id": "delivery", "expected_external_ref": f"handoff:{'a' * 64}:epoch:1", "epoch": 1, "mode": "auto", "state": "queued", "deadline": "2026-07-22T00:00:00Z"}
        published = {"kind": "delivery", "handoff_id": prepared["handoff_id"], "semantic_bead_id": prepared["semantic_bead_id"], "semantic_terminal_ref": prepared["semantic_terminal_ref"], "admission_certificate_digest": prepared["admission_certificate_digest"], "delivery_bead_id": prepared["expected_delivery_bead_id"], "external_ref": prepared["expected_external_ref"], "epoch": 1, "mode": "auto", "state": "queued", "publication": "published", "deadline": prepared["deadline"]}
        committed = {"handoff_id": prepared["handoff_id"], "prepared_digest": "c" * 64, "semantic_bead_id": prepared["semantic_bead_id"], "semantic_terminal_verdict": "PASS", "semantic_terminal_ref": prepared["semantic_terminal_ref"], "admission_certificate_digest": prepared["admission_certificate_digest"], "delivery_bead_id": prepared["expected_delivery_bead_id"], "expected_external_ref": prepared["expected_external_ref"], "epoch": 1, "delivery_payload_ref": "delivery.published.json", "delivery_payload_digest": "d" * 64, "mode": "auto", "state": "queued", "deadline": prepared["deadline"]}
        self.assertTrue(module.committed_handoff_matches(prepared, published, committed, prepared_digest="c" * 64, published_digest="d" * 64))
        for field, wrong in (("semantic_bead_id", "other-semantic"), ("expected_external_ref", "handoff:wrong:epoch:1"), ("epoch", 2), ("delivery_bead_id", "other-delivery")):
            with self.subTest(field=field):
                candidate = dict(committed); candidate[field] = wrong
                self.assertFalse(module.committed_handoff_matches(prepared, published, candidate, prepared_digest="c" * 64, published_digest="d" * 64))

    def test_receipt_has_complete_claim_and_successor_identity_evidence(self) -> None:
        receipt = self.receipt()
        properties = receipt["properties"]
        claim = properties["claim_race_exclusive"]
        self.assertTrue(claim["observed"])
        self.assertEqual(claim["trial_count"], len(claim["trials"]))
        for row in claim["trials"]:
            self.assertEqual(len(row["exit_codes"]), 2)
            self.assertEqual(row["successful_claimants"], 1)
            self.assertIn(row["successful_actor"], row["claimants"])
            self.assertEqual(row["successful_actor"], row["expected_final_assignee"])
            self.assertEqual(row["final_assignee"], row["expected_final_assignee"])
            self.assertIn(row["final_assignee"], row["claimants"])
        successor = properties["deterministic_successor_create_or_discover"]
        self.assertTrue(successor["observed"])
        self.assertTrue(successor["positive_rediscovery"])
        self.assertEqual(successor["external_ref"], f"handoff:{successor['handoff_id']}:epoch:{successor['epoch']}")

    def test_receipt_proves_schema_valid_crash_cuts_and_refusals(self) -> None:
        receipt = self.receipt()
        handoff = receipt["properties"]["marker_first_cross_store_handoff"]
        self.assertTrue(handoff["observed"])
        self.assertEqual([row["cut"] for row in handoff["per_cut"]], list(range(5)))
        for row in handoff["per_cut"]:
            self.assertTrue(row["converged"])
            self.assertTrue(row["exactly_one_delivery_bead"])
            self.assertEqual(row["precommit_authorizations"], 0)
            self.assertTrue(all(row["schema_valid_artifacts"].values()))
        conflict = handoff["negative_preexisting_conflicting_child"]
        self.assertEqual(conflict["action"], "refuse_conflicting_delivery_identity")
        self.assertFalse(conflict["effect_authorized"])
        self.assertEqual(conflict["delivery_count"], 1)
        self.assertFalse(conflict["published"])
        self.assertFalse(conflict["committed"])
        self.assertTrue(handoff["negative_preexisting_conflicting_child_observed"])
        self.assertEqual(handoff["late_conflicting_terminal"], {"action": "refuse_conflicting_terminal", "effect_authorized": False})
        self.assertTrue(handoff["late_conflicting_terminal_observed"])
        self.assertEqual(receipt["properties"]["event_single_flight"]["result"], "NOT_PROVEN")
        self.assertTrue(receipt["cleanup"]["no_attributed_process_remains"])

    def test_probe_requires_explicit_binary_inputs(self) -> None:
        result = subprocess.run([sys.executable, str(HARNESS), "--output", "/tmp/unused"], text=True, capture_output=True, check=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("--bd-bin", result.stderr)

    def test_missing_marker_is_not_counted_as_a_valid_artifact(self) -> None:
        spec = importlib.util.spec_from_file_location("gc33_capability", HARNESS)
        self.assertIsNotNone(spec)
        module = importlib.util.module_from_spec(spec)
        self.assertIsNotNone(spec.loader)
        spec.loader.exec_module(module)
        with self.subTest("missing load"):
            payload, error = module.load_valid(ROOT / "does-not-exist.json", "delivery.v1.schema.json")
            self.assertIsNone(payload)
            self.assertIsNone(error)
            self.assertFalse(payload is not None and error is None)

    def test_selection_algorithm_requires_both_negative_refusals(self) -> None:
        spec = importlib.util.spec_from_file_location("gc33_capability_selection", HARNESS)
        self.assertIsNotNone(spec)
        module = importlib.util.module_from_spec(spec)
        self.assertIsNotNone(spec.loader)
        spec.loader.exec_module(module)
        cuts = [{"exactly_one_delivery_bead": True, "converged": True, "precommit_authorizations": 0, "schema_valid_artifacts": {"prepared": True, "delivery": True, "committed": True}}]
        self.assertTrue(module.handoff_selection_observed(cuts, True, True))
        self.assertFalse(module.handoff_selection_observed(cuts, False, True))
        self.assertFalse(module.handoff_selection_observed(cuts, True, False))

    def test_child_identity_requires_exact_external_ref_handoff_and_epoch(self) -> None:
        spec = importlib.util.spec_from_file_location("gc33_capability_identity", HARNESS)
        self.assertIsNotNone(spec)
        module = importlib.util.module_from_spec(spec)
        self.assertIsNotNone(spec.loader)
        spec.loader.exec_module(module)
        prepared = module.prepared_payload(7)
        child = {"id": prepared["expected_delivery_bead_id"], "external_ref": prepared["expected_external_ref"]}
        self.assertTrue(module.child_identity_matches(prepared, child))
        child["external_ref"] = module.external_ref("b" * 64, 1)
        self.assertFalse(module.child_identity_matches(prepared, child))


if __name__ == "__main__":
    unittest.main()
