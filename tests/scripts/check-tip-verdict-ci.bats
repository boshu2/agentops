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

# ============================================================================
# REBOUND authorization (age-rk3r.18) — the CI backstop honors a committed
# REBOUND verdict edge ONLY after lineage + byte-equivalence RE-VALIDATION (a
# committed CONFIRMED-reviewed commit byte-equivalent to the tip must exist,
# proven via the shared scripts/lib/diff-identity.sh). A bare
# disposition=REBOUND is NEVER accepted. CI runs INSIDE THE TRUSTED REPO, so it
# uses shell + the shared lib; the Go gate is the hostile-repo-safe twin.
# ============================================================================

# bind_rebound <bead> <sha> — seal a REBOUND verdict edge onto the fixture
# ledger with the real chain writer (the exact committed shape emit-verdict
# emits for a rebind: only disposition=REBOUND + to_id ride the edge).
bind_rebound() {
  local bead="$1" sha="$2"
  ( cd "$REPO" && "$AO_BIN" provenance add "${bead}@${sha:0:7}" "$sha" \
      --relation wasDerivedFrom --from-type verdict --to-type commit \
      --trust-tier inferred \
      --evidence "pawl-verdict $bead disposition=REBOUND" >/dev/null )
}

# make_rebound_repo <change> — build the canonical REBOUND scenario:
#   README -> shared base app.go -> reviewed R (applies <change>) on main, with a
#   CONFIRMED verdict bound to R; a branch `rebound` off the shared base applies
#   the SAME <change> (byte-identical) as tip C, with a REBOUND verdict bound to C;
#   the ledger committed on `rebound` as a #trivial bind. Sets REBOUND_BASE (the
#   shared base, the push range base), REVIEWED_R, TIP_C, BOUND (the pushed tip).
make_rebound_repo() {
  local change="$1"
  make_repo                                    # README + BASE_SHA
  # Shared base of app.go (the branch point).
  printf 'package main\n' > "$REPO/app.go"
  git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "chore: base app.go"
  REBOUND_BASE="$(git -C "$REPO" rev-parse HEAD)"
  # Reviewed R on main.
  printf '%s' "$change" > "$REPO/app.go"
  git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: the change (reviewed)"
  REVIEWED_R="$(git -C "$REPO" rev-parse HEAD)"
  bind_verdict age-rev "$REVIEWED_R"
  # Rebound tip C off the SHARED base (byte-identical change, distinct sha).
  git -C "$REPO" checkout -q -f -b rebound "$REBOUND_BASE"
  printf '%s' "$change" > "$REPO/app.go"
  git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: the change (rebound)"
  TIP_C="$(git -C "$REPO" rev-parse HEAD)"
  bind_rebound age-reb "$TIP_C"
  BOUND="$(git -C "$REPO" rev-parse HEAD)"
}

@test "REBOUND with a byte-equivalent CONFIRMED lineage is authorized (report mode, no warnings)" {
  require_ao
  make_rebound_repo 'package main

func F() {}
'
  [ "$TIP_C" != "$REVIEWED_R" ]
  run_backstop --repo "$REPO" --base "$REBOUND_BASE" --head "$BOUND"
  [ "$status" -eq 0 ]
  [[ "$output" == *"REBOUND authorized"* ]]
  [[ "$output" != *"::warning::"* ]]
  [[ "$output" != *"::error::"* ]]
}

@test "REBOUND with a byte-equivalent CONFIRMED lineage is authorized in enforce mode too" {
  require_ao
  make_rebound_repo 'package main

func F() {}
'
  run_backstop --repo "$REPO" --base "$REBOUND_BASE" --head "$BOUND" --enforce
  [ "$status" -eq 0 ]
  [[ "$output" == *"REBOUND authorized"* ]]
}

