package main

import (
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
	HOST    = "localhost:9999"
	TEST_DB = "goslow_test"
)

var DATA_SOURCE = map[string]string{
	"sqlite3":  ":memory:",
	"postgres": "postgres://localhost/" + TEST_DB,
}

type TestCase struct {
	createDefaultRules  bool
	singleDomainUrlPath string
	driver              string
	dataSource          string
}

func NewTestCase(createDefaultRules bool, singleDomainUrlPath string, driver string) *TestCase {
	dataSource, knownDriver := DATA_SOURCE[driver]
	if !knownDriver {
		log.Fatalf("unknown driver: <%s>", driver)
	}
	return &TestCase{
		createDefaultRules:  createDefaultRules,
		singleDomainUrlPath: singleDomainUrlPath,
		driver:              driver,
		dataSource:          dataSource,
	}
}

func (testCase *TestCase) skippable() bool {
	return testCase.driver == "postgres" && testing.Short()
}

type TestCases []*TestCase

var (
	defaultTestCases = TestCases{
		NewTestCase(true, "", "sqlite3"),
		NewTestCase(true, "", "postgres"),
	}

	// TODO: remove duplication
	ruleCreationTestCases = TestCases{
		NewTestCase(true, "", "sqlite3"),
		NewTestCase(true, "", "postgres"),

		NewTestCase(false, "/goslow", "sqlite3"),
		NewTestCase(false, "/goslow", "postgres"),

		NewTestCase(false, "/goslow/", "sqlite3"),
		NewTestCase(false, "/goslow/", "postgres"),

		NewTestCase(false, "/te", "sqlite3"),
		NewTestCase(false, "/te", "postgres"),

		NewTestCase(false, "/te/", "sqlite3"),
		NewTestCase(false, "/te/", "postgres"),

		NewTestCase(false, "/composite/path", "sqlite3"),
		NewTestCase(false, "/composite/path", "postgres"),
	}
)

