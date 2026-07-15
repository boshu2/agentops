#!/usr/bin/env python3
"""Generate every skill inventory projection from SKILL.md metadata."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import sys
from typing import Any

import yaml


ROOT = Path(__file__).resolve().parents[1]


def frontmatter(path: Path) -> dict[str, Any]:
    text = path.read_text(encoding="utf-8")
    parts = text.split("---", 2)
    if len(parts) < 3:
        raise ValueError(f"missing frontmatter: {path}")
    value = yaml.safe_load(parts[1]) or {}
    if not isinstance(value, dict):
        raise ValueError(f"frontmatter is not a mapping: {path}")
    return value


def load_entries() -> list[dict[str, Any]]:
    entries = []
    for path in sorted((ROOT / "skills").glob("*/SKILL.md")):
        if path.parent.name.startswith("_"):
            continue
        data = frontmatter(path)
        name = path.parent.name
        if data.get("name") != name:
            raise ValueError(f"skill name/path mismatch: {path}")
        metadata = data.get("metadata") or {}
        required = ("tier", "dependencies", "capabilities", "effects", "canonical_status", "disposition")
        missing = [field for field in required if field not in metadata]
        if missing:
            raise ValueError(f"{path}: metadata missing {', '.join(missing)}")
        entry = {
            "name": name,
            "description": str(data.get("description", "")).strip(),
            "hexagonal_role": data.get("hexagonal_role", "generic"),
            "consumes": data.get("consumes") or [],
            "produces": data.get("produces") or [],
            "dependencies": metadata.get("dependencies") or [],
            "capabilities": metadata.get("capabilities") or [],
            "effects": metadata.get("effects") or [],
            "canonical_status": metadata.get("canonical_status"),
            "disposition": metadata.get("disposition"),
            "tier": metadata.get("tier"),
            "context_rel": data.get("context_rel") or [],
            "practices": data.get("practices") or [],
            "user_invocable": bool(data.get("user-invocable", False)),
            "graph_root": bool(metadata.get("graph_root", False)),
            "references_count": len(list((path.parent / "references").glob("*"))) if (path.parent / "references").is_dir() else 0,
            "codex_override_present": (ROOT / "skills-codex" / name / "SKILL.md").exists(),
        }
        entries.append(entry)
    validate_graph(entries)
    return entries


def validate_graph(entries: list[dict[str, Any]]) -> None:
    names = {entry["name"] for entry in entries}
    if len(names) != len(entries):
        raise ValueError("duplicate skill name")
    for entry in entries:
        for dep in entry["dependencies"]:
            if dep not in names:
                raise ValueError(f"dangling dependency: {entry['name']} -> {dep}")
    core = {entry["name"]: set(entry["dependencies"]) for entry in entries if entry["name"] in {"rpi", "plan", "implement", "validate"}}
    expected = {"rpi": {"plan", "implement", "validate"}, "plan": set(), "implement": set(), "validate": set()}
    if core != expected:
        raise ValueError(f"core dependency graph mismatch: {core!r}")


def catalog(entries: list[dict[str, Any]]) -> dict[str, Any]:
    return {"schema_version": "3", "skill_count": len(entries), "skills": entries}


def registry(entries: list[dict[str, Any]]) -> dict[str, Any]:
    skills = [
        {
            "name": entry["name"],
            "tier": entry["tier"],
            "path": f"skills/{entry['name']}/",
            "has_skill_md": True,
            "has_references": entry["references_count"] > 0,
            "reference_count": entry["references_count"],
            "disposition": entry["disposition"],
            "capabilities": entry["capabilities"],
            "effects": entry["effects"],
        }
        for entry in entries
    ]
    capabilities = [
        {
            "id": f"skill:{entry['name']}:{capability}",
            "type": "skill",
            "name": capability,
            "path": f"skills/{entry['name']}/SKILL.md",
            "driven_by_skills": [entry["name"]],
        }
        for entry in entries
        for capability in entry["capabilities"]
    ]
    return {
        "schema_version": 3,
        "summary": {"skills": len(skills), "hooks": 0, "knowledge_stores": 0, "job_types": 0, "eval_files": 0, "cli_commands": 0, "workflows": 0, "capabilities": len(capabilities)},
        "surfaces": {"skills": skills, "hooks": [], "knowledge_stores": [], "job_types": [], "schedules": [], "evals": [], "cli_commands": [], "workflows": []},
        "capabilities": capabilities,
        "capability_summary": {"total": len(capabilities), "skills": len(capabilities), "cli_commands": 0, "gates": 0, "reference_impls": 0},
        "cli_top_level_commands": [],
        "cadence_recommendations": [],
    }


def md_table(entries: list[dict[str, Any]]) -> str:
    rows = ["| Skill | Tier | Disposition | Hard dependencies | Capabilities | Effects |", "|---|---|---|---|---|---|"]
    for entry in entries:
        rows.append(
            "| `{}` | {} | `{}` | {} | {} | {} |".format(
                entry["name"], entry["tier"], entry["disposition"],
                ", ".join(f"`{value}`" for value in entry["dependencies"]) or "-",
                ", ".join(f"`{value}`" for value in entry["capabilities"]) or "-",
                ", ".join(f"`{value}`" for value in entry["effects"]) or "-",
            )
        )
    return "\n".join(rows)


def router(entries: list[dict[str, Any]]) -> str:
    groups: dict[str, list[str]] = {}
    for entry in entries:
        groups.setdefault(entry["disposition"], []).append(entry["name"])
    lines = ["<!-- generated from skills/*/SKILL.md metadata -->", "", "# Skill Router", "", f"{len(entries)} live skills. Metadata is the sole inventory and graph source.", ""]
    for disposition in ("keep", "keep_off_path", "keep_strategy", "keep_optional_adapter", "keep_specialist"):
        lines += [f"## {disposition}", "", ", ".join(f"`{name}`" for name in sorted(groups.get(disposition, []))) or "(none)", ""]
    lines += ["## Complete inventory", "", md_table(entries), ""]
    return "\n".join(lines)


def tiers(entries: list[dict[str, Any]]) -> str:
    groups: dict[str, list[str]] = {}
    for entry in entries:
        groups.setdefault(entry["tier"], []).append(entry["name"])
    lines = ["<!-- generated from skills/*/SKILL.md metadata -->", "", "# Skill tiers", ""]
    for tier in sorted(groups):
        lines += [f"## {tier}", "", ", ".join(f"`{name}`" for name in sorted(groups[tier])), ""]
    lines += ["## Inventory", "", md_table(entries), ""]
    return "\n".join(lines)


def domain_map(entries: list[dict[str, Any]]) -> str:
    roles: dict[str, list[str]] = {}
    for entry in entries:
        roles.setdefault(entry["hexagonal_role"], []).append(entry["name"])
    lines = ["<!-- generated from skills/*/SKILL.md metadata -->", "", "# AgentOps skill domain map", ""]
    for role in sorted(roles):
        lines += [f"## {role}", "", ", ".join(f"`{name}`" for name in sorted(roles[role])), ""]
    lines += ["## Inventory", "", md_table(entries), ""]
    return "\n".join(lines)


def graph(entries: list[dict[str, Any]]) -> str:
    lines = ["<!-- generated from skills/*/SKILL.md metadata -->", "", "# AgentOps skill graph", "", "```mermaid", "graph LR"]
    for entry in entries:
        node = entry["name"].replace("-", "_")
        lines.append(f'  {node}["{entry["name"]}"]')
    for entry in entries:
        source = entry["name"].replace("-", "_")
        for dep in sorted(entry["dependencies"]):
            lines.append(f"  {source} --> {dep.replace('-', '_')}")
    lines += ["```", "", "Hard dependencies only. Optional context relationships are listed in the context map.", ""]
    return "\n".join(lines)


def context_map(entries: list[dict[str, Any]]) -> str:
    lines = ["<!-- generated from skills/*/SKILL.md metadata -->", "", "# AgentOps context map", "", "## Hard dependencies", "", "| Source | Target |", "|---|---|"]
    for entry in entries:
        for dep in sorted(entry["dependencies"]):
            lines.append(f"| `{entry['name']}` | `{dep}` |")
    lines += ["", "## Optional context relationships", "", "| Source | Kind | Target |", "|---|---|---|"]
    for entry in entries:
        for rel in sorted(entry["context_rel"], key=lambda item: (str(item.get("kind")), str(item.get("with")))):
            lines.append(f"| `{entry['name']}` | `{rel.get('kind', '')}` | `{rel.get('with', '')}` |")
    lines += ["", "## Data flow", "", "| Skill | Direction | Artifact |", "|---|---|---|"]
    for entry in entries:
        for value in entry["consumes"]:
            lines.append(f"| `{entry['name']}` | consumes | `{value}` |")
        for value in entry["produces"]:
            lines.append(f"| `{entry['name']}` | produces | `{value}` |")
    lines.append("")
    return "\n".join(lines)


def outputs(entries: list[dict[str, Any]]) -> dict[Path, bytes]:
    return {
        ROOT / "skills" / "catalog.json": (json.dumps(catalog(entries), indent=2, sort_keys=True) + "\n").encode(),
        ROOT / "registry.json": (json.dumps(registry(entries), indent=2, sort_keys=True) + "\n").encode(),
        ROOT / "docs" / "SKILL-ROUTER.md": router(entries).encode(),
        ROOT / "docs" / "SKILLS.md": router(entries).encode(),
        ROOT / "skills" / "SKILL-TIERS.md": tiers(entries).encode(),
        ROOT / "docs" / "reference" / "agentops-skill-domain-map.md": domain_map(entries).encode(),
        ROOT / "docs" / "reference" / "agentops-skill-graph.md": graph(entries).encode(),
        ROOT / "docs" / "contracts" / "context-map.md": context_map(entries).encode(),
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--print", choices=["catalog", "registry", "router", "tiers", "domain", "graph", "context"])
    parser.add_argument("--out")
    args = parser.parse_args()
    try:
        entries = load_entries()
        generated = outputs(entries)
        named = {
            "catalog": ROOT / "skills" / "catalog.json",
            "registry": ROOT / "registry.json",
            "router": ROOT / "docs" / "SKILL-ROUTER.md",
            "tiers": ROOT / "skills" / "SKILL-TIERS.md",
            "domain": ROOT / "docs" / "reference" / "agentops-skill-domain-map.md",
            "graph": ROOT / "docs" / "reference" / "agentops-skill-graph.md",
            "context": ROOT / "docs" / "contracts" / "context-map.md",
        }
        if args.print:
            payload = generated[named[args.print]]
            if args.out:
                Path(args.out).write_bytes(payload)
            else:
                sys.stdout.buffer.write(payload)
            return 0
        drift = []
        for path, payload in generated.items():
            if args.check:
                if not path.exists() or path.read_bytes() != payload:
                    drift.append(path.relative_to(ROOT).as_posix())
            else:
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(payload)
        if drift:
            for path in drift:
                print(f"DRIFT: {path}", file=sys.stderr)
            return 1
        print(f"skill mesh: {'up to date' if args.check else 'generated'} ({len(entries)} skills)")
        return 0
    except (OSError, ValueError, yaml.YAMLError) as exc:
        print(f"generate-skill-mesh: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
