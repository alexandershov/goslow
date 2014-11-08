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

func (rule *Rule) Matches(req *http.Request) bool {
	return Matches(rule.path, req.URL.Path) && Matches(rule.method, req.Method)
}

func Matches(pattern, name string) bool {
	// empty pattern matches anything
	if pattern == "" {
		return true
	}
	return pattern == name
}
