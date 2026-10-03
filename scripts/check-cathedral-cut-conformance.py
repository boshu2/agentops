#!/usr/bin/env python3
"""Structural conformance for the Cathedral Cut product boundary.

Every check reads a fact (a JSON schema, a file's existence, a link target, a
module's behavior on fixture input), never prose wording. The skill-dependency
graph is owned by `scripts/check-skill-mesh.py`, not by this gate.
"""

from __future__ import annotations

import ast
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parents[1]
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
# Docs that list schemas for readers; current schemas must precede deprecated
# ones and every linked schema file must exist.
SCHEMA_INDEX_DOCS = ("docs/SCHEMAS.md", "docs/contracts/index.md")
REPO_BLOB_PREFIX = "https://github.com/boshu2/agentops/blob/main/"
MARKDOWN_INLINE_DESTINATION = re.compile(r"\]\(\s*<?([^)\s>]+)>?")
SCHEMA_PATH = re.compile(r"(?:^|/)schemas/([^/]+\.schema\.json)$")
FORBIDDEN_STATE = {
    "owner", "ready", "claim", "priority", "attempt", "attempts", "queue",
    "lease", "admission", "next_action", "next-action", "close", "closure",
    "release", "delivery", "budget", "retry", "retries",
}
FORBIDDEN_SCHEMA_STATE = {
    "retry", "retries", "budget", "queue", "claim", "lease", "admission",
    "next_action", "next-action", "closure", "release", "delivery",
}
# verdict.v2 judges through its PASS/FAIL/NOT_PROVEN enum alone; it carries no
# confidence score (ADR-0005: no confidence score certifies), disposition, or
# next action.
FORBIDDEN_VERDICT_PROPERTIES = {"confidence", "disposition", "next_action"}
# Wave dispatch is now an implementation mode; no separate crank/swarm root.
REMOVED_SKILLS = {
    "discovery", "behavior-first-planning", "goal-design", "converge",
    "evolve", "gc-membrane", "pawl-review", "push", "release", "pr-prep",
    "beads-br", "beads-bv",
    # ADR-0018 (Train 2 retirements): goals was a dead alias for fitness, shared
    # a tombstone with no consumer, and scope's checks folded into plan.
    # Reintroducing any of the three fails this gate.
    "goals", "shared", "scope",
    "product", "one-way-door", "anti-ceremony", "codebase-recon", "pattern-mining",
    "learn", "toil-mining", "bootstrap", "handoff", "scaffold", "workflow-builder",
    "automation-shape-routing", "crank", "converter", "operationalize", "standards",
    "fitness", "status", "route", "human-only-skills", "swarm",
}
REMOVED_MORTEM_ALIASES = {
    "pre-mortem", "pre_mortem", "post-mortem", "post_mortem",
}


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


def check_removed_skills() -> None:
    for name in REMOVED_SKILLS:
        assert not (ROOT / "skills" / name / "SKILL.md").exists(), f"removed skill is live: {name}"
    for name in REMOVED_MORTEM_ALIASES:
        assert not (ROOT / "skills" / name).exists(), f"removed skill alias is live: {name}"


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
    verdict = json.loads((ROOT / "schemas" / "verdict.v2.schema.json").read_text(encoding="utf-8"))
    assert set(verdict["properties"]["verdict"]["enum"]) == {"PASS", "FAIL", "NOT_PROVEN"}
    for enum in verdict_enums(verdict):
        if "PASS" in enum:
            assert set(enum) == {"PASS", "FAIL", "NOT_PROVEN"}, f"verdict.v2 enum {enum} is not the tri-state"
    bad = property_names(verdict).intersection(FORBIDDEN_VERDICT_PROPERTIES)
    assert not bad, f"verdict.v2 retains {sorted(bad)}"


def verdict_enums(value: object) -> list[list]:
    """Every `enum` list anywhere in a schema (verdict and criteria[].result alike)."""
    found: list[list] = []
    if isinstance(value, dict):
        if isinstance(value.get("enum"), list):
            found.append(value["enum"])
        for child in value.values():
            found.extend(verdict_enums(child))
    elif isinstance(value, list):
        for child in value:
            found.extend(verdict_enums(child))
    return found


