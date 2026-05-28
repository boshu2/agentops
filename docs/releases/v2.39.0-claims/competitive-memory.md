# AOP-CLAIM-README-COMPETITIVE-MEMORY — evidence (v2.39.0)

**Claim location:** README.md sections that position AgentOps against
other coding-agent tools (Compound Engineer, Claude-Flow, etc.).

**Claim summary:** AgentOps tracks how its capabilities relate to
competitor surfaces and keeps that comparison fresh.

## Repo surfaces that demonstrate it

- `docs/competitive/` — per-tool comparison docs.
- `scripts/check-competitive-freshness.sh` — gate that fails if any
  comparison file is older than 45 days.
- GOALS.md gate row `competitive-freshness` (weight 3).

## Verification surface

`scripts/check-competitive-freshness.sh` runs locally + in CI; failure
mode is "competitive analysis docs older than 45 days." A maintainer
must refresh the docs to clear the gate.

## Why this is enough

The claim is "we keep this fresh," not "we have the optimal
position." The 45-day window is the operational meaning of "fresh"
and is testable. Position quality is a writer/judgment surface, not a
CI-gated one.
