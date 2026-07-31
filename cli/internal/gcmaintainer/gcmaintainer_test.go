package gcmaintainer

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var fixtureChecks = []string{
	"build-artifact-valid.sh",
	"design-review-approved.sh",
	"gap-analysis-approved.sh",
	"implementation-review-approved.sh",
}

// fixture is a fake Gas City estate: a city with one non-HQ rig, a resolved
// official pack cache, a scripted gc binary driven by FAKE_* env vars, and an
// AgentOps skills source. It is the Go port of the retired
// tests/python/test_gc_maintainer_ops.py harness.
type fixture struct {
	city   string
	rig    string
	pack   string
	gcBin  string
	log    string
	skills string
	// codexBin is a scripted Codex CLI that answers the app-server JSON-RPC
	// handshake from FAKE_CODEX_HOOKS_JSON, so the real trust-seeding code
	// path runs against a temp CODEX_HOME instead of the host's.
	codexBin   string
	codexHome  string
	codexTrust string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{
		city:       filepath.Join(root, "city"),
		rig:        filepath.Join(root, "city", "rigs", "smoke"),
		pack:       filepath.Join(root, "pack", "gascity"),
		gcBin:      filepath.Join(root, "bin", "gc"),
		log:        filepath.Join(root, "gc.log"),
		skills:     filepath.Join(root, "skills"),
		// The fake codex lives outside the fixture bin dir so a test can build
		// a PATH that finds gc but not codex.
		codexBin:   filepath.Join(root, "codexbin", "codex"),
		codexHome:  filepath.Join(root, "codexhome"),
		codexTrust: filepath.Join(root, "codexhome", "config.toml"),
	}
	mustMkdir(t, f.rig)
	mustMkdir(t, filepath.Join(root, "bin"))
	mustWrite(t, filepath.Join(f.city, "city.toml"), "[workspace]\n")
	mustMkdir(t, filepath.Join(f.pack, "assets", "scripts", "checks"))
	mustMkdir(t, filepath.Join(f.pack, "schemas"))
	f.writePackMarker(t, maintainerCommit)
	for _, name := range fixtureChecks {
		mustWrite(t, filepath.Join(f.pack, "assets", "scripts", "checks", name),
			fmt.Sprintf("#!/bin/sh\nprintf 'upstream %s\\n'\n", name))
	}
	mustWrite(t, filepath.Join(f.pack, "assets", "scripts", "validate_build_artifact.py"), "import yaml\n")
	mustWrite(t, filepath.Join(f.pack, "schemas", "gc.build.final-report.v1.schema.json"), "{}\n")
	for _, skill := range requiredSkills {
		mustWrite(t, filepath.Join(f.skills, skill, "SKILL.md"), "# "+skill+"\n")
	}
	f.writeFakeGC(t)
	f.writeFakeCodex(t)
	python := filepath.Join(root, "bin", "python3")
	mustWriteExecutable(t, python, "#!/bin/sh\nexit 0\n")

	t.Setenv("AGENTOPS_GC_SKIP_SERVICE_CHECK", "1")
	t.Setenv("CODEX_HOME", f.codexHome)
	t.Setenv("GC_PYTHON_BIN", python)
	t.Setenv("FAKE_GC_LOG", f.log)
	t.Setenv("FAKE_PACK_COMMIT", maintainerCommit)
	t.Setenv("FAKE_RIG", f.rig)
	f.setDoctor(t, map[string]any{"ok": true, "blocking_failed": 0, "failed": 0, "results": []any{}})
	f.setStatus(t, map[string]any{"ok": true, "partial": false, "health": map[string]any{"signals": []any{}}})
	f.setReady(t, []any{})
	f.setSessions(t, []any{})
	f.setCodexHooks(t)
	f.setAgents(t)
	return f
}

