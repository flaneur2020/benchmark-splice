package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"benchmark-splice/backend"
	"benchmark-splice/mockserver"
)

const throughputInterval = 1 * time.Second

func main() {
	addr := flag.String("addr", ":8081", "listen address")
	chunkSize := flag.Int64("chunk-size", backend.DefaultChunkSize, "chunk size in bytes")
	flag.Parse()

	handler, err := mockserver.New(*chunkSize)
	if err != nil {
		log.Fatal(err)
	}
	go logThroughput(handler)
	log.Printf("mock server listening on %s , chunk_size=%d", *addr, *chunkSize)
	log.Fatal(http.ListenAndServe(*addr, handler))
}

func logThroughput(handler *mockserver.Handler) {
	ticker := time.NewTicker(throughputInterval)
	defer ticker.Stop()

	var previous uint64
	for range ticker.C {
		current := handler.BytesServed()
		delta := current - previous
		previous = current
		if delta == 0 {
			continue
		}

		gibPerSecond := float64(delta) / (1024 * 1024 * 1024) / throughputInterval.Seconds()
		log.Printf("mock server IO throughput: %.6f GiBytes/s", gibPerSecond)
	}
}