func TestZeroSite(t *testing.T) {
	for _, testCase := range runnable(defaultTestCases) {
		runZeroSite(t, testCase)
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

func runZeroSite(t *testing.T, testCase *TestCase) {
	if testCase.driver == "postgres" {
		createDb(TEST_DB)
		defer dropDb(TEST_DB)
	}
	server, goSlowServer := newSubDomainServer(testCase)
	defer goSlowServer.storage.db.Close()
	defer server.Close()
	shouldBeEqual(t, readBody(GET(server.URL, "/", makeHost("0", HOST))), DEFAULT_BODY)
}

func TestRedefineBuiltinSites(t *testing.T) {
	for _, testCase := range runnable(defaultTestCases) {
		runRedefineBuiltinSites(t, testCase)
	}
}

func runRedefineBuiltinSites(t *testing.T, testCase *TestCase) {
	if testCase.driver == "postgres" {
		createDb(TEST_DB)
		defer dropDb(TEST_DB)
	}
	server, goSlowServer := newSubDomainServer(testCase)
	defer goSlowServer.storage.db.Close()
	defer server.Close()
	addRule(t, server, &Rule{Site: "0", Path: "/test", Body: []byte("hop"), Method: "GET"}, http.StatusForbidden)
}

func TestDelay(t *testing.T) {
	for _, testCase := range runnable(defaultTestCases) {
		runDelay(t, testCase)
	}
}

func runDelay(t *testing.T, testCase *TestCase) {
	if testCase.driver == "postgres" {
		createDb(TEST_DB)
		defer dropDb(TEST_DB)
	}
	server, goSlowServer := newSubDomainServer(testCase)
	defer goSlowServer.storage.db.Close()
	defer server.Close()
	shouldRespondIn(t, createGET(server.URL, "/", makeHost("0", HOST)), 0, 0.1)
	shouldRespondIn(t, createGET(server.URL, "/", makeHost("1", HOST)), 1, 1.1)
}

func TestStatus(t *testing.T) {
	for _, testCase := range runnable(defaultTestCases) {
		runStatus(t, testCase)
	}
}

func runStatus(t *testing.T, testCase *TestCase) {
	if testCase.driver == "postgres" {
		createDb(TEST_DB)
		defer dropDb(TEST_DB)
	}
	server, goSlowServer := newSubDomainServer(testCase)
	defer goSlowServer.storage.db.Close()
	defer server.Close()
	for _, statusCode := range []int{200, 404, 500} {
		resp := GET(server.URL, "/", makeHost(strconv.Itoa(statusCode), HOST))
		intShouldBeEqual(t, statusCode, resp.StatusCode)
	}
}

func TestRuleCreation(t *testing.T) {
	for _, testCase := range runnable(ruleCreationTestCases) {
		runRuleCreationTestCase(t, testCase)
	}
}

func runRuleCreationTestCase(t *testing.T, testCase *TestCase) {
	if testCase.driver == "postgres" {
		createDb(TEST_DB)
		defer dropDb(TEST_DB)
	}
	server, goSlowServer := newSubDomainServer(testCase)
	defer goSlowServer.storage.db.Close()
	prefix := testCase.singleDomainUrlPath
	defer server.Close()
	var site = ""
	if prefix == "" {
		site = newSite(server, join(prefix, "/"), "haha")
	} else {
		addRule(t, server, &Rule{Path: join(prefix, "/"), Body: []byte("haha")}, 200)
	}
	shouldBeEqual(t, readBody(GET(server.URL, "/", site)), []byte("haha"))
	addRule(t, server, &Rule{Site: site, Path: join(prefix, "/test"), Body: []byte("hop"), Method: "GET"}, 200)
	shouldBeEqual(t, readBody(GET(server.URL, "/test", site)), []byte("hop"))
	resp := POST(server.URL, "/test", site, "")
	intShouldBeEqual(t, 404, resp.StatusCode)
	addRule(t, server, &Rule{Site: site, Path: join(prefix, "/test"), Body: []byte("for POST"), Method: "POST",
		Delay: time.Duration(100) * time.Millisecond}, 200)
	shouldBeEqual(t, readBody(GET(server.URL, "/test", site)), []byte("hop"))
	shouldBeEqual(t, readBody(POST(server.URL, "/test", site, "")), []byte("for POST"))
	shouldRespondIn(t, createPOST(server.URL, "/test", site, ""), 0.1, 0.15)
}

func newSubDomainServer(testCase *TestCase) (*httptest.Server, *Server) {
	config := DEFAULT_CONFIG // copies DEFAULT_CONFIG
	config.endpoint = HOST
	config.createDefaultRules = testCase.createDefaultRules
	config.singleDomainUrlPath = testCase.singleDomainUrlPath
	if testCase.driver != "" {
		config.driver = testCase.driver
		config.dataSource = testCase.dataSource
	}
	handler := NewServer(&config)
	return httptest.NewServer(handler), handler
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

func shouldBeEqual(t *testing.T, expected, actual []byte) {
	if string(expected) != string(actual) {
		t.Fatalf("<<%v>> != <<%v>>", string(expected), string(actual))
	}
}

func intShouldBeEqual(t *testing.T, expected, actual int) {
	if expected != actual {
		t.Fatalf("<<%v>> != <<%v>>", expected, actual)
	}
}

func shouldRespondIn(t *testing.T, req *http.Request, min, max float64) {
	start := time.Now()
	resp := do(req)
	readBody(resp)
	duration := time.Since(start)
	minDuration := toDuration(min)
	maxDuration := toDuration(max)
	if duration > maxDuration || minDuration > duration {
		t.Fatalf("%s%s answered in %v. Not in the interval [%v; %v]",
			req.Host, req.URL.Path, duration, minDuration, maxDuration)
	}
}

func toDuration(seconds float64) time.Duration {
	return time.Duration(seconds*1000) * time.Millisecond
}

func newSite(server *httptest.Server, path, response string) string {
	resp := POST(server.URL, fmt.Sprintf("%s?output=short&method=GET", path),
		makeHost("create", HOST), response)
	return string(readBody(resp))
}

func POST(url, path, host, payload string) *http.Response {
	log.Printf("posting %s", path)
	req := createPOST(url, path, host, payload)
	return do(req)
}

func createPOST(url, path, host, payload string) *http.Request {
	req, err := http.NewRequest("POST", url+path, strings.NewReader(payload))
	if err != nil {
		log.Fatal(err)
	}
	req.Host = host
	return req
}

func addRule(t *testing.T, server *httptest.Server, rule *Rule, statusCode int) {
	path := rule.Path
	path += "?method=" + rule.Method
	path += fmt.Sprintf("&delay=%f", rule.Delay.Seconds())
	resp := POST(server.URL, path, makeHost("admin-"+rule.Site, HOST),
		string(rule.Body))
	intShouldBeEqual(t, statusCode, resp.StatusCode)
}

func join(elem ...string) string {
	shouldEndWithSlash := strings.HasSuffix(elem[len(elem)-1], "/")
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
