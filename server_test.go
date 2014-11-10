package main


import ("testing"
"net/http"
"time"
"net/http/httptest"
"log"
"fmt"
"io/ioutil"
)

const HOST = "goslow.link"

func TestZero(t *testing.T) {
  server := newSubDomainServer(true)
  defer server.Close()
  shouldBeEqual(t, readBody(GET(server.URL, "", makeHost("0"))), DEFAULT_RESPONSE)
}

func TestDelay(t *testing.T) {
  server := newSubDomainServer(true)
  defer server.Close()
  shouldRespondIn(t, createGET(server.URL, "", makeHost("0")), 0, 0.1)
  shouldRespondIn(t, createGET(server.URL, "", makeHost("1")), 1, 1.1)
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
  return doGET(req)
}

func doGET(req *http.Request) *http.Response {
  resp, err := new(http.Client).Do(req)
  if err != nil {
    log.Fatal(err)
  }
  return resp
}

func createGET(url, path, host string) *http.Request {
  req, err := http.NewRequest("GET", url + "/" + path, nil)
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

func shouldRespondIn(t *testing.T, req *http.Request, min, max float64) {
  start := time.Now()
  resp := doGET(req)
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
