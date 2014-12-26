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
	"strconv"
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

type DomainFunc func(domain string)

func withNewDomain(server *httptest.Server, domainFunc DomainFunc) {
	domain := createDomain(server, &Endpoint{Path: "/path-is-irrelevant"})
	domainFunc(domain)
}

func TestRedefineBuiltinSites(t *testing.T) {
	runAll(t, checkRedefineBuiltinSites, multiDomainTestCases)
}

func checkRedefineBuiltinSites(t *testing.T, server *httptest.Server, testCase *TestCase) {
	for _, site := range []string{"0", "99", "500", "create"} {
		resp := createEndpoint(server, &Endpoint{Site: site, Path: "/test", Response: []byte("hop"), Method: "GET"})
		shouldHaveStatusCode(t, http.StatusForbidden, resp)
	}
}

func TestRedefineNonExistentSite(t *testing.T) {
	runAll(t, checkRedefineNonExistentSite, multiDomainTestCases)
}

func checkRedefineNonExistentSite(t *testing.T, server *httptest.Server, testCase *TestCase) {
	for _, site := range []string{"", "ha", "admin-500"} {
		resp := createEndpoint(server, &Endpoint{Site: site})
		shouldHaveStatusCode(t, http.StatusNotFound, resp)
	}
}

func TestDelay(t *testing.T) {
	runAll(t, checkDelay, multiDomainTestCases)
}

func checkDelay(t *testing.T, server *httptest.Server, testCase *TestCase) {
	shouldRespondIn(t,
		createGET(server.URL, "/", makeFullDomain("0")),
		0, 0.1) // seconds
	shouldRespondIn(t,
		createGET(server.URL, "/", makeFullDomain("1")),
		1, 1.1) // seconds
}

func TestStatus(t *testing.T) {
	runAll(t, checkStatus, multiDomainTestCases)
}

func checkStatus(t *testing.T, server *httptest.Server, testCase *TestCase) {
	for _, statusCode := range []int{200, 404, 500} {
		resp := GET(server.URL, "/", makeFullDomain(strconv.Itoa(statusCode)))
		shouldHaveStatusCode(t, statusCode, resp)
	}
}

func TestEndpointCreation(t *testing.T) {
	runAll(t, checkEndpointCreationTestCase, endpointCreationTestCases)
}

func checkEndpointCreationTestCase(t *testing.T, server *httptest.Server, testCase *TestCase) {
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
	intShouldBeEqual(t, 404, resp.StatusCode)

	// testing POST endpoint
	resp = createEndpoint(server, &Endpoint{Site: site, Path: join(prefix, "/test"), Response: test_post_response, Method: "POST",
		Delay: time.Duration(100) * time.Millisecond})
	shouldHaveStatusCode(t, http.StatusOK, resp)
	// checking that POST endpoint doesn't affect GET
	bytesShouldBeEqual(t, read(GET(server.URL, "/test", domain)), test_response)
	// checking that POST /test endpoint works
	bytesShouldBeEqual(t, read(POST(server.URL, "/test", domain, empty_payload)), test_post_response)
	shouldRespondIn(t, createPOST(server.URL, "/test", domain, empty_payload), 0.1, 0.15)
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

func createRequest(method, url, path, host string, body io.Reader) *http.Request {
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

func intShouldBeEqual(t *testing.T, expected, actual int) {
	if expected != actual {
		t.Fatalf("<<%v>> != <<%v>>", expected, actual)
	}
}

func shouldRespondIn(t *testing.T, req *http.Request, minSeconds, maxSeconds float64) {
	start := time.Now()
	resp := do(req)
	read(resp)
	duration := time.Since(start)
	minDuration := toDuration(minSeconds)
	maxDuration := toDuration(maxSeconds)
	if duration < minDuration || duration > maxDuration {
		t.Fatalf("%s%s answered in %v. Not in the interval [%v; %v]",
			req.Host, req.URL.Path, duration, minDuration, maxDuration)
	}
}

func shouldHaveStatusCode(t *testing.T, statusCode int, resp *http.Response) {
	intShouldBeEqual(t, statusCode, resp.StatusCode)
}

func toDuration(seconds float64) time.Duration {
	return time.Duration(seconds*1000) * time.Millisecond
}

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
	if shouldEndWithSlash && !strings.HasSuffix(joined, "/") {
		joined += "/"
	}
	return joined
}

func createDb(name string) {
	cmd := exec.Command("createdb", name)
	err := cmd.Run()
	if err != nil {
		log.Fatalf("createdb error: %s", err)
	}
}

func dropDb(name string) {
	cmd := exec.Command("dropdb", name)
	err := cmd.Run()
	if err != nil {
		log.Fatalf("dropdb error: %s", err)
	}
}
