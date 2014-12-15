package main

// TODO: look at different places where you can supply good error messages

import (
	"fmt"
	"github.com/alexandershov/go-hashids"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	DEFAULT_RESPONSE    = []byte(`{"goslow": "response"}`)
	DEFAULT_DELAY       = time.Duration(0)
	DEFAULT_STATUS_CODE = http.StatusOK
)

const (
	MIN_DELAY = time.Duration(0) * time.Second
	MAX_DELAY = time.Duration(199) * time.Second

	MIN_STATUS_CODE = 200
	MAX_STATUS_CODE = 599

	ZERO_DELAY_SITE = "0"
	EMPTY_SITE      = ""
)

const (
	ANY_STRING = ""
)

const (
	CREATE_SUBDOMAIN_NAME         = "create"
	ADD_ENDPOINT_SUBDOMAIN_PREFIX = "admin-"
)

var (
	REDIRECT_STATUSES = map[int]bool{301: true, 302: true}
	EMPTY_HEADERS     = map[string]string{}
)

const (
	MAX_GENERATE_SITE_NAME_ATTEMPTS = 5
	GOSLOW_LAUNCH_TIMESTAMP         = 1417447141
)

const (
	DELAY_PARAM       = "delay"
	STATUS_CODE_PARAM = "status"
	METHOD_PARAM      = "method"
)

// Server listens on the address specified by the config,
// stores endpoints in the storage, and generates new site names
// with the hasher.
type Server struct {
	config  *Config
	storage *Storage
	hasher  *hashids.HashID
}

// NewServer returns a new server from the specified config.
func NewServer(config *Config) *Server {
	storage, err := NewStorage(config.driver, config.dataSource)
	if err != nil {
		log.Fatal(err)
	}

	server := &Server{
		config:  config,
		storage: storage,
		hasher:  newHasher(config.siteSalt, config.minSiteLength),
	}
	if config.createDefaultEndpoints {
		server.createDefaultEndpoints()
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

// Server.ServeHTTP implements Handler interface.
// It allows cross domain requests via CORS headers.
func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	var err error = nil
	switch {
	case server.isOptions(req):
		allowCrossDomainRequests(w, req)

	case server.isCreateSite(req):
		err = server.handleCreateSite(w, req)

	case server.isAddEndpoint(req):
		err = server.handleAddEndpoint(w, req)

	default:
		allowCrossDomainRequests(w, req)
		err = server.respondFromEndpoint(w, req)
	}
	if err != nil {
		server.handleError(err, w)
	}
	duration := time.Since(start)
	log.Printf("%s\t%s\t%s\t%s\t%s", getRealIP(req), req.Method, req.Host, req.URL.Path, duration)
}

func getRealIP(req *http.Request) string {
	xRealIP := req.Header.Get("X-Real-IP")
	if xRealIP != "" {
		return xRealIP
	}
	host, _, _ := net.SplitHostPort(req.RemoteAddr)
	return host
}

func (server *Server) isOptions(req *http.Request) bool {
	return req.Method == "OPTIONS"
}

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
	return server.config.isInSingleSiteMode()
}

func getSubdomain(url string) string {
	return strings.Split(url, ".")[0]
}

func (server *Server) handleCreateSite(w http.ResponseWriter, req *http.Request) error {
	site, err := server.generateUniqueSiteName(MAX_GENERATE_SITE_NAME_ATTEMPTS)
	if err != nil {
		return err
	}
	endpoint, err := server.addEndpoint(site, req)
	if err != nil {
		return err
	}
	if wantsShortResponse(req) {
		server.showShortCreateSiteHelp(w, endpoint)
	} else {
		server.showLongCreateSiteHelp(w, endpoint)
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
		time.Sleep(getRandomDurationBetween(10, 20)) // milliseconds
	}
	return "", CantCreateSiteError()
}

func (server *Server) makeSiteNameFrom(numbers []int) (string, error) {
	return server.hasher.Encode(numbers)
}

func generateUniqueNumbers() []int {
	utc := time.Now().UTC()
	seconds := int(utc.Unix()) - GOSLOW_LAUNCH_TIMESTAMP // revisit this line in the year 2037
	milliseconds := (utc.Nanosecond() / 1000000)
	return []int{seconds, milliseconds}
}

