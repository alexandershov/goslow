package main

// TODO: think about using panic/recover for most errors
// TODO: think about replacing isInSingleSiteMode with subclass

import (
	"errors"
	"fmt"
	"github.com/speps/go-hashids"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var DEFAULT_RESPONSE = []byte(`{"goslow": "response"}`)

const (
	MAX_DELAY       = 99
	MIN_STATUS_CODE = 100
	MAX_STATUS_CODE = 599
	ZERO_DELAY_SITE = "0"
	EMPTY_SITE      = ""
	MAX_BODY_SIZE   = 1024 * 1024
)

const (
	CREATE_SUBDOMAIN_NAME     = "create"
	ADD_RULE_SUBDOMAIN_PREFIX = "admin-"
)

const BUG_REPORTS_EMAIL = "codumentary.com@gmail.com"

var REDIRECT_STATUSES = map[int]bool{301: true, 302: true}
var EMPTY_HEADERS = map[string]string{}

const (
	MAX_GENERATE_SITE_NAME_ATTEMPTS = 5
	GOSLOW_EPOCH_START              = 1415975661 // TODO: update it before launch
)

const (
	DELAY_PARAM       = "delay"
	STATUS_CODE_PARAM = "status"
)

type Server struct {
	config  *Config
	storage *Storage
	hasher  *hashids.HashID
}

type TemplateData struct {
	*Rule
	Domain     string
	StringBody string
}

type ApiError struct {
	Message    string
	StatusCode int
}

func (error *ApiError) Error() string {
	return error.Message
}

func NewApiError(message string, statusCode int) *ApiError {
	return &ApiError{Message: message, StatusCode: statusCode}
}

func NewServer(config *Config) *Server {
	storage, err := NewStorage(config.driver, config.dataSource)
	if err != nil {
		log.Fatal(err)
	}

	server := &Server{config: config, storage: storage,
		hasher: newHasher(config.siteSalt, config.minSiteLength),
	}
	if config.createDefaultRules {
		server.createDefaultRules()
	}
	if server.isInSingleSiteMode() {
		server.ensureEmptySiteExists()
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
	var err error = nil
	switch {
	case server.isOptions(req):
		allowCrossDomainRequests(w, req)

	case server.isCreateSite(req):
		err = server.handleCreateSite(w, req)

	case server.isAddRule(req):
		err = server.handleAddRule(w, req)

	default:
		allowCrossDomainRequests(w, req)
		err = server.respondFromRule(w, req)
	}
	if err != nil {
		server.handleError(err, w)
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

func (server *Server) handleCreateSite(w http.ResponseWriter, req *http.Request) error {
	site, err := server.generateUniqueSiteName(MAX_GENERATE_SITE_NAME_ATTEMPTS)
	if err != nil {
		return err
	}
	// TODO: think about add/create naming dualism
	rule, err := server.addRule(site, req)
	if err != nil {
		return err
	}
	if isShortOutput(req) {
		fmt.Fprint(w, server.makeFullDomain(rule.Site))
	} else {
		BANNER_TEMPLATE.Execute(w, nil)
		ADD_RULE_TEMPLATE.Execute(w, server.makeTemplateData(rule))
		io.WriteString(w, "\n")
		CREATE_SITE_TEMPLATE.Execute(w, server.makeTemplateData(rule))
		io.WriteString(w, "\n")
	}
	return nil
}

func (server *Server) generateUniqueSiteName(numAttempts uint) (string, error) {
	for ; numAttempts > 0; numAttempts-- {
		site, err := server.makeSiteNameFrom(generateUniqueNumbers())
		if err != nil {
			break
		}
		err = server.storage.CreateSite(site)
		if err == nil {
			return site, nil
		}
		time.Sleep(getRandomDuration(10, 20))
	}
	return "", errors.New(fmt.Sprintf(`Can't create.
Try again in a few seconds or contact %s for help`, BUG_REPORTS_EMAIL))
}

func (server *Server) makeSiteNameFrom(numbers []int) (string, error) {
	return server.hasher.Encode(numbers)
}

func generateUniqueNumbers() []int {
	utc := time.Now().UTC()
	seconds := int(utc.Unix()) - GOSLOW_EPOCH_START // TODO: fix me at 2037-12-31
	milliseconds := (utc.Nanosecond() / 1000000) % 1000
	return []int{seconds, milliseconds}
}

func (server *Server) addRule(site string, req *http.Request) (*Rule, error) {
	rule, err := server.makeRule(site, req)
	if err != nil {
		return nil, err
	}
	return rule, server.storage.UpsertRule(rule)
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
	path := server.getRulePath(req)
	delay, err := getRuleDelay(values)
	if err != nil {
		return nil, err
	}
	statusCode, err := getRuleStatusCode(values)
	if err != nil {
		return nil, err
	}
	rule := &Rule{Site: site, StatusCode: statusCode, Headers: EMPTY_HEADERS,
		Path: path, Method: values.Get("method"),
		Body: body, Delay: delay}
	return rule, nil
}

// TODO: return human readable error, not default strconv.ParseFload error
// look at different places where you can supply good error message
func getRuleDelay(values url.Values) (time.Duration, error) {
	_, contains := values[DELAY_PARAM]
	if !contains {
		return time.Duration(0), nil
	}
	delayInSeconds, err := strconv.ParseFloat(values.Get(DELAY_PARAM), 64)
	if err != nil {
		return time.Duration(0), err
	}
	return time.Duration(delayInSeconds*1000) * time.Millisecond, nil
}

func getRuleStatusCode(values url.Values) (int, error) {
	_, contains := values[STATUS_CODE_PARAM]
	if !contains {
		return http.StatusOK, nil
	}
	return strconv.Atoi(values.Get(STATUS_CODE_PARAM))
}

func getRandomDuration(minMilliseconds, maxMilliseconds int) time.Duration {
	milliseconds := minMilliseconds + rand.Intn(maxMilliseconds-minMilliseconds+1)
	return time.Duration(milliseconds) * time.Millisecond
}

func (server *Server) handleError(err error, w http.ResponseWriter) {
	log.Print(err)
	apiError, isApiError := err.(*ApiError)
	if isApiError {
		http.Error(w, apiError.Error(), apiError.StatusCode)
	} else {
		http.Error(w, fmt.Sprintf("Internal error: %s. For real", err), http.StatusInternalServerError)
	}
}

func isShortOutput(req *http.Request) bool {
	values, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		return false
	}
	return values.Get("output") == "short"
}

func (server *Server) makeTemplateData(rule *Rule) *TemplateData {
	return &TemplateData{Rule: rule, Domain: server.makeFullDomain(rule.Site),
		StringBody: truncate(string(rule.Body), 80)}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	ellipsis := "..."
	return s[:maxLen-len(ellipsis)] + ellipsis
}

func (server *Server) makeFullDomain(site string) string {
	if server.isInSingleSiteMode() {
		return server.config.endpoint
	}
	return fmt.Sprintf("%s.%s", site, server.config.endpoint)
}

func (server *Server) respondFromRule(w http.ResponseWriter, req *http.Request) error {
	rule, found, err := server.storage.FindRuleMatching(server.getSite(req), req)
	if err != nil {
		return err
	}
	if found {
		applyRule(rule, w)
	} else {
		http.Error(w, "No rule. For real.", http.StatusNotFound)
	}
	return nil
}

func (server *Server) isAddRule(req *http.Request) bool {
	if req.Method != "POST" {
		return false
	}
	if server.isInSingleSiteMode() {
		return server.isAddRulePath(req.URL.Path)
	}
	return strings.HasPrefix(getSubdomain(req.Host), ADD_RULE_SUBDOMAIN_PREFIX)
}

// TODO: rewrite
func (server *Server) isAddRulePath(path string) bool {
	addRulePath := server.config.singleDomainUrlPath
	if !strings.HasPrefix(path, addRulePath) {
		return false
	}
	if strings.HasSuffix(addRulePath, "/") {
		return true
	}
	suffix := strings.TrimPrefix(path, addRulePath)
	return suffix == "" || suffix[0] == '?' || suffix[0] == '/'
}

func (server *Server) handleAddRule(w http.ResponseWriter, req *http.Request) error {
	site := server.getSite(req)
	if isBuiltinSite(site) {
		return NewApiError(fmt.Sprintf("Sorry, you can't change builtin sites"), http.StatusForbidden)
	}
	rule, err := server.addRule(site, req)
	if err != nil {
		return err
	}
	BANNER_TEMPLATE.Execute(w, nil)
	ADD_RULE_TEMPLATE.Execute(w, server.makeTemplateData(rule))
	return nil
}

func (server *Server) getSite(req *http.Request) string {
	if server.isInSingleSiteMode() {
		return ""
	}
	subdomain := getSubdomain(req.Host)
	if server.isAddRule(req) {
		return strings.TrimPrefix(subdomain, ADD_RULE_SUBDOMAIN_PREFIX)
	}
	return subdomain
}

func isBuiltinSite(site string) bool {
	return site == CREATE_SUBDOMAIN_NAME || isDefaultRuleSite(site)
}

func isDefaultRuleSite(site string) bool {
	i, err := strconv.Atoi(site)
	if err != nil {
		return false
	}
	return i <= MAX_STATUS_CODE
}

func (server *Server) getRulePath(req *http.Request) string {
	if server.isInSingleSiteMode() {
		path := strings.TrimPrefix(req.URL.Path, server.config.singleDomainUrlPath)
		return ensureHasPrefix(path, "/")
	}
	return req.URL.Path
}

func ensureHasPrefix(s, prefix string) string {
	if !strings.HasPrefix(s, prefix) {
		return prefix + s
	}
	return s
}

func applyRule(rule *Rule, w http.ResponseWriter) {
	time.Sleep(rule.Delay)

	addHeaders(rule.Headers, w)
	w.WriteHeader(rule.StatusCode)
	w.Write(rule.Body)
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
		err := server.storage.CreateSite(delaySite)
		if err != nil {
			log.Fatal(err)
		}
		err = server.storage.UpsertRule(&Rule{Site: delaySite, Headers: EMPTY_HEADERS, Delay: delay,
			StatusCode: http.StatusOK, Body: DEFAULT_RESPONSE,
		})
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (server *Server) createStatusRules() {
	for i := MIN_STATUS_CODE; i <= MAX_STATUS_CODE; i++ {
		statusSite := strconv.Itoa(i)
		headers := server.headersForStatus(i)
		err := server.storage.CreateSite(statusSite)
		if err != nil {
			log.Fatal(err)
		}
		err = server.storage.UpsertRule(&Rule{Site: statusSite, StatusCode: i,
			Headers: headers, Body: DEFAULT_RESPONSE})
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (server *Server) ensureEmptySiteExists() {
	contains, err := server.storage.ContainsSite(EMPTY_SITE)
	if err != nil {
		log.Fatal(err)
	}
	if !contains {
		err = server.storage.CreateSite(EMPTY_SITE)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (server *Server) headersForStatus(status int) map[string]string {
	_, isRedirect := REDIRECT_STATUSES[status]
	if isRedirect {
		// TODO: check that protocol-independent location is legal HTTP
		// TODO: header should respect current port
		host := fmt.Sprintf("//%s", server.makeFullDomain(ZERO_DELAY_SITE))
		return map[string]string{"Location": host}
	}
	return EMPTY_HEADERS
}

func (server *Server) ListenAndServe() error {
	log.Printf("listening on %s", server.config.listenOn)
	return http.ListenAndServe(server.config.listenOn, server)
}
