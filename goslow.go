// goslow is a slow HTTP server that responds with errors.
// Visit https://github.com/alexandershov/goslow for more details.
package main

import (
	"log"
	"runtime"
)

// main starts a server.
func main() {
	useSeveralCPU()

	config := NewConfigFromArgs()
	server := NewServer(config)

	log.Fatal(server.ListenAndServe())
}

func useSeveralCPU() {
	if runtime.NumCPU() > 1 {
		runtime.GOMAXPROCS(runtime.NumCPU() / 2)
	}
}
