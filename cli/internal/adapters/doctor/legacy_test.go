package doctor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
	"github.com/boshu2/agentops/cli/internal/quality"
)

func TestLegacyChecksIncludesEveryDoctorSafetySection(t *testing.T) {
	root := t.TempDir()
	writeFakeAgentopsRepo(t, root)
	home := t.TempDir()
	linkFixtureSkills(t, filepath.Join(root, "skills"), filepath.Join(home, ".agents", "skills"))
	adapter := LegacyChecks{
		ToolVersion: "1.0.0",
		WorkingDir:  func() (string, error) { return root, nil },
		HomeDir:     func() (string, error) { return home, nil },
		LedgerPath:  func() string { return filepath.Join(root, "ledger.jsonl") },
		Environment: func() []string { return nil },
		Now:         time.Now,
	}
	checks := adapter.Checks(context.Background())
	byName := make(map[string]string, len(checks))
	for _, check := range checks {
		byName[check.Name] = check.Status
	}
	for _, name := range []string{"Skill Links", "Binary Freshness", "Ledger Health", "LAW-0 Guard"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("LegacyChecks missing %q: %v", name, byName)
		}
	}
	for _, removed := range []string{"Plugin", "Codex Sync", "CLI Dependencies", "Reviewer: codex", "Cross-Family Review", "Search Index", "Flywheel Health"} {
		if _, ok := byName[removed]; ok {
			t.Errorf("retired doctor concern %q returned: %v", removed, byName)
		}
	}
}

