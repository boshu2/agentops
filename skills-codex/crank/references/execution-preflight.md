# Execution Preflight

Preflight proves that one wave of the accepted behavioral leaf is ready. It does
not bootstrap an orchestration substrate, search for work outside the leaf, or
create bookkeeping artifacts by habit.

## Required inputs

- exact leaf/bead or execution-packet identity;
- accepted plan and bound Premortem verdict;
- next failing acceptance proof;
- declared write scope, dependencies, risk class, and rollback;
- stable `run_id` for evidence correlation; and
- repository isolation required by local policy.

Missing or mismatched inputs return `BLOCKED` evidence to RPI. Crank does not
repair the packet, create lifecycle control state, or select a different objective.

## Routine route

1. Resolve the accepted leaf through the repository's tracker adapter. If no
   tracker exists, use the supplied packet/description without inventing state.
2. Confirm the branch/worktree is isolated and the declared paths match the
   next wave.
3. Confirm the Premortem-bound acceptance, dependencies, write scope, and risk
   still match. A material delta returns to Discovery; unchanged inputs reuse the
   verdict.
4. Select one direct implementer. The current writer may execute the wave. Do
   not start NTM, ATM, Swarm, a council, or another agent merely because it is
   available.
5. Use parallel dispatch only when the plan names at least two disjoint write
   scopes with separate owners, integration order, and discard paths.
6. Load only directly relevant prior evidence already cited by the plan or a
   concrete current failure. Broad lookup, metrics, ratchet, and archive scans
   stay off the critical path.

## Test-first classification

Behavior-changing code needs a right-reason RED acceptance proof before GREEN.
Docs-only, pure-refactor, or explicitly accepted `--no-test-first` work records
an honest pre-change baseline. Test level follows the changed behavior; e2e is
not mandatory when a lower level proves the contract.

## Output

Return a compact dispatch packet with leaf, wave, write scope, acceptance command,
`metadata.issue_type`, validation metadata, author/owner, base SHA, and stable
run ID. [wave-dispatch.md](wave-dispatch.md) owns the execution handoff;
preflight creates no counter or authorization state.
