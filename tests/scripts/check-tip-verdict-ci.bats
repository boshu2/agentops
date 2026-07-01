#!/usr/bin/env bats
# age-wedge-all-in-dyr0.9: CI verdict backstop — scripts/check-tip-verdict-ci.sh
#
# Fixture repos are built in BATS_TMPDIR with real hash-chained ledgers sealed
# by `ao provenance add`, so the tamper test exercises the REAL chain
# verification, not a stub.

setup_file() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  export REPO_ROOT
  # Resolve a real ao binary: env override > repo build > build fresh.
  if [[ -n "${AO_BIN:-}" && -x "${AO_BIN:-}" ]]; then
    :
  elif [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
    export AO_BIN="$REPO_ROOT/cli/bin/ao"
  elif command -v go >/dev/null 2>&1; then
    ( cd "$REPO_ROOT/cli" && go build -o "$BATS_FILE_TMPDIR/ao" ./cmd/ao )
    export AO_BIN="$BATS_FILE_TMPDIR/ao"
  fi
}

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/check-tip-verdict-ci.sh"
  PRE_PUSH="$REPO_ROOT/scripts/check-pawl-pre-push.sh"
  WAIVER_LIB="$REPO_ROOT/scripts/lib/trivial-waiver.sh"
  TMP="$(mktemp -d "${BATS_TMPDIR}/tip-verdict-ci.XXXXXX")"
}

teardown() {
  rm -rf "$TMP"
}

require_ao() {
  [[ -n "${AO_BIN:-}" && -x "${AO_BIN:-}" ]] || skip "no ao binary and no go toolchain to build one"
  command -v jq >/dev/null 2>&1 || skip "jq unavailable"
}

# make_repo — fixture repo with an init commit; sets REPO, BASE_SHA.
make_repo() {
  REPO="$TMP/repo"
  mkdir -p "$REPO"
  git -C "$TMP" init --quiet repo
  git -C "$REPO" config user.email test@example.com
  git -C "$REPO" config user.name Test
  echo ok > "$REPO/README.md"
  git -C "$REPO" add README.md
  git -C "$REPO" commit --quiet -m "init"
  BASE_SHA="$(git -C "$REPO" rev-parse HEAD)"
}

# add_code_commit <msg> — non-provenance change; sets LAST_SHA.
add_code_commit() {
  echo "change $RANDOM" >> "$REPO/README.md"
  git -C "$REPO" add README.md
  git -C "$REPO" commit --quiet -m "$1"
  LAST_SHA="$(git -C "$REPO" rev-parse HEAD)"
}

# bind_verdict <bead> <sha> — seal a CONFIRMED verdict edge onto the fixture
# ledger with the real chain writer (same edge shape ao provenance emit-verdict
# emits).
bind_verdict() {
  local bead="$1" sha="$2"
  ( cd "$REPO" && "$AO_BIN" provenance add "${bead}@${sha:0:7}" "$sha" \
      --relation wasDerivedFrom --from-type verdict --to-type commit \
      --trust-tier inferred \
      --evidence "pawl-verdict $bead disposition=CONFIRMED" >/dev/null )
}

# add_trivial_provenance_commit — a #trivial commit touching ONLY
# docs/provenance/ (a freshly sealed, chain-valid ledger edge); sets LAST_SHA.
add_trivial_provenance_commit() {
  ( cd "$REPO" && "$AO_BIN" provenance add "age-tvc-seed@0000000" \
      "0000000000000000000000000000000000000001" \
      --relation wasDerivedFrom --from-type verdict --to-type commit \
      --trust-tier inferred \
      --evidence "pawl-verdict age-tvc-seed disposition=CONFIRMED" >/dev/null )
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind edge (age-tvc-seed) #trivial"
  LAST_SHA="$(git -C "$REPO" rev-parse HEAD)"
}

run_backstop() {
  status=0
  output="$(AO_BIN="$AO_BIN" bash "$SCRIPT" "$@" 2>&1)" || status=$?
}

@test "range with every commit verdicted is green (report mode, no warnings)" {
  require_ao
  make_repo
  add_code_commit "feat(x): one (age-tvc.1)"; C1="$LAST_SHA"
  add_code_commit "feat(x): two (age-tvc.2)"; C2="$LAST_SHA"
  bind_verdict age-tvc.1 "$C1"
  bind_verdict age-tvc.2 "$C2"
  run_backstop --repo "$REPO" --base "$BASE_SHA" --head "$C2"
  [ "$status" -eq 0 ]
  [[ "$output" == *"hash chain intact"* ]]
  [[ "$output" == *"verified — bound verdict event age-tvc.1@${C1:0:7}"* ]]
  [[ "$output" == *"verified — bound verdict event age-tvc.2@${C2:0:7}"* ]]
  [[ "$output" != *"::warning::"* ]]
  [[ "$output" != *"::error::"* ]]
}

@test "unverdicted commit: report mode exits 0 and warns naming the sha" {
  require_ao
  make_repo
  add_code_commit "feat(x): proven (age-tvc.1)"; C1="$LAST_SHA"
  bind_verdict age-tvc.1 "$C1"
  add_code_commit "feat(x): unproven (age-tvc.2)"; C2="$LAST_SHA"
  run_backstop --repo "$REPO" --base "$BASE_SHA" --head "$C2"
  [ "$status" -eq 0 ]
  [[ "$output" == *"::warning::commit ${C2:0:12} lacks a bound verdict"* ]]
  [[ "$output" == *"report-only, not failing"* ]]
  [[ "$output" != *"::warning::commit ${C1:0:12}"* ]]
}

