package main

import (
	"net/http"
	"time"
)

// If Path/Method is an empty string, then endpoint handles
// any path/HTTP method.
type Endpoint struct {
	Site       string
	Path       string
	Method     string
	Headers    map[string]string
	Delay      time.Duration
	StatusCode int
	Response   []byte
}

func (endpoint *Endpoint) Matches(req *http.Request) bool {
	return matches(endpoint.Path, req.URL.Path) && matches(endpoint.Method, req.Method)
}

// Empty pattern matches anything.
// Pattern is just a string - special characters (*?.) are not special and interpreted as is.
func matches(pattern, s string) bool {
	if pattern == ANY_STRING {
		return true
	}
	return pattern == s
}
