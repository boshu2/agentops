package search

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// QualityReport holds detection results for the learning pool.
type QualityReport struct {
	TotalLearnings  int      `json:"total_learnings"`
	WithSource      int      `json:"with_source"`
	WithoutSource   int      `json:"without_source"`
	StaleCount      int      `json:"stale_count"`
	DuplicateGroups int      `json:"duplicate_groups"`
	FlaggedPaths    []string `json:"flagged_paths"`
	Score           float64  `json:"score"`
}

// ScanLearningQuality scans a learnings directory and produces a quality report.
func ScanLearningQuality(learningsDir string) (*QualityReport, error) {
	report := &QualityReport{
		FlaggedPaths: []string{},
	}

	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		return report, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(learningsDir, entry.Name())
		hasSource, isStale, assessErr := AssessLearningFile(path)
		if assessErr != nil {
			continue
		}

		report.TotalLearnings++

		if hasSource {
			report.WithSource++
		} else {
			report.WithoutSource++
		}

		if isStale {
			report.StaleCount++
		}

		if !hasSource && isStale {
			report.FlaggedPaths = append(report.FlaggedPaths, path)
		}
	}

	if report.TotalLearnings == 0 {
		report.Score = 0
		return report, nil
	}

	sourceRatio := float64(report.WithSource) / float64(report.TotalLearnings)
	staleRatio := float64(report.StaleCount) / float64(report.TotalLearnings)
	report.Score = sourceRatio * (1 - staleRatio)

	return report, nil
}

// AssessLearningFile checks a single learning file for quality indicators.
// hasSource is true when the file contains a source_bead field in its frontmatter.
// isStale is true when the file's mtime is older than 90 days and no last_reward_at
// appears in the frontmatter within the last 90 days.
func AssessLearningFile(path string) (hasSource bool, isStale bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return false, false, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return false, false, err
	}

	const staleDays = 90
	staleThreshold := time.Now().Add(-staleDays * 24 * time.Hour)
	mtimeStale := info.ModTime().Before(staleThreshold)

	hasSource, lastRewardAt, err := scanLearningFrontMatter(f)
	if err != nil {
		return hasSource, false, err
	}
	return hasSource, learningIsStale(mtimeStale, lastRewardAt, staleThreshold), nil
}

func scanLearningFrontMatter(f *os.File) (bool, time.Time, error) {
	scanner := bufio.NewScanner(f)
	inFrontMatter := false
	hasSource := false
	var lastRewardAt time.Time

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "---" {
			if !inFrontMatter {
				inFrontMatter = true
				continue
			}
			// End of frontmatter
			break
		}

		if !inFrontMatter {
			continue
		}

		if frontMatterHasValue(line, "source_bead:", "source-bead:") {
			hasSource = true
		}

		if val, ok := frontMatterValue(line, "last_reward_at:", "last-reward-at:"); ok {
			if t, parseErr := time.Parse(time.RFC3339, val); parseErr == nil {
				lastRewardAt = t
			}
		}
	}

	return hasSource, lastRewardAt, scanner.Err()
}

func frontMatterHasValue(line string, prefixes ...string) bool {
	val, ok := frontMatterValue(line, prefixes...)
	return ok && val != "" && val != "null" && val != "~"
}

func frontMatterValue(line string, prefixes ...string) (string, bool) {
	for _, prefix := range prefixes {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		_, val, ok := strings.Cut(line, ":")
		return strings.TrimSpace(val), ok
	}
	return "", false
}

func learningIsStale(mtimeStale bool, lastRewardAt time.Time, staleThreshold time.Time) bool {
	return mtimeStale && (lastRewardAt.IsZero() || lastRewardAt.Before(staleThreshold))
}
