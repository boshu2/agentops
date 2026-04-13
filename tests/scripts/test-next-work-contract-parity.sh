#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
GATE_SCRIPT="$REPO_ROOT/scripts/validate-next-work-contract-parity.sh"

PASS_COUNT=0
FAIL_COUNT=0
TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

pass() {
  echo "PASS: $1"
  PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
  echo "FAIL: $1"
  FAIL_COUNT=$((FAIL_COUNT + 1))
}

copy_target() {
  local fixture="$1"
  local rel="$2"
  mkdir -p "$fixture/$(dirname "$rel")"
  /bin/cp "$REPO_ROOT/$rel" "$fixture/$rel"
}

create_fixture() {
  local fixture="$TMP_DIR/fixture-$1"
  mkdir -p "$fixture"

  copy_target "$fixture" "docs/contracts/next-work.schema.md"
  copy_target "$fixture" "skills/post-mortem/references/harvest-next-work.md"
  copy_target "$fixture" "skills/post-mortem/SKILL.md"
  copy_target "$fixture" "skills-codex/post-mortem/SKILL.md"
  copy_target "$fixture" "skills/rpi/references/phase-data-contracts.md"
  copy_target "$fixture" "skills/rpi/references/gate4-loop-and-spawn.md"
  copy_target "$fixture" "cli/cmd/ao/rpi_loop.go"
  copy_target "$fixture" "cli/internal/rpi/types.go"
  copy_target "$fixture" "cli/internal/rpi/helpers.go"
  copy_target "$fixture" "tests/smoke-test.sh"

  echo "$fixture"
}

run_gate() {
  local target_root="$1"
  local output_file="$2"

  set +e
  "$GATE_SCRIPT" "$target_root" >"$output_file" 2>&1
  local status=$?
  set -e
  return "$status"
}

run_gate_without_rg() {
  local target_root="$1"
  local output_file="$2"

  set +e
  PATH="/usr/bin:/bin" "$GATE_SCRIPT" "$target_root" >"$output_file" 2>&1
  local status=$?
  set -e
  return "$status"
}

assert_gate_passes() {
  local description="$1"
  local target_root="$2"
  local output_file
  output_file="$(mktemp "$TMP_DIR/output-pass-XXXXXX")"

  if run_gate "$target_root" "$output_file"; then
    pass "$description"
  else
    echo "--- output ($description) ---"
    cat "$output_file"
    fail "$description"
  fi
}

assert_gate_fails_with() {
  local description="$1"
  local target_root="$2"
  local expected="$3"
  local output_file
  output_file="$(mktemp "$TMP_DIR/output-fail-XXXXXX")"

  if run_gate "$target_root" "$output_file"; then
    echo "--- output ($description) ---"
    cat "$output_file"
    fail "$description (expected failure, got success)"
    return
  fi

  if grep -Fq "$expected" "$output_file"; then
    pass "$description"
  else
    echo "--- output ($description) ---"
    cat "$output_file"
    fail "$description (missing expected text: $expected)"
  fi
}

test_script_executable() {
  if [[ -x "$GATE_SCRIPT" ]]; then
    pass "validate-next-work-contract-parity.sh is executable"
  else
    fail "validate-next-work-contract-parity.sh is not executable"
  fi
}

test_repo_baseline_passes() {
  assert_gate_passes "repository baseline passes next-work contract parity gate" "$REPO_ROOT"
}

test_repo_baseline_passes_without_rg() {
  local output_file
  output_file="$(mktemp "$TMP_DIR/output-pass-no-rg-XXXXXX")"

  if run_gate_without_rg "$REPO_ROOT" "$output_file"; then
    pass "repository baseline passes next-work contract parity gate without rg"
  else
    echo "--- output (repository baseline passes next-work contract parity gate without rg) ---"
    cat "$output_file"
    fail "repository baseline passes next-work contract parity gate without rg"
  fi
}

test_schema_version_drift_fails() {
  local fixture
  fixture="$(create_fixture "schema-version")"
  python3 - <<'PY' "$fixture/docs/contracts/next-work.schema.md"
from pathlib import Path
path = Path(__import__("sys").argv[1])
path.write_text(path.read_text().replace("schema_version: 1.3", "schema_version: 1.2"))
PY

  assert_gate_fails_with \
    "schema version drift fails parity gate" \
    "$fixture" \
    "next-work schema is not at v1.3"
}

