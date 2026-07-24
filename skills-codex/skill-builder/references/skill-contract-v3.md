# Shadow skill-contract.v3 grammar

`metadata.contract_v3` is a readiness declaration. Legacy
`skill_api_version: 1` metadata and the generated catalog-v3 projection remain
the live routing authority until the all-catalog cutover.

The closed schema is `schemas/skill-contract.v3.schema.json`. Every declared
object rejects unknown fields. Set-valued arrays reject duplicates. The
source-owned compiler additionally enforces relationships that JSON Schema
cannot express:

- only `plan` may refine intent, only `rpi` may dispatch phases, only
  `validate` may write a verdict, and only a runtime-layer skill may transport;
- mutating effects and authority agree, observable effects require receipts,
  and process, runtime-session, and credential effects require cleanup;
- binding produced artifacts name an existing schema and validator;
- trigger cases have family-appropriate expectations, unique IDs, no
  normalized prompt collisions, and live alias or neighbor references;
- proof commands are one executable command line and fixture references resolve
  to contained repository files; and
- `rpi` hard-depends on exactly `plan`, `implement`, and `validate`, while every
  other skill has no hard skill dependency.

Effects declare `id`, `kind`, precise `scope`, `authorization`, `cleanup`, and
`receipt`. Artifacts declare `name`, `kind`, `semantics`, and nullable
`schema_ref` and `validator`. Trigger families are `positive`, `negative`,
`ambiguity`, `aliases`, and `nearest_neighbors`. Failure families are
`unavailable`, `timeout`, `partial_evidence`, `partial_mutation`, and
`cleanup`. Proof declares a harness-oriented class, command, and fixture
references.

The compiler has two intentionally small modes:

```bash
python3 skills/skill-builder/scripts/compile_contracts.py check \
  --skill skill-builder
python3 skills/skill-builder/scripts/compile_contracts.py record \
  --skill skill-builder \
  --output skills/skill-builder/receipts/skill-builder-contract-v3-probe.json
```

Check prints canonical receipt bytes and performs no write. Record atomically
writes those exact bytes to the explicit contained path. Receipts bind the
source before and after digest, canonical contract digest, schema digest,
compiler-source identity, and fixture-set identity.

The declared proof is a separate bounded operation:

```bash
python3 skills/skill-builder/scripts/run_contract_probe.py check \
  --skill skill-builder
python3 skills/skill-builder/scripts/run_contract_probe.py record \
  --skill skill-builder \
  --output skills/skill-builder/receipts/skill-builder-contract-v3-probe.json
```

Its deterministic receipt binds the compiled contract, exact source before and
after identity, runner identity, proof class and command, fixture-set digest,
exit code, and output digests. A timeout or source mutation is `NOT_PROVEN`.
