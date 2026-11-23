package kvstore

import (
	"os"
	"path/filepath"
	"strings"
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

func TestImportFailureLeavesStoreUnchanged(t *testing.T) {
	d := t.TempDir()
	target := filepath.Join(d, "x.json")
	os.Mkdir(target, 0o755)
	os.WriteFile(filepath.Join(target, "f"), nil, 0o644)
	s := &Store{path: target, data: map[string]string{"a": "1"}}
	if err := s.Import(strings.NewReader(`{"a":"2","b":"3"}`)); err == nil {
		t.Fatal("expected error")
	}
	if l := s.List(); len(l) != 1 || l[0] != (Pair{"a", "1"}) {
		t.Fatalf("store changed: %v", l)
	}
	if es, _ := os.ReadDir(d); len(es) != 1 {
		t.Fatalf("temp left: %v", es)
	}
}
