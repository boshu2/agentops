#!/usr/bin/env bats
# ag-62jrm: emit-landed-provenance.sh is the milestone-1 SENSOR wiring — it runs
# the provenance emitter for the landing range and commits the ledger growth as
# a trailing sensor commit so the ledger feeds without a human. These cases use
# an injectable fake ao (AO_BIN) per the repo's gate-test convention, so they
# assert the wrapper's guard/commit/non-blocking behavior deterministically
# (the emit logic itself is unit+L2 tested in Go).

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/emit-landed-provenance.sh"
  REPO="$(mktemp -d)"
  cd "$REPO"
  git init -q
  git config user.email t@t.t
  git config user.name t
  mkdir -p docs/provenance
  printf '{"genesis":true}\n' > docs/provenance/ledger.jsonl
  git add -A && git commit -qm "genesis"
  # A second commit so origin/main..HEAD (or HEAD~1..HEAD) is non-empty.
  echo x > x.txt && git add -A && git commit -qm "feat: thing (ag-real)"

  # Fake ao that appends one ledger row, emulating emit-landed writing an edge.
  cat > "$REPO/ao-grow" <<'EOF'
#!/usr/bin/env bash
printf '{"from_id":"ag-real","relation":"wasGeneratedBy"}\n' >> docs/provenance/ledger.jsonl
exit 0
EOF
  # Fake ao that writes nothing (trivial commit, no bead cited).
  cat > "$REPO/ao-noop" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  # Fake ao that fails (emit error — must not block).
  cat > "$REPO/ao-fail" <<'EOF'
#!/usr/bin/env bash
echo "boom" >&2
exit 1
EOF
  chmod +x "$REPO"/ao-*
}

teardown() { rm -rf "$REPO"; }

@test "skip guard: AGENTOPS_PROVENANCE_EMIT_SKIP=1 is a no-op" {
  before="$(git rev-parse HEAD)"
  AGENTOPS_PROVENANCE_EMIT_SKIP=1 AO_BIN="$REPO/ao-grow" run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [ "$(git rev-parse HEAD)" = "$before" ]
}

@test "happy path: ledger grows and a trailing sensor commit is made" {
  before_count="$(git rev-list --count HEAD)"
  AO_BIN="$REPO/ao-grow" run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  # A trailing commit was created.
  after_count="$(git rev-list --count HEAD)"
  [ "$after_count" -eq "$((before_count + 1))" ]
  # The new commit is the provenance sensor commit and touches only the ledger.
  git log -1 --pretty=%s | grep -q "auto-emit landed"
  git show --stat HEAD | grep -q "docs/provenance/ledger.jsonl"
}

@test "no-op when emitter writes nothing: no trailing commit" {
  before="$(git rev-parse HEAD)"
  AO_BIN="$REPO/ao-noop" run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [ "$(git rev-parse HEAD)" = "$before" ]
}

@test "non-blocking: emitter failure exits 0 and makes no commit" {
  before="$(git rev-parse HEAD)"
  AO_BIN="$REPO/ao-fail" run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [ "$(git rev-parse HEAD)" = "$before" ]
}
