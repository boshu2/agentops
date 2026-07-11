#!/usr/bin/env bats
# pawl-review-cold-user.bats — the cold USER front-door e2e (age-hk5zg.1, S1 of the
# pawl-user-front-door packet). A real AgentOps user has: the INSTALLED ao binary, ONE
# reviewer CLI (codex or agy), their OWN git repo — no NTM, no atm, no projects_base, no
# operator config. `ao pawl review` must give them a working cross-family review there.
#
# These tests run the BUILT ao binary COPIED OUTSIDE the checkout (so aoBinaryInside()
# fails and the binary structurally takes the EMBEDDED-bundle stranger path — the exact
# path an installed binary takes), against a throwaway repo under mktemp (never under any
# projects_base). ntm and atm are shadowed by POISONED sentinels that record any
# invocation: the assertion is not merely "ntm was absent" but "the cold path NEVER
# reached for the swarm binary at all". Locks:
#   1. stubbed reviewer present -> a verdict is produced from the embedded bundle
#      (exit 0, CONFIRMED, verdict file written into the user's own state dir);
#   2. reviewer absent -> an HONEST actionable error naming the reviewer CLI (exit 2),
#      NO review artifacts;
#   3. in BOTH cases: zero ntm/atm invocations and never a projects_base/repo-root
#      failure (the sharp edges this packet exists to remove).
# A genuinely REAL cross-family verdict is the accompanying live/manual proof — the
# deterministic suite asserts the plumbing only (stub reviewer, hermetic).

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
  [[ -n "${AO_BIN:-}" && -x "${AO_BIN:-}" ]] || skip "no ao binary and no go toolchain to build one"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"

  # The "installed" binary: OUTSIDE the checkout, so the trust split cannot take the
  # live-checkout dogfood path even though the binary was built from one.
  mkdir -p "$TMP/installed"
  cp "$AO_BIN" "$TMP/installed/ao"

  # Poisoned swarm binaries: any invocation records itself and fails. Proves the cold
  # path never touches NTM rather than assuming its absence.
  POISON="$TMP/poison"; mkdir -p "$POISON"
  for b in ntm atm; do
    printf '#!/usr/bin/env bash\ntouch "%s/swarm-invoked.%s"\nexit 1\n' "$TMP" "$b" > "$POISON/$b"
    chmod +x "$POISON/$b"
  done

  # Stub reviewer dir (prepended only where a test wants the reviewer present).
  STUB="$TMP/stub"; mkdir -p "$STUB"

  # The user's own throwaway repo — under mktemp, NOT under any projects_base.
  USER_REPO="$TMP/user-repo"; mkdir -p "$USER_REPO"; cd "$USER_REPO"
  git init --quiet
  git config user.email cold-user@example.com; git config user.name "Cold User"
  printf 'a line\n' > app.txt
  git add app.txt; git commit --quiet -m init
  printf 'a line\nan added line under review\n' > app.txt
  git add app.txt; git commit --quiet -m "feat(app): a change (age-cold-user)"

  export STUB_SENTINEL="$TMP/called"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"; mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-cold-user.json"
  export PAWL_NO_PREFLIGHT=1   # the deterministic battery is out of scope (host-ao nondeterminism)
  export PAWL_AUTOBIND=0       # a test run must never create a ledger bind commit
  export PAWL_REVIEW_TIMEOUT=60
  # A polluted operator environment must not leak into the "fresh user" simulation.
  unset AGENTOPS_REPO_ROOT PAWL_SESSION PAWL_PROJECT PAWL_SWARM_BIN PAWL_REVIEWER_CHAIN 2>/dev/null || true
}

teardown() { cd "$ORIG_DIR" 2>/dev/null || true; rm -rf "$TMP"; }

_stub_codex_confirm() {
  cat > "$STUB/codex" <<'FAKE'
#!/usr/bin/env bash
[ -n "${STUB_SENTINEL:-}" ] && touch "${STUB_SENTINEL}.codex"
cat >/dev/null
echo codex
echo "Reviewed app.txt:2 — the added line is safe and matches the commit claim. tokens used: 1234"
echo "VERDICT: CONFIRMED"
exit 0
FAKE
  chmod +x "$STUB/codex"
}

# The failures this packet exists to remove: a fresh user must NEVER see the operator's
# warm-service sharp edges from the cold front door.
_assert_no_operator_sharp_edges() {
  [[ "$output" != *"projects_base"* ]]
  [[ "$output" != *"not a direct child"* ]]
  [[ "$output" != *"spawn"* ]]
  [ ! -f "$TMP/swarm-invoked.ntm" ]
  [ ! -f "$TMP/swarm-invoked.atm" ]
}

@test "S1: fresh user + stubbed reviewer -> verdict produced from the embedded bundle, zero NTM involvement" {
  _stub_codex_confirm
  run env PATH="$STUB:$POISON:$PATH" "$TMP/installed/ao" pawl review age-cold-user --scope head
  echo "status=$status output=$output"
  [ "$status" -eq 0 ]
  [[ "$output" == *"CONFIRMED"* ]]
  # The cross-family reviewer actually ran (the verdict is not self-stamped) …
  [ -f "$TMP/called.codex" ]
  # … and the commit-bound verdict landed in the USER's state dir, not the bundle temp.
  [ -f "$VFILE" ]
  grep -q '"verdict": *"CONFIRMED"' "$VFILE"
  _assert_no_operator_sharp_edges
}

@test "S1: fresh user + NO reviewer CLI -> honest actionable error naming the reviewer, no artifacts" {
  # codex deterministically missing (no stub; CODEX_EXEC_BIN points at nothing). The
  # default reviewer chain is codex-only, so no real host reviewer can be consulted.
  run env PATH="$POISON:$PATH" CODEX_EXEC_BIN=absent-codex-xyz \
    "$TMP/installed/ao" pawl review age-cold-user --scope head
  echo "status=$status output=$output"
  [ "$status" -eq 2 ]
  [[ "$output" == *"MISSING DEPENDENCY"* ]]
  [[ "$output" == *"codex"* ]]
  [[ "$output" == *"not a REFUTE"* ]]
  [ ! -f "$VFILE" ]
  [ ! -d "$USER_REPO/.agents/pawl-evidence" ]
  _assert_no_operator_sharp_edges
}

@test "S1: the front door never requires the warm service — review succeeds with the standing-service verbs unusable" {
  # Same green run, but assert the poisoned swarm binaries stayed untouched even though
  # they were FIRST on PATH — the cold path must not even probe them.
  _stub_codex_confirm
  run env PATH="$POISON:$STUB:$POISON:$PATH" "$TMP/installed/ao" pawl review age-cold-user --scope head
  [ "$status" -eq 0 ]
  [ ! -f "$TMP/swarm-invoked.ntm" ]
  [ ! -f "$TMP/swarm-invoked.atm" ]
}
