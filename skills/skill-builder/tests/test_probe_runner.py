#!/usr/bin/env python3
"""Hostile process-tree, stream-bound, and isolation checks for the probe."""

from __future__ import annotations

import hashlib
import os
from pathlib import Path
import signal
import sys
import tempfile
import threading
import unittest
from unittest import mock


REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPT_DIR = REPO_ROOT / "skills/skill-builder/scripts"
sys.path.insert(0, str(SCRIPT_DIR))

import probe_runtime  # noqa: E402
import run_contract_probe  # noqa: E402
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

    def test_actual_termination_signals_clean_the_process_group(self) -> None:
        handled_signals = [signal.SIGINT, signal.SIGTERM]
        if hasattr(signal, "SIGHUP"):
            handled_signals.append(signal.SIGHUP)
        for handled_signal in handled_signals:
            with self.subTest(signal=handled_signal):
                timer = threading.Timer(
                    0.2,
                    lambda value=handled_signal: os.kill(os.getpid(), value),
                )
                timer.start()
                try:
                    execution, _, _ = probe_runtime._execute(
                        [
                            sys.executable,
                            (FIXTURE_ROOT / "spawn-and-sleep.py").as_posix(),
                        ],
                        cwd=REPO_ROOT,
                        environment=os.environ.copy(),
                        timeout_seconds=5.0,
                        retained_bytes=4096,
                        term_grace_seconds=0.5,
                        kill_grace_seconds=1.0,
                    )
                finally:
                    timer.cancel()
                    timer.join()
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

    def test_creation_under_input_excluded_path_is_still_reported(self) -> None:
        code = (
            "from pathlib import Path; "
            "path=Path('skills/skill-builder/ledgers/proof-created.json'); "
            "path.parent.mkdir(parents=True, exist_ok=True); "
            "path.write_text('{}\\n', encoding='utf-8')"
        )
        outcome = run_isolated_command(
            REPO_ROOT,
            [sys.executable, "-c", code],
            timeout_seconds=5.0,
            retained_bytes=4096,
        )
        self.assertEqual(0, outcome["execution"]["exit_code"])
        self.assertEqual(
            [
                "skills/skill-builder/ledgers",
                "skills/skill-builder/ledgers/proof-created.json",
            ],
            outcome["isolation"]["changed_paths"],
        )
        self.assertEqual(
            outcome["isolation"]["changed_paths"],
            outcome["isolation"]["out_of_scope_paths"],
        )

    def test_absolute_symlink_into_live_root_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory(prefix="probe-symlink-") as temporary:
            root = Path(temporary)
            target = root / "target"
            target.write_text("live\n", encoding="utf-8")
            (root / "escape").symlink_to(target.resolve())
            with self.assertRaisesRegex(ValueError, "absolute"):
                probe_runtime._validate_symlinks(root)

    def test_child_environment_contains_no_live_python_path(self) -> None:
        with tempfile.TemporaryDirectory(prefix="probe-environment-") as temporary:
            temporary_root = Path(temporary)
            isolated_root = temporary_root / "repo"
            isolated_root.mkdir()
            home = temporary_root / "home"
            tmpdir = temporary_root / "tmp"
            home.mkdir()
            tmpdir.mkdir()
            with mock.patch.dict(
                os.environ,
                {"PYTHONPATH": str(REPO_ROOT / "skills/skill-builder/scripts")},
            ):
                environment = probe_runtime._isolated_environment(
                    REPO_ROOT,
                    isolated_root,
                    home,
                    tmpdir,
                )
        self.assertNotIn(str(REPO_ROOT), environment.get("PYTHONPATH", ""))
        self.assertEqual(str(isolated_root), environment["PWD"])

    def test_prepare_callback_uses_the_disposable_snapshot(self) -> None:
        observed: dict[str, str] = {}

        def prepare(snapshot: Path) -> tuple[list[str], dict[str, str]]:
            observed["snapshot"] = str(snapshot)
            return [sys.executable, "-c", "pass"], {"root": str(snapshot)}

        outcome = run_isolated_command(
            REPO_ROOT,
            None,
            prepare=prepare,
            timeout_seconds=5.0,
            retained_bytes=4096,
        )
        self.assertEqual(0, outcome["execution"]["exit_code"])
        self.assertEqual(observed["snapshot"], outcome["preparation"]["root"])
        self.assertNotEqual(str(REPO_ROOT), observed["snapshot"])

    def test_probe_compiles_executes_and_receipts_the_same_snapshot(self) -> None:
        snapshot = Path("/disposable/repository-snapshot")
        source_digest = "a" * 64
        runner_identity = {
            "files": [
                {"ref": ref, "sha256": str(index) * 64}
                for index, ref in enumerate(
                    run_contract_probe.RUNNER_REFS,
                    start=1,
                )
            ],
            "digest": "f" * 64,
        }
        compiled = {
            "source": {
                "ref": "skills/skill-builder/SKILL.md",
                "before_sha256": source_digest,
            },
            "contract": {"digest": "b" * 64},
            "compiler": {"digest": "c" * 64},
            "proof": {"command": "python3 snapshot-proof.py"},
        }
        observed: dict[str, object] = {}

        def compile_snapshot(root: Path, skill_name: str) -> dict[str, object]:
            observed["compiled_root"] = root
            observed["skill_name"] = skill_name
            return compiled

        def run_snapshot(
            root: Path,
            command: list[str] | None,
            *,
            prepare: object,
        ) -> dict[str, object]:
            observed["run_root"] = root
            observed["initial_command"] = command
            assert callable(prepare)
            prepared_command, preparation = prepare(snapshot)
            observed["prepared_command"] = prepared_command
            return {
                "execution": {
                    "exit_code": 0,
                    "timed_out": False,
                    "interrupted": False,
                    "cleanup": {"complete": True, "trigger": "none"},
                },
                "isolation": {
                    "live_root_unchanged": True,
                    "live_root_changed_paths": [],
                    "out_of_scope_paths": [],
                },
                "preparation": preparation,
            }

        with (
            mock.patch.object(
                run_contract_probe,
                "compile_skill",
                side_effect=compile_snapshot,
            ),
            mock.patch.object(
                run_contract_probe,
                "file_set_identity",
                return_value=runner_identity,
            ),
            mock.patch.object(
                run_contract_probe,
                "file_sha256",
                return_value=source_digest,
            ),
            mock.patch.object(
                run_contract_probe,
                "run_isolated_command",
                side_effect=run_snapshot,
            ),
        ):
            receipt = run_contract_probe.run_probe(REPO_ROOT, "skill-builder")

        self.assertEqual(snapshot, observed["compiled_root"])
        self.assertEqual(REPO_ROOT, observed["run_root"])
        self.assertIsNone(observed["initial_command"])
        self.assertEqual(
            ["python3", "snapshot-proof.py"],
            observed["prepared_command"],
        )
        self.assertEqual(runner_identity, receipt["runner"])
        self.assertEqual(compiled["proof"], receipt["proof"])
        self.assertEqual("PASS", receipt["result"])


if __name__ == "__main__":
    unittest.main()
