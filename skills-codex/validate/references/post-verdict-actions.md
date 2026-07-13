# Post-Verdict Boundary

Validate has no post-verdict execution phase. After writing the immutable
verdict it returns exactly:

- pinned artifact identity;
- PASS, WARN, or FAIL;
- deterministic commands and evidence;
- findings and structured observations;
- `not_checked` coverage;
- one suggested owner and next action.

The caller decides what happens next. Learn may consume the verdict and its
digest for bookkeeping. A producer may repair a FAIL. A repository-specific
delivery process may publish a PASS. None of those actions belong to Validate,
and none can rewrite the existing verdict.
