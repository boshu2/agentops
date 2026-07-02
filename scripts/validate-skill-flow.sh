#!/usr/bin/env bash
# validate-skill-flow.sh — Enforce skill-flow connectivity across every
# skills/<name>/SKILL.md.
#
# Motivation (follow-up to scripts/audit-skill-metadata.sh, ag-f0i):
#   audit-skill-metadata.sh owns `context_rel.with` resolution. It explicitly
#   deferred two checks as "discovered follow-ups, not enforced":
#     1. `consumes` vocabulary canonicality (open vocabulary, no registry).
#     2. skill-to-skill connectivity ("do all skills flow together?").
#   This gate closes both, plus checks `metadata.dependencies` resolution.
#
# What it checks (FAIL = exit 1):
#   1. CLOSED CONSUMES VOCABULARY. Every `consumes` token must be either a real
#      skill slug or one of the whitelisted EXTERNAL_INPUTS (see below). This
#      turns `consumes` from an open free-text field into a closed contract so
#      the producer->consumer graph can be reasoned about.
#   2. metadata.dependencies RESOLUTION. Every `metadata.dependencies` entry
#      must name an existing skill slug.
#   3. ORPHAN DETECTION. A skill is "connected" if it shares at least one
#      skill-to-skill edge with another skill, counting ALL THREE edge layers
#      (consumes skill-slugs, context_rel.with skill-slugs, metadata.dependencies).
#      Orphans must be listed in the standalone allowlist
#      (scripts/skill-flow-standalone.txt) — intentionally-standalone meta /
#      utility / boundary skills. An un-allowlisted orphan FAILS.
#
# What it REPORTS (informational, never fails):
#   - Cross-layer disagreement: `consumes` skill-slugs vs `metadata.dependencies`
#     (the two fields drifted historically; surfaced so they can be reconciled).
#   - Dead-end produced artifacts (produced but consumed by no skill).
#
# context_rel.with resolution stays owned by audit-skill-metadata.sh — this gate
# does not re-litigate it (and tolerates `*.md` doc targets used by entry-point
# skills such as session-bootstrap).
#
# Usage:
#   bash scripts/validate-skill-flow.sh [--json] [--skills-root DIR] [--allowlist FILE]
#
#   --json          emit a machine-readable verdict on stdout (stdout = data only)
#   --skills-root   directory holding skills/<name>/SKILL.md
#                   (default: <repo>/skills, or $SKILL_FLOW_SKILLS_ROOT)
#   --allowlist     standalone-skill allowlist file
#                   (default: <repo>/scripts/skill-flow-standalone.txt)
#   -h, --help      show this help
#
# Exit codes: 0 = clean, 1 = findings, 2 = usage / environment error.
#
# Contract reference: docs/contracts/skill-flow.md
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

JSON=0
SKILLS_ROOT="${SKILL_FLOW_SKILLS_ROOT:-${REPO_ROOT}/skills}"
ALLOWLIST="${SKILL_FLOW_ALLOWLIST:-${REPO_ROOT}/scripts/skill-flow-standalone.txt}"

usage() {
    cat <<'USAGE'
validate-skill-flow.sh — enforce skill-flow connectivity across all SKILL.md.

Checks (fail): closed consumes vocabulary, metadata.dependencies resolution,
and orphan detection (un-allowlisted skill with zero skill-to-skill edges).
Reports (advisory): consumes vs metadata.dependencies disagreement, dead-end
produced artifacts.

Usage:
  bash scripts/validate-skill-flow.sh [--json] [--skills-root DIR] [--allowlist FILE]

  --json          emit a machine-readable verdict on stdout (stdout = data only)
  --skills-root   directory holding skills/<name>/SKILL.md
                  (default: <repo>/skills, or $SKILL_FLOW_SKILLS_ROOT)
  --allowlist     standalone-skill allowlist file
                  (default: <repo>/scripts/skill-flow-standalone.txt)
  -h, --help      show this help

Exit codes: 0 = clean, 1 = findings, 2 = usage / environment error.
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --json) JSON=1; shift ;;
        --skills-root) SKILLS_ROOT="${2:?--skills-root needs a value}"; shift 2 ;;
        --allowlist) ALLOWLIST="${2:?--allowlist needs a value}"; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        --*) echo "ERROR: unknown flag: $1 (try: bash scripts/validate-skill-flow.sh --help)" >&2; exit 2 ;;
        *) echo "ERROR: unexpected argument: $1 (try: bash scripts/validate-skill-flow.sh --help)" >&2; exit 2 ;;
    esac
done

if [[ ! -d "${SKILLS_ROOT}" ]]; then
    echo "ERROR: skills directory not found at ${SKILLS_ROOT}" >&2
    exit 2
fi

