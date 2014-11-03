package main

import (
	"log"
)

func main() {
	config := NewConfigFromArgs()
	log.Printf("config: %+v", config)
}
