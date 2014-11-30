// goslow is a slow HTTP server that responds with errors.
// Visit https://github.com/alexandershov/goslow for more details.
package main

import (
	"log"
	"runtime"
)

// main starts a slow HTTP server that responds with errors.
func main() {
	// GOMAXPROCS call is ignored if NumCPU returns 1 (GOMAXPROCS(0) doesn't change anything)
	runtime.GOMAXPROCS(runtime.NumCPU() / 2)

	config := NewConfigFromArgs()
	server := NewServer(config)

	log.Fatal(server.ListenAndServe())
}
