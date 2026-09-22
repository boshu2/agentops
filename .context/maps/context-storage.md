# Context storage and routing

Use this map when changing context placement or the `ao config context` route.
Read the linked owners before editing; this page is navigation, not policy.

| Need | Source owner and next check |
|---|---|
| Page placement and admission | [ADR-0016](../../docs/adr/ADR-0016-state-tiers.md), active project-context amendment; [Memory](../../skills/memory/SKILL.md) routes find, capture and curate. |
| Command inputs and output | [NewContextCommand](../../cli/internal/commands/config/context.go); composition is checked by [TestContextJudgmentsComposedPreflight](../../cli/cmd/ao/context_judgments_composition_test.go). |
| Configuration and route decision | [LoadContext](../../cli/internal/config/context.go) owns precedence. [ResolveContext and validateRoute](../../cli/internal/config/context_route.go) bind policy, canonical paths and the native maintenance anchor. |
| Healthy and rejected routes | [context_test.go](../../cli/internal/config/context_test.go): `TestContextPrecedenceAndSameAnchorRecovery`, `TestContextDeniedBeforeNativeRead` and `TestContextNonGitRoots`. These run against both external and direct project bundles; `TestContextProjectBundleBoundaries` checks refused project roots and missing selection. |

`validateRoute` permits an explicitly bound canonical direct
`<consumer>/.context` bundle and retains external-bundle support. Other bundle
roots overlapping the consumer still fail, including the consumer itself,
ancestors and other or nested children. Policy paths must name canonical
destinations; aliases cannot escape the selected policy. No root is inferred or
created. The command still binds source, project, owner, task, model, destination,
policy and the native maintenance anchor. It requires native BD; ordinary
filesystem reading of selected cleared project pages does not. Do not remove the
separate staging/evidence protections in
[evidencepath.Validate](../../cli/internal/evidencepath/root.go) to change bundle placement.

From the repository root, inspect the route and run the existing focused checks:

```bash
rg -n 'func (ResolveContext|validateRoute|preflightRoute)|overlap|evidencepath.Validate' cli/internal/config/context_route.go
(cd cli && go test ./internal/config -run '^TestContext' -count=1)
```

These tests use a synthetic gateway. The separate
[installed-BD recovery test](../../cli/internal/commands/config/context_native_test.go)
is opt-in with `AO_TEST_BD_NATIVE=1`; it creates an isolated synthetic store and
does not make a unit-test pass proof of installed BD behavior.

Correction from the map at Git revision `2db882363`: the earlier statement
that all consumer-overlapping bundles fail described the then-current blanket
rejection. The direct project-bundle exception and the linked boundary tests
supersede that limitation. External draft/evidence protection remains unchanged.

This map records source behavior, not measured reuse benefit or privacy
protection. Recheck it when the linked placement contract, command or route tests
change; these checks establish route behavior, not admission of any particular knowledge page.
