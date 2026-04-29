#!/usr/bin/env bash
set -euo pipefail

json=false
allow_dirty=false
root=""

usage() {
  cat >&2 <<'USAGE'
Usage: scripts/check-closeout-gate.sh [--json] [--allow-dirty] [--root <repo>]

Reports RPI execution-packet proof refs and worktree clean/dirty state.
Fails when proof refs are absent, or when the worktree is dirty unless
--allow-dirty is provided.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --json)
      json=true
      shift
      ;;
    --allow-dirty)
      allow_dirty=true
      shift
      ;;
    --root)
      if [[ $# -lt 2 ]]; then
        usage
        exit 2
      fi
      root="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ -z "$root" ]]; then
  root="$(git rev-parse --show-toplevel 2>/dev/null || pwd -P)"
fi
root="$(cd "$root" && pwd -P)"

dirty_status=""
git_error=""
if git -C "$root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  dirty_status="$(git -C "$root" status --porcelain --untracked-files=all 2>/dev/null || true)"
else
  git_error="not a git worktree"
fi

export CLOSEOUT_ROOT="$root"
export CLOSEOUT_DIRTY_STATUS="$dirty_status"
export CLOSEOUT_GIT_ERROR="$git_error"
export CLOSEOUT_JSON="$json"
export CLOSEOUT_ALLOW_DIRTY="$allow_dirty"

python3 - <<'PY'
import glob
import json
import os
import sys
from pathlib import Path

root = Path(os.environ["CLOSEOUT_ROOT"])
dirty_status = os.environ.get("CLOSEOUT_DIRTY_STATUS", "")
git_error = os.environ.get("CLOSEOUT_GIT_ERROR", "")
json_mode = os.environ.get("CLOSEOUT_JSON") == "true"
allow_dirty = os.environ.get("CLOSEOUT_ALLOW_DIRTY") == "true"

packet_paths = [root / ".agents" / "rpi" / "execution-packet.json"]
packet_paths.extend(Path(p) for p in glob.glob(str(root / ".agents" / "rpi" / "runs" / "*" / "execution-packet.json")))

proof_refs = []
parse_errors = []
for packet_path in sorted(set(packet_paths)):
    if not packet_path.exists():
        continue
    try:
        data = json.loads(packet_path.read_text())
    except Exception as exc:
        parse_errors.append({"packet": str(packet_path.relative_to(root)), "error": str(exc)})
        continue
    artifacts = data.get("proof_artifacts") or []
    if not isinstance(artifacts, list) or not artifacts:
        continue
    run_id = data.get("run_id") or packet_path.parent.name
    proof_refs.append({
        "packet": str(packet_path.relative_to(root)),
        "run_id": run_id,
        "proof_updated_at": data.get("proof_updated_at", ""),
        "artifacts": [str(item) for item in artifacts],
    })

dirty_paths = [line for line in dirty_status.splitlines() if line.strip()]
errors = []
if not proof_refs:
    errors.append("no RPI execution-packet proof_refs found")
if parse_errors:
    errors.append("execution-packet parse errors")
if git_error:
    errors.append(f"worktree disposition unavailable: {git_error}")
elif dirty_paths and not allow_dirty:
    errors.append("worktree dirty")

report = {
    "schema_version": 1,
    "root": str(root),
    "result": "FAIL" if errors else "PASS",
    "closure_replay": {
        "proof_ref_count": len(proof_refs),
        "proof_refs": proof_refs,
        "parse_errors": parse_errors,
    },
    "worktree": {
        "clean": not dirty_paths and not git_error,
        "allow_dirty": allow_dirty,
        "dirty_count": len(dirty_paths),
        "dirty_paths": dirty_paths,
        "error": git_error,
    },
    "errors": errors,
}

if json_mode:
    print(json.dumps(report, indent=2, sort_keys=True))
else:
    print("closeout gate")
    print(f"result: {report['result']}")
    print(f"proof_refs: {len(proof_refs)}")
    for ref in proof_refs:
        print(f"- {ref['packet']} run={ref['run_id']} artifacts={len(ref['artifacts'])}")
    if parse_errors:
        print(f"parse_errors: {len(parse_errors)}")
    if git_error:
        print(f"worktree: unavailable ({git_error})")
    elif dirty_paths:
        print(f"worktree: dirty ({len(dirty_paths)} paths)")
        for path in dirty_paths[:20]:
            print(f"- {path}")
    else:
        print("worktree: clean")
    if errors:
        print("errors:")
        for error in errors:
            print(f"- {error}")

sys.exit(1 if errors else 0)
PY
