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
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// validIDPattern matches safe candidate IDs (alphanumeric, hyphens, underscores).
var validIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// writeEntryFunc is the function used to persist pool entries. Tests can
// replace this to inject write failures.
var writeEntryFunc = defaultWriteEntry

// defaultWriteEntry is the production implementation of entry writing.
func defaultWriteEntry(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}

// validateCandidateID checks if an ID is safe for use in file paths.
func validateCandidateID(id string) error {
	if id == "" {
		return ErrEmptyID
	}
	if len(id) > 128 {
		return ErrIDTooLong
	}
	if !validIDPattern.MatchString(id) {
		return ErrIDInvalidChars
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
	entries, err := p.collectEntries(opts.Status)
	if err != nil {
		return nil, err
	}

	entries = filterByTier(entries, opts.Tier)

	// Sort by added time (newest first)
	slices.SortFunc(entries, func(a, b PoolEntry) int {
		return b.AddedAt.Compare(a.AddedAt)
	})

	total := len(entries)
	entries = paginate(entries, opts.Offset, opts.Limit)

	return &ListResult{
		Entries: entries,
		Total:   total,
	}, nil
}

// collectEntries scans all pool directories, optionally filtering by status.
func (p *Pool) collectEntries(statusFilter types.PoolStatus) ([]PoolEntry, error) {
	dirs := map[types.PoolStatus]string{
		types.PoolStatusPending:  filepath.Join(p.PoolPath, PendingDir),
		types.PoolStatusStaged:   filepath.Join(p.PoolPath, StagedDir),
		types.PoolStatusArchived: filepath.Join(p.PoolPath, ValidatedDir),
		types.PoolStatusRejected: filepath.Join(p.PoolPath, RejectedDir),
	}

	var entries []PoolEntry
	for status, dir := range dirs {
		if statusFilter != "" && statusFilter != status {
			continue
		}
		dirEntries, err := p.scanDirectory(dir, status)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("scan %s: %w", dir, err)
		}
		entries = append(entries, dirEntries...)
	}
	return entries, nil
}

