package main

import (
	"sort"
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

func (store *MemoryStore) GetHostRules(host string) []*Rule {
	rules, contains := store.rules[host]
	if contains {
		return rules
	}
	return make([]*Rule, 0)
}
