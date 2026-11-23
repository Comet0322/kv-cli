package main

import (
	"bytes"
	"os"
	"path/filepath"
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
	for _, a := range [][]string{{}, {"set", "a"}, {"get"}, {"foo"}, {"get", "a", "b"}, {"list", "x"}, {"del"}, {"export", "x"}, {"import"}, {"import", "a", "b"}} {
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

func writeFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "in.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExportEmpty(t *testing.T) {
	setup(t)
	if c, o, e := do(t, "export"); c != 0 || o != "{}\n" || e != "" {
		t.Fatalf("%d %q %q", c, o, e)
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	setup(t)
	do(t, "set", "b", "hello world")
	do(t, "set", "a", "x=y=z")
	do(t, "set", "名前", "とうきょう ☃")
	_, want, _ := do(t, "list")
	c, out, _ := do(t, "export")
	if c != 0 || out != "{\n  \"a\": \"x=y=z\",\n  \"b\": \"hello world\",\n  \"名前\": \"とうきょう ☃\"\n}\n" {
		t.Fatalf("%d %q", c, out)
	}
	backup := writeFile(t, out)

	setup(t)
	if c, o, e := do(t, "import", backup); c != 0 || o != "" || e != "" {
		t.Fatalf("import: %d %q %q", c, o, e)
	}
	if _, got, _ := do(t, "list"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if c, again, _ := do(t, "export"); c != 0 || again != out {
		t.Fatalf("re-export %q", again)
	}
	do(t, "import", backup)
	if _, got, _ := do(t, "list"); got != want {
		t.Fatalf("second import changed store: %q", got)
	}
}

func TestImportMerges(t *testing.T) {
	setup(t)
	do(t, "set", "keep", "1")
	do(t, "set", "both", "old")
	in := writeFile(t, `{"both":"new","added":"2"}`)
	if c, _, e := do(t, "import", in); c != 0 {
		t.Fatalf("%d %q", c, e)
	}
	if _, o, _ := do(t, "list"); o != "added=2\nboth=new\nkeep=1\n" {
		t.Fatalf("%q", o)
	}
}

func TestImportEmptyInputIsNoop(t *testing.T) {
	setup(t)
	do(t, "set", "a", "1")
	for _, content := range []string{"", "null"} {
		if c, _, e := do(t, "import", writeFile(t, content)); c != 0 {
			t.Fatalf("%q: %d %q", content, c, e)
		}
	}
	if _, o, _ := do(t, "list"); o != "a=1\n" {
		t.Fatalf("%q", o)
	}
}

func TestImportInvalidLeavesStoreUntouched(t *testing.T) {
	p := setup(t)
	do(t, "set", "a", "1")
	before, _ := os.ReadFile(p)
	missing := filepath.Join(t.TempDir(), "missing.json")
	cases := map[string]string{
		"bad json":   writeFile(t, "{bad"),
		"array":      writeFile(t, `["a"]`),
		"non-string": writeFile(t, `{"a":"2","b":3}`),
		"missing":    missing,
		"directory":  t.TempDir(),
	}
	for name, f := range cases {
		c, o, e := do(t, "import", f)
		if c != 1 || o != "" || !strings.HasPrefix(e, "kv: ") || !strings.Contains(e, f) {
			t.Errorf("%s: %d %q %q", name, c, o, e)
		}
		if after, _ := os.ReadFile(p); !bytes.Equal(after, before) {
			t.Errorf("%s: store changed: %q", name, after)
		}
	}
}
