# ADR-0016: State Tiers — One Authority per Claim; Projections Never Authoritative

- **Status:** Accepted (2026-07-18)
- **Author:** AgentOps maintainers
- **Builds on:** [ADR-0004](ADR-0004-corpus-moat-unproven-position-on-the-system.md) (corpus moat unproven — position on the verification system), [ADR-0011](ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md) (escape-corpus compounding demoted to hypothesis)
- **Origin:** `.agents/brainstorm/2026-06-19-agentops-memory-state-substrate.md` (the governed-lakehouse brainstorm that first stated the one-authority invariant), `.agents/audits/2026-07-18-agents-writer-matrix.md` (the writer-matrix audit that exposed the junk drawer)
- **Tracking:** epic `age-state-tiers-operationalize-5mzlm` (this ADR is `.1`), tracker epic `age-tracker-bd-dolt-return-jyg2g`

## Context

AgentOps accumulated state the way a workshop accumulates benches: every tool
minted its own directory, and by 2026-07-18 the `.agents/` workspace held 114
top-level directories with no declared owner, lifetime, or authority. The
2026-06-19 memory-state-substrate brainstorm had already named the underlying
confusion — the repo was mixing work-graph state, raw operational memory,
proof/judgment records, curated knowledge, and analytics in one undifferentiated
pile — and had stated the governing invariant: *every durable claim has exactly
one authority, and every derived view names the sources used to build it.*

Two 2026-07-18 decisions made the model operational rather than aspirational:
the tracker returns to bd + Dolt native (epic `age-tracker-bd-dolt-return-jyg2g`;
Gas City is bead-native), and Bo fixed the implementation-language division for
everything that ships. ADR-0004 and ADR-0011 supply the posture this ADR
extends: position on the proven verification/control system, keep unproven
knowledge-accrual machinery out of the product, and never let a derived artifact
masquerade as a source of record.

## Decision

### 1. The four state tiers

| Tier | Location | Authority | Lifetime |
|---|---|---|---|
| **Work** | beads (bd + Dolt, local Mac service per the 2026-07-18 tracker decision) | Source of record for what work exists, its status, dependencies, and close reasons | Permanent, versioned by Dolt |
| **Proof** | `.agents/ao/` | Source of record for verdicts, receipts, evidence, and pinned config | Permanent, append-only |
| **Canon** | `docs/` + `docs/adr/` | Source of record for rules, architecture, and decisions that change future behavior | Permanent, edited deliberately |
| **Scratch + projections** | remainder of `.agents/` | No authority — TTL'd work pad plus generated, manifest-stamped projections | Ephemeral; promotion-or-death |

The work tier is **queried, never indexed**: bead questions go through SQL
(Dolt views) and `bv` graph analytics directly against the store. No stored
index of bead data is ever built or committed — see invariant 2 below.

**Target layout.** The scratch/projection tier collapses from 114 ad-hoc
directories to a closed set of exactly three top-level `.agents/` entries:

- `ao/` — the proof tier (permanent; pawl evidence, verdicts, pinned config),
- `scratch/` — all ephemeral work, convention `scratch/WRITER/DATE-SLUG/`, TTL'd wholesale,
- `projections/` — generated artifacts with manifests, deletable at will.

The closed set is enforced by an `ao doctor` detector
(`fm-ws-noncanonical-topdir`, bead `age-state-tiers-operationalize-5mzlm.7`):
any top-level directory outside the set is a finding, with no fourth
"receipts" exception (doctor receipts live under repo-root `.doctor/`, outside
`.agents/` entirely).

### 2. The invariants

1. **One authority per claim.** Every durable claim has exactly one source of
   record. If two surfaces disagree, one of them is by construction a stale
   projection, and the tier table above says which.
2. **A queryable SOR needs no stored index.** Dolt gives the bead store a full
   SQL engine and `bv` gives it graph analytics; building and storing an index
   over it (a JSONL digest, a wiki index, a cached matrix) creates a second
   surface that can drift. Stored indexes are caches for demonstrably slow
   queries only — never authorities, never committed as truth.
