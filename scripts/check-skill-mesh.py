#!/usr/bin/env python3
import argparse
import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]


def frontmatter(path: Path) -> dict:
    text = path.read_text(encoding="utf-8")
    parts = text.split("---", 2)
    if len(parts) < 3:
        raise ValueError(f"missing frontmatter: {path}")
    return yaml.safe_load(parts[1]) or {}


def load_skills() -> dict[str, dict]:
    result = {}
    for path in sorted((ROOT / "skills").glob("*/SKILL.md")):
        if path.parent.name.startswith("_"):
            continue
        result[path.parent.name] = frontmatter(path)
    return result


def dependencies(data: dict) -> list[str]:
    metadata = data.get("metadata") or {}
    value = metadata.get("dependencies") or []
    return [str(item) for item in value] if isinstance(value, list) else []


def context_targets(data: dict) -> set[str]:
    return {str(item.get("with")) for item in data.get("context_rel", []) if isinstance(item, dict) and item.get("with")}


def historical() -> dict:
    data = yaml.safe_load((ROOT / "docs/contracts/skill-dispositions.yaml").read_text(encoding="utf-8"))
    return data.get("historical", {}), data.get("dispositions", [])


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--optional-edge", action="append", default=[])
    parser.add_argument("--independent", action="append", default=[])
    parser.add_argument("--retired", action="append", default=[])
    parser.add_argument("--require-reachable", default="")
    args = parser.parse_args()
    skills = load_skills()
    failures = []

    for pair in args.optional_edge:
        source, target = pair.split(":", 1)
        if source not in skills or target not in skills:
            failures.append(f"optional edge names missing skill: {pair}")
            continue
        if target not in context_targets(skills[source]):
            failures.append(f"optional context edge absent: {pair}")
        if target in dependencies(skills[source]):
            failures.append(f"optional edge became hard dependency: {pair}")

    for pair in args.independent:
        left, right = pair.split(":", 1)
        if left not in skills or right not in skills:
            failures.append(f"independence names missing skill: {pair}")
            continue
        if right in dependencies(skills[left]) or left in dependencies(skills[right]):
            failures.append(f"substrates have a hard mutual/direct dependency: {pair}")

    history, active = historical()
    active_names = {row.get("skill") for row in active}
    for pair in args.retired:
        old, target = pair.split(":", 1)
        row = history.get(old, {})
        if (ROOT / "skills" / old / "SKILL.md").exists() or (ROOT / "skills-codex" / old / "SKILL.md").exists():
            failures.append(f"retired source still exists: {old}")
        if old in active_names:
            failures.append(f"retired source still active in disposition ledger: {old}")
        if row.get("state") != "merged-into" or row.get("merged-into") != target:
            failures.append(f"historical redirect mismatch: {old} -> {target}")

    required = [name for name in args.require_reachable.split(",") if name]
    inbound = {name: [] for name in skills}
    for source, data in skills.items():
        for target in dependencies(data):
            if target in inbound:
                inbound[target].append(source)
    roots = {name for name, data in skills.items() if (data.get("metadata") or {}).get("graph_root") is True}
    reachable = set(roots)
    stack = list(roots)
    while stack:
        source = stack.pop()
        for target in dependencies(skills[source]):
            if target in skills and target not in reachable:
                reachable.add(target)
                stack.append(target)
    for name in required:
        if name not in skills:
            failures.append(f"required skill absent: {name}")
        elif not inbound[name]:
            failures.append(f"new capability has no inbound entry-point dependency: {name}")
        elif name not in reachable:
            failures.append(f"new capability is not reachable from an explicit graph root: {name}")

    if failures:
        for failure in failures:
            print(f"FAIL: {failure}", file=sys.stderr)
        return 1
    print("skill mesh: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
