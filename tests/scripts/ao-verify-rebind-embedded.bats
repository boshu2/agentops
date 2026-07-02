#!/usr/bin/env bats
# ao-verify-rebind-embedded.bats — `ao verify <bead> --rebind` works ZERO-CONFIG on the
# installed/embedded (stranger) path: it resolves the verdict dir to the USER's TARGET REPO
# (not the extracted-bundle temp dir), finds the prior CONFIRMED there, and writes the
# REBOUND there. (age-rk3r.9)
#
# The bug this guards: runVerifyRebind used to forward --dir only when the user passed one,
# so the extracted pawl-verdict.sh defaulted VDIR to the TEMP BUNDLE's .agents/pawl-verdicts
# and reported a false "prior verdict '<bundle>/.agents/pawl-verdicts/<bead>.json' not found".
#
# Mirrors ao-verify-receipts.bats: build ao OUTSIDE the checkout to a temp path so
# `ao verify --rebind` run inside a throwaway (non-AgentOps) git repo deterministically
# takes the stranger/embedded path. The gate step is skipped (PAWL_REBIND_SKIP_GATE=1) so
# the test is deterministic and needs no `ao gate check` on the throwaway repo — the DEFECT
# being guarded is the verdict-DIR resolution, not the gate.

setup_file() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  export REPO_ROOT
  # Build ao from THIS source to a path OUTSIDE the agentops checkout, so a run inside the
  # throwaway repo takes the stranger/embedded path (aoBinaryInside is false, the repo is
  # not an AgentOps checkout). NEVER a PATH ao — it may predate this command.
  AO_BIN="$BATS_FILE_TMPDIR/ao"
  (cd "$REPO_ROOT/cli" && go build -o "$AO_BIN" ./cmd/ao)
  export AO_BIN
  GIT="$(command -v git)"; export GIT
  for tool in git jq; do
    command -v "$tool" >/dev/null 2>&1 || { echo "# missing required tool: $tool" >&3; return 1; }
  done
}

setup() {
  # Throwaway git repo (NOT an AgentOps checkout) under the bats-managed tmp, outside the
  # agentops checkout → the stranger/embedded path is forced.
  REPO="$BATS_TEST_TMPDIR/stranger"
  mkdir -p "$REPO/.agents/pawl-verdicts"
  "$GIT" -C "$REPO" init --quiet
  "$GIT" -C "$REPO" config user.email t@e.com
  "$GIT" -C "$REPO" config user.name T
  printf 'base\n' > "$REPO/f.txt"; "$GIT" -C "$REPO" add f.txt
  "$GIT" -C "$REPO" commit --quiet -m "feat: base"
  printf 'the-change\n' >> "$REPO/f.txt"; "$GIT" -C "$REPO" add f.txt
  "$GIT" -C "$REPO" commit --quiet -m "feat: the change" --date="2020-01-01T00:00:00"
  SHA_A="$("$GIT" -C "$REPO" rev-parse HEAD)"

  # A FULLY-VALID prior CONFIRMED in the USER repo's verdict dir: real evidence (repo-
  # relative) + a fresh-context refuter (context_id != author_context_id).
  printf 'f.txt:2 reviewed the added line\nfiles reviewed: 1\n' > "$REPO/.agents/pawl-verdicts/ev.txt"
  cat > "$REPO/.agents/pawl-verdicts/mybead.json" <<EOF
{"schema_version":"pawl-verdict.v1","bead_id":"mybead","pr":0,"disposition":"CONFIRMED","generated_at":"2026-01-01T00:00:00Z","author_context_id":"author-ctx","attempt":1,"refuters":[{"family":"claude","verdict":"CONFIRMED","context_id":"fresh-ctx","evidence":".agents/pawl-verdicts/ev.txt"}],"head_sha":"$SHA_A"}
EOF

  # A byte-identical rebase (same change, new sha/date/message).
  "$GIT" -C "$REPO" reset -q --hard HEAD~1
  printf 'the-change\n' >> "$REPO/f.txt"; "$GIT" -C "$REPO" add f.txt
  "$GIT" -C "$REPO" commit --quiet -m "feat: the change (rebased)" --date="2021-06-06T12:00:00"
  SHA_B="$("$GIT" -C "$REPO" rev-parse HEAD)"
}

# run_rebind [extra-args...] — run `ao verify mybead --rebind --head SHA_B` inside the
# throwaway repo (embedded path), gate skipped, no auto-bind. Zero extra config.
run_rebind() {
  run bash -c "cd '$REPO' && PAWL_REBIND_SKIP_GATE=1 PAWL_AUTOBIND=0 '$AO_BIN' verify mybead --rebind --head '$SHA_B' $*"
  echo "# ao verify --rebind exit=$status" >&3
  printf '%s\n' "$output" | sed 's/^/# /' >&3
}

