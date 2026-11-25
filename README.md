# kv-cli

`kv` is a small command-line key-value store. Keys and values are strings, and the data is persisted in a single JSON file, so it survives between invocations without a daemon or a database.

## Requirements

- Go 1.22 or newer (see `go.mod`)

## Build and install

Build a binary named `kv` in the current directory:

```sh
go build -o kv .
./kv list
```

Install into `$GOBIN` (or `$HOME/go/bin` by default):

```sh
go install .
```

Note that `go install .` names the binary after the module, so it is installed as `kv-cli`, not `kv`. If you want `kv` on your `PATH`, use `go build -o kv .` and move the file, or create a symlink.

## Usage

```
kv set <key> <value>
kv get <key>
kv del <key>
kv list
kv export
```

- `set` creates the key or overwrites its value.
- `get` prints the value followed by a newline.
- `del` removes the key.
- `list` prints every pair as `key=value`, sorted by key.
- `export` prints the whole store to stdout as a JSON object in the [data format](#data-format), with keys sorted, two-space indentation and a trailing newline. An empty or missing store prints `{}`. It never modifies the store file.

### Examples

These were run against a fresh store; the output shown is what the commands print.

```sh
$ export KV_FILE=/tmp/demo.json
$ kv set name alice
$ kv set city Taipei
$ kv set count 42
$ kv get name
alice
$ kv list
city=Taipei
count=42
name=alice
$ kv del city
$ kv get city
kv: key "city" not found
$ echo $?
1
```

Values may contain spaces when quoted:

```sh
$ kv set greeting "hello world"
$ kv get greeting
hello world
```

Back up the store with `export`. The output is sorted and has one pair per line, so two backups of the same data are byte-identical and diff cleanly:

```sh
$ kv export
{
  "count": "42",
  "greeting": "hello world",
  "name": "alice"
}
$ kv export > backup.json
```

## Store location

The store is a JSON file. Its path is chosen as follows:

1. If the `KV_FILE` environment variable is set and non-empty, that path is used.
2. Otherwise the file is `.kv.json` in your home directory (`$HOME/.kv.json`).

A missing file is treated as an empty store, so the first `set` creates it. The file is created with mode `0600`, and the directory that contains it must already exist.

## Exit codes

| Code | Meaning |
|------|---------|
| `0`  | Success. |
| `1`  | Runtime error: the key was not found (`get` or `del`), the store file could not be read or parsed, or it could not be written. |
| `2`  | Usage error: no command, an unknown command, or the wrong number of arguments (for example `kv export x`). |

Error messages go to stderr; `get`, `list` and `export` write results to stdout. `export` exits `1` with nothing on stdout if the store file cannot be read or parsed.

## Data format

The store file is a single JSON object mapping string keys to string values:

```json
{"count":"42","name":"alice"}
```

- Keys and values must be strings. Numbers such as `42` are stored as the string `"42"`.
- An empty file is accepted and treated as an empty store.
- `kv export` prints this same format, indented and with keys sorted:

  ```json
  {
    "count": "42",
    "name": "alice"
  }
  ```

- A file that is not a valid JSON object is an error for every command, including `list` and `export`. The CLI does not modify it, so you can fix or delete it by hand.

## Atomic writes

Every `set` and `del` rewrites the whole file safely:

1. The new contents are written to a temporary file created in the same directory as the store (named `.kv-*.tmp`).
2. The temporary file is renamed over the store file.

Because the rename happens within one directory, readers see either the old file or the new one, never a partial write. If a write fails, the temporary file is removed and the in-memory change is rolled back, so no stray `.kv-*.tmp` files are left behind.

## Running the tests

```sh
go test ./...
```

The tests use temporary directories and set `KV_FILE` or `HOME` themselves, so they do not touch your real store.
