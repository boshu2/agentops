package warmind

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MaturityManager manages the maturity lifecycle of warmind learnings.
type MaturityManager struct {
	// Config is the warmind configuration.
	Config MaturityConfig

	// LearningsDir is where promoted learnings are stored.
	LearningsDir string

	// ArchiveDir is where expired learnings go.
	ArchiveDir string

	// CitationTracker tracks citations.
	CitationTracker *CitationTracker
}

// NewMaturityManager creates a new maturity manager.
func NewMaturityManager(baseDir string, cfg Config, tracker *CitationTracker) *MaturityManager {
	return &MaturityManager{
		Config:          cfg.Maturity,
		LearningsDir:    filepath.Join(baseDir, cfg.LearningsDir),
		ArchiveDir:      filepath.Join(baseDir, cfg.ArchiveDir),
		CitationTracker: tracker,
	}
}

// LearningMetadata holds parsed metadata from a learning file.
type LearningMetadata struct {
	ID            string
	FilePath      string
	Author        string
	AuthorEmail   string
	Maturity      Maturity
	Utility       float64
	CitationCount int
	CreatedAt     time.Time
	PromotedAt    time.Time
	LastCitedAt   *time.Time
	ContentHash   string
}

// ScanLearnings scans the learnings directory and returns metadata for all files.
func (m *MaturityManager) ScanLearnings() ([]LearningMetadata, error) {
	files, err := os.ReadDir(m.LearningsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var learnings []LearningMetadata
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		path := filepath.Join(m.LearningsDir, file.Name())
		meta, err := m.parseLearningMetadata(path)
		if err != nil {
			continue
		}

		learnings = append(learnings, *meta)
	}

	return learnings, nil
}

// ComputeDecay calculates the freshness decay for a learning.
// Based on Darr et al. forgetting curve: 17%/week.
func (m *MaturityManager) ComputeDecay(promotedAt time.Time) float64 {
	ageWeeks := time.Since(promotedAt).Hours() / (24 * 7)
	freshness := math.Exp(-m.Config.DecayRate * ageWeeks)

	// Floor at 0.1 - old knowledge still has some value
	if freshness < 0.1 {
		return 0.1
	}
	return freshness
}

// ComputeUtility calculates the effective utility of a learning.
// Utility = freshness * citation_bonus
func (m *MaturityManager) ComputeUtility(meta LearningMetadata) float64 {
	freshness := m.ComputeDecay(meta.PromotedAt)

	// Citation bonus: each citation adds 10% (capped at 2x)
	citationBonus := 1.0 + float64(meta.CitationCount)*0.1
	if citationBonus > 2.0 {
		citationBonus = 2.0
	}

	return freshness * citationBonus
}

// CheckTransition determines if a learning should transition maturity levels.
func (m *MaturityManager) CheckTransition(meta LearningMetadata) (Maturity, string) {
	citationCount := meta.CitationCount

	// Get actual citation count from tracker if available
	if m.CitationTracker != nil {
		if count, err := m.CitationTracker.GetOtherCitationCount(meta.ID, meta.AuthorEmail); err == nil {
			citationCount = count
		}
	}

	switch meta.Maturity {
	case MaturityProvisional:
		// Promote to established if 3+ citations
		if citationCount >= 3 {
			return MaturityEstablished, fmt.Sprintf("promoted: %d citations from other engineers", citationCount)
		}

		// Check for expiry
		age := time.Since(meta.PromotedAt)
		if age > time.Duration(m.Config.ProvisionalExpireDays)*24*time.Hour {
			if citationCount == 0 {
				return "archive", "expired: no citations in provisional period"
			}
		}

	case MaturityEstablished:
		// Check for expiry (longer threshold)
		if meta.LastCitedAt != nil {
			sinceLastCited := time.Since(*meta.LastCitedAt)
			if sinceLastCited > time.Duration(m.Config.EstablishedExpireDays)*24*time.Hour {
				return "archive", "expired: no recent citations"
			}
		}
	}

	return meta.Maturity, ""
}

// ApplyTransition updates a learning's maturity level.
func (m *MaturityManager) ApplyTransition(meta LearningMetadata, newMaturity Maturity, reason string) error {
	if newMaturity == "archive" {
		return m.archive(meta, reason)
	}

	// Update the frontmatter in place
	return m.updateMaturity(meta.FilePath, newMaturity)
}

