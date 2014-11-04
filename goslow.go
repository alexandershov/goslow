package main

import (
	"log"
)

func main() {
	config := NewConfigFromArgs()
	store := NewMemoryStore()
	server := &GoSlowServer{Config: config, Store: store}
	server.AddDefaultRules()

	log.Fatal(server.ListenAndServe())
}
