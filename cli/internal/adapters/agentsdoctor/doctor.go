// practices: [wiki-knowledge-surface, design-by-contract]
package agentsdoctor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/boshu2/agentops/cli/internal/adapters/agentslint"
	"github.com/boshu2/agentops/cli/internal/adapters/agentsurface"
)

// Options configures the combined .agents health check.
type Options struct {
	Contract  string
	Script    string
	AgentsDir string
	SkillsDir string
	JSON      bool
	Strict    bool
	Stdout    io.Writer
	Stderr    io.Writer
}

// Report is the shape returned by `ao agents doctor --json`.
type Report struct {
	Inventory        agentsurface.Inventory `json:"inventory"`
	LintScript       string                 `json:"lint_script"`
	LintExitCode     int                    `json:"lint_exit_code"`
	LintClean        bool                   `json:"lint_clean"`
	OrphanSkills     []string               `json:"orphan_skills"`
	UndocumentedDirs []UndocumentedDir      `json:"undocumented_dirs"`
	Strict           bool                   `json:"strict"`
}

// UndocumentedDir is an entry under .agents/ that is neither catalogued in the
// contract allowlist nor a skill-owned subdir.
type UndocumentedDir struct {
	Name    string `json:"name"`
	FixHint string `json:"fix_hint"`
}

// StrictError is returned when strict mode finds orphan skills or undocumented
// .agents directories.
type StrictError struct {
	OrphanSkills     int
	UndocumentedDirs int
}

func (e *StrictError) Error() string {
	return fmt.Sprintf("strict: %d orphan skills, %d undocumented dirs", e.OrphanSkills, e.UndocumentedDirs)
}

// Run executes the combined inspect/lint/orphan report.
func Run(opts Options) error {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}

	contractData, err := os.ReadFile(opts.Contract)
	if err != nil {
		return fmt.Errorf("reading contract %s: %w", opts.Contract, err)
	}

	inv := agentsurface.Inventory{
		Contract:  opts.Contract,
		Allowlist: agentsurface.ParseAllowlist(string(contractData)),
		Skills:    agentsurface.DiscoverActiveSkills(opts.SkillsDir),
	}

	lintStdout := opts.Stdout
	if opts.JSON {
		lintStdout = io.Discard
	}
	lintExit, lintErr := runLint(opts.Script, opts.JSON, lintStdout, opts.Stderr)
	lintClean := lintErr == nil

	orphanSkills := FindOrphanSkills(inv.Skills, opts.AgentsDir)
	undocumented := FindUndocumentedDirs(opts.AgentsDir, inv.Allowlist, inv.Skills)

	report := Report{
		Inventory:        inv,
		LintScript:       opts.Script,
		LintExitCode:     lintExit,
		LintClean:        lintClean,
		OrphanSkills:     orphanSkills,
		UndocumentedDirs: undocumented,
		Strict:           opts.Strict,
	}

	if opts.JSON {
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else {
		PrintText(opts.Stdout, report)
	}

	if !lintClean {
		return &agentslint.Error{ExitCode: lintExit, Script: opts.Script}
	}
	if opts.Strict && (len(orphanSkills) > 0 || len(undocumented) > 0) {
		return &StrictError{
			OrphanSkills:     len(orphanSkills),
			UndocumentedDirs: len(undocumented),
		}
	}
	return nil
}

func runLint(scriptPath string, jsonOut bool, stdout, stderr io.Writer) (int, error) {
	if _, err := os.Stat(scriptPath); err != nil {
		return -1, fmt.Errorf("lint script not found at %s: %w", scriptPath, err)
	}
	cmdArgs := []string{}
	if jsonOut {
		cmdArgs = append(cmdArgs, "--json")
	}
	c := exec.Command("bash", append([]string{scriptPath}, cmdArgs...)...) // #nosec G204 -- operator-configured repo script path.
	c.Stdout = stdout
	c.Stderr = stderr
	err := c.Run()
	if err == nil {
		return 0, nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), err
	}
	return -1, fmt.Errorf("running %s: %w", scriptPath, err)
}

// FindOrphanSkills returns the names of skills that have no corresponding
// .agents/<skill>/ subdir. Result is sorted.
func FindOrphanSkills(skills []string, agentsDir string) []string {
	out := []string{}
	for _, name := range skills {
		path := filepath.Join(agentsDir, name)
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// FindUndocumentedDirs returns entries under .agents/ that are neither in the
// catalogued allowlist nor a skill-owned subdir. Hidden dirs (.git, .archive)
// are skipped. Result is sorted by name.
func FindUndocumentedDirs(agentsDir string, allowlist, skills []string) []UndocumentedDir {
	allowed := make(map[string]bool, len(allowlist)+len(skills))
	for _, e := range allowlist {
		allowed[e] = true
	}
	for _, e := range skills {
		allowed[e] = true
	}

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil
	}
	out := []UndocumentedDir{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "" || name[0] == '.' {
			continue
		}
		if allowed[name] {
			continue
		}
		out = append(out, UndocumentedDir{
			Name:    name,
			FixHint: fmt.Sprintf("add `%s` to docs/contracts/agents-write-surfaces.md allowlist or remove the dir", name),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// PrintText renders the human report.
func PrintText(out io.Writer, r Report) {
	fmt.Fprintln(out, "ao agents doctor")
	fmt.Fprintf(out, "  Contract: %s\n", r.Inventory.Contract)
	fmt.Fprintf(out, "  Catalogued surfaces: %d\n", len(r.Inventory.Allowlist))
	fmt.Fprintf(out, "  Skill-owned subdirs: %d\n", len(r.Inventory.Skills))
	if r.LintClean {
		fmt.Fprintln(out, "  Lint: PASS")
	} else {
		fmt.Fprintf(out, "  Lint: FAIL (exit %d, script=%s)\n", r.LintExitCode, r.LintScript)
	}
	fmt.Fprintln(out)

	fmt.Fprintf(out, "Orphan skills (skills/<name>/ with no .agents/<name>/): %d\n", len(r.OrphanSkills))
	for _, name := range r.OrphanSkills {
		fmt.Fprintf(out, "  %s -- fix: create .agents/%s/ or document why it's exempt\n", name, name)
	}
	fmt.Fprintln(out)

	fmt.Fprintf(out, "Undocumented dirs (.agents/<name>/ not catalogued, not a skill): %d\n", len(r.UndocumentedDirs))
	for _, d := range r.UndocumentedDirs {
		fmt.Fprintf(out, "  %s -- fix: %s\n", d.Name, d.FixHint)
	}
}
