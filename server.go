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
	MATCHES_ANY_STRING = ""
)

const (
	CREATE_SUBDOMAIN       = "create"
	ADMIN_SUBDOMAIN_PREFIX = "admin-"
)

var (
	EMPTY_HEADERS = map[string]string{}
)

const (
	MAX_GENERATE_SITE_NAME_ATTEMPTS = 5
	GOSLOW_LAUNCH_TIMESTAMP         = 1417447141 // December 1, 2014 18:19:01
)

const (
	DELAY_PARAM       = "delay"
	STATUS_CODE_PARAM = "status"
	METHOD_PARAM      = "method"
)

type Server struct {
	config  *Config
	storage *Storage
	hasher  *hashids.HashID // used to generate new site names
}

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
	d := hashids.NewData()
	d.Salt = salt
	d.MinLength = minLength
	d.Alphabet = "abcdefghijklmnopqrstuvwxyz1234567890"
	return hashids.NewWithData(d)
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
		err = server.createSite(w, req)

	case server.isAdmin(req):
		err = server.handleCreateEndpoint(w, req)

	default:
		allowCrossDomainRequests(w, req)
		err = server.respondFromEndpoint(w, req)
	}

	if err != nil {
		server.handleError(err, w)
	}

	logRequest(req, start)
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
	return getSubdomain(req.Host) == CREATE_SUBDOMAIN
}

func (server *Server) isInSingleSiteMode() bool {
	return server.config.isInSingleSiteMode()
}

func getSubdomain(host string) string {
	return strings.Split(host, ".")[0]
}

