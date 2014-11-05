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
const MIN_STATUS = 100
const MAX_STATUS = 599

var REDIRECT_STATUS map[int]bool = map[int]bool{301: true, 302: true}

type GoSlowServer struct {
	Config *Config
	Store  Store
}

func (server *GoSlowServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.Path)
	rule, found := server.Store.FindRuleFor(r)
	if found {
		log.Printf("sleeping for %v", rule.Delay)
		time.Sleep(rule.Delay)

		AddHeaders(rule.Header, w.Header())
		w.WriteHeader(rule.ResponseStatus)
		io.WriteString(w, rule.Response)
	} else {
		io.WriteString(w, DEFAULT_RESPONSE)
	}
}

func AddHeaders(src http.Header, dst http.Header) {
	if src == nil {
		return
	}
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func (server *GoSlowServer) AddDefaultRules() {
	server.AddDelayRules()
	server.AddStatusRules()
}

func (server *GoSlowServer) AddDelayRules() {
	for delay := 0; delay <= MAX_DELAY; delay++ {
		delayHost := server.MakeFullHost(delay)
		delayInSeconds := time.Duration(delay) * time.Second

		server.Store.AddRule(&Rule{Host: delayHost, Delay: delayInSeconds,
			ResponseStatus: 200, Response: DEFAULT_RESPONSE,
		})
	}
}

func (server *GoSlowServer) MakeFullHost(subdomain int) string {
	return fmt.Sprintf("%d.%s", subdomain, server.Config.Host)
}

func (server *GoSlowServer) AddStatusRules() {
	for status := MIN_STATUS; status <= MAX_STATUS; status++ {
		statusHost := server.MakeFullHost(status)
		header := server.HeaderFor(status)
		server.Store.AddRule(&Rule{Host: statusHost, ResponseStatus: status,
			Header: header, Response: DEFAULT_RESPONSE})
	}
}

func (server *GoSlowServer) HeaderFor(status int) http.Header {
	_, isRedirect := REDIRECT_STATUS[status]
	if isRedirect {
		// TODO: check that protocol-independent location is legal HTTP
		host := fmt.Sprintf("//%s", server.MakeFullHost(0))
		return http.Header{"Location": []string{host}}
	}
	return nil
}

func (server *GoSlowServer) ListenAndServe() error {
	log.Printf("listening on %s", server.Config.Address)
	return http.ListenAndServe(server.Config.Address, server)
}