SKILLS_ROOT="${SKILLS_ROOT}" ALLOWLIST="${ALLOWLIST}" JSON="${JSON}" python3 - <<'PYEOF'
import json
import os
import re
import sys
from pathlib import Path

try:
    import yaml  # type: ignore
except Exception as e:  # pragma: no cover - environment guard
    sys.stderr.write("ERROR: PyYAML is required (pip install pyyaml). underlying: %s\n" % e)
    sys.exit(2)

SKILLS_ROOT = Path(os.environ["SKILLS_ROOT"])
ALLOWLIST = Path(os.environ["ALLOWLIST"])
JSON = os.environ.get("JSON") == "1"

# Closed vocabulary for non-skill `consumes` tokens. These are the *external
# inputs* a skill may read that are not themselves produced by a peer skill
# (VCS state, the br issue store, an upstream API, the repo working tree, the
# onboarding handshake). Adding a new external input is a deliberate act: extend
# this list AND document it in docs/contracts/skill-flow.md.
EXTERNAL_INPUTS = {
    "Cargo.lock",
    "Cargo.toml",
    "br",
    "build-config",
    "cargo-metadata",
    "cli-source",
    "closed-beads",
    "code",
    "code-under-review",
    "codebase",
    "codex-plugin",
    "command-help",
    "command-map",
    "convention-target",
    "crate-docs",
    "crate-source",
    "data-model",
    "environment-contract",
    "error-reports",
    "evidence",
    "existing-docs",
    "existing-tracked-work",
    "external-api",
    "external-source-candidates",
    "failure-report",
    "ffi-bindings",
    "ffi-contracts",
    "gemini-extension",
    "git",
    "git-worktree",
    "github-pr",
    "hook-policy",
    "implementation-examples",
    "installation-docs",
    "manifest-and-lockfile",
    "mcp-server",
    "onboard",
    "operational-constraints",
    "package-metadata",
    "product-requirements",
    "profiler-output",
    "project-context",
    "project-goals",
    "project-source",
    "release-notes",
    "repo-context",
    "repo-tree",
    "repository",
    "runtime-configuration",
    "runtime-metrics",
    "rust-source",
    "service-contract",
    "skill-bundle",
    "source-code",
    "specification",
    "support-history",
    "task-intent",
    "task-question",
    "test-plan",
    "test-results",
    "test-suite",
    "test-target",
    "tests",
}

FRONTMATTER_RE = re.compile(r"^---\s*\n(.*?)\n---\s*\n", re.DOTALL)


def parse_frontmatter(skill_md):
    try:
        text = skill_md.read_text(encoding="utf-8")
    except Exception:
        return {}
    m = FRONTMATTER_RE.match(text)
    if not m:
        return {}
    try:
        data = yaml.safe_load(m.group(1))
    except Exception:
        return {}
    return data if isinstance(data, dict) else {}


def str_list(value):
    if not isinstance(value, list):
        return []
    return [str(x) for x in value if x is not None]


def load_allowlist(path):
    """Return set of allowlisted standalone skill slugs (# comments allowed)."""
    if not path.is_file():
        return set()
    out = set()
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.split("#", 1)[0].strip()
        if line:
            out.add(line)
    return out


# 1. Load every skill's frontmatter.
skill_dirs = sorted(
    p for p in SKILLS_ROOT.iterdir() if p.is_dir() and (p / "SKILL.md").is_file()
)
names = {p.name for p in skill_dirs}

skills = {}
for sd in skill_dirs:
    fm = parse_frontmatter(sd / "SKILL.md")
    consumes = str_list(fm.get("consumes"))
    produces = str_list(fm.get("produces"))
    ctx = []
    for e in (fm.get("context_rel") or []):
        if isinstance(e, dict) and isinstance(e.get("with"), str):
            ctx.append(e["with"].strip())
    md = fm.get("metadata") or {}
    mdeps = str_list(md.get("dependencies")) if isinstance(md, dict) else []
    skills[sd.name] = {
        "consumes": consumes,
        "produces": produces,
        "ctx": ctx,
        "mdeps": mdeps,
    }

allowlist = load_allowlist(ALLOWLIST)

# Every artifact produced by some skill — a `consumes` token may legitimately
# name one (e.g. push consumes git-changes; beads consumes bd-issue).
produced_artifacts = set()
for d in skills.values():
    produced_artifacts.update(d["produces"])

failures = []  # list of (kind, slug, detail)

