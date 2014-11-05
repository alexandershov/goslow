package main

import (
	"net/http"
	"strings"
)

type Store interface {
	AddRule(rule *Rule)
	GetHostRules(host string) []*Rule
}

func FindRule(store Store, r *http.Request) (rule *Rule, found bool) {
	rules := store.GetHostRules(WithoutPort(r.Host))
	for _, rule := range rules {
		if rule.Match(r.URL.Path, r.Method) {
			return rule, true
		}
	}
	return nil, false
}

func WithoutPort(host string) string {
	return strings.Split(host, ":")[0]
}
