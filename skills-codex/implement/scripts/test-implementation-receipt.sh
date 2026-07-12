#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERIFY="$SKILL_DIR/scripts/verify-implementation-receipt.sh"
CLOSE="$SKILL_DIR/scripts/close-with-implementation-receipt.sh"
PRODUCT_ROOT="$(git rev-parse --show-toplevel)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
REPO="$TMP/repo"
ISSUE=""

digest() { shasum -a 256 "$1" | awk '{print $1}'; }
blob_digest() { git -C "$REPO" show "$1:$2" | shasum -a 256 | awk '{print $1}'; }
verify() { (cd "$REPO" && "$VERIFY" --issue "$ISSUE" --receipt "$RECEIPT") >/dev/null 2>&1; }
wait_for_close_snapshot() {
  local attempt=0 manifest
  while (( attempt < 1000 )); do
    for manifest in "$ROOT"/.close-manifest.*; do
      [[ -f "$manifest" ]] && return 0
    done
    /bin/sleep 0.01
    attempt=$((attempt + 1))
  done
  return 1
}
EMPTY_HASH="$(printf '' | shasum -a 256 | awk '{print $1}')"

mkdir -p "$REPO"
git -C "$REPO" init -q
git -C "$REPO" config user.email fixture@example.invalid
git -C "$REPO" config user.name Fixture
mkdir -p "$REPO/scripts/lib" "$REPO/schemas" "$REPO/_beads"
cp "$PRODUCT_ROOT/scripts/pawl-verdict.sh" "$REPO/scripts/pawl-verdict.sh"
cp "$PRODUCT_ROOT/scripts/lib/diff-identity.sh" "$REPO/scripts/lib/diff-identity.sh"
cp "$PRODUCT_ROOT/schemas/pawl-verdict.v1.schema.json" "$REPO/schemas/pawl-verdict.v1.schema.json"
BEADS_DIR="$REPO/_beads" /opt/homebrew/bin/br init --prefix age -q
printf 'disabled\n' >"$REPO/feature.txt"
git -C "$REPO" add feature.txt
git -C "$REPO" commit -qm base
BASE="$(git -C "$REPO" rev-parse HEAD)"
printf '#!/usr/bin/env bash\ngrep -q "^enabled$" feature.txt\n' >"$REPO/test.sh"
chmod +x "$REPO/test.sh"
git -C "$REPO" add test.sh
git -C "$REPO" commit -qm red-contract
OBSERVED="$(git -C "$REPO" rev-parse HEAD)"
printf 'enabled\n' >"$REPO/feature.txt"
git -C "$REPO" add feature.txt
git -C "$REPO" commit -qm implementation
HEAD="$(git -C "$REPO" rev-parse HEAD)"
ISSUE="$(BEADS_DIR="$REPO/_beads" /opt/homebrew/bin/br create --title 'behavior fixture' --type task --description $'```yaml\nacceptance_criteria:\n  - id: ac-1\n    description: behavior fixture passes\n    check_type: command_exit_zero\n    check_command: sleep 1; bash test.sh\n```' --silent)"

ROOT="$REPO/.agents/evidence/implement/$ISSUE/$HEAD"
RECEIPT="$ROOT/$ISSUE-$HEAD-receipt.json"
mkdir -p "$ROOT/evidence" "$REPO/.agents/pawl-verdicts"
jq -n --arg command 'sleep 1; bash test.sh' --arg hash "$EMPTY_HASH" '{command:$command,exit_code:1,output_sha256:$hash}' >"$ROOT/evidence/red.log"
jq -n --arg command 'sleep 1; bash test.sh' --arg hash "$EMPTY_HASH" '{command:$command,exit_code:0,output_sha256:$hash}' >"$ROOT/evidence/green.log"
printf '{"goal":"enable the accepted behavior","expected":"test.sh passes"}\n' >"$ROOT/evidence/behavioral-spec.json"
printf 'files reviewed: 2\nfeature.txt:1 and test.sh:2 satisfy the acceptance contract\n' >"$REPO/review.txt"
cp "$REPO/review.txt" "$ROOT/review.txt"
jq -n --arg bead "$ISSUE" --arg head "$HEAD" '{schema_version:"pawl-verdict.v1",bead_id:$bead,pr:0,head_sha:$head,disposition:"CONFIRMED",generated_at:"2026-07-12T00:00:00Z",mode:"fresh-context",author_context_id:"author",refuters:[{family:"claude",verdict:"CONFIRMED",context_id:"fresh-reviewer",evidence:"review.txt"}]}' >"$REPO/.agents/pawl-verdicts/$ISSUE.json"
cp "$REPO/.agents/pawl-verdicts/$ISSUE.json" "$ROOT/evidence/pawl-verdict.json"

