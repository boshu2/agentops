from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
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
        self.assertIn('sling agentops.mayor "$1" --nudge --json', invoke)
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
        for path, (provider, model, effort) in expected.items():
            config = tomllib.loads(path.read_text(encoding="utf-8"))
            self.assertEqual(config["provider"], provider, path)
            self.assertEqual(config["option_defaults"], {"permission_mode": config["option_defaults"]["permission_mode"], "model": model, "effort": effort}, path)
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
            route = run(
                str(gc_bin), "--city", str(city), "sling", "agentops.mayor", source_bead,
                "--dry-run", "--force", "--json", env=native_env,
            )
            self.assertEqual(route.returncode, 0, route.stderr)
            self.assertEqual(json.loads(route.stdout)["target"], "agentops.mayor")
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
