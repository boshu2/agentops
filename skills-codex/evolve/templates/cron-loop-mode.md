---
template_version: 1
verbatim_markers:
  3-adr-cite: 5bfa1c52a93edff9316b65025a0cc75a97169c61266470cc58037f9a168cf9ba
  unblock-ladder: 704781847dee29c7e239754c15d848d0d3c0f27a5fbfd17a0c87b42d563b519a
  layer-3-authority: 8a4478939885980823d49ac156d27553c4a42258af5399b6e04f6d7f96016f1b
  no-self-stop: 62e97f78951dbd1067a1661c0fb8b8e5d8754ebd2581ddfcba63856ee4e4f48b
---

# /evolve loop-mode cron prompt (cycle {{.CronSelfAdjustCounter}})

You are in /evolve --mode=loop. This is cycle {{.CronSelfAdjustCounter}}.

## Last cycle outcome

Shipped: {{range .ShippedCommits}}{{.Sha}} ({{.Bead}}{{if .Scenario}} #{{.Scenario}}{{end}}); {{end}}
Tests delta: {{.TestsDelta}}

## Sub-beads filed this cycle

{{range .SubBeadsFiledThisCycle}}- {{.}}
{{else}}(none){{end}}

## Recommended next work

{{.NextRecommendedBead}} (advisory; Layer-3 authority may override)

<!-- VERBATIM-PRESERVE:start name="3-adr-cite" -->
Load-bearing context: docs/plans/2026-05-21-evolve-loop-epic-design.md (this epic's design memo) and the bead's per-cycle acceptance refinements.
<!-- VERBATIM-PRESERVE:end -->

<!-- VERBATIM-PRESERVE:start name="unblock-ladder" -->
## 7-step unblock ladder

If `bd ready` returns ≥1 unblocked bead, claim and work one.
If you feel "blocked": surface the blocker and pick the next ready bead. NEVER write a STOP marker.
The ladder (programmatic enforcement: soc-mlbm Wave 2):
1. Shape-compatible ready bead exists → pick it
2. Sibling pattern in another bead's description → surface match
3. Decomposable epic → recommend smaller child
4. Cross-Hop pickup → traverse discovered-from chain
5. Bug-fix fallback → smallest-surface bug
6. Still nothing? Log blocked event, pick smallest available P1 anyway
7. NEVER write STOP. Operator owns stop signals.
<!-- VERBATIM-PRESERVE:end -->

<!-- VERBATIM-PRESERVE:start name="layer-3-authority" -->
## Layer-3 shaping authority

The agent can:
- File new beads as discoveries land (`bd create` + `discovered-from`)
- Restructure the DAG
- Update the cron's recommended-next pointer
- Write ADRs for emergent precedents in `docs/decisions/`

Bounded by: bead-graph coherence, operator-set scope (preferences.yaml), no-self-stop principle.
<!-- VERBATIM-PRESERVE:end -->

<!-- VERBATIM-PRESERVE:start name="no-self-stop" -->
## The no-self-stop principle

In `--mode=loop`, the agent NEVER writes a STOP/DORMANT/KILL marker. `ao loop write-stop-marker` mechanically refuses (exit 1). When stuck: log via `ao loop blocked`, pick smallest-available P1 anyway. Stop signals come from the operator, never the agent.
<!-- VERBATIM-PRESERVE:end -->
