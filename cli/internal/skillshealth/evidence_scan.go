package skillshealth

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var portableSlug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var auditLink = regexp.MustCompile(`\[[^\]]*\]\(([^\s)#?]+)(?:[^)]*)\)`)
var auditInvocation = regexp.MustCompile(`(?:bash|sh|python3?|node|exec)\s+["'` + "`" + `]?((?:\./)?scripts/[a-zA-Z0-9_./-]+\.(?:sh|py|js))`)
var auditNoop = regexp.MustCompile(`(?i)\b(be careful|use best practices|ensure quality|think carefully)\b`)
var effectPatterns = []struct {
	kind    string
	pattern *regexp.Regexp
	meaning string
}{
	{"read", regexp.MustCompile(`(?i)\b(cat|readfile|read_text|open|read)\b`), "Text names a read operation; source authorization and sensitivity are not established."},
	{"disclosure", regexp.MustCompile(`(?i)\b(upload|send|post|publish|disclos|transmit)[a-z]*\b|--data|--upload-file`), "Text names possible external disclosure; destination and runtime controls are not established."},
	{"credentials", regexp.MustCompile(`(?i)\b(credential|secret|password|token|api.key)[a-z_]*\b`), "Text references credentials or secrets; this can be a prohibition, not an access."},
	{"network", regexp.MustCompile(`(?i)\b(curl|wget|https?://|fetch|requests\.)`), "Text names a network operation or endpoint; no network request was made."},
	{"execution", regexp.MustCompile(`(?i)\b(bash|sh|exec|subprocess|eval|os.system)\b`), "Text names command execution; computed arguments and runtime containment are unresolved."},
	{"mutation", regexp.MustCompile(`(?i)\b(rm|unlink|writefile|write_text|mkdir|delete|remove|overwrite)\b|>{1,2}\s*[^=]`), "Text names a possible write or deletion; necessity, approval and rollback are not established."},
	{"remote-code-path", regexp.MustCompile(`(?i)\b(curl|wget)\b[^\n]*\|\s*(bash|sh)\b`), "If executed, remote response bytes flow directly into a shell interpreter. Inspect the cited invocation and its authorization before execution; static presence does not attest runtime controls."},
}

type auditResource struct {
	path, content string
	via           []string
}

// scanAuditResources follows literal references, not arbitrary files added to a package.
// Unresolved external/computed resources stay explicit limitations, never a safety PASS.
func scanAuditResources(r *EvidenceReport, root, boundary, skill string) {
	queue := []auditResource{{"SKILL.md", skill, []string{"SKILL.md"}}}
	seen := map[string]bool{"SKILL.md": true}
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		r.Effects.Inspected = append(r.Effects.Inspected, item.path)
		inFront := false
		markdown := strings.HasSuffix(item.path, ".md")
		prose := strings.Split(item.content, "\n")
		commands := prose
		if markdown {
			prose = markdownProseLines(item.content)
			commands = markdownCommandLines(item.content)
			for _, ref := range markdownResources(prose) {
				target := strings.Trim(ref.target, "<>")
				if strings.HasPrefix(target, "#") || strings.Contains(target, ":") {
					continue
				}
				target = strings.SplitN(strings.SplitN(target, "#", 2)[0], "?", 2)[0]
				followAuditResource(r, root, boundary, &queue, seen, item, target, ref.line, strings.Split(item.content, "\n")[ref.line-1], false)
			}
		}
		for i, line := range strings.Split(item.content, "\n") {
			obs := func(kind, meaning string) LocatedObservation {
				return LocatedObservation{kind, item.path, i + 1, strings.TrimSpace(line), meaning, item.via}
			}
			if markdown && i == 0 && strings.TrimSpace(line) == "---" {
				inFront = true
				continue
			}
			if inFront {
				if strings.HasPrefix(strings.TrimSpace(line), "effects:") {
					r.Effects.Observations = append(r.Effects.Observations, obs("declared-effects", "Metadata declares effects; declarations do not establish implemented or authorized behavior."))
				}
				if strings.TrimSpace(line) == "---" {
					inFront = false
				}
				continue
			}
			for _, p := range effectPatterns {
				if p.pattern.MatchString(line) {
					r.Effects.Observations = append(r.Effects.Observations, obs(p.kind, p.meaning))
				}
			}
			if markdown && prose[i] != "" && auditNoop.MatchString(stripInlineCode(prose[i])) {
				r.Authoring.Suspicions = append(r.Authoring.Suspicions, obs("generic-advice", "Phrase may add no task-specific instruction; compare with the actual task before changing it."))
			}
			for _, ref := range directInvocations(commands[i], markdown && prose[i] != "") {
				followAuditResource(r, root, boundary, &queue, seen, item, ref, i+1, line, true)
			}
		}
	}
}

