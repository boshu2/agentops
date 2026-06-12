# ag-s43tg — Pinned Trigger-Phrase Manifest (skill-prune phase 2, wave 0)

**Status:** FROZEN pre-wave (slice S0, 2026-06-11). Wave-1 fold workers are graded
against THESE pinned fixtures — not against phrases chosen post-hoc.

**What this is:** one trigger-phrase fixture per existing fold-source skill
(39 sources; the work-list's `validation` and `security-suite` are phantoms — no
`skills/<name>/` dir exists — and are excluded). Each fixture was extracted from the
source's frontmatter `description` + `Triggers` list at freeze time (most sources
carry an *empty* `Triggers:` stub, so the fixture falls back to the description
sentence, per the S0 contract). Companion work-list: `docs/plans/skill-prune-phase2.md`.

**Grading rule (the contract):** for every `PIN:` line below,

```sh
grep -Fqi -- "<phrase>" skills/<target>/SKILL.md
```

- **RED (now, pre-augment):** the grep FAILS for all 39 lines — verified at freeze.
- **GREEN (wave-1 exit, per target):** the grep SUCCEEDS — the worker grafted the
  source's trigger surface into the target.

This is the executable proxy behind acceptance Row 1 ("no user-visible capability
vanishes" = every pinned phrase remains grep-discoverable in its fold target),
supplemented by S24's routing spot-check.

**Machine-readable line format:** `PIN: <source> -> <target> :: <phrase>`
(parse with `grep '^PIN: '`; phrase is everything after ` :: `, matched
case-insensitively as a fixed string). Pinned phrases avoid short tokens — e.g.
`UBS` alone false-positives inside "st**ubs**" under `-Fi`.

---

## validate

### vibe → validate
Basis: description (Triggers empty).
PIN: vibe -> validate :: quick readiness or sanity check

### bead-completion-audit → validate
Basis: description (Triggers stub empty).
PIN: bead-completion-audit -> validate :: auditing closed beads for real shipped evidence

## review

### bug-hunt → review
Basis: description (Triggers empty).
PIN: bug-hunt -> review :: bugs and root causes

### codebase-audit → review
Basis: description (Triggers empty).
PIN: codebase-audit -> review :: Domain-parameterized codebase audits

### ubs → review
Basis: description (Triggers empty). Note: bare `UBS` rejected (matches "stubs" under `-Fi`).
PIN: ubs -> review :: reviewing code with UBS

## refactor

### complexity → refactor
Basis: description (Triggers empty).
PIN: complexity -> refactor :: focused refactor hotspots

## security

### deps → security
Basis: description (Triggers empty).
PIN: deps -> security :: dependency risks and updates

## council

### multi-model-triangulation → council
Basis: description (Triggers empty).
PIN: multi-model-triangulation -> council :: get a second opinion

### cross-vendor-trust-gate → council
Basis: description (Triggers empty).
PIN: cross-vendor-trust-gate -> council :: skill-factory final trust gate

## eval-outcomes

### scenario → eval-outcomes
Basis: description (Triggers empty).
PIN: scenario -> eval-outcomes :: Manage holdout scenarios

## discovery

### brainstorm → discovery
Basis: description (Triggers empty).
PIN: brainstorm -> discovery :: Separate goals from implementation

### design → discovery
Basis: description (Triggers empty).
PIN: design -> discovery :: pressure-testing user value

## plan

### planning-workflow → plan
Basis: description (Triggers empty).
PIN: planning-workflow -> plan :: markdown planning methodology

## rpi

### operating-loop-skill → rpi
Basis: description (Triggers stub empty).
PIN: operating-loop-skill -> rpi :: independent validation, closeout, and persistence

### operating-loop-workflow → rpi
Basis: description (Triggers empty).
PIN: operating-loop-workflow -> rpi :: seven-move operating-loop Workflow

## crank

### burndown → crank
Basis: description (Triggers empty).
PIN: burndown -> crank :: finite epic set to all-merged

### ship-loop → crank
Basis: description (Triggers empty).
PIN: ship-loop -> crank :: fast-lane internal ship cycle

## cass

### casr → cass
Basis: description (Triggers empty).
PIN: casr -> cass :: Resume sessions across Claude Code, Codex, Gemini

### cass-memory → cass
Basis: description (Triggers empty).
PIN: cass-memory -> cass :: cm procedural memory

## inject

### session-bootstrap → inject
Basis: description (Triggers empty).
PIN: session-bootstrap -> inject :: Universal AgentOps init prompt

### using-agentops → inject
Basis: description (Triggers empty). Disposition: DECLARED REMOVED (folded into inject);
the embedded copy `cli/embedded/skills/using-agentops` is deleted in S24 (BC5 carve-out
recorded in the plan doc).
PIN: using-agentops -> inject :: Explain AgentOps workflows

## status

### quickstart → status
Basis: description (Triggers empty).
PIN: quickstart -> status :: AgentOps next action

## recover

### trace → recover
Basis: description (Triggers empty).
PIN: trace -> recover :: Trace decisions through artifacts

## flywheel

### ratchet → flywheel
Basis: description (Triggers empty).
PIN: ratchet -> flywheel :: Brownian Ratchet gates

## agy-native

### agy-mcp-plugins → agy-native
Basis: description (Triggers empty).
PIN: agy-mcp-plugins -> agy-native :: MCP servers and AgentOps plugin bundles

### agy-project-worktree-permissions → agy-native
Basis: description (Triggers empty).
PIN: agy-project-worktree-permissions -> agy-native :: scoped --add-dir permissions

### agy-rules-workflows → agy-native
Basis: Triggers list ("AGY rules, agy-loop, AGY schedule").
PIN: agy-rules-workflows -> agy-native :: agy-loop

### agy-sidecar-scheduled-tick → agy-native
Basis: description (Triggers "agy, sidecar, schedule, agentapi" — all too short/ambient; description sentence pinned instead).
PIN: agy-sidecar-scheduled-tick -> agy-native :: recurring AGY sidecar loop tick

## cc-hooks

### cc-cron-ticks → cc-hooks
Basis: description (Triggers stub empty).
PIN: cc-cron-ticks -> cc-hooks :: Claude Code cron routines

### cc-loop-driver → cc-hooks
Basis: description (Triggers stub empty).
PIN: cc-loop-driver -> cc-hooks :: control-plane tick loop

### cc-subagents → cc-hooks
Basis: description (Triggers stub empty).
PIN: cc-subagents -> cc-hooks :: scoped Claude Code subagents

### cc-worktree-isolation → cc-hooks
Basis: description (Triggers stub empty).
PIN: cc-worktree-isolation -> cc-hooks :: separate git worktrees to prevent file collisions

## codex-exec

### codex-goals → codex-exec
Basis: description (Triggers stub empty).
PIN: codex-goals -> codex-exec :: define an objective once and let Codex iterate

### codex-mcp-plugins → codex-exec
Basis: description (Triggers stub empty).
PIN: codex-mcp-plugins -> codex-exec :: wiring MCP servers or plugins into Codex CLI

### codex-sandbox-evidence → codex-exec
Basis: description (Triggers stub empty).
PIN: codex-sandbox-evidence -> codex-exec :: least-privilege sandbox with machine-checkable proof

## ntm

### ntm-browser-test-coordination → ntm
Basis: description (Triggers stub empty).
PIN: ntm-browser-test-coordination -> ntm :: browser or UI tests through NTM panes

### ntm-review-worker-orchestration → ntm
Basis: description (Triggers stub empty).
PIN: ntm-review-worker-orchestration -> ntm :: analysis worker with bounded inputs

## pr-prep

### pr-research → pr-prep
Basis: description (Triggers empty).
PIN: pr-research -> pr-prep :: Research an upstream repo

## implement

### pr-implement → implement
Basis: description (Triggers empty).
PIN: pr-implement -> implement :: scoped OSS PR

---

*Cut (no fixture — no fold target):* `reverse-engineer-rpi` is removed outright in
the final pass; its capability is intentionally dropped, so nothing is pinned.

*Freeze verification (S0):* all 39 PIN phrases grepped ABSENT in their unaugmented
targets on 2026-06-11; `test_trigger_manifest_pinned_and_red` green at freeze.
