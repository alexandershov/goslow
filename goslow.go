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
		store = NewSqlStore(config.Db, config.DbConn)
	}
	server := &GoSlowServer{Config: config, Store: store}
	if config.AddDefaultRules {
		server.AddDefaultRules()
	}

	log.Fatal(server.ListenAndServe())
}
