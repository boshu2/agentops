from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time
import unittest


PACKAGE = Path(__file__).resolve().parents[1]
SCRIPT_NAMES = ("build.sh", "heal.sh", "init.sh", "transaction.sh")
SHARED_FILES = (
    "skills/catalog.json",
    "registry.json",
    "docs/SKILL-ROUTER.md",
    "docs/SKILLS.md",
    "skills/SKILL-TIERS.md",
    "docs/reference/agentops-skill-domain-map.md",
    "docs/reference/agentops-skill-graph.md",
    "docs/contracts/context-map.md",
    "images/claude/manifest.json",
    "images/codex/manifest.json",
    "images/gemini/plugin.json",
    "skills-codex/.agentops-manifest.json",
    "skills-codex-overrides/catalog.json",
)


MESH_STUB = r'''#!/usr/bin/env python3
import os
from pathlib import Path
import subprocess
import sys
import time

root = Path(__file__).resolve().parents[1]
paths = [
    "skills/catalog.json", "registry.json", "docs/SKILL-ROUTER.md", "docs/SKILLS.md",
    "skills/SKILL-TIERS.md", "docs/reference/agentops-skill-domain-map.md",
    "docs/reference/agentops-skill-graph.md", "docs/contracts/context-map.md",
    "images/claude/manifest.json", "images/codex/manifest.json", "images/gemini/plugin.json",
]
for relative in paths:
    target = root / relative
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text("mesh-candidate\n", encoding="utf-8")
gemini = root / "images/gemini/skills/generated/SKILL.md"
gemini.parent.mkdir(parents=True, exist_ok=True)
gemini.write_text("generated\n", encoding="utf-8")
mode = os.environ.get("SB_TEST_MODE", "success")
if mode == "hang-mesh":
    child = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(30)"])
    Path(os.environ["SB_TEST_PID_FILE"]).write_text(str(child.pid), encoding="utf-8")
    time.sleep(30)
if mode == "mesh-fail":
    raise SystemExit(7)
'''


CODEX_STUB = r'''#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
names="${!#}"
IFS=',' read -r -a selected <<<"$names"
for name in "${selected[@]}"; do
  mkdir -p "$root/skills-codex/$name"
  printf '%s\n' 'codex-candidate' >"$root/skills-codex/$name/SKILL.md"
done
mkdir -p "$root/skills-codex-overrides"
printf '%s\n' 'manifest-candidate' >"$root/skills-codex/.agentops-manifest.json"
printf '%s\n' 'catalog-candidate' >"$root/skills-codex-overrides/catalog.json"
[[ "${SB_TEST_MODE:-success}" != codex-fail ]] || exit 7
'''


HASH_STUB = r'''#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
name="${!#}"
printf '%s\n' 'hash-candidate' >"$root/skills-codex/$name/.agentops-generated.json"
'''


class SkillBuilderTransactionTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name).resolve() / "repo"
        scripts = self.root / "skills/skill-builder/scripts"
        scripts.mkdir(parents=True)
        for name in SCRIPT_NAMES:
            shutil.copy2(PACKAGE / "scripts" / name, scripts / name)
        global_scripts = self.root / "scripts"
        global_scripts.mkdir()
        self._write(global_scripts / "generate-skill-mesh.py", MESH_STUB, executable=True)
        self._write(global_scripts / "codex-sync.sh", CODEX_STUB, executable=True)
        self._write(global_scripts / "regen-codex-hashes.sh", HASH_STUB, executable=True)
        for index, relative in enumerate(SHARED_FILES):
            self._write(self.root / relative, f"baseline-{index}\n")
        self._write(self.root / "images/gemini/skills/existing/SKILL.md", "existing\n")
        self.env = os.environ.copy()
        self.env.update(
            {
                "PYTHONDONTWRITEBYTECODE": "1",
                "SKILL_BUILDER_REPO_ROOT": str(self.root),
                "HEAL_REPO_ROOT": str(self.root),
                "SKILL_BUILDER_STEP_TIMEOUT": "2",
            }
        )

    def tearDown(self) -> None:
        self.temp.cleanup()

    @staticmethod
    def _write(path: Path, text: str, *, executable: bool = False) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        if executable:
            path.chmod(0o755)

    def _run(self, script: str, *args: str, env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ["bash", str(self.root / "skills/skill-builder/scripts" / script), *args],
            env=env or self.env,
            capture_output=True,
            text=True,
            timeout=12,
        )

    def _state(self) -> tuple[tuple[str, ...], dict[str, str]]:
        directories = tuple(sorted(p.relative_to(self.root).as_posix() for p in self.root.rglob("*") if p.is_dir()))
        files = {
            p.relative_to(self.root).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in self.root.rglob("*")
            if p.is_file() and not p.is_symlink()
        }
        return directories, files

    def test_direct_build_and_explicit_heal_fix_preserve_positive_capability(self) -> None:
        built = self._run("build.sh", "from-scratch", "bounded-demo")
        self.assertEqual(built.returncode, 0, built.stderr)
        report = json.loads((self.root / ".agents/scratch/skill-builder/bounded-demo-build.json").read_text())
        self.assertTrue(report["structure_check_pass"])
        self.assertTrue((self.root / "skills/bounded-demo/SKILL.md").is_file())
        self.assertTrue((self.root / "skills-codex/bounded-demo/.agentops-generated.json").is_file())

        before_check = self._state()
        checked = self._run("heal.sh", "--check", "skills/bounded-demo")
        self.assertEqual(checked.returncode, 0, checked.stderr)
        self.assertEqual(self._state(), before_check, "check mode must not trigger generators")

        fixed = self._run("heal.sh", "--fix", "skills/bounded-demo")
        self.assertEqual(fixed.returncode, 0, fixed.stderr)
        self.assertEqual((self.root / "skills/catalog.json").read_text(), "mesh-candidate\n")
        self.assertEqual((self.root / "skills-codex/bounded-demo/SKILL.md").read_text(), "codex-candidate\n")

    def test_failed_and_hung_generation_restore_every_managed_surface(self) -> None:
        for mode in ("codex-fail", "hang-mesh"):
            with self.subTest(mode=mode):
                baseline = self._state()
                env = self.env | {"SB_TEST_MODE": mode, "SKILL_BUILDER_STEP_TIMEOUT": "1"}
                pid_file = Path(self.temp.name) / f"{mode}.pid"
                env["SB_TEST_PID_FILE"] = str(pid_file)
                completed = self._run("build.sh", "from-scratch", f"candidate-{mode}", env=env)
                self.assertNotEqual(completed.returncode, 0)
                self.assertEqual(self._state(), baseline)
                if mode == "hang-mesh":
                    pid = int(pid_file.read_text(encoding="utf-8"))
                    for _ in range(100):
                        try:
                            os.kill(pid, 0)
                        except ProcessLookupError:
                            break
                        time.sleep(0.01)
                    else:
                        self.fail(f"timed-out generator descendant {pid} survived")

    def test_external_mode_uses_authorized_root_hash_and_rejects_near_misses_unchanged(self) -> None:
        external = Path(self.temp.name).resolve() / "external"
        external.mkdir()
        source = external / "source.md"
        source.write_text("external signal only\n", encoding="utf-8")
        accepted = self._run(
            "init.sh", "--external", "external-demo",
            "--external-root", str(external), "--from", "source.md",
        )
        self.assertEqual(accepted.returncode, 0, accepted.stderr)
        report_text = (self.root / ".agents/scratch/skill-builder/external-demo-build.json").read_text()
        self.assertNotIn(str(external), report_text)
        self.assertIn(hashlib.sha256(source.read_bytes()).hexdigest(), report_text)
        self.assertNotIn("external signal only", (self.root / "skills/external-demo/SKILL.md").read_text())

        for slug, relative in (("traversal-demo", "../source.md"), ("absolute-demo", str(source))):
            with self.subTest(relative=relative):
                before = self._state()
                rejected = self._run(
                    "init.sh", "--external", slug,
                    "--external-root", str(external), "--from", relative,
                )
                self.assertEqual(rejected.returncode, 2)
                self.assertEqual(self._state(), before)

        outside = Path(self.temp.name).resolve() / "outside.md"
        outside.write_text("outside\n", encoding="utf-8")
        (external / "linked.md").symlink_to(outside)
        before = self._state()
        linked = self._run(
            "init.sh", "--external", "linked-demo",
            "--external-root", str(external), "--from", "linked.md",
        )
        self.assertEqual(linked.returncode, 2)
        self.assertEqual(self._state(), before)

    def test_oversized_input_and_symlinked_managed_surface_fail_before_mutation(self) -> None:
        external = Path(self.temp.name).resolve() / "large"
        external.mkdir()
        (external / "large.md").write_bytes(b"x" * (1048576 + 1))
        baseline = self._state()
        oversized = self._run(
            "init.sh", "--external", "large-demo",
            "--external-root", str(external), "--from", "large.md",
        )
        self.assertEqual(oversized.returncode, 2)
        self.assertEqual(self._state(), baseline)

        registry = self.root / "registry.json"
        registry.unlink()
        outside = Path(self.temp.name).resolve() / "outside-registry"
        outside.write_text("sentinel\n", encoding="utf-8")
        registry.symlink_to(outside)
        rejected = self._run("build.sh", "from-scratch", "symlink-demo")
        self.assertNotEqual(rejected.returncode, 0)
        self.assertEqual(outside.read_text(), "sentinel\n")
        self.assertFalse((self.root / "skills/symlink-demo").exists())

        env = self.env | {"SKILL_CAPABILITIES": json.dumps(["x" * 257])}
        metadata = self._run("init.sh", "--scratch", "metadata-demo", env=env)
        self.assertEqual(metadata.returncode, 2)
        self.assertFalse((self.root / "skills/metadata-demo").exists())


if __name__ == "__main__":
    unittest.main()
