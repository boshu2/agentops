package beads

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

type BeadRecord struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	Body        string   `json:"body"`
	Status      string   `json:"status"`
	IssueType   string   `json:"issue_type"`
	Type        string   `json:"type"`
	Kind        string   `json:"kind"`
	CreatedAt   string   `json:"created_at"`
	Labels      []string `json:"labels"`
	Children    []any    `json:"children"`
}

func (record BeadRecord) DisplayTitle() string {
	if record.Title != "" {
		return record.Title
	}
	return record.Subject
}

func (record BeadRecord) TextBody() string {
	if record.Body != "" {
		return record.Body
	}
	return record.Description
}

func (record BeadRecord) IsEpic() bool {
	for _, value := range []string{record.IssueType, record.Type, record.Kind} {
		if strings.EqualFold(value, "epic") {
			return true
		}
	}
	return len(record.Children) > 0
}

type AuditCommit struct {
	ShortSHA string
	Subject  string
	Body     string
	CommitAt time.Time
	Files    map[string]struct{}
}

type HygieneRepository interface {
	Available() bool
	List(string) ([]BeadRecord, error)
	Show(string) (BeadRecord, error)
	Commits() []AuditCommit
	PatternExists(string) bool
	Close(string, string) error
	Reparent(string, string) error
}

type HygieneContextRepository interface {
	ListContext(context.Context, string) ([]BeadRecord, error)
	ShowContext(context.Context, string) (BeadRecord, error)
	CloseContext(context.Context, string, string) error
	ReparentContext(context.Context, string, string) error
}

type AuditFinding struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Reason   string `json:"reason"`
	Evidence string `json:"evidence,omitempty"`
}

type AuditConsolidation struct {
	File    string   `json:"file"`
	BeadIDs []string `json:"bead_ids"`
}

type AuditSummary struct {
	LikelyFixed    int `json:"likely_fixed"`
	LikelyStale    int `json:"likely_stale"`
	Consolidatable int `json:"consolidatable"`
	Total          int `json:"total"`
	FlaggedPct     int `json:"flagged_pct,omitempty"`
}

type AuditReport struct {
	LikelyFixed    []AuditFinding       `json:"likely_fixed"`
	LikelyStale    []AuditFinding       `json:"likely_stale"`
	Consolidatable []AuditConsolidation `json:"consolidatable"`
	Summary        AuditSummary         `json:"summary"`
	BDAvailable    bool                 `json:"bd_available"`
	Error          string               `json:"error,omitempty"`
	Warnings       []string             `json:"-"`
}

func (report *AuditReport) FlaggedCount() int {
	return report.Summary.LikelyFixed + report.Summary.LikelyStale + report.Summary.Consolidatable
}

type ClusterBead struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	IsEpic bool   `json:"is_epic"`
}

type BeadCluster struct {
	Representative string        `json:"representative"`
	SharedKeywords []string      `json:"shared_keywords"`
	Beads          []ClusterBead `json:"beads"`
}

type ClusterReport struct {
	Clusters    []BeadCluster `json:"clusters"`
	Unclustered []ClusterBead `json:"unclustered"`
	Message     string        `json:"message,omitempty"`
	BDAvailable bool          `json:"bd_available"`
	Applied     int           `json:"applied,omitempty"`
	ApplyErrors []string      `json:"apply_errors,omitempty"`
	Error       string        `json:"error,omitempty"`
}

type HygieneUseCases interface {
	Audit(bool) (*AuditReport, error)
	Cluster(bool) (*ClusterReport, error)
}

type HygieneContextUseCases interface {
	AuditContext(context.Context, bool) (*AuditReport, error)
	ClusterContext(context.Context, bool) (*ClusterReport, error)
}

type HygieneService struct {
	Repository HygieneRepository
}

func (service HygieneService) Audit(autoClose bool) (*AuditReport, error) {
	return service.AuditContext(context.Background(), autoClose)
}

