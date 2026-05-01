package main

import (
	"flag"
	"log"
	"net/http"

	"benchmark-splice/backend"
	"benchmark-splice/backend/iocopy"
	"benchmark-splice/backend/prefetch"
	"benchmark-splice/backend/splice"
	"benchmark-splice/proxy"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	upstream := flag.String("upstream", "http://127.0.0.1:8081/chunk", "mock chunk URL")
	name := flag.String("backend", "io_copy", "backend: prefetch, io_copy, splice")
	chunkSize := flag.Int64("chunk-size", backend.DefaultChunkSize, "chunk size in bytes")
	chunks := flag.Int("chunks", backend.DefaultChunks, "default chunk count")
	readAhead := flag.Int("readahead", backend.DefaultReadAhead, "prefetch read-ahead chunk count")
	flag.Parse()

	var h backend.Handler
	switch *name {
	case "prefetch":
		h = prefetch.New(*readAhead)
	case "io_copy":
		h = iocopy.New()
	case "splice":
		h = splice.New()
	default:
		log.Fatalf("unknown backend %q", *name)
	}

	handler := proxy.New(*upstream, h, proxy.WithChunkSize(*chunkSize), proxy.WithChunks(*chunks))
	log.Printf("proxy listening on %s, backend=%s, upstream=%s", *addr, h.Name(), *upstream)
	log.Fatal(http.ListenAndServe(*addr, handler))
}
