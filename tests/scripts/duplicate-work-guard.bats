#!/usr/bin/env bats
# Regression tests for skills/evolve/scripts/duplicate-work-guard.sh (ag-6jt/ag-2je).
#
# The evolve-cron-rpi discovery loop kept re-seeding tracking beads for work
# already covered by an existing bead or merged PR (ag-b8m≈ag-jov, ag-6kw≈ag-c2i)
# because the only prior guard matched EXACT open-bead titles. These tests pin
# the guard's behavior: it must catch (a) exact normalized title matches and
# (b) same-surface, different-wording matches by significant-token overlap,
# across open AND closed beads — while NOT blocking genuinely novel work.
#
# `bd` is PATH-stubbed to return a fixed bead set so tests are hermetic.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/skills/evolve/scripts/duplicate-work-guard.sh"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"

  # Fixture bead set the stubbed `bd list --all --json` returns.
  cat > "$TMP/beads.json" <<'JSON'
[
  {"id":"ag-jov","status":"closed","title":"Fix dangling validate-hooks-doc-parity.sh ref in docs-release-governance eval canary"},
  {"id":"ag-x1","status":"open","title":"Add dark mode toggle to settings page"},
  {"id":"ag-open1","status":"open","title":"Implement origin main diff in evolve cron rpi discovery"}
]
JSON

  # Stub `bd`: any `list ... --json` invocation prints the fixture array.
  mkdir -p "$TMP/bin"
  cat > "$TMP/bin/bd" <<EOF
#!/usr/bin/env bash
if [ "\$1" = "list" ]; then cat "$TMP/beads.json"; else echo "[]"; fi
EOF
  chmod +x "$TMP/bin/bd"
  PATH="$TMP/bin:$PATH"
}

teardown() {
  PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

@test "exact normalized title match is flagged as duplicate (exit 1)" {
  run "$SCRIPT" "Implement origin main diff in evolve cron rpi discovery"
  [ "$status" -eq 1 ]
  [[ "$output" == *"DUPLICATE"* ]]
  [[ "$output" == *"ag-open1"* ]]
}

@test "same-surface different-wording match is flagged via token overlap (the ag-b8m≈ag-jov class)" {
  run "$SCRIPT" "Repair docs-release-governance eval canary dangling hooks-doc-parity reference"
  [ "$status" -eq 1 ]
  [[ "$output" == *"ag-jov"* ]]
}

@test "closed beads are checked, not only open ones" {
  # ag-jov is closed; a near-duplicate of it must still be caught.
  run "$SCRIPT" "docs-release-governance eval canary dangling parity hooks doc ref"
  [ "$status" -eq 1 ]
  [[ "$output" == *"ag-jov [closed]"* ]]
}

@test "genuinely novel work is NOT flagged (exit 0, no false positive)" {
  run "$SCRIPT" "Configure Prometheus alerting for GPU temperature thresholds"
  [ "$status" -eq 0 ]
  [[ "$output" == *"OK"* ]]
}

@test "missing title argument is a usage error (exit 2)" {
  run "$SCRIPT"
  [ "$status" -eq 2 ]
}

@test "blank title argument is a usage error (exit 2)" {
  run "$SCRIPT" "   "
  [ "$status" -eq 2 ]
}