func (service HygieneService) AuditContext(ctx context.Context, autoClose bool) (*AuditReport, error) {
	report := &AuditReport{LikelyFixed: []AuditFinding{}, LikelyStale: []AuditFinding{}, Consolidatable: []AuditConsolidation{}}
	if service.Repository == nil || !service.Repository.Available() {
		report.Error = "bd CLI not found"
		return report, nil
	}
	report.BDAvailable = true
	open, err := service.list(ctx, "open")
	if err != nil {
		return nil, err
	}
	inProgress, err := service.list(ctx, "in_progress")
	if err != nil {
		return nil, err
	}
	records := append(open, inProgress...)
	report.Summary.Total = len(records)
	commits := service.Repository.Commits()
	fileToBeads := make(map[string]map[string]bool)
	for _, record := range records {
		if record.ID == "" {
			continue
		}
		body := record.TextBody()
		if evidence := GrepCommitsForID(commits, record.ID); evidence != "" {
			report.LikelyFixed = append(report.LikelyFixed, AuditFinding{ID: record.ID, Title: record.DisplayTitle(), Reason: "commit_match", Evidence: evidence})
			if autoClose {
				if closeErr := service.close(ctx, record.ID, "Auto-closed by ao beads audit: commit evidence found: "+evidence); closeErr != nil {
					report.Warnings = append(report.Warnings, closeErr.Error())
				}
			}
			continue
		}
		paths := ExtractAuditFilePaths(body, 10)
		if record.CreatedAt != "" && len(paths) > 0 {
			if evidence := FileChangesSinceCommits(commits, record.CreatedAt, paths); evidence != "" {
				report.LikelyFixed = append(report.LikelyFixed, AuditFinding{ID: record.ID, Title: record.DisplayTitle(), Reason: "file_modified_since_creation", Evidence: evidence})
				if autoClose {
					if closeErr := service.close(ctx, record.ID, "Auto-closed by ao beads audit: mentioned files modified since creation."); closeErr != nil {
						report.Warnings = append(report.Warnings, closeErr.Error())
					}
				}
				continue
			}
		}
		for _, path := range paths {
			if fileToBeads[path] == nil {
				fileToBeads[path] = make(map[string]bool)
			}
			fileToBeads[path][record.ID] = true
		}
		patterns := ExtractAuditPatterns(body, 10)
		found := false
		for _, pattern := range patterns {
			if service.Repository.PatternExists(pattern) {
				found = true
				break
			}
		}
		if len(patterns) > 0 && !found {
			report.LikelyStale = append(report.LikelyStale, AuditFinding{ID: record.ID, Title: record.DisplayTitle(), Reason: "referenced_patterns_not_found"})
		}
	}
	consolidatable := make(map[string]bool)
	for path, ids := range fileToBeads {
		if len(ids) < 2 {
			continue
		}
		beadIDs := SortedMapKeys(ids)
		for _, id := range beadIDs {
			consolidatable[id] = true
		}
		report.Consolidatable = append(report.Consolidatable, AuditConsolidation{File: path, BeadIDs: beadIDs})
	}
	sort.Slice(report.Consolidatable, func(left, right int) bool {
		return report.Consolidatable[left].File < report.Consolidatable[right].File
	})
	report.Summary.LikelyFixed = len(report.LikelyFixed)
	report.Summary.LikelyStale = len(report.LikelyStale)
	report.Summary.Consolidatable = len(consolidatable)
	if report.Summary.Total > 0 {
		report.Summary.FlaggedPct = report.FlaggedCount() * 100 / report.Summary.Total
	}
	return report, nil
}

func (service HygieneService) Cluster(apply bool) (*ClusterReport, error) {
	return service.ClusterContext(context.Background(), apply)
}

func (service HygieneService) ClusterContext(ctx context.Context, apply bool) (*ClusterReport, error) {
	report := &ClusterReport{Clusters: []BeadCluster{}, Unclustered: []ClusterBead{}}
	if service.Repository == nil || !service.Repository.Available() {
		report.Error = "bd CLI not found"
		return report, nil
	}
	report.BDAvailable = true
	records, err := service.list(ctx, "open")
	if err != nil {
		return nil, err
	}
	for index, record := range records {
		if enriched, showErr := service.show(ctx, record.ID); showErr == nil && enriched.ID != "" {
			if enriched.Title == "" {
				enriched.Title = record.Title
			}
			if len(enriched.Labels) == 0 {
				enriched.Labels = record.Labels
			}
			records[index] = enriched
		}
	}
	if len(records) < 2 {
		report.Message = "fewer than 2 open beads — nothing to cluster"
		return report, nil
	}
	report.Clusters, report.Unclustered = ClusterBeadRecords(records)
	if apply {
		for _, cluster := range report.Clusters {
			for _, bead := range cluster.Beads {
				if bead.ID == cluster.Representative {
					continue
				}
				if err := service.reparent(ctx, bead.ID, cluster.Representative); err != nil {
					report.ApplyErrors = append(report.ApplyErrors, fmt.Sprintf("%s -> %s: %v", bead.ID, cluster.Representative, err))
				} else {
					report.Applied++
				}
			}
		}
	}
	return report, nil
}

