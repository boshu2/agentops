// Package gcmaintainer prepares and qualifies the stock Gas City maintainer
// pack for a rig without owning a pack. It is the Go port of the retired
// scripts/gc-maintainer-ops.sh logic (ADR-0016: skill logic ships in Go via
// ao; shell stays thin glue), exposed as `ao gc prepare|check|recover-affinity`.
//
// prepare verifies the official workflow and rig-role pins, snapshots upstream
// validation assets unchanged under the rig's ignored .gc directory, installs
// AgentOps-owned check wrappers, and links AgentOps skills into city/rig Codex
// sinks. check is read-only. recover-affinity only considers ready formula
// beads with gc.session_affinity=require and never re-slings work.
package gcmaintainer

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	// maintainerCommit pins the accepted official gascity pack release.
	maintainerCommit = "3b3b89f2011e06d84459aa7bea1552382f13930a"
	workflowSource   = "https://github.com/gastownhall/gascity-packs/tree/main/gascity"
	rolesSource      = workflowSource + "/roles"
	managedMarker    = "managed-by: agentops gc-maintainer-ops"
)

// requiredSkills are the AgentOps skills that must be visible to provider
// sessions in the city and rig Codex sinks.
var requiredSkills = []string{"using-gc", "plan", "implement", "test", "validate"}

// Options carries the caller-supplied inputs for one maintainer operation.
type Options struct {
	// City is the Gas City root directory (required).
	City string
	// Rig is the rig directory inside the city (required).
	Rig string
	// GCBin is the Gas City 1.4 binary; defaults to `gc` on PATH.
	GCBin string
	// PackDir overrides auto-detection of the resolved official pack root.
	PackDir string
	// SkillsSource overrides resolution of the AgentOps skills directory that
	// city/rig Codex sinks must link to.
	SkillsSource string
	// Apply makes recover-affinity clear stale assignments; default is dry-run.
	Apply bool
	// Stdout and Stderr receive operation output and warnings.
	Stdout io.Writer
	Stderr io.Writer
}

// ops holds the resolved state shared by every maintainer operation.
type ops struct {
	city      string
	rig       string
	rigName   string
	gcBin     string
	packDir   string
	pythonBin string
	// skillsSource is the resolved AgentOps skills directory; empty for
	// operations that do not touch skill links (recover-affinity).
	skillsSource string

	runtimeRoot string
	runtime     string
	checksDir   string

	apply  bool
	stdout io.Writer
	stderr io.Writer
}

// Prepare stages the contained maintainer runtime, installs managed check
// wrappers, links AgentOps skills, and verifies rig health.
func Prepare(opts Options) error {
	o, err := resolve(opts, true)
	if err != nil {
		return err
	}
	if err := o.prepareRuntime(); err != nil {
		return err
	}
	if err := o.checkServiceBinary(); err != nil {
		return err
	}
	if err := o.checkGCHealth(); err != nil {
		return err
	}
	o.reportReady()
	return nil
}

// Check verifies an already-prepared maintainer runtime read-only.
func Check(opts Options) error {
	o, err := resolve(opts, true)
	if err != nil {
		return err
	}
	if err := o.checkRuntime(); err != nil {
		return err
	}
	if err := o.checkSkillLinks(); err != nil {
		return err
	}
	if err := o.checkServiceBinary(); err != nil {
		return err
	}
	if err := o.checkGCHealth(); err != nil {
		return err
	}
	o.reportReady()
	return nil
}

// RecoverAffinity clears (or, by default, only reports) ready formula beads
// whose required session affinity points at a session that is no longer live.
func RecoverAffinity(opts Options) error {
	o, err := resolve(opts, false)
	if err != nil {
		return err
	}
	return o.recoverAffinity()
}

func (o *ops) reportReady() {
	fmt.Fprintf(o.stdout, "maintainer runtime ready: city=%s rig=%s commit=%s\n",
		o.city, o.rigName, maintainerCommit)
}