test_runtime_pattern_fix_rank_fails() {
  local fixture
  fixture="$(create_fixture "pattern-fix-rank")"
  python3 - <<'PY' "$fixture/cli/internal/rpi/helpers.go"
from pathlib import Path
path = Path(__import__("sys").argv[1])
text = path.read_text()
text = text.replace('"feature", "improvement", "tech-debt", "pattern-fix", "bug", "task"', '"feature", "improvement", "tech-debt", "bug", "task"')
path.write_text(text)
PY

  assert_gate_fails_with \
    "runtime pattern-fix ranking drift fails parity gate" \
    "$fixture" \
    "RPI runtime is missing workTypeRank coverage for pattern-fix"
}

test_source_skill_legacy_example_fails() {
  local fixture
  fixture="$(create_fixture "source-legacy-example")"
  python3 - <<'PY' "$fixture/skills/post-mortem/SKILL.md"
from pathlib import Path
path = Path(__import__("sys").argv[1])
text = path.read_text()
start = "#### Step ACT.3: Feed Next-Work"
end = "#### Step ACT.4: Update Marker"
section_start = text.index(start)
section_end = text.index(end, section_start)
replacement = """#### Step ACT.3: Feed Next-Work

Actionable improvements identified during processing -> append to `.agents/rpi/next-work.jsonl`:

```bash
mkdir -p .agents/rpi
# Only append if not already present (dedup by title)
TITLE=\"<improvement-title>\"
if ! grep -q \"\\\"title\\\":\\\"$TITLE\\\"\" .agents/rpi/next-work.jsonl 2>/dev/null; then
  echo \"{\\\"title\\\": \\\"$TITLE\\\", \\\"type\\\": \\\"process-improvement\\\", \\\"severity\\\": \\\"medium\\\", \\\"source\\\": \\\"backlog-processing\\\", \\\"claim_status\\\": \\\"available\\\", \\\"consumed\\\": false, \\\"timestamp\\\": \\\"$(date -Iseconds)\\\"}\" >> .agents/rpi/next-work.jsonl
fi
```

"""
path.write_text(text[:section_start] + replacement + text[section_end:])
PY

  assert_gate_fails_with \
    "source post-mortem skill legacy flat example fails parity gate" \
    "$fixture" \
    "skills/post-mortem/SKILL.md ACT.3 still contains the legacy flat-row append example"
}

test_codex_skill_legacy_example_fails() {
  local fixture
  fixture="$(create_fixture "codex-legacy-example")"
  python3 - <<'PY' "$fixture/skills-codex/post-mortem/SKILL.md"
from pathlib import Path
path = Path(__import__("sys").argv[1])
text = path.read_text()
start = "#### Step ACT.3: Feed Next-Work"
end = "#### Step ACT.4: Update Marker"
section_start = text.index(start)
section_end = text.index(end, section_start)
replacement = """#### Step ACT.3: Feed Next-Work

Actionable improvements identified during processing -> append to `.agents/rpi/next-work.jsonl`:

```bash
mkdir -p .agents/rpi
# Only append if not already present (dedup by title)
TITLE=\"<improvement-title>\"
if ! grep -q \"\\\"title\\\":\\\"$TITLE\\\"\" .agents/rpi/next-work.jsonl 2>/dev/null; then
  echo \"{\\\"title\\\": \\\"$TITLE\\\", \\\"type\\\": \\\"process-improvement\\\", \\\"severity\\\": \\\"medium\\\", \\\"source\\\": \\\"backlog-processing\\\", \\\"claim_status\\\": \\\"available\\\", \\\"consumed\\\": false, \\\"timestamp\\\": \\\"$(date -Iseconds)\\\"}\" >> .agents/rpi/next-work.jsonl
fi
```

"""
path.write_text(text[:section_start] + replacement + text[section_end:])
PY

  assert_gate_fails_with \
    "generated Codex post-mortem skill legacy flat example fails parity gate" \
    "$fixture" \
    "skills-codex/post-mortem/SKILL.md ACT.3 still contains the legacy flat-row append example"
}

test_explicit_item_lifecycle_drift_fails() {
  local fixture
  fixture="$(create_fixture "explicit-lifecycle-drift")"
  mkdir -p "$fixture/.agents/rpi"
  cat > "$fixture/.agents/rpi/next-work.jsonl" <<'EOF'
{"source_epic":"ag-drift","timestamp":"2026-04-13T00:00:00Z","items":[{"title":"Already done","type":"task","severity":"medium","source":"retro-learning","description":"Done item","target_repo":"agentops","consumed":true,"claim_status":"consumed"},{"title":"Still available","type":"task","severity":"medium","source":"retro-learning","description":"Available item","target_repo":"agentops","claim_status":"available"}],"consumed":true,"claim_status":"consumed","claimed_by":null,"claimed_at":null,"consumed_by":"test","consumed_at":"2026-04-13T00:01:00Z"}
EOF

  assert_gate_fails_with \
    "explicit item lifecycle drift fails parity gate" \
    "$fixture" \
    "next-work.jsonl has aggregate/item lifecycle drift"
}

