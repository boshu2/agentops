# GC33 migration provenance map

This map pins the behavior copied or retired by GC33-7. It is a source-line
map for differential fixtures, not a second lifecycle specification.

| Retained behavior | Provenance | Replacement / fixture |
|---|---|---|
| Native Formula execution and cooldown Order | Gas City v1.3.5 `8ffc009ded781a2ada2077f3a29bd712b2def0bf` | Two Formula v2 chains use controller checks at `max_attempts=1`; the enabled rig cooldown invokes one bounded native delivery sweep. Checked output cannot use `on_complete`, and drain consumes its input convoy, so `factory_feeder.py` is the explicit one-shot graph-admission bridge. |
| Bead graph and persistence substrate | Beads v1.1.0 `8e4e59d39f3459a43cf21a3236a13eca4dd874f7` | No adapter-owned ledger; the native-provider boundary is capability-qualified separately. |
| Pack role metadata and projections | embedded gascity-packs `33d3a430a67d1782ad364556cb566bdb01d0afe3` | `role_adapter.py` reads only pinned role TOMLs and v2 schemas. |
| GC33-5A forge auto-merge | retired `factory.py` lines 5176–5292 at the GC33-6 base | `tests/fixtures/gc33/gc33-5a-differential.json`: exact PR/base/head, strict app-bound protection requirements, `OPEN`/non-draft/`CLEAN`, marker-first GraphQL arm, observe-only replay. The native provider pins GitHub REST API `2026-03-10`, projects branch protection and latest check-run fields through `gh api`, and uses GraphQL `enablePullRequestAutoMerge` with `SQUASH`, `expectedHeadOid`, and the stable effect ID as `clientMutationId`. |
| Role-v2 inspect/emit and routing doctor | retired `factory.py` lines 1700–1788 and 5414–5523 at the GC33-6 base | `role_adapter.py` and `test_gc33_factory_migration.py`. |

The removed Python factory controller, integration rigs, merge slots, retry
routes, and delivery records are not reimplemented. Their absence is gated by
`scripts/check-gc-executor.sh`.
