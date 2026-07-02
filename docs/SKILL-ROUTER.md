# Skill Router

Use this when you're not sure which skill to run. For the full flow overview run
`ao session bootstrap`, then `ao lookup` for on-demand context. The same tree is
embedded in [`docs/SKILLS.md`](SKILLS.md) ("Skill Router (Start Here)") — keep
the two in sync when skills are folded or renamed.

```text
What are you trying to do?
│
├─ "Prove it's done / validate" (the Membrane — no verdict = not done)
│   ├─ Code ready to ship? ───────► /validate
│   ├─ Deeper code audit? ────────► /validate --mode=post-impl
│   ├─ Plan ready to build? ──────► /pre-mortem
│   ├─ Independent judges ────────► /council validate recent
│   ├─ Adversarially probe it ────► /red-team  or  /review (bug-hunt mode)
│   ├─ Landing 100+ files? ───────► /pre-land-refuters
│   ├─ Drive fixes to agreement ──► /converge
│   ├─ Mid-epic drift check ──────► /reality-check
│   ├─ Security + release gate ───► /security
│   └─ Work ready to close? ──────► /validate, then /post-mortem
│
├─ "Track it / bookkeep it" (the Bookkeeper)
│   ├─ Break it into issues ──────► /plan
│   ├─ Manage/close issues ───────► /beads-br
│   ├─ Shape a fuzzy idea ────────► /discovery --ideate
│   ├─ Build a single issue ──────► /implement
│   ├─ Where was I? ──────────────► /status
│   └─ Save for next session ─────► /handoff
│
├─ "Build a feature"
│   ├─ Small (1-2 files) ─────────► /implement
│   ├─ Medium (3-6 issues) ───────► /plan → /crank
│   └─ Large (7+ issues) ─────────► /rpi (full pipeline)
│
├─ "Now build it"
│   ├─ Small/single issue ─────────► /implement
│   ├─ Multi-issue epic ───────────► /crank <epic-id>
│   └─ Full flow in one command ───► /rpi "goal"
│
├─ "Fix a bug"
│   ├─ Already scoped? ────────────► /implement <issue-id>
│   └─ Need to investigate? ───────► /review (bug-hunt mode)
│
├─ "Explore or research"
│   ├─ Understand this codebase ──► /research
│   ├─ Compare approaches ────────► /council research <topic>
│   └─ Generate ideas ────────────► /discovery --ideate
│
├─ "Learn from past work"
│   ├─ Turn the corpus into operator surfaces ─► /operationalize
│   ├─ What do we know about X? ──► ao lookup "<query>" / ao search
│   ├─ Save this insight ─────────► /post-mortem --quick "insight"
│   └─ Full retrospective ────────► /post-mortem
│
├─ "Parallelize work"
│   ├─ Multiple independent tasks ► /swarm
│   └─ Full epic with waves ──────► /crank <epic-id>
│
├─ "Ship a release"
│   └─ Changelog + tag ──────────► /release <version>
│
├─ "Session management"
│   ├─ Compile knowledge ─────────► /curate --mode=forge or --mode=compile (experimental tier)
│   ├─ Where was I? ──────────────► /status
│   ├─ Save for next session ─────► /handoff
│   └─ Recover after compaction ──► /recover
│
└─ "First time here" ────────────► ao session bootstrap → /status
```
