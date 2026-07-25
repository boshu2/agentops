#!/usr/bin/env python3
"""Hostile process-tree, stream-bound, and isolation checks for the probe."""

from __future__ import annotations

import hashlib
from pathlib import Path
import sys
import unittest
from unittest import mock


REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPT_DIR = REPO_ROOT / "skills/skill-builder/scripts"
sys.path.insert(0, str(SCRIPT_DIR))

import probe_runtime  # noqa: E402
from probe_runtime import run_isolated_command  # noqa: E402


FIXTURE_ROOT = Path("skills/skill-builder/fixtures/contract-v3/probe-harnesses")


class ProbeRunnerTests(unittest.TestCase):
    def run_fixture(
        self,
        name: str,
        *,
        timeout_seconds: float = 5.0,
        retained_bytes: int = 4096,
    ) -> dict[str, object]:
        return run_isolated_command(
            REPO_ROOT,
            [sys.executable, (FIXTURE_ROOT / name).as_posix()],
            timeout_seconds=timeout_seconds,
            retained_bytes=retained_bytes,
            term_grace_seconds=0.5,
            kill_grace_seconds=1.0,
        )

    def test_streams_are_drained_and_retained_under_hard_bounds(self) -> None:
        outcome = self.run_fixture("large-output.py")
        execution = outcome["execution"]
        stdout = execution["stdout"]
        stderr = execution["stderr"]
        self.assertEqual(200_000, stdout["total_bytes"])
        self.assertEqual(180_000, stderr["total_bytes"])
        self.assertEqual(4096, stdout["retained_bytes"])
        self.assertEqual(4096, stderr["retained_bytes"])
        self.assertTrue(stdout["truncated"])
        self.assertTrue(stderr["truncated"])
        self.assertEqual(hashlib.sha256(b"O" * 200_000).hexdigest(), stdout["sha256"])
        self.assertEqual(hashlib.sha256(b"E" * 180_000).hexdigest(), stderr["sha256"])
        self.assertEqual(hashlib.sha256(b"O" * 4096).hexdigest(), stdout["retained_sha256"])
        self.assertEqual(hashlib.sha256(b"E" * 4096).hexdigest(), stderr["retained_sha256"])

    def test_timeout_terminates_and_reaps_the_whole_process_group(self) -> None:
        outcome = self.run_fixture("spawn-and-sleep.py", timeout_seconds=0.2)
        execution = outcome["execution"]
        cleanup = execution["cleanup"]
        self.assertTrue(execution["timed_out"])
        self.assertEqual("timeout", cleanup["trigger"])
        self.assertTrue(cleanup["term_sent"])
        self.assertTrue(cleanup["kill_sent"])
        self.assertTrue(cleanup["parent_reaped"])
        self.assertTrue(cleanup["process_group_empty"])
        self.assertTrue(cleanup["complete"])

    def test_successful_parent_with_live_descendant_is_not_silent(self) -> None:
        outcome = self.run_fixture("leave-descendant.py")
        execution = outcome["execution"]
        cleanup = execution["cleanup"]
        self.assertEqual(0, execution["exit_code"])
        self.assertEqual("descendants", cleanup["trigger"])
        self.assertTrue(cleanup["term_sent"])
        self.assertTrue(cleanup["parent_reaped"])
        self.assertTrue(cleanup["process_group_empty"])
        self.assertTrue(cleanup["complete"])

    def test_caller_interruption_cleans_the_process_group(self) -> None:
        real_sleep = probe_runtime.time.sleep
        interrupted = False

        def interrupt_once(seconds: float) -> None:
            nonlocal interrupted
            if not interrupted:
                interrupted = True
                raise KeyboardInterrupt
            real_sleep(seconds)

        with mock.patch.object(
            probe_runtime.time,
            "sleep",
            side_effect=interrupt_once,
        ):
            outcome = self.run_fixture("spawn-and-sleep.py")
        execution = outcome["execution"]
        cleanup = execution["cleanup"]
        self.assertTrue(execution["interrupted"])
        self.assertEqual("interrupted", cleanup["trigger"])
        self.assertTrue(cleanup["term_sent"])
        self.assertTrue(cleanup["parent_reaped"])
        self.assertTrue(cleanup["process_group_empty"])
        self.assertTrue(cleanup["complete"])

    def test_mutation_is_confined_to_disposable_copy_and_reported(self) -> None:
        live_target = REPO_ROOT / "skills/skill-builder/ISOLATION-MUTATION"
        self.assertFalse(live_target.exists())
        outcome = self.run_fixture("mutate-copy.py")
        isolation = outcome["isolation"]
        self.assertEqual(
            ["skills/skill-builder/ISOLATION-MUTATION"],
            isolation["changed_paths"],
        )
        self.assertEqual(isolation["changed_paths"], isolation["out_of_scope_paths"])
        self.assertTrue(isolation["live_root_unchanged"])
        self.assertEqual([], isolation["live_root_changed_paths"])
        self.assertFalse(live_target.exists())


if __name__ == "__main__":
    unittest.main()
