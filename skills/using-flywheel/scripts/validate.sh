#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$SKILL_DIR/../.." && pwd)"
bash "$REPO_ROOT/skills/skill-builder/scripts/heal.sh" --check --strict "$SKILL_DIR"
grep -q '^## Constraints$' "$SKILL_DIR/SKILL.md"
grep -Fq 'pipe network bytes to a shell' "$SKILL_DIR/SKILL.md"
grep -Fq 'Declare the maximum workers (8)' "$SKILL_DIR/SKILL.md"
grep -Fq 'observed effects:' "$SKILL_DIR/SKILL.md"
grep -Fq 'confirmed supervisor/process cleanup' "$SKILL_DIR/SKILL.md"
test -x "$SKILL_DIR/scripts/validate-output.sh"

fixture_dir="$(mktemp -d)"
trap 'rm -rf -- "$fixture_dir"' EXIT
python3 - "$fixture_dir" <<'PY'
import copy
import json
import sys
from pathlib import Path

root = Path(sys.argv[1])
base = {
    "schema_version": "flywheel-run-receipt.v1",
    "mode": "observation",
    "authorization_id": "caller:flywheel-fixture",
    "target": "factory.example.test",
    "target_allowlist": ["factory.example.test"],
    "limits": {"command_timeout_seconds": 60, "overall_timeout_seconds": 300, "max_output_bytes": 4096, "max_download_bytes": 0, "max_workers": 0, "max_rounds": 0},
    "provisioning": {"requested": False, "upstream_version": None, "upstream_sha256": None, "download_domains": []},
    "dispatch": {"requested": False, "coordinator": None, "source_intent_id": None},
    "effects": {"network_domains": [], "bytes_downloaded": 0, "credential_identity": None, "workers_dispatched": 0, "rounds_completed": 0, "output_bytes": 128, "writes": [], "supervisor_cleanup_verified": True},
    "status": "complete",
}
def emit(name, value):
    (root / name).write_text(json.dumps(value), encoding="utf-8")
emit("observation.json", base)
provisioning = copy.deepcopy(base)
provisioning.update({"mode": "provisioning"})
provisioning["limits"].update({"max_download_bytes": 268435456})
provisioning["provisioning"] = {"requested": True, "upstream_version": "v1.2.3", "upstream_sha256": "a" * 64, "download_domains": ["downloads.example.test"]}
provisioning["effects"].update({"network_domains": ["downloads.example.test"], "bytes_downloaded": 1024, "credential_identity": "ssh:fixture", "writes": ["host:/opt/flywheel"]})
emit("provisioning.json", provisioning)
dispatch = copy.deepcopy(base)
dispatch.update({"mode": "dispatch", "status": "incomplete"})
dispatch["limits"].update({"max_workers": 4, "max_rounds": 2})
dispatch["dispatch"] = {"requested": True, "coordinator": "flywheel-coordinator", "source_intent_id": "intent-1"}
dispatch["effects"].update({"credential_identity": "ssh:fixture", "workers_dispatched": 4, "rounds_completed": 2, "output_bytes": 1024, "writes": ["factory:worktree"]})
emit("dispatch.json", dispatch)
stopped = copy.deepcopy(dispatch)
stopped.update({"status": "stopped-before-effect"})
stopped["effects"].update({"credential_identity": None, "workers_dispatched": 0, "rounds_completed": 0, "output_bytes": 0, "writes": [], "supervisor_cleanup_verified": False})
emit("stopped.json", stopped)
missing_auth = copy.deepcopy(dispatch)
missing_auth["authorization_id"] = ""
emit("missing-auth.json", missing_auth)
forbidden = copy.deepcopy(dispatch)
forbidden["target"] = "production.example.test"
emit("forbidden-target.json", forbidden)
oversubscribed = copy.deepcopy(dispatch)
oversubscribed["effects"]["workers_dispatched"] = 9
emit("oversubscribed.json", oversubscribed)
unclean = copy.deepcopy(dispatch)
unclean["effects"]["supervisor_cleanup_verified"] = False
emit("unclean.json", unclean)
PY
for accepted in observation provisioning dispatch stopped; do
  bash "$SKILL_DIR/scripts/validate-output.sh" "$fixture_dir/$accepted.json"
done
for rejected in missing-auth forbidden-target oversubscribed unclean; do
  if bash "$SKILL_DIR/scripts/validate-output.sh" "$fixture_dir/$rejected.json"; then
    echo "using-flywheel contract accepted negative fixture: $rejected" >&2
    exit 1
  fi
done
echo 'using-flywheel skill contract: PASS'
