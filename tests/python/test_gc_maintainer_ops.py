from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest


REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPT = REPO_ROOT / "scripts" / "gc-maintainer-ops.sh"
PACK_COMMIT = "3b3b89f2011e06d84459aa7bea1552382f13930a"
WORKFLOW_SOURCE = "https://github.com/gastownhall/gascity-packs/tree/main/gascity"
ROLES_SOURCE = f"{WORKFLOW_SOURCE}/roles"
CHECKS = (
    "build-artifact-valid.sh",
    "design-review-approved.sh",
    "gap-analysis-approved.sh",
    "implementation-review-approved.sh",
)


def tree_digest(root: Path) -> str:
    digest = hashlib.sha256()
    for path in sorted(item for item in root.rglob("*") if item.is_file() or item.is_symlink()):
        digest.update(path.relative_to(root).as_posix().encode())
        digest.update(b"\0")
        if path.is_symlink():
            digest.update(os.readlink(path).encode())
        else:
            digest.update(path.read_bytes())
        digest.update(b"\0")
    return digest.hexdigest()


class MaintainerOpsTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.city = self.root / "city"
        self.rig = self.city / "rigs" / "smoke"
        self.pack = self.root / "pack" / "gascity"
        self.bin = self.root / "bin"
        self.log = self.root / "gc.log"
        self.city.mkdir()
        self.rig.mkdir(parents=True)
        self.bin.mkdir()
        (self.city / "city.toml").write_text("[workspace]\n", encoding="utf-8")
        (self.pack / "assets" / "scripts" / "checks").mkdir(parents=True)
        (self.pack / "schemas").mkdir(parents=True)
        (self.pack.parent / ".gc-bundled-pack-cache.toml").write_text(
            textwrap.dedent(
                f"""\
                schema = 1
                repository = "https://github.com/gastownhall/gascity-packs.git"
                commit = "{PACK_COMMIT}"
                """
            ),
            encoding="utf-8",
        )
        for name in CHECKS:
            (self.pack / "assets" / "scripts" / "checks" / name).write_text(
                f"#!/bin/sh\nprintf 'upstream {name}\\n'\n",
                encoding="utf-8",
            )
        (self.pack / "assets" / "scripts" / "validate_build_artifact.py").write_text(
            "import yaml\n",
            encoding="utf-8",
        )
        (self.pack / "schemas" / "gc.build.final-report.v1.schema.json").write_text(
            "{}\n", encoding="utf-8"
        )
        self._write_fakes()

    def tearDown(self) -> None:
        self.temp.cleanup()

    def _write_fakes(self) -> None:
        gc = self.bin / "gc"
        gc.write_text(
            textwrap.dedent(
                """\
                #!/bin/sh
                set -eu
                printf '%s\\n' "$*" >>"$FAKE_GC_LOG"
                case " $* " in
                  *" import status "*)
                    cat <<EOF
                {"schema_version":"1","ok":true,"imports":[
                  {"name":"pack:gascity","source":"https://github.com/gastownhall/gascity-packs/tree/main/gascity","pin":{"commit":"${FAKE_PACK_COMMIT}"}},
                  {"name":"rig:smoke:gc","source":"https://github.com/gastownhall/gascity-packs/tree/main/gascity/roles","pin":{"commit":"${FAKE_PACK_COMMIT}"}}
                ]}
                EOF
                    ;;
                  *" rig list "*)
                    printf '{"ok":true,"rigs":[{"name":"smoke","path":"%s","hq":false}]}\\n' "$FAKE_RIG"
                    ;;
                  *" doctor "*)
                    printf '%s\\n' "$FAKE_DOCTOR_JSON"
                    ;;
                  *" status "*)
                    printf '%s\\n' "$FAKE_STATUS_JSON"
                    ;;
                  *" session list "*)
                    printf '%s\\n' "$FAKE_SESSIONS_JSON"
                    ;;
                  *" bd ready "*)
                    printf '%s\\n' "$FAKE_READY_JSON"
                    ;;
                  *" bd update "*)
                    printf '{"ok":true}\\n'
                    ;;
                  *)
                    printf 'unexpected fake gc command: %s\\n' "$*" >&2
                    exit 64
                    ;;
                esac
                """
            ),
            encoding="utf-8",
        )
        gc.chmod(0o755)

        ao = self.bin / "ao"
        ao.write_text(
            textwrap.dedent(
                """\
                #!/bin/sh
                set -eu
                dest=""
                while [ "$#" -gt 0 ]; do
                  if [ "$1" = "--dest" ]; then dest="$2"; shift 2; continue; fi
                  shift
                done
                [ -n "$dest" ]
                printf 'ao skills link dest=%s\\n' "$dest" >>"$FAKE_GC_LOG"
                mkdir -p "$dest"
                for skill in using-gc plan implement test validate; do
                  if [ ! -e "$dest/$skill" ] && [ ! -L "$dest/$skill" ]; then
                    ln -s "$AGENTOPS_SOURCE/skills/$skill" "$dest/$skill"
                  fi
                done
                printf '{"ok":true}\\n'
                """
            ),
            encoding="utf-8",
        )
        ao.chmod(0o755)

        python = self.bin / "python3"
        python.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
        python.chmod(0o755)

    def run_ops(
        self,
        command: str,
        *extra: str,
        pack_commit: str = PACK_COMMIT,
        ready: list[dict[str, object]] | None = None,
        sessions: list[dict[str, object]] | None = None,
    ) -> subprocess.CompletedProcess[str]:
        env = os.environ.copy()
        env.update(
            {
                "AGENTOPS_SOURCE": str(REPO_ROOT),
                "AGENTOPS_GC_SKIP_SERVICE_CHECK": "1",
                "FAKE_GC_LOG": str(self.log),
                "FAKE_PACK_COMMIT": pack_commit,
                "FAKE_RIG": str(self.rig),
                "FAKE_DOCTOR_JSON": json.dumps(
                    {
                        "ok": True,
                        "blocking_failed": 0,
                        "failed": 0,
                        "results": [],
                    }
                ),
                "FAKE_STATUS_JSON": json.dumps(
                    {"ok": True, "partial": False, "health": {"signals": []}}
                ),
                "FAKE_READY_JSON": json.dumps(ready or []),
                "FAKE_SESSIONS_JSON": json.dumps({"ok": True, "sessions": sessions or []}),
                "GC_PYTHON_BIN": str(self.bin / "python3"),
                "PATH": f"{self.bin}:{env['PATH']}",
            }
        )
        return subprocess.run(
            [
                str(SCRIPT),
                command,
                "--city",
                str(self.city),
                "--rig",
                str(self.rig),
                "--gc-bin",
                str(self.bin / "gc"),
                "--ao-bin",
                str(self.bin / "ao"),
                "--pack-dir",
                str(self.pack),
                *extra,
            ],
            cwd=REPO_ROOT,
            env=env,
            check=False,
            capture_output=True,
            text=True,
            timeout=30,
        )

    def test_prepare_is_idempotent_and_stages_unmodified_upstream_runtime(self) -> None:
        first = self.run_ops("prepare")
        self.assertEqual(first.returncode, 0, first.stderr)
        second = self.run_ops("prepare")
        self.assertEqual(second.returncode, 0, second.stderr)

        runtime = (
            self.rig
            / ".gc"
            / "agentops-maintainer-runtime"
            / "versions"
            / PACK_COMMIT
        )
        for name in CHECKS:
            source = self.pack / "assets" / "scripts" / "checks" / name
            snapshot = runtime / "gascity" / "assets" / "scripts" / "checks" / name
            self.assertEqual(snapshot.read_bytes(), source.read_bytes())
            wrapper = self.rig / ".gc" / "scripts" / "checks" / name
            self.assertIn("managed-by: agentops gc-maintainer-ops", wrapper.read_text())
        for skill in ("using-gc", "plan", "implement", "test", "validate"):
            self.assertTrue((self.city / ".codex" / "skills" / skill / "SKILL.md").is_file())
            self.assertTrue((self.rig / ".codex" / "skills" / skill / "SKILL.md").is_file())

    def test_prepare_refuses_wrong_pin_before_mutation(self) -> None:
        result = self.run_ops("prepare", pack_commit="deadbeef")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("official gascity workflow and rig-role pins", result.stderr)
        self.assertFalse((self.rig / ".gc").exists())

    def test_prepare_refuses_pack_cache_wrong_pin_before_mutation(self) -> None:
        marker = self.pack.parent / ".gc-bundled-pack-cache.toml"
        marker.write_text(
            marker.read_text(encoding="utf-8").replace(PACK_COMMIT, "deadbeef"),
            encoding="utf-8",
        )
        result = self.run_ops("prepare")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("pack cache marker does not match", result.stderr)
        self.assertFalse((self.rig / ".gc").exists())

    def test_prepare_refuses_foreign_check_wrapper(self) -> None:
        checks = self.rig / ".gc" / "scripts" / "checks"
        checks.mkdir(parents=True)
        foreign = checks / "build-artifact-valid.sh"
        foreign.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
        result = self.run_ops("prepare")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("refusing to overwrite unmanaged check", result.stderr)
        self.assertEqual(foreign.read_text(), "#!/bin/sh\nexit 0\n")

    def test_prepare_refuses_foreign_required_skill(self) -> None:
        foreign = self.city / ".codex" / "skills" / "using-gc"
        foreign.mkdir(parents=True)
        (foreign / "SKILL.md").write_text("# foreign\n", encoding="utf-8")
        result = self.run_ops("prepare")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("does not resolve to this checkout", result.stderr)
        self.assertEqual(
            (foreign / "SKILL.md").read_text(encoding="utf-8"), "# foreign\n"
        )
        self.assertFalse((self.rig / ".gc").exists())

    def test_check_is_read_only_after_prepare(self) -> None:
        prepared = self.run_ops("prepare")
        self.assertEqual(prepared.returncode, 0, prepared.stderr)
        before = tree_digest(self.city)
        checked = self.run_ops("check")
        self.assertEqual(checked.returncode, 0, checked.stderr)
        self.assertEqual(tree_digest(self.city), before)
        self.assertIn("maintainer runtime ready", checked.stdout)

    def test_recover_affinity_is_dry_run_and_only_clears_stale_required_assignment(self) -> None:
        ready = [
            {
                "id": "sm-stale",
                "assignee": "worker-old",
                "metadata": {
                    "gc.session_affinity": "require",
                    "gc.routed_to": "smoke/gc.run-operator",
                },
            },
            {
                "id": "sm-live",
                "assignee": "worker-live",
                "metadata": {
                    "gc.session_affinity": "require",
                    "gc.routed_to": "smoke/gc.run-operator",
                },
            },
            {
                "id": "sm-unrelated",
                "assignee": "worker-old",
                "metadata": {"gc.routed_to": "smoke/gc.run-operator"},
            },
            {
                "id": "sm-unrouted",
                "assignee": "worker-old",
                "metadata": {"gc.session_affinity": "require"},
            },
        ]
        sessions = [
            {
                "id": "af-live",
                "name": "worker-live",
                "session_name": "worker-live",
                "state": "active",
            }
        ]
        dry = self.run_ops("recover-affinity", ready=ready, sessions=sessions)
        self.assertEqual(dry.returncode, 0, dry.stderr)
        self.assertIn("would clear sm-stale", dry.stdout)
        self.assertNotIn(" bd update ", f" {self.log.read_text()} ")

        applied = self.run_ops(
            "recover-affinity", "--apply", ready=ready, sessions=sessions
        )
        self.assertEqual(applied.returncode, 0, applied.stderr)
        updates = [
            line
            for line in self.log.read_text(encoding="utf-8").splitlines()
            if " bd update " in f" {line} "
        ]
        self.assertEqual(len(updates), 1)
        self.assertIn("sm-stale", updates[0])
        self.assertNotIn("sm-live", updates[0])
        self.assertNotIn("sm-unrelated", updates[0])
        self.assertNotIn("sm-unrouted", updates[0])


if __name__ == "__main__":
    unittest.main()
