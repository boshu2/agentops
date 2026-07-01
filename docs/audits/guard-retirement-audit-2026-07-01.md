# Audit: retiring check-*/validate-* guards for archived surfaces (2026-07-01)

> Bead `age-focus-membrane-bookkeeper-m1wg.24` (Wave 4). Question: now that the epic
> archived the corpus/flywheel + RPI/factory satellites, which `check-*`/`validate-*`
> guards became dead weight and should be retired? Sequenced last, after `.13`/`.14`/`.15`
> (archival) and `.18`/`.19` (twin scoping/drop) landed.

## Finding: no guards are dead — archive-not-delete keeps every target alive

The epic's core principle is **archive, not delete** ([ADR-0012](../adr/ADR-0012-focus-surface-on-membrane-bookkeeper-archive-satellites.md)):
commands are hidden behind `//go:build flywheel|legacy` (still compilable), skills are
demoted to the `experimental` tier (still present), and non-spine Codex twins are
`excluded`/frozen (source skills still present). The **only** truly-deleted surface was
`ao cron` (`.15`), a dead compat shim.

Consequently, **every** guard the bead named still has a live target and runs green:

| Guard | Target status | Verdict |
|-------|---------------|---------|
| `validate-codex-lifecycle-guards.sh` | spine twins present (partitioned spine-hard / frozen-ambient by `.18`) | KEEP — runs green |
| `check-docs-no-retired-tech.sh` | the retired-tech policy (bd/Dolt/hooks) still applies | KEEP — runs green |
| `check-workflow-no-retired-tracker.sh` | the br-only workflow policy still applies | KEEP — runs green |
| `validate-codex-rpi-contract.sh` | its inject-twin assertion was already narrowed by `.19`; remaining crank/discovery/validate spine twins present | KEEP — runs green |
| `check-evolve-cycle-logging.sh` | the `evolve` skill is demoted-experimental, not removed | KEEP — runs green |
| `check-no-daemonized-wake.sh` | this is the ANTI-`ao cron`/anti-daemon policy guard (ADR-0009) — retiring it would REMOVE the protection that keeps the deleted shim from returning | KEEP — must not retire |
| `validate-skill-cli-snippets.sh` | reconciled with archival by `age-sydq` (builds the validation binary with `-tags "flywheel legacy"` so archived-command snippets validate) | KEEP — already fixed |

## Conclusion

There are **zero** dead guards to retire. The finding is itself the value: the
archive-not-delete strategy kept the guard surface coherent — no guard was left
pointing at a vanished target (the two guards that touched archived surfaces —
`validate-codex-rpi-contract.sh` and `validate-skill-cli-snippets.sh` — were reconciled
in-place by `.19` and `age-sydq`, not deleted). Retiring any of the above would remove
live coverage or (for `check-no-daemonized-wake.sh`) the very protection against the one
surface this epic deleted. `.24` retires nothing by design.
