from __future__ import annotations

import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from contextlib import redirect_stdout
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
FEEDER = ROOT / "packs/agentops-factory/assets/scripts/factory_feeder.py"


def canonical(value: object) -> bytes:
    return json.dumps(value, ensure_ascii=True, separators=(",", ":"), sort_keys=True).encode() + b"\n"


class FactoryWorkflowTests(unittest.TestCase):
    def feeder(self):
        spec = importlib.util.spec_from_file_location("gc33_factory_feeder", FEEDER)
        assert spec and spec.loader
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def write(self, path: Path, value: object) -> str:
        raw = canonical(value)
        path.write_bytes(raw)
        return hashlib.sha256(raw).hexdigest()

    def delivery(self, root: Path, repository: Path | None = None, tools: dict[str, str] | None = None,
                 base_ref: str = "main") -> tuple[dict[str, object], dict[str, object] | None]:
        evidence = root / ".gc/agentops/factory/evidence/delivery"
        evidence.mkdir(parents=True, exist_ok=True)
        path = root / "native-delivery-context.v1.json"
        if repository is None or tools is None:
            return ({"native_context_path": str(path), "native_context_digest": "d" * 64,
                     "evidence_root": str(evidence), "mode": "auto", "deadline_seconds": 86400}, None)
        executables = {}
        for name, tool in (("gc", "gc"), ("bd", "bd"), ("git", "git"), ("gh", "gh"), ("bash", "bash"), ("agentops-gc-delivery", "delivery")):
            executable = Path(tools[tool])
            executables[name] = {"path": str(executable), "digest": hashlib.sha256(executable.read_bytes()).hexdigest()}
        repository = repository.resolve()
        native = {"schema_version": "gc-delivery-native-context.v1", "rig_id": "agentops",
                  "repository": "boshu2/agentops", "repository_dir": str(repository),
                  "worktree_root": str(root / "delivery-worktrees"), "beads_dir": str(repository / ".beads"),
                  "remote": "origin", "base_ref": base_ref, "successor_capability_digest": "a" * 64,
                  "toolchain_lock_digest": "b" * 64, "toolchain_receipt_path": str(root / "toolchain.json"),
                  "toolchain_receipt_digest": "c" * 64, "beads_representation": "B-successor-delivery-bead",
                  "executables": executables, "check_only_gate_argv": [[tools["bash"], "scripts/check-gc-executor.sh"]]}
        native_digest = self.write(path, native)
        return ({"native_context_path": str(path), "native_context_digest": native_digest,
                 "evidence_root": str(evidence), "mode": "auto", "deadline_seconds": 86400}, native)

    def graph(self) -> dict[str, object]:
        no_fallback = {"allowed": False, "used": False, "reason": None}
        return {"schema_version": "program-graph.v2", "program_id": "p", "intent_digest": "a" * 64, "repository_dir": "/repo", "base_ref": "main", "base_oid": "b" * 40, "workspace_root": "/worktrees", "packet_root": "/packets", "max_parallel": 1, "delivery_group_id": "delivery", "prefix_safety": "safe", "role_policy": {"mayor": {"model": "fable", "reasoning": "adaptive", "provider": "claude", "fallback": no_fallback}, "planner": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": no_fallback}, "worker_pool": {"default": {"model": "terra", "reasoning": "high", "provider": "codex", "fallback": no_fallback}, "overflow": {"model": "opus", "reasoning": "medium", "provider": "claude", "fallback": no_fallback}, "fallback": no_fallback}, "validator": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": no_fallback}, "refiner": {"model": "fable", "reasoning": "adaptive", "provider": "claude", "fallback": no_fallback, "ambiguity_only": True}, "luna": {"model": "luna", "reasoning": "high", "provider": "codex", "fallback": no_fallback, "support_only": True}}, "nodes": [{"id": "semantic", "title": "One bounded change", "intent": "Make the specified change.", "acceptance": ["Focused check passes."], "non_goals": [], "subject": {"includes": ["deploy/gc"], "excludes": [".git"]}, "first_check": "bash scripts/check-gc-executor.sh", "bead_class": "product", "depends_on": [], "write_scope": ["deploy/gc"], "generated_companions": [], "intent_digest": "a" * 64, "role": "implementation", "model": "terra", "reasoning": "high", "provider": "codex", "fallback": no_fallback}]}

    def test_formula_roles_are_checked_once_and_have_no_drain_or_fallback(self) -> None:
        build = (ROOT / "packs/agentops-factory/formulas/agentops-build.toml").read_text()
        experiment = (ROOT / "packs/agentops-factory/formulas/agentops-experiment.toml").read_text()
        for text in (build, experiment):
            self.assertNotIn("drain", text)
            self.assertNotIn("retry", text)
            self.assertIn("max_attempts = 1", text)
        self.assertIn('gc.run_target" = "agentops.mayor"', build)
        self.assertIn('gc.run_target" = "rig/agentops.plan-reviewer"', build)
        self.assertIn('gc.run_target" = "{{implement_target}}"', experiment)
        self.assertIn('"work_dir" = "{{work_dir}}"', experiment)
        self.assertIn('gc.run_target" = "rig/agentops.validator"', experiment)
        self.assertNotIn("refiner", build + experiment)
        self.assertNotIn("luna", build + experiment)

    def test_experiment_formula_binds_pinned_graphv2_attachment_to_source_convoy(self) -> None:
        experiment = (ROOT / "packs/agentops-factory/formulas/agentops-experiment.toml").read_text()
        # GC v1.3.5 stamps a stable graph.v2 attach key only when the rendered
        # recipe references convoy_id. Every semantic bead therefore gets one
        # replay-adoptable workflow instead of an unbound duplicate.
        self.assertIn("{{convoy_id}}", experiment)

    def test_start_binds_mayor_request_before_releasing_build_admission(self) -> None:
        feeder = self.feeder()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            repository = root / "repo"; repository.mkdir()
            intent = root / "intent"; intent.write_text("one exact intent\n", encoding="utf-8")
            tools = {}
            for name in ("bd", "gc", "git", "gh", "bash", "delivery", "role-adapter", "packet-adapter", "factory-check"):
                path = root / name; path.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8"); path.chmod(0o700); tools[name] = str(path.resolve())
            state = {"cook": 0, "close": 0, "workspace": "", "mayor_request": "", "base": "a" * 40}

            def rows():
                values = []
                for reference in sorted(feeder.BUILD_STEP_REFS):
                    metadata = {"gc.root_bead_id": "workflow-1", "gc.step_ref": reference}
                    description = ""
                    if reference == "agentops-build.mayor.iteration.1":
                        metadata.update({"gc.run_target": "agentops.mayor", "work_dir": state["workspace"]})
                        description = "request_path=" + state["mayor_request"]
                    if reference == "agentops-build.plan.iteration.1":
                        metadata.update({"gc.run_target": "rig/agentops.plan-reviewer", "work_dir": state["workspace"]})
                    status = "closed" if reference == "agentops-build.admission" and state["close"] else "open"
                    values.append({"id": "workflow-1" if reference == "agentops-build" else reference.replace(".", "-"), "status": status, "assignee": None, "description": description, "metadata": metadata})
                return values

            original = feeder.run_checked
            def fake(argv, _cwd):
                if argv[0] == tools["git"]:
                    return (str(state["base"]) + "\n").encode()
                if argv[0] == tools["gc"]:
                    state["cook"] += 1
                    state["workspace"] = next(value.removeprefix("work_dir=") for value in argv if value.startswith("work_dir="))
                    state["mayor_request"] = next(value.removeprefix("mayor_request=") for value in argv if value.startswith("mayor_request="))
                    return canonical({"schema_version": "1", "ok": True, "formula": "agentops-build", "mode": "attach", "attach_bead_id": "source-1", "root_id": "workflow-1", "workflow_root_id": "workflow-1", "created": 9})
                if argv[0] == tools["bd"] and argv[1] == "list":
                    return canonical(rows())
                if argv[0] == tools["bd"] and argv[1] == "close":
                    self.assertTrue(Path(state["mayor_request"]).is_file(), "request must exist before admission close")
                    state["close"] += 1
                    return canonical({"id": argv[2], "status": "closed"})
                raise AssertionError(argv)
            delivery, _ = self.delivery(repository, repository, tools)
            feeder.run_checked = fake
            try:
                args = SimpleNamespace(root=str(repository), source_bead="source-1", intent=str(intent), repository=str(repository), base_ref="main", max_parallel=2, bd_bin=tools["bd"], gc_bin=tools["gc"], git_bin=tools["git"], role_adapter=tools["role-adapter"], packet_adapter=tools["packet-adapter"], factory_check=tools["factory-check"], delivery_native_context=delivery["native_context_path"], delivery_native_context_digest=delivery["native_context_digest"], delivery_root=delivery["evidence_root"], delivery_mode="auto", delivery_deadline_seconds=86400, created_at="2026-07-22T00:00:00Z")
                with redirect_stdout(io.StringIO()):
                    self.assertEqual(feeder.start(args), 0)
                state["base"] = "b" * 40
                with redirect_stdout(io.StringIO()):
                    self.assertEqual(feeder.start(args), 0)
            finally:
                feeder.run_checked = original
            self.assertEqual((state["cook"], state["close"]), (1, 1))
            context = json.loads((Path(state["workspace"]) / "build-context.v1.json").read_bytes())
            self.assertEqual(context["base_oid"], "a" * 40)
            request = json.loads(Path(state["mayor_request"]).read_text(encoding="utf-8"))
            self.assertEqual(request["semantic_bead_id"], "agentops-build-mayor-iteration-1")
            self.assertEqual(request["role"], "mayor")

    def test_mayor_check_creates_and_binds_future_plan_request_before_pass(self) -> None:
        feeder = self.feeder()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            workspace = root / "workspace"; workspace.mkdir()
            intent = workspace / "intent-source"; intent.write_text("intent\n", encoding="utf-8")
            intent_digest = hashlib.sha256(intent.read_bytes()).hexdigest()
            graph_path = workspace / "program-graph.v2.json"
            mayor_request_path, mayor_result_path = workspace / "mayor-request.v2.json", workspace / "mayor-response.v2.json"
            plan_request_path, plan_result_path, plan_artifact_path = workspace / "plan-request.v2.json", workspace / "plan-response.v2.json", workspace / "plan-review.v1.json"
            context_path = workspace / "build-context.v1.json"
            files = {}
            for name in ("bd", "gc", "git", "role-adapter", "packet-adapter", "factory-check"):
                path = root / name; path.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8"); path.chmod(0o700); files[name] = str(path)
            steps = {reference: reference.replace(".", "-") for reference in feeder.BUILD_STEP_REFS}
            delivery, _ = self.delivery(root)
            context = {"schema_version": "factory-build-context.v1", "program_id": "program", "source_bead_id": "source", "intent_path": str(intent), "intent_digest": intent_digest, "repository_dir": "/repo", "base_ref": "main", "base_oid": "a" * 40, "root": str(root), "workspace": str(workspace), "candidate_workspace_root": "/workers", "packet_root": "/packets", "max_parallel": 2, "graph_path": str(graph_path), "mayor_request_path": str(mayor_request_path), "mayor_result_path": str(mayor_result_path), "plan_request_path": str(plan_request_path), "plan_result_path": str(plan_result_path), "plan_artifact_path": str(plan_artifact_path), "workflow_root_id": "root", "workflow_steps": steps, "role_adapter": files["role-adapter"], "packet_adapter": files["packet-adapter"], "factory_check": files["factory-check"], "bd_bin": files["bd"], "gc_bin": files["gc"], "git_bin": files["git"], "delivery": delivery, "created_at": "2026-07-22T00:00:00Z"}
            self.write(context_path, context)
            graph = self.graph(); graph.update({"program_id": "program", "intent_digest": intent_digest, "repository_dir": "/repo", "base_ref": "main", "base_oid": "a" * 40, "workspace_root": "/workers", "packet_root": "/packets", "delivery_group_id": "program", "role_policy": feeder.expected_role_policy()}); graph["nodes"][0]["intent_digest"] = intent_digest
            graph_digest = self.write(graph_path, graph)
            request = {"schema_version": "factory-role-request.v2", "request_id": "program.mayor", "program_id": "program", "semantic_bead_id": steps["agentops-build.mayor.iteration.1"], "workspace": str(workspace), "intent_source": str(intent), "intent_digest": intent_digest, "subject_path": str(intent), "subject_digest": intent_digest, "evidence_refs": [{"path": str(intent), "digest": intent_digest}, {"path": str(context_path), "digest": hashlib.sha256(context_path.read_bytes()).hexdigest()}], "prior_context_id": None, "role": "mayor", "requested": {"model": "fable", "reasoning": "adaptive", "provider": "claude", "fallback": {"allowed": False, "used": False, "reason": None}}, "artifact_path": str(graph_path), "result_path": str(mayor_result_path)}
            request_digest = self.write(mayor_request_path, request)
            response = {"schema_version": "factory-role-response.v2", "request_id": "program.mayor", "request_digest": request_digest, "role": "mayor", "semantic_bead_id": request["semantic_bead_id"], "session_context_id": "fable-session", "requested": request["requested"], "actual": {"model": "claude-fable-5", "reasoning": "adaptive", "provider": "claude", "effort": None, "fallback": {"allowed": False, "used": False, "reason": None}}, "artifact_path": str(graph_path), "artifact_digest": graph_digest}
            response_digest = self.write(mayor_result_path, response)
            pointer = {"schema_version": "agentops-factory-check.v1", "kind": "role", "phase": "mayor", "role": "mayor", "checked_bead_id": request["semantic_bead_id"], "semantic_bead_id": request["semantic_bead_id"], "session_context_id": "fable-session", "rig": None, "request_path": str(mayor_request_path), "result_path": str(mayor_result_path), "request_digest": request_digest, "result_digest": response_digest, "artifact_path": str(graph_path), "artifact_digest": graph_digest}
            pointer_path = workspace / "check.json"; self.write(pointer_path, pointer)
            original = feeder.run_checked
            def fake(argv, _cwd):
                if argv[0] == os.sys.executable:
                    return canonical({"ok": True, "response_digest": response_digest})
                if argv[0] == files["bd"] and argv[1] == "show":
                    self.assertTrue(plan_request_path.is_file(), "future Plan request must exist before binding proof")
                    return canonical({"id": steps["agentops-build.plan.iteration.1"], "description": "request_path=" + str(plan_request_path), "metadata": {"gc.run_target": "rig/agentops.plan-reviewer", "work_dir": str(workspace)}})
                raise AssertionError(argv)
            feeder.run_checked = fake
            try:
                with mock.patch.dict(os.environ, {"GC_BEAD_ID": steps["agentops-build.mayor"]}, clear=False):
                    self.assertEqual(feeder.check(SimpleNamespace(request=str(pointer_path))), 0)
            finally:
                feeder.run_checked = original
            plan_request = json.loads(plan_request_path.read_text(encoding="utf-8"))
            self.assertEqual(plan_request["prior_context_id"], "fable-session")
            self.assertEqual(plan_request["subject_digest"], graph_digest)
            self.assertEqual(plan_request["semantic_bead_id"], steps["agentops-build.plan.iteration.1"])

    def test_nonpass_plan_performs_zero_graph_mutations(self) -> None:
        feeder = self.feeder()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            graph = self.graph()
            plan = {"schema_version": "plan-review.v1", "program_id": "p", "intent_digest": "a" * 64, "graph_digest": "b" * 64, "mayor_context_id": "mayor", "reviewer_context_id": "sol", "provider": "codex", "verdict": "NOT_PROVEN", "criteria": [{"id": "scope", "result": "NOT_PROVEN", "reason": "missing"}], "findings": []}
            graph_path, plan_path = root / "graph.json", root / "plan.json"
            self.write(graph_path, graph); self.write(plan_path, plan)
            with self.assertRaises(feeder.FeederError):
                feeder.graph_request(graph, graph_path.read_bytes(), plan, plan_path.read_bytes())
            self.assertFalse((root / ".gc/agentops/factory/evidence").exists())

    def test_plan_check_nonpass_never_enters_admission(self) -> None:
        feeder = self.feeder()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            graph = self.graph(); graph_path = root / "graph.json"; graph_digest = self.write(graph_path, graph)
            plan = {"schema_version": "plan-review.v1", "program_id": "p", "intent_digest": "a" * 64, "graph_digest": graph_digest, "mayor_context_id": "mayor", "reviewer_context_id": "sol", "provider": "codex", "verdict": "FAIL", "criteria": [{"id": "scope", "result": "FAIL", "reason": "unsafe"}], "findings": [{"id": "f", "severity": "blocking", "node_ids": ["semantic"], "reason": "unsafe"}]}
            plan_path = root / "plan.json"; self.write(plan_path, plan)
            pointer_path = root / "pointer.json"; self.write(pointer_path, {"kind": "role", "phase": "plan"})
            context = {"graph_path": str(graph_path), "plan_artifact_path": str(plan_path)}
            original_verify, original_admit = feeder.verify_role_check, feeder.admit
            calls = []
            feeder.verify_role_check = lambda _pointer: ({}, {}, plan, context)
            feeder.admit = lambda _args: calls.append("admit")
            try:
                with self.assertRaises(feeder.FeederError):
                    feeder.check(SimpleNamespace(request=str(pointer_path)))
            finally:
                feeder.verify_role_check, feeder.admit = original_verify, original_admit
            self.assertEqual(calls, [])

    def test_existing_exact_graph_is_adopted_without_second_create(self) -> None:
        feeder = self.feeder()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            graph = self.graph()
            graph_path = root / "graph.json"; self.write(graph_path, graph)
            program_digest = hashlib.sha256(graph_path.read_bytes()).hexdigest()
            expected = feeder.compile_graph_apply_plan(graph, program_digest)["nodes"][0]
            row = {"id": "semantic-1", "title": expected["title"], "description": expected["description"],
                   "issue_type": expected["type"], "priority": expected["priority"], "labels": expected["labels"],
                   "metadata": expected["metadata"], "dependencies": []}
            original = feeder.run_checked
            calls = []
            def fake(argv, _cwd):
                calls.append(argv[1])
                if argv[1] == "list":
                    return canonical([row])
                if argv[1] == "show":
                    return canonical(row)
                raise AssertionError(argv)
            feeder.run_checked = fake
            try:
                self.assertEqual(feeder.adopted_graph_ids("bd", root, graph, program_digest), {"semantic": "semantic-1"})
            finally:
                feeder.run_checked = original
            self.assertNotIn("create", calls)

    def test_compiler_encodes_exact_edges_and_executable_bead_description(self) -> None:
        feeder = self.feeder()
        graph = self.graph()
        second = dict(graph["nodes"][0])
        second.update({"id": "dependent", "title": "Dependent unit", "depends_on": ["semantic"], "write_scope": ["docs"]})
        graph["nodes"].append(second)
        plan = feeder.compile_graph_apply_plan(graph, "b" * 64)
        self.assertEqual(set(plan), {"commit_message", "nodes", "edges"})
        self.assertEqual(plan["edges"], [{"from_key": "dependent", "to_key": "semantic", "type": "blocks"}])
        self.assertIn("First deterministic check:", plan["nodes"][0]["description"])
        self.assertEqual(plan["nodes"][0]["metadata"]["gc.node_id"], "semantic")

    def test_admit_new_graph_lost_success_replay_and_partial_collision_fail_closed(self) -> None:
        feeder = self.feeder()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            repository = root / "repo"; repository.mkdir()
            workers = root / "workers"; packets = root / "packets"
            intent = root / "intent"; intent.write_text("one exact intent\n", encoding="utf-8")
            intent_digest = hashlib.sha256(intent.read_bytes()).hexdigest()
            graph = self.graph()
            graph.update({"intent_digest": intent_digest, "repository_dir": str(repository), "workspace_root": str(workers), "packet_root": str(packets)})
            graph["nodes"][0]["intent_digest"] = intent_digest
            graph_path = root / "graph.json"; graph_digest = self.write(graph_path, graph)
            plan = {"schema_version": "plan-review.v1", "program_id": "p", "intent_digest": "a" * 64, "graph_digest": graph_digest, "mayor_context_id": "mayor", "reviewer_context_id": "sol", "provider": "codex", "verdict": "PASS", "criteria": [{"id": "scope", "result": "PASS", "reason": "bounded"}], "findings": []}
            plan["intent_digest"] = intent_digest
            plan_path = root / "review.json"; self.write(plan_path, plan)
            tools = {}
            for name in ("bd", "gc", "git", "role-adapter", "packet-adapter", "factory-check"):
                path = root / name; path.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8"); path.chmod(0o700); tools[name] = str(path)
            workspace = root / "build"; workspace.mkdir()
            context_path = workspace / "build-context.v1.json"
            build_steps = {reference: "build-" + reference.replace(".", "-") for reference in feeder.BUILD_STEP_REFS}
            delivery, _ = self.delivery(root)
            context = {"schema_version": "factory-build-context.v1", "program_id": "p", "source_bead_id": "source", "intent_path": str(intent), "intent_digest": intent_digest, "repository_dir": str(repository), "base_ref": "main", "base_oid": "b" * 40, "root": str(root), "workspace": str(workspace), "candidate_workspace_root": str(workers), "packet_root": str(packets), "max_parallel": 1, "graph_path": str(graph_path), "mayor_request_path": str(workspace / "mayor-request.json"), "mayor_result_path": str(workspace / "mayor-result.json"), "plan_request_path": str(workspace / "plan-request.json"), "plan_result_path": str(workspace / "plan-result.json"), "plan_artifact_path": str(plan_path), "workflow_root_id": "build-root", "workflow_steps": build_steps, "role_adapter": tools["role-adapter"], "packet_adapter": tools["packet-adapter"], "factory_check": tools["factory-check"], "bd_bin": tools["bd"], "gc_bin": tools["gc"], "git_bin": tools["git"], "delivery": delivery, "created_at": "2026-07-22T00:00:00Z"}
            self.write(context_path, context)
            work_dir, implement_packet, validate_packet, evidence = feeder.node_packet_paths(graph, graph["nodes"][0], graph_digest)
            evidence.mkdir(parents=True)
            self.write(implement_packet, {"schema_version": "gc-execution-envelope.v1", "workspace": str(work_dir)})
            compiled = feeder.compile_graph_apply_plan(graph, graph_digest)["nodes"][0]
            semantic_row = {"id": "semantic-1", "title": compiled["title"], "description": compiled["description"], "issue_type": compiled["type"], "priority": compiled["priority"], "labels": compiled["labels"], "metadata": compiled["metadata"], "dependencies": []}
            experiment_steps = {reference: "experiment-root" if reference == "agentops-experiment" else "experiment-" + reference.replace(".", "-") for reference in feeder.EXPERIMENT_STEP_REFS}
            def experiment_rows():
                rows = []
                for reference, identifier in experiment_steps.items():
                    metadata = {"gc.root_bead_id": "experiment-root", "gc.step_ref": reference}
                    description = ""
                    if reference == "agentops-experiment.implement.iteration.1":
                        metadata.update({"gc.run_target": "rig/agentops.implementer", "work_dir": str(work_dir), "gc.packet_path": str(implement_packet)})
                        description = "packet_path=" + str(implement_packet)
                    if reference == "agentops-experiment.validate.iteration.1":
                        metadata.update({"gc.run_target": "rig/agentops.validator", "work_dir": str(work_dir), "gc.packet_path": str(validate_packet)})
                        description = "packet_path=" + str(validate_packet)
                    rows.append({"id": identifier, "status": "closed" if reference == "agentops-experiment.admission" and state["closed"] else "open", "assignee": None, "description": description, "metadata": metadata})
                return rows
            state = {"created": False, "cooks": 0, "cooked": False, "closed": False}
            original = feeder.run_checked
            original_workspace = feeder.prepare_node_workspace
            def fake(argv, _cwd):
                if argv[0] == tools["bd"] and argv[1] == "list":
                    rows = [semantic_row] if state["created"] else []
                    if state["cooked"]:
                        rows += experiment_rows()
                    return canonical(rows)
                if argv[1:3] == ["create", "--graph"]:
                    state["created"] = True
                    return canonical({"ids": {"semantic": "semantic-1"}})
                if argv[1:3] == ["formula", "cook"]:
                    state["cooks"] += 1
                    state["cooked"] = True
                    return canonical({"ok": True, "formula": "agentops-experiment", "attach_bead_id": "semantic-1", "workflow_root_id": "experiment-root"})
                if argv[0] == tools["bd"] and argv[1:3] == ["dep", "list"]:
                    return b"[]\n"
                if argv[0] == tools["bd"] and argv[1] == "show":
                    if argv[2] == "semantic-1":
                        return canonical(semantic_row)
                    row = next(row for row in experiment_rows() if row["id"] == argv[2])
                    return canonical(row)
                if argv[0] == tools["bd"] and argv[1] == "close":
                    state["closed"] = True
                    return canonical({"id": argv[2], "status": "closed"})
                raise AssertionError(argv)
            feeder.run_checked = fake
            try:
                feeder.prepare_node_workspace = lambda *_args: (str(work_dir), str(implement_packet), str(validate_packet))
                args = SimpleNamespace(root=str(root), graph=str(graph_path), plan=str(plan_path), bd_bin=tools["bd"], gc_bin=tools["gc"], git_bin=tools["git"], intent=str(intent), packet_adapter=tools["packet-adapter"], factory_check=tools["factory-check"], context=str(context_path), created_at="2026-07-22T00:00:00Z")
                with redirect_stdout(io.StringIO()):
                    self.assertEqual(feeder.admit(args), 0)
                # Lost response replay observes all symbolic IDs before any second create.
                receipt = root / ".gc/agentops/factory/evidence/graph-admissions" / f"{graph_digest}.json"
                receipt.unlink()
                with redirect_stdout(io.StringIO()):
                    self.assertEqual(feeder.admit(args), 0)
                self.assertEqual(state["cooks"], 2)
            finally:
                feeder.run_checked = original
                feeder.prepare_node_workspace = original_workspace
            state["created"] = True
            original = feeder.run_checked
            def partial(argv, _cwd):
                if argv[1] == "list":
                    return canonical([{"id": "wrong", "metadata": {"gc.program_digest": graph_digest, "gc.graph_digest": graph_digest, "gc.node_id": "other"}}])
                return original(argv, _cwd)
            feeder.run_checked = partial
            try:
                with self.assertRaises(feeder.FeederError):
                    feeder.adopted_graph_ids("bd", root, graph, graph_digest)
            finally:
                feeder.run_checked = original


if __name__ == "__main__":
    unittest.main()
