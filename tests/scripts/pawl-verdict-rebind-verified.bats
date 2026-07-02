#!/usr/bin/env bats
# age-rk3r.9: patch-id REBOUND verdicts — authorize a BYTE-IDENTICAL rebase WITHOUT a
# full re-review. `pawl-verdict.sh rebind-verified` writes a DISTINCT REBOUND verdict
# (lineage: rebound_from_verdict / rebound_from_sha / patch_id_proof) ONLY when BOTH the
# git patch-id --stable of the reviewed diff equals the new tip's AND the full local gate
# is green on the new tip; `check` then authorizes a REBOUND ONLY when its lineage was
# itself CONFIRMED and its patch_id_proof matches the new tip (re-derived from git, so a
# forged proof cannot slip through). Never forgeable as a fresh CONFIRMED.
#
# The tests build a REAL git repo (patch-id is computed from real commits) and stub `ao`
# via PATH (the verdict sensor is exercised elsewhere; here `ao` just logs + exits 0).
# The full-gate step is pinned per-test via PAWL_REBIND_GATE_CMD (`true`/`false`) so the
# gate outcome is deterministic and needs no built binary.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/pawl-verdict.sh"
  GIT="$(command -v git)"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"
  ORIG_DIR="$PWD"
  mkdir -p "$TMP/bin" "$TMP/verdicts"
  # Stub `ao` so the best-effort verdict-edge emit is a no-op (logged, exit 0).
  cat >"$TMP/bin/ao" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "$TMP/bin/ao"
  export PATH="$TMP/bin:$PATH"

  # A real git repo with a base + a "feat" commit whose diff we will re-bind across a rebase.
  R="$TMP/repo"
  mkdir -p "$R"
  "$GIT" -C "$R" init -q
  "$GIT" -C "$R" config user.email t@t.com
  "$GIT" -C "$R" config user.name t
  printf 'base\n' > "$R/f.txt"
  "$GIT" -C "$R" add f.txt
  "$GIT" -C "$R" commit -qm base
  printf 'the-change\n' >> "$R/f.txt"
  "$GIT" -C "$R" add f.txt
  "$GIT" -C "$R" commit -qm "feat: the change" --date="2020-01-01T00:00:00"
  SHA_A="$("$GIT" -C "$R" rev-parse HEAD)"

  # A prior CONFIRMED verdict binding SHA_A, with a real (non-empty) evidence file.
  printf 'file.go:42 reviewed — no blocking defects\nfiles reviewed: 3\n' > "$TMP/ev.txt"
  AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 bash "$SCRIPT" write mybead 0 \
    --disposition CONFIRMED --head "$SHA_A" --author-context author-ctx \
    --refuter claude:CONFIRMED:fresh-ctx:"$TMP/ev.txt" --dir "$TMP/verdicts" >/dev/null 2>&1
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

# rebase_identical: reset one commit back and re-apply the SAME diff with a new date +
# message → a new sha whose patch-id equals SHA_A's. Prints the new sha.
rebase_identical() {
  "$GIT" -C "$R" reset -q --hard HEAD~1
  printf 'the-change\n' >> "$R/f.txt"
  "$GIT" -C "$R" add f.txt
  "$GIT" -C "$R" commit -qm "feat: the change (rebased, reworded)" --date="2021-06-06T12:00:00" >/dev/null
  "$GIT" -C "$R" rev-parse HEAD
}

# ---------------------------------------------------------------------------
# ACCEPTANCE 1 — a CONFIRMED at sha A + a rebase to sha B with IDENTICAL patch-id and a
# GREEN gate → rebind-verified writes a REBOUND binding B with lineage to A, and `check`
# authorizes the push.
# ---------------------------------------------------------------------------
@test "identical-rebase + green gate: rebind-verified writes a REBOUND and check AUTHORIZES" {
  local SHA_B; SHA_B="$(rebase_identical)"
  [ "$SHA_B" != "$SHA_A" ]

  run env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_GATE_CMD="true" \
    bash "$SCRIPT" rebind-verified mybead 0 --head "$SHA_B" --dir "$TMP/verdicts" --repo-root "$R"
  [ "$status" -eq 0 ]

  local out="$TMP/verdicts/mybead.json"
  [ "$(jq -r .disposition "$out")" = "REBOUND" ]
  [ "$(jq -r .head_sha "$out")" = "$SHA_B" ]
  # lineage present: rebound_from_verdict (points at the archived CONFIRMED), rebound_from_sha
  # = the reviewed sha, patch_id_proof = the shared patch-id.
  [ "$(jq -r .rebound_from_sha "$out")" = "$SHA_A" ]
  [ -n "$(jq -r '.patch_id_proof // ""' "$out")" ]
  local lineage; lineage="$(jq -r .rebound_from_verdict "$out")"
  [ -f "$lineage" ]
  [ "$(jq -r .disposition "$lineage")" = "CONFIRMED" ]

  # check authorizes the merge for the new tip.
  run env AGENTOPS_REPO_ROOT="$R" bash "$SCRIPT" check mybead 0 --dir "$TMP/verdicts" --head "$SHA_B"
  [ "$status" -eq 0 ]
  [[ "$output" == *"merge authorized"* ]]
}

