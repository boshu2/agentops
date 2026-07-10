#!/usr/bin/env bash
# scripts/generate-context-map.sh
#
# Generate docs/contracts/context-map.md from skills/*/SKILL.md frontmatter.
#
# Reads YAML frontmatter from each SKILL.md and emits:
#   - sections grouped by hexagonal_role
#   - Mermaid graph of context_rel edges
#   - data-flow table of consumes/produces
#
# Determinism contract (Fix 4 from .agents/plans/2026-05-12-ddd-hexagonal-plan.md):
#   * Skills sorted alphabetical by slug within every section.
#   * Mermaid edges sorted by (source_slug, target_slug, kind).
#   * Data-flow table rows sorted by (skill_slug, direction, artifact).
#   * NO wall-clock timestamp in the file body. The leading HTML comment is
#     date-free so two runs on different days produce byte-identical output.
#
# Idempotency: running this twice in a row produces zero diff.
#
# Schema reference: schemas/skill-frontmatter.v2.schema.json
# Plan reference:   .agents/plans/2026-05-12-ddd-hexagonal-plan.md (Issue #4)

set -euo pipefail

# Resolve repo root from this script's location (scripts/<this>.sh).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

SKILLS_DIR="${REPO_ROOT}/skills"
OUT_FILE="${REPO_ROOT}/docs/contracts/context-map.md"

if [[ ! -d "${SKILLS_DIR}" ]]; then
  echo "ERROR: skills directory not found at ${SKILLS_DIR}" >&2
  exit 2
fi

mkdir -p "$(dirname "${OUT_FILE}")"

# The Python embedded below does all parsing + emission. We pipe the resolved
# skills directory and output path in via env vars to keep the shell wrapper
# minimal and avoid quoting issues.
SKILLS_DIR="${SKILLS_DIR}" OUT_FILE="${OUT_FILE}" python3 - <<'PYEOF'
import io
import json
import os
import re
import subprocess
import sys
from pathlib import Path

try:
    import yaml  # type: ignore
except Exception as e:
    sys.stderr.write(
        "ERROR: PyYAML is required (pip install pyyaml). underlying: %s\n" % e
    )
    sys.exit(2)

SKILLS_DIR = Path(os.environ["SKILLS_DIR"])
OUT_FILE = Path(os.environ["OUT_FILE"])
REPO_ROOT = SKILLS_DIR.parent

# Valid hexagonal_role enum from schemas/skill-frontmatter.v2.schema.json plus
# "unclassified" used for skills missing the field.
ROLES_ORDER = [
    "domain",
    "driving-adapter",
    "driven-adapter",
    "supporting",
    "generic",
    "unclassified",
]

FRONTMATTER_RE = re.compile(r"^---\s*\n(.*?)\n---\s*\n", re.DOTALL)


def parse_frontmatter(skill_md: Path):
    """Return parsed YAML dict (possibly empty) for a SKILL.md file.

    Tolerates missing frontmatter, malformed YAML (treated as empty dict so
    the skill still appears as 'unclassified'), and frontmatter blocks that
    aren't a YAML mapping at the top level.
    """
    try:
        text = skill_md.read_text(encoding="utf-8")
    except Exception:
        return {}
    m = FRONTMATTER_RE.match(text)
    if not m:
        return {}
    raw = m.group(1)
    try:
        data = yaml.safe_load(raw)
    except Exception:
        return {}
    if not isinstance(data, dict):
        return {}
    return data


def short_description(fm):
    desc = fm.get("description", "")
    if not isinstance(desc, str):
        return ""
    desc = desc.strip()
    # Strip surrounding single/double quotes that survived YAML loading edge
    # cases (defensive — yaml.safe_load normally handles this).
    if (desc.startswith("'") and desc.endswith("'")) or (
        desc.startswith('"') and desc.endswith('"')
    ):
        desc = desc[1:-1].strip()
    # Collapse internal newlines/whitespace runs to single space.
    desc = re.sub(r"\s+", " ", desc)
    return desc


def normalize_string_list(value):
    """Return a sorted list of strings from a list-or-None YAML value.

    Non-string entries are stringified. Empty/missing → []. Result is sorted
    for output determinism (callers may want pre-sort ordering; this helper is
    used only where final order is alphabetic anyway).
    """
    if not value:
        return []
    if not isinstance(value, list):
        return []
    out = []
    for item in value:
        if item is None:
            continue
        out.append(str(item))
    out.sort()
    return out


def normalize_context_rel(value):
    """Return list of (kind, with) tuples, filtered to valid string pairs."""
    if not value or not isinstance(value, list):
        return []
    edges = []
    for item in value:
        if not isinstance(item, dict):
            continue
        kind = item.get("kind")
        target = item.get("with")
        if not isinstance(kind, str) or not isinstance(target, str):
            continue
        kind = kind.strip()
        target = target.strip()
        if not kind or not target:
            continue
        edges.append((kind, target))
    return edges


