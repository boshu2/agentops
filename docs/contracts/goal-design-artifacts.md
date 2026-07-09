# Goal Design Artifacts

Goal-design packets are the schema-backed entry point from human intent into
AgentOps' operating loop. A packet lives under `.agents/goal-design/<slug>/`
as local runtime state and contains exactly two required markdown artifacts:
`intent.md` and `driver.md`.

The tracked contract surfaces are the schemas, templates, checker, docs, and
fixtures in this repository. Generated packets under repo-root `.agents/` stay
untracked runtime output.

## Artifacts

| Artifact | Schema | Purpose |
| --- | --- | --- |
| `.agents/goal-design/<slug>/intent.md` | `schemas/goal-design-intent.v1.schema.json` | Human-shaped source of truth: objective, why, domain terms, BDD behavior, boundaries, evidence, stale inputs, and hard rules. |
| `.agents/goal-design/<slug>/driver.md` | `schemas/goal-design-driver.v1.schema.json` | Loop-driving contract: intent digest, four-loop routing, candidate beads, small-batch gate, route-back rules, execution mode, and validation policy. |

Both files are markdown for human review, with required YAML frontmatter for
machine validation. The body should render the same concepts for operators, but
the frontmatter is the contract the checker validates.

## Writers

- Human operators may draft packets from `docs/templates/goal-design-intent.md`
  and `docs/templates/goal-design-driver.md`.
- The future `goal-design` skill may write `.agents/goal-design/<slug>/`
  because skill-owned `.agents/<skill-name>/` subdirectories are allowed by the
  `.agents` write-surface contract.
- A future CLI may scaffold packets only after the skill earns repeated use; it
  must still write the same two artifacts and run the same checker.

The first slice deliberately adds no CLI, no workflow runner, and no tracked
repo-root `.agents/` packet.

## Readers

- `scripts/check-goal-design-packet.sh` extracts both frontmatter blocks,
  validates them against the versioned schemas, checks digest integrity, and
  verifies cross-file identity.
- `/validate` or an equivalent independent validator reads a checker-clean
  packet before it can file beads, dispatch NTM/Workflow execution, or drive
  implementation work.
- Follow-on planning or skill work may read `driver.md` candidate beads, but a
  candidate list is never a fixed waterfall plan.

## Validation

A packet is valid only when all of these are true:

1. `intent.md` validates against `goal-design-intent.v1`.
2. `driver.md` validates against `goal-design-driver.v1`.
3. `driver.intent_ref.sha256` equals the SHA-256 digest of the current
   `intent.md` bytes in the packet directory.
4. `driver.slug` equals `intent.slug`, and `driver.intent_ref.path` is the
   canonical `.agents/goal-design/<slug>/intent.md` path for that packet.
5. Each `candidate_beads[].behavior` references at least one declared BDD
   scenario by scenario id, such as `S1`, or by the scenario `name`; unknown
   scenario ids are invalid.
6. The driver names independent validation and does not contain self-grading
   language.
7. Candidate beads name one behavior, one bounded context, one write scope, one
   first failing proof, and one close signal.
8. Route-back rules say how validation failures, closed-bead evidence, stale
   candidates, and contradictory promotion signals change the next decision.

Run:

```bash
scripts/check-goal-design-packet.sh .agents/goal-design/<slug>
```

The checker has no authority to mark the packet done by itself. It proves the
contract is structurally usable; an independent `validate` verdict is still the
required close signal before the packet drives work.

## Lifecycle

| State | Meaning | Allowed next move |
| --- | --- | --- |
| `draft` | Human or skill-authored packet exists but has not passed the checker and independent validation. | Patch artifacts and rerun the checker. |
| `validated` | Checker passes and an independent validator has recorded `PASS`. | File the first candidate bead or dispatch one loop tick. |
| `superseded` | Closed-bead evidence or stale inputs made this packet outdated. | Create or revise a replacement packet with a new driver digest. |

Closed bead evidence is the strongest route-back signal. After a bead closes,
revise or reorder remaining candidates from the verdict and evidence rather
than executing the original candidate list unchanged.