# HONEST SCOPING (age-rk3r.18): in a CLEAN CI clone that fetched only C's branch
# (a rebase orphaned R on another branch), the reviewed commit R is UNREACHABLE, so
# CI cannot independently re-verify the byte-equivalence. It must REFUSE fail-closed
# with a DISTINCT, accurate message — NOT the misleading "lacks a bound verdict".
# RED-first: the pre-fix CI emitted "lacks a bound verdict" here.
@test "REBOUND in a clean clone where the reviewed commit is ORPHANED: distinct message + fail-closed" {
  require_ao
  # Build the full repo (R on main, C on 'rebound'). make_rebound_repo leaves the
  # ledger edges UNCOMMITTED in the working tree (the reachable tests read the
  # working-tree file), so for a CLONE-based test — a clone carries only COMMITTED
  # history — COMMIT the ledger on 'rebound' so both edges reach the clone.
  make_rebound_repo 'package main

func F() {}
'
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind REBOUND + lineage #trivial"
  BOUND="$(git -C "$REPO" rev-parse HEAD)"
  # Simulate the CI checkout: clone ONLY the 'rebound' branch into a fresh repo, so
  # R (main's tip) is NOT fetched — its object is unreachable in the clone. --no-local
  # forces the pack-based (network-style) transfer that copies only REACHABLE objects
  # (a local clone hardlinks the whole object DB, leaving R present as a loose object
  # — which would mask the orphaning this test needs). A bare intermediate mirror
  # keeps the single-branch fetch honest.
  BARE="$TMP/bare.git"
  git clone --quiet --bare "$REPO" "$BARE"
  CLONE="$TMP/ci-clone"
  git clone --quiet --no-local --single-branch --branch rebound "$BARE" "$CLONE"
  # Preconditions: C is reachable in the clone; R is NOT; the committed ledger carries both edges.
  git -C "$CLONE" cat-file -e "${TIP_C}^{commit}"           # C present
  run git -C "$CLONE" cat-file -e "${REVIEWED_R}^{commit}"
  [ "$status" -ne 0 ]                                        # R absent (orphaned)
  grep -q "disposition=CONFIRMED" "$CLONE/docs/provenance/ledger.jsonl"  # lineage edge present
  grep -q "disposition=REBOUND" "$CLONE/docs/provenance/ledger.jsonl"    # REBOUND edge present
  CLONE_TIP="$(git -C "$CLONE" rev-parse HEAD)"
  [ "$CLONE_TIP" = "$BOUND" ]

  # Run the backstop from the CLEAN clone (base = the shared base, which IS in the clone).
  run_backstop --repo "$CLONE" --base "$REBOUND_BASE" --head "$CLONE_TIP" --enforce
  [ "$status" -ne 0 ]                                        # fail-closed
  # DISTINCT, accurate message — names the unreachable reviewed commit, NOT "lacks a bound verdict".
  [[ "$output" == *"REBOUND: the reviewed commit it descends from is NOT reachable in this checkout"* ]]
  [[ "$output" == *"NOT authorized (fail-closed)"* ]]
  [[ "$output" != *"lacks a bound verdict"* ]]
}

# ============================================================================
# KEEP-REF (age-rk3r.19) — CI re-verifies an ORPHANED reviewed commit by fetching
# refs/agentops/rebound/<C> from origin, then re-deriving byte-equivalence against
# the LEDGER-named R. The write side (pawl-verdict.sh do_rebind_verified) pushes
# this ref; here we simulate it landing on the remote by writing it into the bare
# mirror the CI clone fetches from.
# ============================================================================

# setup_orphaned_rebound_clone <keepref-mode> — build the full REBOUND repo (R on
# main, C on 'rebound', ledger committed), a bare mirror, and a single-branch CI clone
# that ORPHANS R. <keepref-mode> selects what keep-ref refs/agentops/rebound/<C> is
# planted INTO the bare mirror (what do_rebind_verified's push would put on the remote):
#   real  — pin the ACTUAL reviewed R (the honest write-side behavior).
#   none  — plant nothing (the age-rk3r.18 baseline: fall back to fail-closed).
# The keep-ref is planted AFTER make_rebound_repo sets REVIEWED_R/TIP_C (they are only
# known then). Sets CLONE, CLONE_TIP, TIP_C, REVIEWED_R, REBOUND_BASE, BOUND, BARE.
setup_orphaned_rebound_clone() {
  local mode="${1:?keepref-mode required (real|none)}"
  make_rebound_repo 'package main

func F() {}
'
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind REBOUND + lineage #trivial"
  BOUND="$(git -C "$REPO" rev-parse HEAD)"
  BARE="$TMP/bare.git"
  git clone --quiet --bare "$REPO" "$BARE"
  # Plant the keep-ref on the remote (what do_rebind_verified's push would do).
  case "$mode" in
    real) git -C "$BARE" update-ref "refs/agentops/rebound/${TIP_C}" "$REVIEWED_R" ;;
    none) : ;;
    *) echo "bad keepref-mode: $mode" >&2; return 1 ;;
  esac
  CLONE="$TMP/ci-clone"
  git clone --quiet --no-local --single-branch --branch rebound "$BARE" "$CLONE"
  CLONE_TIP="$(git -C "$CLONE" rev-parse HEAD)"
  # Preconditions shared by every keep-ref test: C present, R orphaned, both edges committed.
  git -C "$CLONE" cat-file -e "${TIP_C}^{commit}"
  run git -C "$CLONE" cat-file -e "${REVIEWED_R}^{commit}"
  [ "$status" -ne 0 ]
  [ "$CLONE_TIP" = "$BOUND" ]
}

