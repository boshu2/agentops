from __future__ import annotations

import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
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
                    "ownership": "proven_agentops_marker",
                    "git": None,
                },
                {
                    "path": str(self.dirty),
                    "realpath": str(self.dirty.resolve()),
                    "ownership": "proven_agentops_marker",
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

        self.name_only = self.root / "agentops-gc-name-only"
        self.name_only.mkdir()
        self.inventory["experiment_paths"].append(
            {
                "path": str(self.name_only),
                "realpath": str(self.name_only.resolve()),
                "ownership": "name_only_agentops_gc",
                "git": None,
            }
        )

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

    def test_refuses_name_only_ownership(self) -> None:
        with self.assertRaisesRegex(MODULE.ReliabilityError, "ambiguous ownership"):
            MODULE.validate_cleanup_plan(self.inventory, self.plan(self.name_only))

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
    def temporary_worktrees(self) -> tuple[tempfile.TemporaryDirectory[str], Path, Path, Path]:
        temporary = tempfile.TemporaryDirectory()
        root = Path(temporary.name) / "repo"
        root.mkdir()
        MODULE.run(["git", "init", "-b", "main", str(root)])
        MODULE.run(["git", "-C", str(root), "config", "user.email", "test@example.invalid"])
        MODULE.run(["git", "-C", str(root), "config", "user.name", "test"])
        (root / "README").write_text("base\n", encoding="utf-8")
        MODULE.run(["git", "-C", str(root), "add", "README"])
        MODULE.run(["git", "-C", str(root), "commit", "-m", "base"])
        candidate = root.parent / "candidate"
        peer = root.parent / "peer"
        MODULE.run(["git", "-C", str(root), "worktree", "add", "-b", "candidate", str(candidate)])
        MODULE.run(["git", "-C", str(root), "worktree", "add", "-b", "peer", str(peer)])
        return temporary, root, candidate, peer

    def candidate_record(self, root: Path, candidate: Path) -> dict[str, object]:
        snapshot = MODULE.git_snapshot(root)
        candidate_path = str(candidate.resolve())
        index = next(
            item["git_index"]
            for item in snapshot["worktrees"]
            if str(Path(item["path"]).resolve()) == candidate_path
        )
        return {
            "worktree": candidate_path,
            "branch": "candidate",
            "index": index,
            "declared_paths": ["allowed.txt"],
            "generated_paths": ["generated/"],
        }

    def clean_process_receipt(self) -> dict[str, object]:
        return MODULE.run_bounded_isolation(
            [sys.executable, "-c", "import os; assert len(os.environ['AGENTOPS_GC_ISOLATION_TOKEN']) == 64"],
            timeout_seconds=1,
        )

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

    def test_candidate_guard_rejects_transient_and_cross_scope_effects(self) -> None:
        before = self.snapshot()
        after = json.loads(json.dumps(before))
        after["worktrees"][1]["head"] = "def"
        after["worktrees"][1]["git"]["status"] = [" M outside.txt"]
        after["refs"] = ["refs/heads/main|abc", "refs/heads/worker|def"]
        for record in (before, after):
            record["worktrees"][1]["git_index"] = "/repo/.git/worktrees/worker/index"
        receipt = MODULE.candidate_git_audit(before, after, {"worktree": "/repo-worker", "branch": "worker", "index": "/repo/.git/worktrees/worker/index", "declared_paths": ["allowed"]}, process_receipt=None)
        self.assertEqual(receipt["git_result"], "FAIL")
        self.assertIn("undeclared candidate write: outside.txt", receipt["findings"])
        self.assertIn("isolation receipt is not proven", receipt["findings"])

    def test_candidate_reflog_rejects_switch_to_peer_and_back(self) -> None:
        temporary, root, candidate_path, _peer_path = self.temporary_worktrees()
        try:
            candidate = self.candidate_record(root, candidate_path)
            MODULE.run(["git", "-C", str(root), "branch", "switch-peer"])
            before = MODULE.git_snapshot(root)
            MODULE.run(["git", "-C", str(candidate_path), "checkout", "switch-peer"])
            MODULE.run(["git", "-C", str(candidate_path), "checkout", "candidate"])
            after = MODULE.git_snapshot(root)
            receipt = MODULE.candidate_git_audit(
                before, after, candidate, process_receipt=self.clean_process_receipt()
            )
            self.assertEqual(receipt["git_result"], "FAIL", receipt)
            self.assertIn("candidate reflog action is not an allowed forward commit", receipt["findings"])
        finally:
            temporary.cleanup()

    def test_real_candidate_git_audit_allows_only_declared_forward_commit(self) -> None:
        temporary, root, candidate_path, _peer_path = self.temporary_worktrees()
        try:
            candidate = self.candidate_record(root, candidate_path)
            before = MODULE.git_snapshot(root)
            (candidate_path / "allowed.txt").write_text("allowed\n", encoding="utf-8")
            MODULE.run(["git", "-C", str(candidate_path), "add", "allowed.txt"])
            MODULE.run(["git", "-C", str(candidate_path), "commit", "-m", "allowed forward"])
            after = MODULE.git_snapshot(root)
            receipt = MODULE.candidate_git_audit(
                before, after, candidate, process_receipt=self.clean_process_receipt()
            )
            self.assertEqual(receipt["result"], "PASS", receipt)
        finally:
            temporary.cleanup()

    def test_real_candidate_git_audit_allows_generated_directory_root_with_trailing_slash(self) -> None:
        temporary, root, candidate_path, _peer_path = self.temporary_worktrees()
        try:
            candidate = self.candidate_record(root, candidate_path)
            before = MODULE.git_snapshot(root)
            generated = candidate_path / "generated"
            generated.mkdir()
            (generated / "receipt.json").write_text("{}\n", encoding="utf-8")
            MODULE.run(["git", "-C", str(candidate_path), "add", "generated/receipt.json"])
            after = MODULE.git_snapshot(root)
            receipt = MODULE.candidate_git_audit(
                before, after, candidate, process_receipt=self.clean_process_receipt()
            )
            self.assertEqual(receipt["result"], "PASS", receipt)
        finally:
            temporary.cleanup()

    def test_real_candidate_git_audit_rejects_candidate_escapes(self) -> None:
        def audit_after(operation: object) -> dict[str, object]:
            temporary, root, candidate_path, _peer_path = self.temporary_worktrees()
            try:
                candidate = self.candidate_record(root, candidate_path)
                before = MODULE.git_snapshot(root)
                operation(root, candidate_path, candidate)
                after = MODULE.git_snapshot(root)
                return MODULE.candidate_git_audit(
                    before, after, candidate, process_receipt=self.clean_process_receipt()
                )
            finally:
                temporary.cleanup()

        with self.subTest("candidate untracked undeclared write"):
            receipt = audit_after(lambda _root, candidate, _record: (candidate / "outside.txt").write_text("x\n", encoding="utf-8"))
            self.assertEqual(receipt["git_result"], "FAIL", receipt)
        with self.subTest("candidate tracked undeclared write"):
            def tracked(_root: Path, candidate: Path, _record: dict[str, object]) -> None:
                (candidate / "outside.txt").write_text("x\n", encoding="utf-8")
                MODULE.run(["git", "-C", str(candidate), "add", "outside.txt"])
            receipt = audit_after(tracked)
            self.assertEqual(receipt["git_result"], "FAIL", receipt)
        with self.subTest("candidate reset rewrite"):
            def reset(_root: Path, candidate: Path, _record: dict[str, object]) -> None:
                (candidate / "allowed.txt").write_text("allowed\n", encoding="utf-8")
                MODULE.run(["git", "-C", str(candidate), "add", "allowed.txt"])
                MODULE.run(["git", "-C", str(candidate), "commit", "-m", "allowed"])
                MODULE.run(["git", "-C", str(candidate), "reset", "--hard", "HEAD~1"])
            receipt = audit_after(reset)
            self.assertEqual(receipt["git_result"], "FAIL", receipt)
            self.assertIn("candidate reflog action is not an allowed forward commit", receipt["findings"])
        with self.subTest("candidate branch substitution"):
            def switch(root: Path, candidate: Path, _record: dict[str, object]) -> None:
                MODULE.run(["git", "-C", str(root), "branch", "switch-peer"])
                MODULE.run(["git", "-C", str(candidate), "checkout", "switch-peer"])
            receipt = audit_after(switch)
            self.assertEqual(receipt["git_result"], "FAIL", receipt)
            self.assertIn("candidate identity changed", receipt["findings"])
        with self.subTest("candidate git-index substitution"):
            temporary, root, candidate_path, _peer_path = self.temporary_worktrees()
            try:
                candidate = self.candidate_record(root, candidate_path)
                before = MODULE.git_snapshot(root)
                substitute = candidate_path / "substitute.index"
                shutil.copyfile(str(candidate["index"]), substitute)
                after = MODULE.git_snapshot(root)
                substituted = next(
                    item for item in after["worktrees"]
                    if str(Path(item["path"]).resolve()) == candidate["worktree"]
                )
                substituted["git_index"] = str(substitute)
                receipt = MODULE.candidate_git_audit(
                    before, after, candidate, process_receipt=self.clean_process_receipt()
                )
            finally:
                temporary.cleanup()
            self.assertEqual(receipt["git_result"], "FAIL", receipt)
            self.assertIn("candidate identity changed", receipt["findings"])

    def test_real_candidate_git_audit_rejects_shared_repository_escapes(self) -> None:
        def audit_after(operation: object) -> dict[str, object]:
            temporary, root, candidate_path, peer_path = self.temporary_worktrees()
            try:
                candidate = self.candidate_record(root, candidate_path)
                before = MODULE.git_snapshot(root)
                operation(root, candidate_path, peer_path)
                after = MODULE.git_snapshot(root)
                return MODULE.candidate_git_audit(
                    before, after, candidate, process_receipt=self.clean_process_receipt()
                )
            finally:
                temporary.cleanup()

        with self.subTest("stash"):
            def stash(_root: Path, candidate: Path, _peer: Path) -> None:
                (candidate / "stash.txt").write_text("stash\n", encoding="utf-8")
                MODULE.run(["git", "-C", str(candidate), "stash", "push", "--include-untracked", "-m", "bad"])
            receipt = audit_after(stash)
            self.assertEqual(receipt["git_result"], "FAIL", receipt)
            self.assertIn("stash state changed", receipt["findings"])
        with self.subTest("noncandidate ref and reflog"):
            receipt = audit_after(lambda root, _candidate, _peer: MODULE.run(["git", "-C", str(root), "branch", "other"]))
            self.assertEqual(receipt["git_result"], "FAIL", receipt)
            self.assertIn("non-candidate ref changed", receipt["findings"])
            self.assertIn("non-candidate reflog changed", receipt["findings"])
        with self.subTest("worktree addition"):
            receipt = audit_after(lambda root, _candidate, _peer: MODULE.run(["git", "-C", str(root), "worktree", "add", "-b", "extra", str(root.parent / "extra")]))
            self.assertEqual(receipt["git_result"], "FAIL", receipt)
            self.assertIn("worktree set changed", receipt["findings"])
        with self.subTest("worktree removal"):
            receipt = audit_after(lambda root, _candidate, peer: MODULE.run(["git", "-C", str(root), "worktree", "remove", "--force", str(peer)]))
            self.assertEqual(receipt["git_result"], "FAIL", receipt)
            self.assertIn("worktree set changed", receipt["findings"])
        for name, target, tracked in (
            ("primary tracked", "primary", True),
            ("primary untracked", "primary", False),
            ("peer tracked", "peer", True),
            ("peer untracked", "peer", False),
        ):
            with self.subTest(name):
                def mutate(root: Path, _candidate: Path, peer: Path, *, target: str = target, tracked: bool = tracked) -> None:
                    worktree = root if target == "primary" else peer
                    path = worktree / ("README" if tracked else "outside.txt")
                    path.write_text("changed\n", encoding="utf-8")
                receipt = audit_after(mutate)
                self.assertEqual(receipt["git_result"], "FAIL", receipt)
                self.assertTrue(any(item.startswith("non-candidate worktree changed") for item in receipt["findings"]))

    def test_candidate_guard_proves_local_git_scope_but_keeps_process_receipt_not_proven(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "repo"; root.mkdir()
            MODULE.run(["git", "init", "-b", "main", str(root)])
            MODULE.run(["git", "-C", str(root), "config", "user.email", "test@example.invalid"])
            MODULE.run(["git", "-C", str(root), "config", "user.name", "test"])
            (root / "README").write_text("base\n", encoding="utf-8")
            MODULE.run(["git", "-C", str(root), "add", "README"]); MODULE.run(["git", "-C", str(root), "commit", "-m", "base"])
            candidate_path = root.parent / "candidate"
            MODULE.run(["git", "-C", str(root), "worktree", "add", "-b", "candidate", str(candidate_path)])
            before = MODULE.git_snapshot(root)
            candidate_realpath = str(candidate_path.resolve())
            index = next(item["git_index"] for item in before["worktrees"] if str(Path(item["path"]).resolve()) == candidate_realpath)
            candidate = {"worktree": candidate_realpath, "branch": "candidate", "index": index, "declared_paths": ["allowed.txt"]}
            (candidate_path / "allowed.txt").write_text("allowed\n", encoding="utf-8")
            dirty = MODULE.git_snapshot(root)
            receipt = MODULE.candidate_git_audit(before, dirty, candidate, process_receipt=None)
            self.assertEqual(receipt["git_result"], "PASS", receipt)
            self.assertEqual(receipt["result"], "NOT_PROVEN")
            MODULE.run(["git", "-C", str(candidate_path), "add", "allowed.txt"]); MODULE.run(["git", "-C", str(candidate_path), "commit", "-m", "allowed"])
            committed = MODULE.git_snapshot(root)
            receipt = MODULE.candidate_git_audit(dirty, committed, candidate, process_receipt=None)
            self.assertEqual(receipt["git_result"], "PASS", receipt)
            process_receipt = MODULE.run_bounded_isolation([sys.executable, "-c", "import os; assert os.environ['AGENTOPS_GC_ISOLATION_TOKEN']"], timeout_seconds=1)
            self.assertEqual(MODULE.candidate_git_audit(committed, committed, candidate, process_receipt=process_receipt)["result"], "PASS")
            for outcome in ("timeout_cleanup_required", "leak_cleanup_required"):
                non_clean = dict(process_receipt); non_clean.update({"outcome": outcome, "completed": False, "timeout": outcome.startswith("timeout"), "leak_detected": outcome.startswith("leak"), "cleanup_required": True})
                self.assertEqual(MODULE.candidate_git_audit(committed, committed, candidate, process_receipt=non_clean)["result"], "FAIL")
            (candidate_path / "undeclared.txt").write_text("no\n", encoding="utf-8")
            MODULE.run(["git", "-C", str(candidate_path), "add", "undeclared.txt"]); MODULE.run(["git", "-C", str(candidate_path), "commit", "-m", "undeclared"])
            undeclared = MODULE.git_snapshot(root)
            self.assertEqual(MODULE.candidate_git_audit(committed, undeclared, candidate, process_receipt=process_receipt)["git_result"], "FAIL")
            for mutate in (
                lambda state: state.__setitem__("stash", ["stash@{0}|dead|bad"]),
                lambda state: state["refs"].append("refs/heads/main|dead"),
                lambda state: state["worktrees"][0]["git"].__setitem__("status", [" M main.txt"]),
            ):
                broken = json.loads(json.dumps(committed)); mutate(broken)
                self.assertEqual(MODULE.candidate_git_audit(committed, broken, candidate, process_receipt=None)["git_result"], "FAIL")

    def test_bounded_isolation_receipts_are_strict_and_cleanup_only_the_child_group(self) -> None:
        clean = MODULE.run_bounded_isolation([sys.executable, "-c", "import os; assert len(os.environ['AGENTOPS_GC_ISOLATION_TOKEN']) == 64"], timeout_seconds=1)
        self.assertEqual(clean["outcome"], "clean")
        self.assertTrue(clean["completed"])
        self.assertFalse(clean["leak_detected"])
        self.assertEqual(clean["surviving_pids"], [])
        another_clean = MODULE.run_bounded_isolation([sys.executable, "-c", "pass"], timeout_seconds=1)
        self.assertNotEqual(clean["isolation_token"], another_clean["isolation_token"])
        timed_out = MODULE.run_bounded_isolation([sys.executable, "-c", "import time; time.sleep(5)"], timeout_seconds=0.05)
        self.assertEqual(timed_out["outcome"], "timeout_cleanup_required")
        self.assertTrue(timed_out["timeout"])
        self.assertTrue(timed_out["cleanup_required"])
        self.assertTrue(timed_out["cleanup_complete"])
        term_resistant = MODULE.run_bounded_isolation(
            [sys.executable, "-c", "import signal, time; signal.signal(signal.SIGTERM, lambda *_: None); time.sleep(5)"],
            timeout_seconds=0.05,
        )
        self.assertEqual(term_resistant["outcome"], "timeout_cleanup_required")
        self.assertTrue(term_resistant["cleanup_complete"])
        sentinel = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(5)"], start_new_session=True)
        try:
            leaked = MODULE.run_bounded_isolation([sys.executable, "-c", "import subprocess, sys; subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(5)'])"], timeout_seconds=1)
            self.assertEqual(leaked["outcome"], "leak_cleanup_required")
            self.assertTrue(leaked["leak_detected"])
            self.assertTrue(leaked["cleanup_required"])
            self.assertTrue(leaked["cleanup_complete"])
            self.assertEqual(leaked["surviving_pids"], [])
            self.assertIsNone(sentinel.poll())
            self.assertNotEqual(os.getpgid(sentinel.pid), leaked["runner_pgid"])
            grandchild = MODULE.run_bounded_isolation(
                [sys.executable, "-c", "import subprocess, sys; subprocess.Popen([sys.executable, '-c', \"import subprocess, sys, time; subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(5)']); time.sleep(5)\"])",],
                timeout_seconds=1,
            )
            self.assertEqual(grandchild["outcome"], "leak_cleanup_required")
            self.assertTrue(grandchild["leak_detected"])
            self.assertTrue(grandchild["cleanup_complete"])
        finally:
            sentinel.terminate()
            sentinel.wait(timeout=2)

    def test_invalid_process_receipt_is_not_proven(self) -> None:
        before = self.snapshot()
        for record in before["worktrees"]:
            if record["path"] == "/repo-worker":
                record["git_index"] = "/repo/.git/worktrees/worker/index"
        invalid = self.clean_process_receipt()
        invalid["isolation_token"] = "not-a-fresh-token"
        receipt = MODULE.candidate_git_audit(
            before,
            json.loads(json.dumps(before)),
            {
                "worktree": "/repo-worker",
                "branch": "worker",
                "index": "/repo/.git/worktrees/worker/index",
                "declared_paths": ["allowed"],
            },
            process_receipt=invalid,
        )
        self.assertEqual(receipt["result"], "NOT_PROVEN", receipt)
        self.assertIn("isolation receipt is invalid", receipt["findings"])

    def test_unborn_repository_is_inventoryable(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            MODULE.run(["git", "init", str(root)])
            status = MODULE.git_status(root)
            self.assertIsNotNone(status)
            self.assertIsNone(status["head"])
            self.assertTrue(status["branch"])


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

    def test_explicit_missing_path_is_recorded_without_claiming_runtime_state(
        self,
    ) -> None:
        missing = Path("/tmp/gc-agentops-v17-controller-city-20260719-missing")
        record = MODULE.path_record(missing)
        self.assertFalse(record["exists"])
        self.assertEqual(record["ownership"], "name_only_agentops_gc")
        self.assertIsNone(record["agentops_marker"])
        self.assertIsNone(record["dolt_state"])

    def test_reads_dolt_state_without_starting_a_server(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            state = root / ".gc" / "runtime" / "packs" / "dolt" / "dolt-state.json"
            state.parent.mkdir(parents=True)
            state.write_text(
                json.dumps(
                    {"pid": 123, "port": 3307, "database": "hq", "running": True}
                ),
                encoding="utf-8",
            )
            record = MODULE.path_record(root)
            self.assertEqual(record["dolt_state"]["pid"], 123)
            self.assertEqual(record["dolt_state"]["database"], "hq")

    def test_process_association_does_not_upgrade_name_only_ownership(self) -> None:
        records = {
            "/fixture/name-only": {"ownership": "name_only_agentops_gc"},
            "/fixture/marked": {"ownership": "proven_agentops_marker"},
        }
        self.assertEqual(
            MODULE.process_path_ownership(records, "/fixture/name-only"),
            "ambiguous_named_path",
        )
        self.assertEqual(
            MODULE.process_path_ownership(records, "/fixture/marked"),
            "proven_path",
        )


class TmuxInventoryTest(unittest.TestCase):
    def test_parses_machine_readable_sessions(self) -> None:
        raw = "mayor\t1\t123\t0\nrefinery\t2\t456\t1\n"
        self.assertEqual(
            MODULE.parse_tmux_sessions(raw),
            [
                {"name": "mayor", "windows": 1, "pid": 123, "attached": False},
                {"name": "refinery", "windows": 2, "pid": 456, "attached": True},
            ],
        )

    def test_rejects_malformed_session_output(self) -> None:
        with self.assertRaisesRegex(
            MODULE.ReliabilityError, "invalid tmux session record"
        ):
            MODULE.parse_tmux_sessions("not-enough-fields\n")


class ToolchainSelectionTest(unittest.TestCase):
    def record(self, command: str, realpath: str, sha: str) -> dict[str, object]:
        return {
            "command": command,
            "realpath": realpath,
            "sha256": sha,
            "selected": True,
        }

    def test_exact_pair_is_unambiguous(self) -> None:
        result = MODULE.selected_toolchain(
            [
                self.record("gc", "/toolchain/bin/gc", "a" * 64),
                self.record("bd", "/toolchain/bin/bd", "b" * 64),
            ]
        )
        self.assertEqual(result["status"], "exact")
        self.assertFalse(result["duplicate_active_identity"])

    def test_duplicate_selected_identity_is_reported(self) -> None:
        result = MODULE.selected_toolchain(
            [
                self.record("gc", "/one/bin/gc", "a" * 64),
                self.record("gc", "/two/bin/gc", "c" * 64),
                self.record("bd", "/one/bin/bd", "b" * 64),
            ]
        )
        self.assertEqual(result["status"], "ambiguous")
        self.assertTrue(result["duplicate_active_identity"])

    def test_binary_inventory_never_executes_observed_binary(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            binary = root / "gc"
            invoked = root / "invoked"
            binary.write_text(f"#!/bin/sh\ntouch '{invoked}'\n", encoding="utf-8")
            binary.chmod(0o755)
            records = MODULE.binary_records(root, root / "gascity", [binary])
            selected = [record for record in records if record.get("selected")]
            self.assertEqual(len(selected), 1)
            self.assertFalse(invoked.exists())
            self.assertEqual(selected[0]["version_source"], "not_executed")


class RegistryContractTest(unittest.TestCase):
    def test_release_relevance_is_required(self) -> None:
        entry = {
            key: "fixture" for key in MODULE.REGISTRY_REQUIRED - {"release_relevance"}
        }
        entry["identities"] = {}
        entry["owner"] = "agentops"
        payload = {"schema_version": 1, "entries": [entry]}
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "known-errors.json"
            path.write_text(json.dumps(payload), encoding="utf-8")
            with self.assertRaisesRegex(MODULE.ReliabilityError, "release_relevance"):
                MODULE.validate_registry(path)


class ForkManifestTest(unittest.TestCase):
    def manifest(self) -> dict[str, object]:
        return {
            "schema_version": 1,
            "observed_at": "2026-07-20T00:00:00Z",
            "upstream": "gastownhall/gascity",
            "fork": "boshu2/gascity",
            "upstream_main": "a" * 40,
            "fork_main": "a" * 40,
            "local_pre_push": {
                "result": "failed_on_observed_official_main",
                "subject_sha": "a" * 40,
                "log_sha256": "d" * 64,
                "bypass_used": True,
                "reason": "Fork synchronization carried no fork-only content.",
            },
            "retained_branches": [
                {
                    "branch": "pr/safety",
                    "head": "b" * 40,
                    "upstream_kind": "pr",
                    "upstream_number": 42,
                    "upstream_url": "https://github.com/gastownhall/gascity/pull/42",
                    "state": "open",
                }
            ],
            "removed_merged_branches": [
                {
                    "branch": "fix/merged",
                    "head": "c" * 40,
                    "upstream_number": 41,
                    "upstream_url": "https://github.com/gastownhall/gascity/pull/41",
                    "state": "merged",
                }
            ],
        }

    def test_accepts_equal_main_and_upstream_linked_retained_branches(self) -> None:
        result = MODULE.validate_fork_manifest(self.manifest())
        self.assertEqual(result["retained_branches"], 1)
        self.assertEqual(len(result["manifest_digest"]), 64)

    def test_rejects_fork_main_drift(self) -> None:
        manifest = self.manifest()
        manifest["fork_main"] = "d" * 40
        with self.assertRaisesRegex(MODULE.ReliabilityError, "fork main"):
            MODULE.validate_fork_manifest(manifest)

    def test_accepts_explicit_reuse_of_prior_official_main_failure(self) -> None:
        manifest = self.manifest()
        manifest["upstream_main"] = "e" * 40
        manifest["fork_main"] = "e" * 40
        manifest["local_pre_push"]["result"] = "prior_official_main_failure_reused"
        result = MODULE.validate_fork_manifest(manifest)
        self.assertEqual(result["retained_branches"], 1)

    def test_rejects_current_failure_receipt_for_a_different_subject(self) -> None:
        manifest = self.manifest()
        manifest["local_pre_push"]["subject_sha"] = "e" * 40
        with self.assertRaisesRegex(MODULE.ReliabilityError, "does not match fork main"):
            MODULE.validate_fork_manifest(manifest)

    def test_rejects_retained_branch_without_open_upstream_work(self) -> None:
        manifest = self.manifest()
        manifest["retained_branches"][0]["state"] = "merged"
        with self.assertRaisesRegex(MODULE.ReliabilityError, "open upstream"):
            MODULE.validate_fork_manifest(manifest)


class InventoryContractTest(unittest.TestCase):
    def inventory(self) -> dict[str, object]:
        payload = {
            "schema_version": 1,
            "generated_at": "2026-07-20T00:00:00Z",
            "experiment_paths": [
                {
                    "realpath": "/fixture/city",
                    "ownership": "name_only_agentops_gc",
                }
            ],
            "processes": [{"pid": 42, "category": "gc_supervisor"}],
            "dolt": {"declared_states": [], "live_processes": []},
            "tmux": {"socket_root": "/fixture/tmux", "servers": []},
            "binaries": [],
            "selected_toolchain": {
                "status": "not_selected",
                "duplicate_active_identity": False,
            },
            "git_repositories": [],
        }
        stable = dict(payload)
        stable.pop("generated_at")
        payload["inventory_digest"] = MODULE.digest(stable)
        return payload

    def test_validates_digest_and_identity_uniqueness(self) -> None:
        result = MODULE.validate_inventory_payload(self.inventory())
        self.assertEqual(result["experiment_paths"], 1)
        self.assertEqual(result["processes"], 1)

    def test_rejects_duplicate_path_identity(self) -> None:
        payload = self.inventory()
        payload["experiment_paths"].append(dict(payload["experiment_paths"][0]))
        stable = dict(payload)
        stable.pop("generated_at")
        stable.pop("inventory_digest")
        payload["inventory_digest"] = MODULE.digest(stable)
        with self.assertRaisesRegex(MODULE.ReliabilityError, "duplicate experiment path"):
            MODULE.validate_inventory_payload(payload)

    def test_rejects_digest_drift(self) -> None:
        payload = self.inventory()
        payload["inventory_digest"] = "0" * 64
        with self.assertRaisesRegex(MODULE.ReliabilityError, "digest"):
            MODULE.validate_inventory_payload(payload)


if __name__ == "__main__":
    unittest.main()
