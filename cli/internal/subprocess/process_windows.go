//go:build windows

package subprocess

import (
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x00002000
	windowsProcessTreeExitCode        = 1
	createSuspended                   = 0x00000004
	threadSuspendResume               = 0x0002
	resumeThreadFailed                = ^uintptr(0)
)

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW        = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJob      = kernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject      = kernel32.NewProc("TerminateJobObject")
	procThread32FirstW          = kernel32.NewProc("Thread32First")
	procThread32NextW           = kernel32.NewProc("Thread32Next")
	procOpenThread              = kernel32.NewProc("OpenThread")
	procResumeThread            = kernel32.NewProc("ResumeThread")
)

type jobObjectBasicLimitInformation struct {
	perProcessUserTimeLimit int64
	perJobUserTimeLimit     int64
	limitFlags              uint32
	minimumWorkingSetSize   uintptr
	maximumWorkingSetSize   uintptr
	activeProcessLimit      uint32
	affinity                uintptr
	priorityClass           uint32
	schedulingClass         uint32
	_                       [8 - unsafe.Sizeof(uintptr(0))]byte
}

type ioCounters struct {
	readOperationCount  uint64
	writeOperationCount uint64
	otherOperationCount uint64
	readTransferCount   uint64
	writeTransferCount  uint64
	otherTransferCount  uint64
}

type jobObjectExtendedLimitInformationValue struct {
	basicLimitInformation jobObjectBasicLimitInformation
	ioInfo                ioCounters
	processMemoryLimit    uintptr
	jobMemoryLimit        uintptr
	peakProcessMemoryUsed uintptr
	peakJobMemoryUsed     uintptr
}

type windowsProcessTree struct {
	mu                 sync.Mutex
	job                syscall.Handle
	waitMillis         uint32
	attached           bool
	terminateRequested bool
	terminated         bool
	closed             bool
}

type threadEntry32 struct {
	size           uint32
	usage          uint32
	threadID       uint32
	ownerProcessID uint32
	basePriority   int32
	deltaPriority  int32
	flags          uint32
}

func configureProcessTree(cmd *exec.Cmd, waitDelay time.Duration) (*windowsProcessTree, error) {
	job, err := createKillOnCloseJob()
	if err != nil {
		return nil, err
	}
	tree := &windowsProcessTree{
		job:        job,
		waitMillis: finiteWindowsWaitMillis(waitDelay),
	}
	// Start suspended so the child cannot exit or create descendants before it
	// is assigned to the Job Object. attach resumes the sole initial thread only
	// after the assignment succeeds.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createSuspended}
	cmd.Cancel = func() error {
		return tree.terminate(cmd)
	}
	return tree, nil
}

func createKillOnCloseJob() (syscall.Handle, error) {
	handle, _, callErr := procCreateJobObjectW.Call(0, 0)
	if handle == 0 {
		return 0, windowsAPIError("CreateJobObjectW", callErr)
	}
	job := syscall.Handle(handle)
	limits := jobObjectExtendedLimitInformationValue{}
	limits.basicLimitInformation.limitFlags = jobObjectLimitKillOnJobClose
	ok, _, callErr := procSetInformationJobObject.Call(
		uintptr(job),
		jobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		unsafe.Sizeof(limits),
	)
	if ok == 0 {
		_ = syscall.CloseHandle(job)
		return 0, windowsAPIError("SetInformationJobObject", callErr)
	}
	return job, nil
}

func (tree *windowsProcessTree) attach(cmd *exec.Cmd) error {
	tree.mu.Lock()
	defer tree.mu.Unlock()
	if tree.closed {
		return fmt.Errorf("job object is already closed")
	}
	if cmd.Process == nil {
		return fmt.Errorf("process handle is unavailable")
	}
	var assignErr error
	if err := cmd.Process.WithHandle(func(processHandle uintptr) {
		ok, _, callErr := procAssignProcessToJob.Call(uintptr(tree.job), processHandle)
		if ok == 0 {
			assignErr = windowsAPIError("AssignProcessToJobObject", callErr)
		}
	}); err != nil {
		return fmt.Errorf("access process handle: %w", err)
	}
	if assignErr != nil {
		return assignErr
	}
	tree.attached = true
	if tree.terminateRequested {
		return tree.terminateLocked()
	}
	return resumeInitialProcessThread(uint32(cmd.Process.Pid))
}

