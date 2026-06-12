#!/usr/bin/env bats
# §5 Derived surfaces & regen — B42–B48

load helpers

setup()    { sandbox_setup; }
teardown() { sandbox_teardown; }

@test "B42: the derived write set is a manifest, never a hard-coded list" {
  # Register a NEW generator + surface via the manifest ONLY.
  LANE="$(new_lane a feat-newgen)"
  cat > "$LANE/scripts/generators/60-new-surface.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
{ echo "# NEW SURFACE"; ls skills | sort; } > docs/NEW-SURFACE.md
EOF
  chmod +x "$LANE/scripts/generators/60-new-surface.sh"
  echo "docs/NEW-SURFACE.md" >> "$LANE/scripts/regen-manifest.txt"
  bash -c "cd '$LANE' && bash scripts/regen-all.sh"
  git -C "$LANE" add -A && git -C "$LANE" commit -qm "register new generator via manifest"
  add_skill "$LANE" zz-b42   # makes docs/NEW-SURFACE.md stale

  sut_before="$(shasum "$LANE/scripts/land.sh")"
  run land "$LANE"
  [ "$status" -eq 0 ]
  [ "$(shasum "$LANE/scripts/land.sh")" = "$sut_before" ]   # land.sh itself not edited
  c="$(fresh_clone)"
  grep -q 'zz-b42' "$c/docs/NEW-SURFACE.md"   # regenerated at land

  # manifest drift: generator writes an UNDECLARED path → check fails
  LD="$(new_lane d feat-drift)"
  cat > "$LD/scripts/generators/70-rogue.sh" <<'EOF'
#!/usr/bin/env bash
echo rogue > docs/UNDECLARED.md
EOF
  chmod +x "$LD/scripts/generators/70-rogue.sh"
  git -C "$LD" add -A && git -C "$LD" commit -qm "rogue generator, undeclared surface"
  run land "$LD"
  [ "$status" -ne 0 ]
  [[ "$output" == *"UNDECLARED"* ]] || [[ "$output" =~ manifest ]]

  # path declared BOTH source-owned and generator-owned → preflight contract error
  LO="$(new_lane o feat-overlap)"
  echo "docs/HAND.md" >> "$LO/scripts/regen-manifest.txt"
  git -C "$LO" commit -qam "declare a source-owned path generator-owned"
  run land "$LO"
  [ "$status" -ne 0 ]
  [[ "$output" == *"docs/HAND.md"* ]]
}

@test "B43: generator failure mid-land aborts with everything restored" {
  LANE="$(new_lane a feat-x)"
  cat > "$LANE/scripts/generators/15-broken.sh" <<'EOF'
#!/usr/bin/env bash
printf 'partial-' > registry.json
echo "generator exploding on purpose" >&2
exit 3
EOF
  chmod +x "$LANE/scripts/generators/15-broken.sh"
  git -C "$LANE" add -A && git -C "$LANE" commit -qm "broken generator"
  add_skill "$LANE" zz-b43
  orig="$(git -C "$LANE" rev-parse HEAD)"
  before="$(remote_main_sha)"

  run land "$LANE"
  [ "$status" -ne 0 ]
  [[ "$output" == *"15-broken"* ]]
  [[ "$output" == *"exploding on purpose"* ]]   # carries the generator's stderr
  [ "$(remote_main_sha)" = "$before" ]
  run land "$LANE" --status
  [[ "$output" == *unheld* ]]
  worktree_clean "$LANE"
  [ "$(git -C "$LANE" rev-parse HEAD)" = "$orig" ]
  ! grep -q '^partial-' "$LANE/registry.json"   # no partial file left
}

