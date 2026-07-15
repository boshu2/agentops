#!/usr/bin/env bats
# check-gate-fixhints-live.bats — self-unfixable-gate meta-check (ebec.12).
# Drives the script against a FIXTURE scripts tree + a stub ao whose live command
# set is controlled, so the dead-hint detection + removal-marker exemption are
# tested deterministically (no dependence on the live ao surface).

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/check-gate-fixhints-live.sh"
  WORK="$BATS_TEST_TMPDIR/fx"
  mkdir -p "$WORK/scripts/lib" "$WORK/bin"
  # a real preamble so the sourced lib resolves
  cp "$REPO_ROOT/scripts/lib/preamble.sh" "$WORK/scripts/lib/preamble.sh"
  cp "$SCRIPT" "$WORK/scripts/check-gate-fixhints-live.sh"
  ( cd "$WORK" && git init -q && git config user.email t@t.local && git config user.name t && git add -A && git commit -qm init )
  # stub ao: live commands are `gate` and `done` only ('corpus' is DEAD)
  cat > "$WORK/bin/ao" <<'SH'
#!/usr/bin/env bash
if [[ "$1" == "__complete" ]]; then printf 'gate\tGate\ndone\tClose\n'; fi
SH
  chmod +x "$WORK/bin/ao"
}

run_meta() { ( cd "$WORK" && PATH="$WORK/bin:$PATH" bash scripts/check-gate-fixhints-live.sh "$@" ); }

@test "clean: fix-hint names a LIVE command -> PASS" {
  printf '#!/usr/bin/env bash\necho "  fix: run ao gate check --fast"\n' > "$WORK/scripts/check-a.sh"
  run run_meta
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
}

@test "dead hint (no marker) -> reported; warn-only exit 0" {
  printf '#!/usr/bin/env bash\necho "  fix: ao corpus snapshot"\n' > "$WORK/scripts/check-b.sh"
  run run_meta
  [ "$status" -eq 0 ]
  [[ "$output" == *"DEAD-FIXHINT"* ]]
  [[ "$output" == *"ao corpus"* ]]
  [[ "$output" == *"warn-only"* ]]
}

@test "dead hint under --strict -> exit 1" {
  printf '#!/usr/bin/env bash\necho "  fix: ao corpus snapshot"\n' > "$WORK/scripts/check-c.sh"
  run run_meta --strict
  [ "$status" -eq 1 ]
  [[ "$output" == *"DEAD-FIXHINT"* ]]
}

@test "dead command WITH a removal marker (historical ref) -> NOT flagged" {
  printf '#!/usr/bin/env bash\necho "  historical: ao corpus snapshot was removed"\n' > "$WORK/scripts/check-d.sh"
  run run_meta --strict
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
}

@test "no ao on PATH -> SKIP (hygiene gate, not a hard dep)" {
  printf '#!/usr/bin/env bash\necho "  fix: ao corpus snapshot"\n' > "$WORK/scripts/check-e.sh"
  run env PATH="/usr/bin:/bin" bash -c "cd '$WORK' && bash scripts/check-gate-fixhints-live.sh"
  [ "$status" -eq 0 ]
  [[ "$output" == *"SKIP"* ]]
}
