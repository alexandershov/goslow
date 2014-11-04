package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const DEFAULT_RESPONSE = `{"default": "response"}`
const MAX_DELAY = 99

type GoSlowServer struct {
	Config *Config
	Store  Store
}

func (server *GoSlowServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.Path)
	rule, found := server.Store.FindRuleFor(r)
	if found {
		time.Sleep(rule.Delay)
		http.Error(w, rule.Response, rule.ResponseCode)
	} else {
		io.WriteString(w, DEFAULT_RESPONSE)
	}
}

func (server *GoSlowServer) AddDefaultRules() {
	for delay := 0; delay <= MAX_DELAY; delay++ {
		delayHost := fmt.Sprintf("%d.%s", delay, server.Config.Host)
		delayInSeconds := time.Duration(delay) * time.Second

		server.Store.AddRule(&Rule{Host: delayHost, Delay: delayInSeconds,
			Response: DEFAULT_RESPONSE, ResponseCode: 200,
		})
	}
}

func (server *GoSlowServer) ListenAndServe() error {
	log.Printf("listening on %s", server.Config.Address)
	return http.ListenAndServe(server.Config.Address, server)
}