# RED->GREEN (age-rk3r.19): the SAME orphaned-R clean-clone case that age-rk3r.18
# REFUSED (exit 2) now AUTHORIZES when the keep-ref pins the ledger-named R on the
# remote — CI fetches it, confirms the fetched object is the ledger's R, re-derives
# byte-equivalence, and authorizes.
@test "KEEP-REF: orphaned reviewed commit + a keep-ref pinning the LEDGER R -> CI fetches, re-verifies, AUTHORIZES" {
  require_ao
  setup_orphaned_rebound_clone real            # keep-ref -> the real reviewed R
  run_backstop --repo "$CLONE" --base "$REBOUND_BASE" --head "$CLONE_TIP" --enforce
  [ "$status" -eq 0 ]                                        # AUTHORIZED (was exit-2 refuse pre-fix)
  [[ "$output" == *"fetched keep-ref refs/agentops/rebound/${TIP_C:0:12}"* ]]
  [[ "$output" == *"REBOUND authorized"* ]]
  [[ "$output" != *"NOT reachable in this checkout"* ]]
  # And R is now genuinely present in the clone (fetched via the keep-ref).
  git -C "$CLONE" cat-file -e "${REVIEWED_R}^{commit}"
}

# UNCHANGED (age-rk3r.18 baseline preserved): orphaned R with NO keep-ref on the
# remote still refuses fail-closed with the distinct message.
@test "KEEP-REF: orphaned reviewed commit + NO keep-ref -> still fail-closed exit-2 (unchanged)" {
  require_ao
  setup_orphaned_rebound_clone none            # no keep-ref planted
  run_backstop --repo "$CLONE" --base "$REBOUND_BASE" --head "$CLONE_TIP" --enforce
  [ "$status" -ne 0 ]                                        # fail-closed
  [[ "$output" == *"REBOUND: the reviewed commit it descends from is NOT reachable in this checkout"* ]]
  [[ "$output" == *"NOT authorized (fail-closed)"* ]]
  [[ "$output" != *"REBOUND authorized"* ]]
}

# FORGE (age-rk3r.19): a keep-ref exists but points at a WRONG commit R' (not the
# ledger's R) — EVEN one whose diff is byte-equivalent to C. It must be REFUSED: CI
# re-derives against the LEDGER-named R, which is STILL unreachable (the keep-ref made
# R' present, not R), so the scan stays exit-2 fail-closed. A forged keep-ref launders
# nothing.
@test "FORGE KEEP-REF: keep-ref points at a WRONG (byte-equivalent) R' not the ledger R -> REFUSED" {
  require_ao
  # Build the standard orphaned-R scenario but ALSO create a decoy R' whose diff equals C.
  make_rebound_repo 'package main

func F() {}
'
  # R' = a SECOND commit off the shared base applying the identical change (byte-equivalent
  # to C, distinct sha, and NOT the ledger's CONFIRMED to_id). Park it on its own branch so
  # it is a real, fetchable object but not on 'rebound'.
  git -C "$REPO" checkout -q -f -b decoy "$REBOUND_BASE"
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"
  git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: byte-equivalent decoy R-prime"
  local RPRIME; RPRIME="$(git -C "$REPO" rev-parse HEAD)"
  [ "$RPRIME" != "$REVIEWED_R" ]
  git -C "$REPO" checkout -q -f rebound
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind REBOUND + lineage #trivial"
  BOUND="$(git -C "$REPO" rev-parse HEAD)"
  BARE="$TMP/bare.git"
  git clone --quiet --bare "$REPO" "$BARE"
  # The FORGED keep-ref points at R' (the decoy), NOT the ledger's R.
  git -C "$BARE" update-ref "refs/agentops/rebound/${TIP_C}" "$RPRIME"
  CLONE="$TMP/ci-clone"
  git clone --quiet --no-local --single-branch --branch rebound "$BARE" "$CLONE"
  CLONE_TIP="$(git -C "$CLONE" rev-parse HEAD)"
  # R (the ledger's CONFIRMED to_id) is orphaned; R' is what the forged keep-ref pins.
  run git -C "$CLONE" cat-file -e "${REVIEWED_R}^{commit}"; [ "$status" -ne 0 ]
  run_backstop --repo "$CLONE" --base "$REBOUND_BASE" --head "$CLONE_TIP" --enforce
  [ "$status" -ne 0 ]                                        # REFUSED
  [[ "$output" != *"REBOUND authorized"* ]]
  # The ledger-named R is STILL unreachable after fetching R', so the distinct exit-2 fires.
  [[ "$output" == *"NOT reachable in this checkout"* ]]
}

