from __future__ import annotations

import importlib.util
import json
import os
from pathlib import Path
import tempfile
import time
import unittest


MODULE_PATH = Path(__file__).parents[1] / "scripts" / "dispatch_once.py"
SPEC = importlib.util.spec_from_file_location("dispatch_once", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)

HELPER = r'''#!/usr/bin/env python3
import json, subprocess, sys, time
mode, log_path, pid_path = sys.argv[1:4]
packet = json.load(sys.stdin)
if mode == "hang": time.sleep(30)
if mode == "large":
    print(json.dumps("x" * 100000)); raise SystemExit(0)
if mode == "fail-secret":
    print("api_key=super-secret", file=sys.stderr); raise SystemExit(7)
if mode == "descendant":
    child = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(30)"])
    open(pid_path, "w", encoding="utf-8").write(str(child.pid))
with open(log_path, "a", encoding="utf-8") as log: log.write(packet["packet_id"] + "\n")
print(json.dumps({"candidate": packet["packet_id"]}))
'''


def packet(identity: str, *paths: str, payload: str | None = None) -> dict:
    value = {"packet_id": identity, "write_scope": {"include": list(paths), "exclude": []}}
    if payload is not None:
        value["payload"] = payload
    return value


class DispatchOnceTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name).resolve()
        self.helper = self.root / "executor.py"
        self.helper.write_text(HELPER, encoding="utf-8")
        self.log = self.root / "calls.log"
        self.pid_file = self.root / "pid"
        self.python = str(Path(os.sys.executable).resolve(strict=True))

    def tearDown(self) -> None:
        self.temp.cleanup()

    def policy(
        self,
        mode: str = "ok",
        *,
        max_input: int = 16_384,
        max_output: int = 16_384,
        max_packets: int = 8,
    ):
        return MODULE.ExecutorPolicy(
            workspace_root=self.root,
            argv=[self.python, str(self.helper), mode, str(self.log), str(self.pid_file)],
            timeout_seconds=0.35,
            max_input_bytes=max_input,
            max_output_bytes=max_output,
            max_packets=max_packets,
        )

    def calls(self) -> list[str]:
        return self.log.read_text(encoding="utf-8").splitlines() if self.log.exists() else []

    def test_authorized_executor_runs_each_packet_once_in_order(self) -> None:
        result = MODULE.dispatch_once(
            [packet("a", "src/a"), packet("b", "src/b")], self.policy()
        )
        self.assertEqual(self.calls(), ["a", "b"])
        self.assertEqual(result, [
            {"packet_id": "a", "result": {"candidate": "a"}},
            {"packet_id": "b", "result": {"candidate": "b"}},
        ])

    def test_behavior_contract_matrix_rejects_callback_incomplete_and_overlap(self) -> None:
        cases = (
            ("callback", lambda: MODULE.dispatch_once([packet("a", "src/a")], lambda _packet: None), TypeError),
            (
                "overlap",
                lambda: MODULE.dispatch_once([packet("a", "src"), packet("b", "src/b")], self.policy()),
                ValueError,
            ),
            (
                "exclude",
                lambda: MODULE.dispatch_once([
                    {"packet_id": "a", "write_scope": {"include": ["src/a"], "exclude": ["src/a/x"]}}
                ], self.policy()),
                ValueError,
            ),
        )
        for name, invoke, error in cases:
            with self.subTest(name=name):
                with self.assertRaises(error):
                    invoke()
                self.assertEqual(self.calls(), [])

    def test_case_glob_and_noncanonical_collisions_fail_before_launch(self) -> None:
        bad_batches = (
            [packet("a", "src/A"), packet("b", "src/a")],
            [packet("a", "src/*/generated"), packet("b", "src/*/manual")],
            [packet("a", "src/./a")],
            [packet("a", "../outside")],
        )
        for batch in bad_batches:
            with self.subTest(batch=batch):
                with self.assertRaises(ValueError):
                    MODULE.dispatch_once(batch, self.policy())
                self.assertEqual(self.calls(), [])

    def test_symlinked_scope_and_unapproved_executor_path_fail_closed(self) -> None:
        outside = self.root.parent / f"outside-{self.root.name}"
        outside.mkdir(exist_ok=True)
        try:
            (self.root / "linked").symlink_to(outside, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "symlink"):
                MODULE.dispatch_once([packet("a", "linked/file")], self.policy())
            executable_link = self.root / "python-link"
            executable_link.symlink_to(self.python)
            with self.assertRaisesRegex(ValueError, "symlink"):
                MODULE.ExecutorPolicy(
                    workspace_root=self.root,
                    argv=[str(executable_link), str(self.helper), "ok", str(self.log), str(self.pid_file)],
                    timeout_seconds=1,
                    max_input_bytes=1024,
                    max_output_bytes=1024,
                    max_packets=1,
                )
            self.assertEqual(self.calls(), [])
        finally:
            outside.rmdir()

    def test_hung_oversized_secret_and_batch_bounds_are_factual_errors(self) -> None:
        hung = MODULE.dispatch_once([packet("a", "src/a")], self.policy("hang"))
        self.assertEqual(hung[0]["error"]["type"], "TimeoutError")
        large = MODULE.dispatch_once([packet("b", "src/b")], self.policy("large", max_output=64))
        self.assertEqual(large[0]["error"]["type"], "ValueError")
        secret = MODULE.dispatch_once([packet("c", "src/c")], self.policy("fail-secret"))
        self.assertEqual(secret[0]["error"]["type"], "RuntimeError")
        self.assertNotIn("super-secret", json.dumps(secret))
        with self.assertRaisesRegex(ValueError, "max_packets"):
            MODULE.dispatch_once([packet("a", "a"), packet("b", "b")], self.policy(max_packets=1))
        with self.assertRaisesRegex(ValueError, "max_input_bytes"):
            MODULE.dispatch_once([packet("x", "x", payload="y" * 1000)], self.policy(max_input=100))

    def test_descendant_is_cleaned_after_executor_returns(self) -> None:
        result = MODULE.dispatch_once([packet("a", "src/a")], self.policy("descendant"))
        self.assertIn("result", result[0])
        pid = int(self.pid_file.read_text(encoding="utf-8"))
        for _ in range(100):
            try:
                os.kill(pid, 0)
            except ProcessLookupError:
                break
            time.sleep(0.01)
        else:
            self.fail(f"executor descendant {pid} survived cleanup")


if __name__ == "__main__":
    unittest.main()
