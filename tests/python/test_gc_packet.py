from __future__ import annotations

import contextlib
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import sys
import tempfile
import types
import unittest
from unittest import mock


sys.dont_write_bytecode = True


ROOT = Path(__file__).resolve().parents[2]
MODULE_PATH = ROOT / "packs" / "agentops-executor" / "assets" / "scripts" / "packet.py"
SPEC = importlib.util.spec_from_file_location("agentops_gc_packet", MODULE_PATH)
assert SPEC and SPEC.loader
packet_module = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(packet_module)


class PacketFixture:
    def __init__(self, root: Path, role: str = "implement", provider: str = "codex") -> None:
        self.root = root
        self.intent = root / "intent.md"
        self.intent.write_text("Acceptance: the bounded behavior is present.\n", encoding="utf-8")
        self.evidence = root / ".gc" / "agentops" / f"packet-{role}"
        self.evidence.mkdir(parents=True)
        self.result = self.evidence / "agent-response.json"
        self.packet_path = self.evidence / "packet.json"
        self.packet = {
            "schema_version": "gc-execution-envelope.v1",
            "packet_id": f"packet-{role}",
            "role": role,
            "provider": provider,
            "intent_source": str(self.intent),
            "intent_digest": hashlib.sha256(self.intent.read_bytes()).hexdigest(),
            "workspace": str(root),
            "subject": {"includes": ["."], "excludes": []},
            "write_scope": ["src"] if role == "implement" else [],
            "evidence_dir": str(self.evidence),
            "result_path": str(self.result),
        }
        if role == "validate":
            baseline = self.evidence / "baseline.json"
            subject = self.evidence / "subject.json"
            scope_receipt = self.evidence / "scope-receipt.json"
            declaration = {
                "declared_roots": ["."],
                "exclusions": sorted(set(packet_module.COMMON_EXCLUDES)),
            }
            baseline.write_text(json.dumps(declaration) + "\n", encoding="utf-8")
            subject.write_text(json.dumps(declaration) + "\n", encoding="utf-8")
            receipt = packet_module.make_scope_receipt(
                {"packet_id": "packet-implement", "role": "implement", "write_scope": ["src"]},
                [],
            )
            scope_receipt.write_text(json.dumps(receipt) + "\n", encoding="utf-8")
            self.packet.update(
                {
                    "baseline_manifest": str(baseline),
                    "subject_manifest": str(subject),
                    "scope_receipt": str(scope_receipt),
                    "author_context_id": "gc-author-session",
                }
            )
        self.write()

    def write(self) -> None:
        self.packet_path.write_text(json.dumps(self.packet, indent=2) + "\n", encoding="utf-8")


class PacketContractTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name).resolve()

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def test_valid_implement_packet_binds_exact_intent_and_paths(self) -> None:
        fixture = PacketFixture(self.root)
        value, paths = packet_module.validate_envelope(fixture.packet_path)
        self.assertEqual(value["role"], "implement")
        self.assertEqual(value["provider"], "codex")
        self.assertEqual(paths["workspace"], self.root)
        self.assertEqual(value["intent_digest"], hashlib.sha256(fixture.intent.read_bytes()).hexdigest())
        self.assertIn(".claude", packet_module.COMMON_EXCLUDES)

    def test_codex_protected_agents_directory_is_not_a_packet_evidence_plane(self) -> None:
        fixture = PacketFixture(self.root)
        protected = self.root / ".agents" / "ao" / "gc" / fixture.packet["packet_id"]
        protected.mkdir(parents=True)
        fixture.packet["evidence_dir"] = str(protected)
        fixture.packet["result_path"] = str(protected / "agent-response.json")
        fixture.packet_path = protected / "packet.json"
        fixture.write()
        with self.assertRaisesRegex(packet_module.PacketError, "canonical packet directory"):
            packet_module.validate_envelope(fixture.packet_path)

    def test_nested_gc_session_metadata_is_excluded_from_subject_manifests(self) -> None:
        fixture = PacketFixture(self.root)
        nested = self.root / "q1-abc-agentops-implement-packet" / ".gc"
        nested.mkdir(parents=True)
        (nested / "settings.json").write_text("{}\n", encoding="utf-8")
        validate_tool = packet_module.load_validate_module()
        manifest = validate_tool.build_manifest(
            self.root,
            fixture.packet["subject"]["includes"],
            packet_module.COMMON_EXCLUDES,
        )
        paths = {entry["path"] for entry in manifest["entries"]}
        self.assertNotIn("q1-abc-agentops-implement-packet/.gc/settings.json", paths)
        self.assertIn("**/.gc", manifest["exclusions"])

    def test_unknown_lifecycle_field_fails_before_dispatch(self) -> None:
        fixture = PacketFixture(self.root)
        fixture.packet["retry"] = {"max_attempts": 3}
        fixture.write()
        dispatched = mock.Mock()
        args = types.SimpleNamespace(
            packet=str(fixture.packet_path), rig="agentops", binding="agentops", timeout=1.0, json=False
        )
        with mock.patch.object(packet_module, "prepare_packet_transport", dispatched), contextlib.redirect_stdout(io.StringIO()) as output:
            result = packet_module.command_run(args)
        self.assertEqual(result, 2)
        dispatched.assert_not_called()
        payload = json.loads(output.getvalue())
        self.assertEqual(payload["error"]["code"], "invalid_envelope")
        self.assertNotIn("verdict", payload)

    def test_intent_digest_mismatch_fails_closed(self) -> None:
        fixture = PacketFixture(self.root)
        fixture.packet["intent_digest"] = "0" * 64
        fixture.write()
        with self.assertRaisesRegex(packet_module.PacketError, "intent digest mismatch"):
            packet_module.validate_envelope(fixture.packet_path)

    def test_provider_is_required_and_closed_to_codex_or_claude(self) -> None:
        fixture = PacketFixture(self.root)
        fixture.packet["provider"] = "gemini"
        fixture.write()
        with self.assertRaisesRegex(packet_module.PacketError, "provider must be codex or claude"):
            packet_module.validate_envelope(fixture.packet_path)

    def test_claude_packet_prepares_deterministic_bead_and_routes_explicit_role(self) -> None:
        fixture = PacketFixture(self.root, provider="claude")
        packet, paths = packet_module.validate_envelope(fixture.packet_path)
        packet_digest = hashlib.sha256(fixture.packet_path.read_bytes()).hexdigest()
        bead_id = f"ao-pkt-{packet_digest[:16]}"
        target = "agentops/agentops.implementer-claude"
        identity = {
            "agentops.packet_id": packet["packet_id"],
            "agentops.packet_digest": packet_digest,
            "agentops.intent_digest": packet["intent_digest"],
            "agentops.target": target,
            "agentops.result_path": str(paths["result"]),
        }
        created = {
            "id": bead_id,
            "title": f"AgentOps implement packet {packet['packet_id']}",
            "description": packet_module.packet_transport_description(packet, paths),
            "status": "open",
            "metadata": identity,
        }
        with (
            mock.patch.dict(os.environ, {"GC_CITY_PATH": "/tmp/city"}, clear=False),
            mock.patch.object(packet_module, "gc_binary", return_value="/tmp/gc"),
            mock.patch.object(packet_module, "transport_record", side_effect=[None, created]),
            mock.patch.object(
                packet_module,
                "require_process",
                side_effect=[
                    json.dumps({
                        "ok": True,
                        "rigs": [{"name": "agentops", "path": str(self.root), "prefix": "ao"}],
                    }),
                    json.dumps(created),
                    json.dumps({"success": True, "bead_id": bead_id}),
                ],
            ) as run,
        ):
            bead, prepared_target = packet_module.prepare_packet_transport(
                packet, paths, "agentops", "agentops",
            )
            packet_module.ensure_transport_routed("agentops", bead, prepared_target)
        self.assertEqual(bead, bead_id)
        self.assertEqual(prepared_target, target)
        description = run.call_args_list[1].args[0][run.call_args_list[1].args[0].index("--description") + 1]
        self.assertIn("execution_provider=claude", description)
        self.assertIn(f"adapter_path={packet_module.Path(packet_module.__file__).resolve()}", description)
        self.assertEqual(run.call_args_list[2].args[0][-4], bead_id)

    def test_already_routed_transport_is_not_slung_twice(self) -> None:
        target = "agentops/agentops.implementer"
        record = {
            "id": "ao-pkt-existing",
            "status": "in_progress",
            "metadata": {"gc.routed_to": target},
        }
        with (
            mock.patch.dict(os.environ, {"GC_CITY_PATH": "/tmp/city"}, clear=False),
            mock.patch.object(packet_module, "transport_record", return_value=record),
            mock.patch.object(packet_module, "require_process") as run,
        ):
            packet_module.ensure_transport_routed("agentops", record["id"], target)
        run.assert_not_called()

    def test_packet_workspace_must_equal_selected_rig_root(self) -> None:
        fixture = PacketFixture(self.root)
        packet, paths = packet_module.validate_envelope(fixture.packet_path)
        other_root = self.root.parent / "other-rig"
        with (
            mock.patch.dict(os.environ, {"GC_CITY_PATH": "/tmp/city"}, clear=False),
            mock.patch.object(packet_module, "gc_binary", return_value="/tmp/gc"),
            mock.patch.object(
                packet_module,
                "require_process",
                return_value=json.dumps({
                    "ok": True,
                    "rigs": [{"name": "agentops", "path": str(other_root), "prefix": "ao"}],
                }),
            ) as run,
        ):
            with self.assertRaisesRegex(packet_module.PacketError, "workspace must equal configured rig root"):
                packet_module.prepare_packet_transport(packet, paths, "agentops", "agentops")
        self.assertEqual(run.call_count, 1)

    def test_runtime_provider_must_match_packet_provider(self) -> None:
        fixture = PacketFixture(self.root, provider="claude")
        packet, paths = packet_module.validate_envelope(fixture.packet_path)
        artifact = fixture.evidence / "receipt.txt"
        artifact.write_text("ok\n", encoding="utf-8")
        response = {
            "schema_version": "gc-agent-response.v1",
            "packet_id": packet["packet_id"],
            "role": "implement",
            "outcome": "candidate",
            "transport_bead_id": "ao-transport-provider",
            "session_context_id": "gc-session-provider",
            "session_name": "impl-claude-1",
            "template": "agentops.implementer-claude",
            "artifacts": [{"path": str(artifact), "sha256": hashlib.sha256(artifact.read_bytes()).hexdigest()}],
            "message": "",
        }
        transport = {"id": "ao-transport-provider", "status": "closed", "assignee": "impl-claude-1"}
        session = {
            "id": "gc-session-provider",
            "session_name": "impl-claude-1",
            "template": "agentops.implementer-claude",
            "provider": "codex",
        }
        with self.assertRaisesRegex(packet_module.PacketError, "session provider"):
            packet_module.validate_agent_response(
                packet, paths, response, "ao-transport-provider", transport, session,
                "agentops.implementer-claude",
            )

    def test_emit_binds_gc_session_identity_and_artifact_digest(self) -> None:
        fixture = PacketFixture(self.root)
        artifact = fixture.evidence / "receipt.txt"
        artifact.write_text("check exited 0\n", encoding="utf-8")
        args = types.SimpleNamespace(
            packet=str(fixture.packet_path),
            bead="ao-transport-1",
            outcome="candidate",
            artifact=[str(artifact)],
            message="handled once",
        )
        with mock.patch.dict(os.environ, {
            "GC_SESSION_ID": "gc-session-impl",
            "GC_SESSION_NAME": "impl-1",
            "GC_PROVIDER": "codex",
            "GC_TEMPLATE": "agentops.implementer",
        }, clear=False):
            with contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(packet_module.command_emit(args), 0)
        response = json.loads(fixture.result.read_text(encoding="utf-8"))
        self.assertEqual(response["session_context_id"], "gc-session-impl")
        self.assertEqual(response["transport_bead_id"], "ao-transport-1")
        self.assertEqual(response["artifacts"][0]["sha256"], hashlib.sha256(artifact.read_bytes()).hexdigest())

    def test_validator_context_must_differ_from_author(self) -> None:
        fixture = PacketFixture(self.root, "validate")
        args = types.SimpleNamespace(
            packet=str(fixture.packet_path),
            bead="ao-transport-2",
            outcome="evidence",
            artifact=[],
            message="",
        )
        with mock.patch.dict(os.environ, {"GC_SESSION_ID": "gc-author-session"}, clear=False):
            with self.assertRaisesRegex(packet_module.PacketError, "collide"):
                packet_module.command_emit(args)

    def test_validate_packet_requires_empty_write_scope(self) -> None:
        fixture = PacketFixture(self.root, "validate")
        fixture.packet["write_scope"] = ["src"]
        fixture.write()
        with self.assertRaisesRegex(packet_module.PacketError, "write_scope must be empty"):
            packet_module.validate_envelope(fixture.packet_path)

    def test_validate_packet_subject_must_match_supplied_manifests(self) -> None:
        fixture = PacketFixture(self.root, "validate")
        fixture.packet["subject"] = {"includes": ["README.md"], "excludes": []}
        fixture.write()
        with self.assertRaisesRegex(packet_module.PacketError, "declared_roots"):
            packet_module.validate_envelope(fixture.packet_path)

    def test_direct_response_cannot_smuggle_semantic_verdict_or_malformed_artifact(self) -> None:
        fixture = PacketFixture(self.root)
        packet, paths = packet_module.validate_envelope(fixture.packet_path)
        response = {
            "schema_version": "gc-agent-response.v1",
            "packet_id": packet["packet_id"],
            "role": "implement",
            "outcome": "candidate",
            "transport_bead_id": "ao-transport-4",
            "session_context_id": "gc-session-impl",
            "session_name": "impl-1",
            "template": "agentops.implementer",
            "artifacts": [{}],
            "message": "",
            "verdict": "PASS",
        }
        transport = {"id": "ao-transport-4", "status": "closed", "assignee": "impl-1"}
        session = {
            "id": "gc-session-impl",
            "session_name": "impl-1",
            "template": "agentops.implementer",
            "provider": "codex",
        }
        with self.assertRaisesRegex(packet_module.PacketError, "unknown=\\['verdict'\\]"):
            packet_module.validate_agent_response(
                packet, paths, response, "ao-transport-4", transport, session,
                "agentops.implementer",
            )
        response.pop("verdict")
        with self.assertRaisesRegex(packet_module.PacketError, "exactly path and sha256"):
            packet_module.validate_agent_response(
                packet, paths, response, "ao-transport-4", transport, session,
                "agentops.implementer",
            )

    def test_successful_response_requires_real_artifact(self) -> None:
        fixture = PacketFixture(self.root)
        args = types.SimpleNamespace(
            packet=str(fixture.packet_path),
            bead="ao-transport-3",
            outcome="candidate",
            artifact=[],
            message="",
        )
        with mock.patch.dict(os.environ, {
            "GC_SESSION_ID": "gc-session-impl",
            "GC_PROVIDER": "codex",
            "GC_TEMPLATE": "agentops.implementer",
        }, clear=False):
            with self.assertRaisesRegex(packet_module.PacketError, "at least one --artifact"):
                packet_module.command_emit(args)

    def test_validator_emit_requires_runtime_bound_verdict_artifact(self) -> None:
        fixture = PacketFixture(self.root, "validate")
        validate_tool = packet_module.load_validate_module()
        exclusions = sorted(set(packet_module.COMMON_EXCLUDES))
        baseline = validate_tool.build_manifest(self.root, ["."], exclusions)
        subject = validate_tool.build_manifest(self.root, ["."], exclusions, baseline)
        baseline_path = Path(fixture.packet["baseline_manifest"])
        subject_path = Path(fixture.packet["subject_manifest"])
        baseline_path.write_text(json.dumps(baseline), encoding="utf-8")
        subject_path.write_text(json.dumps(subject), encoding="utf-8")
        draft = {
            "acceptance_digest": "0" * 64,
            "subject_manifest_digest": "0" * 64,
            "author_context_id": "model-author",
            "validator_context_id": "model-validator",
            "freshness_attestation": {"source": "caller", "attester_identity": "model"},
            "verdict": "PASS",
            "criteria": [{"id": "c1", "result": "PASS", "evidence_refs": ["e1"]}],
            "findings": [],
            "evidence_refs": ["e1"],
            "checked": ["c1"],
            "not_checked": [],
            "validated_at": "2026-07-16T00:00:00Z",
        }
        artifact, verdict_path, _ = validate_tool.store_verdict(
            draft,
            fixture.evidence / "verdicts",
            fixture.intent.read_bytes(),
            subject,
            "gc-author-session",
            "NOT_PROVEN",
            "gc-validator-session",
            "runtime",
            "gc-validator-session",
        )
        self.assertEqual(artifact["validator_context_id"], "gc-validator-session")
        args = types.SimpleNamespace(
            packet=str(fixture.packet_path),
            bead="ao-transport-5",
            outcome="evidence",
            artifact=[str(verdict_path)],
            message="bound verdict",
        )
        with mock.patch.dict(os.environ, {
            "GC_SESSION_ID": "gc-validator-session",
            "GC_PROVIDER": "codex",
            "GC_TEMPLATE": "agentops.validator",
        }, clear=False):
            with contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(packet_module.command_emit(args), 0)

    def test_scope_receipt_is_factual_and_never_a_verdict(self) -> None:
        fixture = PacketFixture(self.root)
        receipt = packet_module.make_scope_receipt(fixture.packet, ["src/new.py", "README.md"])
        self.assertEqual(receipt["status"], "FAIL")
        self.assertEqual(receipt["outside_scope"], ["README.md"])
        self.assertNotIn("verdict", receipt)

    def test_run_reconciles_cached_runtime_result_without_reslinging(self) -> None:
        fixture = PacketFixture(self.root)
        packet, paths = packet_module.validate_envelope(fixture.packet_path)
        baseline_path = fixture.evidence / "runtime-baseline-manifest.json"
        baseline = packet_module.build_manifest(packet, paths, baseline_path)
        (self.root / "src").mkdir()
        (self.root / "src" / "candidate.txt").write_text("candidate\n", encoding="utf-8")
        subject = packet_module.build_manifest(
            packet, paths, fixture.evidence / "runtime-subject-manifest.json", baseline_path,
        )
        changes = packet_module.changed_paths(baseline, subject)
        receipt = packet_module.make_scope_receipt(packet, changes)
        artifact = fixture.evidence / "worker-receipt.json"
        artifact.write_text("{}\n", encoding="utf-8")
        response = {
            "schema_version": "gc-agent-response.v1",
            "packet_id": packet["packet_id"],
            "role": "implement",
            "outcome": "candidate",
            "transport_bead_id": "ao-recover",
            "session_context_id": "gc-recover-session",
            "session_name": "implementer-1",
            "template": "agentops.implementer",
            "artifacts": [{
                "path": str(artifact),
                "sha256": hashlib.sha256(artifact.read_bytes()).hexdigest(),
            }],
            "message": "",
        }
        fixture.result.write_text(json.dumps(response) + "\n", encoding="utf-8")
        runtime = packet_module.success_result(
            packet, paths, response, "ao-recover", "agentops/agentops.implementer",
            {
                "packet_digest": hashlib.sha256(fixture.packet_path.read_bytes()).hexdigest(),
                "actual_changed_paths": changes,
                "subject_manifest_digest": subject["canonical_manifest_digest"],
                "scope_status": receipt["status"],
            },
        )
        (fixture.evidence / "runtime-result.json").write_text(
            json.dumps(runtime) + "\n", encoding="utf-8",
        )
        transport = {"id": "ao-recover", "status": "closed", "assignee": "implementer-1"}
        session = {
            "id": "gc-recover-session", "session_name": "implementer-1",
            "template": "agentops/agentops.implementer", "provider": "codex",
        }
        # The target is the exact runtime template for packet validation.
        response["template"] = "agentops/agentops.implementer"
        fixture.result.write_text(json.dumps(response) + "\n", encoding="utf-8")
        runtime["agent_response"] = response
        (fixture.evidence / "runtime-result.json").write_text(json.dumps(runtime) + "\n", encoding="utf-8")
        args = types.SimpleNamespace(
            packet=str(fixture.packet_path), rig="agentops", binding="agentops",
            timeout=1.0, json=False,
        )
        with (
            mock.patch.object(packet_module, "transport_record", return_value=transport),
            mock.patch.object(packet_module, "runtime_session", return_value=session),
            mock.patch.object(packet_module, "prepare_packet_transport") as prepare,
            contextlib.redirect_stdout(io.StringIO()) as stdout,
        ):
            self.assertEqual(packet_module.command_run(args), 0)
        prepare.assert_not_called()
        self.assertTrue(json.loads(stdout.getvalue())["ok"])

    def test_run_resumes_prepared_transport_without_creating_another_bead(self) -> None:
        fixture = PacketFixture(self.root)
        packet, paths = packet_module.validate_envelope(fixture.packet_path)
        baseline_path = fixture.evidence / "runtime-baseline-manifest.json"
        packet_module.build_manifest(packet, paths, baseline_path)
        target = "agentops/agentops.implementer"
        bead_id = "ao-pkt-prepared"
        state_path = fixture.evidence / "runtime-transport.json"
        state_path.write_text(json.dumps({
            "schema_version": "gc-packet-transport.v1",
            "packet_id": packet["packet_id"],
            "packet_digest": hashlib.sha256(fixture.packet_path.read_bytes()).hexdigest(),
            "intent_digest": packet["intent_digest"],
            "rig": "agentops",
            "binding": "agentops",
            "bead_id": bead_id,
            "target": target,
            "dispatch_phase": "prepared",
            "manifest_file_digests": {
                str(baseline_path): hashlib.sha256(baseline_path.read_bytes()).hexdigest(),
            },
        }) + "\n", encoding="utf-8")
        artifact = fixture.evidence / "worker-receipt.txt"
        artifact.write_text("candidate\n", encoding="utf-8")
        response = {
            "schema_version": "gc-agent-response.v1",
            "packet_id": packet["packet_id"],
            "role": "implement",
            "outcome": "candidate",
            "transport_bead_id": bead_id,
            "session_context_id": "gc-resumed-session",
            "session_name": "implementer-resumed",
            "template": target,
            "artifacts": [{
                "path": str(artifact),
                "sha256": hashlib.sha256(artifact.read_bytes()).hexdigest(),
            }],
            "message": "",
        }
        fixture.result.write_text(json.dumps(response) + "\n", encoding="utf-8")
        transport = {"id": bead_id, "status": "closed", "assignee": "implementer-resumed"}
        session = {
            "id": "gc-resumed-session", "session_name": "implementer-resumed",
            "template": target, "provider": "codex",
        }
        args = types.SimpleNamespace(
            packet=str(fixture.packet_path), rig="agentops", binding="agentops",
            timeout=1.0, json=False,
        )
        with (
            mock.patch.object(packet_module, "prepare_packet_transport") as prepare,
            mock.patch.object(packet_module, "ensure_transport_routed") as ensure,
            mock.patch.object(packet_module, "wait_for_response", return_value=(response, transport)),
            mock.patch.object(packet_module, "runtime_session", return_value=session),
            contextlib.redirect_stdout(io.StringIO()) as stdout,
        ):
            self.assertEqual(packet_module.command_run(args), 0)
        prepare.assert_not_called()
        ensure.assert_called_once_with("agentops", bead_id, target)
        persisted = json.loads(state_path.read_text(encoding="utf-8"))
        self.assertEqual(persisted["dispatch_phase"], "routed")
        self.assertTrue(json.loads(stdout.getvalue())["ok"])


if __name__ == "__main__":
    unittest.main()
