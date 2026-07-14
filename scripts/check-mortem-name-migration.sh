#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
fixtures="$repo_root/tests/fixtures/mortem-name-migration"

python3 - "$repo_root" "$fixtures" <<'PY'
from __future__ import annotations

import pathlib
import re
import sys

repo = pathlib.Path(sys.argv[1])
fixtures = pathlib.Path(sys.argv[2])
stale = re.compile(r"pre[-_ ]mortem|post[-_ ]mortem", re.IGNORECASE)

positive = fixtures / "positive/canonical.md"
negatives = [
    fixtures / "negative/stale-explicit-request.md",
]
for path in [positive, *negatives]:
    if not path.is_file():
        raise SystemExit(f"missing mortem naming fixture: {path.relative_to(repo)}")
if stale.search(positive.read_text(encoding="utf-8")):
    raise SystemExit("positive mortem naming fixture contains a legacy token")
for path in negatives:
    if not stale.search(path.read_text(encoding="utf-8")):
        raise SystemExit(f"negative mortem naming fixture is not negative: {path.relative_to(repo)}")

entrypoints = [
    "AGENTS.md", "docs/agent-workflow-reference.md", "docs/CI-CD.md",
    "docs/contracts/codex-skill-api.md", "docs/contracts/repo-execution-profile.md",
    "README.md", "PRODUCT.md", "GOALS.md",
    "docs/3.0.md", "docs/ARCHITECTURE.md", "docs/SKILL-ROUTER.md",
    "docs/cdlc.md", "docs/newcomer-guide.md", "docs/documentation-index.md",
    "docs/first-value-path.md", "docs/how-it-works.md",
]
active_contracts = [
    "docs/contracts/bc-ports-inventory.md",
    "docs/contracts/bc1-corpus-ports.md",
    "docs/contracts/context-assembly-interface.md",
    "docs/contracts/dispatch-checklist.md",
    "docs/contracts/finding-registry.md",
    "docs/contracts/repo-execution-profile.md",
    "docs/contracts/session-intelligence-trust-model.md",
    "docs/contracts/skill-ports-and-adapters.md",
    "docs/contracts/swarm-evidence.md",
]
active = [repo / path for path in entrypoints + active_contracts]
active.extend(sorted((repo / "docs/architecture").glob("*.md")))

pointer_names = {"pre-mortem", "post-mortem", "pre_mortem", "post_mortem"}
for skill_dir in sorted((repo / "skills").iterdir()):
    if not skill_dir.is_dir() or skill_dir.name in pointer_names:
        continue
    active.extend(sorted(skill_dir.rglob("*.md")))

def allowed_compatibility_line(path: pathlib.Path, line: str) -> bool:
    normalized = line.lower()
    if re.search(r"cli-command-failures-20\d\d-\d\d-\d\d\.md$", path.name):
        return normalized.lstrip().startswith("- output:")
    if "pre-mortem-checks" in normalized:
        return "legacy" in normalized and ("readback" in normalized or "read-only fallback" in normalized)
    if "post-mortem-finding" in normalized:
        return (
            "legacy" in normalized
            or "compatibility" in normalized
            or ("postmortem-finding" in normalized and "|" in line)
        )
    if ".agents/council/" in normalized or "/scripts/.agents/council/" in normalized:
        return bool(re.search(r"/20\d\d-\d\d-\d\d-", line))
    return False

violations: list[str] = []
for path in active:
    if not path.is_file():
        violations.append(f"missing active naming surface: {path.relative_to(repo)}")
        continue
    for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if stale.search(line) and not allowed_compatibility_line(path, line):
            violations.append(f"{path.relative_to(repo)}:{number}:{line.strip()}")

for name, target in (
    ("pre-mortem", "premortem"),
    ("post-mortem", "postmortem"),
    ("pre_mortem", "premortem"),
    ("post_mortem", "postmortem"),
):
    pointer = repo / "skills" / name / "SKILL.md"
    if not pointer.is_file():
        violations.append(f"missing compatibility pointer: {pointer.relative_to(repo)}")
        continue
    body = pointer.read_text(encoding="utf-8")
    if f"redirect_to: {target}" not in body or "implementation: false" not in body:
        violations.append(f"non-pointer legacy skill tree: {pointer.relative_to(repo)}")

if violations:
    raise SystemExit("mortem naming migration violations:\n" + "\n".join(violations))
PY

echo "mortem name migration: PASS (canonical packet fields; explicit skill and directory redirects only)"