@test "unverdicted commit: enforce mode exits nonzero" {
  require_ao
  make_repo
  add_code_commit "feat(x): unproven (age-tvc.3)"; C1="$LAST_SHA"
  run_backstop --repo "$REPO" --base "$BASE_SHA" --head "$C1" --enforce
  [ "$status" -ne 0 ]
  [[ "$output" == *"::warning::commit ${C1:0:12} lacks a bound verdict"* ]]
  [[ "$output" == *"::error::"*"lack proof"* ]]
}

@test "#trivial provenance-only commit is waived in report mode" {
  require_ao
  make_repo
  add_trivial_provenance_commit; C1="$LAST_SHA"
  run_backstop --repo "$REPO" --base "$BASE_SHA" --head "$C1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"::notice::commit ${C1:0:12} waived"* ]]
  [[ "$output" == *"pawl waived"* ]]
  [[ "$output" != *"::warning::"* ]]
}

@test "#trivial provenance-only commit is waived in enforce mode too" {
  require_ao
  make_repo
  add_trivial_provenance_commit; C1="$LAST_SHA"
  run_backstop --repo "$REPO" --base "$BASE_SHA" --head "$C1" --enforce
  [ "$status" -eq 0 ]
  [[ "$output" == *"::notice::commit ${C1:0:12} waived"* ]]
}

@test "#trivial mislabel touching non-provenance paths is NOT waived (report warns)" {
  require_ao
  make_repo
  add_code_commit "chore(x): sneak a real change past the gate #trivial"; C1="$LAST_SHA"
  run_backstop --repo "$REPO" --base "$BASE_SHA" --head "$C1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"waiver REFUSED"* ]]
  [[ "$output" == *"::warning::commit ${C1:0:12} lacks a bound verdict"* ]]
}

@test "tampered ledger exits nonzero in report mode (tamper trumps report-only)" {
  require_ao
  make_repo
  add_code_commit "feat(x): one (age-tvc.1)"; C1="$LAST_SHA"
  bind_verdict age-tvc.1 "$C1"
  content="$(cat "$REPO/docs/provenance/ledger.jsonl")"
  printf '%s\n' "${content//CONFIRMED/TAMPERED}" > "$REPO/docs/provenance/ledger.jsonl"
  run_backstop --repo "$REPO" --base "$BASE_SHA" --head "$C1"
  [ "$status" -ne 0 ]
  [[ "$output" == *"::error::provenance ledger hash chain BROKEN/TAMPERED"* ]]
}

@test "tampered ledger exits nonzero in enforce mode" {
  require_ao
  make_repo
  add_code_commit "feat(x): one (age-tvc.1)"; C1="$LAST_SHA"
  bind_verdict age-tvc.1 "$C1"
  content="$(cat "$REPO/docs/provenance/ledger.jsonl")"
  printf '%s\n' "${content//CONFIRMED/TAMPERED}" > "$REPO/docs/provenance/ledger.jsonl"
  run_backstop --repo "$REPO" --base "$BASE_SHA" --head "$C1" --enforce
  [ "$status" -ne 0 ]
  [[ "$output" == *"BROKEN/TAMPERED"* ]]
}

@test "empty/all-zeros base falls back to tip-only" {
  require_ao
  make_repo
  add_code_commit "feat(x): one (age-tvc.1)"; C1="$LAST_SHA"
  bind_verdict age-tvc.1 "$C1"
  run_backstop --repo "$REPO" --base "0000000000000000000000000000000000000000" --head "$C1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"checking tip commit only"* ]]
  [[ "$output" == *"verified — bound verdict event age-tvc.1@${C1:0:7}"* ]]
}

# ── single-implementation guard (duel-approved hard acceptance) ─────────────
# The #trivial waiver must have exactly ONE implementation
# (scripts/lib/trivial-waiver.sh) that BOTH the pre-push gate and the CI
# backstop source and call — a drifted copy in either caller fails here.

@test "single-implementation: both surfaces source lib/trivial-waiver.sh and call pawl_trivial_waiver" {
  grep -q 'source "\$SCRIPT_DIR/lib/trivial-waiver.sh"' "$PRE_PUSH"
  grep -q 'source "\$SCRIPT_DIR/lib/trivial-waiver.sh"' "$SCRIPT"
  grep -Eq '^[[:space:]]*pawl_trivial_waiver ' "$PRE_PUSH"
  grep -Eq '^[[:space:]]*pawl_trivial_waiver ' "$SCRIPT"
  # And the shared lib actually defines it.
  grep -q '^pawl_trivial_waiver()' "$WAIVER_LIB"
}

@test "single-implementation: the waiver logic lives ONLY in the shared lib (no copies)" {
  # The waiver body's two distinctive load-bearing lines: the diff verification
  # (age-u43w) and the explicit-marker regex (age-w2ny). Each must appear in
  # the lib and in NEITHER caller — a re-inlined copy is the drift this guards.
  grep -q 'diff-tree --no-commit-id --no-renames --name-only' "$WAIVER_LIB"
  ! grep -q 'diff-tree --no-commit-id --no-renames --name-only' "$PRE_PUSH"
  ! grep -q 'diff-tree --no-commit-id --no-renames --name-only' "$SCRIPT"
  grep -q '#trivial\[\[:space:\]\]\*\$' "$WAIVER_LIB"
  ! grep -q '#trivial\[\[:space:\]\]\*\$' "$PRE_PUSH"
  ! grep -q '#trivial\[\[:space:\]\]\*\$' "$SCRIPT"
}
