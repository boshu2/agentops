from __future__ import annotations

import importlib.util
import json
import subprocess
import sys
import tempfile
import threading
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location(
    "gc33_isolation_factory",
    ROOT / "packs/agentops-factory/assets/scripts/factory.py",
)
assert SPEC and SPEC.loader
factory = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(factory)
RELIABILITY_SPEC = importlib.util.spec_from_file_location("gc_reliability", ROOT / "deploy/gc/reliability.py")
assert RELIABILITY_SPEC and RELIABILITY_SPEC.loader
reliability = importlib.util.module_from_spec(RELIABILITY_SPEC); RELIABILITY_SPEC.loader.exec_module(reliability)


class GC33WriterIsolationTest(unittest.TestCase):
    def candidate(self, worktree: Path, branch: str, token: str) -> dict[str, object]:
        index = subprocess.run(["git", "-C", str(worktree), "rev-parse", "--git-path", "index"], text=True, capture_output=True, check=True).stdout.strip()
        return {"worktree": str(worktree), "branch": branch, "git_index": str((worktree / index).resolve()) if not Path(index).is_absolute() else index, "lease_token": token, "fence_epoch": 1}

    def test_width_two_disjoint_and_overlap_serializes(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "repo"; root.mkdir()
            subprocess.run(["git", "init", "-q", "-b", "main", str(root)], check=True)
            subprocess.run(["git", "-C", str(root), "config", "user.email", "test@example.invalid"], check=True)
            subprocess.run(["git", "-C", str(root), "config", "user.name", "test"], check=True)
            (root / "README").write_text("base\n", encoding="utf-8")
            subprocess.run(["git", "-C", str(root), "add", "README"], check=True)
            subprocess.run(["git", "-C", str(root), "commit", "-qm", "base"], check=True)
            product_tree, repair_tree = root.parent / "product", root.parent / "repair"
            subprocess.run(["git", "-C", str(root), "worktree", "add", "-qb", "candidate-product", str(product_tree)], check=True)
            subprocess.run(["git", "-C", str(root), "worktree", "add", "-qb", "candidate-repair", str(repair_tree)], check=True)
            ready = [
                {"id": "product", "bead_class": "product", "ready": True, "admitted_at": 2, "write_scope": ["src/product"], "generated_companions": [], "delivery_group_id": "p", "candidate": self.candidate(product_tree, "candidate-product", "lease-product")},
                {"id": "repair", "bead_class": "delivery_repair", "ready": True, "admitted_at": 1, "write_scope": ["src/repair"], "generated_companions": [], "delivery_group_id": "r", "candidate": self.candidate(repair_tree, "candidate-repair", "lease-repair")},
            ]
            selected = factory.admit_semantic_writers(ready, capacity=2, repair_streak=0)
            self.assertEqual([item["id"] for item in selected["selected"]], ["product", "repair"])
            self.assertEqual(len({item["candidate"]["worktree"] for item in selected["selected"]}), 2)
            self.assertEqual({item["candidate"]["fence_epoch"] for item in selected["selected"]}, {1})
            current_product = {"semantic_bead_id": "product", "lease_token": "lease-product", "fence_epoch": 1}
            self.assertTrue(factory.lease_receipt_is_current("product", dict(current_product), current_product))
            self.assertFalse(factory.lease_receipt_is_current("product", {"semantic_bead_id": "repair", "lease_token": "lease-product", "fence_epoch": 1}, current_product))
            self.assertFalse(factory.lease_receipt_is_current("product", dict(current_product), {"semantic_bead_id": "product", "lease_token": "lease-product-next", "fence_epoch": 2}))
            barrier = threading.Barrier(2)
            code = "import pathlib,sys,time; p=pathlib.Path(sys.argv[1]); p.mkdir(exist_ok=True); (p/sys.argv[2]).write_text('ready'); exec(\"while len(list(p.iterdir())) < 2: time.sleep(.01)\\ntime.sleep(.05)\")"
            def runner(mark: str) -> dict: barrier.wait(); return reliability.run_bounded_isolation([sys.executable, "-c", code, str(root.parent / "barrier"), mark], timeout_seconds=1)
            with ThreadPoolExecutor(max_workers=2) as pool:
                receipts = list(pool.map(runner, ("one", "two")))
            self.assertTrue(all(receipt["outcome"] == "clean" for receipt in receipts))
            self.assertEqual(len({receipt["runner_pgid"] for receipt in receipts}), 2)
            self.assertEqual(len({receipt["isolation_token"] for receipt in receipts}), 2)
            overlap = [dict(ready[0]), dict(ready[1], write_scope=["src/product/generated"])]
            self.assertEqual(len(factory.admit_semantic_writers(overlap, capacity=2, repair_streak=0)["selected"]), 1)
            duplicate = [dict(ready[0]), dict(ready[1], candidate=dict(ready[0]["candidate"]))]
            with self.assertRaisesRegex(factory.FactoryError, "candidate identities collide"):
                factory.admit_semantic_writers(duplicate, capacity=2, repair_streak=0)

    def test_capacity_policy_and_conflicts_are_closed_and_deterministic(self) -> None:
        def writer(identifier: str, kind: str, age: int, path: str) -> dict[str, object]:
            return {"id": identifier, "bead_class": kind, "admitted_at": age, "write_scope": [path], "generated_companions": [], "delivery_group_id": identifier}
        ready = [writer("p2", "product", 2, "p2"), writer("p1", "product", 1, "p1"), writer("r2", "delivery_repair", 2, "r2"), writer("r1", "delivery_repair", 1, "r1"), writer("r3", "delivery_repair", 3, "r3")]
        expected = {1: ["r1"], 2: ["p1", "r1"], 3: ["p1", "r1", "r2"], 4: ["p1", "r1", "r2", "r3"]}
        for capacity, ids in expected.items():
            with self.subTest(capacity=capacity):
                self.assertEqual([item["id"] for item in factory.admit_semantic_writers(ready, capacity=capacity, repair_streak=0)["selected"]], ids)
        self.assertEqual([item["id"] for item in factory.admit_semantic_writers(ready, capacity=1, repair_streak=2)["selected"]], ["p1"])
        self.assertEqual([item["id"] for item in factory.admit_semantic_writers([writer("p", "product", 1, "p")], capacity=4, repair_streak=0)["selected"]], ["p"])
        self.assertEqual([item["id"] for item in factory.admit_semantic_writers([writer("r", "delivery_repair", 1, "r")], capacity=4, repair_streak=0)["selected"]], ["r"])
        blocked = writer("blocked", "product", 0, "blocked"); blocked["dependencies_ready"] = False
        self.assertEqual([item["id"] for item in factory.admit_semantic_writers([blocked, writer("same-b", "product", 1, "b"), writer("same-a", "product", 1, "a")], capacity=4, repair_streak=0)["selected"]], ["same-a", "same-b"])
        for capacity in (0, 5):
            with self.assertRaises(factory.FactoryError): factory.admit_semantic_writers(ready, capacity=capacity, repair_streak=0)
        self.assertTrue(factory.writer_conflicts(writer("a", "product", 1, "src"), writer("b", "product", 2, "src/generated")))
        generated = writer("g", "product", 1, "one"); generated["generated_companions"] = ["build/shared"]
        generated_peer = writer("h", "product", 2, "two"); generated_peer["generated_companions"] = ["build/shared"]
        self.assertTrue(factory.writer_conflicts(generated, generated_peer))
        atomic_a = writer("x", "product", 1, "x"); atomic_a["atomic_group_id"] = "atomic"
        atomic_b = writer("y", "product", 2, "y"); atomic_b["atomic_group_id"] = "atomic"
        self.assertTrue(factory.writer_conflicts(atomic_a, atomic_b))


class GC33RoutingFirewallTest(unittest.TestCase):
    def test_every_construction_interleaving_has_zero_or_one_consumer(self) -> None:
        rows = factory.routing_truth_table(delivery_route="agentops.delivery")
        expected = {
            "staged_delivery": [], "published_delivery": ["delivery"],
            "staged_ambiguity": [], "published_ambiguity": ["refiner"],
        }
        self.assertEqual({row["name"]: row["consumers"] for row in rows}, expected)
        self.assertTrue(factory.validate_routing_truth_table(rows))
        staged_with_route = next(row["work"] for row in rows if row["name"] == "staged_ambiguity") | {"gc.routed_to": "rig/agentops.refiner"}
        self.assertFalse(factory.publishable_route(staged_with_route, "rig/agentops.refiner", "agentops.delivery"))
        self.assertFalse(factory.stock_model_predicate({"state": "closed", "assignee": "rig/agentops.refiner"}, "rig/agentops.refiner"))
        from jsonschema import Draft202012Validator
        schemas = ROOT / "packs/agentops-factory/assets/schemas"
        for row in rows:
            payload = row["work"]["payload"]
            name = "delivery.v1.schema.json" if payload["kind"] == "delivery" else "ambiguity-request.v1.schema.json"
            self.assertTrue(Draft202012Validator(json.loads((schemas / name).read_text())).is_valid(payload))
        malformed = dict(next(row["work"] for row in rows if row["name"] == "published_ambiguity")); malformed["payload"] = dict(malformed["payload"], kind="delivery")
        self.assertFalse(factory.publishable_route(malformed, "rig/agentops.refiner", "agentops.delivery"))
        for mutate in (lambda p: p.pop("deadline"), lambda p: p.__setitem__("extra", True), lambda p: p.__setitem__("publication", "bad")):
            broken = dict(next(row["work"] for row in rows if row["name"] == "published_ambiguity")); broken["payload"] = dict(broken["payload"]); mutate(broken["payload"])
            self.assertFalse(factory.publishable_route(broken, "rig/agentops.refiner", "agentops.delivery"))

    def test_draft_2020_12_schema_and_atomic_publication_are_authoritative(self) -> None:
        delivery = factory._delivery_payload()
        ambiguity = factory._ambiguity_payload()
        self.assertTrue(factory.payload_shape_valid(delivery))
        self.assertTrue(factory.payload_shape_valid(ambiguity))
        bad_cases = [
            (delivery, {"handoff_id": "short"}), (delivery, {"epoch": 0}),
            (delivery, {"deadline": "not-a-date"}), (delivery, {"deadline": "2026-07-22 00:00:00Z"}),
            (delivery, {"effect_gate": {"base_sha": "bad"}}), (delivery, {"effect_gate": {"committed_handoff_digest": "a" * 64, "base_sha": "b" * 40, "expected_remote_head": "c" * 40, "extra": True}}),
            (delivery, {"state": "branch_ready"}), (delivery, {"kind": "ambiguity_request"}),
            (ambiguity, {"facts": []}), (ambiguity, {"facts": [1]}),
            (ambiguity, {"deadline": "not-a-date"}), (ambiguity, {"deadline": "2026-07-22 00:00:00Z"}),
            (ambiguity, {"extra": True}),
        ]
        for payload, mutation in bad_cases:
            with self.subTest(mutation=mutation):
                candidate = dict(payload); candidate.update(mutation)
                self.assertFalse(factory.payload_shape_valid(candidate))
        store = factory.FakePublicationStore(); store.create("partial", "ambiguity")
        store.apply("partial", "schema_version", "ambiguity-request.v1", ready=True)
        before = store.snapshot("partial")
        with self.assertRaises(factory.FactoryError):
            store.publish("partial", delivery_route="agentops.delivery", refiner_route="rig/agentops.refiner")
        self.assertEqual(store.snapshot("partial"), before)
        rows = factory.routing_truth_table(delivery_route="agentops.delivery")
        published_delivery = next(row["work"] for row in rows if row["name"] == "published_delivery")
        published_ambiguity = next(row["work"] for row in rows if row["name"] == "published_ambiguity")
        for work in (dict(published_delivery, delivery_route=None),
                     dict(published_delivery, delivery_route="wrong.delivery"),
                     dict(published_delivery, **{"gc.routed_to": "rig/agentops.refiner"}),
                     dict(published_ambiguity, **{"gc.routed_to": None}),
                     dict(published_ambiguity, **{"gc.routed_to": "rig/agentops.mayor"}),
                     dict(published_ambiguity, delivery_route="agentops.delivery")):
            self.assertFalse(factory.publishable_route(work, "rig/agentops.refiner", "agentops.delivery"))

    def test_exhaustive_subset_readiness_proof(self) -> None:
        proof = factory.exhaustive_construction_proof()
        self.assertEqual(proof, {"construction_states": 2 * ((1 << 16) + (1 << 7)), "published_states": 4, "model_routes": 6})

    def test_routine_delivery_wakes_zero_models_and_one_fake_ambiguity_wakes_once(self) -> None:
        result = factory.run_inert_routing_harness()
        self.assertEqual(result["routine_refiner_starts"], 0)
        self.assertEqual(result["routine_model_claims"], 0)
        self.assertEqual(result["model_claims"], 1)
        self.assertEqual(result["claim_acquisitions"], 2)
        self.assertEqual(result["claim_expirations"], 1)
        self.assertEqual(len(result["providers"]["refiner"]["claims"]), 2)
        self.assertEqual(result["delivery_selections"], 1)
        self.assertEqual(result["ambiguity_starts"], 1)
        self.assertTrue(result["nonbinding_only"])


class GC33ComposedDoctorTest(unittest.TestCase):
    def test_broadened_model_query_fails_static_and_runtime_tripwires(self) -> None:
        self.assertTrue(factory.composed_route_doctor()["ok"])
        self.assertFalse(factory.composed_route_doctor({"refiner": "ready && unassigned"})["ok"])
        self.assertFalse(factory.composed_route_doctor(delivery_route="rig/agentops.refiner")["ok"])
        routes = factory.composed_route_doctor()["routes"]
        broadened = dict(routes, refiner={"route": routes["refiner"], "work_query": "ready+unassigned"})
        self.assertFalse(factory.validate_routing_truth_table(factory.routing_truth_table(delivery_route="agentops.delivery", model_routes=broadened)))

    def test_composed_inventory_has_six_native_routes_and_rejects_each_field_mutation(self) -> None:
        composition = factory.composed_route_doctor()
        inventory = composition["inventory"]
        self.assertTrue(factory.composed_inventory_parity(inventory))
        self.assertEqual(set(inventory), {"mayor", "plan-reviewer", "refiner", "implementer", "implementer-claude", "validator"})
        self.assertEqual(inventory["mayor"]["qualified_name"], "agentops.mayor")
        for role, spec in inventory.items():
            self.assertIsNone(spec["configured_work_query"])
            self.assertIsNone(spec["configured_sling_query"])
            self.assertEqual(spec["effective_sling_query"], f"gc.routed_to={spec['qualified_name']}")
            for field, value in (("qualified_name", "bad.route"), ("scope", "city" if spec["scope"] == "rig" else "rig"),
                                 ("binding", "wrong"), ("suspended", True), ("configured_work_query", "ready+unassigned"),
                                 ("effective_work_query", "ready+unassigned"), ("configured_sling_query", "gc.routed_to=bad.route"),
                                 ("effective_sling_query", "gc.routed_to=agentops.delivery")):
                with self.subTest(role=role, field=field):
                    mutated = factory.composed_route_doctor({role: {field: value}})
                    self.assertFalse(mutated["ok"])
                    self.assertFalse(factory.composed_inventory_parity(mutated["inventory"]))

    def test_doctor_executes_composition_and_exhaustive_proofs(self) -> None:
        self.assertEqual(factory.command_doctor(), 0)
        with mock.patch.object(factory, "composed_route_doctor", return_value={"ok": False, "reason": "injected", "inventory": {}}):
            self.assertEqual(factory.command_doctor(), 2)
        with mock.patch.object(factory, "exhaustive_construction_proof", side_effect=factory.FactoryError("injected", "failure")):
            self.assertEqual(factory.command_doctor(), 2)


if __name__ == "__main__":
    unittest.main()
