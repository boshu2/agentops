package overnight

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/paths"
)

// DefaultWarnOnlyBudget is the number of warn-only rescues the loop grants
// before falling back to strict halting behaviour. The value is small on
// purpose: three rescues are enough to survive a single noisy iteration in
// each of the regression, plateau, and combined axes, while still bounding
// the "warn-only protects me forever" anti-pattern that the 2026-02-22
// evolve-overnight 115-cycle runaway exposed.
const DefaultWarnOnlyBudget = 3

// WarnOnlyBudgetFilename is the on-disk name of the budget state file.
// Kept as a constant so the CLI reset subcommand and the loop path agree.
const WarnOnlyBudgetFilename = "warn-only-budget.json"

// WarnOnlyBudgetState is the persisted shape of the warn-only ratchet budget.
// It is written to .agents/overnight/warn-only-budget.json (one file per
// repository, NOT per run — the budget is a repo-wide rescue counter, not a
// run-local one, so operators cannot silently reset it by starting a new
// run).
//
// The schema is intentionally flat and versioned so future migrations can
// reuse ReadBudget's rescue matrix without breaking callers.
type WarnOnlyBudgetState struct {
	// Version identifies the on-disk schema. v1 is the initial release.
	Version int `json:"version"`

	// Remaining is the number of warn-only rescues still available. When
	// Remaining hits zero the loop must behave as if WarnOnly=false for
	// the remainder of the run.
	Remaining int `json:"remaining"`

	// InitialBudget captures the ceiling at the time Reset was last
	// invoked. It is informational (morning report rendering) and not
	// used for decision logic.
	InitialBudget int `json:"initial_budget"`

	// LastResetAt is the RFC3339 timestamp of the most recent reset.
	// Empty string means "never reset on disk" and callers should treat
	// the state as freshly initialised.
	LastResetAt string `json:"last_reset_at,omitempty"`

	// LastDecrementAt is the RFC3339 timestamp of the most recent
	// rescue consumption. Empty string means "no rescue has been
	// consumed since the last reset".
	LastDecrementAt string `json:"last_decrement_at,omitempty"`
}

// WarnOnlyBudgetPath returns the canonical budget file path for a repo.
// repoRoot should be the directory containing .agents/ — callers typically
// pass the current working directory.
func WarnOnlyBudgetPath(repoRoot string) string {
	return filepath.Join(paths.ResolveFromRoot(repoRoot).AgentsDir, "overnight", WarnOnlyBudgetFilename)
}

// defaultBudgetState returns a freshly-initialised budget with
// InitialBudget = DefaultWarnOnlyBudget. It is the value the rescue matrix
// returns for the "missing" and "corrupt" failure modes.
func defaultBudgetState() WarnOnlyBudgetState {
	return WarnOnlyBudgetState{
		Version:       1,
		Remaining:     DefaultWarnOnlyBudget,
		InitialBudget: DefaultWarnOnlyBudget,
	}
}

// ReadBudget loads the warn-only budget state from disk at the canonical
// path under repoRoot. It implements the "rescue matrix" called out in the
// C3 design: missing, corrupt, and out-of-range states all degrade to a
// fresh default rather than failing the caller, because a broken budget
// file must never wedge the Dream loop.
//
// The second return value is a non-nil rescue reason whenever the on-disk
// state was unusable and a default was substituted. Callers should surface
// it as a degraded note so operators can tell the difference between
// "fresh start" and "budget file silently replaced".
//
// ReadBudget never returns a hard error. An I/O failure other than
// os.ErrNotExist (for example, a permission error on the .agents directory)
// is still surfaced via the rescue reason so the loop can keep running.
func ReadBudget(repoRoot string) (WarnOnlyBudgetState, string) {
	path := WarnOnlyBudgetPath(repoRoot)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Missing: first-run case, no rescue reason needed.
			return defaultBudgetState(), ""
		}
		return defaultBudgetState(), fmt.Sprintf("warn-only budget unreadable, using default: %v", err)
	}
	var state WarnOnlyBudgetState
	if err := json.Unmarshal(raw, &state); err != nil {
		return defaultBudgetState(), fmt.Sprintf("warn-only budget corrupt, using default: %v", err)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if state.InitialBudget <= 0 {
		state.InitialBudget = DefaultWarnOnlyBudget
	}
	// Out-of-range guard: negative Remaining or Remaining above the ceiling
	// both mean the file was tampered with or rolled back to an older
	// schema. Clamp into [0, InitialBudget] and record a rescue reason.
	if state.Remaining < 0 {
		state.Remaining = 0
		return state, "warn-only budget out-of-range (negative), clamped to 0"
	}
	if state.Remaining > state.InitialBudget {
		remaining := state.Remaining
		state.Remaining = state.InitialBudget
		return state, fmt.Sprintf("warn-only budget out-of-range (%d > initial %d), clamped", remaining, state.InitialBudget)
	}
	return state, ""
}

// WriteBudget persists the provided state to the canonical path under
// repoRoot. It uses the standard atomic write pattern
// (CreateTemp → write → Sync → Rename) so a crash mid-write cannot corrupt
// the file. The parent directory is created if missing.
//
// Returning an error is deliberate: the caller (loop or reset subcommand)
// should decide whether to soft-fail with a degraded note or hard-fail the
// operation. ReadBudget's rescue matrix is what prevents downstream wedges;
// WriteBudget's job is only to be correct.
func WriteBudget(repoRoot string, state WarnOnlyBudgetState) error {
	if state.Version == 0 {
		state.Version = 1
	}
	path := WarnOnlyBudgetPath(repoRoot)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("warn-only budget: mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".warn-only-budget.*.tmp")
	if err != nil {
		return fmt.Errorf("warn-only budget: create temp: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we leave before the rename.
	defer func() { _ = os.Remove(tmpName) }()
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(state); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("warn-only budget: encode: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("warn-only budget: sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("warn-only budget: close: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("warn-only budget: rename: %w", err)
	}
	return nil
}

// ResetBudget writes a fresh state with Remaining = initial. Passing
// initial <= 0 falls back to DefaultWarnOnlyBudget so CLI callers can pass
// a zero-value flag without surprise.
func ResetBudget(repoRoot string, initial int) (WarnOnlyBudgetState, error) {
	if initial <= 0 {
		initial = DefaultWarnOnlyBudget
	}
	state := WarnOnlyBudgetState{
		Version:       1,
		Remaining:     initial,
		InitialBudget: initial,
		LastResetAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := WriteBudget(repoRoot, state); err != nil {
		return state, err
	}
	return state, nil
}

// DecrementBudget consumes one rescue from the on-disk budget and returns
// the updated state. If the budget was already at zero it returns the
// unchanged state with exhausted=true so the caller can switch the current
// iteration to strict-halt behaviour.
//
// The timestamp field is updated on every successful decrement so morning
// reports can show "last rescue consumed" without guessing.
func DecrementBudget(repoRoot string) (state WarnOnlyBudgetState, exhausted bool, err error) {
	state, _ = ReadBudget(repoRoot)
	if state.Remaining <= 0 {
		return state, true, nil
	}
	state.Remaining--
	state.LastDecrementAt = time.Now().UTC().Format(time.RFC3339)
	if err := WriteBudget(repoRoot, state); err != nil {
		return state, false, err
	}
	return state, state.Remaining == 0, nil
}
