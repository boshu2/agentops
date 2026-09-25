#!/usr/bin/env bats
# Regression fence for the Codex description projection produced by
# scripts/codex-sync.sh (`codex_catalog_description` and `transform_body`).
#
# WHY THIS EXISTS. The generator cut skill prose at a 44-character word
# boundary before re-appending the `Triggers:` clause, so the always-loaded
# Codex activation catalog shipped 51/56 descriptions that read as fragments
# ("Freshly judge whether a finished change is Triggers: ..."). A catalog whose
# whole job is routing cannot route on half a clause, so the budget that
# produced the truncation defeated the budget's own purpose. These assertions
# pin the repaired projection against the exact defects the 2026-09-02 field
# audit found, so the fragment cannot come back silently.
#
# Later first-sentence truncation also lost use cases and preconditions from
# reality-check and ms. Complete source-description parity is now the oracle;
# fixture regression tests exercise independent required-input and exclusion
# sentences without pinning mutable catalog copy to historical prose.
#
# These run against the generated tree in the checkout, so they are only
# meaningful after `bash scripts/regen-all.sh` has projected the current
# generator. That is the point: they fence the artifact a stranger installs.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
}

# B ── every generated twin retains the source's complete routing signal.
@test "every twin description preserves all source routing content" {
    run python3 - <<'PYCHECK'
import os
import pathlib
import yaml

repo = pathlib.Path(os.environ["REPO_ROOT"])


def description(path):
    frontmatter = path.read_text(encoding="utf-8").split("---", 2)[1]
    return " ".join(yaml.safe_load(frontmatter)["description"].split())


twins = sorted((repo / "skills-codex").glob("*/SKILL.md"))
if not twins:
    raise SystemExit("no skills-codex twins found")

failures = []
for twin in twins:
    source = repo / "skills" / twin.parent.name / "SKILL.md"
    if not source.is_file():
        failures.append(f"{twin.parent.name}: twin has no source skill")
    elif description(twin) != description(source):
        failures.append(f"{twin.parent.name}: source description was changed or truncated")

if failures:
    raise SystemExit("\n".join(failures))
print(f"checked complete descriptions for {len(twins)} twins")
PYCHECK
    echo "$output" >&2
    [ "$status" -eq 0 ]
}

# C ── a CROSS-RUNTIME skill (it names Claude Code AND Codex CLI, or both
# ~/.claude/skills and ~/.codex/skills) must be listed in
# scripts/lint/codex-cross-runtime-skills.txt, and a listed skill's twin must not
# receive the Claude->Codex runtime rewrites. The blanket rewrite collapsed
# using-flywheel's runtime trio to two names and printed one install path twice,
# silently deleting the check for the runtime the step exists to verify. The
# rewrite table is read from scripts/codex-sync.sh itself, so a new rewrite is
# covered without editing this test. A collision is a source line that already
# carries a rewrite's Codex-side text and would gain another copy of it.
@test "cross-runtime skills are listed and their twins skip the runtime rewrites" {
    run python3 - <<'PYCHECK'
import ast
import os
import pathlib
import re

repo = pathlib.Path(os.environ["REPO_ROOT"])
generator = (repo / "scripts" / "codex-sync.sh").read_text(encoding="utf-8")
program = re.search(r"^python3 - <<'PY'\n(.*?)^PY$", generator, re.S | re.M)
if program is None:
    raise SystemExit("scripts/codex-sync.sh: embedded generator program not found")
rewrites = None
for node in ast.parse(program.group(1)).body:
    target = node.target if isinstance(node, ast.AnnAssign) else (
        node.targets[0] if isinstance(node, ast.Assign) else None)
    if isinstance(target, ast.Name) and target.id == "RUNTIME_REWRITES":
        rewrites = ast.literal_eval(node.value)
if not rewrites:
    raise SystemExit("scripts/codex-sync.sh: RUNTIME_REWRITES not found")

mapping = dict(rewrites)
pattern = re.compile("|".join(
    re.escape(old) for old, _ in sorted(rewrites, key=lambda kv: len(kv[0]), reverse=True)))


def body(path):
    return path.read_text(encoding="utf-8").split("---", 2)[2]


listed = set()
for line in (repo / "scripts" / "lint" / "codex-cross-runtime-skills.txt").read_text(
        encoding="utf-8").splitlines():
    line = line.strip()
    if line and not line.startswith("#"):
        listed.add(line)

failures = []
for skill in sorted(listed):
    source, twin = repo / "skills" / skill / "SKILL.md", repo / "skills-codex" / skill / "SKILL.md"
    if not source.is_file() or not twin.is_file():
        failures.append(f"{skill}: listed but skills/ or skills-codex/ SKILL.md is missing")
        continue
    for old, _ in rewrites:
        if body(source).count(old) != body(twin).count(old):
            failures.append(f"{skill}: twin rewrote {old!r} although the skill is cross-runtime")

for source in sorted((repo / "skills").glob("*/SKILL.md")):
    skill = source.parent.name
    for lineno, line in enumerate(body(source).splitlines(), 1):
        rewritten = pattern.sub(lambda m: mapping[m.group(0)], line)
        collided = sorted({new for _, new in rewrites
                           if new in line and rewritten.count(new) > line.count(new)})
        if collided and skill not in listed:
            failures.append(f"{skill}: body line {lineno} names both runtimes {collided}; "
                            "list it in scripts/lint/codex-cross-runtime-skills.txt")

if failures:
    raise SystemExit("\n".join(failures))
print(f"checked {len(listed)} listed cross-runtime twins and every source body")
PYCHECK
    echo "$output" >&2
    [ "$status" -eq 0 ]
}

# D ── the slash-to-\$ rewrite is for slash-COMMAND invocations in prose, never
# for the document title. `# \$route` is not a heading anyone reads.
@test "no Codex twin title starts with '# \$'" {
    run bash -c "grep -l '^# \\\$' \"$REPO_ROOT\"/skills-codex/*/SKILL.md || true"
    [ "$status" -eq 0 ]
    if [ -n "$output" ]; then
        echo "twins with a \$-rewritten title:" >&2
        echo "$output" >&2
    fi
    [ -z "$output" ]
}
