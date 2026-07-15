#!/usr/bin/env python3
"""Structural conformance for the Cathedral Cut product boundary."""

from __future__ import annotations

import ast
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

import yaml


ROOT = Path(__file__).resolve().parents[1]
CORE = ("rpi", "plan", "implement", "validate")
CORE_SCHEMAS = (
    "plan-packet.v1.schema.json",
    "candidate-packet.v1.schema.json",
    "subject-manifest.v1.schema.json",
    "revision-packet.v1.schema.json",
    "verdict.v2.schema.json",
    "rpi-report.v1.schema.json",
)
FORBIDDEN_STATE = {
    "owner", "ready", "claim", "priority", "attempt", "attempts", "queue",
    "lease", "admission", "next_action", "next-action", "close", "closure",
    "release", "delivery", "budget", "retry", "retries",
}
FORBIDDEN_SCHEMA_STATE = {
    "retry", "retries", "budget", "queue", "claim", "lease", "admission",
    "next_action", "next-action", "closure", "release", "delivery",
}
REMOVED_SKILLS = {
    "discovery", "behavior-first-planning", "goal-design", "crank", "converge",
    "evolve", "gc-membrane", "pawl-review", "push", "release", "pr-prep",
    "beads-br", "beads-bv", "pre-mortem", "pre_mortem", "post-mortem",
    "post_mortem",
}
REMOVED_COMMANDS = {
    "pawl", "plan-pawl", "land", "done", "close", "governor", "yield",
    "claim", "next-work", "state", "worktree", "validate", "converge",
    "reconcile", "membrane", "crank",
}
RETIRED_SCHEMAS = {
    "verdict.v1.schema.json", "pawl-verdict.v1.schema.json",
    "validation-receipt.v1.schema.json", "validation-request.v1.schema.json",
    "execution-packet.schema.json", "next-work-batch.v1.schema.json",
    "next-work-item.v1.schema.json", "yieldledger-event.v1.schema.json",
    "claim-registry.v1.schema.json", "verdict-ledger.v1.schema.json",
}


def frontmatter(name: str) -> dict:
    text = (ROOT / "skills" / name / "SKILL.md").read_text(encoding="utf-8")
    parts = text.split("---", 2)
    if len(parts) != 3:
        raise AssertionError(f"{name}: missing frontmatter")
    value = yaml.safe_load(parts[1]) or {}
    if not isinstance(value, dict):
        raise AssertionError(f"{name}: invalid frontmatter")
    return value


def property_names(value: object) -> set[str]:
    names: set[str] = set()
    if isinstance(value, dict):
        props = value.get("properties")
        if isinstance(props, dict):
            names.update(str(key) for key in props)
        for child in value.values():
            names.update(property_names(child))
    elif isinstance(value, list):
        for child in value:
            names.update(property_names(child))
    return names


def check_skill_graph() -> None:
    entries = {path.parent.name: frontmatter(path.parent.name) for path in (ROOT / "skills").glob("*/SKILL.md")}
    expected = {"rpi": {"plan", "implement", "validate"}, "plan": set(), "implement": set(), "validate": set()}
    actual = {
        name: set((entries[name].get("metadata") or {}).get("dependencies") or [])
        for name in CORE
    }
    assert actual == expected, f"core dependency graph mismatch: {actual}"
    for name, entry in entries.items():
        deps = set((entry.get("metadata") or {}).get("dependencies") or [])
        if name != "rpi":
            assert not deps.intersection(CORE), f"{name}: forbidden hard core dependency {deps.intersection(CORE)}"
    for name in REMOVED_SKILLS:
        assert not (ROOT / "skills" / name / "SKILL.md").exists(), f"removed skill is live: {name}"
        assert not (ROOT / "skills-codex" / name / "SKILL.md").exists(), f"removed Codex skill is live: {name}"
    assert (ROOT / "skills" / "premortem" / "SKILL.md").is_file()
    assert (ROOT / "skills" / "postmortem" / "SKILL.md").is_file()
    swarm = entries["swarm"]
    assert not ((swarm.get("metadata") or {}).get("dependencies") or []), "swarm must remain optional"
    assert "dispatch_once" in (ROOT / "skills" / "swarm" / "SKILL.md").read_text(encoding="utf-8")


