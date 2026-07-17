from __future__ import annotations

import contextlib
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import types
import unittest
from unittest import mock


sys.dont_write_bytecode = True


ROOT = Path(__file__).resolve().parents[2]
MODULE_PATH = ROOT / "packs" / "agentops-factory" / "assets" / "scripts" / "factory.py"
SPEC = importlib.util.spec_from_file_location("agentops_gc_factory", MODULE_PATH)
assert SPEC and SPEC.loader
factory = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(factory)


def git(repo: Path, *args: str) -> str:
    completed = subprocess.run(
        ["git", *args], cwd=repo, check=True, capture_output=True, text=True,
    )
    return completed.stdout.strip()


class FakeBeads:
    def __init__(self, records: dict[str, dict] | None = None, ready: set[str] | None = None) -> None:
        self.records = records or {}
        self.ready = ready or set()
        self.events: list[tuple] = []
        self.graph_plan: dict | None = None
        self.graph_ids: dict[str, str] | None = None
        self.created = 0

    def graph_create(self, plan: dict) -> dict[str, str]:
        self.events.append(("graph_create",))
        self.graph_plan = plan
        if self.graph_ids is not None:
            ids = self.graph_ids
        else:
            ids = {node["key"]: f"bd-{index + 1}" for index, node in enumerate(plan["nodes"])}
        by_key = {node["key"]: node for node in plan["nodes"]}
        for key, bead_id in ids.items():
            node = by_key[key]
            self.records.setdefault(bead_id, {
                "id": bead_id,
                "status": "open",
                "description": node.get("description", ""),
                "metadata": dict(node.get("metadata", {})),
            })
        return ids

    def show(self, bead_id: str) -> dict:
        return self.records[bead_id]

    def list_program(self, program_bead: str) -> list[dict]:
        return [
            record for record in self.records.values()
            if record.get("metadata", {}).get("factory.program_bead") == program_bead
        ]

    def list_by_metadata(self, key: str, value: str) -> list[dict]:
        return [
            record for record in self.records.values()
            if record.get("metadata", {}).get(key) == value
        ]

    def ready_ids(self) -> set[str]:
        return set(self.ready)

    def create(self, title: str, description: str, metadata: dict[str, str], labels=None) -> str:
        self.created += 1
        bead_id = f"rescope-{self.created}"
        self.records[bead_id] = {
            "id": bead_id,
            "title": title,
            "description": description,
            "status": "open",
            "metadata": dict(metadata),
            "labels": labels or [],
        }
        self.events.append(("create", bead_id))
        return bead_id

    def update_metadata(self, bead_id: str, fields: dict) -> None:
        self.records[bead_id].setdefault("metadata", {}).update(fields)
        self.events.append(("update", bead_id, dict(fields)))

    def close(self, bead_id: str, reason: str) -> None:
        self.records[bead_id]["status"] = "closed"
        self.events.append(("close", bead_id, reason))

    def dep_add(self, blocked: str, blocker: str, dep_type: str = "blocks") -> None:
        blocker_status = self.records.get(blocker, {}).get("status", "open")
        self.records[blocked].setdefault("dependencies", []).append({
            "id": blocker, "dependency_type": dep_type, "status": blocker_status,
        })
        self.events.append(("dep_add", blocked, blocker, dep_type))


class FactoryTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.repo = self.root / "repo"
        self.repo.mkdir()
        git(self.repo, "init", "-b", "main")
        git(self.repo, "config", "user.name", "Factory Tests")
        git(self.repo, "config", "user.email", "factory@example.invalid")
        (self.repo / "README.md").write_text("base\n", encoding="utf-8")
        git(self.repo, "add", "README.md")
        git(self.repo, "commit", "-m", "base")
        self.base_sha = git(self.repo, "rev-parse", "HEAD")
        self.intent = self.root / "intent.md"
        self.intent.write_text("Build the bounded factory behavior.\n", encoding="utf-8")

    def node(self, node_id: str, scope: str, depends_on=None, provider="codex") -> dict:
        validator = "claude" if provider == "codex" else "codex"
        return {
            "id": node_id,
            "title": f"Experiment {node_id}",
            "intent": f"Implement {node_id}.",
            "acceptance": [f"{node_id} is proven."],
            "non_goals": [],
            "depends_on": depends_on or [],
            "write_scope": [scope],
            "generated_scope": [],
            "subject": {"includes": [scope], "excludes": []},
            "first_check": f"test -e {scope}",
            "provider": provider,
            "validator_provider": validator,
            "risk": "routine",
        }

    def graph(self, nodes: list[dict]) -> dict:
        return {
            "schema_version": "program-graph.v1",
            "program_id": "factory-test",
            "intent_digest": hashlib.sha256(self.intent.read_bytes()).hexdigest(),
            "repository": str(self.repo),
            "base_branch": "main",
            "base_sha": self.base_sha,
            "planning_notes": "Parallel experiments with disjoint scopes.",
            "nodes": nodes,
        }

    def write_graph_review(self, graph: dict) -> tuple[Path, Path]:
        graph_path = self.root / "graph.json"
        graph_path.write_text(json.dumps(graph, indent=2) + "\n", encoding="utf-8")
        review = {
            "schema_version": "plan-review.v1",
            "program_id": graph["program_id"],
            "intent_digest": graph["intent_digest"],
            "graph_digest": hashlib.sha256(graph_path.read_bytes()).hexdigest(),
            "mayor_context_id": "mayor-context",
            "reviewer_context_id": "reviewer-context",
            "provider": "claude",
            "verdict": "PASS",
            "criteria": [
                {"id": "bead-graph", "result": "PASS", "reason": "Dependencies and scopes are explicit."},
                {"id": "delivery", "result": "PASS", "reason": "Refinery is blocked by every experiment bead."},
            ],
            "findings": [],
        }
        review_path = self.root / "review.json"
        review_path.write_text(json.dumps(review, indent=2) + "\n", encoding="utf-8")
        return graph_path, review_path

    def test_program_compiles_to_beads_and_dependency_edges(self) -> None:
        graph = self.graph([
            self.node("alpha", "src/alpha"),
            self.node("beta", "src/beta", depends_on=["alpha"], provider="claude"),
        ])
        factory.validate_graph(graph, graph["intent_digest"], self.repo, self.base_sha)
        plan = factory.compile_bead_plan(
            graph, "1" * 64, "2" * 64, self.intent, "mayor-context", "reviewer-context",
        )

        kinds = {node["key"]: node["metadata"]["factory.kind"] for node in plan["nodes"]}
        self.assertEqual(kinds, {
            "program": "program",
            "experiment-alpha": "experiment",
            "experiment-beta": "experiment",
            "refinery": "refinery",
        })
        edges = {(edge.get("from_key"), edge.get("to_key"), edge["type"]) for edge in plan["edges"]}
        self.assertIn(("experiment-beta", "experiment-alpha", "blocks"), edges)
        self.assertIn(("refinery", "experiment-alpha", "blocks"), edges)
        self.assertIn(("refinery", "experiment-beta", "blocks"), edges)
        self.assertNotIn("state", json.dumps(plan).lower())

    def test_unordered_overlapping_beads_fail_but_ordered_overlap_is_allowed(self) -> None:
        unordered = self.graph([
            self.node("alpha", "src/shared"),
            self.node("beta", "src/shared/file.py", provider="claude"),
        ])
        with self.assertRaisesRegex(factory.FactoryError, "unordered beads alpha and beta overlap"):
            factory.validate_graph(unordered)

        ordered = self.graph([
            self.node("alpha", "src/shared"),
            self.node("beta", "src/shared/file.py", depends_on=["alpha"], provider="claude"),
        ])
        self.assertIs(factory.validate_graph(ordered), ordered)

    def test_admission_atomically_materializes_one_bead_graph(self) -> None:
        graph = self.graph([self.node("alpha", "src/alpha")])
        graph_path, review_path = self.write_graph_review(graph)
        beads = FakeBeads()
        beads.graph_ids = {"program": "bd-program", "experiment-alpha": "bd-alpha", "refinery": "bd-refinery"}
        args = types.SimpleNamespace(
            intent=str(self.intent), graph=str(graph_path), review=str(review_path),
            mayor_context="mayor-context", rig="repo", binding="factory", result=None,
        )

        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_admit(args), 0)
            self.assertEqual(factory.command_admit(args), 0)

        self.assertEqual(beads.events[0], ("graph_create",))
        self.assertEqual(sum(event[0] == "graph_create" for event in beads.events), 1)
        self.assertEqual([event[:2] for event in beads.events[1:4]], [
            ("update", "bd-program"), ("update", "bd-alpha"), ("update", "bd-refinery"),
        ])
        self.assertEqual(beads.graph_plan["commit_message"], "factory: admit factory-test bead graph")
        self.assertFalse(any(path.name == "state.json" for path in self.root.rglob("*")))

    def test_mayor_emits_a_runtime_bound_graph_for_the_assigned_planning_bead(self) -> None:
        evidence = self.repo / ".gc" / "agentops-factory" / "planning" / "factory-test"
        evidence.mkdir(parents=True)
        graph_path = evidence / "program-graph.json"
        graph_path.write_text(json.dumps(self.graph([self.node("alpha", "src/alpha")])) + "\n", encoding="utf-8")
        request_path = evidence / "mayor-request.json"
        result_path = evidence / "mayor-response.json"
        request_path.write_text(json.dumps({
            "schema_version": "factory-role-request.v1",
            "request_id": "factory-test-mayor",
            "role": "mayor",
            "provider": "codex",
            "program_id": "factory-test",
            "workspace": str(self.repo),
            "intent_source": str(self.intent),
            "intent_digest": hashlib.sha256(self.intent.read_bytes()).hexdigest(),
            "repository": str(self.repo),
            "base_branch": "main",
            "base_sha": self.base_sha,
            "subject_path": None,
            "subject_digest": None,
            "mayor_context_id": None,
            "artifact_path": str(graph_path),
            "result_path": str(result_path),
        }) + "\n", encoding="utf-8")
        args = types.SimpleNamespace(request=str(request_path), bead="bd-planning", artifact=str(graph_path))
        runtime = {
            "GC_SESSION_ID": "mayor-session",
            "GC_SESSION_NAME": "mayor-1",
            "GC_TEMPLATE": "factory.mayor",
            "GC_PROVIDER": "codex",
        }

        with mock.patch.dict(factory.os.environ, runtime, clear=False), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_emit_role(args), 0)

        response = json.loads(result_path.read_text(encoding="utf-8"))
        self.assertEqual(response["bead_id"], "bd-planning")
        self.assertEqual(response["session_context_id"], "mayor-session")
        self.assertEqual(response["artifact_digest"], hashlib.sha256(graph_path.read_bytes()).hexdigest())

    def test_role_dispatch_resumes_existing_closed_transport_without_reslinging(self) -> None:
        evidence = self.repo / ".gc" / "agentops-factory" / "resume-role"
        evidence.mkdir(parents=True)
        graph_path = evidence / "program-graph.json"
        graph_path.write_text(json.dumps(self.graph([self.node("alpha", "src/alpha")])) + "\n", encoding="utf-8")
        request_path = evidence / "mayor-request.json"
        result_path = evidence / "mayor-response.json"
        request = {
            "schema_version": "factory-role-request.v1",
            "request_id": "factory-test-mayor-resume",
            "role": "mayor",
            "provider": "codex",
            "program_id": "factory-test",
            "workspace": str(self.repo),
            "intent_source": str(self.intent),
            "intent_digest": hashlib.sha256(self.intent.read_bytes()).hexdigest(),
            "repository": str(self.repo),
            "base_branch": "main",
            "base_sha": self.base_sha,
            "subject_path": None,
            "subject_digest": None,
            "mayor_context_id": None,
            "artifact_path": str(graph_path),
            "result_path": str(result_path),
        }
        request_path.write_text(json.dumps(request) + "\n", encoding="utf-8")
        response = {
            "schema_version": "factory-role-response.v1",
            "request_id": request["request_id"],
            "role": "mayor",
            "provider": "codex",
            "bead_id": "hq-planning",
            "session_context_id": "mayor-session",
            "session_name": "mayor-1",
            "template": "factory.mayor",
            "artifact_path": str(graph_path),
            "artifact_digest": hashlib.sha256(graph_path.read_bytes()).hexdigest(),
        }
        result_path.write_text(json.dumps(response) + "\n", encoding="utf-8")
        beads = FakeBeads({
            "hq-planning": {
                "id": "hq-planning", "status": "closed", "assignee": "mayor-1",
                "metadata": {
                    "factory.kind": "planning",
                    "factory.request_digest": hashlib.sha256(request_path.read_bytes()).hexdigest(),
                },
            },
        })
        session = {
            "session_name": "mayor-1", "template": "factory.mayor", "provider": "codex",
        }

        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "runtime_session", return_value=session),
            mock.patch.object(factory, "output", side_effect=AssertionError("resume must not sling")),
        ):
            resumed = factory.dispatch_role(request_path, "repo", "factory", 1)

        self.assertEqual(resumed["work_bead"], "hq-planning")
        self.assertEqual(resumed["session_context_id"], "mayor-session")

    def test_each_ready_experiment_gets_an_isolated_branch_worktree_and_fence(self) -> None:
        records = {}
        for bead_id, node_id in (("bd-alpha", "alpha"), ("bd-beta", "beta")):
            records[bead_id] = {
                "id": bead_id,
                "status": "open",
                "metadata": {
                    "factory.kind": "experiment",
                    "factory.status": "admitted",
                    "factory.repository": str(self.repo),
                    "factory.base_sha": self.base_sha,
                    "factory.program_id": "factory-test",
                    "factory.program_bead": "bd-program",
                    "factory.node_id": node_id,
                    "factory.attempt": "1",
                    "factory.spec": json.dumps(self.node(node_id, f"src/{node_id}")),
                },
            }
        beads = FakeBeads(records, set(records))
        worktrees = self.root / "worktrees"

        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            for bead_id in records:
                args = types.SimpleNamespace(rig="repo", bead=bead_id, worktree_root=str(worktrees))
                self.assertEqual(factory.command_lease(args), 0)

        alpha = records["bd-alpha"]["metadata"]
        beta = records["bd-beta"]["metadata"]
        self.assertNotEqual(alpha["factory.branch"], beta["factory.branch"])
        self.assertNotEqual(alpha["factory.worktree"], beta["factory.worktree"])
        self.assertNotEqual(alpha["factory.git_index"], beta["factory.git_index"])
        self.assertNotEqual(alpha["factory.lease_token"], beta["factory.lease_token"])
        self.assertEqual(alpha["factory.fence_epoch"], "1")
        self.assertEqual(beta["factory.fence_epoch"], "1")
        self.assertEqual(alpha["factory.candidate_base_sha"], self.base_sha)
        self.assertEqual(beta["factory.candidate_base_sha"], self.base_sha)
        self.assertEqual(alpha["factory.predecessor_beads"], [])

    def test_lease_preparation_reconciles_existing_worktree_without_new_identity(self) -> None:
        record = {
            "id": "bd-alpha", "status": "open",
            "metadata": {
                "factory.kind": "experiment", "factory.status": "admitted",
                "factory.repository": str(self.repo), "factory.base_sha": self.base_sha,
                "factory.program_id": "factory-test", "factory.program_bead": "bd-program",
                "factory.node_id": "alpha", "factory.attempt": "1",
                "factory.spec": json.dumps(self.node("alpha", "alpha.txt")),
            },
        }
        beads = FakeBeads({"bd-alpha": record}, {"bd-alpha"})
        root = self.root / "recovering-worktrees"
        with mock.patch.object(factory, "Beads", return_value=beads):
            first = factory.lease_experiment("repo", "bd-alpha", root)
            record["metadata"]["factory.status"] = "lease_preparing"
            record["metadata"].pop("factory.candidate_base_sha")
            record["metadata"].pop("factory.execution_phase")
            recovered = factory.lease_experiment("repo", "bd-alpha", root)

        self.assertEqual(recovered["worktree"], first["worktree"])
        self.assertEqual(recovered["lease_token"], first["lease_token"])
        self.assertEqual(recovered["fence_epoch"], first["fence_epoch"])
        self.assertEqual(record["metadata"]["factory.status"], "leased")

    def test_program_execute_recovers_an_already_leased_experiment(self) -> None:
        beads, record, _verdict_args = self.leased_experiment("PASS")
        record["metadata"].update({
            "factory.branch": "candidate-pass",
            "factory.git_index": str(self.repo / ".git" / "worktrees" / "candidate-pass" / "index"),
        })
        beads.records["bd-program"] = {
            "id": "bd-program", "status": "open",
            "metadata": {
                "factory.kind": "program", "factory.refinery_bead": "bd-refinery",
            },
        }
        args = types.SimpleNamespace(
            rig="repo", program_bead="bd-program", bead=[],
            worktree_root=str(self.root / "unused-worktrees"), max_parallel=1,
            max_attempts=3, timeout=30, result=None,
        )

        def recovered_execute(*_args, **_kwargs):
            record["metadata"]["factory.status"] = "passed"
            record["status"] = "closed"
            return {
                "bead": "bd-experiment", "node_id": "alpha", "verdict": "PASS",
                "candidate_rig": "fx-alpha", "branch": "candidate-pass",
                "candidate_sha": factory.git_head(Path(record["metadata"]["factory.worktree"])),
                "implementer_context_id": "author", "validator_context_id": "validator",
            }

        with (
            mock.patch.dict(factory.os.environ, {
                "GC_CITY_PATH": str(self.root / "city"), "GC_BIN": "/usr/bin/true",
            }, clear=False),
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "lease_experiment") as lease,
            mock.patch.object(factory, "register_candidate_rig", return_value=("fx-alpha", "factory")),
            mock.patch.object(factory, "execute_experiment", side_effect=recovered_execute),
            contextlib.redirect_stdout(io.StringIO()) as stdout,
        ):
            self.assertEqual(factory.command_execute(args), 0)

        lease.assert_not_called()
        result = json.loads(stdout.getvalue())
        self.assertEqual(result["executed"], 1)
        self.assertEqual(result["waves"][0][0]["bead"], "bd-experiment")

    def test_program_execute_recovers_an_open_verdict_reducer_phase(self) -> None:
        beads, record, _verdict_args = self.leased_experiment("FAIL")
        record["metadata"].update({
            "factory.status": "rejection_preparing",
            "factory.branch": "candidate-fail",
            "factory.git_index": str(self.repo / ".git" / "index"),
            "factory.max_attempts": "3",
        })
        beads.records["bd-program"] = {
            "id": "bd-program", "status": "open",
            "metadata": {
                "factory.kind": "program", "factory.refinery_bead": "bd-refinery",
            },
        }
        beads.records["rescope-preparing"] = {
            "id": "rescope-preparing", "status": "open",
            "metadata": {
                "factory.kind": "rescope", "factory.status": "mayor_required",
                "factory.program_bead": "bd-program", "factory.rejected_bead": "bd-experiment",
            },
        }
        args = types.SimpleNamespace(
            rig="repo", program_bead="bd-program", bead=[],
            worktree_root=str(self.root / "unused-worktrees"), max_parallel=1,
            max_attempts=3, timeout=30, result=None,
        )

        def recover_reducer(*_args, **_kwargs):
            record["metadata"]["factory.status"] = "rejected"
            record["status"] = "closed"
            beads.records["rescope-preparing"]["metadata"]["factory.status"] = "successor_admitted"
            beads.records["rescope-preparing"]["status"] = "closed"
            return {"bead": "bd-experiment", "node_id": "alpha", "verdict": "FAIL"}

        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "lease_experiment") as lease,
            mock.patch.object(factory, "register_candidate_rig", return_value=("fx-alpha", "factory")),
            mock.patch.object(factory, "execute_experiment", side_effect=recover_reducer),
            mock.patch.object(factory, "rescope_rejection") as rescope,
            contextlib.redirect_stdout(io.StringIO()) as stdout,
        ):
            self.assertEqual(factory.command_execute(args), 0)

        lease.assert_not_called()
        rescope.assert_not_called()
        result = json.loads(stdout.getvalue())
        self.assertEqual(result["executed"], 1)
        self.assertEqual(record["status"], "closed")

    def test_program_execute_recovers_a_mayor_required_rescope_then_runs_successor(self) -> None:
        beads, rejected, verdict_args = self.leased_experiment("FAIL")
        beads.records["bd-program"] = {
            "id": "bd-program", "status": "open",
            "metadata": {
                "factory.kind": "program", "factory.refinery_bead": "bd-refinery",
            },
        }
        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_record_verdict(verdict_args), 0)
        rescope_bead = rejected["metadata"]["factory.rescope_bead"]
        successor = {
            "id": "bd-successor", "status": "open",
            "metadata": {
                "factory.kind": "experiment", "factory.status": "admitted",
                "factory.program_bead": "bd-program", "factory.node_id": "alpha-v2",
            },
        }

        def recover_rescope(_rig, bead_id, _timeout):
            self.assertEqual(bead_id, rescope_bead)
            beads.records[rescope_bead]["metadata"]["factory.status"] = "successor_admitted"
            beads.records[rescope_bead]["status"] = "closed"
            beads.records["bd-successor"] = successor
            beads.ready.add("bd-successor")
            return {
                "rescope_bead": rescope_bead, "successor_bead": "bd-successor",
                "transport_bead": "hq-rescope", "mayor_context_id": "fresh-mayor",
            }

        def run_successor(*_args, **_kwargs):
            successor["metadata"]["factory.status"] = "passed"
            successor["status"] = "closed"
            return {"bead": "bd-successor", "node_id": "alpha-v2", "verdict": "PASS"}

        args = types.SimpleNamespace(
            rig="repo", program_bead="bd-program", bead=[],
            worktree_root=str(self.root / "recovery-worktrees"), max_parallel=1,
            max_attempts=3, timeout=30, result=None,
        )
        lease = {
            "bead": "bd-successor", "branch": "candidate-successor",
            "worktree": str(self.repo), "git_index": str(self.repo / ".git" / "index"),
            "lease_token": "lease", "fence_epoch": 1,
            "candidate_base_sha": self.base_sha, "predecessor_beads": [], "predecessor_shas": [],
        }
        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "rescope_rejection", side_effect=recover_rescope) as rescope,
            mock.patch.object(factory, "lease_experiment", return_value=lease),
            mock.patch.object(factory, "register_candidate_rig", return_value=("fx-successor", "factory")),
            mock.patch.object(factory, "execute_experiment", side_effect=run_successor),
            contextlib.redirect_stdout(io.StringIO()) as stdout,
        ):
            self.assertEqual(factory.command_execute(args), 0)

        rescope.assert_called_once()
        result = json.loads(stdout.getvalue())
        self.assertEqual(result["executed"], 1)
        self.assertEqual(result["reconciled"][0]["status"], "successor_admitted")

    def test_dependent_bead_starts_from_admitted_predecessor_content(self) -> None:
        alpha_tree = self.root / "alpha-candidate"
        git(self.repo, "worktree", "add", "-b", "alpha-candidate", str(alpha_tree), self.base_sha)
        (alpha_tree / "alpha.txt").write_text("admitted alpha\n", encoding="utf-8")
        git(alpha_tree, "add", "alpha.txt")
        git(alpha_tree, "commit", "-m", "alpha candidate")
        alpha_sha = git(alpha_tree, "rev-parse", "HEAD")
        alpha = {
            "id": "bd-alpha",
            "status": "closed",
            "metadata": {
                "factory.kind": "experiment",
                "factory.program_bead": "bd-program",
                "factory.node_id": "alpha",
                "factory.verdict": "PASS",
                "factory.candidate_sha": alpha_sha,
                "factory.spec": json.dumps(self.node("alpha", "alpha.txt")),
            },
        }
        beta_spec = self.node("beta", "beta.txt", depends_on=["alpha"], provider="claude")
        beta = {
            "id": "bd-beta",
            "status": "open",
            "metadata": {
                "factory.kind": "experiment",
                "factory.status": "admitted",
                "factory.repository": str(self.repo),
                "factory.base_sha": self.base_sha,
                "factory.program_id": "factory-test",
                "factory.program_bead": "bd-program",
                "factory.node_id": "beta",
                "factory.attempt": "1",
                "factory.spec": json.dumps(beta_spec),
            },
        }
        beads = FakeBeads({"bd-alpha": alpha, "bd-beta": beta}, {"bd-beta"})
        with mock.patch.object(factory, "Beads", return_value=beads):
            lease = factory.lease_experiment("repo", "bd-beta", self.root / "dependent-worktrees")

        beta_tree = Path(lease["worktree"])
        self.assertEqual((beta_tree / "alpha.txt").read_text(encoding="utf-8"), "admitted alpha\n")
        self.assertEqual(lease["predecessor_beads"], ["bd-alpha"])
        self.assertEqual(lease["predecessor_shas"], [alpha_sha])
        self.assertNotEqual(lease["candidate_base_sha"], self.base_sha)

    def test_dependent_bead_resolves_rejected_dependency_through_successor_chain(self) -> None:
        successor_tree = self.root / "alpha-v3-candidate"
        git(self.repo, "worktree", "add", "-b", "alpha-v3-candidate", str(successor_tree), self.base_sha)
        (successor_tree / "alpha-v3.txt").write_text("admitted successor\n", encoding="utf-8")
        git(successor_tree, "add", "alpha-v3.txt")
        git(successor_tree, "commit", "-m", "alpha successor")
        successor_sha = git(successor_tree, "rev-parse", "HEAD")
        rejected = {
            "id": "bd-alpha", "status": "closed",
            "metadata": {
                "factory.kind": "experiment", "factory.program_bead": "bd-program",
                "factory.node_id": "alpha", "factory.verdict": "FAIL",
                "factory.successor_bead": "bd-alpha-v2",
                "factory.spec": json.dumps(self.node("alpha", "alpha.txt")),
            },
        }
        rejected_v2 = {
            "id": "bd-alpha-v2", "status": "closed",
            "metadata": {
                "factory.kind": "experiment", "factory.program_bead": "bd-program",
                "factory.node_id": "alpha-v2", "factory.verdict": "NOT_PROVEN",
                "factory.successor_bead": "bd-alpha-v3",
                "factory.spec": json.dumps(self.node("alpha-v2", "alpha-v2.txt")),
            },
        }
        successor = {
            "id": "bd-alpha-v3", "status": "closed",
            "metadata": {
                "factory.kind": "experiment", "factory.program_bead": "bd-program",
                "factory.node_id": "alpha-v3", "factory.verdict": "PASS",
                "factory.candidate_sha": successor_sha,
                "factory.spec": json.dumps(self.node("alpha-v3", "alpha-v3.txt")),
            },
        }
        beta = {
            "id": "bd-beta", "status": "open",
            "metadata": {
                "factory.kind": "experiment", "factory.status": "admitted",
                "factory.repository": str(self.repo), "factory.base_sha": self.base_sha,
                "factory.program_id": "factory-test", "factory.program_bead": "bd-program",
                "factory.node_id": "beta", "factory.attempt": "1",
                "factory.spec": json.dumps(self.node("beta", "beta.txt", depends_on=["alpha"])),
            },
        }
        beads = FakeBeads({
            "bd-alpha": rejected, "bd-alpha-v2": rejected_v2,
            "bd-alpha-v3": successor, "bd-beta": beta,
        }, {"bd-beta"})
        with mock.patch.object(factory, "Beads", return_value=beads):
            lease = factory.lease_experiment("repo", "bd-beta", self.root / "successor-worktrees")

        beta_tree = Path(lease["worktree"])
        self.assertEqual((beta_tree / "alpha-v3.txt").read_text(encoding="utf-8"), "admitted successor\n")
        self.assertEqual(lease["predecessor_beads"], ["bd-alpha-v3"])
        self.assertEqual(lease["predecessor_shas"], [successor_sha])

    def leased_experiment(self, verdict_result: str) -> tuple[FakeBeads, dict, types.SimpleNamespace]:
        worktree = self.root / f"candidate-{verdict_result.lower()}"
        git(self.repo, "worktree", "add", "-b", f"candidate-{verdict_result.lower()}", str(worktree), self.base_sha)
        (worktree / "candidate.txt").write_text(f"{verdict_result}\n", encoding="utf-8")
        git(worktree, "add", "candidate.txt")
        git(worktree, "commit", "-m", f"candidate {verdict_result}")
        candidate_sha = git(worktree, "rev-parse", "HEAD")
        intent_digest = "3" * 64
        record = {
            "id": "bd-experiment",
            "status": "open",
            "metadata": {
                "factory.kind": "experiment",
                "factory.status": "leased",
                "factory.program_id": "factory-test",
                "factory.program_bead": "bd-program",
                "factory.refinery_bead": "bd-refinery",
                "factory.node_id": "alpha",
                "factory.attempt": "1",
                "factory.spec": json.dumps(self.node("alpha", "candidate.txt")),
                "factory.intent_digest": intent_digest,
                "factory.parent_intent_digest": hashlib.sha256(self.intent.read_bytes()).hexdigest(),
                "factory.intent_source": str(self.intent),
                "factory.repository": str(self.repo),
                "factory.base_branch": "main",
                "factory.base_sha": self.base_sha,
                "factory.graph_digest": "1" * 64,
                "factory.review_digest": "2" * 64,
                "factory.rig": "repo",
                "factory.binding": "factory",
                "factory.adapter_path": str(MODULE_PATH),
                "factory.lease_token": "lease-token",
                "factory.fence_epoch": "1",
                "factory.worktree": str(worktree),
                "factory.candidate_base_sha": self.base_sha,
                "factory.predecessor_beads": [],
                "factory.predecessor_shas": [],
            },
        }
        beads = FakeBeads({
            "bd-experiment": record,
            "bd-refinery": {
                "id": "bd-refinery", "status": "open",
                "metadata": {"factory.kind": "refinery", "factory.program_bead": "bd-program"},
            },
        })
        subject_digest = "4" * 64
        manifest = self.root / f"manifest-{verdict_result}.json"
        manifest.write_text(json.dumps({"canonical_manifest_digest": subject_digest}) + "\n", encoding="utf-8")
        verdict = self.root / f"verdict-{verdict_result}.json"
        verdict.write_text(json.dumps({
            "schema_version": "verdict.v2",
            "verdict": verdict_result,
            "acceptance_digest": intent_digest,
            "subject_manifest_digest": subject_digest,
            "author_context_id": "author-context",
            "validator_context_id": "validator-context",
            "freshness_attestation": {"source": "runtime", "attester_identity": "validator-context"},
        }) + "\n", encoding="utf-8")
        args = types.SimpleNamespace(
            rig="repo", bead="bd-experiment", lease_token="lease-token", fence_epoch=1,
            candidate_sha=candidate_sha, subject_manifest=str(manifest),
            author_context="author-context", verdict=str(verdict),
        )
        return beads, record, args

    def test_pass_certificate_and_terminal_status_are_attached_to_experiment_bead(self) -> None:
        beads, record, args = self.leased_experiment("PASS")
        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_record_verdict(args), 0)

        meta = record["metadata"]
        self.assertEqual(meta["factory.verdict"], "PASS")
        self.assertEqual(meta["factory.status"], "passed")
        self.assertTrue(Path(meta["factory.admission_path"]).is_file())
        self.assertEqual(record["status"], "closed")
        close_count = sum(event[:2] == ("close", "bd-experiment") for event in beads.events)
        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_record_verdict(args), 0)
        self.assertEqual(sum(event[:2] == ("close", "bd-experiment") for event in beads.events), close_count)

    def test_attempt_ceiling_is_persisted_on_rescope_before_rejected_experiment_closes(self) -> None:
        beads, record, args = self.leased_experiment("FAIL")
        record["metadata"]["factory.attempt"] = "3"
        record["metadata"]["factory.max_attempts"] = "3"

        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_record_verdict(args), 0)

        rescope = beads.records[record["metadata"]["factory.rescope_bead"]]
        self.assertEqual(rescope["metadata"]["factory.status"], "hold")
        self.assertIn("attempt 3 of 3", rescope["metadata"]["factory.hold_reason"])
        self.assertEqual(record["status"], "closed")

    def test_attempt_ceiling_repairs_an_existing_mayor_required_rescope_to_hold(self) -> None:
        beads, record, args = self.leased_experiment("FAIL")
        record["metadata"].update({"factory.attempt": "3", "factory.max_attempts": "3"})
        verdict_digest = hashlib.sha256(Path(args.verdict).read_bytes()).hexdigest()
        beads.records["rescope-old"] = {
            "id": "rescope-old", "status": "open",
            "metadata": {
                "factory.kind": "rescope", "factory.status": "mayor_required",
                "factory.program_id": "factory-test", "factory.program_bead": "bd-program",
                "factory.refinery_bead": "bd-refinery", "factory.rejected_bead": "bd-experiment",
                "factory.verdict_digest": verdict_digest,
            },
        }

        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_record_verdict(args), 0)

        rescope = beads.records["rescope-old"]["metadata"]
        self.assertEqual(rescope["factory.status"], "hold")
        self.assertEqual(rescope["factory.max_attempts"], "3")
        self.assertIn("attempt 3 of 3", rescope["factory.hold_reason"])

    def test_rejection_creates_refinery_blocking_rescope_bead_before_closure(self) -> None:
        beads, record, args = self.leased_experiment("NOT_PROVEN")
        beads.records["bd-dependent"] = {
            "id": "bd-dependent",
            "status": "open",
            "metadata": {
                "factory.kind": "experiment",
                "factory.program_bead": "bd-program",
                "factory.spec": json.dumps(self.node("dependent", "dependent.txt", depends_on=["alpha"])),
            },
        }
        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_record_verdict(args), 0)

        rescope = record["metadata"]["factory.rescope_bead"]
        dep_index = beads.events.index(("dep_add", "bd-refinery", rescope, "blocks"))
        close_index = next(index for index, event in enumerate(beads.events) if event[:2] == ("close", "bd-experiment"))
        self.assertLess(dep_index, close_index)
        dependent_dep = beads.events.index(("dep_add", "bd-dependent", rescope, "blocks"))
        self.assertLess(dependent_dep, close_index)
        self.assertEqual(beads.records[rescope]["metadata"]["factory.kind"], "rescope")
        self.assertEqual(record["metadata"]["factory.status"], "rejected")
        create_count = sum(event[0] == "create" for event in beads.events)
        dep_count = sum(event[0] == "dep_add" for event in beads.events)
        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_record_verdict(args), 0)
        self.assertEqual(sum(event[0] == "create" for event in beads.events), create_count)
        self.assertEqual(sum(event[0] == "dep_add" for event in beads.events), dep_count)

    def test_rejection_routes_rescope_bead_to_fresh_mayor_and_admits_new_successor(self) -> None:
        beads, record, args = self.leased_experiment("FAIL")
        record["metadata"]["factory.mayor_context_id"] = "prior-rescope-mayor-context"
        beads.records["bd-program"] = {
            "id": "bd-program",
            "status": "open",
            "metadata": {
                "factory.kind": "program",
                "factory.program_id": "factory-test",
                "factory.repository": str(self.repo),
                "factory.intent_source": str(self.intent),
                "factory.intent_digest": hashlib.sha256(self.intent.read_bytes()).hexdigest(),
                "factory.base_branch": "main",
                "factory.base_sha": self.base_sha,
                "factory.mayor_context_id": "original-mayor-context",
            },
        }
        beads.records["bd-refinery"] = {
            "id": "bd-refinery", "status": "open",
            "metadata": {"factory.kind": "refinery", "factory.program_bead": "bd-program"},
        }
        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_record_verdict(args), 0)
        rescope_bead = record["metadata"]["factory.rescope_bead"]

        hq = FakeBeads({
            "hq-rescope": {"id": "hq-rescope", "status": "closed", "metadata": {}},
        })

        def beads_for(rig=None):
            return hq if rig is None else beads

        def mayor_dispatch(request_path, rig, binding, timeout, **kwargs):
            request = json.loads(Path(request_path).read_text(encoding="utf-8"))
            self.assertEqual(request["role"], "rescope")
            self.assertEqual(request["mayor_context_id"], "prior-rescope-mayor-context")
            self.assertEqual(kwargs["linked_bead"], rescope_bead)
            successor = self.node("alpha-v2", "candidate-v2.txt", provider="claude")
            successor["acceptance"] = self.node("alpha", "candidate.txt")["acceptance"]
            successor["supersedes"] = "alpha"
            Path(request["artifact_path"]).write_text(json.dumps(successor) + "\n", encoding="utf-8")
            return {
                "work_bead": "hq-rescope",
                "session_context_id": "fresh-mayor-context",
            }

        with (
            mock.patch.object(factory, "Beads", side_effect=beads_for),
            mock.patch.object(factory, "dispatch_role", side_effect=mayor_dispatch),
        ):
            result = factory.rescope_rejection("repo", rescope_bead, 10)
            rescope_record = beads.records[rescope_bead]
            rescope_record["status"] = "open"
            rescope_record["metadata"]["factory.status"] = "successor_preparing"
            rescope_record["metadata"].pop("factory.successor_bead")
            record["metadata"].pop("factory.successor_bead")
            hq.records["hq-rescope"]["metadata"].clear()
            replayed = factory.rescope_rejection("repo", rescope_bead, 10)

        successor_bead = result["successor_bead"]
        successor_meta = beads.records[successor_bead]["metadata"]
        self.assertEqual(successor_meta["factory.node_id"], "alpha-v2")
        self.assertEqual(successor_meta["factory.supersedes_bead"], "bd-experiment")
        self.assertEqual(successor_meta["factory.attempt"], "2")
        self.assertEqual(successor_meta["factory.rig"], "repo")
        self.assertEqual(successor_meta["factory.binding"], "factory")
        self.assertEqual(successor_meta["factory.adapter_path"], str(MODULE_PATH))
        self.assertEqual(successor_meta["factory.mayor_context_id"], "fresh-mayor-context")
        self.assertEqual(beads.records[rescope_bead]["status"], "closed")
        self.assertEqual(hq.records["hq-rescope"]["metadata"]["factory.successor_bead"], successor_bead)
        self.assertEqual(result["mayor_context_id"], "fresh-mayor-context")
        self.assertEqual(replayed["successor_bead"], successor_bead)
        self.assertEqual(sum(event[0] == "graph_create" for event in beads.events), 1)

    def test_stale_fence_rejects_verdict_without_mutating_beads(self) -> None:
        beads, _record, args = self.leased_experiment("PASS")
        args.lease_token = "stale-token"
        with mock.patch.object(factory, "Beads", return_value=beads):
            with self.assertRaisesRegex(factory.FactoryError, "lease token or fence epoch is stale"):
                factory.command_record_verdict(args)
        self.assertEqual(beads.events, [])

    def test_refinery_integrates_pass_certificates_in_predecessor_order(self) -> None:
        alpha_tree = self.root / "ref-alpha"
        git(self.repo, "worktree", "add", "-b", "ref-alpha", str(alpha_tree), self.base_sha)
        (alpha_tree / "alpha.txt").write_text("alpha\n", encoding="utf-8")
        git(alpha_tree, "add", "alpha.txt")
        git(alpha_tree, "commit", "-m", "alpha")
        alpha_sha = git(alpha_tree, "rev-parse", "HEAD")

        beta_tree = self.root / "ref-beta"
        git(self.repo, "worktree", "add", "-b", "ref-beta", str(beta_tree), self.base_sha)
        git(beta_tree, "cherry-pick", alpha_sha)
        beta_base = git(beta_tree, "rev-parse", "HEAD")
        (beta_tree / "beta.txt").write_text("beta\n", encoding="utf-8")
        git(beta_tree, "add", "beta.txt")
        git(beta_tree, "commit", "-m", "beta")
        beta_sha = git(beta_tree, "rev-parse", "HEAD")

        def admitted(bead_id: str, node_id: str, candidate_sha: str, candidate_base: str,
                     predecessors: list[str], predecessor_shas: list[str], scope: str) -> dict:
            certificate = self.root / f"{bead_id}-admission.json"
            certificate.write_text(json.dumps({
                "candidate_sha": candidate_sha,
                "candidate_base_sha": candidate_base,
                "predecessor_beads": predecessors,
                "predecessor_shas": predecessor_shas,
                "verdict": "PASS",
            }) + "\n", encoding="utf-8")
            return {
                "id": bead_id,
                "status": "closed",
                "metadata": {
                    "factory.kind": "experiment",
                    "factory.program_bead": "bd-program",
                    "factory.node_id": node_id,
                    "factory.verdict": "PASS",
                    "factory.candidate_sha": candidate_sha,
                    "factory.candidate_base_sha": candidate_base,
                    "factory.predecessor_beads": predecessors,
                    "factory.predecessor_shas": predecessor_shas,
                    "factory.admission_path": str(certificate),
                    "factory.admission_digest": hashlib.sha256(certificate.read_bytes()).hexdigest(),
                    "factory.subject": json.dumps({"includes": [scope], "excludes": []}),
                    "factory.write_scope": json.dumps([scope]),
                    "factory.generated_scope": "[]",
                },
            }

        alpha = admitted("bd-alpha", "alpha", alpha_sha, self.base_sha, [], [], "alpha.txt")
        beta = admitted("bd-beta", "beta", beta_sha, beta_base, ["bd-alpha"], [alpha_sha], "beta.txt")
        refinery = {
            "id": "bd-refinery",
            "status": "in_progress",
            "assignee": "refiner-session",
            "dependencies": [
                {"id": "bd-alpha", "dependency_type": "blocks", "status": "closed"},
                {"id": "bd-beta", "dependency_type": "blocks", "status": "closed"},
            ],
            "metadata": {
                "factory.kind": "refinery",
                "factory.status": "blocked",
                "factory.program_id": "factory-test",
                "factory.program_bead": "bd-program",
                "factory.binding": "factory",
                "factory.repository": str(self.repo),
                "factory.base_branch": "main",
                "factory.base_sha": self.base_sha,
                "factory.intent_digest": "5" * 64,
                "factory.fence_epoch": "0",
                "gc.routed_to": "repo/factory.refiner",
                "gc.session_name": "refiner-session",
            },
        }
        beads = FakeBeads({
            "bd-alpha": alpha, "bd-beta": beta, "bd-refinery": refinery,
        }, {"bd-refinery"})
        args = types.SimpleNamespace(
            rig="repo", refinery_bead="bd-refinery",
            worktree_root=str(self.root / "integration-worktrees"),
            max_candidates=5, remote="origin",
        )
        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_refinery_assemble(args), 0)

        meta = refinery["metadata"]
        integration = Path(meta["factory.integration_worktree"])
        self.assertEqual((integration / "alpha.txt").read_text(encoding="utf-8"), "alpha\n")
        self.assertEqual((integration / "beta.txt").read_text(encoding="utf-8"), "beta\n")
        self.assertEqual(meta["factory.candidate_beads"], ["bd-alpha", "bd-beta"])
        self.assertEqual(meta["factory.status"], "validation_required")
        self.assertTrue(Path(meta["factory.integration_subject_manifest"]).is_file())
        scope = json.loads(Path(meta["factory.integration_scope_receipt"]).read_text(encoding="utf-8"))
        self.assertEqual(scope["status"], "PASS")
        integration_sha = meta["factory.integration_sha"]
        meta["factory.status"] = "assembling"
        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_refinery_assemble(args), 0)
        self.assertEqual(meta["factory.integration_sha"], integration_sha)
        self.assertEqual(meta["factory.status"], "validation_required")
        first_worktree = meta["factory.integration_worktree"]
        meta["factory.status"] = "reassembly_required"
        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_refinery_assemble(args), 0)
        self.assertEqual(meta["factory.fence_epoch"], "2")
        self.assertNotEqual(meta["factory.integration_worktree"], first_worktree)
        self.assertEqual((Path(meta["factory.integration_worktree"]) / "alpha.txt").read_text(), "alpha\n")
        self.assertEqual((Path(meta["factory.integration_worktree"]) / "beta.txt").read_text(), "beta\n")

    def test_refinery_claim_requires_closed_dependencies_and_exact_refiner_session(self) -> None:
        refinery = {
            "id": "bd-refinery",
            "status": "in_progress",
            "assignee": "wrong-session",
            "dependencies": [
                {"id": "bd-alpha", "dependency_type": "blocks", "status": "closed"},
            ],
            "metadata": {
                "factory.kind": "refinery",
                "factory.status": "blocked",
                "factory.binding": "factory",
                "gc.routed_to": "repo/factory.refiner",
                "gc.session_name": "refiner-session",
            },
        }
        beads = FakeBeads({"bd-refinery": refinery})
        args = types.SimpleNamespace(
            rig="repo", refinery_bead="bd-refinery",
            worktree_root=str(self.root / "integration-worktrees"),
            max_candidates=5, remote="origin",
        )
        with mock.patch.object(factory, "Beads", return_value=beads):
            with self.assertRaisesRegex(factory.FactoryError, "not owned by its routed Refiner session"):
                factory.command_refinery_assemble(args)

        refinery["assignee"] = "refiner-session"
        refinery["dependencies"][0]["status"] = "open"
        with mock.patch.object(factory, "Beads", return_value=beads):
            with self.assertRaisesRegex(factory.FactoryError, "unresolved dependencies"):
                factory.command_refinery_assemble(args)

    def test_dynamic_rig_policy_exposes_only_bead_selected_routes(self) -> None:
        city = self.root / "city"
        (city / ".gc").mkdir(parents=True)
        (city / "city.toml").write_text("[workspace]\nname = \"factory-test\"\n", encoding="utf-8")
        rig = "fx-alpha"

        def agent(name: str, suspended: bool = False) -> dict:
            return {
                "qualified_name": f"{rig}/{name}",
                "dir": rig,
                "suspended": suspended,
            }

        allowed = {"factory.implementer-claude", "factory.validator"}
        disallowed = {
            "factory.implementer", "factory.validator-claude",
            "factory.plan-reviewer", "factory.refiner", "codex", "agentops-factory.validator",
        }
        before = [agent(name) for name in sorted(allowed | disallowed)]
        after = [agent(name, name in disallowed) for name in sorted(allowed | disallowed)]
        runtime = {"GC_CITY_PATH": str(city), "GC_BIN": "/usr/bin/true"}
        with (
            mock.patch.dict(factory.os.environ, runtime, clear=False),
            mock.patch.object(factory, "configured_agents", side_effect=[before, after]),
            mock.patch.object(factory, "output", return_value="{}") as output_mock,
        ):
            factory.enforce_rig_agent_policy(
                rig, "factory",
                {"implementer-claude", "validator"},
            )

        config = factory.tomllib.loads((city / "city.toml").read_text(encoding="utf-8"))
        patches = config["patches"]["agent"]
        self.assertEqual(
            {(item["dir"], item["name"], item["suspended"]) for item in patches},
            {(rig, name, True) for name in disallowed},
        )
        output_mock.assert_called_once()

    def test_candidate_rig_policy_is_derived_from_the_experiment_bead_provider_pair(self) -> None:
        record = {
            "id": "bd-alpha", "status": "open",
            "metadata": {
                "factory.program_id": "factory-test", "factory.node_id": "alpha",
                "factory.attempt": "1", "factory.binding": "factory",
                "factory.provider": "claude", "factory.validator_provider": "codex",
            },
        }
        lease = {"worktree": str(self.repo), "branch": "candidate-alpha"}
        selected: list[set[str]] = []

        with (
            mock.patch.object(factory, "city_config_lock", return_value=contextlib.nullcontext()),
            mock.patch.object(factory, "configured_rigs", return_value=[{
                "name": "fx-factory-test-alpha-1", "path": str(self.repo),
            }]),
            mock.patch.object(
                factory, "enforce_rig_agent_policy",
                side_effect=lambda _rig, _binding, roles: selected.append(set(roles)),
            ),
            mock.patch.object(factory, "restore_rig_scaffolding"),
        ):
            rig, binding = factory.register_candidate_rig(lease, record)

        self.assertEqual(rig, "fx-factory-test-alpha-1")
        self.assertEqual(binding, "factory")
        self.assertEqual(selected, [{"implementer-claude", "validator"}])

    def test_integration_validation_copies_intent_into_the_integration_rig(self) -> None:
        integration = self.root / "integration"
        git(self.repo, "worktree", "add", "-b", "integration-test", str(integration), self.base_sha)
        evidence = integration / ".gc" / "agentops-factory" / "bd-refinery"
        evidence.mkdir(parents=True)
        baseline = evidence / "baseline.json"
        subject = evidence / "subject.json"
        scope = evidence / "scope.json"
        baseline.write_text("{}\n", encoding="utf-8")
        subject.write_text(json.dumps({"canonical_manifest_digest": "7" * 64}) + "\n", encoding="utf-8")
        scope.write_text("{}\n", encoding="utf-8")
        refinery = {
            "id": "bd-refinery",
            "status": "open",
            "metadata": {
                "factory.kind": "refinery",
                "factory.status": "validation_required",
                "factory.binding": "factory",
                "factory.program_bead": "bd-program",
                "factory.intent_digest": hashlib.sha256(self.intent.read_bytes()).hexdigest(),
                "factory.fence_epoch": "1",
                "factory.fence_token": "fence-token",
                "factory.integration_sha": self.base_sha,
                "factory.integration_worktree": str(integration),
                "factory.integration_branch": "integration-test",
                "factory.integration_subject": json.dumps({"includes": ["README.md"], "excludes": []}),
                "factory.integration_baseline_manifest": str(baseline),
                "factory.integration_subject_manifest": str(subject),
                "factory.integration_scope_receipt": str(scope),
            },
        }
        program = {
            "id": "bd-program",
            "status": "open",
            "metadata": {"factory.intent_source": str(self.intent)},
        }
        beads = FakeBeads({"bd-refinery": refinery, "bd-program": program})

        def run_packet(packet_path: Path, rig: str, binding: str, timeout: float) -> dict:
            packet = json.loads(packet_path.read_text(encoding="utf-8"))
            copied_intent = Path(packet["intent_source"])
            self.assertTrue(copied_intent.is_relative_to(integration))
            self.assertEqual(copied_intent.read_bytes(), self.intent.read_bytes())
            self.assertEqual(Path(packet["evidence_dir"]), integration / ".gc" / "agentops" / packet["packet_id"])
            verdict = Path(packet["evidence_dir"]) / "verdict.v2.json"
            verdict.parent.mkdir(parents=True, exist_ok=True)
            verdict.write_text(json.dumps({"verdict": "PASS"}) + "\n", encoding="utf-8")
            return {
                "transport": {"session_context_id": "validator-context"},
                "agent_response": {"artifacts": [{"path": str(verdict)}]},
            }

        args = types.SimpleNamespace(
            rig="repo", refinery_bead="bd-refinery", provider="claude", timeout=30,
        )
        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "register_integration_rig", return_value=("integration-rig", "factory")),
            mock.patch.object(factory, "run_executor_packet", side_effect=run_packet),
            mock.patch.object(factory, "command_refinery_validate", return_value=0),
            contextlib.redirect_stdout(io.StringIO()),
        ):
            self.assertEqual(factory.command_refinery_run_validation(args), 0)

    def test_reassembled_epoch_registers_a_new_rig_before_fresh_validation(self) -> None:
        integration = self.root / "integration-epoch-2"
        git(self.repo, "worktree", "add", "-b", "integration-epoch-2", str(integration), self.base_sha)
        evidence = integration / ".gc" / "agentops-factory" / "bd-refinery"
        evidence.mkdir(parents=True)
        baseline = evidence / "baseline.json"
        subject = evidence / "subject.json"
        scope = evidence / "scope.json"
        baseline.write_text("{}\n", encoding="utf-8")
        subject.write_text(json.dumps({"canonical_manifest_digest": "8" * 64}) + "\n", encoding="utf-8")
        scope.write_text("{}\n", encoding="utf-8")
        refinery = {
            "id": "bd-refinery", "status": "open",
            "metadata": {
                "factory.kind": "refinery", "factory.status": "validation_required",
                "factory.binding": "factory", "factory.program_bead": "bd-program",
                "factory.intent_digest": hashlib.sha256(self.intent.read_bytes()).hexdigest(),
                "factory.fence_epoch": "2", "factory.fence_token": "epoch-2-token",
                "factory.integration_sha": self.base_sha,
                "factory.integration_worktree": str(integration),
                "factory.integration_branch": "gc/integration/factory-test/1/2",
                "factory.integration_subject": json.dumps({"includes": ["README.md"], "excludes": []}),
                "factory.integration_baseline_manifest": str(baseline),
                "factory.integration_subject_manifest": str(subject),
                "factory.integration_scope_receipt": str(scope),
            },
        }
        program = {
            "id": "bd-program", "status": "open",
            "metadata": {"factory.intent_source": str(self.intent)},
        }
        beads = FakeBeads({"bd-refinery": refinery, "bd-program": program})
        old_rig = {
            "name": "fx-refinery-bd-refinery-1",
            "path": str(self.root / "integration-epoch-1"),
        }

        def run_packet(packet_path: Path, rig: str, binding: str, timeout: float) -> dict:
            self.assertEqual(rig, "fx-refinery-bd-refinery-2")
            self.assertEqual(binding, "factory")
            packet = json.loads(packet_path.read_text(encoding="utf-8"))
            verdict = Path(packet["evidence_dir"]) / "verdict.v2.json"
            verdict.parent.mkdir(parents=True, exist_ok=True)
            verdict.write_text(json.dumps({"verdict": "PASS"}) + "\n", encoding="utf-8")
            return {
                "transport": {"session_context_id": "epoch-2-validator"},
                "agent_response": {"artifacts": [{"path": str(verdict)}]},
            }

        args = types.SimpleNamespace(
            rig="repo", refinery_bead="bd-refinery", provider="claude", timeout=30,
        )
        with (
            mock.patch.dict(factory.os.environ, {
                "GC_CITY_PATH": str(self.root / "city"), "GC_BIN": "/usr/bin/true",
            }, clear=False),
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "city_config_lock", return_value=contextlib.nullcontext()),
            mock.patch.object(factory, "configured_rigs", return_value=[old_rig]),
            mock.patch.object(factory, "add_gc_rig_with_local_origin", return_value={"ok": True}) as add_rig,
            mock.patch.object(factory, "enforce_rig_agent_policy"),
            mock.patch.object(factory, "restore_rig_scaffolding"),
            mock.patch.object(factory, "run_executor_packet", side_effect=run_packet),
            mock.patch.object(factory, "command_refinery_validate", return_value=0),
            contextlib.redirect_stdout(io.StringIO()),
        ):
            self.assertEqual(factory.command_refinery_run_validation(args), 0)

        self.assertIn("fx-refinery-bd-refinery-2", add_rig.call_args.args[1])

    def test_refinery_deliver_resumes_at_the_recorded_bead_phase(self) -> None:
        refinery = {
            "id": "bd-refinery",
            "status": "open",
            "metadata": {
                "factory.status": "validation_required",
                "factory.program_bead": "bd-program",
            },
        }
        beads = FakeBeads({"bd-refinery": refinery})

        def advance(status: str):
            def command(_args):
                refinery["metadata"]["factory.status"] = status
                return 0
            return command

        args = types.SimpleNamespace(
            rig="repo", refinery_bead="bd-refinery",
            worktree_root=str(self.root / "integration-worktrees"),
            max_candidates=5, remote="origin", provider="claude", timeout=30,
            title=None, draft=True, merge_method="squash",
        )
        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "command_refinery_assemble") as assemble,
            mock.patch.object(factory, "command_refinery_run_validation", side_effect=advance("validated")) as validate,
            mock.patch.object(factory, "command_refinery_publish", side_effect=advance("published")) as publish,
            mock.patch.object(factory, "command_refinery_land", side_effect=advance("landed")) as land,
            mock.patch.object(factory, "reconcile_landed_delivery", return_value={
                "refinery_bead": "bd-refinery", "program_bead": "bd-program",
                "pr_url": "https://example.invalid/pr/1", "landed_sha": "a" * 40,
                "delivery_record": "/tmp/delivery.json",
            }) as reconcile,
            contextlib.redirect_stdout(io.StringIO()),
        ):
            self.assertEqual(factory.command_refinery_deliver(args), 0)

        assemble.assert_not_called()
        validate.assert_called_once()
        publish.assert_called_once()
        land.assert_called_once()
        reconcile.assert_called_once()

    def test_landed_delivery_reconciliation_repairs_partial_bead_closure_idempotently(self) -> None:
        delivery = self.root / "delivery.json"
        delivery.write_text('{"schema_version":"delivery-record.v1","status":"landed"}\n', encoding="utf-8")
        refinery = {
            "id": "bd-refinery", "status": "open",
            "metadata": {
                "factory.kind": "refinery", "factory.status": "landed",
                "factory.program_bead": "bd-program", "factory.pr_url": "https://example.invalid/pr/7",
                "factory.landed_sha": "a" * 40, "factory.delivery_record": str(delivery),
                "factory.delivery_record_digest": hashlib.sha256(delivery.read_bytes()).hexdigest(),
            },
        }
        program = {"id": "bd-program", "status": "open", "metadata": {"factory.kind": "program"}}
        beads = FakeBeads({"bd-refinery": refinery, "bd-program": program})

        first = factory.reconcile_landed_delivery(beads, "bd-refinery")
        close_count = sum(event[0] == "close" for event in beads.events)
        second = factory.reconcile_landed_delivery(beads, "bd-refinery")

        self.assertEqual(first, second)
        self.assertEqual(close_count, 2)
        self.assertEqual(sum(event[0] == "close" for event in beads.events), close_count)
        self.assertEqual(program["metadata"]["factory.status"], "landed")
        self.assertEqual(program["metadata"]["factory.landed_sha"], "a" * 40)

    def test_refinery_land_reconciles_an_already_merged_pr_without_merging_again(self) -> None:
        delivery = self.root / "already-merged-delivery.json"
        delivery.write_text("{}\n", encoding="utf-8")
        integration_sha = "b" * 40
        landed_sha = "c" * 40
        refinery = {
            "id": "bd-refinery", "status": "open",
            "metadata": {
                "factory.kind": "refinery", "factory.status": "published",
                "factory.program_bead": "bd-program", "factory.integration_sha": integration_sha,
                "factory.fence_epoch": "1", "factory.fence_token": "token",
                "factory.pr_url": "https://example.invalid/pr/8", "factory.base_branch": "main",
            },
        }
        program = {"id": "bd-program", "status": "open", "metadata": {"factory.kind": "program"}}
        beads = FakeBeads({"bd-refinery": refinery, "bd-program": program})
        initial = {
            "url": refinery["metadata"]["factory.pr_url"], "number": 8, "state": "MERGED",
            "isDraft": False, "headRefOid": integration_sha, "headRefName": "gc/integration/test",
            "baseRefName": "main", "statusCheckRollup": [],
        }
        final = {
            "url": initial["url"], "number": 8, "state": "MERGED",
            "mergedAt": "2026-07-17T00:00:00Z", "mergeCommit": {"oid": landed_sha},
            "headRefOid": integration_sha, "baseRefName": "main",
        }
        args = types.SimpleNamespace(
            rig="repo", refinery_bead="bd-refinery", merge_method="squash", timeout=30,
        )
        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "check_fence", return_value=self.repo),
            mock.patch.object(factory, "gh_json", side_effect=[initial, final]),
            mock.patch.object(factory, "run_process") as process,
            mock.patch.object(factory, "persist_delivery_record", return_value=delivery),
            contextlib.redirect_stdout(io.StringIO()),
        ):
            self.assertEqual(factory.command_refinery_land(args), 0)

        process.assert_not_called()
        self.assertEqual(refinery["metadata"]["factory.status"], "landed")
        self.assertEqual(refinery["metadata"]["factory.landed_sha"], landed_sha)
        self.assertEqual(refinery["status"], "closed")
        self.assertEqual(program["status"], "closed")


if __name__ == "__main__":
    unittest.main()
