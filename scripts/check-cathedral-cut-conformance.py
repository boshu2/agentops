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
    "subject-manifest.v1.schema.json",
    "verdict.v2.schema.json",
    "rpi-report.v1.schema.json",
)
COMPATIBILITY_SCHEMAS = (
    "plan-packet.v1.schema.json",
    "candidate-packet.v1.schema.json",
    "revision-packet.v1.schema.json",
)
PACKET_FREE_NARRATIVE = (
    "GOALS.md",
    "PROGRAM.md",
    "docs/software-factory.md",
    "docs/seed-definition.md",
    "docs/INCIDENT-RUNBOOK.md",
    "docs/templates/README.md",
    "docs/templates/intent-issue.md",
    "docs/templates/slice-validation.md",
)
LEGACY_PACKET_TOKENS = (
    "PlanPacket", "CandidatePacket", "RevisionPacket",
    "plan-packet.v1", "candidate-packet.v1", "revision-packet.v1",
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
    "beads-br", "beads-bv",
}
REMOVED_MORTEM_ALIASES = {
    "pre-mortem", "pre_mortem", "post-mortem", "post_mortem",
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
            assert not deps, f"{name}: only rpi may declare hard dependencies: {sorted(deps)}"
    for name in REMOVED_SKILLS:
        assert not (ROOT / "skills" / name / "SKILL.md").exists(), f"removed skill is live: {name}"
        assert not (ROOT / "skills-codex" / name / "SKILL.md").exists(), f"removed Codex skill is live: {name}"
    for name in REMOVED_MORTEM_ALIASES:
        assert not (ROOT / "skills" / name).exists(), f"removed skill alias is live: {name}"
        assert not (ROOT / "skills-codex" / name).exists(), f"removed Codex alias is live: {name}"
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
    for filename in COMPATIBILITY_SCHEMAS:
        path = ROOT / "schemas" / filename
        assert path.is_file(), f"missing compatibility schema: {filename}"
        schema = json.loads(path.read_text(encoding="utf-8"))
        assert schema.get("deprecated") is True, f"{filename}: compatibility schema is not deprecated"
    verdict = json.loads((ROOT / "schemas" / "verdict.v2.schema.json").read_text())
    assert set(verdict["properties"]["verdict"]["enum"]) == {"PASS", "FAIL", "NOT_PROVEN"}
    for forbidden in ("WARN", "confidence", "disposition", "next_action", "NOT_BUILT", "NOT_PLANNED"):
        assert forbidden not in json.dumps(verdict), f"verdict.v2 retains {forbidden}"
    for filename in RETIRED_SCHEMAS:
        assert not (ROOT / "schemas" / filename).exists(), f"retired schema is live: {filename}"


def check_packet_free_narrative() -> None:
    for relative in PACKET_FREE_NARRATIVE:
        text = (ROOT / relative).read_text(encoding="utf-8")
        advertised = [name for name in LEGACY_PACKET_TOKENS if name in text]
        assert not advertised, f"{relative}: advertises legacy packets {advertised}"

    schemas_doc = (ROOT / "docs" / "SCHEMAS.md").read_text(encoding="utf-8")
    assert "## Legacy compatibility" in schemas_doc, "docs/SCHEMAS.md: no legacy compatibility section"
    current_schema_section = schemas_doc.split("## Legacy compatibility", 1)[0]
    assert not any(name in current_schema_section for name in LEGACY_PACKET_TOKENS), (
        "docs/SCHEMAS.md: legacy packet listed as a current core schema"
    )

    contracts_doc = (ROOT / "docs" / "contracts" / "index.md").read_text(encoding="utf-8")
    assert "Deprecated compatibility contracts" in contracts_doc, (
        "docs/contracts/index.md: compatibility contracts are not labeled deprecated"
    )
    current_contract_section = contracts_doc.split("Deprecated compatibility contracts", 1)[0]
    assert not any(name in current_contract_section for name in LEGACY_PACKET_TOKENS), (
        "docs/contracts/index.md: legacy packet listed as a current public contract"
    )


def check_single_pass_contract() -> None:
    text = (ROOT / "skills" / "rpi" / "SKILL.md").read_text(encoding="utf-8")
    assert "Stop regardless" in text
    runner = ROOT / "skills" / "rpi" / "scripts" / "run_once.py"
    assert runner.is_file(), "RPI has no executable single-pass reference behavior"
    tree = ast.parse(runner.read_text(encoding="utf-8"), filename=str(runner))
    assert not any(isinstance(node, (ast.For, ast.While)) for node in ast.walk(tree)), (
        "RPI reference behavior must not contain a dispatch loop"
    )
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
    spec = importlib.util.spec_from_file_location("cathedral_validate_contract", path)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    with tempfile.TemporaryDirectory() as raw:
        try:
            module.store_verdict({"verdict": "FAIL"}, Path(raw))
        except module.ContractError:
            pass
        else:
            raise AssertionError("Validate persisted an incomplete verdict.v2 draft")
        assert not list(Path(raw).iterdir()), "Validate wrote an invalid verdict artifact"
    with tempfile.TemporaryDirectory() as raw:
        subject = Path(raw)
        (subject / "value").write_text("same", encoding="utf-8")
        first = module.build_manifest(subject, ["."], [], git_metadata={"commit": "one"})
        second = module.build_manifest(subject, ["."], [], git_metadata={"commit": "two"})
        assert first["canonical_manifest_digest"] == second["canonical_manifest_digest"], (
            "optional Git metadata changes subject content identity"
        )


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
    for filename in (
        "closeout.go",
        "inmemory_closeout.go",
        "convergence_check.go",
        "inmemory_convergence_check.go",
    ):
        assert not (ROOT / "cli" / "internal" / "ports" / filename).exists(), (
            f"lifecycle authority port remains live: {filename}"
        )


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
    try:
        module.dispatch_once(
            [
                {"packet_id": "wide", "write_scope": {"include": ["src/**"]}},
                {"packet_id": "nested", "write_scope": {"include": ["src/lib/**"]}},
            ],
            executor,
        )
    except ValueError:
        pass
    else:
        raise AssertionError("dispatch_once accepted overlapping glob scopes")


def probe_no_substrate_calls() -> None:
    helper = ROOT / "skills" / "validate" / "scripts" / "validate.py"
    rpi_runner = ROOT / "skills" / "rpi" / "scripts" / "run_once.py"
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
        rpi_spec = importlib.util.spec_from_file_location("cathedral_rpi", rpi_runner)
        assert rpi_spec and rpi_spec.loader
        rpi = importlib.util.module_from_spec(rpi_spec)
        rpi_spec.loader.exec_module(rpi)
        resolved_intent = {
            "intent_ref": "conversation:cathedral-probe",
            "acceptance": ["value.txt contains candidate"],
            "write_scope": {"include": ["value.txt"], "exclude": []},
        }
        intent_bytes = module.canonical_bytes(resolved_intent)
        subject_facts = {
            "subject_manifest_digest": payload["canonical_manifest_digest"],
            "subject_manifest": payload,
            "checks": ["manifest"],
        }
        draft = {
            "acceptance_digest": "a" * 64,
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
        calls: list[str] = []

        def plan_phase(_intent: object) -> dict:
            calls.append("plan")
            return resolved_intent

        def implement_phase(received_intent: dict) -> dict:
            calls.append("implement")
            assert received_intent == resolved_intent
            return subject_facts

        def validate_phase(received_intent: dict, received_subject: dict) -> dict:
            calls.append("validate")
            assert received_intent == resolved_intent
            assert received_subject == subject_facts
            artifact, verdict_path, existed = module.store_verdict(
                draft,
                verdict_dir,
                intent_bytes,
                payload,
                "non-git-author",
                "PASS",
            )
            assert not existed
            return {
                "verdict": artifact["verdict"],
                "acceptance_digest": artifact["acceptance_digest"],
                "subject_manifest_digest": artifact["subject_manifest_digest"],
                "verdict_digest": artifact["artifact_digest"],
                "verdict_ref": str(verdict_path),
                "checked": artifact["checked"],
                "not_checked": artifact["not_checked"],
            }

        rpi_report = rpi.invoke_once("temporary non-Git experiment", plan_phase, implement_phase, validate_phase)
        verdict_path = Path(rpi_report["verdict_ref"])
        assert calls == ["plan", "implement", "validate"], f"RPI dispatch trace is {calls}"
        assert rpi_report["status"] == "PASS" and verdict_path.is_file()
        assert verdict_path.parent == verdict_dir
        assert not called.exists(), "Validate helper invoked a Git, tracker, push, or delivery executable"


def main() -> int:
    checks = (
        check_skill_graph,
        check_generated_skill_inventory,
        check_core_schemas,
        check_packet_free_narrative,
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