def check_generated_skill_inventory() -> None:
    source_names = {path.parent.name for path in (ROOT / "skills").glob("*/SKILL.md")}
    codex_names = {path.parent.name for path in (ROOT / "skills-codex").glob("*/SKILL.md")}
    catalog = json.loads((ROOT / "skills" / "catalog.json").read_text(encoding="utf-8"))
    rows = catalog.get("skills") or []
    catalog_names = [row.get("name") for row in rows]
    assert len(catalog_names) == len(set(catalog_names)), "catalog contains duplicate dispositions"
    assert set(catalog_names) == source_names, "catalog does not cover every current skill exactly once"
    assert source_names == codex_names, "source and Codex skill sets differ"
    for row in rows:
        assert isinstance(row.get("disposition"), str) and row["disposition"], (
            f"{row.get('name')}: missing generated disposition"
        )


def check_core_schemas() -> None:
    for path in sorted((ROOT / "schemas").glob("*.json")):
        schema = json.loads(path.read_text(encoding="utf-8"))
        bad = property_names(schema).intersection(FORBIDDEN_SCHEMA_STATE)
        assert not bad, f"{path.name}: retired lifecycle state {sorted(bad)}"
    for filename in CORE_SCHEMAS:
        path = ROOT / "schemas" / filename
        assert path.is_file(), f"missing core schema: {filename}"
        schema = json.loads(path.read_text(encoding="utf-8"))
        bad = property_names(schema).intersection(FORBIDDEN_STATE)
        assert not bad, f"{filename}: lifecycle state {sorted(bad)}"
    verdict = json.loads((ROOT / "schemas" / "verdict.v2.schema.json").read_text())
    assert set(verdict["properties"]["verdict"]["enum"]) == {"PASS", "FAIL", "NOT_PROVEN"}
    for forbidden in ("WARN", "confidence", "disposition", "next_action", "NOT_BUILT", "NOT_PLANNED"):
        assert forbidden not in json.dumps(verdict), f"verdict.v2 retains {forbidden}"
    for filename in RETIRED_SCHEMAS:
        assert not (ROOT / "schemas" / filename).exists(), f"retired schema is live: {filename}"


def check_single_pass_contract() -> None:
    text = (ROOT / "skills" / "rpi" / "SKILL.md").read_text(encoding="utf-8")
    for phase in ("plan", "implement", "validate"):
        assert text.lower().count(f"invoke `/{phase}` once") == 1, f"RPI does not dispatch {phase} exactly once"
    assert "Stop regardless" in text
    report = json.loads((ROOT / "schemas" / "rpi-report.v1.schema.json").read_text())
    assert "next_action" not in property_names(report)


def check_validate_helper() -> None:
    path = ROOT / "skills" / "validate" / "scripts" / "validate.py"
    tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
    forbidden_imports = {"subprocess", "socket", "urllib", "http", "requests", "git", "dulwich"}
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for alias in node.names:
                assert alias.name.split(".")[0] not in forbidden_imports, f"validate helper imports {alias.name}"
        elif isinstance(node, ast.ImportFrom):
            assert (node.module or "").split(".")[0] not in forbidden_imports, f"validate helper imports {node.module}"
        elif isinstance(node, ast.Call) and isinstance(node.func, ast.Attribute):
            assert node.func.attr not in {"system", "popen", "spawn", "execv", "execve"}, f"validate helper launches {node.func.attr}"


def check_tombstones() -> None:
    source = (ROOT / "cli" / "cmd" / "ao" / "removed_command_hint.go").read_text(encoding="utf-8")
    for name in REMOVED_COMMANDS:
        assert f'"{name}"' in source, f"missing tombstone: {name}"
    assert "cli/internal/" not in source and "internal/commands/" not in source
    assert "exec.Command" not in source and "os/exec" not in source
    removed_sources = {
        "pawl.go", "plan_pawl.go", "land.go", "done_composition.go", "close_module.go",
        "governor.go", "yield.go", "claim_module.go", "state.go", "worktree.go",
        "validate.go", "converge.go", "reconcile.go", "membrane.go",
    }
    for filename in removed_sources:
        assert not (ROOT / "cli" / "cmd" / "ao" / filename).exists(), f"old command implementation is live: {filename}"


