#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERIFY="$SKILL_DIR/scripts/verify-landed.sh"
REPO_ROOT="$(cd "$SKILL_DIR/../.." && pwd)"
PAWL="$REPO_ROOT/scripts/pawl-verdict.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

new_fixture() {
  local name="$1"
  FIXTURE="$TMP/$name"
  REMOTE="$TMP/$name.git"
  git init --bare --quiet "$REMOTE"
  git init --quiet "$FIXTURE"
  git -C "$FIXTURE" config user.email test@example.com
  git -C "$FIXTURE" config user.name Test
  git -C "$FIXTURE" remote add origin "$REMOTE"
  mkdir -p "$FIXTURE/scripts/lib" "$FIXTURE/schemas" "$FIXTURE/.agents/pawl-verdicts"
  cp "$PAWL" "$FIXTURE/scripts/pawl-verdict.sh"
  cp "$REPO_ROOT/scripts/lib/diff-identity.sh" "$FIXTURE/scripts/lib/diff-identity.sh"
  cp "$REPO_ROOT/schemas/pawl-verdict.v1.schema.json" "$FIXTURE/schemas/pawl-verdict.v1.schema.json"
  echo init >"$FIXTURE/README.md"
  git -C "$FIXTURE" add README.md scripts schemas
  git -C "$FIXTURE" commit --quiet -m "chore: fixture init"
  git -C "$FIXTURE" branch -M main
}

commit_feature() {
  local bead="$1"
  echo "$bead" >>"$FIXTURE/README.md"
  git -C "$FIXTURE" add README.md
  git -C "$FIXTURE" commit --quiet -m "feat(test): exercise landed verifier ($bead)"
  FEATURE_SHA="$(git -C "$FIXTURE" rev-parse HEAD)"
}

seed_verdict() {
  local bead="$1" sha="$2"
  local evidence="$FIXTURE/evidence-$bead.txt"
  printf 'fresh-context deterministic fixture evidence\n' >"$evidence"
  (cd "$FIXTURE" && AGENTOPS_REPO_ROOT="$FIXTURE" PAWL_AUTOBIND=0 \
    bash "$FIXTURE/scripts/pawl-verdict.sh" write "$bead" 0 \
      --disposition CONFIRMED --head "$sha" --author-context author-ctx \
      --refuter "claude:CONFIRMED:fresh-reviewer-ctx:$evidence" \
      --dir "$FIXTURE/.agents/pawl-verdicts" >/dev/null 2>&1)
}

push_tip() {
  git -C "$FIXTURE" push --quiet -u origin HEAD:main
}

expect_pass() {
  local bead="$1" expected_reviewed="$2" out
  out="$(bash "$VERIFY" "$bead" --repo "$FIXTURE")" || fail "$bead unexpectedly failed"
  [[ "$out" == *"tip=$(git -C "$FIXTURE" rev-parse HEAD)"* ]] || fail "$bead omitted tip SHA"
  [[ "$out" == *"reviewed=$expected_reviewed"* ]] || fail "$bead resolved the wrong reviewed SHA: $out"
}

expect_fail() {
  local bead="$1" label="$2"
  if bash "$VERIFY" "$bead" --repo "$FIXTURE" >"$TMP/out" 2>&1; then
    fail "$label unexpectedly passed"
  fi
}

# Normal feature tip: review and remote identity bind directly to HEAD.
new_fixture normal
commit_feature age-push-normal
seed_verdict age-push-normal "$FEATURE_SHA"
push_tip
expect_pass age-push-normal "$FEATURE_SHA"

# Valid canonical auto-bind tip: remote is bind, while pawl binds the feat parent.
new_fixture bind
commit_feature age-push-bind
seed_verdict age-push-bind "$FEATURE_SHA"
mkdir -p "$FIXTURE/docs/provenance"
printf '{"edge":"bind"}\n' >"$FIXTURE/docs/provenance/ledger.jsonl"
git -C "$FIXTURE" add docs/provenance/ledger.jsonl
git -C "$FIXTURE" commit --quiet -m "chore(provenance): bind pawl CONFIRMED verdict for age-push-bind #trivial"
push_tip
expect_pass age-push-bind "$FEATURE_SHA"

# Marker-only is not a bind: code in the tip keeps reviewed=HEAD and the parent
# verdict must not authorize it.
new_fixture marker-only
commit_feature age-push-marker
seed_verdict age-push-marker "$FEATURE_SHA"
echo code >>"$FIXTURE/README.md"
git -C "$FIXTURE" add README.md
git -C "$FIXTURE" commit --quiet -m "chore(provenance): bind pawl CONFIRMED verdict for age-push-marker #trivial"
push_tip
expect_fail age-push-marker "marker-only invalid tip"
MARKER_TIP="$(git -C "$FIXTURE" rev-parse HEAD)"
grep -Fq -- "reviewed=$MARKER_TIP" "$TMP/out" || fail "marker-only tip was misclassified as an auto-bind"

# Provenance-only without the marker is not misclassified as an auto-bind.
new_fixture provenance-only
commit_feature age-push-provenance
seed_verdict age-push-provenance "$FEATURE_SHA"
mkdir -p "$FIXTURE/docs/provenance"
printf '{"edge":"ordinary-doc"}\n' >"$FIXTURE/docs/provenance/ledger.jsonl"
git -C "$FIXTURE" add docs/provenance/ledger.jsonl
git -C "$FIXTURE" commit --quiet -m "docs(provenance): document age-push-provenance"
PROVENANCE_TIP="$(git -C "$FIXTURE" rev-parse HEAD)"
push_tip
expect_fail age-push-provenance "provenance-only no-marker tip"
grep -Fq -- "reviewed=$PROVENANCE_TIP" "$TMP/out" || fail "provenance-only tip was misclassified as an auto-bind"

# A local tip not equal to freshly fetched origin/main fails before verdict use.
new_fixture stale
commit_feature age-push-stale
seed_verdict age-push-stale "$FEATURE_SHA"
push_tip
echo local >>"$FIXTURE/README.md"
git -C "$FIXTURE" add README.md
git -C "$FIXTURE" commit --quiet -m "fix(test): local-only age-push-stale"
expect_fail age-push-stale "stale remote"
grep -Fq "stale remote identity" "$TMP/out" || fail "stale remote failure was not explicit"

# A verdict cannot compensate for the reviewed commit citing the wrong bead.
new_fixture wrong-bead
commit_feature age-push-other
seed_verdict age-push-wrong-bead "$FEATURE_SHA"
push_tip
expect_fail age-push-wrong-bead "wrong bead citation"
grep -Fq "does not cite bead age-push-wrong-bead" "$TMP/out" || fail "wrong bead citation was not explicit"

# A verdict for the requested bead but an older SHA fails the real pawl check.
new_fixture wrong-head
commit_feature age-push-wrong-head
seed_verdict age-push-wrong-head "$(git -C "$FIXTURE" rev-parse HEAD^)"
push_tip
expect_fail age-push-wrong-head "wrong verdict head"
grep -Fq "does not authorize reviewed=$FEATURE_SHA" "$TMP/out" || fail "wrong verdict head was not explicit"

echo "OK: verify-landed fixtures"
