#!/usr/bin/env bats
# pawl-review.sh — RUN the cross-family membrane review + write the verdict on CONFIRMED.
# The real codex refuter is replaced by a STUB on PATH (canned verdict via $CODEX_STUB,
# exit via $CODEX_EXIT), so these prove the ORCHESTRATION (diff -> review -> parse ->
# verdict/exit) without a live model call. Everything runs inside a temp repo
# (AGENTOPS_REPO_ROOT) so the real repo + its ledger are never touched.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  cat > "$BIN/codex" <<'STUB'
#!/usr/bin/env bash
prompt="$(cat)"   # the refuter prompt
[[ -n "${PKT_CAPTURE:-}" ]] && printf '%s\n' "$prompt" > "$PKT_CAPTURE"
# CONVERGE_AWARE: model the real flow — REFUTE the adversarial pass (cosmetic tail), but
# CONFIRM the calibrated convergence pass (so adversarial records lineage, converge certifies).
if [[ "${CODEX_CONVERGE_AWARE:-0}" == "1" ]]; then
  if grep -qi 'CONVERGENCE pass' <<<"$prompt"; then printf 'codex\nVERDICT: CONFIRMED\n'
  else printf 'codex\nVERDICT: REFUTED\nDEFECTS:\n - cosmetic tail nit\n'; fi
  exit 0
fi
printf 'codex\n%s\n' "${CODEX_STUB:-VERDICT: CONFIRMED}"
exit "${CODEX_EXIT:-0}"
STUB
  chmod +x "$BIN/codex"
  PATH="$BIN:$PATH"
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  echo init > README.md; git add README.md; git commit --quiet -m init
  echo change >> README.md; git add README.md
  git commit --quiet -m "feat(x): a change (age-rev-test)"
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"
  mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-rev-test.json"
  # Default ALL tests to the COLD codex-exec path: the ml8.7 routed branch checks a live
  # pawl-service, and a real standing service may be up on the dev box — without this, the
  # cold-path tests below would spuriously route to it. The routed tests re-enable + stub it.
  export PAWL_NO_SERVICE=1
}

# A stub standing-pawl service. `health` honors STUB_HEALTH_RC. `route <route> <bead> <pkt> <pr>`:
#   - STUB_ROUTE_WRITES=0 -> writes nothing;
#   - STUB_VALID=1        -> writes a GATE-VALID multi-model verdict via the REAL pawl-verdict.sh
#                           write (so pawl-verdict.sh check PASSES);
#   - else (default)      -> writes a MINIMAL/invalid verdict shape (check must REJECT it);
#   exits STUB_ROUTE_RC.
_pawl_service_stub() {
  cat > "$TMP/pawl-stub.sh" <<STUB
#!/usr/bin/env bash
case "\$1" in
  health) exit \${STUB_HEALTH_RC:-0} ;;
  route)
    bead="\$2"; pr="\${4:-0}"; disp="\${STUB_ROUTE_DISP:-CONFIRMED}"
    if [ "\${STUB_ROUTE_WRITES:-1}" = "1" ]; then
      if [ "\${STUB_VALID:-0}" = "1" ]; then
        printf 'opus pane: real review, CONFIRMED\n'  > "$TMP/ev-o.txt"
        printf 'codex pane: real review, CONFIRMED\n' > "$TMP/ev-c.txt"
        bash "$REPO_ROOT/scripts/pawl-verdict.sh" write "\$bead" "\$pr" \
          --disposition "\$disp" --head "$HEAD_SHA" \
          --author-context "pawl-route-author-\$bead" --mode multi-model \
          --refuter "claude:\$disp:opus-pane:$TMP/ev-o.txt" \
          --refuter "gpt:\$disp:codex-pane:$TMP/ev-c.txt" \
          --dir "$AGENTOPS_PAWL_VERDICT_DIR" >/dev/null 2>&1 || true
      else
        printf '{"bead_id":"%s","disposition":"%s","head_sha":"%s","mode":"multi-model"}\n' \
          "\$bead" "\$disp" "$HEAD_SHA" > "$AGENTOPS_PAWL_VERDICT_DIR/\$bead.json"
      fi
    fi
    exit \${STUB_ROUTE_RC:-0} ;;
  *) exit 2 ;;
