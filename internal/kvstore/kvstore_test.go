package kvstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenMissingFileIsEmpty(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "x.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.List()) != 0 {
		t.Fatal("expected empty")
	}
}

func TestSetGetDeleteList(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.json")
	s, _ := Open(p)
	for _, kv := range [][2]string{{"b", "2"}, {"a", "1"}, {"c", "3"}} {
		if err := s.Set(kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
	}
	if v, ok := s.Get("a"); !ok || v != "1" {
		t.Fatalf("get a = %q %v", v, ok)
	}
	if _, ok := s.Get("zz"); ok {
		t.Fatal("zz should be missing")
	}
	l := s.List()
	if len(l) != 3 || l[0].Key != "a" || l[1].Key != "b" || l[2].Key != "c" {
		t.Fatalf("bad order %v", l)
	}
	if err := s.Delete("b"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("b"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	s2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s2.Get("b"); ok || len(s2.List()) != 2 {
		t.Fatal("persistence failed")
	}
}

func TestNoTempFilesLeft(t *testing.T) {
	d := t.TempDir()
	s, _ := Open(filepath.Join(d, "x.json"))
	s.Set("a", "1")
	s.Set("a", "2")
	s.Delete("a")
	es, _ := os.ReadDir(d)
	if len(es) != 1 || es[0].Name() != "x.json" {
		t.Fatalf("unexpected files: %v", es)
	}
}

func TestSetFailureCleansTemp(t *testing.T) {
	d := t.TempDir()
	// target is a directory: Rename fails, temp must be removed.
	target := filepath.Join(d, "x.json")
	os.Mkdir(target, 0o755)
	os.WriteFile(filepath.Join(target, "f"), nil, 0o644)
	s := &Store{path: target, data: map[string]string{}}
	if err := s.Set("a", "1"); err == nil {
		t.Fatal("expected error")
	}
	es, _ := os.ReadDir(d)
	if len(es) != 1 {
		t.Fatalf("temp left: %v", es)
	}
}

func TestOpenInvalidJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.json")
	os.WriteFile(p, []byte("{nope"), 0o644)
	if _, err := Open(p); err == nil {
		t.Fatal("expected error")
	}
}

func TestAC10_SetManySetsAndPersists(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.json")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set("a", "old"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("z", "keep"); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"a": "new", "b": "2", "c": "3"}
	if err := s.SetMany(want); err != nil {
		t.Fatalf("SetMany: %v", err)
	}
	want["z"] = "keep"
	for k, v := range want {
		if got, ok := s.Get(k); !ok || got != v {
			t.Errorf("Get(%q) = %q, %v; want %q", k, got, ok, v)
		}
	}
	s2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range want {
		if got, ok := s2.Get(k); !ok || got != v {
			t.Errorf("reopened Get(%q) = %q, %v; want %q", k, got, ok, v)
		}
	}
	if n := len(s2.List()); n != len(want) {
		t.Errorf("reopened store has %d pairs, want %d", n, len(want))
	}
	es, _ := os.ReadDir(filepath.Dir(p))
	if len(es) != 1 {
		t.Errorf("unexpected files: %v", es)
	}
}

func TestAC10_SetManyFailureRestoresAndCleansTemp(t *testing.T) {
	d := t.TempDir()
	// target is a directory: the save fails.
	target := filepath.Join(d, "x.json")
	os.Mkdir(target, 0o755)
	os.WriteFile(filepath.Join(target, "f"), nil, 0o644)
	s := &Store{path: target, data: map[string]string{"a": "old", "b": "keep"}}
	if err := s.SetMany(map[string]string{"a": "new", "c": "3"}); err == nil {
		t.Fatal("SetMany: expected error")
	}
	if v, ok := s.Get("a"); !ok || v != "old" {
		t.Errorf("Get(a) = %q, %v; want old", v, ok)
	}
	if v, ok := s.Get("b"); !ok || v != "keep" {
		t.Errorf("Get(b) = %q, %v; want keep", v, ok)
	}
	if v, ok := s.Get("c"); ok {
		t.Errorf("Get(c) = %q, true; want absent", v)
	}
	es, _ := os.ReadDir(d)
	if len(es) != 1 {
		t.Errorf("temp left: %v", es)
	}
}
