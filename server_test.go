package main

// TODO: rename domain -> site where appropriate

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"
)

const (
	ANY_DB_DRIVER    = ""
	MULTI_SITE_MODE  = ""
	TEST_DEPLOYED_ON = "localhost:9999"
	TEST_POSTGRES_DB = "goslow_test"
)

var DATA_SOURCE = map[string]string{
	"sqlite3":  DEFAULT_CONFIG.dataSource,
	"postgres": "postgres://localhost/" + TEST_POSTGRES_DB,
}

type TestServer struct {
	goSlowServer  *Server
	runningServer *httptest.Server
}

type ServerTest func(*TestServer)

func TestCreateSite(t *testing.T) {
	withNewMultiSiteServer(func(server *TestServer) {

		shouldCreateSiteWithEndpoint(t, server, &Endpoint{Method: "GET", Path: "/path/", Response: []byte("long")})
		shouldCreateSiteWithEndpoint(t, server, &Endpoint{Method: "GET", Path: "/path", Response: []byte("short")})
	})
}

func shouldCreateSiteWithEndpoint(t *testing.T, server *TestServer, endpoint *Endpoint) {
	site := server.createSite(endpoint)
	req := server.makeRequestFor(&Endpoint{Site: site, Method: endpoint.Method, Path: endpoint.Path})
	shouldRespondWith(t, endpoint.Response, req)
}

func TestZeroSite(t *testing.T) {
	withNewMultiSiteServer(func(server *TestServer) {

		shouldRespondWith(t, DEFAULT_RESPONSE,
			createGET(server.getURL(), "/", makeFullDomain("0")))
	})
}

func withNewMultiSiteServer(serverTest ServerTest) {
	withNewServer(MULTI_SITE_MODE, serverTest)
}

func withNewServer(adminPathPrefix string, serverTest ServerTest) {
	for _, driver := range getDrivers() {
		withNewServerUsing(driver, adminPathPrefix, serverTest)
	}
}

func getDrivers() []string {
	drivers := []string{"sqlite3"}
	if !testing.Short() {
		drivers = append(drivers, "postgres")
	}
	return drivers
}

func withNewServerUsing(driver string, adminPathPrefix string, serverTest ServerTest) {
	if driver == "postgres" {
		createDb(TEST_POSTGRES_DB)
		defer dropDb(TEST_POSTGRES_DB)
	}
	goSlowServer := newGoSlowServer(driver, adminPathPrefix)
	runningServer := httptest.NewServer(goSlowServer)
	defer runningServer.Close()
	defer goSlowServer.storage.db.Close() // close db connections, so we can drop database later
	serverTest(&TestServer{goSlowServer: goSlowServer, runningServer: runningServer})
}

func withServers(adminPathPrefixes []string, serverTest ServerTest) {
	for _, adminPathPrefix := range adminPathPrefixes {
		withNewServer(adminPathPrefix, serverTest)
	}
}

func shouldRespondWith(t *testing.T, expectedResponse []byte, req *http.Request) {
	resp := do(req)
	bytesShouldBeEqual(t, expectedResponse, read(resp))
}

func TestTooLargeDelay(t *testing.T) {
	withServers([]string{MULTI_SITE_MODE, "/goslow"}, func(server *TestServer) {

		dontAllowToCreateEndpointWithDelay(t, server, time.Duration(1000)*time.Second)
	})
}

func dontAllowToCreateEndpointWithDelay(t *testing.T, server *TestServer, delay time.Duration) {
	server.withNewSite(func(site string) {
		resp := server.createEndpoint(&Endpoint{Site: site, Path: "/", Delay: delay})

		shouldHaveStatusCode(t, http.StatusBadRequest, resp)
		shouldHaveStatusCode(t, http.StatusNotFound, GET(server.getURL(), "/", makeFullDomain(site)))
	})
}

type SiteTest func(site string)

func (server *TestServer) withNewSite(siteTest SiteTest) {
	var site string
	if server.isInSingleSiteMode() {
		site = EMPTY_SITE
	} else {
		site = server.createSite(&Endpoint{Path: "/-fake-path-just-to-create-new-site"})
	}

	siteTest(site)
}

func (server *TestServer) isInSingleSiteMode() bool {
	return server.goSlowServer.isInSingleSiteMode()
}

