package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test-store.json")
	return New(path)
}

func TestSetAndGet(t *testing.T) {
	s := tempStore(t)
	if err := s.Set("color", "blue"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get("color")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "blue" {
		t.Errorf("Get(color) = %q, want %q", got, "blue")
	}
}

func TestGetNotFound(t *testing.T) {
	s := tempStore(t)
	_, err := s.Get("missing")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrKeyNotFound", err)
	}
}

func TestSetOverwrite(t *testing.T) {
	s := tempStore(t)
	_ = s.Set("k", "v1")
	_ = s.Set("k", "v2")
	got, _ := s.Get("k")
	if got != "v2" {
		t.Errorf("Get(k) = %q after overwrite, want %q", got, "v2")
	}
}

func TestDelete(t *testing.T) {
	s := tempStore(t)
	_ = s.Set("k", "v")
	if err := s.Delete("k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := s.Get("k")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Get after Delete: error = %v, want ErrKeyNotFound", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	s := tempStore(t)
	err := s.Delete("missing")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Delete(missing) error = %v, want ErrKeyNotFound", err)
	}
}

func TestList(t *testing.T) {
	s := tempStore(t)
	_ = s.Set("a", "1")
	_ = s.Set("b", "2")
	m := s.List()
	if len(m) != 2 {
		t.Fatalf("List() len = %d, want 2", len(m))
	}
	if m["a"] != "1" || m["b"] != "2" {
		t.Errorf("List() = %v, want map[a:1 b:2]", m)
	}
}

func TestListEmpty(t *testing.T) {
	s := tempStore(t)
	m := s.List()
	if len(m) != 0 {
		t.Errorf("List() on empty store = %v, want empty", m)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rt.json")
	s1 := New(path)
	_ = s1.Set("x", "10")
	_ = s1.Set("y", "20")

	s2 := New(path)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, err := s2.Get("x")
	if err != nil {
		t.Fatalf("Get after Load: %v", err)
	}
	if got != "10" {
		t.Errorf("Get(x) after Load = %q, want %q", got, "10")
	}
	got, err = s2.Get("y")
	if err != nil {
		t.Fatalf("Get after Load: %v", err)
	}
	if got != "20" {
		t.Errorf("Get(y) after Load = %q, want %q", got, "20")
	}
}

func TestLoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nofile.json")
	s := New(path)
	if err := s.Load(); err != nil {
		t.Errorf("Load on missing file: %v", err)
	}
	if len(s.List()) != 0 {
		t.Errorf("List after Load missing = %v, want empty", s.List())
	}
}

func TestLoadCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(path, []byte("{invalid"), 0644)
	s := New(path)
	if err := s.Load(); err == nil {
		t.Error("Load on corrupt file: expected error, got nil")
	}
}
