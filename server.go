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
	AllowCrossDomainRequests(w, r)
	if r.Method == "OPTIONS" {
		return
	}
	rule, found := FindRule(server.Store, r)
	if found {
		log.Printf("sleeping for %v", rule.Delay)
		time.Sleep(rule.Delay)

		AddHeaders(rule.Header, w)
		w.WriteHeader(rule.ResponseStatus)
		io.WriteString(w, rule.Response)
	} else {
		io.WriteString(w, DEFAULT_RESPONSE)
	}
}

func AllowCrossDomainRequests(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Credentials", "true")
	header["Access-Control-Allow-Headers"] = r.Header["Access-Control-Request-Headers"]
}

func AddHeaders(header map[string]string, w http.ResponseWriter) {
	responseHeader := w.Header()
	for key, value := range header {
		responseHeader.Add(key, value)
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

		server.Store.AddRule(&Rule{Host: delayHost, Header: EmptyHeader(), Delay: delayInSeconds,
			ResponseStatus: 200, Response: DEFAULT_RESPONSE,
		})
	}
}

func (server *GoSlowServer) MakeFullHost(subdomain int) string {
	return fmt.Sprintf("%d.%s", subdomain, server.Config.Host)
}

func EmptyHeader() map[string]string {
	return make(map[string]string)
}

func (server *GoSlowServer) AddStatusRules() {
	for status := MIN_STATUS; status <= MAX_STATUS; status++ {
		statusHost := server.MakeFullHost(status)
		header := server.HeaderFor(status)
		server.Store.AddRule(&Rule{Host: statusHost, ResponseStatus: status,
			Header: header, Response: DEFAULT_RESPONSE})
	}
}

func (server *GoSlowServer) HeaderFor(status int) map[string]string {
	_, isRedirect := REDIRECT_STATUS[status]
	if isRedirect {
		// TODO: check that protocol-independent location is legal HTTP
		host := fmt.Sprintf("//%s", server.MakeFullHost(0))
		return map[string]string{"Location": host}
	}
	return EmptyHeader()
}

func (server *GoSlowServer) ListenAndServe() error {
	log.Printf("listening on %s", server.Config.Address)
	return http.ListenAndServe(server.Config.Address, server)
}
