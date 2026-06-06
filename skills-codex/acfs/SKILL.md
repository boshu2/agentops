---
name: acfs
description: |
  Operate the ACFS flywheel — health-check, init, and run the agent-flywheel
  substrate via ~/acfs/bin/acfs. The operator's front door to the
  Plan→Coordinate→Execute→Scan→Remember loop and its tool stack
  (br/ntm/dcg/cass/cm/caam/ubs).

  Triggers: "acfs doctor", "is the flywheel up", "init the flywheel", "acfs up",
  "run the loop", "operate the substrate", "what's broken in the stack",
  "ACFS health", "spin up a swarm on this repo", "where do I work / work-root".

  Perfect for:
  - Verifying the substrate before a session; deciding installed-vs-actually-working
  - Wiring a fresh fleet host; driving the operating loop

  Not ideal for:
  - Client-facing AI Partner work (this drives operator binaries — keep it backstage)
  - Building the binaries themselves (fork-and-own, invoke-never-rebuild)
  - Deep host topology (that's bushido.md / fleet-ops)
---

# acfs (Codex)

Codex-native parity wrapper. The full skill content — overview, constraints, the
three-phase workflow, output spec, quality rubric, examples, and troubleshooting —
lives in the sibling base file `../SKILL.md`. Read it first.

Codex execution steps and guardrails for this skill are in `prompt.md` (same dir).
