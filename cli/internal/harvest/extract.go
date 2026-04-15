package harvest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Artifact represents a single extracted knowledge item with provenance.
type Artifact struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Summary     string         `json:"summary,omitempty"`
	Type        string         `json:"type"`        // learning, pattern, research
	SourceRig   string         `json:"source_rig"`  // "agentops-nami"
	SourcePath  string         `json:"source_path"` // Absolute path to source file
	ContentHash string         `json:"content_hash"`
	Confidence  float64        `json:"confidence"`
	Scope       string         `json:"scope"` // project:X, language:X, global
	Date        string         `json:"date"`
	Frontmatter map[string]any `json:"frontmatter,omitempty"`
}

// HarvestWarning records a non-fatal issue encountered during harvest so the
// caller can surface it in the catalog instead of dropping a whole rig.
type HarvestWarning struct {
	Rig     string `json:"rig,omitempty"`
	Path    string `json:"path"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

// ExtractionResult captures extracted artifacts plus the candidate-file count
// and any non-fatal warnings encountered while processing a rig.
type ExtractionResult struct {
	Artifacts      []Artifact
	Warnings       []HarvestWarning
	CandidateFiles int
}

var (
	headingRe  = regexp.MustCompile(`(?m)^#\s+(.+)`)
	datePrefRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})`)
)

// ExtractArtifacts reads markdown files from a rig's .agents/ subdirectories
// and returns parsed Artifact values with provenance metadata plus non-fatal
// warnings for any files or subdirectories that were skipped.
func ExtractArtifacts(rig RigInfo, opts WalkOptions) ([]Artifact, []HarvestWarning) {
	result := ExtractArtifactsWithStats(rig, opts)
	return result.Artifacts, result.Warnings
}

// ExtractArtifactsWithStats reads markdown files from a rig's .agents/
// subdirectories and returns parsed artifacts, warnings, and the number of
// candidate files encountered in the included directories.
func ExtractArtifactsWithStats(rig RigInfo, opts WalkOptions) ExtractionResult {
	var result ExtractionResult

	warn := func(path, stage string, err error) {
		if err == nil {
			return
		}
		result.Warnings = append(result.Warnings, HarvestWarning{
			Rig:     rig.Rig,
			Path:    path,
			Stage:   stage,
			Message: err.Error(),
		})
	}

	for _, subdir := range opts.IncludeDirs {
		dir := filepath.Join(rig.Path, subdir)
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			warn(dir, "read_dir", fmt.Errorf("reading subdir %s: %w", dir, err))
			continue
		}

		artType := singularType(subdir)

		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			ext := strings.ToLower(filepath.Ext(name))
			if ext != ".md" && ext != ".jsonl" {
				continue
			}

			path := filepath.Join(dir, name)
			result.CandidateFiles++

			if opts.MaxFileSize > 0 {
				fi, err := e.Info()
				if err != nil {
					warn(path, "stat", fmt.Errorf("stat %s: %w", path, err))
					continue
				}
				if fi.Size() > opts.MaxFileSize {
					continue
				}
			}

			data, err := os.ReadFile(path)
			if err != nil {
				warn(path, "read_file", fmt.Errorf("reading %s: %w", path, err))
				continue
			}

			content := string(data)
			fm, body, err := parseFrontmatter(content)
			if err != nil {
				warn(path, "parse_frontmatter", fmt.Errorf("parsing frontmatter in %s: %w", path, err))
				continue
			}

			fm = NormalizeFrontmatter(fm)

			title := extractTitle(fm, body, name)
			confidence := extractFloat(fm, "confidence", 0.5)
			scope := extractString(fm, "scope", "project:"+rig.Project)
			date := extractDate(fm, name)
			slug := toSlug(title)
			id := fmt.Sprintf("%s-%s-%s", artType, date, slug)

			result.Artifacts = append(result.Artifacts, Artifact{
				ID:          id,
				Title:       title,
				Summary:     extractString(fm, "summary", ""),
				Type:        artType,
				SourceRig:   rig.Rig,
				SourcePath:  path,
				ContentHash: hashNormalizedContent(body),
				Confidence:  confidence,
				Scope:       scope,
				Date:        date,
				Frontmatter: fm,
			})
		}
	}

	return result
}

// parseFrontmatter splits YAML frontmatter from body content.
// Returns (frontmatter map, body, error). If no frontmatter delimiters
// are found, returns an empty map and the full content as body.
func parseFrontmatter(content string) (map[string]any, string, error) {
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		return map[string]any{}, content, nil
	}

	trimmed := strings.TrimSpace(content)
	// Find the opening delimiter.
	first := strings.Index(trimmed, "---")
	rest := trimmed[first+3:]

	// Find the closing delimiter.
	second := strings.Index(rest, "---")
	if second < 0 {
		return map[string]any{}, content, nil
	}

	fmRaw := rest[:second]
	body := rest[second+3:]

	var fm map[string]any
	err := yaml.Unmarshal([]byte(fmRaw), &fm)
	if err != nil {
		// Salvage pass: the common authoring mistake is an unquoted scalar
		// whose value contains a literal ":" followed by a space (e.g.
		// `description: On edge. Solution: layer the CA bundle`). YAML reads
		// "Solution:" as the start of a nested mapping and errors. Quote
		// every top-level scalar value and retry before giving up.
		if serr := yaml.Unmarshal([]byte(salvageFrontmatterYAML(fmRaw)), &fm); serr == nil {
			if fm == nil {
				fm = map[string]any{}
			}
			return fm, body, nil
		}
		return nil, "", fmt.Errorf("unmarshaling frontmatter: %w", err)
	}
	if fm == nil {
		fm = map[string]any{}
	}

	return fm, body, nil
}

// salvageFrontmatterYAML rewrites a fragment of YAML that failed to parse so
// that any top-level scalar value is double-quoted. It only touches lines
// that look like `key: value` where value is a plain scalar (not a list,
// map, or already-quoted string). This fixes the common mid-value-colon bug
// without changing the semantics of valid documents.
func salvageFrontmatterYAML(fmRaw string) string {
	lines := strings.Split(fmRaw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			out = append(out, line)
			continue
		}
		// Only rewrite lines that start at column 0 — we do not try to
		// salvage nested structures. "Top-level scalar" is conservative on
		// purpose; false positives would break valid documents.
		if line != trimmed {
			out = append(out, line)
			continue
		}
		idx := strings.Index(trimmed, ":")
		if idx < 0 {
			out = append(out, line)
			continue
		}
		key := trimmed[:idx]
		val := strings.TrimSpace(trimmed[idx+1:])
		if val == "" || val == ">" || val == "|" {
			// block-scalar header; leave as-is
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(val, "\"") || strings.HasPrefix(val, "'") {
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(val, "[") || strings.HasPrefix(val, "{") || strings.HasPrefix(val, "-") {
			// list / map / sequence marker; leave alone
			out = append(out, line)
			continue
		}
		// Scalar boolean / null / number pass-through — quoting them is
		// harmless for our downstream consumers (string-or-any map).
		quoted := strings.ReplaceAll(val, `"`, `\"`)
		out = append(out, fmt.Sprintf(`%s: "%s"`, key, quoted))
	}
	return strings.Join(out, "\n")
}

