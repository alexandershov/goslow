package main

import (
	"fmt"
	"github.com/speps/go-hashids"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DEFAULT_RESPONSE = `{"default": "response"}`
const MAX_DELAY = 99
const MIN_STATUS = 100
const MAX_STATUS = 599

var REDIRECT_STATUS map[int]bool = map[int]bool{301: true, 302: true}

type GoSlowServer struct {
	Config  *Config
	storage *Storage
	Hasher  *hashids.HashID
}

func NewGoSlowServer(config *Config) *GoSlowServer {
	storage, err := NewStorage(config.driver, config.dataSource)
	if err != nil {
		log.Fatal(err)
	}

	server := &GoSlowServer{Config: config, storage: storage,
		Hasher: NewHasher(config.keySalt, config.minKeyLength)}
	if config.createDefaultRules {
		if config.singleDomainUrlPath != "" {
			log.Fatal("You can't use both --single-domain-path and --create-default-rules options")

		} else {
			server.CreateDefaultRules()
		}
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
	case server.IsCreateRequest(r):
		server.HandleCreateSubdomain(w, r)
		return
	case server.IsConfigRequest(r):
		log.Printf("Got config request, key: <%s>", server.GetKey(r))
		server.CreateRuleFromRequest(server.GetKey(r), w, r)
		return
	}
	rule, found, err := server.storage.FindRuleMatching(server.GetKey(r), r)
	if err != nil {
		log.Print(err)
		http.Error(w, "Internal error. For real.", 500)
		return
	}
	if found {
		ApplyRule(rule, w)
	} else {
		http.Error(w, "No rule. For real.", 404)
	}
}

func (server *GoSlowServer) IsCreateRequest(r *http.Request) bool {
	if r.Method != "POST" {
		return false
	}

	if server.IsSingleDomain() {
		return false
	}
	return GetSubdomain(r.Host) == "create"
}

func (server *GoSlowServer) IsConfigRequest(r *http.Request) bool {
	if r.Method != "POST" {
		return false
	}
	if server.IsSingleDomain() {
		return strings.HasPrefix(r.URL.Path, server.Config.singleDomainUrlPath)

	}
	return strings.HasPrefix(GetSubdomain(r.Host), "admin-")
}

func (server *GoSlowServer) IsSingleDomain() bool {
	return server.Config.singleDomainUrlPath != ""
}

func (server *GoSlowServer) GetKey(r *http.Request) string {
	if server.IsSingleDomain() {
		return ""
	}
	if server.IsConfigRequest(r) {
		return strings.TrimPrefix(GetSubdomain(r.Host), "admin-")
	}
	return GetSubdomain(r.Host)
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
	subdomain, err := server.CreateNewSubdomain(5)

	if err == nil {
		server.CreateRuleFromRequest(subdomain, w, r)
	} else {
		io.WriteString(w, fmt.Sprintf("ERROR: %s", err))
	}
}

func (server *GoSlowServer) CreateRuleFromRequest(subdomain string, w http.ResponseWriter, r *http.Request) {
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		io.WriteString(w, fmt.Sprintf("ERROR: %s", err))
		return
	}

	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		io.WriteString(w, fmt.Sprintf("ERROR: %s", err))
		return
	}
	delay := 0
	path := server.GetConfigPath(r)
	delay, _ = strconv.Atoi(values.Get("delay"))
	err = server.storage.CreateRule(&Rule{site: subdomain, responseStatus: 200, headers: EmptyHeader(),
		path: path, method: values.Get("method"),
		responseBody: string(payload), delay: time.Duration(delay) * time.Second})
	log.Print(err)
	io.WriteString(w, fmt.Sprintf("Created domain %s\n", subdomain))
}

func (server *GoSlowServer) GetConfigPath(r *http.Request) string {
	if server.IsSingleDomain() {
		return "/" + strings.TrimPrefix(r.URL.Path, server.Config.singleDomainUrlPath)
	}
	return r.URL.Path
}

func (server *GoSlowServer) CreateNewSubdomain(maxAttempts int) (string, error) {
	for {
		subdomain := server.GenerateSubdomainName()
		err := server.storage.CreateSite(subdomain)
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
	log.Printf("sleeping for %v", rule.delay)
	time.Sleep(rule.delay)

	AddHeaders(rule.headers, w)
	w.WriteHeader(rule.responseStatus)
	io.WriteString(w, rule.responseBody)
}

func AddHeaders(header map[string]string, w http.ResponseWriter) {
	responseHeader := w.Header()
	for key, value := range header {
		responseHeader.Add(key, value)
	}
}

func (server *GoSlowServer) CreateDefaultRules() {
	server.CreateDelayRules()
	server.CreateStatusRules()
}

func (server *GoSlowServer) CreateDelayRules() {
	for delay := 0; delay <= MAX_DELAY; delay++ {
		delayHost := strconv.Itoa(delay)
		delayInSeconds := time.Duration(delay) * time.Second

		server.storage.CreateRule(&Rule{site: delayHost, headers: EmptyHeader(), delay: delayInSeconds,
			responseStatus: 200, responseBody: DEFAULT_RESPONSE,
		})
	}
}

func EmptyHeader() map[string]string {
	return make(map[string]string)
}

func (server *GoSlowServer) CreateStatusRules() {
	for status := MIN_STATUS; status <= MAX_STATUS; status++ {
		statusHost := strconv.Itoa(status)
		header := server.HeaderFor(status)
		server.storage.CreateRule(&Rule{site: statusHost, responseStatus: status,
			headers: header, responseBody: DEFAULT_RESPONSE})
	}
}

func (server *GoSlowServer) HeaderFor(status int) map[string]string {
	_, isRedirect := REDIRECT_STATUS[status]
	if isRedirect {
		// TODO: check that protocol-independent location is legal HTTP
		// TODO: header should respect current port
		host := fmt.Sprintf("//0.goslow.link")
		return map[string]string{"Location": host}
	}
	return EmptyHeader()
}

func (server *GoSlowServer) ListenAndServe() error {
	log.Printf("listening on %s", server.Config.address)
	return http.ListenAndServe(server.Config.address, server)
}
