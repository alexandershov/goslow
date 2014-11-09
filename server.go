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
var EMPTY_HEADERS map[string]string = make(map[string]string)

const MAX_GENERATE_SITE_NAME_ATTEMPTS = 5
const DURATION_BETWEEN_GENERATE_SITE_NAME_ATTEMPTS = time.Duration(10) * time.Millisecond

type Server struct {
	config  *Config
	storage *Storage
	hasher  *hashids.HashID
}

func NewServer(config *Config) *Server {
	storage, err := NewStorage(config.driver, config.dataSource)
	if err != nil {
		log.Fatal(err)
	}

	server := &Server{config: config, storage: storage,
		hasher: NewHasher(config.siteSalt, config.minSiteLength)}
	if config.createDefaultRules {
		server.createDefaultRules()
	}
	return server
}

func NewHasher(salt string, minLength int) *hashids.HashID {
	hd := hashids.NewData()
	hd.Salt = salt
	hd.MinLength = minLength
	hd.Alphabet = "abcdefghijklmnopqrstuvwxyz1234567890"
	return hashids.NewWithData(hd)
}

func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Printf("%s %s", req.Method, req.URL.Path)
	allowCrossDomainRequests(w, req)

	switch {
	case server.isOptionsRequest(req):
		// do nothing
	case server.isCreateRequest(req):
		server.createSite(w, req)
	case server.isConfigRequest(req):
		server.createRuleFromRequest(server.GetKey(req), w, req)
	default:
		server.respondFromRule(w, req)
	}
}

func (server *Server) respondFromRule(w http.ResponseWriter, req *http.Request) {
	rule, found, err := server.storage.FindRuleMatching(server.GetKey(req), req)
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

func (server *Server) isOptionsRequest(req *http.Request) bool {
	return req.Method == "OPTIONS"
}

func (server *Server) isCreateRequest(r *http.Request) bool {
	if r.Method != "POST" {
		return false
	}

	if server.IsSingleDomain() {
		return false
	}
	return GetSubdomain(r.Host) == "create"
}

func (server *Server) isConfigRequest(r *http.Request) bool {
	if r.Method != "POST" {
		return false
	}
	if server.IsSingleDomain() {
		return strings.HasPrefix(r.URL.Path, server.config.singleDomainUrlPath)

	}
	return strings.HasPrefix(GetSubdomain(r.Host), "admin-")
}

func (server *Server) IsSingleDomain() bool {
	return server.config.singleDomainUrlPath != ""
}

func (server *Server) GetKey(r *http.Request) string {
	if server.IsSingleDomain() {
		return ""
	}
	if server.isConfigRequest(r) {
		return strings.TrimPrefix(GetSubdomain(r.Host), "admin-")
	}
	return GetSubdomain(r.Host)
}

// TODO: check crossbrowser compatibility
func allowCrossDomainRequests(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Credentials", "true")
	header["Access-Control-Allow-Headers"] = r.Header["Access-Control-Request-Headers"]
}

func GetSubdomain(url string) string {
	return strings.Split(url, ".")[0]
}

func (server *Server) createSite(w http.ResponseWriter, req *http.Request) {
	site, err := server.generateSiteName(MAX_GENERATE_SITE_NAME_ATTEMPTS)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	server.createRuleFromRequest(site, w, req)
}

func (server *Server) createRuleFromRequest(subdomain string, w http.ResponseWriter, r *http.Request) {
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
	err = server.storage.UpsertRule(&Rule{site: subdomain, responseStatus: http.StatusOK, headers: EMPTY_HEADERS,
		path: path, method: values.Get("method"),
		responseBody: string(payload), delay: time.Duration(delay) * time.Second})
	log.Print(err)
	io.WriteString(w, fmt.Sprintf("Created domain %s\n", subdomain))
}

func (server *Server) GetConfigPath(r *http.Request) string {
	if server.IsSingleDomain() {
		return "/" + strings.TrimPrefix(r.URL.Path, server.config.singleDomainUrlPath)
	}
	return r.URL.Path
}

func (server *Server) generateSiteName(maxAttempts uint) (string, error) {
	var err error
	for ; maxAttempts > 0; maxAttempts-- {
		site, err := server.generateSiteNameFrom(time.Now().UTC().UnixNano())
		if err != nil {
			break
		}
		err = server.storage.CreateSite(site)
		if err == nil {
			return site, nil
		}
		time.Sleep(DURATION_BETWEEN_GENERATE_SITE_NAME_ATTEMPTS)
	}
	return "", err
}

func (server *Server) generateSiteNameFrom(nanoseconds int64) (string, error) {
	totalSeconds := int(nanoseconds / 1000000000)
	millisecondsPart := int((nanoseconds / 1000000) % 1000)
	return server.hasher.Encode([]int{totalSeconds, millisecondsPart})
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

func (server *Server) createDefaultRules() {
	server.createDelayRules()
	server.createStatusRules()
}

func (server *Server) createDelayRules() {
	for i := 0; i <= MAX_DELAY; i++ {
		delaySite := strconv.Itoa(i)
		delay := time.Duration(i) * time.Second

		server.storage.UpsertRule(&Rule{site: delaySite, headers: EMPTY_HEADERS, delay: delay,
			responseStatus: http.StatusOK, responseBody: DEFAULT_RESPONSE,
		})
	}
}

func (server *Server) createStatusRules() {
	for status := MIN_STATUS; status <= MAX_STATUS; status++ {
		statusHost := strconv.Itoa(status)
		header := server.headerFor(status)
		server.storage.UpsertRule(&Rule{site: statusHost, responseStatus: status,
			headers: header, responseBody: DEFAULT_RESPONSE})
	}
}

func (server *Server) headerFor(status int) map[string]string {
	_, isRedirect := REDIRECT_STATUS[status]
	if isRedirect {
		// TODO: check that protocol-independent location is legal HTTP
		// TODO: header should respect current port
		host := fmt.Sprintf("//0.goslow.link")
		return map[string]string{"Location": host}
	}
	return EMPTY_HEADERS
}

func (server *Server) ListenAndServe() error {
	log.Printf("listening on %s", server.config.address)
	return http.ListenAndServe(server.config.address, server)
}
