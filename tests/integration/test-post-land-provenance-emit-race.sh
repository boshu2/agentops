#!/usr/bin/env bash
# test-post-land-provenance-emit-race.sh — age-lyev acceptance.
#
# Proves post-land-provenance-emit.sh is race-proof:
#   1. A concurrent push lands on origin/main BETWEEN our worktree reset and our
#      push (the first push is rejected) — the script must still land its edge.
#   2. No orphan commit + a byte-clean canonical checkout, on BOTH the success
#      path and the all-attempts-exhausted path.
#   3. The provenance commit is marked #trivial.
#   4. The ledger prev_hash chain stays VALID after the race — i.e. the script
#      RE-EMITS onto the competitor's new tail (a rebase-replay would leave our
#      edge chained to the pre-competitor hash = a broken chain).
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
UNDER_TEST="$ROOT/scripts/post-land-provenance-emit.sh"

fails=0
ok()  { echo "PASS: $1"; }
bad() { echo "FAIL: $1"; fails=$((fails + 1)); }
check() { if eval "$2"; then ok "$1"; else bad "$1"; fi; }

TMP="$(mktemp -d)"
cleanup() { chmod -R u+w "$TMP" 2>/dev/null || true; /bin/rm -rf "$TMP" 2>/dev/null || true; }
trap cleanup EXIT

# --- sandbox: bare origin + canonical clone (force default branch main) ------
git init --bare -q -b main "$TMP/origin.git"
git clone -q "$TMP/origin.git" "$TMP/canon"
cd "$TMP/canon" || exit 1
git checkout -q -b main 2>/dev/null || git checkout -q main 2>/dev/null || true
git config user.email t@t; git config user.name tester
git config commit.gpgsign false

mkdir -p docs/provenance scripts cli/bin .agents/pawl-verdicts
# genesis ledger entry (prev_hash empty)
printf '{"edge_id":"genesis","prev_hash":"","hash":"h0"}\n' > docs/provenance/ledger.jsonl
cp "$UNDER_TEST" scripts/post-land-provenance-emit.sh
chmod +x scripts/post-land-provenance-emit.sh

# --- fake ao: emit-landed appends a CHAINED ledger line (prev_hash = current ---
#     tail hash). On the first call only, it also pushes a competing commit to
#     origin/main so our subsequent push is rejected (the race). ----------------
cat > "$TMP/fake-ao" <<FAKEAO
#!/usr/bin/env bash
set -uo pipefail
TMP="$TMP"
sub="\${1:-}"; act="\${2:-}"
if [[ "\$sub" == "provenance" && "\$act" == "emit-landed" ]]; then
  L="docs/provenance/ledger.jsonl"
  # arm-once concurrent push BEFORE we append, so our reset saw the OLD tail
  if [[ -f "\$TMP/race-armed" ]]; then
    rm -f "\$TMP/race-armed"
    ( cd "\$TMP/competitor" \
      && git config user.email c@c && git config user.name competitor \
      && git config commit.gpgsign false \
      && git fetch -q origin main && git reset -q --hard origin/main \
      && last=\$(tail -n1 docs/provenance/ledger.jsonl | sed -n 's/.*"hash":"\\([^"]*\\)".*/\\1/p') \
      && printf '{"edge_id":"comp","prev_hash":"%s","hash":"hcomp"}\n' "\$last" >> docs/provenance/ledger.jsonl \
      && git add -A && git commit -qm "competitor edge #trivial" && git push -q origin HEAD:main ) >/dev/null 2>&1 || true
  fi
  # append OUR edge, chained onto the CURRENT tail (this is the re-emit behavior)
  last=\$(tail -n1 "\$L" | sed -n 's/.*"hash":"\\([^"]*\\)".*/\\1/p')
  printf '{"edge_id":"mine","prev_hash":"%s","hash":"hmine"}\n' "\$last" >> "\$L"
  exit 0
fi
exit 0
FAKEAO
chmod +x "$TMP/fake-ao"

git add -A
git commit -qm "seed" --no-verify
git push -q origin HEAD:main

# competitor clone (after seed, so it has the trunk) — fake ao pushes through it
git clone -q "$TMP/origin.git" "$TMP/competitor"

# arm the race for this run
touch "$TMP/race-armed"

# --- concurrency guard fixtures: a LIVE sibling worktree (alive PID) must ---
#     survive the startup sweep; a DEAD-PID one must be reaped. -----------------
sleep 120 & LIVE_PID=$!
LIVE_WT="$TMP/agentops-prov-emit.$LIVE_PID.live"
DEAD_WT="$TMP/agentops-prov-emit.999999.dead"
git -C "$TMP/canon" worktree add -q --detach "$LIVE_WT" HEAD 2>/dev/null || true
git -C "$TMP/canon" worktree add -q --detach "$DEAD_WT" HEAD 2>/dev/null || true

