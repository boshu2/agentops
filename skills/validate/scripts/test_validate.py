from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

import jsonschema


SPEC = importlib.util.spec_from_file_location("validate_tool", Path(__file__).with_name("validate.py"))
tool = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
SPEC.loader.exec_module(tool)


class ValidateV2Tests(unittest.TestCase):
    def draft(self):
        return {
            "acceptance_digest": "a" * 64,
            "subject_manifest_digest": "b" * 64,
            "author_context_id": "author",
            "validator_context_id": "validator",
            "freshness_attestation": {"source": "runtime", "attester_identity": "runtime-1"},
            "verdict": "PASS",
            "criteria": [{"id": "c1", "result": "PASS", "evidence_refs": ["e1"]}],
            "findings": [],
            "evidence_refs": ["e1"],
            "checked": ["c1"],
            "not_checked": [],
            "validated_at": "2026-07-14T00:00:00Z",
        }

    def assert_schema_valid(self, artifact):
        schema = json.loads((Path(__file__).parents[3] / "schemas" / "verdict.v2.schema.json").read_text())
        jsonschema.Draft202012Validator(schema).validate(artifact)

    def test_manifest_is_content_addressed_and_detects_mutation(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            (root / "bin").mkdir()
            subject = root / "bin" / "tool"
            subject.write_text("one", encoding="utf-8")
            subject.chmod(0o755)
            manifest = tool.build_manifest(root, ["bin"], [])
            self.assertTrue(tool.verify_manifest(manifest, root, None)[0])
            subject.write_text("two", encoding="utf-8")
            self.assertFalse(tool.verify_manifest(manifest, root, None)[0])

    def test_git_metadata_is_not_identity_bearing(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            (root / "value").write_text("same", encoding="utf-8")
            first = tool.build_manifest(root, ["."], [], git_metadata={"commit": "one"})
            second = tool.build_manifest(root, ["."], [], git_metadata={"commit": "two"})
            self.assertEqual(first["canonical_manifest_digest"], second["canonical_manifest_digest"])
            self.assertNotEqual(first["git_metadata"], second["git_metadata"])
            self.assertTrue(tool.verify_manifest(first, root, None)[0])
            self.assertTrue(tool.verify_manifest(second, root, None)[0])

    def test_symlink_and_deletion_identity(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            (root / "target").write_text("x", encoding="utf-8")
            (root / "link").symlink_to("target")
            base = tool.build_manifest(root, ["."], [])
            (root / "target").unlink()
            current = tool.build_manifest(root, ["."], [], base)
            kinds = {entry["path"]: entry["kind"] for entry in current["entries"]}
            self.assertEqual(kinds["link"], "symlink")
            self.assertEqual(kinds["target"], "deletion")

    def test_scope_fail_and_not_proven(self):
        plan = {
            "schema_version": "plan-packet.v1",
            "acceptance_digest": "a" * 64,
            "write_scope": {"include": ["src/**"], "exclude": ["src/generated/**"]},
        }
        candidate = {
            "plan_packet_digest": tool.plan_digest(plan),
            "acceptance_digest": "a" * 64,
            "changed_path_coverage_complete": True,
            "actual_changed_paths": ["src/main.go", "docs/readme.md"],
        }
        self.assertEqual(tool.scope_result(plan, candidate)["result"], "FAIL")
        candidate["changed_path_coverage_complete"] = False
        self.assertEqual(tool.scope_result(plan, candidate)["result"], "NOT_PROVEN")

    def test_verdict_identity_floor_and_idempotence(self):
        with tempfile.TemporaryDirectory() as raw:
            draft = self.draft()
            draft["author_context_id"] = "same"
            draft["validator_context_id"] = "same"
            first, path, existed = tool.store_verdict(draft, Path(raw))
            self.assertEqual(first["verdict"], "NOT_PROVEN")
            self.assert_schema_valid(first)
            self.assertFalse(existed)
            second, second_path, existed = tool.store_verdict(draft, Path(raw))
            self.assertTrue(existed)
            self.assertEqual(path, second_path)
            self.assertEqual(json.loads(path.read_text())["artifact_digest"], first["artifact_digest"])

    def test_missing_identity_or_attestation_is_schema_valid_not_proven(self):
        for missing in ("author_context_id", "validator_context_id", "freshness_attestation"):
            with self.subTest(missing=missing), tempfile.TemporaryDirectory() as raw:
                draft = self.draft()
                draft.pop(missing)
                artifact, _path, _existed = tool.store_verdict(draft, Path(raw))
                self.assertEqual(artifact["verdict"], "NOT_PROVEN")
                self.assert_schema_valid(artifact)

    def test_pass_with_failed_criterion_is_downgraded(self):
        with tempfile.TemporaryDirectory() as raw:
            draft = self.draft()
            draft["criteria"][0]["result"] = "FAIL"
            artifact, _path, _existed = tool.store_verdict(draft, Path(raw))
            self.assertEqual(artifact["verdict"], "NOT_PROVEN")
            self.assert_schema_valid(artifact)

    def test_corrupt_existing_digest_yields_new_not_proven_artifact(self):
        with tempfile.TemporaryDirectory() as raw:
            destination = Path(raw)
            draft = self.draft()
            artifact, path, _ = tool.store_verdict(draft, destination)
            path.write_text("corrupt\n", encoding="utf-8")
            replacement, replacement_path, existed = tool.store_verdict(draft, destination)
            self.assertEqual(replacement["verdict"], "NOT_PROVEN")
            self.assertNotEqual(replacement["artifact_digest"], artifact["artifact_digest"])
            self.assertNotEqual(replacement_path, path)
            self.assertFalse(existed)
            self.assert_schema_valid(replacement)

    def test_incomplete_draft_is_rejected_without_writing(self):
        with tempfile.TemporaryDirectory() as raw:
            with self.assertRaisesRegex(tool.ContractError, "missing required fields"):
                tool.store_verdict({"verdict": "FAIL"}, Path(raw))
            self.assertEqual(list(Path(raw).iterdir()), [])

    def test_unknown_field_is_rejected_without_writing(self):
        with tempfile.TemporaryDirectory() as raw:
            draft = self.draft()
            draft["next_action"] = "repair"
            with self.assertRaisesRegex(tool.ContractError, "unknown fields"):
                tool.store_verdict(draft, Path(raw))
            self.assertEqual(list(Path(raw).iterdir()), [])


if __name__ == "__main__":
    unittest.main()
