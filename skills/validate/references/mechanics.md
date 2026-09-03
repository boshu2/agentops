# Validate mechanics

Loaded by `SKILL.md` at the manifest step (helper commands), at cross-family
dispatch (adapters), and at scope disclosure (the homes table). `$SKILL_DIR`
is the directory containing `SKILL.md` — `skills/validate/` in a repository
checkout, `.agents/skills/validate/` in an installed runtime.

## Helper commands

| Command | Required | Optional |
|---|---|---|
| `manifest` | `--root <dir>`, `--include <path>` (repeatable, at least one) | `--exclude <path-or-glob>` (repeatable), `--base-manifest <file>`, `--git-metadata-json <json>`, `--output <file>` |
| `verify-manifest` | `--root <dir>`, `--manifest <file>` | `--base-manifest <file>` |
| `snapshot-intent` | `--source <file>` (`-` reads stdin) | `--workspace <dir>`, `--intent-dir <dir>` |
| `digest` | `<json-file>` positional | none |
| `store-verdict` | `--draft`, `--intent-source`, `--subject-manifest`, `--author-context-id`, `--validator-context-id`, `--freshness-source <runtime\|caller>`, `--freshness-attester-id`, `--scope-result <PASS\|FAIL\|NOT_PROVEN>` | `--workspace <dir>`, `--verdict-dir <dir>` |

```sh
python3 "$SKILL_DIR/scripts/validate.py" manifest \
  --root . --include skills/validate --exclude '**/*.log' --output manifest.json
```

`manifest` uses only filesystem content; Git commit and tree IDs are optional
metadata. `store-verdict` snapshots the exact resolved intent under
`<workspace>/.agents/ao/intents/sha256/<digest>.intent`, then computes and
injects intent and subject digests plus author, validator, and freshness facts
from runtime-derived inputs and receipts, never model transcription. Storage
defaults to `<workspace>/.agents/ao/verdicts/sha256/<digest>.json`; callers
may provide `verdict_dir`. The digest is SHA-256 over canonical JSON with
`artifact_digest` omitted. Writes use a same-directory temporary file, flush,
fsync, and atomic rename. Identical existing content is idempotent success;
conflicting content is an integrity failure represented by `NOT_PROVEN`.
`store-verdict` refuses an empty manifest and refuses a PASS carrying
`not_checked` entries, recording a `validate.integrity` finding.

## Cross-family adapters

| Orchestrating runtime | Cross-family judge leg |
|---|---|
| Claude | a read-only `codex exec` judge leg |
| Codex | a caller-selected interactive Claude session in an NTM pane |

Probe the adapters at runtime through the `agent-native` model-dispatch
recipe (`codex-exec` and/or `ntm`). The judge leg reads and judges; it never
mutates the subject. Record author and validator `model_identity` in evidence
refs and freshness attestation notes; the `verdict.v2` schema is unchanged.

## Where each scope limit lives inside a PASS

| Scope limit | Home | Example |
|---|---|---|
| A criterion proven by a bounded check | `criteria[].reason` on that criterion | "proven by the unit suite; the full integration matrix was not replayed" |
| A declared non-goal or out-of-scope area | the intent source's non-goals, optionally restated as an evidence-backed boundary criterion in `criteria` | "`cli/**` is a declared non-goal; the diff proves it untouched" |
| Residual risk or judgment caveat | the caller-facing report | "the migration path is untested against pre-3.0 stores" |
| Acceptance that genuinely went unverified | `not_checked`, and the result is `NOT_PROVEN` rather than PASS | "criterion 3 needs hardware this context cannot reach" |
