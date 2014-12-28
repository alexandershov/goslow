package main

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
	ANY_DB_DRIVER     = ""
	MULTI_DOMAIN_MODE = ""
	TEST_DEPLOYED_ON  = "localhost:9999"
	TEST_DB           = "goslow_test"
)

var DATA_SOURCE = map[string]string{
	ANY_DB_DRIVER: "", // value doesn't matter
	"sqlite3":     DEFAULT_CONFIG.dataSource,
	"postgres":    "postgres://localhost/" + TEST_DB,
}

type TestCase struct {
	driver                 string
	dataSource             string
	createDefaultEndpoints bool
	adminPathPrefix        string
}

type TestCases []*TestCase

// TODO: do we need *TestCase argument in ServerTest?
type ServerTest func(*testing.T, *httptest.Server, *TestCase)

var (
	multiDomainTestCases = TestCases{
		NewTestCase(ANY_DB_DRIVER, MULTI_DOMAIN_MODE),
	}

	endpointCreationTestCases = TestCases{
		NewTestCase(ANY_DB_DRIVER, MULTI_DOMAIN_MODE),
		NewTestCase(ANY_DB_DRIVER, "/goslow"),
		NewTestCase(ANY_DB_DRIVER, "/goslow/"),
		NewTestCase(ANY_DB_DRIVER, "/te"),
		NewTestCase(ANY_DB_DRIVER, "/te/"),
		NewTestCase(ANY_DB_DRIVER, "/composite/path"),
	}
)

func NewTestCase(driver string, adminPathPrefix string) *TestCase {
	dataSource, knownDriver := DATA_SOURCE[driver]
	if !knownDriver {
		log.Fatalf("unknown driver: <%s>", driver)
	}
	createDefaultEndpoints := (adminPathPrefix == MULTI_DOMAIN_MODE)
	return &TestCase{
		driver:                 driver,
		dataSource:             dataSource,
		createDefaultEndpoints: createDefaultEndpoints,
		adminPathPrefix:        adminPathPrefix,
	}
}

func TestZeroSite(t *testing.T) {
	runAll(t, zeroSiteServerTest, multiDomainTestCases)
}

func runAll(t *testing.T, serverTest ServerTest, testCases TestCases) {
	for _, testCase := range runnable(expandToConcreteDriver(testCases)) {
		run(t, serverTest, testCase)
	}
}

func runnable(testCases TestCases) TestCases {
	runnableTestCases := make(TestCases, 0)
	for _, testCase := range testCases {
		if testCase.runnable() {
			runnableTestCases = append(runnableTestCases, testCase)
		}
	}
	return runnableTestCases
}

func expandToConcreteDriver(testCases TestCases) TestCases {
	concreteTestCases := make(TestCases, 0)
	for _, testCase := range testCases {
		if testCase.driver == ANY_DB_DRIVER {
			concreteTestCases = append(concreteTestCases, NewTestCase("sqlite3", testCase.adminPathPrefix))
			concreteTestCases = append(concreteTestCases, NewTestCase("postgres", testCase.adminPathPrefix))
		} else {
			concreteTestCases = append(concreteTestCases, testCase)
		}
	}
	return concreteTestCases
}

func (testCase *TestCase) runnable() bool {
	return !testCase.skippable()
}

func (testCase *TestCase) skippable() bool {
	return testCase.driver == "postgres" && testing.Short()
}

func run(t *testing.T, serverTest ServerTest, testCase *TestCase) {
	if testCase.driver == "postgres" {
		createDb(TEST_DB)
		defer dropDb(TEST_DB)
	}
	goSlowServer := newGoSlowServer(testCase)
	server := httptest.NewServer(goSlowServer)
	defer server.Close()
	defer goSlowServer.storage.db.Close() // so we can drop database
	serverTest(t, server, testCase)
}

func zeroSiteServerTest(t *testing.T, server *httptest.Server, testCase *TestCase) {
	shouldRespondWith(t, DEFAULT_RESPONSE,
		createGET(server.URL, "/", makeFullDomain("0")))
}

func shouldRespondWith(t *testing.T, expectedResponse []byte, request *http.Request) {
	response := do(request)
	bytesShouldBeEqual(t, expectedResponse, read(response))
}

func TestTooLargeDelay(t *testing.T) {
	runAll(t, tooLargeDelayServerTest, multiDomainTestCases)
}

func tooLargeDelayServerTest(t *testing.T, server *httptest.Server, testCase *TestCase) {
	dontAllowToCreateEndpointWithDelay(t, server, time.Duration(1000)*time.Second)
}

func dontAllowToCreateEndpointWithDelay(t *testing.T, server *httptest.Server, delay time.Duration) {
	withNewDomain(server, func(domain string) {
		resp := createEndpoint(server, &Endpoint{Site: getSite(domain), Path: "/", Delay: delay})

		shouldHaveStatusCode(t, http.StatusBadRequest, resp)
		shouldHaveStatusCode(t, http.StatusNotFound, GET(server.URL, "/", domain))
	})
}

type DomainTest func(domain string)

func withNewDomain(server *httptest.Server, domainTest DomainTest) {
	domain := createDomain(server, &Endpoint{Path: "/path-is-irrelevant"})
	domainTest(domain)
}

func TestChangeBuiltinSites(t *testing.T) {
	runAll(t, changeBuiltinSitesServerTest, multiDomainTestCases)
}

