#!/usr/bin/env bats
# §C Cutover: real-repo migration — B78–B81 (judge amendment 3a).
# These run against the REAL agentops checkout. Every mutating step executes
# in a disposable clone or on a scratch file copy (the B92 hermetic contract);
# the operator checkout is never written.

setup() {
  load helpers2
}

@test "B78: real repo regen write set is declared, strictly formatted, and matches reality" {
  # the manifest exists and is non-empty on the cutover commit
  [ -s "$REAL_REPO_ROOT/scripts/regen-manifest.txt" ]
  [ -x "$REAL_REPO_ROOT/$V_REGEN_MANIFEST" ]

  clone="$(real_repo_clone)"
  real_manifest="$clone/scripts/regen-manifest.txt"
  [ -s "$real_manifest" ]

  # parity holds on the cutover commit
  run bash -c "cd '$clone' && $V_REGEN_MANIFEST"
  [ "$status" -eq 0 ]

  dup="$(grep -v '^#' "$real_manifest" | sed -n '1p')"
  [ -n "$dup" ]
  m="$BATS_TEST_TMPDIR/manifest"

  # STRICT format: duplicate path rejected, named
  cp "$real_manifest" "$m"; printf '%s\n' "$dup" >> "$m"
  run bash -c "cd '$clone' && $V_REGEN_MANIFEST --manifest '$m'"
  [ "$status" -ne 0 ]
  [[ "$output" == *"$dup"* ]]

  # non-normalized entry rejected, named
  cp "$real_manifest" "$m"; printf './zz-non-normalized.txt\n' >> "$m"
  run bash -c "cd '$clone' && $V_REGEN_MANIFEST --manifest '$m'"
  [ "$status" -ne 0 ]
  [[ "$output" == *"zz-non-normalized"* ]]
  cp "$real_manifest" "$m"; printf '../zz-escape.txt\n' >> "$m"
  run bash -c "cd '$clone' && $V_REGEN_MANIFEST --manifest '$m'"
  [ "$status" -ne 0 ]

  # underdeclared: deleting any one line makes parity fail naming the path
  grep -vF "$dup" "$real_manifest" > "$m"
  run bash -c "cd '$clone' && $V_REGEN_MANIFEST --manifest '$m'"
  [ "$status" -ne 0 ]
  [[ "$output" == *"$dup"* ]]

  # overdeclared/stale: a bogus added path is independently detected, named
  cp "$real_manifest" "$m"; printf 'docs/zz-bogus-never-written.txt\n' >> "$m"
  run bash -c "cd '$clone' && $V_REGEN_MANIFEST --manifest '$m'"
  [ "$status" -ne 0 ]
  [[ "$output" == *"zz-bogus-never-written"* ]]

  # source-owned overlap (the B42 rule on the real manifest), named
  cp "$real_manifest" "$m"; printf 'CLAUDE.md\n' >> "$m"
  run bash -c "cd '$clone' && $V_REGEN_MANIFEST --manifest '$m'"
  [ "$status" -ne 0 ]
  [[ "$output" == *"CLAUDE.md"* ]]

  # reality: on a clean tree, an ACTUAL regen run writes ONLY declared paths
  [ -x "$clone/scripts/regen-all.sh" ]
  ( cd "$clone" && bash scripts/regen-all.sh )
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    p="${line:3}"; p="${p#\"}"; p="${p%\"}"
    manifest_covers "$real_manifest" "$p"
  done < <(git -C "$clone" status --porcelain)
}

@test "B79: real repo count-bearing docs are declared in scripts/count-docs.txt" {
  [ -s "$REAL_REPO_ROOT/scripts/count-docs.txt" ]

  clone="$(real_repo_clone)"
  cd_manifest="$clone/scripts/count-docs.txt"
  [ -s "$cd_manifest" ]
  [ -x "$clone/scripts/land.sh" ]

  # strict format shared with B78: normalized repo-relative paths, no dups
  ! grep -Eq '^\.\.?/' "$cd_manifest"
  [ "$(grep -v '^#' "$cd_manifest" | grep -c .)" \
    -eq "$(grep -v '^#' "$cd_manifest" | grep . | sort -u | wc -l | tr -d ' ')" ]

  # every declared doc exists
  while IFS= read -r doc; do
    case "$doc" in \#*|"") continue ;; esac
    [ -f "$clone/$doc" ]
  done < "$cd_manifest"

  # the count checker exits 0 on a clean tree
  run land "$clone" --check-counts
  [ "$status" -eq 0 ]

  # format violation is rejected with the line named (mutate the CLONE's manifest)
  printf '../zz-evil.md\n' >> "$cd_manifest"
  run land "$clone" --check-counts
  [ "$status" -ne 0 ]
  [[ "$output" == *"zz-evil"* ]]
  git -C "$clone" checkout -q -- scripts/count-docs.txt

  # a bare numeric skill-count OUTSIDE marker blocks in a doc NOT in the
  # manifest is caught by the repo-wide sweep, doc named (B48 generalized)
  printf '# rogue\nWe now ship 42 skills.\n' > "$clone/docs/zz-rogue-count.md"
  run land "$clone" --check-counts
  [ "$status" -ne 0 ]
  [[ "$output" == *"zz-rogue-count.md"* ]]
}

