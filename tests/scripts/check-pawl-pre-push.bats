#!/usr/bin/env bats
# age-58o: push-to-main pawl gate via scripts/check-pawl-pre-push.sh

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/check-pawl-pre-push.sh"
  PAWL="$REPO_ROOT/scripts/pawl-verdict.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"
  mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  SHA="cafef00dbabe1234cafef00dbabe1234cafef00d"
  printf 'fresh-context review evidence\n' > "$TMP/evidence.txt"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

seed_verdict() {
  local bead="$1" head="$2"
  bash "$PAWL" write "$bead" 0 \
    --disposition CONFIRMED --head "$head" \
    --author-context author-ctx \
    --refuter "claude:CONFIRMED:fresh-reviewer-ctx:$TMP/evidence.txt" \
    --dir "$AGENTOPS_PAWL_VERDICT_DIR" >/dev/null
}

make_repo_with_commit() {
  local bead="$1"
  local msg="${2:-fix(test): wire pawl ($bead)}"
  REPO="$TMP/repo"
  mkdir -p "$REPO"
  cd "$REPO"
  git init --quiet
  git config user.email test@example.com
  git config user.name Test
  echo ok > README.md
  git add README.md
  git commit --quiet -m "init"
  echo change >> README.md
  git add README.md
  git commit --quiet -m "$msg"
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO" HEAD_SHA
}

# age-u43w: a legitimately-trivial commit touches ONLY the provenance ledger.
make_repo_with_provenance_commit() {
  local msg="$1"
  REPO="$TMP/repo"
  rm -rf "$REPO"
  mkdir -p "$REPO"
  cd "$REPO"
  git init --quiet
  git config user.email test@example.com
  git config user.name Test
  echo ok > README.md
  git add README.md
  git commit --quiet -m "init"
  mkdir -p docs/provenance
  echo '{"edge":1}' >> docs/provenance/ledger.jsonl
  git add docs/provenance/ledger.jsonl
  git commit --quiet -m "$msg"
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO" HEAD_SHA
}

# age-u43w: like make_repo_with_body_commit but the change is provenance-only.
make_repo_with_provenance_body_commit() {
  local subject="$1" body="$2"
  REPO="$TMP/repo"
  rm -rf "$REPO"
  mkdir -p "$REPO"
  cd "$REPO"
  git init --quiet
  git config user.email test@example.com
  git config user.name Test
  echo ok > README.md
  git add README.md
  git commit --quiet -m "init"
  mkdir -p docs/provenance
  echo '{"edge":1}' >> docs/provenance/ledger.jsonl
  git add docs/provenance/ledger.jsonl
  printf '%s\n\n%s\n' "$subject" "$body" | git commit --quiet -F -
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO" HEAD_SHA
}

@test "check-pawl-pre-push skips when no pre-push stdin" {
  run bash "$SCRIPT" </dev/null
  [ "$status" -eq 0 ]
  [[ "$output" == *"no pre-push stdin"* ]]
}

@test "check-pawl-pre-push skips non-main remote ref" {
  make_repo_with_commit age-58o-test-a
  status=0
  output="$(printf 'refs/heads/feat %s refs/heads/feat 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 0 ]
  [[ "$output" != *"PAWL-HOLD"* ]]
}

@test "check-pawl-pre-push blocks main push without verdict" {
  make_repo_with_commit age-58o-test-b
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 1 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
}

@test "check-pawl-pre-push authorizes main push with CONFIRMED verdict" {
  make_repo_with_commit age-58o-test-c
  seed_verdict age-58o-test-c "$HEAD_SHA"
  printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" > "$TMP/push.txt"
  status=0
  output="$(bash "$SCRIPT" < "$TMP/push.txt" 2>&1)" || status=$?
  [ "$status" -eq 0 ]
  [[ "$output" == *"push authorized"* ]]
}

@test "check-pawl-pre-push blocks main push when bead missing from commit" {
  make_repo_with_commit age-58o-test-d "chore: no bead cited"
  seed_verdict age-58o-test-d "$HEAD_SHA"
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 1 ]
  [[ "$output" == *"cites no bead id"* ]]
}