func (server *Server) addEndpoint(site string, req *http.Request) (*Endpoint, error) {
	endpoint, err := server.makeEndpoint(site, req)
	if err != nil {
		return nil, err
	}
	err = server.storage.SaveEndpoint(endpoint)
	return endpoint, err
}

func (server *Server) makeEndpoint(site string, req *http.Request) (*Endpoint, error) {
	values, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		return nil, err
	}
	path := server.getEndpointPath(req)
	method := server.getEndpointMethod(values)
	delay, err := server.getEndpointDelay(values)
	if err != nil {
		return nil, err
	}
	statusCode, err := server.getEndpointStatusCode(values)
	if err != nil {
		return nil, err
	}
	response, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	endpoint := &Endpoint{
		Site:       site,
		Path:       path,
		Method:     method,
		Headers:    EMPTY_HEADERS,
		Delay:      delay,
		StatusCode: statusCode,
		Response:   response,
	}
	return endpoint, nil
}

func (server *Server) getEndpointDelay(values url.Values) (time.Duration, error) {
	_, contains := values[DELAY_PARAM]
	if !contains {
		return DEFAULT_DELAY, nil
	}
	delayRaw := values.Get(DELAY_PARAM)
	delayInSeconds, err := strconv.ParseFloat(delayRaw, 64)
	if err != nil {
		return time.Duration(0), InvalidDelayError(delayRaw)
	}
	delay := time.Duration(delayInSeconds*1000) * time.Millisecond
	if delay > MAX_DELAY {
		return time.Duration(0), DelayIsTooBigError(delay)
	}
	return delay, nil
}

func (server *Server) getEndpointStatusCode(values url.Values) (int, error) {
	_, contains := values[STATUS_CODE_PARAM]
	if !contains {
		return DEFAULT_STATUS_CODE, nil
	}
	return strconv.Atoi(values.Get(STATUS_CODE_PARAM))
}

func (server *Server) getEndpointMethod(values url.Values) string {
	return values.Get(METHOD_PARAM)
}

func getRandomDurationBetween(minMilliseconds, maxMilliseconds int) time.Duration {
	milliseconds := minMilliseconds + rand.Intn(maxMilliseconds-minMilliseconds+1)
	return time.Duration(milliseconds) * time.Millisecond
}

func (server *Server) handleError(err error, w http.ResponseWriter) {
	log.Printf("error: %s", err)
	apiError, isApiError := err.(*ApiError)
	if isApiError {
		http.Error(w, apiError.Error(), apiError.StatusCode)
	} else {
		message := fmt.Sprintf("Internal error: %s.", err)
		http.Error(w, message, http.StatusInternalServerError)
	}
}

func wantsShortResponse(req *http.Request) bool {
	values, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		return false
	}
	return values.Get("output") == "short"
}

func (server *Server) showShortCreateSiteHelp(w http.ResponseWriter, endpoint *Endpoint) {
	fmt.Fprint(w, server.makeFullDomain(endpoint.Site))
}

func (server *Server) showLongCreateSiteHelp(w http.ResponseWriter, endpoint *Endpoint) {
	templateData := server.makeTemplateData(endpoint)
	BANNER_TEMPLATE.Execute(w, nil)
	ENDPOINT_ADDED_TEMPLATE.Execute(w, templateData)
	fmt.Fprintln(w)
	SITE_CREATED_TEMPLATE.Execute(w, templateData)
	fmt.Fprintln(w)
	ADD_ENDPOINT_EXAMPLE_TEMPLATE.Execute(w, server.makeTemplateData(server.makeExampleEndpoint(endpoint)))
}