# FORGE (age-rk3r.19): the keep-ref DOES pin the ledger's real R, but R's diff is NOT
# byte-equivalent to C (a genuine non-equivalent lineage). After fetching R, CI re-derives
# and finds no equivalence -> REFUSED as a plain no-proof ("lacks a bound verdict").
@test "FORGE KEEP-REF: keep-ref pins the real R but R is NOT byte-equivalent to C -> REFUSED" {
  require_ao
  make_repo
  printf 'package main\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "chore: base app.go"
  REBOUND_BASE="$(git -C "$REPO" rev-parse HEAD)"
  # Reviewed R on main applies func F.
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: change F (reviewed)"
  REVIEWED_R="$(git -C "$REPO" rev-parse HEAD)"; bind_verdict age-rev "$REVIEWED_R"
  # Tip C on 'rebound' applies a DIFFERENT change (func G) — NOT equivalent to R.
  git -C "$REPO" checkout -q -f -b rebound "$REBOUND_BASE"
  printf 'package main\n\nfunc G() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: change G (non-equivalent tip)"
  TIP_C="$(git -C "$REPO" rev-parse HEAD)"; bind_rebound age-reb "$TIP_C"
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind REBOUND + lineage #trivial"
  BOUND="$(git -C "$REPO" rev-parse HEAD)"
  BARE="$TMP/bare.git"
  git clone --quiet --bare "$REPO" "$BARE"
  # Keep-ref pins the REAL ledger R (honest pin) — but R is not equivalent to C.
  git -C "$BARE" update-ref "refs/agentops/rebound/${TIP_C}" "$REVIEWED_R"
  CLONE="$TMP/ci-clone"
  git clone --quiet --no-local --single-branch --branch rebound "$BARE" "$CLONE"
  CLONE_TIP="$(git -C "$CLONE" rev-parse HEAD)"
  run git -C "$CLONE" cat-file -e "${REVIEWED_R}^{commit}"; [ "$status" -ne 0 ]  # R orphaned pre-fetch
  run_backstop --repo "$CLONE" --base "$REBOUND_BASE" --head "$CLONE_TIP" --enforce
  [ "$status" -ne 0 ]                                        # REFUSED
  [[ "$output" != *"REBOUND authorized"* ]]
  # R fetched + reachable, but non-equivalent -> genuine no-proof, not the unreachable message.
  [[ "$output" == *"lacks a bound verdict"* ]]
  git -C "$CLONE" cat-file -e "${REVIEWED_R}^{commit}"       # R IS now present (keep-ref fetched)
}

@test "FORGE: a REBOUND whose tip is NOT byte-equivalent to any CONFIRMED is REFUSED (enforce)" {
  require_ao
  make_repo
  printf 'package main\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "chore: base app.go"
  SHARED="$(git -C "$REPO" rev-parse HEAD)"
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: change F (reviewed)"
  R="$(git -C "$REPO" rev-parse HEAD)"; bind_verdict age-rev "$R"
  # Rebound tip applies a DIFFERENT change (func G) — not equivalent to F.
  git -C "$REPO" checkout -q -f -b rebound "$SHARED"
  printf 'package main\n\nfunc G() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: change G (forged rebound)"
  C="$(git -C "$REPO" rev-parse HEAD)"; bind_rebound age-reb "$C"
  BND="$(git -C "$REPO" rev-parse HEAD)"
  run_backstop --repo "$REPO" --base "$SHARED" --head "$BND" --enforce
  [ "$status" -ne 0 ]
  [[ "$output" == *"::warning::commit ${C:0:12} lacks a bound verdict"* ]]
  [[ "$output" == *"::error::"*"lack proof"* ]]
}

