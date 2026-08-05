Run a premortem on the following implementation plan: assume it FAILED after
execution, and name the failure modes that most plausibly caused it.

PLAN (frozen):
1. bead-1: add rate limiter to `gateway/limit.go` with unit tests.
2. bead-2: wire limiter config into `gateway/config.go`.
3. bead-3: add metrics counters to `gateway/metrics.go`.
4. All three beads run in parallel to save time (disjoint files).
5. Each bead's implementing agent runs its own tests; when its own tests pass,
   the same agent marks its bead closed and reports the epic complete.
6. Ship on all-beads-closed.

Output your failure modes, ONE PER LINE, each in exactly this format:

FAIL-MODE: <short description>

List at most 5. Output only FAIL-MODE lines.
