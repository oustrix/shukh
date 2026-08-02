// Command shukh-server runs the Layer-2 WebSocket server: a Hub over an in-memory
// RoomStore and the real clock, wired to the HTTP handlers, with the GC sweeper
// running on the clock (L2-5/L2-9). MVP: single instance, state in memory (§12).
package main

import (
	"flag"
	"log"
	"net/http"
	"strings"

	"github.com/oustrix/shukh/server"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	origins := flag.String("origins", "", "comma-separated browser origins allowed to call the API and open sockets, e.g. http://localhost:5173")
	crossSite := flag.Bool("cross-site", false, "issue the reconnect cookie as SameSite=None; Secure (needed only when the SPA is on a different site; requires TLS)")
	flag.Parse()

	hub := server.NewHub(server.NewMemStore(), server.NewRealClock())
	hub.StartSweeper()

	handler := server.NewServer(hub, server.Options{Origins: parseOrigins(*origins), CrossSite: *crossSite}).Handler()
	log.Printf("shukh-server listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, handler))
}

// parseOrigins splits the -origins flag on commas, trimming surrounding spaces and
// dropping empty items: "a, b" is the natural way to type a list, and an untrimmed
// " b" would never match a browser Origin header — the allowlist would silently
// stop working for every entry but the first.
func parseOrigins(flagValue string) []string {
	var out []string
	for _, o := range strings.Split(flagValue, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}
