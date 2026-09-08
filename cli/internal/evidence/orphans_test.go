package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

type orphanFixture struct {
	t              *testing.T
	root           string
	harness, skill string
}

func orphanFixtureNew(t *testing.T) *orphanFixture {
	f := &orphanFixture{t: t, root: t.TempDir()}
	f.put("scripts/harness.sh", "echo harness\n")
	f.put("scripts/lib/preamble.sh", "echo preamble\n")
	f.put("skills/demo/SKILL.md", "demo skill\n")
	f.harness = "sha256:" + Hash([]byte("echo harness\n"))
	f.skill = "sha256:" + Hash([]byte("demo skill\n"))
	return f
}
func (f *orphanFixture) put(path, text string) {
	f.t.Helper()
	p := filepath.Join(f.root, path)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0600); err != nil {
		f.t.Fatal(err)
	}
}
func (f *orphanFixture) score(path, digest string) {
	f.t.Helper()
	block := fmt.Sprintf(`{"harness":{"path":"scripts/harness.sh","sha256":%q},"preamble":{"path":"scripts/lib/preamble.sh","sha256":%q}}`, digest, "sha256:"+Hash([]byte("echo preamble\n")))
	f.put(path, `{"evaluator":`+block+`,"capture_evaluator":`+block+`}`)
}
func (f *orphanFixture) fixture(digest string) {
	f.put("evals/skill-probes/probe-a/fixture-set.json", fmt.Sprintf(`{"capture_evaluator":{"harness":{"path":"scripts/harness.sh","sha256":%q}},"canonical_skill":{"name":"demo","path":"skills/demo/SKILL.md","sha256":%q}}`, f.harness, digest))
}
func (f *orphanFixture) remove(path string) {
	f.t.Helper()
	if err := os.RemoveAll(filepath.Join(f.root, path)); err != nil {
		f.t.Fatal(err)
	}
}
func (f *orphanFixture) chmod(path string, mode os.FileMode) {
	f.t.Helper()
	if os.Geteuid() == 0 {
		f.t.Skip("permission-denial fixture requires an unprivileged process")
	}
	p := filepath.Join(f.root, path)
	if err := os.Chmod(p, mode); err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { _ = os.Chmod(p, 0700) })
}
func (f *orphanFixture) link(path, target string) {
	f.t.Helper()
	if err := os.Symlink(target, filepath.Join(f.root, path)); err != nil {
		f.t.Fatal(err)
	}
}

// The established shell reader remains unchanged and independent. Every
// synthetic case below compares the complete JSON result (or incomplete exit)
// with it when the developer reference dependencies are installed.
func compareOrphanReference(t *testing.T, f *orphanFixture, changed []string, got *OrphanReceipt, scanErr error) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Log("Python reference comparison unavailable; native assertions still ran")
		return
	}
	_, file, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(file), "../../../scripts/evidence-orphans.sh")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal("Bash reference dependency unavailable")
	}
	cmd := exec.CommandContext(ctx, bash, append([]string{script}, changed...)...)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "EVIDENCE_ORPHANS_ROOT=") && !strings.HasPrefix(e, "PYTHONDONTWRITEBYTECODE=") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	cmd.Env = append(cmd.Env, "EVIDENCE_ORPHANS_ROOT="+f.root, "PYTHONDONTWRITEBYTECODE=1")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal("reference timed out")
	}
	if scanErr != nil {
		var exit *exec.ExitError
		ok := errors.As(err, &exit)
		if !ok || exit.ExitCode() != 2 {
			t.Fatalf("native error %v, reference %v: %s", scanErr, err, output)
		}
		return
	}
	if err != nil {
		t.Fatalf("native completed but reference failed: %v: %s", err, output)
	}
	var want OrphanReceipt
	if err = json.Unmarshal(output, &want); err != nil {
		t.Fatalf("reference JSON: %v: %s", err, output)
	}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("native/reference mismatch\nnative=%+v\nreference=%+v", *got, want)
	}
}

