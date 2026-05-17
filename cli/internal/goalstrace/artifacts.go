package goalstrace

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// beadTokenRe matches a bead ID token (e.g. soc-58nt.4.8) anywhere in free
// text or a file path — the discovery path for bead_produced_artifact.
var beadTokenRe = regexp.MustCompile(`\b[a-z]+-[a-z0-9]+\.[0-9]+(?:\.[0-9]+)*`)

// rpiArtifact is one RPI run artifact discovered under .agents/rpi/runs/.
type rpiArtifact struct {
	// relPath is the artifact path relative to the project root, slash-form.
	relPath string
	// beadID is the bead the artifact frontmatter declares (empty if none).
	beadID string
	// body is the full file content (used for free-text bead scans).
	body string
}

// artifactFrontmatterRe matches a "key: value" line in a leading YAML
// frontmatter block of an RPI artifact.
var artifactFrontmatterRe = regexp.MustCompile(`^([a-z_]+):\s*(.+?)\s*$`)

// loadRPIArtifacts walks .agents/rpi/runs/ for RPI run artifacts (markdown and
// JSON). A missing directory degrades gracefully: the second return is false
// and the caller records a diagnostic.
func loadRPIArtifacts(projectRoot string) ([]rpiArtifact, bool) {
	base := filepath.Join(projectRoot, ".agents", "rpi", "runs")
	if info, err := os.Stat(base); err != nil || !info.IsDir() {
		return nil, false
	}
	// os.Root scopes all reads under base, blocking symlink traversal out of
	// the directory tree and closing the TOCTOU window between WalkDir and
	// the per-file read (gosec G122 / CWE-367).
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, false
	}
	defer func() { _ = root.Close() }()
	var out []rpiArtifact
	_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".json") {
			return nil
		}
		rel, relErr := filepath.Rel(base, path)
		if relErr != nil {
			return nil
		}
		data, rerr := root.ReadFile(filepath.ToSlash(rel))
		if rerr != nil {
			return nil
		}
		out = append(out, parseRPIArtifact(relPath(projectRoot, path), string(data)))
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].relPath < out[j].relPath })
	return out, true
}

// parseRPIArtifact builds an rpiArtifact, extracting the bead_id frontmatter
// field when the file carries a leading YAML frontmatter block.
func parseRPIArtifact(rel, content string) rpiArtifact {
	a := rpiArtifact{relPath: rel, body: content}
	if !strings.HasPrefix(content, "---") {
		return a
	}
	rest := strings.TrimPrefix(content, "---")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return a
	}
	for _, line := range strings.Split(rest[:end], "\n") {
		m := artifactFrontmatterRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		if m[1] == "bead_id" {
			a.beadID = strings.Trim(m[2], `"'`)
		}
	}
	return a
}

// artifactBeadLink resolves the bead an artifact was produced by, and the
// confidence of that link, per ADR-0005 §2.4. Frontmatter bead_id and a bead
// ID in the file path are high confidence; a free-text bead token is low.
func artifactBeadLink(a rpiArtifact) (beadID string, conf Confidence, viaPath bool) {
	if a.beadID != "" {
		return a.beadID, ConfidenceHigh, false
	}
	if m := beadTokenRe.FindString(a.relPath); m != "" {
		return m, ConfidenceHigh, true
	}
	if m := beadTokenRe.FindString(a.body); m != "" {
		return m, ConfidenceLow, false
	}
	return "", ConfidenceLow, false
}

// learningCitesArtifact reports whether a learning cites an artifact and the
// confidence of that citation per ADR-0005 §2.5. An exact relative-path match
// (frontmatter source or body) is high; a bare-filename body match is low.
func learningCitesArtifact(lf learningFile, artifactRel string) (cited bool, conf Confidence) {
	if lf.fm.source != "" && pathMatches(lf.fm.source, artifactRel) {
		return true, ConfidenceHigh
	}
	if strings.Contains(lf.body, artifactRel) {
		return true, ConfidenceHigh
	}
	base := filepath.Base(artifactRel)
	if base != "" && base != artifactRel && strings.Contains(lf.body, base) {
		return true, ConfidenceLow
	}
	return false, ConfidenceLow
}

// pathMatches reports whether a learning's source field references the given
// artifact path, tolerating a leading "./" on either side.
func pathMatches(source, artifactRel string) bool {
	s := strings.TrimPrefix(strings.TrimSpace(source), "./")
	a := strings.TrimPrefix(artifactRel, "./")
	return s == a
}
