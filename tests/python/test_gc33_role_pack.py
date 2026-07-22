from __future__ import annotations

import importlib.util
import json
import hashlib
import contextlib
import io
from copy import deepcopy
from pathlib import Path
import tempfile
import unittest
import os
import subprocess
from unittest import mock

from jsonschema import Draft202012Validator


ROOT = Path(__file__).resolve().parents[2]
FACTORY_PATH = ROOT / "packs/agentops-factory/assets/scripts/factory.py"
SPEC = importlib.util.spec_from_file_location("gc33_role_factory", FACTORY_PATH)
assert SPEC and SPEC.loader
factory = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(factory)
NO_FALLBACK = {"allowed": False, "used": False, "reason": None}


def graph() -> dict:
    return {
        "schema_version": "program-graph.v2", "program_id": "program",
        "intent_digest": "a" * 64, "max_parallel": 1,
        "delivery_group_id": "group", "prefix_safety": "safe",
        "role_policy": {
            "mayor": {"model": "fable", "reasoning": "adaptive", "provider": "claude", "fallback": NO_FALLBACK},
            "planner": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": NO_FALLBACK},
            "validator": {"model": "sol", "reasoning": "high", "provider": "codex", "fallback": NO_FALLBACK},
            "worker_pool": {"default": {"model": "terra", "reasoning": "high", "provider": "codex", "fallback": NO_FALLBACK}, "overflow": {"model": "opus", "reasoning": "medium", "provider": "claude", "fallback": NO_FALLBACK}, "fallback": NO_FALLBACK},
            "refiner": {"model": "fable", "reasoning": "adaptive", "provider": "claude", "fallback": NO_FALLBACK, "ambiguity_only": True},
            "luna": {"model": "luna", "reasoning": "high", "provider": "codex", "fallback": NO_FALLBACK, "support_only": True},
        },
        "nodes": [{"id": "node", "bead_class": "product", "intent_digest": "a" * 64,
                   "depends_on": [], "write_scope": ["src"], "generated_companions": [],
                   "role": "implementation", "model": "terra", "reasoning": "high",
                   "provider": "codex", "fallback": NO_FALLBACK}],
    }


