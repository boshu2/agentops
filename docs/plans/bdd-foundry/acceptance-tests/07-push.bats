#!/usr/bin/env bats
# §7 Push — B53–B56

load helpers

setup()    { sandbox_setup; }
teardown() { sandbox_teardown; }

@test "B53: push failures are classified and leave no half-land" {
  # (1) remote unreachable (network class) — remote dir moved away after gate
  L1="$(new_lane n feat-n)"; add_skill "$L1" zz-b53-n
  before="$(remote_main_sha)"
  cat > "$SANDBOX/yank.sh" <<EOF
#!/usr/bin/env bash
mv "$REMOTE" "$REMOTE.away" 2>/dev/null || true
EOF
  chmod +x "$SANDBOX/yank.sh"
  run env LAND_TEST_AFTER_GATE_CMD="$SANDBOX/yank.sh" bash -c "cd '$L1' && scripts/land.sh"
  st=$status; out1="$output"
  mv "$REMOTE.away" "$REMOTE" 2>/dev/null || true
  [ "$st" -ne 0 ]
  grep -q 'retryable: yes' <<<"$out1"
  [ "$(remote_main_sha)" = "$before" ]
  run land "$L1" --status; [[ "$output" == *unheld* ]]
  # local branch keeps its rebased commits ready for retry
  git -C "$L1" log --format=%s | grep -q 'zz-b53-n'

  # (2) authentication/permission rejected — remote made unwritable
  L2="$(new_lane a2 feat-a2)"; add_skill "$L2" zz-b53-a
  cat > "$SANDBOX/lockdown.sh" <<EOF
#!/usr/bin/env bash
chmod -R a-w "$REMOTE/objects" "$REMOTE/refs" 2>/dev/null || true
EOF
  chmod +x "$SANDBOX/lockdown.sh"
  run env LAND_TEST_AFTER_GATE_CMD="$SANDBOX/lockdown.sh" bash -c "cd '$L2' && scripts/land.sh"
  st=$status; out2="$output"
  chmod -R u+w "$REMOTE/objects" "$REMOTE/refs" 2>/dev/null || true
  [ "$st" -ne 0 ]
  grep -q 'retryable: no' <<<"$out2"
  [ "$(remote_main_sha)" = "$before" ]
  run land "$L2" --status; [[ "$output" == *unheld* ]]

  # (3) remote pre-receive hook rejects
  L3="$(new_lane h feat-h)"; add_skill "$L3" zz-b53-h
  mkdir -p "$REMOTE/hooks"
  cat > "$REMOTE/hooks/pre-receive" <<'EOF'
#!/usr/bin/env bash
echo "policy: rejected by pre-receive" >&2
exit 1
EOF
  chmod +x "$REMOTE/hooks/pre-receive"
  run land "$L3"
  st=$status; out3="$output"
  rm -f "$REMOTE/hooks/pre-receive"
  [ "$st" -ne 0 ]
  grep -q 'retryable: no' <<<"$out3"
  [ "$(remote_main_sha)" = "$before" ]
  run land "$L3" --status; [[ "$output" == *unheld* ]]
  git -C "$L3" log --format=%s | grep -q 'zz-b53-h'
}

@test "B54: a rewound or force-pushed origin/main fails closed" {
  # give main some depth, record the ancestor
  SETUP="$(new_lane setup feat-setup)"; add_skill "$SETUP" zz-b54-depth
  ( cd "$SETUP" && bash scripts/regen-all.sh && git add -A && git commit -qm regen && git push -q origin HEAD:main )
  ancestor="$(git --git-dir="$REMOTE" rev-parse main~1)"

  LANE="$(new_lane a feat-a)"; add_skill "$LANE" zz-b54
  cat > "$SANDBOX/rewind.sh" <<EOF
#!/usr/bin/env bash
[ -f "$SANDBOX/rewound" ] && exit 0
touch "$SANDBOX/rewound"
git --git-dir="$REMOTE" update-ref refs/heads/main "$ancestor"
EOF
  chmod +x "$SANDBOX/rewind.sh"

  run env LAND_TEST_AFTER_GATE_CMD="$SANDBOX/rewind.sh" bash -c \
    "set -x; cd '$LANE' && scripts/land.sh"
  [ "$status" -ne 0 ]
  [[ "$output" =~ origin/main\ moved\ unexpectedly|rewound ]]
  ! grep -E 'push .*(--force|--force-with-lease)' <<<"$output"
  # audit records both the expected and the observed remote SHAs
  audit_entries | grep -q "$ancestor"
  [ "$(remote_main_sha)" = "$ancestor" ]   # land.sh did not "fix" the rewind
}