// writeFakeCodex scripts a Codex CLI that speaks just enough of the app-server
// stdio JSON-RPC protocol: it acknowledges initialize, then answers hooks/list.
//
// Its trustStatus is DERIVED from the trust store the way Codex derives it —
// a key whose recorded trusted_hash matches its current hash is "trusted", a
// recorded key with a different hash is "modified", an unrecorded key is
// "untrusted". A static answer could never model idempotency honestly, because
// the second prepare must see what the first one wrote.
//
// FAKE_CODEX_HOOK_SPEC carries one `key<TAB>hash` per line;
// FAKE_CODEX_FORCE_STATUS pins the status for the cases that need one;
// FAKE_CODEX_RAW_JSON replaces the whole answer. Every invocation is logged.
func (f *fixture) writeFakeCodex(t *testing.T) {
	t.Helper()
	script := `#!/bin/sh
printf '%s cwd=%s\n' "$*" "$PWD" >>"$FAKE_CODEX_LOG"
emit_hooks() {
  if [ -n "${FAKE_CODEX_RAW_JSON:-}" ]; then
    printf '%s\n' "$FAKE_CODEX_RAW_JSON"
    return
  fi
  config="$CODEX_HOME/config.toml"
  body=""
  sep=""
  while IFS='	' read -r key hash; do
    [ -z "$key" ] && continue
    if [ -n "${FAKE_CODEX_FORCE_STATUS:-}" ]; then
      status="$FAKE_CODEX_FORCE_STATUS"
    elif grep -Fq "[hooks.state.\"$key\"]" "$config" 2>/dev/null; then
      if grep -Fq "trusted_hash = \"$hash\"" "$config" 2>/dev/null; then
        status="trusted"
      else
        status="modified"
      fi
    else
      status="untrusted"
    fi
    body="$body$sep{\"key\":\"$key\",\"currentHash\":\"$hash\",\"trustStatus\":\"$status\"}"
    sep=","
  done <<SPEC
$FAKE_CODEX_HOOK_SPEC
SPEC
  printf '{"jsonrpc":"2.0","id":2,"result":{"data":[{"cwd":"%s","hooks":[%s],"warnings":[],"errors":[]}]}}\n' "$PWD" "$body"
}
while IFS= read -r line; do
  case "$line" in
    *'"initialize"'*)
      printf '{"jsonrpc":"2.0","id":1,"result":{"codexHome":"%s"}}\n' "$CODEX_HOME"
      printf '{"jsonrpc":"2.0","method":"remoteControl/status/changed","params":{}}\n'
      ;;
    *'"hooks/list"'*)
      emit_hooks
      ;;
  esac
done
`
	mustWriteExecutable(t, f.codexBin, script)
	t.Setenv("FAKE_CODEX_LOG", filepath.Join(filepath.Dir(f.codexHome), "codex.log"))
}

// setCodexHooks scripts which hook identities the fake Codex reports. Their
// trust status is derived from the live trust store, not pinned here.
func (f *fixture) setCodexHooks(t *testing.T, hooks ...codexHook) {
	t.Helper()
	var spec strings.Builder
	for _, hook := range hooks {
		fmt.Fprintf(&spec, "%s\t%s\n", hook.Key, hook.CurrentHash)
	}
	t.Setenv("FAKE_CODEX_HOOK_SPEC", spec.String())
	t.Setenv("FAKE_CODEX_FORCE_STATUS", "")
	t.Setenv("FAKE_CODEX_RAW_JSON", "")
}

// forceCodexHookStatus pins the status the fake Codex reports for every hook,
// for the cases that must exercise a specific status regardless of the store.
func (f *fixture) forceCodexHookStatus(t *testing.T, status string) {
	t.Helper()
	t.Setenv("FAKE_CODEX_FORCE_STATUS", status)
}

// mustCanonical resolves path the same way the maintainer does, so tests
// compare against the exact strings written into the trust store.
func mustCanonical(t *testing.T, path string) string {
	t.Helper()
	resolved, err := canonical(path)
	if err != nil {
		t.Fatalf("canonical %s: %v", path, err)
	}
	return resolved
}

