#!/usr/bin/env python3
"""Audit generated Codex skills for semantic drift that simple text rewrites miss."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


RULES = [
    {
        "code": "TASK_PRIMITIVE",
        "patterns": [
            r"\bTaskCreate\b",
            r"\bTaskList\b",
            r"\bTaskUpdate\b",
            r"\bTaskGet\b",
            r"\bTaskStop\b",
            r"\bUSE THE TASK TOOL\b",
            r"\bTool:\s*Task(?:Create|Update)?\b",
            r'subagent_type:\s*"Explore"',
        ],
        "ignore_patterns": [
            r"Claude-era primitives",
            r"generated Codex skill still contains",
        ],
        "summary": "Generated Codex body still references Claude-era task primitives.",
    },
    {
        "code": "CLAUDE_BACKEND_REF",
        "patterns": [
            r"backend-claude-teams\.md",
            r"\bclaude agents\b",
            r"\bClaude teams\b",
        ],
        "summary": "Generated Codex body still points at Claude backend artifacts.",
    },
    {
        "code": "DUPLICATE_RUNTIME_REWRITE",
        "patterns": [
            r"Codex sub-agents in Codex sessions, Codex sub-agents in Codex sessions",
            r"Codex session -> Codex sub-agents; Codex session -> Codex sub-agents",
        ],
        "summary": "Mechanical rewrite duplicated the runtime phrase and needs a manual Codex body fix.",
    },
    {
        "code": "CLAUDE_PRIMITIVE_LEAKAGE",
        "patterns": [
            r"\bAskUserQuestion\b",
            r"\bread_file\b",
            r"\bSendMessage\b",
            r"\bTeamCreate\b",
            r"\bTeamDelete\b",
            r"\bclaude-code-latest-features\b",
            r"role:\s*explorer\b",
        ],
        "ignore_patterns": [
            r"(?i)unlike\s+Claude",
            r"(?i)Claude['.]s\s+\w+",
            r"(?i)not\s+(?:use|available|supported)\b",
            r"(?i)do\s+not\s+use\b",
            r"(?i)instead\s+of\b",
            r"(?i)replaced?\s+by\b",
            r"(?i)what\s+NOT\s+to\s+use",
            r"^\s*#",
            r"//\s+",
            r"skill-builder",
            r"\|.*`.*\|.*`.*\|",
        ],
        "summary": "Generated Codex body contains Claude-specific primitives that have no Codex equivalent.",
    },
    {
        "code": "CLAUDE_TOOL_NAMING",
        "patterns": [
            r"\bEdit tool\b",
            r"\bWrite tool\b",
            r"\bRead tool\b",
            r"\bGlob tool\b",
            r"\bGrep tool\b",
            r"\bBash tool\b",
            r"\busing the Edit\b",
            r"\busing the Write\b",
            r"\busing the Read\b",
        ],
        "ignore_patterns": [
            r"^\s*#",
            r"(?i)do\s+not\s+use\b",
            r"(?i)not\s+available\b",
            r"\|.*`.*\|.*`.*\|",
        ],
        "summary": "Generated Codex body uses Claude-specific tool names (Edit/Write/Read) instead of Codex equivalents (apply_diff/write_file/read_file).",
    },
    {
        "code": "STALE_MULTI_AGENT_SYNTAX",
        "patterns": [
            r"\bspawn_agents_on_csv\b",
            r"\breport_agent_job_result\b",
            r"\bTaskOutput\b",
            r"\bwait\(timeout_seconds=\d+",
            r"\bTask\(subagent_type=",
            r"\btask\(subagent_type=",
        ],
        "ignore_patterns": [
            r"(?i)must\s+not\s+appear",
            r"(?i)must\s+not\s+be\s+used",
            r"(?i)do\s+not\s+use\b",
            r"(?i)not\s+available\b",
            r"(?i)not\s+supported\b",
            r"(?i)instead\s+of\b",
            r"(?i)replaced?\s+by\b",
            r"(?i)prohibited",
            r"^\s*#",
            r"\|.*`.*\|.*`.*\|",
        ],
        "summary": "Generated Codex body still references stale multi-agent syntax that is not available in the current Codex runtime.",
    },
    {
        "code": "WRONG_XREF_DIR",
        "patterns": [
            r"\]\(skills/",
            r"\.\.\$[a-zA-Z]",
        ],
        "ignore_patterns": [
            r"^```",
            r"^\s*`",
            r"(?i)directory\s+structure",
            r"(?i)under\s+`?skills/",
            r"(?i)the\s+`?skills/`?\s+",
            r"(?i)in\s+`?skills/`?\s+",
            r"(?i)edit\s+.*skills/",
        ],
        "summary": "Cross-reference uses wrong directory path; skills-codex/ refs should use ../ relative paths.",
    },
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Audit generated Codex skills for semantic parity drift."
    )
    parser.add_argument(
        "--repo-root",
        default=".",
        help="Repository root (default: current directory).",
    )
    parser.add_argument(
        "--skill",
        action="append",
        dest="skills",
        default=[],
        help="Audit only the named skill. Repeat for multiple skills.",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Emit findings as JSON.",
    )
    return parser.parse_args()


def load_catalog(repo_root: Path) -> dict[str, dict]:
    catalog_path = repo_root / "skills-codex-overrides" / "catalog.json"
    if not catalog_path.exists():
        return {}

    with catalog_path.open("r", encoding="utf-8") as handle:
        payload = json.load(handle)

    return {
        entry.get("name", ""): entry
        for entry in payload.get("skills", [])
        if isinstance(entry, dict) and entry.get("name")
    }


def recommendation(repo_root: Path, path: Path, skill: str, treatment: str) -> str:
    override_skill = repo_root / "skills-codex-overrides" / skill / "SKILL.md"
    override_rel = override_skill.relative_to(repo_root).as_posix()
    checked_in_skill = repo_root / "skills-codex" / skill / "SKILL.md"
    checked_in_rel = checked_in_skill.relative_to(repo_root).as_posix()
    audit_cmd = f"bash scripts/audit-codex-parity.sh --skill {skill}"
    path_rel = path.relative_to(repo_root).as_posix()

    if path_rel.startswith("skills-codex-overrides/"):
        return (
            f"Update `{path_rel}` so the override matches the current Codex runtime "
            f"surface, then rerun `{audit_cmd}`."
        )

    if treatment == "bespoke":
        verb = "Update" if override_skill.exists() else "Create"
        return f"{verb} `{override_rel}` and `{checked_in_rel}`, then rerun `{audit_cmd}`."

    return (
        f"Fix the checked-in artifact `{checked_in_rel}`, or promote the skill to `bespoke` "
        "in `skills-codex-overrides/catalog.json` if it needs a durable Codex body rewrite."
    )


def iter_skill_files(repo_root: Path, skills: list[str]) -> list[Path]:
    selected = set(skills)
    skill_files: list[Path] = []

    roots = [
        repo_root / "skills-codex",
        repo_root / "skills-codex-overrides",
    ]
    for skills_root in roots:
        if not skills_root.is_dir():
            continue
        for skill_dir in sorted(skills_root.iterdir()):
            if not skill_dir.is_dir():
                continue
            if selected and skill_dir.name not in selected:
                continue
            for skill_file in sorted(skill_dir.rglob("*.md")):
                skill_files.append(skill_file)

    return skill_files


def load_cross_runtime(repo_root: Path) -> set[str]:
    """Skills exempt from the Claude-naming rules because they legitimately
    document non-Codex runtimes. Shared source of truth with codex-sync and the
    other Codex gates: scripts/lint/codex-cross-runtime-skills.txt."""
    path = repo_root / "scripts" / "lint" / "codex-cross-runtime-skills.txt"
    if not path.exists():
        return set()
    return {
        line.strip()
        for line in path.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    }


def find_findings(
    repo_root: Path,
    skill_file: Path,
    catalog: dict[str, dict],
    cross_runtime: set[str] = frozenset(),
) -> list[dict]:
    relative_path = skill_file.relative_to(repo_root)
    parts = relative_path.parts
    if len(parts) < 2:
        return []
    skill = parts[1]
    treatment = catalog.get(skill, {}).get("treatment", "unknown")

    # parity_only twins under skills-codex/ are GENERATED and verified by the
    # codex-sync byte-exact drift gate; re-auditing generated content is the
    # whack-a-mole this removes. Audit bespoke twins + the override layer only.
    if parts[0] == "skills-codex" and treatment != "bespoke":
        return []

    findings: list[dict] = []

    with skill_file.open("r", encoding="utf-8") as handle:
        for line_number, raw_line in enumerate(handle, start=1):
            line = raw_line.rstrip("\n")
            for rule in RULES:
                # Cross-runtime skills may name Claude tools accurately (e.g. cass
                # documents the Claude Code log format it parses).
                if rule["code"] == "CLAUDE_TOOL_NAMING" and skill in cross_runtime:
                    continue
                ignore_patterns = rule.get("ignore_patterns", [])
                if any(re.search(pattern, line) for pattern in ignore_patterns):
                    continue
                for pattern in rule["patterns"]:
                    if re.search(pattern, line):
                        findings.append(
                            {
                                "code": rule["code"],
                                "skill": skill,
                                "path": skill_file.relative_to(repo_root).as_posix(),
                                "line": line_number,
                                "matched_text": line.strip(),
                                "treatment": treatment,
                                "message": rule["summary"],
                                "recommendation": recommendation(
                                    repo_root, skill_file, skill, treatment
                                ),
                            }
                        )
                        break
    return findings


def main() -> int:
    args = parse_args()
    repo_root = Path(args.repo_root).resolve()
    catalog = load_catalog(repo_root)
    cross_runtime = load_cross_runtime(repo_root)
    skill_files = iter_skill_files(repo_root, args.skills)

    findings: list[dict] = []
    for skill_file in skill_files:
        findings.extend(find_findings(repo_root, skill_file, catalog, cross_runtime))

    if args.json:
        json.dump(findings, sys.stdout, indent=2)
        sys.stdout.write("\n")
    else:
        if not findings:
            print("Codex parity audit passed.")
        else:
            for finding in findings:
                print(
                    f"{finding['code']} {finding['skill']} "
                    f"{finding['path']}:{finding['line']}"
                )
                print(f"  line: {finding['matched_text']}")
                print(f"  treatment: {finding['treatment']}")
                print(f"  action: {finding['recommendation']}")
            print(f"Codex parity audit failed with {len(findings)} finding(s).")

    return 1 if findings else 0


if __name__ == "__main__":
    raise SystemExit(main())
