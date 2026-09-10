You are the producer for a two-step batch-report repair in /app/work.

Unchanged acceptance for the complete task: Parse must reject a blank name,
invalid or overflowing integer, negative amount, and rows with other than two
comma-separated fields. Ignore blank lines and trim fields. Any bad row returns
an error and no partial data. Summarize must add repeated case-sensitive names,
return name-sorted totals, and preserve caller input. Report must integrate both
behaviors. Integer-sum overflow is outside scope.

In this step, repair parse.go, add parsing regressions in a new *_test.go file,
and run the relevant tests. Write a concise HANDOFF.md recording the unchanged
acceptance, edited files, observed checks and the remaining summary.go repair
and integration. Only parse.go, HANDOFF.md and new *_test.go files may change
in this producer step. Leave summary.go unchanged for the successor. Preserve
existing public tests and go.mod. Do not push or add dependencies.

Stop after writing the handoff. Harbor will invoke the successor as its next
native step; do not launch that successor yourself or complete its repair.
The required fresh successor depends on job agent.resume_trajectory=false,
not on this handoff file. Do not claim the whole task completed from this step.
