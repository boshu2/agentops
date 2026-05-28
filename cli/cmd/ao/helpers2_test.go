// practices: [pragmatic-programmer, design-patterns]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/types"
)

// ---------------------------------------------------------------------------
// rpi_loop_supervisor.go: validateLoopNumericConstraints
// ---------------------------------------------------------------------------

func TestHelper2_validateLoopNumericConstraints(t *testing.T) {
	tests := []struct {
		name    string
		cfg     rpiLoopSupervisorConfig
		wantErr string
	}{
		{
			name:    "all valid",
			cfg:     rpiLoopSupervisorConfig{CycleRetries: 3, RetryBackoff: time.Second, CycleDelay: time.Second, CommandTimeout: time.Minute},
			wantErr: "",
		},
		{
			name:    "all zero",
			cfg:     rpiLoopSupervisorConfig{},
			wantErr: "",
		},
		{
			name:    "negative retries",
			cfg:     rpiLoopSupervisorConfig{CycleRetries: -1},
			wantErr: "cycle-retries",
		},
		{
			name:    "negative backoff",
			cfg:     rpiLoopSupervisorConfig{RetryBackoff: -1 * time.Second},
			wantErr: "retry-backoff",
		},
		{
			name:    "negative delay",
			cfg:     rpiLoopSupervisorConfig{CycleDelay: -1 * time.Second},
			wantErr: "cycle-delay",
		},
		{
			name:    "negative timeout",
			cfg:     rpiLoopSupervisorConfig{CommandTimeout: -1 * time.Second},
			wantErr: "command-timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLoopNumericConstraints(&tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rpi_loop_supervisor.go: applyLoopTimingDefaults
// ---------------------------------------------------------------------------

func TestHelper2_applyLoopTimingDefaults(t *testing.T) {
	// applyLoopTimingDefaults short-circuits AutoCleanStaleAfter to 0 when the
	// rpiSupervisor global is true (the supervisor lane disables auto-clean by
	// default). The helper tests below pass cmd=nil and assume the bare-defaults
	// branch, so the global must be restored to false. Without this snapshot,
	// prior tests (e.g. TestResolveLoopSupervisorConfig_AppliesSupervisorDefaults)
	// that set rpiSupervisor=true leak into this test. soc-dorz CI surfaced the
	// dependency once go-build started running on every workflow-touching PR.
	prev := snapshotLoopSupervisorGlobals()
	t.Cleanup(func() { restoreLoopSupervisorGlobals(prev) })
	rpiSupervisor = false

	t.Run("zero values get defaults", func(t *testing.T) {
		cfg := rpiLoopSupervisorConfig{}
		applyLoopTimingDefaults(&cfg, nil)

		if cfg.LeaseTTL != 2*time.Minute {
			t.Fatalf("LeaseTTL: got %v, want 2m", cfg.LeaseTTL)
		}
		if cfg.CommandTimeout != defaultLoopCommandTimeout {
			t.Fatalf("CommandTimeout: got %v, want %v", cfg.CommandTimeout, defaultLoopCommandTimeout)
		}
		if cfg.AutoCleanStaleAfter != 24*time.Hour {
			t.Fatalf("AutoCleanStaleAfter: got %v, want 24h", cfg.AutoCleanStaleAfter)
		}
	})

	t.Run("positive values preserved", func(t *testing.T) {
		cfg := rpiLoopSupervisorConfig{
			LeaseTTL:            5 * time.Minute,
			CommandTimeout:      10 * time.Minute,
			AutoCleanStaleAfter: 48 * time.Hour,
		}
		applyLoopTimingDefaults(&cfg, nil)

		if cfg.LeaseTTL != 5*time.Minute {
			t.Fatalf("LeaseTTL should be preserved, got %v", cfg.LeaseTTL)
		}
		if cfg.CommandTimeout != 10*time.Minute {
			t.Fatalf("CommandTimeout should be preserved, got %v", cfg.CommandTimeout)
		}
		if cfg.AutoCleanStaleAfter != 48*time.Hour {
			t.Fatalf("AutoCleanStaleAfter should be preserved, got %v", cfg.AutoCleanStaleAfter)
		}
	})

	t.Run("negative lease TTL gets default", func(t *testing.T) {
		cfg := rpiLoopSupervisorConfig{LeaseTTL: -1 * time.Minute}
		applyLoopTimingDefaults(&cfg, nil)
		if cfg.LeaseTTL != 2*time.Minute {
			t.Fatalf("negative LeaseTTL should be overridden to 2m, got %v", cfg.LeaseTTL)
		}
	})
}

// ---------------------------------------------------------------------------
// rpi_loop_supervisor.go: applyLoopPathDefaults
// ---------------------------------------------------------------------------

func TestHelper2_applyLoopPathDefaults(t *testing.T) {
	t.Run("empty paths get defaults", func(t *testing.T) {
		cfg := rpiLoopSupervisorConfig{}
		applyLoopPathDefaults(&cfg)

		if cfg.LeasePath != filepath.Join(".agents", "rpi", "supervisor.lock") {
			t.Fatalf("LeasePath: got %q", cfg.LeasePath)
		}
		if cfg.LandingLockPath != filepath.Join(".agents", "rpi", "landing.lock") {
			t.Fatalf("LandingLockPath: got %q", cfg.LandingLockPath)
		}
		if cfg.KillSwitchPath != filepath.Join(".agents", "rpi", "KILL") {
			t.Fatalf("KillSwitchPath: got %q", cfg.KillSwitchPath)
		}
	})

	t.Run("non-empty paths preserved", func(t *testing.T) {
		cfg := rpiLoopSupervisorConfig{
			LeasePath:       "/custom/lease",
			LandingLockPath: "/custom/landing",
			KillSwitchPath:  "/custom/kill",
		}
		applyLoopPathDefaults(&cfg)

		if cfg.LeasePath != "/custom/lease" {
			t.Fatalf("LeasePath should be preserved, got %q", cfg.LeasePath)
		}
		if cfg.LandingLockPath != "/custom/landing" {
			t.Fatalf("LandingLockPath should be preserved, got %q", cfg.LandingLockPath)
		}
		if cfg.KillSwitchPath != "/custom/kill" {
			t.Fatalf("KillSwitchPath should be preserved, got %q", cfg.KillSwitchPath)
		}
	})
}

// ---------------------------------------------------------------------------
// rpi_loop_supervisor.go: applySupervisorBoolDefaults
// ---------------------------------------------------------------------------

func TestHelper2_applySupervisorBoolDefaults(t *testing.T) {
	// We need a minimal cobra.Command with the flags registered so Changed() returns false.
	cmd := newTestRPICommand()

	cfg := rpiLoopSupervisorConfig{
		LeaseEnabled:         false,
		DetachedHeal:         true,
		AutoClean:            false,
		EnsureCleanup:        false,
		CleanupPruneBranches: false,
	}

	applySupervisorBoolDefaults(cmd, &cfg)

	if !cfg.LeaseEnabled {
		t.Fatal("LeaseEnabled should default to true in supervisor mode")
	}
	if cfg.DetachedHeal {
		t.Fatal("DetachedHeal should default to false in supervisor mode")
	}
	if !cfg.AutoClean {
		t.Fatal("AutoClean should default to true in supervisor mode")
	}
	if !cfg.EnsureCleanup {
		t.Fatal("EnsureCleanup should default to true in supervisor mode")
	}
	if !cfg.CleanupPruneBranches {
		t.Fatal("CleanupPruneBranches should default to true in supervisor mode")
	}
}

// ---------------------------------------------------------------------------
// rpi_loop_supervisor.go: applySupervisorPolicyDefaults
// ---------------------------------------------------------------------------

func TestHelper2_applySupervisorPolicyDefaults(t *testing.T) {
	cmd := newTestRPICommand()

	cfg := rpiLoopSupervisorConfig{
		FailurePolicy: "stop",
		CycleRetries:  0,
		CycleDelay:    0,
		GatePolicy:    "off",
	}

	applySupervisorPolicyDefaults(cmd, &cfg)

	if cfg.FailurePolicy != loopFailurePolicyContinue {
		t.Fatalf("FailurePolicy: got %q, want %q", cfg.FailurePolicy, loopFailurePolicyContinue)
	}
	if cfg.CycleRetries != 1 {
		t.Fatalf("CycleRetries: got %d, want 1", cfg.CycleRetries)
	}
	if cfg.CycleDelay != 5*time.Minute {
		t.Fatalf("CycleDelay: got %v, want 5m", cfg.CycleDelay)
	}
	if cfg.GatePolicy != loopGatePolicyRequired {
		t.Fatalf("GatePolicy: got %q, want %q", cfg.GatePolicy, loopGatePolicyRequired)
	}
}

// ---------------------------------------------------------------------------
// rpi_loop_supervisor.go: validateLoopConfigPolicies
// ---------------------------------------------------------------------------

func TestHelper2_validateLoopConfigPolicies(t *testing.T) {
	tests := []struct {
		name    string
		cfg     rpiLoopSupervisorConfig
		wantErr string
	}{
		{
			name: "all valid",
			cfg: rpiLoopSupervisorConfig{
				FailurePolicy: "stop",
				GatePolicy:    "off",
				LandingPolicy: "off",
				BDSyncPolicy:  "auto",
			},
		},
		{
			name: "all valid combo 2",
			cfg: rpiLoopSupervisorConfig{
				FailurePolicy: "continue",
				GatePolicy:    "best-effort",
				LandingPolicy: "commit",
				BDSyncPolicy:  "always",
			},
		},
		{
			name: "all valid combo 3",
			cfg: rpiLoopSupervisorConfig{
				FailurePolicy: "continue",
				GatePolicy:    "required",
				LandingPolicy: "sync-push",
				BDSyncPolicy:  "never",
			},
		},
		{
			name: "invalid failure policy",
			cfg: rpiLoopSupervisorConfig{
				FailurePolicy: "abort",
				GatePolicy:    "off",
				LandingPolicy: "off",
				BDSyncPolicy:  "auto",
			},
			wantErr: "invalid failure-policy",
		},
		{
			name: "invalid gate policy",
			cfg: rpiLoopSupervisorConfig{
				FailurePolicy: "stop",
				GatePolicy:    "strict",
				LandingPolicy: "off",
				BDSyncPolicy:  "auto",
			},
			wantErr: "invalid gate-policy",
		},
		{
			name: "invalid landing policy",
			cfg: rpiLoopSupervisorConfig{
				FailurePolicy: "stop",
				GatePolicy:    "off",
				LandingPolicy: "merge",
				BDSyncPolicy:  "auto",
			},
			wantErr: "invalid landing-policy",
		},
		{
			name: "invalid bd-sync policy",
			cfg: rpiLoopSupervisorConfig{
				FailurePolicy: "stop",
				GatePolicy:    "off",
				LandingPolicy: "off",
				BDSyncPolicy:  "sometimes",
			},
			wantErr: "invalid bd-sync-policy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLoopConfigPolicies(tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rpi_loop_supervisor.go: renderLandingCommitMessage
// ---------------------------------------------------------------------------

func TestHelper2_renderLandingCommitMessage(t *testing.T) {
	tests := []struct {
		name     string
		template string
		cycle    int
		attempt  int
		goal     string
		want     string
	}{
		{
			name:     "default template",
			template: "",
			cycle:    3,
			attempt:  1,
			goal:     "fix-bug",
			want:     "chore(rpi): autonomous cycle 3",
		},
		{
			name:     "custom with all placeholders",
			template: "cycle={{cycle}} attempt={{attempt}} goal={{goal}}",
			cycle:    5,
			attempt:  2,
			goal:     "add-feature",
			want:     "cycle=5 attempt=2 goal=add-feature",
		},
		{
			name:     "whitespace-only template uses default",
			template: "   ",
			cycle:    1,
			attempt:  1,
			goal:     "",
			want:     "chore(rpi): autonomous cycle 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderLandingCommitMessage(tt.template, tt.cycle, tt.attempt, tt.goal)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rpi_loop_supervisor.go: isNoRebaseInProgressMessage
// ---------------------------------------------------------------------------

func TestHelper2_isNoRebaseInProgressMessage(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"", false},
		{"some other error", false},
		{"fatal: No rebase in progress?", true},
		{"  No rebase in progress  ", true},
		{"error: no rebase to abort", true},
		{"No Rebase In Progress", true},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := isNoRebaseInProgressMessage(tt.msg)
			if got != tt.want {
				t.Fatalf("isNoRebaseInProgressMessage(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rpi_loop_supervisor.go: normalizeLoopCommandTimeout
// ---------------------------------------------------------------------------

func TestHelper2_normalizeLoopCommandTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		want    time.Duration
	}{
		{"zero", 0, defaultLoopCommandTimeout},
		{"negative", -5 * time.Second, defaultLoopCommandTimeout},
		{"positive", 10 * time.Minute, 10 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeLoopCommandTimeout(tt.timeout)
			if got != tt.want {
				t.Fatalf("normalizeLoopCommandTimeout(%v) = %v, want %v", tt.timeout, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rpi_loop_supervisor.go: MaxCycleAttempts, ShouldContinueAfterFailure
// ---------------------------------------------------------------------------

func TestHelper2_MaxCycleAttempts(t *testing.T) {
	cfg := rpiLoopSupervisorConfig{CycleRetries: 3}
	if got := cfg.MaxCycleAttempts(); got != 4 {
		t.Fatalf("MaxCycleAttempts: got %d, want 4", got)
	}
	cfg.CycleRetries = 0
	if got := cfg.MaxCycleAttempts(); got != 1 {
		t.Fatalf("MaxCycleAttempts: got %d, want 1", got)
	}
}

func TestHelper2_ShouldContinueAfterFailure(t *testing.T) {
	tests := []struct {
		policy string
		want   bool
	}{
		{"continue", true},
		{"stop", false},
	}
	for _, tt := range tests {
		cfg := rpiLoopSupervisorConfig{FailurePolicy: tt.policy}
		if got := cfg.ShouldContinueAfterFailure(); got != tt.want {
			t.Fatalf("ShouldContinueAfterFailure(%q) = %v, want %v", tt.policy, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// rpi_cancel.go: rpi.ParseCancelSignal
// ---------------------------------------------------------------------------

func TestHelper2_parseCancelSignal(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"TERM", false},
		{"term", false},
		{"SIGTERM", false},
		{"KILL", false},
		{"SIGKILL", false},
		{"INT", false},
		{"SIGINT", false},
		{"", false},
		{"HUP", true},
		{"STOP", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := parseCancelSignal(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseCancelSignal(%q): err=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rpi_cancel.go: filterKillablePIDs
// ---------------------------------------------------------------------------

func TestHelper2_filterKillablePIDs(t *testing.T) {
	tests := []struct {
		name    string
		pids    []int
		selfPID int
		want    []int
	}{
		{
			name:    "normal",
			pids:    []int{100, 200, 300},
			selfPID: 999,
			want:    []int{100, 200, 300},
		},
		{
			name:    "excludes self",
			pids:    []int{100, 999, 200},
			selfPID: 999,
			want:    []int{100, 200},
		},
		{
			name:    "excludes pid 0 and 1",
			pids:    []int{0, 1, 100},
			selfPID: 999,
			want:    []int{100},
		},
		{
			name:    "deduplicates",
			pids:    []int{100, 100, 200},
			selfPID: 999,
			want:    []int{100, 200},
		},
		{
			name:    "empty",
			pids:    nil,
			selfPID: 999,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterKillablePIDs(tt.pids, tt.selfPID)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rpi_cancel.go: dedupeInts
// ---------------------------------------------------------------------------

func TestHelper2_dedupeInts(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		want []int
	}{
		{"empty", nil, nil},
		{"no dupes", []int{3, 1, 2}, []int{1, 2, 3}},
		{"with dupes", []int{5, 3, 5, 1, 3}, []int{1, 3, 5}},
		{"single", []int{42}, []int{42}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedupeInts(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rpi_cancel.go: descendantPIDs
// ---------------------------------------------------------------------------

func TestHelper2_descendantPIDs(t *testing.T) {
	procs := []processInfo{
		{PID: 1, PPID: 0, Command: "init"},
		{PID: 10, PPID: 1, Command: "parent"},
		{PID: 20, PPID: 10, Command: "child1"},
		{PID: 30, PPID: 10, Command: "child2"},
		{PID: 40, PPID: 20, Command: "grandchild"},
		{PID: 50, PPID: 1, Command: "other"},
	}

	got := descendantPIDs(10, procs)
	// Should include 20, 30, 40 but not 10 (self) or 50 (sibling)
	want := []int{20, 30, 40}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	// No children case
	got = descendantPIDs(50, procs)
	if len(got) != 0 {
		t.Fatalf("expected no descendants for PID 50, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// rpi_cancel.go: processExists
// ---------------------------------------------------------------------------

func TestHelper2_processExists(t *testing.T) {
	procs := []processInfo{
		{PID: 10, PPID: 1, Command: "p1"},
		{PID: 20, PPID: 1, Command: "p2"},
	}

	if !processExists(10, procs) {
		t.Fatal("expected PID 10 to exist")
	}
	if processExists(99, procs) {
		t.Fatal("expected PID 99 to not exist")
	}
	if processExists(10, nil) {
		t.Fatal("expected PID 10 to not exist in nil list")
	}
}

// ---------------------------------------------------------------------------
// rpi_cancel.go: supervisorLeaseMetadataExpired
// ---------------------------------------------------------------------------

func TestHelper2_supervisorLeaseMetadataExpired(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		expiresAt string
		want      bool
	}{
		{
			name:      "empty means not expired (backward compat)",
			expiresAt: "",
			want:      false,
		},
		{
			name:      "future not expired",
			expiresAt: now.Add(10 * time.Minute).Format(time.RFC3339),
			want:      false,
		},
		{
			name:      "past is expired",
			expiresAt: now.Add(-10 * time.Minute).Format(time.RFC3339),
			want:      true,
		},
		{
			name:      "corrupted is stale",
			expiresAt: "not-a-date",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := supervisorLeaseMetadata{ExpiresAt: tt.expiresAt}
			got := supervisorLeaseMetadataExpired(meta, now)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// flywheel_close_loop.go: filterAntiPatternTransitions
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// flywheel_close_loop.go: loadExistingIndexEntries
// ---------------------------------------------------------------------------

func TestHelper2_loadExistingIndexEntries(t *testing.T) {
	t.Run("file does not exist", func(t *testing.T) {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "nonexistent.jsonl")
		entries := loadExistingIndexEntries(p)
		if len(entries) != 0 {
			t.Fatalf("expected empty map for missing file, got %d entries", len(entries))
		}
	})

	t.Run("valid JSONL entries", func(t *testing.T) {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "index.jsonl")

		entry1 := IndexEntry{Path: "/a/b.md", ID: "id1", Type: "learning"}
		entry2 := IndexEntry{Path: "/c/d.md", ID: "id2", Type: "pattern"}
		data1, _ := json.Marshal(entry1)
		data2, _ := json.Marshal(entry2)
		writeFile(t, p, string(data1)+"\n"+string(data2)+"\n")

		entries := loadExistingIndexEntries(p)
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		if entries["/a/b.md"].ID != "id1" {
			t.Fatalf("expected id1, got %q", entries["/a/b.md"].ID)
		}
		if entries["/c/d.md"].ID != "id2" {
			t.Fatalf("expected id2, got %q", entries["/c/d.md"].ID)
		}
	})

	t.Run("skips malformed lines", func(t *testing.T) {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "index.jsonl")
		entry := IndexEntry{Path: "/ok.md", ID: "ok", Type: "learning"}
		data, _ := json.Marshal(entry)
		writeFile(t, p, string(data)+"\nnot json\n{\"bad\": true}\n")

		entries := loadExistingIndexEntries(p)
		// Only the first line has a valid Path; the third line has no Path field
		if len(entries) != 1 {
			t.Fatalf("expected 1 valid entry, got %d", len(entries))
		}
	})
}

// ---------------------------------------------------------------------------
// flywheel_close_loop.go: upsertIndexPaths
// ---------------------------------------------------------------------------

func TestHelper2_upsertIndexPaths(t *testing.T) {
	t.Run("skips empty and nonexistent paths", func(t *testing.T) {
		existing := make(map[string]IndexEntry)
		indexed := upsertIndexPaths(existing, []string{"", "/nonexistent/path.md"}, false)
		if indexed != 0 {
			t.Fatalf("expected 0 indexed, got %d", indexed)
		}
	})

	t.Run("indexes real files", func(t *testing.T) {
		tmp := t.TempDir()
		f1 := filepath.Join(tmp, "learning1.md")
		f2 := filepath.Join(tmp, "learning2.md")
		writeFile(t, f1, "---\nid: L001\n---\nSome learning content\n")
		writeFile(t, f2, "---\nid: L002\n---\nAnother learning\n")

		existing := make(map[string]IndexEntry)
		indexed := upsertIndexPaths(existing, []string{f1, f2}, false)
		if indexed != 2 {
			t.Fatalf("expected 2 indexed, got %d", indexed)
		}
		if _, ok := existing[f1]; !ok {
			t.Fatalf("expected entry for %s", f1)
		}
		if _, ok := existing[f2]; !ok {
			t.Fatalf("expected entry for %s", f2)
		}
	})

	t.Run("upserts overwrite existing", func(t *testing.T) {
		tmp := t.TempDir()
		f1 := filepath.Join(tmp, "learning.md")
		writeFile(t, f1, "---\nid: L001\n---\nOriginal content\n")

		existing := map[string]IndexEntry{
			f1: {Path: f1, ID: "old-id"},
		}
		indexed := upsertIndexPaths(existing, []string{f1}, false)
		if indexed != 1 {
			t.Fatalf("expected 1 indexed, got %d", indexed)
		}
		// The entry should have been replaced
		if existing[f1].Path != f1 {
			t.Fatalf("expected entry path to be %s", f1)
		}
	})
}

// ---------------------------------------------------------------------------
// maturity.go: parseFrontmatterFields
// ---------------------------------------------------------------------------

func TestHelper2_parseFrontmatterFields(t *testing.T) {
	t.Run("extracts requested fields", func(t *testing.T) {
		tmp := t.TempDir()
		f := filepath.Join(tmp, "test.md")
		writeFile(t, f, "---\nvalid_until: 2026-06-30\nexpiry_status: active\nmaturity: provisional\n---\nBody content\n")

		fields, err := parseFrontmatterFields(f, "valid_until", "expiry_status")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fields["valid_until"] != "2026-06-30" {
			t.Fatalf("valid_until: got %q", fields["valid_until"])
		}
		if fields["expiry_status"] != "active" {
			t.Fatalf("expiry_status: got %q", fields["expiry_status"])
		}
	})

	t.Run("handles quoted values", func(t *testing.T) {
		tmp := t.TempDir()
		f := filepath.Join(tmp, "test.md")
		writeFile(t, f, "---\nvalid_until: \"2026-06-30\"\n---\n")

		fields, err := parseFrontmatterFields(f, "valid_until")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fields["valid_until"] != "2026-06-30" {
			t.Fatalf("expected quotes stripped, got %q", fields["valid_until"])
		}
	})

	t.Run("no frontmatter", func(t *testing.T) {
		tmp := t.TempDir()
		f := filepath.Join(tmp, "test.md")
		writeFile(t, f, "Just regular content\n")

		fields, err := parseFrontmatterFields(f, "valid_until")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(fields) != 0 {
			t.Fatalf("expected empty fields, got %v", fields)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := parseFrontmatterFields("/nonexistent.md", "valid_until")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

// ---------------------------------------------------------------------------
// maturity.go: isEvictionEligible
// ---------------------------------------------------------------------------

func TestHelper2_isEvictionEligible(t *testing.T) {
	tests := []struct {
		name       string
		utility    float64
		confidence float64
		maturity   string
		want       bool
	}{
		{"established never eligible", 0.1, 0.1, "established", false},
		{"utility too high", 0.5, 0.1, "provisional", false},
		{"confidence too high", 0.1, 0.5, "provisional", false},
		{"eligible provisional", 0.1, 0.1, "provisional", true},
		{"eligible candidate", 0.2, 0.1, "candidate", true},
		{"boundary utility 0.3", 0.3, 0.1, "provisional", false},
		{"boundary confidence 0.3", 0.1, 0.3, "provisional", false},
		{"just under thresholds", 0.29, 0.29, "candidate", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEvictionEligible(tt.utility, tt.confidence, tt.maturity)
			if got != tt.want {
				t.Fatalf("isEvictionEligible(%.2f, %.2f, %q) = %v, want %v",
					tt.utility, tt.confidence, tt.maturity, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// maturity.go: evictionCitationStatus
// ---------------------------------------------------------------------------

func TestHelper2_evictionCitationStatus(t *testing.T) {
	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		file      string
		lastCited map[string]time.Time
		wantStr   string
		wantOK    bool
	}{
		{
			name:      "never cited",
			file:      "/a.md",
			lastCited: map[string]time.Time{},
			wantStr:   "never",
			wantOK:    true,
		},
		{
			name:      "cited before cutoff",
			file:      "/a.md",
			lastCited: map[string]time.Time{"/a.md": cutoff.Add(-24 * time.Hour)},
			wantStr:   "2025-12-31",
			wantOK:    true,
		},
		{
			name:      "cited after cutoff blocks eviction",
			file:      "/a.md",
			lastCited: map[string]time.Time{"/a.md": cutoff.Add(24 * time.Hour)},
			wantStr:   "",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			str, ok := evictionCitationStatus(tt.file, tt.lastCited, cutoff)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if str != tt.wantStr {
				t.Fatalf("str = %q, want %q", str, tt.wantStr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// maturity.go: floatValueFromData, nonEmptyStringFromData
// ---------------------------------------------------------------------------

func TestHelper2_floatValueFromData(t *testing.T) {
	data := map[string]any{
		"utility":    0.75,
		"not_float":  "hello",
		"confidence": 0.0,
	}

	if got := floatValueFromData(data, "utility", 0.5); got != 0.75 {
		t.Fatalf("expected 0.75, got %v", got)
	}
	if got := floatValueFromData(data, "not_float", 0.5); got != 0.5 {
		t.Fatalf("expected default 0.5 for non-float, got %v", got)
	}
	if got := floatValueFromData(data, "missing", 0.3); got != 0.3 {
		t.Fatalf("expected default 0.3 for missing key, got %v", got)
	}
	if got := floatValueFromData(data, "confidence", 0.5); got != 0.0 {
		t.Fatalf("expected 0.0, got %v", got)
	}
}

func TestHelper2_nonEmptyStringFromData(t *testing.T) {
	data := map[string]any{
		"maturity": "provisional",
		"empty":    "",
		"number":   42,
	}

	if got := nonEmptyStringFromData(data, "maturity", "default"); got != "provisional" {
		t.Fatalf("expected provisional, got %q", got)
	}
	if got := nonEmptyStringFromData(data, "empty", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback for empty string, got %q", got)
	}
	if got := nonEmptyStringFromData(data, "number", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback for non-string, got %q", got)
	}
	if got := nonEmptyStringFromData(data, "missing", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback for missing key, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// pool.go / flywheel_close_loop.go: isEligibleTier (promotionContext)
// ---------------------------------------------------------------------------

func TestHelper2_isEligibleTier(t *testing.T) {
	tests := []struct {
		name        string
		tier        types.Tier
		includeGold bool
		want        bool
	}{
		{"silver always eligible", types.TierSilver, false, true},
		{"silver with gold enabled", types.TierSilver, true, true},
		{"gold eligible when included", types.TierGold, true, true},
		{"gold not eligible when excluded", types.TierGold, false, false},
		{"bronze never eligible", types.TierBronze, true, false},
		{"discard never eligible", types.TierDiscard, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &promotionContext{includeGold: tt.includeGold}
			got := ctx.isEligibleTier(tt.tier)
			if got != tt.want {
				t.Fatalf("isEligibleTier(%q, includeGold=%v) = %v, want %v",
					tt.tier, tt.includeGold, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// pool.go: truncateID
// ---------------------------------------------------------------------------

func TestHelper2_truncateID(t *testing.T) {
	tests := []struct {
		id   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"exactlength", 11, "exactlength"},
		{"this-is-a-very-long-id", 10, "this-is..."},
		{"abc", 3, "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := truncateID(tt.id, tt.max)
			if got != tt.want {
				t.Fatalf("truncateID(%q, %d) = %q, want %q", tt.id, tt.max, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// pool.go: repeat
// ---------------------------------------------------------------------------

func TestHelper2_repeat(t *testing.T) {
	if got := repeat("=", 3); got != "===" {
		t.Fatalf("repeat('=', 3) = %q", got)
	}
	if got := repeat("ab", 2); got != "abab" {
		t.Fatalf("repeat('ab', 2) = %q", got)
	}
	if got := repeat("x", 0); got != "" {
		t.Fatalf("repeat('x', 0) = %q", got)
	}
}

// ---------------------------------------------------------------------------
// rpi_loop_supervisor.go: wrapCycleFailure, isInfrastructureCycleFailure, shouldMarkQueueEntryFailed
// ---------------------------------------------------------------------------

func TestHelper2_wrapCycleFailure(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		if got := wrapCycleFailure(cycleFailureTask, "stage", nil); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("wraps task failure", func(t *testing.T) {
		err := wrapCycleFailure(cycleFailureTask, "test", os.ErrNotExist)
		if err == nil {
			t.Fatal("expected error")
		}
		if isInfrastructureCycleFailure(err) {
			t.Fatal("expected task failure, not infrastructure")
		}
		if !shouldMarkQueueEntryFailed(err) {
			t.Fatal("task failures should mark queue entry failed")
		}
	})

	t.Run("wraps infrastructure failure", func(t *testing.T) {
		err := wrapCycleFailure(cycleFailureInfrastructure, "infra", os.ErrPermission)
		if err == nil {
			t.Fatal("expected error")
		}
		if !isInfrastructureCycleFailure(err) {
			t.Fatal("expected infrastructure failure")
		}
		if shouldMarkQueueEntryFailed(err) {
			t.Fatal("infrastructure failures should NOT mark queue entry failed")
		}
	})

	t.Run("does not double-wrap", func(t *testing.T) {
		inner := wrapCycleFailure(cycleFailureTask, "inner", os.ErrNotExist)
		outer := wrapCycleFailure(cycleFailureInfrastructure, "outer", inner)
		// Should keep the original wrapping, not re-wrap
		if isInfrastructureCycleFailure(outer) {
			t.Fatal("should keep original task type, not re-wrap as infrastructure")
		}
	})

	t.Run("empty stage", func(t *testing.T) {
		err := wrapCycleFailure(cycleFailureTask, "", os.ErrNotExist)
		if err == nil {
			t.Fatal("expected error")
		}
		// With empty stage, the message should be the underlying error
		if !strings.Contains(err.Error(), "not exist") {
			t.Fatalf("expected underlying error message, got %q", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// rpi_loop_supervisor.go: appendDirtyPaths
// ---------------------------------------------------------------------------

func TestHelper2_appendDirtyPaths(t *testing.T) {
	dest := make(map[string]struct{})
	appendDirtyPaths(dest, "file1.go\nfile2.go\n\n  file3.go  \n")
	if len(dest) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(dest))
	}
	for _, expected := range []string{"file1.go", "file2.go", "file3.go"} {
		if _, ok := dest[expected]; !ok {
			t.Fatalf("expected %q in paths", expected)
		}
	}

	// Empty string
	dest2 := make(map[string]struct{})
	appendDirtyPaths(dest2, "")
	if len(dest2) != 0 {
		t.Fatalf("expected 0 paths for empty string, got %d", len(dest2))
	}
}

// ---------------------------------------------------------------------------
// rpi_loop_supervisor.go: isLoopKillSwitchSet
// ---------------------------------------------------------------------------

func TestHelper2_isLoopKillSwitchSet(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		cfg := rpiLoopSupervisorConfig{KillSwitchPath: ""}
		set, err := isLoopKillSwitchSet(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if set {
			t.Fatal("expected not set for empty path")
		}
	})

	t.Run("whitespace path", func(t *testing.T) {
		cfg := rpiLoopSupervisorConfig{KillSwitchPath: "   "}
		set, err := isLoopKillSwitchSet(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if set {
			t.Fatal("expected not set for whitespace path")
		}
	})

	t.Run("file does not exist", func(t *testing.T) {
		tmp := t.TempDir()
		cfg := rpiLoopSupervisorConfig{KillSwitchPath: filepath.Join(tmp, "KILL")}
		set, err := isLoopKillSwitchSet(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if set {
			t.Fatal("expected not set for nonexistent file")
		}
	})

	t.Run("file exists", func(t *testing.T) {
		tmp := t.TempDir()
		killPath := filepath.Join(tmp, "KILL")
		writeFile(t, killPath, "kill")
		cfg := rpiLoopSupervisorConfig{KillSwitchPath: killPath}
		set, err := isLoopKillSwitchSet(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !set {
			t.Fatal("expected set when file exists")
		}
	})

	t.Run("path is a directory", func(t *testing.T) {
		tmp := t.TempDir()
		dirPath := filepath.Join(tmp, "KILL")
		if err := os.Mkdir(dirPath, 0755); err != nil {
			t.Fatal(err)
		}
		cfg := rpiLoopSupervisorConfig{KillSwitchPath: dirPath}
		_, err := isLoopKillSwitchSet(cfg)
		if err == nil {
			t.Fatal("expected error for directory path")
		}
		if !strings.Contains(err.Error(), "directory") {
			t.Fatalf("expected directory error, got %q", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// rpi_loop_supervisor.go: resolveLoopConfigPaths
// ---------------------------------------------------------------------------

func TestHelper2_resolveLoopConfigPaths(t *testing.T) {
	cwd := "/project/root"
	cfg := rpiLoopSupervisorConfig{
		LeasePath:       filepath.Join(".agents", "rpi", "supervisor.lock"),
		LandingLockPath: filepath.Join(".agents", "rpi", "landing.lock"),
		KillSwitchPath:  filepath.Join(".agents", "rpi", "KILL"),
	}
	resolveLoopConfigPaths(&cfg, cwd)

	if !filepath.IsAbs(cfg.LeasePath) {
		t.Fatalf("LeasePath should be absolute, got %q", cfg.LeasePath)
	}
	if !filepath.IsAbs(cfg.LandingLockPath) {
		t.Fatalf("LandingLockPath should be absolute, got %q", cfg.LandingLockPath)
	}
	if !filepath.IsAbs(cfg.KillSwitchPath) {
		t.Fatalf("KillSwitchPath should be absolute, got %q", cfg.KillSwitchPath)
	}

	// Already absolute paths should not change
	cfg2 := rpiLoopSupervisorConfig{
		LeasePath:       "/abs/lease.lock",
		LandingLockPath: "/abs/landing.lock",
		KillSwitchPath:  "/abs/KILL",
	}
	resolveLoopConfigPaths(&cfg2, cwd)
	if cfg2.LeasePath != "/abs/lease.lock" {
		t.Fatalf("absolute LeasePath should not change, got %q", cfg2.LeasePath)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeFile moved to testutil_test.go.

// newTestRPICommand creates a minimal cobra.Command with the flags that
// applySupervisorBoolDefaults and applySupervisorPolicyDefaults check.
// None of the flags are "Changed" since no values are explicitly set.
func newTestRPICommand() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("lease", false, "")
	cmd.Flags().Bool("detached-heal", false, "")
	cmd.Flags().Bool("auto-clean", false, "")
	cmd.Flags().Bool("ensure-cleanup", false, "")
	cmd.Flags().Bool("cleanup-prune-branches", false, "")
	cmd.Flags().String("failure-policy", "stop", "")
	cmd.Flags().Int("cycle-retries", 0, "")
	cmd.Flags().Duration("cycle-delay", 0, "")
	cmd.Flags().String("gate-policy", "off", "")
	cmd.Flags().Duration("auto-clean-stale-after", 0, "")
	return cmd
}

// Ensure pool.PoolEntry is usable (reference to prevent import errors).
var _ = pool.PoolEntry{}
