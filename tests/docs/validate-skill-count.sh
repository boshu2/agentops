#!/usr/bin/env bash
# Verify the metadata-generated inventory covers both runtime projections.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
python3 - "$REPO_ROOT" <<'PY'
from pathlib import Path
import json
import sys

root = Path(sys.argv[1])
source = {p.parent.name for p in (root / "skills").glob("*/SKILL.md")}
codex = {p.parent.name for p in (root / "skills-codex").glob("*/SKILL.md")}
catalog = json.loads((root / "skills/catalog.json").read_text(encoding="utf-8"))
rows = catalog.get("skills") or []
names = [row.get("name") for row in rows]
errors = []
if len(names) != len(set(names)):
    errors.append("catalog contains duplicate skill rows")
if set(names) != source:
    errors.append("catalog names do not equal source skill names")
if codex != source:
    errors.append("Codex skill names do not equal source skill names")
for row in rows:
    if not isinstance(row.get("disposition"), str) or not row["disposition"]:
        errors.append(f"{row.get('name')}: missing metadata-derived disposition")
aliases = {"pre-mortem", "pre_mortem", "post-mortem", "post_mortem"}
if aliases.intersection(source | codex):
    errors.append(f"noncanonical mortem aliases remain: {sorted(aliases.intersection(source | codex))}")
if catalog.get("skill_count") != len(source):
    errors.append("catalog skill_count is stale")
if errors:
    for error in errors:
        print(f"MISMATCH: {error}")
    raise SystemExit(1)
print(f"PASS: metadata inventory covers {len(source)} source and Codex skills exactly once")
PY
