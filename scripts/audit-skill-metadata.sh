#!/usr/bin/env bash
# audit-skill-metadata.sh — Audit skill-frontmatter *resolution* across every
# skills/<name>/SKILL.md.
#
# The skill-frontmatter.v2 schema declares `context_rel[].with` as a *skill
# slug*. The existing validate-skill-frontmatter.sh checks presence and schema
# shape, but nothing checks that each `with:` value actually resolves to a peer
# skill directory. This auditor closes that resolution gap: every non-empty
# `context_rel.with` MUST name an existing skill under the skills root.
#
# Scope note (ag-f0i, advisory slice): only the well-defined, non-inventing
# resolution check ships here — `context_rel.with` -> existing skill dir.
# `consumes` may be an artifact-kind OR a skill slug (open vocabulary), and
# `practices`/`produces` canonicality require a canonical registry that does not
# yet exist; those are tracked as discovered follow-ups, not enforced here.
#
# Usage:
#   bash scripts/audit-skill-metadata.sh [--strict] [--json] [--skills-root DIR]
#
#   --strict        exit non-zero when any finding exists (default: advisory, exit 0)
#   --json          emit a machine-readable verdict on stdout (stdout = data only)
#   --skills-root   directory holding skills/<name>/SKILL.md (default: <repo>/skills,
#                   or $SKILL_METADATA_SKILLS_ROOT when set)
#   -h, --help      show this help
#
# Exit codes: 0 = clean (or advisory), 1 = findings in --strict, 2 = usage error.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

STRICT=0
JSON=0
SKILLS_ROOT="${SKILL_METADATA_SKILLS_ROOT:-$ROOT/skills}"

usage() {
    cat <<'USAGE'
audit-skill-metadata.sh — audit skill-frontmatter resolution across all SKILL.md.

Checks that every non-empty context_rel.with value resolves to an existing skill
directory (the skill-frontmatter.v2 schema declares context_rel.with as a skill
slug). Presence/shape stay owned by validate-skill-frontmatter.sh.

Usage:
  bash scripts/audit-skill-metadata.sh [--strict] [--json] [--skills-root DIR]

  --strict        exit non-zero when any finding exists (default: advisory, exit 0)
  --json          emit a machine-readable verdict on stdout (stdout = data only)
  --skills-root   directory holding skills/<name>/SKILL.md
                  (default: <repo>/skills, or $SKILL_METADATA_SKILLS_ROOT)
  -h, --help      show this help

Exit codes: 0 = clean (or advisory), 1 = findings in --strict, 2 = usage error.
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --strict) STRICT=1; shift ;;
        --json) JSON=1; shift ;;
        --skills-root) SKILLS_ROOT="${2:-}"; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        --*) echo "ERROR: unknown flag: $1 (try: bash scripts/audit-skill-metadata.sh --help)" >&2; exit 2 ;;
        *) echo "ERROR: unexpected argument: $1 (try: bash scripts/audit-skill-metadata.sh --help)" >&2; exit 2 ;;
    esac
done

if [[ ! -d "$SKILLS_ROOT" ]]; then
    echo "ERROR: skills root not found: $SKILLS_ROOT" >&2
    exit 1
fi
if ! command -v python3 >/dev/null 2>&1; then
    echo "ERROR: python3 is required" >&2
    exit 1
fi
if ! python3 -c "import yaml" >/dev/null 2>&1; then
    echo "ERROR: python yaml missing (need PyYAML). Try: pip install pyyaml" >&2
    exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
    echo "ERROR: jq is required" >&2
    exit 1
fi

# Parse every SKILL.md frontmatter and emit findings as a JSON object:
#   {"checked_skills": N, "findings": [{file, field, value, message}, ...]}
REPORT="$(
    python3 - "$SKILLS_ROOT" <<'PY'
import glob
import json
import os
import sys

import yaml

skills_root = sys.argv[1]
skill_files = sorted(glob.glob(os.path.join(skills_root, "*", "SKILL.md")))
slugs = {os.path.basename(os.path.dirname(p)) for p in skill_files}

findings = []
for path in skill_files:
    rel = os.path.relpath(path, skills_root)
    with open(path, "r", encoding="utf-8") as fh:
        raw = fh.read()
    if not raw.startswith("---"):
        continue
    parts = raw.split("---", 2)
    if len(parts) < 3:
        continue
    try:
        fm = yaml.safe_load(parts[1])
    except yaml.YAMLError:
        # Frontmatter shape errors are owned by validate-skill-frontmatter.sh.
        continue
    if not isinstance(fm, dict):
        continue
    rels = fm.get("context_rel")
    if not isinstance(rels, list):
        continue
    for entry in rels:
        if not isinstance(entry, dict):
            continue
        target = entry.get("with")
        if not isinstance(target, str) or target == "":
            # Missing/empty `with` is a schema-shape concern, not resolution.
            continue
        if target not in slugs:
            findings.append({
                "file": rel,
                "field": "context_rel.with",
                "value": target,
                "message": (
                    "context_rel.with '%s' does not resolve to a skill under %s"
                    % (target, skills_root)
                ),
            })

print(json.dumps({"checked_skills": len(skill_files), "findings": findings}))
PY
)"
rc=$?
if [[ $rc -ne 0 ]]; then
    echo "ERROR: failed to parse skill frontmatter (python exited $rc)" >&2
    exit 1
fi

CHECKED="$(jq -r '.checked_skills' <<<"$REPORT")"
NUM_FINDINGS="$(jq -r '.findings | length' <<<"$REPORT")"

if [[ "$JSON" -eq 1 ]]; then
    valid="true"
    [[ "$NUM_FINDINGS" -gt 0 ]] && valid="false"
    jq --argjson valid "$valid" --arg root "$SKILLS_ROOT" \
        '{valid: $valid, skills_root: $root, checked_skills: .checked_skills, findings: .findings}' \
        <<<"$REPORT"
else
    if [[ "$NUM_FINDINGS" -gt 0 ]]; then
        while IFS=$'\t' read -r file value; do
            echo "FINDING ${file}: context_rel.with -> '${value}' does not resolve to a skill under ${SKILLS_ROOT}"
        done < <(jq -r '.findings[] | [.file, .value] | @tsv' <<<"$REPORT")
    fi
    echo "audit-skill-metadata: ${CHECKED} skill(s) checked, ${NUM_FINDINGS} unresolved context_rel.with edge(s)"
    if [[ "$NUM_FINDINGS" -gt 0 ]]; then
        echo "fix: point each context_rel.with at an existing skill slug, or remove the edge" >&2
    fi
fi

if [[ "$NUM_FINDINGS" -gt 0 && "$STRICT" -eq 1 ]]; then
    exit 1
fi
exit 0
