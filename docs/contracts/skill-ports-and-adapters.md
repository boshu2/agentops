# Skill ports and adapters

The core skill graph is deliberately small:

```text
rpi -> plan
rpi -> implement
rpi -> validate
```

`plan` emits `plan-packet.v1`. `implement` runs one bounded experiment and emits
`candidate-packet.v1`. `validate` computes exact content identity, obtains one
fresh independent judgment, and atomically stores `verdict.v2`. RPI reports the
result and stops.

All other skills are caller-selected strategies, specialists, setup helpers, or
runtime adapters. They may contribute context or factual evidence. They cannot
change the phase order, dispatch a core phase twice, convert runtime state into a
verdict, or decide what the caller does next.

Skill metadata in `skills/*/SKILL.md` is the sole source for tier, dependencies,
capabilities, effects, canonical status, and disposition. Generated catalogs,
routers, maps, and graphs are projections of that metadata.