test_aggregate_self_drift_fails() {
  local fixture
  fixture="$(create_fixture "aggregate-self-drift")"
  mkdir -p "$fixture/.agents/rpi"
  cat > "$fixture/.agents/rpi/next-work.jsonl" <<'EOF'
{"source_epic":"ag-self-drift","timestamp":"2026-04-13T00:00:00Z","items":[{"title":"Legacy item","type":"task","severity":"medium","source":"retro-learning","description":"Legacy item with no per-item lifecycle","target_repo":"agentops"}],"consumed":true,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":"test","consumed_at":"2026-04-13T00:01:00Z"}
EOF

  assert_gate_fails_with \
    "aggregate self drift fails parity gate" \
    "$fixture" \
    "next-work.jsonl has aggregate lifecycle self drift"
}

test_active_item_enum_drift_fails() {
  local fixture
  fixture="$(create_fixture "active-enum-drift")"
  mkdir -p "$fixture/.agents/rpi"
  cat > "$fixture/.agents/rpi/next-work.jsonl" <<'EOF'
{"source_epic":"ag-enum-drift","timestamp":"2026-04-13T00:00:00Z","items":[{"title":"Write docs","type":"docs","severity":"moderate","source":"post-mortem","description":"Active item with legacy enum values","target_repo":"agentops","consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}
EOF

  assert_gate_fails_with \
    "active item enum drift fails parity gate" \
    "$fixture" \
    "next-work.jsonl has active item enum drift"
}

test_consumed_legacy_enum_drift_passes() {
  local fixture
  fixture="$(create_fixture "consumed-legacy-enum-drift")"
  mkdir -p "$fixture/.agents/rpi"
  cat > "$fixture/.agents/rpi/next-work.jsonl" <<'EOF'
{"source_epic":"ag-consumed-legacy","timestamp":"2026-04-13T00:00:00Z","items":[{"title":"Historical finding","type":"finding","severity":"moderate","source":"finding-router","description":"Consumed historical item with legacy enum values","target_repo":"agentops"}],"consumed":true,"claim_status":"consumed","claimed_by":null,"claimed_at":null,"consumed_by":"test","consumed_at":"2026-04-13T00:01:00Z"}
EOF

  assert_gate_passes \
    "consumed legacy enum drift passes parity gate" \
    "$fixture"
}

test_legacy_aggregate_only_consumed_queue_passes() {
  local fixture
  fixture="$(create_fixture "legacy-aggregate-only")"
  mkdir -p "$fixture/.agents/rpi"
  cat > "$fixture/.agents/rpi/next-work.jsonl" <<'EOF'
{"source_epic":"ag-legacy","timestamp":"2026-03-01T00:00:00Z","items":[{"title":"Legacy item","type":"task","severity":"medium","source":"retro-learning","description":"Legacy item with no per-item lifecycle","target_repo":"agentops"}],"consumed":true,"claim_status":"consumed","claimed_by":null,"claimed_at":null,"consumed_by":"legacy","consumed_at":"2026-03-01T00:01:00Z"}
EOF

  assert_gate_passes \
    "legacy aggregate-only consumed queue passes parity gate" \
    "$fixture"
}

echo "================================"
echo "Testing next-work contract parity gate"
echo "================================"
echo ""

test_script_executable
test_repo_baseline_passes
test_repo_baseline_passes_without_rg
test_schema_version_drift_fails
test_runtime_pattern_fix_rank_fails
test_source_skill_legacy_example_fails
test_codex_skill_legacy_example_fails
test_explicit_item_lifecycle_drift_fails
test_aggregate_self_drift_fails
test_active_item_enum_drift_fails
test_consumed_legacy_enum_drift_passes
test_legacy_aggregate_only_consumed_queue_passes

echo ""
echo "================================"
echo "Results: $PASS_COUNT PASS, $FAIL_COUNT FAIL"
echo "================================"

if [[ $FAIL_COUNT -gt 0 ]]; then
  exit 1
fi
exit 0