esac
STUB
  chmod +x "$TMP/pawl-stub.sh"
  echo "$TMP/pawl-stub.sh"
}

teardown() { cd "$ORIG_DIR" 2>/dev/null || true; rm -rf "$TMP"; }

@test "pawl-review: head CONFIRMED writes a commit-bound verdict that passes check (exit 0)" {
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [[ "$output" == *"CONFIRMED + verdict written"* ]]
  [ -f "$VFILE" ]
  grep -q '"disposition": "CONFIRMED"' "$VFILE"
  grep -q "$HEAD_SHA" "$VFILE"
}

@test "pawl-review: REFUTED prints defects, writes NO verdict, exits 3" {
  CODEX_STUB="$(printf 'VERDICT: REFUTED\nDEFECTS:\n - the foo path is fail-open')" \
    run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 3 ]
  [[ "$output" == *"REFUTED"* ]]
  [[ "$output" == *"fail-open"* ]]
  [ ! -f "$VFILE" ]
}

@test "pawl-review: no clear verdict is fail-closed (exit 1), no verdict written" {
  CODEX_STUB="maybe it is fine?" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 1 ]
  [ ! -f "$VFILE" ]
}

@test "pawl-review: CONFIRMED from a NON-ZERO-exit reviewer run is fail-closed (defect #3)" {
  CODEX_STUB="VERDICT: CONFIRMED" CODEX_EXIT=124 run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 1 ]
  [[ "$output" == *"non-zero"* ]]
  [ ! -f "$VFILE" ]   # a CONFIRMED from a crashed/timed-out reviewer must NOT write a verdict
}

@test "pawl-review: scope=staged CONFIRMED is REVIEW-ONLY — no verdict written (defect #1)" {
  echo more >> README.md; git add README.md   # stage an uncommitted change
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope staged
  [ "$status" -eq 0 ]
  [[ "$output" == *"review-only"* ]]
  [ ! -f "$VFILE" ]   # staged has no commit to bind — must not certify HEAD
}

@test "pawl-review: scope=upstream reviews every branch commit and binds the tip" {
  base="$(git rev-parse HEAD~1)"
  origin="$TMP/origin.git"
  git init --bare --quiet "$origin"
  git remote add origin "$origin"
  git push --quiet origin "$base":refs/heads/main
  git fetch --quiet origin main
  git branch --set-upstream-to=origin/main >/dev/null
  printf 'second branch commit\n' > second.txt
  git add second.txt
  git commit --quiet -m "fix(x): second branch commit (age-rev-test)"
  tip="$(git rev-parse HEAD)"
  packet="$TMP/upstream-packet.txt"

  PKT_CAPTURE="$packet" CODEX_STUB="VERDICT: CONFIRMED" \
    run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope upstream

  [ "$status" -eq 0 ]
  grep -q 'scope upstream' "$packet"
  grep -q 'README.md' "$packet"
  grep -q 'second.txt' "$packet"
  [ "$(jq -r '.head_sha' "$VFILE")" = "$tip" ]
}

@test "pawl-review: scope=upstream fails closed without a configured upstream" {
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope upstream
  [ "$status" -eq 2 ]
  [[ "$output" == *"configured upstream"* ]]
}

@test "pawl-review: same-family author (codex == the codex refuter) is REFUSED (defect #2)" {
  # A codex/openai/gpt author + the codex refuter is a SAME-family review, not the
  # cross-family check this command provides — refuse, write nothing.
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head --author-family codex
  [ "$status" -eq 2 ]
  [[ "$output" == *"same-family"* ]]
  [ ! -f "$VFILE" ]
}