TEST_DIGEST="$(blob_digest "$OBSERVED" test.sh)"
RED_DIGEST="$(digest "$ROOT/evidence/red.log")"
GREEN_DIGEST="$(digest "$ROOT/evidence/green.log")"
REVIEW_DIGEST="$(digest "$ROOT/review.txt")"
VERDICT_DIGEST="$(digest "$ROOT/evidence/pawl-verdict.json")"
SPEC_DIGEST="$(digest "$ROOT/evidence/behavioral-spec.json")"
jq -n --arg issue "$ISSUE" --arg base "$BASE" --arg head "$HEAD" --arg observed "$OBSERVED" --arg td "$TEST_DIGEST" --arg rd "$RED_DIGEST" --arg gd "$GREEN_DIGEST" --arg sd "$SPEC_DIGEST" --arg vd "$VERDICT_DIGEST" --arg review "$REVIEW_DIGEST" '{schema_version:1,issue_id:$issue,base_sha:$base,head_sha:$head,work_class:"behavior",acceptance_ids:["ac-1"],changed_files:["feature.txt","test.sh"],red:{kind:"captured",observed_sha:$observed,command:"sleep 1; bash test.sh",exit_code:1,evidence:{path:"evidence/red.log",sha256:$rd},test_files:[{path:"test.sh",sha256:$td}]},green:[{command:"sleep 1; bash test.sh",exit_code:0,evidence:{path:"evidence/green.log",sha256:$gd}}],behavioral_spec:{path:"evidence/behavioral-spec.json",sha256:$sd},independent_validation:{disposition:"CONFIRMED",pr:0,source_verdict_path:(".agents/pawl-verdicts/"+$issue+".json"),copied_verdict:{path:"evidence/pawl-verdict.json",sha256:$vd},review_evidence:[{path:"review.txt",sha256:$review}]}}' >"$RECEIPT"

verify || { echo "FAIL: valid receipt rejected" >&2; exit 1; }
BACKUP="$TMP/receipt.json"; cp "$RECEIPT" "$BACKUP"
expect_reject() {
  local name="$1" filter="$2"
  jq "$filter" "$BACKUP" >"$RECEIPT"
  if verify; then echo "FAIL: forged $name accepted" >&2; exit 1; fi
  cp "$BACKUP" "$RECEIPT"
}

expect_reject issue-id '.issue_id="other"'
expect_reject unknown-base '.base_sha=("0"*40)'
expect_reject equal-base-head '.base_sha=.head_sha'
expect_reject changed-files '.changed_files=["test.sh"]'
expect_reject missing-red-evidence '.red.evidence.path="evidence/missing.log"'
expect_reject red-digest '.red.evidence.sha256=("0"*64)'
expect_reject synthetic-red '.red.command="exit 2"'
expect_reject paired-synthetic-red '.red.command="exit 2" | .red.exit_code=2'
expect_reject behavior-skipped-spec '.behavioral_spec={skipped_reason:"fake"}'
expect_reject test-digest '.red.test_files[0].sha256=("0"*64)'
expect_reject mutated-test-contract '.red.test_files[0].path="feature.txt" | .red.test_files[0].sha256=("0"*64)'
expect_reject missing-green-evidence '.green[0].evidence.path="evidence/missing.log"'
expect_reject synthetic-green '.green[0].command="exit 1"'
expect_reject traversal '.green[0].evidence.path="../green.log"'
expect_reject behavior-waiver '.red={kind:"waived",reason:"docs-only",evidence:{path:"evidence/red.log",sha256:.red.evidence.sha256}}'
expect_reject waiver-mismatch '.work_class="chore-only" | .red={kind:"waived",reason:"docs-only",evidence:{path:"evidence/red.log",sha256:.red.evidence.sha256}}'
expect_reject pure-refactor-without-baselines '.work_class="pure-refactor" | .red={kind:"waived",reason:"pure-refactor",evidence:{path:"evidence/red.log",sha256:.red.evidence.sha256}}'
expect_reject source-verdict-path '.independent_validation.source_verdict_path=".agents/pawl-verdicts/other.json"'
expect_reject copied-verdict-digest '.independent_validation.copied_verdict.sha256=("0"*64)'
expect_reject review-evidence-digest '.independent_validation.review_evidence[0].sha256=("0"*64)'
expect_reject nonconfirmed-receipt '.independent_validation.disposition="REFUTED"'