// filterByTier returns only entries matching the given tier; returns all if tier is empty.
func filterByTier(entries []PoolEntry, tier types.Tier) []PoolEntry {
	if tier == "" {
		return entries
	}
	filtered := make([]PoolEntry, 0, len(entries))
	for _, e := range entries {
		if e.Candidate.Tier == tier {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// paginate applies offset and limit to a slice of entries.
func paginate(entries []PoolEntry, offset, limit int) []PoolEntry {
	if offset > 0 {
		if offset >= len(entries) {
			return []PoolEntry{}
		}
		entries = entries[offset:]
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

// scanDirectory reads all entries from a pool directory.
func (p *Pool) scanDirectory(dir string, status types.PoolStatus) ([]PoolEntry, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	entries := make([]PoolEntry, 0, len(files))
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, file.Name())
		entry, err := p.readEntry(path)
		if err != nil {
			continue // Skip malformed entries
		}

		enrichEntry(entry, path, status)
		entries = append(entries, *entry)
	}

	return entries, nil
}

// enrichEntry populates computed fields on a pool entry after loading from disk.
func enrichEntry(entry *PoolEntry, path string, status types.PoolStatus) {
	entry.FilePath = path
	entry.Status = status
	entry.Age = time.Since(entry.AddedAt)
	entry.AgeString = formatDuration(entry.Age)

	// Check if approaching 24h auto-promote threshold (warn at 22h, 2h buffer)
	if entry.Candidate.Tier == types.TierSilver && entry.Age > 22*time.Hour {
		entry.ApproachingAutoPromote = true
	}
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

	return nil, fmt.Errorf("%w: %s", ErrCandidateNotFound, candidateID)
}

// FindByPrefix finds candidates whose ID starts with the given prefix.
// Returns matching entries. Useful for shortened ID lookups.
func (p *Pool) FindByPrefix(prefix string) ([]*PoolEntry, error) {
	if prefix == "" {
		return nil, fmt.Errorf("empty prefix")
	}

	dirs := []string{
		filepath.Join(p.PoolPath, PendingDir),
		filepath.Join(p.PoolPath, StagedDir),
		filepath.Join(p.PoolPath, ValidatedDir),
		filepath.Join(p.PoolPath, RejectedDir),
	}

	var matches []*PoolEntry
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			id := strings.TrimSuffix(e.Name(), ".json")
			if strings.HasPrefix(id, prefix) {
				path := filepath.Join(dir, e.Name())
				entry, err := p.readEntry(path)
				if err != nil {
					continue
				}
				entry.FilePath = path
				entry.Age = time.Since(entry.AddedAt)
				entry.AgeString = formatDuration(entry.Age)
				matches = append(matches, entry)
			}
		}
	}

	return matches, nil
}

// Stage moves a candidate from pending to staged.
func (p *Pool) Stage(candidateID string, minTier types.Tier) error {
	entry, err := p.Get(candidateID)
	if err != nil {
		return err
	}

	// Prevent staging rejected candidates
	if entry.Status == types.PoolStatusRejected {
		return ErrStageRejected
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
	p.recordEventOrWarn(ChainEvent{
		Timestamp:   time.Now(),
		Operation:   "stage",
		CandidateID: candidateID,
		FromStatus:  types.PoolStatusPending,
		ToStatus:    types.PoolStatusStaged,
	})

	return nil
}

// Promote moves a staged candidate to learnings/patterns.
func (p *Pool) Promote(candidateID string) (string, error) {
	entry, err := p.Get(candidateID)
	if err != nil {
		return "", err
	}
	if err := validatePromotable(entry); err != nil {
		return "", err
	}

	destDir := promotionDir(p.BaseDir, entry.Candidate.Type)
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return "", fmt.Errorf("create destination: %w", err)
	}

	artifactPath := resolveArtifactPath(destDir, candidateID)

	if err := p.writeArtifact(artifactPath, entry); err != nil {
		return "", fmt.Errorf("write artifact: %w", err)
	}

	if err := os.Remove(entry.FilePath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove pool entry: %v\n", err)
	}

	p.recordEventOrWarn(ChainEvent{
		Timestamp:    time.Now(),
		Operation:    "promote",
		CandidateID:  candidateID,
		FromStatus:   entry.Status,
		ToStatus:     types.PoolStatusArchived,
		ArtifactPath: artifactPath,
	})

	return artifactPath, nil
}

// validatePromotable checks that a pool entry is eligible for promotion.
func validatePromotable(entry *PoolEntry) error {
	if entry.Status == types.PoolStatusRejected {
		return ErrPromoteRejected
	}
	if entry.Status != types.PoolStatusStaged {
		return fmt.Errorf("%w (current: %s)", ErrNotStaged, entry.Status)
	}
	return nil
}

// promotionDir returns the destination directory for a given knowledge type.
func promotionDir(baseDir string, knowledgeType types.KnowledgeType) string {
	switch knowledgeType {
	case types.KnowledgeTypeDecision:
		return filepath.Join(baseDir, ".agents", "patterns")
	case types.KnowledgeTypeSolution, types.KnowledgeTypeLearning,
		types.KnowledgeTypeFailure, types.KnowledgeTypeReference:
		return filepath.Join(baseDir, ".agents", "learnings")
	default:
		return filepath.Join(baseDir, ".agents", "learnings")
	}
}

// resolveArtifactPath generates a unique artifact file path, adding a hash suffix on collision.
func resolveArtifactPath(destDir, candidateID string) string {
	timestamp := time.Now().Format("2006-01-02")
	artifactName := fmt.Sprintf("%s-%s.md", timestamp, candidateID)
	artifactPath := filepath.Join(destDir, artifactName)

	if _, err := os.Stat(artifactPath); err == nil {
		h := sha256.Sum256([]byte(candidateID + time.Now().String()))
		suffix := hex.EncodeToString(h[:4])
		artifactName = fmt.Sprintf("%s-%s-%s.md", timestamp, candidateID, suffix)
		artifactPath = filepath.Join(destDir, artifactName)
	}
	return artifactPath
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

	// Capture status before mutation for chain event
	priorStatus := entry.Status

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
		FromStatus:  priorStatus,
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
	p.recordEventOrWarn(ChainEvent{
		Timestamp:   time.Now(),
		Operation:   "approve",
		CandidateID: candidateID,
		Reason:      note,
		Reviewer:    reviewer,
	})

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
	slices.SortFunc(pending, func(a, b PoolEntry) int {
		return a.AddedAt.Compare(b.AddedAt)
	})

	return pending, nil
}

// MinBulkApproveThreshold is the minimum duration for bulk approval.
// Prevents accidental approval of very recent candidates.
const MinBulkApproveThreshold = time.Hour

// MaxReasonLength is the maximum length for reason/note fields.
// Prevents excessively large review notes that could slow down operations.
const MaxReasonLength = 1000

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
			if id, ok := p.bulkApproveEntry(entry, reviewer, dryRun); ok {
				approved = append(approved, id)
			}
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
	return writeEntryFunc(path, data)
}

