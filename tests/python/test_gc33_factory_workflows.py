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

    @staticmethod
    def routes(rig: str = "agentops") -> dict[str, str]:
        return {"mayor": "agentops.mayor", "plan": f"{rig}/agentops.plan-reviewer", "implementer": f"{rig}/agentops.implementer", "implementer_claude": f"{rig}/agentops.implementer-claude", "validator": f"{rig}/agentops.validator"}

    @staticmethod
    def stable_workflow_rows(formula: str, root_id: str, routes: dict[str, str] | None = None) -> list[dict[str, object]]:
        persisted_refs = {
            "agentops-build": {
                "agentops-build.admission": "agentops-build.admission",
                "agentops-build.mayor": "agentops-build.mayor",
                "agentops-build.mayor.spec": "agentops-build.mayor.spec",
                "agentops-build.mayor.iteration.1": "mayor.iteration.1",
                "agentops-build.plan": "agentops-build.plan",
                "agentops-build.plan.spec": "agentops-build.plan.spec",
                "agentops-build.plan.iteration.1": "plan.iteration.1",
                "agentops-build.workflow-finalize": "agentops-build.workflow-finalize",
            },
            "agentops-experiment": {
                "agentops-experiment.admission": "agentops-experiment.admission",
                "agentops-experiment.implement": "agentops-experiment.implement",
                "agentops-experiment.implement.spec": "agentops-experiment.implement.spec",
                "agentops-experiment.implement.iteration.1": "implement.iteration.1",
                "agentops-experiment.validate": "agentops-experiment.validate",
                "agentops-experiment.validate.spec": "agentops-experiment.validate.spec",
                "agentops-experiment.validate.iteration.1": "validate.iteration.1",
                "agentops-experiment.workflow-finalize": "agentops-experiment.workflow-finalize",
            },
        }[formula]
        rows: list[dict[str, object]] = []
        controls: dict[str, str] = {}
        rows.append({"id": root_id, "metadata": {"gc.kind": "workflow", "gc.formula_contract": "graph.v2"}})
        for reference, persisted_ref in sorted(persisted_refs.items()):
            suffix = reference.removeprefix(formula + ".")
            metadata: dict[str, object] = {"gc.root_bead_id": root_id, "gc.step_ref": persisted_ref}
            if suffix.endswith(".spec"):
                role = suffix.removesuffix(".spec")
                metadata.update({"gc.kind": "spec", "gc.spec_for": role, "gc.spec_for_ref": role})
            elif ".iteration." in suffix:
                role, attempt = suffix.split(".iteration.", 1)
                metadata.update({"gc.step_ref": suffix, "gc.step_id": role, "gc.ralph_step_id": role, "gc.attempt": attempt, "gc.logical_bead_id": controls[role]})
                if routes and role in {"mayor", "plan", "implement", "validate"}:
                    target = routes["mayor" if role == "mayor" else "plan" if role == "plan" else "implementer" if role == "implement" else "validator"]
                    metadata.update({"gc.run_target": target, "gc.routed_to": target, "work_dir": "/work"})
            elif suffix in {"mayor", "plan", "implement", "validate"}:
                controls[suffix] = f"{formula}-{suffix}"
                metadata.update({"gc.kind": "ralph", "gc.step_id": suffix,
                                 "gc.routed_to": "core.control-dispatcher"})
            elif suffix == "workflow-finalize":
                metadata.update({"gc.kind": "workflow-finalize",
                                 "gc.routed_to": "core.control-dispatcher"})
            rows.append({"id": f"{formula}-{suffix.replace('.', '-')}", "metadata": metadata})
        return rows

    def test_formula_roles_are_checked_once_and_have_no_drain_or_fallback(self) -> None:
        build = (ROOT / "packs/agentops-factory/formulas/agentops-build.toml").read_text()
        experiment = (ROOT / "packs/agentops-factory/formulas/agentops-experiment.toml").read_text()
        for text in (build, experiment):
            self.assertNotIn("drain", text)
            self.assertNotIn("retry", text)
            self.assertIn("max_attempts = 1", text)
        self.assertIn('[vars.mayor_target]', build)
        self.assertIn('[vars.plan_target]', build)
        self.assertIn('gc.run_target" = "{{mayor_target}}"', build)
        self.assertIn('gc.run_target" = "{{plan_target}}"', build)
        self.assertIn('gc.run_target" = "{{implement_target}}"', experiment)
        self.assertIn('"work_dir" = "{{work_dir}}"', experiment)
        self.assertIn('[vars.validate_target]', experiment)
        self.assertIn('gc.run_target" = "{{validate_target}}"', experiment)
        self.assertNotIn("refiner", build + experiment)
        self.assertNotIn("luna", build + experiment)

    def test_formula_cook_routes_are_sealed_concrete_rig_targets(self) -> None:
        """Formula cook receives explicit real rig targets; no ambient-current-rig inference."""
        feeder_source = FEEDER.read_text(encoding="utf-8")
        authored = (
            (ROOT / "packs/agentops-factory/formulas/agentops-build.toml").read_text(encoding="utf-8")
            + (ROOT / "packs/agentops-factory/formulas/agentops-experiment.toml").read_text(encoding="utf-8")
            + feeder_source
        )
        self.assertIn('"{{plan_target}}"', authored)
        self.assertIn('"{{validate_target}}"', authored)
        for target in ("agentops.plan-reviewer", "agentops.implementer",
                       "agentops.implementer-claude", "agentops.validator"):
            self.assertIn(target, authored)

    def test_workflow_rows_normalizes_only_exact_stable_attempt_refs(self) -> None:
        feeder = self.feeder()
        rows = self.stable_workflow_rows("agentops-build", "build-root", self.routes())
        original = feeder.all_beads
        feeder.all_beads = lambda *_args: rows
        try:
            steps, records = feeder.workflow_rows("bd", Path("/repo"), "build-root", "agentops-build", feeder.BUILD_STEP_REF_MAP)
        finally:
            feeder.all_beads = original
        self.assertEqual(set(steps), feeder.BUILD_STEP_REFS)
        self.assertEqual(steps["agentops-build.mayor.iteration.1"], "agentops-build-mayor-iteration-1")
        self.assertEqual(records["agentops-build.plan.iteration.1"]["metadata"]["gc.step_ref"], "plan.iteration.1")

    def test_control_dispatcher_routes_are_sealed_only_on_compiled_controls(self) -> None:
        feeder = self.feeder()
        rows = self.stable_workflow_rows("agentops-build", "build-root", self.routes())
        original_all, original_run = feeder.all_beads, feeder.run_checked
        updates: list[str] = []

        def fake(argv, _cwd):
            if argv[1] == "update":
                row = next(item for item in rows if item["id"] == argv[2])
                row["metadata"]["gc.routed_to"] = next(
                    value.removeprefix("gc.routed_to=") for value in argv if value.startswith("gc.routed_to=")
                )
                row["metadata"]["gc.execution_rig_context"] = next(
                    value.removeprefix("gc.execution_rig_context=") for value in argv if value.startswith("gc.execution_rig_context=")
                )
                updates.append(argv[2])
                return canonical({"id": argv[2]})
            if argv[1] == "show":
                return canonical(next(item for item in rows if item["id"] == argv[2]))
            raise AssertionError(argv)

        feeder.all_beads = lambda *_args: rows
        feeder.run_checked = fake
        try:
            _, records = feeder.workflow_rows("bd", Path("/repo"), "build-root", "agentops-build", feeder.BUILD_STEP_REF_MAP)
            sealed = feeder.seal_compiled_control_dispatcher_routes("bd", Path("/repo"), "agentops-build", records, "named-rig")
        finally:
            feeder.all_beads, feeder.run_checked = original_all, original_run
        controls = feeder.formula_control_references("agentops-build")
        self.assertEqual(set(updates), {records[reference]["id"] for reference in controls})
        for reference in controls:
            metadata = sealed[reference]["metadata"]
            self.assertEqual(metadata["gc.routed_to"], "named-rig/core.control-dispatcher")
            self.assertEqual(metadata["gc.execution_rig_context"], "named-rig")
        attempt = sealed["agentops-build.mayor.iteration.1"]["metadata"]
        self.assertNotIn("gc.execution_rig_context", attempt)
        self.assertEqual(attempt["gc.routed_to"], self.routes()["mayor"])

    def test_control_dispatcher_route_rejects_foreign_compiler_drift(self) -> None:
        feeder = self.feeder()
        rows = self.stable_workflow_rows("agentops-experiment", "experiment-root", self.routes())
        control = next(row for row in rows if row["metadata"].get("gc.step_ref") == "agentops-experiment.validate")
        control["metadata"]["gc.routed_to"] = "other/core.control-dispatcher"
        original = feeder.all_beads
        feeder.all_beads = lambda *_args: rows
        try:
            _, records = feeder.workflow_rows("bd", Path("/repo"), "experiment-root", "agentops-experiment", feeder.EXPERIMENT_STEP_REF_MAP)
            with self.assertRaisesRegex(feeder.FeederError, "foreign route"):
                feeder.seal_compiled_control_dispatcher_routes("bd", Path("/repo"), "agentops-experiment", records, "named-rig")
        finally:
            feeder.all_beads = original

    def test_control_dispatcher_route_requires_exact_persisted_reread(self) -> None:
        feeder = self.feeder()
        rows = self.stable_workflow_rows("agentops-build", "build-root", self.routes())
        original_all, original_run = feeder.all_beads, feeder.run_checked
        updates: list[str] = []

        def fake(argv, _cwd):
            if argv[1] == "update":
                updates.append(argv[2])
                return canonical({"id": argv[2]})
            if argv[1] == "show":
                return canonical(next(item for item in rows if item["id"] == argv[2]))
            raise AssertionError(argv)

        feeder.all_beads = lambda *_args: rows
        feeder.run_checked = fake
        try:
            _, records = feeder.workflow_rows(
                "bd", Path("/repo"), "build-root", "agentops-build", feeder.BUILD_STEP_REF_MAP,
            )
            with self.assertRaisesRegex(feeder.FeederError, "did not re-read"):
                feeder.seal_compiled_control_dispatcher_routes(
                    "bd", Path("/repo"), "agentops-build", records, "named-rig",
                )
        finally:
            feeder.all_beads, feeder.run_checked = original_all, original_run
        self.assertEqual(len(updates), 1)

    def test_workflow_rows_rejects_foreign_or_canonical_attempt_refs_and_bad_structure(self) -> None:
        feeder = self.feeder()
        cases: list[tuple[str, list[dict[str, object]]]] = []
        valid = self.stable_workflow_rows("agentops-experiment", "experiment-root", self.routes())
        foreign = json.loads(json.dumps(valid)); next(row for row in foreign if row["metadata"].get("gc.step_ref") == "implement.iteration.1")["metadata"]["gc.step_ref"] = "foreign.iteration.1"; cases.append(("foreign", foreign))
        canonical = json.loads(json.dumps(valid)); next(row for row in canonical if row["metadata"].get("gc.step_ref") == "implement.iteration.1")["metadata"]["gc.step_ref"] = "agentops-experiment.implement.iteration.1"; cases.append(("canonical", canonical))
        wrong_root = json.loads(json.dumps(valid)); next(row for row in wrong_root if row["metadata"].get("gc.step_ref") == "implement.iteration.1")["metadata"]["gc.root_bead_id"] = "foreign-root"; cases.append(("wrong-root", wrong_root))
        wrong_logical = json.loads(json.dumps(valid)); next(row for row in wrong_logical if row["metadata"].get("gc.step_ref") == "implement.iteration.1")["metadata"]["gc.logical_bead_id"] = "wrong-control"; cases.append(("wrong-logical", wrong_logical))
        wrong_step = json.loads(json.dumps(valid)); next(row for row in wrong_step if row["metadata"].get("gc.step_ref") == "implement.iteration.1")["metadata"]["gc.step_id"] = "validate"; cases.append(("wrong-step", wrong_step))
        wrong_attempt = json.loads(json.dumps(valid)); next(row for row in wrong_attempt if row["metadata"].get("gc.step_ref") == "implement.iteration.1")["metadata"]["gc.attempt"] = "2"; cases.append(("wrong-attempt", wrong_attempt))
        malformed_root = json.loads(json.dumps(valid)); malformed_root[0]["metadata"]["gc.formula_contract"] = "v1"; cases.append(("root", malformed_root))
        duplicate = json.loads(json.dumps(valid)); duplicate.append(json.loads(json.dumps(valid[1]))); cases.append(("duplicate", duplicate))
        missing = [row for row in valid if row["metadata"].get("gc.step_ref") != "validate.iteration.1"]; cases.append(("missing", missing))
        for label, rows in cases:
            with self.subTest(label=label):
                original = feeder.all_beads
                feeder.all_beads = lambda *_args, rows=rows: rows
                try:
                    with self.assertRaises(feeder.FeederError):
                        feeder.workflow_rows("bd", Path("/repo"), "experiment-root", "agentops-experiment", feeder.EXPERIMENT_STEP_REF_MAP)
                finally:
                    feeder.all_beads = original

    def test_experiment_formula_binds_pinned_graphv2_attachment_to_source_convoy(self) -> None:
        experiment = (ROOT / "packs/agentops-factory/formulas/agentops-experiment.toml").read_text()
        # GC v1.3.5 stamps a stable graph.v2 attach key only when the rendered
        # recipe references convoy_id. Every semantic bead therefore gets one
        # replay-adoptable workflow instead of an unbound duplicate.
        self.assertIn("{{convoy_id}}", experiment)

    def test_start_binds_city_mayor_assignment_before_releasing_build_admission(self) -> None:
        feeder = self.feeder()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            repository = root / "repo"; repository.mkdir()
            intent = root / "intent"; intent.write_text("one exact intent\n", encoding="utf-8")
            tools = {}
            for name in ("bd", "gc", "git", "gh", "bash", "delivery", "role-adapter", "packet-adapter", "factory-check"):
                path = root / name; path.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8"); path.chmod(0o700); tools[name] = str(path.resolve())
            state = {"cook": 0, "close": 0, "workspace": "", "mayor_request": "", "base": "a" * 40,
                     "malformed": True, "sealed_controls": {}, "mayor_assignee": None, "mayor_assigns": 0}

            def rows():
                values = self.stable_workflow_rows("agentops-build", "workflow-1", self.routes())
                for row in values:
                    metadata = row["metadata"]
                    metadata.update(state["sealed_controls"].get(row["id"], {}))
                    description = ""
                    if metadata.get("gc.step_ref") == "mayor.iteration.1":
                        metadata.update({"work_dir": state["workspace"]})
                        description = "request_path=" + state["mayor_request"]
                    if metadata.get("gc.step_ref") == "plan.iteration.1":
                        metadata.update({"work_dir": state["workspace"]})
                    if state["malformed"] and metadata.get("gc.step_ref") == "mayor.iteration.1":
                        metadata["gc.step_ref"] = "agentops-build.mayor.iteration.1"
                    row.update({"status": "closed" if metadata.get("gc.step_ref") == "agentops-build.admission" and state["close"] else "open",
                                "assignee": state["mayor_assignee"] if metadata.get("gc.step_ref") == "mayor.iteration.1" else None,
                                "description": description})
                return values

            original = feeder.run_checked
            def fake(argv, _cwd):
                if argv[0] == tools["git"]:
                    return (str(state["base"]) + "\n").encode()
                if argv[0] == tools["gc"]:
                    state["cook"] += 1
                    self.assertIn("mayor_target=agentops.mayor", argv)
                    self.assertIn("plan_target=agentops/agentops.plan-reviewer", argv)
                    state["workspace"] = next(value.removeprefix("work_dir=") for value in argv if value.startswith("work_dir="))
                    state["mayor_request"] = next(value.removeprefix("mayor_request=") for value in argv if value.startswith("mayor_request="))
                    return canonical({"schema_version": "1", "ok": True, "formula": "agentops-build", "mode": "attach", "attach_bead_id": "source-1", "root_id": "workflow-1", "workflow_root_id": "workflow-1", "created": 9})
                if argv[0] == tools["bd"] and argv[1] == "list":
                    return canonical(rows())
                if argv[0] == tools["bd"] and argv[1] == "update":
                    if argv[2] == "agentops-build-mayor-iteration-1":
                        self.assertEqual(argv[3:], ["--assignee", "agentops.mayor", "--json"])
                        state["mayor_assignee"] = "agentops.mayor"
                        state["mayor_assigns"] += 1
                        return canonical({"id": argv[2]})
                    state["sealed_controls"][argv[2]] = {
                        "gc.routed_to": next(value.removeprefix("gc.routed_to=") for value in argv if value.startswith("gc.routed_to=")),
                        "gc.execution_rig_context": next(value.removeprefix("gc.execution_rig_context=") for value in argv if value.startswith("gc.execution_rig_context=")),
                    }
                    return canonical({"id": argv[2]})
                if argv[0] == tools["bd"] and argv[1] == "show":
                    return canonical(next(row for row in rows() if row["id"] == argv[2]))
                if argv[0] == tools["bd"] and argv[1] == "close":
                    self.assertTrue(Path(state["mayor_request"]).is_file(), "request must exist before admission close")
                    state["close"] += 1
                    return canonical({"id": argv[2], "status": "closed"})
                raise AssertionError(argv)
            delivery, _ = self.delivery(repository, repository, tools)
            feeder.run_checked = fake
            try:
                args = SimpleNamespace(root=str(repository), source_bead="source-1", intent=str(intent), repository=str(repository), base_ref="main", max_parallel=2, bd_bin=tools["bd"], gc_bin=tools["gc"], git_bin=tools["git"], role_adapter=tools["role-adapter"], packet_adapter=tools["packet-adapter"], factory_check=tools["factory-check"], delivery_native_context=delivery["native_context_path"], delivery_native_context_digest=delivery["native_context_digest"], delivery_root=delivery["evidence_root"], delivery_mode="auto", delivery_deadline_seconds=86400, created_at="2026-07-22T00:00:00Z", rig_id="agentops")
                with self.assertRaises(feeder.FeederError):
                    feeder.start(args)
                self.assertEqual(state["close"], 0, "malformed stable rows must stop before admission close")
                state["malformed"] = False
                with redirect_stdout(io.StringIO()):
                    self.assertEqual(feeder.start(args), 0)
                self.assertEqual(state["mayor_assignee"], "agentops.mayor")
                self.assertEqual(state["mayor_assigns"], 1)
                state["base"] = "b" * 40
                with redirect_stdout(io.StringIO()):
                    self.assertEqual(feeder.start(args), 0)
                self.assertEqual(state["mayor_assigns"], 1, "exact Mayor assignment must be adopted")
                state["mayor_assignee"] = "foreign.mayor"
                with self.assertRaises(feeder.FeederError):
                    feeder.start(args)
                self.assertEqual(state["close"], 1, "foreign Mayor assignment must stop before admission close")
            finally:
                feeder.run_checked = original
            self.assertEqual((state["cook"], state["close"]), (2, 1))
            context = json.loads((Path(state["workspace"]) / "build-context.v1.json").read_bytes())
            self.assertEqual(context["base_oid"], "a" * 40)
            self.assertEqual(context["routes"], self.routes())
            request = json.loads(Path(state["mayor_request"]).read_text(encoding="utf-8"))
            self.assertEqual(request["semantic_bead_id"], "agentops-build-mayor-iteration-1")
            self.assertEqual(request["role"], "mayor")

    def test_start_rejects_invalid_or_mismatched_rig_before_formula_cook(self) -> None:
        feeder = self.feeder()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            repository = root / "repo"; repository.mkdir()
            intent = root / "intent"; intent.write_text("one exact intent\n", encoding="utf-8")
            tools = {}
            for name in ("bd", "gc", "git", "role-adapter", "packet-adapter", "factory-check"):
                path = root / name; path.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8"); path.chmod(0o700); tools[name] = str(path)
            delivery = {"native_context_path": str(root / "native.json"), "native_context_digest": "d" * 64, "evidence_root": str(root), "mode": "auto", "deadline_seconds": 86400}
            args = SimpleNamespace(root=str(repository), source_bead="source-1", intent=str(intent), repository=str(repository), base_ref="main", max_parallel=1, bd_bin=tools["bd"], gc_bin=tools["gc"], git_bin=tools["git"], role_adapter=tools["role-adapter"], packet_adapter=tools["packet-adapter"], factory_check=tools["factory-check"], created_at="2026-07-22T00:00:00Z", rig_id="bad/rig")
            original_delivery, original_run = feeder.delivery_configuration, feeder.run_checked
            cooks: list[list[str]] = []
            feeder.delivery_configuration = lambda *_args: (delivery, "agentops")
            feeder.run_checked = lambda argv, _cwd: cooks.append(argv) or (_ for _ in ()).throw(AssertionError(argv))
            try:
                with self.assertRaisesRegex(feeder.FeederError, "--rig-id has an unsafe identity"):
                    feeder.start(args)
                args.rig_id = "other"
                with self.assertRaisesRegex(feeder.FeederError, "differs from the digest-bound"):
                    feeder.start(args)
            finally:
                feeder.delivery_configuration, feeder.run_checked = original_delivery, original_run
            self.assertEqual(cooks, [])

    def test_build_context_rejects_routes_not_derived_from_its_rig(self) -> None:
        feeder = self.feeder()
        context = {"schema_version": "factory-build-context.v1", "program_id": "p", "source_bead_id": "source", "intent_path": "/intent", "intent_digest": "a" * 64, "repository_dir": "/repo", "base_ref": "main", "base_oid": "b" * 40, "rig_id": "agentops", "routes": self.routes("other"), "root": "/root", "workspace": "/workspace", "candidate_workspace_root": "/workers", "packet_root": "/packets", "max_parallel": 1, "graph_path": "/graph", "mayor_request_path": "/mayor-request", "mayor_result_path": "/mayor-result", "plan_request_path": "/plan-request", "plan_result_path": "/plan-result", "plan_artifact_path": "/plan", "workflow_root_id": "workflow", "workflow_steps": {reference: reference for reference in feeder.BUILD_STEP_REFS}, "role_adapter": "/role-adapter", "packet_adapter": "/packet-adapter", "factory_check": "/factory-check", "bd_bin": "/bd", "gc_bin": "/gc", "git_bin": "/git", "delivery": {"native_context_path": "/native", "native_context_digest": "c" * 64, "evidence_root": "/evidence", "mode": "auto", "deadline_seconds": 86400}, "created_at": "2026-07-22T00:00:00Z"}
        with self.assertRaisesRegex(feeder.FeederError, "routes differ"):
            feeder.validate_build_context(context)

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
            context = {"schema_version": "factory-build-context.v1", "program_id": "program", "source_bead_id": "source", "intent_path": str(intent), "intent_digest": intent_digest, "repository_dir": "/repo", "base_ref": "main", "base_oid": "a" * 40, "rig_id": "agentops", "routes": self.routes(), "root": str(root), "workspace": str(workspace), "candidate_workspace_root": "/workers", "packet_root": "/packets", "max_parallel": 2, "graph_path": str(graph_path), "mayor_request_path": str(mayor_request_path), "mayor_result_path": str(mayor_result_path), "plan_request_path": str(plan_request_path), "plan_result_path": str(plan_result_path), "plan_artifact_path": str(plan_artifact_path), "workflow_root_id": "root", "workflow_steps": steps, "role_adapter": files["role-adapter"], "packet_adapter": files["packet-adapter"], "factory_check": files["factory-check"], "bd_bin": files["bd"], "gc_bin": files["gc"], "git_bin": files["git"], "delivery": delivery, "created_at": "2026-07-22T00:00:00Z"}
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
                    return canonical({"id": steps["agentops-build.plan.iteration.1"], "description": "request_path=" + str(plan_request_path), "metadata": {"gc.run_target": "agentops/agentops.plan-reviewer", "gc.routed_to": "agentops/agentops.plan-reviewer", "work_dir": str(workspace)}})
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
            context = {"schema_version": "factory-build-context.v1", "program_id": "p", "source_bead_id": "source", "intent_path": str(intent), "intent_digest": intent_digest, "repository_dir": str(repository), "base_ref": "main", "base_oid": "b" * 40, "rig_id": "agentops", "routes": self.routes(), "root": str(root), "workspace": str(workspace), "candidate_workspace_root": str(workers), "packet_root": str(packets), "max_parallel": 1, "graph_path": str(graph_path), "mayor_request_path": str(workspace / "mayor-request.json"), "mayor_result_path": str(workspace / "mayor-result.json"), "plan_request_path": str(workspace / "plan-request.json"), "plan_result_path": str(workspace / "plan-result.json"), "plan_artifact_path": str(plan_path), "workflow_root_id": "build-root", "workflow_steps": build_steps, "role_adapter": tools["role-adapter"], "packet_adapter": tools["packet-adapter"], "factory_check": tools["factory-check"], "bd_bin": tools["bd"], "gc_bin": tools["gc"], "git_bin": tools["git"], "delivery": delivery, "created_at": "2026-07-22T00:00:00Z"}
            self.write(context_path, context)
            work_dir, implement_packet, validate_packet, evidence = feeder.node_packet_paths(graph, graph["nodes"][0], graph_digest)
            evidence.mkdir(parents=True)
            self.write(implement_packet, {"schema_version": "gc-execution-envelope.v1", "workspace": str(work_dir)})
            compiled = feeder.compile_graph_apply_plan(graph, graph_digest)["nodes"][0]
            semantic_row = {"id": "semantic-1", "title": compiled["title"], "description": compiled["description"], "issue_type": compiled["type"], "priority": compiled["priority"], "labels": compiled["labels"], "metadata": compiled["metadata"], "dependencies": []}
            def experiment_rows():
                rows = self.stable_workflow_rows("agentops-experiment", "experiment-root", self.routes())
                for row in rows:
                    metadata = row["metadata"]
                    metadata.update(state["sealed_controls"].get(row["id"], {}))
                    description = ""
                    if metadata.get("gc.step_ref") == "implement.iteration.1":
                        metadata.update({"work_dir": str(work_dir), "gc.packet_path": str(implement_packet)})
                        description = "packet_path=" + str(implement_packet)
                    if metadata.get("gc.step_ref") == "validate.iteration.1":
                        metadata.update({"work_dir": str(work_dir), "gc.packet_path": str(validate_packet)})
                        description = "packet_path=" + str(validate_packet)
                    row.update({"status": "closed" if metadata.get("gc.step_ref") == "agentops-experiment.admission" and state["closed"] else "open", "assignee": None, "description": description})
                return rows
            state = {"created": False, "cooks": 0, "cooked": False, "closed": False, "sealed_controls": {}}
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
                    self.assertIn("implement_target=agentops/agentops.implementer", argv)
                    self.assertIn("validate_target=agentops/agentops.validator", argv)
                    state["cooks"] += 1
                    state["cooked"] = True
                    return canonical({"ok": True, "formula": "agentops-experiment", "attach_bead_id": "semantic-1", "workflow_root_id": "experiment-root"})
                if argv[0] == tools["bd"] and argv[1:3] == ["dep", "list"]:
                    return b"[]\n"
                if argv[0] == tools["bd"] and argv[1] == "update":
                    state["sealed_controls"][argv[2]] = {
                        "gc.routed_to": next(value.removeprefix("gc.routed_to=") for value in argv if value.startswith("gc.routed_to=")),
                        "gc.execution_rig_context": next(value.removeprefix("gc.execution_rig_context=") for value in argv if value.startswith("gc.execution_rig_context=")),
                    }
                    return canonical({"id": argv[2]})
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

    def test_experiment_binding_accepts_only_sealed_terra_and_opus_targets(self) -> None:
        feeder = self.feeder()
        routes = self.routes("named-rig")
        for model, target in (("terra", routes["implementer"]), ("opus", routes["implementer_claude"])):
            rows = self.stable_workflow_rows("agentops-experiment", "experiment-root", routes)
            for row in rows:
                metadata = row["metadata"]
                if metadata.get("gc.step_ref") == "implement.iteration.1":
                    metadata.update({"gc.run_target": target, "gc.routed_to": target, "work_dir": "/work", "gc.packet_path": "/implement"})
                    row["description"] = "packet_path=/implement"
                if metadata.get("gc.step_ref") == "validate.iteration.1":
                    metadata.update({"work_dir": "/work", "gc.packet_path": "/validate"})
                    row["description"] = "packet_path=/validate"
                row["assignee"] = None
            original = feeder.all_beads
            feeder.all_beads = lambda *_args: rows
            try:
                steps, records = feeder.workflow_rows("bd", Path("/repo"), "experiment-root", "agentops-experiment", feeder.EXPERIMENT_STEP_REF_MAP)
            finally:
                feeder.all_beads = original
            feeder.experiment_binding(records, steps, {"id": model, "model": model}, "/work", "/implement", "/validate", routes)


if __name__ == "__main__":
    unittest.main()