func followAuditResource(r *EvidenceReport, root, boundary string, queue *[]auditResource, seen map[string]bool, item auditResource, ref string, line int, snippet string, invocation bool) {
	// Commands are relative to the package root; Markdown links to their containing file.
	base := filepath.Dir(item.path)
	if invocation {
		base = "."
	}
	rel := filepath.Clean(filepath.Join(base, ref))
	if filepath.IsAbs(ref) || strings.HasPrefix(ref, "~") {
		r.addFinding("INVALID_RESOURCE", item.path, line, strings.TrimSpace(snippet), "resource link must be relative: "+ref)
		return
	}
	if rel == ".." || strings.HasPrefix(rel, "../") {
		checkCrossPackageResource(r, root, boundary, item, rel, ref, line, snippet)
		return
	}
	if strings.ContainsAny(ref, "<>${}") {
		return
	}
	path := filepath.Join(root, rel)
	actual, err := filepath.EvalSymlinks(path)
	if err == nil && actual != path {
		err = fmt.Errorf("resource uses a symlink: %s", ref)
	}
	if err != nil {
		if invocation && os.IsNotExist(err) && repositoryInvocation(r, root, boundary, item, ref, line, snippet, false) {
			return
		}
		r.addFinding("INVALID_RESOURCE", item.path, line, strings.TrimSpace(snippet), err.Error())
		return
	}
	info, err := os.Lstat(path)
	if err != nil {
		r.addFinding("INVALID_RESOURCE", item.path, line, strings.TrimSpace(snippet), err.Error())
		return
	}
	if info.IsDir() {
		if invocation {
			r.addFinding("INVALID_RESOURCE", item.path, line, strings.TrimSpace(snippet), "script invocation requires a regular file: "+ref)
		}
		return
	}
	if !info.Mode().IsRegular() {
		r.addFinding("INVALID_RESOURCE", item.path, line, strings.TrimSpace(snippet), "required resource is not a regular file: "+ref)
		return
	}
	if invocation {
		repositoryInvocation(r, root, boundary, item, ref, line, snippet, true)
	}
	if seen[rel] {
		return
	}
	seen[rel] = true
	// Binary assets are a resource fact, not executable text; never pretend to inspect them.
	ext := strings.ToLower(filepath.Ext(rel))
	switch ext {
	case ".md", ".sh", ".py", ".js", ".ts", ".go", ".yaml", ".yml", ".json", ".txt":
	default:
		r.Effects.Limitations = append(r.Effects.Limitations, "Resource text not inspected: "+rel)
		return
	}
	text, err := regularText(path)
	if err != nil {
		r.Effects.Limitations = append(r.Effects.Limitations, "Resource text not inspected: "+rel+": "+err.Error())
		return
	}
	if len(seen) > 256 {
		r.Effects.Limitations = append(r.Effects.Limitations, "Static resource traversal stopped at 256 literal resources.")
		return
	}
	via := append(append([]string{}, item.via...), rel)
	*queue = append(*queue, auditResource{rel, text, via})
}

func checkCrossPackageResource(r *EvidenceReport, root, boundary string, item auditResource, rel, ref string, line int, snippet string) {
	path := filepath.Join(root, rel)
	actual, err := filepath.EvalSymlinks(path)
	if err != nil || actual != path || (actual != boundary && !strings.HasPrefix(actual, boundary+string(filepath.Separator))) {
		r.addFinding("INVALID_RESOURCE", item.path, line, strings.TrimSpace(snippet), "resource is missing, aliased or outside the declared repository/catalog: "+ref)
		return
	}
	info, err := os.Lstat(actual)
	if err != nil || (!info.IsDir() && !info.Mode().IsRegular()) {
		r.addFinding("INVALID_RESOURCE", item.path, line, strings.TrimSpace(snippet), "resource is not a regular file or directory: "+ref)
		return
	}
	r.Effects.Observations = append(r.Effects.Observations, LocatedObservation{"external-resource", item.path, line, strings.TrimSpace(snippet), "Resource resolves inside the declared repository/catalog; its contents and transitive effects were not inspected: " + ref, item.via})
}

// Repository workflows also invoke scripts from the declared bundle root. Their
// existence is a resource fact, not proof of the invocation's working directory
// or of the external script's effects; never use arbitrary filesystem fallback.
func repositoryInvocation(r *EvidenceReport, root, boundary string, item auditResource, ref string, line int, snippet string, localCandidate bool) bool {
	if root == boundary {
		return false
	}
	path := filepath.Join(boundary, ref)
	actual, err := filepath.EvalSymlinks(path)
	if err != nil || actual != path || !strings.HasPrefix(actual, boundary+string(filepath.Separator)) {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	meaning := "Literal command resource exists at declared repository/catalog root: " + ref
	if localCandidate {
		meaning = "Multiple literal command candidates exist: package-relative and declared repository/catalog-root " + ref
	}
	meaning += ". Invocation working directory and this external script's transitive effects were not attested or inspected."
	r.Effects.Observations = append(r.Effects.Observations, LocatedObservation{"repository-command-resource", item.path, line, strings.TrimSpace(snippet), meaning, item.via})
	return true
}
