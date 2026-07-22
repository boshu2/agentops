from __future__ import annotations

import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
from types import SimpleNamespace
import unittest


ROOT = Path(__file__).resolve().parents[2]
FEEDER = ROOT / "packs/agentops-factory/assets/scripts/factory_feeder.py"
PROGRAM_START = ROOT / "packs/agentops-factory/assets/scripts/program_start.py"


def canonical(value: object) -> bytes:
    return json.dumps(value, ensure_ascii=True, separators=(",", ":"), sort_keys=True).encode() + b"\n"


def manifest_digest(value: dict[str, object]) -> str:
    identity = {key: item for key, item in value.items() if key not in {"canonical_manifest_digest", "git_metadata"}}
    return hashlib.sha256(json.dumps(identity, ensure_ascii=False, separators=(",", ":"), sort_keys=True).encode()).hexdigest()


class StubPacketModule:
    COMMON_EXCLUDES = [".gc"]

    def __init__(self, feeder, baseline: dict[str, object], subject: dict[str, object]):
        self.feeder, self.baseline, self.subject = feeder, baseline, subject

    def load_object(self, path: Path, _label: str):
        return json.loads(path.read_bytes())

    def build_manifest(self, _packet, _paths, output: Path, _base=None):
        self.feeder.atomic_write(output, self.subject)
        return self.subject

    @staticmethod
    def changed_paths(_baseline, _subject):
        return ["owned.txt"]

    @staticmethod
    def path_matches(path: str, pattern: str):
        return path == pattern or path.startswith(pattern.rstrip("/") + "/")

    @staticmethod
    def make_scope_receipt(packet, changes, *, workspace_changes, workspace_base_sha):
        return {"schema_version": "gc-scope-receipt.v2", "packet_id": packet["packet_id"], "role": "implement", "status": "PASS", "write_scope": packet["write_scope"], "actual_changed_paths": workspace_changes, "outside_scope": [], "workspace_base_sha": workspace_base_sha, "subject_changed_paths": changes, "outside_subject": []}

    @staticmethod
    def success_result(packet, _paths, response, bead_id, target, evidence):
        return {"schema_version": "gc-execution-result.v1", "ok": True, "packet_id": packet["packet_id"], "role": packet["role"], "provider": packet["provider"], "outcome": response["outcome"], "transport": {"bead_id": bead_id, "target": target}, "agent_response": response, "runtime_evidence": evidence}

    @staticmethod
    def validate_envelope(_path, allow_existing_result=False):
        return {}, {}


