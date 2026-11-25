package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"kv-cli/internal/kvstore"
)

const usage = "usage: kv set <key> <value> | get <key> | del <key> | list | export | import <file>"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func storePath() (string, error) {
	if p := os.Getenv("KV_FILE"); p != "" {
		return p, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".kv.json"), nil
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	want := map[string]int{"set": 3, "get": 2, "del": 2, "list": 1, "export": 1, "import": 2}
	n, ok := want[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "kv: unknown command %q\n%s\n", args[0], usage)
		return 2
	}
	if len(args) != n {
		fmt.Fprintf(stderr, "kv: wrong number of arguments for %q\n%s\n", args[0], usage)
		return 2
	}
	path, err := storePath()
	if err != nil {
		fmt.Fprintln(stderr, "kv:", err)
		return 1
	}
	s, err := kvstore.Open(path)
	if err != nil {
		fmt.Fprintln(stderr, "kv:", err)
		return 1
	}
	switch args[0] {
	case "set":
		err = s.Set(args[1], args[2])
	case "get":
		v, found := s.Get(args[1])
		if !found {
			fmt.Fprintf(stderr, "kv: key %q not found\n", args[1])
			return 1
		}
		fmt.Fprintln(stdout, v)
	case "del":
		err = s.Delete(args[1])
		if errors.Is(err, kvstore.ErrNotFound) {
			fmt.Fprintf(stderr, "kv: key %q not found\n", args[1])
			return 1
		}
	case "list":
		for _, p := range s.List() {
			fmt.Fprintf(stdout, "%s=%s\n", p.Key, p.Value)
		}
	case "export":
		err = s.Export(stdout)
	case "import":
		err = importFile(s, args[1])
	}
	if err != nil {
		fmt.Fprintln(stderr, "kv:", err)
		return 1
	}
	return 0
}

func importFile(s *kvstore.Store, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return s.Import(f)
}
