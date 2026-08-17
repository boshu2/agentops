from __future__ import annotations

import importlib.util
import io
import contextlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time
import unittest
import zipfile


SKILL = Path(__file__).resolve().parents[1]
MAIN_PATH = SKILL / "scripts" / "reverse_engineer.py"
CAPTURE = SKILL / "scripts" / "binary" / "capture_cli_help.sh"
ANALYZE = SKILL / "scripts" / "binary" / "analyze_binary.sh"
LIST_ARCHIVES = SKILL / "scripts" / "binary" / "list_embedded_archives.py"
EXTRACT_ARCHIVES = SKILL / "scripts" / "binary" / "extract_embedded_archives.py"
FETCH = SKILL / "scripts" / "fetch_url.py"

SPEC = importlib.util.spec_from_file_location("reverse_engineer", MAIN_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class ReverseEngineerSafetyTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name).resolve()

    def tearDown(self) -> None:
        self.temp.cleanup()

    def executable(self, name: str, body: str) -> Path:
        path = self.root / name
        path.write_text(body, encoding="utf-8")
        path.chmod(0o755)
        return path

    def test_bounded_subprocess_positive_timeout_output_and_descendant_cleanup(self) -> None:
        ok = MODULE._check_output([sys.executable, "-c", "print('ok')"], timeout=1, max_output_bytes=32)
        self.assertEqual(ok.strip(), "ok")
        visible_out = io.StringIO()
        visible_err = io.StringIO()
        with contextlib.redirect_stdout(visible_out), contextlib.redirect_stderr(visible_err):
            quiet = MODULE._run(
                [sys.executable, "-c", "import sys; print('api_key=super-secret'); print('token=also-secret', file=sys.stderr)"],
                timeout=1,
            )
        self.assertEqual(quiet.returncode, 0)
        self.assertNotIn("super-secret", visible_out.getvalue() + visible_err.getvalue())
        with self.assertRaisesRegex(ValueError, "output exceeded"):
            MODULE._check_output([sys.executable, "-c", "print('x'*10000)"], timeout=1, max_output_bytes=64)
        pid_file = self.root / "pid"
        code = (
            "import subprocess,sys,time; "
            "p=subprocess.Popen([sys.executable,'-c','import time; time.sleep(30)']); "
            f"open({str(pid_file)!r},'w').write(str(p.pid)); time.sleep(30)"
        )
        with self.assertRaisesRegex(TimeoutError, "subprocess exceeded"):
            MODULE._run([sys.executable, "-c", code], timeout=0.15)
        pid = int(pid_file.read_text())
        for _ in range(100):
            try:
                os.kill(pid, 0)
            except ProcessLookupError:
                break
            time.sleep(0.01)
        else:
            self.fail(f"timed-out subprocess descendant {pid} survived")

    def test_read_contract_rejects_unapproved_symlink_and_oversized_files(self) -> None:
        allowed = self.root / "allowed"
        allowed.mkdir()
        MODULE._authorize_read_root(allowed)
        regular = allowed / "small.txt"
        regular.write_text("safe", encoding="utf-8")
        self.assertEqual(MODULE._read_text(regular), "safe")
        outside = self.root / "outside.txt"
        outside.write_text("outside", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "outside an authorized root"):
            MODULE._read_text(outside)
        link = allowed / "linked.txt"
        link.symlink_to(outside)
        with self.assertRaisesRegex(ValueError, "symlink"):
            MODULE._read_text(link)
        large = allowed / "large.txt"
        large.write_bytes(b"x" * (MODULE.MAX_READ_BYTES + 1))
        with self.assertRaisesRegex(ValueError, "larger"):
            MODULE._read_text(large)

    def test_capture_help_redacts_secret_and_path_and_requires_timeout(self) -> None:
        binary = self.executable(
            "demo",
            "#!/bin/sh\nprintf '%s\\n' 'Usage: demo [OPTIONS]' 'api_key=super-secret' '/private/host/path'\n",
        )
        out = self.root / "help"
        completed = subprocess.run(["bash", str(CAPTURE), str(binary), str(out)], capture_output=True, text=True)
        self.assertEqual(completed.returncode, 0, completed.stderr)
        retained = (out / "cli-help-tree.txt").read_text(encoding="utf-8")
        self.assertIn("Usage: demo", retained)
        self.assertNotIn("super-secret", retained)
        self.assertNotIn("/private/host/path", retained)

        minimal = self.root / "minimal"
        minimal.mkdir()
        basename = shutil.which("basename")
        assert basename
        (minimal / "basename").symlink_to(basename)
        env = os.environ.copy(); env["PATH"] = str(minimal)
        refused = subprocess.run(["/bin/bash", str(CAPTURE), str(binary), str(self.root / "none")], env=env, capture_output=True, text=True)
        self.assertEqual(refused.returncode, 2)
        self.assertIn("refusing uncapped", refused.stderr)
        self.assertFalse((self.root / "none").exists())

    def test_binary_analysis_retains_aggregate_facts_not_raw_secret_lines(self) -> None:
        binary = self.executable("secret-bin", "#!/bin/sh\n# api_key=super-secret\necho ok\n")
        out = self.root / "analysis"
        completed = subprocess.run(["bash", str(ANALYZE), str(binary), str(out)], capture_output=True, text=True)
        self.assertEqual(completed.returncode, 0, completed.stderr)
        retained = "\n".join(path.read_text(errors="replace") for path in out.iterdir() if path.is_file())
        self.assertNotIn("super-secret", retained)
        self.assertFalse((out / "strings.head.txt").exists())
        self.assertFalse((out / "strings.ai-hits.txt").exists())

    def archive_binary(self, member: str) -> Path:
        payload = io.BytesIO()
        with zipfile.ZipFile(payload, "w") as archive:
            archive.writestr(member, "payload")
        binary = self.root / "archive-bin"
        binary.write_bytes(b"#!/bin/sh\nexit 0\n" + payload.getvalue())
        binary.chmod(0o755)
        return binary

    def test_archive_index_hashes_names_and_extraction_rejects_traversal(self) -> None:
        safe = self.archive_binary("private/secret-name.txt")
        index_dir = self.root / "index"; index_dir.mkdir()
        json_dir = self.root / "json"; json_dir.mkdir()
        result = subprocess.run([
            sys.executable, str(LIST_ARCHIVES), "--binary", str(safe),
            "--out-json", str(json_dir / "archives.json"),
            "--out-index-md", str(index_dir / "archives.md"),
            "--json-root", str(json_dir), "--index-root", str(index_dir),
            "--max-binary-bytes", "1000000",
        ], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        retained = (json_dir / "archives.json").read_text() + (index_dir / "archives.md").read_text()
        self.assertNotIn("private/secret-name.txt", retained)
        self.assertNotIn(str(safe), retained)

        malicious = self.archive_binary("../../outside.txt")
        extract_root = self.root / "extract-root"; extract_root.mkdir()
        rejected = subprocess.run([
            sys.executable, str(EXTRACT_ARCHIVES), "--binary", str(malicious),
            "--out-dir", str(extract_root / "out"), "--output-root", str(extract_root),
            "--max-binary-bytes", "1000000", "--max-candidates", "10",
        ], capture_output=True, text=True)
        self.assertEqual(rejected.returncode, 1)
        self.assertFalse((self.root / "outside.txt").exists())

    def test_fetch_requires_explicit_local_root_and_rejects_escape_without_writing(self) -> None:
        source = self.root / "source"
        source.write_text("bounded sitemap", encoding="utf-8")
        out_root = self.root / "downloads"; out_root.mkdir()
        accepted = subprocess.run([
            sys.executable, str(FETCH), source.as_uri(), "copy",
            "--input-root", str(self.root),
            "--output-root", str(out_root), "--max-bytes", "1024",
        ], capture_output=True, text=True)
        self.assertEqual(accepted.returncode, 0, accepted.stderr)
        self.assertEqual((out_root / "copy").read_text(), "bounded sitemap")
        (out_root / "copy").unlink()
        for url, output in ((source.as_uri(), "copy"), ("https://example.invalid/a", "../escape")):
            with self.subTest(url=url, output=output):
                completed = subprocess.run([
                    sys.executable, str(FETCH), url, output,
                    "--output-root", str(out_root), "--max-bytes", "1024",
                ], capture_output=True, text=True)
                self.assertEqual(completed.returncode, 2)
        outside = self.root / "outside-source"
        outside.write_text("outside", encoding="utf-8")
        linked = self.root / "linked-source"
        linked.symlink_to(outside)
        rejected_link = subprocess.run([
            sys.executable, str(FETCH), linked.as_uri(), "linked-copy",
            "--input-root", str(self.root),
            "--output-root", str(out_root), "--max-bytes", "1024",
        ], capture_output=True, text=True)
        self.assertEqual(rejected_link.returncode, 2)
        self.assertFalse((out_root / "copy").exists())
        self.assertFalse((out_root / "linked-copy").exists())
        self.assertFalse((self.root / "escape").exists())


if __name__ == "__main__":
    unittest.main()
