package main

import (
	"log"
)

func main() {
	config := NewConfigFromArgs()
	server := NewServer(config)

	log.Fatal(server.ListenAndServe())
}
