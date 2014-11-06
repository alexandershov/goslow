package main

import (
	"sort"
)

type MemoryStore struct {
	rules map[string][]*Rule
}

func NewMemoryStore() (Store, error) {
	return &MemoryStore{rules: make(map[string][]*Rule)}, nil
}

func (store *MemoryStore) AddRule(rule *Rule) error {
	rules, contains := store.rules[rule.Host]
	if !contains {
		rules = make([]*Rule, 0)
	}
	rules = append(rules, rule)
	store.rules[rule.Host] = rules
	sort.Sort(RulesByPathLen(rules))
	return nil
}

func (store *MemoryStore) GetHostRules(host string) ([]*Rule, error) {
	rules, contains := store.rules[host]
	if contains {
		return rules, nil
	}
	return make([]*Rule, 0), nil
}