class GC33RolePackTest(unittest.TestCase):
    def test_central_claim_is_one_shot_and_fails_closed(self) -> None:
        claim = ROOT / "packs/agentops-executor/assets/scripts/claim.py"
        self.assertIn('command = ["claim"]', (ROOT / "packs/agentops-executor/commands/claim/command.toml").read_text(encoding="utf-8"))
        self.assertIn('command = ["claim"]', (ROOT / "packs/agentops-factory/commands/claim/command.toml").read_text(encoding="utf-8"))
        self.assertIn("../agentops-executor/assets/scripts/claim.py", (ROOT / "packs/agentops-factory/commands/claim/run.sh").read_text(encoding="utf-8"))
        prompts = [ROOT / path for path in ("packs/agentops-executor/agents/implementer/prompt.template.md", "packs/agentops-executor/agents/implementer-claude/prompt.template.md", "packs/agentops-executor/agents/validator/prompt.template.md", "packs/agentops-factory/agents/mayor/prompt.template.md", "packs/agentops-factory/agents/plan-reviewer/prompt.template.md", "packs/agentops-factory/agents/refiner/prompt.template.md")]
        for prompt in prompts:
            body = prompt.read_text(encoding="utf-8")
            self.assertIn("gc agentops claim", body)
            self.assertNotIn("hook --claim", body)
            self.assertNotIn("action=work", body)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); calls = root / "calls"; fake = root / "gc"
            fake.write_text("#!/usr/bin/env bash\necho x >> \"$GC_FAKE_CALLS\"\nprintf '%s' \"$GC_FAKE_OUTPUT\"\nexit \"${GC_FAKE_EXIT:-0}\"\n", encoding="utf-8")
            fake.chmod(0o755)
            base = {"GC_BIN": str(fake), "GC_FAKE_CALLS": str(calls)}
            work = '{"schema_version":"1","ok":true,"command":"hook","action":"work","reason":"claimed","bead_id":"b-1","assignee":"agent"}'
            ready = '{"schema_version":"1","ok":true,"command":"hook","action":"work","reason":"ready_assignment","bead_id":"b-1","assignee":"agent","route":"agentops"}'
            drain = '{"schema_version":"1","ok":true,"command":"hook","action":"drain","reason":"no_work","drain_acknowledged":true}'
            bad_extra = '{"schema_version":"1","ok":true,"command":"hook","action":"work","reason":"claimed","bead_id":"b-1","assignee":"agent","extra":true}'
            for label, output, code, expected_ok, expected_action in (("assigned", work, "0", True, "assigned"), ("ready", ready, "0", True, "assigned"), ("drain", drain, "0", True, "drain"), ("extra", bad_extra, "0", False, "uncertain"), ("invalid", 'not-json', "0", False, "uncertain"), ("nonzero", work, "7", False, "uncertain")):
                with self.subTest(label=label):
                    calls.unlink(missing_ok=True)
                    result = subprocess.run(["python3", str(claim)], env={**os.environ, **base, "GC_FAKE_OUTPUT": output, "GC_FAKE_EXIT": code}, text=True, capture_output=True, check=False)
                    value = json.loads(result.stdout)
                    self.assertEqual(value["ok"], expected_ok)
                    self.assertEqual(value["action"], expected_action)
                    self.assertEqual(calls.read_text(encoding="utf-8").count("x\n"), 1)
    def test_delivery_policy_is_absent_at_every_composition_boundary(self) -> None:
        factory_agents = ROOT / "packs/agentops-factory/agents"
        self.assertEqual({p.name for p in factory_agents.iterdir() if (p / "agent.toml").is_file()}, {"mayor", "plan-reviewer", "refiner"})
        self.assertFalse((factory_agents / "delivery-policy").exists())
        self.assertFalse((ROOT / "packs/agentops-factory/assets/schemas/delivery-policy-finding.v1.schema.json").exists())
        self.assertNotIn("delivery_policy", factory.ROLE_TEMPLATES)
        self.assertNotIn("delivery_policy", factory.V2_ROUTABLE_ROLES)
        with self.assertRaises(factory.FactoryError):
            factory.role_artifact_contract_v2("delivery_policy")

    def test_v2_schemas_reject_delivery_policy(self) -> None:
        for name in ("program-graph.v2.schema.json", "factory-role-request.v2.schema.json", "factory-role-response.v2.schema.json"):
            schema = json.loads((ROOT / "packs/agentops-factory/assets/schemas" / name).read_text())
            self.assertNotIn("delivery_policy", json.dumps(schema))
        schema = json.loads((ROOT / "packs/agentops-factory/assets/schemas/program-graph.v2.schema.json").read_text())
        candidate = graph(); candidate["role_policy"]["delivery_policy"] = {"model": "sol"}
        self.assertFalse(Draft202012Validator(schema).is_valid(candidate))

    def test_v2_scope_paths_are_canonical_and_closed(self) -> None:
        valid = graph()
        valid["nodes"][0]["write_scope"] = ["./src\\nested"]
        valid["nodes"][0]["generated_companions"] = ["generated/output"]
        self.assertEqual(factory.validate_program_graph_v2(valid, "a" * 64)["nodes"][0]["write_scope"], ["src/nested"])
        cases = (
            ("write_scope", ["/absolute"]), ("write_scope", ["C:\\absolute"]),
            ("write_scope", ["src/../other"]), ("write_scope", ["./"]),
            ("write_scope", ["src", "./src"]),
            ("generated_companions", ["src"]),
        )
        for field, paths in cases:
            candidate = graph(); candidate["nodes"][0][field] = paths
            if field == "generated_companions":
                candidate["nodes"][0]["write_scope"] = ["src"]
            with self.subTest(field=field, paths=paths), self.assertRaises(factory.FactoryError):
                factory.validate_program_graph_v2(candidate, "a" * 64)

    def test_fable_adviser_is_guarded_and_dispatch_is_closed(self) -> None:
        city = (ROOT / "deploy/gc/city.toml").read_text(encoding="utf-8")
        self.assertNotIn("Git delivery authority", city)
        self.assertIn("--dangerously-skip-permissions", city)
        contract = factory.fable_adviser_launch_contract()
        self.assertEqual(contract["credential_environment"], [])
        self.assertEqual(contract["forbidden_environment"], contract["stripped_environment"])
        self.assertEqual(contract["stripped_environment"], ["GITHUB_TOKEN", "GH_TOKEN", "GIT_ASKPASS", "SSH_ASKPASS", "SSH_AUTH_SOCK"])
        wrapper = (ROOT / "deploy/gc/claude-interactive.sh").read_text(encoding="utf-8")
        self.assertIn("claude-fable-5", wrapper)
        self.assertIn("env -u GITHUB_TOKEN -u GH_TOKEN -u GIT_ASKPASS -u SSH_ASKPASS -u SSH_AUTH_SOCK", wrapper)
        self.assertEqual(factory.adviser_dispatch_decision(), {"status": "adviser_isolation_unproven", "eligible": False})
        with self.assertRaisesRegex(factory.FactoryError, "adviser_isolation_unproven"):
            factory.require_adviser_isolation()
        with tempfile.TemporaryDirectory() as directory:
            workspace = Path(directory)
            protected = workspace / "protected.txt"; protected.write_text("before", encoding="utf-8")
            paths = {"workspace": workspace, "artifact": workspace / "finding.json", "result": workspace / "result.json"}
            before = factory.protected_root_manifest(workspace, {paths["artifact"], paths["result"]})
            factory.guard_ambiguity_advice_transport(paths, {}, before, before)
            with self.assertRaises(factory.FactoryError):
                factory.guard_ambiguity_advice_transport(paths, {"GITHUB_TOKEN": "secret"}, before, before)
            protected.write_text("after", encoding="utf-8")
            after = factory.protected_root_manifest(workspace, {paths["artifact"], paths["result"]})
            with self.assertRaises(factory.FactoryError):
                factory.guard_ambiguity_advice_transport(paths, {}, before, after)

    def test_ambiguity_schema_is_nonbinding_only(self) -> None:
        schema = json.loads((ROOT / "packs/agentops-factory/assets/schemas/ambiguity-advice.v1.schema.json").read_text())
        valid = {"schema_version": "ambiguity-advice.v1", "request_id": "request", "context_id": "ctx", "finding": "fact", "nonbinding": True, "mutates_artifacts": False}
        validator = Draft202012Validator(schema)
        self.assertTrue(validator.is_valid(valid))
        for field, value in (("verdict", "PASS"), ("routing_mutation", True), ("mutates_artifacts", True)):
            candidate = deepcopy(valid); candidate[field] = value
            self.assertFalse(validator.is_valid(candidate))

    def test_clean_delivery_has_no_routable_model(self) -> None:
        class NoRuntimeCalls:
            def __getattr__(self, name: str):
                raise AssertionError(f"routine delivery touched runtime method {name}")

        self.assertIsNone(factory.route_ready_refinery(NoRuntimeCalls(), "rig", {"factory.refinery_bead": "delivery"}))
        command = next(action for action in factory.parser()._actions if action.dest == "command")
        self.assertEqual(set(command.choices), {"inspect-role-v2", "emit-role-v2", "doctor"})

    def test_v2_emit_uses_a_live_session_record_and_closes_adviser_before_start(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            workspace = Path(directory)
            intent = workspace / "intent.md"; intent.write_text("intent", encoding="utf-8")
            digest = hashlib.sha256(intent.read_bytes()).hexdigest()
            artifact = workspace / "graph.json"
            mayor_graph = graph(); mayor_graph["intent_digest"] = digest; mayor_graph["nodes"][0]["intent_digest"] = digest
            artifact.write_text(json.dumps(mayor_graph), encoding="utf-8")
            result = workspace / "result.json"
            request = {"schema_version": "factory-role-request.v2", "request_id": "request", "program_id": "program", "semantic_bead_id": "semantic", "workspace": str(workspace), "intent_source": str(intent), "intent_digest": digest, "subject_path": str(intent), "subject_digest": digest, "evidence_refs": [{"path": str(intent), "digest": digest}], "prior_context_id": None, "role": "mayor", "requested": factory.requested_role_policy("mayor", "claude"), "artifact_path": str(artifact), "result_path": str(result)}
            request_path = workspace / "request.json"; request_path.write_text(json.dumps(request), encoding="utf-8")
            session = {"template": "factory.mayor", "provider": "claude", "model": "claude-fable-5", "command": "claude --model claude-fable-5", "live_model_observed": True}
            with mock.patch.object(factory, "runtime_session", return_value=session), mock.patch.dict(factory.os.environ, {"GC_SESSION_ID": "mayor-context"}, clear=True), contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(factory.command_emit_role_v2(type("Args", (), {"request": str(request_path), "artifact": str(artifact)})()), 0)
            response = json.loads(result.read_text(encoding="utf-8"))
            self.assertEqual(response["actual"]["model"], "claude-fable-5")
            advice = deepcopy(request); advice.update({"role": "ambiguity_advice", "prior_context_id": "mayor-context", "requested": factory.requested_role_policy("ambiguity_advice", "claude"), "artifact_path": str(workspace / "advice.json"), "result_path": str(workspace / "advice-result.json")})
            Path(advice["artifact_path"]).write_text(json.dumps({"schema_version": "ambiguity-advice.v1", "request_id": "request", "context_id": "adviser", "finding": "fact", "nonbinding": True, "mutates_artifacts": False}), encoding="utf-8")
            advice_path = workspace / "advice-request.json"; advice_path.write_text(json.dumps(advice), encoding="utf-8")
            with mock.patch.object(factory, "runtime_session", side_effect=AssertionError("must not start Fable")), mock.patch.dict(factory.os.environ, {"GC_SESSION_ID": "adviser"}, clear=True), self.assertRaisesRegex(factory.FactoryError, "adviser_isolation_unproven"):
                factory.command_emit_role_v2(type("Args", (), {"request": str(advice_path), "artifact": advice["artifact_path"]})())


if __name__ == "__main__":
    unittest.main()
