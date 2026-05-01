---
id: result-soc-o6eb.4-w3-daemon-dream-migration-lane
type: crank-result
date: 2026-05-01
issue: soc-o6eb.4
status: complete
---

# W3 Result: Daemon Dream Migration Lane

## Selected lane

Selected `psite-agu` as the single active migration lane for `soc-o6eb.4`.
The durable decision record is
`.agents/decisions/2026-05-01-w3-daemon-dream-migration-lane.md`.

The choice is based on `bd` evidence: `psite-agu` directly migrates the wiki
ingest pipeline to agentopsd-submitted headless agents, already has daemon
activation prep closed through `psite-agu.12`, and its acceptance covers the
`psite-355` argv-size supersedence path.

## Paused lanes

- `soc-q4c`: deferred behind `psite-agu`; keep starvation overlap with
  `soc-ygo.38` unresolved until Tier 2/Tier 3 proof exists.
- `soc-ni8g`: deferred behind inherited open `psite-mto.25` through
  `psite-mto.31` blockers and selected-lane proof.
- `soc-y8b`: deferred as multi-host operating model work.
- `soc-v64.2`: deferred as design-contract work with separate-validator
  requirements.

## Blockers

Primary blockers for the selected lane are the open `psite-agu` migration
children: Wave 1 and 1.5 (`psite-agu.1`, `.2`, `.14`, `.15`, `.16`, `.17`,
`.18`) plus Wave 2 and 3 (`psite-agu.5`, `.6`, `.7`, `.8`).

Semantic blockers remain `psite-355` versus `psite-agu` and `soc-ygo.38`
versus `soc-q4c.6`.

## Activation smoke

Activation smoke to run before downstream migration work:

```bash
systemctl --user cat agentopsd.service | rg -- '--workers' &&
ao daemon ready --timeout 10 &&
test -s ~/.agents/daemon/ledger.jsonl &&
bd show psite-agu.12 psite-agu.13 psite-agu.4 --json |
  jq -e 'all(.[]; .status == "closed")'
```

## Tracker notes to apply

Tracker state was not mutated by this worker. Apply the note text from the
decision record to `soc-o6eb.4`, `psite-agu`, the four paused lane epics, and
`psite-355` in a later tracker-writer pass.

## Discoveries

- `psite-agu.9` is closed but delegates `psite-355` supersedence to
  `psite-agu.5`, so `psite-355` remains a live follow-up until Morai daemon
  cutover is proven.
- `soc-ygo.38` and `soc-q4c.6` should not be merged or closed without concrete
  Tier 2/Tier 3 starvation evidence from the selected path or the OpenClaw
  implementation slice.
