package scope

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

func tmpLock(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, ".agents", "scope.lock")
}

func TestRead_MissingFile_ReturnsEmptyLock(t *testing.T) {
	lock, err := Read(filepath.Join(t.TempDir(), "nonexistent.lock"))
	if err != nil {
		t.Fatalf("Read missing: %v", err)
	}
	if lock == nil {
		t.Fatal("want non-nil empty lock")
	}
	if lock.SchemaVersion != SchemaVersion {
		t.Fatalf("want SchemaVersion %d, got %d", SchemaVersion, lock.SchemaVersion)
	}
	if len(lock.FrozenDirs) != 0 {
		t.Fatalf("want empty FrozenDirs, got %v", lock.FrozenDirs)
	}
}

func TestRead_EmptyFile_ReturnsEmptyLock(t *testing.T) {
	path := tmpLock(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := Read(path)
	if err != nil {
		t.Fatalf("Read empty: %v", err)
	}
	if len(lock.FrozenDirs) != 0 {
		t.Fatalf("want empty FrozenDirs, got %v", lock.FrozenDirs)
	}
}

func TestWriteThenRead_RoundTrip(t *testing.T) {
	path := tmpLock(t)
	want := &Lock{
		SchemaVersion: SchemaVersion,
		FrozenDirs:    []string{"cli/cmd/ao", "skills/scope"},
		AcquiredBy:    "test-session",
	}
	if err := Write(path, want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.SchemaVersion != want.SchemaVersion {
		t.Fatalf("SchemaVersion mismatch: want %d, got %d", want.SchemaVersion, got.SchemaVersion)
	}
	if got.AcquiredBy != "test-session" {
		t.Fatalf("AcquiredBy mismatch: want test-session, got %q", got.AcquiredBy)
	}
	if got.AcquiredAt.IsZero() {
		t.Fatal("AcquiredAt was not populated")
	}
	if len(got.FrozenDirs) != 2 || got.FrozenDirs[0] != "cli/cmd/ao" || got.FrozenDirs[1] != "skills/scope" {
		t.Fatalf("FrozenDirs round-trip mismatch: got %v", got.FrozenDirs)
	}
}

func TestWrite_NormalizesEmptyFrozenDirsToEmptyArray(t *testing.T) {
	path := tmpLock(t)
	if err := Write(path, &Lock{}); err != nil {
		t.Fatalf("Write empty: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("parse: %v", err)
	}
	dirs, ok := generic["frozen_dirs"].([]interface{})
	if !ok {
		t.Fatalf("frozen_dirs not array: %T", generic["frozen_dirs"])
	}
	if len(dirs) != 0 {
		t.Fatalf("want empty array, got %v", dirs)
	}
}

func TestFreeze_AppendsAndDeduplicates(t *testing.T) {
	path := tmpLock(t)
	if err := Freeze(path, []string{"a/", "b"}); err != nil {
		t.Fatalf("Freeze 1: %v", err)
	}
	if err := Freeze(path, []string{"b/", "c/"}); err != nil {
		t.Fatalf("Freeze 2: %v", err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b", "c"}
	sort.Strings(got.FrozenDirs)
	if len(got.FrozenDirs) != len(want) {
		t.Fatalf("want %v, got %v", want, got.FrozenDirs)
	}
	for i := range want {
		if got.FrozenDirs[i] != want[i] {
			t.Fatalf("want %v, got %v", want, got.FrozenDirs)
		}
	}
}

func TestUnfreeze_All(t *testing.T) {
	path := tmpLock(t)
	if err := Freeze(path, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if err := Unfreeze(path, nil); err != nil {
		t.Fatalf("Unfreeze all: %v", err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.FrozenDirs) != 0 {
		t.Fatalf("want empty, got %v", got.FrozenDirs)
	}
}

func TestUnfreeze_OneOnly(t *testing.T) {
	path := tmpLock(t)
	if err := Freeze(path, []string{"a", "b", "c"}); err != nil {
		t.Fatal(err)
	}
	if err := Unfreeze(path, []string{"b/"}); err != nil {
		t.Fatalf("Unfreeze one: %v", err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "c"}
	if len(got.FrozenDirs) != len(want) {
		t.Fatalf("want %v, got %v", want, got.FrozenDirs)
	}
	for i := range want {
		if got.FrozenDirs[i] != want[i] {
			t.Fatalf("want %v, got %v", want, got.FrozenDirs)
		}
	}
}

func TestRead_InvalidJSON_ReturnsError(t *testing.T) {
	path := tmpLock(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Read(path)
	if err == nil {
		t.Fatal("want parse error, got nil")
	}
}

func TestWrite_NilLock_Errors(t *testing.T) {
	if err := Write(tmpLock(t), nil); err == nil {
		t.Fatal("want error on nil lock")
	} else if !errors.Is(err, err) { // sanity
		t.Fatalf("unexpected: %v", err)
	}
}

func TestIsAllowed_Predicate(t *testing.T) {
	cases := []struct {
		name   string
		frozen []string
		target string
		want   bool
	}{
		{"nil-lock-allows", nil, "anything/foo.go", true},
		{"empty-frozen-allows", []string{}, "anything/foo.go", true},
		{"exact-prefix-matches", []string{"cli/cmd/ao"}, "cli/cmd/ao/scope.go", true},
		{"trailing-slash-matches", []string{"cli/cmd/ao/"}, "cli/cmd/ao/scope.go", true},
		{"sibling-rejects", []string{"cli/cmd/ao"}, "cli/cmd/foo/scope.go", false},
		{"unrelated-rejects", []string{"cli/cmd/ao"}, "skills/scope/SKILL.md", false},
		{"nested-allows", []string{"skills"}, "skills/scope/SKILL.md", true},
		{"deep-nested-allows", []string{"a/b/c"}, "a/b/c/d/e/f.go", true},
		{"prefix-bleed-rejects", []string{"foo"}, "foobar/baz.go", false},
		{"multiple-frozen-any-allows", []string{"x", "skills"}, "skills/scope/SKILL.md", true},
		{"multiple-frozen-none-rejects", []string{"x", "y"}, "skills/scope/SKILL.md", false},
		{"exact-dir-allows", []string{"cli/cmd/ao"}, "cli/cmd/ao", true},
	}
	for _, tc := range cases {

		t.Run(tc.name, func(t *testing.T) {
			lock := &Lock{FrozenDirs: tc.frozen}
			if tc.frozen == nil {
				lock = nil
			}
			got := IsAllowed(lock, tc.target)
			if got != tc.want {
				t.Fatalf("IsAllowed(%v, %q) = %v, want %v", tc.frozen, tc.target, got, tc.want)
			}
		})
	}
}

func TestWrite_AtomicityUnderConcurrency(t *testing.T) {
	// L2 race scenario: 100 concurrent Freeze calls. Final lock file must:
	//   - parse as valid JSON (atomic-write invariant — no torn write)
	//   - contain at least one of the requested directories (writers don't all
	//     stomp into nothing)
	//
	// SafeAtomicWrite uses temp+rename, so readers either see the previous
	// file or the next file, never a half-written one.
	path := tmpLock(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)
	errs := make(chan error, N)
	for i := 0; i < N; i++ {

		go func() {
			defer wg.Done()
			if err := Freeze(path, []string{"dir/" + itoa(i)}); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent Freeze: %v", err)
	}

	// Final file must parse and have at least one frozen dir.
	got, err := Read(path)
	if err != nil {
		t.Fatalf("final Read: %v", err)
	}
	if len(got.FrozenDirs) == 0 {
		t.Fatal("expected at least one frozen dir survived the race")
	}
	// All entries must look like "dir/N" — proves no torn write yielded
	// a partial string.
	for _, d := range got.FrozenDirs {
		if len(d) < 4 || d[:4] != "dir/" {
			t.Fatalf("torn or stray entry: %q", d)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	x := n
	for x > 0 {
		digits = append([]byte{byte('0' + x%10)}, digits...)
		x /= 10
	}
	return string(digits)
}
