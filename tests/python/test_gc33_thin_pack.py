from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import platform
import stat
import subprocess
import tarfile
import tempfile
import textwrap
import time
import tomllib
import unittest


REPO_ROOT = Path(__file__).resolve().parents[2]
DEPLOY = REPO_ROOT / "deploy" / "gc"
FACTORY = REPO_ROOT / "packs" / "agentops-factory"
EXECUTOR = REPO_ROOT / "packs" / "agentops-executor"
GC_COMMIT = "8ffc009ded781a2ada2077f3a29bd712b2def0bf"
BD_COMMIT = "8e4e59d39f3459a43cf21a3236a13eca4dd874f7"
PINNED_INTEGRATION = bool(os.environ.get("GC_BIN") and os.environ.get("AGENTOPS_GC_INTEGRATION") == "1")


def run(*argv: str, cwd: Path | None = None, env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        argv,
        cwd=cwd,
        env=env,
        check=False,
        capture_output=True,
        text=True,
        timeout=120,
    )


def git(*argv: str, cwd: Path) -> subprocess.CompletedProcess[str]:
    return run("git", *argv, cwd=cwd)


class ThinPackTests(unittest.TestCase):
    def make_repository(self, root: Path) -> tuple[Path, Path]:
        remote = root / "remote.git"
        repo = root / "repo"
        self.assertEqual(run("git", "init", "--bare", str(remote)).returncode, 0)
        self.assertEqual(run("git", "init", "-b", "main", str(repo)).returncode, 0)
        self.assertEqual(git("config", "user.name", "AgentOps Test", cwd=repo).returncode, 0)
        self.assertEqual(git("config", "user.email", "agentops-test@example.invalid", cwd=repo).returncode, 0)
        (repo / "README.md").write_text("seed\n", encoding="utf-8")
        (repo / ".gitignore").write_text(".beads/*\n!.beads/identity.toml\n.agents/\n.claude/\n", encoding="utf-8")
        self.assertEqual(git("add", "README.md", ".gitignore", cwd=repo).returncode, 0)
        self.assertEqual(git("commit", "-m", "seed", cwd=repo).returncode, 0)
        self.assertEqual(git("remote", "add", "origin", str(remote), cwd=repo).returncode, 0)
        self.assertEqual(git("push", "-u", "origin", "main", cwd=repo).returncode, 0)
        self.assertEqual(run("git", "-C", str(remote), "symbolic-ref", "HEAD", "refs/heads/main").returncode, 0)
        return repo, remote

    def initialize_beads(self, repo: Path, gc_bin: Path) -> str:
        bd_bin = gc_bin.parent / "bd"
        initialized = run(
            str(bd_bin), "init", "--non-interactive", "--skip-agents", "--skip-hooks",
            "--setup-exclude", "--prefix", "test",
            cwd=repo,
        )
        self.assertEqual(initialized.returncode, 0, initialized.stderr)
        # GC v1.3.5 adopts the on-disk prefix, while BD v1.1.0 stores the init
        # prefix in Dolt only. Model the GC-compatible source config that an
        # existing rig carries before official bd bootstrap moves its durable
        # database into a fresh city's managed server.
        config_path = repo / ".beads/config.yaml"
        config_path.write_text(
            "issue_prefix: test\nissue-prefix: test\n" + config_path.read_text(encoding="utf-8"),
            encoding="utf-8",
        )
        created = run(str(bd_bin), "create", "--title", "source bead", "--type", "task", "--json", cwd=repo)
        self.assertEqual(created.returncode, 0, created.stderr)
        source_bead = json.loads(created.stdout)["id"]
        self.assertTrue(source_bead.startswith("test-"))
        committed = run(str(bd_bin), "dolt", "commit", "-m", "source bead", cwd=repo)
        self.assertEqual(committed.returncode, 0, committed.stderr)
        pushed = run(str(bd_bin), "dolt", "push", cwd=repo)
        self.assertEqual(pushed.returncode, 0, pushed.stderr)
        self.assertEqual(git("add", ".gitignore", cwd=repo).returncode, 0)
        if (repo / ".beads/identity.toml").exists():
            self.assertEqual(git("add", "-f", ".beads/identity.toml", cwd=repo).returncode, 0)
        if git("diff", "--cached", "--quiet", cwd=repo).returncode != 0:
            self.assertEqual(git("commit", "-m", "adopt official beads identity", cwd=repo).returncode, 0)
            self.assertEqual(git("push", "origin", "main", cwd=repo).returncode, 0)
        return source_bead

    def test_runtime_surface_is_native_and_single_owner(self) -> None:
        removed = (
            REPO_ROOT / "cli/cmd/agentops-gc-delivery",
            REPO_ROOT / "cli/internal/gcadapter/delivery",
            FACTORY / "assets/schemas",
            FACTORY / "assets/scripts/factory_feeder.py",
            FACTORY / "assets/scripts/program_start.py",
            FACTORY / "assets/scripts/role_adapter.py",
            EXECUTOR / "assets/schemas/gc-execution-envelope.v1.schema.json",
            EXECUTOR / "commands/run-packet",
            DEPLOY / "reliability.py",
            DEPLOY / "known-errors.json",
            DEPLOY / "fork-baseline.json",
            DEPLOY / "claude-interactive.sh",
        )
        self.assertEqual(
            [str(path) for path in removed if path.is_file() or (path.is_dir() and any(item.is_file() for item in path.rglob("*")))],
            [],
        )

        invoke = (DEPLOY / "invoke.sh").read_text(encoding="utf-8")
        # Official single-bead intake: feed slings to the rig-scoped planner with
        # the native formula attached; it never slings to the city-scoped Mayor.
        self.assertIn('sling "$rig_name/agentops.plan-reviewer" "$1"', invoke)
        self.assertIn("--on agentops-experiment", invoke)
        self.assertNotIn("sling agentops.mayor", invoke)
        for obsolete in ("program start", "factory_feeder", "role_adapter", "run-packet"):
            self.assertNotIn(obsolete, invoke)

        city_text = (DEPLOY / "city.toml").read_text(encoding="utf-8").replace("__GC_MAX_ACTIVE_SESSIONS__", "2")
        city = tomllib.loads(city_text)
        self.assertEqual(city["providers"]["claude"]["print_args"], [])
        self.assertNotIn("path", city["providers"]["claude"])
        self.assertNotIn("args_append", city["providers"]["claude"])
        self.assertEqual(city["daemon"]["formula_v2"], True)

    def test_formula_is_one_native_dependency_chain(self) -> None:
        formula = tomllib.loads((FACTORY / "formulas/agentops-experiment.toml").read_text(encoding="utf-8"))
        steps = formula["steps"]
        self.assertEqual([step["id"] for step in steps], ["plan", "implement", "validate", "deliver"])
        self.assertNotIn("needs", steps[0])
        self.assertEqual([step["needs"] for step in steps[1:]], [["plan"], ["implement"], ["validate"]])
        self.assertEqual(
            [step["metadata"]["gc.run_target"] for step in steps],
            ["{{plan_target}}", "{{implement_target}}", "{{validate_target}}", "{{refiner_target}}"],
        )
        self.assertEqual(set(formula["vars"]), {"work_dir", "plan_target", "implement_target", "validate_target", "refiner_target"})

    def test_role_models_match_factory_policy(self) -> None:
        expected = {
            FACTORY / "agents/mayor/agent.toml": ("claude", "fable-5", "adaptive"),
            FACTORY / "agents/plan-reviewer/agent.toml": ("codex", "gpt-5.6-sol", "high"),
            FACTORY / "agents/refiner/agent.toml": ("claude", "fable-5", "adaptive"),
            EXECUTOR / "agents/implementer/agent.toml": ("codex", "gpt-5.6-terra", "high"),
            EXECUTOR / "agents/implementer-claude/agent.toml": ("claude", "opus-4.8", "medium"),
            EXECUTOR / "agents/validator/agent.toml": ("codex", "gpt-5.6-sol", "high"),
        }
        mayor_agent = FACTORY / "agents/mayor/agent.toml"
        for path, (provider, model, effort) in expected.items():
            config = tomllib.loads(path.read_text(encoding="utf-8"))
            self.assertEqual(config["provider"], provider, path)
            self.assertEqual(config["option_defaults"], {"permission_mode": config["option_defaults"]["permission_mode"], "model": model, "effort": effort}, path)
            # Every role is a one_shot worker except the Mayor, which is the
            # standing dispatch shepherd (asserted separately).
            if path == mayor_agent:
                self.assertNotIn("lifecycle", config, path)
            else:
                self.assertEqual(config["lifecycle"], "one_shot", path)
        runtime_text = "\n".join(path.read_text(encoding="utf-8") for path in expected)
        self.assertNotIn("luna", runtime_text.lower())

    def test_exact_official_toolchain_provenance(self) -> None:
        lock = json.loads((DEPLOY / "toolchain.lock.json").read_text(encoding="utf-8"))
        self.assertEqual(lock["schema_version"], 1)
        self.assertEqual(len(lock["accepted_pairs"]), 1)
        pair = lock["accepted_pairs"][0]
        self.assertEqual(pair["status"], "qualified")
        self.assertEqual(pair["gc"]["repository"], "https://github.com/gastownhall/gascity.git")
        self.assertEqual(pair["gc"]["source_commit"], GC_COMMIT)
        self.assertEqual(pair["bd"]["repository"], "https://github.com/steveyegge/beads.git")
        self.assertEqual(pair["bd"]["source_commit"], BD_COMMIT)
        bootstrap = (DEPLOY / "bootstrap.sh").read_text(encoding="utf-8")
        self.assertIn(GC_COMMIT, bootstrap)
        self.assertIn(BD_COMMIT, bootstrap)
        self.assertIn('--prefix "$bead_prefix" --start-suspended --adopt', bootstrap)
        self.assertNotIn('"$bd_bin" init', bootstrap)

    # --- Offline materialize-toolchain fixtures -------------------------------
    #
    # materialize-toolchain.sh downloads official prebuilt release archives and
    # verifies them against the pinned official checksums. These fixtures drive
    # the identical verification chain fully offline: AGENTOPS_GC_ARCHIVE_CACHE
    # supplies pre-placed assets instead of the network, and --lock supplies a
    # fixture lock that pins the fabricated fixture shas. No test hits GitHub.

    @staticmethod
    def platform_tag() -> str | None:
        system = {"Darwin": "darwin", "Linux": "linux"}.get(platform.system())
        machine = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64", "amd64": "amd64"}.get(platform.machine())
        if not system or not machine:
            return None
        return f"{system}_{machine}"

    def build_offline_fixture(
        self,
        root: Path,
        *,
        gc_reported_version: str = "1.3.5",
        gc_reported_commit: str = "8ffc009ded781a2ada2077f3a29bd712b2def0bf",
        bd_reported_version: str = "1.1.0",
        bd_reported_commit: str = "8e4e59d39",
    ) -> dict[str, object]:
        # The lock always pins the correct locked identity; only what the stub
        # binaries *report* varies. The checksum chain stays internally valid for
        # any stub content (checksums file and lock are derived from the actual
        # archive digests), so a reported-identity mismatch isolates the final
        # version/commit binding step from the earlier hash links.
        tag = self.platform_tag()
        if tag is None:
            self.skipTest(f"no release-archive mapping for {platform.system()}/{platform.machine()}")
        cache = root / "cache"
        cache.mkdir()
        gc_script = (
            '#!/bin/sh\n'
            'if [ "$1" = "version" ] && [ "$2" = "--json" ]; then\n'
            '  printf \'{"version":"' + gc_reported_version + '","commit":"'
            + gc_reported_commit + '-dirty","ok":true}\\n\'\n'
            '  exit 0\n'
            'fi\n'
            'exit 1\n'
        )
        bd_script = (
            '#!/bin/sh\n'
            'if [ "$1" = "version" ]; then\n'
            '  printf \'bd version ' + bd_reported_version + ' ('
            + bd_reported_commit + ': fixture)\\n\'\n'
            '  exit 0\n'
            'fi\n'
            'exit 1\n'
        )

        def make_archive(archive_name: str, binname: str, script: str) -> Path:
            src = root / (binname + "_src")
            src.mkdir(exist_ok=True)
            binpath = src / binname
            binpath.write_text(script, encoding="utf-8")
            binpath.chmod(0o755)
            archive_path = cache / archive_name
            with tarfile.open(archive_path, "w:gz") as tf:
                tf.add(binpath, arcname=binname)
            return archive_path

        def sha256(path: Path) -> str:
            return hashlib.sha256(path.read_bytes()).hexdigest()

        gc_archive_name = f"gascity_1.3.5_{tag}.tar.gz"
        bd_archive_name = f"beads_1.1.0_{tag}.tar.gz"
        gc_archive = make_archive(gc_archive_name, "gc", gc_script)
        bd_archive = make_archive(bd_archive_name, "bd", bd_script)

        gc_ck = cache / "gascity_1.3.5_checksums.txt"
        gc_ck.write_text(f"{sha256(gc_archive)}  {gc_archive_name}\n", encoding="utf-8")
        bd_ck = cache / "checksums.txt"
        bd_ck.write_text(f"{sha256(bd_archive)}  {bd_archive_name}\n", encoding="utf-8")

        lock = {
            "schema_version": 1,
            "accepted_pairs": [
                {
                    "id": "gascity-v1.3.5-beads-v1.1.0",
                    "status": "qualified",
                    "gc": {
                        "repository": "https://github.com/gastownhall/gascity.git",
                        "ref": "v1.3.5",
                        "version": "1.3.5",
                        "source_commit": GC_COMMIT,
                        "runtime_commit": "8ffc009d",
                        "release_checksum_asset": "gascity_1.3.5_checksums.txt",
                        "release_checksum_sha256": sha256(gc_ck),
                    },
                    "bd": {
                        "repository": "https://github.com/steveyegge/beads.git",
                        "ref": "v1.1.0",
                        "version": "1.1.0",
                        "source_commit": BD_COMMIT,
                        "runtime_commit": "8e4e59d39",
                        "release_checksum_asset": "checksums.txt",
                        "release_checksum_sha256": sha256(bd_ck),
                    },
                }
            ],
        }
        lock_path = root / "toolchain.lock.json"
        lock_path.write_text(json.dumps(lock, indent=2), encoding="utf-8")
        return {
            "cache": cache,
            "lock": lock_path,
            "gc_ck": gc_ck,
            "bd_ck": bd_ck,
            "gc_archive_name": gc_archive_name,
            "bd_archive_name": bd_archive_name,
            "tag": tag,
        }

    def run_materialize(self, output: Path, lock: Path, cache: Path, extra_env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
        env = os.environ.copy()
        env["AGENTOPS_GC_ARCHIVE_CACHE"] = str(cache)
        if extra_env:
            env.update(extra_env)
        return run(
            str(DEPLOY / "materialize-toolchain.sh"), "--output", str(output), "--lock", str(lock),
            env=env,
        )

    def test_materialize_offline_happy_path_installs_verified_pair(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            fixture = self.build_offline_fixture(root)
            output = root / "toolchain"
            result = self.run_materialize(output, fixture["lock"], fixture["cache"])
            self.assertEqual(result.returncode, 0, result.stderr)
            gc_bin = output / "bin/gc"
            bd_bin = output / "bin/bd"
            self.assertTrue(gc_bin.is_file())
            self.assertTrue(bd_bin.is_file())
            self.assertTrue(gc_bin.stat().st_mode & stat.S_IXUSR)
            self.assertTrue(bd_bin.stat().st_mode & stat.S_IXUSR)
            receipt = json.loads((output / "toolchain.json").read_text(encoding="utf-8"))
            self.assertEqual(receipt["schema_version"], 2)
            self.assertEqual(set(receipt["runtime"]), {"gc", "bd"})
            self.assertEqual(receipt["runtime"]["gc"]["path"], "bin/gc")
            self.assertEqual(receipt["runtime"]["bd"]["path"], "bin/bd")
            self.assertEqual(receipt["runtime"]["gc"]["version"], "1.3.5")
            self.assertEqual(receipt["runtime"]["bd"]["version"], "1.1.0")
            self.assertEqual(receipt["runtime"]["gc"]["commit"], GC_COMMIT)
            self.assertEqual(receipt["runtime"]["bd"]["commit"], "8e4e59d39")
            self.assertEqual(receipt["runtime"]["gc"]["sha256"], hashlib.sha256(gc_bin.read_bytes()).hexdigest())
            self.assertEqual(receipt["runtime"]["bd"]["sha256"], hashlib.sha256(bd_bin.read_bytes()).hexdigest())
            self.assertEqual(receipt["pair"]["status"], "qualified")
            self.assertEqual(receipt["pair"]["gc"]["source_commit"], GC_COMMIT)
            self.assertEqual(receipt["pair"]["bd"]["source_commit"], BD_COMMIT)

    def test_materialize_offline_rejects_checksums_file_tamper(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            fixture = self.build_offline_fixture(root)
            lock_path = fixture["lock"]
            lock = json.loads(lock_path.read_text(encoding="utf-8"))
            lock["accepted_pairs"][0]["gc"]["release_checksum_sha256"] = "0" * 64
            lock_path.write_text(json.dumps(lock, indent=2), encoding="utf-8")
            output = root / "toolchain"
            result = self.run_materialize(output, lock_path, fixture["cache"])
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("checksums asset", result.stderr)
            self.assertIn("sha256 mismatch", result.stderr)
            self.assertFalse(output.exists())

    def test_materialize_offline_rejects_archive_tamper(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            fixture = self.build_offline_fixture(root)
            # Point the checksums file at a wrong archive digest, then re-pin the
            # lock to the tampered checksums file so its own digest still passes.
            gc_ck = fixture["gc_ck"]
            tampered = f"{'0' * 64}  {fixture['gc_archive_name']}\n"
            gc_ck.write_text(tampered, encoding="utf-8")
            lock_path = fixture["lock"]
            lock = json.loads(lock_path.read_text(encoding="utf-8"))
            lock["accepted_pairs"][0]["gc"]["release_checksum_sha256"] = hashlib.sha256(tampered.encode()).hexdigest()
            lock_path.write_text(json.dumps(lock, indent=2), encoding="utf-8")
            output = root / "toolchain"
            result = self.run_materialize(output, lock_path, fixture["cache"])
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("archive", result.stderr)
            self.assertIn("sha256 mismatch", result.stderr)
            self.assertFalse(output.exists())

    def test_materialize_rejects_unsupported_platform(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            fixture = self.build_offline_fixture(root)
            stub_dir = root / "stub"
            stub_dir.mkdir()
            uname = stub_dir / "uname"
            uname.write_text(
                '#!/bin/sh\nif [ "$1" = "-s" ]; then echo Plan9; else echo sparc; fi\n',
                encoding="utf-8",
            )
            uname.chmod(0o755)
            output = root / "toolchain"
            result = self.run_materialize(
                output, fixture["lock"], fixture["cache"],
                extra_env={"PATH": f"{stub_dir}:{os.environ['PATH']}"},
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("unsupported operating system", result.stderr)
            self.assertFalse(output.exists())

    def test_materialize_offline_rejects_gc_commit_mismatch(self) -> None:
        # Valid checksum chain, but the installed gc binary reports a different
        # commit than the lock pins. Exercises the gc version/commit binding.
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            fixture = self.build_offline_fixture(
                root, gc_reported_commit="1234567890abcdef1234567890abcdef12345678",
            )
            output = root / "toolchain"
            result = self.run_materialize(output, fixture["lock"], fixture["cache"])
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("installed gc commit", result.stderr)
            self.assertFalse(output.exists())

    def test_materialize_offline_rejects_bd_version_mismatch(self) -> None:
        # Valid checksum chain, but the installed bd binary reports a different
        # version than the lock pins. Exercises the bd version/commit binding.
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            fixture = self.build_offline_fixture(root, bd_reported_version="9.9.9")
            output = root / "toolchain"
            result = self.run_materialize(output, fixture["lock"], fixture["cache"])
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("installed bd version", result.stderr)
            self.assertIn("9.9.9", result.stderr)
            self.assertFalse(output.exists())

    def test_executable_helper_budget_and_shell_syntax(self) -> None:
        scripts = sorted(DEPLOY.glob("*.sh"))
        line_count = sum(len(path.read_text(encoding="utf-8").splitlines()) for path in scripts)
        self.assertLessEqual(line_count, 2_000)
        for path in scripts:
            self.assertTrue(path.stat().st_mode & stat.S_IXUSR, path)
            result = run("/bin/bash", "-n", str(path))
            self.assertEqual(result.returncode, 0, result.stderr)

    def test_lifecycle_helpers_export_the_pinned_gc_binary(self) -> None:
        for name in ("bootstrap.sh", "invoke.sh", "teardown.sh"):
            text = (DEPLOY / name).read_text(encoding="utf-8")
            self.assertIn("export GC_BIN\n", text, name)

    def test_intake_never_routes_through_the_city_scoped_mayor(self) -> None:
        # Static consistency proof: intake (feed) homes rig beads on the rig
        # planner and never slings the source bead to agentops.mayor. The Mayor
        # is a dispatch shepherd (it slings ready STEP beads to their rig
        # run-targets), so it must never be a sling *target* on the intake path.
        surfaces = list(DEPLOY.glob("*.sh")) + list(FACTORY.rglob("*.md"))
        for path in surfaces:
            text = path.read_text(encoding="utf-8")
            self.assertNotIn("sling agentops.mayor", text, path)
        mayor_prompt = (FACTORY / "agents/mayor/prompt.template.md").read_text(encoding="utf-8")
        # Doctrine invariant: the Mayor dispatches but never claims. It forbids
        # the cross-store claim and never carries an instruction to run it.
        self.assertIn("Do not run `gc hook --claim`", mayor_prompt)
        self.assertNotIn('Run `"$GC_BIN" hook --claim', mayor_prompt)

    @staticmethod
    def _sha256_file(path: Path) -> str:
        return hashlib.sha256(path.read_bytes()).hexdigest()

    @staticmethod
    def _tree_sha(root: Path) -> str:
        h = hashlib.sha256()
        for parent, dirs, files in os.walk(root):
            dirs.sort()
            for name in sorted(files):
                p = os.path.join(parent, name)
                rel = os.path.relpath(p, root)
                h.update(rel.encode() + b"\0" + Path(p).read_bytes() + b"\0")
        return h.hexdigest()

    # gc/bd stubs record one argv element per line so tests assert on exact argv
    # arrays (argument boundaries and quoting survive), never a flattened "$*".
    # Behavior is driven by env; the fixture records the stub's digest
    # dynamically, so the content may change freely.
    #
    # Fidelity: the gc stub REJECTS (nonzero + stderr) any first verb not on the
    # allowlist of verified v1.3.5 surfaces the pack actually drives through gc
    # (bd goes through the separate bd stub). This makes a test fail loudly if the
    # pack ever calls an unverified gc verb. GC_STUB_FAIL=<verb> forces a nonzero
    # exit for an allowlisted verb, to exercise fail-closed error propagation.
    _GC_STUB = (
        "#!/usr/bin/env python3\n"
        "import os, sys\n"
        "argv = sys.argv[1:]\n"
        "log = os.environ.get('GC_ARGV_LOG')\n"
        "if log:\n"
        "    with open(log, 'a', encoding='utf-8') as f:\n"
        "        f.write('\\n'.join(argv) + '\\n')\n"
        "allowed = {'sling', 'status', 'doctor', 'session', 'mail'}\n"
        "if not argv or argv[0] not in allowed:\n"
        "    sys.stderr.write('gc stub: refusing non-allowlisted command: %s\\n' % (argv[0] if argv else '<none>'))\n"
        "    sys.exit(2)\n"
        "fail = os.environ.get('GC_STUB_FAIL')\n"
        "if fail and argv[0] == fail:\n"
        "    sys.stderr.write('gc stub: forced failure for %s\\n' % fail)\n"
        "    sys.exit(1)\n"
        "sys.exit(0)\n"
    )
    _BD_STUB = (
        "#!/usr/bin/env python3\n"
        "import json, os, sys\n"
        "argv = sys.argv[1:]\n"
        "log = os.environ.get('BD_ARGV_LOG')\n"
        "if log:\n"
        "    with open(log, 'a', encoding='utf-8') as f:\n"
        "        f.write('\\n'.join(argv) + '\\n---\\n')\n"
        "mode = os.environ.get('BD_STUB_MODE', 'ok')\n"
        "if 'create' in argv:\n"
        "    print(json.dumps({'id': 'testrig-9'})); sys.exit(0)\n"
        "if 'show' in argv:\n"
        "    bead = argv[argv.index('show') + 1]\n"
        "    if mode == 'notfound':\n"
        "        sys.stderr.write('Issue %s not found\\n' % bead); sys.exit(1)\n"
        "    returned = 'totally-different' if mode == 'mismatch' else bead\n"
        "    print(json.dumps([{'id': returned}])); sys.exit(0)\n"
        "sys.exit(2)\n"
    )

    def _invoke_env(
        self,
        *,
        gc_log: Path | None = None,
        bd_log: Path | None = None,
        bd_mode: str = "ok",
        gc_fail: str | None = None,
    ) -> dict[str, str]:
        env = os.environ.copy()
        if gc_log is not None:
            env["GC_ARGV_LOG"] = str(gc_log)
        if bd_log is not None:
            env["BD_ARGV_LOG"] = str(bd_log)
        env["BD_STUB_MODE"] = bd_mode
        if gc_fail is not None:
            env["GC_STUB_FAIL"] = gc_fail
        return env

    def _managed_city_fixture(self, base: Path, *, rig: Path) -> dict[str, object]:
        """Build a managed-city surface invoke.sh accepts, anchored to the real
        pack lock and the toolchain receipt (not the mutable marker).

        gc and bd are stub binaries whose digests are recorded in a schema-2
        receipt whose pinned pair matches deploy/gc/toolchain.lock.json. The
        worktree helper is the real deploy/gc/worktree.sh, so feed's provenance
        check passes and a real isolated worktree is prepared, fully offline.
        """
        toolchain = base / "toolchain"
        (toolchain / "bin").mkdir(parents=True)
        gc_bin = toolchain / "bin/gc"
        gc_bin.write_text(self._GC_STUB, encoding="utf-8")
        gc_bin.chmod(0o755)
        bd_bin = toolchain / "bin/bd"
        bd_bin.write_text(self._BD_STUB, encoding="utf-8")
        bd_bin.chmod(0o755)
        receipt = {
            "schema_version": 2,
            "runtime": {
                "gc": {"path": "bin/gc", "sha256": self._sha256_file(gc_bin)},
                "bd": {"path": "bin/bd", "sha256": self._sha256_file(bd_bin)},
            },
            "pair": {
                "status": "qualified",
                "gc": {"source_commit": GC_COMMIT},
                "bd": {"source_commit": BD_COMMIT},
            },
        }
        (toolchain / "toolchain.json").write_text(json.dumps(receipt), encoding="utf-8")

        city = base / "city"
        (city / ".gc/agentops-packs/agentops-factory").mkdir(parents=True)
        (city / ".gc/agentops-packs/agentops-factory/pack.toml").write_text("name = 'x'\n", encoding="utf-8")
        (city / ".gc/scripts").mkdir(parents=True)
        (city / ".beads").mkdir(parents=True)
        (city / "city.toml").write_text("[workspace]\n", encoding="utf-8")
        (city / ".beads/dolt-server.port").write_text("3306\n", encoding="utf-8")
        installed_wt = city / ".gc/scripts/agentops-worktree"
        installed_wt.write_bytes((DEPLOY / "worktree.sh").read_bytes())
        installed_wt.chmod(0o755)

        workers = base / "workers"
        marker = city / ".gc/agentops-bootstrap.json"
        marker.write_text(json.dumps({
            "schema_version": 1,
            "state": "ready",
            "city": str(city.resolve()),
            "rig": str(rig.resolve()),
            "rig_name": "testrig",
            "base_ref": "main",
            "worktree_root": str(workers.resolve()),
            "bead_database": "testdb",
            "pack_snapshot": str((city / ".gc/agentops-packs/agentops-factory").resolve()),
            "pack_sha256": self._tree_sha(city / ".gc/agentops-packs"),
            "city_config_sha256": self._sha256_file(city / "city.toml"),
            "toolchain": {
                "gc": {"path": str(gc_bin.resolve()), "sha256": self._sha256_file(gc_bin)},
                "bd": {"path": str(bd_bin.resolve()), "sha256": self._sha256_file(bd_bin)},
            },
            "telemetry": {"metrics_url": "", "logs_url": "", "sdk_disabled": True},
        }), encoding="utf-8")
        return {"city": city, "rig": rig, "workers": workers, "gc_bin": gc_bin, "bd_bin": bd_bin, "toolchain": toolchain}

    def test_feed_verifies_bead_then_slings_full_flag_set(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary) / "has space"  # exercises argv quoting
            base.mkdir()
            repo, _ = self.make_repository(base)
            fixture = self._managed_city_fixture(base, rig=repo)
            gc_log = base / "gc.argv"
            bd_log = base / "bd.argv"
            env = self._invoke_env(gc_log=gc_log, bd_log=bd_log)
            result = run(
                str(DEPLOY / "invoke.sh"), "--city", str(fixture["city"]), "feed", "testrig-1",
                env=env,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            # Bead existence was verified in the rig store before any sling.
            self.assertEqual(
                bd_log.read_text(encoding="utf-8").splitlines(),
                ["-C", str(repo.resolve()), "show", "testrig-1", "--json", "---"],
            )
            # The full production sling flag set, one argv element per line.
            expected_worktree = str(Path(os.path.realpath(fixture["workers"])) / "testrig-1")
            self.assertEqual(gc_log.read_text(encoding="utf-8").splitlines(), [
                "sling", "testrig/agentops.plan-reviewer", "testrig-1",
                "--on", "agentops-experiment", "--nudge", "--json",
                "--var", f"work_dir={expected_worktree}",
                "--var", "plan_target=testrig/agentops.plan-reviewer",
                "--var", "implement_target=testrig/agentops.implementer",
                "--var", "validate_target=testrig/agentops.validator",
                "--var", "refiner_target=testrig/agentops.refiner",
            ])
            # The worktree was really prepared by the pinned pack helper.
            self.assertTrue((Path(expected_worktree) / ".git").exists())

    def test_feed_refuses_missing_bead_without_slinging(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary) / "has space"
            base.mkdir()
            repo, _ = self.make_repository(base)
            fixture = self._managed_city_fixture(base, rig=repo)
            gc_log = base / "gc.argv"
            env = self._invoke_env(gc_log=gc_log, bd_log=base / "bd.argv", bd_mode="notfound")
            result = run(
                str(DEPLOY / "invoke.sh"), "--city", str(fixture["city"]), "feed", "testrig-404",
                env=env,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("bead not found in the rig store", result.stderr)
            self.assertFalse(gc_log.exists(), "sling must not run for a missing bead")
            self.assertFalse((Path(str(fixture["workers"])) / "testrig-404").exists())

    def test_feed_refuses_bead_id_mismatch_without_slinging(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary) / "has space"
            base.mkdir()
            repo, _ = self.make_repository(base)
            fixture = self._managed_city_fixture(base, rig=repo)
            gc_log = base / "gc.argv"
            env = self._invoke_env(gc_log=gc_log, bd_log=base / "bd.argv", bd_mode="mismatch")
            result = run(
                str(DEPLOY / "invoke.sh"), "--city", str(fixture["city"]), "feed", "testrig-1",
                env=env,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("exact single match", result.stderr)
            self.assertFalse(gc_log.exists(), "sling must not run on an inexact match")

    def test_feed_rejects_unsafe_bead_id(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary)
            rig = base / "rig"
            rig.mkdir()
            fixture = self._managed_city_fixture(base, rig=rig)
            result = run(
                str(DEPLOY / "invoke.sh"), "--city", str(fixture["city"]), "feed", "bad id;rm",
                env=self._invoke_env(),
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("unsafe bead id", result.stderr)

    def test_create_writes_managed_rig_store_and_prints_id(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary) / "has space"  # exercises argv quoting
            base.mkdir()
            rig = base / "rig"
            rig.mkdir()
            fixture = self._managed_city_fixture(base, rig=rig)
            bd_log = base / "bd.argv"
            env = self._invoke_env(bd_log=bd_log)
            plain = run(
                str(DEPLOY / "invoke.sh"), "--city", str(fixture["city"]),
                "create", "add a widget", "-d", "with a description", env=env,
            )
            self.assertEqual(plain.returncode, 0, plain.stderr)
            self.assertEqual(plain.stdout, "testrig-9\n")
            first_call = bd_log.read_text(encoding="utf-8").split("---\n")[0].splitlines()
            self.assertEqual(first_call, [
                "-C", str(rig.resolve()), "create", "--title", "add a widget",
                "--type", "task", "--json", "-d", "with a description",
            ])

            json_out = run(
                str(DEPLOY / "invoke.sh"), "--city", str(fixture["city"]),
                "create", "no description", "--json", env=env,
            )
            self.assertEqual(json_out.returncode, 0, json_out.stderr)
            self.assertEqual(json.loads(json_out.stdout)["id"], "testrig-9")
            # No description supplied -> no -d flag forwarded to bd (exact argv).
            calls = [c for c in bd_log.read_text(encoding="utf-8").split("---\n") if c.strip()]
            self.assertEqual(calls[-1].splitlines(), [
                "-C", str(rig.resolve()), "create", "--title", "no description",
                "--type", "task", "--json",
            ])

    def test_mayor_start_wakes_the_named_shepherd_session(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary) / "has space"  # exercises argv quoting
            base.mkdir()
            rig = base / "rig"
            rig.mkdir()
            fixture = self._managed_city_fixture(base, rig=rig)
            gc_log = base / "gc.argv"
            result = run(
                str(DEPLOY / "invoke.sh"), "--city", str(fixture["city"]), "mayor", "start",
                env=self._invoke_env(gc_log=gc_log),
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            # Native start verb: wake the city-scoped mayor named session.
            self.assertEqual(
                gc_log.read_text(encoding="utf-8").splitlines(),
                ["session", "wake", "agentops.mayor"],
            )

    def test_mayor_tell_delivers_a_notified_mail_message(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary) / "has space"
            base.mkdir()
            rig = base / "rig"
            rig.mkdir()
            fixture = self._managed_city_fixture(base, rig=rig)
            gc_log = base / "gc.argv"
            result = run(
                str(DEPLOY / "invoke.sh"), "--city", str(fixture["city"]),
                "mayor", "tell", "dispatch testrig-12",
                env=self._invoke_env(gc_log=gc_log),
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            # The clean v1.3.5 primitive: a notified mail message to the mayor.
            # The whole quoted instruction survives as one argv element.
            self.assertEqual(
                gc_log.read_text(encoding="utf-8").splitlines(),
                ["mail", "send", "--to", "agentops.mayor", "--notify", "-m", "dispatch testrig-12"],
            )

    def test_mayor_tell_refuses_empty_message_without_calling_gc(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary) / "has space"
            base.mkdir()
            rig = base / "rig"
            rig.mkdir()
            fixture = self._managed_city_fixture(base, rig=rig)
            gc_log = base / "gc.argv"
            for empty in ("", "   "):
                result = run(
                    str(DEPLOY / "invoke.sh"), "--city", str(fixture["city"]),
                    "mayor", "tell", empty,
                    env=self._invoke_env(gc_log=gc_log),
                )
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("non-empty message", result.stderr)
            self.assertFalse(gc_log.exists(), "no mail must be sent for an empty message")

    def test_mayor_status_reports_the_socket_before_the_session_exists(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary) / "has space"
            base.mkdir()
            rig = base / "rig"
            rig.mkdir()
            fixture = self._managed_city_fixture(base, rig=rig)
            gc_log = base / "gc.argv"
            result = run(
                str(DEPLOY / "invoke.sh"), "--city", str(fixture["city"]), "mayor", "status",
                env=self._invoke_env(gc_log=gc_log),
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            # Status reads the native session list (stub returns none -> not started).
            self.assertEqual(
                gc_log.read_text(encoding="utf-8").splitlines(),
                ["session", "list", "--state", "all", "--json"],
            )
            expected_socket = "agentops-" + hashlib.sha256(
                os.path.realpath(str(fixture["city"])).encode()
            ).hexdigest()[:20]
            self.assertIn("not started", result.stdout)
            self.assertIn(expected_socket, result.stdout)

    def test_mayor_rejects_unknown_subcommand(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary)
            rig = base / "rig"
            rig.mkdir()
            fixture = self._managed_city_fixture(base, rig=rig)
            result = run(
                str(DEPLOY / "invoke.sh"), "--city", str(fixture["city"]), "mayor", "claim",
                env=self._invoke_env(),
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("unknown mayor operation", result.stderr)

    def test_mayor_status_fails_closed_when_gc_query_fails(self) -> None:
        # Tri-state case (a): a failed gc query must NOT read as "no session".
        # It exits nonzero with "status unavailable" and surfaces the stderr tail.
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary) / "has space"
            base.mkdir()
            rig = base / "rig"
            rig.mkdir()
            fixture = self._managed_city_fixture(base, rig=rig)
            result = run(
                str(DEPLOY / "invoke.sh"), "--city", str(fixture["city"]), "mayor", "status",
                env=self._invoke_env(gc_fail="session"),
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("status unavailable", result.stderr)
            self.assertNotIn("not started", result.stdout)

    def test_gc_stub_rejects_non_allowlisted_verbs(self) -> None:
        # Fidelity guard: the pack must only drive gc through verified v1.3.5
        # verbs. A non-allowlisted verb passthrough fails loudly.
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary)
            rig = base / "rig"
            rig.mkdir()
            fixture = self._managed_city_fixture(base, rig=rig)
            result = run(
                str(DEPLOY / "invoke.sh"), "--city", str(fixture["city"]),
                "--", "totally-made-up-verb",
                env=self._invoke_env(),
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("non-allowlisted command", result.stderr)

    def test_heartbeat_order_is_a_valid_cooldown_nudge(self) -> None:
        # Finding 1: liveness. The pack ships a scheduled order that re-nudges the
        # Mayor. Schema-lint it against the v1.3.5 order surface and the 2-5m band.
        order_path = FACTORY / "orders/shepherd-heartbeat.toml"
        self.assertTrue(order_path.is_file(), order_path)
        order = tomllib.loads(order_path.read_text(encoding="utf-8"))["order"]
        self.assertEqual(order["trigger"], "cooldown")
        self.assertEqual(order.get("scope"), "city")
        # Interval must be a Go duration in the 2-5 minute band.
        self.assertRegex(order["interval"], r"^[1-9][0-9]*m$")
        minutes = int(order["interval"][:-1])
        self.assertGreaterEqual(minutes, 2)
        self.assertLessEqual(minutes, 5)
        # It nudges the Mayor via the same mail-send primitive, with --notify.
        exec_cmd = order["exec"]
        self.assertIn("gc mail send", exec_cmd)
        self.assertIn("--to agentops.mayor", exec_cmd)
        self.assertIn("--notify", exec_cmd)

    def test_routed_bead_recovery_is_wake_not_resling(self) -> None:
        # Finding 3 sanity: the docs must state that re-dispatching an already
        # routed bead is a no-op, and that recovery is waking the worker session —
        # so an agent does not cargo-cult a dead re-sling retry.
        skill = (REPO_ROOT / "skills/using-gc/SKILL.md").read_text(encoding="utf-8")
        self.assertIn("NO-OP", skill)
        self.assertIn("gc session wake <run_target>", skill)
        prompt = (FACTORY / "agents/mayor/prompt.template.md").read_text(encoding="utf-8")
        self.assertIn("re-slinging a routed bead is a NO-OP", prompt)
        self.assertIn("gc session wake <run_target>", prompt)

    def test_mayor_mail_authority_has_no_pause_resume(self) -> None:
        # Finding 5: mail authority is exactly dispatch/status; pause/resume is
        # dropped (stateful, incompatible with wake_mode=fresh), and mail bodies
        # are untrusted data, not instructions.
        prompt = (FACTORY / "agents/mayor/prompt.template.md").read_text(encoding="utf-8")
        self.assertNotIn("resume shepherding", prompt)  # the old stateful verb
        self.assertIn("no pause/resume", prompt.lower())
        self.assertIn("UNTRUSTED DATA", prompt)
        self.assertIn("NEVER executed", prompt)

    def test_named_mayor_session_is_a_standing_always_shepherd(self) -> None:
        # The 3.3 human-and-agent door needs a resident session: mode=always so
        # the controller keeps it live with no demand, and no one_shot lifecycle
        # so the pane stays attachable rather than exiting after one run.
        pack = tomllib.loads((FACTORY / "pack.toml").read_text(encoding="utf-8"))
        mayor_sessions = [
            s for s in pack.get("named_session", []) if s.get("template") == "mayor"
        ]
        self.assertEqual(len(mayor_sessions), 1)
        self.assertEqual(mayor_sessions[0].get("scope"), "city")
        self.assertEqual(mayor_sessions[0].get("mode"), "always")
        agent = tomllib.loads((FACTORY / "agents/mayor/agent.toml").read_text(encoding="utf-8"))
        self.assertNotIn("lifecycle", agent)
        self.assertLessEqual(agent.get("min_active_sessions", 0), agent["max_active_sessions"])

    def test_teardown_waits_for_scoped_drain_and_fails_with_exact_residue(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            city = root / "city"
            marker = city / ".gc/agentops-bootstrap.json"
            marker.parent.mkdir(parents=True)
            fake_gc = root / "gc-stub.sh"
            calls = root / "gc.log"
            fake_gc.write_text(textwrap.dedent("""\
                #!/bin/sh
                printf '%s\\t%s\\n' "${GC_BIN:-}" "$*" >> "$FAKE_GC_LOG"
                exit 0
                """), encoding="utf-8")
            fake_gc.chmod(0o755)
            marker.write_text(json.dumps({
                "schema_version": 1,
                "city": str(city.resolve()),
                "toolchain": {"gc": {"path": str(fake_gc.resolve())}},
                "supervisor_port": 65534,
                "telemetry": {"sdk_disabled": True},
            }), encoding="utf-8")
            env = os.environ.copy()
            env["FAKE_GC_LOG"] = str(calls)

            draining = subprocess.Popen([
                "python3", "-c", "import time; time.sleep(0.8)", str(city.resolve()),
            ])
            started = time.monotonic()
            result = run(
                str(DEPLOY / "teardown.sh"), "--city", str(city), "--wait-timeout", "3",
                env=env,
            )
            elapsed = time.monotonic() - started
            draining.wait(timeout=3)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertGreaterEqual(elapsed, 0.3)
            log = calls.read_text(encoding="utf-8")
            self.assertIn(f"{fake_gc.resolve()}\t--city {city.resolve()} stop --force", log)
            self.assertIn(f"{fake_gc.resolve()}\tsupervisor stop --wait --wait-timeout 3s", log)

            stuck = subprocess.Popen([
                "python3", "-c", "import time; time.sleep(10)", str(city.resolve()),
            ])
            try:
                failed = run(
                    str(DEPLOY / "teardown.sh"), "--city", str(city), "--wait-timeout", "1",
                    env=env,
                )
                self.assertNotEqual(failed.returncode, 0)
                self.assertIn("managed city processes remain", failed.stderr)
                self.assertIn(str(city.resolve()), failed.stderr)
            finally:
                stuck.terminate()
                stuck.wait(timeout=3)

    def test_worktree_helper_creates_one_isolated_idempotent_branch(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            repo, _ = self.make_repository(root)
            workers = root / "workers"
            command = (
                str(DEPLOY / "worktree.sh"), "prepare", "--repo", str(repo), "--root", str(workers),
                "--bead", "age-test.1", "--base-ref", "main",
            )
            first = run(*command)
            self.assertEqual(first.returncode, 0, first.stderr)
            second = run(*command)
            self.assertEqual(second.returncode, 0, second.stderr)
            self.assertEqual(json.loads(first.stdout), json.loads(second.stdout))
            receipt = json.loads(first.stdout)
            self.assertEqual(receipt["branch"], "gc/age-test.1")
            self.assertNotEqual(Path(receipt["worktree"]).resolve(), repo.resolve())

            other = run(
                str(DEPLOY / "worktree.sh"), "prepare", "--repo", str(repo), "--root", str(workers),
                "--bead", "age-test.2", "--base-ref", "main",
            )
            self.assertEqual(other.returncode, 0, other.stderr)
            other_receipt = json.loads(other.stdout)
            self.assertEqual(other_receipt["branch"], "gc/age-test.2")
            self.assertNotEqual(other_receipt["worktree"], receipt["worktree"])
            self.assertNotEqual(
                git("rev-parse", "--git-dir", cwd=Path(other_receipt["worktree"])).stdout.strip(),
                git("rev-parse", "--git-dir", cwd=Path(receipt["worktree"])).stdout.strip(),
            )

    def test_refiner_uses_hosted_checks_and_respects_auto_merge(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            repo, _ = self.make_repository(root)
            workers = root / "workers"
            prepared = run(
                str(DEPLOY / "worktree.sh"), "prepare", "--repo", str(repo), "--root", str(workers),
                "--bead", "age-refine.1", "--base-ref", "main",
            )
            self.assertEqual(prepared.returncode, 0, prepared.stderr)
            worktree = Path(json.loads(prepared.stdout)["worktree"])
            self.assertEqual(git("config", "user.name", "AgentOps Test", cwd=worktree).returncode, 0)
            self.assertEqual(git("config", "user.email", "agentops-test@example.invalid", cwd=worktree).returncode, 0)
            (worktree / "candidate.txt").write_text("validated\n", encoding="utf-8")
            self.assertEqual(git("add", "candidate.txt", cwd=worktree).returncode, 0)
            self.assertEqual(git("commit", "-m", "candidate", cwd=worktree).returncode, 0)
            head = git("rev-parse", "HEAD", cwd=worktree).stdout.strip()
            (repo / "moving-main.txt").write_text("unrelated\n", encoding="utf-8")
            self.assertEqual(git("add", "moving-main.txt", cwd=repo).returncode, 0)
            self.assertEqual(git("commit", "-m", "move main", cwd=repo).returncode, 0)
            self.assertEqual(git("push", "origin", "main", cwd=repo).returncode, 0)

            state = root / "gh-state"
            log = root / "gh.log"
            git_log = root / "git.log"
            git_stub = root / "git-stub.py"
            git_stub.write_text(textwrap.dedent("""\
                #!/usr/bin/env python3
                import os, pathlib, sys
                with pathlib.Path(os.environ["GIT_LOG"]).open("a", encoding="utf-8") as handle:
                    handle.write(" ".join(sys.argv[1:]) + "\\n")
                os.execv(os.environ["REAL_GIT"], [os.environ["REAL_GIT"], *sys.argv[1:]])
                """), encoding="utf-8")
            git_stub.chmod(0o755)
            gh = root / "gh-stub.py"
            gh.write_text(textwrap.dedent("""\
                #!/usr/bin/env python3
                import json, os, pathlib, subprocess, sys
                args = sys.argv[1:]
                state = pathlib.Path(os.environ["GH_STATE"])
                log = pathlib.Path(os.environ["GH_LOG"])
                with log.open("a", encoding="utf-8") as handle: handle.write(" ".join(args) + "\\n")
                if args[:2] == ["pr", "list"]: print("[]")
                elif args[:2] == ["pr", "create"]: print("https://example.invalid/pull/7")
                elif args[:2] == ["pr", "checks"]: pass
                elif args[:2] == ["pr", "merge"]: state.write_text("MERGED", encoding="utf-8")
                elif args[:2] == ["pr", "view"]:
                    merged = state.exists()
                    if args[-1] == "state,mergeStateStatus":
                        print(json.dumps({"state": "MERGED" if merged else "OPEN", "mergeStateStatus": "CLEAN"}))
                    else:
                        head = subprocess.check_output([os.environ["REAL_GIT"], "-C", os.environ["TEST_WORKTREE"], "rev-parse", "HEAD"], text=True).strip()
                        print(json.dumps({"number": 7, "url": "https://example.invalid/pull/7", "headRefOid": head, "state": "OPEN"}))
                else: raise SystemExit("unexpected gh invocation: " + " ".join(args))
                """), encoding="utf-8")
            gh.chmod(0o755)
            env = os.environ.copy()
            env.update({
                "AGENTOPS_GC_GIT_BIN": str(git_stub), "AGENTOPS_GC_GH_BIN": str(gh),
                "REAL_GIT": run("which", "git").stdout.strip(), "GIT_LOG": str(git_log),
                "GH_STATE": str(state), "GH_LOG": str(log), "TEST_WORKTREE": str(worktree),
            })
            result = run(
                str(DEPLOY / "refine.sh"), "--worktree", str(worktree), "--bead", "age-refine.1",
                "--base-ref", "main", "--mode", "auto", env=env,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue(result.stdout.strip(), f"empty receipt; stderr={result.stderr!r}")
            receipt = json.loads(result.stdout)
            self.assertEqual(receipt["mode"], "auto")
            self.assertEqual(receipt["rebases"], 1)
            self.assertEqual(receipt["validated_head"], head)
            self.assertNotEqual(receipt["head"], head)
            calls = log.read_text(encoding="utf-8")
            self.assertIn("pr checks 7 --watch --fail-fast", calls)
            self.assertIn("pr merge 7 --auto --squash --delete-branch", calls)
            self.assertTrue(state.exists())
            self.assertIn("fetch origin main --quiet", git_log.read_text(encoding="utf-8"))

            prepared_manual = run(
                str(DEPLOY / "worktree.sh"), "prepare", "--repo", str(repo), "--root", str(workers),
                "--bead", "age-refine.2", "--base-ref", "main",
            )
            self.assertEqual(prepared_manual.returncode, 0, prepared_manual.stderr)
            manual_worktree = Path(json.loads(prepared_manual.stdout)["worktree"])
            self.assertEqual(git("config", "user.name", "AgentOps Test", cwd=manual_worktree).returncode, 0)
            self.assertEqual(git("config", "user.email", "agentops-test@example.invalid", cwd=manual_worktree).returncode, 0)
            (manual_worktree / "manual.txt").write_text("validated\n", encoding="utf-8")
            self.assertEqual(git("add", "manual.txt", cwd=manual_worktree).returncode, 0)
            self.assertEqual(git("commit", "-m", "manual candidate", cwd=manual_worktree).returncode, 0)
            state.unlink()
            log.write_text("", encoding="utf-8")
            env["TEST_WORKTREE"] = str(manual_worktree)
            manual = run(
                str(DEPLOY / "refine.sh"), "--worktree", str(manual_worktree), "--bead", "age-refine.2",
                "--base-ref", "main", "--mode", "manual", env=env,
            )
            self.assertEqual(manual.returncode, 0, manual.stderr)
            self.assertEqual(json.loads(manual.stdout)["mode"], "manual")
            self.assertNotIn("pr merge", log.read_text(encoding="utf-8"))
            self.assertFalse(state.exists())

    def test_required_telemetry_fails_before_city_creation(self) -> None:
        if not PINNED_INTEGRATION:
            self.skipTest("set GC_BIN and AGENTOPS_GC_INTEGRATION=1 for native integration")
        gc_value = os.environ["GC_BIN"]
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            repo, _ = self.make_repository(root)
            city = root / "city"
            self.initialize_beads(repo, Path(gc_value).resolve())
            result = run(
                str(DEPLOY / "bootstrap.sh"), "--city", str(city), "--rig", str(repo),
                "--gc-bin", str(Path(gc_value).resolve()), "--telemetry-mode", "required",
                "--otel-metrics-url", "http://127.0.0.1:9/metrics",
                "--otel-logs-url", "http://127.0.0.1:9/logs",
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("required OTEL endpoints are unavailable", result.stderr)
            self.assertFalse(city.exists())

    @unittest.skipUnless(PINNED_INTEGRATION, "set GC_BIN and AGENTOPS_GC_INTEGRATION=1 for native integration")
    def test_clean_bootstrap_is_idempotent_with_pinned_native_gc(self) -> None:
        gc_bin = Path(os.environ["GC_BIN"]).resolve()
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            repo, _ = self.make_repository(root)
            city = root / "city"
            source_bead = self.initialize_beads(repo, gc_bin)
            command = (
                str(DEPLOY / "bootstrap.sh"), "--city", str(city), "--rig", str(repo),
                "--gc-bin", str(gc_bin), "--delivery-mode", "manual", "--telemetry-mode", "off",
            )
            first = run(*command)
            self.assertEqual(first.returncode, 0, first.stderr)
            marker_path = Path(first.stdout.strip())
            before = marker_path.read_bytes()
            second = run(*command)
            self.assertEqual(second.returncode, 0, second.stderr)
            self.assertEqual(marker_path.read_bytes(), before)
            marker = json.loads(before)
            self.assertEqual(marker["toolchain"]["gc"]["commit"], GC_COMMIT)
            self.assertEqual(marker["toolchain"]["bd"]["commit"], BD_COMMIT)
            self.assertEqual(marker["delivery_mode"], "manual")
            self.assertEqual(marker["telemetry"]["mode"], "off")
            self.assertEqual(marker["telemetry"]["sdk_disabled"], True)

            native_env = os.environ.copy()
            native_env.update({
                "GC_HOME": str(city / ".gc-home"),
                "PATH": f"{gc_bin.parent}:{native_env['PATH']}",
                "OTEL_SDK_DISABLED": "true",
            })
            config = run(str(gc_bin), "--city", str(city), "--rig", marker["rig_name"], "config", "show", env=native_env)
            self.assertEqual(config.returncode, 0, config.stderr)
            compiled = run(
                str(gc_bin), "--city", str(city), "--rig", marker["rig_name"],
                "formula", "show", "agentops-experiment", "--json", env=native_env,
            )
            self.assertEqual(compiled.returncode, 0, compiled.stderr)
            self.assertEqual(json.loads(compiled.stdout)["name"], "agentops-experiment")
            # Intake resolves to the rig-scoped planner (official single-bead
            # dispatch), not the city-scoped Mayor.
            planner_target = f"{marker['rig_name']}/agentops.plan-reviewer"
            route = run(
                str(gc_bin), "--city", str(city), "sling", planner_target, source_bead,
                "--dry-run", "--json", env=native_env,
            )
            self.assertEqual(route.returncode, 0, route.stderr)
            self.assertIn("agentops.plan-reviewer", json.loads(route.stdout)["target"])
            status = run(str(DEPLOY / "invoke.sh"), "--city", str(city), "status", env=native_env)
            self.assertEqual(status.returncode, 0, status.stderr)
            self.assertEqual(json.loads(status.stdout)["schema_version"], "1")
            doctor = run(str(gc_bin), "--city", str(city), "doctor", "--json", env=native_env)
            self.assertEqual(doctor.returncode, 0, doctor.stdout + doctor.stderr)
            self.assertEqual(json.loads(doctor.stdout)["blocking_failed"], 0)
            teardown = run(str(DEPLOY / "teardown.sh"), "--city", str(city), "--wait-timeout", "10", env=native_env)
            self.assertEqual(teardown.returncode, 0, teardown.stdout + teardown.stderr)
            self.assertEqual(git("status", "--porcelain", cwd=repo).stdout, "")


if __name__ == "__main__":
    unittest.main()
