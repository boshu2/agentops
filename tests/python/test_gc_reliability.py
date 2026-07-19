from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
MODULE_PATH = ROOT / "deploy" / "gc" / "reliability.py"
SPEC = importlib.util.spec_from_file_location("gc_reliability", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class CleanupAdmissionTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.clean = self.root / "gc-agentops-clean"
        self.dirty = self.root / "gc-agentops-dirty"
        self.ambiguous = self.root / "gc-city"
        for path in (self.clean, self.dirty, self.ambiguous):
            path.mkdir()
        self.inventory = {
            "schema_version": 1,
            "gascity_repo": str(self.root / "gascity"),
            "experiment_paths": [
                {
                    "path": str(self.clean),
                    "realpath": str(self.clean.resolve()),
                    "ownership": "agentops_gc_named",
                    "git": None,
                },
                {
                    "path": str(self.dirty),
                    "realpath": str(self.dirty.resolve()),
                    "ownership": "agentops_gc_named",
                    "git": {"dirty": True},
                },
                {
                    "path": str(self.ambiguous),
                    "realpath": str(self.ambiguous.resolve()),
                    "ownership": "ambiguous_gc_named",
                    "git": None,
                },
            ],
            "gascity_worktrees": [],
            "processes": [],
        }

    def tearDown(self) -> None:
        self.temp.cleanup()

    def plan(self, target: Path, **extra: object) -> dict[str, object]:
        action = {
            "operation": "move_path",
            "target": str(target),
            "archive": str(self.root / "archive" / target.name),
        }
        action.update(extra)
        return {"schema_version": 1, "actions": [action]}

    def test_admits_exact_owned_clean_target(self) -> None:
        result = MODULE.validate_cleanup_plan(self.inventory, self.plan(self.clean))
        self.assertEqual(result["admitted_actions"], [0])
        self.assertEqual(len(result["plan_digest"]), 64)

    def test_refuses_ambiguous_target(self) -> None:
        with self.assertRaisesRegex(MODULE.ReliabilityError, "ambiguous ownership"):
            MODULE.validate_cleanup_plan(self.inventory, self.plan(self.ambiguous))

    def test_refuses_dirty_target_without_evidence(self) -> None:
        with self.assertRaisesRegex(MODULE.ReliabilityError, "lacks recoverable evidence"):
            MODULE.validate_cleanup_plan(self.inventory, self.plan(self.dirty))

    def test_admits_dirty_target_with_existing_evidence(self) -> None:
        evidence = self.root / "dirty.patch"
        evidence.write_text("recoverable\n", encoding="utf-8")
        result = MODULE.validate_cleanup_plan(
            self.inventory, self.plan(self.dirty, evidence_ref=str(evidence))
        )
        self.assertEqual(result["admitted_actions"], [0])

    def test_apply_is_dry_run_first_and_idempotent(self) -> None:
        plan = self.plan(self.clean)
        preview = MODULE.apply_cleanup_plan(self.inventory, plan, execute=False, confirmation=None)
        self.assertTrue(self.clean.exists())
        self.assertEqual(preview["results"][0]["result"], "would_apply")
        applied = MODULE.apply_cleanup_plan(
            self.inventory, plan, execute=True, confirmation=preview["plan_digest"]
        )
        self.assertEqual(applied["results"][0]["result"], "applied")
        repeated = MODULE.apply_cleanup_plan(
            self.inventory, plan, execute=True, confirmation=preview["plan_digest"]
        )
        self.assertEqual(repeated["results"][0]["result"], "already_applied")


class GitAuditTest(unittest.TestCase):
    def snapshot(self) -> dict[str, object]:
        return {
            "repo": "/repo",
            "common_dir": "/repo/.git",
            "stash": [],
            "refs": ["refs/heads/main|abc"],
            "reflog": ["abc|HEAD@{0}|start"],
            "worktrees": [
                {
                    "path": "/repo",
                    "head": "abc",
                    "branch": "main",
                    "git": {"status": []},
                },
                {
                    "path": "/repo-worker",
                    "head": "abc",
                    "branch": "worker",
                    "git": {"status": []},
                },
            ],
        }

    def test_clean_snapshot_passes(self) -> None:
        before = self.snapshot()
        self.assertEqual(MODULE.git_audit(before, json.loads(json.dumps(before)))["result"], "PASS")

    def test_detects_stash_creation(self) -> None:
        before = self.snapshot()
        after = json.loads(json.dumps(before))
        after["stash"] = ["stash@{0}|def|WIP"]
        after["refs"].append("refs/stash|def")
        after["reflog"].append("def|refs/stash@{0}|stash: WIP")
        findings = MODULE.git_audit(before, after)["findings"]
        self.assertIn("stash state changed", findings)
        self.assertIn("reflog changed", findings)

    def test_detects_forbidden_head_and_ref_movement(self) -> None:
        before = self.snapshot()
        after = json.loads(json.dumps(before))
        after["refs"] = ["refs/heads/main|def"]
        after["worktrees"][0]["head"] = "def"
        findings = MODULE.git_audit(before, after)["findings"]
        self.assertIn("ref state changed", findings)
        self.assertIn("HEAD or branch changed: /repo", findings)

    def test_detects_cross_worktree_content_change(self) -> None:
        before = self.snapshot()
        after = json.loads(json.dumps(before))
        after["worktrees"][1]["git"]["status"] = [" M outside.txt"]
        findings = MODULE.git_audit(before, after)["findings"]
        self.assertIn("content/status changed: /repo-worker", findings)


class EvidenceFilterTest(unittest.TestCase):
    def test_excludes_runtime_and_credential_roots(self) -> None:
        for path in (".gc/state.json", ".beads/config.yaml", ".codex/auth.json", ".claude/x"):
            self.assertFalse(MODULE.evidence_safe_untracked(path))

    def test_allows_untracked_source_without_parent_escape(self) -> None:
        self.assertTrue(MODULE.evidence_safe_untracked("cmd/gc/new_test.go"))
        self.assertFalse(MODULE.evidence_safe_untracked("../escape"))


class OwnershipClassificationTest(unittest.TestCase):
    def test_finds_nested_agentops_experiment_root(self) -> None:
        root = Path("/tmp/agentops-gc-reliability-cycle1.abc123")
        nested = root / "city" / ".gc-home"
        self.assertEqual(MODULE.agentops_experiment_ancestor(nested).name, root.name)

    def test_does_not_claim_generic_gc_city(self) -> None:
        self.assertIsNone(MODULE.agentops_experiment_ancestor(Path("/Users/bo/dev/gc-city/.gc")))

    def test_tmp_aliases_include_private_tmp(self) -> None:
        aliases = MODULE.path_aliases("/tmp/agentops-gc-reliability-cycle1.abc123")
        self.assertIn("/tmp/agentops-gc-reliability-cycle1.abc123", aliases)
        self.assertIn("/private/tmp/agentops-gc-reliability-cycle1.abc123", aliases)


if __name__ == "__main__":
    unittest.main()
