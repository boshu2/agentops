---
paths:
  - "**/*.py"
---

# Python Conventions

> Canonical source with full examples: `skills/domain/references/standards/python.md`
> This file is kept self-contained for sessions that don't invoke skills.

## Formatting and Linting

- **Black** formatter with 100-character line length.
- **ruff** linter (`ruff check` must pass).
- **mypy** for type checking.

## Style

- Type hints on all public functions.
- Docstrings on all public classes and functions.
- pytest for testing; `conftest.py` for shared fixtures.

## Error Handling

- Never bare `except:` -- always specify the exception type.
- Use `raise ... from e` to preserve stack traces.
- Catch specific exceptions, not `except Exception:`.

## Security

- Never use `eval()`, `exec()`, or `__import__()` with untrusted input.
- Use `secrets` module for tokens, not `random`.
- Validate all external input.

## Testing (Risk-Based)

**Pick the test by risk, not by level.** Start from the acceptance behavior and write the narrowest test that fails when it is wrong: L1 unit for dense logic and edge cases, L2 integration when a boundary (module entry point, file or network I/O) is likely to hide the defect. Add a level only for a distinct risk; L2 is not required by default. See `skills/domain/references/standards/test-pyramid.md` for the selection questions.

- Assert exact expected values (`== expected`), not just `!= wrong`.
- Mock external services, not internal code.
- Add structural invariant tests when adding fields to dataclasses/models.
- **Prefer L2 integration tests** that call module entry points over L1 tests that mock internal collaborators.