# ---------------------------------------------------------------------------
# ACCEPTANCE 2 — ANY hunk difference (patch-id differs) → rebind REFUSES naming the first
# differing file — fail-closed, no REBOUND written.
# ---------------------------------------------------------------------------
@test "one-byte hunk change: rebind-verified REFUSES naming the differing file (no REBOUND)" {
  # A DIFFERENT diff on a new commit — one extra line → a different patch-id.
  "$GIT" -C "$R" reset -q --hard HEAD~1
  printf 'the-change\n' >> "$R/f.txt"
  printf 'EXTRA-LINE\n' >> "$R/f.txt"
  "$GIT" -C "$R" add f.txt
  "$GIT" -C "$R" commit -qm "feat: change plus extra" >/dev/null
  local SHA_C; SHA_C="$("$GIT" -C "$R" rev-parse HEAD)"

  run env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_SKIP_GATE=1 \
    bash "$SCRIPT" rebind-verified mybead 0 --head "$SHA_C" --dir "$TMP/verdicts" --repo-root "$R"
  [ "$status" -ne 0 ]
  [[ "$output" == *"patch-id MISMATCH"* ]]
  [[ "$output" == *"f.txt"* ]]
  # the verdict is UNCHANGED — still the original CONFIRMED, no REBOUND written.
  [ "$(jq -r .disposition "$TMP/verdicts/mybead.json")" = "CONFIRMED" ]
}

# ---------------------------------------------------------------------------
# ACCEPTANCE 3 — gate RED on B → rebind REFUSES (no REBOUND written) even with a matching
# patch-id. The gate re-running the tests on the new base is the behavior check.
# ---------------------------------------------------------------------------
@test "red gate on the new tip: rebind-verified REFUSES even with a matching patch-id (no REBOUND)" {
  local SHA_B; SHA_B="$(rebase_identical)"

  run env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_GATE_CMD="false" \
    bash "$SCRIPT" rebind-verified mybead 0 --head "$SHA_B" --dir "$TMP/verdicts" --repo-root "$R"
  [ "$status" -ne 0 ]
  [[ "$output" == *"gate is RED"* ]]
  [[ "$output" == *"semantic conflict"* ]]
  # no REBOUND written — the verdict is still the original CONFIRMED.
  [ "$(jq -r .disposition "$TMP/verdicts/mybead.json")" = "CONFIRMED" ]
}

# ---------------------------------------------------------------------------
# ACCEPTANCE 4a — a REBOUND whose lineage (rebound_from_verdict) was NOT CONFIRMED →
# check does NOT authorize (fail-closed).
# ---------------------------------------------------------------------------
@test "check: a REBOUND whose lineage is not CONFIRMED does NOT authorize" {
  local SHA_B; SHA_B="$(rebase_identical)"
  env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_GATE_CMD="true" \
    bash "$SCRIPT" rebind-verified mybead 0 --head "$SHA_B" --dir "$TMP/verdicts" --repo-root "$R" >/dev/null 2>&1
  local out="$TMP/verdicts/mybead.json"
  [ "$(jq -r .disposition "$out")" = "REBOUND" ]

  # Corrupt the lineage root to REFUTED — check must refuse.
  local lineage; lineage="$(jq -r .rebound_from_verdict "$out")"
  jq '.disposition="REFUTED"' "$lineage" > "$lineage.tmp" && mv "$lineage.tmp" "$lineage"

  run env AGENTOPS_REPO_ROOT="$R" bash "$SCRIPT" check mybead 0 --dir "$TMP/verdicts" --head "$SHA_B"
  [ "$status" -ne 0 ]
  [[ "$output" != *"merge authorized"* ]]
  [[ "$output" == *"not CONFIRMED"* ]]
}

# ---------------------------------------------------------------------------
# ACCEPTANCE 4b — a REBOUND whose patch_id_proof is absent/mismatched → check does NOT
# authorize (fail-closed). Proof is re-derived from git, so a forged string is caught.
# ---------------------------------------------------------------------------
@test "check: a REBOUND with a mismatched patch_id_proof does NOT authorize" {
  local SHA_B; SHA_B="$(rebase_identical)"
  env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_GATE_CMD="true" \
    bash "$SCRIPT" rebind-verified mybead 0 --head "$SHA_B" --dir "$TMP/verdicts" --repo-root "$R" >/dev/null 2>&1
  local out="$TMP/verdicts/mybead.json"

  # Forge the patch_id_proof to a bogus value on an OTHERWISE-identical rebase. The reviewed-vs-tip
  # equivalence passes (it IS a true rebase), so the DEFENSE-IN-DEPTH proof-consistency check fires:
  # the recorded proof disagrees with the git-re-derived tip patch-id → refuse.
  jq '.patch_id_proof="deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"' "$out" > "$out.tmp" && mv "$out.tmp" "$out"
  run env AGENTOPS_REPO_ROOT="$R" bash "$SCRIPT" check mybead 0 --dir "$TMP/verdicts" --head "$SHA_B"
  [ "$status" -ne 0 ]
  [[ "$output" != *"merge authorized"* ]]
  [[ "$output" == *"disagrees with the re-derived current tip patch-id"* ]]
}

