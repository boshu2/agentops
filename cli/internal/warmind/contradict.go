package warmind

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ContradictionDetector scans for contradictions between team learnings.
type ContradictionDetector struct {
	// Config is the warmind configuration.
	Config ContradictConfig

	// ContradictionsFile is where contradictions are stored.
	ContradictionsFile string

	// LearningsDir is where learnings are stored.
	LearningsDir string

	// MaturityManager for accessing learning metadata.
	MaturityManager *MaturityManager
}

// NewContradictionDetector creates a new contradiction detector.
func NewContradictionDetector(baseDir string, cfg Config, mm *MaturityManager) *ContradictionDetector {
	return &ContradictionDetector{
		Config:             cfg.Contradict,
		ContradictionsFile: filepath.Join(baseDir, cfg.ContradictionsFile),
		LearningsDir:       filepath.Join(baseDir, cfg.LearningsDir),
		MaturityManager:    mm,
	}
}

// OpposingPair represents words that often indicate conflicting advice.
type OpposingPair struct {
	A string
	B string
}

// Common opposing pairs that indicate potential contradictions.
var opposingPairs = []OpposingPair{
	{"always", "never"},
	{"do", "don't"},
	{"use", "avoid"},
	{"enable", "disable"},
	{"prefer", "avoid"},
	{"recommended", "discouraged"},
	{"should", "shouldn't"},
	{"must", "must not"},
	{"required", "optional"},
	{"sync", "async"},
	{"mutable", "immutable"},
	{"global", "local"},
	{"public", "private"},
	{"stateful", "stateless"},
}

// Scan scans all learnings for potential contradictions.
func (cd *ContradictionDetector) Scan() (*ContradictionReport, error) {
	report := &ContradictionReport{
		Scanned:        0,
		NewDetected:    0,
		Contradictions: make([]Contradiction, 0),
	}

	learnings, err := cd.MaturityManager.ScanLearnings()
	if err != nil {
		return report, err
	}

	report.Scanned = len(learnings)

	// Load existing contradictions to avoid duplicates
	existing, _ := cd.LoadAll()
	existingSet := make(map[string]bool)
	for _, c := range existing {
		key := cd.contradictionKey(c.LearningA, c.LearningB)
		existingSet[key] = true
	}

	// Compare all pairs
	for i := 0; i < len(learnings); i++ {
		for j := i + 1; j < len(learnings); j++ {
			a := learnings[i]
			b := learnings[j]

			// Skip if already detected
			key := cd.contradictionKey(a.FilePath, b.FilePath)
			if existingSet[key] {
				continue
			}

			// Check for contradictions
			if conflict := cd.detectContradiction(a, b); conflict != nil {
				report.Contradictions = append(report.Contradictions, *conflict)
				report.NewDetected++

				// Record the contradiction; a persistence failure (disk/permissions)
				// is surfaced rather than silently dropping the team-knowledge record.
				if err := cd.appendContradiction(*conflict); err != nil {
					return report, fmt.Errorf("recording contradiction: %w", err)
				}
			}
		}
	}

	return report, nil
}

// LoadAll loads all contradictions from the file.
func (cd *ContradictionDetector) LoadAll() ([]Contradiction, error) {
	f, err := os.Open(cd.ContradictionsFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var contradictions []Contradiction
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var c Contradiction
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
			continue
		}
		contradictions = append(contradictions, c)
	}

	return contradictions, scanner.Err()
}

// GetPending returns unresolved contradictions.
func (cd *ContradictionDetector) GetPending() ([]Contradiction, error) {
	all, err := cd.LoadAll()
	if err != nil {
		return nil, err
	}

	var pending []Contradiction
	for _, c := range all {
		if c.Status == "pending_review" {
			pending = append(pending, c)
		}
	}
	return pending, nil
}

