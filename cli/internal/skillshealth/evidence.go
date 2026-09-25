package skillshealth

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// LocatedObservation is an inspectable fact or bounded suspicion, never a score.
type LocatedObservation struct {
	Kind         string   `json:"kind"`
	Path         string   `json:"path"`
	Line         int      `json:"line"`
	Snippet      string   `json:"snippet"`
	Meaning      string   `json:"meaning"`
	ReachableVia []string `json:"reachable_via"`
}

type ConformanceEvidence struct {
	Status     string               `json:"status"`
	Profile    string               `json:"profile"`
	AppliesTo  string               `json:"applies_to"`
	Checked    []string             `json:"checked"`
	NotChecked []string             `json:"not_checked"`
	Findings   []LocatedObservation `json:"findings"`
}

type EffectEvidence struct {
	Status       string               `json:"status"`
	Observations []LocatedObservation `json:"observations"`
	Inspected    []string             `json:"inspected"`
	Limitations  []string             `json:"limitations"`
}

type BehavioralEvidence struct {
	Status    string `json:"status"`
	TrialsRun int    `json:"trials_run"`
	Reason    string `json:"reason"`
	Owner     string `json:"owner"`
}

type AuthoringEvidence struct {
	Gating      bool                 `json:"gating"`
	Suspicions  []LocatedObservation `json:"suspicions"`
	Limitations string               `json:"limitations"`
}

// EvidenceReport deliberately has no aggregate verdict or optimization rank.
type EvidenceReport struct {
	SchemaVersion string              `json:"schema_version"`
	Target        string              `json:"target"`
	Conformance   ConformanceEvidence `json:"conformance"`
	Effects       EffectEvidence      `json:"effects"`
	Behavior      BehavioralEvidence  `json:"behavior"`
	Authoring     AuthoringEvidence   `json:"authoring"`
}

// AuditEvidence inspects source bytes only. It never executes package instructions.
func AuditEvidence(repo, target, profile string) (*EvidenceReport, error) {
	root, path, profile, err := auditTarget(repo, target, profile)
	if err != nil {
		return nil, err
	}
	r := &EvidenceReport{
		SchemaVersion: "skill-audit.v2", Target: path,
		Conformance: ConformanceEvidence{Status: "PASS", Profile: profile, AppliesTo: "selected package static contract only", Checked: []string{"regular UTF-8 SKILL.md", "YAML identity and description", "literal local resource closure"}, NotChecked: []string{"semantic completeness", "installed host behavior and invocation policy"}, Findings: []LocatedObservation{}},
		Effects:     EffectEvidence{Status: "NOT_PROVEN", Observations: []LocatedObservation{}, Inspected: []string{}, Limitations: []string{"Only SKILL.md and literal package-local linked or referenced resources are inspected; conditional and computed paths, external commands and dependencies are unresolved.", "Matches are declared or lexical effect observations, not proof of execution, authorization, complete reachability or safety. Prohibitions and examples can also match.", "No runtime controls, source access, disclosure destinations, credentials, approval or interruption behavior were attested."}},
		Behavior:    BehavioralEvidence{Status: "NOT_PROVEN", TrialsRun: 0, Reason: "No behavioral evidence was supplied or evaluated; static conformance does not establish effectiveness.", Owner: "skill-eval / probe-skill / native skill trials"},
		Authoring:   AuthoringEvidence{Suspicions: []LocatedObservation{}, Limitations: "Located wording suspicions are review prompts only. A necessary prohibition is not a defect; no semantic judgment or count-based rank is inferred."},
	}
	skill := filepath.Join(path, "SKILL.md")
	content, err := regularText(skill)
	if err != nil {
		r.addFinding("INVALID_SKILL", "SKILL.md", 1, "", err.Error())
		return r, nil
	}
	data, body, err := auditFrontmatter(content)
	if err != nil {
		r.addFinding("INVALID_FRONTMATTER", "SKILL.md", 1, "", err.Error())
		return r, nil
	}
	if profile == "canonical" {
		fs, e := CheckSource(root, []string{path}, false)
		if e != nil {
			return nil, e
		}
		r.Conformance.Checked = append(r.Conformance.Checked, "AgentOps canonical source metadata and scaffold state")
		for _, f := range fs {
			r.addFinding(f.Code, "SKILL.md", 1, "", f.Message)
		}
	} else {
		checkAuditIdentity(r, data, filepath.Base(path), profile == "portable")
	}
	if strings.TrimSpace(body) == "" {
		r.addFinding("EMPTY_BODY", "SKILL.md", 1, "", "instruction body must be nonempty")
	}
	if profile == "portable" {
		r.Conformance.Checked = append(r.Conformance.Checked, "portable field allowlist and types")
		checkPortableFields(r, data)
	}
	if profile == "external-observation" {
		r.Conformance.NotChecked = append(r.Conformance.NotChecked, "AgentOps repository metadata", "portable and host-specific field policies")
	}
	boundary := filepath.Dir(path)
	if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
		boundary = root
	}
	scanAuditResources(r, path, boundary, content)
	return r, nil
}