@test "B44: nondeterministic generator output is detected before push" {
  LANE="$(new_lane a feat-x)"
  cat > "$LANE/scripts/generators/16-flaky.sh" <<'EOF'
#!/usr/bin/env bash
date +%s%N > docs/SKILL-TIERS.md
EOF
  chmod +x "$LANE/scripts/generators/16-flaky.sh"
  git -C "$LANE" add -A && git -C "$LANE" commit -qm "nondeterministic generator"
  add_skill "$LANE" zz-b44
  before="$(remote_main_sha)"

  run land "$LANE"
  [ "$status" -ne 0 ]
  [[ "$output" == *"nondeterministic generator output"* ]]
  [[ "$output" == *"SKILL-TIERS"* ]]
  [[ "$output" == *"16-flaky"* ]]
  [ "$(remote_main_sha)" = "$before" ]
}

@test "B45: deleting or renaming a skill leaves no stale derived entries" {
  # seed aa-one and bb-two onto main first
  SETUP="$(new_lane setup feat-setup)"
  add_skill "$SETUP" aa-one; add_skill "$SETUP" bb-two
  ( cd "$SETUP" && bash scripts/regen-all.sh && git add -A && git commit -qm regen && git push -q origin HEAD:main )

  # delete aa-one
  LR="$(new_lane rm rm-skill)"
  git -C "$LR" rm -qr skills/aa-one
  git -C "$LR" commit -qm "remove aa-one"
  run land "$LR"
  [ "$status" -eq 0 ]

  # rename bb-two → bb-three
  LM="$(new_lane mv mv-skill)"
  git -C "$LM" mv skills/bb-two skills/bb-three
  sed -i.bak 's/name: bb-two/name: bb-three/' "$LM/skills/bb-three/SKILL.md" && rm -f "$LM/skills/bb-three/SKILL.md.bak"
  git -C "$LM" add -A && git -C "$LM" commit -qm "rename bb-two to bb-three"
  run land "$LM"
  [ "$status" -eq 0 ]

  c="$(fresh_clone)"
  for surface in registry.json docs/context-map.md docs/SKILL-TIERS.md; do
    ! grep -q 'aa-one' "$c/$surface"
    ! grep -q 'bb-two\b' "$c/$surface"
    grep -q 'bb-three' "$c/$surface" || [ "$surface" = registry.json ]
  done
  jq -e '[.skills[].name] | index("aa-one") == null and index("bb-two") == null and index("bb-three") != null' "$c/registry.json"
  [ ! -d "$c/skills-codex/aa-one" ]
  [ ! -d "$c/skills-codex/bb-two" ]
  [ -d "$c/skills-codex/bb-three" ]
  ( cd "$c" && bash scripts/regen-all.sh --check )
}

@test "B46: hand edits to generator-owned files are reset to canonical output" {
  # branch "sneaky": hand-edit registry.json, no source change
  LS="$(new_lane s sneaky)"
  jq '. + {sneaky: true}' "$LS/registry.json" > "$LS/registry.json.tmp" && mv "$LS/registry.json.tmp" "$LS/registry.json"
  git -C "$LS" commit -qam "sneaky hand edit to registry"
  run land "$LS"
  sneaky_out="$output"
  c="$(fresh_clone)"
  ( cd "$c" && bash scripts/regen-all.sh )
  worktree_clean "$c"                       # byte-identical to fresh generator run
  ! jq -e '.sneaky' "$c/registry.json" >/dev/null 2>&1
  grep -qiE 'discard|reset|overwrit' <<<"$sneaky_out"
  grep -q 'registry.json' <<<"$sneaky_out"  # warning names the discarded path

  # branch "mixed": real source change + hand-edited stale registry in one commit
  LM="$(new_lane m mixed)"
  add_skill "$LM" zz-b46-real "wip"
  jq '. + {planted: "wrong"}' "$LM/registry.json" > "$LM/registry.json.tmp" && mv "$LM/registry.json.tmp" "$LM/registry.json"
  git -C "$LM" add -A && git -C "$LM" commit -qm "mixed: source + hand-edited generated"
  run land "$LM"
  [ "$status" -eq 0 ]
  c="$(fresh_clone)"
  [ -d "$c/skills/zz-b46-real" ]                                  # source landed intact
  ! jq -e '.planted' "$c/registry.json" >/dev/null 2>&1           # hand edit gone
  jq -e '.skills[] | select(.name=="zz-b46-real")' "$c/registry.json" >/dev/null
}

