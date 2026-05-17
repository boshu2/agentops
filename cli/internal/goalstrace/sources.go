package goalstrace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// scenarioFile is the subset of a scenario JSON document the walker reads.
type scenarioFile struct {
	ID          string `json:"id"`
	DirectiveID string `json:"directive_id"`
	Goal        string `json:"goal"`
	Status      string `json:"status"`
	// path and promoted are populated by the loader, not the JSON.
	path     string
	promoted bool
}

// scenarioResolution records where (and whether) a scenario ID resolved.
type scenarioResolution struct {
	ID    string
	File  *scenarioFile // nil when the ID does not resolve
	Found bool
}

// resolveScenario looks up a scenario ID, checking the promoted spec location
// first (spec/scenarios/<id>.json) then the ad hoc holdout location
// (.agents/holdout/<id>.json), per ADR-0005 §2.1 resolution order.
func resolveScenario(projectRoot, id string) scenarioResolution {
	promoted := filepath.Join(projectRoot, "spec", "scenarios", id+".json")
	if sf, ok := readScenario(promoted); ok {
		sf.promoted = true
		return scenarioResolution{ID: id, File: sf, Found: true}
	}
	holdout := filepath.Join(projectRoot, ".agents", "holdout", id+".json")
	if sf, ok := readScenario(holdout); ok {
		sf.promoted = false
		return scenarioResolution{ID: id, File: sf, Found: true}
	}
	return scenarioResolution{ID: id, Found: false}
}

// readScenario reads and decodes a scenario JSON file. The second return is
// false when the file is absent or malformed.
func readScenario(path string) (*scenarioFile, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var sf scenarioFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, false
	}
	sf.path = path
	return &sf, true
}

// loadAllScenarios reads every scenario JSON under spec/scenarios/ so the
// walker can discover reverse links (scenario.directive_id pointing at a
// directive). Missing directory yields an empty slice, never an error.
func loadAllScenarios(projectRoot string) []scenarioFile {
	dir := filepath.Join(projectRoot, "spec", "scenarios")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []scenarioFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if sf, ok := readScenario(filepath.Join(dir, e.Name())); ok {
			sf.promoted = true
			out = append(out, *sf)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// frontmatterFields holds the YAML frontmatter fields a learning may declare.
type frontmatterFields struct {
	directiveID string
	scenarioID  string
	source      string
}

// learningFile is a parsed docs/learnings/<date>-<slug>.md file.
type learningFile struct {
	path string
	body string
	fm   frontmatterFields
}

// fmLineRe matches a "key: value" line inside a YAML frontmatter block.
var fmLineRe = regexp.MustCompile(`^([a-z_]+):\s*(.+?)\s*$`)

// loadLearnings reads every learning markdown file under docs/learnings/.
// A missing directory degrades gracefully: the second return is false and the
// caller records a diagnostic rather than crashing (the dir may be untracked).
func loadLearnings(projectRoot string) ([]learningFile, bool) {
	dir := filepath.Join(projectRoot, "docs", "learnings")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false
	}
	var out []learningFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			continue
		}
		out = append(out, parseLearning(path, string(data)))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out, true
}

// parseLearning splits a learning file into frontmatter and body and extracts
// the trace-relevant frontmatter fields.
func parseLearning(path, content string) learningFile {
	lf := learningFile{path: path, body: content}
	if !strings.HasPrefix(content, "---") {
		return lf
	}
	rest := strings.TrimPrefix(content, "---")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return lf
	}
	fmBlock := rest[:end]
	lf.body = strings.TrimPrefix(rest[end+len("\n---"):], "\n")
	for _, line := range strings.Split(fmBlock, "\n") {
		m := fmLineRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		val := strings.Trim(m[2], `"'`)
		switch m[1] {
		case "directive_id":
			lf.fm.directiveID = val
		case "scenario_id":
			lf.fm.scenarioID = val
		case "source":
			lf.fm.source = val
		}
	}
	return lf
}
