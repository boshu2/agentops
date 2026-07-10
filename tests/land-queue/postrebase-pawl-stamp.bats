#!/usr/bin/env bats
# agentops-2pl.7: pawl-land must stamp the post-rebase head before pushing.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  ORIG_PATH="$PATH"
  mkdir -p "$TMP/bin"
  cat >"$TMP/bin/ao" <<'EOS'
#!/usr/bin/env bash
exit 0
EOS
  chmod +x "$TMP/bin/ao"
  export PATH="$TMP/bin:$PATH"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

git_identity() {
  git -C "$1" config user.email "bats@test.local"
  git -C "$1" config user.name "bats-fixture"
  git -C "$1" config gc.auto 0
  git -C "$1" config maintenance.auto false
}

init_main_repo() {
  local repo="$1"
  mkdir -p "$repo"
  git -C "$repo" init --quiet
  git -C "$repo" checkout -q -b main
  git_identity "$repo"
}

copy_pawl_scripts() {
  local repo="$1"
  mkdir -p "$repo/scripts" "$repo/scripts/lib" "$repo/schemas" "$repo/.agents/pawl-verdicts" "$repo/.git/hooks"
  cp "$REPO_ROOT/scripts/pawl-land.sh" "$repo/scripts/pawl-land.sh"
  cp "$REPO_ROOT/scripts/pawl-verdict.sh" "$repo/scripts/pawl-verdict.sh"
  cp "$REPO_ROOT/scripts/check-pawl-pre-push.sh" "$repo/scripts/check-pawl-pre-push.sh"
  # check-pawl-pre-push.sh sources scripts/lib/trivial-waiver.sh and dies if absent.
  cp "$REPO_ROOT/scripts/lib/trivial-waiver.sh" "$repo/scripts/lib/trivial-waiver.sh"
  cp "$REPO_ROOT/schemas/pawl-verdict.v1.schema.json" "$repo/schemas/pawl-verdict.v1.schema.json"
  chmod +x "$repo/scripts/"*.sh
  cat >"$repo/.git/hooks/pre-push" <<'EOS'
#!/usr/bin/env bash
set -euo pipefail
echo pre-push >>"$(git rev-parse --show-toplevel)/.git/pre-push-count"
exec "$(git rev-parse --show-toplevel)/scripts/check-pawl-pre-push.sh"
EOS
  chmod +x "$repo/.git/hooks/pre-push"
}

make_base_fixture() {
  local content="$1"
  BEAD="age-postrebase-pawl-stamp"
  ORIGIN="$TMP/origin.git"
  REPO="$TMP/repo"
  REMOTE_WRITER="$TMP/remote-writer"
  EVIDENCE="$TMP/evidence.txt"

  git init --bare --quiet "$ORIGIN"
  git -C "$ORIGIN" symbolic-ref HEAD refs/heads/main

  init_main_repo "$REPO"
  printf '%s\n' "$content" > "$REPO/shared.txt"
  git -C "$REPO" add shared.txt
  git -C "$REPO" commit --quiet -m "base"
  git -C "$REPO" remote add origin "$ORIGIN"
  git -C "$REPO" push --quiet -u origin main
  copy_pawl_scripts "$REPO"

  git clone --quiet "$ORIGIN" "$REMOTE_WRITER"
  git_identity "$REMOTE_WRITER"

  printf 'fresh-context review evidence\n' > "$EVIDENCE"
}

write_confirmed_verdict() {
  local bead="$1" head="$2"
  bash "$REPO/scripts/pawl-verdict.sh" write "$bead" 0 \
    --disposition CONFIRMED --head "$head" \
    --author-context author-ctx \
    --refuter "gpt:CONFIRMED:fresh-reviewer-ctx:$EVIDENCE" \
    --dir "$REPO/.agents/pawl-verdicts" >/dev/null
}

advance_origin_unrelated() {
  printf 'upstream\n' > "$REMOTE_WRITER/upstream.txt"
  git -C "$REMOTE_WRITER" add upstream.txt
  git -C "$REMOTE_WRITER" commit --quiet -m "chore: upstream unrelated"
  git -C "$REMOTE_WRITER" push --quiet origin main
  UPSTREAM_SHA="$(git -C "$REMOTE_WRITER" rev-parse HEAD)"
}

advance_origin_conflicting() {
  printf 'remote\n' > "$REMOTE_WRITER/shared.txt"
  git -C "$REMOTE_WRITER" add shared.txt
  git -C "$REMOTE_WRITER" commit --quiet -m "chore: upstream conflicting"
  git -C "$REMOTE_WRITER" push --quiet origin main
  UPSTREAM_SHA="$(git -C "$REMOTE_WRITER" rev-parse HEAD)"
}

make_local_bead_commit() {
  local path="$1" content="$2"
  git -C "$REPO" checkout --quiet -b feat/land-queue
  printf '%s\n' "$content" > "$REPO/$path"
  git -C "$REPO" add "$path"
  git -C "$REPO" commit --quiet -m "fix(pawl): post-rebase stamp ($BEAD)"
  A_SHA="$(git -C "$REPO" rev-parse HEAD)"
}

