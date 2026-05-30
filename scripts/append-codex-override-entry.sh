#!/usr/bin/env bash
# append-codex-override-entry.sh — ensure a new skill has a row in
# skills-codex-overrides/catalog.json (ag-cw2y item 4).
#
# Idempotent: appends a parity_only entry (canonical-derived codex form, which is
# what skill-builder produces by default) only if the skill is absent — so a
# newly-scaffolded skill is one-shot-green against validate-codex-override-coverage
# ("source skill missing from Codex catalog"). If the codex form is later made
# bespoke, the author flips treatment to "bespoke" and scaffolds the override dir.
#
# Usage: append-codex-override-entry.sh <skill-name> [repo-root]
set -euo pipefail

SKILL="${1:?usage: append-codex-override-entry.sh <skill-name> [repo-root]}"
REPO_ROOT="${2:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
CATALOG="$REPO_ROOT/skills-codex-overrides/catalog.json"

if [[ ! -f "$CATALOG" ]]; then
  echo "append-codex-override-entry: no catalog at $CATALOG" >&2
  exit 1
fi

SKILL="$SKILL" python3 - "$CATALOG" <<'PY'
import json, os, sys
path = sys.argv[1]
skill = os.environ["SKILL"]
with open(path) as f:
    cat = json.load(f)
skills = cat.setdefault("skills", [])
if any(s.get("name") == skill for s in skills):
    print(f"append-codex-override-entry: '{skill}' already cataloged — no-op")
    sys.exit(0)
skills.append({
    "name": skill,
    "treatment": "parity_only",
    "wave": "catalog-parity",
    "reason": f"TODO: confirm parity_only or flip to bespoke (+scaffold override dir) for {skill}",
})
with open(path, "w") as f:
    json.dump(cat, f, indent=2, ensure_ascii=False)
    f.write("\n")
print(f"append-codex-override-entry: added parity_only entry for '{skill}'")
PY