@test "pawl-review: a DIFFERENT-family author (default claude) is allowed cross-family" {
  # claude author + codex refuter = genuinely cross-family -> writes the verdict.
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head --author-family claude
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
}

@test "pawl-review: same-family guard is CASE-INSENSITIVE (Codex/GPT cannot bypass)" {
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head --author-family Codex
  [ "$status" -eq 2 ]
  [[ "$output" == *"same-family"* ]]
  [ ! -f "$VFILE" ]
}

@test "pawl-review: an echoed template verdict does NOT override the reviewer's FINAL verdict" {
  # The output quotes the prompt template (VERDICT: CONFIRMED / VERDICT: REFUTED) BEFORE
  # the real answer (REFUTED). The earlier template CONFIRMED must not win — last wins.
  CODEX_STUB="$(printf 'Reply with this shape:\nVERDICT: CONFIRMED\n-- or --\nVERDICT: REFUTED\nMy actual answer:\nVERDICT: REFUTED\nDEFECTS:\n - a genuine bug')" \
    run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 3 ]            # REFUTED wins, not the echoed CONFIRMED
  [ ! -f "$VFILE" ]
}

@test "pawl-review: empty staged diff is a precondition error (exit 2)" {
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope staged
  [ "$status" -eq 2 ]
  [[ "$output" == *"empty diff"* ]]
}

@test "pawl-review: a flag with no value is a usage error (exit 2)" {
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope
  [ "$status" -eq 2 ]
  [[ "$output" == *"needs a value"* ]]
}

# age-rk3r.10: the change-id is now OPTIONAL — when omitted it is DERIVED from the branch
# (sanitized), else the short-sha, and the run proceeds (the change-id is a LABEL only, never
# tracker-validated). This supersedes the old "need <bead-id>" usage error (exit 2).
@test "pawl-review: NO change-id on a branch -> derives a sanitized label, says so, runs (age-rk3r.10)" {
  git checkout -q -b feat/derive-me
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" --scope head
  [ "$status" -eq 0 ]
  # It announced the derived label AND that a change-id is a label only (not a bare error).
  [[ "$output" == *"no change-id given"* ]]
  [[ "$output" == *"using the derived label"* ]]
  [[ "$output" != *"need <bead-id>"* ]]
  # 'feat/derive-me' -> filename-safe 'feat-derive-me'; the verdict is keyed on the derived label.
  [[ "$output" == *"feat-derive-me"* ]]
  [ -f "$AGENTOPS_PAWL_VERDICT_DIR/feat-derive-me.json" ]
}

# --- convergence protocol (age-cwo.8 / council C) ---

@test "pawl-review: an ADVERSARIAL run records lineage (diff-hash) for converge" {
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [ -f "$REPO/.agents/pawl-review/age-rev-test.adversarial.json" ]
  grep -q '"diff_hash"' "$REPO/.agents/pawl-review/age-rev-test.adversarial.json"
}

@test "pawl-review: --converge WITHOUT adversarial lineage is advisory-only (exit 4, no verdict)" {
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head --converge
  [ "$status" -eq 4 ]
  [[ "$output" == *"NO adversarial lineage"* ]]
  [ ! -f "$VFILE" ]
}

@test "pawl-review: --converge after the diff CHANGED is advisory-only (exit 4, no verdict)" {
  CODEX_STUB="VERDICT: CONFIRMED" env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head >/dev/null 2>&1
  rm -f "$VFILE"
  echo changed >> README.md; git add README.md; git commit --quiet --amend -m "feat(x): a change (age-rev-test)"
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head --converge
  [ "$status" -eq 4 ]
  [[ "$output" == *"diff CHANGED"* ]]
  [ ! -f "$VFILE" ]
}

@test "pawl-review: --converge requires --scope head (exit 2)" {
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope staged --converge
  [ "$status" -eq 2 ]
  [[ "$output" == *"requires --scope head"* ]]
}

