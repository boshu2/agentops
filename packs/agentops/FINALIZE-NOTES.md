# AgentOps Reference City — finalize notes (gvkj6)

> **This pack is SCAFFOLDING.** It is gc-parse-valid and structurally complete,
> but two things are deferred to the `gvkj6-finalize` pass (post code-rip). Do NOT
> open a PR from this branch until both are resolved + the City is gate-validated.

## What's built (this pass)

```
city.toml                                  reference city — consumes GC substrate
packs/agentops/
  pack.toml                                named_session bindings: mayor + refinery
  agents/mayor/{agent.toml,prompt.template.md}      team-lead: orchestrate/merge/notify
  agents/refinery/{agent.toml,prompt.template.md}   worker: runs `ao rpi` (one unit)
  formulas/rpi-dispatch.toml               THIN 1-step dispatch → `ao rpi <bead>`
  formulas/evolve-dispatch.toml            THIN 1-step dispatch → `ao evolve`
  orders/bead-dispatch.toml                cooldown sweep → rpi-dispatch (refinery pool)
  orders/evolve-cadence.toml               cron → evolve-dispatch (mayor pool)
  orders/compile-corpus.toml               exec order → ao-compile.sh (was: ao compile)
  orders/maturity-scan.toml                exec order → ao maturity --scan
  template-fragments/{agentops-doctrine,operating-loop,standards-brief}.template.md
  assets/scripts/{install-ao.sh,worktree-setup.sh,ao-compile.sh}
  overlay/.claude/settings.json            hook wiring (gc + ao inject)
  overlay/.claude/skills/README.md         DEFERRED placeholder (skills snapshot)
```

## DEFERRED #1 — the skills overlay snapshot

`overlay/.claude/skills/` ships only a README placeholder. It is populated
post-rip with the lean `skills/**` set so the City does not ship skills for killed
features. See `overlay/.claude/skills/README.md` for the finalize checklist.

## DEFERRED #2 — `ao validate --gate` (pending AgentOps CLI addition)

The refinery prompt + the `ao rpi` validate phase reference **`ao validate
--gate`** — it must return a clean exit code (0 = PASS/WARN, non-zero = FAIL) so a
gate can be exit-code-driven (design doc Part 4: "the load-bearing new contract").

**Status: NOT YET IMPLEMENTED.** This is a pending AgentOps CLI addition, part of
the gvkj6-finalize work. Do NOT implement it here (the code-rip owns `cli/`). When
it lands, the validate phase inside `ao rpi` and any gate references resolve to a
real exit code. Until then, treat `ao validate --gate` as a placeholder contract.

NOTE: the reference City does NOT decompose rpi into a GC `check.max_attempts`
retry step (that's the THIN-SEAM-forbidden 4-step formula). The validate phase and
its retry/fresh-agent-on-failure behavior live INSIDE `ao rpi`, so `ao validate
--gate` is called by AgentOps, not by a GC formula step.

## Decoupling note (worktree CLAUDE.md, soc-2rtm0)

The in-CLI gc-bridge (`runtime=gc`) was REMOVED from `ao` in a prior wave. This
reference City is an **external GC pack** consumed by the standalone `gc` binary —
it does NOT depend on the removed in-CLI bridge. The seam is: `gc` (orchestration)
invokes `ao` (the loop) as a subprocess via order/formula/prompt commands. That is
the sovereign-standalone posture the strategy doc endorses.

## Finalize sequence (for the operator)

1. Land the code-rip (lean `skills/`, `ao validate --gate` implemented).
2. Snapshot `skills/**` → `overlay/.claude/skills/` (copy, never symlink).
3. `gc config show --validate --city <this-dir>` → expect exit 0.
4. `ao validate --gate` against the City artifacts → expect PASS.
5. Open the PR.