@test "B47: the strict-JSON verifier is broad and independently testable" {
  LANE="$(new_lane a feat-x)"

  # clean tree → 0
  run land "$LANE" --verify-generated-json
  [ "$status" -eq 0 ]

  # duplicate key in a non-codex generated JSON
  printf '{"skills": [], "skills": []}\n' > "$LANE/registry.json"
  run land "$LANE" --verify-generated-json
  [ "$status" -ne 0 ]
  [[ "$output" == *"registry.json"* ]]
  [[ "$output" =~ duplicate ]]
  git -C "$LANE" checkout -q registry.json

  # invalid UTF-8 bytes
  printf '{"generated_hash": "\xff\xfe"}\n' > "$LANE/skills-codex/seed-skill/.agentops-generated.json"
  run land "$LANE" --verify-generated-json
  [ "$status" -ne 0 ]
  [[ "$output" == *"seed-skill"* ]]
  [[ "$output" =~ [Uu][Tt][Ff] ]]
  git -C "$LANE" checkout -q skills-codex

  # trailing garbage after the closing brace
  printf '{"skills": []}\ntrailing garbage\n' > "$LANE/registry.json"
  run land "$LANE" --verify-generated-json
  [ "$status" -ne 0 ]
  [[ "$output" == *"registry.json"* ]]
  [[ "$output" =~ trailing|garbage ]]
  git -C "$LANE" checkout -q registry.json

  # manifest-driven scope: a NEW generated-JSON manifest entry is checked with
  # no verifier edit
  echo "docs/extra-generated.json" >> "$LANE/scripts/regen-manifest.txt"
  printf '{"a":1,"a":2}\n' > "$LANE/docs/extra-generated.json"
  run land "$LANE" --verify-generated-json
  [ "$status" -ne 0 ]
  [[ "$output" == *"extra-generated.json"* ]]
}

@test "B48: the count checker reads a manifest and survives marker edge cases" {
  LANE="$(new_lane a feat-x)"

  # a NEW doc with a bare numeric skill-count outside marker blocks → fail, named
  echo "We now ship 2 skills in this repo." > "$LANE/docs/NEWDOC.md"
  echo "docs/NEWDOC.md" >> "$LANE/scripts/count-docs.txt"
  run land "$LANE" --check-counts
  [ "$status" -ne 0 ]
  [[ "$output" == *"NEWDOC.md"* ]]
  [[ "$output" =~ :[0-9]+|line ]]
  git -C "$LANE" checkout -q scripts/count-docs.txt; rm -f "$LANE/docs/NEWDOC.md"

  # marker block missing its closing tag → distinct error
  printf '# Broken\n<!-- count:skills -->2 skills, never closed\n' > "$LANE/docs/COUNTS.md"
  run land "$LANE" --check-counts
  [ "$status" -ne 0 ]
  missing_close_out="$output"
  grep -qiE 'clos|unterminated' <<<"$missing_close_out"

  # duplicate marker ids in one doc → distinct error
  printf '<!-- count:skills -->1<!-- /count -->\n<!-- count:skills -->1<!-- /count -->\n' > "$LANE/docs/COUNTS.md"
  run land "$LANE" --check-counts
  [ "$status" -ne 0 ]
  dup_out="$output"
  grep -qiE 'duplicate' <<<"$dup_out"
  [ "$missing_close_out" != "$dup_out" ]

  # non-count numeric is NOT flagged
  git -C "$LANE" checkout -q docs/COUNTS.md
  echo "we tried 47 times" >> "$LANE/docs/HAND.md"
  run land "$LANE" --check-counts
  [ "$status" -eq 0 ]
}