func (server *Server) makeTemplateData(endpoint *Endpoint) *TemplateData {
	return &TemplateData{
		Site:              endpoint.Site,
		Path:              endpoint.Path,
		Method:            endpoint.Method,
		Delay:             endpoint.Delay,
		TruncatedResponse: truncate(string(endpoint.Response), 80),
		CreateDomain:      server.makeFullDomain(CREATE_SUBDOMAIN_NAME),
		Domain:            server.makeFullDomain(endpoint.Site),
		AdminDomain:       server.makeAdminDomain(endpoint.Site),
		adminPathPrefix:   server.config.adminPathPrefix,
	}
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
		return server.config.deployedOn
	}
	return fmt.Sprintf("%s.%s", site, server.config.deployedOn)
}

func (server *Server) makeAdminDomain(site string) string {
	if server.isInSingleSiteMode() {
		return server.config.deployedOn
	}
	return fmt.Sprintf("admin-%s", server.makeFullDomain(site))
}

func (server *Server) makeAdminPath(site string) string {
	adminDomain := server.makeAdminDomain(site)
	return adminDomain + server.config.adminPathPrefix
}

func (server *Server) makeExampleEndpoint(endpoint *Endpoint) *Endpoint {
	example := *endpoint // make a copy
	example.Path = "/christmas"
	example.Response = []byte("hohoho")
	return &example
}

func (server *Server) respondFromEndpoint(w http.ResponseWriter, req *http.Request) error {
	endpoint, found, err := server.storage.FindEndpointMatching(server.getSite(req), req)
	if err != nil {
		return err
	}
	if found {
		applyEndpoint(endpoint, w)
	} else {
		return server.handleUnknownEndpoint(w, req)
	}
	return nil
}

func (server *Server) isAddEndpoint(req *http.Request) bool {
	if req.Method != "POST" {
		return false
	}
	if server.isInSingleSiteMode() {
		return server.isAddEndpointPath(req.URL.Path)
	}
	return strings.HasPrefix(getSubdomain(req.Host), ADD_ENDPOINT_SUBDOMAIN_PREFIX)
}

func (server *Server) isAddEndpointPath(path string) bool {
	adminPath := server.config.adminPathPrefix
	if !strings.HasPrefix(path, adminPath) {
		return false
	}
	if strings.HasSuffix(adminPath, "/") {
		return true
	}
	suffix := strings.TrimPrefix(path, adminPath)
	return suffix == "" || suffix[0] == '?' || suffix[0] == '/'
}

func (server *Server) handleAddEndpoint(w http.ResponseWriter, req *http.Request) error {
	site := server.getSite(req)
	if isBuiltin(site) {
		return ChangeBuiltinSiteError()
	}
	contains, err := server.storage.ContainsSite(site)
	if err != nil {
		return err
	}
	if !contains {
		// TODO: show a long help text here (like in handleUnknownEndpoint)
		return UnknownSiteError(site)
	}
	endpoint, err := server.addEndpoint(site, req)
	if err != nil {
		return err
	}
	BANNER_TEMPLATE.Execute(w, nil)
	ENDPOINT_ADDED_TEMPLATE.Execute(w, server.makeTemplateData(endpoint))
	return nil
}

func (server *Server) getSite(req *http.Request) string {
	if server.isInSingleSiteMode() {
		return EMPTY_SITE
	}
	subdomain := getSubdomain(req.Host)
	if server.isAddEndpoint(req) {
		return strings.TrimPrefix(subdomain, ADD_ENDPOINT_SUBDOMAIN_PREFIX)
	}
	return subdomain
}

func isBuiltin(site string) bool {
	return isCreate(site) || isDefault(site)
}

func isCreate(site string) bool {
	return site == CREATE_SUBDOMAIN_NAME
}

func isDefault(site string) bool {
	i, err := strconv.Atoi(site)
	if err != nil {
		return false
	}
	return i <= MAX_STATUS_CODE
}

