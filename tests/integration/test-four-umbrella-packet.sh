#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
validator="$repo_root/skills/rpi/scripts/validate-execution-packet.py"
fixture_dir="$(mktemp -d "$repo_root/.agents/rpi/s2-four-umbrella.XXXXXX")"
trap 'rm -rf "$fixture_dir"' EXIT

for phase in 1 2 3 4; do
  printf '# Phase %s\n\nEvidence.\n' "$phase" >"$fixture_dir/phase-$phase-summary.md"
done

relative_dir="${fixture_dir#"$repo_root/"}"
packet="$fixture_dir/packet.json"
jq -n --arg dir "$relative_dir" '
  {
    schema_version: 2,
    objective: "prove four ordered umbrella receipts",
    skills_loaded: [
      {name:"rpi", reason:"orchestrator"},
      {name:"discovery", reason:"shape intent"},
      {name:"crank", reason:"implement slices"},
      {name:"validate", reason:"prove acceptance"},
      {name:"learn", reason:"capture post-verdict observations"}
    ],
    phase_receipts: [
      {phase:"discovery", skill:"discovery", status:"DONE", artifact:($dir + "/phase-1-summary.md")},
      {phase:"crank", skill:"crank", status:"DONE", artifact:($dir + "/phase-2-summary.md")},
      {phase:"validate", skill:"validate", status:"PASS", artifact:($dir + "/phase-3-summary.md")},
      {phase:"learn", skill:"learn", status:"DONE", artifact:($dir + "/phase-4-summary.md")}
    ]
  }
' >"$packet"

python3 "$validator" "$packet" >/dev/null

assert_rejected() {
  local candidate="$1"
  local expected="$2"
  local output
  if output="$(python3 "$validator" "$candidate" 2>&1)"; then
    echo "FAIL: validator accepted $candidate" >&2
    exit 1
  fi
  grep -Fq "$expected" <<<"$output" || {
    echo "FAIL: rejection did not name '$expected': $output" >&2
    exit 1
  }
}

jq 'del(.phase_receipts[] | select(.phase == "learn"))' "$packet" >"$fixture_dir/missing-learn.json"
assert_rejected "$fixture_dir/missing-learn.json" "learn"

jq '.phase_receipts = [.phase_receipts[0], .phase_receipts[1], .phase_receipts[3], .phase_receipts[2]]' \
  "$packet" >"$fixture_dir/out-of-order.json"
assert_rejected "$fixture_dir/out-of-order.json" "phase_receipts[2]"

echo "four-umbrella packet contract: PASS"
