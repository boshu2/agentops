#!/usr/bin/env bats
# Regression tests for the Markdown body assembled by extract-release-notes.sh.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SANDBOX="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$SANDBOX/docs/releases"

  cat > "$SANDBOX/CHANGELOG.md" <<'EOF'
# Changelog

## [9.9.9] - 2099-01-01

### Fixed

- A changelog item whose source is
  wrapped across multiple lines.

A changelog paragraph is deliberately
hard-wrapped and should become one logical line.

| Changelog area | Result |
|---|---|
| Release notes | fixed |

## [9.9.8] - 2098-01-01
EOF

  cat > "$SANDBOX/docs/releases/2099-01-01-v9.9.9-notes.md" <<'EOF'
## Highlights

This paragraph is deliberately hard-wrapped
across several source lines so the published
release must reflow it.

A prose line containing `left | right` is still
soft-wrapped prose rather than a table.

## Upgrade Notes

- This list item is deliberately wrapped
  across source lines too.

- Parent item has a soft-wrapped
  continuation that should join.
  - Nested item has a soft-wrapped
    continuation that should join too.
- Sibling item remains distinct.

1. Ordered item is deliberately wrapped
   across source lines too.
2. Ordered sibling remains distinct.

   ### Indented heading
Paragraph after the indented heading is
soft-wrapped but remains a separate block.

Product Area | Fixed
--- | ---:
Release Notes | 1
EOF

  printf '\n%s  \n' "An intentional hard break stays here." \
    >> "$SANDBOX/docs/releases/2099-01-01-v9.9.9-notes.md"

  cat >> "$SANDBOX/docs/releases/2099-01-01-v9.9.9-notes.md" <<'EOF'
This starts a new rendered line.

A backslash hard break stays here.\
This also starts a new rendered line.

  ~~~text
  fenced code
  keeps its lines
  ~~~

    indented code
    keeps its lines too

<div class="release-note">
HTML content stays
on separate source lines.
</div>

> quoted first line
> quoted second line

[release-ref]:
  https://example.test/releases/9.9.9
  "Release details"

[Full changelog](../CHANGELOG.md)
EOF
}

assert_line() {
  grep -Fqx -- "$1" "$SANDBOX/release-notes.md"
}

@test "reflows prose and list continuations in curated notes and changelog" {
  run bash -c "cd '$SANDBOX' && '$REPO_ROOT/scripts/extract-release-notes.sh' v9.9.9 v9.9.8"
  [ "$status" -eq 0 ]

  assert_line "This paragraph is deliberately hard-wrapped across several source lines so the published release must reflow it."
  assert_line "A prose line containing \`left | right\` is still soft-wrapped prose rather than a table."
  assert_line "- This list item is deliberately wrapped across source lines too."
  assert_line "- A changelog item whose source is wrapped across multiple lines."
  assert_line "A changelog paragraph is deliberately hard-wrapped and should become one logical line."
}

@test "retains structural Markdown lines and explicit hard breaks" {
  run bash -c "cd '$SANDBOX' && '$REPO_ROOT/scripts/extract-release-notes.sh' v9.9.9 v9.9.8"
  [ "$status" -eq 0 ]

  assert_line "- Parent item has a soft-wrapped continuation that should join."
  assert_line "  - Nested item has a soft-wrapped continuation that should join too."
  assert_line "- Sibling item remains distinct."
  assert_line "1. Ordered item is deliberately wrapped across source lines too."
  assert_line "2. Ordered sibling remains distinct."

  assert_line "   ### Indented heading"
  assert_line "Paragraph after the indented heading is soft-wrapped but remains a separate block."

  assert_line "Product Area | Fixed"
  assert_line "--- | ---:"
  assert_line "Release Notes | 1"
  assert_line "| Changelog area | Result |"
  assert_line "|---|---|"
  assert_line "| Release notes | fixed |"

  assert_line "An intentional hard break stays here.  "
  assert_line "This starts a new rendered line."
  assert_line "A backslash hard break stays here.\\"
  assert_line "This also starts a new rendered line."

  assert_line "  ~~~text"
  assert_line "  fenced code"
  assert_line "  keeps its lines"
  assert_line "  ~~~"
  assert_line "    indented code"
  assert_line "    keeps its lines too"

  assert_line '<div class="release-note">'
  assert_line "HTML content stays"
  assert_line "on separate source lines."
  assert_line "</div>"
  assert_line "> quoted first line"
  assert_line "> quoted second line"
  assert_line "[release-ref]:"
  assert_line "  https://example.test/releases/9.9.9"
  assert_line '  "Release details"'

  assert_line "[Full changelog](https://github.com/boshu2/agentops/blob/main/docs/CHANGELOG.md)"
}
