// Command shukh-server runs the Layer-2 WebSocket server: a Hub over an in-memory
// RoomStore and the real clock, wired to the HTTP handlers, with the GC sweeper
// running on the clock (L2-5/L2-9). MVP: single instance, state in memory (§12).
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/oustrix/shukh/server"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	hub := server.NewHub(server.NewMemStore(), server.NewRealClock())
	hub.StartSweeper()

	handler := server.NewServer(hub).Handler()
	log.Printf("shukh-server listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, handler))
}
