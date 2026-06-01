package storage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var closeLockedFile = func(f *os.File) error {
	return f.Close()
}

const (
	// DefaultBaseDir is the default storage directory.
	DefaultBaseDir = ".agents/ao"

	// SessionsDir holds session markdown/jsonl files.
	SessionsDir = "sessions"

	// IndexDir holds the session index.
	IndexDir = "index"

	// ProvenanceDir holds provenance records.
	ProvenanceDir = "provenance"

	// IndexFile is the name of the main index file.
	IndexFile = "sessions.jsonl"

	// ProvenanceFile is the name of the provenance graph.
	ProvenanceFile = "graph.jsonl"

	// SlugMaxLength is the maximum length for URL-safe slugs.
	SlugMaxLength = 50

	// SlugMinWordBoundary is the minimum length before trimming at word boundary.
	SlugMinWordBoundary = 30

	// defaultSlug is the fallback slug when text is empty or non-alphanumeric.
	defaultSlug = "session"
)

// FileStorage implements Storage using the local filesystem.
type FileStorage struct {
	// BaseDir is the root directory (e.g., .agents/ao).
	BaseDir string

	// Formatters are the output formats to use.
	Formatters []Formatter

	mu sync.Mutex
}

// FileStorageOption configures a FileStorage instance.
type FileStorageOption func(*FileStorage)

// WithBaseDir sets the base directory.
func WithBaseDir(dir string) FileStorageOption {
	return func(fs *FileStorage) {
		fs.BaseDir = dir
	}
}

// WithFormatters sets the output formatters.
func WithFormatters(formatters ...Formatter) FileStorageOption {
	return func(fs *FileStorage) {
		fs.Formatters = formatters
	}
}

// NewFileStorage creates a new file-based storage.
func NewFileStorage(opts ...FileStorageOption) *FileStorage {
	fs := &FileStorage{
		BaseDir:    DefaultBaseDir,
		Formatters: nil, // Will be set by caller
	}

	for _, opt := range opts {
		opt(fs)
	}

	return fs
}

// Init creates the required directory structure.
func (fs *FileStorage) Init() error {
	dirs := []string{
		filepath.Join(fs.BaseDir, SessionsDir),
		filepath.Join(fs.BaseDir, IndexDir),
		filepath.Join(fs.BaseDir, ProvenanceDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	return nil
}

// WriteSession writes a session to storage using all configured formatters.
// Returns the path to the primary session file.
func (fs *FileStorage) WriteSession(session *Session) (string, error) {
	if session.ID == "" {
		return "", ErrSessionIDRequired
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Generate unique filename: YYYY-MM-DD-{slug}-{sessionID[:7]}.ext
	slug := generateSlug(session.Summary)
	shortID := session.ID
	if len(shortID) > 7 {
		shortID = shortID[:7]
	}

	dateStr := session.Date.Format("2006-01-02")
	baseName := fmt.Sprintf("%s-%s-%s", dateStr, slug, shortID)

	var primaryPath string

	// Write using each formatter
	for i, formatter := range fs.Formatters {
		ext := formatter.Extension()
		filename := baseName + ext
		fullPath := filepath.Join(fs.BaseDir, SessionsDir, filename)

		if err := fs.atomicWrite(fullPath, func(w io.Writer) error {
			return formatter.Format(w, session)
		}); err != nil {
			return "", fmt.Errorf("write %s format: %w", ext, err)
		}

		// First formatter produces the primary path
		if i == 0 {
			primaryPath = fullPath
		}
	}

	return primaryPath, nil
}

// WriteIndex appends an entry to the session index.
func (fs *FileStorage) WriteIndex(entry *IndexEntry) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	indexPath := filepath.Join(fs.BaseDir, IndexDir, IndexFile)

	return fs.withLockedFile(indexPath, func(f *os.File) error {
		hasEntry, err := fs.hasIndexEntry(f, entry.SessionID)
		if err != nil {
			return err
		}
		if hasEntry {
			return nil
		}
		return fs.appendJSONLToFile(f, entry)
	})
}

// WriteProvenance records provenance information.
func (fs *FileStorage) WriteProvenance(record *ProvenanceRecord) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	provPath := filepath.Join(fs.BaseDir, ProvenanceDir, ProvenanceFile)
	return fs.appendJSONL(provPath, record)
}

// ReadSession retrieves a session by ID.
func (fs *FileStorage) ReadSession(sessionID string) (*Session, error) {
	// Find session file by scanning index
	entries, err := fs.ListSessions()
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.SessionID == sessionID {
			// Read the session file
			return fs.readSessionFile(entry.SessionPath)
		}
	}

	return nil, fmt.Errorf("session not found: %s", sessionID)
}

// ListSessions returns all session index entries.
func (fs *FileStorage) ListSessions() ([]IndexEntry, error) {
	indexPath := filepath.Join(fs.BaseDir, IndexDir, IndexFile)

	var entries []IndexEntry
	err := scanJSONLFile(indexPath, func(line []byte) {
		var entry IndexEntry
		if err := json.Unmarshal(line, &entry); err == nil {
			entries = append(entries, entry)
		}
	})
	return entries, err
}

// QueryProvenance finds provenance records for an artifact.
func (fs *FileStorage) QueryProvenance(artifactPath string) ([]ProvenanceRecord, error) {
	provPath := filepath.Join(fs.BaseDir, ProvenanceDir, ProvenanceFile)

	var records []ProvenanceRecord
	err := scanJSONLFile(provPath, func(line []byte) {
		var record ProvenanceRecord
		if err := json.Unmarshal(line, &record); err == nil && record.ArtifactPath == artifactPath {
			records = append(records, record)
		}
	})
	return records, err
}

