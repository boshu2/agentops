#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
skill="$skill_dir/SKILL.md"

grep -q '^name: bootstrap$' "$skill"
grep -Fq 'optionally_create_verdict_storage_directory' "$skill"
grep -Fq 'Create exactly' "$skill"
grep -Fq '.agents/ao/verdicts/sha256/' "$skill"
grep -Fq 'A docs-only bootstrap never creates `.agents/ao/**`' "$skill"
grep -Fq 'whether verdict storage was' "$skill"
grep -Fq 'verdict_storage_requested: false' "$skill_dir/references/examples.md"
test -x "$skill_dir/scripts/validate-output.sh"

fixture_dir="$(mktemp -d)"
trap 'rm -rf -- "$fixture_dir"' EXIT
docs_target="$fixture_dir/docs-target"
storage_target="$fixture_dir/storage-target"
mkdir -p "$docs_target" "$storage_target/.agents/ao/verdicts/sha256"
touch "$docs_target/AGENTS.md"
docs_target="$(cd "$docs_target" && pwd -P)"
storage_target="$(cd "$storage_target" && pwd -P)"
python3 - "$fixture_dir" "$docs_target" "$storage_target" <<'PY'
import copy
import json
import sys
from pathlib import Path

root, docs_target, storage_target = map(Path, sys.argv[1:])
docs = {
    "schema_version": "bootstrap-result.v1",
    "target": str(docs_target),
    "authorization_id": "caller:bootstrap-fixture",
    "verdict_storage_requested": False,
    "requested_documents": [str(docs_target / "AGENTS.md")],
    "created_documents": [str(docs_target / "AGENTS.md")],
    "existing_documents": [], "failed_documents": [],
    "requested_directories": [], "created_directories": [], "existing_directories": [], "failed_directories": [],
    "writes": [str(docs_target / "AGENTS.md")],
}
storage_dir = storage_target / ".agents" / "ao" / "verdicts" / "sha256"
storage = {
    "schema_version": "bootstrap-result.v1",
    "target": str(storage_target),
    "authorization_id": "caller:bootstrap-fixture",
    "verdict_storage_requested": True,
    "requested_documents": [], "created_documents": [], "existing_documents": [], "failed_documents": [],
    "requested_directories": [str(storage_dir)], "created_directories": [str(storage_dir)], "existing_directories": [], "failed_directories": [],
    "writes": [str(storage_dir)],
}
def emit(name, value):
    (root / name).write_text(json.dumps(value), encoding="utf-8")
emit("docs.json", docs)
emit("storage.json", storage)
undeclared_storage = copy.deepcopy(storage)
undeclared_storage["verdict_storage_requested"] = False
emit("undeclared-storage.json", undeclared_storage)
missing_class = copy.deepcopy(docs)
missing_class["created_documents"] = []
missing_class["writes"] = []
emit("missing-class.json", missing_class)
outside = copy.deepcopy(docs)
outside["requested_documents"] = [str(root / "outside.md")]
outside["created_documents"] = [str(root / "outside.md")]
outside["writes"] = [str(root / "outside.md")]
emit("outside.json", outside)
PY
for accepted in docs storage; do
  bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/$accepted.json"
done
for rejected in undeclared-storage missing-class outside; do
  if bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/$rejected.json"; then
    echo "bootstrap contract accepted negative fixture: $rejected" >&2
    exit 1
  fi
done

echo 'bootstrap skill contract: PASS'