func TestOrphanBindingsReferenceParity(t *testing.T) {
	bad := "sha256:" + strings.Repeat("0", 64)
	cases := []struct {
		name      string
		setup     func(*orphanFixture)
		changed   []string
		causes    []string
		artifacts int
		errorText string
	}{
		{"changed", func(f *orphanFixture) { f.score("docs/evals/scorecards/sc.json", f.harness) }, []string{"scripts/harness.sh"}, []string{"changed_path"}, 1, ""},
		{"digest drift", func(f *orphanFixture) { f.score("docs/evals/scorecards/sc.json", bad) }, []string{"unrelated"}, []string{"digest_drift"}, 1, ""},
		{"both", func(f *orphanFixture) { f.score("docs/evals/scorecards/sc.json", bad) }, []string{"scripts/harness.sh"}, []string{"both"}, 1, ""},
		{"missing bound file", func(f *orphanFixture) {
			f.score("docs/evals/scorecards/sc.json", f.harness)
			f.remove("scripts/harness.sh")
		}, nil, []string{"digest_drift"}, 1, ""},
		{"symlink bound file", func(f *orphanFixture) {
			f.score("docs/evals/scorecards/sc.json", f.harness)
			f.remove("scripts/harness.sh")
			f.link("scripts/harness.sh", "lib/preamble.sh")
		}, nil, []string{"digest_drift"}, 1, ""},
		{"matching unrelated", func(f *orphanFixture) { f.score("docs/evals/scorecards/sc.json", f.harness) }, []string{"unrelated"}, nil, 0, ""},
		{"separate counts and dedupe", func(f *orphanFixture) {
			f.score("docs/evals/scorecards/one.json", f.harness)
			f.score("docs/evals/scorecards/two.json", f.harness)
		}, []string{"scripts/harness.sh", "scripts/lib/preamble.sh"}, []string{"changed_path", "changed_path", "changed_path", "changed_path"}, 2, ""},
		{"matching skill", func(f *orphanFixture) { f.fixture(f.skill) }, nil, nil, 0, ""},
		{"rewritten skill", func(f *orphanFixture) { f.fixture(f.skill); f.put("skills/demo/SKILL.md", "changed\n") }, nil, []string{"skill_changed"}, 1, ""},
		{"capture contract", func(f *orphanFixture) {
			f.put("evals/skill-probes/p/capture-contract.json", fmt.Sprintf(`{"evaluator":[],"canonical_skill":{"name":"demo","path":"skills/demo/SKILL.md","sha256":%q}}`, bad))
		}, nil, []string{"skill_changed"}, 1, ""},
		{"missing skill", func(f *orphanFixture) { f.fixture(f.skill); f.remove("skills/demo") }, nil, []string{"skill_changed"}, 1, ""},
		{"fixture evaluator", func(f *orphanFixture) { f.fixture(f.skill) }, []string{"scripts/harness.sh"}, []string{"changed_path"}, 1, ""},
		{"malformed JSON", func(f *orphanFixture) { f.put("docs/evals/scorecards/sc.json", "{broken") }, nil, nil, 0, "malformed JSON"},
		{"path type", func(f *orphanFixture) {
			f.put("docs/evals/scorecards/sc.json", `{"evaluator":{"h":{"path":7,"sha256":"x"}}}`)
		}, nil, nil, 0, "path is not a string"},
		{"missing sha", func(f *orphanFixture) { f.put("docs/evals/scorecards/sc.json", `{"evaluator":{"h":{"path":"x"}}}`) }, nil, nil, 0, "sha256 is not a string"},
		{"block shape", func(f *orphanFixture) { f.put("docs/evals/scorecards/sc.json", `{"evaluator":[]}`) }, nil, nil, 0, "not an object"},
		{"explicit null", func(f *orphanFixture) {
			f.put("docs/evals/scorecards/sc.json", `{"evaluator":null,"capture_evaluator":null}`)
		}, nil, nil, 0, ""},
		{"malformed skill", func(f *orphanFixture) {
			f.put("evals/skill-probes/p/fixture-set.json", `{"canonical_skill":{"name":"demo","path":"skills/demo/SKILL.md"}}`)
		}, nil, nil, 0, "sha256 is not a string"},
		{"walk error", func(f *orphanFixture) {
			f.score("docs/evals/scorecards/locked/sc.json", f.harness)
			f.chmod("docs/evals/scorecards/locked", 0)
		}, nil, nil, 0, "cannot walk"},
		{"empty", func(*orphanFixture) {}, []string{"anything"}, nil, 0, ""},
		{"listed skill", func(f *orphanFixture) { f.fixture(f.skill) }, []string{"skills/demo/SKILL.md"}, []string{"changed_path"}, 1, ""},
		{"listed rewritten skill", func(f *orphanFixture) { f.fixture(f.skill); f.put("skills/demo/SKILL.md", "changed\n") }, []string{"skills/demo/SKILL.md"}, []string{"both"}, 1, ""},
		{"evaluator read error", func(f *orphanFixture) {
			f.score("docs/evals/scorecards/sc.json", f.harness)
			f.chmod("scripts/harness.sh", 0)
		}, nil, nil, 0, "cannot read bound file"},
		{"canonical read error is null", func(f *orphanFixture) { f.fixture(f.skill); f.chmod("skills/demo/SKILL.md", 0) }, nil, []string{"skill_changed"}, 1, ""},
		{"artifact read error", func(f *orphanFixture) {
			f.score("docs/evals/scorecards/sc.json", f.harness)
			f.chmod("docs/evals/scorecards/sc.json", 0)
		}, nil, nil, 0, "malformed JSON"},
		{"skill symlink", func(f *orphanFixture) {
			f.fixture(f.skill)
			f.remove("skills/demo/SKILL.md")
			f.link("skills/demo/SKILL.md", "../../scripts/harness.sh")
		}, nil, []string{"skill_changed"}, 1, ""},
		{"unsafe skill", func(f *orphanFixture) {
			f.put("evals/skill-probes/p/fixture-set.json", `{"canonical_skill":{"name":"../demo","path":"recorded/path","sha256":"x"}}`)
		}, nil, []string{"skill_changed"}, 1, ""},
		{"duplicate keys last value and first category order", func(f *orphanFixture) {
			f.put("docs/evals/scorecards/sc.json", fmt.Sprintf(`{"evaluator":{"z":{"path":"scripts/harness.sh","sha256":"old"},"a":{"path":"scripts/harness.sh","sha256":"other"},"z":{"path":"scripts/harness.sh","sha256":%q}}}`, f.harness))
		}, []string{"scripts/harness.sh"}, []string{"changed_path"}, 1, ""},
		{"unknown and nonfinite metadata", func(f *orphanFixture) {
			f.put("docs/evals/scorecards/sc.json", `{"unknown":[NaN,Infinity,-Infinity],"quoted":"NaN", "evaluator":null}`)
		}, nil, nil, 0, ""},
		{"nonfinite binding is not null", func(f *orphanFixture) { f.put("docs/evals/scorecards/sc.json", `{"evaluator":NaN}`) }, nil, nil, 0, "not an object"},
		{"trailing data", func(f *orphanFixture) { f.put("docs/evals/scorecards/sc.json", `{} {}`) }, nil, nil, 0, "malformed JSON"},
		{"nonobject artifact", func(f *orphanFixture) { f.put("docs/evals/scorecards/sc.json", `[]`) }, nil, nil, 0, "top level"},
		{"changed order and duplicate lines", func(f *orphanFixture) { f.score("docs/evals/scorecards/sc.json", f.harness) }, []string{"scripts/harness.sh", "\n \nother\nscripts/harness.sh"}, []string{"changed_path"}, 1, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := orphanFixtureNew(t)
			tc.setup(f)
			got, err := OrphanBindings(f.root, tc.changed)
			if tc.errorText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errorText) || got != nil {
					t.Fatalf("wanted incomplete %q, got %+v, %v", tc.errorText, got, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if got.BindingCount != len(tc.causes) || got.ArtifactCount != tc.artifacts {
					t.Fatalf("wrong counts: %+v", got)
				}
				for n, cause := range tc.causes {
					if got.Orphaned[n].Cause != cause {
						t.Fatalf("wrong cause: %+v", got)
					}
				}
			}
			compareOrphanReference(t, f, tc.changed, got, err)
		})
	}
}