cp "$ROOT/evidence/red.log" "$TMP/red-output.log"; printf 'altered output\n' >>"$ROOT/evidence/red.log"
if verify; then echo "FAIL: altered captured RED output accepted" >&2; exit 1; fi
cp "$TMP/red-output.log" "$ROOT/evidence/red.log"
UNRELATED_HASH="$(printf 'unrelated\n' | shasum -a 256 | awk '{print $1}')"; jq --arg h "$UNRELATED_HASH" '.output_sha256=$h' "$ROOT/evidence/red.log" >"$TMP/unrelated.json"; cp "$TMP/unrelated.json" "$ROOT/evidence/red.log"; NEW_RED_DIGEST="$(digest "$ROOT/evidence/red.log")"; jq --arg d "$NEW_RED_DIGEST" '.red.evidence.sha256=$d' "$BACKUP" >"$RECEIPT"
if verify; then echo "FAIL: digest-consistent unrelated RED output accepted" >&2; exit 1; fi
cp "$TMP/red-output.log" "$ROOT/evidence/red.log"; cp "$BACKUP" "$RECEIPT"

cp "$BACKUP" "$RECEIPT"
ln -s red.log "$ROOT/evidence/red-link.log"
jq '.red.evidence.path="evidence/red-link.log"' "$BACKUP" >"$RECEIPT"
if verify; then echo "FAIL: symlink evidence accepted" >&2; exit 1; fi
rm "$ROOT/evidence/red-link.log"; cp "$BACKUP" "$RECEIPT"
mkdir "$ROOT/real-dir"; cp "$ROOT/evidence/red.log" "$ROOT/real-dir/red.log"; ln -s real-dir "$ROOT/link-dir"
jq --arg d "$(digest "$ROOT/real-dir/red.log")" '.red.evidence={path:"link-dir/red.log",sha256:$d}' "$BACKUP" >"$RECEIPT"
if verify; then echo "FAIL: symlinked evidence parent accepted" >&2; exit 1; fi
rm "$ROOT/link-dir"; rm -rf "$ROOT/real-dir"; cp "$BACKUP" "$RECEIPT"

WRONG="$ROOT/wrong.json"; cp "$BACKUP" "$WRONG"
if (cd "$REPO" && "$VERIFY" --issue "$ISSUE" --receipt "$WRONG") >/dev/null 2>&1; then echo "FAIL: noncanonical receipt path accepted" >&2; exit 1; fi

SOURCE="$REPO/.agents/pawl-verdicts/$ISSUE.json"; SOURCE_BACKUP="$TMP/pawl.json"; cp "$SOURCE" "$SOURCE_BACKUP"
expect_pawl_reject() {
  local name="$1" filter="$2" new_digest
  jq "$filter" "$SOURCE_BACKUP" >"$SOURCE"
  cp "$SOURCE" "$ROOT/evidence/pawl-verdict.json"
  new_digest="$(digest "$SOURCE")"
  jq --arg d "$new_digest" '.independent_validation.copied_verdict.sha256=$d' "$BACKUP" >"$RECEIPT"
  if verify; then echo "FAIL: forged pawl $name accepted" >&2; exit 1; fi
  cp "$SOURCE_BACKUP" "$SOURCE"; cp "$SOURCE_BACKUP" "$ROOT/evidence/pawl-verdict.json"; cp "$BACKUP" "$RECEIPT"
}
expect_pawl_reject wrong-bead '.bead_id="other"'
expect_pawl_reject wrong-head '.head_sha=("0"*40)'
expect_pawl_reject refuted '.disposition="REFUTED" | .refuters[0].verdict="REFUTED"'
expect_pawl_reject malformed 'del(.generated_at)'

mv "$SOURCE" "$TMP/source-away.json"
if verify; then echo "FAIL: missing canonical verdict accepted" >&2; exit 1; fi
mv "$TMP/source-away.json" "$SOURCE"

