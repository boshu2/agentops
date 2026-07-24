from __future__ import annotations

import importlib.util
from pathlib import Path
import tempfile
import unittest


MODULE_PATH = Path(__file__).parents[1] / "scripts" / "run_once.py"
SPEC = importlib.util.spec_from_file_location("rpi_run_once", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class RunOnceTests(unittest.TestCase):
    def phases(self, verdict: str = "PASS", intent_bytes: bytes = b"intent\n"):
        calls: list[str] = []

        def plan(_intent):
            calls.append("plan")
            return intent_bytes

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
            result = self.invoke(
                lambda _intent: b"intent\n",
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

    def test_plan_must_return_bytes_not_a_reserialized_mapping(self):
        with tempfile.TemporaryDirectory() as raw:
            with self.assertRaisesRegex(TypeError, "exact resolved intent bytes"):
                self.invoke(
                    lambda _intent: {"acceptance": ["works"]},
                    lambda _identity: {},
                    lambda _identity, _candidate: {},
                    raw,
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
            _calls, plan, implement, validate = self.phases()
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
