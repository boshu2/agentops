package warmind

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	// PendingDir holds candidates awaiting scoring.
	PendingDir = "pending"

	// StagedDir holds candidates awaiting citations.
	StagedDir = "staged"

	// RejectedDir holds rejected candidates.
	RejectedDir = "rejected"

	// ChainFile is the audit log of all pool operations.
	ChainFile = "chain.jsonl"
)

// validIDPattern matches safe candidate IDs.
var validIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Pool manages the warmind candidate pool.
type Pool struct {
	// BaseDir is the working directory (repo root).
	BaseDir string

	// PoolPath is the full path to .warmind/pool.
	PoolPath string

	// Config is the warmind configuration.
	Config Config
}

// NewPool creates a new pool manager.
func NewPool(baseDir string, cfg Config) *Pool {
	return &Pool{
		BaseDir:  baseDir,
		PoolPath: filepath.Join(baseDir, cfg.PoolDir),
		Config:   cfg,
	}
}

// Init creates the required directory structure.
func (p *Pool) Init() error {
	dirs := []string{
		filepath.Join(p.PoolPath, PendingDir),
		filepath.Join(p.PoolPath, StagedDir),
		filepath.Join(p.PoolPath, RejectedDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	return nil
}

// validateCandidateID checks if an ID is safe for use in file paths.
func validateCandidateID(id string) error {
	if id == "" {
		return fmt.Errorf("empty candidate ID")
	}
	if len(id) > 128 {
		return fmt.Errorf("candidate ID too long (max 128)")
	}
	if !validIDPattern.MatchString(id) {
		return fmt.Errorf("candidate ID contains invalid characters")
	}
	return nil
}

// Add adds a new candidate to the pending pool.
func (p *Pool) Add(candidate Candidate) error {
	if err := validateCandidateID(candidate.ID); err != nil {
		return fmt.Errorf("invalid candidate ID: %w", err)
	}

	if err := p.Init(); err != nil {
		return fmt.Errorf("init pool: %w", err)
	}

	// Check for duplicates by content hash
	if existing := p.findByContentHash(candidate.ContentHash); existing != nil {
		return fmt.Errorf("duplicate content: already exists as %s", existing.Candidate.ID)
	}

	entry := PoolEntry{
		Candidate: candidate,
		Status:    StatusPending,
		AddedAt:   time.Now(),
		UpdatedAt: time.Now(),
	}

	filename := fmt.Sprintf("%s.json", candidate.ID)
	path := filepath.Join(p.PoolPath, PendingDir, filename)

	if err := p.writeEntry(path, &entry); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}

	p.recordEvent(ChainEvent{
		Timestamp:   time.Now(),
		Operation:   "add",
		CandidateID: candidate.ID,
		ToStatus:    StatusPending,
		Actor:       candidate.Author,
	})

	return nil
}

// Score scores a pending candidate and moves it to staged.
func (p *Pool) Score(candidateID string, scoring ScoringResult) error {
	entry, err := p.Get(candidateID)
	if err != nil {
		return err
	}

	if entry.Status != StatusPending {
		return fmt.Errorf("candidate %s is not pending (status: %s)", candidateID, entry.Status)
	}

	// Reject if below discard threshold
	if scoring.Tier == TierDiscard {
		return p.Reject(candidateID, "auto-reject: score below threshold")
	}

	// Move to staged
	oldPath := filepath.Join(p.PoolPath, PendingDir, candidateID+".json")
	newPath := filepath.Join(p.PoolPath, StagedDir, candidateID+".json")

	entry.Scoring = scoring
	entry.Status = StatusStaged
	entry.UpdatedAt = time.Now()

	if err := p.writeEntry(newPath, entry); err != nil {
		return fmt.Errorf("write staged entry: %w", err)
	}

	if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove pending entry: %v\n", err)
	}

	p.recordEvent(ChainEvent{
		Timestamp:   time.Now(),
		Operation:   "score",
		CandidateID: candidateID,
		FromStatus:  StatusPending,
		ToStatus:    StatusStaged,
		Reason:      fmt.Sprintf("tier=%s score=%.2f", scoring.Tier, scoring.CompositeScore),
	})

	return nil
}