// knowledgeTypeHeading returns the markdown heading prefix for a knowledge type.
func knowledgeTypeHeading(kt types.KnowledgeType) string {
	switch kt {
	case types.KnowledgeTypeLearning:
		return "# Learning: "
	case types.KnowledgeTypeDecision:
		return "# Decision: "
	case types.KnowledgeTypeSolution:
		return "# Solution: "
	case types.KnowledgeTypeFailure:
		return "# Failure: "
	case types.KnowledgeTypeReference:
		return "# Reference: "
	default:
		return "# Knowledge: "
	}
}

// firstLineTitle extracts and truncates the first line of content for use as a title.
func firstLineTitle(content string) string {
	firstLine, _, _ := strings.Cut(content, "\n")
	if len(firstLine) > 80 {
		firstLine = truncateAtWordBoundary(firstLine, 77) + "..."
	}
	return firstLine
}

// writeArtifact writes a promoted candidate as markdown with YAML frontmatter.
// The frontmatter is required for the maturity scanner (ratchet.readLearningData)
// to parse utility, maturity, and feedback fields.
func (p *Pool) writeArtifact(path string, entry *PoolEntry) error {
	var content strings.Builder

	// YAML frontmatter — required for maturity scanner compatibility
	content.WriteString("---\n")
	fmt.Fprintf(&content, "id: %s\n", entry.Candidate.ID)
	fmt.Fprintf(&content, "type: %s\n", entry.Candidate.Type)
	fmt.Fprintf(&content, "date: %s\n", time.Now().Format("2006-01-02"))
	fmt.Fprintf(&content, "tier: %s\n", entry.Candidate.Tier)
	fmt.Fprintf(&content, "utility: %.4f\n", entry.Candidate.Utility)
	fmt.Fprintf(&content, "confidence: %.4f\n", entry.Candidate.Confidence)
	fmt.Fprintf(&content, "maturity: %s\n", entry.Candidate.Maturity)
	fmt.Fprintf(&content, "reward_count: %d\n", entry.Candidate.RewardCount)
	fmt.Fprintf(&content, "helpful_count: %d\n", entry.Candidate.HelpfulCount)
	fmt.Fprintf(&content, "harmful_count: %d\n", entry.Candidate.HarmfulCount)
	if entry.Candidate.Source.SessionID != "" {
		fmt.Fprintf(&content, "source_session: %s\n", entry.Candidate.Source.SessionID)
	} else {
		content.WriteString("source_session: unknown\n")
	}
	content.WriteString("---\n\n")

	// Heading
	content.WriteString(knowledgeTypeHeading(entry.Candidate.Type))
	content.WriteString(firstLineTitle(entry.Candidate.Content))
	content.WriteString("\n\n")

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
	if entry.Candidate.Source.SessionID != "" {
		fmt.Fprintf(&content, "- **Session**: %s\n", entry.Candidate.Source.SessionID)
		fmt.Fprintf(&content, "- **Transcript**: %s\n", entry.Candidate.Source.TranscriptPath)
		fmt.Fprintf(&content, "- **Message**: %d\n\n", entry.Candidate.Source.MessageIndex)
	} else {
		content.WriteString("- **Source**: unknown\n\n")
	}

	return os.WriteFile(path, []byte(content.String()), 0600)
}

// jsonMarshalFunc is used by recordEvent. Tests can replace to inject marshal errors.
var jsonMarshalFunc = json.Marshal

// openChainFileFunc opens the chain file for appending. Tests can replace to inject errors.
var openChainFileFunc = defaultOpenChainFile

func defaultOpenChainFile(path string) (fileWriter, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
}

