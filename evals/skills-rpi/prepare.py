#!/usr/bin/env python3
"""Stage one public task and native Harbor configs; never launches an agent."""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import subprocess
import tarfile
import tempfile
import tomllib
from collections import defaultdict

HARBOR_VERSION = "0.22.0"
CODEX_VERSION = "0.154.0"
MODEL = "openai/gpt-6-astra"


def run(args, **kwargs):
    return subprocess.run(args, check=True, timeout=900, **kwargs)


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def tree_hash(path):
    from dirhash import dirhash
    if any(p.is_symlink() for p in path.rglob("*")):
        raise ValueError(f"source symlink requires explicit review: {path}")
    return dirhash(path, "sha256")


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")
    path.chmod(0o600)


def extract_cli(archive, destination):
    """Import only regular CLI source files from the selected Git archive."""
    root = destination.resolve()
    with tarfile.open(fileobj=archive) as tar:
        for member in tar:
            name = PurePosixPath(member.name)
            if name.is_absolute() or not name.parts or name.parts[0] != "cli" or ".." in name.parts:
                raise ValueError("archive entry outside the CLI source")
            if not member.isdir() and not member.isfile():
                raise ValueError("archive links and special files are not source inputs")
            target = destination.joinpath(*name.parts)
            if not target.resolve().is_relative_to(root):
                raise ValueError("archive target escapes its staging directory")
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
                continue
            target.parent.mkdir(parents=True, exist_ok=True)
            source = tar.extractfile(member)
            if source is None:
                raise ValueError("archive member has no source bytes")
            with source, target.open("xb") as output:
                shutil.copyfileobj(source, output)
            target.chmod(member.mode & 0o777)


def check_staged(path):
    """Read-only admission check for all arms before either native launch."""
    from readout import normalized_config
    frozen = json.loads(path.read_text())
    root = path.parent.resolve()
    for directory, key in (("task", "task_checksum"), ("task/tests", "oracle_sha256"),
                           ("skills", "skills_sha256"), ("control-skills", "control_skills_sha256")):
        if directory == "control-skills" and frozen.get(key) is None:
            continue
        if tree_hash(root / directory) != frozen[key]:
            raise ValueError(f"staged {directory} changed after freeze")
    runtime = frozen["runtime"]
    spec = tomllib.loads((root / "task/task.toml").read_text())
    if (spec["environment"].get("docker_image") != runtime["worker_image_id"] or
            spec["verifier"]["environment"].get("docker_image") != runtime["verifier_image_id"] or
            any(not value.startswith("sha256:") or len(value) != 71 for value in
                (runtime["worker_image_id"], runtime["verifier_image_id"]))):
        raise ValueError("task and frozen image identities differ or are not immutable")
    if (spec["verifier"].get("environment_mode") != "separate" or
            spec["verifier"]["environment"].get("network_mode") != "no-network"):
        raise ValueError("separate no-network verifier is required")
    calibration = root / "calibration.json"
    if not frozen.get("calibration_sha256") or sha(calibration) != frozen["calibration_sha256"]:
        raise ValueError("packaged calibration missing or changed")
    checked = json.loads(calibration.read_text())
    rows = checked["controls"]
    expected_controls = json.loads((root / "task/tests/controls.json").read_text())
    if (checked["verifier_image_id"] != runtime["verifier_image_id"] or
            checked["oracle_sha256"] != frozen["oracle_sha256"] or
            len(rows) != len(expected_controls) or
            {r["control"]: r["expected"] for r in rows} != expected_controls or
            not rows or {r["expected"] for r in rows} != {"pass", "fail"} or
            any(r["expected"] != r["actual"] or
                (r["actual"] == "fail" and r.get("failure_kind") != "candidate") for r in rows)):
        raise ValueError("packaged calibration failed or belongs to another verifier")
    pairs = defaultdict(dict)
    names = set()
    for job in frozen["jobs"]:
        arm, rep = job["arm"], job["rep"]
        if arm not in ("control", "treatment") or not isinstance(rep, int) or isinstance(rep, bool) or rep < 1:
            raise ValueError("invalid pair identity")
        if arm in pairs[rep] or job["job_name"] in names:
            raise ValueError("duplicate pair identity")
        names.add(job["job_name"])
        config_path = Path(job["input_config"])
        if sha(config_path) != job["input_sha256"]:
            raise ValueError("input configuration changed after freeze")
        config = json.loads(config_path.read_text())
        if config.get("job_name") != job["job_name"]:
            raise ValueError("configuration job identity differs")
        tasks, agents = config.get("tasks", []), config.get("agents", [])
        if len(tasks) != 1 or Path(tasks[0]["path"]).resolve() != root / "task":
            raise ValueError("configuration task path differs from frozen task")
        if len(agents) != 1:
            raise ValueError("configuration agent is ambiguous")
        agent = agents[0]
        wanted = ([str(root / "skills")] if arm == "treatment" else
                  [str(root / "control-skills")] if frozen.get("control_skills_sha256") else [])
        if [str(Path(p).resolve()) for p in agent.get("skills", [])] != wanted:
            raise ValueError("configuration package differs from frozen package")
        if (agent.get("model_name") != runtime["model"] or
                agent.get("kwargs", {}).get("version") != runtime["codex_version"] or
                agent.get("kwargs", {}).get("reasoning_effort") != runtime["reasoning_effort"]):
            raise ValueError("configuration runtime differs from frozen runtime")
        pairs[rep][arm] = normalized_config(config)
    if not pairs or any(set(pair) != {"control", "treatment"} for pair in pairs.values()):
        raise ValueError("freeze both arms before either launch")
    if any(pair["control"] != pair["treatment"] for pair in pairs.values()):
        raise ValueError("paired configuration differs beyond package and output paths")
    if len({json.dumps(pair["control"], sort_keys=True) for pair in pairs.values()}) != 1:
        raise ValueError("configuration differs across paired repetitions")
    if frozen.get("control_skills_sha256") == frozen["skills_sha256"]:
        raise ValueError("both arms have the same package identity")
    return {"pairs": len(pairs), "task": frozen["task"], "status": "ready for caller admission"}


