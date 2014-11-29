package main

import (
	"net/http"
	"time"
)

// Rule describes path/HTTP/delay properties of the endpoint.
type Rule struct {
	Site       string
	Path       string
	Method     string
	Headers    map[string]string
	Delay      time.Duration
	StatusCode int
	Body       []byte
}

// Rule.Matches returns true if rule matches a given request.
func (rule *Rule) Matches(req *http.Request) bool {
	return matches(rule.Path, req.URL.Path) && matches(rule.Method, req.Method)
}

// matches returns true if pattern matches the name. Empty pattern matches anything.
// Pattern is just a string - all special characters (*?.) are not special and interpreted as is.
func matches(pattern, name string) bool {
	if pattern == ANY {
		return true
	}
	return pattern == name
}