def schema_link_targets(text: str) -> list[str]:
    """Return the schemas/*.schema.json files a Markdown doc links, in order."""
    targets: list[str] = []
    for destination in MARKDOWN_INLINE_DESTINATION.findall(text):
        path = destination.split("#", 1)[0]
        if path.startswith(REPO_BLOB_PREFIX):
            path = path[len(REPO_BLOB_PREFIX):]
        elif "://" in path:
            continue
        match = SCHEMA_PATH.search(path)
        if match:
            targets.append(match.group(1))
    return targets


# Schema docs that separate current and legacy schemas by `##` section: each
# section must be all-current or all-deprecated.
SECTIONED_SCHEMA_DOCS = ("docs/SCHEMAS.md",)


def check_schema_index_docs() -> None:
    """Schema docs never present a deprecated schema as current.

    The deprecated set is the schemas' own top-level `deprecated: true`; every
    linked schema must exist. A Markdown block (a table, list or paragraph:
    consecutive non-blank lines) may not mix deprecated and current schemas,
    and no current block may follow a deprecated one.
    """
    deprecated = {
        path.name
        for path in (ROOT / "schemas").glob("*.schema.json")
        if json.loads(path.read_text(encoding="utf-8")).get("deprecated") is True
    }
    for relative in SCHEMA_INDEX_DOCS:
        targets = schema_link_targets((ROOT / relative).read_text(encoding="utf-8"))
        assert targets, f"{relative}: links no schemas/*.schema.json file"
        missing = [name for name in targets if not (ROOT / "schemas" / name).is_file()]
        assert not missing, f"{relative}: links missing schemas {missing}"
        text = (ROOT / relative).read_text(encoding="utf-8")
        first_deprecated = None
        for block in re.split(r"\n\s*\n", text):
            names = schema_link_targets(block)
            old = [name for name in names if name in deprecated]
            new = [name for name in names if name not in deprecated]
            assert not (old and new), f"{relative}: one block mixes deprecated {old} with current {new}"
            if old:
                first_deprecated = first_deprecated or old[0]
            elif new and first_deprecated is not None:
                raise AssertionError(
                    f"{relative}: current schema {new[0]} is listed after deprecated {first_deprecated}"
                )
        if relative in SECTIONED_SCHEMA_DOCS:
            for section in re.split(r"(?m)^## ", text)[1:]:
                names = schema_link_targets(section)
                old = [name for name in names if name in deprecated]
                new = [name for name in names if name not in deprecated]
                heading = section.splitlines()[0].strip()
                assert not (old and new), (
                    f"{relative}: section '{heading}' mixes deprecated {old} with current {new}"
                )


def check_validate_helper() -> None:
    path = ROOT / "skills" / "validate" / "tests" / "validate.py"
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


def check_dispatch_once() -> None:
    path = ROOT / "scripts" / "swarm" / "dispatch_once.py"
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
    helper = ROOT / "skills" / "validate" / "tests" / "validate.py"
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
        # Validate's acceptance identity is the sha256 of the intent source bytes.
        intent_source = {
            "intent_ref": "conversation:cathedral-probe",
            "acceptance": ["value.txt contains candidate"],
            "write_scope": {"include": ["value.txt"], "exclude": []},
        }
        intent_bytes = module.canonical_bytes(intent_source)
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
        verdict_dir = temp / ".agents" / "ao" / "verdicts" / "sha256"

        # The in-process call must see the fake executables too, or a Git or
        # tracker call from the helper would reach the real binary unnoticed.
        saved_path = os.environ.get("PATH", "")
        os.environ["PATH"] = env["PATH"]
        try:
            artifact, verdict_path, existed = module.store_verdict(
                draft,
                verdict_dir,
                intent_bytes,
                payload,
                "non-git-author",
                "PASS",
                "non-git-validator",
                "runtime",
                "non-git-validator",
            )
        finally:
            os.environ["PATH"] = saved_path
        assert not existed
        assert artifact["verdict"] == "PASS" and verdict_path.is_file()
        assert verdict_path.parent == verdict_dir
        assert not called.exists(), "Validate helper invoked a Git, tracker, push, or delivery executable"


def main() -> int:
    checks = (
        check_removed_skills,
        check_core_schemas,
        check_schema_index_docs,
        check_validate_helper,
        check_dispatch_once,
        probe_no_substrate_calls,
    )
    failures: list[str] = []
    for check in checks:
        try:
            check()
        except (AssertionError, OSError, ValueError, json.JSONDecodeError) as exc:
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
