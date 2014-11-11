package main


import ("testing"
"net/http"
"time"
"net/http/httptest"
"log"
"fmt"
"strconv"
"strings"
"io/ioutil"
)

const HOST = "goslow.link"

type testCase struct {
  createDefaultRules bool

}

func TestZeroSite(t *testing.T) {
  server := newSubDomainServer(true)
  defer server.Close()
  shouldBeEqual(t, readBody(GET(server.URL, "/", makeHost("0"))), DEFAULT_RESPONSE)
}

func TestDelay(t *testing.T) {
  server := newSubDomainServer(true)
  defer server.Close()
  shouldRespondIn(t, createGET(server.URL, "/", makeHost("0")), 0, 0.1)
  shouldRespondIn(t, createGET(server.URL, "/", makeHost("1")), 1, 1.1)
}

func TestStatus(t *testing.T) {
  server := newSubDomainServer(true)
  defer server.Close()
  for _, statusCode := range []int{200, 404, 500} {
    resp := GET(server.URL, "/", makeHost(strconv.Itoa(statusCode)))
    intShouldBeEqual(t, statusCode, resp.StatusCode)
  }
}

func TestRuleCreation(t *testing.T)  {
  server := newSubDomainServer(true)
  defer server.Close()
  site := newSite(server, "", "haha")
  shouldBeEqual(t, readBody(GET(server.URL, "/", makeHost(site))), "haha")
  addRule(server, &Rule{Site: site, Path: "/test", ResponseBody: "hop", Method: "GET"})
  shouldBeEqual(t, readBody(GET(server.URL, "/test", makeHost(site))), "hop")
  resp := POST(server.URL, "/test", makeHost(site), "")
  intShouldBeEqual(t, 404, resp.StatusCode)
  addRule(server, &Rule{Site: site, Path: "/test", ResponseBody: "for POST", Method: "POST",
  Delay: time.Duration(100) * time.Millisecond})
  shouldBeEqual(t, readBody(GET(server.URL, "/test", makeHost(site))), "hop")
  shouldBeEqual(t, readBody(POST(server.URL, "/test", makeHost(site), "")), "for POST")
  shouldRespondIn(t, createPOST(server.URL, "/test", makeHost(site), ""), 0.1, 0.15)
}

func newSubDomainServer(createDefaultRules bool) *httptest.Server {
  config := *DEFAULT_CONFIG
  config.createDefaultRules = createDefaultRules
  config.singleDomainUrlPath = ""
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
  req, err := http.NewRequest("GET", url + path, nil)
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

func makeHost(subdomain string) string {
  return fmt.Sprintf("%s.%s", subdomain, HOST)
}

func shouldBeEqual(t *testing.T, expected, actual string) {
  if expected != actual {
    t.Errorf("<<%v>> != <<%v>>", expected, actual)
  }
}

func intShouldBeEqual(t *testing.T, expected, actual int) {
  if expected != actual {
    t.Errorf("<<%v>> != <<%v>>", expected, actual)
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
    t.Errorf("%s%s answered in %v. Not in the interval [%v; %v]",
    req.Host, req.URL.Path, duration, minDuration, maxDuration)
  }
}

func toDuration(seconds float64) time.Duration {
  return time.Duration(seconds * 1000) * time.Millisecond
}

func newSite(server *httptest.Server, path, response string) string {
  resp := POST(server.URL, fmt.Sprintf("%s?output=short&method=GET", path), makeHost("create"), response)
  return readBody(resp)
}

func POST(url, path, host, payload string) *http.Response {
  log.Printf("posting %s", path)
  req := createPOST(url, path, host, payload)
  return do(req)
}


func createPOST(url, path, host, payload string) *http.Request {
  req, err := http.NewRequest("POST", url + path, strings.NewReader(payload))
  if err != nil {
    log.Fatal(err)
  }
  req.Host = host
  return req
}

func addRule(server *httptest.Server, rule *Rule) {
  path := rule.Path
  path += "?method=" + rule.Method
  path += fmt.Sprintf("&delay=%f", rule.Delay.Seconds())
  POST(server.URL, path, makeHost("admin-" + rule.Site),
  rule.ResponseBody)
}