if (cd "$REPO" && "$VERIFY" --issue "$ISSUE" --receipt "$RECEIPT" --repo-root "$REPO") >/dev/null 2>&1; then echo "FAIL: removed repo-root bypass accepted" >&2; exit 1; fi
if (cd "$PRODUCT_ROOT" && "$VERIFY" --issue "$ISSUE" --receipt "$RECEIPT") >/dev/null 2>&1; then echo "FAIL: caller-root rebase accepted" >&2; exit 1; fi
if (cd "$REPO" && AGENTOPS_TRACKER=bd "$VERIFY" --issue "$ISSUE" --receipt "$RECEIPT") >/dev/null 2>&1; then echo "FAIL: tracker override accepted" >&2; exit 1; fi
printf 'dirty\n' >>"$REPO/feature.txt"
if verify; then echo "FAIL: dirty tracked tree accepted" >&2; exit 1; fi
git -C "$REPO" restore feature.txt
mkdir "$TMP/fake-bin" "$TMP/fake-home"; printf '#!/bin/sh\nprintf called >"%s"\nexit 0\n' "$TMP/fake-called" >"$TMP/fake-bin/ao"; printf '#!/bin/sh\nprintf shim >"%s"\nexit 0\n' "$TMP/path-shim-called" >"$TMP/fake-bin/sleep"; printf '#!/bin/sh\nprintf fake-bash >"%s"\nexit 0\n' "$TMP/fake-bash-called" >"$TMP/fake-bin/bash"; printf 'printf profile >"%s"\n' "$TMP/profile-called" >"$TMP/fake-home/.bash_profile"; chmod +x "$TMP/fake-bin/ao" "$TMP/fake-bin/sleep" "$TMP/fake-bin/bash"
attack_out=""; attack_out="$(cd "$REPO" && HOME="$TMP/fake-home" PATH="$TMP/fake-bin:$PATH" "$VERIFY" --issue "$ISSUE" --receipt "$RECEIPT" 2>&1)" || { echo "FAIL: OS-identity ao resolution broke under HOME/PATH attack: $attack_out" >&2; exit 1; }
[[ ! -e "$TMP/fake-called" ]] || { echo "FAIL: PATH-injected ao executed" >&2; exit 1; }
[[ ! -e "$TMP/path-shim-called" && ! -e "$TMP/fake-bash-called" && ! -e "$TMP/profile-called" ]] || { echo "FAIL: replay or pawl loaded a PATH/profile shim" >&2; exit 1; }
cp "$SOURCE_BACKUP" "$SOURCE"; jq 'del(.generated_at)' "$SOURCE_BACKUP" >"$SOURCE"; cp "$SOURCE" "$ROOT/evidence/pawl-verdict.json"; forged_digest="$(digest "$SOURCE")"; jq --arg d "$forged_digest" '.independent_validation.copied_verdict.sha256=$d' "$BACKUP" >"$RECEIPT"
if (cd "$REPO" && PATH="$TMP/fake-bin:$PATH" "$VERIFY" --issue "$ISSUE" --receipt "$RECEIPT") >/dev/null 2>&1; then echo "FAIL: fake bash bypassed malformed canonical pawl verdict" >&2; exit 1; fi
[[ ! -e "$TMP/fake-bash-called" ]] || { echo "FAIL: PATH-injected bash executed during verification" >&2; exit 1; }
cp "$SOURCE_BACKUP" "$SOURCE"; cp "$SOURCE_BACKUP" "$ROOT/evidence/pawl-verdict.json"; cp "$BACKUP" "$RECEIPT"
jq 'del(.generated_at)' "$SOURCE_BACKUP" >"$SOURCE"; jq '.independent_validation={disposition:"PENDING"}' "$BACKUP" >"$RECEIPT"
if (cd "$REPO" && PATH="$TMP/fake-bin:$PATH" "$CLOSE" --issue "$ISSUE" --receipt "$RECEIPT") >/dev/null 2>&1; then echo "FAIL: fake bash bypassed close-time canonical pawl check" >&2; exit 1; fi
[[ ! -e "$TMP/fake-bash-called" ]] || { echo "FAIL: PATH-injected bash executed during close" >&2; exit 1; }
cp "$SOURCE_BACKUP" "$SOURCE"; cp "$SOURCE_BACKUP" "$ROOT/evidence/pawl-verdict.json"; cp "$BACKUP" "$RECEIPT"
printf 'printf bash-env >"%s"\n' "$TMP/bash-env-called" >"$TMP/bash-env"
(cd "$REPO" && BASH_ENV="$TMP/bash-env" "$VERIFY" --issue "$ISSUE" --receipt "$RECEIPT") >/dev/null 2>&1 || { echo "FAIL: scrubbed BASH_ENV disrupted verification" >&2; exit 1; }
[[ ! -e "$TMP/bash-env-called" ]] || { echo "FAIL: BASH_ENV executed" >&2; exit 1; }
if (cd "$REPO" && AGENTOPS_CONFIG="$TMP/fake-config" "$VERIFY" --issue "$ISSUE" --receipt "$RECEIPT") >/dev/null 2>&1; then echo "FAIL: config override accepted" >&2; exit 1; fi
mkdir "$TMP/fake-python"; printf 'raise RuntimeError("forged yaml loaded")\n' >"$TMP/fake-python/yaml.py"; printf 'raise RuntimeError("forged jsonschema loaded")\n' >"$TMP/fake-python/jsonschema.py"
if (cd "$REPO" && PYTHONPATH="$TMP/fake-python" "$VERIFY" --issue "$ISSUE" --receipt "$RECEIPT") >/dev/null 2>&1; then echo "FAIL: Python import override accepted" >&2; exit 1; fi