@test "check: a REBOUND missing patch_id_proof entirely does NOT authorize" {
  local SHA_B; SHA_B="$(rebase_identical)"
  env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_GATE_CMD="true" \
    bash "$SCRIPT" rebind-verified mybead 0 --head "$SHA_B" --dir "$TMP/verdicts" --repo-root "$R" >/dev/null 2>&1
  local out="$TMP/verdicts/mybead.json"
  jq 'del(.patch_id_proof)' "$out" > "$out.tmp" && mv "$out.tmp" "$out"
  run env AGENTOPS_REPO_ROOT="$R" bash "$SCRIPT" check mybead 0 --dir "$TMP/verdicts" --head "$SHA_B"
  [ "$status" -ne 0 ]
  [[ "$output" != *"merge authorized"* ]]
  [[ "$output" == *"missing patch_id_proof"* ]]
}

# ---------------------------------------------------------------------------
# rebind-verified refuses to descend from a non-CONFIRMED prior verdict (write-time guard,
# complementing the check-time lineage guard above).
# ---------------------------------------------------------------------------
@test "rebind-verified refuses when the prior verdict is not CONFIRMED" {
  # Rewrite the prior verdict to REFUTED (a REBOUND may only descend from CONFIRMED).
  jq '.disposition="REFUTED"' "$TMP/verdicts/mybead.json" > "$TMP/verdicts/mybead.json.tmp" \
    && mv "$TMP/verdicts/mybead.json.tmp" "$TMP/verdicts/mybead.json"
  local SHA_B; SHA_B="$(rebase_identical)"
  run env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_SKIP_GATE=1 \
    bash "$SCRIPT" rebind-verified mybead 0 --head "$SHA_B" --dir "$TMP/verdicts" --repo-root "$R"
  [ "$status" -ne 0 ]
  [[ "$output" == *"not CONFIRMED"* ]]
}

# ---------------------------------------------------------------------------
# The --rebind CAVEAT is present in the rebind-verified help text VERBATIM (the bead
# requires it stated in both the code comment and the help).
# ---------------------------------------------------------------------------
@test "help text carries the accurate 3-condition CAVEAT (patch-id whitespace-insensitive)" {
  run bash "$SCRIPT" --help
  # The caveat is line-wrapped in the help block; assert the key phrases (each fits on one
  # line) rather than a span that crosses a wrap boundary. The accurate claim names the
  # whitespace-insensitivity of patch-id and the three required conditions.
  [[ "$output" == *"WHITESPACE-"* ]]
  [[ "$output" == *"byte-identical +/- content lines"* ]]
  [[ "$output" == *"green full gate on the new tip"* ]]
}

