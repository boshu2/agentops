#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Shared `ao` snippet resolution machinery (build + resolver core), so this gate
# and the docs.cli-snippets gate resolve against the SAME cobra tree from one
# place. Resolve via the pre-cd absolutized $SCRIPT_DIR (a relative
# ${BASH_SOURCE[0]} would resolve wrongly if a caller cd'd first — the .1 lane's
# round-1 pawl catch).
# shellcheck source=scripts/lib/ao-snippet-resolve.sh
. "$SCRIPT_DIR/lib/ao-snippet-resolve.sh"

# Build (or reuse) the archive-tagged ao binary; sets + exports AO_BIN.
AO_BIN="$(ao_snippet_resolve_bin "$REPO_ROOT")"
# Clean up a lib-built temp binary on exit (no-op when AGENTOPS_AO_BIN was set).
if [[ -n "${AO_SNIPPET_TMP_DIR:-}" ]]; then
  trap 'rm -rf "$AO_SNIPPET_TMP_DIR"' EXIT
fi

export AO_BIN
export REPO_ROOT
# SOUND resolution semantics for the skills gate: `ao <chain> --help`, reject
# cobra's "unknown command" / "Unknown help topic" (see ao_snippet_resolve.py).
# The original `help`-mode predicate (`ao help <chain>` trusting rc==0) was
# UNSOUND — `ao help <anything>` always exits 0, so the gate could not detect a
# removed command at all (age-zggp). The env seam stays for tests / A-B.
export AO_RESOLVE_MODE="${AO_RESOLVE_MODE:-strict}"

python3 - <<'PY'
import os
import pathlib
import re
import shlex
import sys

# Import the shared resolution core (extracted to scripts/lib/).
sys.path.insert(0, os.environ["AO_SNIPPET_LIB_DIR"])
from ao_snippet_resolve import iter_snippets, make_resolver_from_env

repo_root = pathlib.Path(os.environ["REPO_ROOT"])
roots = [repo_root / "skills", repo_root / "skills-codex"]
allowed_suffixes = {".md", ".sh"}
stale_beads_resolver = re.compile(r"BEADS_DIR=\$PWD/_beads|git -C _beads|git add \.beads|git add _beads")
stale_beads_allowed = re.compile(r"\b(anti-pattern|do not|don't|must not|never|reject|fails?|historical|retired)\b", re.IGNORECASE)

resolver = make_resolver_from_env()
failures = []

def validate_snippet(path: pathlib.Path, lineno: int, snippet: str):
    try:
        tokens = shlex.split(snippet)
    except ValueError:
        return

    tokens = resolver.trim_shell_tokens(tokens)
    if not tokens or tokens[0] != "ao":
        return

    if resolver.is_regex_like(tokens):
        return

    command, help_text = resolver.resolve_command(tokens)
    if not command:
        if len(tokens) == 1:
            return
        if all(token.startswith("-") for token in tokens[1:]):
            help_text = resolver.global_help()
            for flag in tokens[1:]:
                normalized = resolver.normalize_flag(flag)
                if normalized not in help_text:
                    failures.append(
                        f"{path.relative_to(repo_root)}:{lineno}: flag {normalized} not found in help for ao"
                    )
            return
        failures.append(f"{path.relative_to(repo_root)}:{lineno}: unknown ao command in snippet: {snippet}")
        return

    flags = []
    for token in tokens[1 + len(command):]:
        if not token.startswith("-"):
            continue
        if len(token) > 1 and token[1:].isdigit():
            continue
        flags.append(resolver.normalize_flag(token))
    for flag in flags:
        if flag not in help_text:
            failures.append(
                f"{path.relative_to(repo_root)}:{lineno}: flag {flag} not found in help for {' '.join(['ao', *command])}"
            )

for root in roots:
    if not root.exists():
        continue
    for path in sorted(root.rglob("*")):
        if not path.is_file():
            continue
        if path.suffix not in allowed_suffixes:
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        for lineno, line in enumerate(text.splitlines(), start=1):
            if stale_beads_resolver.search(line) and not stale_beads_allowed.search(line):
                failures.append(
                    f"{path.relative_to(repo_root)}:{lineno}: stale beads resolver; use BEADS_DIR=\"$(ao beads dir)\" and git -C \"$(ao beads dir)\""
                )
        for lineno, snippet in iter_snippets(text):
            validate_snippet(path, lineno, snippet)

if failures:
    print("Skill CLI snippet validation FAILED:", file=sys.stderr)
    for failure in failures:
        print(f"  {failure}", file=sys.stderr)
    sys.exit(1)

print("Skill CLI snippet validation passed.")
PY