@test "pawl-land rebases onto fresh origin/main before stamping and pushes once" {
  make_base_fixture "base"
  make_local_bead_commit local.txt local
  write_confirmed_verdict "$BEAD" "$A_SHA"
  advance_origin_unrelated

  cd "$REPO"
  run bash "$REPO/scripts/pawl-land.sh" "$BEAD"

  [ "$status" -eq 0 ]
  [[ "$output" == *"LANDED $BEAD"* ]]
  POST_SHA="$(git -C "$REPO" rev-parse HEAD)"
  [ "$POST_SHA" != "$A_SHA" ]
  [ "$(git -C "$REPO" rev-parse HEAD^)" = "$UPSTREAM_SHA" ]
  [ "$(jq -r '.head_sha' "$REPO/.agents/pawl-verdicts/$BEAD.json")" = "$POST_SHA" ]

  run bash -c "printf 'refs/heads/feat/land-queue %s refs/heads/main %s\n' '$POST_SHA' '$UPSTREAM_SHA' | '$REPO/scripts/check-pawl-pre-push.sh'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"push authorized"* ]]

  [ "$(git -C "$ORIGIN" rev-parse refs/heads/main)" = "$POST_SHA" ]
  [ "$(wc -l < "$REPO/.git/pre-push-count" | tr -d ' ')" = "1" ]
}

@test "pawl-land aborts a conflicting rebase cleanly and does not push" {
  make_base_fixture "base"
  make_local_bead_commit shared.txt local
  write_confirmed_verdict "$BEAD" "$A_SHA"
  advance_origin_conflicting

  cd "$REPO"
  run bash "$REPO/scripts/pawl-land.sh" "$BEAD"

  [ "$status" -ne 0 ]
  [[ "$output" == *"rebase onto origin/main failed"* ]]
  [[ "$output" == *"aborted without pushing"* ]]
  [ ! -d "$REPO/.git/rebase-merge" ]
  [ ! -d "$REPO/.git/rebase-apply" ]
  [ "$(git -C "$REPO" rev-parse HEAD)" = "$A_SHA" ]
  [ -z "$(git -C "$REPO" status --porcelain=v1 --untracked-files=no)" ]
  [ "$(jq -r '.head_sha' "$REPO/.agents/pawl-verdicts/$BEAD.json")" = "$A_SHA" ]
  [ "$(git -C "$ORIGIN" rev-parse refs/heads/main)" = "$UPSTREAM_SHA" ]
  [ ! -f "$REPO/.git/pre-push-count" ]
}

@test "pawl-land refuses when origin/main advanced after an upstream-range review" {
  make_base_fixture "base"
  make_local_bead_commit local.txt local
  reviewed_base="$(git -C "$REPO" rev-parse origin/main)"
  write_confirmed_verdict "$BEAD" "$A_SHA"
  advance_origin_unrelated

  cd "$REPO"
  run bash "$REPO/scripts/pawl-land.sh" "$BEAD" 0 "$reviewed_base"

  [ "$status" -ne 0 ]
  [[ "$output" == *"advanced after review"* ]]
  [ "$(git -C "$REPO" rev-parse HEAD)" = "$A_SHA" ]
  [ "$(git -C "$ORIGIN" rev-parse refs/heads/main)" = "$UPSTREAM_SHA" ]
  [ ! -f "$REPO/.git/pre-push-count" ]
}

@test "pawl-land: a provenance-only FEAT (no #trivial marker) stays the verdict target — never rebound to its parent" {
  # Cross-family refute (age-fkps landing): classifying the tip by FILES alone would
  # misread a legitimate provenance-only feat (a deliberate ledger re-seal) as the
  # auto-bind and rebind the verdict to HEAD^. The auto-bind signature is files AND
  # the #trivial subject marker.
  make_base_fixture "base"
  git -C "$REPO" checkout --quiet -b feat/land-queue
  mkdir -p "$REPO/docs/provenance"
  printf '{"reseal":true}\n' > "$REPO/docs/provenance/ledger.jsonl"
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "fix(provenance): deliberate ledger re-seal ($BEAD)"
  A_SHA="$(git -C "$REPO" rev-parse HEAD)"
  write_confirmed_verdict "$BEAD" "$A_SHA"
  advance_origin_unrelated

  cd "$REPO"
  run bash "$REPO/scripts/pawl-land.sh" "$BEAD"

  [ "$status" -eq 0 ]
  POST_SHA="$(git -C "$REPO" rev-parse HEAD)"
  # The verdict must follow the FEAT itself across the rebase — HEAD, not HEAD^.
  [ "$(jq -r '.head_sha' "$REPO/.agents/pawl-verdicts/$BEAD.json")" = "$POST_SHA" ]
  [ "$(git -C "$ORIGIN" rev-parse refs/heads/main)" = "$POST_SHA" ]
}

@test "pawl-land: a REAL auto-bind tip (#trivial marker + provenance-only) rebinds the verdict to the FEAT parent" {
  make_base_fixture "base"
  make_local_bead_commit local.txt local
  mkdir -p "$REPO/docs/provenance"
  printf '{"edge":"bind"}\n' > "$REPO/docs/provenance/ledger.jsonl"
  git -C "$REPO" add docs/provenance/ledger.jsonl
  git -C "$REPO" commit --quiet -m "chore(provenance): bind pawl CONFIRMED verdict for $BEAD #trivial"
  write_confirmed_verdict "$BEAD" "$A_SHA"
  advance_origin_unrelated

  cd "$REPO"
  run bash "$REPO/scripts/pawl-land.sh" "$BEAD"

  [ "$status" -eq 0 ]
  POST_TIP="$(git -C "$REPO" rev-parse HEAD)"
  POST_FEAT="$(git -C "$REPO" rev-parse HEAD^)"
  # Tip is the #trivial bind; the verdict binds the FEAT beneath it.
  [ "$(jq -r '.head_sha' "$REPO/.agents/pawl-verdicts/$BEAD.json")" = "$POST_FEAT" ]
  [ "$(git -C "$ORIGIN" rev-parse refs/heads/main)" = "$POST_TIP" ]
}