func changeBuiltinSitesServerTest(t *testing.T, server *httptest.Server, testCase *TestCase) {
	dontAllowToChangeSite(t, server, http.StatusForbidden, "0")
	dontAllowToChangeSite(t, server, http.StatusForbidden, "599")
	dontAllowToChangeSite(t, server, http.StatusForbidden, "create")
}

func dontAllowToChangeSite(t *testing.T, server *httptest.Server, expectedStatusCode int, site string) {
	resp := createEndpoint(server, &Endpoint{Site: site, Path: "/test", Response: []byte("hop"), Method: "GET"})
	shouldHaveStatusCode(t, expectedStatusCode, resp)
}

func TestChangeUnknownSites(t *testing.T) {
	runAll(t, changeUnknownSitesServerTest, multiDomainTestCases)
}

func changeUnknownSitesServerTest(t *testing.T, server *httptest.Server, testCase *TestCase) {
	dontAllowToChangeSite(t, server, http.StatusNotFound, "")
	dontAllowToChangeSite(t, server, http.StatusNotFound, "uknown-site")
	dontAllowToChangeSite(t, server, http.StatusNotFound, "admin-500")
	dontAllowToChangeSite(t, server, http.StatusNotFound, "admin-create")
}

func TestDelaySites(t *testing.T) {
	runAll(t, delaySitesServerTest, multiDomainTestCases)
}

func delaySitesServerTest(t *testing.T, server *httptest.Server, testCase *TestCase) {
	shouldRespondInTimeInterval(t, 0, 0.1, // seconds
		createGET(server.URL, "/", makeFullDomain("0")),
	)

	shouldRespondInTimeInterval(t, 1, 1.1, // seconds
		createGET(server.URL, "/", makeFullDomain("1")),
	)
}

func TestStatusSites(t *testing.T) {
	runAll(t, statusSitesServerTest, multiDomainTestCases)
}

func statusSitesServerTest(t *testing.T, server *httptest.Server, testCase *TestCase) {
	siteShouldRespondWithStatusCode(t, server, 200, "200")
	siteShouldRespondWithStatusCode(t, server, 404, "404")
	siteShouldRespondWithStatusCode(t, server, 599, "599")
}

// TODO: do we need to carry server argument in this and similar functions?
func siteShouldRespondWithStatusCode(t *testing.T, server *httptest.Server, statusCode int, site string) {
	resp := GET(server.URL, "/", makeFullDomain(site))
	shouldHaveStatusCode(t, statusCode, resp)
}

func TestEndpointCreation(t *testing.T) {
	runAll(t, endpointCreationServerTest, endpointCreationTestCases)
}

func endpointCreationServerTest(t *testing.T, server *httptest.Server, testCase *TestCase) {
	prefix := testCase.adminPathPrefix
	isInSingleSiteMode := prefix != ""
	domain, site := TEST_DEPLOYED_ON, ""
	root_response := []byte("haha")
	test_response := []byte("hop")
	test_post_response := []byte("for POST")
	empty_payload := []byte("")

	if isInSingleSiteMode {
		resp := createEndpoint(server, &Endpoint{Path: join(prefix, "/"), Response: root_response})
		shouldHaveStatusCode(t, http.StatusOK, resp)
	} else {
		domain = createDomain(server, &Endpoint{Path: join(prefix, "/"), Response: root_response})
		site = getSite(domain)
	}

	bytesShouldBeEqual(t, read(GET(server.URL, "/", domain)), root_response)

	// testing GET endpoint
	resp := createEndpoint(server, &Endpoint{Site: site, Path: join(prefix, "/test"), Response: test_response, Method: "GET"})
	shouldHaveStatusCode(t, http.StatusOK, resp)
	// checking that GET /test endpoint works
	bytesShouldBeEqual(t, read(GET(server.URL, "/test", domain)), test_response)
	// checking that GET /test doesn't affect POST
	resp = POST(server.URL, "/test", domain, []byte(""))
	intsShouldBeEqual(t, 404, resp.StatusCode)

	// testing POST endpoint
	resp = createEndpoint(server, &Endpoint{Site: site, Path: join(prefix, "/test"), Response: test_post_response, Method: "POST",
		Delay: time.Duration(100) * time.Millisecond})
	shouldHaveStatusCode(t, http.StatusOK, resp)
	// checking that POST endpoint doesn't affect GET
	bytesShouldBeEqual(t, read(GET(server.URL, "/test", domain)), test_response)
	// checking that POST /test endpoint works
	bytesShouldBeEqual(t, read(POST(server.URL, "/test", domain, empty_payload)), test_post_response)
	shouldRespondInTimeInterval(t, 0.1, 0.15, createPOST(server.URL, "/test", domain, empty_payload))
}

func newGoSlowServer(testCase *TestCase) *Server {
	config := DEFAULT_CONFIG // copies DEFAULT_CONFIG
	config.deployedOn = TEST_DEPLOYED_ON
	config.createDefaultEndpoints = testCase.createDefaultEndpoints
	config.adminPathPrefix = testCase.adminPathPrefix
	config.driver = testCase.driver
	config.dataSource = testCase.dataSource
	return NewServer(&config)
}

// TODO: is it okay to have 3 string arguments?
// TODO: shouldn't host argument come first?
// TODO: same for createGET, POST, and createPOST
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

func createEndpoint(server *httptest.Server, endpoint *Endpoint) *http.Response {
	req := createPOST(server.URL, endpoint.Path, makeFullDomain("admin-"+endpoint.Site),
		endpoint.Response)
	req.URL.RawQuery = getQueryString(endpoint)
	return do(req)
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
