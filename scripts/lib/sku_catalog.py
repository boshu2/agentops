#!/usr/bin/env python3
"""sku_catalog.py — build the SKU capability catalog (registry.json schema v2).

The catalog is a 4th DERIVED projection (alongside the DDD-vocabulary,
Hex-frontmatter, and Gherkin-acceptance source axes). It is a JOIN of sources
that already exist — never hand-authored:

  - docs/contracts/skill-dispositions.yaml  -> bounded_context, hex_role, disposition
  - skills/<name>/SKILL.md frontmatter      -> purpose, consumes, produces, status?
  - skills/SKILL-TIERS.md                   -> tier
  - sku_extract.scan_command_tree(ao)       -> live cobra command tree (cli-command SKUs)
  - sku_extract.extract_skill_commands(...) -> drives_commands (the new join key)
  - .github/workflows/validate.yml          -> gate SKUs (job ids)
  - packs/agentops/                         -> reference-impl SKUs (the GC reference City)

Each SKU entry carries a stable id (``skill:<name>`` / ``cmd:ao.<path>`` /
``gate:<job>`` / ``ref-impl:<name>``), name, type, bounded_context, hex_role,
tier, purpose, status, consumes, produces, drives_commands, driven_by_skills.

The module is imported by scripts/generate-sku-catalog.sh (writes the catalog)
and by scripts/validate-sku-catalog-drift.sh (drift + linkage + coverage gate).
"""

from __future__ import annotations

import json
import pathlib
import re
from typing import Dict, List, Optional, Tuple

import sku_extract

# The 5 bounded contexts (canonical: docs/contracts/bounded-contexts.yaml).
BOUNDED_CONTEXTS = ["BC1", "BC2", "BC3", "BC4", "BC5"]
BC_NAMES = {
    "BC1": "Corpus",
    "BC2": "Validation",
    "BC3": "Loop",
    "BC4": "Factory",
    "BC5": "Runtime",
}

# Explicit command -> BC map for commands with no same-named skill. Sourced from
# the BC↔command digest in the inventory oracle report (section 1.5 of
# .agents/discovery/2026-05-24-capability-inventory-and-gaps.md). A command with
# a same-named skill inherits that skill's BC (see _command_bc); this map is the
# fallback so every cli-command SKU carries a BC (coverage assertion).
COMMAND_BC = {
    # BC1 Corpus — capture / retrieve / compile / cite / promote.
    "corpus": "BC1", "maturity": "BC1", "mine": "BC1", "lookup": "BC1",
    "search": "BC1", "pool": "BC1", "knowledge": "BC1", "mind": "BC1",
    "patterns": "BC1", "findings": "BC1", "extract": "BC1", "dedup": "BC1",
    "contradict": "BC1", "wiki": "BC1", "citation": "BC1", "memory": "BC1",
    "notebook": "BC1", "seed": "BC1", "defrag": "BC1",
    # BC2 Validation — gates.
    "vibe-check": "BC2", "gate": "BC2", "ci": "BC2", "eval": "BC2",
    "retrieval-bench": "BC2", "metrics": "BC2", "constraint": "BC2",
    "anti-patterns": "BC2", "badge": "BC2",
    # BC3 Loop — operating loop.
    "loop": "BC3", "cron": "BC3", "claim": "BC3", "quick-start": "BC3",
    "demo": "BC3",
    # BC4 Factory — skill + claim factory + operator port.
    "skills": "BC4", "registry": "BC4", "agents": "BC4", "config": "BC4",
    "operator": "BC4",
    # BC5 Runtime — harness + operator adapters.
    "codex": "BC5", "harness": "BC5", "push": "BC5", "session": "BC5",
    "sessions": "BC5", "feedback-loop": "BC5", "robot-docs": "BC5",
    "capabilities": "BC5",
    # Cross-cutting — pin to a home BC for coverage (doctor/init/version are
    # Factory-flavored setup/diagnostics).
    "doctor": "BC4", "init": "BC4", "version": "BC4",
}

