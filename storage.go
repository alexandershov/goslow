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

var POSTGRES_PLACEHOLDERS *regexp.Regexp = regexp.MustCompile("\\$\\d+")

const CREATE_SCHEMA_SQL = `
CREATE TABLE IF NOT EXISTS sites(
  site TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS rules (
  site TEXT, path TEXT, method TEXT, headers TEXT,
  delay BIGINT, response_status INT, response_body TEXT,
  PRIMARY KEY(site, path, method),
  FOREIGN KEY(site) REFERENCES sites(site)
);

`

const DELETE_RULE_SQL = `
DELETE FROM rules
WHERE site = $1 AND path = $2 AND method = $3
`

const CREATE_RULE_SQL = `
INSERT INTO rules
(site, method, headers, delay, response_status, response_body)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

const GET_SITE_RULES_SQL = `
SELECT site, path, method, headers, delay, response_status, response_body
FROM rules
WHERE site = $1
ORDER BY LENGTH(path) DESC, LENGTH(method) DESC
`

const CREATE_SITE_SQL = `
INSERT INTO sites
(site)
VALUES ($1)
`

const GET_SITE_SQL = `
SELECT *
FROM sites
WHERE site = $1
`

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
	_, err = storage.db.Exec(storage.dialectify(CREATE_SCHEMA_SQL))
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
	return rules, nil
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
	rows.Scan(&rule.site, &rule.path, &rule.method, &headersJson, &delay, &rule.responseStatus,
		&rule.responseBody)
	rule.delay = time.Duration(delay)
	var err error
	rule.headers, err = jsonToMap(headersJson)
	return rule, err
}

func jsonToMap(js string) (map[string]string, error) {
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
	_, err = tx.Exec(storage.dialectify(DELETE_RULE_SQL), rule.site, rule.path, rule.method)
	if err != nil {
		return err
	}
	_, err = tx.Exec(storage.dialectify(CREATE_RULE_SQL), rule.site, rule.path, rule.method,
		stringMapToJson(rule.headers), rule.delay, rule.responseStatus, rule.responseBody)
	return tx.Commit()
}

func stringMapToJson(m map[string]string) string {
	b, _ := json.Marshal(m)
	return string(b)
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
	return rows.Next(), nil
}