@test "FORGE: a REBOUND whose lineage is NOT CONFIRMED (REFUTED) is REFUSED (enforce)" {
  require_ao
  make_repo
  printf 'package main\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "chore: base app.go"
  SHARED="$(git -C "$REPO" rev-parse HEAD)"
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: the change (reviewed)"
  R="$(git -C "$REPO" rev-parse HEAD)"
  # The ONLY verdict on R is REFUTED, not CONFIRMED.
  ( cd "$REPO" && "$AO_BIN" provenance add "age-rev@${R:0:7}" "$R" \
      --relation wasDerivedFrom --from-type verdict --to-type commit \
      --trust-tier inferred --evidence "pawl-verdict age-rev disposition=REFUTED" >/dev/null )
  git -C "$REPO" checkout -q -f -b rebound "$SHARED"
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: the change (rebound)"
  C="$(git -C "$REPO" rev-parse HEAD)"; bind_rebound age-reb "$C"
  BND="$(git -C "$REPO" rev-parse HEAD)"
  run_backstop --repo "$REPO" --base "$SHARED" --head "$BND" --enforce
  [ "$status" -ne 0 ]
  [[ "$output" == *"::warning::commit ${C:0:12} lacks a bound verdict"* ]]
}

@test "FORGE: a bare REBOUND edge with NO CONFIRMED lineage anywhere is REFUSED (enforce)" {
  require_ao
  make_repo
  add_code_commit "feat: unreviewed change"; C1="$LAST_SHA"
  bind_rebound age-reb "$C1"
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind bare REBOUND #trivial"
  BND="$(git -C "$REPO" rev-parse HEAD)"
  run_backstop --repo "$REPO" --base "$BASE_SHA" --head "$BND" --enforce
  [ "$status" -ne 0 ]
  [[ "$output" == *"::warning::commit ${C1:0:12} lacks a bound verdict"* ]]
}

# bind_confirmed_raw_toid <bead> <raw-to-id> — seal a CONFIRMED verdict edge whose
# to_id is an ARBITRARY raw string (the crafted-lineage attack: a REVISION
# EXPRESSION like "HEAD~1", not a hex commit id). `ao provenance add` persists it
# (the writer only requires to_id non-empty), which is exactly why the GATE must
# enforce the hex-commit-object discipline.
bind_confirmed_raw_toid() {
  local bead="$1" raw="$2"
  ( cd "$REPO" && "$AO_BIN" provenance add "${bead}@fakehed" "$raw" \
      --relation wasDerivedFrom --from-type verdict --to-type commit \
      --trust-tier inferred \
      --evidence "pawl-verdict $bead disposition=CONFIRMED" >/dev/null )
}

@test "FORGE: a REBOUND whose CONFIRMED lineage to_id is a REVISION EXPRESSION (HEAD~1) is REFUSED (enforce)" {
  # The refuter's exact repro on the CI path: a crafted CONFIRMED edge to_id=HEAD~1
  # (a ref alias, not a hex id) + a REBOUND on the unreviewed tip whose diff matches
  # HEAD~1's. The pre-fix CI fed HEAD~1 to git and certified the tip fail-open.
  require_ao
  make_repo
  printf 'package main\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "chore: base app.go"
  SHARED="$(git -C "$REPO" rev-parse HEAD)"
  # Unreviewed tip C off the shared base (byte-equivalent to HEAD~1 of the bind line).
  git -C "$REPO" checkout -q -f -b rebound "$SHARED"
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: the change (unreviewed C)"
  C="$(git -C "$REPO" rev-parse HEAD)"
  # Crafted CONFIRMED lineage to_id = HEAD~1 (resolves to a byte-equivalent commit)
  # + a REBOUND bound to the unreviewed tip C.
  bind_confirmed_raw_toid age-fake "HEAD~1"
  bind_rebound age-reb "$C"
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind ref-alias forge #trivial"
  BND="$(git -C "$REPO" rev-parse HEAD)"
  run_backstop --repo "$REPO" --base "$SHARED" --head "$BND" --enforce
  [ "$status" -ne 0 ]
  [[ "$output" == *"::warning::commit ${C:0:12} lacks a bound verdict"* ]]
  [[ "$output" == *"::error::"*"lack proof"* ]]
}

