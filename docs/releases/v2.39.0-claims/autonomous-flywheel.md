# AOP-CLAIM-README-AUTONOMOUS-FLYWHEEL — evidence (v2.39.0)

**Claim location:** README.md sections describing the knowledge
flywheel: σρ > δ (capture × utility > decay), compounding across
sessions.

**Claim summary:** The AgentOps knowledge flywheel is at escape
velocity. Captured learnings are applied across sessions, raising
the long-run utility delta over the natural decay rate.

## Repo surfaces that demonstrate it

- `cli/cmd/ao/flywheel.go`, `cli/internal/flywheel/*` — the σρδ
  computation and `ao flywheel status --json` surface.
- `scripts/check-flywheel-compounding.sh` — live gate that asserts
  `escape_velocity_compounding=true` against the current corpus.
- `scripts/snapshot-flywheel-compounding.sh` — operator command that
  wraps the live status in a corpus-state evidence envelope.
- `docs/releases/flywheel-compounding-snapshot.json` (tracked) —
  the durable snapshot artifact (HEAD: σρ=0.0488, δ=2.742,
  compounding=true, recorded 2026-05-11).
- `scripts/check-flywheel-compounding-snapshot.sh` — CI gate that
  validates the tracked snapshot is < 14 days old and shows
  `escape_velocity_compounding=true`.
- GOALS.md gate `flywheel-compounding-snapshot` (weight 5).
- `validate-flywheel-compounding-snapshot` CI job (validate.yml).

## Verification surface

The CI gate fires on every push. The snapshot must be refreshed at
least every 14 days; the live gate computes σρδ on demand. Companion
bead: G1 (soc-45sg.1) — closed cycle 24.

## Why this is enough

The flywheel claim has two evidence types:
1. **Live evidence** — `ao flywheel status --json` returning
   compounding=true at any given moment.
2. **Durable evidence** — the committed snapshot showing the value
   at a known git SHA so the claim can be audited retrospectively.

Both are wired and gated. The snapshot value can regress if the
corpus stops compounding (e.g., capture without citation), at which
point the next refresh would write `compounding=false` and CI would
fail.

## Anti-claim

Not claiming the flywheel is at maximum velocity, or that all
captured learnings are utilized. The current state shows σρ=0.0488
above δ/100 = 0.0274 — comfortably above escape velocity but not
saturating.
