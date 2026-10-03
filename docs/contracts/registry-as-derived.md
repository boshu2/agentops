# Generated skill projections

`skills/*/SKILL.md` metadata is the sole source for skill tier, dependencies,
capabilities, effects, canonical status, and disposition.

`scripts/generate-skill-mesh.py` derives the registry, catalog, router, graph,
domain map, context map, counts, and runtime manifests in one pass. Its
`--check` mode verifies every projection without modifying source metadata.

No projection is a second authority. Do not hand-edit generated inventories or
maintain a parallel count, keep-list, dependency graph, or context map. Change
the owning `SKILL.md` metadata and regenerate.

No copy of the skills is generated for any runtime: Codex and Claude both load
`skills/<name>/`. A deleted source skill is removed from generated inventories
on refresh.