# Operating-loop moves (canonical: docs/architecture/operating-loop.md) and a
# primary skill for each. Coverage assertion: every move has >=1 active skill.
LOOP_MOVES = {
    1: ["brainstorm", "discovery", "design"],
    2: ["beads"],
    3: ["plan"],
    4: ["implement"],
    5: ["crank", "swarm", "autodev"],
    6: ["validation", "council", "vibe"],
    7: ["post-mortem", "forge", "retro", "ratchet", "flywheel", "harvest"],
}


def _parse_frontmatter_block(text: str) -> str:
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return ""
    out: List[str] = []
    for line in lines[1:]:
        if line.strip() == "---":
            break
        out.append(line)
    return "\n".join(out)


def parse_skill_frontmatter(skill_md: pathlib.Path) -> Dict[str, object]:
    """Parse the subset of SKILL.md frontmatter the catalog needs.

    Returns a dict with: name, description, hexagonal_role, status (optional),
    consumes (list), produces (list).
    """
    result: Dict[str, object] = {
        "name": skill_md.parent.name,
        "description": "",
        "hexagonal_role": "",
        "status": "",
        "consumes": [],
        "produces": [],
    }
    try:
        text = skill_md.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return result
    block = _parse_frontmatter_block(text)
    current_list: Optional[str] = None
    for raw in block.splitlines():
        if not raw.strip():
            continue
        # List item under a list key.
        item = re.match(r"^-\s+(.+?)\s*$", raw)
        if item and current_list in {"consumes", "produces"}:
            value = item.group(1).strip().strip("\"'")
            # context_rel entries start with `kind:`; skip them defensively.
            if not value.startswith("kind:"):
                result[current_list].append(value)  # type: ignore[union-attr]
            continue
        # Scalar `key: value`.
        scalar = re.match(r"^([A-Za-z_][A-Za-z0-9_-]*):\s*(\S.*)$", raw)
        if scalar:
            current_list = None
            key, val = scalar.group(1), scalar.group(2).strip().strip("\"'")
            if key == "name":
                result["name"] = val
            elif key == "description":
                result["description"] = val
            elif key == "hexagonal_role":
                result["hexagonal_role"] = val
            elif key == "status":
                result["status"] = val
            continue
        # List opener `key:`.
        opener = re.match(r"^([A-Za-z_][A-Za-z0-9_-]*):\s*$", raw)
        if opener:
            current_list = opener.group(1)
            continue
        current_list = None
    return result


def parse_dispositions(yaml_path: pathlib.Path) -> Dict[str, Dict[str, str]]:
    """Parse skill-dispositions.yaml into {skill: {bc, bc_name, hex_role, disposition, rationale}}.

    Hand-rolled line parser (no PyYAML dependency in CI) for the known flat shape.
    """
    out: Dict[str, Dict[str, str]] = {}
    current: Optional[str] = None
    try:
        text = yaml_path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return out
    for raw in text.splitlines():
        line = raw.rstrip()
        m = re.match(r"^\s*-\s+skill:\s*(\S+)", line)
        if m:
            current = m.group(1).strip().strip("\"'")
            out[current] = {"bc": "", "bc_name": "", "hex_role": "", "disposition": "", "rationale": ""}
            continue
        if current is None:
            continue
        dm = re.match(r"^\s+domain:\s*\"?(BC\d)\s+([A-Za-z]+)\"?", line)
        if dm:
            out[current]["bc"] = dm.group(1)
            out[current]["bc_name"] = dm.group(2)
            continue
        hm = re.match(r"^\s+hexagonal_role:\s*(\S+)", line)
        if hm:
            out[current]["hex_role"] = hm.group(1).strip().strip("\"'")
            continue
        pm = re.match(r"^\s+disposition:\s*(\S+)", line)
        if pm:
            out[current]["disposition"] = pm.group(1).strip().strip("\"'")
            continue
        rm = re.match(r"^\s+rationale:\s*\"?(.+?)\"?\s*$", line)
        if rm:
            out[current]["rationale"] = rm.group(1).strip()
            continue
    return out


