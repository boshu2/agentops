from __future__ import annotations

import json
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest


REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPT = REPO_ROOT / "scripts" / "sync-gc-pack.py"


class SyncGCPackTests(unittest.TestCase):
    def setUp(self) -> None:
        self.tempdir = tempfile.TemporaryDirectory()
        self.addCleanup(self.tempdir.cleanup)
        self.root = Path(self.tempdir.name)
        for skill in ("implement", "validate", "using-gc"):
            source = self.root / "skills" / skill
            source.mkdir(parents=True)
            (source / "SKILL.md").write_text(f"# {skill}\n", encoding="utf-8")
            (source / "prompt.md").write_text("codex prompt\n", encoding="utf-8")
            (source / ".agentops-generated.json").write_text("{}\n", encoding="utf-8")

        implement_script = self.root / "skills" / "implement" / "scripts" / "validate.sh"
        implement_script.parent.mkdir()
        implement_script.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
        implement_script.chmod(0o755)

        validate_cache = self.root / "skills" / "validate" / "scripts" / "__pycache__"
        validate_cache.mkdir(parents=True)
        (validate_cache / "validate.cpython-314.pyc").write_bytes(b"cache")
        (self.root / "skills" / "validate" / "scripts" / "helper.pyc").write_bytes(b"cache")

    def run_sync(self, *args: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, str(SCRIPT), "--repo-root", str(self.root), *args],
            check=False,
            capture_output=True,
            text=True,
        )

    def destination(self, skill: str, provider: str = "codex") -> Path:
        suffix = "" if provider == "codex" else "-claude"
        if skill == "implement":
            return self.root / "packs" / "agentops-executor" / "agents" / f"implementer{suffix}" / "skills" / skill
        if skill == "validate":
            return self.root / "packs" / "agentops-executor" / "agents" / f"validator{suffix}" / "skills" / skill
        return self.root / "packs" / "agentops-executor" / "skills" / skill

    def snapshot(self) -> dict[str, tuple[bytes, int, int]]:
        return {
            path.relative_to(self.root).as_posix(): (
                path.read_bytes(),
                stat.S_IMODE(path.stat().st_mode),
                path.stat().st_mtime_ns,
            )
            for path in sorted(self.root.rglob("*"))
            if path.is_file()
        }

    def test_regen_projects_runtime_payload_and_stable_manifest(self) -> None:
        result = self.run_sync()
        self.assertEqual(result.returncode, 0, result.stderr)

        for skill in ("implement", "validate", "using-gc"):
            providers = {
                "implement": ("codex", "claude"),
                "validate": ("codex",),
                "using-gc": ("codex",),
            }[skill]
            for provider in providers:
                destination = self.destination(skill, provider)
                self.assertEqual((destination / "SKILL.md").read_text(encoding="utf-8"), f"# {skill}\n")
                self.assertFalse((destination / "prompt.md").exists())
                self.assertFalse((destination / ".agentops-generated.json").exists())
        self.assertFalse((self.destination("validate") / "scripts" / "__pycache__").exists())
        self.assertFalse((self.destination("validate") / "scripts" / "helper.pyc").exists())
        self.assertTrue((self.destination("implement") / "scripts" / "validate.sh").stat().st_mode & stat.S_IXUSR)
        self.assertTrue((self.destination("implement", "claude") / "scripts" / "validate.sh").stat().st_mode & stat.S_IXUSR)

        manifest_path = self.root / "packs" / "agentops-executor" / "assets" / "generated-skill-manifest.json"
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        self.assertEqual(manifest["schema_version"], "agentops.gc-skill-projection.v1")
        self.assertEqual(manifest["source"], "skills")
        self.assertEqual(
            [row["destination"] for row in manifest["files"]],
            sorted(row["destination"] for row in manifest["files"]),
        )
        for row in manifest["files"]:
            self.assertIn("source", row)
            self.assertIn("destination", row)
            self.assertRegex(row["source_sha256"], r"^[a-f0-9]{64}$")
            self.assertEqual(row["source_sha256"], row["sha256"])

        first = manifest_path.read_bytes()
        result = self.run_sync()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(manifest_path.read_bytes(), first)

    def test_check_detects_drift_without_mutating_any_file(self) -> None:
        self.assertEqual(self.run_sync().returncode, 0)
        drifted = self.destination("implement") / "SKILL.md"
        drifted.write_text("manual drift\n", encoding="utf-8")
        retired = self.destination("validate", "claude") / "stale.txt"
        retired.parent.mkdir(parents=True)
        retired.write_text("retired\n", encoding="utf-8")
        before = self.snapshot()

        result = self.run_sync("--check")

        self.assertEqual(result.returncode, 1)
        self.assertIn("implement", result.stderr)
        self.assertIn("retired projection", result.stderr)
        self.assertEqual(self.snapshot(), before)

    def test_regen_exact_deletes_stale_destination_files(self) -> None:
        self.assertEqual(self.run_sync().returncode, 0)
        stale = self.destination("validate") / "stale.txt"
        stale.write_text("stale\n", encoding="utf-8")
        retired = self.destination("validate", "claude") / "stale.txt"
        retired.parent.mkdir(parents=True)
        retired.write_text("retired\n", encoding="utf-8")

        result = self.run_sync()

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(stale.exists())
        self.assertFalse(retired.parent.exists())

    def test_check_on_missing_projection_is_nonmutating(self) -> None:
        before = self.snapshot()

        result = self.run_sync("--check")

        self.assertEqual(result.returncode, 1)
        self.assertIn("missing", result.stderr)
        self.assertEqual(self.snapshot(), before)
        self.assertFalse((self.root / "packs").exists())


if __name__ == "__main__":
    unittest.main()