func (f *fixture) writePackMarker(t *testing.T, commit string) {
	t.Helper()
	marker := "schema = 1\n" +
		"repository = \"https://github.com/gastownhall/gascity-packs.git\"\n" +
		fmt.Sprintf("commit = %q\n", commit)
	mustWrite(t, filepath.Join(filepath.Dir(f.pack), ".gc-bundled-pack-cache.toml"), marker)
}

// writeFakeGC scripts the gc binary: every invocation is appended to
// FAKE_GC_LOG and answered from the FAKE_* env fixtures. Case order matters —
// "import status" must match before the bare " status " arm.
func (f *fixture) writeFakeGC(t *testing.T) {
	t.Helper()
	script := `#!/bin/sh
set -eu
printf '%s\n' "$*" >>"$FAKE_GC_LOG"
case " $* " in
  *" import status "*)
    cat <<EOF
{"schema_version":"1","ok":true,"imports":[
  {"name":"pack:gascity","source":"https://github.com/gastownhall/gascity-packs/tree/main/gascity","pin":{"commit":"${FAKE_PACK_COMMIT}"}},
  {"name":"rig:smoke:gc","source":"https://github.com/gastownhall/gascity-packs/tree/main/gascity/roles","pin":{"commit":"${FAKE_PACK_COMMIT}"}}
]}
EOF
    ;;
  *" rig list "*)
    printf '{"ok":true,"rigs":[{"name":"smoke","path":"%s","hq":false}]}\n' "$FAKE_RIG"
    ;;
  *" agent list "*)
    printf '%s\n' "$FAKE_AGENTS_JSON"
    ;;
  *" doctor "*)
    printf '%s\n' "$FAKE_DOCTOR_JSON"
    ;;
  *" status "*)
    printf '%s\n' "$FAKE_STATUS_JSON"
    ;;
  *" session list "*)
    printf '%s\n' "$FAKE_SESSIONS_JSON"
    ;;
  *" bd ready "*)
    printf '%s\n' "$FAKE_READY_JSON"
    ;;
  *" bd update "*)
    printf '{"ok":true}\n'
    ;;
  *)
    printf 'unexpected fake gc command: %s\n' "$*" >&2
    exit 64
    ;;
esac
`
	mustWriteExecutable(t, f.gcBin, script)
}

// setAgents scripts the city's configured agent roster.
func (f *fixture) setAgents(t *testing.T, agents ...map[string]any) {
	t.Helper()
	if agents == nil {
		agents = []map[string]any{}
	}
	setJSONEnv(t, "FAKE_AGENTS_JSON", map[string]any{"agents": agents})
}

func (f *fixture) setDoctor(t *testing.T, value any)   { setJSONEnv(t, "FAKE_DOCTOR_JSON", value) }
func (f *fixture) setStatus(t *testing.T, value any)   { setJSONEnv(t, "FAKE_STATUS_JSON", value) }
func (f *fixture) setReady(t *testing.T, value any)    { setJSONEnv(t, "FAKE_READY_JSON", value) }
func (f *fixture) setSessions(t *testing.T, sessions []any) {
	setJSONEnv(t, "FAKE_SESSIONS_JSON", map[string]any{"ok": true, "sessions": sessions})
}

func setJSONEnv(t *testing.T, key string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", key, err)
	}
	t.Setenv(key, string(data))
}

func (f *fixture) options() Options {
	return Options{
		City:         f.city,
		Rig:          f.rig,
		GCBin:        f.gcBin,
		CodexBin:     f.codexBin,
		PackDir:      f.pack,
		SkillsSource: f.skills,
	}
}

// run executes one maintainer operation and returns its stdout and error.
func (f *fixture) run(t *testing.T, op func(Options) error, mutate func(*Options)) (string, error) {
	t.Helper()
	opts := f.options()
	var stdout, stderr bytes.Buffer
	opts.Stdout = &stdout
	opts.Stderr = &stderr
	if mutate != nil {
		mutate(&opts)
	}
	err := op(opts)
	return stdout.String(), err
}

func (f *fixture) logText(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(f.log)
	if err != nil {
		return ""
	}
	return string(data)
}

