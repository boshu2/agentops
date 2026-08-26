# ee outcome / curate / learn self-improvement loops

**Request:** wire ee's self-improvement loop — agents record outcomes, curate
their own signal, and feed it back as learned guidance that improves later runs.

**Decision:** not built. ee is named in `AGENTS.md` as one concrete example of a
caller-selected memory system; nothing beyond that naming is wired, and no
`ee init` runs in this repository.

**Why:**

- The loop's input is an agent grading its own outcome. `AGENTS.md` already
  rules on exactly this: *"correlated agent agreement is not independent
  evidence."* A signal produced by the same family that produced the work cannot
  license the work.
- Measured, not asserted: an independent judge scored a gamed skill 0.28 where
  its own self-grade said 1.00. A self-grade loop optimizes the grader.
- This is the deleted write-half of the old flywheel under a new name. It was
  removed on evidence; re-adding it as a dependency needs new evidence, not a
  new label.

**Reopens when:** the outcome signal comes from a context that did not author
the work — a fresh independent verdict or a deterministic check — and a locked
task set shows later runs beating earlier ones on that signal.