# The close wrapper must detect source-verdict and receipt mutations during replay.
RACE_RECEIPT="$TMP/race-receipt.json"; RACE_SOURCE="$TMP/race-source.json"; cp "$RECEIPT" "$RACE_RECEIPT"; cp "$SOURCE" "$RACE_SOURCE"
jq '.independent_validation={disposition:"PENDING"}' "$RACE_RECEIPT" >"$RECEIPT"
rm -f "$ROOT"/.close-manifest.*
(wait_for_close_snapshot; printf ' ' >>"$SOURCE") & race_pid=$!
if (cd "$REPO" && "$CLOSE" --issue "$ISSUE" --receipt "$RECEIPT") >/dev/null 2>&1; then echo "FAIL: source-verdict race accepted" >&2; exit 1; fi
wait "$race_pid"; cp "$RACE_SOURCE" "$SOURCE"; cp "$RACE_SOURCE" "$ROOT/evidence/pawl-verdict.json"; cp "$RACE_RECEIPT" "$RECEIPT"
jq '.independent_validation={disposition:"PENDING"}' "$RACE_RECEIPT" >"$RECEIPT"; cp "$REPO/review.txt" "$TMP/review-race.txt"
rm -f "$ROOT"/.close-manifest.*
(wait_for_close_snapshot; printf 'mutated review\n' >>"$REPO/review.txt") & race_pid=$!
if (cd "$REPO" && "$CLOSE" --issue "$ISSUE" --receipt "$RECEIPT") >/dev/null 2>&1; then echo "FAIL: review-evidence race accepted" >&2; exit 1; fi
wait "$race_pid"; cp "$TMP/review-race.txt" "$REPO/review.txt"; cp "$TMP/review-race.txt" "$ROOT/review.txt"; cp "$RACE_RECEIPT" "$RECEIPT"
jq '.independent_validation={disposition:"PENDING"}' "$RACE_RECEIPT" >"$RECEIPT"
rm -f "$ROOT"/.close-manifest.*
(wait_for_close_snapshot; printf ' ' >>"$RECEIPT") & race_pid=$!
if (cd "$REPO" && "$CLOSE" --issue "$ISSUE" --receipt "$RECEIPT") >/dev/null 2>&1; then echo "FAIL: receipt race accepted" >&2; exit 1; fi
wait "$race_pid"; cp "$RACE_RECEIPT" "$RECEIPT"