// treeDigest hashes every path, symlink target, and file body under root so a
// read-only command can be proven to write nothing.
func treeDigest(t *testing.T, root string) string {
	t.Helper()
	digest := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		digest.Write([]byte(rel))
		digest.Write([]byte{0})
		switch {
		case entry.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			digest.Write([]byte(link))
		case entry.Type().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			digest.Write(data)
		}
		digest.Write([]byte{0})
		return nil
	})
	if err != nil {
		t.Fatalf("digest %s: %v", root, err)
	}
	return fmt.Sprintf("%x", digest.Sum(nil))
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustWriteExecutable(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func wantErrContaining(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err.Error(), want)
	}
}

func TestPrepare_IdempotentAndStagesUnmodifiedUpstreamRuntime(t *testing.T) {
	f := newFixture(t)
	if _, err := f.run(t, Prepare, nil); err != nil {
		t.Fatalf("first prepare: %v", err)
	}
	if _, err := f.run(t, Prepare, nil); err != nil {
		t.Fatalf("second prepare: %v", err)
	}

	runtimeDir := filepath.Join(f.rig, ".gc", "agentops-maintainer-runtime", "versions", maintainerCommit)
	for _, name := range fixtureChecks {
		source := mustReadFile(t, filepath.Join(f.pack, "assets", "scripts", "checks", name))
		snapshot := mustReadFile(t, filepath.Join(runtimeDir, "gascity", "assets", "scripts", "checks", name))
		if snapshot != source {
			t.Errorf("snapshot of %s differs from upstream source", name)
		}
		wrapper := mustReadFile(t, filepath.Join(f.rig, ".gc", "scripts", "checks", name))
		if !strings.Contains(wrapper, managedMarker) {
			t.Errorf("wrapper %s lacks the managed marker", name)
		}
	}
	for _, skill := range requiredSkills {
		for _, sink := range []string{f.city, f.rig} {
			if !isRegularFile(filepath.Join(sink, ".codex", "skills", skill, "SKILL.md")) {
				t.Errorf("skill %s is not visible under %s/.codex/skills", skill, sink)
			}
		}
	}
}

func TestPrepare_RefusesWrongPinBeforeMutation(t *testing.T) {
	f := newFixture(t)
	t.Setenv("FAKE_PACK_COMMIT", "deadbeef")
	_, err := f.run(t, Prepare, nil)
	wantErrContaining(t, err, "official gascity workflow and rig-role pins")
	if _, statErr := os.Lstat(filepath.Join(f.rig, ".gc")); !os.IsNotExist(statErr) {
		t.Fatalf("prepare mutated the rig before refusing the pin: %v", statErr)
	}
}

func TestPrepare_RefusesPackCacheWrongPinBeforeMutation(t *testing.T) {
	f := newFixture(t)
	f.writePackMarker(t, "deadbeef")
	_, err := f.run(t, Prepare, nil)
	wantErrContaining(t, err, "pack cache marker does not match")
	if _, statErr := os.Lstat(filepath.Join(f.rig, ".gc")); !os.IsNotExist(statErr) {
		t.Fatalf("prepare mutated the rig before refusing the pack marker: %v", statErr)
	}
}

func TestPrepare_RefusesForeignCheckWrapper(t *testing.T) {
	f := newFixture(t)
	foreign := filepath.Join(f.rig, ".gc", "scripts", "checks", "build-artifact-valid.sh")
	mustWrite(t, foreign, "#!/bin/sh\nexit 0\n")
	_, err := f.run(t, Prepare, nil)
	wantErrContaining(t, err, "refusing to overwrite unmanaged check")
	if got := mustReadFile(t, foreign); got != "#!/bin/sh\nexit 0\n" {
		t.Fatalf("foreign wrapper was modified: %q", got)
	}
}

