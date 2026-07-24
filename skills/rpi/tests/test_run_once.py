from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock


MODULE_PATH = Path(__file__).parents[1] / "scripts" / "run_once.py"
SPEC = importlib.util.spec_from_file_location("rpi_run_once", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)
PLAN_MODULE_PATH = Path(__file__).parents[2] / "plan" / "scripts" / "mint_intent.py"
PLAN_SPEC = importlib.util.spec_from_file_location(
    "plan_mint_intent",
    PLAN_MODULE_PATH,
)
assert PLAN_SPEC and PLAN_SPEC.loader
PLAN_MODULE = importlib.util.module_from_spec(PLAN_SPEC)
PLAN_SPEC.loader.exec_module(PLAN_MODULE)


class RunOnceTests(unittest.TestCase):
    def phases(
        self,
        verdict: str = "PASS",
        intent_bytes: bytes = b"intent\n",
        intent_dir: Path | None = None,
    ):
        calls: list[str] = []

        def plan(intent):
            calls.append("plan")
            destination = intent_dir or Path(intent)
            path, digest, _ = MODULE.kernel.mint_intent_snapshot(
                intent_bytes,
                destination,
            )
            return {
                "intent_ref": f".agents/ao/intents/{path.name}",
                "intent_digest": digest,
                "byte_length": len(intent_bytes),
            }

        def implement(identity):
            calls.append("implement")
            self.assertEqual(
                identity["intent_digest"],
                MODULE.kernel.sha256(intent_bytes),
            )
            return {"frozen": True}

        def validate(identity, _candidate):
            calls.append("validate")
            return {
                "verdict": verdict,
                "intent_digest": identity["intent_digest"],
                "proof_identity": {
                    "epoch": 0,
                    "contract_ref": "proof.json",
                    "contract_digest": "a" * 64,
                    "activation_transition_digest": None,
                },
                "before_manifest_digest": "b" * 64,
                "final_manifest_digest": "c" * 64,
                "effect_receipt_digest": "d" * 64,
                "verdict_digest": "e" * 64,
                "verdict_ref": "verdict.json",
                "checked": ["acceptance"],
                "not_checked": [],
            }

        return calls, plan, implement, validate

    def invoke(self, plan, implement, validate, raw: str, correlation=None):
        return MODULE.invoke_once(
            raw,
            invocation_id="invocation:test",
            intent_dir=Path(raw),
            plan_phase=plan,
            implement_phase=implement,
            validate_phase=validate,
            correlation=correlation,
        )

    def test_each_phase_runs_once_and_pass_reports(self):
        with tempfile.TemporaryDirectory() as raw:
            calls, plan, implement, validate = self.phases()
            result = self.invoke(plan, implement, validate, raw)
            self.assertEqual(calls, ["plan", "implement", "validate"])
            self.assertEqual(result["schema_version"], "rpi-report.v2")
            self.assertEqual(result["status"], "PASS")
            self.assertEqual(result["intent_digest"], MODULE.kernel.sha256(b"intent\n"))
            self.assertNotIn("next_action", result)

    def test_whitespace_and_unicode_bytes_remain_distinct(self):
        first = "café\n".encode("utf-8")
        second = "cafe\u0301 \n".encode("utf-8")
        self.assertNotEqual(MODULE.kernel.sha256(first), MODULE.kernel.sha256(second))
        with tempfile.TemporaryDirectory() as raw:
            calls, plan, implement, validate = self.phases(intent_bytes=first)
            result = self.invoke(plan, implement, validate, raw)
            self.assertEqual(result["intent_digest"], MODULE.kernel.sha256(first))
            self.assertNotEqual(result["intent_digest"], MODULE.kernel.sha256(second))
            self.assertEqual(calls, ["plan", "implement", "validate"])

    def test_fail_and_not_proven_report_and_stop(self):
        for verdict in ("FAIL", "NOT_PROVEN"):
            with self.subTest(verdict=verdict), tempfile.TemporaryDirectory() as raw:
                calls, plan, implement, validate = self.phases(verdict)
                result = self.invoke(plan, implement, validate, raw)
                self.assertEqual(result["status"], verdict)
                self.assertEqual(calls, ["plan", "implement", "validate"])

    def test_missing_plan_stops_before_implement(self):
        with tempfile.TemporaryDirectory() as raw:
            calls: list[str] = []
            result = self.invoke(
                lambda _intent: None,
                lambda _identity: calls.append("implement"),
                lambda _identity, _candidate: calls.append("validate"),
                raw,
            )
            self.assertEqual(calls, [])
            self.assertEqual(result["status"], "NOT_PLANNED")

    def test_missing_candidate_stops_before_validate(self):
        with tempfile.TemporaryDirectory() as raw:
            calls: list[str] = []
            path, digest, _ = MODULE.kernel.mint_intent_snapshot(
                b"intent\n",
                Path(raw),
            )
            result = self.invoke(
                lambda _intent: {
                    "intent_ref": f".agents/ao/intents/{path.name}",
                    "intent_digest": digest,
                    "byte_length": len(b"intent\n"),
                },
                lambda _identity: None,
                lambda _identity, _candidate: calls.append("validate"),
                raw,
            )
            self.assertEqual(calls, [])
            self.assertEqual(result["status"], "NOT_BUILT")

    def test_validate_cannot_report_a_different_intent(self):
        with tempfile.TemporaryDirectory() as raw:
            _calls, plan, implement, validate = self.phases()

            def mismatched(identity, subject):
                result = validate(identity, subject)
                result["intent_digest"] = "f" * 64
                return result

            with self.assertRaisesRegex(ValueError, "exact intent bytes"):
                self.invoke(plan, implement, mismatched, raw)

    def test_plan_must_return_the_single_mint_identity_packet(self):
        with tempfile.TemporaryDirectory() as raw:
            with self.assertRaisesRegex(TypeError, "exact intent identity packet"):
                self.invoke(
                    lambda _intent: {"acceptance": ["works"]},
                    lambda _identity: {},
                    lambda _identity, _candidate: {},
                    raw,
                )

    def test_plan_packet_rejects_missing_boolean_and_mismatched_byte_length(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            path, digest, _ = MODULE.kernel.mint_intent_snapshot(b"intent\n", root)
            base = {
                "intent_ref": f".agents/ao/intents/{path.name}",
                "intent_digest": digest,
                "byte_length": len(b"intent\n"),
            }
            hostile = (
                {key: value for key, value in base.items() if key != "byte_length"},
                {**base, "byte_length": True},
                {**base, "byte_length": len(b"intent\n") + 1},
            )
            for packet in hostile:
                with self.subTest(packet=packet), self.assertRaises(
                    (TypeError, MODULE.kernel.ContractError)
                ):
                    self.invoke(
                        lambda _intent, packet=packet: packet,
                        lambda _identity: {},
                        lambda _identity, _candidate: {},
                        raw,
                    )

    def test_serialized_remote_boundary_preserves_single_mint_identity(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            intent_dir = root / "intents"
            living_source = root / "intent.md"
            original = "acceptance: café\n".encode()
            living_source.write_bytes(original)
            calls: list[str] = []

            def plan(_intent):
                calls.append("plan")
                payload = living_source.read_bytes()
                identity = PLAN_MODULE.mint_intent_identity(
                    payload,
                    intent_dir=intent_dir,
                    intent_ref_root=".agents/ao/intents",
                )
                living_source.write_text("acceptance: changed\n", encoding="utf-8")
                return identity

            def remote_packet(value):
                return json.loads(
                    json.dumps(value, sort_keys=True, separators=(",", ":"))
                )

            def implement(identity):
                calls.append("implement")
                transported = remote_packet(dict(identity))
                self.assertEqual(transported, dict(identity))
                self.assertEqual(
                    transported["intent_digest"],
                    MODULE.kernel.sha256(original),
                )
                self.assertEqual(transported["byte_length"], len(original))
                return {"frozen": True, "intent_identity": transported}

            def validate(identity, candidate):
                calls.append("validate")
                transported = remote_packet(
                    {"intent_identity": dict(identity), "subject": candidate}
                )
                self.assertEqual(transported["intent_identity"], dict(identity))
                self.assertEqual(candidate["intent_identity"], dict(identity))
                self.assertEqual(identity["byte_length"], len(original))
                self.assertNotEqual(
                    MODULE.kernel.sha256(living_source.read_bytes()),
                    identity["intent_digest"],
                )
                return {
                    "verdict": "PASS",
                    "intent_digest": identity["intent_digest"],
                    "proof_identity": {
                        "epoch": 0,
                        "contract_ref": "proof.json",
                        "contract_digest": "a" * 64,
                        "activation_transition_digest": None,
                    },
                    "before_manifest_digest": "b" * 64,
                    "final_manifest_digest": "c" * 64,
                    "effect_receipt_digest": "d" * 64,
                    "verdict_digest": "e" * 64,
                    "verdict_ref": "verdict.json",
                    "checked": ["remote packet exact identity"],
                    "not_checked": [],
                }

            original_mint = PLAN_MODULE.kernel.mint_intent_snapshot
            with mock.patch.object(
                PLAN_MODULE.kernel,
                "mint_intent_snapshot",
                wraps=original_mint,
            ) as mint:
                report = MODULE.invoke_once(
                    living_source,
                    invocation_id="invocation:remote",
                    intent_dir=intent_dir,
                    plan_phase=plan,
                    implement_phase=implement,
                    validate_phase=validate,
                )
            self.assertEqual(mint.call_count, 1)
            self.assertEqual(calls, ["plan", "implement", "validate"])
            self.assertEqual(report["intent_digest"], MODULE.kernel.sha256(original))
            self.assertEqual(
                (intent_dir / f"{report['intent_digest']}.intent").read_bytes(),
                original,
            )

    def test_opaque_correlation_is_preserved_without_interpretation(self):
        correlation = {
            "goal_id": "goal:123",
            "experiment_id": "experiment:456",
        }
        for verdict in ("PASS", "FAIL", "NOT_PROVEN"):
            with self.subTest(verdict=verdict), tempfile.TemporaryDirectory() as raw:
                _calls, plan, implement, validate = self.phases(verdict)
                report = self.invoke(
                    plan,
                    implement,
                    validate,
                    raw,
                    correlation=correlation,
                )
                self.assertEqual(report["correlation"], correlation)
        with tempfile.TemporaryDirectory() as raw:
            report = self.invoke(
                lambda _intent: None,
                lambda _identity: None,
                lambda _identity, _candidate: {},
                raw,
                correlation=correlation,
            )
            self.assertEqual(report["correlation"], correlation)

    def test_opaque_correlation_bounds_are_enforced(self):
        over_count = {f"key{index}": "value" for index in range(9)}
        with tempfile.TemporaryDirectory() as raw:
            with self.assertRaisesRegex(
                MODULE.kernel.ContractError,
                "bounded object",
            ):
                self.invoke(
                    lambda _intent: None,
                    lambda _identity: None,
                    lambda _identity, _candidate: {},
                    raw,
                    correlation=over_count,
                )
        with tempfile.TemporaryDirectory() as raw:
            with self.assertRaisesRegex(
                MODULE.kernel.ContractError,
                "exceeds bounds",
            ):
                self.invoke(
                    lambda _intent: None,
                    lambda _identity: None,
                    lambda _identity, _candidate: {},
                    raw,
                    correlation={"key": "x" * 257},
                )

    def test_report_can_be_persisted_durably(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            _calls, plan, implement, validate = self.phases(intent_dir=root / "intents")
            report = MODULE.invoke_once(
                raw,
                invocation_id="invocation:test",
                intent_dir=root / "intents",
                report_dir=root / "reports",
                plan_phase=plan,
                implement_phase=implement,
                validate_phase=validate,
            )
            path = root / "reports" / f"{report['artifact_digest']}.json"
            self.assertTrue(path.is_file())
            self.assertEqual(
                MODULE.kernel.load_json(path),
                report,
            )


if __name__ == "__main__":
    unittest.main()