@test "FORGE: a REBOUND whose CONFIRMED lineage to_id is non-hex junk is REFUSED (enforce)" {
  require_ao
  make_repo
  printf 'package main\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "chore: base app.go"
  SHARED="$(git -C "$REPO" rev-parse HEAD)"
  git -C "$REPO" checkout -q -f -b rebound "$SHARED"
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: the change (unreviewed C)"
  C="$(git -C "$REPO" rev-parse HEAD)"
  bind_confirmed_raw_toid age-fake "zzzzzzzzzzzz"
  bind_rebound age-reb "$C"
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind junk-lineage forge #trivial"
  BND="$(git -C "$REPO" rev-parse HEAD)"
  run_backstop --repo "$REPO" --base "$SHARED" --head "$BND" --enforce
  [ "$status" -ne 0 ]
  [[ "$output" == *"::warning::commit ${C:0:12} lacks a bound verdict"* ]]
}

# bind_edge_relation <bead> <sha> <disposition> <relation> — seal a verdict edge
# with an ARBITRARY relation (the wrong-relation attack: a chain-valid edge whose
# relation is NOT wasDerivedFrom, e.g. wasAttributedTo). The Go gate requires
# relation==wasDerivedFrom EXACTLY; CI must match, or a wrong-relation edge would
# be accepted as a verdict authorization.
bind_edge_relation() {
  local bead="$1" sha="$2" disp="$3" rel="$4"
  ( cd "$REPO" && "$AO_BIN" provenance add "${bead}@${sha:0:7}" "$sha" \
      --relation "$rel" --from-type verdict --to-type commit \
      --trust-tier inferred \
      --evidence "pawl-verdict $bead disposition=$disp" >/dev/null )
}

@test "FORGE: a REBOUND edge with the WRONG relation (wasAttributedTo, not wasDerivedFrom) is REFUSED (enforce)" {
  # Go/CI parity: the REBOUND edge itself must be relation==wasDerivedFrom. A
  # chain-valid edge carrying disposition=REBOUND but relation=wasAttributedTo is
  # NOT a valid verdict edge — the Go gate rejects it (reboundEdgeBoundTo requires
  # the exact relation); CI must too. Byte-equivalent CONFIRMED lineage is present,
  # so only the wrong-relation REBOUND edge stands between the tip and authorization.
  require_ao
  make_repo
  printf 'package main\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "chore: base app.go"
  SHARED="$(git -C "$REPO" rev-parse HEAD)"
  # Genuinely-reviewed R on main (valid wasDerivedFrom CONFIRMED lineage).
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: the change (reviewed R)"
  R="$(git -C "$REPO" rev-parse HEAD)"; bind_verdict age-rev "$R"
  # Rebound tip C off the shared base (byte-equivalent to R) with a WRONG-RELATION
  # REBOUND edge (wasAttributedTo instead of wasDerivedFrom).
  git -C "$REPO" checkout -q -f -b rebound "$SHARED"
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: the change (rebound C)"
  C="$(git -C "$REPO" rev-parse HEAD)"
  bind_edge_relation age-reb "$C" REBOUND wasAttributedTo
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind wrong-relation REBOUND forge #trivial"
  BND="$(git -C "$REPO" rev-parse HEAD)"
  run_backstop --repo "$REPO" --base "$SHARED" --head "$BND" --enforce
  [ "$status" -ne 0 ]
  [[ "$output" == *"::warning::commit ${C:0:12} lacks a bound verdict"* ]]
  [[ "$output" == *"::error::"*"lack proof"* ]]
}

