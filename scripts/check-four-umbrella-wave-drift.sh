#!/usr/bin/env bash
set -euo pipefail

phase=""
case "${1:-}" in
  --phase=pre) phase=pre ;;
  --phase=verify) phase=verify ;;
  *) echo "usage: $0 --phase=pre|--phase=verify S<n>" >&2; exit 2 ;;
esac
slice="${2:-}"
[[ "$slice" =~ ^S[1-8]$ ]] || { echo "invalid slice: $slice" >&2; exit 2; }

repo_root="$(git rev-parse --show-toplevel)"
manifest="$repo_root/docs/contracts/four-umbrella-write-manifests.json"
fixtures="$repo_root/tests/fixtures/four-umbrella-wave-drift"
[[ -f "$manifest" ]] || { echo "missing wave manifest: $manifest" >&2; exit 1; }
jq -e '.s1_frozen_base_sha | type == "string" and length == 40' "$manifest" >/dev/null
jq -e --arg slice "$slice" '.slices[$slice].paths | type == "array" and length > 0' "$manifest" >/dev/null

git -C "$repo_root" fetch --quiet origin main || { echo "origin/main fetch failed" >&2; exit 1; }
manifest_sha256="$(python3 - "$manifest" <<'PY'
import hashlib
import sys
from pathlib import Path
print(hashlib.sha256(Path(sys.argv[1]).read_bytes()).hexdigest())
PY
)"
receipt_dir="$repo_root/.agents/evidence/four-umbrella"
receipt="$receipt_dir/${slice,,}-base.json"

receipt_mode=existing
if [[ "$phase" == "pre" ]]; then
  [[ -z "$(git -C "$repo_root" status --porcelain=v1)" ]] || {
    echo "pre-work drift guard requires a clean tree" >&2
    exit 1
  }
  head_sha="$(git -C "$repo_root" rev-parse HEAD)"
  origin_sha="$(git -C "$repo_root" rev-parse origin/main)"
  [[ "$head_sha" == "$origin_sha" ]] || {
    echo "pre-work HEAD must equal origin/main (HEAD=$head_sha origin/main=$origin_sha)" >&2
    exit 1
  }
  base_sha="$origin_sha"
  receipt_mode=pre
elif [[ -f "$receipt" ]]; then
  jq -e --arg slice "$slice" --arg digest "$manifest_sha256" '
    .schema_version == 1 and .slice == $slice
    and (.base_sha | type == "string" and length == 40)
    and .manifest_sha256 == $digest
  ' "$receipt" >/dev/null || {
    echo "stale or malformed slice base receipt: $receipt" >&2
    exit 1
  }
  base_sha="$(jq -r '.base_sha' "$receipt")"
else
  if [[ "$slice" != "S1" ]]; then
    echo "missing slice base receipt: $receipt (S2-S8 require --phase=pre)" >&2
    exit 1
  fi
  # S1 predates the guard. It alone may bootstrap from the frozen epic base.
  base_sha="$(jq -r '.s1_frozen_base_sha' "$manifest")"
  receipt_mode=bootstrap
fi

git -C "$repo_root" cat-file -e "$base_sha^{commit}" || { echo "slice base SHA is unavailable: $base_sha" >&2; exit 1; }

REPO_ROOT="$repo_root" MANIFEST="$manifest" FIXTURES="$fixtures" PHASE="$phase" SLICE="$slice" BASE_SHA="$base_sha" python3 - <<'PY'
import fnmatch
import json
import os
import subprocess
from pathlib import Path

repo = Path(os.environ["REPO_ROOT"])
manifest_path = Path(os.environ["MANIFEST"])
fixtures = Path(os.environ["FIXTURES"])
phase = os.environ["PHASE"]
slice_id = os.environ["SLICE"]
base_sha = os.environ["BASE_SHA"]
manifest = json.loads(manifest_path.read_text())
patterns = manifest["slices"][slice_id]["paths"]

def matches(path, pattern):
    if pattern.endswith("/**"):
        return path.startswith(pattern[:-3].rstrip("/") + "/")
    return fnmatch.fnmatchcase(path, pattern)

