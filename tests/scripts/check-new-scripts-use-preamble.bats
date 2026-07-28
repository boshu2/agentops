#!/usr/bin/env bats
# check-new-scripts-use-preamble.bats — acceptance for the preamble adoption
# ratchet (age-gate-the-ungated-egwt.10). The contract:
#   * a NEW top-level scripts/*.sh that hand-rolls its preamble (e.g.
#     REPO_ROOT="$(pwd)") is FLAGGED, with the exact paste-able source line;
#   * the same script + a non-empty `# preamble-exempt: <reason>` PASSES;
#   * a GRANDFATHERED script that now sources preamble.sh FAILS demanding a prune
#     (allowlist only shrinks);
#   * a script that sources preamble.sh PASSES;
#   * a `# preamble-exempt:` marker with an EMPTY reason is STILL flagged
#     (anti cargo-cult).
#
# The gate derives its changed set from git, so each test builds a throwaway git
# repo that mirrors the real layout (scripts/lib/preamble.sh + the check script +
# a grandfather snapshot), commits a clean base, then stages the case-under-test
# and runs the gate over --scope staged. Fully offline; never touches the real repo.

setup() {
  SCRIPT_SRC="$BATS_TEST_DIRNAME/../../scripts/check-new-scripts-use-preamble.sh"
  LIB_SRC="$BATS_TEST_DIRNAME/../../scripts/lib/preamble.sh"
  REPO="$(mktemp -d "${TMPDIR:-/tmp}/preamble-ratchet-bats.XXXXXX")"

  mkdir -p "$REPO/scripts/lib"
  cp "$SCRIPT_SRC" "$REPO/scripts/check-new-scripts-use-preamble.sh"
  cp "$LIB_SRC" "$REPO/scripts/lib/preamble.sh"
  # shared ratchet mechanics (age-ratchet-lib-extraction-bv7d.4)
  cp "$BATS_TEST_DIRNAME/../../scripts/lib/ratchet.sh" "$REPO/scripts/lib/ratchet.sh"

  # A minimal grandfather snapshot with ONE pre-existing hand-rolled script that
  # we can later mutate to test the shrink ratchet.
  cat > "$REPO/scripts/legacy-grandfathered.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="$(pwd)"   # hand-rolled — grandfathered, allowed to stay
echo "$REPO_ROOT"
EOF
  {
    echo "# grandfather snapshot (test fixture)"
    echo "scripts/legacy-grandfathered.sh"
  } > "$REPO/scripts/.preamble-grandfather"

  (
    cd "$REPO"
    git init -q
    git config user.email t@t.test
    git config user.name test
    git add -A
    git commit -qm base
  )
}

teardown() {
  rm -rf "$REPO"
}

# run_gate: stage the current worktree state and run the gate over the staged scope.
run_gate() {
  ( cd "$REPO" && git add -A && bash scripts/check-new-scripts-use-preamble.sh --scope staged )
}

@test "ratchet: NEW hand-rolled script is flagged with the paste-able source line" {
  cat > "$REPO/scripts/new-handrolled.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="$(pwd)"
echo "$REPO_ROOT"
EOF
  run run_gate
  [ "$status" -eq 1 ]
  [[ "$output" == *"scripts/new-handrolled.sh"* ]]
  [[ "$output" == *"does not source scripts/lib/preamble.sh"* ]]
  # the exact copy-pasteable fix must be printed
  [[ "$output" == *'lib/preamble.sh"'* ]]
  [[ "$output" == *'BASH_SOURCE'* ]]
}

@test "ratchet: same script + non-empty preamble-exempt reason PASSES" {
  cat > "$REPO/scripts/new-handrolled.sh" <<'EOF'
#!/usr/bin/env bash
# preamble-exempt: standalone bootstrap, no repo context
set -euo pipefail
REPO_ROOT="$(pwd)"
echo "$REPO_ROOT"
EOF
  run run_gate
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
}

