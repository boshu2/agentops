# Optimization Lever Catalog — pull in this order, one at a time

Read this in **Phase 2** after the profile names the top frame. Try levers
top-down: the highest tiers give order-of-magnitude wins; the lowest give
constant-factor wins and add the most risk. Apply ONE, then guard (Phase 3).

## Tier 1 — Algorithmic complexity (biggest wins)

The only lever that changes the *shape* of the curve. Always check first.

- Replace accidental `O(n²)` (nested scans, repeated `list.contains`, string `+=`
  in a loop) with hashing / sorting / indexing → `O(n)` or `O(n log n)`.
- Memoize / cache pure repeated subcomputations.
- Short-circuit and prune: stop early, filter before transform, push predicates
  down to storage so less data is ever touched.
- Pick the right data structure: hash map for lookup, heap for top-k, ring buffer
  for streaming, trie/bitset for membership.

If big-O is already optimal, stop reaching for cleverness here and drop to Tier 2.

## Tier 2 — Cache locality & memory layout

Constant factors that often dominate once big-O is fixed (memory-bound code).

- Struct-of-arrays vs array-of-structs for the hot field; pack hot fields together.
- Sequential access over pointer-chasing; flatten linked structures into arrays.
- Reduce working-set size so it fits in cache; block/tile large traversals.
- Confirm with `perf stat -e cache-misses` *before and after* — this lever only
  pays when misses were actually the cost.

## Tier 3 — Allocation reduction

- Preallocate to known capacity (`make([]T, 0, n)`, `Vec::with_capacity`).
- Reuse buffers / object pools across calls instead of per-call allocation.
- Stream instead of materializing whole collections; avoid intermediate copies.
- Remove redundant serialization, parsing, conversion, formatting from the hot path.
- Verify with the allocation profiler (alloc count / bytes), not wall time alone.

## Tier 4 — Parallelism & core saturation

Only after the serial path is tight. Parallelizing wasteful work just wastes
faster, and it adds correctness risk (races) — guard outputs hard.

- Data-parallel map over independent items; size the pool to core count.
- Pipeline stages so CPU and I/O overlap.
- Bound concurrency and queue depth so throughput does not destroy tail latency.
- **Check the bound first:** `perf stat` for IPC and `mpstat` for per-core load.
  If the stage is memory-bandwidth-bound, more threads give NO win — log that in
  the negative-evidence ledger so it is never re-tried.
- Eliminate false sharing (pad to cache lines) and lock contention (sharding,
  lock-free structures, finer-grained locks) — but only where contention profiles prove it.

## Tier 5 — Micro-optimization (last, smallest, riskiest)

- SIMD / vectorization where the compiler won't auto-vectorize.
- Branch reduction, hoisting invariants out of loops, strength reduction.
- Inlining hints, avoiding virtual dispatch in the hottest loop.
- These trade readability for single-digit-percent gains — keep only if the
  guard proves a win above the noise band, and comment *why*.

## The rollback contract (applies to every tier)

After any lever:
1. Outputs must be byte-identical (or within documented float tolerance) — else revert.
2. The win must exceed the measured noise band — else revert.
3. A reverted lever goes into the negative-evidence ledger with *why it failed*
   (e.g. "memory-bandwidth bound", "outputs drifted on NaN ordering", "within noise").
4. A kept lever is one attributable diff; then re-profile, because the bottleneck has moved.
