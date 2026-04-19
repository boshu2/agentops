"""Generate skill catalog pages at MkDocs build time.

Walks skills/*/SKILL.md and produces:
  - docs/skills/catalog.md      — single-page catalog of every skill
  - docs/skills/<name>.md       — individual skill detail page per skill

Skill pages are generated via the mkdocs-gen-files plugin so they never exist
on disk — this sidesteps the "no symlinks" rule in CLAUDE.md while giving the
published site first-class skill pages.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import mkdocs_gen_files

REPO_ROOT = Path(__file__).resolve().parent.parent.parent
SKILLS_DIR = REPO_ROOT / "skills"

FRONTMATTER_RE = re.compile(r"^---\s*\n(.*?)\n---\s*\n?", re.DOTALL)


def parse_frontmatter(text: str) -> tuple[dict[str, str], str]:
    match = FRONTMATTER_RE.match(text)
    if not match:
        return {}, text
    block = match.group(1)
    body = text[match.end():]
    data: dict[str, str] = {}
    for line in block.splitlines():
        if ":" not in line:
            continue
        key, _, value = line.partition(":")
        data[key.strip()] = value.strip().strip('"').strip("'")
    return data, body


def load_skill(skill_dir: Path) -> dict[str, str] | None:
    skill_md = skill_dir / "SKILL.md"
    if not skill_md.is_file():
        return None
    text = skill_md.read_text(encoding="utf-8")
    fm, body = parse_frontmatter(text)
    return {
        "name": fm.get("name", skill_dir.name),
        "description": fm.get("description", ""),
        "body": body.strip(),
        "slug": skill_dir.name,
        "relpath": str(skill_md.relative_to(REPO_ROOT)),
    }


def collect_skills() -> list[dict[str, str]]:
    if not SKILLS_DIR.is_dir():
        print(f"[gen_skill_pages] skills dir not found: {SKILLS_DIR}", file=sys.stderr)
        return []
    skills: list[dict[str, str]] = []
    for skill_dir in sorted(SKILLS_DIR.iterdir()):
        if not skill_dir.is_dir():
            continue
        skill = load_skill(skill_dir)
        if skill:
            skills.append(skill)
    return skills


def emit_catalog(skills: list[dict[str, str]]) -> None:
    lines = [
        "# Skills Catalog",
        "",
        f"AgentOps ships **{len(skills)} skills**. Each skill is a reusable, "
        "frontmatter-declared capability that Claude, Codex, and other harnesses "
        "can invoke. Skills live under `skills/<name>/SKILL.md`.",
        "",
        "!!! tip",
        "    Use the search bar to jump to a skill by name or keyword.",
        "",
        "## Index",
        "",
    ]
    for skill in skills:
        lines.append(f"- [`{skill['slug']}`]({skill['slug']}.md) — {skill['description']}")
    lines.extend(["", "---", ""])
    for skill in skills:
        lines.append(f"## `{skill['slug']}`")
        lines.append("")
        if skill["description"]:
            lines.append(f"> {skill['description']}")
            lines.append("")
        lines.append(f"[:octicons-book-24: Full page]({skill['slug']}.md){{ .md-button }} "
                     f"[:octicons-mark-github-24: Source](https://github.com/boshu2/agentops/"
                     f"blob/main/{skill['relpath']}){{ .md-button }}")
        lines.append("")

    with mkdocs_gen_files.open("skills/catalog.md", "w") as fh:
        fh.write("\n".join(lines))


def emit_detail(skill: dict[str, str]) -> None:
    page = []
    page.append(f"# `{skill['slug']}`")
    page.append("")
    if skill["description"]:
        page.append(f"> {skill['description']}")
        page.append("")
    page.append(f"**Source:** [`{skill['relpath']}`](https://github.com/boshu2/agentops/"
                f"blob/main/{skill['relpath']})")
    page.append("")
    page.append("---")
    page.append("")
    # Strip the skill's own H1 if present (we already render one above).
    body = skill["body"]
    body = re.sub(r"^#\s+.+\n+", "", body, count=1)
    # Rewrite relative `references/*` links to absolute GitHub links so they resolve
    # in the published site (the per-skill references/ tree is not shipped to docs/).
    body = re.sub(
        r"\(references/([^)]+)\)",
        lambda m: f"(https://github.com/boshu2/agentops/blob/main/skills/{skill['slug']}/references/{m.group(1)})",
        body,
    )
    # Rewrite other same-skill relative paths (e.g., scripts/, templates/) the same way.
    body = re.sub(
        r"\((scripts|templates|fixtures|examples)/([^)]+)\)",
        lambda m: f"(https://github.com/boshu2/agentops/blob/main/skills/{skill['slug']}/{m.group(1)}/{m.group(2)})",
        body,
    )
    # Rewrite `../<other-skill>/SKILL.md` (sibling skill links) to flat catalog pages.
    body = re.sub(
        r"\]\(\.\./([a-zA-Z0-9_-]+)/SKILL\.md\)",
        r"](\1.md)",
        body,
    )
    # SKILL.md files live at skills/<name>/ so `../../docs/X` climbs to repo root,
    # then into docs/X. The generated page lives at docs/skills/<name>.md so the
    # equivalent is `../X`.
    body = re.sub(
        r"\]\(\.\./\.\./docs/([^)]+)\)",
        r"](../\1)",
        body,
    )
    # `../../cli/docs/COMMANDS.md` and `../../cli/docs/HOOKS.md` -> generated CLI pages
    body = re.sub(
        r"\]\(\.\./\.\./cli/docs/COMMANDS\.md\)",
        "](../cli/commands.md)",
        body,
    )
    body = re.sub(
        r"\]\(\.\./\.\./cli/docs/HOOKS\.md\)",
        "](../cli/hooks.md)",
        body,
    )
    # Any other `../../X` from SKILL.md climbs out of skills/ — send to GitHub.
    body = re.sub(
        r"\]\(\.\./\.\./([^)]+)\)",
        r"](https://github.com/boshu2/agentops/blob/main/\1)",
        body,
    )
    # Rewrite `../shared/references/*` (shared skill references) to GitHub.
    body = re.sub(
        r"\]\(\.\./shared/references/([^)]+)\)",
        r"](https://github.com/boshu2/agentops/blob/main/skills/shared/references/\1)",
        body,
    )
    # Any remaining `../<dir>/<file>` is another skill's subdir — send to GitHub.
    body = re.sub(
        r"\]\(\.\./([a-zA-Z0-9_-]+)/([^)]+)\)",
        r"](https://github.com/boshu2/agentops/blob/main/skills/\1/\2)",
        body,
    )
    page.append(body)

    with mkdocs_gen_files.open(f"skills/{skill['slug']}.md", "w") as fh:
        fh.write("\n".join(page))


def emit_skills_index(skills: list[dict[str, str]]) -> None:
    """Write the skills landing page (docs/skills/index.md)."""
    lines = [
        "# Skills",
        "",
        "Skills are the composable units of AgentOps. Each one is a declarative "
        "capability — a prompt contract with optional scripts, references, and "
        "enforced metadata — that any compatible harness (Claude Code, Codex, "
        "OpenCode) can invoke.",
        "",
        f"**{len(skills)} skills ship with AgentOps.**",
        "",
        "<div class=\"grid cards\" markdown>",
        "",
        "- :material-format-list-bulleted: **[Catalog](catalog.md)**  ",
        "  Browse every skill on a single page with descriptions and source links.",
        "",
        "- :material-sitemap: **[Skill Tiers](../skills/catalog.md#index)**  ",
        "  User-facing vs internal skills, dependency graph, and tier taxonomy.",
        "",
        "- :material-routes: **[Decision Tree](../skills-decision-tree.md)**  ",
        "  \"Which skill do I need next?\" — single source of truth.",
        "",
        "- :material-api: **[Skill API](../SKILL-API.md)**  ",
        "  Frontmatter fields, context declarations, and enforcement status.",
        "",
        "</div>",
        "",
        "## Featured Workflows",
        "",
        "| Skill | What it does |",
        "|-------|---------------|",
    ]
    featured = [
        "rpi", "discovery", "crank", "validation", "implement",
        "research", "plan", "review", "vibe", "post-mortem", "retro",
        "swarm", "council", "evolve", "release", "beads",
    ]
    by_slug = {s["slug"]: s for s in skills}
    for slug in featured:
        skill = by_slug.get(slug)
        if not skill:
            continue
        lines.append(f"| [`{slug}`]({slug}.md) | {skill['description']} |")

    lines.extend([
        "",
        "## Skill Tiers",
        "",
        "See [SKILLS.md](../SKILLS.md) for the authoritative tier breakdown and "
        "[SKILL-ROUTER.md](../SKILL-ROUTER.md) for the routing decisions.",
        "",
    ])
    with mkdocs_gen_files.open("skills/index.md", "w") as fh:
        fh.write("\n".join(lines))


def main() -> None:
    skills = collect_skills()
    if not skills:
        print("[gen_skill_pages] no skills found — skipping", file=sys.stderr)
        return
    emit_catalog(skills)
    emit_skills_index(skills)
    for skill in skills:
        emit_detail(skill)
    print(f"[gen_skill_pages] emitted {len(skills)} skill pages", file=sys.stderr)


main()
