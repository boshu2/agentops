// practices: [continuous-delivery, supply-chain-integrity]
package version

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

const testVersion = "9.9.9-test"

func newTestModule(outputMode string) Module {
	return NewModule(clicontract.HostOptions{
		Version:    func() string { return testVersion },
		OutputMode: func() string { return outputMode },
	})
}

func run(t *testing.T, outputMode string) string {
	t.Helper()
	command := newTestModule(outputMode).Command()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatalf("ao version returned error: %v", err)
	}
	return out.String()
}

func TestModule_Contract(t *testing.T) {
	contract := newTestModule("text").Contract()
	if contract.ID != "ao.version" {
		t.Fatalf("contract ID = %q, want ao.version", contract.ID)
	}
	if contract.Output != clicontract.OutputText {
		t.Fatalf("output = %v, want OutputText", contract.Output)
	}
	if contract.Effects != clicontract.EffectPure {
		t.Fatalf("effects = %v, want EffectPure", contract.Effects)
	}
	if contract.Args.Name != "arbitrary" {
		t.Fatalf("args = %q, want arbitrary", contract.Args.Name)
	}
}

func TestModule_CommandAttributes(t *testing.T) {
	command := newTestModule("text").Command()
	if command.Use != "version" {
		t.Errorf("Use = %q, want version", command.Use)
	}
	if command.GroupID != "core" {
		t.Errorf("GroupID = %q, want core", command.GroupID)
	}
}

func TestVersion_ExecuteOutputContainsVersionString(t *testing.T) {
	out := run(t, "text")
	if !strings.Contains(out, "ao version "+testVersion) {
		t.Errorf("output should contain 'ao version %s', got: %s", testVersion, out)
	}
}

func TestVersion_ExecuteOutputContainsGoVersion(t *testing.T) {
	out := run(t, "text")
	goVer := runtime.Version()
	if !strings.Contains(out, goVer) {
		t.Errorf("output should contain Go version %q, got: %s", goVer, out)
	}
}

func TestVersion_ExecuteOutputContainsPlatform(t *testing.T) {
	out := run(t, "text")
	platform := runtime.GOOS + "/" + runtime.GOARCH
	if !strings.Contains(out, platform) {
		t.Errorf("output should contain platform %q, got: %s", platform, out)
	}
}

func TestVersion_ExecuteOutputLineCount(t *testing.T) {
	out := run(t, "text")
	// The version command outputs exactly 3 lines:
	//   ao version <ver>
	//   Go version: <ver>
	//   Platform: <os>/<arch>
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 output lines, got %d: %q", len(lines), out)
	}
}

func TestVersion_JSONOutputEncodesInfo(t *testing.T) {
	out := run(t, "json")
	for _, want := range []string{`"version": "` + testVersion + `"`, `"go_version"`, `"platform"`} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q, got: %s", want, out)
		}
	}
}
