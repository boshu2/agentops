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
    "A caller or explicitly selected bounded outer goal may authorize a new" \
    "authority stay upstream" \
    "breaker enters causal HOLD" \
    "Exactly one bounded fresh helper per HOLD" \
    "incident fits inside the existing allowance" \
    "genuinely spent hard time/cost/quota skip the helper" \
    "never renew a goal allowance" \
    "docs/architecture/rpi-traversal.md"; do
    grep -Fq -- "$required" "$contract"
  done

  # A bounded outer goal owns HOLD and allowance; the RPI product boundary
  # above must still deny retry/budget ownership and retired delivery machinery.
  ! grep -Eq 'ao land|next-work|Plan-Pawl' "$contract"
}
