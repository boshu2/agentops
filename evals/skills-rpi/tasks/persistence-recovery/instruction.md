Repair checkpoint replacement in /app/work/store.go. A cancelled Save currently
overwrites the last recoverable checkpoint before BeforeCommit runs. Stage the
complete payload in a temporary file in the destination directory, flush and
close it, call BeforeCommit once, and replace the checkpoint only if the hook
succeeds. The old checkpoint must remain readable throughout that hook and
after cancellation. Propagate errors and remove temporary files on failure.
A successful Save must survive reopening through Load with the exact payload;
cancelling first creation must leave no checkpoint. Do not change the Store or
Load APIs, add retry policies, or silently swallow errors.

Edit store.go and add regression tests in new *_test.go files. Preserve existing
tests and go.mod. Run go test ./... and get a fresh independent review. Report
what changed, checks, and remaining gaps. Permission preservation and power-loss
durability after rename are outside this fixture's acceptance; do not claim
those from the cancellation test. Do not push or add dependencies.
