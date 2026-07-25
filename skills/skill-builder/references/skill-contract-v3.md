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
- proof commands use a repo-owned executable or an approved interpreter
  followed by a repo-owned script; inline interpreter programs and unrestricted
  PATH commands fail closed; every harness and fixture reference resolves to a
  contained repository file, and the command entrypoint is one of the declared
  harnesses; and
- `rpi` hard-depends on exactly `plan`, `implement`, and `validate`, while every
  other skill has no hard skill dependency.

Effects declare `id`, `kind`, precise `scope`, `authorization`, `cleanup`, and
`receipt`. Artifacts declare `name`, `kind`, `semantics`, and nullable
`schema_ref` and `validator`. Trigger families are `positive`, `negative`,
`ambiguity`, `aliases`, and `nearest_neighbors`. Failure families are
`unavailable`, `timeout`, `partial_evidence`, `partial_mutation`, and
`cleanup`. Proof declares a harness-oriented class, command, nonempty
`harness_refs`, and nonempty `fixture_refs`.

The compiler has two intentionally small modes:

```bash
python3 skills/skill-builder/scripts/compile_contracts.py check \
  --skill skill-builder
python3 skills/skill-builder/scripts/compile_contracts.py record \
  --skill skill-builder \
  --output skills/skill-builder/receipts/skill-builder-contract-v3-compile.json
```

Check prints canonical receipt bytes and performs no write. Record atomically
writes those exact bytes to the explicit contained path. PASS receipts bind
the source before and after digest, canonical contract and schema digests,
compiler-source identity, command entrypoint, and every harness and fixture
digest. Contract rejection still emits a schema-valid FAIL receipt with typed
errors, the source facts available at the failure boundary, and compiler and
schema identities; unavailable contract or proof evidence remains null.

The declared proof is a separate bounded operation:

```bash
python3 skills/skill-builder/scripts/run_contract_probe.py check \
  --skill skill-builder
python3 skills/skill-builder/scripts/run_contract_probe.py record \
  --skill skill-builder \
  --output skills/skill-builder/receipts/skill-builder-contract-v3-probe.json
```

The runner copies the repository to a disposable directory and executes only
there. It drains both streams while retaining at most 65,536 bytes per stream,
records total bytes and truncation, and owns a fresh process group. Timeout,
interruption, or a surviving descendant triggers bounded TERM then KILL
cleanup and reaping.

Its deterministic receipt binds the compiled contract, compiler, exact source,
runner sources, command entrypoint, and every harness and fixture digest. It
also records initial and final isolated manifests, changed and out-of-scope
paths, live-root stability, stream facts, timeout state, and process-group
cleanup. Incomplete cleanup or uncertain live-source identity is
`NOT_PROVEN`; nonzero execution or an out-of-scope isolated change cannot PASS.
