package archcheck

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestGoCLIArchitectureInducedFixtures(t *testing.T) {
	if err := SelfTest(); err != nil {
		t.Fatal(err)
	}
}

func TestSemanticEscapeClassifierPositiveAndNegativeFixtures(t *testing.T) {
	positivePath := "cli/internal/adapters/example/runner.go"
	positiveSource := []byte("package example\nimport (\"os/exec\"; _ \"github.com/boshu2/agentops/cli/internal/trackerresolve\")\nvar _ = exec.Command\n")
	if classes := ClassifySemanticEscapes(positivePath, positiveSource); len(classes) == 0 {
		t.Fatal("synthetic tracker-execution escape was not classified")
	}
	negativePath := "cli/internal/adapters/example/reader.go"
	negativeSource := []byte("package example\nimport \"os\"\nvar _ = os.ReadFile\n")
	if classes := ClassifySemanticEscapes(negativePath, negativeSource); len(classes) != 0 {
		t.Fatalf("read-only adapter classified as %v", classes)
	}
}

func TestRetiredFamilyDoesNotRequireUntrackedLiveOwnerDirectory(t *testing.T) {
	root := t.TempDir()
	ownership := ownershipRecord{LiveOwner: "cli/internal/commands/retired"}
	if violations := checkLiveOwner(root, "retired", ownership); len(violations) != 0 {
		t.Fatalf("retired family required an empty live-owner directory: %v", violations)
	}
	if violations := checkLiveOwner(root, "migrated", ownership); !hasRule(violations, RuleOwnership) {
		t.Fatalf("active family did not require its live owner: %v", violations)
	}
}

