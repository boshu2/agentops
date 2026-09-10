#!/usr/bin/env python3
"""Join a frozen staging manifest to native Harbor configs, without grading work."""
import argparse
import json
from pathlib import Path
from prepare import sha, tree_hash
from harbor.models.job.config import JobConfig


def configuration_matches(expected, actual):
    # Compare native models, including defaults omitted from Harbor's JSON.
    left = JobConfig.model_validate(expected)
    right = JobConfig.model_validate(actual)
    if len(left.agents) != len(right.agents):
        return False
    for wanted, observed in zip(left.agents, right.agents):
        if wanted.env.keys() != observed.env.keys():
            return False
        # Harbor masks this runtime locator. No other environment value is
        # exempt, and this does not attest the authentication/account identity.
        if "CODEX_AUTH_JSON_PATH" in wanted.env:
            if not observed.env["CODEX_AUTH_JSON_PATH"]:
                return False
            observed.env["CODEX_AUTH_JSON_PATH"] = wanted.env["CODEX_AUTH_JSON_PATH"]
    return left == right


def collect(manifests):
    result = {"schema_version": 1, "trials": [], "diagnostics": []}
    for path in manifests:
        frozen = json.loads(path.read_text())
        root = path.parent
        if tree_hash(root / "task") != frozen["task_checksum"]:
            raise ValueError(f"staged task changed after freeze: {path}")
        if tree_hash(root / "skills") != frozen["skills_sha256"]:
            raise ValueError(f"staged package changed after freeze: {path}")
        for job in frozen["jobs"]:
            input_path = Path(job["input_config"])
            if sha(input_path) != job["input_sha256"]:
                raise ValueError(f"input configuration changed: {input_path}")
            expected = json.loads(input_path.read_text())
            native = Path(expected["jobs_dir"]) / job["job_name"] / "config.json"
            if not native.is_file():
                result["diagnostics"].append(f"not observed: {job['job_name']}")
                continue
            if not configuration_matches(expected, json.loads(native.read_text())):
                raise ValueError(f"native configuration differs from frozen launch: {native}")
            result["trials"].append({
                "job_name": job["job_name"], "task": frozen["task"],
                "rep": job["rep"], "arm": job["arm"],
                "task_checksum": frozen["task_checksum"],
                "oracle_sha256": frozen["oracle_sha256"],
                "skills_sha256": frozen["skills_sha256"] if job["arm"] == "treatment" else None,
                "runtime": frozen["runtime"], "isolation": frozen["isolation"],
                "configuration_sha256": sha(native),
                "staging_manifest": {"path": str(path), "sha256": sha(path)},
            })
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--staged", type=Path, action="append", required=True)
    args = parser.parse_args()
    print(json.dumps(collect(args.staged), indent=2))
