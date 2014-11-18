package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	ANY_DB_DRIVER = ""
	TEST_ENDPOINT = "localhost:9999"
	TEST_DB       = "goslow_test"
)

var DATA_SOURCE = map[string]string{
	"sqlite3":  ":memory:",
	"postgres": "postgres://localhost/" + TEST_DB,
}

type TestCase struct {
	createDefaultRules bool
	adminUrlPathPrefix string
	driver             string
	dataSource         string
}

type TestCases []*TestCase

type CheckFunc func(*testing.T, *httptest.Server, *TestCase)

var (
	defaultTestCases = TestCases{
		NewTestCase(true, "", ANY_DB_DRIVER),
	}

	ruleCreationTestCases = TestCases{
		NewTestCase(true, "", ANY_DB_DRIVER),
		NewTestCase(false, "/goslow", ANY_DB_DRIVER),
		NewTestCase(false, "/goslow/", ANY_DB_DRIVER),
		NewTestCase(false, "/te", ANY_DB_DRIVER),
		NewTestCase(false, "/te/", ANY_DB_DRIVER),
		NewTestCase(false, "/composite/path", ANY_DB_DRIVER),
	}
)

func NewTestCase(createDefaultRules bool, adminUrlPathPrefix string, driver string) *TestCase {
	dataSource, knownDriver := DATA_SOURCE[driver]
	if driver != ANY_DB_DRIVER && !knownDriver {
		log.Fatalf("unknown driver: <%s>", driver)
	}
	return &TestCase{
		createDefaultRules: createDefaultRules,
		adminUrlPathPrefix: adminUrlPathPrefix,
		driver:             driver,
		dataSource:         dataSource,
	}
}

func TestZeroSite(t *testing.T) {
	runAll(t, checkZeroSite, defaultTestCases)
}

func runAll(t *testing.T, checkFunc CheckFunc, testCases TestCases) {
	for _, testCase := range runnable(all(testCases)) {
		run(t, checkFunc, testCase)
	}
}

func runnable(testCases TestCases) TestCases {
	runnableTestCases := make(TestCases, 0)
	for _, testCase := range testCases {
		if !testCase.skippable() {
			runnableTestCases = append(runnableTestCases, testCase)
		}
	}
	return runnableTestCases
}

func all(testCases TestCases) TestCases {
	allTestCases := make(TestCases, 0)
	for _, testCase := range testCases {
		if testCase.driver == ANY_DB_DRIVER {
			sqlite3TestCase := NewTestCase(testCase.createDefaultRules, testCase.adminUrlPathPrefix, "sqlite3")
			postgresTestCase := NewTestCase(testCase.createDefaultRules, testCase.adminUrlPathPrefix, "postgres")
			allTestCases = append(allTestCases, sqlite3TestCase)
			allTestCases = append(allTestCases, postgresTestCase)
		} else {
			allTestCases = append(allTestCases, testCase)
		}
	}
	return allTestCases
}

func (testCase *TestCase) skippable() bool {
	return testCase.driver == "postgres" && testing.Short()
}

func run(t *testing.T, checkFunc CheckFunc, testCase *TestCase) {
	if testCase.driver == "postgres" {
		createDb(TEST_DB)
		defer dropDb(TEST_DB)
	}
	goSlowServer := newSubDomainServer(testCase)
	server := httptest.NewServer(goSlowServer)
	defer server.Close()
	defer goSlowServer.storage.db.Close()
	checkFunc(t, server, testCase)
}

func checkZeroSite(t *testing.T, server *httptest.Server, testCase *TestCase) {
	bytesShouldBeEqual(t,
		readBody(GET(server.URL, "/", makeHost("0", TEST_ENDPOINT))),
		DEFAULT_BODY)
}

func TestTooLargeDelay(t *testing.T) {
	runAll(t, checkTooLargeDelay, defaultTestCases)
}

func checkTooLargeDelay(t *testing.T, server *httptest.Server, testCase *TestCase) {
	prefix := testCase.adminUrlPathPrefix
	site := newSite(server, join(prefix, "/booya"), []byte("haha"))
	resp := addRule(server, &Rule{Site: site, Delay: time.Duration(1000) * time.Second})
	shouldHaveStatusCode(t, http.StatusBadRequest, resp)
	resp = GET(server.URL, "/", site)
	shouldHaveStatusCode(t, http.StatusNotFound, resp)
}

func TestRedefineBuiltinSites(t *testing.T) {
	runAll(t, checkRedefineBuiltinSites, defaultTestCases)
}

func checkRedefineBuiltinSites(t *testing.T, server *httptest.Server, testCase *TestCase) {
	for _, site := range []string{"0", "99", "500", "create"} {
		resp := addRule(server, &Rule{Site: site, Path: "/test", Body: []byte("hop"), Method: "GET"})
		shouldHaveStatusCode(t, http.StatusForbidden, resp)
	}
}

