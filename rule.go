package main

import (
	"net/http"
	"time"
)

type Rule struct {
	site           string
	path           string
	method         string
	headers         map[string]string
	delay         time.Duration
	responseStatus int
	responseBody      string
}

func (rule *Rule) Match(r *http.Request) bool {
	return Match(rule.path, r.URL.Path) && Match(rule.method, r.Method)
}

func Match(pattern, name string) bool {
	// empty pattern matches anything
	if pattern == "" {
		return true
	}
	return pattern == name
}