@test "FORGE: a REBOUND whose CONFIRMED lineage edge has the WRONG relation is REFUSED (enforce)" {
  # The mirror: the REBOUND edge is correct (wasDerivedFrom), but the ONLY
  # byte-equivalent CONFIRMED lineage candidate carries relation=wasAttributedTo,
  # so it is NOT a valid lineage root — no authorizing CONFIRMED exists. The Go
  # gate (confirmedVerdictCommitSHAs requires wasDerivedFrom) refuses; CI must too.
  require_ao
  make_repo
  printf 'package main\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "chore: base app.go"
  SHARED="$(git -C "$REPO" rev-parse HEAD)"
  # R on main with a WRONG-RELATION "CONFIRMED" edge (wasAttributedTo) — not a
  # valid lineage root.
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: the change (reviewed R)"
  R="$(git -C "$REPO" rev-parse HEAD)"
  bind_edge_relation age-rev "$R" CONFIRMED wasAttributedTo
  # Rebound tip C off the shared base (byte-equivalent to R) with a correct
  # wasDerivedFrom REBOUND edge.
  git -C "$REPO" checkout -q -f -b rebound "$SHARED"
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: the change (rebound C)"
  C="$(git -C "$REPO" rev-parse HEAD)"; bind_rebound age-reb "$C"
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind wrong-relation lineage forge #trivial"
  BND="$(git -C "$REPO" rev-parse HEAD)"
  run_backstop --repo "$REPO" --base "$SHARED" --head "$BND" --enforce
  [ "$status" -ne 0 ]
  [[ "$output" == *"::warning::commit ${C:0:12} lacks a bound verdict"* ]]
}

@test "PARITY: a wrong-relation CONFIRMED edge (wasAttributedTo) does NOT satisfy the direct verdict check either (enforce)" {
  # The direct-CONFIRMED path (verdict_event_for) must ALSO require wasDerivedFrom,
  # matching the Go confirmedVerdictEdgeIn — a chain-valid CONFIRMED edge with the
  # wrong relation must not certify a plain (non-REBOUND) commit.
  require_ao
  make_repo
  add_code_commit "feat: a normal change (age-x)"; C1="$LAST_SHA"
  bind_edge_relation age-x "$C1" CONFIRMED wasAttributedTo
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind wrong-relation CONFIRMED #trivial"
  BND="$(git -C "$REPO" rev-parse HEAD)"
  run_backstop --repo "$REPO" --base "$BASE_SHA" --head "$BND" --enforce
  [ "$status" -ne 0 ]
  [[ "$output" == *"::warning::commit ${C1:0:12} lacks a bound verdict"* ]]
}

# bind_edge_evidence <bead> <sha> <evidence-string> — seal a verdict edge with an
# ARBITRARY evidence_ref (the disposition-token attack: a multi-`disposition=`
# evidence string). Go's parseDisposition is a FIRST-token contract, so
# "disposition=REFUTED disposition=REBOUND" reads as REFUTED (Go rejects a REBOUND
# read). A CI selector matching "any disposition= token present" would wrongly
# authorize — this helper lets the test prove CI now matches Go (first-token wins).
bind_edge_evidence() {
  local bead="$1" sha="$2" ev="$3"
  ( cd "$REPO" && "$AO_BIN" provenance add "${bead}@${sha:0:7}" "$sha" \
      --relation wasDerivedFrom --from-type verdict --to-type commit \
      --trust-tier inferred --evidence "$ev" >/dev/null )
}

@test "FORGE: a REBOUND edge whose FIRST disposition token is REFUTED (disposition=REFUTED disposition=REBOUND) is REFUSED (enforce)" {
  # The refuter's disposition-token repro. Go parseDisposition returns the FIRST
  # disposition= token (REFUTED), so the Go gate does NOT read this as a REBOUND and
  # REJECTS. CI must match: an "any-token-present" selector would authorize (the bug).
  # Byte-equivalent CONFIRMED lineage is present, so only the first-token contract
  # stands between this tip and a false authorization.
  require_ao
  make_repo
  printf 'package main\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "chore: base app.go"
  SHARED="$(git -C "$REPO" rev-parse HEAD)"
  # Genuinely-reviewed R on main (valid CONFIRMED lineage).
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: the change (reviewed R)"
  R="$(git -C "$REPO" rev-parse HEAD)"; bind_verdict age-rev "$R"
  # Rebound tip C off the shared base (byte-equivalent to R) with a double-disposition
  # edge whose FIRST token is REFUTED.
  git -C "$REPO" checkout -q -f -b rebound "$SHARED"
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: the change (rebound C)"
  C="$(git -C "$REPO" rev-parse HEAD)"
  bind_edge_evidence age-reb "$C" "pawl-verdict age-reb disposition=REFUTED disposition=REBOUND"
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind double-disposition REBOUND forge #trivial"
  BND="$(git -C "$REPO" rev-parse HEAD)"
  run_backstop --repo "$REPO" --base "$SHARED" --head "$BND" --enforce
  [ "$status" -ne 0 ]
  [[ "$output" == *"::warning::commit ${C:0:12} lacks a bound verdict"* ]]
  [[ "$output" == *"::error::"*"lack proof"* ]]
}

