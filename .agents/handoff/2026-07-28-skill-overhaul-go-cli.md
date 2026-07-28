# Handoff: AgentOps skill-system and Go CLI analysis

Date: 2026-07-28
Repository: `/Users/bo/dev/agentops-worktrees/skill-overhaul`

## Caller-supplied goal

Analyze every one of the 49 canonical AgentOps skills individually, establish
each skill's intent and actual behavior from live owners and executable
surfaces, explain how it fits the one-shot RPI loop, and identify concrete
improvements. Perform the same depth of analysis for the Go CLI. The caller
requested Opus 5 workers through NTM, fresh Sol validation, Fable validation,
and a final Sol/Fable `dueling-idea-wizards` plan.

The caller subsequently withdrew orchestration authority and requested that
the current repository state be committed and pushed with this factual
handoff.

## Repository identity before this handoff commit

| Fact | Value |
|---|---|
| Branch | `codex/skill-overhaul-20260724` |
| HEAD | `0088c6e3824da201eabb1e751ac8e976599e0b5c` |
| Tree | `c0c43eefb8042af5a6a7877c0f7f0de80149ffc6` |
| `origin/main` | `f66ab953af332428277a64ddaa9c878f5347572e` |
| `origin/main...HEAD` | `0 behind / 87 ahead` |
| Main ancestry | `origin/main` is an ancestor of HEAD |
| Status | clean before adding this handoff |

`git fetch origin main` was repeated immediately before writing this artifact.
No upstream content was pending.

## Orchestration end state

- Codex subagents `sol_review_adapters13_v14`,
  `sol_review_outcomes12_v13`, and `sol_review_t3_design_v7` were interrupted.
- NTM session `agentops--opus5-verified` was captured with five panes, all idle
  by the final tail capture, and then killed.
- NTM session
  `agentops--skills-fable-execution-audit-20260727` was captured with its one
  Fable pane idle and then killed.
- No NTM session or subagent remains authorized by this handoff.
- No worker edited the designated landing worktree during the final audit and
  design passes.

## Accepted evidence

### Core-12 skill audit

Subject:
`/tmp/agentops-opus5-verified-skill-audit-core-12-v8.md`

- SHA-256:
  `3f9d8051d691fb40885ca45586d6e6bfb03b4ae147545767185b6c14b423b392`
- extent: 811 lines / 105,572 bytes

Fresh Sol review:
`/tmp/agentops-opus5-verified-skill-audit-core-12-v8-review-sol.md`

- SHA-256:
  `f96706c61f80461436c8985823788b03c79de5edd1f9cf39e7356fa99029b6d9`
- disposition: `PASS`

### Workflow-12 skill audit

Subject:
`/tmp/agentops-opus5-verified-skill-audit-workflow-12-v8.md`

- SHA-256:
  `b9d6b3d913990509319d86260ce7dd918490d4ecf94ad9b38012208414da357a`
- extent: 795 lines / 97,069 bytes

Fresh Sol review:
`/tmp/agentops-opus5-verified-skill-audit-workflow-12-v8-review-sol.md`

- SHA-256:
  `325e1dc99ff0db56fecffaefacc690e457d8699d5f6617727bbaf500c474558c`
- disposition: `PASS`

### Program-level Fable validation

Artifact:
`/tmp/agentops-fable-current-program-validation-v1.md`

- SHA-256:
  `ce009f917ba3902d8b4a9bbe9fdef5e5a2a2b121aed963857a6e76a72a526803`
- extent: 117 lines / 15,675 bytes
- advisory conclusion: proceed on the execution track and freeze substantive
  audit churn; allow only exact closure repairs

### Accepted prerequisite candidates

| Candidate | Commit | Tree | Review |
|---|---|---|---|
| Active-proof gate repair | `5ad8f804dbbd133d98d2b34f27bbea5f7f14a2d3` | `9ad4a14ef194f7ee4e1b3f34b344f870e581b256` | `/tmp/agentops-active-proof-gate-repair3-review-sol.md`, SHA-256 `8b10ab5a232300e636096bada88221a13853621784844ad4678f057a5575307d`, `PASS` |
| T2 D0 decision repair | `d9c75bd2994cdf469d364810a5f4fc33ee070bea` | `992a72affa8e65ff9cb29a63c66840872db9a122` | `/tmp/agentops-t2-d0-decision-repair5-review-sol.md`, SHA-256 `5a475a708b1d70ba3065fdc823ea4f115a77502b239123027fdf56213aa09ef5`, `PASS / APPROVE` |