// RecordCitation records a citation for a staged candidate.
func (p *Pool) RecordCitation(candidateID string, citedBy, citedByEmail string, isSelf bool) error {
	entry, err := p.Get(candidateID)
	if err != nil {
		return err
	}

	if entry.Status != StatusStaged {
		return fmt.Errorf("candidate %s is not staged (status: %s)", candidateID, entry.Status)
	}

	// Only count non-self citations (unless config allows self)
	if !isSelf || p.Config.Promotion.SelfCitationAllowed {
		entry.CitationCount++
		now := time.Now()
		entry.LastCitedAt = &now
		entry.UpdatedAt = now

		path := filepath.Join(p.PoolPath, StagedDir, candidateID+".json")
		if err := p.writeEntry(path, entry); err != nil {
			return fmt.Errorf("update entry: %w", err)
		}
	}

	return nil
}

// CheckPromotion checks if a staged candidate is eligible for promotion.
func (p *Pool) CheckPromotion(candidateID string) (bool, string) {
	entry, err := p.Get(candidateID)
	if err != nil {
		return false, err.Error()
	}

	if entry.Status != StatusStaged {
		return false, fmt.Sprintf("not staged (status: %s)", entry.Status)
	}

	age := time.Since(entry.AddedAt)
	cfg := p.Config.Promotion

	switch entry.Scoring.Tier {
	case TierGold:
		if age >= time.Duration(cfg.GoldAutoPromoteHours)*time.Hour {
			return true, "gold tier, age threshold met"
		}
		return false, fmt.Sprintf("gold tier, waiting for %d hours (current: %.1f)", cfg.GoldAutoPromoteHours, age.Hours())

	case TierSilver:
		if entry.CitationCount >= cfg.SilverCitationThreshold {
			return true, fmt.Sprintf("silver tier, %d citations", entry.CitationCount)
		}
		return false, fmt.Sprintf("silver tier, need %d citations (have %d)", cfg.SilverCitationThreshold, entry.CitationCount)

	case TierBronze:
		if entry.CitationCount >= cfg.BronzeCitationThreshold {
			return true, fmt.Sprintf("bronze tier, %d citations", entry.CitationCount)
		}
		return false, fmt.Sprintf("bronze tier, need %d citations (have %d)", cfg.BronzeCitationThreshold, entry.CitationCount)

	default:
		return false, "discard tier, not promotable"
	}
}

// Promote moves a staged candidate to the learnings directory.
func (p *Pool) Promote(candidateID string) (string, error) {
	entry, err := p.Get(candidateID)
	if err != nil {
		return "", err
	}

	eligible, reason := p.CheckPromotion(candidateID)
	if !eligible {
		return "", fmt.Errorf("not eligible for promotion: %s", reason)
	}

	// Create learnings directory if needed
	learningsDir := filepath.Join(p.BaseDir, p.Config.LearningsDir)
	if err := os.MkdirAll(learningsDir, 0700); err != nil {
		return "", fmt.Errorf("create learnings dir: %w", err)
	}

	// Write the learning file
	now := time.Now()
	artifactPath := filepath.Join(learningsDir, fmt.Sprintf("%s-%s.md", now.Format("2006-01-02"), candidateID))

	if err := p.writeLearningArtifact(artifactPath, entry, now); err != nil {
		return "", fmt.Errorf("write artifact: %w", err)
	}

	// Remove from pool
	stagedPath := filepath.Join(p.PoolPath, StagedDir, candidateID+".json")
	if err := os.Remove(stagedPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove staged entry: %v\n", err)
	}

	p.recordEvent(ChainEvent{
		Timestamp:    time.Now(),
		Operation:    "promote",
		CandidateID:  candidateID,
		FromStatus:   StatusStaged,
		ToStatus:     StatusPromoted,
		ArtifactPath: artifactPath,
		Reason:       reason,
	})

	return artifactPath, nil
}