@test "ratchet: grandfathered script that now sources preamble FAILS demanding a prune" {
  # Mutate the grandfathered script to adopt the preamble — the allowlist must shrink.
  cat > "$REPO/scripts/legacy-grandfathered.sh" <<'EOF'
#!/usr/bin/env bash
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"
echo "$REPO_ROOT"
EOF
  run run_gate
  [ "$status" -eq 1 ]
  [[ "$output" == *"scripts/legacy-grandfathered.sh"* ]]
  [[ "$output" == *"only SHRINKS"* ]]
  [[ "$output" == *".preamble-grandfather"* ]]
}

@test "ratchet: NEW script that sources the preamble PASSES" {
  cat > "$REPO/scripts/new-compliant.sh" <<'EOF'
#!/usr/bin/env bash
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"
echo "$REPO_ROOT"
EOF
  run run_gate
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
}

@test "ratchet: empty preamble-exempt reason is STILL flagged (anti cargo-cult)" {
  cat > "$REPO/scripts/new-cargocult.sh" <<'EOF'
#!/usr/bin/env bash
# preamble-exempt:
set -euo pipefail
REPO_ROOT="$(pwd)"
echo "$REPO_ROOT"
EOF
  run run_gate
  [ "$status" -eq 1 ]
  [[ "$output" == *"scripts/new-cargocult.sh"* ]]
  [[ "$output" == *"EMPTY reason"* ]]
}

@test "ratchet: a MODIFIED grandfathered script that stays hand-rolled still PASSES" {
  # Editing a grandfathered script (without adopting the preamble) must NOT
  # retroactively force adoption — zero churn of the existing tree.
  cat >> "$REPO/scripts/legacy-grandfathered.sh" <<'EOF'
echo "an extra line, still hand-rolled"
EOF
  run run_gate
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
}

@test "ratchet: a changed scripts/lib/** file is NOT governed (libs exempt as a class)" {
  # A new sourced library fragment under scripts/lib/ is never scanned.
  cat > "$REPO/scripts/lib/helper.sh" <<'EOF'
#!/usr/bin/env bash
# a sourced fragment, hand-rolls nothing relevant
my_helper() { echo hi; }
EOF
  run run_gate
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
}

@test "ratchet: allowlisting a NEW script in the same diff is REJECTED (snapshot only shrinks)" {
  # The bypass the shrink-only rule closes: ship a hand-rolled script AND append
  # its path to the grandfather snapshot in one change. BOTH halves must fail
  # independently: the snapshot growth is rejected, AND the appended entry
  # grants no protection (grandfather authority = base ∩ working), so the
  # script itself is still flagged as missing the preamble.
  cat > "$REPO/scripts/new-sneaky.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="$(pwd)"
echo "$REPO_ROOT"
EOF
  echo "scripts/new-sneaky.sh" >> "$REPO/scripts/.preamble-grandfather"
  run run_gate
  [ "$status" -eq 1 ]
  # half 1: the snapshot growth itself is rejected
  [[ "$output" == *"gained new entries"* ]]
  [[ "$output" == *"only SHRINKS"* ]]
  # half 2: the appended entry did NOT protect the script — it is still flagged
  [[ "$output" == *"scripts/new-sneaky.sh (new/changed) does not source scripts/lib/preamble.sh"* ]]
}

@test "ratchet: pruning a grandfather entry (shrinking) passes the shrink-only rule" {
  # Removing the grandfathered script AND its snapshot line together is the
  # legal shrink direction.
  rm "$REPO/scripts/legacy-grandfathered.sh"
  grep -v '^scripts/legacy-grandfathered.sh$' "$REPO/scripts/.preamble-grandfather" > "$REPO/.gf" \
    && mv "$REPO/.gf" "$REPO/scripts/.preamble-grandfather"
  run run_gate
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
}

# --- collector fail-closed witnesses (advisory F1) -----------------------------

