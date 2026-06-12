#!/usr/bin/env bats
# §6 Gate — B49–B52

load helpers

setup()    { sandbox_setup; }
teardown() { sandbox_teardown; }

@test "B49: the gate runs every required family and aggregates across all of them" {
  LANE="$(new_lane a feat-allfail)"
  # one violation per gate family (fixture families): a failing shell check,
  # a failing "go test" stand-in, a broken doc link, invalid generated JSON,
  # and generator drift.
  cat > "$LANE/scripts/gate.d/10-bats-family.sh" <<'EOF'
#!/usr/bin/env bash
echo "bats-family: failing check"
exit 1
EOF
  cat > "$LANE/scripts/gate.d/20-go-family.sh" <<'EOF'
#!/usr/bin/env bash
echo "go-family: failing test"
exit 1
EOF
  cat > "$LANE/scripts/gate.d/30-doclinks.sh" <<'EOF'
#!/usr/bin/env bash
grep -q 'docs/DOES-NOT-EXIST.md' docs/HAND.md && { echo "doclinks: broken link docs/DOES-NOT-EXIST.md"; exit 1; }
exit 0
EOF
  chmod +x "$LANE"/scripts/gate.d/*.sh
  echo "[broken](docs/DOES-NOT-EXIST.md)" >> "$LANE/docs/HAND.md"
  printf '{"skills": [], "skills": []}\n' > "$LANE/registry.json"   # invalid generated JSON
  echo "drift" >> "$LANE/docs/context-map.md"                       # generator drift
  git -C "$LANE" add -A && git -C "$LANE" commit -qm "five family violations"

  run land "$LANE"
  [ "$status" -ne 0 ]
  [ "$(grep -c '== gate ==' <<<"$output")" -eq 1 ]   # ONE pass
  grep -q 'bats-family' <<<"$output"
  grep -q 'go-family' <<<"$output"
  grep -q 'DOES-NOT-EXIST' <<<"$output"
  grep -qE 'registry.json.*duplicate|duplicate.*registry.json' <<<"$output"
  grep -qE 'drift' <<<"$output"

  # CI parity: a family required by validate.yml but absent locally fails the parity check
  LP="$(new_lane p feat-parity)"
  sed -i.bak 's/# land-gate-families: .*/# land-gate-families: regen-check json-verify counts gate.d phantom-family/' \
    "$LP/.github/workflows/validate.yml" && rm -f "$LP/.github/workflows/validate.yml.bak"
  git -C "$LP" commit -qam "CI requires phantom-family"
  add_skill "$LP" zz-b49-p
  run land "$LP"
  [ "$status" -ne 0 ]
  [[ "$output" == *"phantom-family"* ]]

  # infrastructure preflight failure still fails fast (B22 discipline), not aggregated
  LI="$(new_lane i feat-infra)"; add_skill "$LI" zz-b49-i
  rm "$LI/scripts/regen-all.sh"
  run land "$LI"
  [ "$status" -ne 0 ]
  ! grep -q '== gate ==' <<<"$output"
}

@test "B50: a hung gate check is killed by timeout, not waited on forever" {
  LANE="$(new_lane a feat-hang)"
  cat > "$LANE/scripts/gate.d/50-hang.sh" <<EOF
#!/usr/bin/env bash
echo \$\$ > "$SANDBOX/hang-pid"
sleep 100000 &
echo \$! > "$SANDBOX/hang-child-pid"
wait
EOF
  chmod +x "$LANE/scripts/gate.d/50-hang.sh"
  git -C "$LANE" add -A && git -C "$LANE" commit -qm "hung gate check"
  add_skill "$LANE" zz-b50
  before="$(remote_main_sha)"

  t0="$(date +%s)"
  run env LAND_GATE_TIMEOUT=10 bash -c "cd '$LANE' && scripts/land.sh"
  t1="$(date +%s)"
  [ "$status" -ne 0 ]
  [ $((t1 - t0)) -le 20 ]                  # within 2x the timeout
  [[ "$output" == *"50-hang"* ]]           # names the timed-out check
  # the entire process group is dead — no orphan sleeper survives
  sleep 0.5
  if [ -f "$SANDBOX/hang-child-pid" ]; then
    ! kill -0 "$(cat "$SANDBOX/hang-child-pid")" 2>/dev/null
  fi
  if [ -f "$SANDBOX/hang-pid" ]; then
    ! kill -0 "$(cat "$SANDBOX/hang-pid")" 2>/dev/null
  fi
  [ "$(remote_main_sha)" = "$before" ]
  run land "$LANE" --status
  [[ "$output" == *unheld* ]]
}

@test "B51: a base main that is already red is reported as such, never blamed on the lane" {
  # plant drift on main itself
  d="$(mktemp -d "$SANDBOX/red.XXXX")"; git clone -q "$REMOTE" "$d/c"
  ( cd "$d/c" && echo "hand-planted drift" >> docs/context-map.md \
    && git commit -qam "plant drift on main" && git push -q origin main )
  # confirm the base is red
  c0="$(fresh_clone)"
  ! ( cd "$c0" && bash scripts/regen-all.sh --check )

  LANE="$(new_lane a feat-clean)"
  add_skill "$LANE" zz-b51
  lane_tip="$(git -C "$LANE" rev-parse HEAD)"
  before="$(remote_main_sha)"

  run land "$LANE"
  [ "$status" -ne 0 ]
  [[ "$output" == *"base main is already failing"* ]]
  # base failures listed separately from (zero) branch-introduced failures
  grep -qE 'base' <<<"$output"
  ! grep -qE 'zz-b51.*fail|branch.*introduced.*[1-9]' <<<"$output"
  [ "$(remote_main_sha)" = "$before" ]
  [ "$(git -C "$LANE" rev-parse HEAD)" = "$lane_tip" ]
  worktree_clean "$LANE"
}

@test "B52: post-land verification runs the full gate on a fresh clone of the remote" {
  LANE="$(new_lane a feat-x)"
  add_skill "$LANE" zz-b52
  # poison the LANE's working tree with uncommitted-but-gate-relevant local
  # state AFTER landing to prove verification cannot be reusing it
  run land "$LANE"
  [ "$status" -eq 0 ]
  echo "local-only poison" >> "$LANE/docs/context-map.md"

  # the discipline: verification = fresh clone FROM the fixture bare remote
  c="$(fresh_clone)"
  [ "$c" != "$LANE" ]
  ( cd "$c" && bash scripts/regen-all.sh --check )
  ! grep -q 'local-only poison' "$c/docs/context-map.md"
  # full gate bundle passes on the fresh clone
  for g in "$c"/scripts/gate.d/*.sh; do
    [ -e "$g" ] || continue
    ( cd "$c" && bash "$g" )
  done
  run bash -c "cd '$c' && scripts/land.sh --verify-generated-json"
  [ "$status" -eq 0 ]
  run bash -c "cd '$c' && scripts/land.sh --check-counts"
  [ "$status" -eq 0 ]
}
