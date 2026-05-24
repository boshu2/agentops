{{ define "standards-brief" }}
## Standards Brief

Full standards live in the `.claude/skills/standards/` corpus (shipped via the
overlay). The load-bearing rules:

### Go
- Cyclomatic complexity: warn at 15, fail at 25. Run `golangci-lint run`.
- Before committing: `cd cli && go build ./... && go vet ./... && go test ./...`.
- Always check errors; wrap with `fmt.Errorf("doing X: %w", err)`; use
  `errors.Is(err, target)`. Accept interfaces, return structs.
- Test naming `<source>_test.go` / `Test<Uppercase>`; never `cov*_test.go`. No
  coverage-padding or zero-assertion smoke tests. Assert exact values.

### Python
- Black (100-col), ruff, mypy. Type hints + docstrings on public surfaces.
- Never bare `except:`; `raise ... from e`; catch specific exceptions.
- `secrets` (not `random`) for tokens; never `eval`/`exec` on untrusted input.

### Test shape (AI-native)
**L2 first, L1 always.** Write L2 integration tests (where bugs are found) first,
then L1 unit tests for regression safety. Prefer integration tests that call entry
points over unit tests that mock internal collaborators.

### Hard CI rules
- **No symlinks** anywhere in the repo — copy shared files instead.
- Every `references/*.md` must be linked from its `SKILL.md`.
- No TODOs in `SKILL.md` (use `bd` instead). No hardcoded secrets.
{{ end }}
