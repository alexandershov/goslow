package main

import (
	"net/http"
	"time"
)

type Rule struct {
	Host           string
	Path           string
	Method         string
	Header         map[string]string
	Delay          time.Duration
	ResponseStatus int
	Response       string
}

func (rule *Rule) Match(r *http.Request) bool {
	return Match(rule.Path, r.URL.Path) && Match(rule.Method, r.Method)
}

func Match(pattern, name string) bool {
	// empty pattern matches anything
	if pattern == "" {
		return true
	}
	return pattern == name
}

type RulesByPathLen []*Rule

func (a RulesByPathLen) Len() int {
	return len(a)
}

func (a RulesByPathLen) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

// longer path goes first
// TODO: do secondary sort by .Method len
func (a RulesByPathLen) Less(i, j int) bool {
	return len(a[i].Path) > len(a[j].Path)
}