func writeFakeAgentopsRepo(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "cli", "cmd", "ao")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cli", "go.mod"), []byte(agentopsModuleLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nvar version = \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"plan", "validate"} {
		skillDir := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func linkFixtureSkills(t *testing.T, source, dest string) {
	t.Helper()
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	names, err := canonicalSkillNames(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if err := os.Symlink(filepath.Join(source, name), filepath.Join(dest, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func makeFakeAgentopsRepo(t *testing.T, declared string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "cli", "cmd", "ao")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cli", "go.mod"), []byte(agentopsModuleLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nvar version = \""+declared+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func pristineAdapter(t *testing.T, cwd string) LegacyChecks {
	t.Helper()
	home := t.TempDir()
	return LegacyChecks{
		ToolVersion: "1.0.0",
		WorkingDir:  func() (string, error) { return cwd, nil },
		HomeDir:     func() (string, error) { return home, nil },
		LedgerPath:  func() string { return filepath.Join(cwd, "ledger.jsonl") },
		Environment: func() []string { return nil },
		Now:         time.Now,
	}
}

// TestLegacyChecksDeclareAudience is the registry test: every check the adapter
// emits (inside a clone, where all checks run) must declare a valid audience.
func TestLegacyChecksDeclareAudience(t *testing.T) {
	root := t.TempDir()
	writeFakeAgentopsRepo(t, root)
	t.Chdir(root)
	t.Setenv("HOME", t.TempDir())
	adapter := pristineAdapter(t, root)
	checks := adapter.Checks(context.Background())
	if len(checks) == 0 {
		t.Fatal("no checks emitted")
	}
	valid := map[string]bool{
		quality.AudienceInstalledUser: true,
		quality.AudienceRepoDev:       true,
	}
	for _, check := range checks {
		if !valid[check.Audience] {
			t.Errorf("check %q declares invalid audience %q", check.Name, check.Audience)
		}
	}
}

var runnableFixPattern = regexp.MustCompile(`^(ao |br |brew |npm |curl |https://)`)

// TestDoctor_FixStringsRunnableByAudience asserts every installed-user check's
// Fix is either empty or a command runnable from the reader's own context —
// never a repo-relative script path.
func TestDoctor_FixStringsRunnableByAudience(t *testing.T) {
	root := t.TempDir()
	writeFakeAgentopsRepo(t, root)
	adapter := pristineAdapter(t, root)
	checks := adapter.Checks(context.Background())
	sawFix := false
	for _, check := range checks {
		if check.Audience != quality.AudienceInstalledUser || check.Fix == "" {
			continue
		}
		sawFix = true
		if !runnableFixPattern.MatchString(check.Fix) {
			t.Errorf("check %q fix %q is not runnable from the user's context", check.Name, check.Fix)
		}
	}
	if !sawFix {
		t.Fatal("expected at least one installed-user check to carry a Fix")
	}
}

// TestDoctor_PristineInstallGreen: a pristine install outside any agentops clone
// exits success (no fail, no above-info warning) and names no repo-relative
// script in any check detail.
func TestDoctor_PristineInstallGreen(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("HOME", t.TempDir())
	adapter := pristineAdapter(t, root)
	checks := adapter.Checks(context.Background())

	for _, check := range checks {
		switch check.Status {
		case quality.StatusFail:
			t.Errorf("pristine install must not fail: %q — %s", check.Name, check.Detail)
		case quality.StatusWarn:
			t.Errorf("pristine install must not warn above info: %q — %s", check.Name, check.Detail)
		}
		if strings.Contains(check.Detail, "scripts/") || strings.Contains(check.Detail, ".sh") {
			t.Errorf("check %q names a repo-relative script: %s", check.Name, check.Detail)
		}
		if strings.Contains(strings.ToLower(check.Detail), "required") && check.Status != quality.StatusPass {
			t.Errorf("check %q labels an optional dep required: %s", check.Name, check.Detail)
		}
	}

	output := quality.ComputeResult(checks)
	if quality.HasRequiredFailure(output.Checks) {
		t.Fatalf("pristine install has a required failure: %s", output.Summary)
	}
	if output.Result != "HEALTHY" {
		t.Fatalf("pristine result = %q, want HEALTHY (%s)", output.Result, output.Summary)
	}
}

func TestLegacyChecksOutsideCloneRemainInformational(t *testing.T) {
	root := t.TempDir()
	adapter := pristineAdapter(t, root)
	checks := adapter.Checks(context.Background())
	for _, check := range checks {
		if check.Name == "Skill Links" && check.Status != quality.StatusInfo {
			t.Fatalf("outside-repo links = %+v", check)
		}
		if check.Status == quality.StatusWarn || check.Status == quality.StatusFail {
			t.Errorf("outside-repo doctor should be green: %+v", check)
		}
	}
}

func TestCheckSkillLinksExactMissingAndStale(t *testing.T) {
	root := t.TempDir()
	writeFakeAgentopsRepo(t, root)
	home := t.TempDir()
	dest := filepath.Join(home, ".agents", "skills")
	linkFixtureSkills(t, filepath.Join(root, "skills"), dest)
	if check := CheckSkillLinks(root, home); check.Status != quality.StatusPass || !strings.Contains(check.Detail, "2 canonical skills") {
		t.Fatalf("exact links = %+v", check)
	}
	if err := os.Remove(filepath.Join(dest, "plan")); err != nil {
		t.Fatal(err)
	}
	if check := CheckSkillLinks(root, home); check.Status != quality.StatusWarn || !strings.Contains(check.Detail, "1 missing") {
		t.Fatalf("missing link = %+v", check)
	}
	if err := os.Symlink(filepath.Join(root, "skills", "retired"), filepath.Join(dest, "retired")); err != nil {
		t.Fatal(err)
	}
	if check := CheckSkillLinks(root, home); check.Status != quality.StatusWarn || !strings.Contains(check.Detail, "1 stale") {
		t.Fatalf("stale link = %+v", check)
	}
}

// checkoutOnlyCommands are commands an installed user cannot run: `ao skills
// link` resolves the repository's own skills/ directory and fails closed
// outside a checkout, so naming it to a reader who has no checkout is advice
// they cannot take. Observed live: doctor told an installed user to "run from
// the AgentOps checkout and inspect `ao skills link --dry-run`".
var checkoutOnlyCommands = []string{"ao skills link", "ao skills unlink", "from the AgentOps checkout", "from the repo checkout"}

// TestCheckSkillLinks_InstalledUserAdviceIsActionable is the acceptance for the
// audience defect: outside any agentops checkout, the Skill Links check may not
// tell the reader to do something only a checkout owner can do. It must name
// the affected root and a remedy performable from where the reader stands.
func TestCheckSkillLinks_InstalledUserAdviceIsActionable(t *testing.T) {
	home := t.TempDir()
	portable := filepath.Join(home, ".agents", "skills")
	if err := os.MkdirAll(portable, 0o755); err != nil {
		t.Fatal(err)
	}
	// A dangling link: the target was deleted (a skill retired upstream).
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), filepath.Join(portable, "retired")); err != nil {
		t.Fatal(err)
	}

	check := CheckSkillLinks("", home)
	if check.Status != quality.StatusWarn {
		t.Fatalf("dangling portable link status = %q, want warn (%+v)", check.Status, check)
	}
	for _, banned := range checkoutOnlyCommands {
		if strings.Contains(check.Detail, banned) {
			t.Errorf("installed-user detail names checkout-only advice %q: %s", banned, check.Detail)
		}
		if strings.Contains(check.Fix, banned) {
			t.Errorf("installed-user fix names checkout-only advice %q: %s", banned, check.Fix)
		}
	}
	if !strings.Contains(check.Detail, "1 dangling") {
		t.Errorf("detail does not report the dangling count: %s", check.Detail)
	}
	if !strings.Contains(check.Detail, portable) {
		t.Errorf("detail does not name the affected root %s: %s", portable, check.Detail)
	}
	if !strings.Contains(check.Detail, "reinstall") {
		t.Errorf("detail offers no remedy the reader can perform: %s", check.Detail)
	}
}

// TestCountLiveSkillLinks_BrokenMeansDanglingOnly pins the semantics of
// "broken" after the overcount fix: broken == the symlink does not resolve,
// exactly what `find -L <root> -maxdepth 1 -type l` prints. The previous
// implementation derived brokenness from a single stat of <link>/SKILL.md,
// which also failed for links that resolve perfectly well but do not name a
// skill package — so doctor could report "N broken" while a dangling-symlink
// sweep of the same tree found fewer, or none.
func TestCountLiveSkillLinks_BrokenMeansDanglingOnly(t *testing.T) {
	source := t.TempDir()
	root := t.TempDir()

	mkSkill := func(name string) string {
		dir := filepath.Join(source, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	link := func(target, name string) {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}

	// Live: resolves to a directory holding SKILL.md.
	link(mkSkill("plan"), "plan")
	link(mkSkill("validate"), "validate")
	// Dangling: target does not exist. The ONLY broken condition.
	link(filepath.Join(source, "deleted-upstream"), "retired")
	// Foreign: resolves fine, but is not a skill package. Nothing to repair —
	// counting this as broken is the overcount under test.
	sharedDir := filepath.Join(source, "shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link(sharedDir, "shared")
	// Foreign: a link to a regular FILE. <link>/SKILL.md cannot stat, but the
	// link resolves — also not broken.
	regular := filepath.Join(source, "notes.md")
	if err := os.WriteFile(regular, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link(regular, "notes.md")
	// Not a link at all: a real directory (a plugin copy). Counted nowhere.
	if err := os.MkdirAll(filepath.Join(root, "copied"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := countLiveSkillLinks(root)
	want := skillLinkCounts{Live: 2, Broken: 1, Foreign: 2}
	if got != want {
		t.Fatalf("countLiveSkillLinks = %+v, want %+v", got, want)
	}
}

// TestCountLiveSkillLinks_AgreesWithDanglingSweep is the cross-check that
// closes the reported contradiction ("doctor says 2 broken, find says 0
// dangling"): the Broken count must equal an independent dangling-symlink
// sweep of the same directory, computed here without touching production code.
func TestCountLiveSkillLinks_AgreesWithDanglingSweep(t *testing.T) {
	source := t.TempDir()
	root := t.TempDir()
	skill := filepath.Join(source, "plan")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("# plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(source, "shared")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	for target, name := range map[string]string{
		skill:                            "plan",
		shared:                           "shared",
		filepath.Join(source, "vanished"): "vanished",
	} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	dangling := 0
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, lerr := os.Lstat(path)
		if lerr != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		if _, serr := os.Stat(path); serr != nil {
			dangling++
		}
	}

	if got := countLiveSkillLinks(root).Broken; got != dangling {
		t.Fatalf("doctor Broken = %d but the dangling sweep found %d", got, dangling)
	}
}

func TestBinaryFreshnessCheck(t *testing.T) {
	root := makeFakeAgentopsRepo(t, "9.9.9")
	if check := BinaryFreshnessCheck(filepath.Join(root, "cli", "cmd"), "9.9.9"); check.Status != "pass" {
		t.Fatalf("matching check = %+v", check)
	}
	check := BinaryFreshnessCheck(root, "1.0.0")
	for _, want := range []string{"1.0.0", "9.9.9", "scripts/preflight-uat-binary.sh", "brew upgrade agentops"} {
		if check.Status != "warn" || !strings.Contains(check.Detail, want) {
			t.Fatalf("stale check lacks %q: %+v", want, check)
		}
	}
	if check := BinaryFreshnessCheck(t.TempDir(), "1.0.0"); check.Status != "pass" || !strings.Contains(check.Detail, "outside the agentops repo") {
		t.Fatalf("outside-repo check = %+v", check)
	}
	rcRoot := makeFakeAgentopsRepo(t, "3.3.0-rc")
	if check := BinaryFreshnessCheck(rcRoot, "3.3.0"); check.Status != "pass" || !strings.Contains(check.Detail, "release build") {
		t.Fatalf("release build against rc source = %+v", check)
	}
}

func appendFixtureEdges(t *testing.T, path string, count int) {
	t.Helper()
	store := provenancegraph.NewStore(path)
	for index := range count {
		if _, err := store.Append(provenancegraph.Edge{
			FromID: fmt.Sprintf("age-test.%d", index), FromType: "bead", ToID: "abc1234", ToType: "commit",
			Relation: "wasGeneratedBy", TrustTier: "authored", TS: time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			t.Fatalf("append edge %d: %v", index, err)
		}
	}
}

func TestCheckLedgerHealthIntactMissingAndTampered(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "ledger.jsonl")
	if check := CheckLedgerHealth(missing, time.Now); check.Status != "pass" || !strings.Contains(check.Detail, "no ledger records yet") {
		t.Fatalf("missing ledger = %+v", check)
	}

	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	appendFixtureEdges(t, path, 2)
	if check := CheckLedgerHealth(path, time.Now); check.Status != "pass" || !strings.Contains(check.Detail, "2 records") || !strings.Contains(check.Detail, "newest record") {
		t.Fatalf("intact ledger = %+v", check)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(data, []byte(`"to_id":"abc1234"`), []byte(`"to_id":"abc1235"`), 1)
	if bytes.Equal(tampered, data) {
		t.Fatal("fixture did not contain to_id")
	}
	if err := os.WriteFile(path, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	check := CheckLedgerHealth(path, time.Now)
	if check.Status != quality.StatusWarn || check.Required || !strings.Contains(check.Detail, "chain breaks at line 1") || !strings.Contains(check.Detail, "ao provenance verify") {
		t.Fatalf("tampered ledger = %+v", check)
	}
}

func TestCheckLaw0Guard(t *testing.T) {
	for _, test := range []struct {
		name, status, detail string
		environment          []string
	}{
		{name: "clean", environment: []string{"AGENTOPS_MODE=local"}, status: "pass", detail: "no reviewer path configured"},
		{name: "empty", status: "pass", detail: "no reviewer path configured"},
		{name: "review command print flag", environment: []string{"AGENTOPS_REVIEWER_CMD=claude" + " -p review"}, status: "fail", detail: "unset AGENTOPS_REVIEWER_CMD"},
		{name: "reviewer print word", environment: []string{"MY_REVIEWER_BIN=claude" + " --print"}, status: "fail", detail: "unset MY_REVIEWER_BIN"},
		{name: "unscoped ignored", environment: []string{"SHELL_HISTORY=claude -p review"}, status: "pass", detail: "no reviewer path configured"},
	} {
		t.Run(test.name, func(t *testing.T) {
			check := CheckLaw0Guard(test.environment)
			if check.Status != test.status || !check.Required || !strings.Contains(check.Detail, test.detail) {
				t.Fatalf("check = %+v", check)
			}
		})
	}
}

func TestBinaryFreshnessRejectsSymlinkedRepoMarkers(t *testing.T) {
	root := t.TempDir()
	external := filepath.Join(t.TempDir(), "go.mod")
	if err := os.WriteFile(external, []byte(agentopsModuleLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "cli", "go.mod")); err != nil {
		t.Fatal(err)
	}
	if _, ok := FindAgentopsRepoRoot(root); ok {
		t.Fatal("symlinked go.mod identified an AgentOps checkout")
	}
}

func TestRepoDeclaredVersionRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cli", "cmd", "ao")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, (1<<16)+100)
	copy(body[(1<<16)+1:], `var version = "forged"`)
	if err := os.WriteFile(filepath.Join(path, "main.go"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(path, "..", "..", ".."))
	if _, ok := RepoDeclaredVersion(root); ok {
		t.Fatal("oversized main.go yielded a version")
	}
}