class SemanticTransitionTests(unittest.TestCase):
    def feeder(self):
        spec = importlib.util.spec_from_file_location("gc33_semantic_feeder", FEEDER)
        assert spec and spec.loader
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def program_start(self):
        spec = importlib.util.spec_from_file_location("gc33_program_start", PROGRAM_START)
        assert spec and spec.loader
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def git(self, repository: Path, *args: str) -> str:
        return subprocess.check_output(["git", "-C", str(repository), *args], text=True).strip()

    def repository(self, root: Path) -> tuple[Path, str]:
        repository = root / "repo"
        repository.mkdir()
        subprocess.run(["git", "init", "-q", "-b", "main", str(repository)], check=True)
        subprocess.run(["git", "-C", str(repository), "config", "user.name", "GC Test"], check=True)
        subprocess.run(["git", "-C", str(repository), "config", "user.email", "gc@example.invalid"], check=True)
        (repository / "owned.txt").write_text("before\n", encoding="utf-8")
        subprocess.run(["git", "-C", str(repository), "add", "owned.txt"], check=True)
        subprocess.run(["git", "-C", str(repository), "commit", "-qm", "base"], check=True)
        (repository / ".gc").mkdir()
        return repository, self.git(repository, "rev-parse", "HEAD")

    def test_candidate_freeze_is_deterministic_without_moving_head_or_index(self) -> None:
        feeder = self.feeder()
        with tempfile.TemporaryDirectory() as directory:
            repository, base = self.repository(Path(directory))
            (repository / "owned.txt").write_text("after\n", encoding="utf-8")
            index = Path(self.git(repository, "rev-parse", "--path-format=absolute", "--git-path", "index"))
            before_index = index.read_bytes()
            binding = {"context": {"git_bin": "/usr/bin/git", "created_at": "2026-07-22T00:00:00Z"}, "graph": {"base_oid": base, "program_id": "program"}, "recorded": {"work_dir": str(repository)}, "node_id": "node"}
            first = feeder.freeze_git_candidate(binding, ["owned.txt"], "a" * 64)
            second = feeder.freeze_git_candidate(binding, ["owned.txt"], "a" * 64)
            self.assertEqual(first, second)
            self.assertEqual(self.git(repository, "rev-parse", "HEAD"), base)
            self.assertEqual(index.read_bytes(), before_index)
            self.assertEqual(self.git(repository, "status", "--porcelain"), "M owned.txt")

    def test_program_entry_snapshots_one_bead_once(self) -> None:
        program = self.program_start()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            repository = root / "repo"; repository.mkdir()
            bd = root / "bd"
            bd.write_text('#!/bin/sh\nprintf \'%s\\n\' \'[{"id":"age-1","title":"first","status":"open"}]\'\n', encoding="utf-8")
            bd.chmod(0o700)
            first = program.bead_intent(repository, bd, "age-1")
            before = first.read_bytes()
            self.assertEqual(json.loads(before)["bead"]["title"], "first")
            bd.write_text('#!/bin/sh\nprintf \'%s\\n\' \'[{"id":"age-1","title":"changed","status":"closed"}]\'\n', encoding="utf-8")
            second = program.bead_intent(repository, bd, "age-1")
            self.assertEqual(second, first)
            self.assertEqual(second.read_bytes(), before)

    def test_program_entry_and_feeder_reject_symlinked_trust_inputs(self) -> None:
        program = self.program_start()
        feeder = self.feeder()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            target = root / "target"
            target.write_text("trusted bytes\n", encoding="utf-8")
            link = root / "link"
            link.symlink_to(target)

            with self.assertRaisesRegex(ValueError, "must not be a symlink"):
                program.exact_file(str(link), "program input")
            with self.assertRaisesRegex(feeder.FeederError, "must not be a symlink"):
                feeder.regular_file(str(link), "feeder input")

    def test_implement_transition_freezes_changed_manifest_and_derives_validate_packet(self) -> None:
        feeder = self.feeder()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            repository, base = self.repository(root)
            evidence = repository / ".gc/agentops/program-node"
            evidence.mkdir(parents=True)
            intent = evidence / "intent"; intent.write_text("intent\n", encoding="utf-8")
            baseline = {"schema_version": "subject-manifest.v1", "declared_roots": ["owned.txt"], "exclusions": [".gc"], "entries": [{"path": "owned.txt", "kind": "file", "executable": False, "digest": hashlib.sha256(b"before\n").hexdigest()}]}
            baseline["canonical_manifest_digest"] = manifest_digest(baseline)
            subject = {"schema_version": "subject-manifest.v1", "declared_roots": ["owned.txt"], "exclusions": [".gc"], "base_manifest_digest": baseline["canonical_manifest_digest"], "entries": [{"path": "owned.txt", "kind": "file", "executable": False, "digest": hashlib.sha256(b"after\n").hexdigest()}]}
            subject["canonical_manifest_digest"] = manifest_digest(subject)
            (evidence / "runtime-baseline-manifest.json").write_bytes(canonical(baseline))
            (repository / "owned.txt").write_text("after\n", encoding="utf-8")
            packet = {"schema_version": "gc-execution-envelope.v1", "packet_id": "program-node", "role": "implement", "provider": "codex", "intent_source": str(intent), "intent_digest": hashlib.sha256(intent.read_bytes()).hexdigest(), "workspace": str(repository), "subject": {"includes": ["owned.txt"], "excludes": []}, "write_scope": ["owned.txt"], "evidence_dir": str(evidence), "result_path": str(evidence / "agent-response.json"), "factory_admission_receipt": str(root / "receipt.json"), "factory_node_id": "node"}
            packet_path = evidence / "implement-envelope.json"; packet_path.write_bytes(canonical(packet))
            (root / "receipt.json").write_bytes(b"{}\n")
            validate_path = evidence / "validate-envelope.json"
            module = StubPacketModule(feeder, baseline, subject)
            session = {"id": "author", "model": "gpt-5.6-terra", "reasoning": "high", "provider": "codex", "effort": "high", "fallback": {"allowed": False, "used": False, "reason": None}}
            binding = {"packet_module": module, "packet": packet, "packet_raw": packet_path.read_bytes(), "paths": {"workspace": repository, "evidence": evidence}, "response": {"outcome": "candidate"}, "response_raw": b"response", "session": session, "attempt": {"id": "attempt"}, "target": "agentops/agentops.implementer", "receipt": {"program_digest": "b" * 64}, "receipt_path": root / "receipt.json", "node": {"model": "terra", "reasoning": "high", "provider": "codex"}, "node_id": "node", "recorded": {"semantic_bead_id": "semantic", "work_dir": str(repository), "validate_packet": str(validate_path)}, "context": {"git_bin": "/usr/bin/git", "created_at": "2026-07-22T00:00:00Z"}, "graph": {"program_id": "program", "intent_digest": packet["intent_digest"], "base_oid": base}}
            result = feeder.implement_transition(binding)
            self.assertTrue(result["freeze_path"].is_file())
            freeze = result["freeze"]
            self.assertEqual(freeze["changed_paths"], ["owned.txt"])
            self.assertEqual(freeze["delivery_manifest_content_digest"], json.loads(Path(freeze["delivery_manifest"]).read_bytes())["canonical_manifest_digest"])
            validate = json.loads(validate_path.read_bytes())
            self.assertEqual(validate["role"], "validate")
            self.assertEqual(validate["write_scope"], [])
            self.assertEqual(validate["author_context_id"], "author")
            self.assertEqual(self.git(repository, "rev-parse", "HEAD"), base)
            feeder.git_changes = lambda *_args: []
            with self.assertRaises(feeder.PhaseFailure) as failure:
                feeder.implement_transition(binding)
            self.assertEqual(failure.exception.code, "not_built")
            self.assertEqual(str(failure.exception), "implementation produced no changed paths")

    def test_check_never_delivers_nonpass_and_not_built_stops_validate(self) -> None:
        feeder = self.feeder()
        with tempfile.TemporaryDirectory() as directory:
            pointer_path = Path(directory) / "pointer.json"
            pointer_path.write_bytes(canonical({"kind": "packet", "phase": "validate"}))
            binding = {"receipt": {}, "graph": {}, "context": {"bd_bin": "bd", "root": directory}}
            calls: list[str] = []
            feeder.packet_binding = lambda _pointer: binding
            feeder.validate_transition = lambda _binding: {"terminal": {"status": "FAIL"}}
            feeder.release_ready_admissions = lambda *_args: calls.append("release")
            feeder.drive_initial_delivery = lambda *_args: calls.append("delivery")
            self.assertEqual(feeder.check(SimpleNamespace(request=str(pointer_path))), 0)
            self.assertEqual(calls, ["release"])
            pointer_path.write_bytes(canonical({"kind": "packet", "phase": "implement"}))
            feeder.implement_transition = lambda _binding: (_ for _ in ()).throw(feeder.PhaseFailure("build", "failed"))
            feeder.not_built_terminal = lambda *_args: calls.append("not_built")
            with self.assertRaises(feeder.PhaseFailure):
                feeder.check(SimpleNamespace(request=str(pointer_path)))
            self.assertEqual(calls[-1], "not_built")

    def test_not_proven_terminal_survives_an_unreadable_candidate_freeze(self) -> None:
        feeder = self.feeder()
        binding = {
            "graph": {"program_id": "program", "intent_digest": "a" * 64},
            "receipt": {"program_digest": "b" * 64}, "node_id": "node",
            "recorded": {"semantic_bead_id": "semantic"},
            "packet": {"author_context_id": "author"}, "session": {"id": "validator"},
            "context": {"created_at": "2026-07-22T00:00:00Z"},
        }
        stored: list[dict[str, object]] = []
        feeder.candidate_freeze_for_validate = lambda _binding: (_ for _ in ()).throw(
            feeder.PhaseFailure("candidate_freeze", "missing freeze")
        )
        feeder.store_semantic_terminal = lambda _binding, terminal, _certificate: stored.append(terminal)
        terminal = feeder.not_proven_terminal(binding, feeder.PhaseFailure("candidate_freeze", "missing freeze"))
        feeder.validate_schema(terminal, "semantic-terminal.v1.schema.json", "semantic terminal")
        self.assertEqual(stored, [terminal])
        self.assertEqual(terminal["status"], "NOT_PROVEN")
        self.assertIsNone(terminal["candidate_freeze_ref"])
        self.assertEqual(terminal["author_context_id"], "author")
        self.assertEqual(terminal["validator_context_id"], "validator")

    def test_terminal_summary_binds_exact_pass_certificate_bytes(self) -> None:
        feeder = self.feeder()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            program_digest, intent_digest = "a" * 64, "b" * 64
            node, semantic = "node", "semantic"
            no_fallback = {"allowed": False, "used": False, "reason": None}
            author = {"context_id": "author", "requested_model": "terra", "requested_reasoning": "high", "requested_provider": "codex", "actual_model": "gpt-5.6-terra", "actual_reasoning": "high", "actual_provider": "codex", "actual_effort": "high", "fallback": no_fallback}
            validator = {"context_id": "validator", "requested_model": "sol", "requested_reasoning": "high", "requested_provider": "codex", "actual_model": "gpt-5.6-sol", "actual_reasoning": "high", "actual_provider": "codex", "actual_effort": "high", "fallback": no_fallback}
            certificate = {"schema_version": "admission-certificate.v2", "semantic_bead_id": semantic, "intent_digest": intent_digest, "verdict": "PASS", "candidate": {"commit": "c" * 40, "tree": "d" * 40, "content_digest": "e" * 64}, "store": {"identity": "beads:test", "digest": "f" * 64}, "changed_path_manifest": "0" * 64, "verdict_digest": "1" * 64, "evidence_digest": "2" * 64, "attestations": {"author": author, "validator": validator}, "delivery_group_id": "group", "prefix_safety": "safe"}
            certificate_path = root / ".gc/agentops/factory/evidence/admission-certificates" / program_digest / node / "certificate.json"
            certificate_path.parent.mkdir(parents=True)
            certificate_path.write_bytes(canonical(certificate))
            terminal = {"schema_version": "semantic-terminal.v1", "program_id": "program", "program_digest": program_digest, "node_id": node, "semantic_bead_id": semantic, "intent_digest": intent_digest, "status": "PASS", "candidate_freeze_ref": str(root / "freeze.json"), "candidate_freeze_digest": "3" * 64, "verdict_ref": str(root / "verdict.json"), "verdict_digest": "4" * 64, "author_context_id": "author", "validator_context_id": "validator", "error": None, "created_at": "2026-07-22T00:00:00Z"}
            terminal_path = root / ".gc/agentops/factory/evidence/semantic-terminals" / program_digest / node / "terminal.json"
            terminal_path.parent.mkdir(parents=True)
            terminal_path.write_bytes(canonical(terminal))
            summary = {"verdict": "PASS", "terminal_digest": hashlib.sha256(terminal_path.read_bytes()).hexdigest(), "certificate_digest": hashlib.sha256(certificate_path.read_bytes()).hexdigest()}
            self.assertEqual(feeder.verified_terminal_material(root, semantic, program_digest, node, intent_digest, str(terminal_path), summary), terminal)
            summary["certificate_digest"] = "9" * 64
            with self.assertRaises(feeder.PhaseFailure):
                feeder.verified_terminal_material(root, semantic, program_digest, node, intent_digest, str(terminal_path), summary)

    def test_pass_handoff_takes_two_bounded_steps_and_replays_without_advancing(self) -> None:
        feeder = self.feeder()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            native_path = root / "native.json"
            tools = {}
            for name in ("gc", "bd", "git", "gh", "bash", "agentops-gc-delivery"):
                path = root / name; path.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8"); path.chmod(0o700)
                tools[name] = {"path": str(path), "digest": hashlib.sha256(path.read_bytes()).hexdigest()}
            native = {"schema_version": "gc-delivery-native-context.v1", "rig_id": "agentops", "repository": "boshu2/agentops", "repository_dir": str(root), "worktree_root": str(root / "worktrees"), "beads_dir": str(root / ".beads"), "remote": "origin", "base_ref": "main", "successor_capability_digest": "a" * 64, "toolchain_lock_digest": "b" * 64, "toolchain_receipt_path": str(root / "toolchain.json"), "toolchain_receipt_digest": "c" * 64, "beads_representation": "B-successor-delivery-bead", "executables": tools, "check_only_gate_argv": [[tools["bash"]["path"], "scripts/check-gc-executor.sh"]]}
            native_path.write_bytes(canonical(native)); native_digest = hashlib.sha256(native_path.read_bytes()).hexdigest()
            delivery_root = root / ".gc/agentops/factory/evidence/delivery"; delivery_root.mkdir(parents=True)
            terminal_path = root / "terminal.json"; terminal_path.write_bytes(b"{}\n")
            certificate_path = root / "certificate.json"; certificate_path.write_bytes(b"{}\n")
            manifest_path = root / "manifest.json"; manifest_path.write_bytes(b"{}\n")
            binding = {"context": {"root": str(root), "repository_dir": str(root), "created_at": "2026-07-22T00:00:00Z", "bd_bin": tools["bd"]["path"], "delivery": {"native_context_path": str(native_path), "native_context_digest": native_digest, "evidence_root": str(delivery_root), "mode": "auto", "deadline_seconds": 86400}}, "graph": {"base_ref": "main", "base_oid": "d" * 40}, "receipt": {"program_digest": "e" * 64}, "recorded": {"semantic_bead_id": "semantic"}, "node_id": "node"}
            result = {"terminal": {"status": "PASS"}, "terminal_path": terminal_path, "certificate_path": certificate_path, "certificate_digest": "f" * 64, "freeze": {"candidate": {"commit": "1" * 40}, "delivery_manifest": str(manifest_path), "delivery_manifest_content_digest": "2" * 64}}
            identity = feeder.delivery_identity(binding, result)
            request_path = delivery_root / "handoffs" / identity["handoff_id"] / "epochs" / "000001" / "delivery-request.v1.json"
            request_path.parent.mkdir(parents=True)
            request_path.write_bytes(b"request\n")
            state: dict[str, object] = {"rows": [], "steps": 0}
            def step(_argv, _cwd):
                state["steps"] = int(state["steps"]) + 1
                if state["steps"] == 2:
                    record = {"schema_version": "gc.delivery.v1", "revision": 1, "handoff_id": identity["handoff_id"], "state": "queued", "publication": "pending", "ready_at": binding["context"]["created_at"], "deadline": identity["deadline"], "semantic_bead": "semantic", "terminal_ref": identity["terminal_ref"], "certificate": result["certificate_digest"], "mode": "auto", "rig": "agentops", "repository": "boshu2/agentops", "remote": "origin", "candidate": "1" * 40, "manifest": "2" * 64, "epoch": {"number": 1, "base_ref": "main", "base_oid": "d" * 40, "branch": "gc/delivery/" + identity["handoff_id"][:20]}}
                    reference = {"schema_version": "gc.delivery.request-ref.v1", "path": str(request_path.relative_to(delivery_root)), "digest": hashlib.sha256(request_path.read_bytes()).hexdigest()}
                    state["rows"] = [{"id": identity["delivery_bead_id"], "external_ref": identity["external_ref"], "metadata": {"gc.kind": "delivery", "gc.delivery.v1": canonical(record).decode().strip(), "gc.delivery_request": canonical(reference).decode().strip()}}]
                    return {"status": "successor_created", "effect": "beads.create"}
                return {"status": "prepared"}
            feeder.run_delivery_step = step
            feeder.all_beads = lambda *_args: list(state["rows"])
            receipt = feeder.drive_initial_delivery(binding, result)
            self.assertEqual(state["steps"], 2)
            self.assertEqual(receipt["delivery_bead_id"], identity["delivery_bead_id"])
            feeder.run_delivery_step = lambda *_args: self.fail("replay advanced an existing delivery")
            self.assertEqual(feeder.drive_initial_delivery(binding, result), receipt)


if __name__ == "__main__":
    unittest.main()
