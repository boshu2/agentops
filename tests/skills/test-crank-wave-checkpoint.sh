#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VALIDATOR="$ROOT/skills/crank/scripts/validate-wave-checkpoint.sh"

PASS=0
FAIL=0

pass() {
  echo "PASS: $1"
  PASS=$((PASS + 1))
}

fail() {
  echo "FAIL: $1"
  FAIL=$((FAIL + 1))
}

run_expect_success() {
  local name="$1"
  shift
  if "$@" >/tmp/crank-wave-checkpoint.out 2>/tmp/crank-wave-checkpoint.err; then
    pass "$name"
  else
    fail "$name"
    cat /tmp/crank-wave-checkpoint.err
  fi
}

run_expect_failure() {
  local name="$1"
  shift
  if "$@" >/tmp/crank-wave-checkpoint.out 2>/tmp/crank-wave-checkpoint.err; then
    fail "$name"
    cat /tmp/crank-wave-checkpoint.out
  else
    pass "$name"
  fi
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp" /tmp/crank-wave-checkpoint.out /tmp/crank-wave-checkpoint.err' EXIT

git -C "$tmp" init -q
git -C "$tmp" config user.email test@example.com
git -C "$tmp" config user.name "Test User"
# Insulate the fixture commit from the operator's global git config: some
# environments set commit.gpgsign=true or a custom gpg.ssh.program that would
# require a real signing key and fail inside a throwaway fixture.
git -C "$tmp" config commit.gpgsign false
git -C "$tmp" config tag.gpgsign false
printf 'fixture\n' > "$tmp/README.md"
git -C "$tmp" add README.md
git -C "$tmp" -c core.hooksPath=/dev/null commit -q -m "fixture"
sha="$(git -C "$tmp" rev-parse HEAD)"

write_checkpoint() {
  local path="$1"
  local checkpoint_sha="$2"
  local timestamp="$3"
  cat > "$path" <<EOF
{
  "schema_version": 1,
  "wave": 1,
  "timestamp": "$timestamp",
  "tasks_completed": ["na-1"],
  "tasks_failed": [],
  "files_changed": ["README.md"],
  "git_sha": "$checkpoint_sha",
  "acceptance_verdict": "PASS",
  "commit_strategy": "wave-batch",
  "mutations_this_wave": 0,
  "total_mutations": 0,
  "mutation_budget": {
    "task_added": {"used": 0, "limit": 5},
    "task_reordered": {"used": 0, "limit": 3}
  }
}
EOF
}

valid="$tmp/valid.json"
bad_sha="$tmp/bad-sha.json"
bad_time="$tmp/bad-time.json"
missing_field="$tmp/missing-field.json"
now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

write_checkpoint "$valid" "$sha" "$now"
write_checkpoint "$bad_sha" "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" "$now"
write_checkpoint "$bad_time" "$sha" "not-a-date"
jq 'del(.git_sha)' "$valid" > "$missing_field"

run_expect_success "valid checkpoint passes" bash "$VALIDATOR" "$valid" "$tmp"
run_expect_failure "non-resolving git_sha fails" bash "$VALIDATOR" "$bad_sha" "$tmp"
run_expect_failure "invalid timestamp fails" bash "$VALIDATOR" "$bad_time" "$tmp"
run_expect_failure "missing required field fails" bash "$VALIDATOR" "$missing_field" "$tmp"

echo ""
echo "Results: $PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]]
