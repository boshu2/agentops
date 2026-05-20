#!/usr/bin/env bats
# Regression tests for scripts/smart-skill-md-merge.sh (soc-trhs).

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/smart-skill-md-merge.sh"
  TMP="$(mktemp -d)"
}

teardown() {
  rm -rf "$TMP"
}

@test "resolves additive list-item conflict: keeps both sides" {
  cat >"$TMP/SKILL.md" <<'EOF'
# Skill

## Reference Documents

<<<<<<< HEAD
- [references/foo.md](references/foo.md) — foo
=======
- [references/bar.md](references/bar.md) — bar
>>>>>>> branch
- [references/baz.md](references/baz.md) — baz
EOF

  run "$SCRIPT" "$TMP/SKILL.md"
  [ "$status" -eq 0 ]
  run grep -c '<<<<<<<\|=======\|>>>>>>>' "$TMP/SKILL.md"
  [ "$status" -eq 1 ]  # grep exits 1 on zero matches
  grep -qF "references/foo.md" "$TMP/SKILL.md"
  grep -qF "references/bar.md" "$TMP/SKILL.md"
  grep -qF "references/baz.md" "$TMP/SKILL.md"
}

@test "bails on non-list conflict: file unchanged" {
  cat >"$TMP/SKILL.md" <<'EOF'
# Skill

<<<<<<< HEAD
This is a paragraph from ours.
=======
This is a paragraph from theirs.
>>>>>>> branch
EOF
  before=$(cat "$TMP/SKILL.md")

  run "$SCRIPT" "$TMP/SKILL.md"
  [ "$status" -eq 1 ]
  # File still contains conflict markers
  grep -q '<<<<<<<' "$TMP/SKILL.md"
}

@test "handles multiple list items per side" {
  cat >"$TMP/SKILL.md" <<'EOF'
## Reference Documents

<<<<<<< HEAD
- [refs/a.md](refs/a.md)
- [refs/b.md](refs/b.md)
=======
- [refs/c.md](refs/c.md)
- [refs/d.md](refs/d.md)
>>>>>>> branch
EOF

  run "$SCRIPT" "$TMP/SKILL.md"
  [ "$status" -eq 0 ]
  for ref in a b c d; do
    grep -qF "refs/${ref}.md" "$TMP/SKILL.md"
  done
}

@test "dedupes identical items across sides" {
  cat >"$TMP/SKILL.md" <<'EOF'
## Reference Documents

<<<<<<< HEAD
- [refs/shared.md](refs/shared.md)
- [refs/a.md](refs/a.md)
=======
- [refs/shared.md](refs/shared.md)
- [refs/b.md](refs/b.md)
>>>>>>> branch
EOF

  run "$SCRIPT" "$TMP/SKILL.md"
  [ "$status" -eq 0 ]
  # shared.md appears exactly once
  run grep -c "refs/shared.md" "$TMP/SKILL.md"
  [ "$output" = "1" ]
}

@test "--check exits 0 for resolvable, doesn't mutate file" {
  cat >"$TMP/SKILL.md" <<'EOF'
## Reference Documents

<<<<<<< HEAD
- [refs/a.md](refs/a.md)
=======
- [refs/b.md](refs/b.md)
>>>>>>> branch
EOF
  before=$(cat "$TMP/SKILL.md")

  run "$SCRIPT" --check "$TMP/SKILL.md"
  [ "$status" -eq 0 ]
  after=$(cat "$TMP/SKILL.md")
  [ "$before" = "$after" ]
}

@test "--check exits 1 for non-list conflict" {
  cat >"$TMP/SKILL.md" <<'EOF'
<<<<<<< HEAD
prose ours
=======
prose theirs
>>>>>>> branch
EOF

  run "$SCRIPT" --check "$TMP/SKILL.md"
  [ "$status" -eq 1 ]
}

@test "refuses non-markdown files" {
  cat >"$TMP/code.go" <<'EOF'
<<<<<<< HEAD
- a
=======
- b
>>>>>>> branch
EOF

  run "$SCRIPT" "$TMP/code.go"
  [ "$status" -eq 2 ]
}

@test "exits 2 when file has no conflicts" {
  cat >"$TMP/SKILL.md" <<'EOF'
# Clean
- [refs/a.md](refs/a.md)
EOF

  run "$SCRIPT" "$TMP/SKILL.md"
  [ "$status" -eq 2 ]
}

@test "handles indented list items (sub-lists)" {
  cat >"$TMP/SKILL.md" <<'EOF'
<<<<<<< HEAD
  - nested ours
=======
  - nested theirs
>>>>>>> branch
EOF

  run "$SCRIPT" "$TMP/SKILL.md"
  [ "$status" -eq 0 ]
  grep -qF "nested ours" "$TMP/SKILL.md"
  grep -qF "nested theirs" "$TMP/SKILL.md"
}

@test "preserves surrounding content unchanged" {
  cat >"$TMP/SKILL.md" <<'EOF'
# Title

Some intro paragraph.

## Reference Documents

<<<<<<< HEAD
- [refs/a.md](refs/a.md)
=======
- [refs/b.md](refs/b.md)
>>>>>>> branch

## See Also

End matter.
EOF

  run "$SCRIPT" "$TMP/SKILL.md"
  [ "$status" -eq 0 ]
  grep -qF "# Title" "$TMP/SKILL.md"
  grep -qF "Some intro paragraph." "$TMP/SKILL.md"
  grep -qF "## See Also" "$TMP/SKILL.md"
  grep -qF "End matter." "$TMP/SKILL.md"
}
