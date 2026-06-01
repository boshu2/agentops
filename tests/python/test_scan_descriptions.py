"""Unit + integration tests for skills/skill-builder/scripts/scan_descriptions.py.

Uses vanilla unittest + importlib so it runs without pytest installed.
Focus: the three trigger-detection forms (matching audit.sh), suggestion
generation, and the CLI entry point's exit codes.
"""

from __future__ import annotations

import importlib.util
import io
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPT = REPO_ROOT / "skills" / "skill-builder" / "scripts" / "scan_descriptions.py"


def _load_module():
    """Load the scanner module by path (no package install required)."""
    spec = importlib.util.spec_from_file_location("scan_descriptions", SCRIPT)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    # Register before exec so dataclass introspection can resolve the module
    # (required on Python 3.14+ for importlib-loaded modules with dataclasses).
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


scan = _load_module()


def _write_skill(root: Path, name: str, frontmatter: str, body: str = "# Title\n") -> Path:
    """Create root/<name>/SKILL.md with the given frontmatter + body."""
    skill_dir = root / name
    skill_dir.mkdir(parents=True)
    md = skill_dir / "SKILL.md"
    md.write_text(f"---\n{frontmatter}\n---\n{body}", encoding="utf-8")
    return md


class TestTriggerDetection(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.root = Path(self._tmp.name)

    def tearDown(self):
        self._tmp.cleanup()

    def test_explicit_marker_in_description_detected(self):
        md = _write_skill(
            self.root,
            "alpha",
            'name: alpha\ndescription: Does a thing. Triggers: "do thing", "alpha".',
        )
        result = scan.scan_skill(md)
        self.assertTrue(result.has_trigger)
        self.assertIn("explicit-marker", result.forms)

    def test_block_scalar_use_when_detected(self):
        md = _write_skill(
            self.root,
            "beta",
            "name: beta\ndescription: |\n  Does a thing.\n  **Use when:** you need a thing.",
        )
        result = scan.scan_skill(md)
        self.assertTrue(result.has_trigger)
        self.assertIn("block-scalar", result.forms)

    def test_triggers_list_with_three_items_detected(self):
        md = _write_skill(
            self.root,
            "gamma",
            "name: gamma\ndescription: Does a thing.\nmetadata:\n  triggers:\n"
            "    - one\n    - two\n    - three",
        )
        result = scan.scan_skill(md)
        self.assertTrue(result.has_trigger)
        self.assertIn("triggers-list", result.forms)

    def test_two_item_triggers_list_not_enough(self):
        md = _write_skill(
            self.root,
            "delta",
            "name: delta\ndescription: Does a thing.\nmetadata:\n  triggers:\n    - one\n    - two",
        )
        result = scan.scan_skill(md)
        self.assertFalse(result.has_trigger)

    def test_plain_description_missing_trigger_gets_suggestion(self):
        md = _write_skill(
            self.root,
            "scope-creep",
            "name: scope-creep\ndescription: Audits the scope of a change.",
        )
        result = scan.scan_skill(md)
        self.assertFalse(result.has_trigger)
        self.assertEqual(result.forms, [])
        self.assertTrue(result.suggestion.startswith("Triggers:"))
        self.assertIn('"scope-creep"', result.suggestion)

    def test_suggestion_has_no_duplicate_words(self):
        md = _write_skill(
            self.root, "compile", "name: compile\ndescription: Compile the corpus."
        )
        result = scan.scan_skill(md)
        # "compile compile" must not appear — verb equals the only name token.
        self.assertNotIn("compile compile", result.suggestion)


class TestCli(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.root = Path(self._tmp.name)

    def tearDown(self):
        self._tmp.cleanup()

    def test_strict_exits_one_when_a_skill_lacks_trigger(self):
        _write_skill(self.root, "no-trig", "name: no-trig\ndescription: Plain description.")
        with redirect_stdout(io.StringIO()):
            code = scan.main([str(self.root), "--strict", "--quiet"])
        self.assertEqual(code, 1)

    def test_strict_exits_zero_when_all_have_triggers(self):
        _write_skill(
            self.root,
            "ok",
            'name: ok\ndescription: Does X. Triggers: "ok", "do x".',
        )
        with redirect_stdout(io.StringIO()):
            code = scan.main([str(self.root), "--strict", "--quiet"])
        self.assertEqual(code, 0)

    def test_missing_dir_exits_two(self):
        code = scan.main([str(self.root / "does-not-exist")])
        self.assertEqual(code, 2)

    def test_json_output_reports_counts(self):
        _write_skill(self.root, "a", "name: a\ndescription: Plain.")
        _write_skill(self.root, "b", 'name: b\ndescription: X. Triggers: "b", "x".')
        buf = io.StringIO()
        with redirect_stdout(buf):
            scan.main([str(self.root), "--json"])
        import json

        payload = json.loads(buf.getvalue())
        self.assertEqual(payload["scanned"], 2)
        self.assertEqual(payload["missing"], 1)


class TestRealCorpus(unittest.TestCase):
    def test_scans_the_live_skills_directory(self):
        skills_dir = REPO_ROOT / "skills"
        results = scan.scan_corpus(skills_dir)
        self.assertGreater(len(results), 0)
        # Every result must carry the name of its directory or frontmatter.
        self.assertTrue(all(r.name for r in results))


if __name__ == "__main__":
    unittest.main()
