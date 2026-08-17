from __future__ import annotations

import importlib.util
import json
import os
from pathlib import Path
import tempfile
import time
import unittest


MODULE_PATH = Path(__file__).parents[1] / "scripts" / "run_once.py"
SPEC = importlib.util.spec_from_file_location("rpi_run_once", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)

INTENT_DIGEST = "c" * 64
SUBJECT_DIGEST = "a" * 64
HELPER = r'''#!/usr/bin/env python3
import json, os, subprocess, sys, time
phase, mode, log_path, pid_path = sys.argv[1:5]
payload = json.load(sys.stdin)
if mode == "hang":
    time.sleep(30)
if mode == "oversized":
    sys.stdout.write('"' + ('x' * 100000) + '"')
    raise SystemExit(0)
if mode == "spawn-descendant":
    child = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(30)"])
    open(pid_path, "w", encoding="utf-8").write(str(child.pid))
with open(log_path, "a", encoding="utf-8") as log:
    log.write(phase + "\n")
if phase == "anti_ceremony":
    decision = "STOP" if mode == "stop" else "CONTINUE"
    print(json.dumps({
        "decision": decision,
        "reason": "The frozen outcome still requires implementation proof.",
        "frozen_outcome": str(payload["intent"]),
        "parked_process_work": [],
        "remaining_proof": [] if decision == "STOP" else ["implementation", "fresh validation"],
        "stop_condition": "Stop after one fresh validation result.",
    }))
elif phase == "plan":
    print("null" if mode == "none" else json.dumps({
        "intent_ref": "bead:agentops-test",
        "acceptance": ["works"],
        "acceptance_digest": "c" * 64,
    }))
elif phase == "implement":
    print("null" if mode == "none" else json.dumps({"subject_manifest_digest": "a" * 64}))
elif phase == "validate":
    print(json.dumps({
        "verdict": mode if mode in ("PASS", "FAIL", "NOT_PROVEN") else "PASS",
        "acceptance_digest": "c" * 64,
        "subject_manifest_digest": "a" * 64,
        "author_context_id": "author-ctx",
        "validator_context_id": "validator-ctx",
        "freshness_attestation": {"source": "runtime", "attester_identity": "runtime:rpi-test"},
        "checked": ["acceptance"],
        "not_checked": [],
    }))
else:
    raise SystemExit(9)
'''


class RunOnceTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name).resolve()
        self.helper = self.root / "phase.py"
        self.helper.write_text(HELPER, encoding="utf-8")
        self.log = self.root / "calls.log"
        self.pid_file = self.root / "child.pid"
        self.python = str(Path(os.sys.executable).resolve(strict=True))

    def tearDown(self) -> None:
        self.temp.cleanup()

    def policy(self, *, max_input_bytes: int = 16_384, **modes: str):
        commands = {}
        for phase in MODULE.PHASES:
            mode = modes.get(phase, "PASS" if phase == "validate" else "normal")
            commands[phase] = {
                "argv": [
                    self.python,
                    str(self.helper),
                    phase,
                    mode,
                    str(self.log),
                    str(self.pid_file),
                ],
                "cwd": ".",
            }
        return MODULE.ExecutionPolicy(
            workspace_root=self.root,
            commands=commands,
            timeout_seconds=0.4,
            max_input_bytes=max_input_bytes,
            max_output_bytes=16_384,
        )

    def calls(self) -> list[str]:
        if not self.log.exists():
            return []
        return self.log.read_text(encoding="utf-8").splitlines()

    def test_authorized_commands_run_once_in_order_and_report(self) -> None:
        result = MODULE.invoke_once({"intent": "ship"}, self.policy())
        self.assertEqual(self.calls(), list(MODULE.PHASES))
        self.assertEqual(result["status"], "PASS")
        self.assertEqual(result["acceptance_digest"], INTENT_DIGEST)
        self.assertEqual(result["subject_manifest_digest"], SUBJECT_DIGEST)
        self.assertNotIn("next_action", result)

    def test_stop_and_incomplete_results_do_not_trigger_later_phases(self) -> None:
        cases = (
            ({"anti_ceremony": "stop"}, "NOT_PLANNED", ["anti_ceremony"]),
            ({"plan": "none"}, "NOT_PLANNED", ["anti_ceremony", "plan"]),
            (
                {"implement": "none"},
                "NOT_BUILT",
                ["anti_ceremony", "plan", "implement"],
            ),
        )
        for modes, expected_status, expected_calls in cases:
            with self.subTest(modes=modes):
                self.log.unlink(missing_ok=True)
                result = MODULE.invoke_once("intent", self.policy(**modes))
                self.assertEqual(result["status"], expected_status)
                self.assertEqual(self.calls(), expected_calls)

    def test_behavior_contract_matrix_rejects_prior_ambient_shapes(self) -> None:
        # Baseline defect: arbitrary in-process callables were the public API.
        # Candidate: a complete immutable command policy is required.
        with self.assertRaises(TypeError):
            MODULE.invoke_once("intent", lambda _value: {})
        incomplete = {
            phase: {"argv": [self.python, str(self.helper), phase, "normal", str(self.log), str(self.pid_file)]}
            for phase in MODULE.PHASES[:-1]
        }
        with self.assertRaisesRegex(ValueError, "exactly"):
            MODULE.ExecutionPolicy(
                workspace_root=self.root,
                commands=incomplete,
                timeout_seconds=1,
                max_input_bytes=1024,
                max_output_bytes=1024,
            )
        self.assertEqual(self.calls(), [])

    def test_unapproved_path_symlink_and_out_of_root_cwd_fail_before_launch(self) -> None:
        commands = self.policy().commands
        near = {name: {"argv": list(spec[0]), "cwd": "."} for name, spec in commands.items()}
        near["plan"]["cwd"] = "../outside"
        with self.assertRaisesRegex(ValueError, "stay inside"):
            MODULE.ExecutionPolicy(
                workspace_root=self.root,
                commands=near,
                timeout_seconds=1,
                max_input_bytes=1024,
                max_output_bytes=1024,
            )
        executable_link = self.root / "python-link"
        executable_link.symlink_to(self.python)
        near["plan"] = {"argv": [str(executable_link), str(self.helper), "plan", "normal", str(self.log), str(self.pid_file)]}
        near["plan"]["cwd"] = "."
        with self.assertRaisesRegex(ValueError, "must not be a symlink"):
            MODULE.ExecutionPolicy(
                workspace_root=self.root,
                commands=near,
                timeout_seconds=1,
                max_input_bytes=1024,
                max_output_bytes=1024,
            )
        self.assertEqual(self.calls(), [])

    def test_hung_phase_and_oversized_io_fail_closed(self) -> None:
        with self.assertRaisesRegex(TimeoutError, "plan"):
            MODULE.invoke_once("intent", self.policy(plan="hang"))
        self.assertEqual(self.calls(), ["anti_ceremony"])
        self.log.unlink(missing_ok=True)
        with self.assertRaisesRegex(ValueError, "output exceeds"):
            MODULE.invoke_once("intent", self.policy(plan="oversized"))
        self.assertEqual(self.calls(), ["anti_ceremony"])
        tiny = self.policy(max_input_bytes=8)
        with self.assertRaisesRegex(ValueError, "input exceeds"):
            MODULE.invoke_once("too-large", tiny)
        with self.assertRaisesRegex(AttributeError, "immutable"):
            tiny.max_input_bytes = 1024

    def test_descendant_is_removed_after_successful_phase_exit(self) -> None:
        result = MODULE.invoke_once("intent", self.policy(plan="spawn-descendant"))
        self.assertEqual(result["status"], "PASS")
        pid = int(self.pid_file.read_text(encoding="utf-8"))
        for _ in range(100):
            try:
                os.kill(pid, 0)
            except ProcessLookupError:
                break
            time.sleep(0.01)
        else:
            self.fail(f"phase descendant {pid} survived cleanup")


if __name__ == "__main__":
    unittest.main()
