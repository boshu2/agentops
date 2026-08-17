#!/usr/bin/env python3

from __future__ import annotations

import hashlib
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


SKILL_DIR = Path(__file__).resolve().parents[1]
SCRIPT = SKILL_DIR / "scripts" / "recent_human.py"


def record(timestamp: str, message: str, client_id: str | None, *, payload_type: str = "user_message") -> dict[str, object]:
    return {
        "timestamp": timestamp,
        "type": "event_msg",
        "payload": {"type": payload_type, "client_id": client_id, "message": message},
    }


class RecentHumanCliTest(unittest.TestCase):
    def run_cli(
        self,
        root: Path,
        *paths: str,
        since: str = "2026-07-14T10:00:00Z",
        until: str = "2026-07-14T11:00:00Z",
        max_files: int = 8,
        max_file_bytes: int = 100_000,
        max_line_bytes: int = 50_000,
        max_messages: int = 100,
        max_text_chars: int = 256,
    ) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [
                sys.executable,
                str(SCRIPT),
                "--since", since,
                "--until", until,
                "--input-root", str(root),
                "--max-files", str(max_files),
                "--max-file-bytes", str(max_file_bytes),
                "--max-line-bytes", str(max_line_bytes),
                "--max-messages", str(max_messages),
                "--max-text-chars", str(max_text_chars),
                *paths,
            ],
            check=False,
            capture_output=True,
            text=True,
        )

    @staticmethod
    def write_jsonl(path: Path, values: list[object | str]) -> None:
        with path.open("w", encoding="utf-8") as handle:
            for value in values:
                handle.write(value if isinstance(value, str) else json.dumps(value))
                handle.write("\n")

    @staticmethod
    def source_id(relative: str) -> str:
        return "source-" + hashlib.sha256(relative.encode()).hexdigest()[:16]

    def test_extracts_redacts_excludes_and_deduplicates_without_absolute_paths(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            self.write_jsonl(root / "a.jsonl", [
                {"timestamp": "2026-07-14T09:59:00Z", "type": "session_meta"},
                record("2026-07-14T10:05:00Z", "# My request for Codex:\nDeploy /private/work/app with api_key=super-secret", "client-a"),
                record("2026-07-14T10:06:00Z", "Keep this twice", "client-duplicate"),
                record("2026-07-14T10:07:00Z", "You are a fresh-context, cross-family reviewer — review this.", "generated"),
                record("2026-07-14T10:08:00Z", "No id", None),
                "{bad-json",
            ])
            self.write_jsonl(root / "b.jsonl", [
                record("2026-07-14T10:06:30Z", "Keep this twice", "client-duplicate"),
                record("2026-07-14T10:00:00Z", "At since", "client-since"),
                record("2026-07-14T11:00:00Z", "At until", "client-until"),
            ])

            completed = self.run_cli(root, "b.jsonl", "a.jsonl")
            self.assertEqual(completed.returncode, 0, completed.stderr)
            self.assertNotIn(str(root), completed.stdout)
            self.assertNotIn("super-secret", completed.stdout)
            self.assertNotIn("/private/work/app", completed.stdout)
            result = json.loads(completed.stdout)
            self.assertEqual(result["inputs"], [
                {"source_id": self.source_id("a.jsonl"), "relative_path": "a.jsonl"},
                {"source_id": self.source_id("b.jsonl"), "relative_path": "b.jsonl"},
            ])
            self.assertEqual([row["redacted_text"] for row in result["messages"]], [
                "At since",
                "Deploy [REDACTED-PATH] with api_key=[REDACTED]",
                "Keep this twice",
            ])
            self.assertTrue(all("text" not in row for row in result["messages"]))
            self.assertEqual(result["counts"]["excluded"]["duplicate_client_id"], 1)
            self.assertEqual(result["counts"]["excluded"]["missing_client_id"], 1)
            self.assertEqual(result["counts"]["excluded"]["machine_echo"], 1)

    def test_output_is_independent_of_input_order(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            self.write_jsonl(root / "a.jsonl", [record("2026-07-14T10:00:00Z", "First", "a")])
            self.write_jsonl(root / "b.jsonl", [record("2026-07-14T10:01:00Z", "Second", "b")])
            left = self.run_cli(root, "a.jsonl", "b.jsonl")
            right = self.run_cli(root, "b.jsonl", "a.jsonl")
            self.assertEqual(left.returncode, 0, left.stderr)
            self.assertEqual(left.stdout, right.stdout)

    def test_behavior_contract_matrix_rejects_indirect_unapproved_inputs(self) -> None:
        with tempfile.TemporaryDirectory() as directory, tempfile.TemporaryDirectory() as outside_dir:
            root = Path(directory).resolve()
            outside = Path(outside_dir).resolve() / "session.jsonl"
            self.write_jsonl(outside, [record("2026-07-14T10:00:00Z", "outside", "id")])
            (root / "linked.jsonl").symlink_to(outside)
            for supplied in (str(outside), "../session.jsonl", "linked.jsonl"):
                with self.subTest(supplied=supplied):
                    completed = self.run_cli(root, supplied)
                    self.assertEqual(completed.returncode, 2)
                    self.assertEqual(completed.stdout, "")

    def test_oversized_file_line_message_count_and_excerpt_are_bounded(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            self.write_jsonl(root / "large.jsonl", [record("2026-07-14T10:00:00Z", "word " * 300, "one")])
            for kwargs, needle in (
                ({"max_file_bytes": 20}, "max-file-bytes"),
                ({"max_line_bytes": 20}, "max_line_bytes"),
            ):
                with self.subTest(kwargs=kwargs):
                    completed = self.run_cli(root, "large.jsonl", **kwargs)
                    self.assertEqual(completed.returncode, 2)
                    self.assertIn(needle, completed.stderr)
                    self.assertEqual(completed.stdout, "")
            completed = self.run_cli(root, "large.jsonl", max_text_chars=32)
            self.assertEqual(completed.returncode, 0, completed.stderr)
            message = json.loads(completed.stdout)["messages"][0]["redacted_text"]
            self.assertTrue(message.endswith("…[TRUNCATED]"))
            self.write_jsonl(root / "two.jsonl", [
                record("2026-07-14T10:00:00Z", "one", "one"),
                record("2026-07-14T10:01:00Z", "two", "two"),
            ])
            completed = self.run_cli(root, "two.jsonl", max_messages=1)
            self.assertEqual(completed.returncode, 2)
            self.assertEqual(completed.stdout, "")

    def test_requires_timezone_ordered_window_and_explicit_finite_bounds(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            self.write_jsonl(root / "empty.jsonl", [])
            no_timezone = self.run_cli(root, "empty.jsonl", since="2026-07-14T10:00:00")
            self.assertEqual(no_timezone.returncode, 2)
            reversed_window = self.run_cli(root, "empty.jsonl", since="2026-07-14T11:00:00Z", until="2026-07-14T10:00:00Z")
            self.assertEqual(reversed_window.returncode, 2)
            unbounded = self.run_cli(root, "empty.jsonl", max_messages=10001)
            self.assertEqual(unbounded.returncode, 2)


if __name__ == "__main__":
    unittest.main()
