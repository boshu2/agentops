# Skill Disposition Triage — 2026-06-16 (Wave F)

Mechanical snapshot from [`docs/contracts/skill-dispositions.yaml`](../contracts/skill-dispositions.yaml) (grep counts, not judgment).

## Counts by disposition

| Disposition | Count |
|-------------|------:|
| `keep` | 38 |
| `update` | 27 |
| `refactor` | 7 |
| `cut` (terminal `state:` rows) | see ledger |

## Triage checklist (Wave F scope)

- [ ] **update (27):** batch by bounded context; each batch needs a bead + mechanical acceptance before edit.
- [ ] **refactor (7):** prioritize loop-adjacent skills (`implement`, `crank`, `push`) before peripheral adapters.
- [ ] **cut candidates:** only via `ao skills retire` + disposition ledger flip — no drive-by deletes.
- [ ] **Codex parity:** any disposition change that touches `skills/` must run `make regen-check` before land.

## Catalog hygiene note

Historical CHANGELOG entries still reference Gas City as the default out-of-session substrate. Current doctrine (2026-06): **NTM/ATM + Agent Mail** is the live reference substrate; Gas City remains an optional SDK adapter, not the default operator path. See [Unreleased] in `CHANGELOG.md`.