# ===========================================================================
# DEFECT 1 (RED-first) — patch-id is WHITESPACE-INSENSITIVE. A rebase whose ONLY change is
# added leading whitespace on a content line (e.g. Python indentation, semantically load-
# bearing) produces the SAME patch-id but DIFFERENT diff bytes. rebind-verified must REFUSE
# it via the whitespace-significant content-line check; check must not authorize a REBOUND
# whose tip content bytes differ from the reviewed commit. Proven RED against the pre-fix
# code (patch-id match alone authorized).
# ===========================================================================
@test "DEFECT1: a whitespace-only rebase (same patch-id, different bytes) is REFUSED" {
  # Reviewed commit adds an UNINDENTED statement; the "rebase" adds the SAME statement but
  # INDENTED — identical patch-id, different content bytes.
  "$GIT" -C "$R" reset -q --hard HEAD~1
  # First restore the reviewed (unindented) state so the CONFIRMED verdict's sha still exists,
  # then build the whitespace-variant as the new tip.
  # (setup already left SHA_A = "the-change" appended unindented.)
  # Build a python-shaped file to make the whitespace semantically meaningful + obvious.
  printf 'def g():\n    return 1\n' > "$R/a.py"
  "$GIT" -C "$R" add a.py
  "$GIT" -C "$R" commit -qm "base py" >/dev/null
  printf 'def g():\n    return 1\nx = 2\n' > "$R/a.py"   # UNINDENTED — the reviewed change
  "$GIT" -C "$R" add a.py
  "$GIT" -C "$R" commit -qm "feat: add x (reviewed)" --date="2020-01-02T00:00:00" >/dev/null
  local SHA_REV; SHA_REV="$("$GIT" -C "$R" rev-parse HEAD)"
  # A fresh CONFIRMED binding the reviewed (unindented) commit.
  printf 'a.py:3 reviewed the added statement\nfiles reviewed: 1\n' > "$TMP/ev2.txt"
  AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 bash "$SCRIPT" write wsbead 0 \
    --disposition CONFIRMED --head "$SHA_REV" --author-context author-ctx \
    --refuter claude:CONFIRMED:fresh-ctx:"$TMP/ev2.txt" --dir "$TMP/verdicts" >/dev/null 2>&1

  # Rebase to the INDENTED variant — SAME patch-id, DIFFERENT bytes.
  "$GIT" -C "$R" reset -q --hard HEAD~1
  printf 'def g():\n    return 1\n    x = 2\n' > "$R/a.py"   # INDENTED — a real semantic change
  "$GIT" -C "$R" add a.py
  "$GIT" -C "$R" commit -qm "feat: add x indented (rebase)" --date="2021-06-06T12:00:00" >/dev/null
  local SHA_WS; SHA_WS="$("$GIT" -C "$R" rev-parse HEAD)"

  # Precondition: the two commits DO share a patch-id (the trap). If not, this test is not
  # exercising the whitespace hole.
  local p_rev p_ws
  p_rev="$("$GIT" -C "$R" show "$SHA_REV" --no-color | "$GIT" patch-id --stable | cut -d' ' -f1)"
  p_ws="$("$GIT" -C "$R" show "$SHA_WS" --no-color | "$GIT" patch-id --stable | cut -d' ' -f1)"
  [ "$p_rev" = "$p_ws" ]

  # rebind-verified must REFUSE (content-line mismatch), even with the gate skipped.
  run env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_SKIP_GATE=1 \
    bash "$SCRIPT" rebind-verified wsbead 0 --head "$SHA_WS" --dir "$TMP/verdicts" --repo-root "$R"
  [ "$status" -ne 0 ]
  [[ "$output" == *"CONTENT-LINE MISMATCH"* ]]
  # no REBOUND written — still the reviewed CONFIRMED.
  [ "$(jq -r .disposition "$TMP/verdicts/wsbead.json")" = "CONFIRMED" ]
}

# ===========================================================================
# DEFECT 2 (RED-first) — a REBOUND must be NO EASIER to authorize than the CONFIRMED it
# inherits. A thin/self-stamped CONFIRMED that lacks evidence (or fresh-context) does NOT
# itself authorize a merge, so it must not be laundered into an authorizing REBOUND.
# rebind-verified must REFUSE to build from it; check must REFUSE a REBOUND with such a
# lineage AND a REBOUND whose own panel fails the gates. Proven RED against the pre-fix code
# (the REBOUND path returned success BEFORE the roster/fresh-context/evidence/floor gates,
# and lineage was only `.disposition=="CONFIRMED"`).
# ===========================================================================
@test "DEFECT2: a thin (evidence-less / non-fresh) CONFIRMED cannot be laundered into a REBOUND" {
  # Hand-write a THIN CONFIRMED: schema-valid, but NO refuter evidence and the refuter ran
  # in the AUTHOR's own context (not a fresh red-team) — so it would NOT itself authorize.
  local SHA_A2; SHA_A2="$("$GIT" -C "$R" rev-parse HEAD)"   # setup's reviewed commit
  cat > "$TMP/verdicts/thinbead.json" <<EOF
{"schema_version":"pawl-verdict.v1","bead_id":"thinbead","pr":0,"disposition":"CONFIRMED","generated_at":"2026-01-01T00:00:00Z","author_context_id":"same-ctx","attempt":1,"refuters":[{"family":"claude","verdict":"CONFIRMED","context_id":"same-ctx"}],"head_sha":"$SHA_A2"}
EOF
  # Sanity: the thin CONFIRMED does NOT itself authorize (no evidence + not fresh-context).
  run env AGENTOPS_REPO_ROOT="$R" bash "$SCRIPT" check thinbead 0 --dir "$TMP/verdicts" --head "$SHA_A2"
  [ "$status" -ne 0 ]

  # A genuine identical rebase of the reviewed commit.
  local SHA_B; SHA_B="$(rebase_identical)"

  # (a) rebind-verified must REFUSE to build a REBOUND from the thin lineage.
  run env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_SKIP_GATE=1 \
    bash "$SCRIPT" rebind-verified thinbead 0 --head "$SHA_B" --dir "$TMP/verdicts" --repo-root "$R"
  [ "$status" -ne 0 ]
  [[ "$output" == *"NOT a fully-valid authorizing CONFIRMED"* ]]
  [ "$(jq -r .disposition "$TMP/verdicts/thinbead.json")" = "CONFIRMED" ]   # no REBOUND written

  # (b) check must ALSO refuse a hand-forged REBOUND that points at the thin lineage with a
  #     VALID patch-id proof + content match (the laundering attempt directly at the gate).
  local pid; pid="$("$GIT" -C "$R" show "$SHA_B" --no-color | "$GIT" patch-id --stable | cut -d' ' -f1)"
  cp "$TMP/verdicts/thinbead.json" "$TMP/verdicts/thin-lineage.json"
  cat > "$TMP/verdicts/laundered.json" <<EOF
{"schema_version":"pawl-verdict.v1","bead_id":"laundered","pr":0,"disposition":"REBOUND","generated_at":"2026-01-02T00:00:00Z","author_context_id":"same-ctx","attempt":1,"refuters":[{"family":"claude","verdict":"CONFIRMED","context_id":"same-ctx"}],"head_sha":"$SHA_B","rebound_from_verdict":"$TMP/verdicts/thin-lineage.json","rebound_from_sha":"$SHA_B","patch_id_proof":"$pid"}
EOF
  run env AGENTOPS_REPO_ROOT="$R" bash "$SCRIPT" check laundered 0 --dir "$TMP/verdicts" --head "$SHA_B"
  [ "$status" -ne 0 ]
  [[ "$output" != *"merge authorized"* ]]
}