@test "B80: the manifested prose docs carry generator-owned marker blocks; hand-asserted counts are extinct" {
  [ -s "$REAL_REPO_ROOT/scripts/count-docs.txt" ]

  clone="$(real_repo_clone)"
  # every manifested doc carries marker blocks
  ndocs=0
  while IFS= read -r doc; do
    case "$doc" in \#*|"") continue ;; esac
    grep -q '<!-- count:skills -->' "$clone/$doc"
    ndocs=$((ndocs + 1))
  done < "$clone/scripts/count-docs.txt"
  [ "$ndocs" -ge 1 ]

  # the repo-wide out-of-marker sweep returns 0 matches on the cutover commit
  run land "$clone" --check-counts
  [ "$status" -eq 0 ]

  # editing one marker value to a wrong number, then running the generator,
  # restores the generated value (byte-level)
  doc="$(grep -v '^#' "$clone/scripts/count-docs.txt" | grep . | head -1)"
  pre="$(sha256_file "$clone/$doc")"
  perl -0pi -e 's|(<!-- count:skills -->)[0-9]+(<!-- /count -->)|${1}999999${2}|s' "$clone/$doc"
  [ "$(sha256_file "$clone/$doc")" != "$pre" ]
  ( cd "$clone" && bash scripts/regen-all.sh )
  [ "$(sha256_file "$clone/$doc")" = "$pre" ]

  # the conversion commit itself introduces zero drift
  c2="$(real_repo_clone)"
  run bash -c "cd '$c2' && bash scripts/regen-all.sh --check"
  [ "$status" -eq 0 ]
}

@test "B81: real validate.yml declares land-gate-families STRUCTURALLY and the parity check holds" {
  wf="$REAL_REPO_ROOT/.github/workflows/validate.yml"
  grep -q 'land-gate-families' "$wf"
  [ -x "$REAL_REPO_ROOT/$V_GATE_PARITY" ]

  clone="$(real_repo_clone)"
  cwf="$clone/.github/workflows/validate.yml"

  # exactly one declaration; parity (land.sh families ⊇ declared; every
  # declared family maps to a real CI job/step) holds on the cutover commit
  [ "$(grep -c 'land-gate-families' "$cwf")" -ge 1 ]
  run bash -c "cd '$clone' && $V_GATE_PARITY"
  [ "$status" -eq 0 ]

  w="$BATS_TEST_TMPDIR/validate.yml"

  # a commented-out declaration is REJECTED (structural parse, not grep)
  sed -E 's/^([[:space:]]*)(land-gate-families)/\1# \2/' "$cwf" > "$w"
  run bash -c "cd '$clone' && $V_GATE_PARITY --workflow '$w'"
  [ "$status" -ne 0 ]

  # duplicate declarations are REJECTED
  decl="$(grep -E 'land-gate-families' "$cwf" | head -1)"
  { cat "$cwf"; printf '%s\n' "$decl"; } > "$w"
  run bash -c "cd '$clone' && $V_GATE_PARITY --workflow '$w'"
  [ "$status" -ne 0 ]

  # an empty family list is REJECTED
  sed -E 's/^([[:space:]]*land-gate-families:).*/\1 ""/' "$cwf" > "$w"
  run bash -c "cd '$clone' && $V_GATE_PARITY --workflow '$w'"
  [ "$status" -ne 0 ]

  # removing one family token fails the parity check naming the family
  fam="$(grep -E 'land-gate-families' "$cwf" | head -1 \
    | sed -E 's/.*land-gate-families:[[:space:]]*"?//; s/".*//' | awk '{print $1}')"
  [ -n "$fam" ]
  sed -E "s/(land-gate-families:[[:space:]]*\"?)$fam[[:space:]]*/\1/" "$cwf" > "$w"
  run bash -c "cd '$clone' && $V_GATE_PARITY --workflow '$w'"
  [ "$status" -ne 0 ]
  [[ "$output" == *"$fam"* ]]
}