func (server *Server) getEndpointPath(req *http.Request) string {
	if server.isInSingleSiteMode() {
		path := strings.TrimPrefix(req.URL.Path, server.config.adminPathPrefix)
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

func applyEndpoint(endpoint *Endpoint, w http.ResponseWriter) {
	time.Sleep(endpoint.Delay)
	addHeaders(endpoint.Headers, w.Header())
	w.WriteHeader(endpoint.StatusCode)
	w.Write(endpoint.Response)
}

func addHeaders(headers map[string]string, responseHeader http.Header) {
	for key, value := range headers {
		responseHeader.Add(key, value)
	}
}

func (server *Server) handleUnknownEndpoint(w http.ResponseWriter, req *http.Request) error {
	w.WriteHeader(http.StatusNotFound)

	site := server.getSite(req)
	containsSite, err := server.storage.ContainsSite(site)
	if err != nil {
		return err
	}

	endpoint := &Endpoint{Site: site, Path: server.getEndpointPath(req)}
	templateData := server.makeTemplateData(endpoint)

	BANNER_TEMPLATE.Execute(w, nil)

	switch {
	case server.isInSingleSiteMode():
		exampleTemplateData := *templateData
		exampleTemplateData.TruncatedResponse = "hohoho"
		// TODO: why do we need both templateData and exampleTemplateData?
		UNKNOWN_ENDPOINT_TEMPLATE.Execute(w, templateData)
		ADD_ENDPOINT_EXAMPLE_TEMPLATE.Execute(w, exampleTemplateData)

	case isCreate(site):
		exampleTemplateData := *templateData
		exampleTemplateData.TruncatedResponse = "hohoho"
		CREATE_SITE_HELP_TEMPLATE.Execute(w, templateData)
		fmt.Fprintln(w) // TODO: why we need two calls to Fprintln?
		fmt.Fprintln(w)
		CREATE_SITE_EXAMPLE_TEMPLATE.Execute(w, exampleTemplateData)

	case isBuiltin(site):
		UNKNOWN_ERROR_TEMPLATE.Execute(w, templateData)

	case !containsSite:
		exampleTemplateData := *templateData
		exampleTemplateData.TruncatedResponse = "hohoho"
		UNKNOWN_SITE_TEMPLATE.Execute(w, templateData)
		CREATE_SITE_EXAMPLE_TEMPLATE.Execute(w, exampleTemplateData)

	default: // when will this branch be chosen?
		exampleTemplateData := *templateData
		exampleTemplateData.TruncatedResponse = "hohoho"
		// TODO: why do we need both templateData and exampleTemplateData?
		UNKNOWN_ENDPOINT_TEMPLATE.Execute(w, templateData)
		ADD_ENDPOINT_EXAMPLE_TEMPLATE.Execute(w, exampleTemplateData)
	}
	return nil
}

func (server *Server) createDefaultEndpoints() {
	server.createSitesInRange(int(MIN_DELAY/time.Second), int(MAX_DELAY/time.Second))
	server.createSitesInRange(MIN_STATUS_CODE, MAX_STATUS_CODE)
}

func (server *Server) createSitesInRange(minSite, maxSite int) {
	for i := minSite; i <= maxSite; i++ {
		site := strconv.Itoa(i)
		err := server.storage.CreateSite(site)
		if err != nil {
			log.Fatal(err)
		}
		endpoint := &Endpoint{
			Site:       site,
			Path:       ANY_STRING,
			Method:     ANY_STRING,
			Headers:    server.headersFor(i),
			Delay:      server.delayFor(i),
			StatusCode: server.statusFor(i),
			Response:   DEFAULT_RESPONSE,
		}
		err = server.storage.SaveEndpoint(endpoint)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (server *Server) headersFor(site int) map[string]string {
	_, isRedirect := REDIRECT_STATUSES[site]
	if isRedirect {
		host := fmt.Sprintf("//%s", server.makeFullDomain(ZERO_DELAY_SITE))
		return map[string]string{"Location": host}
	}
	return EMPTY_HEADERS
}

func (server *Server) delayFor(site int) time.Duration {
	if time.Duration(site)*time.Second <= MAX_DELAY {
		return time.Duration(site) * time.Second
	}
	return time.Duration(0)
}

func (server *Server) statusFor(site int) int {
	if site >= MIN_STATUS_CODE && site <= MAX_STATUS_CODE {
		return site
	}
	return http.StatusOK
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

// Server.ListenAndServe listens on the address specified by the config.
func (server *Server) ListenAndServe() error {
	log.Printf("listening on %s", server.config.listenOn)
	return http.ListenAndServe(server.config.listenOn, server)
}
