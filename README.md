## github.com/sa6mwa/anystore

AnyStore is a Go thread-safe key/value store featuring optional mutex-style
encrypted persistence for shared access from one or more instance(s). The
persistence feature requires a system (file and operating system) supporting
`syscall.Flock` (Linux, BSD, Darwin, NFSv4, etc).

AnyStore also feature a configuration mode with convenience-functions `Stash`,
`Unstash` and `EditThing`. Whether you choose to hard-code an encryption key in
the application or provide one via environment variables, using `Stash`,
`Unstash` and `EditThing` is simple...

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/sa6mwa/anystore"
)

// All fields need to be exported.
type MyConfig struct {
	ListenAddress string
	Username      string
	Token         string
	Endpoints     []*Endpoint
}

type Endpoint struct {
	ID   int
	Name string
	URL  string
}

func main() {
	defaultConf := &MyConfig{
		ListenAddress: "0.0.0.0:1234",
		Username:      "superuser",
		Token:         "abc123",
		Endpoints: []*Endpoint{
			{ID: 1, Name: "Endpoint 1", URL: "https://endpoint1.local"},
			{ID: 2, Name: "Endpoint 2", URL: "https://endpoint2.local"},
		},
	}
	file := "~/.myconfigfile.db"

	var configuration MyConfig

	if err := anystore.Unstash(&anystore.StashConfig{
		File:          file,
		EncryptionKey: anystore.DefaultEncryptionKey,
		Key:           "configuration",
		Thing:         &configuration,
	}, defaultConf); err != nil {
		log.Fatal(err)
	}

	if len(os.Args) > 1 && os.Args[1] == "edit" {
		if err := anystore.EditThing(&anystore.StashConfig{
			File:          file,
			EncryptionKey: anystore.DefaultEncryptionKey,
			Key:           "configuration",
			Thing:         &configuration,
			// Editor: "/usr/bin/emacs",
		}, defaultConf); err != nil {
			log.Fatal(err)
		}
	}

	j, err := json.MarshalIndent(configuration, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(j))
}
```

The `Stash` and `Unstash` functions also support `io.Reader` and `io.Writer`
(`io.WriteCloser`). `Stash` will write to both a file and `io.Writer` if
configured with both via the `anystore.StashConfig` struct. `Unstash` will
prefer `io.Reader` over file if both are provided. `Unstash` will successfully
unstash from the `os.File` `io.Reader` of
`os.Open(previously_stashed_file_by_Stash)`.

Example with only reader and writer...

```go
greeting := "Hello world"
var receivedGreeting string

reader, writer := io.Pipe()
defer reader.Close() // Stash closes the writer, it's an io.ReadCloser

errch := make(chan error)

go func() {
  defer close(errch)
  if err := anystore.Unstash(&anystore.StashConfig{
    Reader: reader,
    Key:    "secret",
    Thing:  &receivedGreeting,
  }, nil); err != nil {
    errch <- err
  }
  errch <- nil
}()

if err := anystore.Stash(&anystore.StashConfig{
  Writer: writer,
  Key:    "secret",
  Thing:  &greeting,
}); err != nil {
  log.Fatal(err)
}

err := <-errch
if err != nil {
  log.Fatal(err)
}

fmt.Println(receivedGreeting)
```



## Encrypted by default

There is a default encryption key constant (`anystore.DefaultEncryptionKey`)
that will be used if no user-defined key is provided. It is obviously not secure
to use the default asymmetric key as it is publicly known. You can generate your
own random AES-256 base64-encoded encryption key using `./cmd/newkey`...

```sh
go run github.com/sa6mwa/anystore/cmd/newkey
```

## Persistence, not performance

The persistence-feature is not designed for performance, but for simplicity,
durability, and concurrent access by multiple processes/instances. The entire
key/value store (`map[any]any`) is loaded and persisted on retrieving or storing
every key/value pair making it slow with many keys (can be sharded manually
by managing several AnyStores).

Concurrent access relies entirely on locking a lockfile using `syscall.Flock`
(`flock(2)`). When new keys are stored, they are saved in a temporary file which
is renamed to the main encrypted GOB file. A rename operation is atomic and
survivable in case of failure. The `flock` on the lockfile is released when the
rename has completed successfully. On load, the lock on the lockfile is not
acquired - the operation relies on the atomic nature of `rename`.

```
$ go test -bench=. -run='^#' -count=5
goos: linux
goarch: amd64
pkg: github.com/sa6mwa/anystore
cpu: AMD A8-3870 APU with Radeon(tm) HD Graphics
BenchmarkStoreAndLoadPersistence-4            28          40328319 ns/op
BenchmarkStoreAndLoadPersistence-4            30          41765093 ns/op
BenchmarkStoreAndLoadPersistence-4            30          40365892 ns/op
BenchmarkStoreAndLoadPersistence-4            30          42452397 ns/op
BenchmarkStoreAndLoadPersistence-4            33          40912477 ns/op
BenchmarkStoreAndLoad-4                   386727              3453 ns/op
BenchmarkStoreAndLoad-4                   374486              3328 ns/op
BenchmarkStoreAndLoad-4                   348756              3503 ns/op
BenchmarkStoreAndLoad-4                   405081              3678 ns/op
BenchmarkStoreAndLoad-4                   331292              3680 ns/op
PASS
ok      github.com/sa6mwa/anystore      17.826s
```

## Fuzzing

Run the fuzz test with `-parallel=1` or it will fail unexpectedly for unknown
reasons. Running the tests again with `go test -v` automatically re-trying the
failed fuzz testdata has so far never failed.

```
$ go test -v -fuzz=. -run='^#' -parallel=1
=== RUN   FuzzConcurrentPersistence
fuzz: elapsed: 0s, gathering baseline coverage: 0/8 completed
fuzz: elapsed: 3s, gathering baseline coverage: 2/8 completed
fuzz: elapsed: 6s, gathering baseline coverage: 2/8 completed
fuzz: elapsed: 9s, gathering baseline coverage: 2/8 completed
fuzz: elapsed: 12s, gathering baseline coverage: 4/8 completed
fuzz: elapsed: 15s, gathering baseline coverage: 4/8 completed
fuzz: elapsed: 18s, gathering baseline coverage: 4/8 completed
fuzz: elapsed: 21s, gathering baseline coverage: 7/8 completed
fuzz: elapsed: 21s, gathering baseline coverage: 8/8 completed, now fuzzing with 1 workers
fuzz: elapsed: 24s, execs: 12 (2/sec), new interesting: 0 (total: 8)
fuzz: elapsed: 27s, execs: 12 (0/sec), new interesting: 0 (total: 8)
fuzz: elapsed: 30s, execs: 12 (0/sec), new interesting: 0 (total: 8)
fuzz: elapsed: 33s, execs: 12 (0/sec), new interesting: 0 (total: 8)
fuzz: elapsed: 36s, execs: 12 (0/sec), new interesting: 0 (total: 8)
```
