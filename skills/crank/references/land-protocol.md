# Serialized Multi-Lane Delivery Protocol

This is a repository delivery adapter, not a Crank phase. Crank ends after wave
evidence; Validate produces immutable proof; Learn records plan impact. Delivery
consumes those artifacts without asking an LLM for another landing verdict.

## Direct-main serialization

Parallel implementation is safe only in isolated, non-overlapping worktrees.
Delivery is serial: one lane at a time may be between fetching the current
remote tip and confirming its own push. Git rejects a stale non-fast-forward
push; the lane then integrates the new tip and retries.

1. Commit one coherent, independently revertible bead arc.
2. Run the repository's deterministic scoped gate.
3. Fetch `origin/main` and integrate it if the remote tip moved.
4. If integration changes the payload, rerun the deterministic scoped gate.
5. Push using the repository-selected command.
6. Fetch again and prove the delivered commit is an ancestor of the remote
   branch before closing tracker state.

For this repository, the routine gate is:

```bash
ao gate check --fast --scope head
```

The exact push destination depends on operator and repository policy. A PR,
user-owned CI, or direct push may all consume the same immutable validation
proof. AgentOps does not own a global Git queue.

## Failure handling

| Failure | Response |
|---|---|
| Remote tip moved | Fetch, integrate, rerun the scoped gate if the tree changed, and retry. |
| Gate fails after integration | Return to the operating loop with the new evidence; do not push. |
| Push reports success but ancestry fails | Treat delivery as unproven and investigate the destination/ref before tracker close. |
| Another lane is delivering | Wait until that lane confirms or abandons its push window. |

Never infer delivery from a local commit, a successful validation verdict, or a
push log line alone. Completion requires confirmed remote ancestry and the
repository's terminal tracker policy.