// scanJSONLFile opens a JSONL file and calls fn for each non-empty line.
// Returns nil (not an error) if the file does not exist.
func scanJSONL(r io.Reader, fn func(line []byte)) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fn(scanner.Bytes())
	}
	return scanner.Err()
}

func scanJSONLFile(path string, fn func(line []byte)) (err error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	return scanJSONL(f, fn)
}

// Close releases any resources.
func (fs *FileStorage) Close() error {
	return nil // No resources to release for file storage
}

// atomicWrite writes to a temp file and renames atomically.
// writeSyncClose writes content to a file, syncs to disk, and closes.
// On write or sync failure, the file is closed before returning.
func writeSyncClose(f *os.File, writeFunc func(io.Writer) error) error {
	if err := writeFunc(f); err != nil {
		_ = f.Close() //nolint:errcheck // cleanup in error path
		return fmt.Errorf("write content: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close() //nolint:errcheck // cleanup in error path
		return fmt.Errorf("sync file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	return nil
}

func (fs *FileStorage) atomicWrite(path string, writeFunc func(io.Writer) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on error
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath) //nolint:errcheck // cleanup in error path
		}
	}()

	if err := writeSyncClose(tmpFile, writeFunc); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename to final: %w", err)
	}

	success = true
	return nil
}

// appendJSONL appends a JSON line to a file atomically.
func (fs *FileStorage) appendJSONL(path string, v any) error {
	return fs.withLockedFile(path, func(f *os.File) error {
		return fs.appendJSONLToFile(f, v)
	})
}

func (fs *FileStorage) appendJSONLToFile(f *os.File, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek end: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write line: %w", err)
	}

	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", err)
	}

	return nil
}

// hasIndexEntry checks if a session ID already exists in the locked index file.
func (fs *FileStorage) hasIndexEntry(f *os.File, sessionID string) (bool, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false, fmt.Errorf("seek start: %w", err)
	}

	found := false
	err := scanJSONL(f, func(line []byte) {
		var entry IndexEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return
		}
		if entry.SessionID == sessionID {
			found = true
		}
	})
	if err != nil {
		return false, err
	}

	return found, nil
}

func (fs *FileStorage) withLockedFile(path string, fn func(f *os.File) error) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}

	locked := false
	// Deferred cleanup: unlock (if locked) then close. The first error
	// encountered is propagated only when the function would otherwise
	// return nil, so the caller always sees the most significant error.
	defer func() {
		if locked {
			if unlockErr := unlockFile(f); unlockErr != nil && err == nil {
				err = fmt.Errorf("unlock file: %w", unlockErr)
			}
		}
		if closeErr := closeLockedFile(f); closeErr != nil && err == nil {
			err = fmt.Errorf("close file: %w", closeErr)
		}
	}()

	if err := lockFile(f); err != nil {
		// Close the file explicitly — the deferred Close will no-op on an
		// already-closed file, and we must not leak the descriptor.
		if closeErr := f.Close(); closeErr != nil {
			return fmt.Errorf("lock file: %w; close file: %v", err, closeErr)
		}
		return fmt.Errorf("lock file: %w", err)
	}
	locked = true

	return fn(f)
}

// readSessionFile reads a session from a JSONL file.
func (fs *FileStorage) readSessionFile(path string) (*Session, error) {
	// For JSONL format, read and parse
	if strings.HasSuffix(path, ".jsonl") {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = f.Close() //nolint:errcheck // read-only, errors non-critical
		}()

		scanner := bufio.NewScanner(f)
		if scanner.Scan() {
			var session Session
			if err := json.Unmarshal(scanner.Bytes(), &session); err != nil {
				return nil, err
			}
			return &session, nil
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("read session file: %w", err)
		}
		return nil, ErrEmptySessionFile
	}

	// For markdown format, we'd need to parse YAML frontmatter
	// For now, return error if not JSONL
	return nil, fmt.Errorf("unsupported format: %s", filepath.Ext(path))
}

// generateSlug creates a URL-safe slug from text.
func generateSlug(text string) string {
	if text == "" {
		return defaultSlug
	}

	s := slugify(strings.ToLower(text))
	s = truncateSlug(s)

	if s == "" {
		return defaultSlug
	}
	return s
}

// isSlugChar returns true for lowercase alphanumeric characters kept in slugs.
func isSlugChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

// slugify replaces non-alphanumeric runs with single hyphens and trims leading/trailing hyphens.
func slugify(input string) string {
	var result strings.Builder
	lastHyphen := false
	for _, r := range input {
		if isSlugChar(r) {
			result.WriteRune(r)
			lastHyphen = false
		} else if !lastHyphen {
			result.WriteRune('-')
			lastHyphen = true
		}
	}
	return strings.Trim(result.String(), "-")
}

// truncateSlug limits the slug to SlugMaxLength, preferring word boundaries.
func truncateSlug(s string) string {
	if len(s) <= SlugMaxLength {
		return s
	}
	s = s[:SlugMaxLength]
	if idx := strings.LastIndex(s, "-"); idx > SlugMinWordBoundary {
		s = s[:idx]
	}
	return s
}

// GetBaseDir returns the configured base directory.
func (fs *FileStorage) GetBaseDir() string {
	return fs.BaseDir
}

// SessionsPath returns the full path to the sessions directory.
func (fs *FileStorage) SessionsPath() string {
	return filepath.Join(fs.BaseDir, SessionsDir)
}

// IndexPath returns the full path to the index file.
func (fs *FileStorage) IndexPath() string {
	return filepath.Join(fs.BaseDir, IndexDir, IndexFile)
}

// ProvenancePath returns the full path to the provenance file.
func (fs *FileStorage) ProvenancePath() string {
	return filepath.Join(fs.BaseDir, ProvenanceDir, ProvenanceFile)
}