# ===========================================================================
# HEAD==newhead GUARD (RED-first) — the required green gate runs against the worktree's
# CURRENT HEAD, not against --head directly. With --head B while the worktree is checked out
# at an UNRELATED commit C and a gate that passes only on C, the gate validates the WRONG
# tree yet a REBOUND for B would be stamped — violating "the gate is green on the NEW TIP".
# rebind-verified must REFUSE fail-closed when HEAD != the rebound tip. Proven RED against
# the pre-fix code (exit 0 + REBOUND for B though the gate ran on C).
# ===========================================================================
@test "HEAD-guard: --head B while worktree HEAD is at unrelated C (gate passes on C) is REFUSED" {
  # setup left SHA_A = the reviewed commit and a fresh CONFIRMED at SHA_A. Rebase to B.
  local SHA_B; SHA_B="$(rebase_identical)"
  # Now check out an UNRELATED commit C (a divergent branch off the base), so HEAD != B.
  "$GIT" -C "$R" checkout -q -b other HEAD~1
  printf 'unrelated\n' > "$R/g.txt"; "$GIT" -C "$R" add g.txt
  "$GIT" -C "$R" commit -q -m "unrelated C"
  local SHA_C; SHA_C="$("$GIT" -C "$R" rev-parse HEAD)"
  [ "$SHA_C" != "$SHA_B" ]

  # A gate that passes ONLY when the worktree HEAD is C (i.e. it ran against the wrong tree).
  local gate="[ \"\$('$GIT' -C '$R' rev-parse HEAD)\" = \"$SHA_C\" ]"
  run env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_GATE_CMD="$gate" \
    bash "$SCRIPT" rebind-verified mybead 0 --head "$SHA_B" --dir "$TMP/verdicts" --repo-root "$R"
  [ "$status" -ne 0 ]
  [[ "$output" == *"gate must run against the rebound tip"* ]]
  [[ "$output" == *"check out"* ]]
  # no REBOUND written — the verdict is still the reviewed CONFIRMED.
  [ "$(jq -r .disposition "$TMP/verdicts/mybead.json")" = "CONFIRMED" ]
}

@test "HEAD-guard: with HEAD checked out AT the rebound tip, the gate runs and the REBOUND is written" {
  # The positive control for the guard: HEAD == --head (the normal post-rebase state) → the
  # gate runs against the rebound tip and (passing) authorizes the REBOUND.
  local SHA_B; SHA_B="$(rebase_identical)"
  [ "$("$GIT" -C "$R" rev-parse HEAD)" = "$SHA_B" ]   # HEAD is at the rebound tip
  run env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_GATE_CMD="true" \
    bash "$SCRIPT" rebind-verified mybead 0 --head "$SHA_B" --dir "$TMP/verdicts" --repo-root "$R"
  [ "$status" -eq 0 ]
  [ "$(jq -r .disposition "$TMP/verdicts/mybead.json")" = "REBOUND" ]
  [ "$(jq -r .head_sha "$TMP/verdicts/mybead.json")" = "$SHA_B" ]
}

# ===========================================================================
# MODE / BINARY EQUIVALENCE (RED-first, age-rk3r.9) — the REBOUND equivalence proof must be
# FILE-MODE and BINARY aware, and must prove the reviewed commit (rebound_from_sha) IS
# EQUIVALENT to the tip (not merely stored-proof==tip). A tip that differs from the reviewed
# commit ONLY by file mode (e.g. a data file flipped 100644→100755, made EXECUTABLE) or by
# binary blob content must be REFUSED — patch-id/content-lines used to miss it.
# ===========================================================================

