// Package store provides a file-backed key-value store.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

// ErrKeyNotFound is returned when a key does not exist.
var ErrKeyNotFound = errors.New("key not found")

// Store is an in-memory key-value store backed by a JSON file.
type Store struct {
	path string
	mu   sync.Mutex
	data map[string]string
}

// New creates a Store that persists to the given file path.
func New(path string) *Store {
	return &Store{
		path: path,
		data: make(map[string]string),
	}
}

// Set adds or updates a key-value pair and persists to disk.
func (s *Store) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return s.save()
}

// Get returns the value for key, or ErrKeyNotFound.
func (s *Store) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[key]
	if !ok {
		return "", fmt.Errorf("get %q: %w", key, ErrKeyNotFound)
	}
	return v, nil
}

// Delete removes a key, or returns ErrKeyNotFound.
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; !ok {
		return fmt.Errorf("delete %q: %w", key, ErrKeyNotFound)
	}
	delete(s.data, key)
	return s.save()
}

// List returns a copy of all key-value pairs.
func (s *Store) List() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.data))
	for k, v := range s.data {
		out[k] = v
	}
	return out
}

// Save writes the store to disk as JSON.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal store: %w", err)
	}
	if err := os.WriteFile(s.path, b, 0644); err != nil {
		return fmt.Errorf("write store %q: %w", s.path, err)
	}
	return nil
}

// Load reads the store from disk. Missing file starts empty.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read store %q: %w", s.path, err)
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return fmt.Errorf("unmarshal store: %w", err)
	}
	return nil
}