func TestOrphanMissingDirectoriesAndAbsoluteBindings(t *testing.T) {
	f := orphanFixtureNew(t)
	missing := filepath.Join(f.root, "absent")
	got, err := OrphanBindings(missing, nil)
	if err != nil || got.BindingCount != 0 {
		t.Fatalf("legacy absent scan roots: %+v %v", got, err)
	}
	target := filepath.Join(f.root, "scripts/harness.sh")
	f.put("docs/evals/scorecards/sc.json", fmt.Sprintf(`{"evaluator":{"h":{"path":%q,"sha256":%q}}}`, target, f.harness))
	got, err = OrphanBindings(f.root, []string{target})
	if err != nil || got.BindingCount != 1 || got.Orphaned[0].CurrentSHA256 == nil || *got.Orphaned[0].CurrentSHA256 != f.harness {
		t.Fatalf("absolute binding semantics: %+v %v", got, err)
	}
	compareOrphanReference(t, f, []string{target}, got, err)
}

func TestOrphanChangedUniversalNewlines(t *testing.T) {
	for _, changed := range []string{"scripts/harness.sh\r\nother\r\n", "scripts/harness.sh\rother\r"} {
		t.Run(fmt.Sprintf("%q", changed), func(t *testing.T) {
			f := orphanFixtureNew(t)
			f.score("docs/evals/scorecards/sc.json", f.harness)
			got, err := OrphanBindings(f.root, []string{changed})
			if err != nil {
				t.Fatal(err)
			}
			compareOrphanReference(t, f, []string{changed}, got, nil)
			if !reflect.DeepEqual(got.Changed, []string{"scripts/harness.sh", "other"}) || got.BindingCount != 1 || got.Orphaned[0].Cause != "changed_path" {
				t.Fatalf("universal newline mismatch: %+v", got)
			}
		})
	}
}
