#!/usr/bin/env bats
# age-kg5l: TAG-SIBLING PACKET CONTEXT. When the reviewed diff touches a build-tagged
# (//go:build) .go file, the packet appends a clearly-delimited CONTEXT-NOT-DIFF section of the
# tag-SIBLING files (same-dir foo.go -> foo_*.go + legacy/flywheel base variants) — the
# tag-guarded expectations a refuter reviewing only the diff cannot see (the 2026-07-02
# false-REFUTED class). Siblings appear AFTER the diff and are truncated (never the diff) to keep
# the packet under the inline ceiling; a non-tag diff produces a byte-identical packet to today.
#
# build_tag_sibling_context is pure, so the unit tests source it in a subshell; the integration
# tests capture the actual packet fed to the (stubbed) reviewer.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  # Reviewer stub: capture the packet fed on stdin (when PKT_CAPTURE is set), then CONFIRM.
  cat > "$BIN/codex" <<'STUB'
#!/usr/bin/env bash
if [[ -n "${PKT_CAPTURE:-}" ]]; then cat > "$PKT_CAPTURE"; else cat >/dev/null; fi
printf 'codex\nVERDICT: CONFIRMED\n'
exit 0
STUB
  chmod +x "$BIN/codex"
  printf '#!/usr/bin/env bash\nexit 0\n' > "$BIN/ao"; chmod +x "$BIN/ao"   # hermetic emit
  PATH="$BIN:$PATH"

  REPO="$TMP/repo"; mkdir -p "$REPO/cli/cmd/ao"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  printf '//go:build flywheel\n\npackage ao\n\nfunc Foo() int { return 1 }\n' > cli/cmd/ao/foo.go
  printf '//go:build legacy\n\npackage ao\n\n// LEGACY-EXPECTATION-MARKER: Foo returns 42\nfunc FooLegacy() int { return 42 }\n' > cli/cmd/ao/foo_legacy.go
  printf 'package ao\n\nfunc Bar() int { return 1 }\n' > cli/cmd/ao/bar.go   # NON-tag, unrelated
  git add -A; git commit --quiet -m init
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"; mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  export PAWL_NO_SERVICE=1
  # synthetic diffs for the pure unit tests
  export TAGDIFF="$(printf 'diff --git a/cli/cmd/ao/foo.go b/cli/cmd/ao/foo.go\n--- a/cli/cmd/ao/foo.go\n+++ b/cli/cmd/ao/foo.go\n@@ -5 +5 @@\n-func Foo() int { return 1 }\n+func Foo() int { return 2 }\n')"
  export NONDIFF="$(printf 'diff --git a/cli/cmd/ao/bar.go b/cli/cmd/ao/bar.go\n--- a/cli/cmd/ao/bar.go\n+++ b/cli/cmd/ao/bar.go\n@@ -3 +3 @@\n-func Bar() int { return 1 }\n+func Bar() int { return 2 }\n')"
  export ROOT="$REPO"
}

teardown() { cd "$ORIG_DIR" 2>/dev/null || true; rm -rf "$TMP"; }

# ---------------------------------------------------------------------------
# unit: build_tag_sibling_context (sourced in a subshell so `set -u` never leaks)
# ---------------------------------------------------------------------------
@test "build_tag_sibling_context: a tag-touching diff yields a delimited sibling section with the sibling content, excluding the touched file" {
  run bash -c 'source "'"$SCRIPT"'"; build_tag_sibling_context "$TAGDIFF" "$ROOT" 65536'
  [ "$status" -eq 0 ]
  [[ "$output" == *"CONTEXT (NOT PART OF THE DIFF)"* ]]                       # delimited section
  [[ "$output" == *"sibling (context, unchanged): cli/cmd/ao/foo_legacy.go"* ]]  # names the sibling
  [[ "$output" == *"LEGACY-EXPECTATION-MARKER: Foo returns 42"* ]]            # includes its content
  [[ "$output" != *"sibling (context, unchanged): cli/cmd/ao/foo.go "* ]]     # the touched file is NOT a sibling
}

