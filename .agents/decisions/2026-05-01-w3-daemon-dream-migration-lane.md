---
id: decision-2026-05-01-w3-daemon-dream-migration-lane
type: decision
date: 2026-05-01
issue: soc-o6eb.4
status: accepted
---

# W3 Daemon Dream Migration Lane

## Selected lane

Selected lane: `psite-agu` - Pipeline rearchitecture: agentopsd +
headless agents.

Rationale grounded in live `bd` evidence:

- `psite-agu` is the only candidate whose stated scope directly migrates the
  wiki ingest pipeline from shell-script plus OpenClaw argv coupling to
  agentopsd-submitted headless agents.
- Its acceptance criteria match the W3 daemon/Dream objective: daemon ledger as
  the sole per-source event truth, no shell-pipeline `-m "$VAR"` callers, the
  YT smoke chain through the daemon path, and eventual `psite-355` supersedence.
- The lane already has activation prep closed: `psite-agu.10` pipeline
  component census, `psite-agu.11` daemon capability gate and AgentOps SHA pin,
  `psite-agu.12` daemon worker activation plus ledger first-write smoke,
  `psite-agu.13` day-window constraint replication, and `psite-agu.4`
  OpenClaw demotion to operator-only.
- Compared with the other candidates, it is the narrowest executable daemon
  migration lane. `soc-q4c` keeps OpenClaw as the execution bus, `soc-ni8g`
  depends on seven open Mt. Olympus production blockers, `soc-y8b` is a
  broader multi-host rollout, and `soc-v64.2` is design-contract work rather
  than runtime migration.

No downstream migration work is implemented by this decision.

## Paused lanes

- `soc-q4c` - pause while `psite-agu` is active. Later note: defer OpenClaw
  execution-bus migration behind the agentopsd lane; keep `soc-q4c.6` as the
  concrete starvation acceptance surface and reconcile it with `soc-ygo.38`
  only after daemon Tier 2/Tier 3 movement is proven or explicitly ruled out.
- `soc-ni8g` - pause behind `psite-agu` activation and inherited Mt. Olympus
  blockers. Later note: resume only after `psite-mto.25` through
  `psite-mto.31` have closure evidence and the daemon lane has A/B or ledger
  proof for Dream/Morai/T3 cutover.
- `soc-y8b` - pause as an operating-model rollout, not the next Dream migration
  lane. Later note: resume after the local daemon path is stable enough that
  multi-host dispatcher, token, phone, and session-template work will not
  mask migration failures.
- `soc-v64.2` - pause as design-contract precision work. Later note: use it
  before Olympus archive/tag or when a selected daemon implementation child
  needs an explicit contract decision; keep its non-worker validation
  requirement intact.

## Blockers

- `psite-agu.1`, `psite-agu.2`, `psite-agu.14`, `psite-agu.15`,
  `psite-agu.16`, `psite-agu.17`, and `psite-agu.18` remain the open Wave 1
  and Wave 1.5 migration blockers before Tier 2/Tier 3 retirement.
- `psite-agu.5`, `psite-agu.6`, `psite-agu.7`, and `psite-agu.8` remain the
  Wave 2 and Wave 3 blockers for Morai/T3 daemon cutover, shell-pipeline
  retirement, and final docs.
- `psite-355` should stay open until `psite-agu.5` proves the daemon
  morai-enrich path and closes the argv-size bug plus its children as
  superseded. `psite-agu.9` is already closed, but its close reason says that
  supersedence was folded into `psite-agu.5` acceptance.
- `soc-ygo.38` and `soc-q4c.6` are the same Morai/T3 starvation problem at
  different graph levels. Do not close or merge either until the selected lane
  produces concrete Tier 2/Tier 3 movement evidence or documents intentional
  throttling.

## Activation smoke

Run this smoke before starting the first downstream `psite-agu` migration child:

```bash
systemctl --user cat agentopsd.service | rg -- '--workers' &&
ao daemon ready --timeout 10 &&
test -s ~/.agents/daemon/ledger.jsonl &&
bd show psite-agu.12 psite-agu.13 psite-agu.4 --json |
  jq -e 'all(.[]; .status == "closed")'
```

This checks the worker-enabled daemon service, daemon readiness, ledger
presence, and the closed tracker gates that make `psite-agu` activatable.

## Tracker notes to apply

- `soc-o6eb.4`: W3 selected `psite-agu` as the single daemon/Dream migration
  lane; paused `soc-q4c`, `soc-ni8g`, `soc-y8b`, and `soc-v64.2`; no
  downstream migration or mass reparenting performed.
- `psite-agu`: selected for the next daemon/Dream migration lane because it
  directly removes shell/OpenClaw argv coupling and already has daemon
  activation gates closed.
- `soc-q4c`: defer behind `psite-agu`; retain `soc-q4c.6` as the starvation
  implementation surface and reconcile with `soc-ygo.38` after daemon lane
  proof.
- `soc-ni8g`: defer behind `psite-mto.25` through `psite-mto.31` and selected
  lane A/B or ledger proof.
- `soc-y8b`: defer until local daemon migration stability is proven; do not use
  multi-host rollout to validate the core Dream/Morai/T3 migration.
- `soc-v64.2`: defer until archive/design-contract timing or an implementation
  child needs the contract; preserve separate-validator requirement.
- `psite-355`: leave open until `psite-agu.5` closure proves daemon
  morai-enrich cutover and explicitly closes/supersedes `psite-355` children.

## Discoveries

- `psite-agu.9` is closed, but its close reason redirects `psite-355`
  supersedence into `psite-agu.5`; future workers should not treat the closed
  `psite-agu.9` bead as proof that `psite-355` is already resolved.
- The W1 semantic-overlap warning remains active: `soc-ygo.38` is investigation
  context and `soc-q4c.6` is an implementation slice for the same starvation
  symptom. The selected lane should produce or cite evidence before either is
  closed or related.
