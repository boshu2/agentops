"""Unit + integration tests for scripts/ms-adoption-probe.py.

Uses vanilla unittest + importlib so it runs without pytest installed
(matches tests/python/run-tests.sh conventions).

Fixture fidelity: tests/python/fixtures/ms-adoption-sample.jsonl is built from
REAL Claude Code transcript-line envelopes (the message/content/tool_use/
timestamp shape, sessionId/uuid/requestId/version fields preserved) copied out
of live ~/.claude/projects sessions and redacted — never a hand-built shape.
The one Grep-tool line reuses a real assistant tool_use envelope with the
documented Grep input schema (pattern/path/glob). See age-50pr.
"""

from __future__ import annotations

import datetime as _dt
import importlib.util
import io
import json
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPT = REPO_ROOT / "scripts" / "ms-adoption-probe.py"
FIXTURE = Path(__file__).resolve().parent / "fixtures" / "ms-adoption-sample.jsonl"


def _load_module():
    spec = importlib.util.spec_from_file_location("ms_adoption_probe", SCRIPT)
    assert spec and spec.loader, f"cannot load {SCRIPT}"
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


class TestClassifyToolUse(unittest.TestCase):
    def setUp(self):
        self.m = _load_module()

    def test_mcp_ms_search_is_ms(self):
        self.assertEqual(self.m.classify_tool_use("mcp__ms__search", {"query": "x"}), "ms")

    def test_bash_ms_search_is_ms(self):
        self.assertEqual(
            self.m.classify_tool_use("Bash", {"command": 'ms search "flaky test"'}), "ms"
        )

    def test_bash_grep_over_skills_is_grep(self):
        self.assertEqual(
            self.m.classify_tool_use(
                "Bash", {"command": "grep -n foo skills/validate/SKILL.md"}
            ),
            "grep",
        )

    def test_bash_rg_over_skills_is_grep(self):
        self.assertEqual(
            self.m.classify_tool_use("Bash", {"command": "rg pattern skills/"}), "grep"
        )

    def test_bash_grep_not_over_skills_is_none(self):
        # grep but not a skills/ path -> not a skill hand-search
        self.assertIsNone(
            self.m.classify_tool_use("Bash", {"command": "grep -n foo cli/main.go"})
        )

    def test_bash_plain_command_is_none(self):
        self.assertIsNone(self.m.classify_tool_use("Bash", {"command": "ls -la"}))

    def test_grep_tool_targeting_skills_is_grep(self):
        self.assertEqual(
            self.m.classify_tool_use("Grep", {"pattern": "x", "path": "skills/"}), "grep"
        )

    def test_glob_tool_targeting_skills_is_grep(self):
        self.assertEqual(
            self.m.classify_tool_use("Glob", {"glob": "skills/**/SKILL.md"}), "grep"
        )

    def test_grep_tool_not_targeting_skills_is_none(self):
        self.assertIsNone(
            self.m.classify_tool_use("Grep", {"pattern": "x", "path": "cli/"})
        )

    def test_unrelated_tool_is_none(self):
        self.assertIsNone(self.m.classify_tool_use("Read", {"file_path": "skills/x"}))


class TestCountRecords(unittest.TestCase):
    def setUp(self):
        self.m = _load_module()
        self.assertTrue(FIXTURE.exists(), f"fixture missing: {FIXTURE}")
        self.records = [
            json.loads(line)
            for line in FIXTURE.read_text().splitlines()
            if line.strip()
        ]

    def test_fixture_shape_is_real(self):
        # Guard the fidelity assumption: every record carries the real envelope.
        self.assertEqual(len(self.records), 6)
        for r in self.records:
            self.assertIn("message", r)
            self.assertIn("timestamp", r)
            self.assertIsInstance(r["message"].get("content"), list)

    def test_counts_without_window_include_all(self):
        # No cutoff -> the out-of-window ms line is also counted.
        out = self.m.count_records(self.records, None)
        self.assertEqual(out, {"ms_calls": 3, "grep_calls": 2})

    def test_window_excludes_old_line(self):
        cutoff = _dt.datetime(2026, 6, 10, tzinfo=_dt.timezone.utc)
        out = self.m.count_records(self.records, cutoff)
        # 2 ms (mcp + bash) + 2 grep (bash + Grep-tool); the 2026-06-01 ms line drops.
        self.assertEqual(out, {"ms_calls": 2, "grep_calls": 2})


class TestRunProbeAndCli(unittest.TestCase):
    def setUp(self):
        self.m = _load_module()
        # Build a projects-dir layout: <root>/<project>/<session>.jsonl
        self.tmp = tempfile.TemporaryDirectory()
        proj = Path(self.tmp.name) / "-Users-x-dev-repo"
        proj.mkdir(parents=True)
        (proj / "session.jsonl").write_text(FIXTURE.read_text())

    def tearDown(self):
        self.tmp.cleanup()

    def test_run_probe_ratio(self):
        cutoff = _dt.datetime(2026, 6, 10, tzinfo=_dt.timezone.utc)
        out = self.m.run_probe(Path(self.tmp.name), cutoff)
        self.assertEqual(out["ms_calls"], 2)
        self.assertEqual(out["grep_calls"], 2)
        self.assertEqual(out["ratio"], 1.0)
        self.assertEqual(out["sessions_scanned"], 1)

    def test_ratio_none_when_no_grep(self):
        # Empty projects dir -> zero counts, ratio None (divide-by-zero guard).
        empty = tempfile.TemporaryDirectory()
        self.addCleanup(empty.cleanup)
        out = self.m.run_probe(Path(empty.name), None)
        self.assertEqual(out["ms_calls"], 0)
        self.assertEqual(out["grep_calls"], 0)
        self.assertIsNone(out["ratio"])

    def test_cli_emits_json(self):
        buf = io.StringIO()
        with redirect_stdout(buf):
            rc = self.m.main(
                ["--projects-dir", self.tmp.name, "--since", "2026-06-10"]
            )
        self.assertEqual(rc, 0)
        payload = json.loads(buf.getvalue())
        self.assertIn("window", payload)
        self.assertEqual(payload["ms_calls"], 2)
        self.assertEqual(payload["grep_calls"], 2)
        self.assertEqual(payload["ratio"], 1.0)
        self.assertEqual(payload["window"]["since"], "2026-06-10T00:00:00+00:00")


if __name__ == "__main__":
    unittest.main()
