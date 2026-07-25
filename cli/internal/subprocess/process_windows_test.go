//go:build windows

package subprocess

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestConfigureProcessTreeCreatesKillOnCloseJob(t *testing.T) {
	cmd := exec.Command("cmd", "/C", "exit", "/b", "0")
	tree, err := configureProcessTree(cmd, 250*time.Millisecond)
	if err != nil {
		t.Fatalf("configureProcessTree: %v", err)
	}
	t.Cleanup(func() {
		if err := tree.close(); err != nil {
			t.Errorf("close job: %v", err)
		}
	})
	if tree.job == 0 {
		t.Fatal("job handle = 0")
	}
	if tree.waitMillis != 250 {
		t.Fatalf("waitMillis = %d, want 250", tree.waitMillis)
	}
	wantFlags := uint32(syscall.CREATE_NEW_PROCESS_GROUP | createSuspended)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&wantFlags != wantFlags {
		t.Fatalf("SysProcAttr = %#v, want creation flags %#x", cmd.SysProcAttr, wantFlags)
	}
	if cmd.Cancel == nil {
		t.Fatal("Cancel hook is nil")
	}
}

func TestWindowsProcessTreeTerminatesAssignedProcess(t *testing.T) {
	cmd := exec.Command("cmd", "/C", "ping", "-t", "127.0.0.1")
	tree, err := configureProcessTree(cmd, 2*time.Second)
	if err != nil {
		t.Fatalf("configureProcessTree: %v", err)
	}
	t.Cleanup(func() {
		if err := tree.close(); err != nil {
			t.Errorf("close job: %v", err)
		}
	})
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := tree.attach(cmd); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		t.Fatalf("attach: %v", err)
	}
	if err := tree.terminate(cmd); err != nil {
		t.Fatalf("terminate: %v", err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("Wait error = nil, want job termination exit")
	}
}

func TestWindowsProcessTreeDefersTerminationUntilAttach(t *testing.T) {
	cmd := exec.Command("cmd", "/C", "ping", "-t", "127.0.0.1")
	tree, err := configureProcessTree(cmd, 2*time.Second)
	if err != nil {
		t.Fatalf("configureProcessTree: %v", err)
	}
	t.Cleanup(func() {
		if err := tree.close(); err != nil {
			t.Errorf("close job: %v", err)
		}
	})
	if err := tree.terminate(cmd); err != nil {
		t.Fatalf("request termination before attach: %v", err)
	}
	if !tree.terminateRequested || tree.terminated {
		t.Fatalf("tree state = %+v, want pending termination", tree)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := tree.attach(cmd); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		t.Fatalf("attach: %v", err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("Wait error = nil, want deferred job termination exit")
	}
	if !tree.terminated {
		t.Fatal("tree was not marked terminated after deferred request")
	}
}

func TestFiniteWindowsWaitMillis(t *testing.T) {
	tests := []struct {
		name  string
		delay time.Duration
		want  uint32
	}{
		{name: "default", delay: 0, want: uint32(defaultWaitDelay / time.Millisecond)},
		{name: "round up", delay: time.Millisecond + time.Nanosecond, want: 2},
		{name: "exact", delay: 250 * time.Millisecond, want: 250},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := finiteWindowsWaitMillis(test.delay); got != test.want {
				t.Fatalf("finiteWindowsWaitMillis(%s) = %d, want %d", test.delay, got, test.want)
			}
		})
	}
}