// TODO: build url with url.URL not Sprintf
// TODO: endpoint is strange argument here, think of something better
// TODO: add test for this function
func (server *TestServer) createSite(endpoint *Endpoint) string {
	if server.isInSingleSiteMode() {
		log.Fatalf("Can't create site: running in single site mode. Site endpoint: %v", endpoint)
	}
	resp := POST(server.getURL(), fmt.Sprintf("%s?output=short&method=%s", endpoint.Path, endpoint.Method),
		makeFullDomain("create"), endpoint.Response)
	domain := string(read(resp))
	return getSubdomain(domain)
}

func (server *TestServer) getURL() string {
	return server.runningServer.URL
}

func (server *TestServer) getAdminPathPrefix() string {
	return server.goSlowServer.config.adminPathPrefix
}

func (server *TestServer) createEndpoint(endpoint *Endpoint) *http.Response {
	site := "admin-" + endpoint.Site
	path := endpoint.Path
	if server.isInSingleSiteMode() {
		site = EMPTY_SITE
		path = join(server.getAdminPathPrefix(), endpoint.Path)
	}

	req := createPOST(server.getURL(), path, makeFullDomain(site),
		endpoint.Response)
	req.URL.RawQuery = getQueryString(endpoint)
	return do(req)
}

func TestChangeBuiltinSites(t *testing.T) {
	withNewMultiSiteServer(func(server *TestServer) {

		dontAllowToChangeSite(t, server, http.StatusForbidden, "0")
		dontAllowToChangeSite(t, server, http.StatusForbidden, "599")
		dontAllowToChangeSite(t, server, http.StatusForbidden, "create")
	})
}

func dontAllowToChangeSite(t *testing.T, server *TestServer, expectedStatusCode int, site string) {
	resp := server.createEndpoint(&Endpoint{Site: site, Method: "GET", Path: "/test", Response: []byte("hop")})
	shouldHaveStatusCode(t, expectedStatusCode, resp)
}

func TestChangeUnknownSites(t *testing.T) {
	withNewMultiSiteServer(func(server *TestServer) {

		dontAllowToChangeSite(t, server, http.StatusNotFound, "")
		dontAllowToChangeSite(t, server, http.StatusNotFound, "uknown-site")
		dontAllowToChangeSite(t, server, http.StatusNotFound, "admin-500")
		dontAllowToChangeSite(t, server, http.StatusNotFound, "admin-create")
	})
}

func TestDelaySites(t *testing.T) {
	withNewMultiSiteServer(func(server *TestServer) {

		shouldRespondInTimeInterval(t, 0, 0.1, // seconds
			createGET(server.getURL(), "/", makeFullDomain("0")))

		shouldRespondInTimeInterval(t, 1, 1.1, // seconds
			createGET(server.getURL(), "/", makeFullDomain("1")))
	})
}

// TODO: does every multi-site test needs to begin with "withNewMultiSiteServer(func...)?"
func TestStatusSites(t *testing.T) {
	withNewMultiSiteServer(func(server *TestServer) {

		siteShouldRespondWithStatusCode(t, server, 200, "200")
		siteShouldRespondWithStatusCode(t, server, 404, "404")
		siteShouldRespondWithStatusCode(t, server, 599, "599")
	})
}

// TODO: do we need to carry server argument in this and similar functions?
func siteShouldRespondWithStatusCode(t *testing.T, server *TestServer, expectedStatusCode int, site string) {
	resp := GET(server.getURL(), "/", makeFullDomain(site))
	shouldHaveStatusCode(t, expectedStatusCode, resp)
}

func shouldRespondWithStatusCode(t *testing.T, expectedStatusCode int, req *http.Request) {
	resp := do(req)
	shouldHaveStatusCode(t, expectedStatusCode, resp)
}

func TestCreateEndpoint(t *testing.T) {
	withServers([]string{MULTI_SITE_MODE, "/goslow"}, func(server *TestServer) {
		server.withNewSite(func(site string) {

			shouldCreateEndpoint(t, server, &Endpoint{Site: site, Method: "GET", Path: "/test", Response: []byte("test-get")})
			shouldRespondWithStatusCode(t, http.StatusNotFound,
				server.makeRequestFor(&Endpoint{Site: site, Method: "GET", Path: "/unknown"}))
			shouldRespondWithStatusCode(t, http.StatusNotFound,
				server.makeRequestFor(&Endpoint{Site: site, Method: "POST", Path: "/test"}))
		})
	})
}

func shouldCreateEndpoint(t *testing.T, server *TestServer, endpoint *Endpoint) {
	server.withNewSite(func(site string) {
		server.createEndpoint(endpoint)
		shouldRespondWith(t, endpoint.Response, server.makeRequestFor(endpoint))
	})
}

