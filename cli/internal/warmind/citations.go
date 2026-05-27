package warmind

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CitationTracker manages warmind citation tracking.
type CitationTracker struct {
	// CitationsFile is where citations are stored.
	CitationsFile string

	// BaseDir is the repo root.
	BaseDir string

	// Config is the warmind configuration.
	Config Config
}

// NewCitationTracker creates a new citation tracker.
func NewCitationTracker(baseDir string, cfg Config) *CitationTracker {
	return &CitationTracker{
		CitationsFile: filepath.Join(baseDir, cfg.CitationsFile),
		BaseDir:       baseDir,
		Config:        cfg,
	}
}

// RecordCitation records a citation event.
func (ct *CitationTracker) RecordCitation(artifactPath, artifactID, query, sessionID string) error {
	// Get current git user
	citedBy, citedByEmail := getGitUser()

	// Check if this is a self-citation
	artifactAuthor, artifactAuthorEmail := getArtifactAuthor(artifactPath)
	isSelfCitation := (citedBy == artifactAuthor) || (citedByEmail != "" && citedByEmail == artifactAuthorEmail)

	citation := Citation{
		ArtifactPath:   artifactPath,
		ArtifactID:     artifactID,
		CitedAt:        time.Now(),
		CitedBy:        citedBy,
		CitedByEmail:   citedByEmail,
		SessionID:      sessionID,
		Query:          query,
		WorkspacePath:  ct.BaseDir,
		IsSelfCitation: isSelfCitation,
	}

	// Append to citations file
	if err := ct.appendCitation(citation); err != nil {
		return fmt.Errorf("append citation: %w", err)
	}

	return nil
}

// GetCitations returns all citations for an artifact.
func (ct *CitationTracker) GetCitations(artifactID string) ([]Citation, error) {
	all, err := ct.LoadAll()
	if err != nil {
		return nil, err
	}

	var citations []Citation
	for _, c := range all {
		if c.ArtifactID == artifactID {
			citations = append(citations, c)
		}
	}
	return citations, nil
}

// GetOtherCitationCount returns citations from OTHER engineers (non-self).
func (ct *CitationTracker) GetOtherCitationCount(artifactID, authorEmail string) (int, error) {
	citations, err := ct.GetCitations(artifactID)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, c := range citations {
		if !c.IsSelfCitation {
			count++
		}
	}
	return count, nil
}

// LoadAll loads all citations from the file.
func (ct *CitationTracker) LoadAll() ([]Citation, error) {
	f, err := os.Open(ct.CitationsFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var citations []Citation
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		var c Citation
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
			continue
		}
		citations = append(citations, c)
	}

	return citations, scanner.Err()
}

// Prune removes citations older than maxAge.
func (ct *CitationTracker) Prune(maxAge time.Duration) (int, error) {
	all, err := ct.LoadAll()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	var kept []Citation
	pruned := 0

	for _, c := range all {
		if c.CitedAt.After(cutoff) {
			kept = append(kept, c)
		} else {
			pruned++
		}
	}

	if pruned == 0 {
		return 0, nil
	}

	// Rewrite the file with only kept citations
	if err := ct.rewriteAll(kept); err != nil {
		return 0, err
	}

	return pruned, nil
}

// GetLastCitedAt returns when an artifact was last cited.
func (ct *CitationTracker) GetLastCitedAt(artifactID string) (*time.Time, error) {
	citations, err := ct.GetCitations(artifactID)
	if err != nil {
		return nil, err
	}

	if len(citations) == 0 {
		return nil, nil
	}

	var latest time.Time
	for _, c := range citations {
		if c.CitedAt.After(latest) {
			latest = c.CitedAt
		}
	}

	return &latest, nil
}

// GetCitationStats returns citation statistics for reporting.
func (ct *CitationTracker) GetCitationStats() (*CitationStats, error) {
	all, err := ct.LoadAll()
	if err != nil {
		return nil, err
	}

	stats := &CitationStats{
		Total:       len(all),
		ByCiter:     make(map[string]int),
		ByArtifact:  make(map[string]int),
		SelfCount:   0,
		OtherCount:  0,
	}

	for _, c := range all {
		stats.ByCiter[c.CitedBy]++
		stats.ByArtifact[c.ArtifactID]++
		if c.IsSelfCitation {
			stats.SelfCount++
		} else {
			stats.OtherCount++
		}
	}

	return stats, nil
}

// CitationStats holds citation statistics.
type CitationStats struct {
	Total      int
	SelfCount  int
	OtherCount int
	ByCiter    map[string]int
	ByArtifact map[string]int
}

// --- Internal helpers ---

func (ct *CitationTracker) appendCitation(c Citation) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(ct.CitationsFile), 0700); err != nil {
		return err
	}

	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(ct.CitationsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(append(data, '\n'))
	return err
}

func (ct *CitationTracker) rewriteAll(citations []Citation) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(ct.CitationsFile), 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(ct.CitationsFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, c := range citations {
		data, err := json.Marshal(c)
		if err != nil {
			continue
		}
		f.Write(append(data, '\n'))
	}

	return nil
}

// getGitUser returns the current git user name and email.
func getGitUser() (string, string) {
	name := ""
	email := ""

	if out, err := exec.Command("git", "config", "user.name").Output(); err == nil {
		name = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "config", "user.email").Output(); err == nil {
		email = strings.TrimSpace(string(out))
	}

	// Fallback to system user
	if name == "" {
		if user := os.Getenv("USER"); user != "" {
			name = user
		} else if user := os.Getenv("USERNAME"); user != "" {
			name = user
		}
	}

	return name, email
}

// getArtifactAuthor extracts author info from a learning file's frontmatter.
func getArtifactAuthor(path string) (string, string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}

	lines := strings.Split(string(content), "\n")
	inFrontmatter := false
	author := ""
	authorEmail := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break // End of frontmatter
		}

		if !inFrontmatter {
			continue
		}

		if strings.HasPrefix(trimmed, "author:") {
			author = strings.TrimSpace(strings.TrimPrefix(trimmed, "author:"))
			author = strings.Trim(author, "\"'")
		}
		if strings.HasPrefix(trimmed, "author_email:") {
			authorEmail = strings.TrimSpace(strings.TrimPrefix(trimmed, "author_email:"))
			authorEmail = strings.Trim(authorEmail, "\"'")
		}
	}

	return author, authorEmail
}

// GetCurrentGitUser is exported for use by other packages.
func GetCurrentGitUser() (string, string) {
	return getGitUser()
}
