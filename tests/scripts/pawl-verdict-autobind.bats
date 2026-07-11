#!/usr/bin/env bats
# pawl-verdict.sh — VERIFIED verdict-edge emission + ledger auto-bind
# (age-wedge-all-in-dyr0.3).
#
# The old sensor was `ao provenance emit-verdict … || true` — a silently dead
# sensor. These tests prove the replacement:
#   - emit is CHECKED: a failed emit never blocks the verdict, but warns LOUDLY
#     (names the corrective command, prints a SECONDARY-STATUS marker);
#   - on a real append, STANDALONE runs auto-commit ONLY
#     docs/provenance/ledger.jsonl with the established #trivial message shape
#     (the check-pawl-pre-push.sh age-u43w waiver stays satisfied);
#   - PRE-PUSH context (PAWL_PREPUSH marker or inherited GIT_PREFIX) NEVER
#     creates a commit — the row is parked and the bind one-liner printed;
#   - PAWL_AUTOBIND=0 opts out; unrelated staged work is never swept in.
#
# `ao` is STUBBED via PATH (records argv; controllable exit; optionally appends
# a REAL-SHAPE verdict edge to the ledger, mimicking a genuine emit). All git
# state lives in a temp repo under $BATS_TMPDIR — NEVER the real repo.

setup() {
  AGENTOPS_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$AGENTOPS_ROOT/scripts/pawl-verdict.sh"
  PREPUSH_GATE="$AGENTOPS_ROOT/scripts/check-pawl-pre-push.sh"
  TMP="$(mktemp -d "${BATS_TMPDIR:-/tmp}/pawl-autobind.XXXXXX")"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; VDIR="$TMP/verdicts"
  mkdir -p "$BIN" "$VDIR"
  AO_LOG="$TMP/ao.log"; : > "$AO_LOG"

  # Isolated ledger-root git repo: docs/ + schemas/ so the script's ledger-root
  # walk (mirroring ao resolveLedgerPath) resolves HERE, from cwd.
  REPO="$TMP/repo"
  mkdir -p "$REPO/docs/provenance" "$REPO/schemas"
  git -C "$REPO" init -q
  git -C "$REPO" config user.email pawl-test@example.com
  git -C "$REPO" config user.name pawl-test
  printf '%s\n' '{"schema_version":"agentops-sdlc-provenance.v1","from_id":"genesis","from_type":"bead","to_id":"genesis-commit","to_type":"commit","relation":"wasGeneratedBy","evidence_ref":"genesis","trust_tier":"authored","ts":"2026-06-13T23:55:06Z","prev_hash":"","payload_hash":"52b578a2","hash":"ae78526f"}' \
    > "$REPO/docs/provenance/ledger.jsonl"
  git -C "$REPO" add -A
  git -C "$REPO" commit -qm "init ledger"
  HEAD0="$(git -C "$REPO" rev-parse HEAD)"
  SHA="1111111222222233333334444444555555566666"
  LEDGER="$REPO/docs/provenance/ledger.jsonl"
  BIND_MSG="chore(provenance): bind pawl CONFIRMED verdict for age-autobind-test #trivial"

  # Stub ao. AO_STUB_APPEND=1 -> append a REAL-shape verdict edge (fixture
  # fidelity: field set + order match the production writer's rows) to the
  # temp repo ledger. AO_STUB_EXIT / AO_STUB_ERR control failure modes.
  cat > "$BIN/ao" <<EOF
#!/usr/bin/env bash
echo "\$*" >> "$AO_LOG"
# Append ONLY on the emit-verdict subcommand — production \`ao yield emit\`
# (also routed through this stub by do_write) never touches the provenance
# ledger, and mimicking that matters: an append attributed to the wrong
# subcommand re-dirties the ledger after the bind commit.
if [[ "\${AO_STUB_APPEND:-0}" == "1" && "\${1:-}" == "provenance" && "\${2:-}" == "emit-verdict" ]]; then
  printf '%s\n' '{"schema_version":"agentops-sdlc-provenance.v1","from_id":"age-autobind-test@1111111","from_type":"verdict","to_id":"1111111222222233333334444444555555566666","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"pawl-verdict age-autobind-test disposition=CONFIRMED","trust_tier":"inferred","ts":"2026-07-01T00:00:00Z","prev_hash":"ae78526f","payload_hash":"deadbeef","hash":"beefdead"}' >> "$REPO/docs/provenance/ledger.jsonl"
fi
if [[ -n "\${AO_STUB_ERR:-}" ]]; then echo "\$AO_STUB_ERR" >&2; fi
exit "\${AO_STUB_EXIT:-0}"
EOF
  chmod +x "$BIN/ao"
  export PATH="$BIN:$PATH"
  cd "$REPO"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

# run_write [VAR=val ...] — invoke `pawl-verdict.sh write` hermetically:
# hook/pre-push markers and repo-root overrides are UNSET unless a test
# re-adds them (env applies -u first, then the NAME=VALUE args).
run_write() {
  env -u GIT_PREFIX -u GIT_DIR -u PAWL_PREPUSH -u PAWL_AUTOBIND -u AGENTOPS_REPO_ROOT -u AO_BIN "$@" \
    bash "$SCRIPT" write age-autobind-test 0 \
    --disposition CONFIRMED --head "$SHA" \
    --author-context author-ctx \
    --refuter claude:CONFIRMED:refuter-ctx \
    --dir "$VDIR"
}

# ---------------------------------------------------------------------------
# (a) successful emit, STANDALONE -> ledger committed, exact message + file set
# ---------------------------------------------------------------------------
@test "standalone: successful emit auto-binds ONLY the ledger with the #trivial message" {
  run run_write AO_STUB_APPEND=1
  echo "# status=$status" >&3
  echo "# output=$output" >&3
  [ "$status" -eq 0 ]

  # verdict artifact written + sensor fired with the artifact path
  [ -f "$VDIR/age-autobind-test.json" ]
  grep -q "provenance emit-verdict --file $VDIR/age-autobind-test.json" "$AO_LOG"

  # exactly ONE new commit, exact message shape (trailing #trivial subject)
  tip="$(git -C "$REPO" rev-parse HEAD)"
  [ "$tip" != "$HEAD0" ]
  [ "$(git -C "$REPO" rev-parse "$tip^")" = "$HEAD0" ]
  subject="$(git -C "$REPO" log -1 --format=%s "$tip")"
  [ "$subject" = "$BIND_MSG" ]

  # files-in-commit: the ledger path and NOTHING else
  files="$(git -C "$REPO" diff-tree --no-commit-id --name-only -r "$tip")"
  [ "$files" = "docs/provenance/ledger.jsonl" ]

  # worktree left clean for the ledger path
  [ -z "$(git -C "$REPO" status --porcelain -- docs/provenance/ledger.jsonl)" ]
  [[ "$output" == *"auto-bound verdict ledger edge"* ]]
}

@test "standalone: the bind commit satisfies the check-pawl-pre-push #trivial waiver (age-u43w)" {
  run run_write AO_STUB_APPEND=1
  [ "$status" -eq 0 ]
  tip="$(git -C "$REPO" rev-parse HEAD)"
  [ "$tip" != "$HEAD0" ]

  # Feed the REAL pre-push gate the bind commit as the pushed tip: it must be
  # waived (provenance-ledger-only #trivial), authorizing the push.
  run env AGENTOPS_REPO_ROOT="$REPO" bash -c \
    "printf 'refs/heads/main %s refs/heads/main %s\n' '$tip' '$HEAD0' | bash '$PREPUSH_GATE'"
  echo "# gate status=$status output=$output" >&3
  [ "$status" -eq 0 ]
  [[ "$output" == *"pawl waived"* ]]
}

# ---------------------------------------------------------------------------
# (b) emit failure -> verdict still produced, LOUD warning, NO commit
# ---------------------------------------------------------------------------
@test "emit failure: verdict still written, loud corrective warning, no commit" {
  run run_write AO_STUB_EXIT=3 AO_STUB_ERR="emit-verdict: ledger open failed"
  echo "# status=$status" >&3
  echo "# output=$output" >&3
  [ "$status" -eq 7 ]                              # F2 (age-pawl-intent-zhndq.2): fail-CLOSED EDGE-UNBOUND (was fail-open 0)
  [ -f "$VDIR/age-autobind-test.json" ]            # the verdict stands (the recovery input)
  [[ "$output" == *"WARNING — provenance verdict-edge emit FAILED"* ]]
  [[ "$output" == *"exited 3"* ]]
  [[ "$output" == *"ledger open failed"* ]]        # captured stderr surfaced
  [[ "$output" == *"ao provenance emit-verdict --file $VDIR/age-autobind-test.json"* ]]
  [[ "$output" == *"SECONDARY-STATUS: provenance-emit=1"* ]]
  [ "$(git -C "$REPO" rev-parse HEAD)" = "$HEAD0" ]  # no commit created
}

@test "emit success with NO append (idempotent no-op): no commit is created" {
  run run_write   # stub exits 0 but does not touch the ledger
  [ "$status" -eq 0 ]
  [ "$(git -C "$REPO" rev-parse HEAD)" = "$HEAD0" ]
  [[ "$output" != *"auto-bound"* ]]
}

# ---------------------------------------------------------------------------
# (c) pre-push context -> NO commit, one-liner printed, row parked
# ---------------------------------------------------------------------------
@test "pre-push (PAWL_PREPUSH=1): no commit, ledger row parked, bind one-liner printed" {
  run run_write AO_STUB_APPEND=1 PAWL_PREPUSH=1
  echo "# status=$status" >&3
  echo "# output=$output" >&3
  [ "$status" -eq 0 ]
  [ "$(git -C "$REPO" rev-parse HEAD)" = "$HEAD0" ]      # HEAD untouched mid-push
  grep -q '"from_type":"verdict"' "$LEDGER"              # row IS written…
  [ -n "$(git -C "$REPO" status --porcelain -- docs/provenance/ledger.jsonl)" ]  # …but uncommitted
  [[ "$output" == *"PRE-PUSH context detected"* ]]
  [[ "$output" == *"after the push completes"* ]]
  [[ "$output" == *"git -C $REPO add -- docs/provenance/ledger.jsonl"* ]]
  [[ "$output" == *"$BIND_MSG"* ]]                       # the exact one-liner message
}

@test "pre-push (inherited GIT_PREFIX, the git-hook env): no commit, row parked" {
  run run_write AO_STUB_APPEND=1 GIT_PREFIX=
  [ "$status" -eq 0 ]
  [ "$(git -C "$REPO" rev-parse HEAD)" = "$HEAD0" ]
  [ -n "$(git -C "$REPO" status --porcelain -- docs/provenance/ledger.jsonl)" ]
  [[ "$output" == *"PRE-PUSH context detected"* ]]
}

# ---------------------------------------------------------------------------
# (d) PAWL_AUTOBIND=0 -> opt out, no commit
# ---------------------------------------------------------------------------
@test "PAWL_AUTOBIND=0: no commit, opt-out note + bind command printed" {
  run run_write AO_STUB_APPEND=1 PAWL_AUTOBIND=0
  [ "$status" -eq 0 ]
  [ "$(git -C "$REPO" rev-parse HEAD)" = "$HEAD0" ]
  [ -n "$(git -C "$REPO" status --porcelain -- docs/provenance/ledger.jsonl)" ]
  [[ "$output" == *"auto-bind OFF (PAWL_AUTOBIND=0)"* ]]
  [[ "$output" == *"$BIND_MSG"* ]]
}

# ---------------------------------------------------------------------------
# (e) unrelated staged changes are NEVER swept into the bind commit
# ---------------------------------------------------------------------------
@test "unrelated staged file stays staged and OUT of the bind commit" {
  echo unrelated > "$REPO/other.txt"
  git -C "$REPO" add -- other.txt

  run run_write AO_STUB_APPEND=1
  echo "# status=$status" >&3
  echo "# output=$output" >&3
  [ "$status" -eq 0 ]
  tip="$(git -C "$REPO" rev-parse HEAD)"
  [ "$tip" != "$HEAD0" ]

  # bind commit holds ONLY the ledger
  files="$(git -C "$REPO" diff-tree --no-commit-id --name-only -r "$tip")"
  [ "$files" = "docs/provenance/ledger.jsonl" ]

  # the unrelated file is STILL staged, untouched
  git -C "$REPO" diff --cached --name-only | grep -qx "other.txt"
  [ "$(cat "$REPO/other.txt")" = "unrelated" ]
}

# ---------------------------------------------------------------------------
# rebind re-fires the checked emit + auto-bind too (the second former || true site)
# ---------------------------------------------------------------------------
@test "rebind: checked emit fires and auto-binds on append" {
  run run_write   # plain write (no append -> no commit) to create the verdict
  [ "$status" -eq 0 ]
  [ "$(git -C "$REPO" rev-parse HEAD)" = "$HEAD0" ]
  : > "$AO_LOG"

  newsha="aaaaaaaabbbbbbbbccccccccdddddddd00001111"
  run env -u GIT_PREFIX -u GIT_DIR -u PAWL_PREPUSH -u PAWL_AUTOBIND -u AGENTOPS_REPO_ROOT -u AO_BIN \
    AO_STUB_APPEND=1 \
    bash "$SCRIPT" rebind age-autobind-test 0 --head "$newsha" --dir "$VDIR"
  echo "# status=$status output=$output" >&3
  [ "$status" -eq 0 ]
  grep -q "provenance emit-verdict --file $VDIR/age-autobind-test.json" "$AO_LOG"
  tip="$(git -C "$REPO" rev-parse HEAD)"
  [ "$tip" != "$HEAD0" ]
  [ "$(git -C "$REPO" log -1 --format=%s "$tip")" = "$BIND_MSG" ]
  [ "$(git -C "$REPO" diff-tree --no-commit-id --name-only -r "$tip")" = "docs/provenance/ledger.jsonl" ]
}

# ---------------------------------------------------------------------------
# missing ao binary -> loud warning naming the fix, verdict unaffected
# ---------------------------------------------------------------------------
@test "no trusted ao resolvable: verdict written, loud warning names the corrective command" {
  # PAWL_UNTRUSTED_REPO=1 with AO_BIN unset is the deterministic "no trusted ao"
  # shape (_ao_bin refuses PATH lookup on the untrusted path by design).
  run run_write PAWL_UNTRUSTED_REPO=1
  echo "# status=$status" >&3
  echo "# output=$output" >&3
  [ "$status" -eq 7 ]                              # F2: fail-CLOSED EDGE-UNBOUND (was fail-open 0)
  [ -f "$VDIR/age-autobind-test.json" ]
  [[ "$output" == *"no trusted ao binary found"* ]]
  [[ "$output" == *"ao provenance emit-verdict --file $VDIR/age-autobind-test.json"* ]]
  [[ "$output" == *"SECONDARY-STATUS: provenance-emit=1"* ]]
  [ "$(git -C "$REPO" rev-parse HEAD)" = "$HEAD0" ]
}

# ---------------------------------------------------------------------------
# (age-7krl) foreign same-file ledger rows swept into the bind commit
#
# The path-scoped bind commit captures the whole working-tree-vs-HEAD delta of
# docs/provenance/ledger.jsonl — so any rows already sitting UNCOMMITTED before
# this run's emit (another lane's leftovers or an aborted run) ride into the bind
# commit even though this run never emitted them. The auto-bind must WARN loudly
# (count + NAME the foreign rows) but NOT block, NOT change what is committed, and
# NOT reorder rows (hash-chained ledger rows must not be rewritten).
# ---------------------------------------------------------------------------

# (a) one pre-existing foreign row -> warned + still committed
@test "foreign uncommitted row: warned (counted + named) and still swept into the bind commit" {
  local frow='{"schema_version":"agentops-sdlc-provenance.v1","from_id":"age-FOREIGN@9999999","from_type":"verdict","to_id":"9999999888888877777776666666555555544444","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"foreign leftover row","trust_tier":"inferred","ts":"2026-06-30T00:00:00Z","prev_hash":"ae78526f","payload_hash":"cafef00d","hash":"f00dcafe"}'
  printf '%s\n' "$frow" >> "$LEDGER"
  # sanity: the leftover is uncommitted before the run
  [ -n "$(git -C "$REPO" status --porcelain -- docs/provenance/ledger.jsonl)" ]

  run run_write AO_STUB_APPEND=1
  echo "# status=$status" >&3
  echo "# output=$output" >&3
  [ "$status" -eq 0 ]

  # LOUD warning: counts 1 foreign row and NAMES it (from_id -> to_id)
  [[ "$output" == *"WARNING — the auto-bind commit will SWEEP IN 1 pre-existing ledger"* ]]
  [[ "$output" == *"age-FOREIGN@9999999 -> 9999999888888877777776666666555555544444"* ]]
  [[ "$output" == *"SECONDARY-STATUS: autobind-foreign-rows=1"* ]]

  # NOT blocked: the bind commit still lands, exact #trivial subject, ledger-only
  tip="$(git -C "$REPO" rev-parse HEAD)"
  [ "$tip" != "$HEAD0" ]
  [ "$(git -C "$REPO" log -1 --format=%s "$tip")" = "$BIND_MSG" ]
  [ "$(git -C "$REPO" diff-tree --no-commit-id --name-only -r "$tip")" = "docs/provenance/ledger.jsonl" ]

  # BOTH rows are in the committed ledger — nothing dropped (warn-only)
  run git -C "$REPO" show "$tip:docs/provenance/ledger.jsonl"
  [[ "$output" == *'"from_id":"age-FOREIGN@9999999"'* ]]
  [[ "$output" == *'"from_id":"age-autobind-test@1111111"'* ]]

  # worktree left clean for the ledger path
  [ -z "$(git -C "$REPO" status --porcelain -- docs/provenance/ledger.jsonl)" ]
}

# (b) clean ledger -> auto-bind fires but NO foreign-row warning
@test "clean ledger: successful emit auto-binds with NO foreign-row warning" {
  run run_write AO_STUB_APPEND=1
  echo "# status=$status output=$output" >&3
  [ "$status" -eq 0 ]
  # sanity: the clean path still auto-binds
  [ "$(git -C "$REPO" rev-parse HEAD)" != "$HEAD0" ]
  # NO foreign-row warning of any kind on a clean ledger
  [[ "$output" != *"SWEEP IN"* ]]
  [[ "$output" != *"autobind-foreign-rows"* ]]
}

# (a') multiple pre-existing foreign rows -> all counted and named
@test "multiple foreign rows: all counted and named in the warning" {
  local f1 f2
  f1='{"schema_version":"agentops-sdlc-provenance.v1","from_id":"age-FOR1@9999999","from_type":"verdict","to_id":"aaaa111122223333444455556666777788889999","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"leftover 1","trust_tier":"inferred","ts":"2026-06-30T00:00:00Z","prev_hash":"ae78526f","payload_hash":"cafef00d","hash":"f00dcafe"}'
  f2='{"schema_version":"agentops-sdlc-provenance.v1","from_id":"age-FOR2@8888888","from_type":"verdict","to_id":"bbbb111122223333444455556666777788889999","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"leftover 2","trust_tier":"inferred","ts":"2026-06-30T01:00:00Z","prev_hash":"f00dcafe","payload_hash":"beadfeed","hash":"feedbead"}'
  printf '%s\n%s\n' "$f1" "$f2" >> "$LEDGER"

  run run_write AO_STUB_APPEND=1
  echo "# status=$status output=$output" >&3
  [ "$status" -eq 0 ]
  [[ "$output" == *"SWEEP IN 2 pre-existing ledger"* ]]
  [[ "$output" == *"age-FOR1@9999999 -> aaaa111122223333444455556666777788889999"* ]]
  [[ "$output" == *"age-FOR2@8888888 -> bbbb111122223333444455556666777788889999"* ]]
  [[ "$output" == *"autobind-foreign-rows=2"* ]]
  # still lands, ledger-only
  tip="$(git -C "$REPO" rev-parse HEAD)"
  [ "$tip" != "$HEAD0" ]
  [ "$(git -C "$REPO" diff-tree --no-commit-id --name-only -r "$tip")" = "docs/provenance/ledger.jsonl" ]
}

# (c) untracked-ledger edge: git cannot diff it, so the whole file is foreign
@test "untracked ledger: pre-existing rows are treated as foreign and warned" {
  # Model a truly UNTRACKED ledger: drop it from history, then recreate it in the
  # working tree with a leftover row (absent from HEAD and the index, so git diff
  # cannot see it and the pre-existing content is entirely foreign).
  git -C "$REPO" rm -q -- docs/provenance/ledger.jsonl
  git -C "$REPO" commit -qm "drop ledger from history"
  local base; base="$(git -C "$REPO" rev-parse HEAD)"
  mkdir -p "$REPO/docs/provenance"
  local frow='{"schema_version":"agentops-sdlc-provenance.v1","from_id":"age-UNTRACKED@7777777","from_type":"verdict","to_id":"7777777666666655555554444444333333322222","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"untracked leftover","trust_tier":"inferred","ts":"2026-06-30T00:00:00Z","prev_hash":"","payload_hash":"cafef00d","hash":"f00dcafe"}'
  printf '%s\n' "$frow" > "$LEDGER"
  # sanity: genuinely untracked (ls-files errors on an untracked path)
  if git -C "$REPO" ls-files --error-unmatch -- docs/provenance/ledger.jsonl >/dev/null 2>&1; then
    echo "expected ledger to be untracked" >&2; return 1
  fi

  run run_write AO_STUB_APPEND=1
  echo "# status=$status output=$output" >&3
  [ "$status" -eq 0 ]
  [[ "$output" == *"SWEEP IN 1 pre-existing ledger"* ]]
  [[ "$output" == *"age-UNTRACKED@7777777 -> 7777777666666655555554444444333333322222"* ]]
  [[ "$output" == *"autobind-foreign-rows=1"* ]]

  # bind commit lands on the new base, ledger-only, holds both rows
  tip="$(git -C "$REPO" rev-parse HEAD)"
  [ "$tip" != "$base" ]
  [ "$(git -C "$REPO" diff-tree --no-commit-id --name-only -r "$tip")" = "docs/provenance/ledger.jsonl" ]
  run git -C "$REPO" show "$tip:docs/provenance/ledger.jsonl"
  [[ "$output" == *"age-UNTRACKED@7777777"* ]]
  [[ "$output" == *"age-autobind-test@1111111"* ]]
}
