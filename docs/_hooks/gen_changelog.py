"""Publish the root CHANGELOG.md as the site's CHANGELOG.md page at build time.

The root CHANGELOG.md is the only tracked changelog. This hook writes it into
the virtual docs tree so docs pages that link `CHANGELOG.md` keep resolving,
without a second tracked copy that has to be kept in sync.
"""

from __future__ import annotations

import sys
from pathlib import Path

import mkdocs_gen_files

REPO_ROOT = Path(__file__).resolve().parent.parent.parent
SOURCE = REPO_ROOT / "CHANGELOG.md"


def main() -> None:
    if not SOURCE.is_file():
        # Fail the build rather than publishing a site whose changelog links 404.
        raise SystemExit("[gen_changelog] FATAL: missing source CHANGELOG.md at the repo root.")
    with mkdocs_gen_files.open("CHANGELOG.md", "w") as fh:
        fh.write(SOURCE.read_text(encoding="utf-8"))
    # edit_uri points at docs/; the source lives one level up.
    mkdocs_gen_files.set_edit_path("CHANGELOG.md", "../CHANGELOG.md")
    print("[gen_changelog] emitted CHANGELOG.md from the root changelog", file=sys.stderr)


main()