fixture_names = [
    "clean-s1.json", "dirty-pre-work.json", "missing-manifest.json",
    "upstream-overlap.json", "out-of-manifest.json",
    "missing-base-receipt.json", "stale-base-receipt.json",
    "post-prior-wave-base.json",
]
for name in fixture_names:
    path = fixtures / name
    if not path.is_file():
        raise SystemExit(f"missing drift fixture: {path.relative_to(repo)}")
    case = json.loads(path.read_text())
    allowed = bool(case.get("manifest_present", True))
    if case.get("phase") == "pre" and case.get("local_changes"):
        allowed = False
    if case.get("upstream_changes") and any(
        matches(p, pat) for p in case["upstream_changes"] for pat in case.get("manifest_paths", patterns)
    ):
        allowed = False
    if case.get("phase") == "verify" and any(
        not any(matches(p, pat) for pat in case.get("manifest_paths", patterns))
        for p in case.get("local_changes", [])
    ):
        allowed = False
    if case.get("receipt_required") and not case.get("receipt_present", False):
        allowed = False
    if case.get("receipt_present") and case.get("receipt_manifest_sha256") != case.get("current_manifest_sha256"):
        allowed = False
    if case.get("expected_base") and case.get("receipt_base") != case.get("expected_base"):
        allowed = False
    if allowed != case["expected_pass"]:
        raise SystemExit(f"drift fixture {name} does not exercise its expected verdict")

status = subprocess.run(
    ["git", "status", "--porcelain=v1"], cwd=repo, text=True, capture_output=True, check=True
).stdout.splitlines()
changes = []
for line in status:
    path = line[3:]
    if " -> " in path:
        before, after = path.split(" -> ", 1)
        changes.extend([before, after])
    else:
        changes.append(path)
changes = sorted(set(changes))
if phase == "pre" and changes:
    raise SystemExit("pre-work drift guard requires a clean tree: " + ", ".join(changes))
if phase == "verify":
    committed = subprocess.run(
        ["git", "diff", "--name-only", f"{base_sha}..HEAD"],
        cwd=repo, text=True, capture_output=True, check=True,
    ).stdout.splitlines()
    committed_outside = [p for p in committed if not any(matches(p, pat) for pat in patterns)]
    if committed_outside:
        raise SystemExit("out-of-manifest committed changes: " + ", ".join(committed_outside))
    outside = [p for p in changes if not any(matches(p, pat) for pat in patterns)]
    if outside:
        raise SystemExit("out-of-manifest local changes: " + ", ".join(outside))

upstream = subprocess.run(
    ["git", "diff", "--name-only", f"{base_sha}..origin/main"],
    cwd=repo, text=True, capture_output=True, check=True,
).stdout.splitlines()
overlap = sorted(p for p in upstream if any(matches(p, pat) for pat in patterns))
(repo / ".agents" / "evidence" / "four-umbrella").mkdir(parents=True, exist_ok=True)
payload = [
    {"id": slice_id, "subject": "selected slice manifest", "files": overlap or [f".guard/{slice_id}/selected"]},
    {"id": "origin-main", "subject": "upstream since frozen base", "files": overlap or [".guard/origin/non-overlap"]},
]
path = repo / ".agents" / "evidence" / "four-umbrella" / f"{slice_id.lower()}-overlap-input.json"
path.write_text(json.dumps(payload, indent=2) + "\n")
print(path)
PY

overlap_input="$repo_root/.agents/evidence/four-umbrella/${slice,,}-overlap-input.json"
set +e
overlap_output="$(bash "$repo_root/scripts/check-file-manifest-overlap.sh" "$overlap_input" 2>&1)"
overlap_status=$?
set -e
printf '%s\n' "$overlap_output"
if grep -Eq '^(WARN|SKIP):' <<<"$overlap_output"; then
  echo "overlap checker skipped or degraded" >&2
  exit 1
fi
if [[ $overlap_status -ne 0 ]]; then
  echo "upstream overlaps $slice ownership since $base_sha" >&2
  exit 1
fi
if [[ "$receipt_mode" != "existing" ]]; then
  mkdir -p "$receipt_dir"
  RECEIPT="$receipt" SLICE="$slice" BASE_SHA="$base_sha" MANIFEST_SHA256="$manifest_sha256" RECEIPT_MODE="$receipt_mode" python3 - <<'PY'
import json
import os
from datetime import datetime, timezone
from pathlib import Path

path = Path(os.environ["RECEIPT"])
payload = {
    "schema_version": 1,
    "slice": os.environ["SLICE"],
    "base_sha": os.environ["BASE_SHA"],
    "manifest_sha256": os.environ["MANIFEST_SHA256"],
    "captured_at": datetime.now(timezone.utc).isoformat(),
}
if os.environ["RECEIPT_MODE"] == "bootstrap":
    payload["bootstrap"] = "frozen-s1-base"
tmp = path.with_suffix(".tmp")
tmp.write_text(json.dumps(payload, indent=2) + "\n")
tmp.replace(path)
PY
fi
echo "four-umbrella wave drift: PASS ($phase $slice, base=$base_sha)"
