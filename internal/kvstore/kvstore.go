// Package kvstore is a tiny string key-value store persisted as a JSON object.
package kvstore

import (
	"encoding/json"
	"errors"
	"fmt"
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
	path   string
	data   map[string]string
	format int // 1 for a bare-object (v1) file, 2 for the envelope; zero means no migration
}

// Format reports the format of the store file: 1 for a bare-object file, otherwise 2.
func (s *Store) Format() int {
	if s.format == 1 {
		return 1
	}
	return 2
}

// Open loads the store at path; a missing file is an empty store.
//
// Format detection: an empty file is a v1 store. A file whose top-level
// "version" is a JSON number is a v2 envelope {"version":2,"data":{...}}.
// Anything else must be a v1 bare object of string values.
func Open(path string) (*Store, error) {
	s := &Store{path: path, data: map[string]string{}, format: 2}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		s.format = 1
		return s, nil
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if top == nil {
		return nil, fmt.Errorf("parse %s: top-level value is not an object", path)
	}
	if raw, ok := top["version"]; ok && isJSONNumber(raw) {
		return openV2(s, top)
	}
	s.format = 1
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return s, nil
}

// openV2 fills s from a v2 envelope. top is the parsed top-level object.
func openV2(s *Store, top map[string]json.RawMessage) (*Store, error) {
	data, hasData := top["data"]
	if len(top) != 2 || !hasData {
		return nil, fmt.Errorf("parse %s: v2 store must hold exactly version and data", s.path)
	}
	var version float64
	if err := json.Unmarshal(top["version"], &version); err != nil || version != 2 {
		return nil, fmt.Errorf("parse %s: unsupported format version %s", s.path, top["version"])
	}
	var d map[string]string
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.path, err)
	}
	if d == nil {
		return nil, fmt.Errorf("parse %s: data is not an object", s.path)
	}
	s.data = d
	s.format = 2
	return s, nil
}

// isJSONNumber reports whether raw is a JSON number literal.
func isJSONNumber(raw json.RawMessage) bool {
	return len(raw) > 0 && (raw[0] == '-' || (raw[0] >= '0' && raw[0] <= '9'))
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
