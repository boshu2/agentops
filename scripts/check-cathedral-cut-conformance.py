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
        assert not (ROOT / "skills-codex" / name / "SKILL.md").exists(), f"removed Codex skill is live: {name}"
    for name in REMOVED_MORTEM_ALIASES:
        assert not (ROOT / "skills" / name).exists(), f"removed skill alias is live: {name}"
        assert not (ROOT / "skills-codex" / name).exists(), f"removed Codex alias is live: {name}"


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


def check_bounded_repair_contract() -> None:
    """RPI's reference adapter stops under the convergence law (ADR-0017).

    Behavior probes only: fixture validation rounds in, stop reason out. Every
    stop reason `run_once.py` declares must be reached by a probe, a round past
    the caller's bound is never consumed, and the one bounded experiment
    dispatches each phase once, on FAIL as on PASS.
    """
    runner = ROOT / "skills" / "rpi" / "scripts" / "run_once.py"
    assert runner.is_file(), "RPI has no executable reference behavior"
    spec = importlib.util.spec_from_file_location("rpi_run_once_canary", runner)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    dg = lambda ch: ch * 64  # noqa: E731
    def leg(status, ids, digest, evidence=("acceptance-receipt",), classes=None):
        classes = classes or {}
        return {
            "status": status,
            "findings": [
                {"id": i, "summary": i, **({"class": classes[i]} if i in classes else {})}
                for i in ids
            ],
            "subject_digest": digest,
            "evidence_refs": list(evidence),
            "validator_family": "fresh",
            "checked": ["acceptance"],
            "not_checked": [],
        }
    fixed_a = {"ref": "fixed-a", "subject_digest": dg("b"), "resolves": ["a"]}
    progressing = [leg("FAIL", ["a", "b"], dg("a")), leg("FAIL", ["b"], dg("b"), [fixed_a])]
    canaries = {
        "repair_budget_exhausted": (progressing + [leg("FAIL", ["b"], dg("c"))], {"repair_rounds": 1}),
        "new_finding_requires_causal_review": (
            [leg("FAIL", ["a"], dg("a")), leg("FAIL", ["b"], dg("b"), [fixed_a])], {"repair_rounds": 2}),
        "reopened_finding": (progressing + [leg("FAIL", ["a"], dg("c"))], {"repair_rounds": 3}),
        "recurring_finding_class": ([
            leg("FAIL", ["a", "b"], dg("a"), classes={"a": "race", "b": "docs"}),
            leg("FAIL", ["b"], dg("b"), [fixed_a], classes={"b": "docs"}),
            leg("FAIL", ["c"], dg("c"), classes={"c": "race"}),
        ], {"repair_rounds": 3}),
        "no_acceptance_progress": ([leg("FAIL", ["a"], dg("a")), leg("PASS", [], dg("b"))], {"repair_rounds": 2}),
        "introduced_regression": ([leg("FAIL", ["a"], dg("a")), leg("FAIL", ["b"], dg("b"), [
            fixed_a, {"ref": "comparison", "subject_digest": dg("b"), "introduced": ["b"]}])], {"repair_rounds": 2}),
        "not_converged": ([leg("FAIL", ["a"], dg("a")), leg("FAIL", ["b", "c"], dg("b"), [
            fixed_a, {"ref": "prior-reproduction", "subject_digest": dg("a"), "preexisting": ["b", "c"]}])],
            {"repair_rounds": 2}),
        # A selected cross-family leg that is missing cannot converge.
        "diversity_unsatisfied": ([leg("PASS", [], dg("a"))], {"cross_model": True}),
        "converged": ([leg("FAIL", ["a"], dg("a")), leg("PASS", [], dg("b"), [fixed_a])], {"repair_rounds": 2}),
    }
    unprobed = set(module.STOP_REASONS) - set(canaries)
    assert not unprobed, f"declared stop reasons without a behavior probe: {sorted(unprobed)}"
    for expected, (rounds, options) in canaries.items():
        outcome = module.run_repair_phase(rounds, **options)
        assert outcome["stop_reason"] == expected, (
            f"law canary {expected}: reference behavior stopped with {outcome['stop_reason']!r}"
        )
    diverse = module.run_repair_phase(canaries["diversity_unsatisfied"][0], cross_model=True)
    assert diverse["report"]["status"] == "NOT_PROVEN", "a single-family PASS certified a cross-family selection"
    for digest in (dg("a"), dg("b")):
        flip = module.run_repair_phase([leg("FAIL", ["a"], dg("a")), leg("PASS", [], digest)])
        assert flip["report"]["status"] == "NOT_PROVEN", "byte or verdict movement alone must not certify progress"
    # A poison round beyond the caller's bound must never be normalized.
    poison = module.run_repair_phase(progressing + [{"status": "poison-not-a-round"}], repair_rounds=1)
    assert poison["stop_reason"] == "repair_budget_exhausted" and poison["rounds_used"] == 1, (
        "the repair phase consumed a round past the caller's bound"
    )
    assert module.run_repair_phase([leg("FAIL", ["a"], dg("a"))], repair_rounds=0)["stop_reason"] == "repair_budget_exhausted"

    # The one bounded experiment never re-dispatches a phase after FAIL.
    calls: list[str] = []

    def phase(name: str, result: dict):
        def run(*_args: object) -> dict:
            calls.append(name)
            return result
        return run

    failed = module.invoke_once(
        "fail-path probe",
        phase("anti-ceremony", {
            "decision": "CONTINUE",
            "reason": "The probe needs one failing traversal.",
            "frozen_outcome": "Observe one FAIL traversal",
            "parked_process_work": [],
            "remaining_proof": ["fresh validation"],
            "stop_condition": "Stop after one fresh validation result.",
        }),
        phase("plan", {"intent_ref": "probe", "acceptance_digest": dg("d")}),
        phase("implement", {"subject_manifest_digest": dg("e")}),
        phase("validate", {
            "verdict": "FAIL",
            "acceptance_digest": dg("d"),
            "subject_manifest_digest": dg("e"),
            "author_context_id": "probe-author",
            "validator_context_id": "probe-validator",
            "freshness_attestation": {"source": "runtime", "attester_identity": "probe"},
        }),
    )
    assert calls == ["anti-ceremony", "plan", "implement", "validate"], f"FAIL dispatch trace is {calls}"
    assert failed["status"] == "FAIL", f"FAIL traversal reported {failed['status']!r}"


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
        # The intent SOURCE is bytes; the acceptance identity is sha256 of those
        # bytes; the resolved mapping carries that identity as a declared fact.
        # Deriving the bytes from the mapping that already contains the digest
        # would be circular, and folding the two together is what let the
        # RPI/Validate digest disagreement hide: this probe used to set
        # `intent_bytes = canonical_bytes(resolved_intent)`, which is precisely
        # the one input where a canonical-JSON digest of the mapping and a byte
        # digest of the source coincide. Keeping them separate means the probe
        # exercises the identity rather than a coincidence.
        intent_source = {
            "intent_ref": "conversation:cathedral-probe",
            "acceptance": ["value.txt contains candidate"],
            "write_scope": {"include": ["value.txt"], "exclude": []},
        }
        intent_bytes = module.canonical_bytes(intent_source)
        resolved_intent = {
            **intent_source,
            "acceptance_digest": hashlib.sha256(intent_bytes).hexdigest(),
        }
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
        verdict_dir = temp / ".agents" / "ao" / "verdicts" / "sha256"
        calls: list[str] = []

        def anti_ceremony_guard(_intent: object) -> dict:
            calls.append("anti-ceremony")
            return {
                "decision": "CONTINUE",
                "reason": "The frozen outcome still requires implementation proof.",
                "frozen_outcome": "Write and prove the non-Git candidate",
                "parked_process_work": [],
                "remaining_proof": ["manifest", "fresh validation"],
                "stop_condition": "Stop after one fresh validation result.",
            }

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
                "non-git-validator",
                "runtime",
                "non-git-validator",
            )
            assert not existed
            return {
                "verdict": artifact["verdict"],
                "acceptance_digest": artifact["acceptance_digest"],
                "subject_manifest_digest": artifact["subject_manifest_digest"],
                "verdict_digest": artifact["artifact_digest"],
                "verdict_ref": str(verdict_path),
                "author_context_id": artifact["author_context_id"],
                "validator_context_id": artifact["validator_context_id"],
                "freshness_attestation": artifact["freshness_attestation"],
                "checked": artifact["checked"],
                "not_checked": artifact["not_checked"],
            }

        rpi_report = rpi.invoke_once(
            "temporary non-Git experiment",
            anti_ceremony_guard,
            plan_phase,
            implement_phase,
            validate_phase,
        )
        verdict_path = Path(rpi_report["verdict_ref"])
        assert calls == ["anti-ceremony", "plan", "implement", "validate"], f"RPI dispatch trace is {calls}"
        assert rpi_report["status"] == "PASS" and verdict_path.is_file()
        assert verdict_path.parent == verdict_dir
        assert not called.exists(), "Validate helper invoked a Git, tracker, push, or delivery executable"


def main() -> int:
    checks = (
        check_removed_skills,
        check_core_schemas,
        check_schema_index_docs,
        check_bounded_repair_contract,
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