3. **SQL views cannot go stale; filesystem-input snapshots can.** A Dolt view
   is re-evaluated against live bead data on every query, so it needs no
   freshness apparatus. Any projection whose inputs are *files* (source trees,
   skill catalogs, audit scans) is a snapshot and MUST carry a manifest
   (`generated_by`, `inputs`, `generated_at`) so a reader can detect staleness
   mechanically.
4. **Promotion-or-death, via quarantine-rename — never delete.** Scratch
   content either promotes to a tier with authority or expires. Expiry is a
   quarantine-rename with a receipt, with actual disposal a separate operator
   decision — never a direct delete at TTL. The safety rationale is corrected
   from earlier drafts: `.agents/` is gitignored (only `ao/config.yaml` is
   tracked) and cass indexes session transcripts, not script-generated
   artifacts, so a generated file in scratch can be the *only copy in
   existence*. A TTL that deletes would destroy unrecoverable state.
5. **The three-question promotion rule.** At TTL triage, ask in order:
   1. *Is it cited by a verdict or bead close?* Then it is evidence — it
      belongs in `ao/` or the close reason. Citation IS promotion, automatic.
   2. *Would it change what a future agent does* (a rule, gotcha, constraint,
      decision)? Promote to docs/ADR if repo-wide, to the bead description or
      a comment if scoped to one work item.
   3. *No identifiable consumer?* That is hoarding. Let the TTL expire it to
      quarantine. Might-be-useful-someday is not a consumer.

### 3. The division rule (implementation language, fixed 2026-07-18)

- **Mechanism, trust, and receipts ship in the `ao` Go binary.** Anything that
  enforces, verifies, mutates user files, or writes receipts is Go. A single
  static binary IS the distribution story.
- **Know-how ships as skills with references.** Recipes, judgment guidance,
  and reusable method live in `skills/**/SKILL.md` plus reference files.
- **Glue is POSIX `sh` only**, thin argument-plumbing over declared tools.
- **Python never ships in skills.** Interpreter dependencies break determinism
  on user machines; a skill that needs logic beyond `sh` glue routes that logic
  into an `ao` subcommand instead.
- **Prototype anywhere, ship in Go.** The scratch tier accepts any language —
  prototypes die by TTL, so they carry no maintenance debt. What survives the
  promotion questions is rewritten into Go before it ships.
- **Promotion into `ao` has a usage bar.** A new subcommand requires either a
  gate that needs it or demonstrated repeated cross-session use, because every
  subcommand is permanent maintenance surface. The cautionary case is the
  memory-moat machinery: roughly 4,400 lines of Go accreted around an unproven
  claim and had to be removed wholesale (`age-7grl`, per ADR-0004's honesty
  posture). Worked example of the intended flow: the writer-matrix audit was
  prototyped as scratch Python, proved its value, and ships as an `ao doctor`
  detector in Go (bead `age-state-tiers-operationalize-5mzlm.2`).

## Consequences

- The 114-directory `.agents/` junk drawer is migrated once to the three-dir
  layout (bead `.6`) and the closed set is enforced forever after (bead `.7`);
  writers that mint non-canonical directories are source bugs, not conventions.
- No tool may build a stored index over bead data; bead reporting goes through
  Dolt SQL views and `bv` (the beads-views skill,
  `age-tracker-bd-dolt-return-jyg2g.8`). Filesystem-input projections without
  manifests are findings.
- TTL enforcement anywhere in the system quarantine-renames with a receipt;
  a deleting TTL is a defect against this ADR.
- Shipping Python inside a skill is a gate failure, not a style nit; the
  prototype path through scratch exists precisely so that rule has no cost.
- [docs/agents-dir-hygiene.md](../agents-dir-hygiene.md) remains the operating
  manual for the scratch tier (TTL mechanics, drift aliases, doctor detectors)
  and will be rewritten around this tier table (bead `.5`); where the two
  disagree, this ADR wins.