func (server *TestServer) makeRequestFor(endpoint *Endpoint) *http.Request {
	return createRequest(getMethodForRequest(endpoint), server.getURL(), endpoint.Path,
		makeFullDomain(endpoint.Site), nil)
}

func getMethodForRequest(endpoint *Endpoint) string {
	if endpoint.Method == MATCHES_ANY_STRING {
		return "GET" // MATCHES_ANY_STRING is ephemeral, return something concrete
	}
	return endpoint.Method
}

// TODO: refactor
func TestEndpointMethodsClash(t *testing.T) {
	withServers([]string{MULTI_SITE_MODE, "/goslow"}, func(server *TestServer) {
		server.withNewSite(func(site string) {

			getEndpoint := &Endpoint{Site: site, Method: "GET", Path: "/test", Response: []byte("test-get")}
			postEndpoint := &Endpoint{Site: site, Method: "POST", Path: "/test", Response: []byte("test-post")}
			shouldCreateEndpoint(t, server, getEndpoint)
			shouldCreateEndpoint(t, server, postEndpoint)

			shouldRespondWith(t, getEndpoint.Response, server.makeRequestFor(getEndpoint))
			shouldRespondWith(t, postEndpoint.Response, server.makeRequestFor(postEndpoint))
		})
	})
}

// TODO: refactor, remove duplication with TestEndpointMethodsClash
func TestEndpointAnyMethod(t *testing.T) {
	withServers([]string{MULTI_SITE_MODE, "/goslow"}, func(server *TestServer) {
		server.withNewSite(func(site string) {

			anyMethodEndpoint := &Endpoint{Site: site, Method: "", Path: "/test", Response: []byte("test-any-method")}
			shouldCreateEndpoint(t, server, anyMethodEndpoint)

			shouldRespondWith(t, anyMethodEndpoint.Response, server.makeRequestFor(withMethod(anyMethodEndpoint, "GET")))
			shouldRespondWith(t, anyMethodEndpoint.Response, server.makeRequestFor(withMethod(anyMethodEndpoint, "POST")))
		})
	})
}

func withMethod(endpoint *Endpoint, method string) *Endpoint {
	copy := *endpoint
	copy.Method = method
	return &copy
}

func withPath(endpoint *Endpoint, path string) *Endpoint {
	copy := *endpoint
	copy.Path = path
	return &copy
}

func TestEndpointPathsClash(t *testing.T) {
	withServers([]string{MULTI_SITE_MODE, "/goslow"}, func(server *TestServer) {
		server.withNewSite(func(site string) {

			shortEndpoint := &Endpoint{Site: site, Method: "GET", Path: "/test", Response: []byte("test-short")}
			longEndpoint := withPath(shortEndpoint, "/test/") // with trailing slash
			longEndpoint.Response = []byte("test-long")
			shouldCreateEndpoint(t, server, shortEndpoint)
			shouldCreateEndpoint(t, server, longEndpoint)

			shouldRespondWith(t, shortEndpoint.Response, server.makeRequestFor(shortEndpoint))
			shouldRespondWith(t, longEndpoint.Response, server.makeRequestFor(longEndpoint))
		})
	})
}

func TestEndpointDelay(t *testing.T) {
	withServers([]string{MULTI_SITE_MODE, "/goslow"}, func(server *TestServer) {
		server.withNewSite(func(site string) {

			delayEndpoint := &Endpoint{Site: site, Method: "GET", Path: "/test", Delay: time.Duration(100) * time.Millisecond, Response: []byte("test-delay")}
			shouldCreateEndpoint(t, server, delayEndpoint)

			shouldRespondInTimeInterval(t, 0.1, 0.15, server.makeRequestFor(delayEndpoint))
		})
	})
}

func withNewSingleSiteServer(adminPathPrefix string, serverTest ServerTest) {
	withNewServer(adminPathPrefix, serverTest)
}

func newGoSlowServer(driver string, adminPathPrefix string) *Server {
	config := DEFAULT_CONFIG // copies DEFAULT_CONFIG
	config.deployedOn = TEST_DEPLOYED_ON
	config.driver = driver
	config.dataSource = getDataSource(driver)
	config.createDefaultEndpoints = (adminPathPrefix == "")
	config.adminPathPrefix = adminPathPrefix
	return NewServer(&config)
}

