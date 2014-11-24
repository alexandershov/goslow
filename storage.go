package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"net/http"
	"regexp"
	"strings"
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
	_, err = storage.db.Exec(storage.dialectifySchema(CREATE_SCHEMA_IF_NOT_EXISTS_SQL))
	return storage, err
}

func (storage *Storage) FindRuleMatching(site string, req *http.Request) (rule *Rule, found bool, err error) {
	rules, err := storage.getRules(site)
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

func (storage *Storage) getRules(site string) ([]*Rule, error) {
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
	if storage.isPostgres() {
		return sql
	}
	return POSTGRES_PLACEHOLDERS.ReplaceAllString(sql, "?")
}

func (storage *Storage) isPostgres() bool {
	return storage.driver == "postgres"
}

func (storage *Storage) dialectifySchema(sql string) string {
	if storage.isPostgres() {
		return sql
	}
	// it's not pretty, but it covered by tests
	return strings.Replace(sql, " BYTEA,", " BLOB,", -1)
}

func makeRule(rows *sql.Rows) (*Rule, error) {
	rule := new(Rule)
	var headersJson string
	var delay int64
	err := rows.Scan(&rule.Site, &rule.Path, &rule.Method, &headersJson, &delay,
		&rule.StatusCode, &rule.Body)
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
			return nil, fmt.Errorf("Expecting string, got %+v", value)
		}
	}
	return m, nil
}

func (storage *Storage) SaveRule(rule *Rule) error {
	tx, err := storage.db.Begin()
	if err != nil {
		return err
	}
	// if tx gets commited then tx.Rollback() basically has no effect
	// if there's some error, then we always want to rollback
	defer tx.Rollback()
	// upsert as delete-n-insert isn't correct in all cases
	// (e.g concurrent upserts of the same rule will lead to "duplicate key value violates unique constraint")
	// but is practical enough because concurrent upserts of the same rule are going to be extremely rare
	_, err = tx.Exec(storage.dialectify(DELETE_RULE_SQL), rule.Site, rule.Path, rule.Method)
	if err != nil {
		return err
	}
	headersJson, err := stringMapToJson(rule.Headers)
	if err != nil {
		return err
	}
	_, err = tx.Exec(storage.dialectify(INSERT_RULE_SQL), rule.Site, rule.Path, rule.Method,
		headersJson, int64(rule.Delay), rule.StatusCode, rule.Body)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func stringMapToJson(m map[string]string) (js string, err error) {
	b, err := json.Marshal(m)
	return string(b), err
}

func (storage *Storage) CreateSite(site string) error {
	_, err := storage.db.Exec(storage.dialectify(INSERT_SITE_SQL), site)
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