# md_setup <mode-args>: replace the default text change with a `f.dat` change that adds a line;
# the caller's mode-args ("--chmod=+x" or "") decide whether the reviewed commit also flips the
# mode. Writes a fresh CONFIRMED lineage at the reviewed sha. Prints the reviewed sha.
md_write_reviewed() {
  local chmod_flag="$1"
  "$GIT" -C "$R" reset -q --hard HEAD~1                       # drop the default text change
  printf 'data\n' > "$R/f.dat"; "$GIT" -C "$R" add f.dat
  "$GIT" -C "$R" commit -qm "base dat" >/dev/null
  printf 'data\nX\n' > "$R/f.dat"
  if [ -n "$chmod_flag" ]; then "$GIT" -C "$R" add "$chmod_flag" f.dat; else "$GIT" -C "$R" add f.dat; fi
  "$GIT" -C "$R" commit -qm "reviewed: add X${chmod_flag:+ +chmod}" --date="2020-01-01T00:00:00" >/dev/null
  local rev; rev="$("$GIT" -C "$R" rev-parse HEAD)"
  printf 'f.dat:2 reviewed the added line\nfiles reviewed: 1\n' > "$TMP/ev2.txt"
  AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 bash "$SCRIPT" write mdbead 0 \
    --disposition CONFIRMED --head "$rev" --author-context author-ctx \
    --refuter claude:CONFIRMED:fresh-ctx:"$TMP/ev2.txt" --dir "$TMP/verdicts" >/dev/null 2>&1
  printf '%s' "$rev"
}

@test "MODE: a tip that drops the reviewed commit's chmod (100755→100644) is REFUSED by rebind-verified (content/mode signature)" {
  local REV; REV="$(md_write_reviewed --chmod=+x)"          # reviewed: text + made executable
  # tip: SAME text, NO chmod (stays 100644) — a real, security-relevant difference.
  "$GIT" -C "$R" reset -q --hard HEAD~1
  printf 'data\nX\n' > "$R/f.dat"; "$GIT" -C "$R" add f.dat
  "$GIT" -C "$R" commit -qm "tip: add X (no chmod)" --date="2021-06-06T12:00:00" >/dev/null
  local TIP; TIP="$("$GIT" -C "$R" rev-parse HEAD)"

  run env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_SKIP_GATE=1 \
    bash "$SCRIPT" rebind-verified mdbead 0 --head "$TIP" --dir "$TMP/verdicts" --repo-root "$R"
  [ "$status" -ne 0 ]
  # patch-id (mode present) OR the mode-aware content signature refuses — either way, no REBOUND.
  [[ "$output" == *"MISMATCH"* ]]
  [ "$(jq -r .disposition "$TMP/verdicts/mdbead.json")" = "CONFIRMED" ]
}

@test "MODE: a hand-forged REBOUND (proof set to the tip patch-id) whose tip drops the reviewed chmod is REFUSED by check" {
  # This is the FORGED-PROOF path: check must prove reviewed==tip from git, not trust the proof.
  local REV; REV="$(md_write_reviewed --chmod=+x)"
  "$GIT" -C "$R" reset -q --hard HEAD~1
  printf 'data\nX\n' > "$R/f.dat"; "$GIT" -C "$R" add f.dat
  "$GIT" -C "$R" commit -qm "tip: add X (no chmod)" --date="2021-06-06T12:00:00" >/dev/null
  local TIP; TIP="$("$GIT" -C "$R" rev-parse HEAD)"
  local tip_pid; tip_pid="$("$GIT" -C "$R" show "$TIP" --no-color --no-ext-diff | "$GIT" patch-id --stable | awk 'NR==1{print $1}')"

  # Archive the CONFIRMED lineage at the reviewed sha, and hand-write a REBOUND with the FORGED
  # proof = the tip's own patch-id (the bypass the reviewed-vs-tip comparison must defeat).
  cp "$TMP/verdicts/mdbead.json" "$TMP/verdicts/mdbead.confirmed.json"
  cat > "$TMP/verdicts/mdbead.json" <<EOF
{"schema_version":"pawl-verdict.v1","bead_id":"mdbead","pr":0,"disposition":"REBOUND","generated_at":"2026-01-02T00:00:00Z","author_context_id":"author-ctx","attempt":1,"refuters":[{"family":"claude","verdict":"CONFIRMED","context_id":"fresh-ctx","evidence":"$TMP/ev2.txt"}],"head_sha":"$TIP","rebound_from_verdict":"$TMP/verdicts/mdbead.confirmed.json","rebound_from_sha":"$REV","patch_id_proof":"$tip_pid"}
EOF
  run env AGENTOPS_REPO_ROOT="$R" bash "$SCRIPT" check mdbead 0 --dir "$TMP/verdicts" --head "$TIP"
  [ "$status" -ne 0 ]
  [[ "$output" != *"merge authorized"* ]]
  [[ "$output" == *"NOT the same change"* || "$output" == *"signature differs"* ]]
}

