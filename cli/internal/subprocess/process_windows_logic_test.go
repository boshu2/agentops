package subprocess

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWindowsThreadEnumerationUsesUnsuffixedKernel32Symbols(t *testing.T) {
	if thread32FirstSymbol != "Thread32First" {
		t.Fatalf("Thread32First symbol = %q, want unsuffixed export", thread32FirstSymbol)
	}
	if thread32NextSymbol != "Thread32Next" {
		t.Fatalf("Thread32Next symbol = %q, want unsuffixed export", thread32NextSymbol)
	}
}

func TestTerminateWindowsJobPollsActiveProcessesToZero(t *testing.T) {
	active := []uint32{2, 1, 0}
	queries := 0
	terminated := false
	now := time.Unix(0, 0)
	api := windowsAPI{
		terminateJob: func(job windowsHandle, exitCode uint32) error {
			if job != 1 || exitCode != windowsProcessTreeExitCode {
				t.Fatalf("terminate args = %d/%d, want 1/%d", job, exitCode, windowsProcessTreeExitCode)
			}
			terminated = true
			return nil
		},
		queryJobActiveProcesses: func(windowsHandle) (uint32, error) {
			if !terminated {
				t.Fatal("queried ActiveProcesses before terminating the job")
			}
			value := active[queries]
			queries++
			return value, nil
		},
	}
	clock := windowsClock{
		now: func() time.Time { return now },
		sleep: func(delay time.Duration) {
			now = now.Add(delay)
		},
	}

	if err := terminateWindowsJobWithAPI(api, clock, 1, windowsProcessTreeExitCode, 100*time.Millisecond); err != nil {
		t.Fatalf("terminateWindowsJobWithAPI: %v", err)
	}
	if queries != len(active) {
		t.Fatalf("active-process queries = %d, want %d", queries, len(active))
	}
}

func TestWaitForWindowsJobEmptyDoesNotReportCompletionWhileProcessesRemain(t *testing.T) {
	now := time.Unix(0, 0)
	queries := 0
	api := windowsAPI{
		queryJobActiveProcesses: func(windowsHandle) (uint32, error) {
			queries++
			return 1, nil
		},
	}
	clock := windowsClock{
		now: func() time.Time { return now },
		sleep: func(delay time.Duration) {
			now = now.Add(delay)
		},
	}

	err := waitForWindowsJobEmpty(api, clock, 1, 25*time.Millisecond)
	if err == nil {
		t.Fatal("waitForWindowsJobEmpty returned completed while ActiveProcesses remained nonzero")
	}
	if queries < 2 {
		t.Fatalf("active-process queries = %d, want bounded polling rather than a single observation", queries)
	}
}

func TestWaitForWindowsJobEmptyPreservesOpaqueQueryFailure(t *testing.T) {
	queryErr := errors.New("query sentinel")
	api := windowsAPI{
		queryJobActiveProcesses: func(windowsHandle) (uint32, error) {
			return 0, queryErr
		},
	}
	clock := windowsClock{now: time.Now, sleep: func(time.Duration) {}}

	if err := waitForWindowsJobEmpty(api, clock, 1, time.Second); !errors.Is(err, queryErr) {
		t.Fatalf("error = %v, want query failure identity", err)
	}
}

func TestResumeInitialProcessThreadUsesTypedDWORDAndJoinsCloseFailures(t *testing.T) {
	resumeErr := errors.New("resume sentinel")
	threadCloseErr := errors.New("thread close sentinel")
	snapshotCloseErr := errors.New("snapshot close sentinel")
	const (
		snapshotHandle windowsHandle = 10
		threadHandle   windowsHandle = 20
		processID                    = 42
	)
	api := windowsAPI{
		createThreadSnapshot: func() (windowsHandle, error) {
			return snapshotHandle, nil
		},
		thread32First: func(_ windowsHandle, entry *threadEntry32) (bool, error) {
			entry.ownerProcessID = processID
			entry.threadID = 7
			return true, nil
		},
		thread32Next: func(windowsHandle, *threadEntry32) (bool, error) {
			t.Fatal("Thread32Next must not run after the owning thread is found")
			return false, nil
		},
		openThread: func(threadID uint32) (windowsHandle, error) {
			if threadID != 7 {
				t.Fatalf("thread ID = %d, want 7", threadID)
			}
			return threadHandle, nil
		},
		resumeThread: func(windowsHandle) (windowsDWORD, error) {
			// On 64-bit hosts this value is not equal to ^uintptr(0). The
			// Win32 contract is specifically the 32-bit DWORD value -1.
			return windowsDWORD(0xffffffff), resumeErr
		},
		closeHandle: func(handle windowsHandle) error {
			switch handle {
			case threadHandle:
				return threadCloseErr
			case snapshotHandle:
				return snapshotCloseErr
			default:
				t.Fatalf("unexpected handle %d", handle)
				return nil
			}
		},
	}

	err := resumeInitialProcessThreadWithAPI(api, processID)
	for _, want := range []error{resumeErr, threadCloseErr, snapshotCloseErr} {
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want identity %v", err, want)
		}
	}
}

func TestResumeInitialProcessThreadRejectsIncompleteResumeAndJoinsCloseFailures(t *testing.T) {
	threadCloseErr := errors.New("thread close sentinel")
	snapshotCloseErr := errors.New("snapshot close sentinel")
	const (
		snapshotHandle windowsHandle = 10
		threadHandle   windowsHandle = 20
		processID                    = 42
	)
	api := windowsAPI{
		createThreadSnapshot: func() (windowsHandle, error) {
			return snapshotHandle, nil
		},
		thread32First: func(_ windowsHandle, entry *threadEntry32) (bool, error) {
			entry.ownerProcessID = processID
			entry.threadID = 7
			return true, nil
		},
		openThread: func(uint32) (windowsHandle, error) {
			return threadHandle, nil
		},
		resumeThread: func(windowsHandle) (windowsDWORD, error) {
			return 2, nil
		},
		closeHandle: func(handle windowsHandle) error {
			switch handle {
			case threadHandle:
				return threadCloseErr
			case snapshotHandle:
				return snapshotCloseErr
			default:
				return nil
			}
		},
	}

	err := resumeInitialProcessThreadWithAPI(api, processID)
	if !strings.Contains(err.Error(), "prior suspend count 2") {
		t.Fatalf("error = %v, want incomplete-resume diagnostic", err)
	}
	for _, want := range []error{threadCloseErr, snapshotCloseErr} {
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want close identity %v", err, want)
		}
	}
}

func TestCreateKillOnCloseJobJoinsConfigurationAndCloseFailures(t *testing.T) {
	configureErr := errors.New("configure job sentinel")
	closeErr := errors.New("job close sentinel")
	const job windowsHandle = 11
	api := windowsAPI{
		createJobObject: func() (windowsHandle, error) {
			return job, nil
		},
		setJobKillOnClose: func(handle windowsHandle) error {
			if handle != job {
				t.Fatalf("job handle = %d, want %d", handle, job)
			}
			return configureErr
		},
		closeHandle: func(handle windowsHandle) error {
			if handle != job {
				t.Fatalf("close handle = %d, want %d", handle, job)
			}
			return closeErr
		},
	}

	_, err := createKillOnCloseJobWithAPI(api)
	if !errors.Is(err, configureErr) || !errors.Is(err, closeErr) {
		t.Fatalf("error = %v, want configuration and CloseHandle identities", err)
	}
}
