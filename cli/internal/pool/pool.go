// Package pool manages knowledge candidate pools for the quality pipeline.
// Candidates flow through: pending → staged → promoted (or rejected)
package pool

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// validIDPattern matches safe candidate IDs (alphanumeric, hyphens, underscores).
var validIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// validateCandidateID checks if an ID is safe for use in file paths.
func validateCandidateID(id string) error {
	if id == "" {
		return fmt.Errorf("candidate ID cannot be empty")
	}
	if len(id) > 128 {
		return fmt.Errorf("candidate ID too long (max 128 characters)")
	}
	if !validIDPattern.MatchString(id) {
		return fmt.Errorf("candidate ID contains invalid characters (only alphanumeric, hyphen, underscore allowed)")
	}
	return nil
}

const (
	// PoolDir is the base directory for pool storage.
	PoolDir = ".agents/pool"

	// PendingDir holds candidates awaiting scoring/review.
	PendingDir = "pending"

	// StagedDir holds candidates ready for promotion.
	StagedDir = "staged"

	// RejectedDir holds rejected candidates for audit.
	RejectedDir = "rejected"

	// ValidatedDir holds validated candidates (legacy name for staged).
	ValidatedDir = "validated"

	// IndexFile is the JSONL index of all pool entries.
	IndexFile = "index.jsonl"

	// ChainFile records all pool operations.
	ChainFile = "chain.jsonl"
)

// PoolEntry extends types.PoolEntry with operational fields.
type PoolEntry struct {
	types.PoolEntry

	// FilePath is where this entry is stored.
	FilePath string `json:"file_path,omitempty"`

	// Age is how long since the entry was added.
	Age time.Duration `json:"-"`

	// AgeString is the human-readable age.
	AgeString string `json:"age,omitempty"`

	// ApproachingAutoPromote indicates if nearing 24h threshold.
	ApproachingAutoPromote bool `json:"approaching_auto_promote,omitempty"`
}

// ChainEvent records a pool operation.
type ChainEvent struct {
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Operation is the action taken (add, stage, promote, reject).
	Operation string `json:"operation"`

	// CandidateID is the affected candidate.
	CandidateID string `json:"candidate_id"`

	// FromStatus is the previous status.
	FromStatus types.PoolStatus `json:"from_status,omitempty"`

	// ToStatus is the new status.
	ToStatus types.PoolStatus `json:"to_status,omitempty"`

	// Reason explains why the operation occurred.
	Reason string `json:"reason,omitempty"`

	// Reviewer is who performed the operation.
	Reviewer string `json:"reviewer,omitempty"`

	// ArtifactPath is the destination path for promotions.
	ArtifactPath string `json:"artifact_path,omitempty"`
}

// Pool manages the candidate pool.
type Pool struct {
	// BaseDir is the working directory.
	BaseDir string

	// PoolPath is the full path to .agents/pool.
	PoolPath string
}

// NewPool creates a new pool manager.
func NewPool(baseDir string) *Pool {
	return &Pool{
		BaseDir:  baseDir,
		PoolPath: filepath.Join(baseDir, PoolDir),
	}
}

