package gates

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkflowCoverage compares the Go gate registry with one GitHub Actions
// workflow. It is diagnostic by default; --require-workflow-parity can promote
// MissingScripts to a blocking gate once CI is ready to flip authority.
type WorkflowCoverage struct {
	WorkflowPath            string              `json:"workflow_path"`
	WorkflowScriptCount     int                 `json:"workflow_script_count"`
	RegistryScriptCount     int                 `json:"registry_script_count"`
	MissingScriptCount      int                 `json:"missing_script_count"`
	MissingBlockingCount    int                 `json:"missing_blocking_count"`
	MissingAdvisoryCount    int                 `json:"missing_advisory_count"`
	MissingDeferredCount    int                 `json:"missing_deferred_count"`
	RegistryOnlyScriptCount int                 `json:"registry_only_script_count"`
	WorkflowScripts         []WorkflowScriptRef `json:"workflow_scripts"`
	RegistryScripts         []string            `json:"registry_scripts"`
	MissingScripts          []string            `json:"missing_scripts"`
	MissingBlockingScripts  []string            `json:"missing_blocking_scripts"`
	MissingAdvisoryScripts  []string            `json:"missing_advisory_scripts"`
	MissingDeferredScripts  []DeferredScriptRef `json:"missing_deferred_scripts"`
	RegistryOnlyScripts     []string            `json:"registry_only_scripts"`
}

// WorkflowScriptRef records a script reference found in a workflow run block.
type WorkflowScriptRef struct {
	Script   string `json:"script"`
	Job      string `json:"job"`
	Step     string `json:"step,omitempty"`
	Advisory bool   `json:"advisory"`
}

// DeferredScriptRef records a workflow script that is intentionally not counted
// as an actionable Go-gate parity gap.
type DeferredScriptRef struct {
	Script string `json:"script"`
	Reason string `json:"reason"`
}

type workflowFile struct {
	Jobs map[string]workflowJob `yaml:"jobs"`
}

type workflowJob struct {
	ContinueOnError any            `yaml:"continue-on-error"`
	Steps           []workflowStep `yaml:"steps"`
}

type workflowStep struct {
	Name            string `yaml:"name"`
	Run             string `yaml:"run"`
	ContinueOnError any    `yaml:"continue-on-error"`
}

var workflowScriptPattern = regexp.MustCompile("(?:^|[\\s\"'`])(?:\\./)?((?:scripts|skills/[A-Za-z0-9._/-]+/scripts)/[A-Za-z0-9._/-]+\\.(?:sh|py))")

var deferredWorkflowScripts = map[string]string{
	"scripts/check-agents-hash-snapshot.sh":      "stateful capture/diff bracket around the Go test phase",
	"scripts/check-cmd-ao-coverage.sh":           "requires the coverage profile produced by the preceding Go test step",
	"scripts/check-eval-workbench.sh":            "requires validate.yml's prior workbench Python venv setup",
	"scripts/check-outcomes-holdout-leak.sh":     "validate.yml derives a dynamic payload list; needs native input modeling before delegation",
	"scripts/eval-agent-harness.sh":              "validate.yml asserts dry-run JSON shape after invoking the script",
	"scripts/lint-evidence-lines.sh":             "PR-body advisory that requires GitHub PR context and GH_TOKEN",
	"scripts/release-smoke-test.sh":              "workflow uses --skip-build after a prior build; standalone form should become a composed/native check",
	"scripts/security-gate.sh":                   "requires validate.yml's security toolchain setup and scanner policy context",
	"scripts/skill-eval.sh":                      "changed-skill scoped gate with pinned ms toolchain/cache setup",
	"scripts/test-agentops-contract-canaries.sh": "requires validate.yml's canary toolchain setup and artifact context",
	"scripts/verify-gate-claim.sh":               "AP7 verifier needs completed sibling workflow logs from the summary job",
}

// RegistryWorkflowCoverage returns workflow-vs-registry script coverage.
func RegistryWorkflowCoverage(reg *Registry, repoRoot, workflowRel string) (*WorkflowCoverage, error) {
	if reg == nil {
		return nil, fmt.Errorf("gates: registry required")
	}
	if repoRoot == "" {
		return nil, fmt.Errorf("gates: repo root required")
	}
	if workflowRel == "" {
		workflowRel = filepath.Join(".github", "workflows", "validate.yml")
	}
	workflowPath := workflowRel
	if !filepath.IsAbs(workflowPath) {
		workflowPath = filepath.Join(repoRoot, workflowRel)
	}
	refs, workflowScripts, err := workflowScriptRefs(workflowPath)
	if err != nil {
		return nil, err
	}
	registryScripts := registryBackingScripts(reg)

	missing := sortedSetDiff(workflowScripts, registryScripts)
	missingBlocking, missingAdvisory, missingDeferred := classifyMissingWorkflowScripts(refs, registryScripts)
	registryOnly := sortedSetDiff(registryScripts, workflowScripts)
	return &WorkflowCoverage{
		WorkflowPath:            workflowRel,
		WorkflowScriptCount:     len(workflowScripts),
		RegistryScriptCount:     len(registryScripts),
		MissingScriptCount:      len(missing),
		MissingBlockingCount:    len(missingBlocking),
		MissingAdvisoryCount:    len(missingAdvisory),
		MissingDeferredCount:    len(missingDeferred),
		RegistryOnlyScriptCount: len(registryOnly),
		WorkflowScripts:         refs,
		RegistryScripts:         setToSortedSlice(registryScripts),
		MissingScripts:          missing,
		MissingBlockingScripts:  missingBlocking,
		MissingAdvisoryScripts:  missingAdvisory,
		MissingDeferredScripts:  missingDeferred,
		RegistryOnlyScripts:     registryOnly,
	}, nil
}

