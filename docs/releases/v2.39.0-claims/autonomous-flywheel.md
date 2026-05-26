# AOP-CLAIM-README-AUTONOMOUS-FLYWHEEL — evidence (v2.39.0)

**Claim location:** README.md sections describing the knowledge
flywheel: σρ > δ (capture × utility > decay), compounding across
sessions.

**Claim summary:** The AgentOps knowledge flywheel has a measured
escape-velocity surface. Captured learnings can be applied across
sessions, and the committed snapshot records whether current corpus
health is above or below the long-run utility/decay threshold.

## Repo surfaces that demonstrate it

- `cli/cmd/ao/flywheel.go`, `cli/internal/flywheel/*` — the σρδ
  computation and `ao flywheel status --json` surface.
- `scripts/check-flywheel-compounding.sh` — live gate that asserts
  `escape_velocity_compounding=true` when strict escape-velocity proof
  is required against the current corpus.
- `scripts/snapshot-flywheel-compounding.sh` — operator command that
  wraps the live status in a corpus-state evidence envelope.
- `docs/releases/flywheel-compounding-snapshot.json` (tracked) —
  the durable snapshot artifact. Historical v2.39 evidence recorded
  σρ=0.0488, δ=2.742, compounding=true on 2026-05-11; current snapshots
  may report lower health and should still be committed when fresh.
- `scripts/check-flywheel-compounding-snapshot.sh` — CI gate that
  validates the tracked snapshot is < 14 days old and contains readable
  `escape_velocity_compounding` health evidence.
- GOALS.md gate `flywheel-compounding-snapshot` (weight 5).
- `validate-flywheel-compounding-snapshot` CI job (validate.yml).

## Verification surface

The CI gate fires on every push. The snapshot must be refreshed at
least every 14 days; the live gate computes σρδ on demand. Companion
bead: G1 (soc-45sg.1) — closed cycle 24.

## Why this is enough

The flywheel claim has two evidence types:
1. **Live evidence** — `ao flywheel status --json` returning current
   σρδ and compounding health.
2. **Durable evidence** — the committed snapshot showing the value
   at a known git SHA so the claim can be audited retrospectively.

Both are wired and gated. The snapshot value can regress if the
corpus stops compounding (e.g., capture without citation), at which
point the next refresh writes `compounding=false` and CI surfaces that
as current health evidence. Strict local enforcement is available with
`AGENTOPS_FLYWHEEL_SNAPSHOT_REQUIRE_COMPOUNDING=1`.

## Anti-claim

Not claiming the flywheel is always above escape velocity, at maximum
velocity, or that all captured learnings are utilized. A fresh snapshot
can honestly report `compounding=false`; that is the action signal to
increase applied/reference citation flow and mine orphaned research.