func getDataSource(driver string) string {
	dataSource, knownDriver := DATA_SOURCE[driver]
	if !knownDriver {
		log.Fatalf("unknown driver: <%s>", driver)
	}
	return dataSource
}

// TODO: is it okay to have 3 string arguments?
// TODO: shouldn't host argument come first or at least before path?
// TODO: same for createGET, POST, and createPOST
// TODO: rename host to domain everywhere?
// TODO: GET, POST, createGET, createPOST, and createRequest should be the TestServer methods
// and look like this: server.GET("0", "/")
func GET(url, path, host string) *http.Response {
	req := createGET(url, path, host)
	return do(req)
}

func do(req *http.Request) *http.Response {
	resp, err := new(http.Client).Do(req)
	if err != nil {
		log.Fatal(err)
	}
	return resp
}

func createGET(url, path, host string) *http.Request {
	return createRequest("GET", url, path, host, nil)
}

// TODO: replace 4 string arguments with maybe template url.URL argument
func createRequest(method, url, path, host string, body io.Reader) *http.Request {
	// TODO: build url with url.URL.String() instead of string concatenation
	req, err := http.NewRequest(method, url+path, body)
	if err != nil {
		log.Fatal(err)
	}
	req.Host = host
	return req
}

func read(resp *http.Response) []byte {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	return body
}

func makeFullDomain(site string) string {
	return fmt.Sprintf("%s.%s", site, TEST_DEPLOYED_ON)
}

func bytesShouldBeEqual(t *testing.T, expected, actual []byte) {
	if !bytes.Equal(expected, actual) {
		t.Fatalf("<<%v>> != <<%v>>", string(expected), string(actual))
	}
}

func intsShouldBeEqual(t *testing.T, expected, actual int) {
	if expected != actual {
		t.Fatalf("<<%v>> != <<%v>>", expected, actual)
	}
}

func shouldRespondInTimeInterval(t *testing.T, minSeconds, maxSeconds float64, req *http.Request) {
	start := time.Now()
	resp := do(req)
	read(resp)

	duration := time.Since(start)
	minDuration := secondsToDuration(minSeconds)
	maxDuration := secondsToDuration(maxSeconds)

	if duration < minDuration || duration > maxDuration {
		t.Fatalf("%s%s answered in %v. Not in the interval [%v; %v]",
			req.Host, req.URL.Path, duration, minDuration, maxDuration)
	}
}

// TODO: shouldn't all should* methods accept *http.Request and not *http.Response?
func shouldHaveStatusCode(t *testing.T, statusCode int, resp *http.Response) {
	intsShouldBeEqual(t, statusCode, resp.StatusCode)
}

// TODO: build url with url.URL not Sprintf
// TODO: endpoint is strange argument here, think of something better
// TODO: add test for this function
func createDomain(server *httptest.Server, endpoint *Endpoint) string {
	resp := POST(server.URL, fmt.Sprintf("%s?output=short&method=GET", endpoint.Path),
		makeFullDomain("create"), endpoint.Response)
	return string(read(resp))
}

func POST(url, path, host string, payload []byte) *http.Response {
	req := createPOST(url, path, host, payload)
	return do(req)
}

func createPOST(url, path, host string, payload []byte) *http.Request {
	return createRequest("POST", url, path, host, bytes.NewReader(payload))
}

func getSite(domain string) string {
	return strings.Split(domain, ".")[0]
}

func getQueryString(endpoint *Endpoint) string {
	params := url.Values{}
	params.Set("method", endpoint.Method)
	params.Set("delay", fmt.Sprintf("%f", endpoint.Delay.Seconds()))
	return params.Encode()
}

// Wrapper around path.Join. Preserves trailing slash.
func join(elem ...string) string {
	lastElem := elem[len(elem)-1]
	shouldEndWithSlash := strings.HasSuffix(lastElem, "/")
	joined := path.Join(elem...)
	if shouldEndWithSlash {
		return ensureHasSuffix(joined, "/")
	}
	return joined
}

func ensureHasSuffix(s, suffix string) string {
	if !strings.HasSuffix(s, suffix) {
		return s + suffix
	}
	return s
}

func createDb(name string) {
	runCommand("createdb", name)
}

func runCommand(name string, arg ...string) {
	cmd := exec.Command(name, arg...)
	err := cmd.Run()
	if err != nil {
		log.Fatalf("%s error: %s", name, err)
	}
}

func dropDb(name string) {
	runCommand("dropdb", name)
}