@test "check-pawl-pre-push waives #trivial commits on main (provenance-only)" {
  make_repo_with_provenance_commit "chore(provenance): edge (age-58o-test-e) #trivial"
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 0 ]
  [[ "$output" == *"pawl waived"* ]]
}

@test "check-pawl-pre-push does NOT waive #trivial that touches non-provenance paths (age-u43w)" {
  # The bypass this closes: a substantive change mislabeled #trivial. README.md is
  # NOT in the provenance allowlist, so the waiver must be REFUSED and the pawl
  # required — even with a perfectly-formed trailing #trivial tag.
  make_repo_with_commit age-u43w-test "chore(x): sneak a real change past the gate #trivial"
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 1 ]
  [[ "$output" == *"waiver REFUSED"* ]]
  [[ "$output" != *"pawl waived"* ]]
}

@test "check-pawl-pre-push: refused #trivial FALLS THROUGH to the pawl, authorizes WITH a verdict (age-u43w)" {
  # Proves the refuse branch does NOT hard-HOLD: a #trivial commit touching a
  # non-provenance path, cited bead + CONFIRMED verdict, must REFUSE the waiver yet
  # still AUTHORIZE via the normal pawl path (fall-through, not return 1).
  make_repo_with_commit age-u43w-fallthrough "feat(x): real change (age-u43w-fallthrough) #trivial"
  seed_verdict age-u43w-fallthrough "$HEAD_SHA"
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 0 ]
  [[ "$output" == *"waiver REFUSED"* ]]
  [[ "$output" == *"push authorized"* ]]
}

# age-w2ny: build a commit whose body (after the subject + blank line) is fully
# operator-controlled, so we can place #trivial in prose vs. on its own line.
make_repo_with_body_commit() {
  local subject="$1" body="$2"
  REPO="$TMP/repo"
  rm -rf "$REPO"
  mkdir -p "$REPO"
  cd "$REPO"
  git init --quiet
  git config user.email test@example.com
  git config user.name Test
  echo ok > README.md
  git add README.md
  git commit --quiet -m "init"
  echo change >> README.md
  git add README.md
  printf '%s\n\n%s\n' "$subject" "$body" | git commit --quiet -F -
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO" HEAD_SHA
}

@test "check-pawl-pre-push does NOT waive #trivial mentioned only in body prose (age-w2ny)" {
  make_repo_with_body_commit \
    "feat(x): real feature (age-w2ny-test-a)" \
    "This explains code that marks something #trivial in an inline sentence."
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 1 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  [[ "$output" != *"pawl waived"* ]]
}

@test "check-pawl-pre-push does NOT waive #trivial mentioned mid-subject as prose (age-w2ny)" {
  # The #trivial token is NOT a trailing tag — it is prose inside the subject.
  make_repo_with_commit age-w2ny-test-c "fix(pawl): prevent #trivial from bypassing the gate (age-w2ny-test-c)"
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 1 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  [[ "$output" != *"pawl waived"* ]]
}

@test "check-pawl-pre-push waives #trivial as a standalone trailer line in the body (age-w2ny)" {
  make_repo_with_provenance_body_commit \
    "chore(x): provenance-only edge (age-w2ny-test-b)" \
    "some body explanation here

#trivial"
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 0 ]
  [[ "$output" == *"pawl waived"* ]]
}

@test "check-pawl-pre-push honors AGENTOPS_PREPUSH_SKIP_PAWL=1" {
  make_repo_with_commit age-58o-test-f
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | env AGENTOPS_PREPUSH_SKIP_PAWL=1 bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 0 ]
  [[ "$output" == *"skipped"* ]]
}

# ── age-8ais: #trivial-tip range check ──────────────────────────────────────
# The escape (age-rk3r.5, 2026-07-02): the #trivial waiver keyed on the TIP
# alone, so a non-trivial commit pushed BEHIND a #trivial tip was never seen by
# the cockpit gate (`ao gate check --scope head` scopes to the tip's files) — a
# test-isolation ratchet-red landed on main. The gate is now RE-TARGETED at the
# newest non-trivial commit in the pushed range. These fixtures build the mixed
# range with a REAL remote sha (the range base), the shape a genuine
# push-to-main supplies. AGENTOPS_PREPUSH_GATE_CMD stands in for the real
# `ao gate check`; it runs inside the re-targeted DETACHED worktree, so
# `git rev-parse HEAD` there is the commit actually gated (proves re-targeting).

