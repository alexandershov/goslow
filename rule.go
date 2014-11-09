package main

import (
	"net/http"
	"time"
)

type Rule struct {
	Site           string
	Path           string
	Method         string
	Headers        map[string]string
	Delay          time.Duration
	ResponseStatus int
	ResponseBody   string
}

func (rule *Rule) Matches(req *http.Request) bool {
	return matches(rule.Path, req.URL.Path) && matches(rule.Method, req.Method)
}

func matches(pattern, name string) bool {
	// empty pattern matches anything
	if pattern == "" {
		return true
	}
	return pattern == name
}
