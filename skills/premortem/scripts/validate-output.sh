#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 || ! -f "$1" ]]; then
  echo "usage: $0 <plan-verdict.json> [repository-root]" >&2
  exit 2
fi

VERDICT_PATH="$1"
REPO_ROOT="${2:-$(git -C "$(dirname "$VERDICT_PATH")" rev-parse --show-toplevel 2>/dev/null || pwd)}"
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SCHEMA_PATH="$SKILL_DIR/schemas/plan-verdict.schema.json"

python3 - "$VERDICT_PATH" "$REPO_ROOT" "$SCHEMA_PATH" <<'PY'
from __future__ import annotations

import hashlib
import json
import sys
from pathlib import Path

try:
    from jsonschema import Draft202012Validator
except ImportError as exc:
    print(f"premortem plan verdict: jsonschema is required: {exc}", file=sys.stderr)
    raise SystemExit(2)

verdict_path = Path(sys.argv[1]).resolve()
root = Path(sys.argv[2]).resolve()
schema_path = Path(sys.argv[3]).resolve()

try:
    payload = json.loads(verdict_path.read_text(encoding="utf-8"))
    schema = json.loads(schema_path.read_text(encoding="utf-8"))
except (OSError, json.JSONDecodeError) as exc:
    print(f"premortem plan verdict: unreadable JSON: {exc}", file=sys.stderr)
    raise SystemExit(1)

errors = sorted(
    Draft202012Validator(schema).iter_errors(payload),
    key=lambda item: [str(part) for part in item.absolute_path],
)
if errors:
    error = errors[0]
    location = "/".join(str(part) for part in error.absolute_path) or "<root>"
    print(f"premortem plan verdict: schema violation at {location}: {error.message}", file=sys.stderr)
    raise SystemExit(1)

if payload["author_id"] == payload["judge_id"]:
    print("premortem plan verdict: author_id must differ from judge_id", file=sys.stderr)
    raise SystemExit(1)

relative = Path(payload["plan"]["path"])
if relative.is_absolute():
    print("premortem plan verdict: plan path must be repository-relative", file=sys.stderr)
    raise SystemExit(1)
plan_path = (root / relative).resolve()
try:
    plan_path.relative_to(root)
except ValueError:
    print("premortem plan verdict: plan path escapes repository root", file=sys.stderr)
    raise SystemExit(1)
if not plan_path.is_file():
    print(f"premortem plan verdict: plan does not exist: {relative}", file=sys.stderr)
    raise SystemExit(1)

actual = hashlib.sha256(plan_path.read_bytes()).hexdigest()
if actual != payload["plan"]["sha256"]:
    print(
        f"premortem plan verdict: stale plan digest: expected {payload['plan']['sha256']}, got {actual}",
        file=sys.stderr,
    )
    raise SystemExit(1)

print(
    f"premortem plan verdict valid: {payload['verdict']} "
    f"{payload['plan']['path']}@{payload['plan']['sha256']}"
)
PY