// GitHubAnnotations emits one annotation summarizing the parity gap.
func (c *WorkflowCoverage) GitHubAnnotations(w io.Writer) {
	if c == nil || c.MissingBlockingCount == 0 {
		return
	}
	msg := fmt.Sprintf("%d blocking validate.yml script(s) are not registered in ao gate check: %s",
		c.MissingBlockingCount,
		strings.Join(c.MissingBlockingScripts, ", "),
	)
	fmt.Fprintf(w, "::warning title=%s::%s\n",
		escapeGitHubAnnotation("ao-gate-workflow-coverage"),
		escapeGitHubAnnotation(msg),
	)
}

func classifyMissingWorkflowScripts(refs []WorkflowScriptRef, registryScripts map[string]struct{}) ([]string, []string, []DeferredScriptRef) {
	type scriptClass struct {
		blocking bool
		advisory bool
	}
	classes := map[string]scriptClass{}
	for _, ref := range refs {
		if _, ok := registryScripts[ref.Script]; ok {
			continue
		}
		class := classes[ref.Script]
		if ref.Advisory {
			class.advisory = true
		} else {
			class.blocking = true
		}
		classes[ref.Script] = class
	}

	var blocking []string
	var advisory []string
	var deferred []DeferredScriptRef
	for script, class := range classes {
		if reason, ok := deferredWorkflowScripts[script]; ok {
			deferred = append(deferred, DeferredScriptRef{Script: script, Reason: reason})
			continue
		}
		if class.blocking {
			blocking = append(blocking, script)
		} else if class.advisory {
			advisory = append(advisory, script)
		}
	}
	sort.Strings(blocking)
	sort.Strings(advisory)
	sort.Slice(deferred, func(i, j int) bool { return deferred[i].Script < deferred[j].Script })
	return blocking, advisory, deferred
}

func workflowScriptRefs(path string) ([]WorkflowScriptRef, map[string]struct{}, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- repo-local workflow path supplied by caller.
	if err != nil {
		return nil, nil, fmt.Errorf("gates: read workflow %s: %w", path, err)
	}
	var wf workflowFile
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		return nil, nil, fmt.Errorf("gates: parse workflow %s: %w", path, err)
	}

	var refs []WorkflowScriptRef
	seenRefs := map[string]struct{}{}
	scripts := map[string]struct{}{}
	for jobName, job := range wf.Jobs {
		for _, step := range job.Steps {
			if step.Run == "" {
				continue
			}
			for _, script := range scriptsInRunBlock(step.Run) {
				ref := WorkflowScriptRef{
					Script:   script,
					Job:      jobName,
					Step:     step.Name,
					Advisory: boolish(job.ContinueOnError) || boolish(step.ContinueOnError),
				}
				key := ref.Job + "\x00" + ref.Step + "\x00" + ref.Script
				if _, ok := seenRefs[key]; ok {
					continue
				}
				seenRefs[key] = struct{}{}
				refs = append(refs, ref)
				scripts[script] = struct{}{}
			}
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Script != refs[j].Script {
			return refs[i].Script < refs[j].Script
		}
		if refs[i].Job != refs[j].Job {
			return refs[i].Job < refs[j].Job
		}
		return refs[i].Step < refs[j].Step
	})
	return refs, scripts, nil
}

func scriptsInRunBlock(run string) []string {
	seen := map[string]struct{}{}
	for _, line := range strings.Split(run, "\n") {
		if !workflowLineMayInvokeScript(line) {
			continue
		}
		matches := workflowScriptPattern.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			script := strings.TrimPrefix(m[1], "./")
			seen[script] = struct{}{}
		}
	}
	return setToSortedSlice(seen)
}

func workflowLineMayInvokeScript(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return false
	}
	if strings.HasPrefix(trimmed, "chmod ") ||
		strings.HasPrefix(trimmed, "echo ") ||
		strings.HasPrefix(trimmed, "printf ") {
		return false
	}
	return workflowScriptPattern.MatchString(trimmed)
}

func registryBackingScripts(reg *Registry) map[string]struct{} {
	out := map[string]struct{}{}
	for _, check := range reg.All() {
		if check.Backing == "" {
			continue
		}
		script := check.Backing
		if !strings.Contains(script, "/") {
			script = filepath.Join("scripts", script)
		}
		out[filepath.ToSlash(script)] = struct{}{}
	}
	return out
}

func boolish(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	default:
		return false
	}
}

func setToSortedSlice(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func sortedSetDiff(a, b map[string]struct{}) []string {
	var out []string
	for v := range a {
		if _, ok := b[v]; !ok {
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
