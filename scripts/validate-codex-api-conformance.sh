#!/usr/bin/env bash
# validate-codex-api-conformance.sh — check that Codex can load skills/ and
# that each skill keeps its Codex invocation policy.
#
# Codex loads the canonical skills/ tree directly: the plugin manifest points
# at ./skills and `ao skills link` symlinks the same directories. There is no
# generated copy, so this is the one place the Codex-specific facts are held.
# Every rule below is a fact observed from the Codex skill loader (`skills/list`
# on codex-cli 0.156.1), not a wording preference:
#
#   1. A SKILL.md nested below skills/<name>/ is loaded as a skill of its own.
#      The loader walks the tree, so a fixture or scaffold SKILL.md under
#      skills/ ships to every plugin user.
#   2. A skill is refused when its frontmatter is not a YAML mapping, repeats a
#      key, has no non-empty `description`, or has a `name` over 64 characters.
#      Host-only frontmatter fields and long descriptions load fine.
#   3. Codex reads the invocation policy from agents/openai.yaml, never from
#      SKILL.md. A skill marked `disable-model-invocation: true` therefore
#      needs `policy.allow_implicit_invocation: false` there, or Codex selects
#      it implicitly. A malformed agents/openai.yaml is ignored silently, which
#      has the same effect, so the file must parse.
#
# Usage: scripts/validate-codex-api-conformance.sh
# Env:   CODEX_SKILLS_ROOT  skills root to check (default: <repo>/skills)
# Exit:  0 = pass, 1 = findings.
# Contract: docs/contracts/codex-skill-api.md
# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

SKILLS_ROOT="${CODEX_SKILLS_ROOT:-$REPO_ROOT/skills}"

if [[ ! -d "$SKILLS_ROOT" ]]; then
  echo "Error: skills directory not found: $SKILLS_ROOT" >&2
  exit 1
fi

python3 - "$SKILLS_ROOT" <<'PY'
import sys
from pathlib import Path

import yaml
from yaml.constructor import ConstructorError
from yaml.resolver import BaseResolver

skills_root = Path(sys.argv[1]).resolve()
failures: list[str] = []
checked = 0


class UniqueKeyLoader(yaml.SafeLoader):
    """Reject a repeated mapping key, as the Codex loader does."""


def construct_unique_mapping(loader: UniqueKeyLoader, node: yaml.Node, deep: bool = False) -> dict:
    mapping: dict = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        try:
            duplicate = key in mapping
        except TypeError as exc:
            raise ConstructorError(
                "while constructing a mapping", node.start_mark,
                "found an unhashable mapping key", key_node.start_mark,
            ) from exc
        if duplicate:
            raise ConstructorError(
                "while constructing a mapping", node.start_mark,
                f"found duplicate key {key!r}", key_node.start_mark,
            )
        mapping[key] = loader.construct_object(value_node, deep=deep)
    return mapping


UniqueKeyLoader.add_constructor(BaseResolver.DEFAULT_MAPPING_TAG, construct_unique_mapping)


def fail(skill: str, message: str) -> None:
    failures.append(f"  FAIL [{skill}] {message}")


def frontmatter(skill: str, skill_md: Path) -> dict | None:
    try:
        text = skill_md.read_text(encoding="utf-8")
    except (OSError, UnicodeError) as exc:
        fail(skill, f"SKILL.md is not loadable UTF-8: {exc}")
        return None
    if not text.startswith("---\n"):
        fail(skill, "SKILL.md must start with YAML frontmatter")
        return None
    end = text.find("\n---\n", 4)
    if end < 0:
        fail(skill, "SKILL.md frontmatter is not closed")
        return None
    try:
        data = yaml.load(text[4:end], Loader=UniqueKeyLoader)
    except yaml.YAMLError as exc:
        fail(skill, f"invalid YAML frontmatter: {' '.join(str(exc).split())}")
        return None
    if not isinstance(data, dict):
        fail(skill, "frontmatter must be a mapping")
        return None
    return data


def invocation_policy(skill: str, skill_dir: Path) -> tuple[bool, object]:
    """Return (readable, allow_implicit_invocation) from agents/openai.yaml."""
    path = skill_dir / "agents" / "openai.yaml"
    if not path.exists():
        return True, None
    try:
        data = yaml.load(path.read_text(encoding="utf-8"), Loader=UniqueKeyLoader)
    except (OSError, UnicodeError, yaml.YAMLError) as exc:
        fail(skill, f"agents/openai.yaml is not valid YAML: {' '.join(str(exc).split())}")
        return False, None
    if data is None:
        return True, None
    if not isinstance(data, dict):
        fail(skill, "agents/openai.yaml must be a mapping")
        return False, None
    policy = data.get("policy")
    if policy is None:
        return True, None
    if not isinstance(policy, dict):
        fail(skill, "agents/openai.yaml policy must be a mapping")
        return False, None
    allow = policy.get("allow_implicit_invocation")
    if allow is not None and not isinstance(allow, bool):
        fail(skill, "agents/openai.yaml policy.allow_implicit_invocation must be a boolean")
        return False, None
    return True, allow


for nested in sorted(skills_root.rglob("SKILL.md")):
    relative = nested.relative_to(skills_root)
    if len(relative.parts) != 2:
        fail(relative.parts[0], f"nested {relative.as_posix()} would be loaded by Codex as its own skill")

for skill_dir in sorted(path for path in skills_root.iterdir() if path.is_dir()):
    skill = skill_dir.name
    skill_md = skill_dir / "SKILL.md"
    if not skill_md.is_file():
        continue  # reported above when a SKILL.md hides deeper in the tree
    checked += 1
    data = frontmatter(skill, skill_md)
    if data is None:
        continue
    description = data.get("description")
    if not isinstance(description, str) or not description.strip():
        fail(skill, "description must be a nonempty string")
    name = data.get("name")
    if name is not None and (not isinstance(name, str) or len(name) > 64):
        fail(skill, "name must be a string of at most 64 characters")
    disabled = data.get("disable-model-invocation", False)
    if not isinstance(disabled, bool):
        fail(skill, "disable-model-invocation must be a boolean")
        disabled = False
    readable, allow = invocation_policy(skill, skill_dir)
    if readable and disabled and allow is not False:
        fail(
            skill,
            "disable-model-invocation: true needs agents/openai.yaml with "
            "policy.allow_implicit_invocation: false, or Codex invokes the skill implicitly",
        )

if checked == 0:
    failures.append("  FAIL [<root>] no skill packages found")

for line in failures:
    print(line)
if failures:
    print(f"Codex skill conformance FAILED with {len(failures)} finding(s).")
    raise SystemExit(1)
print(f"  PASS [codex] {checked} package(s)")
print("Codex skill conformance passed.")
PY
