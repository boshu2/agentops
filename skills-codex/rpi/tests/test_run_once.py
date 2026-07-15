from __future__ import annotations

import importlib.util
from pathlib import Path
import unittest


MODULE_PATH = Path(__file__).parents[1] / "scripts" / "run_once.py"
SPEC = importlib.util.spec_from_file_location("rpi_run_once", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class RunOnceTests(unittest.TestCase):
    def phases(self, verdict: str = "PASS"):
        calls: list[str] = []

        def plan(intent):
            calls.append("plan")
            return {"schema_version": "plan-packet.v1", "intent": intent}

        def implement(_plan):
            calls.append("implement")
            return {"schema_version": "candidate-packet.v1"}

        def validate(_plan, _candidate):
            calls.append("validate")
            return {
                "verdict": verdict,
                "subject_manifest_digest": "a" * 64,
                "verdict_digest": "b" * 64,
                "verdict_ref": "/tmp/verdict.json",
                "checked": ["acceptance"],
                "not_checked": [],
            }

        return calls, plan, implement, validate

    def test_each_phase_runs_once_and_pass_reports(self):
        calls, plan, implement, validate = self.phases()
        result = MODULE.invoke_once("intent", plan, implement, validate)
        self.assertEqual(calls, ["plan", "implement", "validate"])
        self.assertEqual(result["status"], "PASS")
        self.assertNotIn("next_action", result)

    def test_fail_reports_and_stops_without_another_dispatch(self):
        calls, plan, implement, validate = self.phases("FAIL")
        result = MODULE.invoke_once("intent", plan, implement, validate)
        self.assertEqual(calls, ["plan", "implement", "validate"])
        self.assertEqual(result["status"], "FAIL")

    def test_missing_plan_stops_before_implement(self):
        calls: list[str] = []
        result = MODULE.invoke_once(
            "intent",
            lambda _intent: None,
            lambda _plan: calls.append("implement"),
            lambda _plan, _candidate: calls.append("validate"),
        )
        self.assertEqual(calls, [])
        self.assertEqual(result["status"], "NOT_PLANNED")

    def test_missing_candidate_stops_before_validate(self):
        calls: list[str] = []
        result = MODULE.invoke_once(
            "intent",
            lambda _intent: {"plan": "packet"},
            lambda _plan: None,
            lambda _plan, _candidate: calls.append("validate"),
        )
        self.assertEqual(calls, [])
        self.assertEqual(result["status"], "NOT_BUILT")


if __name__ == "__main__":
    unittest.main()