func (service HygieneService) list(ctx context.Context, status string) ([]BeadRecord, error) {
	if contextual, ok := service.Repository.(HygieneContextRepository); ok {
		return contextual.ListContext(ctx, status)
	}
	return service.Repository.List(status)
}

func (service HygieneService) show(ctx context.Context, id string) (BeadRecord, error) {
	if contextual, ok := service.Repository.(HygieneContextRepository); ok {
		return contextual.ShowContext(ctx, id)
	}
	return service.Repository.Show(id)
}

func (service HygieneService) close(ctx context.Context, id, note string) error {
	if contextual, ok := service.Repository.(HygieneContextRepository); ok {
		return contextual.CloseContext(ctx, id, note)
	}
	return service.Repository.Close(id, note)
}

func (service HygieneService) reparent(ctx context.Context, id, parent string) error {
	if contextual, ok := service.Repository.(HygieneContextRepository); ok {
		return contextual.ReparentContext(ctx, id, parent)
	}
	return service.Repository.Reparent(id, parent)
}

func ParseBDRecordList(raw []byte) ([]BeadRecord, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, nil
	}
	var records []BeadRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func ParseBDRecord(raw []byte) (BeadRecord, error) {
	trimmed := []byte(strings.TrimSpace(string(raw)))
	if len(trimmed) == 0 {
		return BeadRecord{}, nil
	}
	var records []BeadRecord
	if json.Unmarshal(trimmed, &records) == nil {
		if len(records) == 0 {
			return BeadRecord{}, nil
		}
		return records[0], nil
	}
	var record BeadRecord
	if err := json.Unmarshal(trimmed, &record); err != nil {
		return BeadRecord{}, err
	}
	return record, nil
}

func ExtractAuditFilePaths(description string, limit int) []string {
	pattern := regexp.MustCompile(`[a-zA-Z0-9_./-]+\.[a-zA-Z]{1,6}`)
	seen := make(map[string]bool)
	var paths []string
	for _, match := range pattern.FindAllString(description, -1) {
		if !strings.Contains(match, "/") || seen[match] {
			continue
		}
		seen[match] = true
		paths = append(paths, match)
		if limit > 0 && len(paths) >= limit {
			break
		}
	}
	return paths
}

func ExtractAuditPatterns(description string, limit int) []string {
	seen := make(map[string]bool)
	var patterns []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value], patterns = true, append(patterns, value)
		}
	}
	for _, match := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(description, -1) {
		add(match[1])
		if limit > 0 && len(patterns) >= limit/2 {
			break
		}
	}
	for _, match := range regexp.MustCompile(`\b[a-z][a-zA-Z0-9_]{5,}\b`).FindAllString(description, -1) {
		add(match)
		if limit > 0 && len(patterns) >= limit {
			break
		}
	}
	return patterns
}