// Resolve marks a contradiction as resolved.
func (cd *ContradictionDetector) Resolve(id, resolvedBy, resolution string) error {
	all, err := cd.LoadAll()
	if err != nil {
		return err
	}

	found := false
	now := time.Now()
	for i := range all {
		if all[i].ID == id {
			all[i].Status = "resolved"
			all[i].ResolvedAt = &now
			all[i].ResolvedBy = resolvedBy
			all[i].Resolution = resolution
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("contradiction not found: %s", id)
	}

	return cd.rewriteAll(all)
}

// Dismiss marks a contradiction as dismissed (false positive).
func (cd *ContradictionDetector) Dismiss(id, dismissedBy, reason string) error {
	all, err := cd.LoadAll()
	if err != nil {
		return err
	}

	found := false
	now := time.Now()
	for i := range all {
		if all[i].ID == id {
			all[i].Status = "dismissed"
			all[i].ResolvedAt = &now
			all[i].ResolvedBy = dismissedBy
			all[i].Resolution = "dismissed: " + reason
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("contradiction not found: %s", id)
	}

	return cd.rewriteAll(all)
}

// ContradictionReport holds the results of a contradiction scan.
type ContradictionReport struct {
	Scanned        int
	NewDetected    int
	Contradictions []Contradiction
}

// --- Internal helpers ---

func (cd *ContradictionDetector) detectContradiction(a, b LearningMetadata) *Contradiction {
	// Read content
	contentA, err := os.ReadFile(a.FilePath)
	if err != nil {
		return nil
	}
	contentB, err := os.ReadFile(b.FilePath)
	if err != nil {
		return nil
	}

	textA := strings.ToLower(string(contentA))
	textB := strings.ToLower(string(contentB))

	// Check if they're about similar topics (must overlap)
	keywordsA := extractKeywords(string(contentA))
	keywordsB := extractKeywords(string(contentB))
	overlap := jaccardSimilarity(keywordsA, keywordsB)

	// Require some topic overlap (otherwise they're just about different things)
	if overlap < 0.15 {
		return nil
	}

	// Count opposing signals
	signals := 0
	var foundPairs []string

	for _, pair := range opposingPairs {
		aHasFirst := strings.Contains(textA, pair.A)
		aHasSecond := strings.Contains(textA, pair.B)
		bHasFirst := strings.Contains(textB, pair.A)
		bHasSecond := strings.Contains(textB, pair.B)

		// A says X, B says opposite of X
		if (aHasFirst && bHasSecond) || (aHasSecond && bHasFirst) {
			signals++
			foundPairs = append(foundPairs, fmt.Sprintf("%s vs %s", pair.A, pair.B))
		}
	}

	// Check for direct negation patterns
	negationPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\bdo\s+use\b`),
		regexp.MustCompile(`\bdon'?t\s+use\b`),
		regexp.MustCompile(`\balways\s+\w+\b`),
		regexp.MustCompile(`\bnever\s+\w+\b`),
	}

	for _, pattern := range negationPatterns {
		matchesA := pattern.FindAllString(textA, -1)
		matchesB := pattern.FindAllString(textB, -1)

		for _, ma := range matchesA {
			for _, mb := range matchesB {
				if cd.areOpposingStatements(ma, mb) {
					signals++
				}
			}
		}
	}

	// Require minimum signals to avoid false positives
	if signals < cd.Config.MinSignals {
		return nil
	}

	// Generate a summary
	summary := fmt.Sprintf("Found %d opposing signals between learnings", signals)
	if len(foundPairs) > 0 {
		summary += fmt.Sprintf(" (%s)", strings.Join(foundPairs[:min(3, len(foundPairs))], ", "))
	}

	return &Contradiction{
		ID:              generateContradictionID(a.ID, b.ID),
		DetectedAt:      time.Now(),
		LearningA:       a.FilePath,
		LearningAAuthor: a.Author,
		LearningB:       b.FilePath,
		LearningBAuthor: b.Author,
		ConflictType:    "opposing_recommendations",
		Summary:         summary,
		Status:          "pending_review",
	}
}

func (cd *ContradictionDetector) areOpposingStatements(a, b string) bool {
	a = strings.ToLower(a)
	b = strings.ToLower(b)

	// Check for do/don't patterns
	if strings.Contains(a, "do ") && strings.Contains(b, "don") {
		return true
	}
	if strings.Contains(a, "don") && strings.Contains(b, "do ") {
		return true
	}

	// Check for always/never
	if strings.Contains(a, "always") && strings.Contains(b, "never") {
		return true
	}
	if strings.Contains(a, "never") && strings.Contains(b, "always") {
		return true
	}

	return false
}

func (cd *ContradictionDetector) contradictionKey(pathA, pathB string) string {
	// Normalize order so (A,B) and (B,A) produce the same key
	if pathA > pathB {
		pathA, pathB = pathB, pathA
	}
	return pathA + "|" + pathB
}

func (cd *ContradictionDetector) appendContradiction(c Contradiction) (err error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(cd.ContradictionsFile), 0700); err != nil {
		return err
	}

	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(cd.ContradictionsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("appending contradiction: %w", err)
	}
	return nil
}

func (cd *ContradictionDetector) rewriteAll(contradictions []Contradiction) (err error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(cd.ContradictionsFile), 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(cd.ContradictionsFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	for _, c := range contradictions {
		data, mErr := json.Marshal(c)
		if mErr != nil {
			continue
		}
		if _, wErr := f.Write(append(data, '\n')); wErr != nil {
			return fmt.Errorf("rewriting contradictions: %w", wErr)
		}
	}

	return nil
}

func generateContradictionID(idA, idB string) string {
	// Normalize order
	if idA > idB {
		idA, idB = idB, idA
	}
	hash := ContentHash(idA + idB)
	return "contra-" + hash[:12]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