# init(base) → non-trivial feat commit → #trivial provenance-only tip.
make_repo_mixed_range() {
  local feat_msg="$1" tip_msg="$2"
  REPO="$TMP/repo"
  rm -rf "$REPO"
  mkdir -p "$REPO"
  cd "$REPO"
  git init --quiet
  git config user.email test@example.com
  git config user.name Test
  echo ok > README.md
  git add README.md
  git commit --quiet -m "init"
  BASE_SHA="$(git rev-parse HEAD)"
  echo change >> README.md
  git add README.md
  git commit --quiet -m "$feat_msg"
  FEAT_SHA="$(git rev-parse HEAD)"
  mkdir -p docs/provenance
  echo '{"edge":1}' >> docs/provenance/ledger.jsonl
  git add docs/provenance/ledger.jsonl
  git commit --quiet -m "$tip_msg"
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO" HEAD_SHA BASE_SHA FEAT_SHA
}

# init(base) → TWO provenance-only #trivial commits (a pure-trivial range).
make_repo_two_trivial_range() {
  REPO="$TMP/repo"
  rm -rf "$REPO"
  mkdir -p "$REPO"
  cd "$REPO"
  git init --quiet
  git config user.email test@example.com
  git config user.name Test
  echo ok > README.md
  git add README.md
  git commit --quiet -m "init"
  BASE_SHA="$(git rev-parse HEAD)"
  mkdir -p docs/provenance
  echo '{"edge":1}' >> docs/provenance/ledger.jsonl
  git add docs/provenance/ledger.jsonl
  git commit --quiet -m "chore(provenance): edge 1 (age-8ais-pt1) #trivial"
  echo '{"edge":2}' >> docs/provenance/ledger.jsonl
  git add docs/provenance/ledger.jsonl
  git commit --quiet -m "chore(provenance): edge 2 (age-8ais-pt2) #trivial"
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO" HEAD_SHA BASE_SHA
}

# The rk3r.5 shape: a *_test.go adding raw os.Setenv (ratchet violation) as the
# non-trivial commit, hidden behind a #trivial provenance-only tip.
make_repo_rk3r5_shape() {
  REPO="$TMP/repo"
  rm -rf "$REPO"
  mkdir -p "$REPO"
  cd "$REPO"
  git init --quiet
  git config user.email test@example.com
  git config user.name Test
  echo ok > README.md
  git add README.md
  git commit --quiet -m "init"
  BASE_SHA="$(git rev-parse HEAD)"
  mkdir -p cli/cmd/ao
  printf 'package main\n\nimport (\n\t"os"\n\t"testing"\n)\n\nfunc TestFoo(t *testing.T) {\n\tos.Setenv("X", "1")\n\tos.Setenv("Y", "2")\n\t_ = t\n}\n' > cli/cmd/ao/foo_test.go
  git add cli/cmd/ao/foo_test.go
  git commit --quiet -m "feat(ao): add test with raw os.Setenv (age-8ais-rk3r5)"
  FEAT_SHA="$(git rev-parse HEAD)"
  mkdir -p docs/provenance
  echo '{"edge":1}' >> docs/provenance/ledger.jsonl
  git add docs/provenance/ledger.jsonl
  git commit --quiet -m "chore(provenance): bind pawl verdict (age-8ais-rk3r5-tip) #trivial"
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO" HEAD_SHA BASE_SHA FEAT_SHA
}

