#!/usr/bin/env python3
"""Local deterministic control calibration; never launches a model."""
import argparse
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import tempfile

loader = importlib.util.spec_from_file_location("verify", Path(__file__).with_name("verify.py"))
verify = importlib.util.module_from_spec(loader)
loader.loader.exec_module(verify)


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
                    actual = "fail"
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
