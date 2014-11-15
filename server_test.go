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

func TestZeroSite(t *testing.T) {
	server, _ := newSubDomainServer(TestCase{createDefaultRules: true, singleDomainUrlPath: ""})
	defer server.Close()
	shouldBeEqual(t, readBody(GET(server.URL, "/", makeHost("0", HOST))), string(DEFAULT_BODY))
}

func TestRedefineBuiltinSites(t *testing.T) {
	server, _ := newSubDomainServer(TestCase{createDefaultRules: true, singleDomainUrlPath: ""})
	defer server.Close()
	addRule(t, server, &Rule{Site: "0", Path: "/test", Body: []byte("hop"), Method: "GET"}, http.StatusForbidden)
}

func TestDelay(t *testing.T) {
	server, _ := newSubDomainServer(TestCase{createDefaultRules: true, singleDomainUrlPath: ""})
	defer server.Close()
	shouldRespondIn(t, createGET(server.URL, "/", makeHost("0", HOST)), 0, 0.1)
	shouldRespondIn(t, createGET(server.URL, "/", makeHost("1", HOST)), 1, 1.1)
}

func TestStatus(t *testing.T) {
	server, _ := newSubDomainServer(TestCase{createDefaultRules: true, singleDomainUrlPath: ""})
	defer server.Close()
	for _, statusCode := range []int{200, 404, 500} {
		resp := GET(server.URL, "/", makeHost(strconv.Itoa(statusCode), HOST))
		intShouldBeEqual(t, statusCode, resp.StatusCode)
	}
}

func TestRuleCreation(t *testing.T) {
	for _, driver := range []string{"sqlite3", "postgres"} {
		dataSource := DATA_SOURCE[driver]
		runRuleCreationTestCase(t, TestCase{createDefaultRules: true,
			singleDomainUrlPath: "",
			driver:              driver,
			dataSource:          dataSource,
		})
		runRuleCreationTestCase(t, TestCase{createDefaultRules: false,
			singleDomainUrlPath: "/goslow",
			driver:              driver,
			dataSource:          dataSource,
		})
		runRuleCreationTestCase(t, TestCase{createDefaultRules: false,
			singleDomainUrlPath: "/goslow/",
			driver:              driver,
			dataSource:          dataSource,
		})
		runRuleCreationTestCase(t, TestCase{createDefaultRules: false,
			singleDomainUrlPath: "/te",
			driver:              driver,
			dataSource:          dataSource,
		})
		runRuleCreationTestCase(t, TestCase{createDefaultRules: false,
			singleDomainUrlPath: "/te/",
			driver:              driver,
			dataSource:          dataSource,
		})
		runRuleCreationTestCase(t, TestCase{createDefaultRules: false,
			singleDomainUrlPath: "/composite/path",
			driver:              driver,
			dataSource:          dataSource,
		})
	}
}

func runRuleCreationTestCase(t *testing.T, testCase TestCase) {
	log.Printf("running test")
	if testCase.driver == "postgres" && testing.Short() {
		return
	}
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
	shouldBeEqual(t, readBody(GET(server.URL, "/", site)), "haha")
	addRule(t, server, &Rule{Site: site, Path: join(prefix, "/test"), Body: []byte("hop"), Method: "GET"}, 200)
	shouldBeEqual(t, readBody(GET(server.URL, "/test", site)), "hop")
	resp := POST(server.URL, "/test", site, "")
	intShouldBeEqual(t, 404, resp.StatusCode)
	addRule(t, server, &Rule{Site: site, Path: join(prefix, "/test"), Body: []byte("for POST"), Method: "POST",
		Delay: time.Duration(100) * time.Millisecond}, 200)
	shouldBeEqual(t, readBody(GET(server.URL, "/test", site)), "hop")
	shouldBeEqual(t, readBody(POST(server.URL, "/test", site, "")), "for POST")
	shouldRespondIn(t, createPOST(server.URL, "/test", site, ""), 0.1, 0.15)
}

func newSubDomainServer(testCase TestCase) (*httptest.Server, *Server) {
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

func readBody(resp *http.Response) string {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	return string(body)
}

func makeHost(subdomain, host string) string {
	return fmt.Sprintf("%s.%s", subdomain, host)
}

func shouldBeEqual(t *testing.T, expected, actual string) {
	if expected != actual {
		t.Fatalf("<<%v>> != <<%v>>", expected, actual)
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
	return readBody(resp)
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
