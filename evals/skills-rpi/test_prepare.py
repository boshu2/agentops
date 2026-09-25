"""Pre-launch comparison checks; no model calls."""
import json
from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
import prepare
from taskbank import calibrate


class PairedPreflight(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        for name in ("task/tests", "skills", "control-skills"):
            (self.root / name).mkdir(parents=True)
            (self.root / name / "content").write_text(name)
        self.worker = "sha256:" + "a" * 64
        self.verifier = "sha256:" + "b" * 64
        prepare.write_json(self.root / "task/tests/controls.json", {"correct": "pass", "wrong": "fail"})
        (self.root / "task/task.toml").write_text(
            f'[environment]\ndocker_image="{self.worker}"\n'
            f'[verifier]\nenvironment_mode="separate"\n'
            f'[verifier.environment]\ndocker_image="{self.verifier}"\nnetwork_mode="no-network"\n')
        self.frozen = {"schema_version": 1, "task": "agentops/test",
            "task_checksum": prepare.tree_hash(self.root / "task"),
            "oracle_sha256": prepare.tree_hash(self.root / "task/tests"),
            "skills_sha256": prepare.tree_hash(self.root / "skills"),
            "control_skills_sha256": prepare.tree_hash(self.root / "control-skills"),
            "runtime": {"harbor_version": prepare.HARBOR_VERSION, "codex_version": prepare.CODEX_VERSION,
                        "model": prepare.MODEL, "reasoning_effort": "xhigh",
                        "worker_image_id": self.worker, "verifier_image_id": self.verifier},
            "jobs": []}
        calibration = {"verifier_image_id": self.verifier,
                       "oracle_sha256": self.frozen["oracle_sha256"],
                       "controls": [{"control": "correct", "expected": "pass", "actual": "pass"},
                                    {"control": "wrong", "expected": "fail", "actual": "fail"}]}
        prepare.write_json(self.root / "calibration.json", calibration)
        self.frozen["calibration_sha256"] = prepare.sha(self.root / "calibration.json")
        for arm, package in (("control", "control-skills"), ("treatment", "skills")):
            name = "test-" + arm + "-1"
            config = {"job_name": name, "jobs_dir": str(self.root / "jobs"),
                "n_attempts": 1, "n_concurrent_trials": 1,
                "tasks": [{"path": str(self.root / "task")}],
                "agents": [{"name": "codex", "model_name": prepare.MODEL,
                    "skills": [str(self.root / package)], "kwargs": {
                        "version": prepare.CODEX_VERSION, "reasoning_effort": "xhigh"}}]}
            prepare.write_json(self.root / (name + ".json"), config)
            self.frozen["jobs"].append({"job_name": name, "arm": arm, "rep": 1,
                "input_config": str(self.root / (name + ".json")),
                "input_sha256": prepare.sha(self.root / (name + ".json"))})
        self.save()

    def save(self):
        prepare.write_json(self.root / "staged.json", self.frozen)

    def check(self):
        return prepare.check_staged(self.root / "staged.json")

    def mutate_config(self, change):
        job = self.frozen["jobs"][1]
        path = Path(job["input_config"])
        config = json.loads(path.read_text())
        change(config)
        prepare.write_json(path, config)
        job["input_sha256"] = prepare.sha(path)
        self.save()

    def test_old_new_packages_share_one_frozen_pair(self):
        self.assertEqual(self.check()["pairs"], 1)

    def test_package_or_task_mutation_rejected(self):
        for name in ("skills", "control-skills", "task/tests"):
            with self.subTest(name=name):
                p = self.root / name / "content"
                old = p.read_text()
                p.write_text("changed")
                with self.assertRaises(ValueError):
                    self.check()
                p.write_text(old)

    def test_runtime_identity_cannot_disagree_with_task(self):
        self.frozen["runtime"]["worker_image_id"] = "sha256:" + "c" * 64
        self.save()
        with self.assertRaisesRegex(ValueError, "image"):
            self.check()

    def test_pair_configuration_cannot_differ_beyond_package_or_outputs(self):
        self.mutate_config(lambda c: c["agents"][0].update(extra_instructions=["different policy"]))
        with self.assertRaisesRegex(ValueError, "configuration"):
            self.check()

    def test_unbound_task_path_is_not_normalized_away(self):
        self.mutate_config(lambda c: c["tasks"][0].update(path="/different/task"))
        with self.assertRaisesRegex(ValueError, "task"):
            self.check()

    def test_missing_arm_rejected_before_either_launch(self):
        self.frozen["jobs"].pop()
        self.save()
        with self.assertRaisesRegex(ValueError, "both arms"):
            self.check()

    def test_calibration_cannot_be_replaced_or_fail(self):
        p = self.root / "calibration.json"
        value = json.loads(p.read_text())
        value["controls"][0]["actual"] = "fail"
        prepare.write_json(p, value)
        with self.assertRaisesRegex(ValueError, "calibration"):
            self.check()
        self.frozen["calibration_sha256"] = prepare.sha(p)
        self.save()
        with self.assertRaisesRegex(ValueError, "calibration"):
            self.check()

    def test_omitted_semantic_control_cannot_be_admitted(self):
        p = self.root / "calibration.json"
        value = json.loads(p.read_text())
        value["controls"].pop()
        prepare.write_json(p, value)
        self.frozen["calibration_sha256"] = prepare.sha(p)
        self.save()
        with self.assertRaisesRegex(ValueError, "calibration"):
            self.check()


class PackagedCalibration(unittest.TestCase):
    def test_broken_entrypoint_does_not_satisfy_negative_control(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.assertEqual(calibrate.packaged_outcome(root, 1), "error")
            (root / "reward.txt").write_text("0\n")
            self.assertEqual(calibrate.packaged_outcome(root, 1), "error")
            prepare.write_json(root / "grade.json", {"endpoint_pass": False})
            self.assertEqual(calibrate.packaged_outcome(root, 1), "fail")
            self.assertEqual(calibrate.packaged_outcome(root, 127), "error")


if __name__ == "__main__":
    unittest.main()
