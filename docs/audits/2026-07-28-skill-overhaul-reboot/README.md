# Skill overhaul reboot — salvaged audit evidence

Working evidence for
[the 2026-07-28 reboot plan](../../plans/2026-07-28-skill-overhaul-reboot.md).

## raw/

Final versions of the four deep per-skill audits produced by the 2026-07-24
program (Opus 5 workers, fresh Sol reviews), salvaged from `/tmp` before the
program was rebooted. Together they cover all 49 canonical skills.

| Audit | Review disposition |
|---|---|
| `...core-12-v8.md` | PASS |
| `...workflow-12-v8.md` | PASS |
| `...outcomes-12-v13.md` | REQUEST_CHANGES — report meta only; reviewer: "the twelve-skill technical audit and exact manifest remain credible" |
| `...adapters-13-v14.md` | REQUEST_CHANGES — self-account of edit history only; technical sections byte-identical to reviewed v13 |
| `...fable-current-program-validation-v1.md` | advisory program-level validation |

The paired `-review-sol.md` files are the reviews themselves. Treat every
audit claim as a hypothesis against the live tree: the audits ran against a
seed-era tree, and `main` has since merged #988, #995, and #996.

## checklists/

Distilled, per-skill actionable checklists extracted from the raw audits.
These are the working inputs for the reboot waves; same hypothesis caveat.
