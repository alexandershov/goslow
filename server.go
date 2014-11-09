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
  "errors"
)

const DEFAULT_RESPONSE = `{"default": "response"}`
const MAX_DELAY = 99
const MIN_STATUS = 100
const MAX_STATUS = 599

const CREATE_SUBDOMAIN_NAME = "create"
const BUG_REPORTS_EMAIL = "codumentary.com@gmail.com"

var REDIRECT_STATUSES map[int]bool = map[int]bool{301: true, 302: true}
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
		hasher: newHasher(config.siteSalt, config.minSiteLength)}
	if config.createDefaultRules {
		server.createDefaultRules()
	}
	return server
}

func newHasher(salt string, minLength int) *hashids.HashID {
	hd := hashids.NewData()
	hd.Salt = salt
	hd.MinLength = minLength
	hd.Alphabet = "abcdefghijklmnopqrstuvwxyz1234567890"
	return hashids.NewWithData(hd)
}

func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Printf("%s %s", req.Method, req.URL.Path)
	switch {
	case server.isOptions(req):
		allowCrossDomainRequests(w, req)
	case server.isCreateSite(req):
		server.createSite(w, req)
	case server.isChangeSite(req):
		server.changeSite(server.GetKey(req), req)
	default:
		allowCrossDomainRequests(w, req)
		server.respondFromRule(w, req)
	}
}

func (server *Server) isOptions(req *http.Request) bool {
	return req.Method == "OPTIONS"
}

// TODO: check crossbrowser compatibility
func allowCrossDomainRequests(w http.ResponseWriter, req *http.Request) {
	header := w.Header()
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Credentials", "true")
	header["Access-Control-Allow-Headers"] = req.Header["Access-Control-Request-Headers"]
}

func (server *Server) isCreateSite(req *http.Request) bool {
	if req.Method != "POST" {
		return false
	}

	if server.isInSingleSiteMode() {
		return false
	}
	return getSubdomain(req.Host) == CREATE_SUBDOMAIN_NAME
}

func (server *Server) isInSingleSiteMode() bool {
	return server.config.singleDomainUrlPath != ""
}

func getSubdomain(url string) string {
	return strings.Split(url, ".")[0]
}

func (server *Server) createSite(w http.ResponseWriter, req *http.Request) {
	site, err := server.generateSiteName(MAX_GENERATE_SITE_NAME_ATTEMPTS)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	server.changeSite(site, req)
}

func (server *Server) generateSiteName(maxAttempts uint) (string, error) {
  for ; maxAttempts > 0; maxAttempts-- {
    site, err := server.makeSiteNameFrom(generateUniqueNumbers())
    if err != nil {
      log.Print(err)
      break
    }
    err = server.storage.CreateSite(site)
    if err == nil {
      return site, nil
    }
    log.Print(err)
    time.Sleep(DURATION_BETWEEN_GENERATE_SITE_NAME_ATTEMPTS)
  }
  return "", errors.New(fmt.Sprintf(`Can't create site.
Try again in a few seconds or contact %s for help`, BUG_REPORTS_EMAIL))
}

func (server *Server) makeSiteNameFrom(numbers []int) (string, error) {
  return server.hasher.Encode(numbers)
}

func generateUniqueNumbers() []int {
  utc := time.Now().UTC()
  second := int(utc.Unix()) // TODO: fix me at 2037-12-31
  millisecond := utc.Nanosecond() / 1000000
  return []int{second, millisecond}
}

func (server *Server) changeSite(site string, req *http.Request) error {
  rule, err := server.makeRule(site, req)
  if err != nil {
    return err
  }
  return server.storage.UpsertRule(rule)
}

func (server *Server) makeRule(site string, req *http.Request) (*Rule, error) {
  body, err := ioutil.ReadAll(req.Body)
  if err != nil {
    return nil, err
  }

  values, err := url.ParseQuery(req.URL.RawQuery)
  if err != nil {
    return nil, err
  }
  delay := 0
  path := server.GetConfigPath(req)
  delay, _ = strconv.Atoi(values.Get("delay"))
  return &Rule{site: site, responseStatus: http.StatusOK, headers: EMPTY_HEADERS,
    path: path, method: values.Get("method"),
    responseBody: string(body), delay: time.Duration(delay) * time.Second}, nil
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

func (server *Server) isChangeSite(r *http.Request) bool {
	if r.Method != "POST" {
		return false
	}
	if server.isInSingleSiteMode() {
		return strings.HasPrefix(r.URL.Path, server.config.singleDomainUrlPath)

	}
	return strings.HasPrefix(getSubdomain(r.Host), "admin-")
}

func (server *Server) GetKey(req *http.Request) string {
	if server.isInSingleSiteMode() {
		return ""
	}
	if server.isChangeSite(req) {
		return strings.TrimPrefix(getSubdomain(req.Host), "admin-")
	}
	return getSubdomain(req.Host)
}



func (server *Server) GetConfigPath(r *http.Request) string {
	if server.isInSingleSiteMode() {
		return "/" + strings.TrimPrefix(r.URL.Path, server.config.singleDomainUrlPath)
	}
	return r.URL.Path
}



func ApplyRule(rule *Rule, w http.ResponseWriter) {
	log.Printf("sleeping for %v", rule.delay)
	time.Sleep(rule.delay)

	addHeaders(rule.headers, w)
	w.WriteHeader(rule.responseStatus)
	io.WriteString(w, rule.responseBody)
}

func addHeaders(headers map[string]string, w http.ResponseWriter) {
	responseHeader := w.Header()
	for key, value := range headers {
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
	for i := MIN_STATUS; i <= MAX_STATUS; i++ {
		statusSite := strconv.Itoa(i)
		headers := server.headersForStatus(i)
		server.storage.UpsertRule(&Rule{site: statusSite, responseStatus: i,
			headers: headers, responseBody: DEFAULT_RESPONSE})
	}
}

func (server *Server) headersForStatus(status int) map[string]string {
	_, isRedirect := REDIRECT_STATUSES[status]
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
