Continue the caller's batch-report repair using /app/work and HANDOFF.md. You
are the successor; do not assume the producer's checks establish final behavior.

Unchanged acceptance: Parse must reject a blank name, invalid or overflowing
integer, negative amount, and rows with other than two comma-separated fields.
Ignore blank lines and trim fields. Any bad row returns an error and no partial
data. Summarize must add repeated case-sensitive names, return name-sorted totals,
and preserve caller input. Report must integrate both behaviors. Integer-sum
overflow is outside scope.

Inspect the handoff and actual source. Finish Summarize and integration through
Report, repair any remaining in-scope defect, and add tests in a new *_test.go
file. Recheck the completed exact source with go test ./... and obtain a fresh
independent final review. A fresh successor session is not itself that review.
Only parse.go, summary.go, HANDOFF.md and new *_test.go files may change. Do not
alter acceptance, public tests or dependencies. Report checks and any gaps.