@test "B55: a land pushes exactly one ref — refs/heads/main, fast-forward by construction" {
  # remote also carries a feature ref and a tag
  SETUP="$(new_lane setup feat-x-remote)"
  add_skill "$SETUP" zz-b55-remote
  git -C "$SETUP" push -q origin feat-x-remote
  git -C "$SETUP" tag -a seed-tag -m seed
  git -C "$SETUP" push -q origin seed-tag
  featx_before="$(remote_ref_sha heads/feat-x-remote)"
  tag_before="$(remote_ref_sha tags/seed-tag)"

  LANE="$(new_lane a feat-x)"
  add_skill "$LANE" zz-b55
  git -C "$LANE" tag local-tag                      # local tag on the lane
  git -C "$LANE" branch -q --set-upstream-to=origin/feat-x-remote feat-x 2>/dev/null || true

  run bash -c "set -x; cd '$LANE' && scripts/land.sh"
  [ "$status" -eq 0 ]
  trace="$output"
  # the push refspec targets ONLY refs/heads/main
  pushlines="$(grep -E '\bgit push\b' <<<"$trace" | grep -v -- '--dry-run' || true)"
  [ -n "$pushlines" ]
  while IFS= read -r line; do
    grep -qE 'refs/heads/main|[[:space:]]main([[:space:]]|$)|HEAD:refs/heads/main|HEAD:main' <<<"$line"
    ! grep -qE -- '--force|--force-with-lease|--tags|refs/notes|feat-x-remote' <<<"$line"
  done <<<"$pushlines"

  # remote tags + feature ref bit-identical to before
  [ "$(remote_ref_sha heads/feat-x-remote)" = "$featx_before" ]
  [ "$(remote_ref_sha tags/seed-tag)" = "$tag_before" ]
  [ "$(remote_ref_sha tags/local-tag)" = "ABSENT" ]
}

@test "B56: repeated out-of-band churn exhausts the bounded retry and aborts cleanly" {
  cat > "$SANDBOX/churn.sh" <<EOF
#!/usr/bin/env bash
set -e
n=\$(cat "$SANDBOX/churn-count" 2>/dev/null || echo 0)
[ "\$n" -ge 2 ] && exit 0
echo \$((n+1)) > "$SANDBOX/churn-count"
d="\$(mktemp -d "$SANDBOX/churn.XXXX")"
git clone -q "$REMOTE" "\$d/c"
cd "\$d/c"
mkdir -p "skills/zz-churn-\$n" && printf -- '---\nname: zz-churn-%s\n---\n' "\$n" > "skills/zz-churn-\$n/SKILL.md"
bash scripts/regen-all.sh
git add -A && git commit -qm "oob churn \$n" && git push -q origin main
EOF
  chmod +x "$SANDBOX/churn.sh"

  LANE="$(new_lane a feat-a)"; add_skill "$LANE" zz-b56
  run env LAND_TEST_AFTER_GATE_CMD="$SANDBOX/churn.sh" LAND_MAX_REBASE_ATTEMPTS=2 \
    bash -c "cd '$LANE' && scripts/land.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"out-of-band churn"* ]]
  attempts="$(sed -n 's/.*rebase attempts: \([0-9][0-9]*\).*/\1/p' <<<"$output" | tail -1)"
  [ -n "$attempts" ] && [ "$attempts" -le 2 ]
  c="$(fresh_clone)"; [ ! -d "$c/skills/zz-b56" ]

  # the branch remains landable: a rerun against a quiet remote lands it
  run land "$LANE"
  [ "$status" -eq 0 ]
  c="$(fresh_clone)"; [ -d "$c/skills/zz-b56" ]

  # if the gate FAILS after absorbing out-of-band changes, the failure is
  # attributed to the absorbed base, not the lane
  cat > "$SANDBOX/poison.sh" <<EOF
#!/usr/bin/env bash
set -e
[ -f "$SANDBOX/poison-done" ] && exit 0
touch "$SANDBOX/poison-done"
d="\$(mktemp -d "$SANDBOX/poison.XXXX")"
git clone -q "$REMOTE" "\$d/c"
cd "\$d/c"
echo "planted drift from absorbed base" >> docs/context-map.md
git commit -qam "oob: poison the base" && git push -q origin main
EOF
  chmod +x "$SANDBOX/poison.sh"
  L2="$(new_lane p feat-p)"; add_skill "$L2" zz-b56-p
  run env LAND_TEST_AFTER_GATE_CMD="$SANDBOX/poison.sh" bash -c "cd '$L2' && scripts/land.sh"
  [ "$status" -ne 0 ]
  grep -qiE 'base|absorbed' <<<"$output"
}
