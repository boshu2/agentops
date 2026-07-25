package subprocess

import (
	"errors"
	"fmt"
	"time"
	"unsafe"
)

const (
	thread32FirstSymbol        = "Thread32First"
	thread32NextSymbol         = "Thread32Next"
	resumeThreadFailed         = windowsDWORD(0xffffffff)
	windowsProcessTreeExitCode = 1
	windowsPollInterval        = 10 * time.Millisecond
)

var errWindowsNoMoreFiles = errors.New("windows thread snapshot has no more files")

type windowsHandle uintptr
type windowsDWORD uint32

type threadEntry32 struct {
	size           uint32
	_              uint32
	threadID       uint32
	ownerProcessID uint32
	_              int32
	_              int32
	_              uint32
}

type windowsAPI struct {
	createJobObject         func() (windowsHandle, error)
	setJobKillOnClose       func(windowsHandle) error
	terminateJob            func(windowsHandle, uint32) error
	queryJobActiveProcesses func(windowsHandle) (uint32, error)
	createThreadSnapshot    func() (windowsHandle, error)
	thread32First           func(windowsHandle, *threadEntry32) (bool, error)
	thread32Next            func(windowsHandle, *threadEntry32) (bool, error)
	openThread              func(uint32) (windowsHandle, error)
	resumeThread            func(windowsHandle) (windowsDWORD, error)
	closeHandle             func(windowsHandle) error
}

type windowsClock struct {
	now   func() time.Time
	sleep func(time.Duration)
}

func createKillOnCloseJobWithAPI(api windowsAPI) (windowsHandle, error) {
	job, err := api.createJobObject()
	if err != nil {
		return 0, err
	}
	if err := api.setJobKillOnClose(job); err != nil {
		closeErr := api.closeHandle(job)
		if closeErr != nil {
			closeErr = fmt.Errorf("CloseHandle(job) after configuration failure: %w", closeErr)
		}
		return 0, errors.Join(err, closeErr)
	}
	return job, nil
}

func terminateWindowsJobWithAPI(
	api windowsAPI,
	clock windowsClock,
	job windowsHandle,
	exitCode uint32,
	timeout time.Duration,
) error {
	if err := api.terminateJob(job, exitCode); err != nil {
		return err
	}
	return waitForWindowsJobEmpty(api, clock, job, timeout)
}

func waitForWindowsJobEmpty(api windowsAPI, clock windowsClock, job windowsHandle, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = defaultWaitDelay
	}
	deadline := clock.now().Add(timeout)
	for {
		active, err := api.queryJobActiveProcesses(job)
		if err != nil {
			return fmt.Errorf("query terminated job active processes: %w", err)
		}
		if active == 0 {
			return nil
		}
		now := clock.now()
		if !now.Before(deadline) {
			return fmt.Errorf("wait for terminated job to empty timed out after %s with %d active processes", timeout, active)
		}
		delay := min(windowsPollInterval, deadline.Sub(now))
		clock.sleep(delay)
	}
}

func resumeInitialProcessThreadWithAPI(api windowsAPI, processID uint32) (returnErr error) {
	snapshot, err := api.createThreadSnapshot()
	if err != nil {
		return fmt.Errorf("snapshot process threads: %w", err)
	}
	defer func() {
		if closeErr := api.closeHandle(snapshot); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("CloseHandle(thread snapshot): %w", closeErr))
		}
	}()

	entry := threadEntry32{size: uint32(unsafe.Sizeof(threadEntry32{}))}
	ok, enumErr := api.thread32First(snapshot, &entry)
	if !ok {
		return threadEnumerationError("Thread32First", processID, enumErr)
	}
	for {
		if entry.ownerProcessID == processID {
			thread, openErr := api.openThread(entry.threadID)
			if openErr != nil {
				return fmt.Errorf("OpenThread: %w", openErr)
			}
			resumeResult, resumeErr := api.resumeThread(thread)
			closeErr := api.closeHandle(thread)

			var operationErr error
			if resumeResult == resumeThreadFailed {
				if resumeErr == nil {
					resumeErr = errors.New("ResumeThread failed without a diagnostic")
				}
				operationErr = fmt.Errorf("ResumeThread: %w", resumeErr)
			}
			if closeErr != nil {
				closeErr = fmt.Errorf("CloseHandle(thread): %w", closeErr)
			}
			return errors.Join(operationErr, closeErr)
		}

		entry.size = uint32(unsafe.Sizeof(threadEntry32{}))
		ok, enumErr = api.thread32Next(snapshot, &entry)
		if !ok {
			return threadEnumerationError("Thread32Next", processID, enumErr)
		}
	}
}

func threadEnumerationError(operation string, processID uint32, err error) error {
	if errors.Is(err, errWindowsNoMoreFiles) {
		return fmt.Errorf("initial thread for process %d was not found", processID)
	}
	if err == nil {
		err = errors.New("enumeration failed without a diagnostic")
	}
	return fmt.Errorf("%s: %w", operation, err)
}