def parse_tiers(tiers_path: pathlib.Path) -> Dict[str, str]:
    """Parse SKILL-TIERS.md into {skill: tier}.

    Two table shapes:
      user-facing:  | **<skill>** | <tier> | description |
      internal:     | <skill> | <tier> | category | purpose |
    """
    valid_tiers = {
        "judgment", "execution", "knowledge", "product", "session", "contribute",
        "cross-vendor", "library", "background", "meta", "utility",
    }
    out: Dict[str, str] = {}
    try:
        text = tiers_path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return out
    for raw in text.splitlines():
        cells = [c.strip() for c in raw.split("|")]
        # cells[0] is '' (leading pipe). Need at least name + tier.
        if len(cells) < 3:
            continue
        name_cell = cells[1]
        tier_cell = cells[2]
        name = name_cell.strip("*` ").strip()
        if tier_cell in valid_tiers and re.match(r"^[a-z][a-z0-9-]*$", name or ""):
            out.setdefault(name, tier_cell)
    return out


def compute_status(skill: str, frontmatter_status: str, disposition: str, rationale: str) -> str:
    """Derive lifecycle status (active | deprecated | planned | alias-of:<sku>).

    Precedence:
      1. explicit frontmatter status: (deprecated | planned)
      2. merge-review + rationale mentions an alias absorbed into a sibling
         -> alias-of:skill:<sibling>
      3. else active
    """
    fm = (frontmatter_status or "").lower()
    if fm in {"deprecated", "planned"}:
        return fm
    if disposition == "merge-review" and "alias" in rationale.lower():
        # Extract the sibling skill the rationale says it was absorbed into,
        # e.g. "Absorbed into `council` as `--mode=debate`; thin alias ...".
        m = re.search(r"absorbed into `?([a-z][a-z0-9-]*)`?", rationale, re.IGNORECASE)
        if m:
            return "alias-of:skill:" + m.group(1)
        return "deprecated"
    return "active"


def build_skill_skus(
    repo_root: pathlib.Path,
    valid_commands,
    dispositions: Dict[str, Dict[str, str]],
    tiers: Dict[str, str],
) -> List[Dict[str, object]]:
    """One SKU per skill, joining frontmatter + dispositions + tier + drives_commands."""
    skills_dir = repo_root / "skills"
    skus: List[Dict[str, object]] = []
    for skill_md in sorted(skills_dir.glob("*/SKILL.md")):
        skill_dir = skill_md.parent
        name = skill_dir.name
        fm = parse_skill_frontmatter(skill_md)
        disp = dispositions.get(name, {})
        drives = sku_extract.extract_skill_commands(skill_dir, valid_commands)
        refs_dir = skill_dir / "references"
        ref_count = (
            len([p for p in refs_dir.rglob("*.md") if p.is_file()]) if refs_dir.is_dir() else 0
        )
        status = compute_status(
            name, str(fm.get("status", "")), disp.get("disposition", ""), disp.get("rationale", "")
        )
        skus.append(
            {
                "sku": "skill:" + name,
                "name": name,
                "type": "skill",
                "bounded_context": disp.get("bc", ""),
                "hex_role": disp.get("hex_role", fm.get("hexagonal_role", "")),
                "tier": tiers.get(name, "unknown"),
                "purpose": fm.get("description", ""),
                "status": status,
                "disposition": disp.get("disposition", ""),
                "consumes": fm.get("consumes", []),
                "produces": fm.get("produces", []),
                "drives_commands": drives,
                "path": f"skills/{name}/",
                "references": ref_count,
            }
        )
    return skus


def _command_bc(name: str, skill_skus: List[Dict[str, object]]) -> str:
    """Resolve a command's BC: same-named skill first, then the explicit map.

    Returns the BC of a skill with the same name if one exists (the command is
    that skill's CLI surface), else the explicit COMMAND_BC fallback, else empty.
    """
    for sku in skill_skus:
        if sku["name"] == name and sku["bounded_context"]:
            return str(sku["bounded_context"])
    return COMMAND_BC.get(name, "")


