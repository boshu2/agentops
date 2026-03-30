package harvest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	Scope       string         `json:"scope"`    // project:X, language:X, global
	Date        string         `json:"date"`
	Frontmatter map[string]any `json:"frontmatter,omitempty"`
}

var (
	headingRe  = regexp.MustCompile(`(?m)^#\s+(.+)`)
	datePrefRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})`)
)

// ExtractArtifacts reads markdown files from a rig's .agents/ subdirectories
// and returns parsed Artifact values with provenance metadata.
func ExtractArtifacts(rig RigInfo, opts WalkOptions) ([]Artifact, error) {
	var artifacts []Artifact

	for _, subdir := range opts.IncludeDirs {
		dir := filepath.Join(rig.Path, subdir)
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("reading subdir %s: %w", dir, err)
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

			if opts.MaxFileSize > 0 {
				fi, err := e.Info()
				if err != nil {
					return nil, fmt.Errorf("stat %s: %w", path, err)
				}
				if fi.Size() > opts.MaxFileSize {
					continue
				}
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", path, err)
			}

			content := string(data)
			fm, body, err := parseFrontmatter(content)
			if err != nil {
				return nil, fmt.Errorf("parsing frontmatter in %s: %w", path, err)
			}

			fm = NormalizeFrontmatter(fm)

			title := extractTitle(fm, body, name)
			confidence := extractFloat(fm, "confidence", 0.3)
			scope := extractString(fm, "scope", "project:"+rig.Project)
			date := extractDate(fm, name)
			slug := toSlug(title)
			id := fmt.Sprintf("%s-%s-%s", artType, date, slug)

			artifacts = append(artifacts, Artifact{
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

	return artifacts, nil
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
	if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
		return nil, "", fmt.Errorf("unmarshaling frontmatter: %w", err)
	}
	if fm == nil {
		fm = map[string]any{}
	}

	return fm, body, nil
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
		return def
	default:
		return def
	}
}
