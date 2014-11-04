package main

import (
	"net/http"
	"sort"
	"strings"
)

type MemoryStore struct {
	rules map[string][]*Rule
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{rules: make(map[string][]*Rule)}
}

func (store *MemoryStore) AddRule(rule *Rule) {
	rules, contains := store.rules[rule.Host]
	if !contains {
		rules = make([]*Rule, 0)
	}
	rules = append(rules, rule)
	store.rules[rule.Host] = rules
	sort.Sort(RulesByPathLen(rules))
}

func (store *MemoryStore) FindRuleFor(r *http.Request) (rule *Rule, found bool) {
	rules, contains := store.rules[WithoutPort(r.Host)]
	if contains {
		for _, rule := range rules {
			if rule.Match(r.URL.Path, r.Method) {
				return rule, true
			}
		}
	}
	return nil, false
}

func WithoutPort(host string) string {
	return strings.Split(host, ":")[0]
}
