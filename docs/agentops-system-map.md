# AgentOps — System Map

```
┌──────────────────────────────────────────────────────────────────┐
│                    AgentOps at a Glance                          │
├───────────────────┬──────────────────────┬───────────────────────┤
│    52  Skills     │  121 CLI Commands    │   13 Hook Entries     │
│  (workflows)      │  (ao binary)         │  (auto-enforcement)   │
└───────────────────┴──────────────────────┴───────────────────────┘
```

---

## The Pipeline — Skills Calling Skills

The top-level skill `/rpi` chains the full pipeline. Each node is a skill. Arrows show calls.

```
                         ┌─────────────┐
                         │   /evolve   │  ← loops /rpi overnight
                         └──────┬──────┘    fitness-gated
                                │ calls
                                ▼
┌───────────────────────────────────────────────────────────────────┐
│                             /rpi                                  │
│                    (full pipeline orchestrator)                   │
└──┬──────────┬───────────┬─────────────┬──────────┬────────────────┘
   │          │           │             │          │
   ▼          ▼           ▼             ▼          ▼
/research   /plan    /pre-mortem     /crank    /post-mortem
   │          │           │             │          │
   │          │      calls /council     │          ├── calls /council
   │          │                         │          └── calls /retro
   │          │                         │
   │          │              ┌──────────┴──────────┐
   │          │              │        /crank       │
   │          │              │  (wave executor)    │
   │          │              └──────────┬──────────┘
   │          │                         │ spawns N parallel
   │          │                         ▼
   │          │                   /implement
   │          │                   /implement    ← one per issue
   │          │                   /implement
   │          │                         │
   │          │                         ▼
   │          │                      /vibe  ←── calls /council
   │          │                             ←── calls /complexity
   │          │                             ←── calls /bug-hunt
   │          │
   └──────────┴──────────────────────────────────────────────────────
```

---

## Judgment Layer — Everything Flows Through Council

`/council` is the core validation primitive. Three skills wrap it:

```
                   ┌──────────────────────────────┐
                   │           /council           │
                   │  (independent judges debate, │
                   │   verdict gates delivery)    │
                   └───────────┬──────────────────┘
                               │ used by
          ┌────────────────────┼────────────────────┐
          ▼                    ▼                    ▼
   /pre-mortem              /vibe              /post-mortem
   (validate plans          (validate code     (wrap-up +
    before building)         before shipping)   learnings)
```

---

## Knowledge Layer — Skills Calling the CLI

Skills hand off to `ao` to persist knowledge across sessions:

```
   SKILL                   ao CLI COMMAND              RESULT
   ─────                   ──────────────              ──────
/research          →    ao lookup                  Prior knowledge loaded into session
/retro             →    ao forge transcript        Learnings extracted from session
/retro             →    ao pool promote            Validated learnings promoted
/evolve            →    ao goals measure           Fitness checked before next cycle
/rpi               →    ao ratchet record          Progress gate checkpointed
/implement         →    ao ratchet check           Gate verified before work starts
/post-mortem       →    finding-compiler.sh        Findings become artifacts, checks, and constraints
/post-mortem       →    ao flywheel close-loop     Citation feedback and lifecycle updates applied
```

---

## Prevention Ratchet

The closed-loop prevention path is file-native:

```
/post-mortem or /pre-mortem
        │
        ▼
.agents/findings/registry.jsonl
        │
        ▼
hooks/finding-compiler.sh
        │
        ├──> .agents/findings/<id>.md
        ├──> .agents/planning-rules/<id>.md
        ├──> .agents/pre-mortem-checks/<id>.md
        └──> .agents/constraints/index.json   (mechanical + active only)
                                              │
                                              ▼
                                   hooks/task-validation-gate.sh
```

`/plan`, `/pre-mortem`, `/vibe`, and `/post-mortem` load compiled planning and review artifacts first, then fall back to the registry when compiled outputs are missing. `task-validation-gate.sh` is the shift-left enforcement surface for active mechanical findings.

---

## CLI Command Groups (38 commands, 121 including subcommands)

```
KNOWLEDGE FLYWHEEL          VALIDATION GATES         SESSION / LIFECYCLE
──────────────────          ────────────────         ───────────────────
ao forge                    ao gate pending          ao session close
ao pool ingest              ao gate approve          ao rpi status
ao pool promote             ao gate reject           ao rpi cancel
ao lookup                   ao ratchet status        ao hooks list
ao lookup                   ao ratchet record        ao config
ao search                   ao ratchet check
ao dedup                    ao ratchet promote       METRICS / HEALTH
ao curate                                            ────────────────
                            GOALS / FITNESS          ao metrics health
MEMORY TOOLS                ───────────────          ao metrics flywheel
────────────                ao goals measure         ao metrics report
ao mind                     ao goals steer           ao flywheel status
ao notebook                 ao goals add             ao maturity
ao memory                   ao goals prune           ao doctor
ao trace                    ao goals history
ao extract                  ao goals drift           UTILITIES
                                                     ─────────
                                                     ao search
                                                     ao constraint
                                                     ao badge
                                                     ao version
```

---

## Hooks — Automatic Enforcement (13 hook entries across 7 trigger points)

Hooks fire without human involvement. The AI cannot bypass them.

```
TRIGGER                   HOOK                        WHAT IT DOES
───────                   ────                        ────────────
Session starts         session-start.sh            Inject prior knowledge into briefing
Session ends           session-end-maintenance.sh  Harvest learnings, run maintenance
Agent stops            ao-flywheel-close.sh        Close the learning loop
Task completes         task-validation-gate.sh     Execute active compiled constraints and metadata checks
Every tool call        go-complexity-precommit.sh  Block functions over complexity budget
Pre-commit             skill-lint-gate.sh          Reject malformed skills
Pre-commit             dangerous-git-guard.sh      Block force-pushes to main
Pre-commit             pre-mortem-gate.sh          Require pre-mortem for large changes
Worker stop            subagent-stop.sh            Clean up parallel agent state
Worktree created       worktree-setup.sh           Initialize isolated workspace
Worktree merged        worktree-cleanup.sh         Remove stale branches
```

---

## Skill Tiers at a Glance

```
JUDGMENT             EXECUTION              KNOWLEDGE           INTERNAL
────────             ─────────              ─────────           ────────
council              research               retro               inject
vibe                 plan                   forge               extract
pre-mortem           implement              flywheel            ratchet
post-mortem          crank                  goals               standards
                     swarm                                      beads
                     rpi                                        shared
                     evolve
                     release
                     doc
                     status
                     handoff
                     quickstart
                     brainstorm
                     bug-hunt
                     complexity
                     + 14 more
```

---

*52 skills · 121 CLI commands · 13 hook entries · 0 telemetry · everything in plain files*
