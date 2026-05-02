# Retrieval Comparison Contract

The search-eval path of `ao retrieval-bench` is the deterministic decision
gate for changing AgentOps retrieval behavior. It compares named backends over
the same manifest, search root, and `k`, then reports additive metrics for each
backend.

## Command Surface

The canonical comparison smoke is:

```bash
bash scripts/retrieval-quality-smoke.sh
```

By default the smoke runs:

```bash
ao retrieval-bench \
  --search-eval cli/cmd/ao/testdata/retrieval-bench/search-eval-manifest.json \
  --search-root "$REPO_ROOT" \
  --search-compare-backends local-lexical,ao-auto,agentic-rg,wiki-link-expand,rerank-llamacpp \
  --json
```

The smoke must run offline. It unsets `AGENTOPS_RETRIEVAL_RERANK_ENDPOINT`, so
`rerank-llamacpp` proves the unset-endpoint fallback rather than contacting a
live model.

## Report Shape

Comparison JSON is an object with:

- `id`, `manifest_path`, `search_root`, `queries`, and `k`
- `backends`, an array of per-backend reports

Each backend report must include:

- `backend`
- `queries`, `k`, `hits`, and `missing_ground_truth`
- `any_relevant_at_k`
- `avg_precision_at_k`
- `mean_reciprocal_rank`
- `results`, with per-query result paths and hit metadata

All fields are additive relative to legacy single-backend JSON unless explicitly
documented in a future versioned contract.

## Backend Semantics

- `local-lexical` is the canonical default backend.
- `ao-auto` is a deterministic file-backed adapter. It may select an internal
  strategy, but it must not require services or network access.
- `agentic-rg` is the deterministic file-backed search adapter for repository
  knowledge surfaces and session turns.
- `wiki-link-expand` starts from file-backed results, expands local wiki-style
  links, and may only return existing paths under allowed repository knowledge
  roots.
- `rerank-llamacpp` is opt-in. When `AGENTOPS_RETRIEVAL_RERANK_ENDPOINT` is
  unset, it returns the base file-backed ordering. When set, the endpoint may
  reorder candidates only; it must not introduce unknown paths.

Every backend result path must resolve under the allowed search roots and exist
at evaluation time.

## Promotion Thresholds

A backend can replace `local-lexical` as the default only when all of these are
true:

- The comparison smoke passes.
- `any_relevant_at_k` is greater than or equal to `local-lexical`.
- `mean_reciprocal_rank` is greater than or equal to `local-lexical`.
- `missing_ground_truth` does not increase.
- The result is stable across at least two consecutive checked-in evidence
  runs or an equivalent reviewable CI artifact.

`rerank-llamacpp` cannot become the default while its endpoint is only an
operator-local environment variable. Promotion requires repo-owned endpoint
configuration, documented failure behavior, and the same offline fallback
contract.

## Deferred Stores

Qdrant and Neo4j remain deferred. Reconsider them only after file-backed
comparison metrics justify the added lifecycle cost, and only through a
separate contract that covers service startup, persistence, data migration,
fallback behavior, and CI/offline operation.
