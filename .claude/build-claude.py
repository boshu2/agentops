#!/usr/bin/env python3
"""
Build CLAUDE.md from base + extension.

Usage:
    python .claude/build-claude.py [--check]
"""
import sys
from pathlib import Path


def get_repo_root() -> Path:
    """Find repo root (where .claude/ lives)."""
    current = Path(__file__).resolve().parent.parent
    if (current / ".claude").exists():
        return current
    raise RuntimeError("Cannot find repo root")


def main():
    check_mode = "--check" in sys.argv
    repo_root = get_repo_root()
    claude_dir = repo_root / ".claude"

    base_file = claude_dir / "CLAUDE-base.md"
    extension_file = claude_dir / "CLAUDE-extension.md"
    output_file = repo_root / "CLAUDE.md"

    if not base_file.exists():
        print(f"ERROR: Base not found: {base_file}")
        sys.exit(1)

    if not extension_file.exists():
        print(f"ERROR: Extension not found: {extension_file}")
        sys.exit(1)

    base = base_file.read_text()
    extension = extension_file.read_text()

    # Extension first (identity), then base (methodology)
    generated = f"{extension}\n\n---\n\n{base}"

    if check_mode:
        if output_file.exists() and output_file.read_text() == generated:
            print("CLAUDE.md is up to date.")
            sys.exit(0)
        else:
            print("CLAUDE.md is STALE. Run: python .claude/build-claude.py")
            sys.exit(1)
    else:
        output_file.write_text(generated)
        print(f"Generated: {output_file}")


if __name__ == "__main__":
    main()
