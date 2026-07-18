package gates

// GoCLIArchitectureCheck is shared by the registry and its induced routing
// tests so the release authority cannot drift from the tested path set.
func GoCLIArchitectureCheck() Check {
	return Check{
		ID:       "go.cli-architecture",
		Tiers:    Fast | Full,
		Blocking: true,
		Backing:  "check-go-cli-architecture.sh",
		Match: []string{
			"cli/internal/commands/**",
			"cli/internal/adapters/**",
			"cli/internal/clicontract/**",
			"cli/cmd/ao/**",
			"cli/testdata/compatibility-baseline/families/**",
			"cli/internal/archcheck/**",
			"scripts/check-go-cli-architecture.sh",
			"tests/go_cli_architecture.bats",
		},
		RepairHint: "move direct effects behind a narrow port/adapter, remove legacy ownership, then rerun bash scripts/check-go-cli-architecture.sh",
	}
}
