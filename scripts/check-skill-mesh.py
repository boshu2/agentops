#!/usr/bin/env python3
"""Check that every skill projection agrees with SKILL.md metadata.

This gate is the single owner of the skill dependency graph (RPI depends on
the three core operations; no other skill declares hard dependencies) and of
acceptance authority (one verdict.v2 producer; advisory skills cannot judge).
"""

from __future__ import annotations

import json
from pathlib import Path
import subprocess
import sys

import yaml


ROOT = Path(__file__).resolve().parents[1]
CORE = {"rpi", "plan", "implement", "validate"}
EXPECTED_CORE = {
    "rpi": {"plan", "implement", "validate"},
    "plan": set(),
    "implement": set(),
    "validate": set(),
}
DISPOSITIONS = {
    "keep",
    "keep_off_path",
    "keep_strategy",
    "keep_optional_adapter",
    "keep_specialist",
}


def frontmatter(path: Path) -> dict:
    parts = path.read_text(encoding="utf-8").split("---", 2)
    if len(parts) < 3:
        raise ValueError(f"missing frontmatter: {path}")
    value = yaml.safe_load(parts[1]) or {}
    if not isinstance(value, dict):
        raise ValueError(f"frontmatter is not a mapping: {path}")
    return value


def fail(message: str, failures: list[str]) -> None:
    failures.append(message)


def main() -> int:
    failures: list[str] = []
    skills: dict[str, dict] = {}
    for path in sorted((ROOT / "skills").glob("*/SKILL.md")):
        name = path.parent.name
        data = frontmatter(path)
        metadata = data.get("metadata") or {}
        if data.get("name") != name:
            fail(f"name/path mismatch: {path}", failures)
        if metadata.get("disposition") not in DISPOSITIONS:
            fail(f"missing or invalid disposition: {name}", failures)
        for field in ("tier", "dependencies", "capabilities", "effects", "canonical_status"):
            if field not in metadata:
                fail(f"missing metadata.{field}: {name}", failures)
        skills[name] = data

    names = set(skills)
    for name, data in skills.items():
        metadata = data.get("metadata") or {}
        for dependency in metadata.get("dependencies") or []:
            if dependency not in names:
                fail(f"dangling dependency: {name} -> {dependency}", failures)

    core_graph = {
        name: set((skills.get(name, {}).get("metadata") or {}).get("dependencies") or [])
        for name in CORE
    }
    if core_graph != EXPECTED_CORE:
        fail(f"core dependency graph mismatch: {core_graph!r}", failures)
    for name, data in sorted(skills.items()):
        dependencies = (data.get("metadata") or {}).get("dependencies") or []
        if name != "rpi" and dependencies:
            fail(f"only rpi may declare hard dependencies: {name} -> {sorted(dependencies)}", failures)

    verdict_producers = sorted(
        name for name, data in skills.items() if "verdict.v2" in (data.get("produces") or [])
    )
    if len(verdict_producers) != 1:
        fail(f"exactly one skill must produce verdict.v2, found {verdict_producers}", failures)
    for name in sorted(skills):
        data = skills[name]
        metadata = data.get("metadata") or {}
        judges = "judge_acceptance" in (metadata.get("capabilities") or [])
        if name in verdict_producers and not judges:
            fail(f"verdict.v2 producer does not declare judge_acceptance: {name}", failures)
        if judges and not (data.get("produces") or []) and not (metadata.get("effects") or []):
            fail(f"advisory skill (no produces, no effects) declares judge_acceptance: {name}", failures)

    catalog = json.loads((ROOT / "skills/catalog.json").read_text(encoding="utf-8"))
    catalog_names = [entry.get("name") for entry in catalog.get("skills", [])]
    if set(catalog_names) != names or len(catalog_names) != len(names):
        fail("skills/catalog.json inventory does not equal source metadata", failures)
    if catalog.get("skill_count") != len(names):
        fail("skills/catalog.json skill_count is stale", failures)

    registry = json.loads((ROOT / "registry.json").read_text(encoding="utf-8"))
    registry_names = {
        entry.get("name") for entry in registry.get("surfaces", {}).get("skills", [])
    }
    if registry_names != names:
        fail("registry.json skill inventory does not equal source metadata", failures)

    overrides = json.loads(
        (ROOT / "skills-codex-overrides/catalog.json").read_text(encoding="utf-8")
    )
    override_names = {entry.get("name") for entry in overrides.get("skills", [])}
    if override_names != names:
        fail("Codex override catalog does not equal source metadata", failures)

    generated = subprocess.run(
        [sys.executable, str(ROOT / "scripts/generate-skill-mesh.py"), "--check"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    if generated.returncode:
        fail(generated.stderr.strip() or generated.stdout.strip() or "skill projections drifted", failures)

    if failures:
        for message in failures:
            print(f"FAIL: {message}", file=sys.stderr)
        return 1
    print(f"skill mesh: PASS ({len(names)} skills; metadata is authoritative)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
