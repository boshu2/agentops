package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type contextFixtureGateway struct {
	native   NativeContext
	comments []AnchorComment
	err      error
	reads    int
}

func (g *contextFixtureGateway) NativeContext(context.Context, string) (NativeContext, error) {
	g.reads++
	return g.native, g.err
}
func (g *contextFixtureGateway) AnchorComments(context.Context, string, string) ([]AnchorComment, error) {
	g.reads++
	return g.comments, g.err
}
func writeContextJSON(t *testing.T, path string, value any) {
	t.Helper()
	b, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func writeContextYAML(t *testing.T, path string, value ContextConfig) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	b, e := yaml.Marshal(&Config{Context: value})
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func contextFixture(t *testing.T) (ContextConfig, ContextPolicy, ContextRequest, *contextFixtureGateway, string) {
	t.Helper()
	t.Setenv("AGENTOPS_CONFIG", "")
	base, canonicalErr := filepath.EvalSymlinks(t.TempDir())
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	t.Setenv("HOME", filepath.Join(base, "home"))
	consumer := filepath.Join(base, "consumer")
	t.Chdir(base)
	for key := range (&ContextConfig{}).fields() {
		t.Setenv("AGENTOPS_CONTEXT_"+strings.ToUpper(key), "")
	}
	for _, k := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
		t.Setenv(k, "")
		if err := os.Unsetenv(k); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"consumer", "source", "bundle", "evidence", "staging", "home"} {
		if e := os.Mkdir(filepath.Join(base, name), 0700); e != nil {
			t.Fatal(e)
		}
	}
	c := ContextConfig{SourceID: filepath.Join(base, "source"), ProjectID: "project-native-id", OwnerScope: "personal", BundleID: "bundle-id", BundleRoot: filepath.Join(base, "bundle"), EvidenceRoot: filepath.Join(base, "evidence"), StagingRoot: filepath.Join(base, "staging"), AccessPolicyRef: filepath.Join(base, "policy.json"), OwnerPolicyRef: filepath.Join(base, "owner-policy"), TaskPolicyRef: filepath.Join(base, "task-policy"), ModelPolicyRef: filepath.Join(base, "model-policy"), DestinationPolicyRef: filepath.Join(base, "destination-policy"), MaintenanceWorkRef: "fixture-anchor"}
	for _, path := range []string{c.OwnerPolicyRef, c.TaskPolicyRef, c.ModelPolicyRef, c.DestinationPolicyRef} {
		if e := os.WriteFile(path, []byte("synthetic policy locator"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	p := ContextPolicy{SchemaVersion: 1, SourceID: c.SourceID, ProjectID: c.ProjectID, OwnerScope: c.OwnerScope, TaskRef: "fixture-task", ModelRef: "fixture-model", DestinationRef: "fixture-destination", BundleID: c.BundleID, BundleRoot: c.BundleRoot, EvidenceRoot: c.EvidenceRoot, StagingRoot: c.StagingRoot, OwnerPolicyRef: c.OwnerPolicyRef, TaskPolicyRef: c.TaskPolicyRef, ModelPolicyRef: c.ModelPolicyRef, DestinationPolicyRef: c.DestinationPolicyRef, MaintenanceWorkRef: c.MaintenanceWorkRef}
	writeContextJSON(t, c.AccessPolicyRef, p)
	path := filepath.Join(base, "home", ".agents", "ao", "config.yaml")
	writeContextYAML(t, path, c)
	r := ContextRequest{SourceID: c.SourceID, ProjectID: c.ProjectID, OwnerScope: c.OwnerScope, TaskRef: p.TaskRef, ModelRef: p.ModelRef, DestinationRef: p.DestinationRef, ConsumerRoot: consumer, NativeDirectory: base}
	b, _ := json.Marshal(ContextFact{Type: "context.route.v1", FactID: "route-1", Route: &c})
	g := &contextFixtureGateway{native: NativeContext{BDVersion: "1.2.2", SchemaVersion: 1, Backend: "dolt", ProjectID: c.ProjectID, BeadsDir: c.SourceID}, comments: []AnchorComment{{ID: "1", IssueID: c.MaintenanceWorkRef, Text: string(b)}}}
	return c, p, r, g, path
}
func TestContextPrecedenceAndSameAnchorRecovery(t *testing.T) {
	c, _, req, g, path := contextFixture(t)
	got, err := ResolveContext(context.Background(), g, req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Route != c || got.Recovered || got.Sources["bundle_root"] != SourceHome {
		t.Fatalf("route/source mismatch: %+v", got)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	req.Overrides = ContextConfig{AccessPolicyRef: c.AccessPolicyRef, MaintenanceWorkRef: c.MaintenanceWorkRef}
	req.Recover = true
	recovered, err := ResolveContext(context.Background(), g, req)
	if err != nil {
		t.Fatal(err)
	}
	if !recovered.Recovered || recovered.Route != got.Route || recovered.Route.MaintenanceWorkRef != c.MaintenanceWorkRef {
		t.Fatalf("recovery replaced identity: %+v", recovered)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("recovery recreated config")
	}
	req.Recover = false
	if _, err = ResolveContext(context.Background(), g, req); err == nil {
		t.Fatal("missing route accepted without explicit recovery")
	}
}
func TestContextEveryFieldUsesExistingPrecedence(t *testing.T) {
	_, _, _, _, _ = contextFixture(t)
	home := ContextConfig{}
	project := ContextConfig{}
	flags := ContextConfig{}
	for key, p := range home.fields() {
		*p = "home-" + key
		*project.fields()[key] = "project-" + key
		*flags.fields()[key] = "flag-" + key
	}
	writeContextYAML(t, homeConfigPath(), home)
	got, sources, err := LoadContext(ContextConfig{})
	if err != nil || !reflect.DeepEqual(got, home) || sources["bundle_root"] != SourceHome {
		t.Fatalf("home: %+v %v", got, err)
	}
	writeContextYAML(t, projectConfigPath(), project)
	got, sources, err = LoadContext(ContextConfig{})
	if err != nil || got != project || sources["bundle_root"] != SourceProject {
		t.Fatalf("project: %+v %v", got, err)
	}
	for key := range home.fields() {
		t.Setenv("AGENTOPS_CONTEXT_"+strings.ToUpper(key), "env-"+key)
	}
	got, sources, err = LoadContext(ContextConfig{})
	if err != nil || sources["bundle_root"] != SourceEnv {
		t.Fatal(err)
	}
	for key, p := range got.fields() {
		if *p != "env-"+key {
			t.Fatalf("environment lost %s", key)
		}
	}
	got, sources, err = LoadContext(flags)
	if err != nil || got != flags || sources["bundle_root"] != SourceFlag {
		t.Fatalf("flags: %+v %v", got, err)
	}
	for key := range home.fields() {
		t.Setenv("AGENTOPS_CONTEXT_"+strings.ToUpper(key), "")
	}
	explicit := filepath.Join(t.TempDir(), "explicit.yaml")
	writeContextYAML(t, explicit, ContextConfig{BundleRoot: "explicit-only"})
	t.Setenv("AGENTOPS_CONFIG", explicit)
	got, _, err = LoadContext(ContextConfig{})
	if err != nil || got.BundleRoot != "explicit-only" || got.OwnerScope != "" {
		t.Fatalf("explicit leaked ambient config: %+v %v", got, err)
	}
	if err = os.Remove(explicit); err != nil {
		t.Fatal(err)
	}
	if _, _, err = LoadContext(ContextConfig{}); err == nil {
		t.Fatal("missing explicit config fell back")
	}
}
func TestContextDeniedBeforeNativeRead(t *testing.T) {
	for _, name := range []string{"missing-policy", "missing-task-policy", "owner", "project", "source", "task", "model", "destination", "anchor", "policy-version", "configured-owner", "configured-root", "missing-root", "invalid-config"} {
		t.Run(name, func(t *testing.T) {
			c, p, req, g, path := contextFixture(t)
			switch name {
			case "missing-policy":
				os.Remove(c.AccessPolicyRef)
			case "missing-task-policy":
				os.Remove(c.TaskPolicyRef)
			case "owner":
				req.OwnerScope = "employer"
			case "project":
				req.ProjectID = "other"
			case "source":
				req.SourceID = filepath.Dir(c.SourceID)
			case "task":
				req.TaskRef = "other"
			case "model":
				req.ModelRef = "other"
			case "destination":
				req.DestinationRef = "public"
			case "anchor":
				req.Overrides.MaintenanceWorkRef = "replacement"
			case "policy-version":
				p.SchemaVersion = 2
				writeContextJSON(t, c.AccessPolicyRef, p)
			case "configured-owner":
				c.OwnerScope = "customer"
				writeContextYAML(t, path, c)
			case "configured-root":
				c.EvidenceRoot = c.StagingRoot
				writeContextYAML(t, path, c)
			case "missing-root":
				os.Remove(c.EvidenceRoot)
			case "invalid-config":
				os.WriteFile(path, []byte("context: ["), 0600)
			}
			if _, err := ResolveContext(context.Background(), g, req); err == nil {
				t.Fatal("denied route accepted")
			}
			if g.reads != 0 {
				t.Fatalf("private native source read before denial: %d", g.reads)
			}
		})
	}
}
func TestContextNonGitRoots(t *testing.T) {
	for _, kind := range []string{"bundle", "consumer", "unrelated-worktree", "bare-objects", "linked-worktree", "linked-admin", "symlink-bundle", "symlink-object", "active-objects", "overlap"} {
		for _, which := range []string{"staging", "evidence"} {
			t.Run(kind+"/"+which, func(t *testing.T) {
				c, p, req, g, path := contextFixture(t)
				base := filepath.Dir(c.BundleRoot)
				bad := c.BundleRoot
				git := func(args ...string) {
					t.Helper()
					cmd := exec.Command("git", args...)
					if b, e := cmd.CombinedOutput(); e != nil {
						t.Fatalf("git: %v %s", e, b)
					}
				}
				switch kind {
				case "consumer":
					bad = req.ConsumerRoot
				case "unrelated-worktree":
					bad = filepath.Join(base, "unrelated")
					git("init", "-q", bad)
				case "bare-objects":
					repo := filepath.Join(base, "bare")
					git("init", "--bare", "-q", repo)
					bad = filepath.Join(repo, "objects")
				case "linked-worktree", "linked-admin":
					repo := filepath.Join(base, "other")
					git("init", "-q", repo)
					git("-C", repo, "-c", "user.name=Synthetic", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "fixture")
					linked := filepath.Join(base, "linked")
					git("-C", repo, "worktree", "add", "-q", linked)
					bad = linked
					if kind == "linked-admin" {
						bad = filepath.Join(repo, ".git", "worktrees", "linked")
					}
				case "symlink-bundle":
					bad = filepath.Join(base, "alias")
					os.Symlink(c.BundleRoot, bad)
				case "symlink-object":
					repo := filepath.Join(base, "bare")
					git("init", "--bare", "-q", repo)
					bad = filepath.Join(base, "alias")
					os.Symlink(filepath.Join(repo, "objects"), bad)
				case "active-objects":
					bad = filepath.Join(base, "relocated-objects")
					os.Mkdir(bad, 0700)
					t.Setenv("GIT_OBJECT_DIRECTORY", bad)
				case "overlap":
					bad = c.EvidenceRoot
					if which == "evidence" {
						bad = c.StagingRoot
					}
				}
				if which == "staging" {
					c.StagingRoot = bad
					p.StagingRoot = bad
				} else {
					c.EvidenceRoot = bad
					p.EvidenceRoot = bad
				}
				writeContextJSON(t, c.AccessPolicyRef, p)
				writeContextYAML(t, path, c)
				if _, err := ResolveContext(context.Background(), g, req); err == nil {
					t.Fatalf("accepted %s", bad)
				}
				if g.reads != 0 {
					t.Fatal("read native source before path denial")
				}
			})
		}
	}
}
func TestContextAnchorFailuresAndCompleteFacts(t *testing.T) {
	c, _, req, g, _ := contextFixture(t)
	for i := 2; i <= 60; i++ {
		g.comments = append(g.comments, AnchorComment{ID: strconv.Itoa(i), IssueID: c.MaintenanceWorkRef, Text: "unrelated ordinary comment"})
	}
	withdrawal := ContextFact{Type: "context.withdrawal.v1", FactID: "negative-1", BundleID: c.BundleID, PageID: "page", PageDigest: strings.Repeat("a", 64), Counterevidence: []string{"native:counterexample"}}
	b, _ := json.Marshal(withdrawal)
	g.comments = append(g.comments, AnchorComment{ID: "61", IssueID: c.MaintenanceWorkRef, Text: string(b)})
	result, err := ResolveContext(context.Background(), g, req)
	if err != nil {
		t.Fatal(err)
	}
	if result.CommentsRead != 61 || len(result.Facts) != 2 || result.Facts[1].FactID != "negative-1" {
		t.Fatal("late withdrawal lost")
	}
	g.err = errors.New("native read failure")
	if _, err = ResolveContext(context.Background(), g, req); err == nil {
		t.Fatal("native failure interpreted as clearance")
	}
	g.err = nil
	saved := g.comments
	g.comments = nil
	if _, err = ResolveContext(context.Background(), g, req); err == nil {
		t.Fatal("missing route fact created empty replacement")
	}
	g.comments = saved
	g.native.ProjectID = "other"
	if _, err = ResolveContext(context.Background(), g, req); err == nil {
		t.Fatal("native source conflict accepted")
	}
}

func TestContextFactContract(t *testing.T) {
	c, _, _, _, _ := contextFixture(t)
	withdrawal := ContextFact{Type: "context.withdrawal.v1", FactID: "negative", BundleID: c.BundleID, PageID: "page", PageDigest: strings.Repeat("a", 64), Counterevidence: []string{"native:counterexample"}}
	resolution := ContextFact{Type: "context.resolution.v1", FactID: "resolution", BundleID: c.BundleID, PageID: "page", PageDigest: withdrawal.PageDigest, ResolvesFactID: withdrawal.FactID, ReviewRef: "evidence:exact-fresh-review", ReviewDigest: strings.Repeat("b", 64), SuccessorDigest: strings.Repeat("c", 64)}
	for _, fact := range []ContextFact{withdrawal, resolution} {
		data, _ := json.Marshal(fact)
		if got, err := ParseContextFacts([]AnchorComment{{ID: "native-string-id", IssueID: c.MaintenanceWorkRef, Text: string(data)}}, c.MaintenanceWorkRef); err != nil || len(got) != 1 {
			t.Fatalf("valid fact: %+v %v", got, err)
		}
	}
	for _, name := range []string{"missing-review", "missing-successor", "missing-counterevidence", "duplicate-key", "wrong-anchor", "missing-comment-id", "duplicate-comment", "unknown-type", "truncated-json", "duplicate-fact"} {
		t.Run(name, func(t *testing.T) {
			fact := resolution
			switch name {
			case "missing-review":
				fact.ReviewRef = ""
			case "missing-successor":
				fact.SuccessorDigest = ""
			case "missing-counterevidence":
				fact = withdrawal
				fact.Counterevidence = nil
			case "unknown-type":
				fact.Type = "context.resolution.v2"
			}
			data, _ := json.Marshal(fact)
			text := string(data)
			if name == "duplicate-key" {
				text = strings.Replace(text, `"fact_id":"resolution"`, `"fact_id":"first","fact_id":"resolution"`, 1)
			}
			if name == "truncated-json" {
				text = text[:len(text)-1]
			}
			comments := []AnchorComment{{ID: "comment-1", IssueID: c.MaintenanceWorkRef, Text: text}}
			switch name {
			case "wrong-anchor":
				comments[0].IssueID = "other"
			case "missing-comment-id":
				comments[0].ID = ""
			case "duplicate-comment":
				comments = append(comments, comments[0])
			case "duplicate-fact":
				comments = append(comments, AnchorComment{ID: "comment-2", IssueID: c.MaintenanceWorkRef, Text: text})
			}
			if _, err := ParseContextFacts(comments, c.MaintenanceWorkRef); err == nil {
				t.Fatal("malformed/ambiguous fact accepted")
			}
		})
	}
}
func TestContextRejectsAmbiguousPolicyAndAliasedPolicyRoot(t *testing.T) {
	for _, name := range []string{"duplicate", "unknown", "case-key", "symlink-escape", "version"} {
		t.Run(name, func(t *testing.T) {
			c, p, req, g, _ := contextFixture(t)
			switch name {
			case "symlink-escape":
				alias := filepath.Join(filepath.Dir(c.BundleRoot), "redirected-policy-root")
				if err := os.Symlink(c.StagingRoot, alias); err != nil {
					t.Fatal(err)
				}
				p.StagingRoot = alias
			case "version":
				g.native.BDVersion = "9.0.0"
			}
			data, _ := json.Marshal(p)
			text := string(data)
			switch name {
			case "duplicate":
				text = strings.Replace(text, `"schema_version":1`, `"schema_version":2,"schema_version":1`, 1)
			case "unknown":
				text = strings.Replace(text, `"schema_version":1`, `"schema_version":1,"allow_anything":true`, 1)
			case "case-key":
				text = strings.Replace(text, `"schema_version":1`, `"Schema_Version":1`, 1)
			}
			if err := os.WriteFile(c.AccessPolicyRef, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := ResolveContext(context.Background(), g, req); err == nil {
				t.Fatal("ambiguous/unsupported policy accepted")
			}
		})
	}
}
