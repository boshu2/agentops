#!/usr/bin/env bats
# L3 (legible membrane plan, docs/plans/2026-09-02-legible-membrane-plan.md):
# README.md and docs/install-day2-ops.md state the runtime requirements of the
# skills that execute `ao` or a Python script. These checks are facts only:
# both docs carry the identical table, every `ao …` command a row names
# resolves in the live cobra tree (the shared scripts/lib/ao_snippet_resolve.py
# strict resolver), every `.py` script a row names exists, and every skill whose
# procedure invokes `ao` or runs a Python script has a row. Whether a row says
# hard, conditional or optional is a reviewer's judgment, not checked here.

setup_file() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  if [[ -z "${AGENTOPS_AO_BIN:-}" ]]; then
    AGENTOPS_AO_BIN="$BATS_FILE_TMPDIR/ao"
    (cd "$REPO_ROOT/cli" && go build -o "$AGENTOPS_AO_BIN" ./cmd/ao)
  fi
  export AGENTOPS_AO_BIN
}

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  README="$REPO_ROOT/README.md"
  INSTALL_DOC="$REPO_ROOT/docs/install-day2-ops.md"
  export REPO_ROOT INSTALL_DOC
}

# Data rows only: "| `skill` | ... " (excludes the header and separator
# rows, which do not start with a backtick-quoted skill name).
extract_table_rows() {
  grep -E '^\| `[a-z-]+` \|' "$1"
}

# Run a Python assertion block with the table rows of docs/install-day2-ops.md
# parsed as `rows` ([(skill, needs, why)]), the shared snippet resolver as
# `resolver` (strict mode) and `iter_snippets` in scope.
check_runtime_table() {
  local prelude
  prelude="$(cat <<'PY'
import os
import pathlib
import re
import shlex
import sys

repo = pathlib.Path(os.environ["REPO_ROOT"])
sys.path.insert(0, str(repo / "scripts" / "lib"))
from ao_snippet_resolve import Resolver, iter_snippets

resolver = Resolver(os.environ["AGENTOPS_AO_BIN"], mode="strict")
rows = []
for line in pathlib.Path(os.environ["INSTALL_DOC"]).read_text(encoding="utf-8").splitlines():
    if re.match(r"^\| `[a-z-]+` \|", line):
        cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
        rows.append((cells[0].strip("`"), cells[1], cells[2]))
assert rows, "no runtime-requirements rows parsed"


def resolves(snippet):
    """True when an `ao …` snippet names a real command in the cobra tree.

    A word is a subcommand only when its strict-mode help differs from its
    parent's: cobra answers `ao <group> <unknown> --help` with the group's own
    help, so the shared probe alone accepts a renamed or removed subcommand.
    A command group followed by a word that is not a subcommand fails."""
    try:
        tokens = resolver.trim_shell_tokens(shlex.split(snippet))
    except ValueError:
        return False
    if len(tokens) < 2 or tokens[0] != "ao" or resolver.is_regex_like(tokens):
        return False
    chain, rest = [], tokens[1:]
    while rest and re.match(r"^[a-z][a-z0-9-]*$", rest[0]):
        ok, text = resolver._probe(chain + [rest[0]])
        if not ok or text == resolver._probe(chain)[1]:
            break
        chain, rest = chain + [rest[0]], rest[1:]
    if not chain:
        return False
    is_group = "Available Commands:" in resolver._probe(chain)[1]
    return not (rest and is_group and re.match(r"^[a-z][a-z0-9-]*$", rest[0]))
PY
)"
  run python3 -c "$prelude
$1"
  echo "$output" >&2
  [ "$status" -eq 0 ]
}

@test "README.md and docs/install-day2-ops.md carry the identical runtime-requirements table" {
  readme_rows="$(extract_table_rows "$README")"
  install_rows="$(extract_table_rows "$INSTALL_DOC")"
  [ -n "$readme_rows" ]
  [ -n "$install_rows" ]
  diff <(echo "$readme_rows") <(echo "$install_rows")
}

@test "every runtime-requirements row names a skill, live ao commands and existing Python scripts" {
  check_runtime_table '
failures = []
for skill, needs, why in rows:
    skill_dir = repo / "skills" / skill
    if not (skill_dir / "SKILL.md").is_file():
        failures.append(f"{skill}: no skills/{skill}/SKILL.md")
        continue
    for _, snippet in iter_snippets(why):
        tokens = snippet.split()
        if len(tokens) > 1 and tokens[0] == "ao" and not resolves(snippet):
            failures.append(f"{skill}: {snippet!r} is not a live ao command")
    for script in re.findall(r"[\w./-]+\.py\b", why):
        homes = (repo / script, skill_dir / script, skill_dir / "scripts" / script)
        if not any(home.is_file() for home in homes):
            failures.append(f"{skill}: {script} exists in neither the repository nor skills/{skill}")
if failures:
    raise SystemExit("\n".join(failures))
'
}

@test "every skill whose procedure invokes ao or runs a Python script has a runtime-requirements row" {
  check_runtime_table '
SHELL_FENCES = {"", "bash", "sh", "shell", "console", "zsh"}
PYTHON_RUN = re.compile(r"\bpython3?\s+[^\s`]*\.py\b")


def procedure_lines(path):
    """Body lines outside table rows and outside non-shell fenced blocks."""
    fence, info = None, ""
    for line in path.read_text(encoding="utf-8").split("---", 2)[2].splitlines():
        marker = re.match(r"^\s*(`{3,}|~{3,})\s*(\S*)", line)
        if marker and fence is None:
            fence, info = marker.group(1)[0], marker.group(2).lower()
            continue
        if marker and fence == marker.group(1)[0] and not marker.group(2):
            fence = None
            continue
        if fence is not None and info not in SHELL_FENCES:
            continue
        if line.lstrip().startswith("|"):
            continue
        yield line


listed = {skill for skill, _, _ in rows}
missing = []
for path in sorted((repo / "skills").glob("*/SKILL.md")):
    lines = list(procedure_lines(path))
    runs_ao = any(resolves(snippet) for _, snippet in iter_snippets("\n".join(lines)))
    runs_python = any(PYTHON_RUN.search(line) for line in lines)
    if (runs_ao or runs_python) and path.parent.name not in listed:
        kind = "ao" if runs_ao else "python3"
        missing.append(f"{path.parent.name}: invokes {kind} but has no runtime-requirements row")
if missing:
    raise SystemExit("\n".join(missing))
'
}
