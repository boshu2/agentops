# age-anc — jq schema-fallback accept path

**Bead:** `age-anc`  
**Date:** 2026-06-17  
**Commit:** 3941747d9a2e3f7540fdf64a6e2e53d433fc5726

## Problem

On hosts without `check-jsonschema` or `python3 -c 'import jsonschema'`, `scripts/pawl-verdict.sh` falls back to strict jq validation. The fallback used `(["CONFIRMED",...]) | index(.disposition)` which indexes an array with a string field from the wrong context — jq error or false reject on every valid verdict.

## Fix

Bind enum values before `index()` for `disposition`, `mode`, and refuter `verdict`:

```jq
(.disposition as $d | (["CONFIRMED","REFUTED","ESCALATE","HOLD"] | index($d)) != null)
```

## Proof

```bash
bats tests/scripts/pawl-verdict-jq-schema-fallback.bats
```

Shadows jsonschema validators via PATH wrapper; asserts `check` authorizes a valid CONFIRMED verdict.
