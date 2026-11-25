package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func do(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var o, e bytes.Buffer
	c := run(args, &o, &e)
	return c, o.String(), e.String()
}

func setup(t *testing.T) string {
	p := filepath.Join(t.TempDir(), "kv.json")
	t.Setenv("KV_FILE", p)
	return p
}

func TestSetGet(t *testing.T) {
	setup(t)
	if c, o, e := do(t, "set", "a", "1"); c != 0 || o != "" || e != "" {
		t.Fatalf("set: %d %q %q", c, o, e)
	}
	if c, o, _ := do(t, "get", "a"); c != 0 || o != "1\n" {
		t.Fatalf("get: %d %q", c, o)
	}
}

func TestGetMissing(t *testing.T) {
	setup(t)
	c, o, e := do(t, "get", "missing")
	if c != 1 || o != "" || e == "" {
		t.Fatalf("%d %q %q", c, o, e)
	}
}

func TestDel(t *testing.T) {
	setup(t)
	do(t, "set", "a", "1")
	if c, _, _ := do(t, "del", "a"); c != 0 {
		t.Fatal(c)
	}
	if c, o, e := do(t, "del", "a"); c != 1 || o != "" || e == "" {
		t.Fatalf("%d %q %q", c, o, e)
	}
}

func TestList(t *testing.T) {
	setup(t)
	if c, o, e := do(t, "list"); c != 0 || o != "" || e != "" {
		t.Fatalf("empty: %d %q %q", c, o, e)
	}
	do(t, "set", "b", "2")
	do(t, "set", "a", "1")
	if c, o, _ := do(t, "list"); c != 0 || o != "a=1\nb=2\n" {
		t.Fatalf("%d %q", c, o)
	}
}

func TestUsageErrors(t *testing.T) {
	setup(t)
	for _, a := range [][]string{{}, {"set", "a"}, {"get"}, {"foo"}, {"get", "a", "b"}, {"list", "x"}, {"del"}} {
		c, o, e := do(t, a...)
		if c != 2 || o != "" || e == "" {
			t.Errorf("%v: %d %q %q", a, c, o, e)
		}
	}
}

func TestBadJSON(t *testing.T) {
	p := setup(t)
	os.WriteFile(p, []byte("{bad"), 0o644)
	if c, o, e := do(t, "list"); c != 1 || o != "" || e == "" {
		t.Fatalf("%d %q %q", c, o, e)
	}
}

func TestDefaultPathHome(t *testing.T) {
	h := t.TempDir()
	t.Setenv("KV_FILE", "")
	t.Setenv("HOME", h)
	if c, _, e := do(t, "set", "a", "1"); c != 0 {
		t.Fatalf("%d %s", c, e)
	}
	if _, err := os.Stat(filepath.Join(h, ".kv.json")); err != nil {
		t.Fatal(err)
	}
}

func TestExportSortedIndented(t *testing.T) {
	setup(t)
	do(t, "set", "b", "2")
	do(t, "set", "a", "1")
	c, o, e := do(t, "export")
	if c != 0 || o != "{\n  \"a\": \"1\",\n  \"b\": \"2\"\n}\n" || e != "" {
		t.Fatalf("%d %q %q", c, o, e)
	}
}