// Init creates the required directory structure.
func (p *Pool) Init() error {
	dirs := []string{
		filepath.Join(p.PoolPath, PendingDir),
		filepath.Join(p.PoolPath, StagedDir),
		filepath.Join(p.PoolPath, ValidatedDir), // Alias for staged
		filepath.Join(p.PoolPath, RejectedDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	return nil
}

// ListOptions configures pool listing.
type ListOptions struct {
	// Tier filters by quality tier.
	Tier types.Tier

	// Status filters by pool status.
	Status types.PoolStatus

	// Offset skips the first N results (for pagination).
	Offset int

	// Limit caps the number of results.
	Limit int
}

// ListResult contains pool entries and pagination metadata.
type ListResult struct {
	// Entries is the page of results.
	Entries []PoolEntry

	// Total is the count of all matching entries before pagination.
	Total int
}

// List returns pool entries matching the options.
func (p *Pool) List(opts ListOptions) ([]PoolEntry, error) {
	result, err := p.ListPaginated(opts)
	if err != nil {
		return nil, err
	}
	return result.Entries, nil
}

// ListPaginated returns pool entries with pagination metadata.
func (p *Pool) ListPaginated(opts ListOptions) (*ListResult, error) {
	var entries []PoolEntry

	// Scan all pool directories
	dirs := map[types.PoolStatus]string{
		types.PoolStatusPending:  filepath.Join(p.PoolPath, PendingDir),
		types.PoolStatusStaged:   filepath.Join(p.PoolPath, StagedDir),
		types.PoolStatusArchived: filepath.Join(p.PoolPath, ValidatedDir),
		types.PoolStatusRejected: filepath.Join(p.PoolPath, RejectedDir),
	}

	for status, dir := range dirs {
		// Skip if filtering by status and this doesn't match
		if opts.Status != "" && opts.Status != status {
			continue
		}

		dirEntries, err := p.scanDirectory(dir, status)
		if err != nil {
			if os.IsNotExist(err) {
				// Directory doesn't exist yet, expected for new pools
				continue
			}
			// Other errors (permission, disk) should be reported
			return nil, fmt.Errorf("scan %s: %w", dir, err)
		}

		entries = append(entries, dirEntries...)
	}

	// Filter by tier if specified
	if opts.Tier != "" {
		filtered := make([]PoolEntry, 0)
		for _, e := range entries {
			if e.Candidate.Tier == opts.Tier {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Sort by added time (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].AddedAt.After(entries[j].AddedAt)
	})

	total := len(entries)

	// Apply offset
	if opts.Offset > 0 {
		if opts.Offset >= len(entries) {
			entries = nil
		} else {
			entries = entries[opts.Offset:]
		}
	}

	// Apply limit
	if opts.Limit > 0 && len(entries) > opts.Limit {
		entries = entries[:opts.Limit]
	}

	return &ListResult{
		Entries: entries,
		Total:   total,
	}, nil
}

// scanDirectory reads all entries from a pool directory.
func (p *Pool) scanDirectory(dir string, status types.PoolStatus) ([]PoolEntry, error) {
	var entries []PoolEntry

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, file.Name())
		entry, err := p.readEntry(path)
		if err != nil {
			continue // Skip malformed entries
		}

		entry.FilePath = path
		entry.Status = status
		entry.Age = time.Since(entry.AddedAt)
		entry.AgeString = formatDuration(entry.Age)

		// Check if approaching 24h auto-promote threshold (warn at 22h, 2h buffer)
		if entry.Candidate.Tier == types.TierSilver && entry.Age > 22*time.Hour {
			entry.ApproachingAutoPromote = true
		}

		entries = append(entries, *entry)
	}

	return entries, nil
}

// readEntry loads a single pool entry from file.
func (p *Pool) readEntry(path string) (*PoolEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entry PoolEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

// Get retrieves a specific candidate by ID.
func (p *Pool) Get(candidateID string) (*PoolEntry, error) {
	// Validate ID to prevent path traversal
	if err := validateCandidateID(candidateID); err != nil {
		return nil, fmt.Errorf("invalid candidate ID: %w", err)
	}

	// Search all directories
	dirs := []string{
		filepath.Join(p.PoolPath, PendingDir),
		filepath.Join(p.PoolPath, StagedDir),
		filepath.Join(p.PoolPath, ValidatedDir),
		filepath.Join(p.PoolPath, RejectedDir),
	}

	// Build expected filename for exact match
	expectedFilename := candidateID + ".json"

	for _, dir := range dirs {
		path := filepath.Join(dir, expectedFilename)
		entry, err := p.readEntry(path)
		if err != nil {
			continue
		}
		entry.FilePath = path
		entry.Age = time.Since(entry.AddedAt)
		entry.AgeString = formatDuration(entry.Age)
		return entry, nil
	}

	return nil, fmt.Errorf("candidate not found: %s", candidateID)
}

// Stage moves a candidate from pending to staged.
func (p *Pool) Stage(candidateID string, minTier types.Tier) error {
	entry, err := p.Get(candidateID)
	if err != nil {
		return err
	}

	// Prevent staging rejected candidates
	if entry.Status == types.PoolStatusRejected {
		return fmt.Errorf("cannot stage rejected candidate")
	}

	// Validate tier threshold
	if !isAboveThreshold(entry.Candidate.Tier, minTier) {
		return fmt.Errorf("candidate tier %s below minimum %s", entry.Candidate.Tier, minTier)
	}

	// Move file atomically
	newPath := filepath.Join(p.PoolPath, StagedDir, filepath.Base(entry.FilePath))
	if err := atomicMove(entry.FilePath, newPath); err != nil {
		return fmt.Errorf("move to staged: %w", err)
	}

	// Update status
	entry.Status = types.PoolStatusStaged
	entry.UpdatedAt = time.Now()

	// Write updated entry
	if err := p.writeEntry(newPath, entry); err != nil {
		return fmt.Errorf("write staged entry: %w", err)
	}

	// Record chain event
	if err := p.recordEvent(ChainEvent{
		Timestamp:   time.Now(),
		Operation:   "stage",
		CandidateID: candidateID,
		FromStatus:  types.PoolStatusPending,
		ToStatus:    types.PoolStatusStaged,
	}); err != nil {
		// Non-fatal, continue
		fmt.Fprintf(os.Stderr, "Warning: failed to record event: %v\n", err)
	}

	return nil
}

// Promote moves a staged candidate to learnings/patterns.
func (p *Pool) Promote(candidateID string) (string, error) {
	entry, err := p.Get(candidateID)
	if err != nil {
		return "", err
	}

	// Prevent promoting rejected candidates
	if entry.Status == types.PoolStatusRejected {
		return "", fmt.Errorf("cannot promote rejected candidate")
	}
	if entry.Status != types.PoolStatusStaged {
		return "", fmt.Errorf("candidate must be staged before promotion (current: %s)", entry.Status)
	}

	// Determine destination based on type
	var destDir string
	switch entry.Candidate.Type {
	case types.KnowledgeTypeLearning, types.KnowledgeTypeSolution:
		destDir = filepath.Join(p.BaseDir, ".agents", "learnings")
	case types.KnowledgeTypeDecision:
		destDir = filepath.Join(p.BaseDir, ".agents", "patterns")
	default:
		destDir = filepath.Join(p.BaseDir, ".agents", "learnings")
	}

	if err := os.MkdirAll(destDir, 0700); err != nil {
		return "", fmt.Errorf("create destination: %w", err)
	}

	// Generate artifact filename using full candidate ID to avoid collisions.
	// Previous truncation to 8 chars caused all candidates with shared prefixes
	// (e.g., "pend-202...") to overwrite each other — data loss bug.
	timestamp := time.Now().Format("2006-01-02")
	artifactName := fmt.Sprintf("%s-%s.md", timestamp, candidateID)
	artifactPath := filepath.Join(destDir, artifactName)

	// Guard against unlikely collisions (same ID promoted twice in one day).
	if _, err := os.Stat(artifactPath); err == nil {
		h := sha256.Sum256([]byte(candidateID + time.Now().String()))
		suffix := hex.EncodeToString(h[:4])
		artifactName = fmt.Sprintf("%s-%s-%s.md", timestamp, candidateID, suffix)
		artifactPath = filepath.Join(destDir, artifactName)
	}

	// Write artifact as markdown
	if err := p.writeArtifact(artifactPath, entry); err != nil {
		return "", fmt.Errorf("write artifact: %w", err)
	}

	// Remove from pool
	if err := os.Remove(entry.FilePath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove pool entry: %v\n", err)
	}

	// Record chain event
	if err := p.recordEvent(ChainEvent{
		Timestamp:    time.Now(),
		Operation:    "promote",
		CandidateID:  candidateID,
		FromStatus:   entry.Status,
		ToStatus:     types.PoolStatusArchived,
		ArtifactPath: artifactPath,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to record event: %v\n", err)
	}

	return artifactPath, nil
}

// Reject marks a candidate as rejected.
func (p *Pool) Reject(candidateID, reason, reviewer string) error {
	// Validate reason length
	if len(reason) > MaxReasonLength {
		return ErrReasonTooLong
	}

	entry, err := p.Get(candidateID)
	if err != nil {
		return err
	}

	// Move to rejected directory atomically
	newPath := filepath.Join(p.PoolPath, RejectedDir, filepath.Base(entry.FilePath))
	if err := atomicMove(entry.FilePath, newPath); err != nil {
		return fmt.Errorf("move to rejected: %w", err)
	}

	// Update entry
	entry.Status = types.PoolStatusRejected
	entry.UpdatedAt = time.Now()
	entry.HumanReview = &types.HumanReview{
		Reviewed:   true,
		Approved:   false,
		Reviewer:   reviewer,
		Notes:      reason,
		ReviewedAt: time.Now(),
	}

	if err := p.writeEntry(newPath, entry); err != nil {
		return fmt.Errorf("write rejected entry: %w", err)
	}

	// Record chain event
	if err := p.recordEvent(ChainEvent{
		Timestamp:   time.Now(),
		Operation:   "reject",
		CandidateID: candidateID,
		FromStatus:  entry.Status,
		ToStatus:    types.PoolStatusRejected,
		Reason:      reason,
		Reviewer:    reviewer,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to record event: %v\n", err)
	}

	return nil
}

// Approve records human approval for a bronze candidate.
func (p *Pool) Approve(candidateID, note, reviewer string) error {
	// Validate note length
	if len(note) > MaxReasonLength {
		return ErrReasonTooLong
	}

	entry, err := p.Get(candidateID)
	if err != nil {
		return err
	}

	// Check if already reviewed
	if entry.HumanReview != nil && entry.HumanReview.Reviewed {
		return fmt.Errorf("already reviewed by %s", entry.HumanReview.Reviewer)
	}

	// Update entry with review
	entry.HumanReview = &types.HumanReview{
		Reviewed:   true,
		Approved:   true,
		Reviewer:   reviewer,
		Notes:      note,
		ReviewedAt: time.Now(),
	}
	entry.UpdatedAt = time.Now()

	if err := p.writeEntry(entry.FilePath, entry); err != nil {
		return fmt.Errorf("write approved entry: %w", err)
	}

	// Record chain event
	if err := p.recordEvent(ChainEvent{
		Timestamp:   time.Now(),
		Operation:   "approve",
		CandidateID: candidateID,
		Reason:      note,
		Reviewer:    reviewer,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to record event: %v\n", err)
	}

	return nil
}

// ListPendingReview returns bronze candidates awaiting human review.
func (p *Pool) ListPendingReview() ([]PoolEntry, error) {
	entries, err := p.List(ListOptions{
		Tier:   types.TierBronze,
		Status: types.PoolStatusPending,
	})
	if err != nil {
		return nil, err
	}

	// Filter to only those without review
	var pending []PoolEntry
	for _, e := range entries {
		if e.HumanReview == nil || !e.HumanReview.Reviewed {
			pending = append(pending, e)
		}
	}

	// Sort by age (oldest first for urgency)
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].AddedAt.Before(pending[j].AddedAt)
	})

	return pending, nil
}

// MinBulkApproveThreshold is the minimum duration for bulk approval.
// Prevents accidental approval of very recent candidates.
const MinBulkApproveThreshold = time.Hour

// MaxReasonLength is the maximum length for reason/note fields.
// Prevents excessively large review notes that could slow down operations.
const MaxReasonLength = 1000

// ErrThresholdTooLow is returned when bulk approval threshold is below minimum.
var ErrThresholdTooLow = fmt.Errorf("threshold must be >= 1h")

// ErrReasonTooLong is returned when reason/note exceeds MaxReasonLength.
var ErrReasonTooLong = fmt.Errorf("reason/note exceeds maximum length of %d characters", MaxReasonLength)

// BulkApprove approves all silver candidates older than threshold.
// Returns ErrThresholdTooLow if olderThan < 1h to prevent accidental mass approval.
func (p *Pool) BulkApprove(olderThan time.Duration, reviewer string, dryRun bool) ([]string, error) {
	if olderThan < MinBulkApproveThreshold {
		return nil, ErrThresholdTooLow
	}

	entries, err := p.List(ListOptions{
		Tier:   types.TierSilver,
		Status: types.PoolStatusPending,
	})
	if err != nil {
		return nil, err
	}

	var approved []string
	for _, entry := range entries {
		if entry.Age >= olderThan {
			if dryRun {
				approved = append(approved, entry.Candidate.ID)
				continue
			}

			if err := p.Approve(entry.Candidate.ID, "bulk-approve: auto-promoted after threshold", reviewer); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to approve %s: %v\n", entry.Candidate.ID, err)
				continue
			}
			approved = append(approved, entry.Candidate.ID)
		}
	}

	return approved, nil
}

// Add adds a new candidate to the pending pool.
func (p *Pool) Add(candidate types.Candidate, scoring types.Scoring) error {
	return p.AddAt(candidate, scoring, time.Now())
}

// AddAt adds a new candidate to the pending pool with a caller-supplied AddedAt timestamp.
// This is useful when ingesting historical artifacts where "age" should reflect the original
// creation/modification time, not the ingestion time.
func (p *Pool) AddAt(candidate types.Candidate, scoring types.Scoring, addedAt time.Time) error {
	// Validate ID to prevent path traversal
	if err := validateCandidateID(candidate.ID); err != nil {
		return fmt.Errorf("invalid candidate ID: %w", err)
	}

	if err := p.Init(); err != nil {
		return fmt.Errorf("init pool: %w", err)
	}

	entry := types.PoolEntry{
		Candidate:     candidate,
		ScoringResult: scoring,
		Status:        types.PoolStatusPending,
		AddedAt:       addedAt,
		UpdatedAt:     time.Now(),
	}

	// Mark if gate required
	if scoring.GateRequired {
		entry.HumanReview = &types.HumanReview{
			Reviewed: false,
		}
	}

	// Write to pending directory
	filename := fmt.Sprintf("%s.json", candidate.ID)
	path := filepath.Join(p.PoolPath, PendingDir, filename)

	if err := p.writeEntry(path, &PoolEntry{PoolEntry: entry}); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}

	// Record chain event
	if err := p.recordEvent(ChainEvent{
		Timestamp:   time.Now(),
		Operation:   "add",
		CandidateID: candidate.ID,
		ToStatus:    types.PoolStatusPending,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to record event: %v\n", err)
	}

	return nil
}

// writeEntry writes a pool entry to JSON file.
func (p *Pool) writeEntry(path string, entry *PoolEntry) error {
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// writeArtifact writes a promoted candidate as markdown.
func (p *Pool) writeArtifact(path string, entry *PoolEntry) error {
	var content strings.Builder

	// Title
	switch entry.Candidate.Type {
	case types.KnowledgeTypeLearning:
		content.WriteString("# Learning: ")
	case types.KnowledgeTypeDecision:
		content.WriteString("# Decision: ")
	case types.KnowledgeTypeSolution:
		content.WriteString("# Solution: ")
	default:
		content.WriteString("# Knowledge: ")
	}

	// Use first line of content as title
	firstLine := entry.Candidate.Content
	if idx := strings.Index(firstLine, "\n"); idx > 0 {
		firstLine = firstLine[:idx]
	}
	if len(firstLine) > 80 {
		firstLine = truncateAtWordBoundary(firstLine, 77) + "..."
	}
	content.WriteString(firstLine)
	content.WriteString("\n\n")

	// Metadata
	content.WriteString(fmt.Sprintf("**ID**: %s\n", entry.Candidate.ID))
	content.WriteString(fmt.Sprintf("**Date**: %s\n", time.Now().Format("2006-01-02")))
	content.WriteString(fmt.Sprintf("**Tier**: %s\n", entry.Candidate.Tier))
	content.WriteString("**Schema Version**: 1\n")
	content.WriteString("\n")

	// MemRL fields
	content.WriteString("## MemRL Metrics\n\n")
	content.WriteString(fmt.Sprintf("- **Utility**: %.2f\n", entry.Candidate.Utility))
	content.WriteString(fmt.Sprintf("- **Confidence**: %.2f\n", entry.Candidate.Confidence))
	content.WriteString(fmt.Sprintf("- **Maturity**: %s\n", entry.Candidate.Maturity))
	content.WriteString("\n")

	// Content
	content.WriteString("## What We Learned\n\n")
	content.WriteString(entry.Candidate.Content)
	content.WriteString("\n\n")

	// Context
	if entry.Candidate.Context != "" {
		content.WriteString("## Context\n\n")
		content.WriteString(entry.Candidate.Context)
		content.WriteString("\n\n")
	}

	// Provenance
	content.WriteString("## Source\n\n")
	content.WriteString(fmt.Sprintf("- **Session**: %s\n", entry.Candidate.Source.SessionID))
	content.WriteString(fmt.Sprintf("- **Transcript**: %s\n", entry.Candidate.Source.TranscriptPath))
	content.WriteString(fmt.Sprintf("- **Message**: %d\n", entry.Candidate.Source.MessageIndex))
	content.WriteString("\n")

	return os.WriteFile(path, []byte(content.String()), 0600)
}

// recordEvent appends an event to the chain file.
func (p *Pool) recordEvent(event ChainEvent) (err error) {
	chainPath := filepath.Join(p.PoolPath, ChainFile)

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(chainPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	_, err = f.Write(append(data, '\n'))
	return err
}

// GetChain returns all chain events.
func (p *Pool) GetChain() (events []ChainEvent, err error) {
	chainPath := filepath.Join(p.PoolPath, ChainFile)

	f, err := os.Open(chainPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event ChainEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		events = append(events, event)
	}

	return events, scanner.Err()
}

// isAboveThreshold checks if a tier meets the minimum.
func isAboveThreshold(tier, minTier types.Tier) bool {
	tierOrder := map[types.Tier]int{
		types.TierGold:    3,
		types.TierSilver:  2,
		types.TierBronze:  1,
		types.TierDiscard: 0,
	}

	return tierOrder[tier] >= tierOrder[minTier]
}

// formatDuration formats a duration as human-readable.
func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// truncateAtWordBoundary truncates a string at the last space before limit.
// If no space is found before limit, it truncates at limit exactly.
func truncateAtWordBoundary(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	// Find the last space before the limit
	lastSpace := strings.LastIndex(s[:limit], " ")
	if lastSpace == -1 {
		// No space found, truncate at limit
		return s[:limit]
	}
	return s[:lastSpace]
}

// atomicMove moves a file atomically using the pattern:
// write-to-temp → sync → chmod 0600 → rename
// This prevents partial writes and data corruption during moves.
func atomicMove(srcPath, destPath string) error {
	// Generate random suffix for temp file
	randBytes := make([]byte, 4)
	if _, err := rand.Read(randBytes); err != nil {
		return fmt.Errorf("generate random suffix: %w", err)
	}
	tempPath := destPath + ".tmp." + hex.EncodeToString(randBytes)

	// Read source file
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}

	// Write to temp file
	tempFile, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	// Write data
	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()    //nolint:errcheck // cleanup in error path
		_ = os.Remove(tempPath) //nolint:errcheck // cleanup in error path
		return fmt.Errorf("write temp file: %w", err)
	}

	// Sync to disk
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()    //nolint:errcheck // cleanup in error path
		_ = os.Remove(tempPath) //nolint:errcheck // cleanup in error path
		return fmt.Errorf("sync temp file: %w", err)
	}

	// Close before rename
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath) //nolint:errcheck // cleanup in error path
		return fmt.Errorf("close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, destPath); err != nil {
		_ = os.Remove(tempPath) //nolint:errcheck // cleanup in error path
		return fmt.Errorf("rename to destination: %w", err)
	}

	// Remove source file
	if err := os.Remove(srcPath); err != nil {
		// Non-fatal: the move succeeded, just cleanup failed
		fmt.Fprintf(os.Stderr, "Warning: failed to remove source file %s: %v\n", srcPath, err)
	}

	return nil
}
