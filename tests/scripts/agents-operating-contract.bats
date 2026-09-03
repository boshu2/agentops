#!/usr/bin/env bats

@test "root contract routes the RPI product boundary" {
  repo_root="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  contract="$repo_root/AGENTS.md"
  [ -f "$contract" ]

  for required in \
    "operations layer for agentic engineering" \
    "federated integration graph" \
    "Standard RPI traversal" \
    "RPI -> Plan -> Implement -> fresh Validate -> repair to convergence -> report" \
    "Persist \`verdict.v2\` only when" \
    "It owns no retry" \
    "fresh independent judgment" \
    "docs/architecture/rpi-traversal.md"; do
    grep -Fq -- "$required" "$contract"
  done

  ! grep -Eq 'AUTO-REDO|HOLD|ANDON|ao land|next-work|Plan-Pawl' "$contract"
}
