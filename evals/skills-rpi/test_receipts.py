"""Comparison integrity tests; no model, container or network calls."""
import json
from pathlib import Path
import tempfile
import unittest
from prepare import sha, tree_hash, write_json
from receipts import collect


class ReceiptIntegrity(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        for directory in ("task", "skills"):
            (self.root / directory).mkdir()
            (self.root / directory / "content.md").write_text(directory)
        native = self.root / "jobs" / "repair-control-1"
        native.mkdir(parents=True)
        self.config = {"job_name": "repair-control-1", "jobs_dir": str(self.root / "jobs"),
                       "n_concurrent_trials": 1, "agents": [{"name": "codex", "model_name": "openai/gpt-6-astra"}]}
        write_json(self.root / "input.json", self.config)
        write_json(native / "config.json", self.config)
        self.native = native / "config.json"
        self.manifest = self.root / "staged.json"
        write_json(self.manifest, {"schema_version": 1, "task": "agentops/repair",
            "task_checksum": tree_hash(self.root / "task"), "oracle_sha256": "f" * 64,
            "skills_sha256": tree_hash(self.root / "skills"), "runtime": {}, "isolation": {},
            "jobs": [{"job_name": "repair-control-1", "arm": "control", "rep": 1,
                      "input_config": str(self.root / "input.json"), "input_sha256": sha(self.root / "input.json")}]})

    def test_native_defaults_and_pending_jobs_remain_honest(self):
        self.assertEqual(collect([self.manifest])["trials"][0]["configuration_sha256"], sha(self.native))
        self.native.unlink()
        result = collect([self.manifest])
        self.assertEqual(result["trials"], [])
        self.assertEqual(len(result["diagnostics"]), 1)

    def test_extra_worker_environment_is_rejected(self):
        altered = json.loads(self.native.read_text())
        altered["agents"][0]["env"] = {"INJECT_DIFFERENT_POLICY": "yes"}
        write_json(self.native, altered)
        with self.assertRaisesRegex(ValueError, "differs"):
            collect([self.manifest])

    def test_changed_model_cannot_keep_receipt(self):
        altered = json.loads(self.native.read_text())
        altered["agents"][0]["model_name"] = "openai/different-model"
        write_json(self.native, altered)
        with self.assertRaisesRegex(ValueError, "differs"):
            collect([self.manifest])

    def test_changed_task_or_package_cannot_keep_receipt(self):
        for directory in ("task", "skills"):
            path = self.root / directory / "content.md"
            original = path.read_text()
            path.write_text("changed")
            with self.assertRaisesRegex(ValueError, "changed after freeze"):
                collect([self.manifest])
            path.write_text(original)

    def test_source_symlink_is_not_followed_into_private_content(self):
        (self.root / "task" / "link").symlink_to(self.root / "input.json")
        with self.assertRaisesRegex(ValueError, "symlink"):
            tree_hash(self.root / "task")

    def test_extra_instructions_are_not_an_ignored_default(self):
        altered = dict(self.config, extra_instructions=["different acceptance"])
        write_json(self.native, altered)
        with self.assertRaisesRegex(ValueError, "differs"):
            collect([self.manifest])


if __name__ == "__main__":
    unittest.main()
