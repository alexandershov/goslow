package main

import (
	"log"
)

func main() {
	config := NewConfigFromArgs()
	var store Store
	if config.Driver == "memory" {
		store = NewMemoryStore()
	} else {
		store = NewSqlStore(config.Driver, config.DataSource)
	}
	server := &GoSlowServer{Config: config, Store: store}
	if config.AddDefaultRules {
		server.AddDefaultRules()
	}

	log.Fatal(server.ListenAndServe())
}
