// Package kvstore is a tiny string key-value store persisted as a JSON object.
package kvstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// ErrNotFound is returned by Delete when the key does not exist.
var ErrNotFound = errors.New("key not found")

// Pair is a key/value entry.
type Pair struct {
	Key   string
	Value string
}

// Store is a file-backed key-value store.
type Store struct {
	path string
	data map[string]string
}

// Open loads the store at path; a missing file is an empty store.
func Open(path string) (*Store, error) {
	s := &Store{path: path, data: map[string]string{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if s.data == nil {
		s.data = map[string]string{}
	}
	return s, nil
}

// Get returns the value for key.
func (s *Store) Get(key string) (string, bool) {
	v, ok := s.data[key]
	return v, ok
}

// Set stores value under key and persists atomically.
func (s *Store) Set(key, value string) error {
	old, had := s.data[key]
	s.data[key] = value
	if err := s.save(); err != nil {
		if had {
			s.data[key] = old
		} else {
			delete(s.data, key)
		}
		return err
	}
	return nil
}

// Delete removes key, returning ErrNotFound if absent.
func (s *Store) Delete(key string) error {
	old, ok := s.data[key]
	if !ok {
		return ErrNotFound
	}
	delete(s.data, key)
	if err := s.save(); err != nil {
		s.data[key] = old
		return err
	}
	return nil
}

// List returns all pairs sorted by key.
func (s *Store) List() []Pair {
	out := make([]Pair, 0, len(s.data))
	for k, v := range s.data {
		out = append(out, Pair{k, v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Export writes the data to w as a JSON object with sorted keys, two-space indentation and a trailing newline.
func (s *Store) Export(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(s.data)
}

// save writes to a temp file in the same directory, then renames over the target.
func (s *Store) save() error {
	b, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".kv-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, s.path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