func GrepCommitsForID(commits []AuditCommit, id string) string {
	var lines []string
	for _, commit := range commits {
		if strings.Contains(commit.Subject, id) || strings.Contains(commit.Body, id) {
			lines = append(lines, commit.ShortSHA+" "+commit.Subject)
			if len(lines) == 3 {
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

func FileChangesSinceCommits(commits []AuditCommit, createdAt string, paths []string) string {
	since, _ := time.Parse(time.RFC3339, strings.TrimSpace(createdAt))
	var chunks []string
	for _, path := range paths {
		var lines []string
		for _, commit := range commits {
			if !since.IsZero() && !commit.CommitAt.After(since) {
				continue
			}
			if _, exists := commit.Files[path]; exists {
				lines = append(lines, commit.ShortSHA+" "+commit.Subject)
				if len(lines) == 3 {
					break
				}
			}
		}
		if len(lines) > 0 {
			chunks = append(chunks, strings.Join(lines, "\n"))
		}
	}
	return strings.Join(chunks, "\n")
}

func ClusterBeadRecords(records []BeadRecord) ([]BeadCluster, []ClusterBead) {
	if len(records) == 0 {
		return []BeadCluster{}, []ClusterBead{}
	}
	parent := make([]int, len(records))
	for index := range parent {
		parent[index] = index
	}
	var find func(int) int
	find = func(index int) int {
		if parent[index] != index {
			parent[index] = find(parent[index])
		}
		return parent[index]
	}
	for left := range records {
		for right := left + 1; right < len(records); right++ {
			if ScoreBeadOverlap(records[left], records[right]) >= 2 {
				parent[find(right)] = find(left)
			}
		}
	}
	groups := make(map[int][]BeadRecord)
	for index, record := range records {
		groups[find(index)] = append(groups[find(index)], record)
	}
	var clusters []BeadCluster
	var unclustered []ClusterBead
	for _, group := range groups {
		sort.Slice(group, func(left, right int) bool { return group[left].ID < group[right].ID })
		if len(group) == 1 {
			unclustered = append(unclustered, ClusterBead{ID: group[0].ID, Title: group[0].DisplayTitle(), IsEpic: group[0].IsEpic()})
			continue
		}
		beads := make([]ClusterBead, 0, len(group))
		for _, record := range group {
			beads = append(beads, ClusterBead{ID: record.ID, Title: record.DisplayTitle(), IsEpic: record.IsEpic()})
		}
		clusters = append(clusters, BeadCluster{Representative: ClusterRepresentative(group), SharedKeywords: SharedClusterKeywords(group), Beads: beads})
	}
	sort.Slice(clusters, func(left, right int) bool { return clusters[left].Representative < clusters[right].Representative })
	sort.Slice(unclustered, func(left, right int) bool { return unclustered[left].ID < unclustered[right].ID })
	return clusters, unclustered
}

var clusterStopWords = map[string]bool{"the": true, "a": true, "an": true, "in": true, "to": true, "for": true, "of": true, "and": true, "or": true, "with": true, "is": true, "are": true, "be": true, "was": true, "were": true, "by": true, "on": true, "at": true, "from": true, "as": true, "this": true, "that": true, "it": true, "its": true, "into": true}

func ScoreBeadOverlap(left, right BeadRecord) int {
	return intersectionCount(tokenSet(left.DisplayTitle()+" "+left.TextBody()), tokenSet(right.DisplayTitle()+" "+right.TextBody())) +
		2*intersectionCount(pathSet(left.TextBody()), pathSet(right.TextBody())) + 3*intersectionCount(stringSet(left.Labels), stringSet(right.Labels))
}

func ClusterRepresentative(records []BeadRecord) string {
	for _, record := range records {
		if record.IsEpic() {
			return record.ID
		}
	}
	if len(records) > 0 {
		return records[0].ID
	}
	return ""
}

func SharedClusterKeywords(records []BeadRecord) []string {
	if len(records) == 0 {
		return nil
	}
	shared := tokenSet(records[0].DisplayTitle() + " " + records[0].TextBody())
	for _, record := range records[1:] {
		current := tokenSet(record.DisplayTitle() + " " + record.TextBody())
		for keyword := range shared {
			if !current[keyword] {
				delete(shared, keyword)
			}
		}
	}
	return SortedMapKeys(shared)
}

func tokenSet(input string) map[string]bool {
	result := make(map[string]bool)
	for _, token := range regexp.MustCompile(`[^a-z0-9/]+`).Split(strings.ToLower(input), -1) {
		if len(token) >= 3 && !clusterStopWords[token] {
			result[token] = true
		}
	}
	return result
}

func pathSet(input string) map[string]bool {
	result := make(map[string]bool)
	for _, match := range regexp.MustCompile(`[a-zA-Z0-9_./-]+/[a-zA-Z0-9_./-]+`).FindAllString(input, -1) {
		if strings.Contains(match, ".") {
			result[match] = true
		}
	}
	return result
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool)
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result[value] = true
		}
	}
	return result
}

func intersectionCount(left, right map[string]bool) int {
	count := 0
	for value := range left {
		if right[value] {
			count++
		}
	}
	return count
}

func SortedMapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

var _ HygieneUseCases = HygieneService{}
