#!/usr/bin/env bats
# Tests for scripts/validate-release-notes.sh — the release-notes standard gate.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/validate-release-notes.sh"
  # Sandbox repo so we can drop fixture notes into docs/releases/ without touching
  # the real tree. The script resolves paths relative to its own location, so we
  # copy it into the sandbox and run it there.
  SANDBOX="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$SANDBOX/scripts" "$SANDBOX/docs/releases"
  cp "$SCRIPT" "$SANDBOX/scripts/validate-release-notes.sh"
  chmod +x "$SANDBOX/scripts/validate-release-notes.sh"
}

# A minimal conforming MINOR/PATCH notes file (no Breaking Changes section).
write_patch_notes() {
  cat > "$SANDBOX/docs/releases/2026-05-25-v$1-notes.md" <<'EOF'
## Highlights

A conforming patch release.

## Upgrade Notes

- No manual action required.

## At a Glance

| Product Area | Added | Changed | Refactored | Fixed | Deprecated/Removed |
|---|---:|---:|---:|---:|---:|
| Eval, Validation, and Release Gates | 0 | 0 | 0 | 1 | 0 |

## Product Areas

### Eval, Validation, and Release Gates

- Fixed: a release gate no longer misfires.

## Known Issues

- No release-blocking known issues.

[Full changelog](../CHANGELOG.md)
EOF
}

@test "passes on a conforming patch notes file" {
  write_patch_notes "9.9.9"
  run "$SANDBOX/scripts/validate-release-notes.sh" v9.9.9
  [ "$status" -eq 0 ]
  [[ "$output" == *"conforms to the release-notes standard"* ]]
}

@test "accepts a version with or without leading v" {
  write_patch_notes "9.9.9"
  run "$SANDBOX/scripts/validate-release-notes.sh" 9.9.9
  [ "$status" -eq 0 ]
}

@test "fails when no curated notes file exists for the version" {
  run "$SANDBOX/scripts/validate-release-notes.sh" v1.2.3
  [ "$status" -ne 0 ]
  [[ "$output" == *"no curated release-notes file"* ]]
}

@test "fails when a required section is missing" {
  write_patch_notes "9.9.9"
  # Remove the Known Issues section heading.
  sed -i '/^## Known Issues$/d' "$SANDBOX/docs/releases/2026-05-25-v9.9.9-notes.md"
  run "$SANDBOX/scripts/validate-release-notes.sh" v9.9.9
  [ "$status" -ne 0 ]
  [[ "$output" == *"missing required section: ## Known Issues"* ]]
}

@test "fails a major release without a Breaking Changes section" {
  write_patch_notes "4.0.0"
  run "$SANDBOX/scripts/validate-release-notes.sh" v4.0.0
  [ "$status" -ne 0 ]
  [[ "$output" == *"must include a '## Breaking Changes' section"* ]]
}

@test "passes a major release with a Breaking Changes section" {
  cat > "$SANDBOX/docs/releases/2026-05-25-v4.0.0-notes.md" <<'EOF'
## Highlights

A conforming major.

## Upgrade Notes

- See docs/MIGRATION-4.0.md.

## Breaking Changes

- The old command is removed; use the new one.

## At a Glance

| Product Area | Added | Changed | Refactored | Fixed | Deprecated/Removed |
|---|---:|---:|---:|---:|---:|
| CLI and Operator Commands | 0 | 0 | 0 | 0 | 1 |

## Product Areas

### CLI and Operator Commands

- Removed: the old command, replaced by the new one.

## Known Issues

- No release-blocking known issues.

[Full changelog](../CHANGELOG.md)
EOF
  run "$SANDBOX/scripts/validate-release-notes.sh" v4.0.0
  [ "$status" -eq 0 ]
}

@test "fails a non-canonical product-area heading" {
  write_patch_notes "9.9.9"
  sed -i 's/^### Eval, Validation, and Release Gates$/### Made Up Area/' \
    "$SANDBOX/docs/releases/2026-05-25-v9.9.9-notes.md"
  run "$SANDBOX/scripts/validate-release-notes.sh" v9.9.9
  [ "$status" -ne 0 ]
  [[ "$output" == *"non-canonical product-area heading"* ]]
}

@test "fails a product-area bullet without a canonical action label" {
  write_patch_notes "9.9.9"
  sed -i 's/^- Fixed: a release gate no longer misfires.$/- a release gate no longer misfires./' \
    "$SANDBOX/docs/releases/2026-05-25-v9.9.9-notes.md"
  run "$SANDBOX/scripts/validate-release-notes.sh" v9.9.9
  [ "$status" -ne 0 ]
  [[ "$output" == *"canonical action label"* ]]
}

@test "reports the tier (hotfix) for an X.Y.Z version" {
  write_patch_notes "9.9.9"
  run "$SANDBOX/scripts/validate-release-notes.sh" v9.9.9
  [ "$status" -eq 0 ]
  [[ "$output" == *"tier hotfix"* ]]
}

@test "reports the tier (minor) for an X.Y.0 version" {
  write_patch_notes "9.9.0"
  run "$SANDBOX/scripts/validate-release-notes.sh" v9.9.0
  [ "$status" -eq 0 ]
  [[ "$output" == *"tier minor"* ]]
}

@test "minor (X.Y.0) does NOT require a Breaking Changes section" {
  write_patch_notes "9.9.0"
  run "$SANDBOX/scripts/validate-release-notes.sh" v9.9.0
  [ "$status" -eq 0 ]
}

@test "coverage gate FAILS when a touched area is missing from the notes" {
  write_patch_notes "9.9.9"   # notes document only "Eval, Validation, and Release Gates"
  printf 'cli/cmd/ao/a.go\ncli/cmd/ao/b.go\ncli/cmd/ao/c.go\n' > "$SANDBOX/changed.txt"
  run "$SANDBOX/scripts/validate-release-notes.sh" v9.9.9 --changed-files "$SANDBOX/changed.txt"
  [ "$status" -ne 0 ]
  [[ "$output" == *"CLI and Operator Commands"* ]]
  [[ "$output" == *"no '### CLI and Operator Commands' section"* ]]
}

@test "coverage gate PASSES when all touched areas are documented" {
  write_patch_notes "9.9.9"   # documents "Eval, Validation, and Release Gates"
  printf 'tests/scripts/x.bats\ntests/scripts/y.bats\nevals/z.json\n' > "$SANDBOX/changed.txt"
  run "$SANDBOX/scripts/validate-release-notes.sh" v9.9.9 --changed-files "$SANDBOX/changed.txt"
  [ "$status" -eq 0 ]
}

@test "coverage gate ignores areas below the file threshold" {
  write_patch_notes "9.9.9"
  printf 'cli/cmd/ao/just-one.go\n' > "$SANDBOX/changed.txt"   # 1 file < threshold 3
  run "$SANDBOX/scripts/validate-release-notes.sh" v9.9.9 --changed-files "$SANDBOX/changed.txt"
  [ "$status" -eq 0 ]
}
