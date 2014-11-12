package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"net/http"
	"regexp"
	"time"
)

// Matches $1, $2, $3, ...
var POSTGRES_PLACEHOLDERS *regexp.Regexp = regexp.MustCompile("\\$\\d+")

type Storage struct {
	driver     string
	dataSource string
	db         *sql.DB
}

func NewStorage(driver string, dataSource string) (*Storage, error) {
	db, err := sql.Open(driver, dataSource)
	if err != nil {
		return nil, err
	}
	storage := &Storage{driver: driver, dataSource: dataSource, db: db}
	_, err = storage.db.Exec(storage.dialectify(CREATE_SCHEMA_IF_NOT_EXISTS_SQL))
	return storage, err
}

func (storage *Storage) FindRuleMatching(site string, req *http.Request) (rule *Rule, found bool, err error) {
	rules, err := storage.getSiteRules(site)
	if err != nil {
		return nil, false, err
	}
	for _, rule := range rules {
		if rule.Matches(req) {
			return rule, true, nil
		}
	}
	return nil, false, nil
}

func (storage *Storage) getSiteRules(site string) ([]*Rule, error) {
	rules := make([]*Rule, 0)
	rows, err := storage.db.Query(storage.dialectify(GET_SITE_RULES_SQL), site)
	if err != nil {
		return rules, err
	}
	defer rows.Close()

	for rows.Next() {
		rule, err := makeRule(rows)
		if err != nil {
			return rules, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (storage *Storage) dialectify(sql string) string {
	if storage.driver == "postgres" {
		return sql
	}
	return POSTGRES_PLACEHOLDERS.ReplaceAllString(sql, "?")
}

func makeRule(rows *sql.Rows) (*Rule, error) {
	rule := new(Rule)
	var headersJson string
	var delay int64
	err := rows.Scan(&rule.Site, &rule.Path, &rule.Method, &headersJson, &delay, &rule.StatusCode,
		&rule.Body)
	if err != nil {
		return rule, err
	}
	rule.Delay = time.Duration(delay)
	rule.Headers, err = jsonToStringMap(headersJson)
	return rule, err
}

func jsonToStringMap(js string) (map[string]string, error) {
	object := make(map[string]interface{})
	err := json.Unmarshal([]byte(js), &object)
	if err != nil {
		return nil, err
	}
	return objectToStringMap(object)
}

func objectToStringMap(object map[string]interface{}) (map[string]string, error) {
	m := make(map[string]string)
	for key, value := range object {
		switch value.(type) {
		case string:
			m[key] = value.(string)
		default:
			return nil, errors.New(fmt.Sprintf("Expecting string, got %+v", value))
		}
	}
	return m, nil
}

func (storage *Storage) UpsertRule(rule *Rule) error {
	tx, err := storage.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(storage.dialectify(DELETE_RULE_SQL), rule.Site, rule.Path, rule.Method)
	if err != nil {
		return err
	}
	headersJson, err := stringMapToJson(rule.Headers)
	if err != nil {
		return err
	}
	_, err = tx.Exec(storage.dialectify(CREATE_RULE_SQL), rule.Site, rule.Path, rule.Method,
		headersJson, int64(rule.Delay), rule.StatusCode, rule.Body)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func stringMapToJson(m map[string]string) (string, error) {
	b, err := json.Marshal(m)
	return string(b), err
}

func (storage *Storage) CreateSite(site string) error {
	_, err := storage.db.Exec(storage.dialectify(CREATE_SITE_SQL), site)
	return err
}

func (storage *Storage) ContainsSite(site string) (bool, error) {
	rows, err := storage.db.Query(storage.dialectify(GET_SITE_SQL), site)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	containsSite := rows.Next()
	return containsSite, rows.Err()
}
