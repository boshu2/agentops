#!/usr/bin/env python3
"""sku_extract.py — shared primitive: extract the ao commands a skill drives.

This is the missing JOIN KEY for the SKU capability catalog (oracle gap #1).
It is derived from the skill BODY (SKILL.md + references/*), never hand-authored
in frontmatter — consistent with the repo rule banning hand-edited inventory maps.

Two responsibilities:

  1. ``extract_skill_commands(skill_dir, valid_commands)`` — for one skill, return
     the sorted, de-duplicated list of real ``ao <command>`` references found in its
     body. Only commands that resolve in ``valid_commands`` (the live cobra tree)
     are returned, so a stale ``ao schedule`` reference is NOT silently promoted to
     a join edge.

  2. ``scan_command_tree(ao_bin)`` — build the authoritative command tree from the
     live binary: the set of valid command paths (``"inject"``, ``"goals measure"``)
     and the set of valid flags per command path. ``ao capabilities`` supplies the
     top-level command names (strict — ``ao help <unknown>`` returns 0 and cannot be
     trusted), and ``ao help <cmd>`` supplies subcommands + flags.

The catalog generator imports ``extract_skill_commands`` / ``scan_command_tree``.
The drift gate's linkage-integrity check reuses the same tree so the two surfaces
can never disagree about what a "real command" is.

Run standalone for debugging::

    scripts/lib/sku_extract.py --ao-bin /path/to/ao            # dump command tree
    scripts/lib/sku_extract.py --ao-bin /path/to/ao --skill rpi  # dump one skill's commands
"""

from __future__ import annotations

import argparse
import json
import os
import pathlib
import re
import shlex
import subprocess
import sys
from typing import Dict, List, Optional, Set, Tuple

# A bare `ao <token>` where token is a lowercase word (a command name candidate).
WORDISH = re.compile(r"^[a-z][a-z0-9-]*$")
CONTROL_TOKENS = {"|", "||", "&&", ";", "&"}


def iter_ao_snippets(path: pathlib.Path):
    """Yield candidate ``ao ...`` command strings from a markdown/shell file.

    Mirrors validate-skill-cli-snippets.sh: backtick spans containing ``ao`` and
    lines that start with ``ao ``.
    """
    try:
        text = path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return
    for line in text.splitlines():
        if "ao " not in line and not line.strip().startswith("ao"):
            continue
        for match in re.finditer(r"`([^`]*\bao\b[^`]*)`", line):
            yield match.group(1).strip()
        stripped = line.strip()
        if stripped.startswith("ao "):
            yield stripped


def _command_candidates(tokens: List[str]) -> List[str]:
    """Return the leading word tokens after ``ao`` (potential subcommand path)."""
    candidates: List[str] = []
    for token in tokens[1:]:
        if token.startswith("-"):
            break
        if any(ch in token for ch in "<>[]{}=$()"):
            break
        if not WORDISH.match(token):
            break
        candidates.append(token)
    return candidates


def resolve_command_path(
    snippet: str, valid_commands: Set[Tuple[str, ...]]
) -> Optional[Tuple[str, ...]]:
    """Resolve a snippet to the longest valid command path, or None.

    ``valid_commands`` is the set of command-path tuples from the live tree
    (e.g. ``("inject",)``, ``("goals", "measure")``). The longest matching prefix
    of the snippet's word tokens wins, so ``ao goals measure --x`` resolves to
    ``("goals", "measure")`` not ``("goals",)``.
    """
    try:
        tokens = shlex.split(snippet)
    except ValueError:
        return None
    # Trim at the first shell control token.
    trimmed: List[str] = []
    for token in tokens:
        if token in CONTROL_TOKENS or token.startswith(("|", ">", "<")):
            break
        trimmed.append(token.rstrip(";"))
    tokens = [t for t in trimmed if t]
    if not tokens or tokens[0] != "ao":
        return None
    candidates = _command_candidates(tokens)
    for end in range(len(candidates), 0, -1):
        path = tuple(candidates[:end])
        if path in valid_commands:
            return path
    return None


def extract_skill_commands(
    skill_dir: pathlib.Path, valid_commands: Set[Tuple[str, ...]]
) -> List[str]:
    """Return sorted unique ``ao <cmd>`` strings the skill drives (real commands only)."""
    found: Set[str] = set()
    targets: List[pathlib.Path] = []
    skill_md = skill_dir / "SKILL.md"
    if skill_md.is_file():
        targets.append(skill_md)
    refs = skill_dir / "references"
    if refs.is_dir():
        targets.extend(sorted(p for p in refs.rglob("*") if p.is_file() and p.suffix in {".md", ".sh"}))
    for path in targets:
        for snippet in iter_ao_snippets(path):
            path_tuple = resolve_command_path(snippet, valid_commands)
            if path_tuple:
                found.add("ao " + " ".join(path_tuple))
    return sorted(found)


