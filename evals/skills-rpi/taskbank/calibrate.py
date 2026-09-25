#!/usr/bin/env python3
"""Local deterministic control calibration; never launches a model."""
import argparse
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import uuid

loader = importlib.util.spec_from_file_location("verify", Path(__file__).with_name("verify.py"))
verify = importlib.util.module_from_spec(loader)
loader.loader.exec_module(verify)


def packaged_outcome(log, exit_code):
    try:
        reward = (log / "reward.txt").read_text().strip()
        grade = json.loads((log / "grade.json").read_text())
        if exit_code == 0 and reward == "1" and grade.get("endpoint_pass") is True:
            return "pass"
        if (exit_code == 1 and reward == "0" and grade.get("endpoint_pass") is False
                and grade.get("failure_kind") == "candidate"):
            return "fail"
    except (OSError, ValueError):
        pass
    return "error"  # Broken packaging is not a successful negative control.


def packaged(task, verifier_image, output):
    """Exercise the frozen container entry point, not an imported host grader."""
    controls = json.loads((task / "tests/controls.json").read_text())
    if not controls or set(controls.values()) != {"pass", "fail"}:
        raise ValueError("calibration requires positive and negative controls")
    results = []
    for control, expected in controls.items():
        if control in (".", "..") or Path(control).name != control:
            raise ValueError("control must name one task-local fixture")
        log = output / control
        log.mkdir(parents=True, exist_ok=False)
        with tempfile.TemporaryDirectory(prefix="packaged-control-") as tmp:
            candidate = Path(tmp) / "candidate"
            shutil.copytree(task / "environment/source", candidate)
            overlay = task / ("solution" if control == "correct" else "tests/controls/" + control)
            if control != "noop" and not overlay.is_dir():
                raise ValueError("missing declared control fixture: " + control)
            if overlay.exists():
                shutil.copytree(overlay, candidate, dirs_exist_ok=True)
            name = "agentops-calibration-" + uuid.uuid4().hex
            command = ["docker", "create", "--name", name, "--network", "none",
                       "--cpus", "2", "--memory", "4g", "--entrypoint", "/bin/sh",
                       verifier_image, "/tests/test.sh"]
            subprocess.run(command, capture_output=True, timeout=30, check=True)
            try:
                # Copy through Docker, as Harbor does; daemon and caller need
                # not share a filesystem (remote/VM Docker contexts are valid).
                subprocess.run(["docker", "cp", str(candidate) + "/.", name + ":/app/work"],
                               capture_output=True, timeout=30, check=True)
                completed = subprocess.run(["docker", "start", "-a", name], capture_output=True, timeout=180)
                (log / "container.log").write_bytes(completed.stdout + completed.stderr)
                copied = subprocess.run(["docker", "cp", name + ":/logs/verifier/.", str(log)],
                                        capture_output=True, timeout=30)
                actual = packaged_outcome(log, completed.returncode) if copied.returncode == 0 else "error"
            finally:
                subprocess.run(["docker", "rm", "-f", name], capture_output=True, timeout=30, check=True)
            row = {"control": control, "expected": expected, "actual": actual,
                   "exit_code": completed.returncode,
                   "failure_kind": "candidate" if actual == "fail" else None}
            results.append(row)
            print(control, actual, "OK" if actual == expected else "MISMATCH", flush=True)
    return results


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--tasks", nargs="*", default=[])
    parser.add_argument("--output", type=Path, required=True, help="external non-Git evidence directory")
    args = parser.parse_args()
    tasks = Path(__file__).parent.parent / "tasks"
    selected = args.tasks or ["input-scope", "persistence-recovery", "fresh-handoff", "validator-controls", "bounded-stop"]
    args.output.mkdir(parents=True, exist_ok=True)
    results = []
    for name in selected:
        task = tasks / name
        controls = json.loads((task / "tests/controls.json").read_text())
        for control, expected in controls.items():
            with tempfile.TemporaryDirectory(prefix="fixture-control-") as tmp:
                candidate = Path(tmp) / "candidate"
                shutil.copytree(task / "environment/source", candidate)
                overlay = task / ("solution" if control == "correct" else "tests/controls/" + control)
                if overlay.exists():
                    shutil.copytree(overlay, candidate, dirs_exist_ok=True)
                log = args.output / name / control
                log.mkdir(parents=True, exist_ok=True)
                try:
                    result = verify.grade(task / "environment/source", candidate, task / "tests", log)
                    actual = "pass"
                except (ValueError, OSError, subprocess.SubprocessError) as error:
                    result = {"error": str(error)}
                    if isinstance(error, verify.VerdictMismatch):
                        result["case_results"] = error.case_results
                    actual = "fail" if isinstance(error, verify.CandidateRejected) else "error"
                row = {"task": name, "control": control, "expected": expected, "actual": actual, **result}
                (log / "result.json").write_text(json.dumps(row, indent=2) + "\n")
                results.append(row)
                print(name, control, actual, "OK" if actual == expected else "MISMATCH", flush=True)
    version = subprocess.check_output(["go", "version"], text=True).strip()
    summary = {"go_version": version, "controls": results}
    (args.output / "calibration.json").write_text(json.dumps(summary, indent=2) + "\n")
    return 0 if all(row["actual"] == row["expected"] for row in results) else 1


if __name__ == "__main__":
    raise SystemExit(main())
