#!/usr/bin/env python3
"""Generate a starter architecture report from codebase analysis.

Usage: ./scaffold-report.py [project_path] [output_file]

Performs quick analysis and generates a markdown template pre-filled
with discovered information about the codebase.
"""

import argparse
from datetime import datetime
import json
import os
from pathlib import Path
import re
import sys

try:
    import tomllib
except ModuleNotFoundError:  # pragma: no cover - Python < 3.11 fallback
    tomllib = None


IGNORED_DIRS = {
    ".git",
    ".hg",
    ".svn",
    "node_modules",
    "target",
    "dist",
    "build",
    ".next",
    "__pycache__",
    ".venv",
    "venv",
    "vendor",
}


def first_existing(path: str, candidates: list[str]) -> list[str]:
    """Return candidate files that exist under path."""
    root = Path(path)
    return [candidate for candidate in candidates if (root / candidate).exists()]


def source_files(path: str, extensions: list[str]):
    """Yield source files under path while skipping generated/vendor trees."""
    root = Path(path)
    wanted = set(extensions)
    for file_path in root.rglob("*"):
        if not file_path.is_file():
            continue
        if file_path.suffix not in wanted:
            continue
        rel_path = file_path.relative_to(root)
        if any(part in IGNORED_DIRS for part in rel_path.parts):
            continue
        yield file_path, str(rel_path)


def find_files_matching(
    path: str, extensions: list[str], pattern: str, limit: int = 5
) -> list[str]:
    """Find source files with at least one line matching pattern."""
    matches: list[str] = []
    matcher = re.compile(pattern)
    for file_path, rel_path in source_files(path, extensions):
        try:
            with file_path.open("r", encoding="utf-8", errors="ignore") as handle:
                if any(matcher.search(line) for line in handle):
                    matches.append(rel_path)
        except OSError:
            continue
        if len(matches) >= limit:
            break
    return matches


def count_lines(path: str, extensions: list[str]) -> int:
    """Count lines of code for given extensions."""
    total = 0
    for file_path, _rel_path in source_files(path, extensions):
        try:
            with file_path.open("r", encoding="utf-8", errors="ignore") as handle:
                total += sum(1 for _ in handle)
        except OSError:
            continue
    return total


def detect_language(path: str) -> tuple[str, list[str]]:
    """Detect primary language from files present."""
    checks = [
        ("Cargo.toml", "Rust", [".rs"]),
        ("package.json", "TypeScript/JavaScript", [".ts", ".tsx", ".js", ".jsx"]),
        ("go.mod", "Go", [".go"]),
        ("pyproject.toml", "Python", [".py"]),
        ("requirements.txt", "Python", [".py"]),
        ("Gemfile", "Ruby", [".rb"]),
    ]
    for marker, lang, exts in checks:
        if (Path(path) / marker).exists():
            return lang, exts
    return "Unknown", []


def find_entry_points(path: str, lang: str) -> list[str]:
    """Find likely entry points based on language."""
    entries = []
    if lang == "Rust":
        entries = find_files_matching(path, [".rs"], r"\bfn\s+main\s*\(")
    elif lang == "Go":
        entries = find_files_matching(path, [".go"], r"\bfunc\s+main\s*\(")
    elif "Python" in lang:
        entries = find_files_matching(path, [".py"], r"if\s+__name__\s*==")
    elif "TypeScript" in lang or "JavaScript" in lang:
        entries = first_existing(
            path,
            [
                "src/app/page.tsx",
                "src/app/layout.tsx",
                "app/page.tsx",
                "app/layout.tsx",
                "src/pages/index.tsx",
                "src/pages/index.jsx",
                "pages/index.tsx",
                "pages/index.jsx",
                "src/main.ts",
                "src/main.tsx",
                "src/index.ts",
                "src/index.tsx",
                "src/server.ts",
                "src/server.js",
            ],
        )
    return [e for e in entries if e][:5]


def find_key_types(path: str, lang: str) -> list[str]:
    """Find major type definitions."""
    types = []
    if lang == "Rust":
        types = find_files_matching(path, [".rs"], r"^\s*pub\s+struct\s+")
    elif lang == "Go":
        types = find_files_matching(path, [".go"], r"^\s*type\s+\w+\s+struct\b")
    elif "TypeScript" in lang or "JavaScript" in lang:
        types = find_files_matching(
            path,
            [".ts", ".tsx", ".js", ".jsx"],
            r"^\s*(export\s+)?(default\s+)?(abstract\s+)?(type|interface|class)\s+",
        )
    return [t for t in types if t][:5]


