Complete the two-file batch-report repair in /app/work. Parse must reject a
blank name, invalid or overflowing integer, negative amount, and rows with
other than two comma-separated fields. Ignore blank lines and trim fields.
Any bad row returns an error and no partial data. Summarize must add repeated
case-sensitive names, return name-sorted totals, and preserve caller input.
Report must integrate both behaviors. Integer-sum overflow is outside scope.

The caller requires a fresh-session handoff before completion. In the producer
session, repair parse.go, add relevant tests in a new *_test.go file, and record
a concise HANDOFF.md with the unchanged acceptance, files edited, checks and
the remaining summary.go integration. Stop after that handoff. The runner must
start a separate successor session with this workspace and HANDOFF.md, without
the producer trajectory. That successor repairs summary.go, runs integration
tests and obtains a fresh independent final review. The runner owns the actual
session transition; writing a handoff file does not establish that it happened.

Only parse.go, summary.go, HANDOFF.md and new *_test.go files may change. Keep
existing public tests and go.mod. Do not push or add dependencies. State any
missing session transition or review as unproven.
