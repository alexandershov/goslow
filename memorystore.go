package main

import (
	"fmt"
	"errors"
	"sort"
)

// TODO: add read/write locking
type MemoryStore struct {
	rules map[string][]*Rule
	domains map[string]bool
}

func NewMemoryStore() (Store, error) {
	store := &MemoryStore{rules: make(map[string][]*Rule),
		domains: make(map[string]bool)}
	return store, nil
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

func (store *MemoryStore) AddNewDomain(domain string) error {
	_, contains := store.domains[domain]
	if contains {
		return errors.New(fmt.Sprintf("Domain named %s already exists", domain))
	}
	store.domains[domain] = true
	return nil
}


func (store *MemoryStore) ContainsDomain(domain string) (bool, error) {
	_, contains := store.domains[domain]
	return contains, nil
}
