# Live Proof — typed forge extraction end-to-end (age-gcx / age-2jf)

**Date:** 2026-06-17 · **Verdict:** PASS (live, not a compile-only claim)

The honesty condition for closing age-2jf: the `--typed` forge opt-in must actually
run against a live LAW-0 model and produce schema-valid typed records + sealed
PROV-O edges. This is that proof.

## Command (real codex backend, no fake)

```
AGENTOPS_FORGE_TYPED=1 ao forge transcript multi-extract.jsonl --typed
```
- `ao` built from origin/main @ a356206cd (includes age-nr7 wiring + age-nzx codex strict-schema fix).
- Backend: `codex exec` (Codex Pro sub, LAW-0 compliant) via runCodexExec. No `claude -p`.
- Run in an isolated workdir (/tmp/b2-proof) over cli/testdata/transcripts/multi-extract.jsonl.

## Result — typed path fired (NO heuristic fallback)

Output did NOT contain `typed extraction failed` / `falling back to heuristic`.
7 typed records emitted to `.agents/knowledge/pending/*.learning.json`:

```
learn-2026-01-24-artifactjwtexpclaimvalidationhandler.learning.json
learn-2026-01-24-artifactowaspauthenticationcheatsheet.learning.json
learn-2026-01-24-artifactvalidationcache.learning.json
learn-2026-01-24-decisiontokenhashkeyedvalidationcache.learning.json
learn-2026-01-24-findingmiddlewarecachebrokesessionisolation.learning.json
learn-2026-01-24-learningcacheonlytokenvalidationresult.learning.json
learn-2026-01-24-learningvalidatejwtexpexplicitly.learning.json
```

### Records validate against learning.v1 (7/7)

Each validated against schemas/learning.v1.schema.json + schema_version==1, non-empty
content, category in {architecture,debugging,process,testing,security}. Sample:

```json
{
  "learning_id": "learn-2026-01-24-findingmiddlewarecachebrokesessionisolation",
  "content": "Caching at the HTTP middleware level broke session isolation between users by caching the entire auth response including user-specific data.",
  "category": "process", "confidence": 0.5, "utility_score": 0,
  "source_session": "test-session-002", "created_at": "2026-01-24T11:00:00Z",
  "tombstoned": false, "schema_version": 1
}
```

`VALID 7/7 records; invalid=0`

### PROV-O edges sealed + chain verified (the graph is no longer faked)

`docs/provenance/ledger.jsonl` (2 sealed, hash-chained edges):

```
artifact-validation-cache       wasGeneratedBy  decision-token-hash-keyed-validation-cache
learning-cache-only-token-...   wasInformedBy   finding-middleware-cache-broke-session-isolation
```

```
$ ao provenance verify
OK: provenance ledger chain intact (2 record(s))
```

## Conclusion

The native extraction engine works END-TO-END live: real codex extraction →
typed learning.v1 records (7/7 valid) → sealed, verified PROV-O edges. This
satisfies age-2jf's functional + live-proven criteria. The forge DEFAULT remains
prose (opt-in only); flipping the default stays gated on a valid positive A/B
ruler (age-jrq → age-kf-s1-close-loop-0ly.4).

Two real bugs were found and fixed BY this live proof (not by unit tests):
- **age-nr7**: forgeTypedClient returned nil (the path was dead) + relations were
  never sealed (AppendRelation had zero callers — the graph stayed faked).
- **age-nzx**: CompileSchema emitted codex-invalid schemas (OpenAI strict structured
  outputs require every property in `required`) — now pinned by a shared
  `ValidateCodexStrictSchema` validator (judge schema cross-pinned) + a live codex
  contract smoke so this class of "our code's claim vs the real external contract"
  drift is caught before a live run.
