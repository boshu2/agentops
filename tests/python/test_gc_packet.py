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
        with mock.patch.object(packet_module, "sling_packet", dispatched), contextlib.redirect_stdout(io.StringIO()) as output:
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

    def test_claude_packet_routes_to_explicit_claude_role(self) -> None:
        fixture = PacketFixture(self.root, provider="claude")
        packet, paths = packet_module.validate_envelope(fixture.packet_path)
        with (
            mock.patch.dict(os.environ, {"GC_CITY_PATH": "/tmp/city"}, clear=False),
            mock.patch.object(packet_module, "gc_binary", return_value="/tmp/gc"),
            mock.patch.object(
                packet_module,
                "require_process",
                side_effect=[
                    json.dumps({"ok": True, "rigs": [{"name": "agentops", "path": str(self.root)}]}),
                    '{"success":true,"bead_id":"ao-1"}',
                ],
            ) as run,
        ):
            bead, target = packet_module.sling_packet(packet, paths, "agentops", "agentops")
        self.assertEqual(bead, "ao-1")
        self.assertEqual(target, "agentops/agentops.implementer-claude")
        description = run.call_args_list[1].kwargs["input_text"]
        self.assertIn("execution_provider=claude", description)
        self.assertIn(f"adapter_path={packet_module.Path(packet_module.__file__).resolve()}", description)

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
                return_value=json.dumps({"ok": True, "rigs": [{"name": "agentops", "path": str(other_root)}]}),
            ) as run,
        ):
            with self.assertRaisesRegex(packet_module.PacketError, "workspace must equal configured rig root"):
                packet_module.sling_packet(packet, paths, "agentops", "agentops")
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


if __name__ == "__main__":
    unittest.main()
