package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	for _, a := range [][]string{{}, {"set", "a"}, {"get"}, {"foo"}, {"get", "a", "b"}, {"list", "x"}, {"del"}, {"export", "x"}} {
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

func TestExportEmptyStore(t *testing.T) {
	p := setup(t)
	if c, o, e := do(t, "export"); c != 0 || o != "{}\n" || e != "" {
		t.Fatalf("missing: %d %q %q", c, o, e)
	}
	os.WriteFile(p, nil, 0o600)
	if c, o, e := do(t, "export"); c != 0 || o != "{}\n" || e != "" {
		t.Fatalf("empty file: %d %q %q", c, o, e)
	}
	do(t, "set", "a", "1")
	do(t, "del", "a")
	if c, o, e := do(t, "export"); c != 0 || o != "{}\n" || e != "" {
		t.Fatalf("emptied: %d %q %q", c, o, e)
	}
}

func TestExportTrickyValues(t *testing.T) {
	setup(t)
	want := map[string]string{
		"space":     "hello world",
		"quote":     `say "hi"`,
		"backslash": `C:\dir\`,
		"newline":   "line1\nline2",
		"unicode":   "台北 café ✓",
		"control":   "a\x01b\tc",
		"html":      "<a & b>",
		"empty":     "",
	}
	for k, v := range want {
		if c, _, e := do(t, "set", k, v); c != 0 {
			t.Fatalf("set %s: %d %q", k, c, e)
		}
	}
	c, o, e := do(t, "export")
	if c != 0 || e != "" {
		t.Fatalf("%d %q", c, e)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(o), &got); err != nil {
		t.Fatalf("invalid JSON %q: %v", o, err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d pairs, want %d: %q", len(got), len(want), o)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
}

func TestExportCorruptStore(t *testing.T) {
	p := setup(t)
	os.WriteFile(p, []byte("{bad"), 0o644)
	c, o, e := do(t, "export")
	if c != 1 || o != "" || !strings.HasPrefix(e, "kv: ") {
		t.Fatalf("%d %q %q", c, o, e)
	}
	if b, _ := os.ReadFile(p); string(b) != "{bad" {
		t.Fatalf("store changed: %q", b)
	}
}

func TestExportDoesNotModifyStore(t *testing.T) {
	p := setup(t)
	do(t, "set", "a", "1")
	do(t, "set", "b", "2")
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	os.Chtimes(p, old, old)
	before, _ := os.ReadFile(p)
	if c, _, e := do(t, "export"); c != 0 {
		t.Fatalf("%d %q", c, e)
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatalf("bytes changed: %q -> %q", before, after)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.ModTime().Equal(old) {
		t.Fatalf("mtime changed: %v", fi.ModTime())
	}
	es, _ := os.ReadDir(filepath.Dir(p))
	if len(es) != 1 {
		t.Fatalf("unexpected files: %v", es)
	}
}

func TestUsageListsExport(t *testing.T) {
	setup(t)
	if c, _, e := do(t); c != 2 || !strings.Contains(e, "| export") {
		t.Fatalf("%d %q", c, e)
	}
}

func TestExportManyKeysSorted(t *testing.T) {
	setup(t)
	want := "{\n"
	for i := 0; i < 20; i++ {
		want += fmt.Sprintf("  \"k%02d\": \"%d\"", i, i)
		if i < 19 {
			want += ","
		}
		want += "\n"
	}
	want += "}\n"
	for i := 19; i >= 0; i-- {
		do(t, "set", fmt.Sprintf("k%02d", i), fmt.Sprint(i))
	}
	if c, o, e := do(t, "export"); c != 0 || o != want || e != "" {
		t.Fatalf("%d %q %q", c, o, e)
	}
}

func TestExportDefaultPathHome(t *testing.T) {
	h := t.TempDir()
	t.Setenv("KV_FILE", "")
	t.Setenv("HOME", h)
	do(t, "set", "a", "1")
	if c, o, e := do(t, "export"); c != 0 || o != "{\n  \"a\": \"1\"\n}\n" || e != "" {
		t.Fatalf("%d %q %q", c, o, e)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestExportStdoutWriteError(t *testing.T) {
	setup(t)
	do(t, "set", "a", "1")
	var e bytes.Buffer
	if c := run([]string{"export"}, failWriter{}, &e); c != 1 || !strings.HasPrefix(e.String(), "kv: ") {
		t.Fatalf("%d %q", c, e.String())
	}
}
