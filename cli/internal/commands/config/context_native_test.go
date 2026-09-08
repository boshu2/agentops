package config_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	adapter "github.com/boshu2/agentops/cli/internal/adapters/config"
	command "github.com/boshu2/agentops/cli/internal/commands/config"
	app "github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/evidence"
	"gopkg.in/yaml.v3"
)

// Opt-in because it creates a synthetic embedded Dolt fixture using installed BD.
// Every command has an explicit directory, isolated HOME and a timeout. It never
// accesses the repository's native store or initializes a consumer project.
func TestContextInstalledBDRecovery(t *testing.T) {
	if os.Getenv("AO_TEST_BD_NATIVE") != "1" {
		t.Skip("set AO_TEST_BD_NATIVE=1 to exercise installed BD with a synthetic store")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Fatal(err)
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"BEADS_DIR", "BEADS_DB", "BEADS_DOLT_SERVER_HOST", "BEADS_DOLT_SERVER_PORT", "BEADS_DOLT_SERVER_DATABASE", "BD_GLOBAL", "AGENTOPS_CONFIG", "GIT_DIR", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_INDEX_FILE", "GIT_WORK_TREE"} {
		t.Setenv(key, "")
		if err = os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"SOURCE_ID", "PROJECT_ID", "OWNER_SCOPE", "BUNDLE_ID", "BUNDLE_ROOT", "EVIDENCE_ROOT", "STAGING_ROOT", "ACCESS_POLICY_REF", "OWNER_POLICY_REF", "TASK_POLICY_REF", "MODEL_POLICY_REF", "DESTINATION_POLICY_REF", "MAINTENANCE_WORK_REF"} {
		t.Setenv("AGENTOPS_CONTEXT_"+key, "")
	}
	home := filepath.Join(base, "home")
	native := filepath.Join(base, "native")
	consumer := filepath.Join(base, "consumer")
	for _, path := range []string{home, native, consumer, filepath.Join(base, "bundle"), filepath.Join(base, "staging"), filepath.Join(base, "evidence")} {
		if err = os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("BEADS_ACTOR", "synthetic-context-fixture")
	t.Chdir(consumer)
	run := func(name string, dir string, args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		c := exec.CommandContext(ctx, name, args...)
		c.Dir = dir
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, b)
		}
		return b
	}
	bd := func(args ...string) []byte {
		t.Helper()
		return run("bd", native, append([]string{"--sandbox", "--actor", "synthetic-context-fixture"}, args...)...)
	}
	t.Logf("installed %s", strings.TrimSpace(string(bd("version"))))
	bd("init", "--non-interactive", "--skip-agents", "--skip-hooks", "--prefix", "ctxfixture")
	var n app.NativeContext
	if err = json.Unmarshal(bd("--readonly", "context", "--json"), &n); err != nil {
		t.Fatal(err)
	}
	if n.Backend != "dolt" || n.BeadsDir != filepath.Join(native, ".beads") {
		t.Fatalf("fixture routing escaped: %+v", n)
	}
	var anchor struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(bd("create", "Synthetic retained maintenance anchor", "--type", "epic", "--json"), &anchor); err != nil {
		t.Fatal(err)
	}
	c := app.ContextConfig{SourceID: n.BeadsDir, ProjectID: n.ProjectID, OwnerScope: "synthetic-personal", BundleID: "synthetic-bundle", BundleRoot: filepath.Join(base, "bundle"), EvidenceRoot: filepath.Join(base, "evidence"), StagingRoot: filepath.Join(base, "staging"), AccessPolicyRef: filepath.Join(base, "policy.json"), OwnerPolicyRef: filepath.Join(base, "owner-policy"), TaskPolicyRef: filepath.Join(base, "task-policy"), ModelPolicyRef: filepath.Join(base, "model-policy"), DestinationPolicyRef: filepath.Join(base, "destination-policy"), MaintenanceWorkRef: anchor.ID}
	p := app.ContextPolicy{SchemaVersion: 1, SourceID: c.SourceID, ProjectID: c.ProjectID, OwnerScope: c.OwnerScope, TaskRef: "synthetic-task", ModelRef: "synthetic-model", DestinationRef: "synthetic-destination", BundleID: c.BundleID, BundleRoot: c.BundleRoot, EvidenceRoot: c.EvidenceRoot, StagingRoot: c.StagingRoot, OwnerPolicyRef: c.OwnerPolicyRef, TaskPolicyRef: c.TaskPolicyRef, ModelPolicyRef: c.ModelPolicyRef, DestinationPolicyRef: c.DestinationPolicyRef, MaintenanceWorkRef: anchor.ID}
	writeJSON := func(path string, value any) {
		t.Helper()
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeJSON(c.AccessPolicyRef, p)
	for _, path := range []string{c.OwnerPolicyRef, c.TaskPolicyRef, c.ModelPolicyRef, c.DestinationPolicyRef} {
		if err = os.WriteFile(path, []byte("synthetic policy locator"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(home, ".agents", "ao", "config.yaml")
	if err = os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(&app.Config{Context: c})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	factPath := filepath.Join(base, "fact.json")
	writeJSON(factPath, app.ContextFact{Type: "context.route.v1", FactID: "route-1", Route: &c})
	bd("comments", "add", anchor.ID, "-f", factPath, "--json")
	for i := 0; i < 55; i++ {
		bd("comments", "add", anchor.ID, fmt.Sprintf("Synthetic ordinary comment %d", i), "--json")
	}
	withdrawal := app.ContextFact{Type: "context.withdrawal.v1", FactID: "withdrawal-1", BundleID: c.BundleID, PageID: "synthetic-page", PageDigest: strings.Repeat("a", 64), Counterevidence: []string{"synthetic:counterexample"}}
	writeJSON(factPath, withdrawal)
	bd("comments", "add", anchor.ID, "-f", factPath, "--json")
	var child struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(bd("create", "Synthetic investigation", "--parent", anchor.ID, "--metadata", `{"ao.context.bundle_id":"synthetic-bundle","ao.context.fact_id":"withdrawal-1"}`, "--json"), &child); err != nil {
		t.Fatal(err)
	}
	bd("close", child.ID, "--reason", "Synthetic fixture closed; anchor withdrawal remains", "--json")
	var closed []struct{ ID, Status string }
	if err = json.Unmarshal(bd("--readonly", "list", "--all", "--status", "closed", "--parent", anchor.ID, "--metadata-field", "ao.context.fact_id=withdrawal-1", "--limit", "0", "--json"), &closed); err != nil {
		t.Fatal(err)
	}
	if len(closed) != 1 || closed[0].ID != child.ID || closed[0].Status != "closed" {
		t.Fatalf("native closed flat-metadata query mismatch: %s", closed)
	}
	run("git", consumer, "init", "-q")
	run("git", consumer, "-c", "user.name=Synthetic", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "synthetic consumer")
	if err = os.WriteFile(filepath.Join(c.BundleRoot, "retained.md"), []byte("synthetic retained knowledge"), 0600); err != nil {
		t.Fatal(err)
	}
	run("git", c.BundleRoot, "init", "-q")
	run("git", c.BundleRoot, "add", "retained.md")
	run("git", c.BundleRoot, "-c", "user.name=Synthetic", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "synthetic retained knowledge")
	beforeConsumer := contextTreeDigest(t, consumer)
	beforeBundle := contextTreeDigest(t, c.BundleRoot)
	invoke := func(extra ...string) ([]byte, error) {
		t.Helper()
		cmd := command.NewContextCommand(app.NewCommandService(adapter.Gateway{}))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		args := []string{"--source-id", c.SourceID, "--project-id", c.ProjectID, "--owner-scope", c.OwnerScope, "--task-ref", p.TaskRef, "--model-ref", p.ModelRef, "--destination-ref", p.DestinationRef, "--consumer-root", consumer, "--native-directory", native}
		cmd.SetArgs(append(args, extra...))
		err := cmd.ExecuteContext(context.Background())
		return out.Bytes(), err
	}
	result, err := invoke()
	if err != nil {
		t.Fatalf("resolve: %v %s", err, result)
	}
	var initial app.ContextResult
	if err = json.Unmarshal(result, &initial); err != nil {
		t.Fatal(err)
	}
	if initial.CommentsRead != 57 || len(initial.Facts) != 2 || initial.Facts[1].FactID != "withdrawal-1" {
		t.Fatalf("direct complete read lost fact after 50: %+v", initial)
	}
	if err = os.Remove(configPath); err != nil {
		t.Fatal(err)
	}
	recoveryFlags := []string{"--recover", "--access-policy-ref", c.AccessPolicyRef, "--maintenance-work-ref", anchor.ID}
	result, err = invoke(recoveryFlags...)
	if err != nil {
		t.Fatalf("recover: %v %s", err, result)
	}
	var recovered app.ContextResult
	if err = json.Unmarshal(result, &recovered); err != nil {
		t.Fatal(err)
	}
	if recovered.Route != initial.Route || !recovered.Recovered || recovered.CommentsRead != 57 {
		t.Fatal("recovery did not preserve exact bundle and native withdrawal source")
	}
	root, err := invoke(append(recoveryFlags, "--field", "evidence_root")...)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := evidence.SnapshotIntent(strings.TrimSpace(string(root)), []byte("synthetic exact intent"), consumer, c.BundleRoot)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot == nil {
		t.Fatal("existing evidence consumer did not use resolved root")
	}
	if contextTreeDigest(t, consumer) != beforeConsumer || contextTreeDigest(t, c.BundleRoot) != beforeBundle {
		t.Fatal("lookup/recovery/evidence consumption changed consumer files, index, objects or retained knowledge")
	}
	if _, err = os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatal("recovery wrote replacement config")
	}
	if _, err = (adapter.Gateway{}).AnchorComments(context.Background(), native, "ctxfixture-missing"); err == nil {
		t.Fatal("native missing anchor accepted")
	}
	if _, err = (adapter.Gateway{}).AnchorComments(context.Background(), filepath.Join(base, "missing-source"), anchor.ID); err == nil {
		t.Fatal("native source read error accepted")
	}
	t.Log("BD direct comments returned all 57 comments including the withdrawal after comment 50; closed-child flat metadata query returned the investigation; same anchor and bundle recovered after home config removal; consumer files/index/all Git objects and bundle bytes unchanged; resolved evidence root consumed by SnapshotIntent; missing anchor and source errors rejected")
}
func contextTreeDigest(t *testing.T, root string) string {
	t.Helper()
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "%s\x00%s\x00", rel, d.Type())
		if !d.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			hash.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(hash.Sum(nil))
}