# 1. Generate the catalog in-memory and consume it as the sole graph
# projection. This avoids a second dependency/context parser in the context-map
# generator while still reflecting uncommitted frontmatter edits.
catalog_run = subprocess.run(
    ["bash", str(REPO_ROOT / "scripts/generate-skill-catalog.sh"), "--stdout"],
    cwd=REPO_ROOT,
    text=True,
    capture_output=True,
)
if catalog_run.returncode != 0:
    sys.stderr.write(catalog_run.stderr)
    sys.exit(catalog_run.returncode)
catalog = json.loads(catalog_run.stdout)
catalog_entries = sorted(catalog.get("skills", []), key=lambda item: item.get("name", ""))

skills_by_role = {role: [] for role in ROLES_ORDER}
mermaid_edges = []  # (source_slug, target_slug, kind)
dataflow_rows = []  # (skill_slug, direction, artifact)
dependency_edges = []  # (source_slug, target_slug)
graph_roots = []
user_invocable = []

for entry in catalog_entries:
    slug = str(entry.get("name", ""))
    role = entry.get("hexagonal_role")
    if not isinstance(role, str) or role not in ROLES_ORDER:
        role = "unclassified"
    desc = str(entry.get("description", "")).strip()
    skills_by_role[role].append((slug, desc))

    for rel in entry.get("context_rel", []):
        if isinstance(rel, dict) and rel.get("kind") and rel.get("with"):
            mermaid_edges.append((slug, str(rel["with"]), str(rel["kind"])))

    for art in sorted(entry.get("consumes", [])):
        dataflow_rows.append((slug, "consumes", art))
    for art in sorted(entry.get("produces", [])):
        dataflow_rows.append((slug, "produces", art))
    for dep in sorted(entry.get("dependencies", [])):
        dependency_edges.append((slug, str(dep)))
    if entry.get("graph_root") is True:
        graph_roots.append(slug)
    if entry.get("user_invocable") is True:
        user_invocable.append(slug)

# 2. Deterministic sort of all output collections.
for role in ROLES_ORDER:
    skills_by_role[role].sort(key=lambda t: t[0])
mermaid_edges.sort(key=lambda t: (t[0], t[1], t[2]))
dataflow_rows.sort(key=lambda t: (t[0], t[1], t[2]))
dependency_edges.sort()
graph_roots.sort()
user_invocable.sort()

# Topology diagnostics are derived from the catalog, never hand-maintained.
names = [str(entry.get("name", "")) for entry in catalog_entries]
known = set(names)
duplicate_nodes = sorted({name for name in names if names.count(name) > 1})
dangling_edges = sorted(
    [(src, dst, "dependency") for src, dst in dependency_edges if dst not in known]
    + [(src, dst, "context:" + kind) for src, dst, kind in mermaid_edges if dst not in known]
)
deps = {name: [] for name in known}
inbound = {name: 0 for name in known}
for src, dst in dependency_edges:
    if dst in known:
        deps[src].append(dst)
        inbound[dst] += 1

reachable = set()
stack = list(graph_roots)
while stack:
    name = stack.pop()
    if name in reachable:
        continue
    reachable.add(name)
    stack.extend(deps.get(name, []))
unreachable = sorted(name for name in known if name not in reachable and name not in graph_roots)
zero_inbound = sorted(name for name in known if inbound[name] == 0)

cycles = []
state = {}
path = []
def visit(name):
    state[name] = 1
    path.append(name)
    for dep in sorted(deps.get(name, [])):
        if state.get(dep, 0) == 0:
            visit(dep)
        elif state.get(dep) == 1:
            start = path.index(dep)
            cycle = path[start:] + [dep]
            if cycle not in cycles:
                cycles.append(cycle)
    path.pop()
    state[name] = 2
for name in sorted(known):
    if state.get(name, 0) == 0:
        visit(name)
cycles.sort()

topology_failures = []
if duplicate_nodes: topology_failures.append("duplicate nodes: " + ", ".join(duplicate_nodes))
if dangling_edges: topology_failures.append("dangling edges: " + ", ".join("%s->%s(%s)" % edge for edge in dangling_edges))
if cycles: topology_failures.append("dependency cycles: " + "; ".join(" -> ".join(c) for c in cycles))
if unreachable: topology_failures.append("unreachable non-roots: " + ", ".join(unreachable))
if topology_failures:
    sys.stderr.write("generate-context-map: invalid skill topology: %s\n" % "; ".join(topology_failures))
    sys.exit(1)