@test "pawl-review: the FULL convergence flow — adversarial REFUTES tail (records lineage), --converge CONFIRMS + writes the verdict" {
  # adversarial REFUTES (cosmetic tail) -> exit 3, NO verdict, but lineage IS recorded
  CODEX_CONVERGE_AWARE=1 run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 3 ]
  [ -f "$REPO/.agents/pawl-review/age-rev-test.adversarial.json" ]
  [ ! -f "$VFILE" ]
  # converge on the SAME diff: calibrated real-safety bar CONFIRMS over the lineaged diff
  CODEX_CONVERGE_AWARE=1 run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head --converge
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  grep -q '"disposition": "CONFIRMED"' "$VFILE"
  # BOTH bars recorded: the adversarial findings are folded into the converge evidence as
  # ACCEPTED-AS-TAIL (the dogfood-caught flaw fix — a REFUTED lineage's defects are auditable).
  grep -q 'ACCEPTED AS TAIL' "$REPO/.agents/pawl-evidence/age-rev-test-pawl-review.txt"
  grep -q 'cosmetic tail nit' "$REPO/.agents/pawl-evidence/age-rev-test-pawl-review.txt"
}

# --- age-rk3r.9: converge lineage DUAL KEY (patch-id + whitespace-significant content sig) ---

# py_setup: replace the default README change with an UNINDENTED Python statement change on a
# fresh commit, so a later reindent produces the SAME patch-id (patch-id is whitespace-
# insensitive) but DIFFERENT content bytes. Prints nothing; resets HEAD_SHA/VFILE targets.
py_setup() {
  git -C "$REPO" reset --quiet --hard HEAD~1                       # drop the default README change
  printf 'def g():\n    return 1\n' > "$REPO/a.py"; git -C "$REPO" add a.py
  git -C "$REPO" commit --quiet -m "base py (age-rev-test)"
  printf 'def g():\n    return 1\nx = 2\n' > "$REPO/a.py"; git -C "$REPO" add a.py   # UNINDENTED
  git -C "$REPO" commit --quiet -m "feat(x): add x unindented (age-rev-test)" --date="2020-01-01T00:00:00"
}

@test "pawl-review: --converge REFUSES a whitespace-only change (same patch-id, different content) — no stale-lineage certify (age-rk3r.9)" {
  py_setup
  # Record adversarial lineage on the UNINDENTED change (full flow: adversarial REFUTES tail
  # but records lineage, then it exists for the converge attempt).
  CODEX_CONVERGE_AWARE=1 run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 3 ]
  [ -f "$REPO/.agents/pawl-review/age-rev-test.adversarial.json" ]
  # the lineage stores BOTH keys.
  grep -q '"diff_hash"' "$REPO/.agents/pawl-review/age-rev-test.adversarial.json"
  grep -q '"content_sig"' "$REPO/.agents/pawl-review/age-rev-test.adversarial.json"

  # REBASE to an INDENTED same-patch-id change (semantically load-bearing whitespace).
  printf 'def g():\n    return 1\n    x = 2\n' > "$REPO/a.py"; git -C "$REPO" add a.py
  git -C "$REPO" commit --quiet --amend -m "feat(x): add x INDENTED (age-rev-test)" --date="2021-06-06T12:00:00"
  # sanity: the patch-id is UNCHANGED (this is the trap the dual key must catch).
  local pid_old pid_new
  pid_old="$(sed -n 's/.*"diff_hash":"\([a-f0-9]*\)".*/\1/p' "$REPO/.agents/pawl-review/age-rev-test.adversarial.json" | head -1)"
  pid_new="$(git -C "$REPO" show HEAD --no-color --no-ext-diff | git patch-id --stable | awk 'NR==1{print $1}')"
  [ "$pid_old" = "$pid_new" ]

  rm -f "$VFILE"
  # --converge must REFUSE (content bytes changed) — NOT certify on the stale lineage.
  CODEX_CONVERGE_AWARE=1 run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head --converge
  [ "$status" -eq 4 ]
  [[ "$output" == *"CONTENT BYTES changed"* ]]
  [ ! -f "$VFILE" ]                                   # no stale-lineage CONFIRMED written
}

