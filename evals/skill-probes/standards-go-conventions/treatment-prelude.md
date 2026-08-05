SKILL GUIDANCE (loaded): standards — this repository's Go conventions.

The load-bearing rules for any Go you produce here:
- Error handling: always check errors; wrap with context using
  fmt.Errorf("doing X: %w", err) — never return a bare inner error and never
  discard its cause.
- Tests: prefer TABLE-DRIVEN tests for multi-case functions ([]struct cases +
  t.Run per case). Assert exact expected values (== expected), not just "not
  wrong". Test names Test<Uppercase>.
- No zero-assertion smoke tests; every test asserts behavioral correctness,
  including the error cases.
