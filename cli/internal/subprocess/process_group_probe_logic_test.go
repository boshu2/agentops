//go:build darwin || linux

package subprocess

import (
	"errors"
	"os"
	"slices"
	"syscall"
	"testing"
)

func TestProbeLinuxProcessGroupSkipsUnrelatedUnreadableStats(t *testing.T) {
	var statReads []int
	err := probeLinuxProcessGroup(42, linuxProcessGroupProbeOps{
		listPIDs: func() ([]int, error) {
			return []int{100, 200}, nil
		},
		processGroup: func(pid int) (int, error) {
			switch pid {
			case 100:
				return 7, nil
			case 200:
				return 42, nil
			default:
				t.Fatalf("processGroup(%d) called for unexpected PID", pid)
				return 0, nil
			}
		},
		readState: func(pid int) (string, int, error) {
			statReads = append(statReads, pid)
			if pid == 100 {
				return "", 0, syscall.EACCES
			}
			return linuxZombieProcessState, 42, nil
		},
	})
	if !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("probeLinuxProcessGroup error = %v, want ESRCH", err)
	}
	if want := []int{200}; !slices.Equal(statReads, want) {
		t.Fatalf("stat reads = %v, want %v", statReads, want)
	}
}

func TestProbeLinuxProcessGroupFailsClosedForOpaqueMembership(t *testing.T) {
	statRead := false
	err := probeLinuxProcessGroup(42, linuxProcessGroupProbeOps{
		listPIDs: func() ([]int, error) {
			return []int{100}, nil
		},
		processGroup: func(int) (int, error) {
			return 0, syscall.EPERM
		},
		readState: func(int) (string, int, error) {
			statRead = true
			return "", 0, nil
		},
	})
	if !errors.Is(err, syscall.EPERM) {
		t.Fatalf("probeLinuxProcessGroup error = %v, want EPERM", err)
	}
	if statRead {
		t.Fatal("readState called after process-group membership was opaque")
	}
}

func TestProbeLinuxProcessGroupFailsClosedForOpaqueTargetState(t *testing.T) {
	err := probeLinuxProcessGroup(42, linuxProcessGroupProbeOps{
		listPIDs: func() ([]int, error) {
			return []int{100}, nil
		},
		processGroup: func(int) (int, error) {
			return 42, nil
		},
		readState: func(int) (string, int, error) {
			return "", 0, syscall.EACCES
		},
	})
	if !errors.Is(err, syscall.EACCES) {
		t.Fatalf("probeLinuxProcessGroup error = %v, want EACCES", err)
	}
}

func TestProbeLinuxProcessGroupIgnoresDisappearingProcesses(t *testing.T) {
	t.Run("before membership query", func(t *testing.T) {
		statRead := false
		err := probeLinuxProcessGroup(42, linuxProcessGroupProbeOps{
			listPIDs: func() ([]int, error) {
				return []int{100}, nil
			},
			processGroup: func(int) (int, error) {
				return 0, syscall.ESRCH
			},
			readState: func(int) (string, int, error) {
				statRead = true
				return "", 0, nil
			},
		})
		if !errors.Is(err, syscall.ESRCH) {
			t.Fatalf("probeLinuxProcessGroup error = %v, want ESRCH", err)
		}
		if statRead {
			t.Fatal("readState called after process disappeared")
		}
	})

	t.Run("before state read", func(t *testing.T) {
		err := probeLinuxProcessGroup(42, linuxProcessGroupProbeOps{
			listPIDs: func() ([]int, error) {
				return []int{100}, nil
			},
			processGroup: func(int) (int, error) {
				return 42, nil
			},
			readState: func(int) (string, int, error) {
				return "", 0, os.ErrNotExist
			},
		})
		if !errors.Is(err, syscall.ESRCH) {
			t.Fatalf("probeLinuxProcessGroup error = %v, want ESRCH", err)
		}
	})
}

func TestProbeLinuxProcessGroupFailsClosedWhenProcUnavailable(t *testing.T) {
	procErr := errors.New("procfs unavailable")
	err := probeLinuxProcessGroup(42, linuxProcessGroupProbeOps{
		listPIDs: func() ([]int, error) {
			return nil, procErr
		},
		processGroup: func(int) (int, error) {
			t.Fatal("processGroup called when procfs enumeration failed")
			return 0, nil
		},
		readState: func(int) (string, int, error) {
			t.Fatal("readState called when procfs enumeration failed")
			return "", 0, nil
		},
	})
	if !errors.Is(err, procErr) {
		t.Fatalf("probeLinuxProcessGroup error = %v, want %v", err, procErr)
	}
	if errors.Is(err, syscall.ESRCH) {
		t.Fatalf("probeLinuxProcessGroup error = %v, must not claim absence", err)
	}
}

func TestProbeLinuxProcessGroupFiltersZombies(t *testing.T) {
	var statReads []int
	err := probeLinuxProcessGroup(42, linuxProcessGroupProbeOps{
		listPIDs: func() ([]int, error) {
			return []int{100, 101}, nil
		},
		processGroup: func(int) (int, error) {
			return 42, nil
		},
		readState: func(pid int) (string, int, error) {
			statReads = append(statReads, pid)
			if pid == 100 {
				return linuxZombieProcessState, 42, nil
			}
			return "S", 42, nil
		},
	})
	if err != nil {
		t.Fatalf("probeLinuxProcessGroup: %v", err)
	}
	if want := []int{100, 101}; !slices.Equal(statReads, want) {
		t.Fatalf("stat reads = %v, want %v", statReads, want)
	}
}

func TestProbeLinuxProcessGroupIgnoresMembershipChangedBeforeStateRead(t *testing.T) {
	err := probeLinuxProcessGroup(42, linuxProcessGroupProbeOps{
		listPIDs: func() ([]int, error) {
			return []int{100}, nil
		},
		processGroup: func(int) (int, error) {
			return 42, nil
		},
		readState: func(int) (string, int, error) {
			return "S", 7, nil
		},
	})
	if !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("probeLinuxProcessGroup error = %v, want ESRCH", err)
	}
}
