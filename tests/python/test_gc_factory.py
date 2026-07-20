from __future__ import annotations

import contextlib
import hashlib
import importlib.util
import inspect
import io
import json
import os
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

    def close(self, bead_id: str, reason: str, work_outcome: str,
              commit: str | None = None, branch: str | None = None,
              work_dir: str | Path | None = None) -> None:
        fields = {
            "gc.outcome": "fail" if work_outcome in {"blocked", "abandoned"} else "pass",
            "gc.work_outcome": work_outcome,
        }
        if work_outcome == "shipped":
            fields["gc.work_commit"] = commit
            fields["gc.work_branch"] = branch
            if work_dir is not None:
                fields["gc.work_dir"] = str(Path(work_dir).resolve(strict=False))
        self.records[bead_id].setdefault("metadata", {}).update(fields)
        self.records[bead_id]["status"] = "closed"
        self.events.append(("close", bead_id, reason))

    def defer(self, bead_id: str, reason: str) -> None:
        self.records[bead_id]["status"] = "deferred"
        self.events.append(("defer", bead_id, reason))

    def hold_delivery(self, bead_id: str) -> None:
        record = self.records[bead_id]
        record["status"] = "deferred"
        record["assignee"] = ""
        for key in ("gc.routed_to", "gc.session_name", "gc.work_dir"):
            record.setdefault("metadata", {}).pop(key, None)
        self.events.append(("hold_delivery", bead_id))

    def retry_delivery(self, bead_id: str, route: str) -> None:
        record = self.records[bead_id]
        record["status"] = "open"
        record["assignee"] = ""
        record.setdefault("metadata", {})["gc.routed_to"] = route
        for key in (
            "factory.delivery_hold",
            "factory.delivery_hold_code",
            "factory.delivery_hold_reason",
            "factory.refiner_context_id",
            "factory.refiner_model",
            "factory.refiner_model_policy",
            "factory.refiner_model_source",
            "gc.session_name",
            "gc.work_branch",
            "gc.work_dir",
        ):
            record.setdefault("metadata", {}).pop(key, None)
        self.events.append(("retry_delivery", bead_id))

    def dep_add(self, blocked: str, blocker: str, dep_type: str = "blocks") -> None:
        blocker_status = self.records.get(blocker, {}).get("status", "open")
        self.records[blocked].setdefault("dependencies", []).append({
            "id": blocker, "dependency_type": dep_type, "status": blocker_status,
        })
        self.events.append(("dep_add", blocked, blocker, dep_type))

    def acquire_merge_slot(self, holder: str, timeout: float) -> str:
        self.events.append(("merge_slot_acquire", holder, timeout))
        return "bd-merge-slot"

    def release_merge_slot(self, holder: str, slot_id: str) -> None:
        self.events.append(("merge_slot_release", holder, slot_id))


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

    def test_runtime_json_parser_accepts_native_beads_progress_before_pretty_json(self) -> None:
        raw = "✓ gate resolved\n\nChecked 1 gates\n{\n  \"checked\": 1,\n  \"resolved\": 1\n}\n"
        self.assertEqual(factory.parse_json_output(raw, "bd gate check"), {
            "checked": 1, "resolved": 1,
        })

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
            "execution_role": "implementation",
            "provider": provider,
            "worker_model_policy": "gpt-5.6-terra" if provider == "codex" else "opus-4.8",
            "validator_provider": validator,
            "validator_model_policy": "gpt-5.6-sol" if validator == "codex" else "opus-4.8",
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

    def test_atomic_text_writer_creates_new_file_and_preserves_existing_mode(self) -> None:
        target = self.root / "atomic" / "evidence.txt"

        factory.write_text_atomic(target, "first\n")
        self.assertEqual(target.read_text(encoding="utf-8"), "first\n")
        self.assertEqual(target.stat().st_mode & 0o777, 0o644)

        target.chmod(0o600)
        factory.write_text_atomic(target, "second\n")
        self.assertEqual(target.read_text(encoding="utf-8"), "second\n")
        self.assertEqual(target.stat().st_mode & 0o777, 0o600)

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
        program = next(node for node in plan["nodes"] if node["key"] == "program")["metadata"]
        refinery = next(node for node in plan["nodes"] if node["key"] == "refinery")["metadata"]
        self.assertEqual(program["factory.mayor_provider"], "codex")
        self.assertEqual(program["factory.plan_reviewer_provider"], "claude")
        self.assertEqual(refinery["factory.refiner_provider"], "codex")
        self.assertEqual(refinery["factory.integration_validator_provider"], "claude")

    def test_program_admission_binds_review_artifact_to_selected_provider(self) -> None:
        graph = self.graph([self.node("alpha", "src/alpha")])
        graph_path, review_path = self.write_graph_review(graph)
        review = json.loads(review_path.read_text(encoding="utf-8"))
        review["provider"] = "codex"
        review_path.write_text(json.dumps(review) + "\n", encoding="utf-8")

        with self.assertRaisesRegex(factory.FactoryError, "plan review provider must be 'claude'"):
            factory.admit_program(
                self.intent,
                graph_path,
                review_path,
                "mayor-context",
                "repo",
                reviewer_provider="claude",
            )

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

    def test_program_nodes_are_implementation_only_and_model_pinned(self) -> None:
        wrong_role = self.graph([self.node("alpha", "src/alpha")])
        wrong_role["nodes"][0]["execution_role"] = "refiner"
        with self.assertRaisesRegex(factory.FactoryError, "execution_role must be implementation"):
            factory.validate_graph(wrong_role)

        wrong_worker = self.graph([self.node("alpha", "src/alpha")])
        wrong_worker["nodes"][0]["worker_model_policy"] = "gpt-5.6-sol"
        with self.assertRaisesRegex(factory.FactoryError, "worker_model_policy must be gpt-5.6-terra"):
            factory.validate_graph(wrong_worker)

        wrong_validator = self.graph([self.node("alpha", "src/alpha")])
        wrong_validator["nodes"][0]["validator_model_policy"] = "gpt-5.6-sol"
        with self.assertRaisesRegex(factory.FactoryError, "validator_model_policy must be opus-4.8"):
            factory.validate_graph(wrong_validator)

    def test_role_inventory_maps_terra_sol_and_opus_without_a_fable_alias(self) -> None:
        self.assertEqual(factory.FACTORY_ROLE_MODELS[("implement", "codex")], ("gpt-5.6-terra", "gpt-5.6-terra"))
        for role in ("validate", "mayor", "rescope", "plan-review", "refiner"):
            self.assertEqual(factory.FACTORY_ROLE_MODELS[(role, "codex")], ("gpt-5.6-sol", "gpt-5.6-sol"))
        for role in ("implement", "validate", "mayor", "rescope", "plan-review", "refiner"):
            self.assertEqual(factory.FACTORY_ROLE_MODELS[(role, "claude")], ("opus-4.8", "claude-opus-4-8"))
        self.assertNotIn("fable", repr(factory.FACTORY_ROLE_MODELS).lower())
        self.assertEqual(factory.lifecycle_target("mayor", "claude", "repo", "factory"), "factory.mayor-claude")
        self.assertEqual(factory.lifecycle_target("refiner", "claude", "repo", "factory"), "repo/factory.refiner-claude")

    def test_lifecycle_prompts_keep_candidate_validator_policy_distinct_from_workers(self) -> None:
        for role in ("mayor", "mayor-claude", "plan-reviewer", "plan-reviewer-claude"):
            prompt = (factory.PACK_ROOT / "agents" / role / "prompt.template.md").read_text(encoding="utf-8")
            self.assertIn("Codex candidate Validator", prompt, role)
            self.assertIn("Sol", prompt, role)
            self.assertIn("Never infer Validator policy from Worker policy", prompt, role)
            self.assertIn(
                '--set-metadata gc.outcome=pass --set-metadata gc.work_outcome=no-op',
                prompt,
                role,
            )

    def test_refiners_continue_one_long_running_delivery_process(self) -> None:
        for role in ("refiner", "refiner-claude"):
            prompt = (factory.PACK_ROOT / "agents" / role / "prompt.template.md").read_text(encoding="utf-8")
            self.assertIn("600000 milliseconds", prompt, role)
            self.assertIn("Never launch a second delivery process", prompt, role)
            self.assertIn("same background task or session handle", prompt, role)

    def test_factory_commands_default_to_the_deployment_import_binding(self) -> None:
        command_action = next(action for action in factory.parser()._actions if action.dest == "command")
        for command in ("plan", "admit"):
            command_parser = command_action.choices[command]
            binding_action = next(action for action in command_parser._actions if action.dest == "binding")
            self.assertEqual(binding_action.default, "agentops", command)

    def test_lifecycle_roles_use_stable_role_work_directories(self) -> None:
        for role in (
            "mayor", "mayor-claude", "plan-reviewer", "plan-reviewer-claude",
            "refiner", "refiner-claude",
        ):
            config = factory.tomllib.loads(
                (factory.PACK_ROOT / "agents" / role / "agent.toml").read_text(encoding="utf-8")
            )
            self.assertEqual(config.get("work_dir"), f".gc/agents/{role}", role)

    def test_fresh_roles_use_provider_hooks_without_duplicate_nudges(self) -> None:
        role_roots = (
            ROOT / "packs" / "agentops-factory" / "agents",
            ROOT / "packs" / "agentops-executor" / "agents",
        )
        seen: set[str] = set()
        work_queries: set[str] = set()
        for role_root in role_roots:
            for role_dir in sorted(path for path in role_root.iterdir() if path.is_dir()):
                config = factory.tomllib.loads(
                    (role_dir / "agent.toml").read_text(encoding="utf-8")
                )
                provider = config["provider"]
                self.assertEqual(config.get("wake_mode"), "fresh", role_dir.name)
                self.assertEqual(config.get("install_agent_hooks"), [provider], role_dir.name)
                self.assertNotIn("nudge", config, role_dir.name)
                work_query = config.get("work_query", "")
                self.assertIn("GC_TRIGGER_WORK_BEAD_ID", work_query, role_dir.name)
                self.assertIn('bd show "$trigger" --json', work_query, role_dir.name)
                self.assertLess(work_query.index("bd show"), work_query.index("bd list"), role_dir.name)
                self.assertLess(work_query.index("bd show"), work_query.rindex("bd ready"), role_dir.name)
                subprocess.run(["/bin/sh", "-n", "-c", work_query], check=True)
                work_queries.add(work_query)
                prompt = (role_dir / "prompt.template.md").read_text(encoding="utf-8")
                self.assertIn("$GC_TRIGGER_WORK_BEAD_ID", prompt, role_dir.name)
                self.assertIn("action=drain", prompt, role_dir.name)
                self.assertIn("reason=no_work", prompt, role_dir.name)
                seen.add(role_dir.name)
        self.assertEqual(len(seen), 10)
        self.assertEqual(len(work_queries), 1)

    def test_admission_atomically_materializes_one_bead_graph(self) -> None:
        graph = self.graph([self.node("alpha", "src/alpha")])
        graph_path, review_path = self.write_graph_review(graph)
        beads = FakeBeads()
        beads.graph_ids = {"program": "bd-program", "experiment-alpha": "bd-alpha", "refinery": "bd-refinery"}
        args = types.SimpleNamespace(
            intent=str(self.intent), graph=str(graph_path), review=str(review_path),
            mayor_context="mayor-context", rig="repo", binding="factory", result=None,
        )

        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            contextlib.redirect_stdout(io.StringIO()),
        ):
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

    def test_mayor_inspect_exposes_complete_program_graph_contract(self) -> None:
        evidence = self.repo / ".gc" / "agentops-factory" / "inspect-mayor"
        evidence.mkdir(parents=True)
        request_path = evidence / "request.json"
        artifact_path = evidence / "program-graph.json"
        result_path = evidence / "response.json"
        request_path.write_text(json.dumps({
            "schema_version": "factory-role-request.v1",
            "request_id": "inspect-mayor",
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
            "artifact_path": str(artifact_path),
            "result_path": str(result_path),
        }) + "\n", encoding="utf-8")

        with contextlib.redirect_stdout(io.StringIO()) as stdout:
            self.assertEqual(factory.command_inspect_role(types.SimpleNamespace(request=str(request_path))), 0)
        inspected = json.loads(stdout.getvalue())
        contract = inspected["artifact_contract"]
        self.assertEqual(contract["artifact_kind"], "program-graph")
        self.assertIn("nodes", contract["required_top_level"])
        self.assertIn("provider", contract["node_required"])
        self.assertIn("validator_provider", contract["node_required"])
        self.assertIn("worker_model_policy", contract["node_required"])
        self.assertIn("validator_model_policy", contract["node_required"])
        self.assertEqual(contract["provider_values"], ["claude", "codex"])
        self.assertEqual(contract["execution_role"], "implementation")
        self.assertEqual(contract["model_policy"]["worker"]["codex"], "gpt-5.6-terra")
        self.assertEqual(contract["model_policy"]["validator"]["codex"], "gpt-5.6-sol")
        self.assertEqual(contract["model_policy"]["lifecycle"]["refiner"]["codex"], "gpt-5.6-sol")
        self.assertEqual(contract["model_policy"]["lifecycle"]["refiner"]["claude"], "opus-4.8")
        self.assertEqual(contract["top_level_fields"]["planning_notes"]["type"], "string")
        self.assertEqual(contract["top_level_fields"]["nodes"]["type"], "array")
        self.assertEqual(contract["node_fields"]["acceptance"]["type"], "array")
        self.assertEqual(contract["subject_fields"]["includes"]["type"], "array")
        self.assertIn("post-implementation", contract["first_check_semantics"])
        self.assertIn("must exit 0", contract["first_check_semantics"])
        self.assertIn(".gc/**", contract["first_check_semantics"])
        self.assertIn("pre-existing caller changes", contract["first_check_semantics"])

    def test_plan_review_inspect_exposes_the_same_model_policy_contract(self) -> None:
        request = {"role": "plan-review", "provider": "claude"}
        paths = {"request": self.root / "plan-review-request.json"}
        with (
            mock.patch.object(factory, "absolute_path", return_value=paths["request"]),
            mock.patch.object(factory, "validate_role_request", return_value=(request, paths)),
            contextlib.redirect_stdout(io.StringIO()) as stdout,
        ):
            self.assertEqual(factory.command_inspect_role(types.SimpleNamespace(request=str(paths["request"]))), 0)
        contract = json.loads(stdout.getvalue())["artifact_contract"]["model_policy"]
        self.assertEqual(contract["worker"]["codex"], "gpt-5.6-terra")
        self.assertEqual(contract["validator"]["codex"], "gpt-5.6-sol")
        self.assertEqual(contract["validator"]["claude"], "opus-4.8")
        self.assertEqual(contract["lifecycle"]["plan-review"]["codex"], "gpt-5.6-sol")

    def test_factory_owned_commit_has_a_deterministic_identity_without_ambient_config(self) -> None:
        repo = self.root / "identity-free-repo"
        repo.mkdir()
        isolated_git = {
            "HOME": str(self.root / "empty-home"),
            "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_CONFIG_NOSYSTEM": "1",
        }
        factory.run_process(["git", "init", "-b", "main"], cwd=repo, env=isolated_git)
        (repo / "candidate.txt").write_text("candidate\n", encoding="utf-8")
        factory.run_process(["git", "add", "candidate.txt"], cwd=repo, env=isolated_git)
        factory.run_process(
            factory.factory_git_command("commit", "-m", "factory candidate"),
            cwd=repo,
            env=isolated_git,
        )
        identity = factory.output(
            ["git", "log", "-1", "--format=%an%x00%ae%x00%cn%x00%ce"],
            cwd=repo,
            env=isolated_git,
        ).split("\x00")
        self.assertEqual(identity, [
            factory.FACTORY_GIT_NAME,
            factory.FACTORY_GIT_EMAIL,
            factory.FACTORY_GIT_NAME,
            factory.FACTORY_GIT_EMAIL,
        ])
        self.assertEqual(
            factory.factory_git_command("cherry-pick", "a" * 40)[-2:],
            ["cherry-pick", "a" * 40],
        )

    def test_factory_aborts_only_a_git_proven_interrupted_cherry_pick(self) -> None:
        git(self.repo, "checkout", "-b", "candidate")
        (self.repo / "README.md").write_text("candidate\n", encoding="utf-8")
        git(self.repo, "add", "README.md")
        git(self.repo, "commit", "-m", "candidate")
        candidate_sha = git(self.repo, "rev-parse", "HEAD")

        git(self.repo, "checkout", "main")
        (self.repo / "README.md").write_text("integration\n", encoding="utf-8")
        git(self.repo, "add", "README.md")
        git(self.repo, "commit", "-m", "integration")
        integration_head = git(self.repo, "rev-parse", "HEAD")
        conflicted = factory.run_process(
            factory.factory_git_command("cherry-pick", candidate_sha),
            cwd=self.repo,
            check=False,
        )
        self.assertNotEqual(conflicted.returncode, 0)
        self.assertTrue(factory.abort_interrupted_factory_cherry_pick(self.repo))
        self.assertEqual(git(self.repo, "rev-parse", "HEAD"), integration_head)
        self.assertEqual((self.repo / "README.md").read_text(encoding="utf-8"), "integration\n")
        self.assertEqual(git(self.repo, "status", "--porcelain", "--untracked-files=no"), "")
        self.assertFalse(factory.abort_interrupted_factory_cherry_pick(self.repo))

    def test_plan_reviewer_inspect_exposes_complete_review_contract(self) -> None:
        evidence = self.repo / ".gc" / "agentops-factory" / "inspect-review"
        evidence.mkdir(parents=True)
        graph_path = evidence / "program-graph.json"
        graph_path.write_text(json.dumps(self.graph([self.node("alpha", "src/alpha")])) + "\n", encoding="utf-8")
        request_path = evidence / "request.json"
        review_path = evidence / "plan-review.json"
        result_path = evidence / "response.json"
        request_path.write_text(json.dumps({
            "schema_version": "factory-role-request.v1",
            "request_id": "inspect-review",
            "role": "plan-review",
            "provider": "claude",
            "program_id": "factory-test",
            "workspace": str(self.repo),
            "intent_source": str(self.intent),
            "intent_digest": hashlib.sha256(self.intent.read_bytes()).hexdigest(),
            "repository": str(self.repo),
            "base_branch": "main",
            "base_sha": self.base_sha,
            "subject_path": str(graph_path),
            "subject_digest": hashlib.sha256(graph_path.read_bytes()).hexdigest(),
            "mayor_context_id": "mayor-context",
            "artifact_path": str(review_path),
            "result_path": str(result_path),
        }) + "\n", encoding="utf-8")

        with contextlib.redirect_stdout(io.StringIO()) as stdout:
            self.assertEqual(factory.command_inspect_role(types.SimpleNamespace(request=str(request_path))), 0)
        inspected = json.loads(stdout.getvalue())
        contract = inspected["artifact_contract"]
        self.assertIn("criteria", contract["required_top_level"])
        self.assertEqual(contract["criterion_required"], ["id", "result", "reason"])
        self.assertIn("severity", contract["finding_allowed"])
        self.assertEqual(contract["verdict_values"], ["FAIL", "NOT_PROVEN", "PASS"])
        self.assertEqual(contract["top_level_fields"]["criteria"]["type"], "array")
        self.assertEqual(contract["criterion_fields"]["reason"]["type"], "string")
        self.assertEqual(contract["finding_fields"]["node_ids"]["type"], "array")

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
            "model": "gpt-5.6-sol", "model_source": "launch_command",
        }

        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "runtime_session", return_value=session),
            mock.patch.object(factory, "output", side_effect=AssertionError("resume must not sling")),
        ):
            resumed = factory.dispatch_role(request_path, "repo", "factory", 1)

        self.assertEqual(resumed["work_bead"], "hq-planning")
        self.assertEqual(resumed["session_context_id"], "mayor-session")
        self.assertEqual(beads.records["hq-planning"]["metadata"]["factory.model"], "gpt-5.6-sol")
        self.assertEqual(beads.records["hq-planning"]["metadata"]["factory.model_source"], "launch_command")

    def test_runtime_session_falls_back_to_closed_city_session_bead(self) -> None:
        city_beads = FakeBeads({
            "mayor-session": {
                "id": "mayor-session",
                "issue_type": "session",
                "status": "closed",
                "metadata": {
                    "provider": "codex",
                    "template": "factory.mayor",
                    "session_name": "mayor-1",
                    "state": "drained",
                    "command": "codex --model gpt-5.6-sol",
                },
            },
        })

        with (
            mock.patch.object(factory, "city_path", return_value=str(self.root / "city")),
            mock.patch.object(factory, "gc_binary", return_value="/gc"),
            mock.patch.object(factory, "output", return_value='{"sessions": []}'),
            mock.patch.object(factory, "Beads", return_value=city_beads) as beads_class,
        ):
            session = factory.runtime_session("mayor-session")

        beads_class.assert_called_once_with(None)
        self.assertEqual(session, {
            "id": "mayor-session",
            "provider": "codex",
            "template": "factory.mayor",
            "session_name": "mayor-1",
            "state": "drained",
            "status": "closed",
            "model": "gpt-5.6-sol",
            "model_source": "launch_command",
        })

    def test_runtime_session_rejects_malformed_durable_session_identity(self) -> None:
        city_beads = FakeBeads({
            "mayor-session": {
                "id": "mayor-session",
                "issue_type": "session",
                "status": "closed",
                "metadata": {
                    "provider": "codex",
                    "template": "factory.mayor",
                    "state": "drained",
                },
            },
        })

        with (
            mock.patch.object(factory, "city_path", return_value=str(self.root / "city")),
            mock.patch.object(factory, "gc_binary", return_value="/gc"),
            mock.patch.object(factory, "output", return_value='{"sessions": []}'),
            mock.patch.object(factory, "Beads", return_value=city_beads),
        ):
            with self.assertRaisesRegex(factory.FactoryError, "session_name"):
                factory.runtime_session("mayor-session")

    def test_runtime_session_rejects_implicit_or_ambiguous_model(self) -> None:
        record = {
            "id": "mayor-session",
            "issue_type": "session",
            "status": "closed",
            "metadata": {
                "provider": "codex",
                "template": "factory.mayor",
                "session_name": "mayor-1",
                "state": "drained",
                "command": "codex",
            },
        }
        city_beads = FakeBeads({"mayor-session": record})
        with (
            mock.patch.object(factory, "city_path", return_value=str(self.root / "city")),
            mock.patch.object(factory, "gc_binary", return_value="/gc"),
            mock.patch.object(factory, "output", return_value='{"sessions": []}'),
            mock.patch.object(factory, "Beads", return_value=city_beads),
        ):
            with self.assertRaisesRegex(factory.FactoryError, "exactly one explicit model"):
                factory.runtime_session("mayor-session")

            record["metadata"]["command"] = "codex --model gpt-5.6-terra -m gpt-5.6-sol"
            with self.assertRaisesRegex(factory.FactoryError, "exactly one explicit model"):
                factory.runtime_session("mayor-session")

    def test_role_dispatch_reads_response_only_after_bead_closure(self) -> None:
        evidence = self.repo / ".gc" / "agentops-factory" / "crash-recovery-role"
        evidence.mkdir(parents=True)
        graph_path = evidence / "program-graph.json"
        graph_path.write_text(json.dumps(self.graph([self.node("alpha", "src/alpha")])) + "\n", encoding="utf-8")
        request_path = evidence / "mayor-request.json"
        result_path = evidence / "mayor-response.json"
        request = {
            "schema_version": "factory-role-request.v1",
            "request_id": "factory-test-mayor-crash-recovery",
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

        def response() -> dict:
            return {
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

        result_path.write_text(json.dumps(response()) + "\n", encoding="utf-8")
        record = {
            "id": "hq-planning", "status": "in_progress", "assignee": "mayor-1",
            "metadata": {
                "factory.kind": "planning",
                "factory.request_digest": hashlib.sha256(request_path.read_bytes()).hexdigest(),
            },
        }
        beads = FakeBeads({"hq-planning": record})
        original_show = beads.show
        show_count = 0

        def close_with_final_response(bead_id: str) -> dict:
            nonlocal show_count
            show_count += 1
            if show_count == 2:
                graph_path.write_text(
                    json.dumps(self.graph([self.node("alpha", "src/final")])) + "\n",
                    encoding="utf-8",
                )
                result_path.write_text(json.dumps(response()) + "\n", encoding="utf-8")
                record["status"] = "closed"
            return original_show(bead_id)

        beads.show = close_with_final_response  # type: ignore[method-assign]
        session = {
            "session_name": "mayor-1", "template": "factory.mayor", "provider": "codex",
            "model": "gpt-5.6-sol", "model_source": "launch_command",
        }
        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "runtime_session", return_value=session),
            mock.patch.object(factory, "output", side_effect=AssertionError("recovery must not resling")),
        ):
            recovered = factory.dispatch_role(request_path, "repo", "factory", 1)

        self.assertEqual(recovered["artifact_digest"], hashlib.sha256(graph_path.read_bytes()).hexdigest())

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

    def test_program_execute_routes_dependency_ready_refinery_to_sol_refiner(self) -> None:
        beads = FakeBeads({
            "bd-program": {
                "id": "bd-program", "status": "open",
                "metadata": {
                    "factory.kind": "program",
                    "factory.refinery_bead": "bd-refinery",
                    "factory.binding": "factory",
                    "factory.refiner_provider": "codex",
                },
            },
            "bd-refinery": {
                "id": "bd-refinery", "status": "open", "assignee": None,
                "metadata": {
                    "factory.kind": "refinery",
                    "factory.program_bead": "bd-program",
                    "factory.binding": "factory",
                    "factory.refiner_provider": "codex",
                    "factory.integration_validator_provider": "claude",
                },
            },
        }, {"bd-refinery"})
        args = types.SimpleNamespace(
            rig="repo", program_bead="bd-program", bead=[],
            worktree_root=str(self.root / "route-refinery-worktrees"),
            max_parallel=1, max_attempts=3, timeout=30, result=None,
        )
        sling_result = json.dumps({"success": True, "bead_id": "bd-refinery"})
        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "gc_binary", return_value="/gc"),
            mock.patch.object(factory, "city_path", return_value=str(self.root / "city")),
            mock.patch.object(factory, "output", return_value=sling_result) as output,
            contextlib.redirect_stdout(io.StringIO()) as stdout,
        ):
            self.assertEqual(factory.command_execute(args), 0)

        command = output.call_args.args[0]
        self.assertIn("repo/factory.refiner", command)
        self.assertIn("bd-refinery", command)
        result = json.loads(stdout.getvalue())
        self.assertTrue(result["refinery_ready"])
        self.assertEqual(result["refinery_route"], "repo/factory.refiner")
        self.assertEqual(result["executed"], 0)

    def test_ready_refinery_can_route_to_opus_refiner_with_sol_integration_validator(self) -> None:
        beads = FakeBeads({
            "bd-refinery": {
                "id": "bd-refinery", "status": "open", "assignee": None,
                "metadata": {
                    "factory.kind": "refinery", "factory.binding": "factory",
                    "factory.refiner_provider": "claude",
                    "factory.integration_validator_provider": "codex",
                },
            },
        }, {"bd-refinery"})
        sling_result = json.dumps({"success": True, "bead_id": "bd-refinery"})
        with (
            mock.patch.object(factory, "gc_binary", return_value="/gc"),
            mock.patch.object(factory, "city_path", return_value=str(self.root / "city")),
            mock.patch.object(factory, "output", return_value=sling_result) as output,
        ):
            target = factory.route_ready_refinery(
                beads, "repo", {"factory.refinery_bead": "bd-refinery"},
            )

        self.assertEqual(target, "repo/factory.refiner-claude")
        self.assertIn("repo/factory.refiner-claude", output.call_args.args[0])

    def test_resume_experiment_rejoins_the_automatic_refinery_route(self) -> None:
        beads, record, _verdict_args = self.leased_experiment("PASS")
        record["metadata"].update({
            "factory.branch": "candidate-pass",
            "factory.git_index": str(self.repo / ".git" / "index"),
            "factory.max_attempts": "3",
        })
        beads.records["bd-program"] = {
            "id": "bd-program", "status": "open",
            "metadata": {
                "factory.kind": "program",
                "factory.refinery_bead": "bd-refinery",
                "factory.binding": "factory",
            },
        }
        beads.ready.add("bd-refinery")
        args = types.SimpleNamespace(
            rig="repo", bead="bd-experiment", max_attempts=3, timeout=30, result=None,
        )

        def completed_experiment(*_args, **_kwargs):
            record["status"] = "closed"
            record["metadata"]["factory.status"] = "passed"
            return {"bead": "bd-experiment", "verdict": "PASS"}

        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "register_candidate_rig", return_value=("fx-alpha", "factory")),
            mock.patch.object(factory, "execute_experiment", side_effect=completed_experiment),
            mock.patch.object(factory, "route_ready_refinery", return_value="repo/factory.refiner") as route,
            contextlib.redirect_stdout(io.StringIO()) as stdout,
        ):
            self.assertEqual(factory.command_resume_experiment(args), 0)

        route.assert_called_once()
        result = json.loads(stdout.getvalue())
        self.assertTrue(result["refinery_ready"])
        self.assertEqual(result["refinery_route"], "repo/factory.refiner")

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

    def test_program_execute_closes_open_rejected_source_before_rescope_dispatch(self) -> None:
        for selected, verdict in (([], "FAIL"), (["rescope-open-source"], "NOT_PROVEN")):
            with self.subTest(selected=selected):
                beads, record, _verdict_args = self.leased_experiment(verdict)
                record["metadata"].update({
                    "factory.status": "rejected",
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
                beads.records["rescope-open-source"] = {
                    "id": "rescope-open-source", "status": "open",
                    "metadata": {
                        "factory.kind": "rescope", "factory.status": "mayor_required",
                        "factory.program_bead": "bd-program",
                        "factory.rejected_bead": "bd-experiment",
                    },
                }
                args = types.SimpleNamespace(
                    rig="repo", program_bead="bd-program", bead=selected,
                    worktree_root=str(self.root / "unused-open-source-worktrees"),
                    max_parallel=1, max_attempts=3, timeout=30, result=None,
                )

                def recover_source(*_args, **_kwargs):
                    self.assertEqual(record["status"], "open")
                    record["status"] = "closed"
                    beads.records["rescope-open-source"]["metadata"]["factory.status"] = "successor_admitted"
                    beads.records["rescope-open-source"]["status"] = "closed"
                    return {"bead": "bd-experiment", "node_id": "alpha", "verdict": "FAIL"}

                with (
                    mock.patch.object(factory, "Beads", return_value=beads),
                    mock.patch.object(factory, "lease_experiment") as lease,
                    mock.patch.object(factory, "register_candidate_rig", return_value=("fx-alpha", "factory")),
                    mock.patch.object(factory, "execute_experiment", side_effect=recover_source),
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
                "factory.provider": "codex",
                "factory.validator_provider": "claude",
                "factory.implementation_scope_status": "PASS",
                "factory.first_check_exit_code": "0",
                "factory.author_context_id": "author-context",
                "factory.author_model": "gpt-5.6-terra",
                "factory.author_model_policy": "gpt-5.6-terra",
                "factory.author_model_source": "launch_command",
                "factory.validator_context_id": "validator-context",
                "factory.validator_model": "claude-opus-4-8",
                "factory.validator_model_policy": "opus-4.8",
                "factory.validator_model_source": "launch_command",
                "factory.rig": "repo",
                "factory.binding": "factory",
                "factory.adapter_path": str(MODULE_PATH),
                "factory.lease_token": "lease-token",
                "factory.fence_epoch": "1",
                "factory.worktree": str(worktree),
                "factory.branch": "candidate-test",
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
        self.assertEqual(meta["gc.work_outcome"], "shipped")
        self.assertEqual(meta["gc.work_commit"], args.candidate_sha)
        self.assertEqual(meta["gc.work_branch"], "candidate-test")
        self.assertEqual(record["status"], "closed")
        close_count = sum(event[:2] == ("close", "bd-experiment") for event in beads.events)
        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_record_verdict(args), 0)
        self.assertEqual(sum(event[:2] == ("close", "bd-experiment") for event in beads.events), close_count)

    def test_parallel_verdict_reducer_does_not_redirect_process_global_stdout(self) -> None:
        beads, _record, args = self.leased_experiment("PASS")
        stdout = io.StringIO()

        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(stdout):
            active_stream = factory.sys.stdout
            self.assertEqual(factory.command_record_verdict(args, emit=False), 0)
            self.assertIs(factory.sys.stdout, active_stream)

        self.assertEqual(stdout.getvalue(), "")
        self.assertNotIn("redirect_stdout", inspect.getsource(factory.execute_experiment))

    def test_pass_accepts_beads_numeric_zero_for_durable_first_check(self) -> None:
        beads, record, args = self.leased_experiment("PASS")
        # bd show decodes metadata scalars that look numeric as JSON numbers.
        record["metadata"]["factory.first_check_exit_code"] = 0

        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_record_verdict(args), 0)

        self.assertEqual(record["metadata"]["factory.status"], "passed")

    def test_pass_is_rejected_when_the_durable_first_check_failed(self) -> None:
        beads, record, args = self.leased_experiment("PASS")
        record["metadata"]["factory.first_check_exit_code"] = "1"

        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            self.assertRaisesRegex(factory.FactoryError, "first_check_exit_code=0"),
        ):
            factory.command_record_verdict(args)

        self.assertEqual(record["status"], "open")
        self.assertNotIn("factory.admission_path", record["metadata"])

    def test_pass_is_rejected_without_durable_validator_runtime_attestation(self) -> None:
        beads, record, args = self.leased_experiment("PASS")
        record["metadata"].pop("factory.validator_model")

        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            self.assertRaisesRegex(factory.FactoryError, "durable validate attestation"),
        ):
            factory.command_record_verdict(args)

        self.assertEqual(record["status"], "open")
        self.assertNotIn("factory.admission_path", record["metadata"])

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
        self.assertEqual(record["metadata"]["gc.work_outcome"], "blocked")

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
                "factory.mayor_provider": "codex",
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
        self.assertEqual(beads.records[rescope_bead]["metadata"]["gc.work_outcome"], "no-op")
        self.assertEqual(hq.records["hq-rescope"]["metadata"]["factory.successor_bead"], successor_bead)
        self.assertEqual(result["mayor_context_id"], "fresh-mayor-context")
        self.assertEqual(replayed["successor_bead"], successor_bead)
        self.assertEqual(sum(event[0] == "graph_create" for event in beads.events), 1)

    def test_direct_rescope_and_successor_reject_an_open_rejected_source(self) -> None:
        beads, record, verdict_args = self.leased_experiment("FAIL")
        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_record_verdict(verdict_args), 0)
        rescope_bead = record["metadata"]["factory.rescope_bead"]
        record["status"] = "open"

        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "dispatch_role") as dispatch,
            self.assertRaisesRegex(factory.FactoryError, "terminal closed rejected experiment"),
        ):
            factory.rescope_rejection("repo", rescope_bead, 10)

        dispatch.assert_not_called()
        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            self.assertRaisesRegex(factory.FactoryError, "terminal closed rejected experiment"),
        ):
            factory.command_successor(types.SimpleNamespace(
                rig="repo", rejected_bead="bd-experiment",
                rescope_bead=rescope_bead, proposal=str(self.intent),
            ))

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
        # Local experiment scopes exclude their sibling to prevent concurrent
        # writers from crossing ownership boundaries. Those local exclusions
        # must not erase the sibling from the assembled integration subject.
        alpha["metadata"]["factory.subject"] = json.dumps({
            "includes": ["alpha.txt"], "excludes": ["beta.txt"],
        })
        beta["metadata"]["factory.subject"] = json.dumps({
            "includes": ["beta.txt"], "excludes": ["alpha.txt"],
        })
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
                "factory.refiner_provider": "codex",
                "factory.integration_validator_provider": "claude",
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
            max_candidates=5, remote="origin", merge_slot_timeout=30,
        )
        refiner_runtime = {
            "id": "refiner-context",
            "session_name": "refiner-session",
            "template": "repo/factory.refiner",
            "provider": "codex",
            "model": "gpt-5.6-sol",
            "model_source": "launch_command",
        }
        runtime = mock.patch.object(factory, "runtime_session", return_value=refiner_runtime)
        environment = mock.patch.dict(factory.os.environ, {
            "GC_SESSION_ID": "refiner-context",
            "GC_SESSION_NAME": "refiner-session",
        }, clear=False)
        with mock.patch.object(factory, "Beads", return_value=beads), mock.patch.object(
            factory, "register_integration_rig", return_value=("fx-refinery", "factory"),
        ), runtime, environment, contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_refinery_assemble(args), 0)

        meta = refinery["metadata"]
        integration = Path(meta["factory.integration_worktree"])
        self.assertEqual((integration / "alpha.txt").read_text(encoding="utf-8"), "alpha\n")
        self.assertEqual((integration / "beta.txt").read_text(encoding="utf-8"), "beta\n")
        self.assertEqual(meta["factory.candidate_beads"], ["bd-alpha", "bd-beta"])
        self.assertEqual(meta["factory.refiner_context_id"], "refiner-context")
        self.assertEqual(meta["factory.refiner_model"], "gpt-5.6-sol")
        self.assertEqual(meta["factory.refiner_model_policy"], "gpt-5.6-sol")
        self.assertEqual(meta["factory.status"], "validation_required")
        self.assertEqual(meta["factory.merge_slot_id"], "bd-merge-slot")
        self.assertEqual(meta["factory.merge_slot_holder"], "factory-refinery:bd-refinery")
        self.assertEqual(meta["factory.merge_slot_released"], "true")
        self.assertIn(("merge_slot_acquire", "factory-refinery:bd-refinery", 30), beads.events)
        self.assertIn(("merge_slot_release", "factory-refinery:bd-refinery", "bd-merge-slot"), beads.events)
        self.assertTrue(Path(meta["factory.integration_subject_manifest"]).is_file())
        self.assertEqual(meta["factory.integration_subject"], {
            "includes": ["alpha.txt", "beta.txt"], "excludes": [],
        })
        subject_manifest = json.loads(
            Path(meta["factory.integration_subject_manifest"]).read_text(encoding="utf-8")
        )
        self.assertEqual(
            [entry["path"] for entry in subject_manifest["entries"]],
            ["alpha.txt", "beta.txt"],
        )
        scope = json.loads(Path(meta["factory.integration_scope_receipt"]).read_text(encoding="utf-8"))
        self.assertEqual(scope["status"], "PASS")
        integration_sha = meta["factory.integration_sha"]
        meta["factory.status"] = "assembling"
        with mock.patch.object(factory, "Beads", return_value=beads), mock.patch.object(
            factory, "register_integration_rig", return_value=("fx-refinery", "factory"),
        ), runtime, environment, contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_refinery_assemble(args), 0)
        self.assertEqual(meta["factory.integration_sha"], integration_sha)
        self.assertEqual(meta["factory.status"], "validation_required")
        first_worktree = meta["factory.integration_worktree"]
        meta["factory.status"] = "reassembly_required"
        with mock.patch.object(factory, "Beads", return_value=beads), mock.patch.object(
            factory, "register_integration_rig", return_value=("fx-refinery-2", "factory"),
        ), runtime, environment, contextlib.redirect_stdout(io.StringIO()):
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
                "factory.refiner_provider": "codex",
                "factory.integration_validator_provider": "claude",
                "gc.routed_to": "repo/factory.refiner",
                "gc.session_name": "refiner-session",
            },
        }
        beads = FakeBeads({"bd-refinery": refinery})
        args = types.SimpleNamespace(
            rig="repo", refinery_bead="bd-refinery",
            worktree_root=str(self.root / "integration-worktrees"),
            max_candidates=5, remote="origin", merge_slot_timeout=30,
        )
        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.dict(factory.os.environ, {"GC_SESSION_NAME": "refiner-session"}, clear=False),
        ):
            with self.assertRaisesRegex(factory.FactoryError, "not owned by its routed Refiner session"):
                factory.command_refinery_assemble(args)

        refinery["assignee"] = "refiner-session"
        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.dict(factory.os.environ, {"GC_SESSION_NAME": "different-session"}, clear=False),
            self.assertRaisesRegex(factory.FactoryError, "not owned by its routed Refiner session"),
        ):
            factory.command_refinery_assemble(args)

        refinery["dependencies"][0]["status"] = "open"
        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.dict(factory.os.environ, {"GC_SESSION_NAME": "refiner-session"}, clear=False),
        ):
            with self.assertRaisesRegex(factory.FactoryError, "unresolved dependencies"):
                factory.command_refinery_assemble(args)

    def test_refinery_claim_accepts_canonical_gc_assignee_without_redundant_session_metadata(self) -> None:
        refinery = {
            "id": "bd-refinery",
            "status": "in_progress",
            "assignee": "refiner-session",
            "dependencies": [],
            "metadata": {
                "factory.kind": "refinery",
                "factory.status": "ready",
                "factory.binding": "factory",
                "factory.program_bead": "bd-program",
                "factory.refiner_provider": "codex",
                "factory.integration_validator_provider": "claude",
                "gc.routed_to": "repo/factory.refiner",
            },
        }
        beads = FakeBeads({"bd-refinery": refinery}, {"bd-refinery"})
        args = types.SimpleNamespace(
            rig="repo", refinery_bead="bd-refinery",
            worktree_root=str(self.root / "integration-worktrees"),
            max_candidates=5, remote="origin", merge_slot_timeout=30,
        )
        runtime = {
            "id": "refiner-context",
            "session_name": "refiner-session",
            "template": "repo/factory.refiner",
            "provider": "codex",
            "model": "gpt-5.6-sol",
            "model_source": "launch_command",
        }
        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "runtime_session", return_value=runtime),
            mock.patch.dict(factory.os.environ, {
                "GC_SESSION_ID": "refiner-context",
                "GC_SESSION_NAME": "refiner-session",
            }, clear=False),
            self.assertRaisesRegex(factory.FactoryError, "no PASSed experiment beads"),
        ):
            factory.command_refinery_assemble(args)
        self.assertEqual(refinery["metadata"]["factory.refiner_context_id"], "refiner-context")
        self.assertEqual(refinery["metadata"]["factory.refiner_model"], "gpt-5.6-sol")

    def test_refinery_rejects_claimed_runtime_with_worker_model(self) -> None:
        refinery = {
            "id": "bd-refinery",
            "status": "in_progress",
            "assignee": "refiner-session",
            "dependencies": [],
            "metadata": {
                "factory.kind": "refinery",
                "factory.status": "ready",
                "factory.binding": "factory",
                "factory.refiner_provider": "codex",
                "factory.integration_validator_provider": "claude",
                "gc.routed_to": "repo/factory.refiner",
                "gc.session_name": "refiner-session",
            },
        }
        beads = FakeBeads({"bd-refinery": refinery}, {"bd-refinery"})
        args = types.SimpleNamespace(
            rig="repo", refinery_bead="bd-refinery",
            worktree_root=str(self.root / "integration-worktrees"),
            max_candidates=5, remote="origin", merge_slot_timeout=30,
        )
        worker_runtime = {
            "id": "refiner-context",
            "session_name": "refiner-session",
            "template": "repo/factory.refiner",
            "provider": "codex",
            "model": "gpt-5.6-terra",
            "model_source": "launch_command",
        }
        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "runtime_session", return_value=worker_runtime),
            mock.patch.dict(factory.os.environ, {
                "GC_SESSION_ID": "refiner-context",
                "GC_SESSION_NAME": "refiner-session",
            }, clear=False),
            self.assertRaisesRegex(factory.FactoryError, "Refiner runtime model"),
        ):
            factory.command_refinery_assemble(args)
        self.assertNotIn("factory.refiner_model", refinery["metadata"])

    def test_dynamic_rig_policy_exposes_only_bead_selected_routes(self) -> None:
        city = self.root / "city"
        (city / ".gc").mkdir(parents=True)
        (city / "city.toml").write_text("[workspace]\nname = \"factory-test\"\n", encoding="utf-8")
        rig = "fx-alpha"

        def agent(name: str, suspended: bool = False, work_dir: str | None = None) -> dict:
            value = {
                "qualified_name": f"{rig}/{name}",
                "dir": rig,
                "suspended": suspended,
            }
            if work_dir is not None:
                value["work_dir"] = work_dir
            return value

        allowed = {"factory.implementer-claude", "factory.validator"}
        disallowed = {
            "factory.implementer", "factory.validator-claude",
            "factory.mayor", "factory.mayor-claude", "factory.plan-reviewer",
            "factory.refiner", "factory.refiner-claude", "codex", "agentops-factory.validator",
        }
        rig_root = self.root / "candidate" / "alpha"
        # gc.pack_workspace carries the candidate directory name, so Gas City
        # resolves a routed session as <work_dir>/<pack_workspace>. The native
        # agent base must be the candidate's parent, not the candidate itself.
        session_work_dir = str(rig_root.parent.resolve(strict=False))
        before = [agent(name) for name in sorted(allowed | disallowed)]
        after = [
            agent(name, name in disallowed, session_work_dir if name in allowed else None)
            for name in sorted(allowed | disallowed)
        ]
        runtime = {"GC_CITY_PATH": str(city), "GC_BIN": "/usr/bin/true"}
        with (
            mock.patch.dict(factory.os.environ, runtime, clear=False),
            mock.patch.object(factory, "configured_agents", side_effect=[before, after]),
            mock.patch.object(factory, "output", return_value="{}") as output_mock,
        ):
            changed = factory.enforce_rig_agent_policy(
                rig, "factory",
                {"implementer-claude", "validator"},
                rig_root,
            )

        self.assertTrue(changed)
        config = factory.tomllib.loads((city / "city.toml").read_text(encoding="utf-8"))
        patches = config["patches"]["agent"]
        suspended = {
            (item["dir"], item["name"], item["suspended"])
            for item in patches if item.get("suspended") is True
        }
        workspaces = {
            (item["dir"], item["name"], item["work_dir"])
            for item in patches if "work_dir" in item
        }
        self.assertEqual(suspended, {(rig, name, True) for name in disallowed})
        self.assertEqual(workspaces, {(rig, name, session_work_dir) for name in allowed})
        output_mock.assert_called_once()

    def test_dynamic_rig_registration_hides_origin_then_restores_exact_urls(self) -> None:
        first = "https://example.invalid/factory.git"
        second = str(self.root / "forge-mirror.git")
        git(self.repo, "remote", "add", "origin", first)
        git(self.repo, "config", "--add", "remote.origin.url", second)
        original_output = factory.output
        observed: dict[str, object] = {}

        def output_while_registering(argv, **kwargs):
            if "rig" in argv and "add" in argv:
                current = subprocess.run(
                    ["git", "config", "--get-all", "remote.origin.url"],
                    cwd=self.repo, check=False, capture_output=True, text=True,
                )
                observed["urls"] = current.stdout.splitlines()
                observed["env"] = kwargs.get("env")
                return '{"ok":true}'
            return original_output(argv, **kwargs)

        command = [
            "/gc", "--city", str(self.root / "city"), "rig", "add", str(self.repo),
            "--name", "fx-test", "--prefix", "fx-test", "--default-branch", "main", "--json",
        ]
        def enforce_while_origin_is_hidden(rig_name):
            observed["enforced_rig"] = rig_name
            current = subprocess.run(
                ["git", "config", "--get-all", "remote.origin.url"],
                cwd=self.repo, check=False, capture_output=True, text=True,
            )
            observed["enforced_urls"] = current.stdout.splitlines()

        with (
            mock.patch.object(factory, "output", side_effect=output_while_registering),
            mock.patch.object(factory, "enforce_dynamic_rig_local_only", side_effect=enforce_while_origin_is_hidden),
        ):
            result = factory.add_gc_rig_without_forge_origin(self.repo, command)

        self.assertEqual(result, {"ok": True})
        self.assertEqual(observed["urls"], [])
        self.assertEqual(observed["env"], {
            "BD_DOLT_SYNC_CLI_REMOTES": "false",
            "BEADS_DOLT_SYNC_CLI_REMOTES": "false",
        })
        self.assertEqual(observed["enforced_rig"], "fx-test")
        self.assertEqual(observed["enforced_urls"], [])
        restored = subprocess.run(
            ["git", "config", "--get-all", "remote.origin.url"],
            cwd=self.repo, check=True, capture_output=True, text=True,
        ).stdout.splitlines()
        self.assertEqual(restored, [first, second])

    def test_dynamic_rig_registration_recovers_a_parked_origin(self) -> None:
        url = "https://example.invalid/factory.git"
        git(self.repo, "remote", "add", "agentops-factory-origin", url)
        original_output = factory.output

        def output_while_registering(argv, **kwargs):
            if "rig" in argv and "add" in argv:
                return '{"ok":true}'
            return original_output(argv, **kwargs)

        command = [
            "/gc", "--city", str(self.root / "city"), "rig", "add", str(self.repo),
            "--name", "fx-test", "--prefix", "fx-test", "--default-branch", "main", "--json",
        ]

        with (
            mock.patch.object(factory, "output", side_effect=output_while_registering),
            mock.patch.object(factory, "enforce_dynamic_rig_local_only"),
        ):
            result = factory.add_gc_rig_without_forge_origin(self.repo, command)

        self.assertEqual(result, {"ok": True})
        self.assertEqual(git(self.repo, "remote", "get-url", "origin"), url)
        parked = subprocess.run(
            ["git", "remote", "get-url", "agentops-factory-origin"],
            cwd=self.repo, check=False, capture_output=True, text=True,
        )
        self.assertNotEqual(parked.returncode, 0)

    def test_dynamic_rig_local_only_enforcement_removes_synthesized_dolt_remotes(self) -> None:
        remote_lists = iter([
            json.dumps([
                {"name": "origin", "url": "https://doltremoteapi.dolthub.com/origin"},
                {"name": "backup", "url": "file:///tmp/backup"},
            ]),
            "[]",
        ])
        calls: list[list[str]] = []

        def dynamic_output(argv, **_kwargs):
            calls.append(argv)
            if argv[-2:] == ["list", "--json"] and "remote" not in argv:
                return "[]"
            if "remote" in argv and argv[-2:] == ["list", "--json"]:
                return next(remote_lists)
            if "remote" in argv and "remove" in argv:
                return '{"ok":true}'
            raise AssertionError(argv)

        with (
            mock.patch.object(factory, "gc_binary", return_value="/gc"),
            mock.patch.object(factory, "city_path", return_value="/city"),
            mock.patch.object(factory, "output", side_effect=dynamic_output),
        ):
            factory.enforce_dynamic_rig_local_only("fx-test")

        removals = [call for call in calls if "remove" in call]
        self.assertEqual([call[-2] for call in removals], ["origin", "backup"])

    def test_dynamic_rig_registration_discards_only_canonical_bd_init_commit(self) -> None:
        gitignore = self.repo / ".gitignore"
        gitignore.write_text("*.log\n", encoding="utf-8")
        git(self.repo, "add", ".gitignore")
        git(self.repo, "commit", "-m", "track ignore rules")
        base = git(self.repo, "rev-parse", "HEAD")
        gitignore.write_text(
            "*.log\n\n# Beads / Dolt files (added by bd init)\n.dolt/\n*.db\n.beads-credential-key\n",
            encoding="utf-8",
        )
        git(self.repo, "add", ".gitignore")
        git(self.repo, "commit", "-m", "bd init: initialize beads issue tracking")
        candidate = self.repo / "candidate.txt"
        candidate.write_text("worker bytes stay untouched\n", encoding="utf-8")

        changed = factory.discard_transient_rig_init_commits(self.repo, base)

        self.assertEqual(changed, [".gitignore"])
        self.assertEqual(git(self.repo, "rev-parse", "HEAD"), base)
        self.assertEqual(gitignore.read_bytes(), b"*.log\n")
        self.assertEqual(candidate.read_text(encoding="utf-8"), "worker bytes stay untouched\n")
        self.assertEqual(git(self.repo, "status", "--porcelain", "--untracked-files=no"), "")

    def test_dynamic_rig_registration_discards_canonical_staged_beads_init(self) -> None:
        gitignore = self.repo / ".gitignore"
        gitignore.write_text("*.log\n", encoding="utf-8")
        git(self.repo, "add", ".gitignore")
        git(self.repo, "commit", "-m", "track ignore rules")
        base = git(self.repo, "rev-parse", "HEAD")
        beads = self.repo / ".beads"
        beads.mkdir()
        (beads / "config.yaml").write_text("issue_prefix: fx-alpha\n", encoding="utf-8")
        (beads / "metadata.json").write_text('{"database":"dolt"}\n', encoding="utf-8")
        gitignore.write_text(
            "*.log\n\n# Beads / Dolt files (added by bd init)\n"
            ".dolt/\n*.db\n.beads-credential-key\n.beads/proxieddb/\n",
            encoding="utf-8",
        )
        git(self.repo, "add", ".beads/config.yaml", ".beads/metadata.json", ".gitignore")
        # Gas City adds its runtime stanza after Beads stages the canonical
        # initialization set; the staged and working-tree bytes intentionally
        # differ at this point.
        gitignore.write_text(
            gitignore.read_text(encoding="utf-8")
            + "\n# Gas City\n.beads/*\n!.beads/identity.toml\n",
            encoding="utf-8",
        )

        changed = factory.discard_transient_rig_init_commits(self.repo, base)

        self.assertEqual(changed, [
            ".beads/config.yaml", ".beads/metadata.json", ".gitignore",
        ])
        self.assertEqual(gitignore.read_bytes(), b"*.log\n")
        self.assertEqual(git(self.repo, "diff", "--cached", "--name-only"), "")
        self.assertEqual(git(self.repo, "status", "--porcelain"), "")
        self.assertIn(
            "agentops-factory-exclude",
            git(self.repo, "config", "--worktree", "--get", "core.excludesFile"),
        )
        self.assertIn(
            ".beads/config.yaml",
            git(self.repo, "check-ignore", ".beads/config.yaml"),
        )

    def test_dynamic_rig_removes_gc_discovered_duplicate_factory_binding(self) -> None:
        city = self.root / "city"
        city.mkdir()
        rig = "fx-alpha"
        (city / "city.toml").write_text(
            "\n".join([
                "[workspace]",
                'name = "factory-test"',
                "",
                "[[rigs]]",
                f'name = "{rig}"',
                "",
                "[rigs.imports.agentops-factory]",
                f"source = {json.dumps(str(factory.PACK_ROOT))}",
                "",
            ]),
            encoding="utf-8",
        )
        runtime = {"GC_CITY_PATH": str(city), "GC_BIN": "/usr/bin/true"}
        with (
            mock.patch.dict(factory.os.environ, runtime, clear=False),
            mock.patch.object(factory, "output", return_value="") as output_mock,
        ):
            removed = factory.remove_duplicate_factory_imports(rig, "factory")

        self.assertEqual(removed, ["agentops-factory"])
        output_mock.assert_called_once_with([
            "/usr/bin/true", "--city", str(city), "--rig", rig,
            "import", "remove", "agentops-factory",
        ])

    def test_candidate_worktree_hides_tracked_parent_beads_before_fresh_rig_init(self) -> None:
        beads_dir = self.repo / ".beads"
        beads_dir.mkdir()
        (beads_dir / "config.yaml").write_text(
            "issue_prefix: repo\nissue-prefix: repo\n", encoding="utf-8",
        )
        (beads_dir / "metadata.json").write_text(
            '{"database":"repo"}\n', encoding="utf-8",
        )
        (beads_dir / "interactions.jsonl").write_text(
            '{"kind":"parent"}\n', encoding="utf-8",
        )
        git(
            self.repo, "add", ".beads/config.yaml", ".beads/metadata.json",
            ".beads/interactions.jsonl",
        )
        git(self.repo, "commit", "-m", "track parent beads identity")
        candidate = self.root / "candidate-rig"
        git(self.repo, "worktree", "add", "-b", "candidate-rig", str(candidate), "HEAD")

        adopt = factory.prepare_worktree_beads_identity(candidate, "fxcandidate")

        self.assertFalse(adopt)
        self.assertFalse((candidate / ".beads" / "config.yaml").exists())
        self.assertFalse((candidate / ".beads" / "metadata.json").exists())
        index = git(candidate, "ls-files", "-v", ".beads")
        self.assertTrue(all(line.startswith("S ") for line in index.splitlines()))
        (candidate / ".beads" / "interactions.jsonl").write_text(
            '{"kind":"candidate-runtime"}\n', encoding="utf-8",
        )
        self.assertEqual(git(candidate, "status", "--porcelain", "--untracked-files=no"), "")

    def test_candidate_worktree_adopts_matching_isolated_beads_on_retry(self) -> None:
        beads_dir = self.repo / ".beads"
        beads_dir.mkdir()
        (beads_dir / "config.yaml").write_text(
            "issue_prefix: repo\nissue-prefix: repo\n", encoding="utf-8",
        )
        (beads_dir / "metadata.json").write_text(
            '{"database":"repo"}\n', encoding="utf-8",
        )
        git(self.repo, "add", ".beads/config.yaml", ".beads/metadata.json")
        git(self.repo, "commit", "-m", "track parent beads identity")
        candidate = self.root / "candidate-rig-retry"
        git(self.repo, "worktree", "add", "-b", "candidate-rig-retry", str(candidate), "HEAD")
        self.assertFalse(factory.prepare_worktree_beads_identity(candidate, "fxcandidate"))
        (candidate / ".beads" / "config.yaml").write_text(
            "issue_prefix: fxcandidate\nissue-prefix: fxcandidate\n", encoding="utf-8",
        )
        (candidate / ".beads" / "metadata.json").write_text(
            '{"database":"fxcandidate"}\n', encoding="utf-8",
        )

        self.assertTrue(factory.prepare_worktree_beads_identity(candidate, "fxcandidate"))

    def test_candidate_rig_policy_is_derived_from_the_experiment_bead_provider_pair(self) -> None:
        record = {
            "id": "bd-alpha", "status": "open",
            "metadata": {
                "factory.program_id": "factory-test", "factory.node_id": "alpha",
                "factory.attempt": "1", "factory.binding": "factory",
                "factory.provider": "claude", "factory.validator_provider": "codex",
            },
        }
        lease = {
            "worktree": str(self.repo), "branch": "candidate-alpha",
            "candidate_base_sha": self.base_sha,
        }
        selected: list[set[str]] = []
        restore = mock.Mock()

        with (
            mock.patch.object(factory, "city_config_lock", return_value=contextlib.nullcontext()),
            mock.patch.object(factory, "configured_rigs", return_value=[{
                "name": "fx-factory-test-alpha-1", "path": str(self.repo),
            }]),
            mock.patch.object(
                factory, "enforce_rig_agent_policy",
                side_effect=lambda _rig, _binding, roles, _root: selected.append(set(roles)) or False,
            ),
            mock.patch.object(factory, "remove_duplicate_factory_imports", return_value=[]),
            mock.patch.object(factory, "enforce_dynamic_rig_local_only"),
            mock.patch.object(factory, "reload_city_config") as reload_city,
            mock.patch.object(factory, "restore_rig_scaffolding", restore),
        ):
            rig, binding = factory.register_candidate_rig(lease, record)

        self.assertEqual(rig, "fx-factory-test-alpha-1")
        self.assertEqual(binding, "factory")
        self.assertEqual(selected, [{"implementer-claude", "validator"}])
        restore.assert_called_once_with(self.repo)
        reload_city.assert_called_once_with(
            "fx-factory-test-alpha-1", "factory",
            {"implementer-claude", "validator"},
        )

        record["metadata"]["factory.candidate_rig"] = rig
        restore.reset_mock()
        with (
            mock.patch.object(factory, "city_config_lock", return_value=contextlib.nullcontext()),
            mock.patch.object(factory, "configured_rigs", return_value=[{
                "name": rig, "path": str(self.repo),
            }]),
            mock.patch.object(factory, "enforce_rig_agent_policy", return_value=False),
            mock.patch.object(factory, "remove_duplicate_factory_imports", return_value=[]),
            mock.patch.object(factory, "enforce_dynamic_rig_local_only"),
            mock.patch.object(factory, "reload_city_config") as reload_city,
            mock.patch.object(factory, "restore_rig_scaffolding", restore),
        ):
            recovered_rig, recovered_binding = factory.register_candidate_rig(lease, record)
        self.assertEqual((recovered_rig, recovered_binding), (rig, binding))
        restore.assert_not_called()
        reload_city.assert_not_called()

    def test_dynamic_rig_reload_uses_native_async_acceptance_and_live_rig_status(self) -> None:
        accepted = subprocess.CompletedProcess(
            [], 0, '{"ok":true,"outcome":"accepted","async":true}\n', "",
        )
        stale = [
            {
                "qualified_name": "fx-alpha/factory.implementer-claude",
                "suspended": False,
            },
            {
                "qualified_name": "fx-alpha/factory.validator-claude",
                "suspended": False,
            },
        ]
        converged = [
            {
                "qualified_name": "fx-alpha/factory.implementer-claude",
                "suspended": False,
            },
            {
                "qualified_name": "fx-alpha/factory.validator",
                "suspended": False,
            },
            {
                "qualified_name": "fx-alpha/factory.validator-claude",
                "suspended": True,
            },
        ]

        with (
            mock.patch.dict(factory.os.environ, {
                "GC_CITY_PATH": str(self.root / "city"),
                "GC_BIN": "/usr/bin/true",
            }, clear=False),
            mock.patch.object(factory, "run_process", return_value=accepted) as run,
            mock.patch.object(
                factory, "controller_rig_agents", side_effect=[stale, converged],
            ) as status,
            mock.patch.object(factory.time, "sleep") as sleep,
        ):
            factory.reload_city_config(
                "fx-alpha", "factory", {"implementer-claude", "validator"}, timeout=30,
            )

        self.assertEqual(
            run.call_args.args[0],
            [
                "/usr/bin/true", "--city", str(self.root / "city"),
                "reload", "--async", "--json",
            ],
        )
        self.assertEqual(status.call_args_list, [mock.call("fx-alpha"), mock.call("fx-alpha")])
        sleep.assert_called_once_with(2.0)

    def test_dynamic_rig_reload_retries_native_backpressure_until_policy_is_live(self) -> None:
        busy = subprocess.CompletedProcess(
            [], 1, '{"ok":false,"outcome":"busy"}\n',
            "Reload request could not be accepted because another reload is already in progress.\n",
        )
        accepted = subprocess.CompletedProcess(
            [], 0, '{"ok":true,"outcome":"accepted","async":true}\n', "",
        )
        stale = [{
            "qualified_name": "fx-alpha/factory.validator-claude",
            "suspended": False,
        }]
        converged = [{
            "qualified_name": "fx-alpha/factory.validator",
            "suspended": False,
        }]

        with (
            mock.patch.dict(factory.os.environ, {
                "GC_CITY_PATH": str(self.root / "city"),
                "GC_BIN": "/usr/bin/true",
            }, clear=False),
            mock.patch.object(factory, "run_process", side_effect=[busy, accepted]) as run,
            mock.patch.object(
                factory, "controller_rig_agents",
                side_effect=[stale, stale, stale, stale, stale, stale, converged],
            ),
            mock.patch.object(factory.time, "sleep") as sleep,
        ):
            factory.reload_city_config("fx-alpha", "factory", {"validator"}, timeout=30)

        self.assertEqual(run.call_count, 2)
        self.assertEqual(sleep.call_count, 6)

    def test_beads_calls_use_the_bounded_control_plane_timeout_by_default(self) -> None:
        completed = subprocess.CompletedProcess(
            [], 0, json.dumps([{"id": "repo-alpha"}]) + "\n", "",
        )
        with (
            mock.patch.dict(factory.os.environ, {
                "GC_CITY_PATH": str(self.root / "city"),
                "GC_BIN": "/usr/bin/true",
            }, clear=False),
            mock.patch.object(factory, "run_process", return_value=completed) as run,
        ):
            self.assertEqual(factory.Beads("repo").show("repo-alpha")["id"], "repo-alpha")

        self.assertEqual(run.call_args.kwargs["timeout"], factory.CONTROL_PLANE_TIMEOUT_SECONDS)
        self.assertEqual(factory.CONTROL_PLANE_TIMEOUT_SECONDS, 120)

    def test_refinery_merge_slot_uses_native_beads_queue_and_releases_before_validation(self) -> None:
        beads = factory.Beads("repo")

        def result(code: int, value: dict) -> subprocess.CompletedProcess[str]:
            return subprocess.CompletedProcess([], code, json.dumps(value) + "\n", "")

        responses = [
            result(0, {"id": "repo-merge-slot", "status": "open"}),
            result(1, {
                "id": "repo-merge-slot", "acquired": False,
                "holder": "other-refinery", "waiting": True, "position": 1,
            }),
            result(0, {
                "id": "repo-merge-slot", "acquired": True,
                "holder": "factory-refinery:bd-refinery",
            }),
            result(0, {"id": "repo-merge-slot", "released": True}),
        ]
        with (
            mock.patch.object(beads, "run", side_effect=responses) as run,
            mock.patch.object(factory.time, "sleep") as sleep,
            factory.refinery_merge_slot(beads, "bd-refinery", 30) as slot,
        ):
            self.assertEqual(slot, {
                "id": "repo-merge-slot",
                "holder": "factory-refinery:bd-refinery",
            })

        self.assertEqual(run.call_count, 4)
        sleep.assert_called_once_with(2.0)
        self.assertEqual(run.call_args_list[-1].args, (
            "merge-slot", "release", "--holder",
            "factory-refinery:bd-refinery", "--json",
        ))

    def test_proven_integration_rig_skips_redundant_controller_reload(self) -> None:
        rig = "fx-refinery-bd-refinery-2"
        with (
            mock.patch.object(factory, "city_config_lock", return_value=contextlib.nullcontext()),
            mock.patch.object(factory, "configured_rigs", return_value=[{
                "name": rig, "path": str(self.repo),
            }]),
            mock.patch.object(factory, "enforce_dynamic_rig_local_only"),
            mock.patch.object(factory, "remove_duplicate_factory_imports", return_value=[]),
            mock.patch.object(factory, "enforce_rig_agent_policy", return_value=False),
            mock.patch.object(factory, "reload_city_config") as reload_city,
            mock.patch.object(factory, "discard_transient_rig_init_commits"),
            mock.patch.object(factory, "restore_rig_scaffolding"),
        ):
            actual = factory.register_integration_rig(
                self.repo, "bd-refinery", "gc/integration/factory-test/1/2",
                "factory", recorded_rig=rig,
            )

        self.assertEqual(actual, (rig, "factory"))
        reload_city.assert_not_called()

    def test_integration_rig_prefix_is_unique_per_refinery(self) -> None:
        first = factory.integration_rig_prefix("ag-8f2", "1")
        second = factory.integration_rig_prefix("ag-48s", "1")

        self.assertNotEqual(first, second)
        self.assertLessEqual(len(first), 16)
        self.assertLessEqual(len(second), 16)

    def test_candidate_rig_prefix_is_unique_across_programs_with_the_same_node(self) -> None:
        first_rig = factory.safe_identifier("fx", "program-one", "claude-canary", "1")
        second_rig = factory.safe_identifier("fx", "program-two", "claude-canary", "1")

        first = factory.candidate_rig_prefix(first_rig)
        second = factory.candidate_rig_prefix(second_rig)

        self.assertNotEqual(first, second)
        self.assertLessEqual(len(first), 16)
        self.assertLessEqual(len(second), 16)

    def test_scope_not_proven_is_freshly_validated_and_sent_to_mayor(self) -> None:
        worktree = self.root / "scope-not-proven"
        git(self.repo, "worktree", "add", "-b", "scope-not-proven", str(worktree), self.base_sha)
        description = "Implement the bounded Alpha behavior.\n"
        intent_digest = hashlib.sha256(description.encode()).hexdigest()
        evidence = worktree / ".gc" / "agentops" / "bd-alpha-implement"
        evidence.mkdir(parents=True)
        baseline = evidence / "runtime-baseline-manifest.json"
        subject = evidence / "runtime-subject-manifest.json"
        scope = evidence / "runtime-scope-receipt.json"
        subject_digest = "7" * 64
        baseline.write_text("{}\n", encoding="utf-8")
        subject.write_text(
            json.dumps({"canonical_manifest_digest": subject_digest}) + "\n",
            encoding="utf-8",
        )
        scope.write_text(json.dumps({"status": "NOT_PROVEN"}) + "\n", encoding="utf-8")
        record = {
            "id": "bd-alpha", "status": "open", "description": description,
            "metadata": {
                "factory.kind": "experiment", "factory.status": "leased",
                "factory.program_id": "factory-test", "factory.program_bead": "bd-program",
                "factory.refinery_bead": "bd-refinery", "factory.node_id": "alpha",
                "factory.attempt": "1", "factory.max_attempts": "3",
                "factory.spec": json.dumps(self.node("alpha", "alpha.txt")),
                "factory.intent_digest": intent_digest,
                "factory.subject": json.dumps({"includes": ["alpha.txt"], "excludes": []}),
                "factory.write_scope": json.dumps(["alpha.txt"]),
                "factory.generated_scope": json.dumps([]),
                "factory.first_check": "/usr/bin/false",
                "factory.provider": "codex", "factory.validator_provider": "claude",
                "factory.rig": "repo", "factory.binding": "factory",
                "factory.adapter_path": str(MODULE_PATH),
                "factory.lease_token": "lease-token", "factory.fence_epoch": "1",
                "factory.worktree": str(worktree),
                "factory.candidate_base_sha": self.base_sha,
                "factory.predecessor_beads": [], "factory.predecessor_shas": [],
            },
        }
        beads = FakeBeads({
            "bd-alpha": record,
            "bd-refinery": {
                "id": "bd-refinery", "status": "open",
                "metadata": {"factory.kind": "refinery", "factory.program_bead": "bd-program"},
            },
        })
        lease = {
            "worktree": str(worktree), "branch": "scope-not-proven",
            "lease_token": "lease-token", "fence_epoch": 1,
        }
        roles: list[str] = []

        def run_packet(packet_path: Path, _rig: str, _binding: str, _timeout: float) -> dict:
            packet = json.loads(packet_path.read_text(encoding="utf-8"))
            roles.append(packet["role"])
            if packet["role"] == "implement":
                return {
                    "transport": {"session_context_id": "worker-context"},
                    "runtime_evidence": {
                        "scope_status": "NOT_PROVEN", "actual_changed_paths": [],
                        "baseline_manifest": str(baseline),
                        "subject_manifest": str(subject), "scope_receipt": str(scope),
                        "session": {
                            "id": "worker-context", "provider": "codex",
                            "model": "gpt-5.6-terra", "model_policy": "gpt-5.6-terra",
                            "model_source": "launch_command",
                        },
                    },
                }
            verdict = Path(packet["evidence_dir"]) / "verdict.v2.json"
            verdict.parent.mkdir(parents=True, exist_ok=True)
            verdict.write_text(json.dumps({
                "schema_version": "verdict.v2", "verdict": "NOT_PROVEN",
                "acceptance_digest": intent_digest,
                "subject_manifest_digest": subject_digest,
                "author_context_id": "worker-context",
                "validator_context_id": "validator-context",
                "freshness_attestation": {
                    "source": "runtime", "attester_identity": "validator-context",
                },
            }) + "\n", encoding="utf-8")
            return {
                "transport": {"session_context_id": "validator-context"},
                "runtime_evidence": {"session": {
                    "id": "validator-context", "provider": "claude",
                    "model": "claude-opus-4-8", "model_policy": "opus-4.8",
                    "model_source": "launch_command",
                }},
                "agent_response": {"artifacts": [{"path": str(verdict)}]},
            }

        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(factory, "run_executor_packet", side_effect=run_packet),
            mock.patch.object(
                factory, "rescope_rejection",
                return_value={"successor_bead": "bd-alpha-v2"},
            ) as rescope,
        ):
            result = factory.execute_experiment(
                "repo", "bd-alpha", lease, "fx-alpha", "factory", 30, 3,
            )

        self.assertEqual(roles, ["implement", "validate"])
        self.assertEqual(result["verdict"], "NOT_PROVEN")
        self.assertEqual(result["successor_bead"], "bd-alpha-v2")
        self.assertEqual(record["status"], "closed")
        self.assertEqual(record["metadata"]["factory.runtime_scope_status"], "NOT_PROVEN")
        self.assertEqual(record["metadata"]["factory.implementation_scope_status"], "FAIL")
        rescope.assert_called_once()

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
                "factory.refiner_provider": "codex",
                "factory.integration_validator_provider": "claude",
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
                "runtime_evidence": {"session": {
                    "id": "validator-context", "provider": "claude",
                    "model": "claude-opus-4-8", "model_policy": "opus-4.8",
                    "model_source": "launch_command",
                }},
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
        self.assertEqual(refinery["metadata"]["factory.integration_validator_model"], "claude-opus-4-8")
        self.assertEqual(refinery["metadata"]["factory.integration_validator_model_policy"], "opus-4.8")

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
                "factory.refiner_provider": "codex",
                "factory.integration_validator_provider": "claude",
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
                "runtime_evidence": {"session": {
                    "id": "epoch-2-validator", "provider": "claude",
                    "model": "claude-opus-4-8", "model_policy": "opus-4.8",
                    "model_source": "launch_command",
                }},
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
            mock.patch.object(factory, "add_gc_rig_without_forge_origin", return_value={"ok": True}) as add_rig,
            mock.patch.object(factory, "remove_duplicate_factory_imports", return_value=[]),
            mock.patch.object(factory, "enforce_rig_agent_policy", return_value=False),
            mock.patch.object(factory, "reload_city_config") as reload_city,
            mock.patch.object(factory, "restore_rig_scaffolding"),
            mock.patch.object(factory, "run_executor_packet", side_effect=run_packet),
            mock.patch.object(factory, "command_refinery_validate", return_value=0),
            contextlib.redirect_stdout(io.StringIO()),
        ):
            self.assertEqual(factory.command_refinery_run_validation(args), 0)

        self.assertIn("fx-refinery-bd-refinery-2", add_rig.call_args.args[1])
        add_argv = add_rig.call_args.args[1]
        self.assertEqual(
            add_argv[add_argv.index("--prefix") + 1],
            factory.integration_rig_prefix("bd-refinery", "2"),
        )
        reload_city.assert_called_once_with(
            "fx-refinery-bd-refinery-2", "factory",
            {"validator", "validator-claude"},
        )

    def test_refinery_deliver_resumes_at_the_recorded_bead_phase(self) -> None:
        refinery = {
            "id": "bd-refinery",
            "status": "in_progress",
            "metadata": {
                "factory.kind": "refinery",
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
            max_candidates=5, remote="origin", merge_slot_timeout=30,
            provider="claude", timeout=30,
            title=None, draft=True, merge_method="squash",
        )
        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(
                factory, "attest_refiner_claim", return_value=refinery["metadata"],
            ),
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

    def test_refinery_deliver_requires_claimed_refiner(self) -> None:
        refinery = {
            "id": "bd-refinery",
            "status": "open",
            "metadata": {"factory.kind": "refinery", "factory.status": "ready"},
        }
        args = types.SimpleNamespace(
            rig="repo", refinery_bead="bd-refinery",
            worktree_root=str(self.root / "integration-worktrees"),
            max_candidates=5, remote="origin", merge_slot_timeout=30,
            provider="claude", timeout=30,
            title=None, draft=True, merge_method="squash",
        )
        with (
            mock.patch.object(factory, "Beads", return_value=FakeBeads({"bd-refinery": refinery})),
            self.assertRaisesRegex(factory.FactoryError, "requires a Refiner-claimed Refinery bead"),
        ):
            factory.command_refinery_deliver(args)

    def test_qualification_mode_closes_canary_without_publish_or_merge(self) -> None:
        refinery = {
            "id": "bd-refinery", "status": "in_progress",
            "metadata": {
                "factory.kind": "refinery",
                "factory.delivery_mode": "qualify",
                "factory.status": "validated",
                "factory.integration_verdict": "PASS",
                "factory.integration_verdict_digest": "1" * 64,
                "factory.integration_subject_digest": "2" * 64,
                "factory.integration_validator_context_id": "validator-context",
                "factory.program_id": "factory-test",
                "factory.program_bead": "bd-program",
                "factory.fence_epoch": "1",
                "factory.fence_token": "fence-token",
                "factory.base_branch": "main",
                "factory.delivery_base_sha": self.base_sha,
                "factory.candidate_shas": [],
                "factory.integration_branch": "gc/integration/factory-test/1/1",
                "factory.integration_sha": self.base_sha,
                "factory.integration_worktree": str(self.repo),
            },
        }
        program = {"id": "bd-program", "status": "open", "metadata": {"factory.kind": "program"}}
        beads = FakeBeads({"bd-refinery": refinery, "bd-program": program})
        args = types.SimpleNamespace(rig="repo", refinery_bead="bd-refinery")

        with mock.patch.object(factory, "Beads", return_value=beads), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(factory.command_refinery_qualify(args), 0)
            self.assertEqual(factory.command_refinery_qualify(args), 0)

        self.assertEqual(refinery["status"], "closed")
        self.assertEqual(program["status"], "closed")
        self.assertEqual(refinery["metadata"]["factory.status"], "qualified")
        self.assertEqual(refinery["metadata"]["gc.work_outcome"], "shipped")
        self.assertEqual(program["metadata"]["gc.work_outcome"], "shipped")
        self.assertEqual(program["metadata"]["factory.status"], "qualified")
        self.assertEqual(program["metadata"]["factory.qualified_sha"], self.base_sha)
        self.assertEqual(
            program["metadata"]["factory.qualification_record"],
            refinery["metadata"]["factory.qualification_record"],
        )
        record_path = Path(refinery["metadata"]["factory.qualification_record"])
        record = json.loads(record_path.read_text(encoding="utf-8"))
        self.assertEqual(record["status"], "qualified")
        self.assertIsNone(record["pr"])
        self.assertIsNone(record["landed_sha"])
        self.assertEqual(sum(event[0] == "close" for event in beads.events), 2)

    def test_refinery_deliver_defers_terminal_adapter_failure_instead_of_hot_looping(self) -> None:
        refinery = {
            "id": "bd-refinery",
            "status": "in_progress",
            "assignee": "refiner-session",
            "metadata": {
                "factory.status": "validated",
                "factory.program_bead": "bd-program",
                "gc.routed_to": "repo/factory.refiner-claude",
                "gc.session_name": "refiner-session",
                "gc.work_dir": "/tmp/refiner-session",
            },
        }
        beads = FakeBeads({"bd-refinery": refinery})
        args = types.SimpleNamespace(
            rig="repo", refinery_bead="bd-refinery",
            worktree_root=str(self.root / "integration-worktrees"),
            max_candidates=5, remote="origin", merge_slot_timeout=30,
            provider="claude", timeout=30,
            title=None, draft=True, merge_method="squash",
        )
        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            mock.patch.object(
                factory, "attest_refiner_claim", return_value=refinery["metadata"],
            ),
            mock.patch.object(
                factory, "command_refinery_publish",
                side_effect=factory.FactoryError("integration_moved", "integration branch moved after fencing"),
            ),
            self.assertRaisesRegex(factory.FactoryError, "integration branch moved"),
        ):
            factory.command_refinery_deliver(args)

        self.assertEqual(refinery["status"], "deferred")
        self.assertEqual(refinery["assignee"], "")
        self.assertEqual(refinery["metadata"]["factory.status"], "reassembly_required")
        self.assertEqual(refinery["metadata"]["factory.delivery_hold_code"], "integration_moved")
        self.assertNotIn("gc.routed_to", refinery["metadata"])
        self.assertNotIn("gc.session_name", refinery["metadata"])
        self.assertNotIn("gc.work_dir", refinery["metadata"])
        self.assertEqual(sum(event[0] == "hold_delivery" for event in beads.events), 1)

    def test_refinery_retry_returns_a_delivery_hold_to_its_canonical_route(self) -> None:
        refinery = {
            "id": "bd-refinery",
            "status": "deferred",
            "assignee": "stale-refiner-session",
            "metadata": {
                "factory.kind": "refinery",
                "factory.status": "ready",
                "factory.binding": "factory",
                "factory.delivery_hold": True,
                "factory.delivery_hold_code": "refinery_claim_invalid",
                "factory.delivery_hold_reason": "old adapter contract",
                "factory.refiner_context_id": "old-refiner-context",
                "factory.refiner_model": "gpt-5.6-sol",
                "factory.refiner_model_policy": "gpt-5.6-sol",
                "factory.refiner_model_source": "launch_command",
                "factory.refiner_provider": "codex",
                "factory.integration_validator_provider": "claude",
                "gc.routed_to": "repo/factory.refiner",
                "gc.session_name": "stale-refiner-session",
                "gc.work_branch": "main",
                "gc.work_dir": "/tmp/stale-refiner",
            },
        }
        beads = FakeBeads({"bd-refinery": refinery})
        args = types.SimpleNamespace(rig="repo", refinery_bead="bd-refinery")

        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            contextlib.redirect_stdout(io.StringIO()),
        ):
            self.assertEqual(factory.command_refinery_retry(args), 0)

        self.assertEqual(refinery["status"], "open")
        self.assertEqual(refinery["assignee"], "")
        self.assertEqual(refinery["metadata"]["gc.routed_to"], "repo/factory.refiner")
        self.assertNotIn("factory.delivery_hold", refinery["metadata"])
        self.assertNotIn("factory.refiner_context_id", refinery["metadata"])
        self.assertNotIn("gc.session_name", refinery["metadata"])
        self.assertNotIn("gc.work_dir", refinery["metadata"])
        self.assertIn(("retry_delivery", "bd-refinery"), beads.events)

    def test_refinery_retry_rejects_semantic_integration_rejection(self) -> None:
        refinery = {
            "id": "bd-refinery",
            "status": "deferred",
            "metadata": {
                "factory.kind": "refinery",
                "factory.status": "integration_rejected",
                "factory.binding": "factory",
                "factory.delivery_hold": "true",
                "gc.routed_to": "repo/factory.refiner",
            },
        }
        beads = FakeBeads({"bd-refinery": refinery})
        args = types.SimpleNamespace(rig="repo", refinery_bead="bd-refinery")

        with (
            mock.patch.object(factory, "Beads", return_value=beads),
            self.assertRaisesRegex(factory.FactoryError, "not retryable"),
        ):
            factory.command_refinery_retry(args)
        self.assertEqual(refinery["status"], "deferred")

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
                "factory.base_branch": "main", "factory.integration_worktree": str(self.repo),
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
        self.assertEqual(refinery["metadata"]["gc.work_outcome"], "shipped")
        self.assertEqual(program["metadata"]["gc.work_outcome"], "shipped")

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
                "factory.integration_worktree": str(self.repo),
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
