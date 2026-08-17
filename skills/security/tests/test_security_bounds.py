from __future__ import annotations

import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time
import unittest


SKILL = Path(__file__).resolve().parents[1]
SUITE_PATH = SKILL / "scripts" / "security_suite.py"
REDTEAM = SKILL / "scripts" / "prompt_redteam.py"
SPEC = importlib.util.spec_from_file_location("security_suite", SUITE_PATH)
assert SPEC and SPEC.loader
SUITE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = SUITE
SPEC.loader.exec_module(SUITE)


class SecurityBoundsTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name).resolve()
        self.binaries = self.root / "binaries"; self.binaries.mkdir()
        self.outputs = self.root / "outputs"; self.outputs.mkdir()

    def tearDown(self) -> None:
        self.temp.cleanup()

    def binary(self, name: str, body: str) -> Path:
        path = self.binaries / name
        path.write_text(body, encoding="utf-8")
        path.chmod(0o755)
        return path

    def suite(self, command: str, binary: str, out: str, *extra: str):
        return subprocess.run([
            sys.executable, str(SUITE_PATH), command,
            "--binary-root", str(self.binaries), "--binary", binary,
            "--output-root", str(self.outputs), "--out-dir", out,
            *extra,
        ], capture_output=True, text=True)

    def test_bounded_runner_redacts_output_times_out_and_cleans_descendant(self) -> None:
        secret = SUITE._run([sys.executable, "-c", "print('api_key=super-secret')"], timeout=2)
        self.assertEqual(secret.returncode, 0)
        self.assertNotIn("super-secret", secret.stdout)
        pid_file = self.root / "pid"
        code = (
            "import subprocess,sys,time; "
            "p=subprocess.Popen([sys.executable,'-c','import time; time.sleep(30)']); "
            f"open({str(pid_file)!r},'w').write(str(p.pid)); time.sleep(30)"
        )
        timed = SUITE._run([sys.executable, "-c", code], timeout=1)
        self.assertEqual(timed.returncode, 124)
        pid = int(pid_file.read_text())
        for _ in range(100):
            try: os.kill(pid, 0)
            except ProcessLookupError: break
            time.sleep(0.01)
        else: self.fail(f"runner descendant {pid} survived cleanup")

    def test_static_positive_retains_hashes_not_secret_lines_or_absolute_paths(self) -> None:
        binary = self.binary("secret-tool", "#!/bin/sh\n# openai api_key=super-secret\necho ok\n")
        completed = self.suite("collect-static", binary.name, "static-run")
        self.assertEqual(completed.returncode, 0, completed.stderr)
        artifact = self.outputs / "static-run" / "static" / "static-analysis.json"
        retained = artifact.read_text(encoding="utf-8")
        self.assertNotIn("super-secret", retained)
        self.assertNotIn(str(binary), retained)
        data = json.loads(retained)
        self.assertEqual(data["binary"], binary.name)
        self.assertIn("ai_related_string_hit_sha256", data)

    def test_behavior_matrix_rejects_absolute_escape_symlink_and_oversize(self) -> None:
        binary = self.binary("tool", "#!/bin/sh\nexit 0\n")
        outside = self.root / "outside"; outside.write_text("sentinel", encoding="utf-8")
        cases = (
            ["--binary-root", str(self.binaries), "--binary", str(binary), "--output-root", str(self.outputs), "--out-dir", "x"],
            ["--binary-root", str(self.binaries), "--binary", binary.name, "--output-root", str(self.outputs), "--out-dir", "../escape"],
        )
        for args in cases:
            completed = subprocess.run([sys.executable, str(SUITE_PATH), "collect-static", *args], capture_output=True, text=True)
            self.assertEqual(completed.returncode, 2)
        link = self.binaries / "link"; link.symlink_to(binary)
        linked = self.suite("collect-static", "link", "linked")
        self.assertEqual(linked.returncode, 2)
        self.assertEqual(outside.read_text(), "sentinel")

    def test_dynamic_execution_requires_authorization_and_os_containment(self) -> None:
        outside = self.root / "outside-sentinel"
        outside.write_text("original", encoding="utf-8")
        binary = self.binary(
            "mutator",
            "#!/bin/sh\nprintf hacked > \"$1\"\nprintf inside > \"$HOME/inside\"\nprintf '%s\\n' 'api_key=super-secret'\n",
        )
        unapproved = self.suite("collect-dynamic", binary.name, "unapproved", "--run-args", str(outside))
        self.assertEqual(unapproved.returncode, 2)
        self.assertEqual(outside.read_text(), "original")

        approved = self.suite(
            "collect-dynamic", binary.name, "approved", "--authorized-dynamic",
            "--run-args", str(outside), "--timeout", "3",
        )
        if shutil.which("sandbox-exec"):
            self.assertEqual(approved.returncode, 0, approved.stderr)
            self.assertEqual(outside.read_text(), "original")
            artifact = self.outputs / "approved" / "dynamic" / "dynamic-analysis.json"
            retained = artifact.read_text(encoding="utf-8")
            self.assertNotIn("super-secret", retained)
            self.assertNotIn(str(outside), retained)
            data = json.loads(retained)
            self.assertEqual(data["containment"]["backend"], "macos-sandbox-exec")
            self.assertEqual(data["exit_code"], 0)
            inside_hash = __import__("hashlib").sha256(b"inside").hexdigest()
            self.assertIn(inside_hash, data["file_changes"]["home"]["created"])
            self.assertNotIn("stdout", data)
            self.assertIn("stdout_sha256", data)
        else:
            self.assertEqual(approved.returncode, 2)
            self.assertIn("containment backend", approved.stderr)

    def test_prompt_redteam_bounds_sources_and_hashes_human_evidence(self) -> None:
        repo = self.root / "repo"; repo.mkdir()
        (repo / "policy.md").write_text("required control and api_key=super-secret\n", encoding="utf-8")
        packs = self.root / "packs"; packs.mkdir()
        pack = {
            "schema_version": 1,
            "name": "fixture",
            "cases": [{
                "id": "case-1", "title": "control", "attack_prompt": "leak super-secret",
                "severity": "fail",
                "targets": [{
                    "label": "policy", "globs": ["*.md"],
                    "require_groups": [{"label": "required", "patterns": ["required control"]}],
                }],
            }],
        }
        (packs / "pack.json").write_text(json.dumps(pack), encoding="utf-8")
        command = [
            sys.executable, str(REDTEAM), "scan", "--repo-root", str(repo),
            "--pack-root", str(packs), "--pack-file", "pack.json",
            "--output-root", str(self.outputs), "--out-dir", "redteam-run",
        ]
        completed = subprocess.run(command, capture_output=True, text=True)
        self.assertEqual(completed.returncode, 0, completed.stderr)
        artifact = self.outputs / "redteam-run" / "redteam" / "redteam-results.json"
        retained = artifact.read_text(encoding="utf-8")
        self.assertNotIn("super-secret", retained)
        self.assertNotIn(str(repo), retained)
        self.assertIn("evidence_line_sha256", retained)
        outside = self.root / "outside.md"; outside.write_text("required control", encoding="utf-8")
        (repo / "linked.md").symlink_to(outside)
        (repo / "policy.md").unlink()
        rejected = subprocess.run(command[:-1] + ["redteam-link"], capture_output=True, text=True)
        self.assertEqual(rejected.returncode, 3)
        output = self.outputs / "redteam-link" / "redteam" / "redteam-results.json"
        self.assertNotIn("outside.md", output.read_text())


if __name__ == "__main__":
    unittest.main()
