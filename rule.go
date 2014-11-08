package main

import (
	"net/http"
	"time"
)

type Rule struct {
	host           string
	path           string
	method         string
	header         map[string]string
	delay         time.Duration
	responseStatus int
	response       string
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