func TestGoCLIArchitectureFamilyOwnershipAndScope(t *testing.T) {
	root := t.TempDir()
	runFixtureGit(t, root, "init")
	runFixtureGit(t, root, "config", "user.email", "test@example.com")
	runFixtureGit(t, root, "config", "user.name", "Test")

	baseline := filepath.Join(root, "cli", "testdata", "compatibility-baseline", "families", "demo")
	moduleDir := filepath.Join(root, "cli", "internal", "commands", "demo")
	ownerDir := filepath.Join(root, "cli", "internal", "demo")
	for _, dir := range []string{baseline, moduleDir, ownerDir, filepath.Join(root, "cli", "cmd", "ao")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ownership := ownershipRecord{
		SchemaVersion: 1,
		Family:        "demo",
		Profiles:      map[string]string{"default": "present", "flywheel": "absent", "legacy": "absent", "combined": "present"},
		LegacySymbols: []string{"oldDemoCmd"},
		LiveOwner:     "cli/internal/demo",
		AllowedPaths: []string{
			"cli/internal/commands/demo/**",
			"cli/internal/demo/**",
			"cli/testdata/compatibility-baseline/families/demo/lineage.json",
		},
	}
	ownershipBytes, err := json.Marshal(ownership)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(baseline, "ownership.json"), ownershipBytes)
	writeFixture(t, filepath.Join(root, "README.md"), []byte("fixture\n"))
	writeFixture(t, filepath.Join(root, ".gitignore"), []byte(".agents/\n"))
	runFixtureGit(t, root, "add", ".")
	runFixtureGit(t, root, "commit", "-m", "freeze")
	freeze := fixtureGitOutput(t, root, "rev-parse", "HEAD")

	sum := sha256.Sum256(ownershipBytes)
	scopePath := filepath.Join(root, ".agents", "evidence", "family-demo-scope.json")
	scopeBytes, err := json.Marshal(Inventory{SchemaVersion: 1, Family: "demo", AllowedPaths: ownership.AllowedPaths})
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, scopePath, scopeBytes)
	scopeSum := sha256.Sum256(scopeBytes)
	lineage, err := json.Marshal(lineageRecord{
		SchemaVersion:   1,
		Family:          "demo",
		FreezeSHA:       freeze,
		OwnershipSHA256: hex.EncodeToString(sum[:]),
		ScopeSHA256:     hex.EncodeToString(scopeSum[:]),
		MigrationState:  "migrating",
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(baseline, "lineage.json"), lineage)
	writeFixture(t, filepath.Join(moduleDir, "module.go"), []byte(`package demo
import "github.com/boshu2/agentops/cli/internal/clicontract"
func NewModule() Module { return Module{} }
type Module struct{}
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{Profiles: clicontract.ProfileDefault | clicontract.ProfileCombined}
}
`))
	writeFixture(t, filepath.Join(ownerDir, "owner.go"), []byte("package demo\n"))
	runFixtureGit(t, root, "add", ".")
	runFixtureGit(t, root, "commit", "-m", "migrate")

	violations, err := Check(Options{Root: root, Family: "demo", VerifyScope: scopePath})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("valid family fixture: %v", violations)
	}
	violations, err = Check(Options{Root: root, AllMigrated: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("valid family escaped cumulative mode: %v", violations)
	}
	writeFixture(t, filepath.Join(root, "unrelated.txt"), []byte("active scope escape\n"))
	runFixtureGit(t, root, "add", ".")
	runFixtureGit(t, root, "commit", "-m", "active escape")
	violations, err = Check(Options{Root: root, AllMigrated: true})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(violations, RuleScope) {
		t.Fatalf("unsealed migration stopped enforcing freeze-to-HEAD scope: %v", violations)
	}
	if err := os.Remove(filepath.Join(root, "unrelated.txt")); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, "add", ".")
	runFixtureGit(t, root, "commit", "-m", "remove active escape")
	accepted := fixtureGitOutput(t, root, "rev-parse", "HEAD")
	lineage, err = json.Marshal(lineageRecord{
		SchemaVersion:   1,
		Family:          "demo",
		FreezeSHA:       freeze,
		OwnershipSHA256: hex.EncodeToString(sum[:]),
		ScopeSHA256:     hex.EncodeToString(scopeSum[:]),
		MigrationState:  "migrated",
		AcceptedSHA:     accepted,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(baseline, "lineage.json"), lineage)
	runFixtureGit(t, root, "add", ".")
	runFixtureGit(t, root, "commit", "-m", "seal migration")
	violations, err = Check(Options{Root: root, AllMigrated: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("valid sealed family: %v", violations)
	}
	parkedModule := filepath.Join(root, "parked-module")
	if err := os.Rename(moduleDir, parkedModule); err != nil {
		t.Fatal(err)
	}
	violations, err = Check(Options{Root: root, AllMigrated: true})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(violations, RuleOwnership) {
		t.Fatalf("cumulative mode forgot a deleted migrated module: %v", violations)
	}
	writeFixture(t, filepath.Join(baseline, "lineage.json"), []byte("{"))
	if _, err := Check(Options{Root: root, AllMigrated: true}); err == nil {
		t.Fatal("cumulative mode skipped deleted module after lineage corruption")
	}
	writeFixture(t, filepath.Join(baseline, "lineage.json"), lineage)
	if err := os.Rename(parkedModule, moduleDir); err != nil {
		t.Fatal(err)
	}
	parkedBaseline := filepath.Join(root, "parked-baseline")
	if err := os.Rename(moduleDir, parkedModule); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(baseline, parkedBaseline); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(Options{Root: root, AllMigrated: true}); err == nil {
		t.Fatal("cumulative mode forgot a family after both module and baseline were deleted")
	}
	if err := os.Rename(parkedBaseline, baseline); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(parkedModule, moduleDir); err != nil {
		t.Fatal(err)
	}

	writeFixture(t, filepath.Join(baseline, "ownership.json"), append(ownershipBytes, '\n'))
	violations, err = Check(Options{Root: root, Family: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(violations, RuleOwnership) {
		t.Fatalf("tampered ownership digest was not rejected: %v", violations)
	}
	writeFixture(t, filepath.Join(baseline, "ownership.json"), ownershipBytes)

	mutatedOwnership := append([]byte(nil), ownershipBytes...)
	mutatedOwnership = append(mutatedOwnership, '\n')
	mutatedSum := sha256.Sum256(mutatedOwnership)
	mutatedLineage, err := json.Marshal(lineageRecord{
		SchemaVersion:   1,
		Family:          "demo",
		FreezeSHA:       freeze,
		OwnershipSHA256: hex.EncodeToString(mutatedSum[:]),
		ScopeSHA256:     hex.EncodeToString(scopeSum[:]),
		MigrationState:  "migrated",
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(baseline, "ownership.json"), mutatedOwnership)
	writeFixture(t, filepath.Join(baseline, "lineage.json"), mutatedLineage)
	violations, err = Check(Options{Root: root, Family: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(violations, RuleOwnership) {
		t.Fatalf("coherent ownership+lineage rewrite escaped frozen Git binding: %v", violations)
	}
	writeFixture(t, filepath.Join(baseline, "ownership.json"), ownershipBytes)
	writeFixture(t, filepath.Join(baseline, "lineage.json"), lineage)

	tamperedScopeBytes, err := json.Marshal(Inventory{SchemaVersion: 1, Family: "demo", AllowedPaths: append(append([]string(nil), ownership.AllowedPaths...), "unrelated/**")})
	if err != nil {
		t.Fatal(err)
	}
	tamperedScopeSum := sha256.Sum256(tamperedScopeBytes)
	tamperedScopeLineage, err := json.Marshal(lineageRecord{
		SchemaVersion:   1,
		Family:          "demo",
		FreezeSHA:       freeze,
		OwnershipSHA256: hex.EncodeToString(sum[:]),
		ScopeSHA256:     hex.EncodeToString(tamperedScopeSum[:]),
		MigrationState:  "migrated",
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, scopePath, tamperedScopeBytes)
	writeFixture(t, filepath.Join(baseline, "lineage.json"), tamperedScopeLineage)
	violations, err = Check(Options{Root: root, Family: "demo", VerifyScope: scopePath})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(violations, RuleScope) {
		t.Fatalf("coherent scope+lineage rewrite escaped ownership binding: %v", violations)
	}
	writeFixture(t, scopePath, scopeBytes)
	writeFixture(t, filepath.Join(baseline, "lineage.json"), lineage)

	writeFixture(t, filepath.Join(moduleDir, "module.go"), []byte(`package demo
import (
	"github.com/boshu2/agentops/cli/internal/clicontract"
	fake "example.com/not-clicontract"
)
func NewModule() Module { return Module{} }
type Module struct{}
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{Profiles: clicontract.ProfileDefault}
}
var deadSpoof = fake.CommandContract{Profiles: fake.ProfileDefault | fake.ProfileCombined}
`))
	violations, err = Check(Options{Root: root, Family: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(violations, RuleProfileReachability) {
		t.Fatalf("wrong profile membership was not rejected: %v", violations)
	}
	writeFixture(t, filepath.Join(moduleDir, "module.go"), []byte(`package demo
import "github.com/boshu2/agentops/cli/internal/clicontract"
type Module struct{}
func (Module) NewModule() Module { return Module{} }
func (Module) Contract() clicontract.CommandContract {
	_ = clicontract.CommandContract{Profiles: clicontract.ProfileDefault | clicontract.ProfileCombined}
	return makeWrongContract()
}
func makeWrongContract() clicontract.CommandContract { return clicontract.CommandContract{} }
`))
	violations, err = Check(Options{Root: root, Family: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(violations, RuleOwnership) || !hasRule(violations, RuleProfileReachability) {
		t.Fatalf("method constructor/dead contract literal escaped: %v", violations)
	}
	writeFixture(t, filepath.Join(moduleDir, "module.go"), []byte(`package demo
import "github.com/boshu2/agentops/cli/internal/clicontract"
type Module struct{}
func NewModule() int { return 0 }
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{Profiles: choose(clicontract.ProfileDefault, clicontract.ProfileCombined)}
}
func choose(left, _ clicontract.ProfileSet) clicontract.ProfileSet { return left }
`))
	violations, err = Check(Options{Root: root, Family: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(violations, RuleOwnership) || !hasRule(violations, RuleProfileReachability) {
		t.Fatalf("wrong constructor result/computed profile mask escaped: %v", violations)
	}
	writeFixture(t, filepath.Join(moduleDir, "module.go"), []byte(`package demo
import "github.com/boshu2/agentops/cli/internal/clicontract"
func NewModule() Module { return Module{} }
type Module struct{}
func (Module) Contract(_ int) any {
	return clicontract.CommandContract{Profiles: clicontract.ProfileDefault | clicontract.ProfileCombined}
}
`))
	violations, err = Check(Options{Root: root, Family: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(violations, RuleProfileReachability) {
		t.Fatal("wrong Contract signature escaped profile ownership check")
	}
	writeFixture(t, filepath.Join(moduleDir, "module.go"), []byte(`package demo
import "github.com/boshu2/agentops/cli/internal/clicontract"
func NewModule() Module { return Module{} }
type Module struct{}
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{Profiles: clicontract.ProfileDefault | clicontract.ProfileCombined}
}
`))

	legacyPath := filepath.Join(root, "cli", "cmd", "ao", "legacy.go")
	writeFixture(t, legacyPath, []byte("package main\nvar oldDemoCmd any\n"))
	violations, err = Check(Options{Root: root, Family: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(violations, RuleLegacySymbol) {
		t.Fatalf("legacy symbol was not rejected: %v", violations)
	}
	if err := os.Remove(legacyPath); err != nil {
		t.Fatal(err)
	}

	writeFixture(t, filepath.Join(root, "unrelated.txt"), []byte("later family work\n"))
	runFixtureGit(t, root, "add", ".")
	runFixtureGit(t, root, "commit", "-m", "later work")
	violations, err = Check(Options{Root: root, Family: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if hasRule(violations, RuleScope) {
		t.Fatalf("sealed family blocked later unrelated work: %v", violations)
	}
}

func TestGoCLIArchitectureOwnershipProfilesAreExact(t *testing.T) {
	valid := ownershipRecord{
		SchemaVersion: 1,
		Family:        "demo",
		LiveOwner:     "cli/internal/demo",
		AllowedPaths:  []string{"cli/internal/commands/demo/**"},
		Profiles:      map[string]string{"default": "present", "flywheel": "absent", "legacy": "absent", "combined": "present"},
	}
	if !validOwnershipRecord(valid, "demo") {
		t.Fatal("valid exact ownership record rejected")
	}
	for name, mutate := range map[string]func(map[string]string){
		"missing": func(profiles map[string]string) { delete(profiles, "legacy") },
		"extra":   func(profiles map[string]string) { profiles["debug"] = "absent" },
		"value":   func(profiles map[string]string) { profiles["legacy"] = "maybe" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			candidate.Profiles = map[string]string{"default": "present", "flywheel": "absent", "legacy": "absent", "combined": "present"}
			mutate(candidate.Profiles)
			if validOwnershipRecord(candidate, "demo") {
				t.Fatalf("malformed profiles accepted: %v", candidate.Profiles)
			}
		})
	}
}

func TestGoCLIArchitectureAcceptedBoundaryOwnsModuleIntroduction(t *testing.T) {
	for _, moduleTiming := range []string{"freeze", "after-seal"} {
		t.Run(moduleTiming, func(t *testing.T) {
			root := t.TempDir()
			runFixtureGit(t, root, "init")
			runFixtureGit(t, root, "config", "user.email", "test@example.com")
			runFixtureGit(t, root, "config", "user.name", "Test")
			baseline := filepath.Join(root, "cli", "testdata", "compatibility-baseline", "families", "demo")
			modulePath := filepath.Join(root, "cli", "internal", "commands", "demo", "module.go")
			ownerPath := filepath.Join(root, "cli", "internal", "demo", "owner.go")
			ownership := ownershipRecord{
				SchemaVersion: 1,
				Family:        "demo",
				Profiles:      map[string]string{"default": "present", "flywheel": "absent", "legacy": "absent", "combined": "present"},
				LiveOwner:     "cli/internal/demo",
				AllowedPaths: []string{
					"cli/internal/commands/demo/**",
					"cli/internal/demo/**",
					"cli/testdata/compatibility-baseline/families/demo/lineage.json",
				},
			}
			ownershipBytes, err := json.Marshal(ownership)
			if err != nil {
				t.Fatal(err)
			}
			writeFixture(t, filepath.Join(baseline, "ownership.json"), ownershipBytes)
			if moduleTiming == "freeze" {
				writeFixture(t, modulePath, validDemoModule())
			}
			runFixtureGit(t, root, "add", ".")
			runFixtureGit(t, root, "commit", "-m", "freeze")
			freeze := fixtureGitOutput(t, root, "rev-parse", "HEAD")
			sum := sha256.Sum256(ownershipBytes)
			lineage, err := json.Marshal(lineageRecord{SchemaVersion: 1, Family: "demo", FreezeSHA: freeze, OwnershipSHA256: hex.EncodeToString(sum[:]), MigrationState: "migrating"})
			if err != nil {
				t.Fatal(err)
			}
			writeFixture(t, filepath.Join(baseline, "lineage.json"), lineage)
			writeFixture(t, ownerPath, []byte("package demo\n"))
			runFixtureGit(t, root, "add", ".")
			runFixtureGit(t, root, "commit", "-m", "accepted candidate")
			accepted := fixtureGitOutput(t, root, "rev-parse", "HEAD")
			lineage, err = json.Marshal(lineageRecord{SchemaVersion: 1, Family: "demo", FreezeSHA: freeze, OwnershipSHA256: hex.EncodeToString(sum[:]), MigrationState: "migrated", AcceptedSHA: accepted})
			if err != nil {
				t.Fatal(err)
			}
			writeFixture(t, filepath.Join(baseline, "lineage.json"), lineage)
			runFixtureGit(t, root, "add", ".")
			runFixtureGit(t, root, "commit", "-m", "seal")
			if moduleTiming == "after-seal" {
				writeFixture(t, modulePath, validDemoModule())
				runFixtureGit(t, root, "add", ".")
				runFixtureGit(t, root, "commit", "-m", "late module")
			}
			violations, err := Check(Options{Root: root, Family: "demo"})
			if err != nil {
				t.Fatal(err)
			}
			if !hasRule(violations, RuleOwnership) {
				t.Fatalf("module introduced at %s escaped accepted boundary: %v", moduleTiming, violations)
			}
		})
	}
}

func validDemoModule() []byte {
	return []byte(`package demo
import "github.com/boshu2/agentops/cli/internal/clicontract"
func NewModule() Module { return Module{} }
type Module struct{}
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{Profiles: clicontract.ProfileDefault | clicontract.ProfileCombined}
}
`)
}

func TestGoCLIArchitectureInventoryIsDeterministicAndLiteral(t *testing.T) {
	root := t.TempDir()
	runFixtureGit(t, root, "init")
	runFixtureGit(t, root, "config", "user.email", "test@example.com")
	runFixtureGit(t, root, "config", "user.name", "Test")
	writeFixture(t, filepath.Join(root, "cli", "cmd", "ao", "beads.go"), []byte(`package main
import (
	"os"
	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)
var beadsCmd any
func runBeads() { _, _ = os.ReadFile("x"); _ = trackerresolve.BR }
`))
	runFixtureGit(t, root, "add", ".")
	runFixtureGit(t, root, "commit", "-m", "inventory")

	first, err := BuildInventory(root, "beads")
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildInventory(root, "beads")
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("inventory is nondeterministic:\n%s\n%s", firstJSON, secondJSON)
	}
	if !slices.Contains(first.OwnerFiles, "cli/cmd/ao/beads.go") || !slices.Contains(first.LegacySymbols, "runBeads") {
		t.Fatalf("inventory omitted literal owner/symbol: %+v", first)
	}
	if !slices.Contains(first.Effects, RuleFilesystem) || !slices.Contains(first.Effects, RuleTracker) {
		t.Fatalf("inventory omitted observed effects: %+v", first.Effects)
	}
	if !slices.Contains(first.OwnerCandidates, "cli/internal/trackerresolve/**") {
		t.Fatalf("inventory omitted internal owner candidate: %+v", first.OwnerCandidates)
	}
}

func writeFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func runFixtureGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func fixtureGitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(bytesTrimSpace(output))
}

func bytesTrimSpace(data []byte) []byte {
	start, end := 0, len(data)
	for start < end && (data[start] == ' ' || data[start] == '\n' || data[start] == '\r' || data[start] == '\t') {
		start++
	}
	for end > start && (data[end-1] == ' ' || data[end-1] == '\n' || data[end-1] == '\r' || data[end-1] == '\t') {
		end--
	}
	return data[start:end]
}
