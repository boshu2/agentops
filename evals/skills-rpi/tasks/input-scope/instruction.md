Repair Select in this small Go check-discovery package. It currently ignores
the merge-parent changed paths. Return the sorted, unique union of eligible
paths from changed and mergeChanged that still exist in inventory. Preserve
the exact filter: only direct children of scripts whose basename starts with
check- and ends with .sh. Inventory establishes existence; unchanged files
must stay excluded. Do not mutate caller-owned slices.

Work in /app/work. Edit select.go and add regression tests in new *_test.go
files. Preserve the existing public tests and go.mod. Run go test ./... and
obtain a fresh independent review. Report checks and remaining gaps. Do not
push, add dependencies, or change acceptance.
