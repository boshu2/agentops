#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" || -L "$1" ]]; then
  echo "usage: $0 <bootstrap-result.json>" >&2
  exit 2
fi

python3 - "$1" <<'PY'
import json
from pathlib import Path
import sys

path = Path(sys.argv[1])
raw = path.read_bytes()
if len(raw) > 65536:
    raise SystemExit("bootstrap output: artifact exceeds 65536 bytes")
try:
    value = json.loads(raw)
except json.JSONDecodeError as exc:
    raise SystemExit(f"bootstrap output: unreadable JSON: {exc}")
required = {
    "schema_version", "target", "authorization_id", "verdict_storage_requested",
    "requested_documents", "created_documents", "existing_documents", "failed_documents",
    "requested_directories", "created_directories", "existing_directories", "failed_directories",
    "writes",
}
if not isinstance(value, dict) or set(value) != required:
    raise SystemExit("bootstrap output: unexpected or missing fields")
def text(item, maximum=1024):
    return isinstance(item, str) and 0 < len(item) <= maximum
if value["schema_version"] != "bootstrap-result.v1" or not text(value["authorization_id"], 256) or not text(value["target"]) or not isinstance(value["verdict_storage_requested"], bool):
    raise SystemExit("bootstrap output: invalid identity or authorization")
target = Path(value["target"]).resolve(strict=True)
if str(target) != value["target"] or not target.is_dir() or target == Path(target.anchor) or target == Path.home().resolve():
    raise SystemExit("bootstrap output: forbidden target")
array_fields = required - {"schema_version", "target", "authorization_id", "verdict_storage_requested"}
for field in array_fields:
    items = value[field]
    if not isinstance(items, list) or len(items) > 50 or len(set(items)) != len(items) or not all(text(item) and Path(item).is_absolute() for item in items):
        raise SystemExit(f"bootstrap output: malformed {field}")
    for item in items:
        candidate = Path(item).resolve(strict=False)
        try:
            candidate.relative_to(target)
        except ValueError:
            raise SystemExit(f"bootstrap output: {field} contains a path outside target")
doc_classes = [set(value[field]) for field in ("created_documents", "existing_documents", "failed_documents")]
dir_classes = [set(value[field]) for field in ("created_directories", "existing_directories", "failed_directories")]
if any(left & right for index, left in enumerate(doc_classes) for right in doc_classes[index + 1:]) or set().union(*doc_classes) != set(value["requested_documents"]):
    raise SystemExit("bootstrap output: document classifications do not match requests")
if any(left & right for index, left in enumerate(dir_classes) for right in dir_classes[index + 1:]) or set().union(*dir_classes) != set(value["requested_directories"]):
    raise SystemExit("bootstrap output: directory classifications do not match requests")
expected_writes = set(value["created_documents"] + value["created_directories"])
if set(value["writes"]) != expected_writes:
    raise SystemExit("bootstrap output: writes do not match created paths")
for field in ("created_documents", "existing_documents"):
    if not all(Path(item).is_file() and not Path(item).is_symlink() for item in value[field]):
        raise SystemExit(f"bootstrap output: {field} contains an absent or symlinked file")
for field in ("created_directories", "existing_directories"):
    if not all(Path(item).is_dir() and not Path(item).is_symlink() for item in value[field]):
        raise SystemExit(f"bootstrap output: {field} contains an absent or symlinked directory")
verdict_dir = str(target / ".agents" / "ao" / "verdicts" / "sha256")
if value["verdict_storage_requested"]:
    if value["requested_directories"] != [verdict_dir] or verdict_dir not in set().union(*dir_classes):
        raise SystemExit("bootstrap output: requested verdict storage is not classified")
else:
    forbidden_prefix = str(target / ".agents" / "ao")
    if any(item == forbidden_prefix or item.startswith(forbidden_prefix + "/") for item in value["requested_directories"] + value["writes"]):
        raise SystemExit("bootstrap output: docs-only run declared .agents/ao effects")
print(f"valid bootstrap-result.v1: {path}")
PY
