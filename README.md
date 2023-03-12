## github.com/sa6mwa/anystore

AnyStore is a Go thread-safe key/value store featuring optional mutex-style
encrypted persistence for shared access from one or more instance(s). The
persistence feature requires a system supporting `syscall.Flock` (Linux, BSD,
Darwin, etc).

## Persistence, not performance

The persistence-feature is not designed for performance, but for simplicity,
durability and concurrent access by concurrent processes/instances. The entire
key/value store (`map[any]any`) is loaded and persisted on retrieving or storing
every key/value pair making it slow with many keys, but can be sharded manually
by managing several AnyStores. Concurrent access relies entirely on locking a
lockfile using `syscall.Flock` (`flock(2)`). When new keys are stored, they are
saved in a temporary file which is renamed to the main encrypted GOB file. A
rename operation is atomic and survivable in case of failure. The `flock` on the
lockfile is released once the rename has completed successfully. On load, the
lock on the lockfile is not acquired - the operation relies on the atomic nature
of `rename`.

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