@test "check-pawl-pre-push: PURE-#trivial range takes the fast path, gate NOT invoked (age-8ais)" {
  make_repo_two_trivial_range
  # A gate command that would leave a marker + fail loudly IF ever invoked.
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main %s\n' "$HEAD_SHA" "$BASE_SHA" \
    | env AGENTOPS_PREPUSH_GATE_CMD="touch $TMP/GATE_INVOKED; false" bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 0 ]
  [ ! -f "$TMP/GATE_INVOKED" ]                 # fast path: the gate was never run
  [[ "$output" == *"pawl waived"* ]]
  [[ "$output" != *"re-targeting"* ]]
}

@test "check-pawl-pre-push: MIXED range with a #trivial tip re-targets the gate at the non-trivial commit; PASS allows (age-8ais)" {
  make_repo_mixed_range "feat(x): real change (age-8ais-pass)" "chore(provenance): edge (age-8ais-pass-tip) #trivial"
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main %s\n' "$HEAD_SHA" "$BASE_SHA" \
    | env AGENTOPS_PREPUSH_GATE_CMD="git rev-parse HEAD > $TMP/gated_head; true" bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 0 ]
  [[ "$output" == *"re-targeting the cockpit gate"* ]]
  [[ "$output" == *"cockpit gate PASSED"* ]]
  [[ "$output" == *"push authorized"* ]]
  # The gate ran against the NON-trivial feat commit, not the #trivial tip.
  [ "$(cat "$TMP/gated_head")" = "$FEAT_SHA" ]
  [ "$(cat "$TMP/gated_head")" != "$HEAD_SHA" ]
}

@test "check-pawl-pre-push: MIXED range with a #trivial tip re-targets the gate; FAIL blocks the push (age-8ais)" {
  make_repo_mixed_range "feat(x): real change (age-8ais-fail)" "chore(provenance): edge (age-8ais-fail-tip) #trivial"
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main %s\n' "$HEAD_SHA" "$BASE_SHA" \
    | env AGENTOPS_PREPUSH_GATE_CMD="git rev-parse HEAD > $TMP/gated_head; false" bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 1 ]
  [[ "$output" == *"cockpit gate FAILED"* ]]
  [[ "$output" == *"push refused"* ]]
  [ "$(cat "$TMP/gated_head")" = "$FEAT_SHA" ]
}

@test "check-pawl-pre-push: BLOCKS the rk3r.5 shape — raw os.Setenv in *_test.go behind a #trivial tip (age-8ais)" {
  make_repo_rk3r5_shape
  # Stand in for the test-isolation ratchet: FAIL when the re-targeted worktree
  # HEAD introduces a raw os.Setenv in a *_test.go — the exact escape. The mock
  # inspects the worktree it is run in, so it only fails if re-targeting exposed
  # the hidden feat commit (a tip-scoped run would see only docs/provenance/).
  local ratchet='if git show --format= --name-only HEAD | grep -q "_test\.go$" && git show HEAD | grep -q "os\.Setenv("; then echo "ratchet: raw os.Setenv added in a *_test.go" >&2; exit 1; fi; exit 0'
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main %s\n' "$HEAD_SHA" "$BASE_SHA" \
    | env AGENTOPS_PREPUSH_GATE_CMD="$ratchet" bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 1 ]
  [[ "$output" == *"ratchet: raw os.Setenv"* ]]
  [[ "$output" == *"cockpit gate FAILED"* ]]
  [[ "$output" == *"push refused"* ]]
}

@test "check-pawl-pre-push: a NON-trivial TIP is unchanged — the range gate never fires (age-8ais)" {
  # Acceptance #3: when the tip itself is non-trivial, the flow is the normal
  # pawl-verdict path; the new re-targeted gate must NOT be invoked.
  make_repo_with_commit age-8ais-nttip
  seed_verdict age-8ais-nttip "$HEAD_SHA"
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" \
    | env AGENTOPS_PREPUSH_GATE_CMD="touch $TMP/GATE_INVOKED; false" bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 0 ]
  [ ! -f "$TMP/GATE_INVOKED" ]                 # range gate never fires for a non-trivial tip
  [[ "$output" == *"push authorized"* ]]
  [[ "$output" != *"re-targeting"* ]]
}

