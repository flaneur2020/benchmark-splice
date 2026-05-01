package main

import (
	"flag"
	"log"
	"net/http"

	"benchmark-splice/backend"
	"benchmark-splice/mockserver"
)

func main() {
	addr := flag.String("addr", ":8081", "listen address")
	chunkSize := flag.Int64("chunk-size", backend.DefaultChunkSize, "chunk size in bytes")
	flag.Parse()

	handler, err := mockserver.New(*chunkSize)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("mock server listening on %s, chunk_size=%d", *addr, *chunkSize)
	log.Fatal(http.ListenAndServe(*addr, handler))
}
