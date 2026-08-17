#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" || -L "$1" ]]; then
  echo "usage: $0 <research-findings.json>" >&2
  exit 2
fi

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
python3 - "$1" "$skill_dir/schemas/findings.json" <<'PY'
import json
import re
import sys
from pathlib import Path

from jsonschema import Draft7Validator

artifact_path = Path(sys.argv[1])
raw = artifact_path.read_bytes()
if len(raw) > 524288:
    raise SystemExit("research output: artifact exceeds 524288 bytes")
try:
    artifact = json.loads(raw)
    schema = json.loads(Path(sys.argv[2]).read_bytes())
except json.JSONDecodeError as exc:
    raise SystemExit(f"research output: unreadable JSON: {exc}")
errors = sorted(Draft7Validator(schema).iter_errors(artifact), key=lambda item: list(item.path))
if errors:
    first = errors[0]
    location = ".".join(str(part) for part in first.path) or "<root>"
    # Do not echo the rejected value: it may be the credential or restricted
    # excerpt whose declaration is being rejected.
    raise SystemExit(f"research output: schema violation at {location} ({first.validator})")
text = raw.decode("utf-8")
secret_patterns = {
    "private-key": r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----",
    "aws-access-key": r"\b(?:AKIA|ASIA)[A-Z0-9]{16}\b",
    "github-token": r"\bgh[opusr]_[A-Za-z0-9]{20,}\b|\bgithub_pat_[A-Za-z0-9_]{20,}\b",
    "bearer-token": r"(?i)\bAuthorization\s*:\s*Bearer\s+[A-Za-z0-9._~+/=-]{12,}",
    "password-assignment": r"(?i)\b(?:password|passwd|secret|api[_-]?key)\s*[:=]\s*[^\s<\[][^\r\n]{5,}",
}
hits = [name for name, pattern in secret_patterns.items() if re.search(pattern, text)]
effects = artifact["effects"]
if hits and effects["sensitive_data"] != "approved":
    raise SystemExit("research output: sensitive categories require exact output approval: " + ",".join(hits))
if effects["sensitive_data"] == "redacted" and "[REDACTED:" not in text:
    raise SystemExit("research output: redacted disposition lacks a redaction marker")
print(f"valid research findings: {artifact_path}")
PY
