package overnight

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

// findingFilenameRe matches canonical finding filenames of the form
// "f-YYYY-MM-DD-NNN.md". registry.*.json, README.md, and other names are
// intentionally rejected (pm-MISS-06 fix).
var findingFilenameRe = regexp.MustCompile(`^f-\d{4}-\d{2}-\d{2}-\d{3}\.md$`)

// routedFinding is the structured next-work item written by RouteFindings.
//
// Fields map to the existing next-work.jsonl item shape. New routes always
// set Type="tech-debt" and Source="council-finding" to satisfy the v1.3
// next-work enum contract (see docs/contracts/next-work.schema.md). The
// original finding provenance lives in SourcePath and in the finding file's
// own frontmatter. Severity defaults to "medium" when the finding body does
// not declare one.
type routedFinding struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Source      string `json:"source"`
	Description string `json:"description"`
	TargetRepo  string `json:"target_repo"`
	SourcePath  string `json:"source_path"`
}

// nextWorkLine is the persisted JSONL line shape appended by the router.
//
// Only the fields the router needs are typed; we intentionally mirror the
// keys used by existing next-work.jsonl lines so downstream consumers
// (triage, swarm, flywheel) can ingest both historical and router-written
// entries without branching.
type nextWorkLine struct {
	SourceEpic  string          `json:"source_epic"`
	Timestamp   string          `json:"timestamp"`
	Items       []routedFinding `json:"items"`
	Consumed    bool            `json:"consumed"`
	ClaimStatus string          `json:"claim_status"`
	ClaimedBy   *string         `json:"claimed_by"`
	ClaimedAt   *string         `json:"claimed_at"`
}

// RouteFindings scans .agents/findings/*.md under cwd, classifies each
// finding, dedups against .agents/rpi/next-work.jsonl by finding ID, and
// appends NEW entries to next-work.jsonl in append-only mode.
//
// Only files matching the regex ^f-\d{4}-\d{2}-\d{2}-\d{3}\.md$ are treated
// as findings (pm-MISS-06 fix). registry.*.json and README.md are ignored.
//
// Returns: count of items routed, list of degraded conditions, error.
// Soft-fails on missing .agents/findings/ (returns 0, ["no findings dir"], nil).
// Soft-fails on missing .agents/rpi/next-work.jsonl (creates it).
//
// Append atomicity: the routed batch is written as a single O_APPEND line
// with a trailing newline and fsync. NFS append is NOT atomic; callers MUST
// ensure next-work.jsonl lives on a local POSIX filesystem.
func RouteFindings(cwd string) (routed int, degraded []string, err error) {
	findingsDir := filepath.Join(cwd, ".agents", "findings")
	nextWorkPath := filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl")

	info, statErr := os.Stat(findingsDir)
	if os.IsNotExist(statErr) {
		return 0, []string{"no findings dir"}, nil
	}
	if statErr != nil {
		return 0, nil, fmt.Errorf("stat findings dir: %w", statErr)
	}
	if !info.IsDir() {
		return 0, nil, fmt.Errorf("findings path is not a directory: %s", findingsDir)
	}

	seenIDs, loadErr := loadNextWorkIDs(nextWorkPath)
	if loadErr != nil {
		return 0, nil, fmt.Errorf("load next-work ids: %w", loadErr)
	}

	entries, readErr := os.ReadDir(findingsDir)
	if readErr != nil {
		return 0, nil, fmt.Errorf("read findings dir: %w", readErr)
	}

	var newItems []routedFinding
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		base := entry.Name()
		id := findingID(base)
		if id == "" {
			continue
		}
		if seenIDs[id] {
			continue
		}
		fullPath := filepath.Join(findingsDir, base)
		rf, parseErr := parseFinding(id, fullPath, cwd)
		if parseErr != nil {
			degraded = append(degraded, fmt.Sprintf("finding %s: %v", id, parseErr))
			continue
		}
		if rf.Description == "" {
			degraded = append(degraded, fmt.Sprintf("finding %s: empty body", id))
		}
		newItems = append(newItems, rf)
		seenIDs[id] = true
	}

	if len(newItems) == 0 {
		return 0, degraded, nil
	}

	if mkErr := os.MkdirAll(filepath.Dir(nextWorkPath), 0o755); mkErr != nil {
		return 0, degraded, fmt.Errorf("mkdir next-work dir: %w", mkErr)
	}

	line := nextWorkLine{
		SourceEpic:  "dream-findings-router",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Items:       newItems,
		Consumed:    false,
		ClaimStatus: "pending",
		ClaimedBy:   nil,
		ClaimedAt:   nil,
	}
	encoded, marshalErr := json.Marshal(line)
	if marshalErr != nil {
		return 0, degraded, fmt.Errorf("marshal next-work line: %w", marshalErr)
	}

	f, openErr := os.OpenFile(nextWorkPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if openErr != nil {
		return 0, degraded, fmt.Errorf("open next-work.jsonl: %w", openErr)
	}
	defer func() { _ = f.Close() }()

	payload := append(encoded, '\n')
	if _, writeErr := f.Write(payload); writeErr != nil {
		return 0, degraded, fmt.Errorf("write next-work.jsonl: %w", writeErr)
	}
	if syncErr := f.Sync(); syncErr != nil {
		return 0, degraded, fmt.Errorf("sync next-work.jsonl: %w", syncErr)
	}

	return len(newItems), degraded, nil
}