func (server *Server) createSite(w http.ResponseWriter, req *http.Request) error {
	site, err := server.generateUniqueSiteName(MAX_GENERATE_SITE_NAME_ATTEMPTS)
	if err != nil {
		return err
	}
	endpoint, err := server.createEndpoint(site, req)
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

func (server *Server) generateUniqueSiteName(maxAttempts int) (string, error) {
	for i := 0; i < maxAttempts; i++ {
		site, err := server.makeSiteNameFrom(generateUniqueNumbers())
		if err != nil {
			log.Print(err)
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
	secondsSinceLaunch := int(utc.Unix()) - GOSLOW_LAUNCH_TIMESTAMP // revisit this line in the year 2037
	milliseconds := (utc.Nanosecond() / 1000000)
	return []int{secondsSinceLaunch, milliseconds}
}

func (server *Server) createEndpoint(site string, req *http.Request) (*Endpoint, error) {
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
		Path:       server.getEndpointPath(req),
		Method:     server.getEndpointMethod(values),
		Headers:    EMPTY_HEADERS,
		Delay:      delay,
		StatusCode: statusCode,
		Response:   response,
	}
	return endpoint, nil
}

func (server *Server) getEndpointDelay(values url.Values) (time.Duration, error) {
	_, hasDelay := values[DELAY_PARAM]
	if !hasDelay {
		return DEFAULT_DELAY, nil
	}

	rawDelay := values.Get(DELAY_PARAM)
	delayInSeconds, err := strconv.ParseFloat(rawDelay, 64)
	if err != nil {
		return time.Duration(0), InvalidDelayError(rawDelay)
	}

	delay := secondsToDuration(delayInSeconds)
	if delay > MAX_DELAY {
		return time.Duration(0), DelayIsTooBigError(delay)
	}
	return delay, nil
}

// Convert with millisecond precision
func secondsToDuration(seconds float64) time.Duration {
	milliseconds := seconds * 1000
	return time.Duration(milliseconds) * time.Millisecond
}

func (server *Server) getEndpointStatusCode(values url.Values) (int, error) {
	_, hasStatusCode := values[STATUS_CODE_PARAM]
	if !hasStatusCode {
		return DEFAULT_STATUS_CODE, nil
	}
	return strconv.Atoi(values.Get(STATUS_CODE_PARAM))
}

func (server *Server) getEndpointMethod(values url.Values) string {
	_, hasMethod := values[METHOD_PARAM]
	if !hasMethod {
		return MATCHES_ANY_STRING
	}
	return values.Get(METHOD_PARAM)
}

func (server *Server) getEndpointPath(req *http.Request) string {
	if server.isInSingleSiteMode() {
		path := strings.TrimPrefix(req.URL.Path, server.config.adminPathPrefix)
		return ensureHasPrefix(path, "/")
	}
	return req.URL.Path
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

func logRequest(req *http.Request, start time.Time) {
	duration := time.Since(start)
	logRecord := []string{getRealIP(req), req.Method, req.Host, req.URL.Path, duration.String()}
	log.Println(strings.Join(logRecord, "\t"))
}

func getRealIP(req *http.Request) string {
	xRealIP := req.Header.Get("X-Real-IP")
	if xRealIP != "" {
		return xRealIP
	}
	host, _, _ := net.SplitHostPort(req.RemoteAddr)
	return host
}

func wantsShortResponse(req *http.Request) bool {
	values, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil { // impossible path, error from url.ParseQuery is handled in makeEndpoint
		log.Print(err)
		return false
	}
	return values.Get("output") == "short"
}

func (server *Server) showShortCreateSiteHelp(w http.ResponseWriter, endpoint *Endpoint) {
	fmt.Fprint(w, server.makeFullDomain(endpoint.Site))
}

func (server *Server) showLongCreateSiteHelp(w http.ResponseWriter, endpoint *Endpoint) {
	BANNER_TEMPLATE.Execute(w, nil)
	templateData := server.makeTemplateData(endpoint)
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
		CreateDomain:      server.makeFullDomain(CREATE_SUBDOMAIN),
		Domain:            server.makeFullDomain(endpoint.Site),
		AdminDomain:       server.makeAdminDomain(endpoint.Site),
		AdminPathPrefix:   server.config.adminPathPrefix,
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
	adminSubdomain := fmt.Sprintf("admin-%s", site)
	return server.makeFullDomain(adminSubdomain)
}

func (server *Server) makeExampleEndpoint(endpoint *Endpoint) *Endpoint {
	example := *endpoint // make a copy
	example.Path = "/christmas"
	example.Response = []byte("hohoho")
	return &example
}

func (server *Server) respondFromEndpoint(w http.ResponseWriter, req *http.Request) error {
	endpoint, found, err := server.storage.FindEndpoint(server.getSite(req), req)
	if err != nil {
		return err
	}
	if found {
		respondWith(endpoint, w)
	} else {
		return server.handleUnknownEndpoint(w, req)
	}
	return nil
}

func (server *Server) isAdmin(req *http.Request) bool {
	if req.Method != "POST" {
		return false
	}
	if server.isInSingleSiteMode() {
		return server.isAdminPath(req.URL.Path)
	}
	return strings.HasPrefix(getSubdomain(req.Host), ADMIN_SUBDOMAIN_PREFIX)
}

// TODO: refactor
func (server *Server) isAdminPath(path string) bool {
	adminPathPrefix := server.config.adminPathPrefix
	if !strings.HasPrefix(path, adminPathPrefix) {
		return false
	}
	if strings.HasSuffix(adminPathPrefix, "/") {
		return true
	}
	suffix := strings.TrimPrefix(path, adminPathPrefix)
	return suffix == "" || suffix[0] == '?' || suffix[0] == '/'
}

// TODO: rename
func (server *Server) handleCreateEndpoint(w http.ResponseWriter, req *http.Request) error {
	site := server.getSite(req)
	if !canChange(site) {
		return CantChangeBuiltinSiteError()
	}
	siteExists, err := server.storage.SiteExists(site)
	if err != nil {
		return err
	}
	if !siteExists {
		// TODO: show a long help text here (like in handleUnknownEndpoint)
		return UnknownSiteError(site)
	}
	endpoint, err := server.createEndpoint(site, req)
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
	if server.isAdmin(req) {
		return strings.TrimPrefix(subdomain, ADMIN_SUBDOMAIN_PREFIX)
	}
	return subdomain
}

func canChange(site string) bool {
	return !isCreate(site) && !isDefault(site)
}

// TODO: rename
func isCreate(site string) bool {
	return site == CREATE_SUBDOMAIN
}

func isDefault(site string) bool {
	i, err := strconv.Atoi(site)
	if err != nil {
		return false
	}
	return i <= MAX_STATUS_CODE
}

func ensureHasPrefix(s, prefix string) string {
	if !strings.HasPrefix(s, prefix) {
		return prefix + s
	}
	return s
}

func respondWith(endpoint *Endpoint, w http.ResponseWriter) {
	time.Sleep(endpoint.Delay)
	addHeaders(endpoint.Headers, w.Header())
	w.WriteHeader(endpoint.StatusCode)
	w.Write(endpoint.Response)
}

func addHeaders(headers map[string]string, responseHeader http.Header) {
	for header, value := range headers {
		responseHeader.Add(header, value)
	}
}

func (server *Server) handleUnknownEndpoint(w http.ResponseWriter, req *http.Request) error {
	w.WriteHeader(http.StatusNotFound)

	site := server.getSite(req)
	siteExists, err := server.storage.SiteExists(site)
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

	case !canChange(site):
		UNKNOWN_ERROR_TEMPLATE.Execute(w, templateData)

	case !siteExists:
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

// TODO: extract the for loop body to a separate method
func (server *Server) createSitesInRange(minSite, maxSite int) {
	for i := minSite; i <= maxSite; i++ {
		site := strconv.Itoa(i)
		err := server.storage.CreateSite(site)
		if err != nil {
			log.Fatal(err)
		}
		endpoint := &Endpoint{
			Site:       site,
			Path:       MATCHES_ANY_STRING,
			Method:     MATCHES_ANY_STRING,
			Headers:    server.headersFor(i),
			Delay:      server.delayFor(i),
			StatusCode: server.statusCodeFor(i),
			Response:   DEFAULT_RESPONSE,
		}
		err = server.storage.SaveEndpoint(endpoint)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (server *Server) headersFor(site int) map[string]string {
	if isRedirect(site) {
		zeroDelayURL := fmt.Sprintf("//%s", server.makeFullDomain(ZERO_DELAY_SITE))
		return map[string]string{"Location": zeroDelayURL}
	}
	return EMPTY_HEADERS
}

func isRedirect(statusCode int) bool {
	return statusCode == http.StatusMovedPermanently || statusCode == http.StatusFound
}

func (server *Server) delayFor(site int) time.Duration {
	delay := time.Duration(site) * time.Second
	if delay <= MAX_DELAY {
		return delay
	}
	return time.Duration(0)
}

func (server *Server) statusCodeFor(site int) int {
	if site >= MIN_STATUS_CODE && site <= MAX_STATUS_CODE {
		return site
	}
	return http.StatusOK
}

func (server *Server) ensureEmptySiteExists() {
	emptyExists, err := server.storage.SiteExists(EMPTY_SITE)
	if err != nil {
		log.Fatal(err)
	}
	if !emptyExists {
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
