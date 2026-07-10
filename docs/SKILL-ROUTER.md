# Skill Router

Use this when you're not sure which skill to run. For the full flow overview run
`ao session bootstrap`, then `ao lookup` for on-demand context. The same tree is
embedded in [`docs/SKILLS.md`](SKILLS.md) ("Skill Router (Start Here)") — keep
the two in sync when skills are folded or renamed.

To search skills by intent instead of reading this tree, use `ms search "<task>"`
(or `mcp__ms__search`) — the skill-search engine over both corpora ([`skills/ms/SKILL.md`](../skills/ms/SKILL.md)).

```text
What are you trying to do?
│
├─ "Prove it's done / validate" (the Membrane — no verdict = not done)
│   ├─ Code ready to ship? ───────► /validate
│   ├─ Deeper code audit? ────────► /validate --mode=post-impl
│   ├─ Plan ready to build? ──────► /pre-mortem
│   ├─ Independent judges ────────► /council validate recent
│   ├─ Adversarially probe it ────► /red-team  or  /review (bug-hunt mode)
│   ├─ Fresh immutable landing lane ► /pawl-review → ao pawl
│   ├─ Drive fixes to agreement ──► /converge
│   ├─ Mid-epic drift check ──────► /reality-check
│   ├─ Security + release gate ───► /security
│   └─ Work ready to close? ──────► /validate, then /post-mortem
│
├─ "Track it / bookkeep it" (the Bookkeeper)
│   ├─ Break it into issues ──────► /plan
│   ├─ Manage/close issues ───────► /beads-br
│   ├─ Turn a goal into a loop-ready packet ─► /goal-design
│   ├─ Shape a fuzzy idea ────────► /idea-genie → /discovery
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
│   ├─ Understand this codebase ──► /codebase-recon
│   ├─ Compare approaches ────────► /council research <topic>
│   ├─ Generate ideas ────────────► /idea-genie
│   ├─ Contest a one-way-door choice ► /dueling-idea-genies
│   └─ Research an external topic ► /research
│
├─ "Learn from past work"
│   ├─ Extract a recurring code shape ─► /pattern-mining → /operationalize
│   ├─ Turn the corpus into operator surfaces ─► /operationalize
│   ├─ What do we know about X? ──► ao lookup "<query>" / ao search
│   ├─ Save this insight ─────────► /post-mortem --quick "insight"
│   └─ Full retrospective ────────► /post-mortem
│
├─ "Parallelize work"
│   ├─ Multiple independent tasks ► /swarm
│   ├─ Full epic with waves ──────► /crank <epic-id>
│   └─ Persistent pane roles/factory ► /agent-native + /ntm (+ /agent-mail on contention)
│
├─ "City-shaped multi-quest work" (gas city — operator choice, coexists with NTM; never auto-routed)
│   ├─ Stand up / drive / admin / unstick a city ─► /using-gc
│   └─ The close door + pawl-verdict.v1 internals ► gc-membrane (reference)
│
├─ "Ship a release"
│   └─ Changelog + tag ──────────► /release <version>
│
├─ "Session management"
│   ├─ Compile a reusable operator surface ► /operationalize
│   ├─ Capture the session learning ─► /post-mortem
│   ├─ Where was I? ──────────────► /status
│   ├─ Save for next session ─────► /handoff
│   └─ Recover after compaction ──► /recover
│
└─ "First time here" ────────────► ao session bootstrap → /status
```
