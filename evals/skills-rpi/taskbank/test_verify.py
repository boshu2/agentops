"""Narrow verifier integrity tests; no runtime credentials or model calls."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

loader = importlib.util.spec_from_file_location("verify", Path(__file__).with_name("verify.py"))
verify = importlib.util.module_from_spec(loader)
loader.loader.exec_module(verify)


class VerifierIntegrityTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="verifier-integrity-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.task = Path(__file__).parent.parent / "tasks/input-scope"
        self.baseline = self.root / "baseline"
        self.candidate = self.root / "candidate"
        self.log = self.root / "log"
        self.log.mkdir()
        shutil.copytree(self.task / "environment/source", self.baseline)
        shutil.copytree(self.baseline, self.candidate)

    def grade(self):
        return verify.grade(self.baseline, self.candidate, self.task / "tests", self.log)

    def test_module_rewrite_is_rejected(self):
        (self.candidate / "go.mod").write_text("module bypass\n")
        with self.assertRaisesRegex(ValueError, "out-of-scope change: go.mod"):
            self.grade()

    def test_public_test_deletion_is_rejected(self):
        (self.candidate / "select_test.go").unlink()
        with self.assertRaisesRegex(ValueError, "out-of-scope change"):
            self.grade()

    def test_symlink_source_is_rejected(self):
        path = self.candidate / "select.go"
        path.unlink()
        path.symlink_to(self.baseline / "select.go")
        with self.assertRaisesRegex(ValueError, "symlink"):
            self.grade()

    def test_candidate_test_cannot_replace_evaluator_oracle(self):
        (self.candidate / "oracle_test.go").write_text("package inputscope\n")
        with self.assertRaisesRegex(ValueError, "endpoint oracle failed"):
            self.grade()

    def test_reset_preserves_pristine_source_across_failure_and_success(self):
        before = verify.snapshot(self.baseline)
        with self.assertRaisesRegex(ValueError, "endpoint oracle failed"):
            self.grade()
        self.assertEqual(verify.snapshot(self.baseline), before)
        shutil.copy2(self.task / "solution/select.go", self.candidate / "select.go")
        first = self.grade()
        second = self.grade()
        self.assertEqual(first, second)
        self.assertEqual(verify.snapshot(self.baseline), before)
        self.assertFalse((self.baseline / "oracle_test.go").exists())

    def test_recorded_receipts_bind_actual_candidate_source(self):
        task = self.task.parent / "validator-controls/environment/source"
        for name in ["a", "b"]:
            record = json.loads((task / f"receipts/candidate-{name}.json").read_text())
            source = (task / record["source_path"]).read_bytes()
            self.assertEqual(verify.hashlib.sha256(source).hexdigest(), record["source_sha256"])
            self.assertEqual(record["exit_code"], 0)
            self.assertIn("go version", record["runtime"])
        self.assertFalse((task / "receipts/candidate-c.json").exists())

    def test_case_grades_distinguish_false_acceptance_and_blocking(self):
        expected = {"a": "FAIL", "b": "PASS", "c": "NOT_PROVEN"}
        with self.assertRaises(verify.VerdictMismatch) as caught:
            verify.judge_verdicts(expected, {"a": "PASS", "b": "NOT_PROVEN", "c": "PASS"})
        results = caught.exception.case_results
        self.assertEqual(results["a"]["disposition"], "false_acceptance")
        self.assertEqual(results["b"]["disposition"], "false_blocker")
        self.assertEqual(results["c"]["disposition"], "missing_evidence_accepted")


if __name__ == "__main__":
    unittest.main()