Neither prerequisite commit is an ancestor of the landing HEAD recorded above.
Neither was merged during the final orchestration pass.

## Existing but not accepted

### Core RPI placement addendum

Latest subject:
`/tmp/agentops-core-12-rpi-placement-addendum-v4.md`

- SHA-256:
  `a16319986a19fe493d79e4fc84f22d83afb7b9844fb3ed92c4d7adb17b5b45db`
- extent: 806 lines / 68,996 bytes
- status: Fable-authored disclosure repair; no independent review binds v4

The v3 fresh Sol review passed the twelve rows and substantive B1-B4 repairs
but rejected three disclosure statements. V4 was intended to repair only those
statements.

### Outcomes-12 skill audit

Subject:
`/tmp/agentops-opus5-verified-skill-audit-outcomes-12-v13.md`

- SHA-256:
  `3eacc27504c36c23346f37372be59ad6a40ddeafd37376568d4cc1476c5bfe24`
- extent: 1,349 lines / 205,016 bytes

Fresh Sol review:
`/tmp/agentops-opus5-verified-skill-audit-outcomes-12-v13-review-sol.md`

- SHA-256:
  `81810c98b9286740d33cb13dd9eb354f397e86a1fbda572deae41e0446b33810`
- disposition: `REQUEST_CHANGES`
- review says the twelve-skill technical audit and exact manifest remain
  credible
- unresolved report findings: inconsistent provenance counts, two stale
  deictic labels, an 8-versus-9 section count, a false `craft-goal` strength
  and validator length, and an inaccurate description of the `doc` validator

### Adapters-13 skill audit

Subject:
`/tmp/agentops-opus5-verified-skill-audit-adapters-13-v14.md`

- SHA-256:
  `79bbd141a27429e5d12434a8eacbd32a53476260f7273bb395a744c1265a0885`
- extent: 1,183 lines / 142,998 bytes

Fresh Sol review:
`/tmp/agentops-opus5-verified-skill-audit-adapters-13-v14-review-sol.md`

- SHA-256:
  `d8e33740ce4ebb9cba7a78aa42af7ecf453fb3f9faf382fe79054dd9d7a9ea2f`
- disposition: `REQUEST_CHANGES`
- review reproduced the technical analysis and accepted the prior RC-3 footer
  repair
- sole unresolved finding: v14 falsely says it rewrote one paragraph although
  the editorial diff also changed metadata, history framing, and the footer;
  technical sections 2-11 were byte-identical to v13

### Go CLI audit

Latest completed subject:
`/tmp/agentops-opus5-verified-go-cli-deep-reaudit-v13.md`

- SHA-256:
  `9b909e400362c3153d068e2237c597d69eb9afb1db31d27b52723302079b51b5`
- extent: 1,256 lines / 134,327 bytes

Independent seal preflight:
`/tmp/agentops-opus5-verified-go-cli-deep-reaudit-v13-preflight.md`

- SHA-256:
  `f91ba9df148e8a4ac20ae8f855bd9a6fee1c4c67a55fbcc1880c8293b95bc681`
- disposition: `REPAIR_BEFORE_SOL`
- preflight found two blocking self-account defects and three minor
  provenance/cross-reference defects
- preflight found the technical regions byte-identical to the Sol-reviewed v10
  regions and reproduced the eight findings, six T8 conditions, 71 packages,
  18 command families, 548 Go files, 121,899 lines, and the reported green
  battery

The requested v14 repair file does not exist. The Opus pane was stopped before
publishing it.

### S0/H0 implementation plan

Latest subject:
`/tmp/agentops-opus5-s0-h0-implementation-plan-v13.md`

- SHA-256:
  `85c80c1f24ae58507eadf7dbb119136010e7e2f16673973cffe8e35e363d9612`
- extent: 3,940 lines / 253,098 bytes
- status: authored after v12 was rejected; no independent review binds v13