# CHECK 1: closed consumes vocabulary. A consumes token must resolve to one of:
# a peer skill slug, a whitelisted external input, or an artifact produced by
# some skill. Anything else is a typo or an undeclared dependency.
for slug in sorted(skills):
    for tok in skills[slug]["consumes"]:
        if tok in names or tok in EXTERNAL_INPUTS or tok in produced_artifacts:
            continue
        failures.append((
            "consumes-vocabulary",
            slug,
            "consumes '%s' resolves to nothing: not a skill slug, not a "
            "whitelisted external input (%s), and not an artifact any skill "
            "produces" % (tok, ", ".join(sorted(EXTERNAL_INPUTS))),
        ))

# CHECK 2: metadata.dependencies resolution.
for slug in sorted(skills):
    for tok in skills[slug]["mdeps"]:
        if tok not in names:
            failures.append((
                "metadata-dependencies",
                slug,
                "metadata.dependencies '%s' does not resolve to a skill slug" % tok,
            ))

# Build undirected skill-to-skill edge set across all three layers.
edges = set()
for slug, d in skills.items():
    for tok in d["consumes"]:
        if tok in names and tok != slug:
            edges.add(frozenset((slug, tok)))
    for tok in d["ctx"]:
        if tok in names and tok != slug:
            edges.add(frozenset((slug, tok)))
    for tok in d["mdeps"]:
        if tok in names and tok != slug:
            edges.add(frozenset((slug, tok)))

degree = {slug: 0 for slug in skills}
for e in edges:
    for slug in e:
        degree[slug] += 1

orphans = sorted(s for s in skills if degree[s] == 0)

# CHECK 3: orphans must be allowlisted.
unallowed_orphans = [s for s in orphans if s not in allowlist]
for slug in unallowed_orphans:
    failures.append((
        "orphan",
        slug,
        "no skill-to-skill edge in consumes/context_rel/metadata.dependencies; "
        "wire an edge or add to scripts/skill-flow-standalone.txt with a rationale",
    ))

# Stale allowlist entries (allowlisted but actually connected, or not a skill).
stale_allowlist = sorted(
    s for s in allowlist if s not in skills or (s in skills and degree[s] > 0)
)

# Informational: consumes-skill vs metadata.dependencies disagreement.
disagreements = []
for slug in sorted(skills):
    cs = {t for t in skills[slug]["consumes"] if t in names}
    md = set(skills[slug]["mdeps"])
    if (cs or md) and cs != md:
        disagreements.append((slug, sorted(cs), sorted(md)))

# Informational: dead-end produced artifacts (produced, consumed by no skill).
consumed_tokens = set()
for d in skills.values():
    consumed_tokens.update(d["consumes"])
produced = {}
for slug, d in skills.items():
    for art in d["produces"]:
        produced.setdefault(art, []).append(slug)
dead_end = sorted(a for a in produced if a not in consumed_tokens)

verdict = "PASS" if not failures else "FAIL"

if JSON:
    print(json.dumps({
        "verdict": verdict,
        "skills_checked": len(skills),
        "edges": len(edges),
        "orphans": orphans,
        "failures": [
            {"kind": k, "skill": s, "detail": d} for (k, s, d) in failures
        ],
        "stale_allowlist": stale_allowlist,
        "disagreements": [
            {"skill": s, "consumes": cs, "metadata_dependencies": md}
            for (s, cs, md) in disagreements
        ],
        "dead_end_artifacts": dead_end,
    }, indent=2, sort_keys=True))
else:
    print("validate-skill-flow: %d skill(s), %d skill-to-skill edge(s)" % (
        len(skills), len(edges)))
    print("  orphans: %d (allowlisted standalone: %d)" % (
        len(orphans), len([o for o in orphans if o in allowlist])))
    if disagreements:
        print("")
        print("INFO: %d skill(s) where consumes(skills) != metadata.dependencies "
              "(reconcile, not fatal):" % len(disagreements))
        for slug, cs, md in disagreements:
            print("  - %-26s consumes=%s metadata.deps=%s" % (
                slug, cs or "-", md or "-"))
    if dead_end:
        print("")
        print("INFO: %d produced artifact(s) consumed by no skill "
              "(output-type annotation, not fatal):" % len(dead_end))
        for art in dead_end:
            print("  - %s (from: %s)" % (art, ", ".join(produced[art])))
    if stale_allowlist:
        print("")
        print("WARN: %d stale allowlist entry/entries (now connected or not a "
              "skill — remove from scripts/skill-flow-standalone.txt):"
              % len(stale_allowlist))
        for slug in stale_allowlist:
            print("  - %s" % slug)
    if failures:
        print("")
        print("FAIL: %d finding(s):" % len(failures))
        for kind, slug, detail in failures:
            print("  [%s] %s/SKILL.md: %s" % (kind, slug, detail))
        print("")
        print("fix: see docs/contracts/skill-flow.md")
    else:
        print("")
        print("OK: skill flow is connected and the consumes vocabulary is closed.")

sys.exit(0 if verdict == "PASS" else 1)
PYEOF
