// Package kvstore is a tiny string key-value store persisted as a JSON object.
package kvstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
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
	data, err := decode(b)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	s.data = data
	return s, nil
}

// decode parses a JSON object of string values; empty input or null is an empty map.
func decode(b []byte) (map[string]string, error) {
	m := map[string]string{}
	if len(b) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]string{}
	}
	return m, nil
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

// Export writes the whole store to w as indented JSON, the same format as the store file.
func (s *Store) Export(w io.Writer) error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// Import merges the JSON object read from r into the store with one atomic save.
// Keys in r overwrite existing ones; other keys stay. On error the store is unchanged.
func (s *Store) Import(r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	in, err := decode(b)
	if err != nil {
		return err
	}
	old := s.data
	s.data = maps.Clone(old)
	maps.Copy(s.data, in)
	if err := s.save(); err != nil {
		s.data = old
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
