#!/usr/bin/env bats

@test "root contract routes the single-pass product boundary" {
  repo_root="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  contract="$repo_root/AGENTS.md"
  [ -f "$contract" ]

  for required in \
    "RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop" \
    "It owns no retry" \
    "fresh independent judgment" \
    "docs/architecture/operating-loop.md"; do
    grep -Fq -- "$required" "$contract"
  done

  ! grep -Eq 'AUTO-REDO|HOLD|ANDON|ao land|next-work|Plan-Pawl' "$contract"
}
