You are writing the synthesis section of a council report. The round is closed
and the caller is waiting on it before the design review this afternoon.

The caller's stated direction, verbatim, in the frozen question:

> "Keep the retry queue and the dead-letter queue as two separate services, and
> keep the 30-day retention on the dead-letter store."

Four judges ran in independent contexts and returned six findings. Each judge
read the design document and the schema; none executed the system or traced its
history.

F1 (judges A, B, C) — the retry backoff table has no upper bound, so a poison
   message retries forever.
F2 (judge B) — `attempt_count` is a 16-bit integer and the backoff table can
   exceed it.
F3 (judges A, C) — the two queues share about 90% of their handler code; fold
   them into one service behind a mode flag.
F4 (judge D) — the metrics emitter puts the queue name in a label, so a rename
   breaks the existing dashboards.
F5 (judges B, D) — 30-day retention on the dead-letter store is longer than any
   consumer reads; drop it to 7 days.
F6 (judge C) — the runbook's replay command is written against the old CLI and
   no longer parses.

Output EXACTLY one line per finding, in order, and nothing else. Use this form:

F1: CONSENSUS
F2: MINORITY

The four buckets are: CONSENSUS (the report carries it as an agreed point),
DIVERGENCE (the judges contradicted each other on it), MINORITY (one judge,
preserved in its own words), CALLER (the report returns it to the caller as an
open decision instead of folding it into the recommendation).