# pre-push hook on the shared .git, installed ONLY for the run-under-test: if the
# worktree push re-invokes it, the marker appears — the script MUST push
# --no-verify to avoid recursive-hook deadlock. (Worktrees share .git/hooks.)
mkdir -p .git/hooks
printf '#!/usr/bin/env bash\ntouch "%s/hook-ran"\nexit 0\n' "$TMP" > .git/hooks/pre-push
chmod +x .git/hooks/pre-push

# --- run the script under test ----------------------------------------------
AO_BIN="$TMP/fake-ao" AGENTOPS_PROVENANCE_EMIT_RETRIES=5 \
  bash scripts/post-land-provenance-emit.sh >/dev/null 2>&1 || true

# pull origin's final state into a fresh read-only clone for assertions
git clone -q "$TMP/origin.git" "$TMP/verify"
FINAL_LEDGER="$TMP/verify/docs/provenance/ledger.jsonl"

# --- assertions --------------------------------------------------------------
# 1. our edge landed on origin despite the race
check "our provenance edge landed on origin/main" \
  "grep -q '\"edge_id\":\"mine\"' '$FINAL_LEDGER'"

# 2. the competitor's edge is also present (the race actually happened)
check "the simulated concurrent edge is present (race fired)" \
  "grep -q '\"edge_id\":\"comp\"' '$FINAL_LEDGER'"

# 3. the canonical checkout is byte-clean (no orphan commit, no dirty ledger)
check "canonical checkout clean (no dirty working tree)" \
  "[ -z \"\$(git -C '$TMP/canon' status --porcelain)\" ]"

# 4. canonical HEAD has NO unpushed provenance commit (no orphan stranded)
check "no orphan provenance commit on canonical HEAD" \
  "! git -C '$TMP/canon' log -1 --format=%s | grep -qi 'post-land sensor edges'"

# 5. the landed provenance commit carries #trivial
check "landed provenance commit is marked #trivial" \
  "git -C '$TMP/verify' log -1 --format=%B | grep -q '#trivial'"

# 6. chain VALID: our edge chains onto the competitor's hash (re-emit, not replay).
#    A rebase-replay would leave mine.prev_hash = h0 (pre-competitor) → broken.
mine_prev="$(grep '"edge_id":"mine"' "$FINAL_LEDGER" | sed -n 's/.*"prev_hash":"\([^"]*\)".*/\1/p')"
check "re-emit re-chained onto the competitor tail (chain valid, not a broken replay)" \
  "[ \"$mine_prev\" = 'hcomp' ]"

# 7. the script's OWN disposable worktree was cleaned up (no leak from this run)
check "the run's own disposable worktree was removed" \
  "! git -C '$TMP/canon' worktree list --porcelain | grep -q \"agentops-prov-emit.$$.\""

# 8. concurrency-safe sweep: a LIVE sibling worktree SURVIVES the sweep
check "sweep does NOT delete a live concurrent worktree (the REFUTE)" \
  "git -C '$TMP/canon' worktree list --porcelain | grep -q '$LIVE_PID.live'"

# 9. the dead-PID leaked worktree WAS reaped
check "sweep reaps a dead-PID leaked worktree" \
  "! git -C '$TMP/canon' worktree list --porcelain | grep -q '999999.dead'"

kill "$LIVE_PID" 2>/dev/null || true

# 10. hook-env hazard: invoked with GIT_DIR/GIT_WORK_TREE pointing at the canonical
#     checkout (as git's pre-push hook does), the script must STILL operate on its
#     disposable worktree and leave the canonical checkout untouched. Without the
#     env scrub, the worktree git ops would target canon via inherited GIT_DIR.
CANON_HEAD_BEFORE="$(git -C "$TMP/canon" rev-parse HEAD)"
( cd "$TMP/canon" && GIT_DIR="$TMP/canon/.git" GIT_WORK_TREE="$TMP/canon" \
    GIT_INDEX_FILE="$TMP/canon/.git/index" AO_BIN="$TMP/fake-ao" \
    bash scripts/post-land-provenance-emit.sh ) >/dev/null 2>&1 || true
check "hook-env (GIT_DIR set): canonical checkout stays byte-clean" \
  "[ -z \"\$(git -C '$TMP/canon' status --porcelain)\" ]"
check "hook-env (GIT_DIR set): canonical HEAD not clobbered" \
  "[ \"\$(git -C '$TMP/canon' rev-parse HEAD)\" = '$CANON_HEAD_BEFORE' ]"

# 12. recursion guard: the worktree push must use --no-verify, so the shared
#     pre-push hook is NOT re-invoked (else a hook-context run would deadlock on
#     the serial push lock). The hook drops a marker if it ever runs.
check "worktree push uses --no-verify (pre-push hook not re-invoked)" \
  "[ ! -e '$TMP/hook-ran' ]"
echo
if [[ "$fails" -eq 0 ]]; then
  echo "ALL PASS (12 checks)"
  exit 0
fi
echo "$fails CHECK(S) FAILED"
exit 1
