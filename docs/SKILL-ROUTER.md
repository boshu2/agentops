# Skill Router

Use this when you're not sure which skill to run.

> **Derived copy.** The canonical router tree lives in the "Skill Router (Start
> Here)" section of [`SKILLS.md`](SKILLS.md). This file is a standalone copy for
> quick linking; when the two diverge, `SKILLS.md` wins — update it first, then
> mirror the change here.

```text
What are you trying to do?
│
├─ "Not sure what to do yet"
│   └─ Generate options first ─────► /brainstorm
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
│   └─ Full flow in one command ───► /rpi "goal"
│
├─ "Fix a bug"
│   ├─ Already scoped? ────────────► /implement <issue-id>
│   └─ Need to investigate? ───────► /bug-hunt
│
├─ "Build a feature"
│   ├─ Small (1-2 files) ─────────► /implement
│   ├─ Medium (3-6 issues) ───────► /plan → /crank
│   └─ Large (7+ issues) ─────────► /rpi (full pipeline)
│
├─ "Validate something"
│   ├─ Work ready to close? ──────► /validation
│   ├─ Code quality only? ───────► /vibe
│   ├─ Plan ready to build? ──────► /pre-mortem
│   └─ Quick sanity check? ───────► /council --quick validate
│
├─ "Explore or research"
│   ├─ Understand this codebase ──► /research
│   ├─ Compare approaches ────────► /council research <topic>
│   └─ Generate ideas ────────────► /brainstorm
│
├─ "Learn from past work"
│   ├─ Turn the corpus into operator surfaces ─► /knowledge-activation
│   ├─ What do we know about X? ──► ao lookup "<query>" / ao search
│   ├─ Save this insight ─────────► /retro --quick "insight"
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
│   ├─ Prep or review Dream runs ─► /dream
│   ├─ Where was I? ──────────────► /status
│   ├─ Save for next session ─────► /handoff
│   └─ Recover after compaction ──► /recover
│
└─ "First time here" ────────────► ao quick-start → /quickstart
```
