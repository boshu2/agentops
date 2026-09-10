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


def prepare(task, output, skills, auth_file, reps):
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
    output.mkdir(mode=0o700, parents=True)
    staged = output / "task"
    shutil.copytree(task, staged)
    frozen_skills = output / "skills"
    shutil.copytree(skills, frozen_skills)
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
    manifest = {
        "schema_version": 1, "task": config["task"]["name"],
        "task_checksum": tree_hash(staged), "oracle_sha256": tree_hash(tests),
        "skills_sha256": tree_hash(frozen_skills),
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
                    "skills": [str(frozen_skills)] if arm == "treatment" else [],
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
    print(f"Prepared {len(manifest['jobs'])} native configs; no trials started: {output}")


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--task", type=Path, required=True)
    p.add_argument("--output", type=Path, required=True)
    p.add_argument("--skills", type=Path, required=True)
    p.add_argument("--auth-file", type=Path, required=True)
    p.add_argument("--reps", type=int, default=2)
    a = p.parse_args()
    os.umask(0o077)
    prepare(a.task.resolve(), a.output.resolve(), a.skills.resolve(), a.auth_file.resolve(), a.reps)


if __name__ == "__main__":
    main()