@test "ao verify --rebind (embedded, zero-config) finds the prior verdict in the USER repo and writes the REBOUND there" {
  # RED (pre-fix): the prior verdict was sought in the extracted-bundle temp dir →
  # "not found". GREEN (fixed): --dir defaults to <userRepo>/.agents/pawl-verdicts.
  run_rebind
  [ "$status" -eq 0 ]
  # NOT the false "not found" — the prior verdict was located in the user repo.
  [[ "$output" != *"not found — nothing to re-bind"* ]]

  # The REBOUND was written IN THE USER REPO's verdict dir (not the bundle temp).
  local out="$REPO/.agents/pawl-verdicts/mybead.json"
  [ "$(jq -r .disposition "$out")" = "REBOUND" ]
  [ "$(jq -r .head_sha "$out")" = "$SHA_B" ]
  # lineage archived in the USER repo too. (Match on the repo-relative suffix — macOS maps
  # /var → /private/var, so an absolute-prefix compare against $REPO is fragile; assert the
  # lineage lives under the user repo's verdict dir AND is NOT in the extracted bundle.)
  local lineage; lineage="$(jq -r .rebound_from_verdict "$out")"
  [[ "$lineage" == *"/stranger/.agents/pawl-verdicts/mybead.confirmed-"* ]]
  [[ "$lineage" != *"ao-pawl-"* ]]
  [ -f "$lineage" ]
  [ "$(jq -r .disposition "$lineage")" = "CONFIRMED" ]
}

@test "ao verify --rebind (embedded) does NOT reference the extracted-bundle temp dir in its verdict path" {
  # Explicit regression pin for the exact bug: no ao-pawl-*/.agents path in the output.
  run_rebind
  [ "$status" -eq 0 ]
  [[ "$output" != *"ao-pawl-"*".agents/pawl-verdicts"* ]]
}

@test "ao verify --rebind (embedded) still honors an explicit --dir override" {
  # COPY the prior verdict into a NON-default dir and point --rebind at it explicitly; the
  # evidence stays repo-relative (resolved against the user repo via YIELD_ROOT).
  local alt="$REPO/alt-verdicts"
  mkdir -p "$alt"
  cp "$REPO/.agents/pawl-verdicts/mybead.json" "$alt/mybead.json"
  run_rebind --dir "$alt"
  [ "$status" -eq 0 ]
  [ "$(jq -r .disposition "$alt/mybead.json")" = "REBOUND" ]
  [ "$(jq -r .head_sha "$alt/mybead.json")" = "$SHA_B" ]
  # The default dir's verdict was NOT touched (the override won).
  [ "$(jq -r .disposition "$REPO/.agents/pawl-verdicts/mybead.json")" = "CONFIRMED" ]
}

# ===========================================================================
# TRUST BOUNDARY (RED-first, age-rk3r.9) — `ao verify --rebind` WITHOUT --head must resolve
# the default head via TRUSTED git (absolute, PATH stripped of repo-internal entries), NEVER
# ambient exec.Command("git") on the user's PATH before the sanitizer. A repo with an early
# `$REPO/bin/git` on PATH must NOT execute that planted binary — the exact planted-git hole
# the pawl flow (.1/.6/.12 / runPawlReview / trustedGit) closes everywhere else.
# ===========================================================================
@test "ao verify --rebind (embedded, NO --head) never executes a planted \$REPO/bin/git (trust boundary)" {
  # Plant a git shim in the repo that TOUCHES a sentinel then delegates to real git. If the
  # default-head resolution runs ambient git with $REPO/bin early on PATH, the sentinel appears.
  local realgit; realgit="$(command -v git)"
  local sentinel="$REPO/PLANTED_GIT_RAN"
  mkdir -p "$REPO/bin"
  cat > "$REPO/bin/git" <<EOF
#!/usr/bin/env bash
touch "$sentinel"
exec "$realgit" "\$@"
EOF
  chmod +x "$REPO/bin/git"
  rm -f "$sentinel"

  # Run --rebind WITHOUT --head (forces default-head resolution), with $REPO/bin FIRST on PATH.
  run bash -c "cd '$REPO' && PATH='$REPO/bin':\"\$PATH\" PAWL_REBIND_SKIP_GATE=1 PAWL_AUTOBIND=0 '$AO_BIN' verify mybead --rebind"
  echo "# exit=$status" >&3
  printf '%s\n' "$output" | sed 's/^/# /' >&3

  # THE trust-boundary assertion: the planted git must NEVER have executed.
  [ ! -f "$sentinel" ]

  # AND the rebind still resolved the correct default head (HEAD == SHA_B, the post-rebase tip)
  # and wrote the REBOUND — trusted git resolves the REAL head, so functionality is intact.
  [ "$status" -eq 0 ]
  local out="$REPO/.agents/pawl-verdicts/mybead.json"
  [ "$(jq -r .disposition "$out")" = "REBOUND" ]
  [ "$(jq -r .head_sha "$out")" = "$SHA_B" ]
}