// RunMaintenance runs the full maturity maintenance cycle.
func (m *MaturityManager) RunMaintenance() (*MaturityReport, error) {
	report := &MaturityReport{
		Scanned:     0,
		Promoted:    0,
		Archived:    0,
		Transitions: make([]MaturityTransition, 0),
	}

	learnings, err := m.ScanLearnings()
	if err != nil {
		return report, err
	}

	// Check minimum corpus size
	if len(learnings) < m.Config.MinCorpusSize {
		report.SkippedReason = fmt.Sprintf("corpus size %d below minimum %d", len(learnings), m.Config.MinCorpusSize)
		return report, nil
	}

	report.Scanned = len(learnings)

	for _, meta := range learnings {
		newMaturity, reason := m.CheckTransition(meta)
		if newMaturity == meta.Maturity || reason == "" {
			continue
		}

		if err := m.ApplyTransition(meta, newMaturity, reason); err != nil {
			continue
		}

		transition := MaturityTransition{
			LearningID: meta.ID,
			FilePath:   meta.FilePath,
			FromStatus: string(meta.Maturity),
			ToStatus:   string(newMaturity),
			Reason:     reason,
		}
		report.Transitions = append(report.Transitions, transition)

		switch newMaturity {
		case MaturityEstablished:
			report.Promoted++
		case "archive":
			report.Archived++
		}
	}

	return report, nil
}

// MaturityReport holds the results of a maintenance run.
type MaturityReport struct {
	Scanned       int
	Promoted      int
	Archived      int
	Transitions   []MaturityTransition
	SkippedReason string
}

// MaturityTransition records a single maturity change.
type MaturityTransition struct {
	LearningID string
	FilePath   string
	FromStatus string
	ToStatus   string
	Reason     string
}

// --- Internal helpers ---

func (m *MaturityManager) parseLearningMetadata(path string) (*LearningMetadata, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	meta := &LearningMetadata{
		FilePath: path,
		Maturity: MaturityProvisional,
	}

	lines := strings.Split(string(content), "\n")
	inFrontmatter := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break
		}

		if !inFrontmatter {
			continue
		}

		// Parse key: value
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"'")

		switch key {
		case "id":
			meta.ID = value
		case "author":
			meta.Author = value
		case "author_email":
			meta.AuthorEmail = value
		case "maturity":
			meta.Maturity = Maturity(value)
		case "utility":
			if f, err := strconv.ParseFloat(value, 64); err == nil {
				meta.Utility = f
			}
		case "citation_count":
			if i, err := strconv.Atoi(value); err == nil {
				meta.CitationCount = i
			}
		case "created_at":
			if t, err := time.Parse(time.RFC3339, value); err == nil {
				meta.CreatedAt = t
			}
		case "promoted_at":
			if t, err := time.Parse(time.RFC3339, value); err == nil {
				meta.PromotedAt = t
			}
		case "content_hash":
			meta.ContentHash = value
		}
	}

	// Get last cited from tracker if available
	if m.CitationTracker != nil && meta.ID != "" {
		if lastCited, err := m.CitationTracker.GetLastCitedAt(meta.ID); err == nil {
			meta.LastCitedAt = lastCited
		}
	}

	return meta, nil
}

func (m *MaturityManager) archive(meta LearningMetadata, reason string) error {
	// Ensure archive directory exists
	if err := os.MkdirAll(m.ArchiveDir, 0700); err != nil {
		return err
	}

	// Move the file
	destPath := filepath.Join(m.ArchiveDir, filepath.Base(meta.FilePath))
	if err := os.Rename(meta.FilePath, destPath); err != nil {
		return err
	}

	// Update the frontmatter to record archival
	return m.updateArchiveMetadata(destPath, reason)
}

func (m *MaturityManager) updateMaturity(path string, newMaturity Maturity) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Replace maturity line in frontmatter
	re := regexp.MustCompile(`(?m)^maturity:\s*\S+$`)
	updated := re.ReplaceAllString(string(content), fmt.Sprintf("maturity: %s", newMaturity))

	return os.WriteFile(path, []byte(updated), 0600)
}

func (m *MaturityManager) updateArchiveMetadata(path, reason string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var result []string
	inFrontmatter := false
	archivedAtAdded := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				result = append(result, line)
				continue
			}

			// End of frontmatter - add archived_at if not already added
			if !archivedAtAdded {
				result = append(result, fmt.Sprintf("archived_at: %s", time.Now().Format(time.RFC3339)))
				result = append(result, fmt.Sprintf("archive_reason: %s", reason))
			}
			result = append(result, line)
			// Add remaining lines
			result = append(result, lines[i+1:]...)
			break
		}

		if inFrontmatter {
			if strings.HasPrefix(trimmed, "maturity:") {
				result = append(result, "maturity: archived")
				continue
			}
		}

		result = append(result, line)
	}

	// If no maturity line was found, we simply proceed and write the file back
	// with the archived_at metadata appended.
	return os.WriteFile(path, []byte(strings.Join(result, "\n")), 0600)
}
