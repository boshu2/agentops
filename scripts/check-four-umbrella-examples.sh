#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
fixtures="$repo_root/tests/fixtures/four-umbrella-examples"

python3 - "$repo_root" "$fixtures" <<'PY'
from __future__ import annotations

import json
import pathlib
import re
import sys

try:
    import jsonschema
except ImportError as exc:
    raise SystemExit("check-four-umbrella-examples requires python3 jsonschema") from exc

repo = pathlib.Path(sys.argv[1])
fixtures = pathlib.Path(sys.argv[2])
paths = {
    "requests": fixtures / "explicit-skill-requests.txt",
    "execution": fixtures / "execution-packet.json",
    "learning": fixtures / "learning-packet.json",
    "impact": fixtures / "plan-impact.json",
    "legacy": fixtures / "negative/legacy-skill-requests.txt",
    "missing_learn": fixtures / "negative/execution-packet-missing-learn.json",
}
for path in paths.values():
    if not path.is_file():
        raise SystemExit(f"missing four-umbrella example fixture: {path.relative_to(repo)}")

requests = [line.strip() for line in paths["requests"].read_text().splitlines() if line.strip()]
expected_order = ["/discovery", "/premortem", "/crank", "/validate", "/learn", "/postmortem"]
actual_order = [line.split()[0] for line in requests]
if actual_order != expected_order:
    raise SystemExit(f"explicit skill request inventory drifted: {actual_order!r}")

legacy_pattern = re.compile(r"/(?:pre|post)[-_]mortem\b", re.IGNORECASE)
if legacy_pattern.search(paths["requests"].read_text()):
    raise SystemExit("current explicit skill requests contain a legacy mortem slug")
if not legacy_pattern.search(paths["legacy"].read_text()):
    raise SystemExit("legacy-request negative control is not negative")

execution = json.loads(paths["execution"].read_text())
execution_schema = json.loads((repo / "schemas/execution-packet.schema.json").read_text())
jsonschema.Draft202012Validator(execution_schema).validate(execution)
if execution.get("schema_version") != 3 or "pre_mortem_verdict" in execution:
    raise SystemExit("current execution example must use canonical schema-v3 mortem fields")
phases = [receipt.get("phase") for receipt in execution.get("phase_receipts", [])]
if phases != ["discovery", "crank", "validate", "learn"]:
    raise SystemExit(f"execution example must carry four ordered receipts; got {phases!r}")

missing_learn = json.loads(paths["missing_learn"].read_text())
missing_phases = [receipt.get("phase") for receipt in missing_learn.get("phase_receipts", [])]
if "learn" in missing_phases:
    raise SystemExit("missing-Learn negative control unexpectedly contains Learn")

learning = json.loads(paths["learning"].read_text())
learning_schema = json.loads((repo / "skills/learn/schemas/learn-receipt.schema.json").read_text())
jsonschema.Draft202012Validator(learning_schema).validate(learning)
if learning["plan_impact"]["disposition"] != "no_change":
    raise SystemExit("learning example must demonstrate the no-change continuation")

impact = json.loads(paths["impact"].read_text())
required_impact = {
    "schema_version", "source", "disposition", "summary", "evidence_refs",
    "proposed_changes", "orchestrator_decision", "premortem_required",
}
if set(impact) != required_impact:
    raise SystemExit(f"plan-impact example keys drifted: {sorted(impact)}")
if impact["schema_version"] != "plan-impact.v1" or impact["disposition"] != "material_change":
    raise SystemExit("plan-impact example must demonstrate a material-change handoff")
if not impact["evidence_refs"] or not impact["proposed_changes"]:
    raise SystemExit("material plan impact must cite evidence and propose a change")
if impact["orchestrator_decision"] != "replan" or impact["premortem_required"] is not True:
    raise SystemExit("material plan impact must return through orchestrator re-plan and Premortem")

surface_paths = [
    repo / "README.md",
    repo / "docs/newcomer-guide.md",
    repo / "docs/first-value-path.md",
    repo / "docs/contracts/four-umbrella-examples.md",
]
surface_text = "\n".join(path.read_text(encoding="utf-8") for path in surface_paths)
if legacy_pattern.search(surface_text):
    raise SystemExit("current README/newcomer/lifecycle examples contain a legacy explicit mortem request")
for token in ("/premortem", "/validate", "/learn", "/postmortem"):
    if token not in surface_text:
        raise SystemExit(f"current lifecycle examples do not demonstrate {token}")

readme_and_newcomer = (repo / "README.md").read_text() + (repo / "docs/newcomer-guide.md").read_text()
if re.search(r"/pawl(?:-review)?\b", readme_and_newcomer, re.IGNORECASE):
    raise SystemExit("Pawl is still presented as a current completion step in README/newcomer examples")
if "Validation completion and Git delivery are separate" not in readme_and_newcomer:
    raise SystemExit("newcomer surfaces must state that validation and Git delivery are separate")

inventory = (repo / "docs/contracts/four-umbrella-examples.md").read_text()
for name in ("explicit-skill-requests.txt", "execution-packet.json", "learning-packet.json", "plan-impact.json"):
    if name not in inventory:
        raise SystemExit(f"four-umbrella example inventory does not link {name}")

canonical_surfaces = {
    "README.md": "Discovery shapes behavior",
    "docs/3.0.md": "Discovery → Crank → Validate → Learn",
    "docs/architecture/operating-loop.md": "Validation completion and Git delivery are separate transitions.",
    "docs/architecture/intent-to-validated-code.md": "Discovery → Crank → Validate → Learn",
    "docs/newcomer-guide.md": "Validation completion and Git delivery are separate.",
}
for relative, required in canonical_surfaces.items():
    body = (repo / relative).read_text(encoding="utf-8")
    if required not in body:
        raise SystemExit(f"{relative} does not carry the canonical four-umbrella/delivery boundary: {required}")

stale_lifecycle = re.compile(
    r"research\s*(?:→|->)\s*plan\s*(?:→|->)\s*implement\s*(?:→|->)\s*validate"
    r"|Discovery\s*(?:→|->)\s*Implementation\s*(?:→|->)\s*Validation"
    r"|full\s+3-phase\s+lifecycle"
    r"|\bao land\b"
    r"|pawl-gate writes the terminal verdict",
    re.IGNORECASE,
)
primary_text = "\n".join(
    (repo / relative).read_text(encoding="utf-8")
    for relative in canonical_surfaces
)
match = stale_lifecycle.search(primary_text)
if match:
    raise SystemExit(f"primary lifecycle docs still teach a retired completion route: {match.group(0)!r}")
PY

echo "four-umbrella examples: PASS (requests, packets, newcomer path, delivery boundary)"