// Reject rejects a candidate.
func (p *Pool) Reject(candidateID, reason string) error {
	entry, err := p.Get(candidateID)
	if err != nil {
		return err
	}

	priorStatus := entry.Status

	// Find the current path
	var oldPath string
	switch entry.Status {
	case StatusPending:
		oldPath = filepath.Join(p.PoolPath, PendingDir, candidateID+".json")
	case StatusStaged:
		oldPath = filepath.Join(p.PoolPath, StagedDir, candidateID+".json")
	default:
		return fmt.Errorf("cannot reject candidate with status %s", entry.Status)
	}

	// Move to rejected
	newPath := filepath.Join(p.PoolPath, RejectedDir, candidateID+".json")
	now := time.Now()

	entry.Status = StatusRejected
	entry.RejectedAt = &now
	entry.RejectionReason = reason
	entry.UpdatedAt = now

	if err := p.writeEntry(newPath, entry); err != nil {
		return fmt.Errorf("write rejected entry: %w", err)
	}

	if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove old entry: %v\n", err)
	}

	p.recordEvent(ChainEvent{
		Timestamp:   time.Now(),
		Operation:   "reject",
		CandidateID: candidateID,
		FromStatus:  priorStatus,
		ToStatus:    StatusRejected,
		Reason:      reason,
	})

	return nil
}

// Get retrieves a candidate by ID from any pool directory.
func (p *Pool) Get(candidateID string) (*PoolEntry, error) {
	if err := validateCandidateID(candidateID); err != nil {
		return nil, fmt.Errorf("invalid candidate ID: %w", err)
	}

	dirs := []string{
		filepath.Join(p.PoolPath, PendingDir),
		filepath.Join(p.PoolPath, StagedDir),
		filepath.Join(p.PoolPath, RejectedDir),
	}

	filename := candidateID + ".json"

	for _, dir := range dirs {
		path := filepath.Join(dir, filename)
		entry, err := p.readEntry(path)
		if err == nil {
			return entry, nil
		}
	}

	return nil, fmt.Errorf("candidate not found: %s", candidateID)
}

// SetCitationCount updates the citation count for a candidate.
// Used by force promotion to bypass citation checks.
func (p *Pool) SetCitationCount(candidateID string, count int) error {
	if err := validateCandidateID(candidateID); err != nil {
		return fmt.Errorf("invalid candidate ID: %w", err)
	}

	dirs := []string{
		filepath.Join(p.PoolPath, PendingDir),
		filepath.Join(p.PoolPath, StagedDir),
	}

	filename := candidateID + ".json"

	for _, dir := range dirs {
		path := filepath.Join(dir, filename)
		entry, err := p.readEntry(path)
		if err == nil {
			entry.CitationCount = count
			entry.UpdatedAt = time.Now()
			return p.writeEntry(path, entry)
		}
	}

	return fmt.Errorf("candidate not found: %s", candidateID)
}

// ListOptions configures pool listing.
type ListOptions struct {
	Status Status
	Tier   Tier
	Limit  int
}

