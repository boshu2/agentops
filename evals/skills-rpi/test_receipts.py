"""Comparison integrity tests; no model, container or network calls."""
import json
import io
import tarfile
from pathlib import Path
import tempfile
import unittest
from prepare import extract_cli, sha, tree_hash, write_json
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


class ArchiveInputs(unittest.TestCase):
    def archive(self, name, kind=tarfile.REGTYPE):
        raw = io.BytesIO()
        with tarfile.open(fileobj=raw, mode="w") as tar:
            entry = tarfile.TarInfo(name)
            entry.type = kind
            entry.linkname = "/outside" if kind in (tarfile.SYMTYPE, tarfile.LNKTYPE) else ""
            entry.mode = 0o755
            entry.size = 3 if kind == tarfile.REGTYPE else 0
            tar.addfile(entry, io.BytesIO(b"src") if entry.size else None)
        raw.seek(0)
        return raw

    def test_only_regular_source_is_imported(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            extract_cli(self.archive("cli/check.sh"), root)
            self.assertEqual((root / "cli/check.sh").read_bytes(), b"src")
            self.assertEqual((root / "cli/check.sh").stat().st_mode & 0o777, 0o755)

    def test_paths_and_links_cannot_escape(self):
        for name, kind in (("../outside", tarfile.REGTYPE), ("/cli/outside", tarfile.REGTYPE),
                           ("cli/../outside", tarfile.REGTYPE), ("other/file", tarfile.REGTYPE),
                           ("cli/link", tarfile.SYMTYPE), ("cli/link", tarfile.LNKTYPE),
                           ("cli/device", tarfile.CHRTYPE)):
            with self.subTest(name=name, kind=kind), tempfile.TemporaryDirectory() as tmp:
                with self.assertRaises(ValueError):
                    extract_cli(self.archive(name, kind), Path(tmp))

    def test_existing_directory_link_cannot_redirect_import(self):
        with tempfile.TemporaryDirectory() as tmp, tempfile.TemporaryDirectory() as outside:
            root = Path(tmp)
            (root / "cli").symlink_to(outside)
            with self.assertRaises(ValueError):
                extract_cli(self.archive("cli/file"), root)
            self.assertEqual(list(Path(outside).iterdir()), [])


if __name__ == "__main__":
    unittest.main()
