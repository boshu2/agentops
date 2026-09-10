#!/usr/bin/env python3
"""Separate, evaluator-owned endpoint grader for the standalone task bank."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile


class VerdictMismatch(ValueError):
    def __init__(self, case_results):
        super().__init__("verdicts do not match the case-level oracle")
        self.case_results = case_results
        self.grade_context = {}


def judge_verdicts(expected, actual):
    is_object = isinstance(actual, dict)
    submitted = actual if is_object else {}
    results = []
    for case, wanted in expected.items():
        got = submitted.get(case)
        if got == wanted:
            classification = "justified_not_proven" if wanted == "NOT_PROVEN" else "correct"
        elif wanted == "PASS" and got in ("FAIL", "NOT_PROVEN"):
            classification = "false_blocker"
        elif wanted in ("FAIL", "NOT_PROVEN") and got == "PASS":
            classification = "false_acceptance"
        elif got is None:
            classification = "missing"
        else:
            classification = "incorrect_disposition"
        results.append({"case_id": case, "expected": wanted, "actual": got,
                        "classification": classification})
    if not is_object or submitted != expected:
        raise VerdictMismatch(results)
    return results


def snapshot(root):
    if root.is_symlink() or not root.is_dir():
        raise ValueError("subject root must be a real directory")
    result = {}
    for path in sorted(root.rglob("*")):
        name = path.relative_to(root).as_posix()
        if name == ".git" or name.startswith(".git/"):
            continue
        if path.is_symlink():
            raise ValueError("symlink in subject: " + name)
        if path.is_file():
            if path.stat().st_size > 1024 * 1024:
                raise ValueError("oversize subject file: " + name)
            result[name] = path.read_bytes()
    return result


def digest(files):
    h = hashlib.sha256()
    for name, data in sorted(files.items()):
        h.update(name.encode() + b"\0" + hashlib.sha256(data).digest())
    return h.hexdigest()


def grade(baseline, candidate, tests, log):
    spec = json.loads((tests / "spec.json").read_text())
    before, after = snapshot(baseline), snapshot(candidate)
    allowed = set(spec["editable"])
    for name in sorted(before.keys() | after.keys()):
        if before.get(name) == after.get(name):
            continue
        added_test = name not in before and name.endswith("_test.go") and spec.get("allow_added_tests", False)
        if name not in allowed and not added_test:
            raise ValueError("out-of-scope change: " + name)
    for name in spec.get("production", []):
        if name not in after:
            raise ValueError("missing source: " + name)
    with tempfile.TemporaryDirectory(prefix="fixture-oracle-") as tmp:
        clean = Path(tmp)
        shutil.copytree(baseline, clean, dirs_exist_ok=True, ignore=shutil.ignore_patterns(".git"))
        for name in spec.get("production", []):
            (clean / name).write_bytes(after[name])
        for oracle in tests.glob("*_test.go"):
            shutil.copy2(oracle, clean / oracle.name)
        env = dict(os.environ, GOWORK="off", GOFLAGS="", GOTOOLCHAIN="local", GOPROXY="off")
        check = subprocess.run(["go", "test", "-count=1", "-timeout=30s", "./..."], cwd=clean, env=env,
                               stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=60)
        (log / "go-test.log").write_bytes(check.stdout)
        if check.returncode:
            raise ValueError("endpoint oracle failed; see go-test.log")
    result = {"endpoint_pass": True, "subject_sha256": digest(after),
              "checked": spec["checked"], "not_checked": spec.get("not_checked", [])}
    if spec.get("limitations"):
        result["limitations"] = spec["limitations"]
    if spec.get("verdicts"):
        try:
            verdicts = json.loads(after.get("verdicts.json", b"{}"))
        except (ValueError, UnicodeDecodeError):
            # The trusted subject was checked, but no case verdict was supplied
            # in a readable document. Retain all expected cases as missing.
            verdicts = None
        try:
            result["case_results"] = judge_verdicts(spec["verdicts"], verdicts)
        except VerdictMismatch as error:
            error.grade_context = {**result, "endpoint_pass": False}
            raise
    if spec.get("dispositions"):
        actual = json.loads(after.get("dispositions.json", b"{}"))
        if actual != spec["dispositions"]:
            raise ValueError("recorded dispositions do not match fixed caller authority")
    return result


def main():
    baseline, candidate, tests, log = map(Path, sys.argv[1:])
    log.mkdir(parents=True, exist_ok=True)
    (log / "reward.txt").write_text("0\n")
    try:
        result = grade(baseline, candidate, tests, log)
        # The Harbor scalar is endpoint correctness, never proof of an unobserved
        # session transition or live stop. Read grade.json before comparison.
        (log / "reward.txt").write_text("1\n")
        status = 0
    except (ValueError, OSError, subprocess.SubprocessError) as error:
        (log / "reward.txt").write_text("0\n")
        result = {"endpoint_pass": False, "error": str(error)}
        if isinstance(error, VerdictMismatch):
            result.update(error.grade_context)
            result["case_results"] = error.case_results
        status = 1
    (log / "grade.json").write_text(json.dumps(result, indent=2) + "\n")
    return status


if __name__ == "__main__":
    raise SystemExit(main())
