package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestAC1_ExportSortedCompactWithNewline(t *testing.T) {
	setup(t)
	do(t, "set", "b", "2")
	do(t, "set", "a", "1")
	c, o, e := do(t, "export")
	if c != 0 || e != "" {
		t.Fatalf("export: exit %d, stderr %q", c, e)
	}
	if want := "{\"a\":\"1\",\"b\":\"2\"}\n"; o != want {
		t.Fatalf("stdout = %q, want %q", o, want)
	}
}

func TestAC2_ExportRoundTripsSpecialValues(t *testing.T) {
	setup(t)
	want := map[string]string{
		"space":     "hello world",
		"quote":     `say "hi"`,
		"backslash": `C:\dir\file`,
		"newline":   "line1\nline2",
		"html":      "a<b>&c",
		"unicode":   "héllo 世界 🎉",
		"empty":     "",
		"k ey\"<&>": "key with specials",
	}
	for k, v := range want {
		if c, _, e := do(t, "set", k, v); c != 0 {
			t.Fatalf("set %q: exit %d %q", k, c, e)
		}
	}
	c, o, e := do(t, "export")
	if c != 0 || e != "" {
		t.Fatalf("export: exit %d, stderr %q", c, e)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(o), &got); err != nil {
		t.Fatalf("stdout is not a JSON object of strings: %v; stdout %q", err, o)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded export = %#v, want %#v", got, want)
	}
	if !strings.Contains(o, "a<b>&c") {
		t.Errorf("stdout does not contain <, > and & literally: %q", o)
	}
	for _, esc := range []string{`\u003c`, `\u003e`, `\u0026`} {
		if strings.Contains(o, esc) {
			t.Errorf("stdout contains escape %s: %q", esc, o)
		}
	}
}

func TestAC3_ExportEmptyStore(t *testing.T) {
	p := setup(t)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("store file should not exist yet: %v", err)
	}
	c, o, e := do(t, "export")
	if c != 0 || e != "" || o != "{}\n" {
		t.Fatalf("export: exit %d, stdout %q, stderr %q; want 0, %q, \"\"", c, o, e, "{}\n")
	}
}

func TestAC4_ExportExtraArgsIsUsageError(t *testing.T) {
	setup(t)
	do(t, "set", "a", "1")
	for _, a := range [][]string{{"export", "extra"}, {"export", "a", "b"}} {
		c, o, e := do(t, a...)
		if c != 2 || o != "" || e == "" {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want 2, empty, non-empty", a, c, o, e)
		}
	}
	// The usage text printed on any usage error mentions export.
	for _, a := range [][]string{{"export", "extra"}, {}, {"foo"}, {"get"}, {"set", "a"}, {"list", "x"}, {"del"}} {
		c, _, e := do(t, a...)
		if c != 2 {
			t.Errorf("%v: exit %d, want 2", a, c)
		}
		if !strings.Contains(e, "export") {
			t.Errorf("%v: usage text does not mention export: %q", a, e)
		}
	}
}

func TestAC5_ExportBadStoreFile(t *testing.T) {
	p := setup(t)
	if err := os.WriteFile(p, []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, o, e := do(t, "export")
	if c != 1 || o != "" || !strings.HasPrefix(e, "kv: ") {
		t.Fatalf("export: exit %d, stdout %q, stderr %q; want 1, empty, prefix \"kv: \"", c, o, e)
	}
}

// readmeUsage returns the README's "## Usage" section, up to the next "## " heading.
func readmeUsage(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i := strings.Index(s, "\n## Usage\n")
	if i < 0 {
		t.Fatal("README has no ## Usage section")
	}
	s = s[i+len("\n## Usage\n"):]
	if j := strings.Index(s, "\n## "); j >= 0 {
		s = s[:j]
	}
	return s
}

func TestAC11_ReadmeDocumentsExport(t *testing.T) {
	u := readmeUsage(t)
	// The fenced usage block lists `kv export`.
	parts := strings.SplitN(u, "```", 3)
	if len(parts) < 3 {
		t.Fatal("Usage section has no fenced block")
	}
	listed := false
	for _, l := range strings.Split(parts[1], "\n") {
		if strings.TrimSpace(l) == "kv export" {
			listed = true
		}
	}
	if !listed {
		t.Errorf("Usage block does not list `kv export`:\n%s", parts[1])
	}
	// A bullet describes it: sorted JSON object on stdout, {} for an empty store.
	var bullet string
	lines := strings.Split(u, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "- ") && strings.Contains(l, "export") {
			bullet = l
			for _, c := range lines[i+1:] {
				if strings.TrimSpace(c) == "" || strings.HasPrefix(c, "- ") {
					break
				}
				bullet += " " + strings.TrimSpace(c)
			}
			break
		}
	}
	if bullet == "" {
		t.Fatalf("Usage section has no bullet describing export:\n%s", u)
	}
	lb := strings.ToLower(bullet)
	for _, w := range []string{"json", "sort", "{}"} {
		if !strings.Contains(lb, w) {
			t.Errorf("export bullet does not mention %q: %s", w, bullet)
		}
	}
	if !strings.Contains(lb, "stdout") && !strings.Contains(lb, "standard output") {
		t.Errorf("export bullet does not mention stdout: %s", bullet)
	}
	if !strings.Contains(lb, "empty") {
		t.Errorf("export bullet does not say {} is for an empty store: %s", bullet)
	}
}
