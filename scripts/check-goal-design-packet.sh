#!/usr/bin/env bash
# Validate a goal-design packet directory containing intent.md and driver.md.
#
# Exit codes:
#   0 - packet is valid
#   1 - packet is invalid
#   2 - usage, dependency, or repository setup error
set -eEuo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/check-goal-design-packet.sh <packet-dir>

Validates .agents/goal-design/<slug>/intent.md and driver.md frontmatter against
schemas/goal-design-*.v1.schema.json, then checks driver intent_ref.sha256
against the current intent.md bytes plus cross-file identity.
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ "$#" -ne 1 ]]; then
  usage >&2
  exit 2
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "check-goal-design-packet: python3 is required" >&2
  exit 2
fi

if ! python3 -c "import yaml, jsonschema" >/dev/null 2>&1; then
  echo "check-goal-design-packet: python deps missing (need PyYAML and jsonschema)" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if REPO_ROOT="$(git -C "$SCRIPT_DIR/.." rev-parse --show-toplevel 2>/dev/null)"; then
  :
else
  REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
fi
PACKET_DIR="$1"

python3 - "$REPO_ROOT" "$PACKET_DIR" <<'PY'
import hashlib
import json
import re
import sys
from pathlib import Path

import yaml
from jsonschema import Draft202012Validator, FormatChecker

repo_root = Path(sys.argv[1]).resolve()
packet_dir = Path(sys.argv[2])
if not packet_dir.is_absolute():
    packet_dir = (Path.cwd() / packet_dir).resolve()

intent_path = packet_dir / "intent.md"
driver_path = packet_dir / "driver.md"
intent_schema_path = repo_root / "schemas" / "goal-design-intent.v1.schema.json"
driver_schema_path = repo_root / "schemas" / "goal-design-driver.v1.schema.json"


def fail(message: str, code: int = 1) -> None:
    print(f"FAIL: {message}", file=sys.stderr)
    raise SystemExit(code)


def require_file(path: Path) -> None:
    if not path.is_file():
        fail(f"required file not found: {path}", 2)


def load_frontmatter(path: Path):
    text = path.read_text(encoding="utf-8")
    if not text.startswith("---\n"):
        fail(f"{path.name} missing YAML frontmatter")
    end = text.find("\n---\n", 4)
    if end < 0:
        fail(f"{path.name} has unterminated YAML frontmatter")
    raw = text[4:end]
    try:
        data = yaml.safe_load(raw)
    except yaml.YAMLError as exc:
        fail(f"{path.name} YAML parse error: {exc}")
    if not isinstance(data, dict):
        fail(f"{path.name} frontmatter did not parse as a mapping")
    return data, text


def load_schema(path: Path):
    try:
        schema = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        fail(f"schema is invalid JSON: {path}: {exc}", 2)
    Draft202012Validator.check_schema(schema)
    return schema


def validate(label: str, data, schema) -> None:
    validator = Draft202012Validator(schema, format_checker=FormatChecker())
    errors = sorted(validator.iter_errors(data), key=lambda error: list(error.absolute_path))
    if errors:
        error = errors[0]
        loc = "/".join(str(part) for part in error.absolute_path) or "<root>"
        fail(f"{label} schema violation at {loc}: {error.message}")


def check_cross_file_identity(intent_data, driver_data) -> None:
    intent_slug = str(intent_data["slug"])
    driver_slug = str(driver_data["slug"])
    if intent_slug != driver_slug:
        fail(f"slug mismatch: intent.slug {intent_slug!r} does not match driver.slug {driver_slug!r}")

    expected_intent_ref = f".agents/goal-design/{intent_slug}/intent.md"
    actual_intent_ref = str(driver_data["intent_ref"]["path"])
    if actual_intent_ref != expected_intent_ref:
        fail(
            "driver intent_ref.path must identify packet intent: "
            f"expected {expected_intent_ref!r}, found {actual_intent_ref!r}"
        )


def normalize_label(value: str) -> str:
    return " ".join(value.casefold().split())


def check_candidate_scenario_mapping(intent_data, driver_data) -> None:
    scenarios = intent_data["bdd"]["scenarios"]
    scenario_ids = {str(scenario["id"]) for scenario in scenarios}
    scenario_names = {
        normalize_label(str(scenario["name"]))
        for scenario in scenarios
        if str(scenario["name"]).strip()
    }

    for index, candidate in enumerate(driver_data["candidate_beads"]):
        behavior = str(candidate["behavior"])
        scenario_refs = re.findall(r"\bS[0-9]+\b", behavior)
        unknown_refs = sorted({ref for ref in scenario_refs if ref not in scenario_ids})
        if unknown_refs:
            fail(
                f"candidate_beads/{index}/behavior references scenario ids not present "
                f"in intent.bdd.scenarios: {', '.join(unknown_refs)}"
            )
        if scenario_refs:
            continue

        normalized_behavior = normalize_label(behavior)
        if not any(name in normalized_behavior for name in scenario_names):
            fail(
                f"candidate_beads/{index}/behavior must reference an id or name "
                "from intent.bdd.scenarios"
            )


for required in (intent_path, driver_path, intent_schema_path, driver_schema_path):
    require_file(required)

intent_data, intent_text = load_frontmatter(intent_path)
driver_data, driver_text = load_frontmatter(driver_path)

validate("intent.md", intent_data, load_schema(intent_schema_path))
validate("driver.md", driver_data, load_schema(driver_schema_path))

expected_sha = hashlib.sha256(intent_path.read_bytes()).hexdigest()
actual_sha = driver_data["intent_ref"]["sha256"]
if expected_sha != actual_sha:
    fail(
        "driver intent_ref.sha256 is stale: "
        f"expected {expected_sha}, found {actual_sha}"
    )

check_cross_file_identity(intent_data, driver_data)
check_candidate_scenario_mapping(intent_data, driver_data)
print(f"goal-design packet valid: {packet_dir}")
PY