# The library documents every chosen collection command as fail-closed rc 2.
# This gate's caller swallowed that rc with `2>/dev/null || true` — the
# byte-identical idiom the python-ratchet repair removed: a dead Git read
# became an EMPTY changed set and the gate certified green over a diff nobody
# read. Both witnesses seed a REAL violation first, so a swallowed failure
# would have to produce a visibly wrong green. The shim targets `git diff`,
# the staged-scope collection command, so the witness encodes the contract,
# not either implementation.
shim_git_diff() {
  local emit_row="${1:-}"
  mkdir -p "$REPO/bin"
  {
    echo '#!/usr/bin/env bash'
    echo 'if [ "$1" = "diff" ]; then'
    if [[ -n "$emit_row" ]]; then
      printf '  printf "%s"\n' "$emit_row"
    fi
    echo '  echo "fatal: injected diff failure" >&2'
    echo '  exit 128'
    echo 'fi'
    echo 'exec /usr/bin/env -u PATH PATH="/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin" git "$@"'
  } > "$REPO/bin/git"
  chmod +x "$REPO/bin/git"
}

@test "ratchet: a failing changed-scope collector exits 2, never a silent PASS" {
  cat > "$REPO/scripts/new-handrolled.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="$(pwd)"
echo "$REPO_ROOT"
EOF
  ( cd "$REPO" && git add -A )
  shim_git_diff
  run bash -c "cd '$REPO' && PATH=\"$REPO/bin:\$PATH\" bash scripts/check-new-scripts-use-preamble.sh --scope staged"
  [ "$status" -eq 2 ]
  # The gate's own refusal must be loud...
  [[ "$output" == *"refusing to certify"* ]]
  # ...and the library's stderr must be PRESERVED, not routed to /dev/null.
  [[ "$output" == *"git diff --cached failed"* ]]
  # A swallowed failure would certify green over an unchecked diff.
  [[ "$output" != *"PASS"* ]]
}

@test "ratchet: a PARTIAL collector (row emitted, then death) is never certified against" {
  cat > "$REPO/scripts/new-handrolled.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="$(pwd)"
echo "$REPO_ROOT"
EOF
  ( cd "$REPO" && git add -A )
  # The shim emits one plausible name-status row, THEN dies: truncated bytes
  # must not be processed as a changed set.
  shim_git_diff 'A\tscripts/ghost.sh\n'
  run bash -c "cd '$REPO' && PATH=\"$REPO/bin:\$PATH\" bash scripts/check-new-scripts-use-preamble.sh --scope staged"
  [ "$status" -eq 2 ]
  [[ "$output" == *"refusing to certify"* ]]
  # The partially collected path must never surface as a finding — findings
  # over truncated bytes are as wrong as certifying green over them.
  [[ "$output" != *"ghost.sh"* ]]
  [[ "$output" != *"PASS"* ]]
}

@test "ratchet: the INITIAL grandfather snapshot may be added (nothing to ratchet against)" {
  # A repo whose base commit has no snapshot at all: adding the first snapshot
  # is the cutoff itself, not growth.
  INIT="$(mktemp -d "${TMPDIR:-/tmp}/preamble-ratchet-init.XXXXXX")"
  mkdir -p "$INIT/scripts/lib"
  cp "$SCRIPT_SRC" "$INIT/scripts/check-new-scripts-use-preamble.sh"
  cp "$LIB_SRC" "$INIT/scripts/lib/preamble.sh"
  # shared ratchet mechanics — this SECOND skeleton needs the lib too (the
  # per-copy-site table from premortem FM5 called out exactly this fixture)
  cp "$BATS_TEST_DIRNAME/../../scripts/lib/ratchet.sh" "$INIT/scripts/lib/ratchet.sh"
  cat > "$INIT/scripts/old-handrolled.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="$(pwd)"
EOF
  (
    cd "$INIT"
    git init -q
    git config user.email t@t.test
    git config user.name test
    git add -A
    git commit -qm base
  )
  {
    echo "# grandfather snapshot (initial)"
    echo "scripts/old-handrolled.sh"
  } > "$INIT/scripts/.preamble-grandfather"
  run bash -c 'cd "'"$INIT"'" && git add -A && bash scripts/check-new-scripts-use-preamble.sh --scope staged'
  rm -rf "$INIT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
}