# Cross-family counterexample (REFUTED the newest-only re-target): the common
# landing-train shape pushes [feat1, #trivial-bind, feat2, #trivial-bind] in one
# range. When the OLDER substantive commit (feat1) carries the violation and the
# newer one (feat2) is clean, gating only the NEWEST non-trivial commit PASSES
# and the violation lands. EVERY non-trivial commit must gate, newest-first,
# fail-fast.
#
# init(base) → OLDER substantive w/ raw os.Setenv in a *_test.go → #trivial bind
# → NEWER clean substantive → #trivial provenance-only tip.
make_repo_multi_substantive_older_violation() {
  REPO="$TMP/repo"
  rm -rf "$REPO"
  mkdir -p "$REPO"
  cd "$REPO"
  git init --quiet
  git config user.email test@example.com
  git config user.name Test
  echo ok > README.md
  git add README.md
  git commit --quiet -m "init"
  BASE_SHA="$(git rev-parse HEAD)"
  mkdir -p cli/cmd/ao
  printf 'package main\n\nimport (\n\t"os"\n\t"testing"\n)\n\nfunc TestFoo(t *testing.T) {\n\tos.Setenv("X", "1")\n\t_ = t\n}\n' > cli/cmd/ao/foo_test.go
  git add cli/cmd/ao/foo_test.go
  git commit --quiet -m "feat(ao): OLDER substantive with raw os.Setenv (age-8ais-multi-old)"
  OLD_SHA="$(git rev-parse HEAD)"
  mkdir -p docs/provenance
  echo '{"edge":1}' >> docs/provenance/ledger.jsonl
  git add docs/provenance/ledger.jsonl
  git commit --quiet -m "chore(provenance): bind pawl verdict for multi-old (age-8ais-multi-bind1) #trivial"
  echo clean-change >> README.md
  git add README.md
  git commit --quiet -m "feat(x): NEWER clean substantive (age-8ais-multi-new)"
  NEW_SHA="$(git rev-parse HEAD)"
  echo '{"edge":2}' >> docs/provenance/ledger.jsonl
  git add docs/provenance/ledger.jsonl
  git commit --quiet -m "chore(provenance): bind pawl verdict for multi-new (age-8ais-multi-tip) #trivial"
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO" HEAD_SHA BASE_SHA OLD_SHA NEW_SHA
}

@test "check-pawl-pre-push: BLOCKS when the OLDER of two substantive commits carries the violation behind a #trivial tip (age-8ais counterexample)" {
  make_repo_multi_substantive_older_violation
  # Same ratchet stand-in as the rk3r.5 test, plus an order recorder: each gated
  # worktree appends its HEAD, proving EVERY non-trivial commit is gated
  # newest-first and fail-fast stops at the older violation.
  local ratchet='git rev-parse HEAD >> '"$TMP"'/gated_order; if git show --format= --name-only HEAD | grep -q "_test\.go$" && git show HEAD | grep -q "os\.Setenv("; then echo "ratchet: raw os.Setenv added in a *_test.go" >&2; exit 1; fi; exit 0'
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main %s\n' "$HEAD_SHA" "$BASE_SHA" \
    | env AGENTOPS_PREPUSH_GATE_CMD="$ratchet" bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 1 ]
  [[ "$output" == *"ratchet: raw os.Setenv"* ]]
  # The NEWER clean substantive commit gated first and PASSED…
  [[ "$output" == *"cockpit gate PASSED for non-trivial commit ${NEW_SHA:0:12}"* ]]
  # …then the OLDER one gated, FAILED, and the push was refused NAMING it.
  [[ "$output" == *"cockpit gate FAILED for non-trivial commit ${OLD_SHA:0:12}"* ]]
  [[ "$output" == *"push refused"* ]]
  [[ "$output" != *"push authorized"* ]]
  # Order + fail-fast: exactly [NEW, OLD] — nothing gated after the failure,
  # and the #trivial commits were never gated.
  [ "$(wc -l < "$TMP/gated_order" | tr -d ' ')" = "2" ]
  [ "$(sed -n '1p' "$TMP/gated_order")" = "$NEW_SHA" ]
  [ "$(sed -n '2p' "$TMP/gated_order")" = "$OLD_SHA" ]
}