def prepare(task, output, skills, auth_file, reps, control_skills=None):
    # The caller chooses protected non-Git storage. Existing runs are immutable.
    if output.exists():
        raise ValueError("output already exists; do not overwrite a run")
    if not auth_file.is_file():
        raise ValueError("explicit native Codex auth locator is missing")
    if reps < 1:
        raise ValueError("reps must be positive")
    import importlib.metadata
    if importlib.metadata.version("harbor") != HARBOR_VERSION:
        raise ValueError("use the pinned Harbor environment")
    tree_hash(task)
    tree_hash(skills)
    if control_skills is not None:
        tree_hash(control_skills)
    output.mkdir(mode=0o700, parents=True)
    staged = output / "task"
    shutil.copytree(task, staged)
    frozen_skills = output / "skills"
    shutil.copytree(skills, frozen_skills)
    frozen_control = output / "control-skills" if control_skills is not None else None
    if frozen_control is not None:
        shutil.copytree(control_skills, frozen_control)
    config = tomllib.loads((task / "task.toml").read_text())
    slug = config["task"]["name"].split("/")[-1]
    repository = Path(__file__).resolve().parents[2]
    env = staged / "environment"
    tests = staged / "tests"
    if (env / "source").is_dir():
        shutil.copytree(env / "source", tests / "source")
        helper = task.parent.parent / "taskbank" / "verify.py"
        shutil.copy2(helper, tests / "verify.py")
    else:
        commit = config["metadata"]["source_commit"]
        with tempfile.TemporaryFile() as archive:
            run(["git", "-C", str(repository), "archive", commit, "cli"], stdout=archive)
            archive.seek(0)
            extract_cli(archive, env)
        shutil.copytree(env / "cli", tests / "cli")
    helpers = repository / "scripts" / "lib"
    if not (env / "helpers").exists():
        shutil.copytree(helpers, env / "helpers")
    worker_tag = f"agentops-skill-eval-worker:{slug}-v1"
    verifier_tag = f"agentops-skill-eval-verifier:{slug}-v1"
    for context, tag in ((env, worker_tag), (tests, verifier_tag)):
        with (output / f"{context.name}-build.log").open("w") as log:
            run(["docker", "build", "-t", tag, str(context)], stdout=log, stderr=log)
    ids = {}
    for kind, tag in (("worker_image_id", worker_tag), ("verifier_image_id", verifier_tag)):
        ids[kind] = run(["docker", "image", "inspect", "--format", "{{.Id}}", tag],
                        capture_output=True, text=True).stdout.strip()
        # A later variant can replace the mutable build tag. Keep this exact
        # local image reachable for the already-frozen native configuration.
        run(["docker", "tag", ids[kind],
             "agentops-skill-eval-frozen:" + ids[kind].removeprefix("sha256:")])
    text = (staged / "task.toml").read_text()
    # Freeze local image identities after builds; no mutable tag is launched.
    text = text.replace(f'docker_image = "{verifier_tag}"',
                        f'docker_image = "{ids["verifier_image_id"]}"')
    if f'docker_image = "{worker_tag}"' in text:
        text = text.replace(f'docker_image = "{worker_tag}"',
                            f'docker_image = "{ids["worker_image_id"]}"')
    else:
        text = text.replace("[environment]\n", f'[environment]\ndocker_image = "{ids["worker_image_id"]}"\n')
    (staged / "task.toml").write_text(text)
    from harbor.models.task.task import Task
    from harbor.models.job.config import JobConfig
    Task(staged)
    calibration_sha256 = None
    if (tests / "controls.json").is_file():
        from taskbank.calibrate import packaged
        controls = packaged(staged, ids["verifier_image_id"], output / "calibration")
        write_json(output / "calibration.json", {
            "verifier_image_id": ids["verifier_image_id"], "oracle_sha256": tree_hash(tests),
            "controls": controls})
        if any(row["actual"] != row["expected"] for row in controls):
            raise ValueError("packaged verifier calibration failed; no launch configs published")
        calibration_sha256 = sha(output / "calibration.json")
    manifest = {
        "schema_version": 1, "task": config["task"]["name"],
        "task_checksum": tree_hash(staged), "oracle_sha256": tree_hash(tests),
        "skills_sha256": tree_hash(frozen_skills),
        "control_skills_sha256": tree_hash(frozen_control) if frozen_control is not None else None,
        "calibration_sha256": calibration_sha256,
        "runtime": {"harbor_version": HARBOR_VERSION, "codex_version": CODEX_VERSION,
                    "model": MODEL, "reasoning_effort": "xhigh", **ids},
        "isolation": {"separate_verifier": True, "private_inputs_excluded": True,
                      "network_policy": "worker OpenAI allowlist; separate verifier no-network"},
        "jobs": [],
    }
    for rep in range(1, reps + 1):
        for arm in ("control", "treatment"):
            name = f"{slug}-{arm}-{rep}"
            job = {
                "job_name": name, "jobs_dir": str(output / "jobs"),
                "n_attempts": 1, "n_concurrent_trials": 1, "retry": {"max_retries": 0},
                "environment": {"type": "docker", "delete": True},
                "agents": [{"name": "codex", "model_name": MODEL,
                    "resume_trajectory": False, "override_setup_timeout_sec": 180,
                    "skills": [str(frozen_skills)] if arm == "treatment" else [str(frozen_control)] if frozen_control else [],
                    "env": {"CODEX_AUTH_JSON_PATH": str(auth_file)},
                    "kwargs": {"version": CODEX_VERSION, "reasoning_effort": "xhigh",
                        "web_search": "disabled", "config": {
                            "model": MODEL.split("/", 1)[1], "model_reasoning_effort": "xhigh",
                            "web_search": "disabled", "features": {
                                "hooks": False, "apps": False, "plugins": False, "multi_agent": False}}}}],
                "tasks": [{"path": str(staged)}],
            }
            JobConfig.model_validate(job)
            # Pydantic's presentation redaction would corrupt the auth path.
            path = output / f"{name}.json"
            write_json(path, job)
            manifest["jobs"].append({"job_name": name, "arm": arm, "rep": rep,
                                     "input_config": str(path), "input_sha256": sha(path)})
    write_json(output / "staged.json", manifest)
    if calibration_sha256:
        check_staged(output / "staged.json")
    print(f"Prepared {len(manifest['jobs'])} native configs; no trials started: {output}")
    if not calibration_sha256:
        print("Packaged calibration unavailable; --check-staged will refuse live admission.")


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--check-staged", type=Path, help="recheck the whole frozen pair immediately before a native launch")
    p.add_argument("--task", type=Path)
    p.add_argument("--output", type=Path)
    p.add_argument("--skills", type=Path)
    p.add_argument("--control-skills", type=Path, help="optional complete baseline package for an old/new comparison")
    p.add_argument("--auth-file", type=Path)
    p.add_argument("--reps", type=int, default=2)
    a = p.parse_args()
    os.umask(0o077)
    if a.check_staged:
        print(json.dumps(check_staged(a.check_staged.resolve())))
        return
    if not all((a.task, a.output, a.skills, a.auth_file)):
        p.error("preparation requires --task, --output, --skills and --auth-file")
    prepare(a.task.resolve(), a.output.resolve(), a.skills.resolve(), a.auth_file.resolve(), a.reps,
            a.control_skills.resolve() if a.control_skills else None)


if __name__ == "__main__":
    main()
