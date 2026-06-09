# Tools and Signals

Use these to accelerate audits. Favor primary tooling for the stack.

## General
- rg: fast text search (entry points, error handling, TODOs)
- ast-grep: structural matching for refactors and pattern checks
- ubs: bug scanner for Rust codebases

## Security
- cargo audit / npm audit (dependency CVEs)
- rg patterns for secrets and injection primitives
- ubs / clippy for unsafe patterns

## UX / Accessibility
- Lighthouse (accessibility score)
- axe-core (a11y issues)
- Manual keyboard navigation and screen reader checks

## Performance
- profilers (perf, flamegraphs, pprof, chrome devtools)
- query logs for N+1 detection
- bundle analyzers for frontend payloads

## API
- OpenAPI/Swagger validation
- contract tests for request/response shapes
- schema validators (zod, joi, serde)

## Copy
- spell/grammar checks
- UX writing lint rules
- consistent tone guidelines

## CLI
- --help coverage review
- shellcheck for bash scripts
- snapshot tests for output
