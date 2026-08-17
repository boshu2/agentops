from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


SKILL = Path(__file__).resolve().parents[1]
DISPATCH = SKILL / "hooks" / "policy-dispatch.sh"
EDIT_GUARD = SKILL / "hooks" / "installed-skill-edit-guard.sh"
COORD_GUARD = SKILL / "hooks" / "skill-first-coord-guard.sh"
INSTALL = SKILL / "scripts" / "install-hooks.sh"


class HookBehaviorTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name).resolve()
        self.tmp = self.root / "tmp"
        self.tmp.mkdir()
        self.env = os.environ.copy()
        self.env["TMPDIR"] = str(self.tmp)
        self.env["HOME"] = str(self.root)

    def tearDown(self) -> None:
        self.temp.cleanup()

    def run_hook(self, script: Path, payload: dict | str, env: dict[str, str] | None = None):
        raw = payload if isinstance(payload, str) else json.dumps(payload)
        return subprocess.run(
            ["bash", str(script)], input=raw, env=env or self.env,
            capture_output=True, text=True,
        )

    def test_dispatch_behavior_matrix_clean_deny_oversize_and_override(self) -> None:
        clean = {"tool_name": "Bash", "session_id": "s", "tool_input": {"command": "printf ok"}}
        result = self.run_hook(DISPATCH, clean)
        self.assertEqual((result.returncode, result.stdout, result.stderr), (0, "", ""))

        denied = {"tool_name": "Bash", "session_id": "s", "tool_input": {"command": "git add _beads/file"}}
        result = self.run_hook(DISPATCH, denied)
        self.assertEqual(result.returncode, 2)
        self.assertEqual(result.stdout, "")
        self.assertIn("core.git:add-beads-ledger", result.stderr)

        oversized = self.run_hook(DISPATCH, "x" * 65537)
        self.assertEqual(oversized.returncode, 2)
        self.assertIn("exceeds", oversized.stderr)

        external = self.root / "external.json"
        external.write_text(json.dumps({
            "schema": "hooks-manifest.v2",
            "policies": [{
                "id": "evil.any:command", "predicate_class": "pure", "mode": "deny",
                "route_message": "deny everything", "rationale": "x", "value_proof": "x",
                "matchers": [{"tools": ["Bash"], "field": "command", "pattern": ".*"}],
            }],
        }), encoding="utf-8")
        override_env = self.env.copy()
        override_env["AOP_POLICIES"] = str(external)
        ignored = self.run_hook(DISPATCH, clean, override_env)
        self.assertEqual((ignored.returncode, ignored.stdout, ignored.stderr), (0, "", ""))

    def test_permission_route_is_explicit_and_consumed_once(self) -> None:
        installed = self.root / "installed"
        installed.mkdir()
        dispatcher = installed / "policy-dispatch.sh"
        shutil.copy2(DISPATCH, dispatcher)
        (installed / "policies.json").write_text(json.dumps({
            "schema": "hooks-manifest.v2",
            "policies": [{
                "id": "test.shell:route", "predicate_class": "pure", "mode": "route",
                "route_message": "ask once", "rationale": "test", "value_proof": "test",
                "matchers": [{"tools": ["Bash"], "field": "command", "pattern": "^danger$"}],
            }],
        }), encoding="utf-8")
        payload = {"tool_name": "Bash", "session_id": "route-session", "tool_input": {"command": "danger"}}
        disabled = self.run_hook(dispatcher, payload)
        self.assertEqual(disabled.returncode, 2)
        enabled_env = self.env.copy()
        enabled_env["AOP_ENABLE_PERMISSION_ROUTING"] = "1"
        first = self.run_hook(dispatcher, payload, enabled_env)
        self.assertEqual(first.returncode, 0, first.stderr)
        self.assertEqual(json.loads(first.stdout)["hookSpecificOutput"]["permissionDecision"], "ask")
        second = self.run_hook(dispatcher, payload, enabled_env)
        self.assertEqual(second.returncode, 2)
        self.assertEqual(second.stdout, "")

    def test_telemetry_is_opt_in_fixed_bounded_and_contains_no_raw_value(self) -> None:
        payload = {"tool_name": "Bash", "session_id": "s2", "tool_input": {"command": "git add _beads/secret"}}
        arbitrary = self.root / "arbitrary.jsonl"
        old_env = self.env.copy()
        old_env["AGENTOPS_GUARDRAIL_TELEMETRY"] = str(arbitrary)
        self.run_hook(DISPATCH, payload, old_env)
        self.assertFalse(arbitrary.exists())
        telemetry = self.root / "telemetry"
        telemetry.mkdir()
        opt_in = self.env.copy()
        opt_in["AOP_TELEMETRY_ROOT"] = str(telemetry)
        self.run_hook(DISPATCH, payload, opt_in)
        retained = (telemetry / "guardrail-telemetry.jsonl").read_text(encoding="utf-8")
        self.assertNotIn("_beads/secret", retained)
        self.assertEqual(json.loads(retained)["path_sha256"], hashlib.sha256(b"git add _beads/secret").hexdigest())

    def test_standalone_guards_bound_input_suppress_raw_path_and_do_not_trigger_on_prose(self) -> None:
        path = "/Users/alice/.codex/skills/private-skill/SKILL.md"
        payload = {"session_id": "one", "tool_input": {"file_path": path}}
        fired = self.run_hook(EDIT_GUARD, payload)
        self.assertEqual(fired.returncode, 2)
        self.assertNotIn(path, fired.stderr)
        clean = self.run_hook(EDIT_GUARD, {"session_id": "two", "tool_input": {"file_path": "skills/source/SKILL.md", "content": path}})
        self.assertEqual((clean.returncode, clean.stdout, clean.stderr), (0, "", ""))
        oversized = self.run_hook(COORD_GUARD, "x" * 65537)
        self.assertEqual(oversized.returncode, 2)
        prose = self.run_hook(COORD_GUARD, {"session_id": "three", "tool_input": {"command": "printf '%s' 'ntm send'"}})
        self.assertEqual((prose.returncode, prose.stdout, prose.stderr), (0, "", ""))


class InstallerTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name).resolve()
        self.env = os.environ.copy()
        self.env["HOME"] = str(self.root)

    def tearDown(self) -> None:
        self.temp.cleanup()

    def run_install(self, *args: str, env: dict[str, str] | None = None):
        return subprocess.run(["bash", str(INSTALL), *args], env=env or self.env, capture_output=True, text=True)

    def test_user_install_is_fixed_transactional_and_idempotent(self) -> None:
        arbitrary = self.root / "arbitrary-settings.json"
        env = self.env.copy()
        env["SETTINGS"] = str(arbitrary)
        first = self.run_install("--user", env=env)
        self.assertEqual(first.returncode, 0, first.stderr)
        settings = self.root / ".claude" / "settings.json"
        hooks = self.root / ".claude" / "hooks" / "aop"
        self.assertTrue((hooks / "policy-dispatch.sh").is_file())
        self.assertFalse(arbitrary.exists())
        data = json.loads(settings.read_text(encoding="utf-8"))
        commands = [hook["command"] for row in data["hooks"]["PreToolUse"] for hook in row["hooks"]]
        self.assertEqual(len(commands), 2)
        self.assertTrue(all(command == str(hooks / "policy-dispatch.sh") for command in commands))
        second = self.run_install("--user", env=env)
        self.assertEqual(second.returncode, 0, second.stderr)
        self.assertEqual(json.loads(settings.read_text())["hooks"]["PreToolUse"], data["hooks"]["PreToolUse"])

    def test_unapproved_symlink_relative_and_oversized_scopes_fail_unchanged(self) -> None:
        relative = self.run_install("--project-root", "relative")
        self.assertEqual(relative.returncode, 2)
        outside = self.root / "outside"
        outside.mkdir()
        linked = self.root / "linked"
        linked.symlink_to(outside, target_is_directory=True)
        symlinked = self.run_install("--project-root", str(linked))
        self.assertEqual(symlinked.returncode, 2)
        claude = outside / ".claude"
        claude.mkdir()
        settings = claude / "settings.json"
        settings.write_text("x" * (1024 * 1024 + 1), encoding="utf-8")
        before = settings.stat().st_size
        oversized = self.run_install("--project-root", str(outside))
        self.assertEqual(oversized.returncode, 2)
        self.assertEqual(settings.stat().st_size, before)
        self.assertFalse((claude / "hooks").exists())


if __name__ == "__main__":
    unittest.main()
