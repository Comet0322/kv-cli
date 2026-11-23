package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"kv-cli/internal/kvstore"
)

const usage = "usage: kv set <key> <value> | get <key> | del <key> | list | export | import"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
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

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	want := map[string]int{"set": 3, "get": 2, "del": 2, "list": 1, "export": 1, "import": 1}
	n, ok := want[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "kv: unknown command %q\n%s\n", args[0], usage)
		return 2
	}
	if len(args) != n {
		fmt.Fprintf(stderr, "kv: wrong number of arguments for %q\n%s\n", args[0], usage)
		return 2
	}
	var in map[string]string
	if args[0] == "import" {
		b, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintln(stderr, "kv:", err)
			return 1
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(b, &raw); err != nil {
			fmt.Fprintln(stderr, "kv: invalid import input:", err)
			return 1
		}
		if raw == nil {
			fmt.Fprintln(stderr, "kv: invalid import input: not a JSON object")
			return 1
		}
		in = make(map[string]string, len(raw))
		for k, v := range raw {
			var str string
			if err := json.Unmarshal(v, &str); err != nil || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
				fmt.Fprintf(stderr, "kv: invalid import input: value for key %q is not a string\n", k)
				return 1
			}
			in[k] = str
		}
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
	case "import":
		if len(in) > 0 {
			err = s.SetMany(in)
		}
	case "export":
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		m := map[string]string{}
		for _, p := range s.List() {
			m[p.Key] = p.Value
		}
		if err = enc.Encode(m); err == nil {
			stdout.Write(buf.Bytes())
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "kv:", err)
		return 1
	}
	return 0
}
