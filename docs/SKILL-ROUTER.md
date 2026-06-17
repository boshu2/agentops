# Skill Router

Use this when you're not sure which skill to run. For the full flow overview run
`ao session bootstrap`, then `/inject` for on-demand context. The same tree is
embedded in [`docs/SKILLS.md`](SKILLS.md) ("Skill Router (Start Here)") — keep
the two in sync when skills are folded or renamed.

```text
What are you trying to do?
│
├─ "Not sure what to do yet"
│   └─ Generate options first ─────► /discovery --ideate
│
├─ "I have an idea"
│   └─ Understand code + context ──► /research
│
├─ "I know what I want to build"
│   └─ Break it into issues ───────► /plan
│
├─ "Now build it"
│   ├─ Small/single issue ─────────► /implement
│   ├─ Multi-issue epic ───────────► /crank <epic-id>
│   └─ Full flow in one command ───► /rpi "goal"  (one turn's executor, not primary)
│
├─ "Fix a bug"
│   ├─ Already scoped? ────────────► /implement <issue-id>
│   └─ Need to investigate? ───────► /review (bug-hunt mode)
│
├─ "Build a feature"
│   ├─ Small (1-2 files) ─────────► /implement
│   ├─ Medium (3-6 issues) ───────► /plan → /crank
│   └─ Large (7+ issues) ─────────► /rpi (full pipeline)
│
├─ "Validate something"
│   ├─ Code ready to ship? ───────► /validate
│   ├─ Plan ready to build? ──────► /pre-mortem
│   ├─ Work ready to close? ──────► /validate, then /post-mortem
│   └─ Quick sanity check? ───────► /council --quick validate
│
├─ "Explore or research"
│   ├─ Understand this codebase ──► /research
│   ├─ Compare approaches ────────► /council research <topic>
│   └─ Generate ideas ────────────► /discovery --ideate
│
├─ "Learn from past work"
│   ├─ What do we know about X? ──► ao lookup "<query>" / ao search
│   ├─ Turn corpus into operator surfaces ─► /operationalize
│   ├─ Save this insight ─────────► /post-mortem --quick "insight"
│   └─ Run a retrospective ───────► /post-mortem
│
├─ "Parallelize work"
│   ├─ Multiple independent tasks ► /swarm
│   └─ Full epic with waves ──────► /crank <epic-id>
│
├─ "Ship a release"
│   └─ Changelog + tag ──────────► /release <version>
│
├─ "Session management"
│   ├─ Compile knowledge ─────────► /forge or /compile
│   ├─ Where was I? ──────────────► /status
│   ├─ Save for next session ─────► /handoff
│   └─ Recover after compaction ──► /recover
│
└─ "First time here" ────────────► ao session bootstrap → /status
```
