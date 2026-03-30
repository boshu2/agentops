# Four-Surface Closure Validation

> From spec-as-leverage-point analysis and 26 uncaptured retro lessons.
> The #1 post-mortem gap: validating code was changed but not that all surfaces were updated.

## The Four Surfaces

Every completed feature or fix touches up to four surfaces. Post-mortem must validate ALL four, not just code:

### Surface 1: Code
- Implementation matches the spec/plan
- Tests pass (L0-L3 as appropriate)
- No regressions introduced
- Complexity budget respected

### Surface 2: Documentation
- README/docs updated if user-facing behavior changed
- API docs reflect new endpoints/parameters
- SKILL.md updated if skill behavior changed
- CHANGELOG entry added for releases
- Inline comments updated where logic changed

### Surface 3: Examples
- Usage examples updated to reflect new behavior
- Tutorial/guide code samples still work
- CLI help text reflects new flags/commands
- Error messages are accurate and helpful

### Surface 4: Proof
- Acceptance criteria from the plan are satisfied (runnable gates pass)
- Test coverage exists for the new behavior (not just existing tests still pass)
- CI/CD pipeline reflects the change (if infrastructure was modified)
- Security scan passes if security-sensitive code was changed
- Performance benchmarks pass if performance-critical code was changed

## Closure Checklist

During post-mortem Phase 1 (council validation), each judge should check:

| Surface | Check | Pass Criteria |
|---------|-------|---------------|
| Code | `git diff --stat` matches planned scope | All planned files modified, no unplanned files |
| Code | Test suite passes | Zero failures |
| Docs | Relevant docs updated | No stale references to old behavior |
| Docs | Skill counts synced (if skills changed) | `validate-doc-release.sh` passes |
| Examples | Usage examples tested | Examples produce expected output |
| Examples | CLI help reflects changes | `--help` output matches implementation |
| Proof | Acceptance criteria gates run | All gates return 0 |
| Proof | New test coverage exists | New behavior has dedicated tests |
| Proof | CI pipeline updated (if needed) | Pipeline runs new checks |

## Common Gaps (from 26 uncaptured retro lessons)

1. **Code ships, docs don't** -- implementation complete but README still describes old behavior
2. **Tests pass, proof missing** -- existing tests pass but no new test covers the new feature
3. **Examples rot** -- code samples in docs reference removed functions or old APIs
4. **Skill counts drift** -- skill added/removed but `sync-skill-counts.sh` not run
5. **CLI docs stale** -- new flag added but `generate-cli-reference.sh` not run
6. **Proof by assertion** -- "it works" without a runnable command demonstrating it

## Integration with Post-Mortem Phases

### Phase 1 (Council Validation)
Add four-surface check to judge instructions:
> Verify all four surfaces are addressed: Code, Documentation, Examples, and Proof. A PASS verdict requires all four surfaces validated, not just code correctness.

### Phase 2 (Learning Extraction)
Extract learnings from any surface gaps found:
> If a surface was missed, create a learning: "Feature X shipped without [surface] update -- add to pre-mortem checklist."

### Phase 5 (Retirement)
Retire learnings about surface gaps that now have mechanical enforcement:
> If a CI check now catches the gap (e.g., doc-release-gate catches skill count drift), retire the learning and reference the gate.
