# benchmark-splice

Go benchmark for comparing three HTTP proxy backends:

- `backend/prefetch`: downloads each 10MiB chunk into memory before writing it downstream, with 4-chunk read-ahead.
- `backend/iocopy`: streams each chunk with `io.Copy`.
- `backend/splice`: Linux-only zero-copy path using `splice(2)`.

## Servers

Start the mock chunk server:

```sh
go run ./cmd/mock-server -addr :8081
```

Start a proxy server:

```sh
go run ./cmd/proxy-server -addr :8080 -upstream http://127.0.0.1:8081/chunk -backend io_copy
```

Backends are `prefetch`, `io_copy`, and `splice`. The mock server returns one random 10MiB chunk from `/chunk`. The proxy exposes `/download` and concatenates 512 chunks by default, for a 5120MiB response.

## Benchmark

On Linux:

```sh
go test -run '^$' -bench '^BenchmarkDownload512Chunks$' -benchmem ./...
```

On this macOS workspace, run the Linux benchmark through Lima:

```sh
limactl shell docker -- bash -lc 'set -euo pipefail; GOROOT=/tmp/go1.24.5; if [ ! -x "$GOROOT/bin/go" ]; then rm -rf "$GOROOT" && mkdir -p "$GOROOT" && curl -fsSL https://go.dev/dl/go1.24.5.linux-arm64.tar.gz | tar -C "$GOROOT" --strip-components=1 -xz; fi; cd /Users/yazhou/code/benchmark-splice && PATH="$GOROOT/bin:$PATH" go test -run "^$" -bench "^BenchmarkDownload512Chunks$" -benchmem ./...'
```

Sample result from Lima on Apple Silicon:

```text
goos: linux
goarch: arm64
pkg: benchmark-splice
BenchmarkDownload512Chunks/prefetch-4         	       1	2910753460 ns/op	1844.44 MB/s	26773425792 B/op	   96779 allocs/op
BenchmarkDownload512Chunks/io_copy-4          	       1	1223050750 ns/op	4389.60 MB/s	26075960 B/op	   66561 allocs/op
BenchmarkDownload512Chunks/splice-4           	       1	 707237167 ns/op	7591.10 MB/s	 5373464 B/op	   48228 allocs/op
```
