#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
script_path="$repo_root/scripts/check-closeout-gate.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

make_repo() {
  local dir="$1"
  git -C "$dir" init -q
  git -C "$dir" config user.email "closeout-gate@example.test"
  git -C "$dir" config user.name "Closeout Gate Test"
  mkdir -p "$dir/.agents/rpi/runs/run-closeout"
  cat > "$dir/.agents/rpi/execution-packet.json" <<'JSON'
{
  "run_id": "latest-closeout",
  "epic_id": "ag-closeout",
  "bead_id": "ag-closeout",
  "tracking_repo_root": "__ROOT__",
  "beads_dir": "__ROOT__/.beads",
  "pr_url": "https://github.com/example/agentops/pull/204",
  "merge_commit": "abc123",
  "proof_updated_at": "2026-04-29T00:00:00Z",
  "proof_artifacts": [
    ".agents/daemon/ledger.jsonl",
    "docs/daemon-migration.md"
  ]
}
JSON
  python3 - "$dir/.agents/rpi/execution-packet.json" "$dir" <<'PY'
import pathlib
import sys
path = pathlib.Path(sys.argv[1])
root = sys.argv[2]
path.write_text(path.read_text().replace("__ROOT__", root))
PY
  cat > "$dir/.agents/rpi/runs/run-closeout/execution-packet.json" <<'JSON'
{
  "run_id": "run-closeout",
  "proof_artifacts": [
    ".agents/rpi/runs/run-closeout/execution-packet.json"
  ]
}
JSON
  git -C "$dir" add .
  git -C "$dir" commit -q -m "fixture closeout proof"
}

assert_json_value() {
  local json_file="$1"
  local expr="$2"
  python3 - "$json_file" "$expr" <<'PY'
import json
import sys
path, expr = sys.argv[1:3]
data = json.load(open(path))
if not eval(expr, {"data": data}):
    raise SystemExit(1)
PY
}

test_passes_with_proof_and_clean_worktree() {
  local dir
  dir="$(mktemp -d "${TMPDIR:-/tmp}/closeout-clean.XXXXXX")"
  make_repo "$dir"
  local out
  out="$(mktemp "${TMPDIR:-/tmp}/closeout-clean-out.XXXXXX")"
  "$script_path" --root "$dir" --json > "$out"
  assert_json_value "$out" "data['result'] == 'PASS'"
  assert_json_value "$out" "data['closure_replay']['proof_ref_count'] == 2"
  assert_json_value "$out" "data['provenance']['packet_count'] >= 1"
  assert_json_value "$out" "data['provenance']['refs'][0]['pr_url'] == 'https://github.com/example/agentops/pull/204'"
  assert_json_value "$out" "data['worktree']['clean'] is True"
}

test_fails_dirty_by_default() {
  local dir
  dir="$(mktemp -d "${TMPDIR:-/tmp}/closeout-dirty.XXXXXX")"
  make_repo "$dir"
  printf 'dirty\n' > "$dir/dirty.txt"
  local out err
  out="$(mktemp "${TMPDIR:-/tmp}/closeout-dirty-out.XXXXXX")"
  err="$(mktemp "${TMPDIR:-/tmp}/closeout-dirty-err.XXXXXX")"
  if "$script_path" --root "$dir" --json > "$out" 2>"$err"; then
    fail "dirty worktree should fail by default"
  fi
  assert_json_value "$out" "data['result'] == 'FAIL'"
  assert_json_value "$out" "data['worktree']['dirty_count'] == 1"
}

test_allow_dirty_reports_but_passes() {
  local dir
  dir="$(mktemp -d "${TMPDIR:-/tmp}/closeout-allow-dirty.XXXXXX")"
  make_repo "$dir"
  printf 'dirty\n' > "$dir/dirty.txt"
  local out
  out="$(mktemp "${TMPDIR:-/tmp}/closeout-allow-dirty-out.XXXXXX")"
  "$script_path" --root "$dir" --allow-dirty --json > "$out"
  assert_json_value "$out" "data['result'] == 'PASS'"
  assert_json_value "$out" "data['worktree']['clean'] is False"
  assert_json_value "$out" "data['worktree']['allow_dirty'] is True"
}

test_fails_without_proof_refs() {
  local dir
  dir="$(mktemp -d "${TMPDIR:-/tmp}/closeout-no-proof.XXXXXX")"
  git -C "$dir" init -q
  git -C "$dir" config user.email "closeout-gate@example.test"
  git -C "$dir" config user.name "Closeout Gate Test"
  mkdir -p "$dir/.agents/rpi"
  echo '{"run_id":"no-proof"}' > "$dir/.agents/rpi/execution-packet.json"
  git -C "$dir" add .
  git -C "$dir" commit -q -m "fixture without proof"
  local out
  out="$(mktemp "${TMPDIR:-/tmp}/closeout-no-proof-out.XXXXXX")"
  if "$script_path" --root "$dir" --json > "$out"; then
    fail "missing proof refs should fail"
  fi
  assert_json_value "$out" "data['result'] == 'FAIL'"
  assert_json_value "$out" "'no RPI execution-packet proof_refs found' in data['errors']"
}

test_passes_with_proof_and_clean_worktree
test_fails_dirty_by_default
test_allow_dirty_reports_but_passes
test_fails_without_proof_refs

echo "PASS: check-closeout-gate.sh"