# A genuinely non-behavior docs-only slice closes without inventing a RED run.
DOC_BASE="$(git -C "$REPO" rev-parse HEAD)"
printf '# Operator notes\n' >"$REPO/README.md"
git -C "$REPO" add README.md
git -C "$REPO" commit -qm docs-only
DOC_HEAD="$(git -C "$REPO" rev-parse HEAD)"
ISSUE="$(BEADS_DIR="$REPO/_beads" /opt/homebrew/bin/br create --title 'docs fixture' --type task --description $'```yaml\nacceptance_criteria:\n  - id: ac-doc\n    description: docs fixture passes\n    check_type: command_exit_zero\n    check_command: test -s README.md\n```' --silent)"; ROOT="$REPO/.agents/evidence/implement/$ISSUE/$DOC_HEAD"; RECEIPT="$ROOT/$ISSUE-$DOC_HEAD-receipt.json"
mkdir -p "$ROOT/evidence"
jq -n --arg base "$DOC_BASE" --arg head "$DOC_HEAD" '{kind:"waiver",reason:"docs-only",base_sha:$base,head_sha:$head}' >"$ROOT/evidence/waiver.log"
jq -n --arg command 'test -s README.md' --arg hash "$EMPTY_HASH" '{command:$command,exit_code:0,output_sha256:$hash}' >"$ROOT/evidence/green.log"
printf 'files reviewed: 1\nREADME.md:1 is documentation only\n' >"$REPO/review.txt"
cp "$REPO/review.txt" "$ROOT/review.txt"
jq -n --arg bead "$ISSUE" --arg head "$DOC_HEAD" '{schema_version:"pawl-verdict.v1",bead_id:$bead,pr:0,head_sha:$head,disposition:"CONFIRMED",generated_at:"2026-07-12T00:00:00Z",mode:"fresh-context",author_context_id:"author",refuters:[{family:"claude",verdict:"CONFIRMED",context_id:"fresh-reviewer",evidence:"review.txt"}]}' >"$REPO/.agents/pawl-verdicts/$ISSUE.json"
cp "$REPO/.agents/pawl-verdicts/$ISSUE.json" "$ROOT/evidence/pawl-verdict.json"
WAIVER_DIGEST="$(digest "$ROOT/evidence/waiver.log")"; GREEN_DIGEST="$(digest "$ROOT/evidence/green.log")"; REVIEW_DIGEST="$(digest "$ROOT/review.txt")"; VERDICT_DIGEST="$(digest "$ROOT/evidence/pawl-verdict.json")"
jq -n --arg issue "$ISSUE" --arg base "$DOC_BASE" --arg head "$DOC_HEAD" --arg wd "$WAIVER_DIGEST" --arg gd "$GREEN_DIGEST" --arg vd "$VERDICT_DIGEST" --arg rd "$REVIEW_DIGEST" '{schema_version:1,issue_id:$issue,base_sha:$base,head_sha:$head,work_class:"docs-only",acceptance_ids:["ac-doc"],changed_files:["README.md"],red:{kind:"waived",reason:"docs-only",evidence:{path:"evidence/waiver.log",sha256:$wd}},green:[{command:"test -s README.md",exit_code:0,evidence:{path:"evidence/green.log",sha256:$gd}}],behavioral_spec:{skipped_reason:"docs-only"},independent_validation:{disposition:"CONFIRMED",pr:0,source_verdict_path:(".agents/pawl-verdicts/"+$issue+".json"),copied_verdict:{path:"evidence/pawl-verdict.json",sha256:$vd},review_evidence:[{path:"review.txt",sha256:$rd}]}}' >"$RECEIPT"
verify || { echo "FAIL: valid docs-only waiver rejected" >&2; exit 1; }
jq '.independent_validation={disposition:"PENDING"}' "$RECEIPT" >"$TMP/pending.json"; mv "$TMP/pending.json" "$RECEIPT"
(cd "$REPO" && PATH="$TMP/fake-bin:$PATH" "$CLOSE" --issue "$ISSUE" --receipt "$RECEIPT") >/dev/null 2>&1 || { echo "FAIL: atomic close wrapper rejected valid docs receipt under PATH attack" >&2; exit 1; }
[[ ! -e "$TMP/fake-bash-called" ]] || { echo "FAIL: PATH-injected bash executed during valid close" >&2; exit 1; }
BEADS_DIR="$REPO/_beads" /opt/homebrew/bin/br show "$ISSUE" --json | jq -e '.[0].status=="closed"' >/dev/null || { echo "FAIL: wrapper did not close exact bead" >&2; exit 1; }

