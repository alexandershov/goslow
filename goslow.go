package main

import (
	"log"
)

func main() {
	config := NewConfigFromArgs()
	server := NewGoSlowServer(config)

	log.Fatal(server.ListenAndServe())
}