def build_cli_command_skus(
    ao_bin: str,
    valid_commands,
    flags_by_path: Dict[Tuple[str, ...], set],
    skill_skus: List[Dict[str, object]],
    command_shorts: Dict[Tuple[str, ...], str],
) -> List[Dict[str, object]]:
    """One SKU per TOP-LEVEL ao command (the real cobra nodes — fixes the bogus 163).

    driven_by_skills is the reverse of skill.drives_commands.
    """
    # Build reverse index: command-path-string -> [skill names].
    reverse: Dict[str, List[str]] = {}
    for sku in skill_skus:
        for cmd in sku["drives_commands"]:  # type: ignore[index]
            reverse.setdefault(str(cmd), []).append(str(sku["name"]))

    skus: List[Dict[str, object]] = []
    top_level = sorted(p[0] for p in valid_commands if len(p) == 1)
    for name in top_level:
        path = (name,)
        flags = sorted(f for f in flags_by_path.get(path, set()) if f != "--help")
        driven = sorted(set(reverse.get("ao " + name, [])))
        skus.append(
            {
                "sku": "cmd:ao." + name,
                "name": "ao " + name,
                "type": "cli-command",
                "bounded_context": _command_bc(name, skill_skus),
                "purpose": command_shorts.get(path, ""),
                "status": "active",
                "driven_by_skills": driven,
                "flags": flags,
                "path": "cli/cmd/ao/",
            }
        )
    return skus


def parse_gate_jobs(workflow_path: pathlib.Path) -> List[str]:
    """Return the validate.yml job ids (top-level keys under `jobs:`)."""
    jobs: List[str] = []
    in_jobs = False
    try:
        text = workflow_path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return jobs
    for raw in text.splitlines():
        if re.match(r"^jobs:\s*$", raw):
            in_jobs = True
            continue
        if in_jobs:
            if re.match(r"^[A-Za-z]", raw):  # dedented top-level key ends jobs
                break
            m = re.match(r"^  ([a-z][a-z0-9-]+):\s*$", raw)
            if m and m.group(1) not in {"push"}:
                jobs.append(m.group(1))
    return jobs


def build_gate_skus(workflow_path: pathlib.Path) -> List[Dict[str, object]]:
    """One SKU per CI gate job (type=gate, BC2 Validation — gates ARE validation)."""
    skus: List[Dict[str, object]] = []
    for job in parse_gate_jobs(workflow_path):
        skus.append(
            {
                "sku": "gate:" + job,
                "name": job,
                "type": "gate",
                "bounded_context": "BC2",
                "purpose": "CI validation gate (.github/workflows/validate.yml).",
                "status": "active",
                "path": ".github/workflows/validate.yml",
            }
        )
    return skus


def build_reference_impl_skus(repo_root: pathlib.Path) -> List[Dict[str, object]]:
    """SKUs for the Gas City reference City pack (the reference-impl axis)."""
    pack = repo_root / "packs" / "agentops"
    if not (pack / "pack.toml").is_file():
        return []
    skus: List[Dict[str, object]] = [
        {
            "sku": "ref-impl:agentops-pack",
            "name": "agentops reference City pack",
            "type": "reference-impl",
            "bounded_context": "BC5",
            "purpose": "Reference Gas City pack: mayor + refinery agents drive ao rpi/evolve loops.",
            "status": "active",
            "path": "packs/agentops/",
        }
    ]
    agents_dir = pack / "agents"
    if agents_dir.is_dir():
        for agent in sorted(p.name for p in agents_dir.iterdir() if p.is_dir()):
            skus.append(
                {
                    "sku": "ref-impl:agentops-agent." + agent,
                    "name": "agentops " + agent + " agent",
                    "type": "reference-impl",
                    "bounded_context": "BC5",
                    "purpose": f"Reference City {agent} agent role.",
                    "status": "active",
                    "path": f"packs/agentops/agents/{agent}/",
                }
            )
    return skus


