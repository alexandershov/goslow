package main

import (
	"log"
)

func main() {
	config := NewConfigFromArgs()
	var store Store
	if config.Db == "" {
		store = NewMemoryStore()
	} else {
		store = NewSqlStore(config.Db)
	}
	server := &GoSlowServer{Config: config, Store: store}
	server.AddDefaultRules()

	log.Fatal(server.ListenAndServe())
}
