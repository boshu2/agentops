package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/boshu2/agentops/cli/internal/storage"
)

// MutateContext carries the per-run state every Mutate call needs. It is built
// once per `fix`/`undo` invocation and shared across all fixers in the run.
type MutateContext struct {
	RunID        string
	RunDir       string
	Capabilities *Capabilities
	RepoRoot     string
	HomeDir      string
	FixerID      string
	Locks        *LockManager
	DryRun       bool

	actionsFile *os.File
	actionsMu   sync.Mutex
	start       time.Time
}

// NewMutateContext builds a MutateContext for a run. actionsFile is the
// already-open append handle for the run's actions.jsonl.
func NewMutateContext(ra *RunArtifact, caps *Capabilities, homeDir string, locks *LockManager, actionsFile *os.File, dryRun bool) *MutateContext {
	return &MutateContext{
		RunID:        ra.RunID,
		RunDir:       ra.RunDir,
		Capabilities: caps,
		RepoRoot:     ra.RepoRoot,
		HomeDir:      homeDir,
		Locks:        locks,
		DryRun:       dryRun,
		actionsFile:  actionsFile,
		start:        time.Now(),
	}
}

// WithFixer returns a shallow copy of ctx scoped to a fixer id. The actions
// file handle, mutex, lock manager, and start time are shared.
func (ctx *MutateContext) WithFixer(fixerID string) *MutateContext {
	return &MutateContext{
		RunID:        ctx.RunID,
		RunDir:       ctx.RunDir,
		Capabilities: ctx.Capabilities,
		RepoRoot:     ctx.RepoRoot,
		HomeDir:      ctx.HomeDir,
		FixerID:      fixerID,
		Locks:        ctx.Locks,
		DryRun:       ctx.DryRun,
		actionsFile:  ctx.actionsFile,
		start:        ctx.start,
	}
}

// ActionResult is the outcome of a single Mutate call.
type ActionResult struct {
	OK         bool
	BeforeHash string
	AfterHash  string
	Err        error
}

// ActionRecord is one line in actions.jsonl. doctor undo reads these in reverse.
type ActionRecord struct {
	Path         string `json:"path"`
	Op           string `json:"op"`
	BeforeHash   string `json:"before_hash"`
	AfterHash    string `json:"after_hash"`
	BeforeMode   string `json:"before_mode,omitempty"`
	BackupPath   string `json:"backup_path,omitempty"` // immutable, relative to the owning run
	StartedAtNS  int64  `json:"started_at_ns"`
	FinishedAtNS int64  `json:"finished_at_ns"`
	RunID        string `json:"run_id"`
	FixerID      string `json:"fixer_id"`
	OK           bool   `json:"ok"`
	RenameTo     string `json:"rename_to,omitempty"`
	Existed      bool   `json:"existed"`
	Error        string `json:"error,omitempty"`
	RolledBack   bool   `json:"rolled_back,omitempty"`
}

// readOrEmpty reads a file, returning nil bytes (no error) if it does not exist.
func readOrEmpty(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	}
	return b, err
}