def check_dispatch_once() -> None:
    path = ROOT / "skills" / "swarm" / "scripts" / "dispatch_once.py"
    spec = importlib.util.spec_from_file_location("cathedral_dispatch_once", path)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    packets = [
        {"packet_id": "one", "write_scope": {"include": ["a"]}},
        {"packet_id": "two", "write_scope": {"include": ["b"]}},
    ]
    calls: list[str] = []

    def executor(packet: dict) -> str:
        calls.append(packet["packet_id"])
        if packet["packet_id"] == "two":
            raise RuntimeError("observed error")
        return "candidate"

    results = module.dispatch_once(packets, executor)
    assert calls == ["one", "two"], f"dispatch count/order mismatch: {calls}"
    assert results[0]["result"] == "candidate"
    assert results[1]["error"]["message"] == "observed error"


def probe_no_substrate_calls() -> None:
    helper = ROOT / "skills" / "validate" / "scripts" / "validate.py"
    with tempfile.TemporaryDirectory() as raw:
        temp = Path(raw)
        subject = temp / "subject"
        subject.mkdir()
        (subject / "value.txt").write_text("candidate\n", encoding="utf-8")
        fake_bin = temp / "bin"
        fake_bin.mkdir()
        called = temp / "called"
        for name in ("git", "ao", "br", "bd", "push", "release"):
            executable = fake_bin / name
            executable.write_text(f"#!/bin/sh\necho {name} >> '{called}'\nexit 97\n", encoding="utf-8")
            executable.chmod(0o755)
        env = dict(os.environ)
        env["PATH"] = str(fake_bin) + os.pathsep + env.get("PATH", "")
        result = subprocess.run(
            [sys.executable, str(helper), "manifest", "--root", str(subject), "--include", "."],
            cwd=temp, env=env, text=True, capture_output=True, check=False,
        )
        assert result.returncode == 0, result.stderr
        payload = json.loads(result.stdout)
        assert payload["schema_version"] == "subject-manifest.v1"
        spec = importlib.util.spec_from_file_location("cathedral_validate", helper)
        assert spec and spec.loader
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        acceptance = "a" * 64
        plan = {
            "schema_version": "plan-packet.v1",
            "acceptance_digest": acceptance,
            "write_scope": {"include": ["value.txt"], "exclude": []},
        }
        candidate = {
            "schema_version": "candidate-packet.v1",
            "plan_packet_digest": module.plan_digest(plan),
            "acceptance_digest": acceptance,
            "subject_manifest_digest": payload["canonical_manifest_digest"],
            "changed_path_coverage_complete": True,
            "actual_changed_paths": ["value.txt"],
        }
        assert module.scope_result(plan, candidate)["result"] == "PASS"
        draft = {
            "acceptance_digest": acceptance,
            "subject_manifest_digest": payload["canonical_manifest_digest"],
            "author_context_id": "non-git-author",
            "validator_context_id": "non-git-validator",
            "freshness_attestation": {"source": "runtime", "attester_identity": "probe"},
            "verdict": "PASS",
            "criteria": [{"id": "acceptance", "result": "PASS", "evidence_refs": ["probe"]}],
            "findings": [],
            "evidence_refs": ["probe"],
            "checked": ["acceptance"],
            "not_checked": [],
            "validated_at": "2026-07-14T00:00:00Z",
        }
        verdict_dir = temp / ".agentops" / "verdicts" / "sha256"
        artifact, verdict_path, existed = module.store_verdict(draft, verdict_dir)
        assert artifact["verdict"] == "PASS" and verdict_path.is_file() and not existed
        assert verdict_path.parent == verdict_dir
        assert not called.exists(), "Validate helper invoked a Git, tracker, push, or delivery executable"


def main() -> int:
    checks = (
        check_skill_graph,
        check_generated_skill_inventory,
        check_core_schemas,
        check_single_pass_contract,
        check_validate_helper,
        check_tombstones,
        check_dispatch_once,
        probe_no_substrate_calls,
    )
    failures: list[str] = []
    for check in checks:
        try:
            check()
        except (AssertionError, OSError, ValueError, json.JSONDecodeError, yaml.YAMLError) as exc:
            failures.append(f"{check.__name__}: {exc}")
    if failures:
        print("Cathedral Cut conformance failed:", file=sys.stderr)
        for failure in failures:
            print(f"- {failure}", file=sys.stderr)
        return 1
    print("Cathedral Cut conformance: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