@test "build_tag_sibling_context: a NON-tag diff yields EMPTY (packet stays byte-identical to today)" {
  run bash -c 'source "'"$SCRIPT"'"; build_tag_sibling_context "$NONDIFF" "$ROOT" 65536'
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "build_tag_sibling_context: an oversize sibling is TRUNCATED to the budget (never exceeds it) with a truncation note" {
  # add a huge tag-sibling; at a small budget the section must be truncated to fit.
  yes 'padpadpadpadpadpadpadpadpadpadpadpadpadpadpadpadpad' 2>/dev/null | head -n 5000 >> "$REPO/cli/cmd/ao/foo_huge_flywheel.go"
  run bash -c 'source "'"$SCRIPT"'"; build_tag_sibling_context "$TAGDIFF" "$ROOT" 2000 | wc -c | tr -d " "'
  [ "$status" -eq 0 ]
  [ "$output" -le 2000 ]                                                      # within the byte budget
  run bash -c 'source "'"$SCRIPT"'"; build_tag_sibling_context "$TAGDIFF" "$ROOT" 2000'
  [[ "$output" == *"truncated to fit the packet size ceiling"* ]]            # truncation is announced
}

@test "build_tag_sibling_context: a budget below the truncation-note floor emits NOTHING (never overflows)" {
  # cross-family refute (age-kg5l landing): with budget=1 the truncation note alone (~99B)
  # leaked past the caller's absolute ceiling. Emission must fit the budget exactly or be empty.
  yes 'padpadpadpadpadpadpadpadpadpadpadpadpadpadpadpadpad' 2>/dev/null | head -n 200 >> "$REPO/cli/cmd/ao/foo_huge_flywheel.go"
  for b in 99 50 1; do
    run bash -c 'source "'"$SCRIPT"'"; build_tag_sibling_context "$TAGDIFF" "$ROOT" '"$b"' | wc -c | tr -d " "'
    [ "$status" -eq 0 ]
    [ "$output" -le "$b" ]
  done
}

# ---------------------------------------------------------------------------
# integration: the ACTUAL packet fed to the reviewer
# ---------------------------------------------------------------------------
@test "packet (tag diff): contains BOTH the diff and the tag-sibling CONTEXT section" {
  printf '//go:build flywheel\n\npackage ao\n\nfunc Foo() int { return 2 }\n' > "$REPO/cli/cmd/ao/foo.go"
  git -C "$REPO" commit --quiet -am "feat(ao): change tagged Foo (age-tag-test)"
  PKT="$TMP/packet.txt"
  PKT_CAPTURE="$PKT" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-tag-test --scope head
  [ "$status" -eq 0 ]
  [ -f "$PKT" ]
  grep -q "func Foo() int { return 2 }" "$PKT"                 # the DIFF is present (intact)
  grep -q "CONTEXT (NOT PART OF THE DIFF)" "$PKT"              # the sibling section is appended
  grep -q "cli/cmd/ao/foo_legacy.go" "$PKT"                    # names the tag sibling
  grep -q "LEGACY-EXPECTATION-MARKER" "$PKT"                   # includes the sibling's expectation
}

@test "packet (non-tag diff): NO sibling section appended (unchanged flow)" {
  printf 'package ao\n\nfunc Bar() int { return 2 }\n' > "$REPO/cli/cmd/ao/bar.go"
  git -C "$REPO" commit --quiet -am "feat(ao): change untagged Bar (age-nontag-test)"
  PKT="$TMP/packet2.txt"
  PKT_CAPTURE="$PKT" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-nontag-test --scope head
  [ "$status" -eq 0 ]
  [ -f "$PKT" ]
  grep -q "func Bar() int { return 2 }" "$PKT"                 # the diff is present
  ! grep -q "CONTEXT (NOT PART OF THE DIFF)" "$PKT"            # NO sibling section for a non-tag diff
}
