package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"strconv"
	"github.com/speps/go-hashids"
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
	Hasher *hashids.HashID
}

func NewGoSlowServer(config *Config) *GoSlowServer {
	store, err := NewStore(config.Driver, config.DataSource)
	if err != nil {
		log.Fatal(err)
	}
	
	server := &GoSlowServer{Config: config, Store: store,
		Hasher: NewHasher(config.KeySalt, config.MinKeyLength)}
	if config.AddDefaultRules {
		server.AddDefaultRules()
	}
	return server
}

func NewHasher(salt string, minKeyLength int) *hashids.HashID {
	hd := hashids.NewData()
	hd.Salt = salt
	hd.MinLength = minKeyLength
	hd.Alphabet = "abcdefghijklmnopqrstuvwxyz1234567890"
	return hashids.NewWithData(hd)
}

func (server *GoSlowServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.Path)
	AllowCrossDomainRequests(w, r)
	switch {
	case r.Method == "OPTIONS":
		return
	case GetSubdomain(r.Host) == "create" && r.Method == "POST":
		server.HandleCreateSubdomain(w, r)
		return
	}
	rule, found, err := FindRule(server.Store, r)
	if err != nil {
		log.Print(err)
		http.Error(w, "Internal error. For real.", 500)
		return
	}
	if found {
		ApplyRule(rule, w)
	} else {
		io.WriteString(w, DEFAULT_RESPONSE)
	}
}

// TODO: check crossbrowser compatibility
func AllowCrossDomainRequests(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Credentials", "true")
	header["Access-Control-Allow-Headers"] = r.Header["Access-Control-Request-Headers"]
}

func GetSubdomain(url string) string {
	return strings.Split(url, ".")[0]
}

func (server *GoSlowServer) HandleCreateSubdomain(w http.ResponseWriter, r *http.Request) {
	subdomain, err := server.AddNewSubdomain(5)
	if err == nil {
		payload, err := ioutil.ReadAll(r.Body)
		var delay = 0
		delay, err = strconv.Atoi(r.FormValue("delay"))
		host := server.MakeFullHost(subdomain)
		err = server.Store.AddRule(&Rule{Host: host, ResponseStatus: 200, Header: EmptyHeader(),
			Path: r.URL.Path,
			Response: string(payload), Delay: time.Duration(delay) * time.Second})
		log.Print(err)
		io.WriteString(w, fmt.Sprintf("Created domain %s\n", subdomain))
	} else {
		io.WriteString(w, fmt.Sprintf("ERROR: %s", err))
	}
}

func (server *GoSlowServer) AddNewSubdomain(maxAttempts int) (string, error) {
	for {
		subdomain := server.GenerateSubdomainName()
		err := server.Store.AddNewDomain(subdomain)
		if err == nil {
			return subdomain, nil
		}
		if maxAttempts <= 0 {
			return "", err
		}
		maxAttempts--
		time.Sleep(time.Duration(10) * time.Millisecond)
	}
}


func (server *GoSlowServer) GenerateSubdomainName() string {
	nanoseconds := time.Now().UTC().UnixNano()
	totalSeconds := int(nanoseconds / 1000000000)
	millisecondsPart := int((nanoseconds / 1000000) % 1000)
	hash, _ := server.Hasher.Encode([]int{totalSeconds, millisecondsPart})
	return hash
}


func ApplyRule(rule *Rule, w http.ResponseWriter) {
	log.Printf("sleeping for %v", rule.Delay)
	time.Sleep(rule.Delay)

	AddHeaders(rule.Header, w)
	w.WriteHeader(rule.ResponseStatus)
	io.WriteString(w, rule.Response)
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
		delayHost := server.MakeFullHost(strconv.Itoa(delay))
		delayInSeconds := time.Duration(delay) * time.Second

		server.Store.AddRule(&Rule{Host: delayHost, Header: EmptyHeader(), Delay: delayInSeconds,
			ResponseStatus: 200, Response: DEFAULT_RESPONSE,
		})
	}
}

func (server *GoSlowServer) MakeFullHost(subdomain string) string {
	return fmt.Sprintf("%s.%s", subdomain, server.Config.Host)
}

func EmptyHeader() map[string]string {
	return make(map[string]string)
}

func (server *GoSlowServer) AddStatusRules() {
	for status := MIN_STATUS; status <= MAX_STATUS; status++ {
		statusHost := server.MakeFullHost(strconv.Itoa(status))
		header := server.HeaderFor(status)
		server.Store.AddRule(&Rule{Host: statusHost, ResponseStatus: status,
			Header: header, Response: DEFAULT_RESPONSE})
	}
}

func (server *GoSlowServer) HeaderFor(status int) map[string]string {
	_, isRedirect := REDIRECT_STATUS[status]
	if isRedirect {
		// TODO: check that protocol-independent location is legal HTTP
		// TODO: header should respect current port
		host := fmt.Sprintf("//%s", server.MakeFullHost("0"))
		return map[string]string{"Location": host}
	}
	return EmptyHeader()
}

func (server *GoSlowServer) ListenAndServe() error {
	log.Printf("listening on %s", server.Config.Address)
	return http.ListenAndServe(server.Config.Address, server)
}