// NormalizeFrontmatter standardizes field names in a frontmatter map.
func NormalizeFrontmatter(raw map[string]any) map[string]any {
	if raw == nil {
		return map[string]any{}
	}

	// category -> type
	if v, ok := raw["category"]; ok {
		if _, hasType := raw["type"]; !hasType {
			raw["type"] = v
		}
		delete(raw, "category")
	}

	// score -> confidence
	if v, ok := raw["score"]; ok {
		if _, hasCf := raw["confidence"]; !hasCf {
			raw["confidence"] = v
		}
		delete(raw, "score")
	}

	// Ensure confidence is float64.
	if v, ok := raw["confidence"]; ok {
		raw["confidence"] = toFloat64(v)
	}

	return raw
}

// extractTitle returns a title from frontmatter, first heading, or filename.
func extractTitle(fm map[string]any, body, filename string) string {
	if t, ok := fm["title"]; ok {
		if s, ok := t.(string); ok && s != "" {
			return s
		}
	}

	if m := headingRe.FindStringSubmatch(body); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	// Fallback: filename without extension and date prefix.
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	base = datePrefRe.ReplaceAllString(base, "")
	base = strings.TrimLeft(base, "-_ ")
	if base == "" {
		base = filename
	}
	return base
}

// singularType converts a subdir name to singular artifact type.
func singularType(subdir string) string {
	switch subdir {
	case "learnings":
		return "learning"
	case "patterns":
		return "pattern"
	default:
		return subdir
	}
}

// toSlug converts a string to a kebab-case slug.
func toSlug(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), "-")
	if len(s) > 60 {
		s = s[:60]
		// Trim trailing hyphen from truncation.
		s = strings.TrimRight(s, "-")
	}
	return s
}

// hashNormalizedContent computes a SHA256 hash of normalized body content.
// Duplicated from cmd/ao/ because that package is not importable here.
func hashNormalizedContent(body string) string {
	s := strings.ToLower(strings.TrimSpace(body))
	s = strings.ReplaceAll(s, "#", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "---", "")
	s = strings.Join(strings.Fields(s), " ")
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// extractString returns a string from the frontmatter map or a default.
func extractString(fm map[string]any, key, def string) string {
	if v, ok := fm[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return def
}

// extractFloat returns a float64 from the frontmatter map or a default.
func extractFloat(fm map[string]any, key string, def float64) float64 {
	if v, ok := fm[key]; ok {
		return toFloat64WithDefault(v, def)
	}
	return def
}

// extractDate returns a date string from frontmatter or filename.
func extractDate(fm map[string]any, filename string) string {
	if v, ok := fm["date"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}

	if m := datePrefRe.FindString(filename); m != "" {
		return m
	}

	return "unknown"
}

// toFloat64 converts various numeric types to float64.
func toFloat64(v any) float64 {
	return toFloat64WithDefault(v, 0)
}

// toFloat64WithDefault converts various numeric types to float64 with a fallback.
func toFloat64WithDefault(v any, def float64) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		switch strings.ToLower(n) {
		case "high":
			return 0.9
		case "medium":
			return 0.6
		case "low":
			return 0.3
		default:
			if f, err := strconv.ParseFloat(n, 64); err == nil {
				return f
			}
			return def
		}
	default:
		return def
	}
}