// resolve validates the caller inputs and establishes the shared preamble: an
// exact non-HQ rig, the pinned official imports, the resolved pack, and a
// PyYAML-capable Python. When needSkills is set it also resolves the AgentOps
// skills source used for Codex sink links.
func resolve(opts Options, needSkills bool) (*ops, error) {
	o := &ops{apply: opts.Apply, stdout: opts.Stdout, stderr: opts.Stderr}
	if o.stdout == nil {
		o.stdout = os.Stdout
	}
	if o.stderr == nil {
		o.stderr = os.Stderr
	}

	if opts.City == "" || opts.Rig == "" {
		return nil, fmt.Errorf("--city and --rig are required")
	}
	if !isDir(opts.City) {
		return nil, fmt.Errorf("city directory does not exist: %s", opts.City)
	}
	if !isDir(opts.Rig) {
		return nil, fmt.Errorf("rig directory does not exist: %s", opts.Rig)
	}
	var err error
	if o.city, err = canonical(opts.City); err != nil {
		return nil, fmt.Errorf("resolve city path: %w", err)
	}
	if o.rig, err = canonical(opts.Rig); err != nil {
		return nil, fmt.Errorf("resolve rig path: %w", err)
	}

	if o.gcBin, err = resolveGCBin(opts.GCBin); err != nil {
		return nil, err
	}
	if needSkills {
		if o.skillsSource, err = resolveSkillsSource(opts.SkillsSource); err != nil {
			return nil, err
		}
	}

	if o.rigName, err = o.resolveRigName(); err != nil {
		return nil, err
	}
	if err = o.verifyImportPins(); err != nil {
		return nil, err
	}
	if o.packDir, err = resolvePackDir(opts.PackDir); err != nil {
		return nil, err
	}
	if err = validatePackDir(o.packDir); err != nil {
		return nil, err
	}
	if o.pythonBin, err = selectPython(); err != nil {
		return nil, err
	}

	o.runtimeRoot = filepath.Join(o.rig, ".gc", "agentops-maintainer-runtime")
	o.runtime = filepath.Join(o.runtimeRoot, "versions", maintainerCommit)
	o.checksDir = filepath.Join(o.rig, ".gc", "scripts", "checks")
	return o, nil
}

func resolveGCBin(explicit string) (string, error) {
	bin := explicit
	if bin == "" {
		bin, _ = exec.LookPath("gc")
	}
	if bin == "" || !isExecutableFile(bin) {
		return "", fmt.Errorf("gc binary is not executable")
	}
	return canonical(bin)
}

// resolveRigName matches the caller's rig path against the city's registered
// non-HQ rigs and requires exactly one exact match.
func (o *ops) resolveRigName() (string, error) {
	var payload struct {
		Rigs []struct {
			Name string `json:"name"`
			Path string `json:"path"`
			HQ   bool   `json:"hq"`
		} `json:"rigs"`
	}
	if err := o.gcJSON(&payload, "cannot list city rigs", "--city", o.city, "rig", "list", "--json"); err != nil {
		return "", err
	}
	name := ""
	matches := 0
	for _, rig := range payload.Rigs {
		if rig.HQ || rig.Name == "" || rig.Path == "" {
			continue
		}
		path, err := canonical(rig.Path)
		if err != nil {
			continue
		}
		if path == o.rig {
			name = rig.Name
			matches++
		}
	}
	if matches != 1 {
		return "", fmt.Errorf("rig path is not an exact non-HQ rig in this city: %s", o.rig)
	}
	return name, nil
}

// verifyImportPins requires the official gascity workflow import and this
// rig's role import to both be installed at the accepted commit.
func (o *ops) verifyImportPins() error {
	var payload struct {
		OK      bool `json:"ok"`
		Imports []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
			Pin    struct {
				Commit string `json:"commit"`
			} `json:"pin"`
		} `json:"imports"`
	}
	if err := o.gcJSON(&payload, "cannot inspect installed imports", "--city", o.city, "import", "status", "--json"); err != nil {
		return err
	}
	workflowPinned := false
	rolesPinned := false
	rigImport := "rig:" + o.rigName + ":gc"
	for _, imp := range payload.Imports {
		if imp.Source == workflowSource && imp.Pin.Commit == maintainerCommit {
			workflowPinned = true
		}
		if imp.Name == rigImport && imp.Source == rolesSource && imp.Pin.Commit == maintainerCommit {
			rolesPinned = true
		}
	}
	if !payload.OK || !workflowPinned || !rolesPinned {
		return fmt.Errorf("official gascity workflow and rig-role pins are not both installed at %s", maintainerCommit)
	}
	return nil
}

// canonical resolves path to an absolute path with all symlinks evaluated,
// matching the shell helper's realpath behavior.
func canonical(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}