def md_escape_pipe(s: str) -> str:
    """Escape pipe chars for markdown table cells."""
    return s.replace("|", "\\|")


# 3. Emit markdown.
buf = io.StringIO()
buf.write("<!-- generated from skills/*/SKILL.md frontmatter -->\n")
buf.write("\n")
buf.write("# AgentOps Context Map\n")
buf.write("\n")
buf.write(
    "Generated from SKILL.md frontmatter. See "
    "[ADR-0001](https://github.com/boshu2/agentops/blob/main/docs/adr/ADR-0001-ddd-hexagonal-adoption.md)\n"
)
buf.write(
    "and [CDLC](https://github.com/boshu2/agentops/blob/main/docs/cdlc.md)"
    " for the architectural rationale.\n"
)
buf.write("\n")

buf.write("## Skills by hexagonal role\n")
buf.write("\n")
for role in ROLES_ORDER:
    buf.write("### %s\n" % role)
    buf.write("\n")
    rows = skills_by_role[role]
    if not rows:
        if role == "unclassified":
            buf.write("- (no unclassified skills)\n")
        else:
            buf.write("- (no skills in this role yet)\n")
    else:
        for slug, desc in rows:
            if desc:
                buf.write("- `%s` — %s\n" % (slug, desc))
            else:
                buf.write("- `%s`\n" % slug)
    buf.write("\n")

buf.write("## Context relationships\n")
buf.write("\n")

buf.write("## Execution dependencies\n")
buf.write("\n")
buf.write("`A --> B` means skill A declares B in `metadata.dependencies`. ")
buf.write("`user-invocable` does not make an orphan a graph root.\n\n")
buf.write("```mermaid\n")
buf.write("graph LR\n")
for name in names:
    buf.write('  %s["%s"]\n' % (name.replace("-", "_"), name))
for src, dst in dependency_edges:
    if dst in known:
        buf.write("  %s --> %s\n" % (src.replace("-", "_"), dst.replace("-", "_")))
buf.write("```\n\n")

buf.write("## Topology diagnostics\n\n")
buf.write("| Diagnostic | Values |\n")
buf.write("|---|---|\n")
buf.write("| Explicit graph roots | %s |\n" % (", ".join("`%s`" % x for x in graph_roots) or "_(none)_"))
buf.write("| User-invocable skills | %s |\n" % (", ".join("`%s`" % x for x in user_invocable) or "_(none)_"))
buf.write("| Zero-inbound skills | %s |\n" % (", ".join("`%s`" % x for x in zero_inbound) or "_(none)_"))
buf.write("| Dangling targets | _(none)_ |\n")
buf.write("| Dependency cycles | _(none)_ |\n")
buf.write("| Unreachable non-roots | _(none)_ |\n\n")
buf.write("```mermaid\n")
buf.write("graph LR\n")
if mermaid_edges:
    for src, tgt, kind in mermaid_edges:
        # Quote the edge label so kinds with hyphens like `acl-wraps` render
        # cleanly. Node IDs use the slug verbatim — slugs are already
        # mermaid-safe (lowercase, hyphens permitted in node ids).
        buf.write('  %s -- "%s" --> %s\n' % (src, kind, tgt))
else:
    buf.write("  %% no context_rel edges declared yet\n")
buf.write("```\n")
buf.write("\n")

buf.write("## Data flow (consumes / produces)\n")
buf.write("\n")
buf.write("| Skill | Direction | Artifact |\n")
buf.write("|-------|-----------|----------|\n")
if dataflow_rows:
    for slug, direction, artifact in dataflow_rows:
        buf.write(
            "| `%s` | %s | %s |\n"
            % (slug, direction, md_escape_pipe(artifact))
        )
else:
    buf.write("| _(none)_ | _(none)_ | _(no consumes/produces declared yet)_ |\n")

# Final newline; write atomically by going through a tmp file.
content = buf.getvalue()
tmp = OUT_FILE.with_suffix(OUT_FILE.suffix + ".tmp")
tmp.write_text(content, encoding="utf-8")
os.replace(tmp, OUT_FILE)

# Emit a short summary to stdout so the orchestrator / CI can eyeball it.
totals = {role: len(skills_by_role[role]) for role in ROLES_ORDER}
print("wrote %s" % OUT_FILE)
print("skills: %d total" % sum(totals.values()))
for role in ROLES_ORDER:
    print("  %s: %d" % (role, totals[role]))
print("context_rel edges: %d" % len(mermaid_edges))
print("dependency edges: %d" % len(dependency_edges))
print("graph roots: %d" % len(graph_roots))
print("data-flow rows: %d" % len(dataflow_rows))
PYEOF
