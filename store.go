package main

import (
	"net/http"
	"strings"
  "database/sql"
  "encoding/json"
  "errors"
  "fmt"
    _ "github.com/lib/pq"
  _ "github.com/mattn/go-sqlite3"
  "regexp"
  "time"
)

var AGNOSTIC_SQL *regexp.Regexp = regexp.MustCompile("\\$\\d+")

const DELETE_RULE_SQL = `
DELETE FROM rules
WHERE host = $1 AND path = $2 AND method = $3
`

const INSERT_RULE_SQL = `
INSERT INTO rules
(host, path, method, header, delay, response_status, response)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

const GET_RULES_SQL = `
SELECT host, path, method, header, delay, response_status, response
FROM rules
WHERE host = $1
ORDER BY path DESC
`

const INSERT_DOMAIN_SQL = `
INSERT INTO domains
(domain)
VALUES ($1)
`

const GET_DOMAIN_SQL = `
SELECT *
FROM domains
WHERE domain = $1
`

type Store struct {
  Driver     string
  DataSource string
  Db         *sql.DB
}

func NewStore(driver string, dataSource string) (*Store, error) {
  db, err := sql.Open(driver, dataSource)
  if err != nil {
    return nil, err
  }
  err = db.Ping()
  if err != nil {
    return nil, err
  }
  return &Store{Driver: driver, DataSource: dataSource, Db: db}, nil
}


func (store *Store) FindRule(key string, r *http.Request) (rule *Rule, found bool, err error) {
	rules, err := store.GetSiteRules(key)
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



func (store *Store) CreateRule(rule *Rule) error {
  _, err := store.Db.Exec(store.Agnostic(DELETE_RULE_SQL), rule.host, rule.path, rule.method)
  if err != nil {
    return err
  }
  _, err = store.Db.Exec(store.Agnostic(INSERT_RULE_SQL), rule.host, rule.path, rule.method,
    MapToJson(rule.header), rule.delay, rule.responseStatus, rule.response)
  return err
}

func (store *Store) Agnostic(sql string) string {
  if store.Driver == "postgres" {
    return sql
  }
  return AGNOSTIC_SQL.ReplaceAllString(sql, "?")
}

func (store *Store) GetSiteRules(host string) ([]*Rule, error) {
  rows, err := store.Db.Query(store.Agnostic(GET_RULES_SQL), host)
  if err != nil {
    return nil, err
  }
  defer rows.Close()
  rules := make([]*Rule, 0)
  for rows.Next() {
    rule, err := ReadRule(rows)
    if err != nil {
      return nil, err
    }
    rules = append(rules, rule)
  }
  return rules, nil
}

func ReadRule(rows *sql.Rows) (*Rule, error) {
  rule := new(Rule)
  var header string
  var delay int64
  rows.Scan(&rule.host, &rule.path, &rule.method, &header, &delay, &rule.responseStatus,
    &rule.response)
  var err error
  rule.header, err = JsonToMap(header)
  if err != nil {
    return nil, err
  }
  rule.delay = time.Duration(delay)
  return rule, nil
}

func JsonToMap(j string) (map[string]string, error) {
  parsed := make(map[string]interface{})
  err := json.Unmarshal([]byte(j), &parsed)
  if err != nil {
    return nil, err
  }
  m := make(map[string]string)
  for key, value := range parsed {
    switch t := value.(type) {
    case string:
      m[key] = value.(string)
    default:
      return nil, errors.New(fmt.Sprintf("Expecting string, got %T", t))
    }
  }
  return m, nil
}

func MapToJson(m map[string]string) string {
  b, _ := json.Marshal(m)
  return string(b)
}

func (store *Store) CreateSite(domain string) error {
  _, err := store.Db.Exec(store.Agnostic(INSERT_DOMAIN_SQL), domain)
  return err
}

func (store *Store) ContainsSite(domain string) (bool, error) {
  rows, err := store.Db.Query(store.Agnostic(GET_DOMAIN_SQL), domain)
  if err != nil {
    return false, err
  }
  defer rows.Close()
  return rows.Next(), nil
}
