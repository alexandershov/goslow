// goslow is a slow HTTP server that responds with errors.
// Visit https://github.com/alexandershov/goslow for more details.
package main

import (
	"log"
)

// main starts a slow HTTP server that responds with errors.
func main() {
	config := NewConfigFromArgs()
	server := NewServer(config)

	log.Fatal(server.ListenAndServe())
}
