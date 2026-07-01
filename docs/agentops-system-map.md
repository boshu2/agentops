# AgentOps — System Map

```
┌──────────────────────────────────────────────────────────────────┐
│                    AgentOps at a Glance                          │
├───────────────────┬──────────────────────┬───────────────────────┤
│   70 Skills       │   76 CLI Commands    │   Hookless (CI-gated) │
│  (workflows)      │  (ao binary)         │  (validate.yml)       │
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
/post-mortem       →    ao compile                 Findings become artifacts, checks, and constraints
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
ao flywheel / compile  (explicit command — no auto-hook)
        │
        ├──> .agents/findings/<id>.md
        ├──> .agents/planning-rules/<id>.md
        ├──> .agents/pre-mortem-checks/<id>.md
        └──> .agents/constraints/index.json   (mechanical + active only)
                                              │
                                              ▼
                              .github/workflows/validate.yml  (CI gate)
```

`/plan`, `/pre-mortem`, `/vibe`, and `/post-mortem` load compiled planning and review artifacts first, then fall back to the registry when compiled outputs are missing. AgentOps 3.0 is hookless: enforcement of active mechanical findings is shift-left via the CI gates in `.github/workflows/validate.yml`, not an auto-firing hook.

---

## CLI Command Groups (76 commands including subcommands)

```
KNOWLEDGE FLYWHEEL          VALIDATION GATES         SESSION / LIFECYCLE
──────────────────          ────────────────         ───────────────────
ao forge                    ao gate pending          ao session close
ao pool ingest              ao gate approve          ao rpi status
ao pool promote             ao gate reject           ao rpi cancel
ao lookup                   ao ratchet status        ao session bootstrap
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

## Enforcement — Hookless, CI-Gated

AgentOps 3.0 is hookless: it ships zero hooks by default and nothing auto-injects or auto-enforces at session boundaries. Validation that used to live in shell hooks is now the authoritative CI gate in `.github/workflows/validate.yml` (T0/T1/T2 tiers, all required), with explicit `ao` commands and skills doing the in-session work. An opt-in `hooks-authoring` skill lets you add your own hooks if you want always-on local signals, but the corpus carries none.

```
WAS A HOOK                  NOW
──────────                  ───
Session-boundary staging    ao session bootstrap / ao inject (explicit, run on demand)
Learning-loop close         ao flywheel close-loop (called by /post-mortem)
Task/constraint validation   .github/workflows/validate.yml CI gates
Complexity budget            golangci-lint + validate.yml
Skill / git / pre-mortem     validate.yml gates + /pre-mortem skill
checks                       (re-author as opt-in via hooks-authoring skill)
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

*70 skills · 76 CLI commands · hookless (CI-gated) · 0 telemetry · everything in plain files*
