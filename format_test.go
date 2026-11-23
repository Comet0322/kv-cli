package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixture writes a store file with the given content and mode.
func writeFixture(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// readFixture returns the bytes of a store file.
func readFixture(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAC1_VersionMissingFileIs2AndCreatesNothing(t *testing.T) {
	p := setup(t)
	c, o, e := do(t, "version")
	if c != 0 || o != "2\n" || e != "" {
		t.Fatalf("version on missing file: exit %d stdout %q stderr %q; want exit 0, stdout %q, stderr empty", c, o, e, "2\n")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("version created the store file: stat err = %v; want not-exist", err)
	}
}

func TestAC1_VersionWithArgumentIsUsageError(t *testing.T) {
	setup(t)
	c, o, e := do(t, "version", "x")
	if c != 2 || o != "" || !strings.Contains(e, usage) {
		t.Fatalf("version x: exit %d stdout %q stderr %q; want exit 2, empty stdout, stderr containing %q", c, o, e, usage)
	}
}

func TestAC2_VersionOfV1StorePrints1AndLeavesFile(t *testing.T) {
	p := setup(t)
	const orig = `{"a":"1"}`
	writeFixture(t, p, orig, 0o644)
	c, o, e := do(t, "version")
	if c != 0 || o != "1\n" || e != "" {
		t.Fatalf("version on v1 store: exit %d stdout %q stderr %q; want exit 0, stdout %q, stderr empty", c, o, e, "1\n")
	}
	if got := readFixture(t, p); got != orig {
		t.Fatalf("version changed the v1 store file: got %q, want %q", got, orig)
	}
}

func TestAC3_VersionOfV2StorePrints2AndLeavesFile(t *testing.T) {
	p := setup(t)
	const orig = `{"version":2,"data":{"a":"1"}}`
	writeFixture(t, p, orig, 0o644)
	c, o, e := do(t, "version")
	if c != 0 || o != "2\n" || e != "" {
		t.Fatalf("version on v2 store: exit %d stdout %q stderr %q; want exit 0, stdout %q, stderr empty", c, o, e, "2\n")
	}
	if got := readFixture(t, p); got != orig {
		t.Fatalf("version changed the v2 store file: got %q, want %q", got, orig)
	}
}

func TestAC4_VersionOfInvalidJSONIsRuntimeError(t *testing.T) {
	p := setup(t)
	writeFixture(t, p, `{nope`, 0o644)
	c, o, e := do(t, "version")
	if c != 1 || o != "" || e == "" {
		t.Fatalf("version on invalid JSON: exit %d stdout %q stderr %q; want exit 1, empty stdout, non-empty stderr", c, o, e)
	}
}

func TestAC5_GetOnV2StorePrintsValue(t *testing.T) {
	p := setup(t)
	writeFixture(t, p, `{"version":2,"data":{"a":"1","b":"2"}}`, 0o644)
	c, o, e := do(t, "get", "a")
	if c != 0 || o != "1\n" || e != "" {
		t.Fatalf("get a on v2 store: exit %d stdout %q stderr %q; want exit 0, stdout %q, stderr empty", c, o, e, "1\n")
	}
}

func TestAC5_ListOnV2StorePrintsSortedPairs(t *testing.T) {
	p := setup(t)
	writeFixture(t, p, `{"version":2,"data":{"b":"2","a":"1"}}`, 0o644)
	c, o, e := do(t, "list")
	if c != 0 || o != "a=1\nb=2\n" || e != "" {
		t.Fatalf("list on v2 store: exit %d stdout %q stderr %q; want exit 0, stdout %q, stderr empty", c, o, e, "a=1\nb=2\n")
	}
}

func TestAC5_NonStringV2ValueIsRuntimeError(t *testing.T) {
	rows := []struct {
		name string
		args []string
	}{
		{"AC5_get_non_string_v2_value", []string{"get", "a"}},
		{"AC5_list_non_string_v2_value", []string{"list"}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			p := setup(t)
			writeFixture(t, p, `{"version":2,"data":{"a":1}}`, 0o644)
			c, o, e := do(t, r.args...)
			if c != 1 || o != "" || e == "" {
				t.Fatalf("%v on v2 store with number value: exit %d stdout %q stderr %q; want exit 1, empty stdout, non-empty stderr", r.args, c, o, e)
			}
		})
	}
}

func TestAC6_VersionOfEmptyStoreIs1(t *testing.T) {
	p := setup(t)
	writeFixture(t, p, "", 0o644)
	c, o, e := do(t, "version")
	if c != 0 || o != "1\n" || e != "" {
		t.Fatalf("version on empty store file: exit %d stdout %q stderr %q; want exit 0, stdout %q, stderr empty", c, o, e, "1\n")
	}
}

func TestI13_GetAndListNeverCreateMissingFile(t *testing.T) {
	p := setup(t)
	dir := filepath.Dir(p)
	c, o, e := do(t, "get", "a")
	if c != 1 || o != "" || e == "" {
		t.Fatalf("get a on missing file: exit %d stdout %q stderr %q; want exit 1, empty stdout, non-empty stderr", c, o, e)
	}
	c, o, e = do(t, "list")
	if c != 0 || o != "" || e != "" {
		t.Fatalf("list on missing file: exit %d stdout %q stderr %q; want exit 0, empty stdout and stderr", c, o, e)
	}
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 0 {
		t.Fatalf("get and list left %d entries in %s; want 0", len(es), dir)
	}
}

func TestI14_GetAndListNeverWriteV1Store(t *testing.T) {
	p := setup(t)
	const orig = `{"a":"1","b":"2"}`
	writeFixture(t, p, orig, 0o644)
	if c, o, e := do(t, "get", "a"); c != 0 || o != "1\n" || e != "" {
		t.Fatalf("get a on v1 store: exit %d stdout %q stderr %q; want exit 0, stdout %q, stderr empty", c, o, e, "1\n")
	}
	if c, o, e := do(t, "list"); c != 0 || o != "a=1\nb=2\n" || e != "" {
		t.Fatalf("list on v1 store: exit %d stdout %q stderr %q; want exit 0, stdout %q, stderr empty", c, o, e, "a=1\nb=2\n")
	}
	if got := readFixture(t, p); got != orig {
		t.Fatalf("get and list changed the v1 store file: got %q, want %q", got, orig)
	}
	if _, err := os.Stat(p + ".v1.bak"); !os.IsNotExist(err) {
		t.Fatalf("get and list created a backup: stat err = %v; want not-exist", err)
	}
}

func TestI15_NonStringV1ValueIsRuntimeErrorForEveryCommand(t *testing.T) {
	rows := []struct {
		name string
		args []string
	}{
		{"I15_get", []string{"get", "a"}},
		{"I15_list", []string{"list"}},
		{"I15_set", []string{"set", "b", "2"}},
		{"I15_del", []string{"del", "a"}},
		{"I15_version", []string{"version"}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			p := setup(t)
			const orig = `{"a":1}`
			writeFixture(t, p, orig, 0o644)
			c, o, e := do(t, r.args...)
			if c != 1 || o != "" || e == "" {
				t.Fatalf("%v on v1 store with number value: exit %d stdout %q stderr %q; want exit 1, empty stdout, non-empty stderr", r.args, c, o, e)
			}
			if got := readFixture(t, p); got != orig {
				t.Fatalf("%v changed the store file: got %q, want %q", r.args, got, orig)
			}
		})
	}
}

func TestI16_EmptyStoreListIsEmpty(t *testing.T) {
	p := setup(t)
	writeFixture(t, p, "", 0o644)
	c, o, e := do(t, "list")
	if c != 0 || o != "" || e != "" {
		t.Fatalf("list on empty store file: exit %d stdout %q stderr %q; want exit 0, empty stdout and stderr", c, o, e)
	}
}