def get_dependencies(path: str, lang: str) -> list[str]:
    """Extract top dependencies."""
    deps = []
    if lang == "Rust":
        cargo_toml = Path(path) / "Cargo.toml"
        if cargo_toml.exists() and tomllib is not None:
            try:
                cargo = tomllib.loads(cargo_toml.read_text(encoding="utf-8"))
                dependencies = cargo.get("dependencies", {})
                if isinstance(dependencies, dict):
                    deps.extend(str(name) for name in dependencies.keys())
            except (OSError, tomllib.TOMLDecodeError):
                return deps
    elif "TypeScript" in lang or "JavaScript" in lang:
        package_json = Path(path) / "package.json"
        if package_json.exists():
            try:
                package = json.loads(package_json.read_text(encoding="utf-8"))
                dependencies = package.get("dependencies", {})
                if isinstance(dependencies, dict):
                    deps.extend(str(name) for name in dependencies.keys())
            except (OSError, json.JSONDecodeError):
                return deps
    return [d for d in deps if d][:5]


def generate_report(path: str, output_file: str | None = None):
    """Generate the report scaffold."""
    path = os.path.abspath(path)
    project_name = os.path.basename(path)

    lang, exts = detect_language(path)
    loc = count_lines(path, exts) if exts else 0
    entries = find_entry_points(path, lang)
    types = find_key_types(path, lang)
    deps = get_dependencies(path, lang)

    report = f"""# {project_name} - Technical Architecture Report

## Executive Summary

**{project_name}** is a [CLI tool / web service / library] that [main purpose]. Built with {lang}.

**Key Statistics:**
- ~{loc:,} lines of code
- Language: {lang}
- Key dependencies: {", ".join(deps[:5]) if deps else "[analyze Cargo.toml/package.json]"}

---

## Entry Points

| Entry | Location | Purpose |
|-------|----------|---------|
"""
    for entry in entries[:5]:
        report += f"| Main | `{entry}` | [describe] |\n"
    if not entries:
        report += "| [entry] | `src/main.*` | [describe] |\n"

    report += """
---

## Key Types

| Type | Location | Purpose |
|------|----------|---------|
"""
    for t in types[:5]:
        report += f"| [TypeName] | `{t}` | [describe] |\n"
    if not types:
        report += "| [TypeName] | `src/model.*` | [describe] |\n"

    report += """
---

## Data Flow

```
[Input Source]
     |
     v
[Entry Point] --> parses/validates
     |
     v
[Handler/Controller] --> orchestrates
     |
     v
[Core Domain Logic] --> business rules
     |
     v
[Storage/External] --> persists/calls
     |
     v
[Output/Response]
```

**Happy Path Description:**
1. User invokes [command/endpoint]
2. [Entry] parses input and creates [Type]
3. [Handler] calls [Core] which processes...
4. Result is [stored/returned/displayed]

---

## External Dependencies

| Dependency | Purpose | Critical? |
|------------|---------|-----------|
"""
    for dep in deps[:5]:
        report += f"| {dep} | [purpose] | Yes/No |\n"
    if not deps:
        report += "| [dep] | [purpose] | Yes/No |\n"

    report += f"""
---

## Configuration

| Source | Location/Example | Priority |
|--------|------------------|----------|
| Environment var | `APP_*` | 1 (highest) |
| Config file | `~/.config/{{project}}/config.toml` | 2 |
| CLI flag | `--flag` | 3 |
| Default | Hardcoded | 4 (lowest) |

---

## Test Infrastructure

| Type | Location | Count |
|------|----------|-------|
| Unit tests | `src/**/*_test.*` | ~[N] |
| Integration | `tests/integration/` | ~[N] |
| E2E | `tests/e2e/` | ~[N] |

**Running Tests:**
```bash
# [language-specific test command]
```

---

## Notes & Gotchas

- [Any non-obvious behavior]
- [Known limitations]
- [Areas needing improvement]

---

*Generated: {datetime.now().strftime("%Y-%m-%d")}*
*Status: DRAFT - requires human review and completion*
"""

    if output_file:
        output_path = Path(output_file)
        output_path.parent.mkdir(parents=True, exist_ok=True)
        with output_path.open("w", encoding="utf-8") as f:
            f.write(report)
        print(f"Report scaffold written to: {output_path}")
    else:
        print(report)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Generate a starter technical architecture report scaffold"
    )
    parser.add_argument(
        "project_path",
        nargs="?",
        default=".",
        help="Path to the project to analyze (default: current directory)",
    )
    parser.add_argument(
        "output_file",
        nargs="?",
        help="Optional output markdown file (default: stdout)",
    )
    args = parser.parse_args()

    project_path = Path(args.project_path).resolve()
    if not project_path.exists():
        print(f"ERROR: project path not found: {project_path}", file=sys.stderr)
        return 1
    if not project_path.is_dir():
        print(f"ERROR: project path is not a directory: {project_path}", file=sys.stderr)
        return 1

    generate_report(str(project_path), args.output_file)
    return 0


if __name__ == "__main__":
    sys.exit(main())
