package rpi

import (
	"syscall"
	"testing"
	"time"
)

func TestParseCancelSignal(t *testing.T) {
	cases := []struct {
		in   string
		want syscall.Signal
		err  bool
	}{
		{"", syscall.SIGTERM, false},
		{"TERM", syscall.SIGTERM, false},
		{"SIGTERM", syscall.SIGTERM, false},
		{"KILL", syscall.SIGKILL, false},
		{"SIGKILL", syscall.SIGKILL, false},
		{"INT", syscall.SIGINT, false},
		{"SIGINT", syscall.SIGINT, false},
		{"sigkill", syscall.SIGKILL, false}, // lowercase OK
		{"BOGUS", 0, true},
	}
	for _, tc := range cases {
		got, err := ParseCancelSignal(tc.in)
		if (err != nil) != tc.err {
			t.Errorf("%q: err = %v, wantErr = %v", tc.in, err, tc.err)
			continue
		}
		if !tc.err && got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestDedupeInts(t *testing.T) {
	got := DedupeInts([]int{3, 1, 2, 1, 3, 4})
	want := []int{1, 2, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] = %d", i, got[i])
		}
	}
}

func TestFilterKillablePIDs(t *testing.T) {
	got := FilterKillablePIDs([]int{0, 1, 99, 100, 99, 0}, 100)
	// 0, 1 excluded; 100 is self; duplicates deduped
	want := []int{99}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDescendantPIDs(t *testing.T) {
	// Process tree: 100 -> 200 -> 300, 100 -> 400
	procs := []ProcessInfo{
		{PID: 100, PPID: 1, Command: "parent"},
		{PID: 200, PPID: 100, Command: "child1"},
		{PID: 300, PPID: 200, Command: "grandchild"},
		{PID: 400, PPID: 100, Command: "child2"},
		{PID: 999, PPID: 1, Command: "unrelated"},
	}
	got := DescendantPIDs(100, procs)
	want := map[int]bool{200: true, 300: true, 400: true}
	if len(got) != 3 {
		t.Fatalf("got %v, want 3 descendants", got)
	}
	for _, pid := range got {
		if !want[pid] {
			t.Errorf("unexpected pid %d", pid)
		}
	}
}

func TestProcessExists(t *testing.T) {
	procs := []ProcessInfo{{PID: 100}, {PID: 200}}
	if !ProcessExists(100, procs) {
		t.Error("100 should exist")
	}
	if ProcessExists(500, procs) {
		t.Error("500 should not exist")
	}
}

func TestCollectRunProcessPIDs(t *testing.T) {
	procs := []ProcessInfo{
		{PID: 100, PPID: 1, Command: "ao rpi orchestrator"},
		{PID: 200, PPID: 100, Command: "child"},
		{PID: 300, PPID: 1, Command: "ao-rpi-abc123-p2-worker"},
		{PID: 400, PPID: 1, Command: "other task in /tmp/worktree/rpi/abc123"},
		{PID: 999, PPID: 1, Command: "unrelated"},
	}
	got := CollectRunProcessPIDs(100, "abc123", "/tmp/worktree/rpi/abc123", procs)
	found := map[int]bool{}
	for _, pid := range got {
		found[pid] = true
	}
	if !found[100] || !found[200] {
		t.Errorf("orchestrator+descendant missing: %v", got)
	}
	if !found[300] {
		t.Errorf("session-needle match missing: %v", got)
	}
	if !found[400] {
		t.Errorf("worktree-match missing: %v", got)
	}
	if found[999] {
		t.Errorf("unrelated PID included: %v", got)
	}
}

func TestSupervisorLeaseExpired(t *testing.T) {
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

	// No ExpiresAt -> never expires
	if SupervisorLeaseExpired(SupervisorLeaseMetadata{}, now) {
		t.Error("empty expiry should not be expired")
	}

	// Future expiry
	m := SupervisorLeaseMetadata{ExpiresAt: "2026-04-22T13:00:00Z"}
	if SupervisorLeaseExpired(m, now) {
		t.Error("future expiry should not be expired")
	}

	// Past expiry
	m2 := SupervisorLeaseMetadata{ExpiresAt: "2026-04-22T10:00:00Z"}
	if !SupervisorLeaseExpired(m2, now) {
		t.Error("past expiry should be expired")
	}

	// Malformed -> stale/expired
	m3 := SupervisorLeaseMetadata{ExpiresAt: "not a time"}
	if !SupervisorLeaseExpired(m3, now) {
		t.Error("malformed expiry should be treated as expired")
	}
}

func TestParseProcessList(t *testing.T) {
	data := []byte(`
  100     1 init
  200   100 child proc with args
  not a number line
  300   200 grandchild
`)
	got, err := ParseProcessList(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d procs, want 3: %+v", len(got), got)
	}
	if got[0].PID != 100 || got[0].PPID != 1 || got[0].Command != "init" {
		t.Errorf("first: %+v", got[0])
	}
	if got[1].Command != "child proc with args" {
		t.Errorf("second command = %q", got[1].Command)
	}
}

func TestParseProcessList_EmptyInput(t *testing.T) {
	got, err := ParseProcessList([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %+v", got)
	}
}