def _run_help(ao_bin: str, command: List[str]) -> subprocess.CompletedProcess:
    return subprocess.run(
        [ao_bin, "help", *command],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        check=False,
    )


# Cobra sections that head NON-command blocks. Any other column-0 line ending with
# ":" is a command block: either the default "Available/Additional Commands:" or a
# custom cobra.Group title (e.g. `ao pawl`'s front-door/operator split), whose titles
# are arbitrary prose and need not contain the word "Commands".
_NON_COMMAND_SECTIONS = {
    "Usage:",
    "Aliases:",
    "Examples:",
    "Flags:",
    "Global Flags:",
    "Additional help topics:",
}


def _parse_subcommands(help_text: str) -> List[str]:
    """Parse the command blocks of cobra help (default + group-titled) into names."""
    subs: List[str] = []
    in_block = False
    for line in help_text.splitlines():
        stripped = line.rstrip()
        if re.match(r"^[A-Za-z].*:\s*$", stripped) and stripped not in _NON_COMMAND_SECTIONS:
            in_block = True
            continue
        if in_block:
            if not stripped:
                in_block = False
                continue
            if not line.startswith(("  ", "\t")):
                in_block = False
                continue
            # Command rows render as "  <name><2+ spaces><short>"; the >=2-space gap
            # keeps Examples-style "  ao pawl ..." lines from parsing as commands.
            m = re.match(r"^\s+([a-z][a-z0-9-]*)(\s{2,}|\s*$)", line)
            if m and m.group(1) not in {"help", "completion"}:
                subs.append(m.group(1))
    return subs


def _parse_flags(help_text: str) -> Set[str]:
    """Parse long flags (``--flag``) from a cobra help text."""
    flags: Set[str] = set()
    for m in re.finditer(r"(--[a-z][a-z0-9-]*)", help_text):
        flags.add(m.group(1))
    return flags


def _top_level_commands(ao_bin: str) -> List[str]:
    """Authoritative top-level command names from ``ao capabilities`` (strict)."""
    result = subprocess.run(
        [ao_bin, "capabilities"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError("ao capabilities failed; cannot build command tree")
    data = json.loads(result.stdout)
    names: List[str] = []
    for group in data.get("command_groups", []):
        for cmd in group.get("commands", []):
            name = cmd.get("name")
            if name and name not in {"help", "completion"}:
                names.append(name)
    return sorted(set(names))


def scan_command_tree(ao_bin: str, max_depth: int = 2) -> Tuple[Set[Tuple[str, ...]], Dict[Tuple[str, ...], Set[str]]]:
    """Build (valid_command_paths, flags_by_path) from the live binary.

    Top-level command names come from ``ao capabilities`` (the only strict source —
    ``ao help <unknown>`` returns 0). Subcommands and flags come from walking
    ``ao help <path>`` up to ``max_depth`` levels.
    """
    paths: Set[Tuple[str, ...]] = set()
    flags: Dict[Tuple[str, ...], Set[str]] = {}
    global_help = _run_help(ao_bin, [])
    flags[()] = _parse_flags(global_help.stdout)

    frontier: List[Tuple[str, ...]] = [(c,) for c in _top_level_commands(ao_bin)]
    for path in frontier:
        paths.add(path)
    depth = 1
    while frontier and depth <= max_depth:
        next_frontier: List[Tuple[str, ...]] = []
        for path in frontier:
            help_result = _run_help(ao_bin, list(path))
            help_text = help_result.stdout
            flags[path] = _parse_flags(help_text)
            for sub in _parse_subcommands(help_text):
                child = path + (sub,)
                if child not in paths:
                    paths.add(child)
                    next_frontier.append(child)
        frontier = next_frontier
        depth += 1
    return paths, flags


def _main(argv: List[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ao-bin", required=True, help="path to the ao binary")
    parser.add_argument("--skill", help="dump the commands a single skill drives")
    parser.add_argument("--repo-root", default=os.environ.get("REPO_ROOT", "."))
    args = parser.parse_args(argv)

    valid_commands, _flags = scan_command_tree(args.ao_bin)
    if args.skill:
        skill_dir = pathlib.Path(args.repo_root) / "skills" / args.skill
        cmds = extract_skill_commands(skill_dir, valid_commands)
        print(json.dumps(cmds, indent=2))
    else:
        out = sorted(" ".join(p) for p in valid_commands)
        print(json.dumps(out, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(_main(sys.argv[1:]))