// findingID parses a filename like "f-2026-03-22-001.md" and returns
// "f-2026-03-22-001". Returns "" for non-conforming names or
// calendar-invalid dates (e.g. "f-9999-99-99-001.md").
func findingID(basename string) string {
	if !findingFilenameRe.MatchString(basename) {
		return ""
	}
	// Extract YYYY-MM-DD portion (chars 2..12 of "f-YYYY-MM-DD-NNN.md").
	dateStr := basename[2:12]
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		return ""
	}
	return strings.TrimSuffix(basename, ".md")
}

// loadNextWorkIDs reads next-work.jsonl and returns the set of finding IDs
// already present in any "items" array across any status. Returns empty
// set (not nil) on missing file.
//
// The function is tolerant of historical lines that do not carry finding IDs
// (most pre-router lines) — those items simply contribute nothing to the
// set. Malformed lines are skipped without error so the router never fails
// a whole dedup pass on one bad historical record.
func loadNextWorkIDs(path string) (map[string]bool, error) {
	ids := make(map[string]bool)
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return ids, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	// Allow long JSONL lines; default 64KB is too small for dense batches.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var line struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		if jerr := json.Unmarshal(raw, &line); jerr != nil {
			continue
		}
		for _, item := range line.Items {
			if item.ID != "" {
				ids[item.ID] = true
			}
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, scanErr
	}
	return ids, nil
}

// parseFinding reads a finding file and returns a next-work item.
//
// Title comes from the YAML frontmatter's "title:" line if present,
// otherwise from the first Markdown H1 after the frontmatter. Description
// is the first non-empty paragraph under the "## Summary" heading, or the
// first non-empty paragraph of the body if no Summary section exists.
// Severity comes from the frontmatter's "severity:" line, defaulting to
// "medium" when absent.
func parseFinding(id, path, cwd string) (routedFinding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return routedFinding{}, err
	}
	content := string(data)

	frontmatter, body := splitFrontmatter(content)
	title := extractFrontmatterField(frontmatter, "title")
	severity := extractFrontmatterField(frontmatter, "severity")
	if severity == "" {
		severity = "medium"
	} else {
		severity = mapSeverity(severity)
	}
	if title == "" {
		title = extractFirstHeading(body)
	}

	description := extractSummary(body)
	if description == "" {
		description = extractFirstParagraph(body)
	}

	relPath, relErr := filepath.Rel(cwd, path)
	if relErr != nil {
		relPath = path
	}

	return routedFinding{
		ID:          id,
		Title:       title,
		Type:        "tech-debt",
		Severity:    severity,
		Source:      "council-finding",
		Description: description,
		TargetRepo:  filepath.Base(cwd),
		SourcePath:  relPath,
	}, nil
}

// splitFrontmatter returns (frontmatter, body). If the file does not open
// with a "---" fence the whole content is treated as body.
func splitFrontmatter(content string) (string, string) {
	trimmed := strings.TrimLeft(content, "\n")
	if !strings.HasPrefix(trimmed, "---") {
		return "", content
	}
	rest := strings.TrimPrefix(trimmed, "---")
	rest = strings.TrimLeft(rest, "\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", content
	}
	fm := rest[:end]
	body := rest[end+len("\n---"):]
	body = strings.TrimLeft(body, "-\n")
	return fm, body
}

// extractFrontmatterField returns the quoted or bare value of a simple
// top-level "key: value" line. Lists and nested maps are not supported;
// they return "".
func extractFrontmatterField(frontmatter, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(frontmatter, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		value = strings.Trim(value, `"'`)
		if strings.HasPrefix(value, "[") {
			return ""
		}
		return value
	}
	return ""
}

// mapSeverity normalizes the finding-schema severity vocabulary to the
// next-work.jsonl severity enum, which is strictly "low|medium|high"
// (see docs/contracts/next-work.schema.md §Enums → Severity). Finding-schema
// values above "high" ("critical", "blocker") collapse to "high" rather than
// emitting a value the next-work contract rejects.
func mapSeverity(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "minor", "low":
		return "low"
	case "medium", "significant":
		return "medium"
	case "high", "major", "critical", "blocker":
		return "high"
	default:
		return "medium"
	}
}

// extractFirstHeading returns the text after the first "# " H1 line.
func extractFirstHeading(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "#"))
		}
	}
	return ""
}

// extractSummary returns the first non-empty paragraph following a
// "## Summary" heading, or "" when the section is absent.
func extractSummary(body string) string {
	lines := strings.Split(body, "\n")
	inSummary := false
	var buf []string
	for _, line := range lines {
		if strings.HasPrefix(line, "## Summary") {
			inSummary = true
			continue
		}
		if !inSummary {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			break
		}
		if strings.TrimSpace(line) == "" {
			if len(buf) > 0 {
				break
			}
			continue
		}
		buf = append(buf, strings.TrimSpace(line))
	}
	return strings.Join(buf, " ")
}

// extractFirstParagraph returns the first non-empty paragraph of the body,
// skipping heading lines.
func extractFirstParagraph(body string) string {
	var buf []string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(buf) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		buf = append(buf, trimmed)
	}
	return strings.Join(buf, " ")
}