The v12 review found undefined intent identities, a circular
gate/barrier/final-manifest dependency, stale gate requirements for merge-only
recipes, and incorrect stable anchors.

### T3 direct-to-Go design

Latest subject:
`/tmp/agentops-opus5-t3-go-port-design-v7.md`

- SHA-256:
  `d44983b6ff48d418d7941e917c88ced2ff94fb91a8e1f5b6daab0d3e7aee11b1`
- extent: 1,751 lines / 124,809 bytes
- status: no completed fresh Sol review binds v7

V7 added the fifteen stable T3 acceptance IDs, an aggregate packet, concrete
packet mechanics, all thirteen runnable leaves, and serialized slice barriers.
Its tail contains contradictory v6/v7 provenance language that had been placed
in the interrupted validator's review scope.

### T5 direct-to-Go design

Latest subject:
`/tmp/agentops-opus5-t5-go-port-design-v7.md`

- SHA-256:
  `d3c34ae2e4d39f661abbdec5ba7f8b45e7e1d737b0f21a49bac875be7016a7d3`
- extent: 1,677 lines / 106,161 bytes
- status: authored; no independent review binds v7

V7 was intended to replace literal artifact-digest placeholders with a
fail-closed generator, pin probe identity, and enforce intent-first barriers.

### T4 non-circular design

Latest completed subject:
`/tmp/agentops-opus5-t4-noncircular-repair-design-v5.md`

- SHA-256:
  `77b1d5330b9dd731d99a8f8db08d292c09353aa23d05522f0c500d5964bd678f`
- extent: 1,825 lines / 119,500 bytes

Fresh Sol review:
`/tmp/agentops-opus5-t4-noncircular-repair-design-v5-review-sol.md`

- SHA-256:
  `472a79b1bffda5c1791e29e30780c3a3c6d4bf3be20c5e5fcfe3dfb2572c6c73`
- disposition: `REQUEST_CHANGES`
- review demonstrated that the published generator could store `PASS` while a
  cited typed receipt was `FAIL`
- other blockers: impossible chronology, fail-open implementation identity,
  abridged criteria, and semantic receipt IDs without executable producers

The requested v6 repair file does not exist. The Opus pane was stopped before
publishing it. No tracked T4 write was authorized.

## Synthesis state

Readiness artifact:
`/tmp/agentops-49-skill-synthesis-readiness-preflight-v2.md`

- SHA-256:
  `0cc284988a490f5cc544318379e964b7533141f28e9423843f7ccef6f77944e7`
- proves the intended 12 + 12 + 12 + 13 partition equals 49 unique canonical
  skills

The prepared synthesis prompt at
`/tmp/agentops-49-skill-synthesis-v4-prompt.md` is stale: it names superseded
core-placement, outcomes, and adapters inputs. No final 49-row synthesis file
was produced.

The four audit groups use incompatible native severity ledgers. Their numbers
must not be added into a synthetic corpus-wide severity total.

## Missing completion evidence

The following evidence does not exist:

- a final accepted 49-skill synthesis;
- a final accepted Go CLI audit successor to v13;
- a final dependency-ordered Go implementation roadmap;
- a reviewed S0/H0 v13 design;
- a reviewed T3 v7 design;
- a reviewed T5 v7 design;
- a safe T4 v6 design;
- explicit authority for a tracked T4 write;
- the requested final Sol/Fable plan duel over the final plan;
- implementation commits derived from the unfinished S0/H0, T3, T4, or T5
  designs;
- a fresh exact-subject verdict over a combined integrated tree;
- a push of `codex/skill-overhaul-20260724` before this handoff commit.

No item in this section should be inferred as complete from a prompt,
in-progress pane, or predecessor review.

## Scratch-artifact boundary

The `/tmp` reports listed above are local advisory evidence and are not copied
into Git by this handoff. Their exact paths, digests, and extents are recorded
so their existence and identity can be checked on this host. The repository
contains the implementation/history already present in its 87 commits plus
this compact factual handoff; it does not contain the 474 temporary audit,
prompt, and review files accumulated under `/tmp`.

No continuation, work assignment, verdict, merge decision, or release decision
is selected by this handoff.