@test "BINARY: a tip whose binary blob differs from the reviewed commit's (same size) is REFUSED by rebind-verified" {
  # reviewed adds binary blob X; tip adds a DIFFERENT same-size binary blob Y.
  "$GIT" -C "$R" reset -q --hard HEAD~1
  printf 'readme\n' > "$R/readme"; "$GIT" -C "$R" add readme; "$GIT" -C "$R" commit -qm "base bin" >/dev/null
  printf '\x00\x01\x02\x03' > "$R/b.bin"; "$GIT" -C "$R" add b.bin
  "$GIT" -C "$R" commit -qm "reviewed: binary X" --date="2020-01-01T00:00:00" >/dev/null
  local REV; REV="$("$GIT" -C "$R" rev-parse HEAD)"
  printf 'b.bin: binary reviewed\nfiles reviewed: 1\n' > "$TMP/evb.txt"
  AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 bash "$SCRIPT" write binbead 0 \
    --disposition CONFIRMED --head "$REV" --author-context author-ctx \
    --refuter claude:CONFIRMED:fresh-ctx:"$TMP/evb.txt" --dir "$TMP/verdicts" >/dev/null 2>&1
  "$GIT" -C "$R" reset -q --hard HEAD~1
  printf '\x00\x01\x02\xFF' > "$R/b.bin"; "$GIT" -C "$R" add b.bin
  "$GIT" -C "$R" commit -qm "tip: binary Y" --date="2021-06-06T12:00:00" >/dev/null
  local TIP; TIP="$("$GIT" -C "$R" rev-parse HEAD)"

  run env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_SKIP_GATE=1 \
    bash "$SCRIPT" rebind-verified binbead 0 --head "$TIP" --dir "$TMP/verdicts" --repo-root "$R"
  [ "$status" -ne 0 ]
  [ "$(jq -r .disposition "$TMP/verdicts/binbead.json")" = "CONFIRMED" ]
}

@test "MODE positive control: a TRUE equivalent rebase (same content AND same mode) still authorizes" {
  local REV; REV="$(md_write_reviewed "")"                   # reviewed: text only, NO chmod
  # tip: SAME text, SAME (default) mode — a genuine byte-identical rebase.
  "$GIT" -C "$R" reset -q --hard HEAD~1
  printf 'data\nX\n' > "$R/f.dat"; "$GIT" -C "$R" add f.dat
  "$GIT" -C "$R" commit -qm "tip: add X reworded" --date="2021-06-06T12:00:00" >/dev/null
  local TIP; TIP="$("$GIT" -C "$R" rev-parse HEAD)"

  run env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_GATE_CMD="true" \
    bash "$SCRIPT" rebind-verified mdbead 0 --head "$TIP" --dir "$TMP/verdicts" --repo-root "$R"
  [ "$status" -eq 0 ]
  [ "$(jq -r .disposition "$TMP/verdicts/mdbead.json")" = "REBOUND" ]
  [ "$(jq -r .head_sha "$TMP/verdicts/mdbead.json")" = "$TIP" ]
  # and check authorizes it.
  run env AGENTOPS_REPO_ROOT="$R" bash "$SCRIPT" check mdbead 0 --dir "$TMP/verdicts" --head "$TIP"
  [ "$status" -eq 0 ]
  [[ "$output" == *"merge authorized"* ]]
}