func TestRedefineNonExistentSite(t *testing.T) {
	runAll(t, checkRedefineNonExistentSite, defaultTestCases)
}

func checkRedefineNonExistentSite(t *testing.T, server *httptest.Server, testCase *TestCase) {
	for _, site := range []string{"", "ha", "admin-500"} {
		resp := addRule(server, &Rule{Site: site})
		shouldHaveStatusCode(t, http.StatusNotFound, resp)
	}
}

func TestDelay(t *testing.T) {
	runAll(t, checkDelay, defaultTestCases)
}

func checkDelay(t *testing.T, server *httptest.Server, testCase *TestCase) {
	shouldRespondIn(t,
		createGET(server.URL, "/", makeHost("0", TEST_ENDPOINT)),
		0, 0.1) // seconds
	shouldRespondIn(t,
		createGET(server.URL, "/", makeHost("1", TEST_ENDPOINT)),
		1, 1.1) // seconds
}

func TestStatus(t *testing.T) {
	runAll(t, checkStatus, defaultTestCases)
}

func checkStatus(t *testing.T, server *httptest.Server, testCase *TestCase) {
	for _, statusCode := range []int{200, 404, 500} {
		resp := GET(server.URL, "/", makeHost(strconv.Itoa(statusCode), TEST_ENDPOINT))
		shouldHaveStatusCode(t, statusCode, resp)
	}
}

func TestRuleCreation(t *testing.T) {
	runAll(t, checkRuleCreationTestCase, ruleCreationTestCases)
}

// TODO: refactor
func checkRuleCreationTestCase(t *testing.T, server *httptest.Server, testCase *TestCase) {
	prefix := testCase.adminUrlPathPrefix
	site := ""
	if prefix == "" {
		site = newSite(server, join(prefix, "/"), []byte("haha"))
	} else {
		resp := addRule(server, &Rule{Path: join(prefix, "/"), Body: []byte("haha")})
		shouldHaveStatusCode(t, http.StatusOK, resp)
	}
	bytesShouldBeEqual(t, readBody(GET(server.URL, "/", site)), []byte("haha"))
	resp := addRule(server, &Rule{Site: site, Path: join(prefix, "/test"), Body: []byte("hop"), Method: "GET"})
	shouldHaveStatusCode(t, http.StatusOK, resp)

	bytesShouldBeEqual(t, readBody(GET(server.URL, "/test", site)), []byte("hop"))
	resp = POST(server.URL, "/test", site, []byte(""))
	intShouldBeEqual(t, 404, resp.StatusCode)
	resp = addRule(server, &Rule{Site: site, Path: join(prefix, "/test"), Body: []byte("for POST"), Method: "POST",
		Delay: time.Duration(100) * time.Millisecond})
	shouldHaveStatusCode(t, http.StatusOK, resp)
	bytesShouldBeEqual(t, readBody(GET(server.URL, "/test", site)), []byte("hop"))
	bytesShouldBeEqual(t, readBody(POST(server.URL, "/test", site, []byte(""))), []byte("for POST"))
	shouldRespondIn(t, createPOST(server.URL, "/test", site, []byte("")), 0.1, 0.15)
}

func newSubDomainServer(testCase *TestCase) *Server {
	config := DEFAULT_CONFIG // copies DEFAULT_CONFIG
	config.endpoint = TEST_ENDPOINT
	config.createDefaultRules = testCase.createDefaultRules
	config.adminUrlPathPrefix = testCase.adminUrlPathPrefix
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
	req, err := http.NewRequest("GET", url+path, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Host = host
	return req
}

func readBody(resp *http.Response) []byte {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	return body
}

func makeHost(subdomain, host string) string {
	return fmt.Sprintf("%s.%s", subdomain, host)
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
	readBody(resp)
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

// TODO: rename, it returns a full domain, not a site
func newSite(server *httptest.Server, path string, response []byte) string {
	resp := POST(server.URL, fmt.Sprintf("%s?output=short&method=GET", path),
		makeHost("create", TEST_ENDPOINT), response)
	return string(readBody(resp))
}

func POST(url, path, host string, payload []byte) *http.Response {
	req := createPOST(url, path, host, payload)
	return do(req)
}

func createPOST(url, path, host string, payload []byte) *http.Request {
	req, err := http.NewRequest("POST", url+path, bytes.NewReader(payload))
	if err != nil {
		log.Fatal(err)
	}
	req.Host = host
	return req
}

// TODO: use normal GET-parameters builder, not Sprintf and string concatenation
func addRule(server *httptest.Server, rule *Rule) *http.Response {
	path := rule.Path
	path += "?method=" + rule.Method
	path += fmt.Sprintf("&delay=%f", rule.Delay.Seconds())
	return POST(server.URL, path, makeHost("admin-"+rule.Site, TEST_ENDPOINT),
		rule.Body)
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