@test "pawl-review: --converge REUSES lineage on a TRUE byte-identical rebase (same patch-id AND content) — the positive control (age-rk3r.9)" {
  py_setup
  # Record adversarial lineage on the unindented change.
  CODEX_CONVERGE_AWARE=1 run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 3 ]
  [ -f "$REPO/.agents/pawl-review/age-rev-test.adversarial.json" ]

  # A TRUE byte-identical rebase: re-commit the SAME content with a new date/message only.
  printf 'def g():\n    return 1\nx = 2\n' > "$REPO/a.py"; git -C "$REPO" add a.py
  git -C "$REPO" commit --quiet --amend -m "feat(x): add x unindented REWORDED (age-rev-test)" --date="2021-06-06T12:00:00"
  rm -f "$VFILE"
  # --converge REUSES the lineage (both keys match) and CONFIRMS.
  CODEX_CONVERGE_AWARE=1 run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head --converge
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  grep -q '"disposition": "CONFIRMED"' "$VFILE"
}

@test "pawl-review: --converge REFUSES a no-final-newline change (same patch-id, different bytes) — the shared-lib denylist closes the allowlist gap (age-rk3r.9)" {
  # The 4th byte-category the OLD allowlist content_sig leaked (after whitespace/mode/binary):
  # git's `\ No newline at end of file` marker. Now that pawl-review's --converge content_sig
  # comes from the SHARED scripts/lib/diff-identity.sh (the byte-exact denylist), a tip that
  # drops the final newline (same patch-id, same +/- text) must FAIL the lineage → full review.
  git -C "$REPO" reset --quiet --hard HEAD~1                       # drop the default README change
  printf 'data\n' > "$REPO/a.txt"; git -C "$REPO" add a.txt
  git -C "$REPO" commit --quiet -m "base txt (age-rev-test)"
  printf 'data\nX\n' > "$REPO/a.txt"; git -C "$REPO" add a.txt     # newline-TERMINATED
  git -C "$REPO" commit --quiet -m "feat(x): add X (newline-terminated)" --date="2020-01-01T00:00:00"

  # Record adversarial lineage on the newline-terminated change.
  CODEX_CONVERGE_AWARE=1 run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 3 ]
  [ -f "$REPO/.agents/pawl-review/age-rev-test.adversarial.json" ]

  # Amend to DROP the final newline — same +/- text, SAME patch-id, different diff bytes.
  printf 'data\nX' > "$REPO/a.txt"; git -C "$REPO" add a.txt
  git -C "$REPO" commit --quiet --amend -m "feat(x): add X (no final newline)" --date="2021-06-06T12:00:00"
  # sanity: the patch-id is UNCHANGED (the trap the byte-exact content_sig must catch).
  local pid_old pid_new
  pid_old="$(sed -n 's/.*"diff_hash":"\([a-f0-9]*\)".*/\1/p' "$REPO/.agents/pawl-review/age-rev-test.adversarial.json" | head -1)"
  pid_new="$(git -C "$REPO" show HEAD --no-color --no-ext-diff | git patch-id --stable | awk 'NR==1{print $1}')"
  [ "$pid_old" = "$pid_new" ]

  rm -f "$VFILE"
  CODEX_CONVERGE_AWARE=1 run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head --converge
  [ "$status" -eq 4 ]
  [[ "$output" == *"CONTENT BYTES changed"* ]]
  [ ! -f "$VFILE" ]                                   # no stale-lineage CONFIRMED written
}

# --- ml8.7: route the default pawl through the standing service (opus+codex duel) ---

