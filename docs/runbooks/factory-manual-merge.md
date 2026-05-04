# Factory Manual Merge Runbook

Factory coding lanes produce reviewable worktree branches. They do not merge
automatically in milestone 1.

## Preconditions

- Routing policy has `auto_merge_enabled: false`.
- Routing policy has `manual_merge_by_default: true`.
- The candidate lane has `merge_eligibility.validation_commands`.
- The factory projection shows the job in `pending_manual_merges`.
- The validation listed in `validations` has `status: "passed"`.

## Manual Merge

1. Inspect the pending merge entry in the factory projection:
   - `job_id`
   - `run_id`
   - `slot_id`
   - `manual_command`
   - `conflicts`
2. Inspect the owned worktree entry:
   - `worktree_id`
   - `path`
   - `branch`
   - `base_commit`
   - `merge_disposition`
3. Review validation evidence from `merge_eligibility.validation_commands`.
   Every listed command must have a matching passed validation record before
   merge review continues.
4. Review artifacts, diffs, transcripts, and logs from the projection pointer
   lists. Treat missing evidence as a failed gate.
5. Run the `manual_command` only after validation is green and conflicts are
   understood.
6. Record the outcome with a `factory.merge_decision` event:
   - `manual_merged` when the operator lands the candidate.
   - `rejected` when review fails.
   - `abandoned` when the candidate is no longer relevant.

## Validation Failure Recovery

Validation failure is terminal for that worker job. The worker must record a
`factory.job_terminal` event with `status: "failed"`,
`retained_worktree: true`, and artifact references for validation output,
diffs, logs, and transcripts.

Recovery steps:

1. Inspect `blocked_validations` for the failed `validation_id`, `commands`,
   `artifacts`, and `artifact_refs`.
2. Inspect `terminal_jobs` for the failed job and confirm
   `retained_worktree: true`.
3. Inspect `retained_failed_worktrees` and use the retained `path` for manual
   diagnosis.
4. Decide disposition:
   - `rejected` if the patch is not worth saving.
   - `abandoned` if the work is superseded.
   - a new factory job if the patch should be repaired in a fresh slot.
5. Do not delete retained worktrees until the disposition is recorded and the
   operator has copied any evidence needed outside local runtime state.

## Invariants

- Failed validation blocks merge.
- Automatic merge remains disabled unless a future promoted policy explicitly
  changes the contract.
- GasCity / Mt. Olympus production coding lanes remain disabled for milestone 1.
- Repo-root `.agents/` artifacts remain local runtime state and must not be
  reintroduced into git history.