func TestPrepare_RefusesForeignRequiredSkill(t *testing.T) {
	f := newFixture(t)
	foreign := filepath.Join(f.city, ".codex", "skills", "using-gc", "SKILL.md")
	mustWrite(t, foreign, "# foreign\n")
	_, err := f.run(t, Prepare, nil)
	wantErrContaining(t, err, "does not resolve to the active skills source")
	if got := mustReadFile(t, foreign); got != "# foreign\n" {
		t.Fatalf("foreign skill was modified: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(f.rig, ".gc")); !os.IsNotExist(statErr) {
		t.Fatalf("prepare mutated the rig before refusing the foreign skill: %v", statErr)
	}
}

func TestCheck_IsReadOnlyAfterPrepare(t *testing.T) {
	f := newFixture(t)
	if _, err := f.run(t, Prepare, nil); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	before := treeDigest(t, f.city)
	stdout, err := f.run(t, Check, nil)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if got := treeDigest(t, f.city); got != before {
		t.Fatal("check modified the city tree")
	}
	if !strings.Contains(stdout, "maintainer runtime ready") {
		t.Fatalf("check stdout %q lacks the ready line", stdout)
	}
}

func TestRecoverAffinity_DryRunAndOnlyClearsStaleRequiredAssignment(t *testing.T) {
	f := newFixture(t)
	f.setReady(t, []any{
		map[string]any{
			"id":       "sm-stale",
			"assignee": "worker-old",
			"metadata": map[string]any{
				"gc.session_affinity": "require",
				"gc.routed_to":        "smoke/gc.run-operator",
			},
		},
		map[string]any{
			"id":       "sm-live",
			"assignee": "worker-live",
			"metadata": map[string]any{
				"gc.session_affinity": "require",
				"gc.routed_to":        "smoke/gc.run-operator",
			},
		},
		map[string]any{
			"id":       "sm-unrelated",
			"assignee": "worker-old",
			"metadata": map[string]any{"gc.routed_to": "smoke/gc.run-operator"},
		},
		map[string]any{
			"id":       "sm-unrouted",
			"assignee": "worker-old",
			"metadata": map[string]any{"gc.session_affinity": "require"},
		},
	})
	f.setSessions(t, []any{
		map[string]any{
			"id":           "af-live",
			"name":         "worker-live",
			"session_name": "worker-live",
			"state":        "active",
		},
	})

	stdout, err := f.run(t, RecoverAffinity, nil)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !strings.Contains(stdout, "would clear sm-stale") {
		t.Fatalf("dry-run stdout %q lacks the would-clear line", stdout)
	}
	if !strings.Contains(stdout, "Dry run only") {
		t.Fatalf("dry-run stdout %q lacks the dry-run notice", stdout)
	}
	if strings.Contains(" "+f.logText(t)+" ", " bd update ") {
		t.Fatal("dry run issued a bd update")
	}

	if _, err := f.run(t, RecoverAffinity, func(opts *Options) { opts.Apply = true }); err != nil {
		t.Fatalf("apply: %v", err)
	}
	var updates []string
	for line := range strings.SplitSeq(f.logText(t), "\n") {
		if strings.Contains(" "+line+" ", " bd update ") {
			updates = append(updates, line)
		}
	}
	if len(updates) != 1 {
		t.Fatalf("expected exactly 1 bd update, got %d: %v", len(updates), updates)
	}
	for _, unexpected := range []string{"sm-live", "sm-unrelated", "sm-unrouted"} {
		if strings.Contains(updates[0], unexpected) {
			t.Fatalf("bd update touched %s: %s", unexpected, updates[0])
		}
	}
	if !strings.Contains(updates[0], "sm-stale") {
		t.Fatalf("bd update did not target sm-stale: %s", updates[0])
	}
}

func TestRecoverAffinity_ReportsNoStaleAssignments(t *testing.T) {
	f := newFixture(t)
	stdout, err := f.run(t, RecoverAffinity, nil)
	if err != nil {
		t.Fatalf("recover-affinity: %v", err)
	}
	if !strings.Contains(stdout, "No stale required session-affinity assignments found.") {
		t.Fatalf("stdout %q lacks the no-stale notice", stdout)
	}
}