// recordEvent appends an event to the chain file.
func (p *Pool) recordEvent(event ChainEvent) (err error) {
	chainPath := filepath.Join(p.PoolPath, ChainFile)

	data, err := jsonMarshalFunc(event)
	if err != nil {
		return err
	}

	f, err := openChainFileFunc(chainPath)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if _, err = f.Write(append(data, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// readCloser abstracts a readable, closeable resource for testing.
type readCloser interface {
	io.Reader
	Close() error
}

// openIfExistsFunc opens a file for reading, returning (nil, nil) if it does not exist.
// Tests can replace this to inject close errors.
var openIfExistsFunc = defaultOpenIfExists

func defaultOpenIfExists(path string) (readCloser, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

// openIfExists is a convenience wrapper used by non-GetChain callers.
func openIfExists(path string) (*os.File, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return f, err
}

// GetChain returns all chain events.
func (p *Pool) GetChain() (events []ChainEvent, err error) {
	f, err := openIfExistsFunc(filepath.Join(p.PoolPath, ChainFile))
	if err != nil || f == nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	events, err = scanChainEvents(f)
	return
}

// scanChainEvents reads all chain events from r, skipping malformed lines.
func scanChainEvents(r io.Reader) ([]ChainEvent, error) {
	scanner := bufio.NewScanner(r)
	var events []ChainEvent
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
	// Fast path: byte length bounds rune count from above, so any input that
	// already fits within `limit` bytes also fits within `limit` runes and
	// needs no allocation.
	if len(s) <= limit {
		return s
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	// Build the rune-safe prefix and search for the last space within it.
	prefix := string(runes[:limit])
	lastSpace := strings.LastIndex(prefix, " ")
	if lastSpace == -1 {
		// No space found, truncate at limit
		return prefix
	}
	// lastSpace is a byte offset in prefix; space is ASCII so slice is safe.
	return prefix[:lastSpace]
}

// atomicMove moves a file atomically using the pattern:
// write-to-temp → sync → chmod 0600 → rename
// This prevents partial writes and data corruption during moves.
// randReadFunc reads random bytes. Tests can replace to inject errors.
var randReadFunc = rand.Read

func atomicMove(srcPath, destPath string) error {
	// Generate random suffix for temp file
	randBytes := make([]byte, 4)
	if _, err := randReadFunc(randBytes); err != nil {
		return fmt.Errorf("generate random suffix: %w", err)
	}
	tempPath := destPath + ".tmp." + hex.EncodeToString(randBytes)

	// Read source file
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}

	// Write, sync, and close into temp file
	if err := writeTempFile(tempPath, data); err != nil {
		return err
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

// fileWriter abstracts file write operations for testing.
type fileWriter interface {
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

// openTempFileFunc creates a temp file. Tests can replace to return a mock writer.
var openTempFileFunc = defaultOpenTempFile

func defaultOpenTempFile(tempPath string) (fileWriter, error) {
	return os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
}

// writeTempFile creates a temp file, writes data, syncs to disk, and closes.
// On any error before Close it cleans up the temp file before returning.
func writeTempFile(tempPath string, data []byte) error {
	tempFile, err := openTempFileFunc(tempPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()    //nolint:errcheck // cleanup in error path
		_ = os.Remove(tempPath) //nolint:errcheck // cleanup in error path
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()    //nolint:errcheck // cleanup in error path
		_ = os.Remove(tempPath) //nolint:errcheck // cleanup in error path
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath) //nolint:errcheck // cleanup in error path
		return fmt.Errorf("close temp file: %w", err)
	}

	return nil
}

// recordEventOrWarn records a chain event, printing a warning on failure.
// Non-fatal: callers should not abort on recording failures.
func (p *Pool) recordEventOrWarn(event ChainEvent) {
	if err := p.recordEvent(event); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to record event: %v\n", err)
	}
}

// bulkApproveEntry approves a single entry, or records it as dry-run.
// Returns (candidateID, true) on success or dry-run, ("", false) on error.
func (p *Pool) bulkApproveEntry(entry PoolEntry, reviewer string, dryRun bool) (string, bool) {
	if dryRun {
		return entry.Candidate.ID, true
	}
	if err := p.Approve(entry.Candidate.ID, "bulk-approve: auto-promoted after threshold", reviewer); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to approve %s: %v\n", entry.Candidate.ID, err)
		return "", false
	}
	return entry.Candidate.ID, true
}