func (tree *windowsProcessTree) terminate(_ *exec.Cmd) error {
	tree.mu.Lock()
	defer tree.mu.Unlock()
	return tree.terminateLocked()
}

func (tree *windowsProcessTree) terminateLocked() error {
	if tree.closed {
		return fmt.Errorf("job object is already closed")
	}
	if tree.terminated {
		return nil
	}
	if !tree.attached {
		// CommandContext can observe cancellation in the narrow interval after
		// Start and before attach. Record the request; attach will assign the
		// still-suspended process and terminate the populated job before it runs.
		tree.terminateRequested = true
		return nil
	}
	ok, _, callErr := procTerminateJobObject.Call(uintptr(tree.job), windowsProcessTreeExitCode)
	if ok == 0 {
		return windowsAPIError("TerminateJobObject", callErr)
	}
	waitResult, err := syscall.WaitForSingleObject(tree.job, tree.waitMillis)
	if err != nil {
		return fmt.Errorf("wait for terminated job object: %w", err)
	}
	switch waitResult {
	case syscall.WAIT_OBJECT_0:
		tree.terminated = true
		return nil
	case syscall.WAIT_TIMEOUT:
		return fmt.Errorf("wait for terminated job object timed out after %dms", tree.waitMillis)
	default:
		return fmt.Errorf("wait for terminated job object returned status %#x", waitResult)
	}
}

func (tree *windowsProcessTree) close() error {
	tree.mu.Lock()
	defer tree.mu.Unlock()
	if tree.closed {
		return nil
	}
	tree.closed = true
	if err := syscall.CloseHandle(tree.job); err != nil {
		return fmt.Errorf("CloseHandle(job): %w", err)
	}
	return nil
}

func resumeInitialProcessThread(processID uint32) error {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("snapshot process threads: %w", err)
	}
	defer func() {
		_ = syscall.CloseHandle(snapshot)
	}()

	entry := threadEntry32{size: uint32(unsafe.Sizeof(threadEntry32{}))}
	ok, _, callErr := procThread32FirstW.Call(
		uintptr(snapshot),
		uintptr(unsafe.Pointer(&entry)),
	)
	for ok != 0 {
		if entry.ownerProcessID == processID {
			thread, _, openErr := procOpenThread.Call(threadSuspendResume, 0, uintptr(entry.threadID))
			if thread == 0 {
				return windowsAPIError("OpenThread", openErr)
			}
			resumeResult, _, resumeErr := procResumeThread.Call(thread)
			closeErr := syscall.CloseHandle(syscall.Handle(thread))
			if resumeResult == resumeThreadFailed {
				return windowsAPIError("ResumeThread", resumeErr)
			}
			if closeErr != nil {
				return fmt.Errorf("CloseHandle(thread): %w", closeErr)
			}
			return nil
		}
		entry.size = uint32(unsafe.Sizeof(threadEntry32{}))
		ok, _, callErr = procThread32NextW.Call(
			uintptr(snapshot),
			uintptr(unsafe.Pointer(&entry)),
		)
	}
	if errno, isErrno := callErr.(syscall.Errno); isErrno && errno == syscall.ERROR_NO_MORE_FILES {
		return fmt.Errorf("initial thread for process %d was not found", processID)
	}
	return windowsAPIError("Thread32Next", callErr)
}

func finiteWindowsWaitMillis(delay time.Duration) uint32 {
	if delay <= 0 {
		delay = defaultWaitDelay
	}
	millis := delay / time.Millisecond
	if delay%time.Millisecond != 0 {
		millis++
	}
	if millis < 1 {
		return 1
	}
	const maxFiniteWait = uint64(syscall.INFINITE - 1)
	if uint64(millis) > maxFiniteWait {
		return uint32(maxFiniteWait)
	}
	return uint32(millis)
}

func windowsAPIError(name string, callErr error) error {
	if errno, ok := callErr.(syscall.Errno); ok && errno == 0 {
		return fmt.Errorf("%s failed", name)
	}
	return fmt.Errorf("%s: %w", name, callErr)
}
