---
date: 2026-05-30
kind: agent-behavior
status: reviewed
scope: cross-session
source_bead: ag-1aav
repeats: true
related: [verify-each-premise-before-executing, external-validation-beats-self-report]
---
# Claim a PR is green only AFTER running the gates — never infer it

## What happened

Shipped PR #632 (a docs-only domain-language entry) and told the operator it
"should sail," then "back to your beads," then ended the session. The PR had
**4 red checks**. Two were mine and trivially preventable:

- `markdownlint MD049` — used `_underscore_` emphasis where the repo enforces `*asterisk*`.
- stale `registry.json` — adding a `references/*.md` bumps the reference count;
  `scripts/generate-registry.sh` must be re-run (this also failed bats #769, the
  registry-drift gate).

(The other two: a bats failure that was the *same* stale-registry cascade, and the
required `claude-review` gate hitting Anthropic's weekly rate limit — infra, not mine.)

## The pattern (why this is a REPEAT, not a one-off)

This is the **self-report-over-verification** failure mode, and it fired **twice in
one session** — also when I amplified a sub-agent's "zero adversarial evidence" claim
without grounding it in the operator's real, adopted system. It echoes existing
memories `verify-each-premise-before-executing` and `external-validation-beats-self-report`.
Per the promotion ratchet: noticed-twice → durable behavior constraint.

## The behavior change (do this)

Before telling the user a PR is green / done / mergeable:

1. **Run the surface's local mechanical gates.** For a `skills/` change:
   `markdownlint` (or the repo's lint) **and** `bash scripts/generate-registry.sh --check`.
   For Go: `cd cli && make test`. Never skip because "it's only docs / small."
2. **Read `gh pr checks <n>`** (or `gh pr view --json statusCheckRollup`) and report the
   *actual* state — do not assert green from the diff.
3. Treat "it's trivial, it'll pass" as the tell that I'm about to skip verification.

## Mechanical hook (CLI-for-deterministic)

This is exactly a "deterministic + repeated → CLI/CI" candidate (see the
`primitive-selection` domain entry): the right long-term fix is a pre-push/CI check
that runs `generate-registry.sh --check` + lint on `skills/**` changes so a human
claim is never the gate. Until then it is an enforced agent habit.
