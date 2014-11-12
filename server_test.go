package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"
)

const HOST = "localhost:9999"

type TestCase struct {
	createDefaultRules  bool
	singleDomainUrlPath string
}

func TestZeroSite(t *testing.T) {
	server := newSubDomainServer(true, "")
	defer server.Close()
	shouldBeEqual(t, readBody(GET(server.URL, "/", makeHost("0", HOST))), DEFAULT_RESPONSE)
}

func TestDelay(t *testing.T) {
	server := newSubDomainServer(true, "")
	defer server.Close()
	shouldRespondIn(t, createGET(server.URL, "/", makeHost("0", HOST)), 0, 0.1)
	shouldRespondIn(t, createGET(server.URL, "/", makeHost("1", HOST)), 1, 1.1)
}

func TestStatus(t *testing.T) {
	server := newSubDomainServer(true, "")
	defer server.Close()
	for _, statusCode := range []int{200, 404, 500} {
		resp := GET(server.URL, "/", makeHost(strconv.Itoa(statusCode), HOST))
		intShouldBeEqual(t, statusCode, resp.StatusCode)
	}
}

func TestRuleCreation(t *testing.T) {
	runRuleCreationTestCase(t, TestCase{true, ""})
	runRuleCreationTestCase(t, TestCase{false, "/goslow"})
	runRuleCreationTestCase(t, TestCase{false, "/goslow/"})
	runRuleCreationTestCase(t, TestCase{false, "/te"})
	runRuleCreationTestCase(t, TestCase{false, "/te/"})
	runRuleCreationTestCase(t, TestCase{false, "/composite/path"})
}

func runRuleCreationTestCase(t *testing.T, testCase TestCase) {
	log.Printf("running test")
	server := newSubDomainServer(testCase.createDefaultRules, testCase.singleDomainUrlPath)
	prefix := testCase.singleDomainUrlPath
	defer server.Close()
	var site = ""
	if prefix == "" {
		site = newSite(server, join(prefix, "/"), "haha")
	} else {
		addRule(t, server, &Rule{Path: join(prefix, "/"), Body: "haha"})
	}
	shouldBeEqual(t, readBody(GET(server.URL, "/", site)), "haha")
	addRule(t, server, &Rule{Site: site, Path: join(prefix, "/test"), Body: "hop", Method: "GET"})
	shouldBeEqual(t, readBody(GET(server.URL, "/test", site)), "hop")
	resp := POST(server.URL, "/test", site, "")
	intShouldBeEqual(t, 404, resp.StatusCode)
	addRule(t, server, &Rule{Site: site, Path: join(prefix, "/test"), Body: "for POST", Method: "POST",
		Delay: time.Duration(100) * time.Millisecond})
	shouldBeEqual(t, readBody(GET(server.URL, "/test", site)), "hop")
	shouldBeEqual(t, readBody(POST(server.URL, "/test", site, "")), "for POST")
	shouldRespondIn(t, createPOST(server.URL, "/test", site, ""), 0.1, 0.15)
}

func newSubDomainServer(createDefaultRules bool, singleDomainUrlPath string) *httptest.Server {
	config := *DEFAULT_CONFIG
	config.endpoint = HOST
	config.createDefaultRules = createDefaultRules
	config.singleDomainUrlPath = singleDomainUrlPath
	handler := NewServer(&config)
	return httptest.NewServer(handler)
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

func addRule(t *testing.T, server *httptest.Server, rule *Rule) {
	path := rule.Path
	path += "?method=" + rule.Method
	path += fmt.Sprintf("&delay=%f", rule.Delay.Seconds())
	resp := POST(server.URL, path, makeHost("admin-"+rule.Site, HOST),
		rule.Body)
	intShouldBeEqual(t, 200, resp.StatusCode)
}

func join(elem ...string) string {
	shouldEndWithSlash := strings.HasSuffix(elem[len(elem)-1], "/")
	joined := path.Join(elem...)
	if shouldEndWithSlash && !strings.HasSuffix(joined, "/") {
		joined += "/"
	}
	return joined
}