// Mutate is the single chokepoint through which every `fix`/`undo` disk write
// flows. It implements the 8-step shape from MUTATE-CHOKEPOINT.md: per-path
// lock, before_hash, in-scope precondition, verbatim backup (mode+mtime
// preserved, cmp-strict verified), plan, atomic execute, after_hash, and an
// fsync'd actions.jsonl line. It is the ONLY function in the doctor subsystem
// allowed to write to user-state disk under fix.
func Mutate(ctx *MutateContext, path string, op Op) (ActionResult, error) {
	// Step 1 — per-path advisory lock.
	if !ctx.DryRun {
		guard, err := ctx.Locks.Acquire(path)
		if err != nil {
			return ActionResult{Err: err}, err
		}
		defer func() { _ = guard.Release() }()
	}

	// Step 2 — before_hash.
	beforeBytes, err := readOrEmpty(path)
	if err != nil {
		return ActionResult{Err: err}, fmt.Errorf("doctor: read %s: %w", path, err)
	}
	beforeHash := sha256Hex(beforeBytes)
	info, statErr := os.Stat(path)
	existed := statErr == nil
	beforeMode := ""
	if existed {
		beforeMode = fmt.Sprintf("%o", info.Mode().Perm())
	}

	// Step 3 — preconditions: in-scope path + executable op.
	if err := EnsureInScope(ctx.Capabilities, ctx.RepoRoot, ctx.HomeDir, path); err != nil {
		return ActionResult{Err: err}, err
	}
	if err := EnsureOpAllowed(ctx.Capabilities, op); err != nil {
		return ActionResult{Err: err}, err
	}

	// Step 4 — one immutable backup per action, contained within this run.
	backupPath := ""
	if !ctx.DryRun && existed {
		backupPath, err = writeActionBackup(ctx, beforeBytes, info)
		if err != nil {
			return ActionResult{Err: err}, fmt.Errorf("doctor: backup %s: %w", path, err)
		}
		current, readErr := os.ReadFile(path)
		if readErr != nil || sha256Hex(current) != beforeHash {
			err := fmt.Errorf("doctor: source changed while backing up %s", path)
			return ActionResult{Err: err}, err
		}
	}

	// Step 5/6 — plan + atomic execute. plan is implicit in op; execute is atomic.
	startedNS := time.Since(ctx.start).Nanoseconds()
	if ctx.DryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] would mutate %s: %s\n", path, DescribeOp(op))
		return ActionResult{OK: true, BeforeHash: beforeHash, AfterHash: beforeHash}, nil
	}
	if err := executeAtomic(path, op); err != nil {
		return ActionResult{Err: err}, fmt.Errorf("doctor: execute %s on %s: %w", op.kind(), path, err)
	}

	// Step 7 — after_hash.
	afterBytes, err := readOrEmpty(path)
	if err != nil {
		return ActionResult{Err: err}, fmt.Errorf("doctor: read-back %s: %w", path, err)
	}
	afterHash := sha256Hex(afterBytes)

	// Step 8 — record the action line, fsync'd.
	rel, relErr := filepath.Rel(ctx.RepoRoot, path)
	if relErr != nil {
		rel = path
	}
	rec := ActionRecord{
		Path:         rel,
		Op:           op.kind(),
		BeforeHash:   beforeHash,
		AfterHash:    afterHash,
		BeforeMode:   beforeMode,
		BackupPath:   backupPath,
		StartedAtNS:  startedNS,
		FinishedAtNS: time.Since(ctx.start).Nanoseconds(),
		RunID:        ctx.RunID,
		FixerID:      ctx.FixerID,
		OK:           true,
		Existed:      existed,
	}
	if r, ok := op.(Rename); ok {
		rec.RenameTo = r.To
	}
	if err := ctx.appendAction(rec); err != nil {
		return ActionResult{Err: err}, err
	}

	return ActionResult{OK: true, BeforeHash: beforeHash, AfterHash: afterHash}, nil
}

// appendAction writes one fsync'd line to actions.jsonl under the actions mutex.
func (ctx *MutateContext) appendAction(rec ActionRecord) error {
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("doctor: marshal action record: %w", err)
	}
	line = append(line, '\n')
	ctx.actionsMu.Lock()
	defer ctx.actionsMu.Unlock()
	if _, err := ctx.actionsFile.Write(line); err != nil {
		return fmt.Errorf("doctor: append actions.jsonl: %w", err)
	}
	return ctx.actionsFile.Sync()
}

// executeAtomic performs the op's disk write using the appropriate atomic
// primitive. File writes use os.CreateTemp in the SAME directory + os.Rename.
func executeAtomic(path string, op Op) error {
	parent := filepath.Dir(path)
	switch v := op.(type) {
	case WriteFile:
		return atomicWrite(path, v.Content, v.Mode)
	case AppendFile:
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		if _, err := f.Write(v.Content); err != nil {
			return err
		}
		return f.Sync()
	case Rename:
		if err := os.MkdirAll(filepath.Dir(v.To), 0o755); err != nil {
			return err
		}
		return os.Rename(path, v.To)
	case Chmod:
		return os.Chmod(path, v.Mode)
	case SymlinkAtomic:
		tmp := filepath.Join(parent, fmt.Sprintf(".%s.doctor-symlink.%d", filepath.Base(path), time.Now().UnixNano()))
		if err := os.Symlink(v.Target, tmp); err != nil {
			return err
		}
		if err := os.Rename(tmp, path); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		return nil
	case DbExec, DbMigrate:
		return ErrDBOpsUnused
	default:
		return fmt.Errorf("doctor: unknown op %T", op)
	}
}

// atomicWrite writes content to path durably and atomically, delegating to the
// canonical storage.AtomicWriteFile (same-dir temp + fsync + chmod + rename, and
// it creates a missing parent dir — matching WriteFile's create-or-overwrite
// semantics). A WriteFile op with an unset mode defaults to 0o644, preserved here.
func atomicWrite(path string, content []byte, mode os.FileMode) error {
	if mode == 0 {
		mode = 0o644
	}
	return storage.AtomicWriteFile(path, content, mode)
}