def build_catalog(repo_root: pathlib.Path, ao_bin: str) -> Dict[str, object]:
    """Assemble the full SKU catalog (the `capabilities` block + summary counts)."""
    valid_commands, flags_by_path = sku_extract.scan_command_tree(ao_bin)
    command_shorts = _command_shorts(ao_bin)
    dispositions = parse_dispositions(repo_root / "docs/contracts/skill-dispositions.yaml")
    tiers = parse_tiers(repo_root / "skills/SKILL-TIERS.md")

    skill_skus = build_skill_skus(repo_root, valid_commands, dispositions, tiers)
    cli_skus = build_cli_command_skus(
        ao_bin, valid_commands, flags_by_path, skill_skus, command_shorts
    )
    gate_skus = build_gate_skus(repo_root / ".github/workflows/validate.yml")
    ref_skus = build_reference_impl_skus(repo_root)

    capabilities = skill_skus + cli_skus + gate_skus + ref_skus
    by_type: Dict[str, int] = {}
    for sku in capabilities:
        by_type[str(sku["type"])] = by_type.get(str(sku["type"]), 0) + 1

    return {
        "capabilities": capabilities,
        "capability_summary": {
            "total": len(capabilities),
            "skills": by_type.get("skill", 0),
            "cli_commands": by_type.get("cli-command", 0),
            "gates": by_type.get("gate", 0),
            "reference_impls": by_type.get("reference-impl", 0),
        },
        "cli_top_level_commands": sorted(p[0] for p in valid_commands if len(p) == 1),
    }


def check_linkage_integrity(
    catalog: Dict[str, object], valid_commands
) -> List[str]:
    """Every drives_commands entry must resolve to a real ao command path.

    This is the gate the corpus was missing (oracle gaps #1/#2/#3): it closes
    the loop so a stale ``ao schedule`` reference cannot ship undetected. Returns
    a list of failure strings (empty = pass).
    """
    valid_strings = {"ao " + " ".join(p) for p in valid_commands}
    failures: List[str] = []
    for sku in catalog.get("capabilities", []):  # type: ignore[union-attr]
        if sku.get("type") != "skill":
            continue
        for cmd in sku.get("drives_commands", []):
            if cmd not in valid_strings:
                failures.append(
                    f"{sku['sku']}: drives_commands entry '{cmd}' does not resolve "
                    f"to a real ao command"
                )
    return failures


def check_coverage(catalog: Dict[str, object]) -> List[str]:
    """Every BC and every operating-loop move must have >=1 ACTIVE skill.

    A skill counts as covering a move/BC only if its status is ``active`` (an
    alias-of / deprecated / planned skill is not load-bearing coverage). Returns
    failure strings (empty = pass).
    """
    failures: List[str] = []
    active_skills = {
        str(s["name"]): str(s["bounded_context"])
        for s in catalog.get("capabilities", [])  # type: ignore[union-attr]
        if s.get("type") == "skill" and s.get("status") == "active"
    }
    # BC coverage: each BC has >=1 active skill.
    covered_bcs = set(active_skills.values())
    for bc in BOUNDED_CONTEXTS:
        if bc not in covered_bcs:
            failures.append(f"bounded context {bc} ({BC_NAMES[bc]}) has no active skill")
    # BC coverage: each BC has >=1 cli-command SKU.
    cmd_bcs = {
        str(s["bounded_context"])
        for s in catalog.get("capabilities", [])  # type: ignore[union-attr]
        if s.get("type") == "cli-command" and s.get("bounded_context")
    }
    for bc in BOUNDED_CONTEXTS:
        if bc not in cmd_bcs:
            failures.append(f"bounded context {bc} ({BC_NAMES[bc]}) has no cli-command SKU")
    # Loop-move coverage: each move has >=1 active skill.
    for move, candidate_skills in LOOP_MOVES.items():
        if not any(s in active_skills for s in candidate_skills):
            failures.append(
                f"operating-loop move {move} has no active skill "
                f"(candidates: {', '.join(candidate_skills)})"
            )
    return failures


def _command_shorts(ao_bin: str) -> Dict[Tuple[str, ...], str]:
    """Map top-level command path -> cobra Short string (from ao capabilities)."""
    import subprocess

    result = subprocess.run(
        [ao_bin, "capabilities"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        check=False,
    )
    shorts: Dict[Tuple[str, ...], str] = {}
    if result.returncode != 0:
        return shorts
    try:
        data = json.loads(result.stdout)
    except json.JSONDecodeError:
        return shorts
    for group in data.get("command_groups", []):
        for cmd in group.get("commands", []):
            name = cmd.get("name")
            if name:
                shorts[(name,)] = cmd.get("short", "")
    return shorts
