# Troubleshooting

## A verdict is `NOT_PROVEN`

Check the criterion results and `not_checked` list. Common causes are missing or
colliding author/validator context IDs, no freshness attestation, incomplete
changed-path coverage, subject mutation, corrupt evidence, or persistence
failure. Correct the input or environment and let the caller decide whether to
start a new invocation.

## A verdict is `FAIL`

`FAIL` means a concrete acceptance defect was proven for the exact subject.
Validate records the complete finding set and stops. It does not repair,
re-plan, retry, or choose a next action.

## Deterministic checks fail

Run the failing repository command directly. `ao gate check` is only a
deterministic test runner; it does not validate semantics or grant delivery
permission.

## Generated skill projections drift

Run:

```bash
scripts/regen-all.sh
scripts/regen-all.sh --check
```

Edit `skills/<slug>/SKILL.md` metadata rather than generated registries, maps,
image copies, or parity twins.

## A removed command is invoked

The one-release tombstone names the surviving alternative and exits nonzero.
It never forwards to the retired implementation. See [MIGRATION.md](MIGRATION.md).

## Skills are missing after an update

Refresh the installation with the platform installer, then start a fresh agent
session so its skill inventory reloads. Installers remove obsolete generated
links for deleted skills.
