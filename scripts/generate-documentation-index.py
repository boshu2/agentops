#!/usr/bin/env python3
"""Generate the small public documentation index from live files."""

from __future__ import annotations

import argparse
from pathlib import Path
import sys


ROOT = Path(__file__).resolve().parents[1]
TARGET = ROOT / "docs" / "documentation-index.md"


def title(path: Path) -> str:
    if path.suffix == ".md":
        for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
            if line.startswith("# "):
                return line[2:].strip()
    return path.stem.replace("-", " ").replace("_", " ").title()


def render() -> str:
    lines = [
        "# Documentation index",
        "",
        "This index is generated from live files by `scripts/generate-documentation-index.py`.",
        "Dated plans, audits, releases, and archive material are historical evidence, not current authority.",
        "",
        "## Product and workflow",
        "",
        "- [README](../README.md)",
        "- [Product boundary](../PRODUCT.md)",
        "- [Fitness goals](../GOALS.md)",
        "- [Program boundary](../PROGRAM.md)",
        "- [Operating loop](architecture/operating-loop.md)",
        "- [Gas City factory](architecture/gas-city-factory.md)",
        "- [Agent workflow](agent-workflow-reference.md)",
        "- [Repository CI and delivery](CI-CD.md)",
        "- [Component map](architecture/component-map.md)",
        "- [Ports and adapters](architecture/ports-and-adapters.md)",
        "- [Skill router](SKILL-ROUTER.md)",
        "- [Skill graph](reference/agentops-skill-graph.md)",
        "",
        "## Contracts",
        "",
    ]
    for path in sorted((ROOT / "docs" / "contracts").iterdir()):
        if path.is_file() and not path.name.startswith("."):
            lines.append(f"- [{title(path)}](contracts/{path.name})")
    lines.append("")
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    expected = render()
    if args.check:
        if not TARGET.exists() or TARGET.read_text(encoding="utf-8") != expected:
            print("documentation index drift", file=sys.stderr)
            return 1
        return 0
    TARGET.write_text(expected, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
