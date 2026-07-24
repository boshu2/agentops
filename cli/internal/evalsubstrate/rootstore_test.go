package evalsubstrate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestRootStoreAtomicWriteAndConcurrentReplacement(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store, err := OpenRootStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	const writers = 12
	var wait sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wait.Add(1)
		go func(value byte) {
			defer wait.Done()
			errs <- store.WriteAtomic("runs/run-1/manifest.json", []byte{value}, 0o600)
		}(byte('a' + i))
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("WriteAtomic: %v", err)
		}
	}
	data, err := store.ReadFile("runs/run-1/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 || data[0] < 'a' || data[0] >= 'a'+writers {
		t.Fatalf("unexpected final data %q", data)
	}
	entries, err := os.ReadDir(filepath.Join(root, "runs", "run-1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temporary file leaked: %s", entry.Name())
		}
	}
}

func TestRootStoreRejectsTraversalAndAbsolutePaths(t *testing.T) {
	t.Parallel()
	store, err := OpenRootStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	for _, hostile := range []string{"../escape", "runs/../../escape", "/absolute", `C:\escape`, "C:/escape", `runs\escape`, "//server/share"} {
		if err := store.WriteAtomic(hostile, []byte("x"), 0o600); err == nil {
			t.Fatalf("WriteAtomic(%q) unexpectedly succeeded", hostile)
		}
		if _, err := store.ReadFile(hostile); err == nil {
			t.Fatalf("ReadFile(%q) unexpectedly succeeded", hostile)
		}
	}
}

func TestRootStoreRejectsSymlinkParentEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires platform privileges on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "runs")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	store, err := OpenRootStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if err := store.WriteAtomic("runs/escaped.json", []byte("owned"), 0o600); err == nil {
		t.Fatal("WriteAtomic through escaping symlink unexpectedly succeeded")
	}
	if _, err := store.ReadFile("runs/secret.json"); err == nil {
		t.Fatal("ReadFile through escaping symlink unexpectedly succeeded")
	}
	if _, err := os.Stat(filepath.Join(outside, "escaped.json")); !os.IsNotExist(err) {
		t.Fatalf("outside target was created: %v", err)
	}
}
