//go:build windows

package subprocess

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	jobObjectBasicAccountingInformation = 1
	jobObjectExtendedLimitInformation   = 9
	jobObjectLimitKillOnJobClose        = 0x00002000
	createSuspended                     = 0x00000004
	threadSuspendResume                 = 0x0002
)

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW        = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJob      = kernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject      = kernel32.NewProc("TerminateJobObject")
	procQueryInformationJob     = kernel32.NewProc("QueryInformationJobObject")
	procThread32First           = kernel32.NewProc(thread32FirstSymbol)
	procThread32Next            = kernel32.NewProc(thread32NextSymbol)
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

type jobObjectBasicAccountingInformationValue struct {
	totalUserTime             int64
	totalKernelTime           int64
	thisPeriodTotalUserTime   int64
	thisPeriodTotalKernelTime int64
	totalPageFaultCount       uint32
	totalProcesses            uint32
	activeProcesses           uint32
	totalTerminatedProcesses  uint32
}

type windowsNativeAPI struct {
	windowsAPI
	assignProcessToJob func(windowsHandle, uintptr) error
}

type windowsProcessTree struct {
	mu                 sync.Mutex
	api                windowsNativeAPI
	clock              windowsClock
	job                windowsHandle
	waitMillis         uint32
	attached           bool
	terminateRequested bool
	terminated         bool
	closed             bool
}

func configureProcessTree(cmd *exec.Cmd, waitDelay time.Duration) (*windowsProcessTree, error) {
	job, err := createKillOnCloseJobWithAPI(nativeWindowsAPI.windowsAPI)
	if err != nil {
		return nil, err
	}
	tree := &windowsProcessTree{
		job:        job,
		api:        nativeWindowsAPI,
		clock:      systemWindowsClock(),
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

var nativeWindowsAPI = windowsNativeAPI{
	windowsAPI: windowsAPI{
		createJobObject:         nativeCreateJobObject,
		setJobKillOnClose:       nativeSetJobKillOnClose,
		terminateJob:            nativeTerminateJob,
		queryJobActiveProcesses: nativeQueryJobActiveProcesses,
		createThreadSnapshot:    nativeCreateThreadSnapshot,
		thread32First:           nativeThread32First,
		thread32Next:            nativeThread32Next,
		openThread:              nativeOpenThread,
		resumeThread:            nativeResumeThread,
		closeHandle:             nativeCloseHandle,
	},
	assignProcessToJob: nativeAssignProcessToJob,
}

func systemWindowsClock() windowsClock {
	return windowsClock{
		now:   time.Now,
		sleep: time.Sleep,
	}
}

func nativeCreateJobObject() (windowsHandle, error) {
	handle, _, callErr := procCreateJobObjectW.Call(0, 0)
	if handle == 0 {
		return 0, windowsAPIError("CreateJobObjectW", callErr)
	}
	return windowsHandle(handle), nil
}

func nativeSetJobKillOnClose(job windowsHandle) error {
	limits := jobObjectExtendedLimitInformationValue{}
	limits.basicLimitInformation.limitFlags = jobObjectLimitKillOnJobClose
	ok, _, callErr := procSetInformationJobObject.Call(
		uintptr(job),
		jobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		unsafe.Sizeof(limits),
	)
	if ok == 0 {
		return windowsAPIError("SetInformationJobObject", callErr)
	}
	return nil
}

func nativeAssignProcessToJob(job windowsHandle, processHandle uintptr) error {
	ok, _, callErr := procAssignProcessToJob.Call(uintptr(job), processHandle)
	if ok == 0 {
		return windowsAPIError("AssignProcessToJobObject", callErr)
	}
	return nil
}

func nativeTerminateJob(job windowsHandle, exitCode uint32) error {
	ok, _, callErr := procTerminateJobObject.Call(uintptr(job), uintptr(exitCode))
	if ok == 0 {
		return windowsAPIError("TerminateJobObject", callErr)
	}
	return nil
}

func nativeQueryJobActiveProcesses(job windowsHandle) (uint32, error) {
	info := jobObjectBasicAccountingInformationValue{}
	ok, _, callErr := procQueryInformationJob.Call(
		uintptr(job),
		jobObjectBasicAccountingInformation,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
		0,
	)
	if ok == 0 {
		return 0, windowsAPIError("QueryInformationJobObject", callErr)
	}
	return info.activeProcesses, nil
}

func nativeCreateThreadSnapshot() (windowsHandle, error) {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return 0, err
	}
	return windowsHandle(snapshot), nil
}

func nativeThread32First(snapshot windowsHandle, entry *threadEntry32) (bool, error) {
	ok, _, callErr := procThread32First.Call(
		uintptr(snapshot),
		uintptr(unsafe.Pointer(entry)),
	)
	return nativeThreadEnumerationResult(ok, callErr)
}

func nativeThread32Next(snapshot windowsHandle, entry *threadEntry32) (bool, error) {
	ok, _, callErr := procThread32Next.Call(
		uintptr(snapshot),
		uintptr(unsafe.Pointer(entry)),
	)
	return nativeThreadEnumerationResult(ok, callErr)
}

func nativeThreadEnumerationResult(ok uintptr, callErr error) (bool, error) {
	if ok != 0 {
		return true, nil
	}
	if errors.Is(callErr, syscall.ERROR_NO_MORE_FILES) {
		return false, errWindowsNoMoreFiles
	}
	return false, normalizeWindowsCallError(callErr)
}

func nativeOpenThread(threadID uint32) (windowsHandle, error) {
	thread, _, callErr := procOpenThread.Call(threadSuspendResume, 0, uintptr(threadID))
	if thread == 0 {
		return 0, normalizeWindowsCallError(callErr)
	}
	return windowsHandle(thread), nil
}

func nativeResumeThread(thread windowsHandle) (windowsDWORD, error) {
	result, _, callErr := procResumeThread.Call(uintptr(thread))
	resumed := windowsDWORD(result)
	if resumed == resumeThreadFailed {
		return resumed, normalizeWindowsCallError(callErr)
	}
	return resumed, nil
}

func nativeCloseHandle(handle windowsHandle) error {
	return syscall.CloseHandle(syscall.Handle(handle))
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
		assignErr = tree.api.assignProcessToJob(tree.job, processHandle)
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
	return resumeInitialProcessThreadWithAPI(tree.api.windowsAPI, uint32(cmd.Process.Pid))
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
	timeout := time.Duration(tree.waitMillis) * time.Millisecond
	if err := terminateWindowsJobWithAPI(
		tree.api.windowsAPI,
		tree.clock,
		tree.job,
		windowsProcessTreeExitCode,
		timeout,
	); err != nil {
		return err
	}
	tree.terminated = true
	return nil
}

func (tree *windowsProcessTree) close() error {
	tree.mu.Lock()
	defer tree.mu.Unlock()
	if tree.closed {
		return nil
	}
	tree.closed = true
	if err := tree.api.closeHandle(tree.job); err != nil {
		return fmt.Errorf("CloseHandle(job): %w", err)
	}
	return nil
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
	return fmt.Errorf("%s: %w", name, normalizeWindowsCallError(callErr))
}

func normalizeWindowsCallError(callErr error) error {
	if errno, ok := callErr.(syscall.Errno); ok && errno == 0 {
		return errors.New("Windows API failed without a diagnostic")
	}
	if callErr == nil {
		return errors.New("Windows API failed without a diagnostic")
	}
	return callErr
}
