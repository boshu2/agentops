"""Narrow verifier integrity tests; no runtime credentials or model calls."""
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import sys
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
        results = {row["case_id"]: row for row in caught.exception.case_results}
        self.assertEqual(results["a"]["classification"], "false_acceptance")
        self.assertEqual(results["b"]["classification"], "false_blocker")
        self.assertEqual(results["c"]["classification"], "false_acceptance")


class VerifierCaseOutputTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="verifier-case-output-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        task = Path(__file__).parent.parent / "tasks/validator-controls"
        self.baseline = task / "environment/source"
        self.candidate = self.root / "candidate"
        self.tests = self.root / "tests"
        self.log = self.root / "log"
        shutil.copytree(self.baseline, self.candidate)
        shutil.copytree(task / "tests", self.tests)
        self.correct = json.loads((task / "solution/verdicts.json").read_text())

    def run_verifier(self, verdicts):
        text = verdicts if isinstance(verdicts, str) else json.dumps(verdicts)
        (self.candidate / "verdicts.json").write_text(text)
        completed = subprocess.run(
            [sys.executable, str(Path(__file__).with_name("verify.py")),
             str(self.baseline), str(self.candidate), str(self.tests), str(self.log)],
            capture_output=True, text=True, timeout=60,
        )
        result = json.loads((self.log / "grade.json").read_text())
        reward = (self.log / "reward.txt").read_text().strip()
        return completed.returncode, reward, result

    def test_correct_verdicts_keep_all_case_denominators(self):
        status, reward, result = self.run_verifier(self.correct)
        self.assertEqual((status, reward, result["endpoint_pass"]), (0, "1", True))
        self.assertEqual(result["case_results"], [
            {"case_id": "candidate-a", "expected": "FAIL", "actual": "FAIL", "classification": "correct"},
            {"case_id": "candidate-b", "expected": "PASS", "actual": "PASS", "classification": "correct"},
            {"case_id": "candidate-c", "expected": "NOT_PROVEN", "actual": "NOT_PROVEN", "classification": "justified_not_proven"},
        ])
        self.assertTrue(result["checked"])
        self.assertEqual(result["not_checked"], [])

    def test_wrong_verdicts_keep_every_case_and_failure_status(self):
        status, reward, result = self.run_verifier({
            "candidate-a": "PASS", "candidate-b": "NOT_PROVEN", "candidate-c": "PASS",
        })
        self.assertEqual((status, reward, result["endpoint_pass"]), (1, "0", False))
        rows = result["case_results"]
        self.assertEqual([row["expected"] for row in rows], ["FAIL", "PASS", "NOT_PROVEN"])
        self.assertEqual([row["classification"] for row in rows], ["false_acceptance", "false_blocker", "false_acceptance"])
        self.assertTrue(result["checked"])
        self.assertEqual(result["not_checked"], [])
        self.assertEqual(len(result["subject_sha256"]), 64)

    def test_rejecting_clean_candidate_is_false_blocker(self):
        status, reward, result = self.run_verifier({**self.correct, "candidate-b": "FAIL"})
        self.assertEqual((status, reward), (1, "0"))
        rows = {row["case_id"]: row for row in result["case_results"]}
        self.assertEqual(rows["candidate-a"]["classification"], "correct")
        self.assertEqual(rows["candidate-b"]["classification"], "false_blocker")
        self.assertEqual(rows["candidate-c"]["classification"], "justified_not_proven")

    def test_missing_or_unreadable_verdicts_retain_expected_cases(self):
        for supplied in [{"candidate-a": "NOT_PROVEN", "candidate-b": "PASS"}, "{broken", []]:
            with self.subTest(supplied=supplied):
                status, reward, result = self.run_verifier(supplied)
                self.assertEqual((status, reward), (1, "0"))
                self.assertEqual(len(result["case_results"]), 3)
                row = result["case_results"][2]
                self.assertEqual(row["classification"], "missing")
                self.assertIsNone(row["actual"])
        self.assertEqual(verify.judge_verdicts({"a": "FAIL"}, {"a": "FAIL"})[0]["classification"], "correct")
        with self.assertRaises(verify.VerdictMismatch) as caught:
            verify.judge_verdicts({"a": "FAIL"}, {"a": "NOT_PROVEN"})
        self.assertEqual(caught.exception.case_results[0]["classification"], "incorrect_disposition")

    def test_source_integrity_failure_leaves_case_grading_unmeasured(self):
        (self.candidate / "candidate_b/lease.go").write_text("package candidate_b\n")
        status, reward, result = self.run_verifier(self.correct)
        self.assertEqual((status, reward), (1, "0"))
        self.assertIn("out-of-scope", result["error"])
        self.assertNotIn("case_results", result)

    def test_oracle_failure_leaves_case_grading_unmeasured(self):
        (self.tests / "oracle_test.go").write_text(
            'package leases_test\nimport "testing"\n'
            'func TestBrokenControl(t *testing.T) { t.Fatal("invalid oracle") }\n'
        )
        status, reward, result = self.run_verifier(self.correct)
        self.assertEqual((status, reward), (1, "0"))
        self.assertIn("oracle failed", result["error"])
        self.assertNotIn("case_results", result)


if __name__ == "__main__":
    unittest.main()
