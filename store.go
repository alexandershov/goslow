package main

import (
	"net/http"
	"strings"
)

type Store interface {
	AddRule(rule *Rule) error
	GetHostRules(host string) ([]*Rule, error)
}

func NewStore(driver string, dataSource string) (Store, error) {
	if driver == "memory" {
		return NewMemoryStore()
	}
	return NewSqlStore(driver, dataSource)
}

func FindRule(store Store, r *http.Request) (rule *Rule, found bool, err error) {
	rules, err := store.GetHostRules(WithoutPort(r.Host))
	if err != nil {
		return nil, false, err
	}
	for _, rule := range rules {
		if rule.Match(r) {
			return rule, true, nil
		}
	}
	return nil, false, nil
}

func WithoutPort(host string) string {
	return strings.Split(host, ":")[0]
}
