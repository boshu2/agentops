package fixture

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestActualBuilderRecovery(t *testing.T) {
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	r := filepath.Join(base, "fixture")
	c := exec.Command("python3", "setup.py", r)
	if out, e := c.CombinedOutput(); e != nil {
		t.Fatalf("setup %v %s", e, out)
	}
	read := func(n string) []byte {
		b, e := os.ReadFile(filepath.Join(r, n))
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	failure := read("evidence/failed-build.json")
	var prior map[string]any
	if e := json.Unmarshal(failure, &prior); e != nil {
		t.Fatal(e)
	}
	if prior["structure_check_pass"] != false || prior["authoring_state"] != "scaffold" || prior["semantics_evaluated"] != false || strings.TrimSpace(string(read("evidence/build.rc"))) == "0" {
		t.Fatal("not a real failed partial build")
	}
	if !bytes.Contains(read("evidence/build.stderr"), []byte("projection incomplete")) {
		t.Fatal("wrong failure seam")
	}
	original := read("evidence/retained-source.md")
	c = exec.Command("bash", "workflow.sh", r)
	if out, e := c.CombinedOutput(); e != nil {
		t.Fatalf("workflow %v %s", e, out)
	}
	if !bytes.Equal(failure, read("evidence/failed-build.json")) || !bytes.Equal(original, read("evidence/retained-source.md")) {
		t.Fatal("original evidence changed")
	}
	source := read("repo/skills/recovery-pilot/SKILL.md")
	projection := read("repo/skills-codex/recovery-pilot/SKILL.md")
	// Adapter command semantics belong to the independent native review: an
	// equivalent git -C invocation need not contain one literal command phrase.
	for _, want := range []string{"Retained caller note: fixture-retained-729."} {
		if !bytes.Contains(source, []byte(want)) || !bytes.Contains(projection, []byte(want)) {
			t.Fatalf("missing retained behavior %s", want)
		}
	}
	if bytes.Contains(source, []byte("authoring_state: scaffold")) || bytes.Contains(source, []byte("TODO:")) {
		t.Fatal("incomplete source")
	}
	ao := os.Getenv("AO_SKILL_BUILDER_BIN")
	if ao == "" {
		ao = "/usr/local/bin/ao"
	}
	repo := filepath.Join(r, "repo")
	c = exec.Command(ao, "skills", "check-source", "--repo", repo, "--strict", "skills/recovery-pilot")
	if out, e := c.CombinedOutput(); e != nil {
		t.Fatalf("source check %v %s", e, out)
	}
	c = exec.Command("bash", "scripts/codex-sync.sh", "--check", "--only", "recovery-pilot")
	c.Dir = repo
	if out, e := c.CombinedOutput(); e != nil {
		t.Fatalf("projection %v %s", e, out)
	}
	c = exec.Command("bash", "scripts/regen-codex-hashes.sh", "--check", "--only", "recovery-pilot")
	c.Dir = repo
	if out, e := c.CombinedOutput(); e != nil {
		t.Fatalf("projection hash %v %s", e, out)
	}
	var audit map[string]any
	if e := json.Unmarshal(read("out/audit.json"), &audit); e != nil {
		t.Fatal(e)
	}
	if audit["schema_version"] != "skill-audit.v2" || !bytes.Contains(read("out/audit.json"), []byte("NOT_PROVEN")) {
		t.Fatal("audit promoted static facts")
	}
	var summary map[string]any
	if e := json.Unmarshal(read("out/summary.json"), &summary); e != nil {
		t.Fatal(e)
	}
	stage := summary["failed_stage"]
	if summary["source_retained"] != true || (stage != "codex_projection" && stage != "projection") || summary["recovery_complete"] != true || summary["semantics_evaluated"] != false || summary["original_report_preserved"] != true {
		t.Fatal("false completion summary")
	}
}
