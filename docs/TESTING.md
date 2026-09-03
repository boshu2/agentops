# Testing

Use the narrowest deterministic check that proves the changed behavior during an
experiment. Run the repository's ordinary full deterministic suite once on the
complete candidate.

## Core contract checks

```bash
python3 -m unittest discover -s skills/validate/tests
./scripts/check-cathedral-cut-conformance.py
python3 scripts/generate-skill-mesh.py --check
cd cli && go test ./...
```

The Validate helper is also probed in a temporary non-Git directory with no
`ao` on `PATH`. Fake `git`, `ao`, tracker, push, and delivery executables ensure
identity and verdict helpers do not acquire hidden substrate dependencies.

`ao gate check` is an ordinary test runner. Its exit status is not a semantic
verdict and does not authorize delivery.

## Explicit skill requests

`tests/explicit-skill-requests/` holds one prompt per shipped skill that a
caller invokes by name (`prompts/<skill>.txt`). `run-all.sh` feeds each prompt
to a fresh session and asserts the named skill is what fires, before any tool
call. The static half runs without a model:
`evals/agentops-core/fixtures/explicit-skill-prompts-smoke.sh` proves every
prompt names a skill that exists in both `skills/` and `skills-codex/`, so a
retired skill cannot linger as a live prompt.
