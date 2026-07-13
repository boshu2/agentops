#!/usr/bin/env bash
set -euo pipefail

mode="${1:-source}"
if [[ "$mode" != "source" ]]; then
  echo "usage: scripts/validate-workflow-contract.sh source" >&2
  exit 2
fi

repo_root="$(git rev-parse --show-toplevel)"
fixtures="$repo_root/tests/fixtures/four-umbrella-workflow"

REPO_ROOT="$repo_root" FIXTURES="$fixtures" python3 - <<'PY'
import json
import os
from pathlib import Path

root = Path(os.environ["REPO_ROOT"])
fixtures = Path(os.environ["FIXTURES"])
failures: list[str] = []


def text(relative: str) -> str:
    path = root / relative
    if not path.is_file():
        failures.append(f"missing source contract: {relative}")
        return ""
    return path.read_text(encoding="utf-8")


def require(relative: str, *markers: str) -> None:
    body = text(relative)
    for marker in markers:
        if marker not in body:
            failures.append(f"{relative} missing contract marker: {marker}")


require(
    "skills/rpi/SKILL.md",
    "Validate -> Learn -> orchestrator",
    "Learn is the only post-verdict handoff",
    "Only the orchestrator may invoke Premortem",
)
require(
    "skills/rpi/references/agile-replan-loop.md",
    "Validate -> Learn -> orchestrator",
    "material_change",
    "no_change",
    "terminal",
)
require(
    "skills/learn/SKILL.md",
    "plan_impact",
    "returns it to the orchestrator",
    "does not invoke Premortem",
)
require(
    "skills/premortem/SKILL.md",
    "changed plan",
    "explicit orchestrator request",
)
require(
    "skills/discovery/SKILL.md",
    "explicit orchestrator re-plan request",
)
require(
    "skills/crank/SKILL.md",
    "wave evidence to Validate",
    "does not invoke Discovery, Learn, or Premortem",
)
require(
    "skills/evolve/SKILL.md",
    "Validate -> Learn -> orchestrator",
    "changed plan through Premortem",
)
require(
    "skills-codex/rpi/SKILL.md",
    "Validate -> Learn -> orchestrator",
    "Learn is the only post-verdict handoff",
    "Only the orchestrator may invoke Premortem",
)
require(
    "skills-codex/rpi/references/agile-replan-loop.md",
    "Validate -> Learn -> orchestrator",
    "material_change",
    "no_change",
    "terminal",
)
require(
    "skills-codex/learn/SKILL.md",
    "plan_impact",
    "returns it to the orchestrator",
    "does not invoke Premortem",
)
require(
    "skills-codex/premortem/SKILL.md",
    "changed plan",
    "explicit orchestrator request",
)
require(
    "skills-codex/discovery/SKILL.md",
    "explicit orchestrator re-plan request",
)
require(
    "skills-codex/crank/SKILL.md",
    "wave evidence to Validate",
    "does not invoke Discovery, Learn, or Premortem",
)
require(
    "skills-codex/evolve/SKILL.md",
    "Validate -> Learn -> orchestrator",
    "changed plan through Premortem",
)

learn_schema_path = root / "skills/learn/schemas/learn-receipt.schema.json"
try:
    learn_schema = json.loads(learn_schema_path.read_text(encoding="utf-8"))
except (OSError, json.JSONDecodeError) as exc:
    failures.append(f"invalid Learn receipt schema: {exc}")
else:
    required = set(learn_schema.get("required", []))
    if not {"remaining_work", "plan_impact"}.issubset(required):
        failures.append("Learn receipt must require remaining_work and plan_impact")
    disposition = (
        learn_schema.get("properties", {})
        .get("plan_impact", {})
        .get("properties", {})
        .get("disposition", {})
        .get("enum", [])
    )
    if set(disposition) != {"material_change", "no_change", "terminal"}:
        failures.append("Learn plan_impact must expose exactly material_change, no_change, terminal")
    codex_schema_path = root / "skills-codex/learn/schemas/learn-receipt.schema.json"
    if not codex_schema_path.is_file():
        failures.append("missing Codex Learn receipt schema")
    elif codex_schema_path.read_bytes() != learn_schema_path.read_bytes():
        failures.append("source and Codex Learn receipt schemas differ")


def validate_packet(packet: dict) -> list[str]:
    errors: list[str] = []
    if packet.get("validate_next") != "learn":
        errors.append("Validate must hand off to Learn")
    if packet.get("learn_next") != "orchestrator":
        errors.append("Learn must return to the orchestrator")

    remaining = packet.get("remaining_work")
    disposition = packet.get("learn_disposition")
    decision = packet.get("orchestrator_decision")
    changed = packet.get("plan_changed")
    premortem = packet.get("premortem_invoked")

    if remaining is True and disposition == "material_change":
        if decision != "replan" or changed is not True or premortem is not True:
            errors.append("material delta requires orchestrator replan, changed plan, then Premortem")
    elif remaining is True and disposition == "no_change":
        if decision not in {"retry", "continue", "stop", "escalate"}:
            errors.append("no_change requires an explicit orchestrator decision")
        if changed is not False or premortem is not False:
            errors.append("no_change must not fabricate a plan change or invoke Premortem")
    elif remaining is False and disposition == "terminal":
        if decision != "close" or changed is not False or premortem is not False:
            errors.append("terminal work must close without plan mutation or Premortem")
    else:
        errors.append("remaining_work and Learn disposition are inconsistent")
    return errors


positive = sorted(fixtures.glob("valid-*.json"))
negative = sorted(fixtures.glob("invalid-*.json"))
if len(positive) < 3:
    failures.append("workflow contract needs material, no-change, and terminal positive fixtures")
if len(negative) < 3:
    failures.append("workflow contract needs direct-Premortem and silent-retry negative fixtures")

for path in positive:
    try:
        packet = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        failures.append(f"{path.name} is invalid JSON: {exc}")
        continue
    errors = validate_packet(packet)
    if errors:
        failures.append(f"positive fixture {path.name} rejected: {'; '.join(errors)}")

for path in negative:
    try:
        packet = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        failures.append(f"{path.name} is invalid JSON: {exc}")
        continue
    if not validate_packet(packet):
        failures.append(f"negative fixture {path.name} was accepted")

if failures:
    for failure in failures:
        print(f"FAIL: {failure}")
    raise SystemExit(f"four-umbrella workflow contract: FAIL ({len(failures)})")

print("four-umbrella workflow contract: PASS")
PY