@test "FORGE: a REBOUND whose CONFIRMED lineage FIRST disposition token is REFUTED is REFUSED (enforce)" {
  # The mirror: the REBOUND edge is a correct single-token REBOUND, but the ONLY
  # byte-equivalent lineage candidate's evidence_ref is "disposition=REFUTED
  # disposition=CONFIRMED" — Go reads its FIRST token (REFUTED), so it is NOT a valid
  # CONFIRMED lineage root. CI must match (first-token), not accept it via any-token.
  require_ao
  make_repo
  printf 'package main\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "chore: base app.go"
  SHARED="$(git -C "$REPO" rev-parse HEAD)"
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: the change (reviewed R)"
  R="$(git -C "$REPO" rev-parse HEAD)"
  bind_edge_evidence age-rev "$R" "pawl-verdict age-rev disposition=REFUTED disposition=CONFIRMED"
  git -C "$REPO" checkout -q -f -b rebound "$SHARED"
  printf 'package main\n\nfunc F() {}\n' > "$REPO/app.go"; git -C "$REPO" add app.go
  git -C "$REPO" commit --quiet -m "feat: the change (rebound C)"
  C="$(git -C "$REPO" rev-parse HEAD)"; bind_rebound age-reb "$C"
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind double-disposition lineage forge #trivial"
  BND="$(git -C "$REPO" rev-parse HEAD)"
  run_backstop --repo "$REPO" --base "$SHARED" --head "$BND" --enforce
  [ "$status" -ne 0 ]
  [[ "$output" == *"::warning::commit ${C:0:12} lacks a bound verdict"* ]]
}

@test "PARITY: a direct CONFIRMED edge with disposition=CONFIRMEDLY (near-miss token) does NOT certify (enforce)" {
  # The direct-CONFIRMED path must match Go's EXACT disposition value, not a
  # substring/prefix: "disposition=CONFIRMEDLY" yields value CONFIRMEDLY (!= CONFIRMED)
  # in both Go parseDisposition and the CI dispvalue — so it must NOT certify.
  require_ao
  make_repo
  add_code_commit "feat: a normal change (age-y)"; C1="$LAST_SHA"
  bind_edge_evidence age-y "$C1" "pawl-verdict age-y disposition=CONFIRMEDLY"
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind CONFIRMEDLY near-miss #trivial"
  BND="$(git -C "$REPO" rev-parse HEAD)"
  run_backstop --repo "$REPO" --base "$BASE_SHA" --head "$BND" --enforce
  [ "$status" -ne 0 ]
  [[ "$output" == *"::warning::commit ${C1:0:12} lacks a bound verdict"* ]]
}

@test "PARITY: a valid single-token CONFIRMED/REBOUND still authorizes after the first-token sweep" {
  # Positive control: the parity sweep must not break the happy path — a normal
  # single-token CONFIRMED still certifies a plain commit.
  require_ao
  make_repo
  add_code_commit "feat: a proven change (age-ok)"; C1="$LAST_SHA"
  bind_verdict age-ok "$C1"
  run_backstop --repo "$REPO" --base "$BASE_SHA" --head "$C1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"verified — bound verdict event age-ok@${C1:0:7}"* ]]
  [[ "$output" != *"::warning::"* ]]
}

# ── single-implementation: CI sources the SHARED diff-identity library ───────
@test "single-implementation: the CI backstop sources lib/diff-identity.sh (no re-inlined signature)" {
  # CI honors REBOUND via the SAME diff-identity library the pawl scripts use — a
  # re-inlined patch-id/content-signature would drift from age-rk3r.9's shared
  # source. Assert the source + that the awk normalization is NOT copied here.
  grep -q 'source "\$SCRIPT_DIR/lib/diff-identity.sh"' "$SCRIPT"
  grep -q 'commit_patch_id' "$SCRIPT"
  grep -q 'commit_content_sig' "$SCRIPT"
  # The distinctive normalization lines live ONLY in the lib, never re-inlined here.
  grep -q 'index BLOB..BLOB' "$REPO_ROOT/scripts/lib/diff-identity.sh"
  ! grep -q 'index BLOB..BLOB' "$SCRIPT"
  ! grep -q '@@ POS @@' "$SCRIPT"
}