# ===========================================================================
# NO-NEWLINE (RED-first, age-rk3r.9 denylist refactor) — git's `\ No newline at end of file`
# marker is the 4th byte-category the OLD keep-only allowlist forgot (after whitespace, mode,
# binary). A tip whose diff drops the final newline has the SAME patch-id AND the SAME +/-
# text as the reviewed commit, but a DIFFERENT diff (the `\ No newline` marker). The denylist
# content signature keeps that marker byte-exact, so rebind-verified + check REFUSE.
# ===========================================================================
@test "NO-NEWLINE: a tip missing the final newline (same patch-id, same +/- text) is REFUSED by check" {
  # reviewed: add a line WITH a trailing newline.
  "$GIT" -C "$R" reset -q --hard HEAD~1
  printf 'data\n' > "$R/f.dat"; "$GIT" -C "$R" add f.dat; "$GIT" -C "$R" commit -qm "base dat" >/dev/null
  printf 'data\nX\n' > "$R/f.dat"; "$GIT" -C "$R" add f.dat
  "$GIT" -C "$R" commit -qm "reviewed: add X (newline-terminated)" --date="2020-01-01T00:00:00" >/dev/null
  local REV; REV="$("$GIT" -C "$R" rev-parse HEAD)"
  printf 'f.dat:2 reviewed\nfiles reviewed: 1\n' > "$TMP/evn.txt"
  # A fully-valid CONFIRMED lineage at the reviewed sha.
  cat > "$TMP/verdicts/nlbead.confirmed.json" <<EOF
{"schema_version":"pawl-verdict.v1","bead_id":"nlbead","pr":0,"disposition":"CONFIRMED","generated_at":"2026-01-01T00:00:00Z","author_context_id":"author-ctx","attempt":1,"refuters":[{"family":"claude","verdict":"CONFIRMED","context_id":"fresh-ctx","evidence":"$TMP/evn.txt"}],"head_sha":"$REV"}
EOF
  # tip: add the SAME line WITHOUT a trailing newline — same patch-id, same +/- text.
  "$GIT" -C "$R" reset -q --hard HEAD~1
  printf 'data\nX' > "$R/f.dat"; "$GIT" -C "$R" add f.dat
  "$GIT" -C "$R" commit -qm "tip: add X (no final newline)" --date="2021-06-06T12:00:00" >/dev/null
  local TIP; TIP="$("$GIT" -C "$R" rev-parse HEAD)"
  # sanity: the patch-ids MATCH (the trap the byte-exact signature must catch).
  local p_rev p_tip
  p_rev="$("$GIT" -C "$R" show "$REV" --no-color --no-ext-diff | "$GIT" patch-id --stable | awk 'NR==1{print $1}')"
  p_tip="$("$GIT" -C "$R" show "$TIP" --no-color --no-ext-diff | "$GIT" patch-id --stable | awk 'NR==1{print $1}')"
  [ "$p_rev" = "$p_tip" ]

  # A hand-forged REBOUND with proof = the tip patch-id (the bypass), lineage = the newline-
  # terminated reviewed commit. check must REFUSE (byte-exact content signature differs).
  cat > "$TMP/verdicts/nlbead.json" <<EOF
{"schema_version":"pawl-verdict.v1","bead_id":"nlbead","pr":0,"disposition":"REBOUND","generated_at":"2026-01-02T00:00:00Z","author_context_id":"author-ctx","attempt":1,"refuters":[{"family":"claude","verdict":"CONFIRMED","context_id":"fresh-ctx","evidence":"$TMP/evn.txt"}],"head_sha":"$TIP","rebound_from_verdict":"$TMP/verdicts/nlbead.confirmed.json","rebound_from_sha":"$REV","patch_id_proof":"$p_tip"}
EOF
  run env AGENTOPS_REPO_ROOT="$R" bash "$SCRIPT" check nlbead 0 --dir "$TMP/verdicts" --head "$TIP"
  [ "$status" -ne 0 ]
  [[ "$output" != *"merge authorized"* ]]
  [[ "$output" == *"byte-exact content signature differs"* ]]
}

@test "NO-NEWLINE: rebind-verified also REFUSES a tip that drops the final newline" {
  # reviewed: newline-terminated line; write a real CONFIRMED via the production writer.
  "$GIT" -C "$R" reset -q --hard HEAD~1
  printf 'data\n' > "$R/f.dat"; "$GIT" -C "$R" add f.dat; "$GIT" -C "$R" commit -qm "base dat" >/dev/null
  printf 'data\nX\n' > "$R/f.dat"; "$GIT" -C "$R" add f.dat
  "$GIT" -C "$R" commit -qm "reviewed: add X (newline-terminated)" --date="2020-01-01T00:00:00" >/dev/null
  local REV; REV="$("$GIT" -C "$R" rev-parse HEAD)"
  printf 'f.dat:2 reviewed\nfiles reviewed: 1\n' > "$TMP/evn.txt"
  AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 bash "$SCRIPT" write nlbead2 0 \
    --disposition CONFIRMED --head "$REV" --author-context author-ctx \
    --refuter claude:CONFIRMED:fresh-ctx:"$TMP/evn.txt" --dir "$TMP/verdicts" >/dev/null 2>&1
  # tip: same line, no final newline.
  "$GIT" -C "$R" reset -q --hard HEAD~1
  printf 'data\nX' > "$R/f.dat"; "$GIT" -C "$R" add f.dat
  "$GIT" -C "$R" commit -qm "tip: add X (no final newline)" --date="2021-06-06T12:00:00" >/dev/null
  local TIP; TIP="$("$GIT" -C "$R" rev-parse HEAD)"

  run env AGENTOPS_REPO_ROOT="$R" PAWL_AUTOBIND=0 PAWL_REBIND_SKIP_GATE=1 \
    bash "$SCRIPT" rebind-verified nlbead2 0 --head "$TIP" --dir "$TMP/verdicts" --repo-root "$R"
  [ "$status" -ne 0 ]
  [[ "$output" == *"CONTENT-LINE MISMATCH"* ]]
  [ "$(jq -r .disposition "$TMP/verdicts/nlbead2.json")" = "CONFIRMED" ]   # no REBOUND written
}