func auditTarget(repo, target, profile string) (string, string, string, error) {
	root, err := filepath.Abs(repo)
	if err != nil {
		return "", "", "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", "", err
	}
	for _, part := range strings.Split(filepath.ToSlash(target), "/") {
		if part == ".." {
			return "", "", "", fmt.Errorf("target traversal is not accepted: %s", target)
		}
	}
	path, err := filepath.Abs(target)
	if err != nil {
		return "", "", "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", "", err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || resolved != path {
		return "", "", "", fmt.Errorf("target must be a real package directory: %s", path)
	}
	if profile == "" {
		switch filepath.Dir(path) {
		case filepath.Join(root, "skills"):
			profile = "canonical"
		case filepath.Join(root, "skills-codex"):
			profile = "portable"
		default:
			profile = "external-observation"
		}
	}
	if profile == "repo-runtime" {
		profile = "canonical"
	}
	if profile != "canonical" && profile != "portable" && profile != "external-observation" {
		return "", "", "", fmt.Errorf("unknown profile %q; use canonical, portable or external-observation", profile)
	}
	return root, path, profile, nil
}

func (r *EvidenceReport) addFinding(kind, path string, line int, snippet, meaning string) {
	r.Conformance.Status = "FAIL"
	r.Conformance.Findings = append(r.Conformance.Findings, LocatedObservation{kind, path, line, snippet, meaning, []string{"SKILL.md"}})
}

func regularText(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("must be a regular non-symlink file: %s", path)
	}
	if info.Size() > 2*1024*1024 {
		return "", fmt.Errorf("resource exceeds static inspection limit of 2 MiB: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("resource is not UTF-8: %s", path)
	}
	return string(data), nil
}

func auditFrontmatter(content string) (map[string]any, string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return nil, "", fmt.Errorf("leading YAML frontmatter is missing")
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		return nil, "", fmt.Errorf("YAML frontmatter is not closed")
	}
	var data map[string]any
	if err := yaml.Unmarshal([]byte(content[4:4+end]), &data); err != nil {
		return nil, "", err
	}
	if data == nil {
		return nil, "", fmt.Errorf("frontmatter must be a mapping")
	}
	return data, content[4+end+5:], nil
}

func checkAuditIdentity(r *EvidenceReport, data map[string]any, slug string, portable bool) {
	name, ok := data["name"].(string)
	if !ok || name != slug || !portableSlug.MatchString(name) || len(name) > 64 {
		r.addFinding("NAME_MISMATCH", "SKILL.md", 1, "", "name must match directory and use 1-64 lowercase letters/digits with single internal hyphens")
	}
	desc, ok := data["description"].(string)
	if !ok || strings.TrimSpace(desc) == "" || (portable && utf8.RuneCountInString(desc) > 1024) {
		r.addFinding("INVALID_DESCRIPTION", "SKILL.md", 1, "", "description must be a nonempty string (portable maximum 1024 characters)")
	}
}

func checkPortableFields(r *EvidenceReport, data map[string]any) {
	allowed := map[string]bool{"name": true, "description": true, "license": true, "compatibility": true, "metadata": true, "allowed-tools": true}
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !allowed[k] {
			r.addFinding("HOST_ONLY_FIELD", "SKILL.md", 1, k, "field is unsupported by the selected portable profile")
		}
	}
	for _, k := range []string{"license", "compatibility", "allowed-tools"} {
		if v, ok := data[k]; ok {
			s, valid := v.(string)
			if !valid || strings.TrimSpace(s) == "" || (k == "compatibility" && utf8.RuneCountInString(s) > 500) || (k == "allowed-tools" && strings.ContainsAny(s, ",\t\n")) {
				r.addFinding("INVALID_FIELD", "SKILL.md", 1, k, "invalid portable field type or value")
			}
		}
	}
	if v, ok := data["metadata"]; ok {
		m, valid := v.(map[string]any)
		if valid {
			for _, v := range m {
				if _, ok := v.(string); !ok {
					valid = false
				}
			}
		}
		if !valid {
			r.addFinding("INVALID_METADATA", "SKILL.md", 1, "metadata", "portable metadata must map strings to strings")
		}
	}
}
