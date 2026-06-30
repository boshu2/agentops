# Audit: operator/personal-skill leakage in the published catalog (2026-06-30)

> Bead `age-focus-membrane-bookkeeper-m1wg.11`. Question: does the published skill
> catalog (registry.json, `docs/SKILLS.md`, the mkdocs site) expose operator/personal
> skills — athena, wealth-mentor, the bo-* brand/voice/myth family, or substrate
> internals — that should stay off the product surface?

## Finding: clean (no leakage)

The published catalog presents **product skills only**. Audited 2026-06-30 against the
repo at the time of writing:

- **77 skills under `skills/`**, **zero** operator/personal-identity skills among them.
  `athena`, `wealth-mentor`, `bo-voice`, `bo-brand`, `brand-story`, `on-brand`, and
  `jargon-translator` are **not present** in `skills/` or `skills-codex/`, and are
  **not referenced** in `docs/SKILLS.md` or `registry.json`.
- The reason is **structural, not incidental.** The published surface is generated from
  `skills/**/SKILL.md` into `registry.json` + the domain maps + the site. The operator's
  personal-identity skills live in the operator's own `~/.claude/skills` (and the jsm
  personal corpus), which symlink *into* the repo checkout for local use but are never
  *committed* to it. They therefore cannot enter the generated catalog.

## Clarification: substrate skills are product skills, not operator-personal

The bead flagged "NTM/ATM/pawl internals." On inspection these are **legitimate product
substrate skills**, not operator-personal identity, and stay in the catalog:

- `ntm`, `using-atm`, `vibing-with-ntm`, `dual-pane-atm`, `swarm` — the out-of-session
  orchestration substrate documented in PRODUCT.md and `docs/3.0.md`.
- `pawl` is the membrane's acceptance gate surfaced on the `ao` CLI — the proven core,
  not an operator-only internal.

These are part of what AgentOps *is*. They are not denied.

## Durable guard

To keep "gated out" true rather than incidentally-true, this audit ships an enforcing
guard instead of a point-in-time note:

- `scripts/check-no-operator-skills.sh` — fail-closed denylist guard. Fails if any
  operator/personal-identity slug appears as a `skills/` or `skills-codex/` directory, or
  is referenced as a skill in `docs/SKILLS.md` / `registry.json`.
- Wired into the release gate as **`skill.no-operator-leakage`** (blocking, fast+full,
  matches `skills/**`) in `cli/internal/gates/checks/seed.go`.
- Regression coverage: `tests/scripts/check-no-operator-skills.bats` (6 cases, incl. the
  substrate-skills-not-flagged case).

### Denylist scope (deliberately narrow)

Only **unambiguous operator-personal-identity** slugs are denied: `athena`,
`wealth-mentor`, `bo-voice`, `bo-brand`, `brand-story`, `on-brand`, `jargon-translator`.
General craft skills (e.g. `de-slopify`, `teacher-mode`) are product skills and are **not**
on the list. Add a slug to the `DENYLIST` array in the guard only when it is clearly a
Bo-personal identity skill that must never publish.
