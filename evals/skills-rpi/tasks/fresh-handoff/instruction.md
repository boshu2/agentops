Complete the two-file batch-report repair in /app/work. Parse must reject a
blank name, invalid or overflowing integer, negative amount, and rows with
other than two comma-separated fields. Ignore blank lines and trim fields.
Any bad row returns an error and no partial data. Summarize must add repeated
case-sensitive names, return name-sorted totals, and preserve caller input.
Report must integrate both behaviors. Integer-sum overflow is outside scope.

This is a native Harbor task with producer and successor steps. Their complete
prompts live in steps/producer/instruction.md and steps/successor/instruction.md.
The caller requires a fresh successor conversation over the producer's workspace
and HANDOFF.md, with job agent.resume_trajectory=false. The producer repairs
parsing and hands off; the successor integrates the summary repair and obtains
a fresh independent final review. Writing HANDOFF.md alone does not establish
that the native transition or final review happened.

Only parse.go, summary.go, HANDOFF.md and new *_test.go files may change. Keep
existing public tests and go.mod. Do not push or add dependencies. State any
missing session transition or review as unproven.