# Pure-refactor waivers must run the exact bead command at both endpoints and bind unchanged drivers.
PURE_BASE="$(git -C "$REPO" rev-parse HEAD)"; printf 'enabled\n# reorganized without behavior change\n' >"$REPO/feature.txt"; git -C "$REPO" add feature.txt; git -C "$REPO" commit -qm pure-refactor; PURE_HEAD="$(git -C "$REPO" rev-parse HEAD)"
ISSUE="$(BEADS_DIR="$REPO/_beads" /opt/homebrew/bin/br create --title 'pure refactor fixture' --type task --description $'```yaml\nacceptance_criteria:\n  - id: ac-refactor\n    description: existing behavior remains green\n    check_type: command_exit_zero\n    check_command: bash test.sh\n```' --silent)"; ROOT="$REPO/.agents/evidence/implement/$ISSUE/$PURE_HEAD"; RECEIPT="$ROOT/$ISSUE-$PURE_HEAD-receipt.json"; mkdir -p "$ROOT/evidence"
jq -n --arg base "$PURE_BASE" --arg head "$PURE_HEAD" '{kind:"waiver",reason:"pure-refactor",base_sha:$base,head_sha:$head}' >"$ROOT/evidence/waiver.log"; jq -n --arg command 'bash test.sh' --arg hash "$EMPTY_HASH" '{command:$command,exit_code:0,output_sha256:$hash}' >"$ROOT/evidence/base.log"; cp "$ROOT/evidence/base.log" "$ROOT/evidence/head.log"; cp "$ROOT/evidence/base.log" "$ROOT/evidence/green.log"; printf 'files reviewed: 1\nfeature.txt:2 is a refactor with unchanged test.sh\n' >"$REPO/review.txt"; cp "$REPO/review.txt" "$ROOT/review.txt"
jq -n --arg bead "$ISSUE" --arg head "$PURE_HEAD" '{schema_version:"pawl-verdict.v1",bead_id:$bead,pr:0,head_sha:$head,disposition:"CONFIRMED",generated_at:"2026-07-12T00:00:00Z",mode:"fresh-context",author_context_id:"author",refuters:[{family:"claude",verdict:"CONFIRMED",context_id:"fresh-reviewer",evidence:"review.txt"}]}' >"$REPO/.agents/pawl-verdicts/$ISSUE.json"; cp "$REPO/.agents/pawl-verdicts/$ISSUE.json" "$ROOT/evidence/pawl-verdict.json"
WD="$(digest "$ROOT/evidence/waiver.log")"; BD="$(digest "$ROOT/evidence/base.log")"; HD="$(digest "$ROOT/evidence/head.log")"; GD="$(digest "$ROOT/evidence/green.log")"; RD="$(digest "$ROOT/review.txt")"; VD="$(digest "$ROOT/evidence/pawl-verdict.json")"; TD="$(blob_digest "$PURE_BASE" test.sh)"
jq -n --arg issue "$ISSUE" --arg base "$PURE_BASE" --arg head "$PURE_HEAD" --arg wd "$WD" --arg bd "$BD" --arg hd "$HD" --arg gd "$GD" --arg rd "$RD" --arg vd "$VD" --arg td "$TD" '{schema_version:1,issue_id:$issue,base_sha:$base,head_sha:$head,work_class:"pure-refactor",acceptance_ids:["ac-refactor"],changed_files:["feature.txt"],red:{kind:"waived",reason:"pure-refactor",evidence:{path:"evidence/waiver.log",sha256:$wd},baseline_before:[{command:"bash test.sh",exit_code:0,evidence:{path:"evidence/base.log",sha256:$bd}}],baseline_after:[{command:"bash test.sh",exit_code:0,evidence:{path:"evidence/head.log",sha256:$hd}}],test_drivers:[{path:"test.sh",sha256:$td}]},green:[{command:"bash test.sh",exit_code:0,evidence:{path:"evidence/green.log",sha256:$gd}}],behavioral_spec:{skipped_reason:"pure-refactor"},independent_validation:{disposition:"CONFIRMED",pr:0,source_verdict_path:(".agents/pawl-verdicts/"+$issue+".json"),copied_verdict:{path:"evidence/pawl-verdict.json",sha256:$vd},review_evidence:[{path:"review.txt",sha256:$rd}]}}' >"$RECEIPT"
verify || { echo "FAIL: valid pure-refactor waiver rejected" >&2; exit 1; }
cp "$RECEIPT" "$TMP/pure.json"; jq '.red.test_drivers[0].sha256=("0"*64)' "$TMP/pure.json" >"$RECEIPT"; if verify; then echo "FAIL: changed pure-refactor driver accepted" >&2; exit 1; fi
printf 'implementation receipt forged-dimension fixtures: PASS\n'
