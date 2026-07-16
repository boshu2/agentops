package archcheck

import "fmt"

// SelfTest proves every enforced dimension with an induced violation and pins
// the adjacent false-positive boundaries (comments, strings, tests, adapters).
func SelfTest() error {
	cases := []struct {
		name   string
		source string
		rule   Rule
	}{
		{"process", `package demo; import "os/exec"; func run(){ _ = exec.Command("x") }`, RuleProcess},
		{"process-exit", `package demo; import "os"; func run(){ os.Exit(1) }`, RuleProcess},
		{"filesystem", `package demo; import "os"; func run(){ _, _ = os.ReadFile("x") }`, RuleFilesystem},
		{"filesystem-path", `package demo; import "path/filepath"; func run(){ _, _ = filepath.EvalSymlinks("x") }`, RuleFilesystem},
		{"filesystem-dot-import", `package demo; import . "path/filepath"; func run(){ _, _ = EvalSymlinks("x") }`, RuleFilesystem},
		{"environment", `package demo; import "os"; func run(){ _ = os.Getenv("X") }`, RuleEnvironment},
		{"network", `package demo; import "net/http"; func run(){ _, _ = http.Get("https://example.invalid") }`, RuleNetwork},
		{"tracker", `package demo; import "github.com/boshu2/agentops/cli/internal/trackerresolve"; var _ = trackerresolve.BR`, RuleTracker},
		{"clock", `package demo; import "time"; func run(){ _ = time.Now() }`, RuleClock},
		{"context-background", `package demo; import "context"; func run(){ _ = context.Background() }`, RuleContext},
		{"context-todo", `package demo; import ctx "context"; func run(){ _ = ctx.TODO() }`, RuleContext},
		{"service-bag", `package demo; type Dependencies struct { Any any }`, RuleServiceBag},
		{"composition", `package demo; import "github.com/boshu2/agentops/cli/internal/composition"; var _ = composition.Build`, RuleCompositionImport},
		{"concrete-adapter", `package demo; import "github.com/boshu2/agentops/cli/internal/adapters/tracker_br"; var _ = tracker_br.New`, RuleConcreteAdapter},
	}
	for _, test := range cases {
		violations, err := CheckSource("cli/internal/commands/demo/module.go", []byte(test.source))
		if err != nil {
			return fmt.Errorf("self-test %s: %w", test.name, err)
		}
		if !hasRule(violations, test.rule) {
			return fmt.Errorf("self-test %s: induced %s violation was not caught: %v", test.name, test.rule, violations)
		}
	}

	clean := `package demo
// exec.Command, os.ReadFile, os.Getenv, http.Get, time.Now are examples only.
const documentation = "exec.Command os.ReadFile os.Getenv http.Get time.Now trackerresolve"
func NewModule(port NarrowPort) Module { return Module{} }
type NarrowPort interface { Run() error }
type Module struct{}
`
	violations, err := CheckSource("cli/internal/commands/demo/module.go", []byte(clean))
	if err != nil {
		return err
	}
	if len(violations) != 0 {
		return fmt.Errorf("self-test comments/strings/narrow port false positive: %v", violations)
	}

	raw := `package raw_exec; import "os/exec"; func Run(){ _ = exec.Command("x") }`
	violations, err = CheckSource("cli/internal/adapters/raw_exec/adapter.go", []byte(raw))
	if err != nil {
		return err
	}
	if len(violations) != 0 {
		return fmt.Errorf("self-test declared adapter false positive: %v", violations)
	}
	violations, err = CheckSource("cli/internal/commands/demo/module_test.go", []byte(cases[0].source))
	if err != nil {
		return err
	}
	if len(violations) != 0 {
		return fmt.Errorf("self-test test-file false positive: %v", violations)
	}
	return nil
}

func hasRule(violations []Violation, rule Rule) bool {
	for _, violation := range violations {
		if violation.Rule == rule {
			return true
		}
	}
	return false
}