// List returns pool entries matching the options.
func (p *Pool) List(opts ListOptions) ([]PoolEntry, error) {
	var entries []PoolEntry

	statusDirs := map[Status]string{
		StatusPending:  filepath.Join(p.PoolPath, PendingDir),
		StatusStaged:   filepath.Join(p.PoolPath, StagedDir),
		StatusRejected: filepath.Join(p.PoolPath, RejectedDir),
	}

	for status, dir := range statusDirs {
		if opts.Status != "" && opts.Status != status {
			continue
		}

		dirEntries, err := p.scanDirectory(dir, status)
		if err != nil {
			continue
		}
		entries = append(entries, dirEntries...)
	}

	// Filter by tier
	if opts.Tier != "" {
		filtered := make([]PoolEntry, 0, len(entries))
		for _, e := range entries {
			if e.Scoring.Tier == opts.Tier {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Sort by added time (newest first)
	slices.SortFunc(entries, func(a, b PoolEntry) int {
		return b.AddedAt.Compare(a.AddedAt)
	})

	// Apply limit
	if opts.Limit > 0 && len(entries) > opts.Limit {
		entries = entries[:opts.Limit]
	}

	return entries, nil
}

// ListStaged returns all staged candidates awaiting citations.
func (p *Pool) ListStaged() ([]PoolEntry, error) {
	return p.List(ListOptions{Status: StatusStaged})
}

// ListPending returns all pending candidates awaiting scoring.
func (p *Pool) ListPending() ([]PoolEntry, error) {
	return p.List(ListOptions{Status: StatusPending})
}

// GetChain returns all chain events.
func (p *Pool) GetChain() ([]ChainEvent, error) {
	chainPath := filepath.Join(p.PoolPath, ChainFile)

	f, err := os.Open(chainPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []ChainEvent
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

// --- Internal helpers ---

func (p *Pool) scanDirectory(dir string, status Status) ([]PoolEntry, error) {
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
			continue
		}

		entry.Status = status
		entries = append(entries, *entry)
	}

	return entries, nil
}

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

func (p *Pool) writeEntry(path string, entry *PoolEntry) error {
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (p *Pool) findByContentHash(hash string) *PoolEntry {
	if hash == "" {
		return nil
	}

	dirs := []string{
		filepath.Join(p.PoolPath, PendingDir),
		filepath.Join(p.PoolPath, StagedDir),
	}

	for _, dir := range dirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}

			path := filepath.Join(dir, file.Name())
			entry, err := p.readEntry(path)
			if err != nil {
				continue
			}

			if entry.Candidate.ContentHash == hash {
				return entry
			}
		}
	}

	return nil
}

func (p *Pool) recordEvent(event ChainEvent) {
	chainPath := filepath.Join(p.PoolPath, ChainFile)

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	f, err := os.OpenFile(chainPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	f.Write(append(data, '\n'))
}

func (p *Pool) writeLearningArtifact(path string, entry *PoolEntry, promotedAt time.Time) error {
	var content strings.Builder

	// YAML frontmatter
	content.WriteString("---\n")
	fmt.Fprintf(&content, "id: %s\n", entry.Candidate.ID)
	fmt.Fprintf(&content, "type: learning\n")
	fmt.Fprintf(&content, "author: %s\n", entry.Candidate.Author)
	if entry.Candidate.AuthorEmail != "" {
		fmt.Fprintf(&content, "author_email: %s\n", entry.Candidate.AuthorEmail)
	}
	fmt.Fprintf(&content, "created_at: %s\n", entry.Candidate.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&content, "promoted_at: %s\n", promotedAt.Format(time.RFC3339))
	if entry.Candidate.Category != "" {
		fmt.Fprintf(&content, "category: %s\n", entry.Candidate.Category)
	}
	if len(entry.Candidate.Tags) > 0 {
		fmt.Fprintf(&content, "tags: [%s]\n", strings.Join(entry.Candidate.Tags, ", "))
	}
	fmt.Fprintf(&content, "confidence: %.4f\n", entry.Candidate.Confidence)
	fmt.Fprintf(&content, "tier: %s\n", entry.Scoring.Tier)
	fmt.Fprintf(&content, "utility: %.4f\n", entry.Scoring.CompositeScore)
	fmt.Fprintf(&content, "maturity: provisional\n")
	fmt.Fprintf(&content, "citation_count: %d\n", entry.CitationCount)
	fmt.Fprintf(&content, "content_hash: %s\n", entry.Candidate.ContentHash)
	content.WriteString("warmind: true\n")
	content.WriteString("---\n\n")

	// Title
	fmt.Fprintf(&content, "# %s\n\n", entry.Candidate.Title)

	// Content
	content.WriteString(entry.Candidate.Content)
	content.WriteString("\n")

	return os.WriteFile(path, []byte(content.String()), 0600)
}

// ContentHash computes a SHA256 hash of normalized content for dedup.
func ContentHash(content string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(content), " "))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