@test "pawl-review: routes; a CONFIRMED route whose verdict PASSES pawl-verdict.sh check exits 0 (ml8.7)" {
  stub="$(_pawl_service_stub)"
  run env PATH="$BIN:$PATH" PAWL_NO_SERVICE=0 PAWL_SERVICE_SCRIPT="$stub" \
    STUB_HEALTH_RC=0 STUB_ROUTE_RC=0 STUB_ROUTE_DISP=CONFIRMED STUB_VALID=1 \
    bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [[ "$output" == *"routing through the standing pawl-service"* ]]
  [[ "$output" == *"VERIFIED by pawl-verdict.sh check"* ]]
  [ -f "$VFILE" ]
  grep -q CONFIRMED "$VFILE"
}

@test "pawl-review: a routed CONFIRMED with an INVALID verdict FAILS the real check and falls back — never fail-open (ml8.7)" {
  # The route claims rc=0 but writes a minimal/invalid verdict (STUB_VALID=0). The shallow
  # head+disposition test would wrongly accept it; pawl-verdict.sh check REJECTS it, so the
  # routed branch must fall back to the cold codex-exec, not exit 0 on the bad verdict.
  stub="$(_pawl_service_stub)"
  run env PATH="$BIN:$PATH" PAWL_NO_SERVICE=0 PAWL_SERVICE_SCRIPT="$stub" \
    STUB_HEALTH_RC=0 STUB_ROUTE_RC=0 STUB_ROUTE_DISP=CONFIRMED STUB_VALID=0 CODEX_STUB="VERDICT: CONFIRMED" \
    bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [[ "$output" == *"falling back to cold codex-exec"* ]]
}

@test "pawl-review: --scope staged is NEVER routed (review-only, no commit to bind) even with a healthy service (ml8.7)" {
  echo more >> README.md; git add README.md   # an uncommitted staged change
  stub="$(_pawl_service_stub)"
  run env PATH="$BIN:$PATH" PAWL_NO_SERVICE=0 PAWL_SERVICE_SCRIPT="$stub" \
    STUB_HEALTH_RC=0 CODEX_STUB="VERDICT: CONFIRMED" \
    bash "$SCRIPT" age-rev-test --scope staged
  [ "$status" -eq 0 ]
  [[ "$output" != *"routing through the standing pawl-service"* ]]   # routed branch skipped for staged
  [[ "$output" == *"review-only"* ]]
  [ ! -f "$VFILE" ]   # staged never writes a HEAD-bound verdict
}

@test "pawl-review: a routed REFUTED (panes disagree) exits 3 (ml8.7)" {
  stub="$(_pawl_service_stub)"
  run env PATH="$BIN:$PATH" PAWL_NO_SERVICE=0 PAWL_SERVICE_SCRIPT="$stub" \
    STUB_HEALTH_RC=0 STUB_ROUTE_RC=1 STUB_ROUTE_DISP=REFUTED \
    bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 3 ]
  [[ "$output" == *"PAWL ROUTE: REFUTED"* ]]
}

@test "pawl-review: route success-rc but NO head-bound verdict falls back to cold codex-exec — never fail-open (ml8.7)" {
  stub="$(_pawl_service_stub)"
  run env PATH="$BIN:$PATH" PAWL_NO_SERVICE=0 PAWL_SERVICE_SCRIPT="$stub" \
    STUB_HEALTH_RC=0 STUB_ROUTE_RC=0 STUB_ROUTE_WRITES=0 CODEX_STUB="VERDICT: CONFIRMED" \
    bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [[ "$output" == *"falling back to cold codex-exec"* ]]
  [ -f "$VFILE" ]
}

@test "pawl-review: PAWL_NO_SERVICE=1 uses the cold path, never routes (ml8.7 opt-out)" {
  stub="$(_pawl_service_stub)"
  run env PATH="$BIN:$PATH" PAWL_NO_SERVICE=1 PAWL_SERVICE_SCRIPT="$stub" \
    STUB_HEALTH_RC=0 CODEX_STUB="VERDICT: CONFIRMED" \
    bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [[ "$output" != *"routing through the standing pawl-service"* ]]
  [ -f "$VFILE" ]
}
