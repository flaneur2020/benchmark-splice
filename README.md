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
go test -run '^$' -bench '^BenchmarkDownload512Chunks$' -benchmem . -args -upstream http://127.0.0.1:8081/chunk
```

Configure client-side concurrent downloads with `-client-concurrency` after `-args`:

```sh
go test -run '^$' -bench '^BenchmarkDownload512Chunks$' -benchmem . -args -upstream http://127.0.0.1:8081/chunk -client-concurrency=4
```

Sample result:

```text
# go test -run '^$' -bench '^BenchmarkDownload512Chunks$' -benchmem . -args -upstream http://10.20.33.2:8081/chunk -client-concurrency=16
goos: linux
goarch: amd64
pkg: benchmark-splice
cpu: AMD EPYC 7542 32-Core Processor
BenchmarkDownload512Chunks/prefetch/client_concurrency_16-128         	       1	30780339015 ns/op	2790.72 MB/s	192402831032 B/op	 1033966 allocs/op
BenchmarkDownload512Chunks/io_copy/client_concurrency_16-128          	       1	29557244038 ns/op	2906.20 MB/s	482459616 B/op	  757211 allocs/op
BenchmarkDownload512Chunks/splice/client_concurrency_16-128           	       1	29290101045 ns/op	2932.71 MB/s	57903696 B/op	  450179 allocs/op
PASS
ok  	benchmark-splice	89.684s
```
